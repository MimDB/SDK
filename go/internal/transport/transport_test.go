package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestDo_SuccessUnwrap verifies that Do correctly unwraps the MimDB envelope
// and deserializes the "data" field into the destination struct.
func TestDo_SuccessUnwrap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"name":"test"},"error":null,"meta":{"request_id":"r"}}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, Config{
		APIKey:      "test-key",
		AdminSecret: "test-secret",
	})

	var dest struct {
		Name string `json:"name"`
	}
	err := client.Do(context.Background(), http.MethodGet, "/test", nil, &dest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dest.Name != "test" {
		t.Errorf("Name = %q, want %q", dest.Name, "test")
	}
}

// TestDo_ErrorUnwrap verifies that Do returns a TransportError with the correct
// fields when the server returns an error envelope.
func TestDo_ErrorUnwrap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"data":null,"error":{"code":"AUTH-0001","message":"invalid token"},"meta":{"request_id":"req-456"}}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, Config{
		APIKey:      "test-key",
		AdminSecret: "test-secret",
	})

	err := client.Do(context.Background(), http.MethodGet, "/test", nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var tErr *TransportError
	if !errors.As(err, &tErr) {
		t.Fatalf("expected *TransportError, got %T: %v", err, err)
	}
	if tErr.Code != "AUTH-0001" {
		t.Errorf("Code = %q, want %q", tErr.Code, "AUTH-0001")
	}
	if tErr.Message != "invalid token" {
		t.Errorf("Message = %q, want %q", tErr.Message, "invalid token")
	}
	if tErr.HTTPStatus != http.StatusUnauthorized {
		t.Errorf("HTTPStatus = %d, want %d", tErr.HTTPStatus, http.StatusUnauthorized)
	}
	if tErr.RequestID != "req-456" {
		t.Errorf("RequestID = %q, want %q", tErr.RequestID, "req-456")
	}
}

// TestDoList_Pagination verifies that DoList extracts raw data, cursor, and
// has_more from the paginated envelope response.
func TestDoList_Pagination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"id":1},{"id":2}],"error":null,"meta":{"request_id":"r","next_cursor":"abc","has_more":true}}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, Config{
		APIKey:      "test-key",
		AdminSecret: "test-secret",
	})

	var result ListResult
	err := client.DoList(context.Background(), http.MethodGet, "/items", nil, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.NextCursor != "abc" {
		t.Errorf("NextCursor = %q, want %q", result.NextCursor, "abc")
	}
	if !result.HasMore {
		t.Error("HasMore = false, want true")
	}

	// Verify raw data is preserved.
	var items []struct{ ID int `json:"id"` }
	if err := json.Unmarshal(result.Data, &items); err != nil {
		t.Fatalf("failed to unmarshal Data: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("len(items) = %d, want 2", len(items))
	}
	if items[0].ID != 1 || items[1].ID != 2 {
		t.Errorf("items = %+v, want [{1},{2}]", items)
	}
}

// TestDoJSON_RawResponse verifies that DoJSON decodes plain JSON (no envelope)
// directly into the destination, as used by PostgREST proxy endpoints.
func TestDoJSON_RawResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":1},{"id":2}]`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, Config{
		APIKey:      "test-key",
		AdminSecret: "test-secret",
	})

	var dest []struct {
		ID int `json:"id"`
	}
	err := client.DoJSON(context.Background(), http.MethodGet, "/rest/items", nil, &dest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dest) != 2 {
		t.Fatalf("len(dest) = %d, want 2", len(dest))
	}
	if dest[0].ID != 1 || dest[1].ID != 2 {
		t.Errorf("dest = %+v, want [{1},{2}]", dest)
	}
}

// TestDoJSON_PostgRESTError verifies that DoJSON correctly parses PostgREST-format
// errors (which use "details" plural, not "detail" singular).
func TestDoJSON_PostgRESTError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":"PGRST204","message":"Column not found","details":"Column 'foo' does not exist","hint":"Check column name"}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, Config{
		APIKey:      "test-key",
		AdminSecret: "test-secret",
	})

	var dest []struct{}
	err := client.DoJSON(context.Background(), http.MethodGet, "/rest/items", nil, &dest)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var tErr *TransportError
	if !errors.As(err, &tErr) {
		t.Fatalf("expected *TransportError, got %T: %v", err, err)
	}
	if tErr.Code != "PGRST204" {
		t.Errorf("Code = %q, want %q", tErr.Code, "PGRST204")
	}
	if tErr.Message != "Column not found" {
		t.Errorf("Message = %q, want %q", tErr.Message, "Column not found")
	}
	if tErr.Detail != "Column 'foo' does not exist" {
		t.Errorf("Detail = %q, want %q", tErr.Detail, "Column 'foo' does not exist")
	}
	if tErr.HTTPStatus != http.StatusBadRequest {
		t.Errorf("HTTPStatus = %d, want %d", tErr.HTTPStatus, http.StatusBadRequest)
	}
}

// TestDo_SetsDefaultHeaders verifies that the transport sets the apikey header
// and Authorization: Bearer header from Config on every request.
func TestDo_SetsDefaultHeaders(t *testing.T) {
	var capturedHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":null,"error":null,"meta":{"request_id":"r"}}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, Config{
		APIKey:      "my-api-key",
		AdminSecret: "my-admin-secret",
	})

	err := client.Do(context.Background(), http.MethodGet, "/test", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := capturedHeaders.Get("apikey"); got != "my-api-key" {
		t.Errorf("apikey header = %q, want %q", got, "my-api-key")
	}
	if got := capturedHeaders.Get("Authorization"); got != "Bearer my-admin-secret" {
		t.Errorf("Authorization header = %q, want %q", got, "Bearer my-admin-secret")
	}
	if got := capturedHeaders.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type header = %q, want %q", got, "application/json")
	}
}

// TestDo_PerRequestHeaders verifies that RequestOptions.Headers are added to the
// request and can override default headers.
func TestDo_PerRequestHeaders(t *testing.T) {
	var capturedHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":null,"error":null,"meta":{"request_id":"r"}}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, Config{
		APIKey:      "my-api-key",
		AdminSecret: "my-admin-secret",
	})

	opts := RequestOptions{
		Headers: map[string]string{
			"Prefer":       "return=representation",
			"Content-Type": "text/plain",
			"X-Custom":     "custom-value",
		},
	}
	err := client.Do(context.Background(), http.MethodGet, "/test", nil, nil, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := capturedHeaders.Get("Prefer"); got != "return=representation" {
		t.Errorf("Prefer header = %q, want %q", got, "return=representation")
	}
	if got := capturedHeaders.Get("Content-Type"); got != "text/plain" {
		t.Errorf("Content-Type header = %q, want %q (should be overridden)", got, "text/plain")
	}
	if got := capturedHeaders.Get("X-Custom"); got != "custom-value" {
		t.Errorf("X-Custom header = %q, want %q", got, "custom-value")
	}
	// Default headers should still be present.
	if got := capturedHeaders.Get("apikey"); got != "my-api-key" {
		t.Errorf("apikey header = %q, want %q", got, "my-api-key")
	}
}

// TestDo_AccessTokenOverride verifies that RequestOptions.AccessToken overrides
// the AdminSecret in the Authorization header.
func TestDo_AccessTokenOverride(t *testing.T) {
	var capturedHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":null,"error":null,"meta":{"request_id":"r"}}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, Config{
		APIKey:      "my-api-key",
		AdminSecret: "my-admin-secret",
	})

	opts := RequestOptions{
		AccessToken: "user-jwt-token",
	}
	err := client.Do(context.Background(), http.MethodGet, "/test", nil, nil, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := capturedHeaders.Get("Authorization"); got != "Bearer user-jwt-token" {
		t.Errorf("Authorization header = %q, want %q", got, "Bearer user-jwt-token")
	}
}

// TestDo_204NoContent verifies that a 204 No Content response is handled
// gracefully without attempting to parse an empty body.
func TestDo_204NoContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, Config{
		APIKey:      "test-key",
		AdminSecret: "test-secret",
	})

	err := client.Do(context.Background(), http.MethodDelete, "/test", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestDoRaw_ReturnsRawResponse verifies that DoRaw returns the raw HTTP response
// without any processing.
func TestDoRaw_ReturnsRawResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("raw binary data"))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, Config{
		APIKey:      "test-key",
		AdminSecret: "test-secret",
	})

	resp, err := client.DoRaw(context.Background(), http.MethodGet, "/file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	if string(body) != "raw binary data" {
		t.Errorf("body = %q, want %q", string(body), "raw binary data")
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// TestDoUpload_SendsRawBody verifies that DoUpload sends a raw body with the
// correct headers applied.
func TestDoUpload_SendsRawBody(t *testing.T) {
	var capturedBody string
	var capturedHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		b, _ := io.ReadAll(r.Body)
		capturedBody = string(b)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, Config{
		APIKey:      "test-key",
		AdminSecret: "test-secret",
	})

	body := strings.NewReader("file contents here")
	opts := RequestOptions{
		Headers: map[string]string{
			"Content-Type":   "application/octet-stream",
			"Upload-Offset":  "0",
			"Tus-Resumable":  "1.0.0",
		},
	}
	err := client.DoUpload(context.Background(), http.MethodPatch, "/upload/abc", body, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedBody != "file contents here" {
		t.Errorf("body = %q, want %q", capturedBody, "file contents here")
	}
	if got := capturedHeaders.Get("Tus-Resumable"); got != "1.0.0" {
		t.Errorf("Tus-Resumable = %q, want %q", got, "1.0.0")
	}
	if got := capturedHeaders.Get("Content-Type"); got != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want %q", got, "application/octet-stream")
	}
}

// TestDo_SerializesBody verifies that Do correctly serializes the request body
// as JSON when a non-nil body is provided.
func TestDo_SerializesBody(t *testing.T) {
	var capturedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		capturedBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"id":"123"},"error":null,"meta":{"request_id":"r"}}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, Config{
		APIKey:      "test-key",
		AdminSecret: "test-secret",
	})

	reqBody := map[string]string{"name": "test-project"}
	var dest struct {
		ID string `json:"id"`
	}
	err := client.Do(context.Background(), http.MethodPost, "/projects", reqBody, &dest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]string
	if err := json.Unmarshal([]byte(capturedBody), &parsed); err != nil {
		t.Fatalf("request body is not valid JSON: %v", err)
	}
	if parsed["name"] != "test-project" {
		t.Errorf("request body name = %q, want %q", parsed["name"], "test-project")
	}
	if dest.ID != "123" {
		t.Errorf("dest.ID = %q, want %q", dest.ID, "123")
	}
}

// TestTransportError_Error verifies the string representation of TransportError.
func TestTransportError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  TransportError
		want string
	}{
		{
			name: "code and message",
			err:  TransportError{Code: "AUTH-0001", Message: "invalid token", HTTPStatus: 401},
			want: "AUTH-0001: invalid token",
		},
		{
			name: "code, message, and detail",
			err:  TransportError{Code: "PGRST204", Message: "Column not found", Detail: "Column 'foo' does not exist", HTTPStatus: 400},
			want: "PGRST204: Column not found (Column 'foo' does not exist)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestDoList_ErrorUnwrap verifies that DoList returns a TransportError when the
// server returns an error envelope.
func TestDoList_ErrorUnwrap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"data":null,"error":{"code":"AUTHZ-0001","message":"insufficient permissions"},"meta":{"request_id":"req-789"}}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, Config{
		APIKey:      "test-key",
		AdminSecret: "test-secret",
	})

	var result ListResult
	err := client.DoList(context.Background(), http.MethodGet, "/items", nil, &result)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var tErr *TransportError
	if !errors.As(err, &tErr) {
		t.Fatalf("expected *TransportError, got %T: %v", err, err)
	}
	if tErr.Code != "AUTHZ-0001" {
		t.Errorf("Code = %q, want %q", tErr.Code, "AUTHZ-0001")
	}
	if tErr.HTTPStatus != http.StatusForbidden {
		t.Errorf("HTTPStatus = %d, want %d", tErr.HTTPStatus, http.StatusForbidden)
	}
}

// TestDoUpload_ErrorResponse verifies that DoUpload returns a TransportError
// when the upload endpoint returns an error status.
func TestDoUpload_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		_, _ = w.Write([]byte(`{"data":null,"error":{"code":"STOR-0003","message":"file too large"},"meta":{"request_id":"req-up"}}`))
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.URL, Config{
		APIKey:      "test-key",
		AdminSecret: "test-secret",
	})

	body := strings.NewReader("large file data")
	err := client.DoUpload(context.Background(), http.MethodPost, "/upload", body)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var tErr *TransportError
	if !errors.As(err, &tErr) {
		t.Fatalf("expected *TransportError, got %T: %v", err, err)
	}
	if tErr.Code != "STOR-0003" {
		t.Errorf("Code = %q, want %q", tErr.Code, "STOR-0003")
	}
}

// TestNewHTTPClient_DefaultHTTPClient verifies that NewHTTPClient uses a default
// http.Client when Config.HTTPClient is nil.
func TestNewHTTPClient_DefaultHTTPClient(t *testing.T) {
	client := NewHTTPClient("http://localhost", Config{
		APIKey:      "test",
		AdminSecret: "test",
	})
	if client == nil {
		t.Fatal("NewHTTPClient returned nil")
	}
}

// TestNewHTTPClient_CustomHTTPClient verifies that a custom http.Client from
// Config is used instead of the default.
func TestNewHTTPClient_CustomHTTPClient(t *testing.T) {
	custom := &http.Client{}
	client := NewHTTPClient("http://localhost", Config{
		APIKey:      "test",
		AdminSecret: "test",
		HTTPClient:  custom,
	})
	if client == nil {
		t.Fatal("NewHTTPClient returned nil")
	}
}
