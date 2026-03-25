// Package transport provides the HTTP communication layer for the MimDB Go SDK.
//
// It supports two response modes:
//   - Envelope mode (Do, DoList) for native MimDB APIs that wrap responses in
//     {data, error, meta}.
//   - Raw JSON mode (DoJSON) for PostgREST proxy endpoints that return plain
//     JSON without the envelope.
//
// Additionally, DoRaw and DoUpload support binary/streaming use cases such as
// file downloads and tus resumable uploads.
package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Config holds transport-level configuration that is set once at client creation
// and reused across all requests.
type Config struct {
	// APIKey is sent as the "apikey" header on every request.
	APIKey string

	// AdminSecret is sent as "Authorization: Bearer <secret>" unless
	// overridden by RequestOptions.AccessToken.
	AdminSecret string

	// HTTPClient allows callers to supply a custom *http.Client (e.g. for
	// timeouts, proxies, or TLS configuration). If nil, a default client is
	// used.
	HTTPClient *http.Client
}

// RequestOptions holds per-request overrides that are merged on top of the
// defaults from Config.
type RequestOptions struct {
	// Headers contains additional or override headers. These are applied last,
	// so they take priority over all defaults.
	Headers map[string]string

	// AccessToken, if set, sends "Authorization: Bearer <token>" instead of
	// the AdminSecret from Config. This is used when acting on behalf of an
	// authenticated end-user.
	AccessToken string
}

// HTTPClient handles HTTP requests for MimDB APIs.
type HTTPClient struct {
	baseURL string
	config  Config
	http    *http.Client
}

// NewHTTPClient creates a new transport client bound to the given base URL.
// If Config.HTTPClient is nil, a default http.Client is used.
func NewHTTPClient(baseURL string, config Config) *HTTPClient {
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &HTTPClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		config:  config,
		http:    httpClient,
	}
}

// TransportError holds structured error fields returned by the API. This is an
// internal type; the calling package converts it to the public *mimdb.APIError.
type TransportError struct {
	// Code is the machine-readable error code (e.g. "AUTH-0001", "PGRST204").
	Code string

	// Message is the human-readable error description.
	Message string

	// Detail provides additional context. PostgREST uses "details" (plural)
	// in its JSON; both are normalized into this field.
	Detail string

	// HTTPStatus is the HTTP status code from the response.
	HTTPStatus int

	// RequestID is extracted from the envelope meta when available.
	RequestID string
}

// Error implements the error interface. The format is "CODE: message" or
// "CODE: message (detail)" when a detail string is present.
func (e *TransportError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Detail)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// ListResult holds raw paginated data from envelope responses. The Data field
// is kept as json.RawMessage so callers can decode it into their target type.
type ListResult struct {
	// Data is the raw JSON array from the "data" field of the envelope.
	Data json.RawMessage

	// NextCursor is an opaque token for fetching the next page.
	NextCursor string

	// HasMore indicates whether additional pages exist.
	HasMore bool
}

// envelope is the internal representation of the MimDB response wrapper.
type envelope struct {
	Data  json.RawMessage `json:"data"`
	Error json.RawMessage `json:"error"`
	Meta  envelopeMeta    `json:"meta"`
}

// envelopeMeta holds pagination and request tracking fields from the envelope.
type envelopeMeta struct {
	RequestID  string `json:"request_id"`
	NextCursor string `json:"next_cursor"`
	HasMore    bool   `json:"has_more"`
}

// envelopeError is the error object within the MimDB envelope.
type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail"`
}

// postgrestError is the error format returned by PostgREST proxy endpoints.
// Note: PostgREST uses "details" (plural) and "hint" rather than "detail".
type postgrestError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details"`
	Hint    string `json:"hint"`
}

// Do sends a request and unwraps the MimDB envelope ({data, error, meta}).
// The "data" field is decoded into dest. If the envelope contains an error,
// a *TransportError is returned.
func (c *HTTPClient) Do(ctx context.Context, method, path string, body any, dest any, opts ...RequestOptions) error {
	resp, err := c.doRequest(ctx, method, path, body, opts)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	return c.decodeEnvelope(resp, dest)
}

// DoList sends a request and extracts paginated envelope data along with cursor
// metadata. The raw JSON array is stored in dest.Data for caller-side decoding.
func (c *HTTPClient) DoList(ctx context.Context, method, path string, body any, dest *ListResult, opts ...RequestOptions) error {
	resp, err := c.doRequest(ctx, method, path, body, opts)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("transport: failed to decode envelope: %w", err)
	}

	if tErr := c.extractEnvelopeError(&env, resp.StatusCode); tErr != nil {
		return tErr
	}

	dest.Data = env.Data
	dest.NextCursor = env.Meta.NextCursor
	dest.HasMore = env.Meta.HasMore
	return nil
}

// DoJSON sends a request and decodes the response as plain JSON (no envelope).
// This is used for PostgREST proxy endpoints. PostgREST errors are detected by
// non-2xx status codes and parsed from the PostgREST error format.
func (c *HTTPClient) DoJSON(ctx context.Context, method, path string, body any, dest any, opts ...RequestOptions) error {
	resp, err := c.doRequest(ctx, method, path, body, opts)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	if resp.StatusCode >= 400 {
		return c.decodePostgRESTError(resp)
	}

	if dest != nil {
		if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
			return fmt.Errorf("transport: failed to decode JSON response: %w", err)
		}
	}
	return nil
}

// DoRaw sends a request and returns the raw *http.Response. The caller is
// responsible for closing the response body. This is useful for streaming
// downloads or inspecting response headers directly.
func (c *HTTPClient) DoRaw(ctx context.Context, method, path string, opts ...RequestOptions) (*http.Response, error) {
	req, err := c.buildRequest(ctx, method, path, nil, opts)
	if err != nil {
		return nil, err
	}
	return c.http.Do(req)
}

// DoUpload sends a request with a raw body (io.Reader) and arbitrary headers.
// This is intended for file uploads (e.g. tus resumable protocol). If the
// server returns a non-2xx status, the response body is parsed as a MimDB
// envelope error.
func (c *HTTPClient) DoUpload(ctx context.Context, method, path string, body io.Reader, opts ...RequestOptions) error {
	req, err := c.buildRequest(ctx, method, path, nil, opts)
	if err != nil {
		return err
	}
	if body != nil {
		req.Body = io.NopCloser(body)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("transport: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	if resp.StatusCode >= 400 {
		return c.decodeUploadError(resp)
	}

	return nil
}

// buildRequest creates an *http.Request with headers applied in priority order:
//  1. Default Content-Type: application/json (overridable)
//  2. Config-level "apikey" header
//  3. Config-level Authorization: Bearer <AdminSecret>
//  4. RequestOptions.AccessToken overrides Authorization
//  5. RequestOptions.Headers override everything
func (c *HTTPClient) buildRequest(ctx context.Context, method, path string, body any, opts []RequestOptions) (*http.Request, error) {
	url := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("transport: failed to encode request body: %w", err)
		}
		bodyReader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("transport: failed to create request: %w", err)
	}

	// 1. Default Content-Type.
	req.Header.Set("Content-Type", "application/json")

	// 2. Config-level apikey.
	if c.config.APIKey != "" {
		req.Header.Set("apikey", c.config.APIKey)
	}

	// 3. Config-level Authorization.
	if c.config.AdminSecret != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.AdminSecret)
	}

	// Apply per-request options if provided.
	if len(opts) > 0 {
		opt := opts[0]

		// 4. AccessToken overrides AdminSecret.
		if opt.AccessToken != "" {
			req.Header.Set("Authorization", "Bearer "+opt.AccessToken)
		}

		// 5. Per-request headers override everything.
		for k, v := range opt.Headers {
			req.Header.Set(k, v)
		}
	}

	return req, nil
}

// doRequest is a convenience wrapper that builds and executes the request.
func (c *HTTPClient) doRequest(ctx context.Context, method, path string, body any, opts []RequestOptions) (*http.Response, error) {
	req, err := c.buildRequest(ctx, method, path, body, opts)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("transport: request failed: %w", err)
	}
	return resp, nil
}

// decodeEnvelope reads the full envelope, checks for errors, and decodes the
// data field into dest.
func (c *HTTPClient) decodeEnvelope(resp *http.Response, dest any) error {
	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("transport: failed to decode envelope: %w", err)
	}

	if tErr := c.extractEnvelopeError(&env, resp.StatusCode); tErr != nil {
		return tErr
	}

	if dest != nil && len(env.Data) > 0 && string(env.Data) != "null" {
		if err := json.Unmarshal(env.Data, dest); err != nil {
			return fmt.Errorf("transport: failed to decode data: %w", err)
		}
	}

	return nil
}

// extractEnvelopeError checks the envelope for an error object and returns a
// *TransportError if one is present.
func (c *HTTPClient) extractEnvelopeError(env *envelope, statusCode int) *TransportError {
	if len(env.Error) == 0 || string(env.Error) == "null" {
		// No error in the envelope, but a non-2xx status is still an error.
		if statusCode >= 400 {
			return &TransportError{
				Code:       fmt.Sprintf("HTTP-%d", statusCode),
				Message:    http.StatusText(statusCode),
				HTTPStatus: statusCode,
				RequestID:  env.Meta.RequestID,
			}
		}
		return nil
	}

	var envErr envelopeError
	if err := json.Unmarshal(env.Error, &envErr); err != nil {
		return &TransportError{
			Code:       fmt.Sprintf("HTTP-%d", statusCode),
			Message:    "failed to parse error from envelope",
			HTTPStatus: statusCode,
			RequestID:  env.Meta.RequestID,
		}
	}

	return &TransportError{
		Code:       envErr.Code,
		Message:    envErr.Message,
		Detail:     envErr.Detail,
		HTTPStatus: statusCode,
		RequestID:  env.Meta.RequestID,
	}
}

// decodePostgRESTError parses a PostgREST-format error response body.
func (c *HTTPClient) decodePostgRESTError(resp *http.Response) *TransportError {
	var pgErr postgrestError
	if err := json.NewDecoder(resp.Body).Decode(&pgErr); err != nil {
		return &TransportError{
			Code:       fmt.Sprintf("HTTP-%d", resp.StatusCode),
			Message:    "failed to parse PostgREST error",
			HTTPStatus: resp.StatusCode,
		}
	}

	return &TransportError{
		Code:       pgErr.Code,
		Message:    pgErr.Message,
		Detail:     pgErr.Details,
		HTTPStatus: resp.StatusCode,
	}
}

// decodeUploadError attempts to parse the response body as a MimDB envelope
// error. If parsing fails, it falls back to a generic transport error.
func (c *HTTPClient) decodeUploadError(resp *http.Response) *TransportError {
	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return &TransportError{
			Code:       fmt.Sprintf("HTTP-%d", resp.StatusCode),
			Message:    http.StatusText(resp.StatusCode),
			HTTPStatus: resp.StatusCode,
		}
	}

	if tErr := c.extractEnvelopeError(&env, resp.StatusCode); tErr != nil {
		return tErr
	}

	return &TransportError{
		Code:       fmt.Sprintf("HTTP-%d", resp.StatusCode),
		Message:    http.StatusText(resp.StatusCode),
		HTTPStatus: resp.StatusCode,
	}
}
