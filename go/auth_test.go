package mimdb

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestAuth_SignUp verifies that SignUp sends a POST to /v1/auth/{ref}/signup
// with email and password in the body, and correctly destructures the
// authResponse into separate *User and *Tokens return values.
func TestAuth_SignUp(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/auth/testref/signup" {
			t.Errorf("path = %q, want /v1/auth/testref/signup", r.URL.Path)
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
				"access_token":  "at-signup",
				"refresh_token": "rt-signup",
				"expires_in":    3600,
				"user": map[string]any{
					"id":              "user-1",
					"email":           "test@example.com",
					"email_confirmed": false,
					"token_version":   0,
					"created_at":      "2024-01-01T00:00:00Z",
					"updated_at":      "2024-01-01T00:00:00Z",
				},
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-signup"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	user, tokens, err := client.Auth().SignUp(context.Background(), "test@example.com", "password123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify request body.
	var body map[string]string
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	if body["email"] != "test@example.com" {
		t.Errorf("body email = %q, want %q", body["email"], "test@example.com")
	}
	if body["password"] != "password123" {
		t.Errorf("body password = %q, want %q", body["password"], "password123")
	}

	// Verify user extraction.
	if user == nil {
		t.Fatal("user is nil")
	}
	if user.ID != "user-1" {
		t.Errorf("user.ID = %q, want %q", user.ID, "user-1")
	}
	if user.Email == nil || *user.Email != "test@example.com" {
		t.Errorf("user.Email = %v, want %q", user.Email, "test@example.com")
	}

	// Verify tokens extraction.
	if tokens == nil {
		t.Fatal("tokens is nil")
	}
	if tokens.AccessToken != "at-signup" {
		t.Errorf("tokens.AccessToken = %q, want %q", tokens.AccessToken, "at-signup")
	}
	if tokens.RefreshToken != "rt-signup" {
		t.Errorf("tokens.RefreshToken = %q, want %q", tokens.RefreshToken, "rt-signup")
	}
	if tokens.ExpiresIn != 3600 {
		t.Errorf("tokens.ExpiresIn = %d, want %d", tokens.ExpiresIn, 3600)
	}
}

// TestAuth_SignIn verifies that SignIn sends a POST to
// /v1/auth/{ref}/token?grant_type=password with email and password in the body.
func TestAuth_SignIn(t *testing.T) {
	var capturedBody []byte
	var capturedPath string
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		capturedPath = r.URL.Path
		capturedQuery = r.URL.RawQuery

		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"access_token":  "at-signin",
				"refresh_token": "rt-signin",
				"expires_in":    7200,
				"user": map[string]any{
					"id":              "user-2",
					"email":           "login@example.com",
					"email_confirmed": true,
					"token_version":   1,
					"created_at":      "2024-01-01T00:00:00Z",
					"updated_at":      "2024-06-15T12:00:00Z",
				},
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-signin"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "myref",
		APIKey:     "test-key",
	})

	user, tokens, err := client.Auth().SignIn(context.Background(), "login@example.com", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify path and query param.
	if capturedPath != "/v1/auth/myref/token" {
		t.Errorf("path = %q, want /v1/auth/myref/token", capturedPath)
	}
	if capturedQuery != "grant_type=password" {
		t.Errorf("query = %q, want grant_type=password", capturedQuery)
	}

	// Verify request body has email + password.
	var body map[string]string
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	if body["email"] != "login@example.com" {
		t.Errorf("body email = %q, want %q", body["email"], "login@example.com")
	}
	if body["password"] != "secret" {
		t.Errorf("body password = %q, want %q", body["password"], "secret")
	}

	// Verify user extraction.
	if user == nil {
		t.Fatal("user is nil")
	}
	if user.ID != "user-2" {
		t.Errorf("user.ID = %q, want %q", user.ID, "user-2")
	}
	if user.Email == nil || *user.Email != "login@example.com" {
		t.Errorf("user.Email = %v, want %q", user.Email, "login@example.com")
	}
	if !user.EmailConfirmed {
		t.Error("user.EmailConfirmed = false, want true")
	}

	// Verify tokens extraction.
	if tokens == nil {
		t.Fatal("tokens is nil")
	}
	if tokens.AccessToken != "at-signin" {
		t.Errorf("tokens.AccessToken = %q, want %q", tokens.AccessToken, "at-signin")
	}
	if tokens.RefreshToken != "rt-signin" {
		t.Errorf("tokens.RefreshToken = %q, want %q", tokens.RefreshToken, "rt-signin")
	}
	if tokens.ExpiresIn != 7200 {
		t.Errorf("tokens.ExpiresIn = %d, want %d", tokens.ExpiresIn, 7200)
	}
}

// TestAuth_Refresh verifies that Refresh sends a POST to
// /v1/auth/{ref}/token?grant_type=refresh_token with the refresh token in the
// body, and returns only *Tokens (no user).
func TestAuth_Refresh(t *testing.T) {
	var capturedBody []byte
	var capturedPath string
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		capturedPath = r.URL.Path
		capturedQuery = r.URL.RawQuery

		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"access_token":  "at-refreshed",
				"refresh_token": "rt-refreshed",
				"expires_in":    3600,
				"user": map[string]any{
					"id":            "user-3",
					"email":         "refresh@example.com",
					"token_version": 2,
					"created_at":    "2024-01-01T00:00:00Z",
					"updated_at":    "2024-01-01T00:00:00Z",
				},
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-refresh"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "refref",
		APIKey:     "test-key",
	})

	tokens, err := client.Auth().Refresh(context.Background(), "old-refresh-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify path and query param.
	if capturedPath != "/v1/auth/refref/token" {
		t.Errorf("path = %q, want /v1/auth/refref/token", capturedPath)
	}
	if capturedQuery != "grant_type=refresh_token" {
		t.Errorf("query = %q, want grant_type=refresh_token", capturedQuery)
	}

	// Verify request body has refresh_token.
	var body map[string]string
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	if body["refresh_token"] != "old-refresh-token" {
		t.Errorf("body refresh_token = %q, want %q", body["refresh_token"], "old-refresh-token")
	}

	// Verify tokens extraction.
	if tokens == nil {
		t.Fatal("tokens is nil")
	}
	if tokens.AccessToken != "at-refreshed" {
		t.Errorf("tokens.AccessToken = %q, want %q", tokens.AccessToken, "at-refreshed")
	}
	if tokens.RefreshToken != "rt-refreshed" {
		t.Errorf("tokens.RefreshToken = %q, want %q", tokens.RefreshToken, "rt-refreshed")
	}
	if tokens.ExpiresIn != 3600 {
		t.Errorf("tokens.ExpiresIn = %d, want %d", tokens.ExpiresIn, 3600)
	}
}

// TestAuth_Logout verifies that Logout sends a POST to /v1/auth/{ref}/logout
// with the refresh token in the body, and handles the 204 No Content response.
func TestAuth_Logout(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/auth/logref/logout" {
			t.Errorf("path = %q, want /v1/auth/logref/logout", r.URL.Path)
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
		ProjectRef: "logref",
		APIKey:     "test-key",
	})

	err := client.Auth().Logout(context.Background(), "rt-to-revoke")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify request body has refresh_token.
	var body map[string]string
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	if body["refresh_token"] != "rt-to-revoke" {
		t.Errorf("body refresh_token = %q, want %q", body["refresh_token"], "rt-to-revoke")
	}
}

// TestAuth_RequiresProjectRef verifies that all auth methods return an error
// when the client is configured without a ProjectRef.
func TestAuth_RequiresProjectRef(t *testing.T) {
	client := NewClient("https://api.mimdb.dev", Options{
		AdminSecret: "admin-secret",
	})
	ctx := context.Background()

	t.Run("SignUp", func(t *testing.T) {
		_, _, err := client.Auth().SignUp(ctx, "a@b.com", "pass")
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("SignIn", func(t *testing.T) {
		_, _, err := client.Auth().SignIn(ctx, "a@b.com", "pass")
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("Refresh", func(t *testing.T) {
		_, err := client.Auth().Refresh(ctx, "rt")
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("Logout", func(t *testing.T) {
		err := client.Auth().Logout(ctx, "rt")
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("GetUser", func(t *testing.T) {
		_, err := client.Auth().GetUser(ctx)
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("UpdateUser", func(t *testing.T) {
		_, err := client.Auth().UpdateUser(ctx, UpdateUserRequest{})
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("ListSessions", func(t *testing.T) {
		_, err := client.Auth().ListSessions(ctx)
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("VerifyEmail", func(t *testing.T) {
		err := client.Auth().VerifyEmail(ctx, "token")
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("AdminListUsers", func(t *testing.T) {
		_, err := client.Auth().AdminListUsers(ctx)
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("AdminUpdateUser", func(t *testing.T) {
		_, err := client.Auth().AdminUpdateUser(ctx, "uid", AdminUpdateUserRequest{})
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("ForgotPassword", func(t *testing.T) {
		err := client.Auth().ForgotPassword(ctx, "a@b.com")
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})

	t.Run("ResetPassword", func(t *testing.T) {
		err := client.Auth().ResetPassword(ctx, "tok", "pass")
		if err == nil {
			t.Fatal("expected error for missing ProjectRef")
		}
	})
}

// TestAuth_ErrorResponse verifies that a 401 error response from the auth
// endpoint is correctly wrapped into *APIError with the appropriate fields.
func TestAuth_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": nil,
			"error": map[string]string{
				"code":    "AUTH-0002",
				"message": "invalid credentials",
				"detail":  "email or password is incorrect",
			},
			"meta": map[string]string{"request_id": "req-err"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "errref",
		APIKey:     "test-key",
	})

	_, _, err := client.Auth().SignIn(context.Background(), "bad@example.com", "wrong")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != "AUTH-0002" {
		t.Errorf("Code = %q, want %q", apiErr.Code, "AUTH-0002")
	}
	if apiErr.Message != "invalid credentials" {
		t.Errorf("Message = %q, want %q", apiErr.Message, "invalid credentials")
	}
	if apiErr.Detail != "email or password is incorrect" {
		t.Errorf("Detail = %q, want %q", apiErr.Detail, "email or password is incorrect")
	}
	if apiErr.HTTPStatus != http.StatusUnauthorized {
		t.Errorf("HTTPStatus = %d, want %d", apiErr.HTTPStatus, http.StatusUnauthorized)
	}
	if apiErr.RequestID != "req-err" {
		t.Errorf("RequestID = %q, want %q", apiErr.RequestID, "req-err")
	}
}

// TestAuth_GetUser verifies that GetUser sends a GET to /v1/auth/{ref}/user
// with the user's access token in the Authorization header.
func TestAuth_GetUser(t *testing.T) {
	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/v1/auth/testref/user" {
			t.Errorf("path = %q, want /v1/auth/testref/user", r.URL.Path)
		}
		capturedAuth = r.Header.Get("Authorization")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":              "user-100",
				"email":           "me@example.com",
				"email_confirmed": true,
				"token_version":   3,
				"user_metadata":   map[string]any{"nickname": "tester"},
				"created_at":      "2024-01-01T00:00:00Z",
				"updated_at":      "2024-06-01T00:00:00Z",
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-getuser"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})
	client.SetAccessToken("user-jwt-token")

	user, err := client.Auth().GetUser(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify access token was sent.
	if capturedAuth != "Bearer user-jwt-token" {
		t.Errorf("Authorization = %q, want %q", capturedAuth, "Bearer user-jwt-token")
	}

	if user == nil {
		t.Fatal("user is nil")
	}
	if user.ID != "user-100" {
		t.Errorf("user.ID = %q, want %q", user.ID, "user-100")
	}
	if user.Email == nil || *user.Email != "me@example.com" {
		t.Errorf("user.Email = %v, want %q", user.Email, "me@example.com")
	}
}

// TestAuth_UpdateUser verifies that UpdateUser sends a PUT to
// /v1/auth/{ref}/user with user_metadata in the body.
func TestAuth_UpdateUser(t *testing.T) {
	var capturedBody []byte
	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %q, want PUT", r.Method)
		}
		if r.URL.Path != "/v1/auth/testref/user" {
			t.Errorf("path = %q, want /v1/auth/testref/user", r.URL.Path)
		}
		capturedAuth = r.Header.Get("Authorization")

		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":            "user-100",
				"email":         "me@example.com",
				"user_metadata": map[string]any{"nickname": "updated"},
				"token_version": 3,
				"created_at":    "2024-01-01T00:00:00Z",
				"updated_at":    "2024-06-15T00:00:00Z",
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-updateuser"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})
	client.SetAccessToken("user-jwt-token")

	user, err := client.Auth().UpdateUser(context.Background(), UpdateUserRequest{
		UserMetadata: map[string]any{"nickname": "updated"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify access token was sent.
	if capturedAuth != "Bearer user-jwt-token" {
		t.Errorf("Authorization = %q, want %q", capturedAuth, "Bearer user-jwt-token")
	}

	// Verify request body contains user_metadata.
	var body map[string]any
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("failed to parse body: %v", err)
	}
	meta, ok := body["user_metadata"].(map[string]any)
	if !ok {
		t.Fatalf("body missing user_metadata, got %v", body)
	}
	if meta["nickname"] != "updated" {
		t.Errorf("user_metadata.nickname = %v, want %q", meta["nickname"], "updated")
	}

	if user == nil {
		t.Fatal("user is nil")
	}
	if user.ID != "user-100" {
		t.Errorf("user.ID = %q, want %q", user.ID, "user-100")
	}
}

// TestAuth_ListSessions verifies that ListSessions sends a GET to
// /v1/auth/{ref}/sessions and returns a []Session.
func TestAuth_ListSessions(t *testing.T) {
	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/v1/auth/testref/sessions" {
			t.Errorf("path = %q, want /v1/auth/testref/sessions", r.URL.Path)
		}
		capturedAuth = r.Header.Get("Authorization")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id":           "sess-1",
					"user_id":      "user-100",
					"ip_address":   "1.2.3.4",
					"user_agent":   "Go-SDK/1.0",
					"created_at":   "2024-06-01T00:00:00Z",
					"last_seen_at": "2024-06-15T12:00:00Z",
				},
				{
					"id":           "sess-2",
					"user_id":      "user-100",
					"created_at":   "2024-06-10T00:00:00Z",
					"last_seen_at": "2024-06-15T12:00:00Z",
				},
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-sessions"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})
	client.SetAccessToken("user-jwt-token")

	sessions, err := client.Auth().ListSessions(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify access token was sent.
	if capturedAuth != "Bearer user-jwt-token" {
		t.Errorf("Authorization = %q, want %q", capturedAuth, "Bearer user-jwt-token")
	}

	if len(sessions) != 2 {
		t.Fatalf("len(sessions) = %d, want 2", len(sessions))
	}
	if sessions[0].ID != "sess-1" {
		t.Errorf("sessions[0].ID = %q, want %q", sessions[0].ID, "sess-1")
	}
	if sessions[0].IPAddress == nil || *sessions[0].IPAddress != "1.2.3.4" {
		t.Errorf("sessions[0].IPAddress = %v, want %q", sessions[0].IPAddress, "1.2.3.4")
	}
	if sessions[1].ID != "sess-2" {
		t.Errorf("sessions[1].ID = %q, want %q", sessions[1].ID, "sess-2")
	}
}

// TestAuth_VerifyEmail verifies that VerifyEmail sends a POST to
// /v1/auth/{ref}/verify with the verification token in the body.
func TestAuth_VerifyEmail(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/auth/testref/verify" {
			t.Errorf("path = %q, want /v1/auth/testref/verify", r.URL.Path)
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

	err := client.Auth().VerifyEmail(context.Background(), "verify-token-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var body map[string]string
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("failed to parse body: %v", err)
	}
	if body["token"] != "verify-token-abc" {
		t.Errorf("body token = %q, want %q", body["token"], "verify-token-abc")
	}
}

// TestAuth_OAuthURL verifies that OAuthURL constructs the correct URL without
// making any HTTP calls.
func TestAuth_OAuthURL(t *testing.T) {
	client := NewClient("https://api.mimdb.dev", Options{
		ProjectRef: "myref",
		APIKey:     "test-key",
	})

	url := client.Auth().OAuthURL("github", "https://myapp.com/callback")

	expected := "https://api.mimdb.dev/v1/auth/myref/oauth/github?redirect_to=https%3A%2F%2Fmyapp.com%2Fcallback"
	if url != expected {
		t.Errorf("OAuthURL =\n  %q\nwant\n  %q", url, expected)
	}
}

// TestAuth_AdminListUsers verifies that AdminListUsers sends a GET to
// /v1/auth/{ref}/users (NOT /admin/users) with optional query params.
func TestAuth_AdminListUsers(t *testing.T) {
	t.Run("no options", func(t *testing.T) {
		var capturedPath string
		var capturedQuery string
		var capturedAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want GET", r.Method)
			}
			capturedPath = r.URL.Path
			capturedQuery = r.URL.RawQuery
			capturedAuth = r.Header.Get("Authorization")

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "u-1", "email": "a@b.com", "token_version": 0, "created_at": "2024-01-01T00:00:00Z", "updated_at": "2024-01-01T00:00:00Z"},
				},
				"error": nil,
				"meta":  map[string]string{"request_id": "req-adminlist"},
			})
		}))
		defer srv.Close()

		client := NewClient(srv.URL, Options{
			ProjectRef: "testref",
			APIKey:     "service-role-key",
		})

		users, err := client.Auth().AdminListUsers(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if capturedPath != "/v1/auth/testref/users" {
			t.Errorf("path = %q, want /v1/auth/testref/users", capturedPath)
		}
		if capturedQuery != "" {
			t.Errorf("query = %q, want empty", capturedQuery)
		}
		// Verify the API key is sent as a Bearer token for admin endpoints.
		if capturedAuth != "Bearer service-role-key" {
			t.Errorf("Authorization = %q, want %q", capturedAuth, "Bearer service-role-key")
		}
		if len(users) != 1 {
			t.Fatalf("len(users) = %d, want 1", len(users))
		}
		if users[0].ID != "u-1" {
			t.Errorf("users[0].ID = %q, want %q", users[0].ID, "u-1")
		}
	})

	t.Run("with options", func(t *testing.T) {
		var capturedPath string
		var capturedQuery string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedPath = r.URL.Path
			capturedQuery = r.URL.RawQuery

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data":  []map[string]any{},
				"error": nil,
				"meta":  map[string]string{"request_id": "req-adminlist2"},
			})
		}))
		defer srv.Close()

		client := NewClient(srv.URL, Options{
			ProjectRef: "testref",
			APIKey:     "service-role-key",
		})

		_, err := client.Auth().AdminListUsers(context.Background(), AdminListUsersOptions{
			Email:  "search@example.com",
			Limit:  25,
			Offset: 50,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if capturedPath != "/v1/auth/testref/users" {
			t.Errorf("path = %q, want /v1/auth/testref/users", capturedPath)
		}

		// Parse query params to verify individually (order-independent).
		q := parseQuery(capturedQuery)
		if q["email"] != "search@example.com" {
			t.Errorf("query email = %q, want %q", q["email"], "search@example.com")
		}
		if q["limit"] != "25" {
			t.Errorf("query limit = %q, want %q", q["limit"], "25")
		}
		if q["offset"] != "50" {
			t.Errorf("query offset = %q, want %q", q["offset"], "50")
		}
	})
}

// TestAuth_AdminUpdateUser verifies that AdminUpdateUser sends a PATCH to
// /v1/auth/{ref}/users/{userId} with app_metadata in the body, and sends the
// API key as a Bearer token in the Authorization header.
func TestAuth_AdminUpdateUser(t *testing.T) {
	var capturedBody []byte
	var capturedPath string
	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %q, want PATCH", r.Method)
		}
		capturedPath = r.URL.Path
		capturedAuth = r.Header.Get("Authorization")

		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":           "user-42",
				"email":        "admin-target@example.com",
				"app_metadata": map[string]any{"role": "moderator"},
				"token_version": 1,
				"created_at":    "2024-01-01T00:00:00Z",
				"updated_at":    "2024-06-15T00:00:00Z",
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-adminupdate"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "service-role-key",
	})

	user, err := client.Auth().AdminUpdateUser(context.Background(), "user-42", AdminUpdateUserRequest{
		AppMetadata: map[string]any{"role": "moderator"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedPath != "/v1/auth/testref/users/user-42" {
		t.Errorf("path = %q, want /v1/auth/testref/users/user-42", capturedPath)
	}
	// Verify the API key is sent as a Bearer token for admin endpoints.
	if capturedAuth != "Bearer service-role-key" {
		t.Errorf("Authorization = %q, want %q", capturedAuth, "Bearer service-role-key")
	}

	// Verify request body contains app_metadata.
	var body map[string]any
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("failed to parse body: %v", err)
	}
	meta, ok := body["app_metadata"].(map[string]any)
	if !ok {
		t.Fatalf("body missing app_metadata, got %v", body)
	}
	if meta["role"] != "moderator" {
		t.Errorf("app_metadata.role = %v, want %q", meta["role"], "moderator")
	}

	if user == nil {
		t.Fatal("user is nil")
	}
	if user.ID != "user-42" {
		t.Errorf("user.ID = %q, want %q", user.ID, "user-42")
	}
}

// TestAuth_ForgotPassword verifies that ForgotPassword sends a POST to
// /v1/auth/{ref}/forgot-password with the email in the body.
func TestAuth_ForgotPassword(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/auth/testref/forgot-password" {
			t.Errorf("path = %q, want /v1/auth/testref/forgot-password", r.URL.Path)
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

	err := client.Auth().ForgotPassword(context.Background(), "forgot@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var body map[string]string
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("failed to parse body: %v", err)
	}
	if body["email"] != "forgot@example.com" {
		t.Errorf("body email = %q, want %q", body["email"], "forgot@example.com")
	}
}

// TestAuth_ResetPassword verifies that ResetPassword sends a POST to
// /v1/auth/{ref}/reset-password with token and new_password in the body.
func TestAuth_ResetPassword(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/auth/testref/reset-password" {
			t.Errorf("path = %q, want /v1/auth/testref/reset-password", r.URL.Path)
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

	err := client.Auth().ResetPassword(context.Background(), "reset-token-xyz", "newpassword123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var body map[string]string
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("failed to parse body: %v", err)
	}
	if body["token"] != "reset-token-xyz" {
		t.Errorf("body token = %q, want %q", body["token"], "reset-token-xyz")
	}
	if body["new_password"] != "newpassword123" {
		t.Errorf("body new_password = %q, want %q", body["new_password"], "newpassword123")
	}
}

// TestAuth_LazyInit verifies that Auth() returns the same AuthClient instance
// on repeated calls (lazy singleton via sync.Once).
func TestAuth_LazyInit(t *testing.T) {
	client := NewClient("https://api.mimdb.dev", Options{
		ProjectRef: "ref",
		APIKey:     "k",
	})
	a1 := client.Auth()
	a2 := client.Auth()
	if a1 != a2 {
		t.Error("Auth() should return the same instance on repeated calls")
	}
}

// parseQuery is a test helper that splits a raw query string into a key=value
// map. Values are URL-decoded. It handles only simple single-value params
// (sufficient for these tests).
func parseQuery(raw string) map[string]string {
	result := make(map[string]string)
	if raw == "" {
		return result
	}
	for _, pair := range strings.Split(raw, "&") {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			val, err := url.QueryUnescape(parts[1])
			if err != nil {
				val = parts[1]
			}
			result[parts[0]] = val
		}
	}
	return result
}
