package mimdb

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// base64StdEncoding is an alias for the standard base64 encoding, used by test
// helpers that parse tus Upload-Metadata headers.
var base64StdEncoding = base64.StdEncoding

// TestStorage_ListBuckets verifies that ListBuckets sends a GET to the correct
// path, passes cursor/limit query parameters, and deserializes the paginated
// envelope response into a Page[Bucket].
func TestStorage_ListBuckets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/v1/storage/testref/buckets" {
			t.Errorf("path = %q, want /v1/storage/testref/buckets", r.URL.Path)
		}
		if r.URL.Query().Get("cursor") != "cur1" {
			t.Errorf("cursor = %q, want %q", r.URL.Query().Get("cursor"), "cur1")
		}
		if r.URL.Query().Get("limit") != "10" {
			t.Errorf("limit = %q, want %q", r.URL.Query().Get("limit"), "10")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id":         "bucket-1",
					"project_id": "proj-1",
					"name":       "avatars",
					"public":     true,
					"created_at": "2024-01-01T00:00:00Z",
				},
				{
					"id":         "bucket-2",
					"project_id": "proj-1",
					"name":       "uploads",
					"public":     false,
					"created_at": "2024-06-15T00:00:00Z",
				},
			},
			"error": nil,
			"meta": map[string]any{
				"request_id":  "req-list",
				"next_cursor": "cur2",
				"has_more":    true,
			},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	page, err := client.Storage().ListBuckets(context.Background(), ListOptions{
		Cursor: "cur1",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(page.Data) != 2 {
		t.Fatalf("len(page.Data) = %d, want 2", len(page.Data))
	}
	if page.Data[0].Name != "avatars" {
		t.Errorf("Data[0].Name = %q, want %q", page.Data[0].Name, "avatars")
	}
	if !page.Data[0].Public {
		t.Errorf("Data[0].Public = false, want true")
	}
	if page.Data[1].Name != "uploads" {
		t.Errorf("Data[1].Name = %q, want %q", page.Data[1].Name, "uploads")
	}
	if page.NextCursor != "cur2" {
		t.Errorf("NextCursor = %q, want %q", page.NextCursor, "cur2")
	}
	if !page.HasMore {
		t.Error("HasMore = false, want true")
	}
}

// TestStorage_CreateBucket verifies that CreateBucket sends a POST with only
// name and public in the body, and returns the created *Bucket.
func TestStorage_CreateBucket(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/storage/testref/buckets" {
			t.Errorf("path = %q, want /v1/storage/testref/buckets", r.URL.Path)
		}

		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":         "bucket-new",
				"project_id": "proj-1",
				"name":       "photos",
				"public":     true,
				"created_at": "2024-03-01T00:00:00Z",
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-create"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	bucket, err := client.Storage().CreateBucket(context.Background(), CreateBucketRequest{
		Name:   "photos",
		Public: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify request body only has name + public.
	var body map[string]any
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	if body["name"] != "photos" {
		t.Errorf("body name = %v, want %q", body["name"], "photos")
	}
	if body["public"] != true {
		t.Errorf("body public = %v, want true", body["public"])
	}
	// Ensure no extra fields like file_size_limit or allowed_types.
	if _, ok := body["file_size_limit"]; ok {
		t.Error("body should not contain file_size_limit")
	}
	if _, ok := body["allowed_types"]; ok {
		t.Error("body should not contain allowed_types")
	}

	if bucket == nil {
		t.Fatal("bucket is nil")
	}
	if bucket.Name != "photos" {
		t.Errorf("bucket.Name = %q, want %q", bucket.Name, "photos")
	}
	if !bucket.Public {
		t.Error("bucket.Public = false, want true")
	}
}

// TestStorage_UpdateBucket verifies that UpdateBucket sends a PATCH to the
// correct path with the update body, and handles a 204 No Content response.
func TestStorage_UpdateBucket(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %q, want PATCH", r.Method)
		}
		if r.URL.Path != "/v1/storage/testref/buckets/photos" {
			t.Errorf("path = %q, want /v1/storage/testref/buckets/photos", r.URL.Path)
		}

		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	pub := true
	limit := int64(1048576)
	err := client.Storage().UpdateBucket(context.Background(), "photos", UpdateBucketRequest{
		Public:        &pub,
		FileSizeLimit: &limit,
		AllowedTypes:  []string{"image/png", "image/jpeg"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the PATCH body.
	var body map[string]any
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	if body["public"] != true {
		t.Errorf("body public = %v, want true", body["public"])
	}
	if body["file_size_limit"] != float64(1048576) {
		t.Errorf("body file_size_limit = %v, want 1048576", body["file_size_limit"])
	}
	types, ok := body["allowed_types"].([]any)
	if !ok || len(types) != 2 {
		t.Errorf("body allowed_types = %v, want [image/png, image/jpeg]", body["allowed_types"])
	}
}

// TestStorage_DeleteBucket verifies that DeleteBucket sends a DELETE to the
// correct path and handles a 204 No Content response.
func TestStorage_DeleteBucket(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		if r.URL.Path != "/v1/storage/testref/buckets/photos" {
			t.Errorf("path = %q, want /v1/storage/testref/buckets/photos", r.URL.Path)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	err := client.Storage().DeleteBucket(context.Background(), "photos")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestStorage_Upload verifies that Upload sends a POST with the raw body and
// correct Content-Type header to /v1/storage/{ref}/object/{bucket}/{path}.
func TestStorage_Upload(t *testing.T) {
	var capturedBody []byte
	var capturedContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/storage/testref/object/photos/avatar.png" {
			t.Errorf("path = %q, want /v1/storage/testref/object/photos/avatar.png", r.URL.Path)
		}

		capturedContentType = r.Header.Get("Content-Type")

		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":  nil,
			"error": nil,
			"meta":  map[string]string{"request_id": "req-upload"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	content := "fake-png-data"
	err := client.Storage().Upload(
		context.Background(),
		"photos",
		"avatar.png",
		strings.NewReader(content),
		UploadOptions{ContentType: "image/png"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedContentType != "image/png" {
		t.Errorf("Content-Type = %q, want %q", capturedContentType, "image/png")
	}
	if string(capturedBody) != content {
		t.Errorf("body = %q, want %q", string(capturedBody), content)
	}
}

// TestStorage_Download verifies that Download sends a GET to the correct path
// and returns an io.ReadCloser containing the response body.
func TestStorage_Download(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/v1/storage/testref/object/photos/avatar.png" {
			t.Errorf("path = %q, want /v1/storage/testref/object/photos/avatar.png", r.URL.Path)
		}

		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("fake-png-bytes"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	rc, err := client.Storage().Download(context.Background(), "photos", "avatar.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	if string(data) != "fake-png-bytes" {
		t.Errorf("body = %q, want %q", string(data), "fake-png-bytes")
	}
}

// TestStorage_DeleteObject verifies that Delete sends a DELETE to
// /v1/storage/{ref}/object/{bucket}/{path}.
func TestStorage_DeleteObject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		if r.URL.Path != "/v1/storage/testref/object/photos/avatar.png" {
			t.Errorf("path = %q, want /v1/storage/testref/object/photos/avatar.png", r.URL.Path)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	err := client.Storage().Delete(context.Background(), "photos", "avatar.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestStorage_ListObjects verifies that List sends a GET to
// /v1/storage/{ref}/object/{bucket} with pagination and prefix query
// parameters, and deserializes the paginated envelope into Page[StorageObject].
func TestStorage_ListObjects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/v1/storage/testref/object/photos" {
			t.Errorf("path = %q, want /v1/storage/testref/object/photos", r.URL.Path)
		}
		if r.URL.Query().Get("prefix") != "avatars/" {
			t.Errorf("prefix = %q, want %q", r.URL.Query().Get("prefix"), "avatars/")
		}
		if r.URL.Query().Get("limit") != "5" {
			t.Errorf("limit = %q, want %q", r.URL.Query().Get("limit"), "5")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"name":       "avatars/user1.png",
					"size":       12345,
					"mime_type":  "image/png",
					"created_at": "2024-01-10T00:00:00Z",
				},
			},
			"error": nil,
			"meta": map[string]any{
				"request_id":  "req-list-obj",
				"next_cursor": "",
				"has_more":    false,
			},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	page, err := client.Storage().List(context.Background(), "photos", ListOptions{
		Prefix: "avatars/",
		Limit:  5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(page.Data) != 1 {
		t.Fatalf("len(page.Data) = %d, want 1", len(page.Data))
	}
	if page.Data[0].Name != "avatars/user1.png" {
		t.Errorf("Data[0].Name = %q, want %q", page.Data[0].Name, "avatars/user1.png")
	}
	if page.Data[0].Size != 12345 {
		t.Errorf("Data[0].Size = %d, want 12345", page.Data[0].Size)
	}
	if page.Data[0].MimeType != "image/png" {
		t.Errorf("Data[0].MimeType = %q, want %q", page.Data[0].MimeType, "image/png")
	}
	if page.HasMore {
		t.Error("HasMore = true, want false")
	}
}

// TestStorage_SignedURL verifies that SignedURL sends a POST to
// /v1/storage/{ref}/sign/{bucket}/{path} with expires_in in the body, and
// extracts the signedURL field from the envelope response.
func TestStorage_SignedURL(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/storage/testref/sign/photos/avatar.png" {
			t.Errorf("path = %q, want /v1/storage/testref/sign/photos/avatar.png", r.URL.Path)
		}

		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"signedURL": "https://cdn.example.com/signed/photos/avatar.png?token=abc123",
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-sign"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	url, err := client.Storage().SignedURL(context.Background(), "photos", "avatar.png", 3600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify request body has expires_in.
	var body map[string]any
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	if body["expires_in"] != float64(3600) {
		t.Errorf("body expires_in = %v, want 3600", body["expires_in"])
	}

	expected := "https://cdn.example.com/signed/photos/avatar.png?token=abc123"
	if url != expected {
		t.Errorf("signedURL = %q, want %q", url, expected)
	}
}

// TestStorage_PublicURL verifies that PublicURL constructs the correct URL
// without making an HTTP call.
func TestStorage_PublicURL(t *testing.T) {
	client := NewClient("https://api.mimdb.dev", Options{
		ProjectRef: "myref",
		APIKey:     "test-key",
	})

	url := client.Storage().PublicURL("photos", "avatar.png")
	expected := "https://api.mimdb.dev/v1/storage/myref/public/photos/avatar.png"
	if url != expected {
		t.Errorf("PublicURL = %q, want %q", url, expected)
	}
}

// TestStorage_PublicURL_NestedPath verifies that PublicURL handles nested
// object paths correctly.
func TestStorage_PublicURL_NestedPath(t *testing.T) {
	client := NewClient("https://api.mimdb.dev", Options{
		ProjectRef: "myref",
		APIKey:     "test-key",
	})

	url := client.Storage().PublicURL("assets", "images/products/hero.jpg")
	expected := "https://api.mimdb.dev/v1/storage/myref/public/assets/images/products/hero.jpg"
	if url != expected {
		t.Errorf("PublicURL = %q, want %q", url, expected)
	}
}

// TestStorage_CreateResumableUpload verifies that CreateResumableUpload sends
// a POST with the required tus protocol headers and extracts the upload ID from
// the Location response header.
func TestStorage_CreateResumableUpload(t *testing.T) {
	var capturedHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/storage/testref/upload/resumable" {
			t.Errorf("path = %q, want /v1/storage/testref/upload/resumable", r.URL.Path)
		}

		capturedHeaders = r.Header.Clone()

		// Return a Location header containing the upload ID.
		w.Header().Set("Location", "/v1/storage/testref/upload/resumable/upload-abc-123")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	upload, err := client.Storage().CreateResumableUpload(
		context.Background(),
		"photos",
		"large-video.mp4",
		10485760, // 10MB
		UploadOptions{ContentType: "video/mp4"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify tus protocol headers.
	if v := capturedHeaders.Get("Tus-Resumable"); v != "1.0.0" {
		t.Errorf("Tus-Resumable = %q, want %q", v, "1.0.0")
	}
	if v := capturedHeaders.Get("Upload-Length"); v != "10485760" {
		t.Errorf("Upload-Length = %q, want %q", v, "10485760")
	}
	if v := capturedHeaders.Get("Content-Length"); v != "0" {
		t.Errorf("Content-Length = %q, want %q", v, "0")
	}

	// Verify Upload-Metadata contains base64-encoded bucket, object, and content type.
	metadata := capturedHeaders.Get("Upload-Metadata")
	if metadata == "" {
		t.Fatal("Upload-Metadata header is empty")
	}
	// Parse the metadata: "key base64value,key2 base64value2"
	metaPairs := parseUploadMetadata(metadata)
	if v, ok := metaPairs["bucketName"]; !ok || v != "photos" {
		t.Errorf("Upload-Metadata bucketName = %q, want %q", v, "photos")
	}
	if v, ok := metaPairs["objectName"]; !ok || v != "large-video.mp4" {
		t.Errorf("Upload-Metadata objectName = %q, want %q", v, "large-video.mp4")
	}
	if v, ok := metaPairs["contentType"]; !ok || v != "video/mp4" {
		t.Errorf("Upload-Metadata contentType = %q, want %q", v, "video/mp4")
	}

	// Verify the returned ResumableUpload has the correct upload ID.
	if upload == nil {
		t.Fatal("upload is nil")
	}
	if upload.UploadID() != "upload-abc-123" {
		t.Errorf("UploadID = %q, want %q", upload.UploadID(), "upload-abc-123")
	}
}

// TestStorage_ResumableUpload_SendChunk verifies that SendChunk sends a PATCH
// with the correct tus headers, Upload-Offset, Content-Type, and raw body.
func TestStorage_ResumableUpload_SendChunk(t *testing.T) {
	var capturedHeaders http.Header
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %q, want PATCH", r.Method)
		}
		if r.URL.Path != "/v1/storage/testref/upload/resumable/upload-abc-123" {
			t.Errorf("path = %q, want /v1/storage/testref/upload/resumable/upload-abc-123", r.URL.Path)
		}

		capturedHeaders = r.Header.Clone()

		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	upload := newResumableUpload(client.Storage(), "upload-abc-123", "photos", "large-video.mp4")

	chunkData := []byte("chunk-of-binary-data")
	err := upload.SendChunk(context.Background(), 1024, chunkData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify tus headers.
	if v := capturedHeaders.Get("Tus-Resumable"); v != "1.0.0" {
		t.Errorf("Tus-Resumable = %q, want %q", v, "1.0.0")
	}
	if v := capturedHeaders.Get("Upload-Offset"); v != "1024" {
		t.Errorf("Upload-Offset = %q, want %q", v, "1024")
	}
	if v := capturedHeaders.Get("Content-Type"); v != "application/offset+octet-stream" {
		t.Errorf("Content-Type = %q, want %q", v, "application/offset+octet-stream")
	}

	// Verify body.
	if string(capturedBody) != "chunk-of-binary-data" {
		t.Errorf("body = %q, want %q", string(capturedBody), "chunk-of-binary-data")
	}
}

// TestStorage_ResumableUpload_Status verifies that Status sends a HEAD request
// with the tus header and parses the Upload-Offset from the response.
func TestStorage_ResumableUpload_Status(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("method = %q, want HEAD", r.Method)
		}
		if r.URL.Path != "/v1/storage/testref/upload/resumable/upload-abc-123" {
			t.Errorf("path = %q, want /v1/storage/testref/upload/resumable/upload-abc-123", r.URL.Path)
		}
		if v := r.Header.Get("Tus-Resumable"); v != "1.0.0" {
			t.Errorf("Tus-Resumable = %q, want %q", v, "1.0.0")
		}

		w.Header().Set("Upload-Offset", "524288")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	upload := newResumableUpload(client.Storage(), "upload-abc-123", "photos", "large-video.mp4")

	offset, err := upload.Status(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if offset != 524288 {
		t.Errorf("offset = %d, want 524288", offset)
	}
}

// TestStorage_ResumableUpload_Cancel verifies that Cancel sends a DELETE
// request with the tus header.
func TestStorage_ResumableUpload_Cancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		if r.URL.Path != "/v1/storage/testref/upload/resumable/upload-abc-123" {
			t.Errorf("path = %q, want /v1/storage/testref/upload/resumable/upload-abc-123", r.URL.Path)
		}
		if v := r.Header.Get("Tus-Resumable"); v != "1.0.0" {
			t.Errorf("Tus-Resumable = %q, want %q", v, "1.0.0")
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	upload := newResumableUpload(client.Storage(), "upload-abc-123", "photos", "large-video.mp4")

	err := upload.Cancel(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// parseUploadMetadata is a test helper that parses the tus Upload-Metadata
// header format ("key base64val,key2 base64val2") into a decoded key-value map.
func parseUploadMetadata(header string) map[string]string {
	result := make(map[string]string)
	pairs := strings.Split(header, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), " ", 2)
		if len(parts) != 2 {
			continue
		}
		decoded, err := base64Decode(parts[1])
		if err != nil {
			continue
		}
		result[parts[0]] = decoded
	}
	return result
}

// base64Decode is a test helper that decodes a standard base64 string.
func base64Decode(s string) (string, error) {
	b, err := base64StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// TestStorage_RequiresProjectRef verifies that all storage methods return an
// error when the client is configured without a ProjectRef.
func TestStorage_RequiresProjectRef(t *testing.T) {
	client := NewClient("https://api.mimdb.dev", Options{
		AdminSecret: "admin-secret",
	})
	ctx := context.Background()

	t.Run("ListBuckets", func(t *testing.T) {
		_, err := client.Storage().ListBuckets(ctx, ListOptions{})
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("CreateBucket", func(t *testing.T) {
		_, err := client.Storage().CreateBucket(ctx, CreateBucketRequest{Name: "b"})
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("UpdateBucket", func(t *testing.T) {
		err := client.Storage().UpdateBucket(ctx, "b", UpdateBucketRequest{})
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("DeleteBucket", func(t *testing.T) {
		err := client.Storage().DeleteBucket(ctx, "b")
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("Upload", func(t *testing.T) {
		err := client.Storage().Upload(ctx, "b", "p", strings.NewReader("x"), UploadOptions{})
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("Download", func(t *testing.T) {
		_, err := client.Storage().Download(ctx, "b", "p")
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		err := client.Storage().Delete(ctx, "b", "p")
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("List", func(t *testing.T) {
		_, err := client.Storage().List(ctx, "b", ListOptions{})
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("SignedURL", func(t *testing.T) {
		_, err := client.Storage().SignedURL(ctx, "b", "p", 60)
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("CreateResumableUpload", func(t *testing.T) {
		_, err := client.Storage().CreateResumableUpload(ctx, "b", "p", 100, UploadOptions{})
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})
}
