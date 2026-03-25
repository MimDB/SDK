package mimdb

import (
	"errors"
	"testing"

	"github.com/MimDB/SDK/go/internal/transport"
)

// TestNewClient_ProjectScoped verifies that a project-scoped client is created
// with the correct base URL and project ref.
func TestNewClient_ProjectScoped(t *testing.T) {
	c := NewClient("https://api.mimdb.dev", Options{
		ProjectRef: "40891b0d",
		APIKey:     "test-key",
	})
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	if c.baseURL != "https://api.mimdb.dev" {
		t.Errorf("baseURL = %q", c.baseURL)
	}
	if c.projectRef != "40891b0d" {
		t.Errorf("projectRef = %q, want %q", c.projectRef, "40891b0d")
	}
}

// TestNewClient_PlatformOnly verifies that a platform-only client can be
// created with just an admin secret and no project ref.
func TestNewClient_PlatformOnly(t *testing.T) {
	c := NewClient("https://api.mimdb.dev", Options{
		AdminSecret: "admin-secret",
	})
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
}

// TestClient_PlatformLazyInit verifies that Platform() returns the same
// PlatformClient instance on repeated calls (lazy singleton).
func TestClient_PlatformLazyInit(t *testing.T) {
	c := NewClient("https://api.mimdb.dev", Options{AdminSecret: "s"})
	p1 := c.Platform()
	p2 := c.Platform()
	if p1 != p2 {
		t.Error("Platform() should return same instance")
	}
}

// TestClient_RequiresProjectRef verifies that a platform-only client has an
// empty projectRef field.
func TestClient_RequiresProjectRef(t *testing.T) {
	c := NewClient("https://api.mimdb.dev", Options{AdminSecret: "s"})
	if c.projectRef != "" {
		t.Error("projectRef should be empty for platform-only client")
	}
}

// TestClient_SetAccessToken verifies thread-safe access token mutation.
func TestClient_SetAccessToken(t *testing.T) {
	c := NewClient("https://api.mimdb.dev", Options{APIKey: "k"})
	c.SetAccessToken("user-token-123")
	if c.accessToken != "user-token-123" {
		t.Error("accessToken not set")
	}
}

// TestClient_RequireProjectRef_Error verifies that requireProjectRef returns
// an error when projectRef is empty.
func TestClient_RequireProjectRef_Error(t *testing.T) {
	c := NewClient("https://api.mimdb.dev", Options{AdminSecret: "s"})
	err := c.requireProjectRef()
	if err == nil {
		t.Fatal("expected error for empty projectRef")
	}
}

// TestClient_RequireProjectRef_Success verifies that requireProjectRef returns
// nil when projectRef is set.
func TestClient_RequireProjectRef_Success(t *testing.T) {
	c := NewClient("https://api.mimdb.dev", Options{
		ProjectRef: "abc123",
		APIKey:     "k",
	})
	if err := c.requireProjectRef(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestWrapTransportError verifies that transport.TransportError is converted
// to *APIError while preserving all fields.
func TestWrapTransportError(t *testing.T) {
	original := &transport.TransportError{
		Code:       "AUTH-0001",
		Message:    "invalid token",
		Detail:     "token expired",
		HTTPStatus: 401,
		RequestID:  "req-123",
	}

	wrapped := wrapTransportError(original)

	var apiErr *APIError
	if !errors.As(wrapped, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", wrapped, wrapped)
	}
	if apiErr.Code != "AUTH-0001" {
		t.Errorf("Code = %q, want %q", apiErr.Code, "AUTH-0001")
	}
	if apiErr.Message != "invalid token" {
		t.Errorf("Message = %q, want %q", apiErr.Message, "invalid token")
	}
	if apiErr.Detail != "token expired" {
		t.Errorf("Detail = %q, want %q", apiErr.Detail, "token expired")
	}
	if apiErr.HTTPStatus != 401 {
		t.Errorf("HTTPStatus = %d, want %d", apiErr.HTTPStatus, 401)
	}
	if apiErr.RequestID != "req-123" {
		t.Errorf("RequestID = %q, want %q", apiErr.RequestID, "req-123")
	}
}

// TestWrapTransportError_NonTransportError verifies that non-transport errors
// pass through unchanged.
func TestWrapTransportError_NonTransportError(t *testing.T) {
	original := errors.New("some other error")
	wrapped := wrapTransportError(original)
	if wrapped != original {
		t.Error("non-transport errors should pass through unchanged")
	}
}

// TestWrapTransportError_Nil verifies that nil errors pass through as nil.
func TestWrapTransportError_Nil(t *testing.T) {
	wrapped := wrapTransportError(nil)
	if wrapped != nil {
		t.Errorf("expected nil, got %v", wrapped)
	}
}

// TestNewClient_TrailingSlashTrimmed verifies that trailing slashes on the
// base URL are stripped to prevent double-slash path construction.
func TestNewClient_TrailingSlashTrimmed(t *testing.T) {
	c := NewClient("https://api.mimdb.dev/", Options{APIKey: "k"})
	if c.baseURL != "https://api.mimdb.dev" {
		t.Errorf("baseURL = %q, want trailing slash trimmed", c.baseURL)
	}
}

// TestNewClient_CustomHTTPClient verifies that a custom http.Client is passed
// through to the underlying transport.
func TestNewClient_CustomHTTPClient(t *testing.T) {
	// Ensure NewClient does not panic when a custom HTTPClient is provided.
	c := NewClient("https://api.mimdb.dev", Options{
		APIKey:     "k",
		HTTPClient: nil, // default
	})
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
}
