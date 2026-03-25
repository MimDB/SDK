package mimdb

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/MimDB/SDK/go/internal/transport"
)

// tusVersion is the tus resumable upload protocol version supported by this
// SDK. All tus requests include this value in the Tus-Resumable header.
const tusVersion = "1.0.0"

// StorageClient provides access to the MimDB Storage API for managing buckets
// and objects (upload, download, delete, list, signed URLs, and public URLs).
//
// Obtain a StorageClient via [Client.Storage]. It requires a ProjectRef to be
// configured on the parent Client, since all storage endpoints are scoped to a
// specific project.
type StorageClient struct {
	client *Client
}

// CreateBucketRequest holds the parameters for creating a new storage bucket.
// Only the bucket name and public visibility flag are accepted at creation time;
// use [StorageClient.UpdateBucket] to configure file size limits and allowed
// MIME types.
type CreateBucketRequest struct {
	// Name is the unique name for the bucket within the project.
	Name string `json:"name"`

	// Public controls whether objects in the bucket are publicly accessible
	// without authentication.
	Public bool `json:"public"`
}

// UpdateBucketRequest holds the parameters for updating an existing storage
// bucket. All fields are optional; only non-nil/non-empty values are sent.
type UpdateBucketRequest struct {
	// Public changes the bucket's public visibility. A nil value leaves the
	// current setting unchanged.
	Public *bool `json:"public,omitempty"`

	// FileSizeLimit sets the maximum file size in bytes for uploads to this
	// bucket. A nil value leaves the current limit unchanged.
	FileSizeLimit *int64 `json:"file_size_limit,omitempty"`

	// AllowedTypes restricts uploads to these MIME types. An empty slice
	// leaves the current setting unchanged.
	AllowedTypes []string `json:"allowed_types,omitempty"`
}

// signedURLRequest is the request body for generating a signed URL.
type signedURLRequest struct {
	ExpiresIn int `json:"expires_in"`
}

// signedURLResponse is the envelope data returned by the signed URL endpoint.
type signedURLResponse struct {
	SignedURL string `json:"signedURL"`
}

// storageBasePath returns the URL prefix for storage endpoints scoped to the
// given project ref.
func storageBasePath(ref string) string {
	return fmt.Sprintf("/v1/storage/%s", ref)
}

// buildListQuery appends cursor, limit, and prefix query parameters to the
// given base path. Only non-zero values are included.
func buildListQuery(basePath string, opts ListOptions) string {
	params := url.Values{}
	if opts.Cursor != "" {
		params.Set("cursor", opts.Cursor)
	}
	if opts.Limit > 0 {
		params.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Prefix != "" {
		params.Set("prefix", opts.Prefix)
	}
	if encoded := params.Encode(); encoded != "" {
		return basePath + "?" + encoded
	}
	return basePath
}

// parseRawError reads the response body from a DoRaw call and attempts to parse
// it as a MimDB envelope error. If parsing succeeds, a structured *APIError is
// returned. For HEAD responses (no body) or unparseable bodies, a generic
// *APIError with the HTTP status code is returned.
func parseRawError(resp *http.Response) *APIError {
	// HEAD responses have no body; return a generic error.
	if resp.Body == nil || resp.Request != nil && resp.Request.Method == http.MethodHead {
		return &APIError{
			Code:       fmt.Sprintf("HTTP-%d", resp.StatusCode),
			Message:    http.StatusText(resp.StatusCode),
			HTTPStatus: resp.StatusCode,
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil || len(body) == 0 {
		return &APIError{
			Code:       fmt.Sprintf("HTTP-%d", resp.StatusCode),
			Message:    http.StatusText(resp.StatusCode),
			HTTPStatus: resp.StatusCode,
		}
	}

	// Try to parse as a MimDB envelope: {"data":..., "error":{...}, "meta":{...}}
	var env struct {
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Detail  string `json:"detail"`
		} `json:"error"`
		Meta struct {
			RequestID string `json:"request_id"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(body, &env); err == nil && env.Error != nil && env.Error.Code != "" {
		return &APIError{
			Code:       env.Error.Code,
			Message:    env.Error.Message,
			Detail:     env.Error.Detail,
			HTTPStatus: resp.StatusCode,
			RequestID:  env.Meta.RequestID,
		}
	}

	return &APIError{
		Code:       fmt.Sprintf("HTTP-%d", resp.StatusCode),
		Message:    http.StatusText(resp.StatusCode),
		HTTPStatus: resp.StatusCode,
	}
}

// ---------- Bucket Operations ----------

// ListBuckets retrieves a paginated list of storage buckets for the project.
// Use ListOptions to control cursor-based pagination.
//
//	page, err := client.Storage().ListBuckets(ctx, mimdb.ListOptions{Limit: 20})
func (s *StorageClient) ListBuckets(ctx context.Context, opts ListOptions) (*Page[Bucket], error) {
	if err := s.client.requireProjectRef(); err != nil {
		return nil, err
	}

	basePath := fmt.Sprintf("%s/buckets", storageBasePath(s.client.projectRef))
	path := buildListQuery(basePath, opts)

	var result transport.ListResult
	if err := s.client.transport.DoList(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, wrapTransportError(err)
	}

	var buckets []Bucket
	if len(result.Data) > 0 && string(result.Data) != "null" {
		if err := json.Unmarshal(result.Data, &buckets); err != nil {
			return nil, fmt.Errorf("mimdb: failed to decode buckets: %w", err)
		}
	}

	return &Page[Bucket]{
		Data:       buckets,
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	}, nil
}

// CreateBucket creates a new storage bucket with the given name and public
// visibility. Returns the newly created Bucket.
//
//	bucket, err := client.Storage().CreateBucket(ctx, mimdb.CreateBucketRequest{
//	    Name:   "avatars",
//	    Public: true,
//	})
func (s *StorageClient) CreateBucket(ctx context.Context, req CreateBucketRequest) (*Bucket, error) {
	if err := s.client.requireProjectRef(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("%s/buckets", storageBasePath(s.client.projectRef))

	var bucket Bucket
	if err := s.client.transport.Do(ctx, http.MethodPost, path, req, &bucket); err != nil {
		return nil, wrapTransportError(err)
	}
	return &bucket, nil
}

// UpdateBucket updates an existing bucket's configuration by name. Only fields
// set in the request are modified. The server responds with 204 No Content on
// success.
//
//	pub := true
//	err := client.Storage().UpdateBucket(ctx, "avatars", mimdb.UpdateBucketRequest{
//	    Public: &pub,
//	})
func (s *StorageClient) UpdateBucket(ctx context.Context, name string, req UpdateBucketRequest) error {
	if err := s.client.requireProjectRef(); err != nil {
		return err
	}

	path := fmt.Sprintf("%s/buckets/%s", storageBasePath(s.client.projectRef), name)

	if err := s.client.transport.Do(ctx, http.MethodPatch, path, req, nil); err != nil {
		return wrapTransportError(err)
	}
	return nil
}

// DeleteBucket deletes a storage bucket by name. The bucket must be empty
// before it can be deleted. The server responds with 204 No Content on success.
//
//	err := client.Storage().DeleteBucket(ctx, "avatars")
func (s *StorageClient) DeleteBucket(ctx context.Context, name string) error {
	if err := s.client.requireProjectRef(); err != nil {
		return err
	}

	path := fmt.Sprintf("%s/buckets/%s", storageBasePath(s.client.projectRef), name)

	if err := s.client.transport.Do(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return wrapTransportError(err)
	}
	return nil
}

// ---------- Object Operations ----------

// Upload uploads a file to the specified bucket and path. The body is streamed
// directly to the server. Use UploadOptions.ContentType to set the MIME type;
// if empty, the server will attempt to detect it.
//
//	f, _ := os.Open("avatar.png")
//	err := client.Storage().Upload(ctx, "photos", "avatar.png", f, mimdb.UploadOptions{
//	    ContentType: "image/png",
//	})
func (s *StorageClient) Upload(ctx context.Context, bucket, path string, body io.Reader, opts UploadOptions) error {
	if err := s.client.requireProjectRef(); err != nil {
		return err
	}

	urlPath := fmt.Sprintf("%s/object/%s/%s", storageBasePath(s.client.projectRef), bucket, path)

	reqOpts := transport.RequestOptions{
		Headers: make(map[string]string),
	}
	if opts.ContentType != "" {
		reqOpts.Headers["Content-Type"] = opts.ContentType
	}

	if err := s.client.transport.DoUpload(ctx, http.MethodPost, urlPath, body, reqOpts); err != nil {
		return wrapTransportError(err)
	}
	return nil
}

// Download retrieves the contents of an object from the specified bucket and
// path. The caller is responsible for closing the returned io.ReadCloser.
//
//	rc, err := client.Storage().Download(ctx, "photos", "avatar.png")
//	if err != nil { ... }
//	defer rc.Close()
//	data, _ := io.ReadAll(rc)
func (s *StorageClient) Download(ctx context.Context, bucket, path string) (io.ReadCloser, error) {
	if err := s.client.requireProjectRef(); err != nil {
		return nil, err
	}

	urlPath := fmt.Sprintf("%s/object/%s/%s", storageBasePath(s.client.projectRef), bucket, path)

	resp, err := s.client.transport.DoRaw(ctx, http.MethodGet, urlPath)
	if err != nil {
		return nil, wrapTransportError(err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return nil, parseRawError(resp)
	}

	return resp.Body, nil
}

// Delete removes an object from the specified bucket and path. The server
// responds with 204 No Content on success.
//
//	err := client.Storage().Delete(ctx, "photos", "avatar.png")
func (s *StorageClient) Delete(ctx context.Context, bucket, path string) error {
	if err := s.client.requireProjectRef(); err != nil {
		return err
	}

	urlPath := fmt.Sprintf("%s/object/%s/%s", storageBasePath(s.client.projectRef), bucket, path)

	if err := s.client.transport.Do(ctx, http.MethodDelete, urlPath, nil, nil); err != nil {
		return wrapTransportError(err)
	}
	return nil
}

// List retrieves a paginated list of objects in the specified bucket. Use
// ListOptions to control cursor-based pagination and prefix filtering.
//
//	page, err := client.Storage().List(ctx, "photos", mimdb.ListOptions{
//	    Prefix: "avatars/",
//	    Limit:  20,
//	})
func (s *StorageClient) List(ctx context.Context, bucket string, opts ListOptions) (*Page[StorageObject], error) {
	if err := s.client.requireProjectRef(); err != nil {
		return nil, err
	}

	basePath := fmt.Sprintf("%s/object/%s", storageBasePath(s.client.projectRef), bucket)
	urlPath := buildListQuery(basePath, opts)

	var result transport.ListResult
	if err := s.client.transport.DoList(ctx, http.MethodGet, urlPath, nil, &result); err != nil {
		return nil, wrapTransportError(err)
	}

	var objects []StorageObject
	if len(result.Data) > 0 && string(result.Data) != "null" {
		if err := json.Unmarshal(result.Data, &objects); err != nil {
			return nil, fmt.Errorf("mimdb: failed to decode storage objects: %w", err)
		}
	}

	return &Page[StorageObject]{
		Data:       objects,
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	}, nil
}

// SignedURL generates a temporary signed URL for the specified object. The
// expiresIn parameter controls how many seconds the URL remains valid.
//
//	url, err := client.Storage().SignedURL(ctx, "photos", "avatar.png", 3600)
func (s *StorageClient) SignedURL(ctx context.Context, bucket, path string, expiresIn int) (string, error) {
	if err := s.client.requireProjectRef(); err != nil {
		return "", err
	}

	urlPath := fmt.Sprintf("%s/sign/%s/%s", storageBasePath(s.client.projectRef), bucket, path)

	var resp signedURLResponse
	if err := s.client.transport.Do(ctx, http.MethodPost, urlPath, signedURLRequest{ExpiresIn: expiresIn}, &resp); err != nil {
		return "", wrapTransportError(err)
	}

	return resp.SignedURL, nil
}

// PublicURL constructs the public URL for an object in a public bucket. No HTTP
// call is made; the URL is assembled from the client's base URL, project ref,
// bucket name, and object path.
//
//	url := client.Storage().PublicURL("photos", "avatar.png")
func (s *StorageClient) PublicURL(bucket, path string) string {
	return fmt.Sprintf(
		"%s/v1/storage/%s/public/%s/%s",
		s.client.baseURL,
		s.client.projectRef,
		bucket,
		path,
	)
}

// ---------- Resumable Uploads (tus protocol) ----------

// ResumableUpload tracks an in-progress tus resumable upload. It is created by
// [StorageClient.CreateResumableUpload] and provides methods to send chunks,
// check upload status, and cancel the upload.
//
// The tus protocol (v1.0.0) allows large files to be uploaded in chunks with
// the ability to resume after network interruptions.
type ResumableUpload struct {
	client   *StorageClient
	uploadID string
	bucket   string
	path     string
}

// UploadID returns the server-assigned identifier for this resumable upload.
// This value is extracted from the Location header returned by the tus creation
// request and is used in all subsequent chunk, status, and cancel requests.
func (u *ResumableUpload) UploadID() string {
	return u.uploadID
}

// newResumableUpload creates a ResumableUpload with pre-set fields. This is
// used internally and in tests to construct an upload handle without calling
// the server.
func newResumableUpload(client *StorageClient, uploadID, bucket, path string) *ResumableUpload {
	return &ResumableUpload{
		client:   client,
		uploadID: uploadID,
		bucket:   bucket,
		path:     path,
	}
}

// resumableBasePath returns the URL prefix for tus resumable upload endpoints
// scoped to the given project ref.
func resumableBasePath(ref string) string {
	return fmt.Sprintf("/v1/storage/%s/upload/resumable", ref)
}

// encodeTusMetadata encodes key-value pairs into the tus Upload-Metadata header
// format: "key base64value,key2 base64value2". Keys are sent as-is; values are
// base64-encoded per the tus specification.
func encodeTusMetadata(pairs [][2]string) string {
	parts := make([]string, 0, len(pairs))
	for _, kv := range pairs {
		encoded := base64.StdEncoding.EncodeToString([]byte(kv[1]))
		parts = append(parts, kv[0]+" "+encoded)
	}
	return strings.Join(parts, ",")
}

// CreateResumableUpload initiates a tus resumable upload for a file of the
// given size. The server responds with a Location header containing the upload
// URL; the upload ID is extracted from that URL.
//
// Use the returned [ResumableUpload] to send chunks, check progress, or cancel
// the upload.
//
//	upload, err := client.Storage().CreateResumableUpload(ctx, "videos", "intro.mp4", fileSize, mimdb.UploadOptions{
//	    ContentType: "video/mp4",
//	})
//	if err != nil { ... }
//	err = upload.SendChunk(ctx, 0, firstChunk)
func (s *StorageClient) CreateResumableUpload(ctx context.Context, bucket, path string, size int64, opts UploadOptions) (*ResumableUpload, error) {
	if err := s.client.requireProjectRef(); err != nil {
		return nil, err
	}

	urlPath := resumableBasePath(s.client.projectRef)

	contentType := opts.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	metadata := encodeTusMetadata([][2]string{
		{"bucketName", bucket},
		{"objectName", path},
		{"contentType", contentType},
	})

	reqOpts := transport.RequestOptions{
		Headers: map[string]string{
			"Tus-Resumable":  tusVersion,
			"Upload-Length":  strconv.FormatInt(size, 10),
			"Upload-Metadata": metadata,
			"Content-Length": "0",
		},
	}

	resp, err := s.client.transport.DoRaw(ctx, http.MethodPost, urlPath, reqOpts)
	if err != nil {
		return nil, wrapTransportError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, parseRawError(resp)
	}

	location := resp.Header.Get("Location")
	if location == "" {
		return nil, fmt.Errorf("mimdb: tus creation response missing Location header")
	}

	// Extract the upload ID from the last path segment of the Location URL.
	uploadID := location[strings.LastIndex(location, "/")+1:]
	if uploadID == "" {
		return nil, fmt.Errorf("mimdb: failed to extract upload ID from Location: %s", location)
	}

	return &ResumableUpload{
		client:   s,
		uploadID: uploadID,
		bucket:   bucket,
		path:     path,
	}, nil
}

// SendChunk uploads a chunk of data starting at the given byte offset. The
// offset must match the server's current Upload-Offset for the upload to
// succeed.
//
//	err := upload.SendChunk(ctx, 0, firstChunk)
//	err = upload.SendChunk(ctx, int64(len(firstChunk)), secondChunk)
func (u *ResumableUpload) SendChunk(ctx context.Context, offset int64, data []byte) error {
	if err := u.client.client.requireProjectRef(); err != nil {
		return err
	}

	urlPath := fmt.Sprintf("%s/%s", resumableBasePath(u.client.client.projectRef), u.uploadID)

	reqOpts := transport.RequestOptions{
		Headers: map[string]string{
			"Tus-Resumable": tusVersion,
			"Upload-Offset": strconv.FormatInt(offset, 10),
			"Content-Type":  "application/offset+octet-stream",
		},
	}

	if err := u.client.client.transport.DoUpload(ctx, http.MethodPatch, urlPath, bytes.NewReader(data), reqOpts); err != nil {
		return wrapTransportError(err)
	}
	return nil
}

// Status queries the server for the current upload progress. It returns the
// byte offset that has been successfully received so far.
//
//	offset, err := upload.Status(ctx)
//	fmt.Printf("uploaded %d bytes so far\n", offset)
func (u *ResumableUpload) Status(ctx context.Context) (int64, error) {
	if err := u.client.client.requireProjectRef(); err != nil {
		return 0, err
	}

	urlPath := fmt.Sprintf("%s/%s", resumableBasePath(u.client.client.projectRef), u.uploadID)

	reqOpts := transport.RequestOptions{
		Headers: map[string]string{
			"Tus-Resumable": tusVersion,
		},
	}

	resp, err := u.client.client.transport.DoRaw(ctx, http.MethodHead, urlPath, reqOpts)
	if err != nil {
		return 0, wrapTransportError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return 0, parseRawError(resp)
	}

	offsetStr := resp.Header.Get("Upload-Offset")
	if offsetStr == "" {
		return 0, fmt.Errorf("mimdb: tus status response missing Upload-Offset header")
	}

	offset, err := strconv.ParseInt(offsetStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("mimdb: failed to parse Upload-Offset %q: %w", offsetStr, err)
	}

	return offset, nil
}

// Cancel aborts the resumable upload and requests that the server delete any
// partially uploaded data.
//
//	err := upload.Cancel(ctx)
func (u *ResumableUpload) Cancel(ctx context.Context) error {
	if err := u.client.client.requireProjectRef(); err != nil {
		return err
	}

	urlPath := fmt.Sprintf("%s/%s", resumableBasePath(u.client.client.projectRef), u.uploadID)

	reqOpts := transport.RequestOptions{
		Headers: map[string]string{
			"Tus-Resumable": tusVersion,
		},
	}

	resp, err := u.client.client.transport.DoRaw(ctx, http.MethodDelete, urlPath, reqOpts)
	if err != nil {
		return wrapTransportError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return parseRawError(resp)
	}

	return nil
}
