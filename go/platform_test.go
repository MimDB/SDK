package mimdb

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestPlatform_ListOrganizations verifies that ListOrganizations sends a GET to
// the correct path and deserializes the envelope response into []Organization.
func TestPlatform_ListOrganizations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/v1/platform/organizations" {
			t.Errorf("path = %q, want /v1/platform/organizations", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id":         "org-1",
					"name":       "Test Org",
					"slug":       "test-org",
					"created_at": "2024-01-01T00:00:00Z",
					"updated_at": "2024-01-01T00:00:00Z",
				},
				{
					"id":         "org-2",
					"name":       "Another Org",
					"slug":       "another-org",
					"created_at": "2024-06-15T12:00:00Z",
					"updated_at": "2024-06-15T12:00:00Z",
				},
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-list"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{AdminSecret: "test-secret"})
	orgs, err := client.Platform().ListOrganizations(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(orgs) != 2 {
		t.Fatalf("len(orgs) = %d, want 2", len(orgs))
	}
	if orgs[0].ID != "org-1" {
		t.Errorf("orgs[0].ID = %q, want %q", orgs[0].ID, "org-1")
	}
	if orgs[0].Name != "Test Org" {
		t.Errorf("orgs[0].Name = %q, want %q", orgs[0].Name, "Test Org")
	}
	if orgs[0].Slug != "test-org" {
		t.Errorf("orgs[0].Slug = %q, want %q", orgs[0].Slug, "test-org")
	}
	if orgs[1].ID != "org-2" {
		t.Errorf("orgs[1].ID = %q, want %q", orgs[1].ID, "org-2")
	}
	if orgs[1].Name != "Another Org" {
		t.Errorf("orgs[1].Name = %q, want %q", orgs[1].Name, "Another Org")
	}
}

// TestPlatform_CreateOrganization verifies that CreateOrganization sends a POST
// with the correct JSON body and returns the created *Organization.
func TestPlatform_CreateOrganization(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/platform/organizations" {
			t.Errorf("path = %q, want /v1/platform/organizations", r.URL.Path)
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
				"id":         "org-new",
				"name":       "New Org",
				"slug":       "new-org",
				"created_at": "2024-03-01T00:00:00Z",
				"updated_at": "2024-03-01T00:00:00Z",
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-create"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{AdminSecret: "test-secret"})
	org, err := client.Platform().CreateOrganization(context.Background(), CreateOrgRequest{
		Name: "New Org",
		Slug: "new-org",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the request body was sent correctly.
	var reqBody map[string]string
	if err := json.Unmarshal(capturedBody, &reqBody); err != nil {
		t.Fatalf("request body is not valid JSON: %v", err)
	}
	if reqBody["name"] != "New Org" {
		t.Errorf("request body name = %q, want %q", reqBody["name"], "New Org")
	}
	if reqBody["slug"] != "new-org" {
		t.Errorf("request body slug = %q, want %q", reqBody["slug"], "new-org")
	}

	// Verify the returned organization.
	if org == nil {
		t.Fatal("expected non-nil organization")
	}
	if org.ID != "org-new" {
		t.Errorf("org.ID = %q, want %q", org.ID, "org-new")
	}
	if org.Name != "New Org" {
		t.Errorf("org.Name = %q, want %q", org.Name, "New Org")
	}
	if org.Slug != "new-org" {
		t.Errorf("org.Slug = %q, want %q", org.Slug, "new-org")
	}
}

// TestPlatform_GetOrganization verifies that GetOrganization sends a GET to the
// correct path with the orgID interpolated and returns the *Organization.
func TestPlatform_GetOrganization(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/v1/platform/organizations/org-42" {
			t.Errorf("path = %q, want /v1/platform/organizations/org-42", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":         "org-42",
				"name":       "Fetched Org",
				"slug":       "fetched-org",
				"created_at": "2024-01-01T00:00:00Z",
				"updated_at": "2024-01-01T00:00:00Z",
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-get"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{AdminSecret: "test-secret"})
	org, err := client.Platform().GetOrganization(context.Background(), "org-42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if org == nil {
		t.Fatal("expected non-nil organization")
	}
	if org.ID != "org-42" {
		t.Errorf("org.ID = %q, want %q", org.ID, "org-42")
	}
	if org.Name != "Fetched Org" {
		t.Errorf("org.Name = %q, want %q", org.Name, "Fetched Org")
	}
	if org.Slug != "fetched-org" {
		t.Errorf("org.Slug = %q, want %q", org.Slug, "fetched-org")
	}
}

// TestPlatform_ListProjects verifies that ListProjects sends a GET to the
// correct path with the orgID interpolated and returns []Project.
func TestPlatform_ListProjects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/v1/platform/organizations/org-99/projects" {
			t.Errorf("path = %q, want /v1/platform/organizations/org-99/projects", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id":          "proj-1",
					"org_id":      "org-99",
					"ref":         "abc123",
					"name":        "My Project",
					"schema_name": "my_project",
					"region":      "us-east-1",
					"status":      "active",
					"settings":    map[string]any{"feature_x": true},
					"created_at":  "2024-02-01T00:00:00Z",
					"updated_at":  "2024-02-01T00:00:00Z",
				},
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-projects"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{AdminSecret: "test-secret"})
	projects, err := client.Platform().ListProjects(context.Background(), "org-99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("len(projects) = %d, want 1", len(projects))
	}
	if projects[0].ID != "proj-1" {
		t.Errorf("projects[0].ID = %q, want %q", projects[0].ID, "proj-1")
	}
	if projects[0].OrgID != "org-99" {
		t.Errorf("projects[0].OrgID = %q, want %q", projects[0].OrgID, "org-99")
	}
	if projects[0].Ref != "abc123" {
		t.Errorf("projects[0].Ref = %q, want %q", projects[0].Ref, "abc123")
	}
	if projects[0].Name != "My Project" {
		t.Errorf("projects[0].Name = %q, want %q", projects[0].Name, "My Project")
	}
	if projects[0].Region != "us-east-1" {
		t.Errorf("projects[0].Region = %q, want %q", projects[0].Region, "us-east-1")
	}
	if projects[0].Status != "active" {
		t.Errorf("projects[0].Status = %q, want %q", projects[0].Status, "active")
	}
}

// TestPlatform_ErrorResponse verifies that platform methods convert API error
// envelopes to *APIError, allowing callers to inspect structured error details.
func TestPlatform_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": nil,
			"error": map[string]any{
				"code":    "AUTH-0001",
				"message": "invalid admin secret",
				"detail":  "the provided secret is expired",
			},
			"meta": map[string]string{"request_id": "req-err"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{AdminSecret: "bad-secret"})
	_, err := client.Platform().ListOrganizations(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != "AUTH-0001" {
		t.Errorf("Code = %q, want %q", apiErr.Code, "AUTH-0001")
	}
	if apiErr.Message != "invalid admin secret" {
		t.Errorf("Message = %q, want %q", apiErr.Message, "invalid admin secret")
	}
	if apiErr.Detail != "the provided secret is expired" {
		t.Errorf("Detail = %q, want %q", apiErr.Detail, "the provided secret is expired")
	}
	if apiErr.HTTPStatus != http.StatusUnauthorized {
		t.Errorf("HTTPStatus = %d, want %d", apiErr.HTTPStatus, http.StatusUnauthorized)
	}
	if apiErr.RequestID != "req-err" {
		t.Errorf("RequestID = %q, want %q", apiErr.RequestID, "req-err")
	}
}

// TestPlatform_CreateProject verifies that CreateProject sends a POST to
// /v1/platform/projects with the correct JSON body and returns *ProjectWithKeys
// containing the project, its API keys, and the database connection string.
func TestPlatform_CreateProject(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/platform/projects" {
			t.Errorf("path = %q, want /v1/platform/projects", r.URL.Path)
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
				"project": map[string]any{
					"id":          "proj-new",
					"org_id":      "org-1",
					"ref":         "xyzabc",
					"name":        "New Project",
					"schema_name": "new_project",
					"region":      "us-west-2",
					"status":      "active",
					"settings":    map[string]any{},
					"created_at":  "2024-04-01T00:00:00Z",
					"updated_at":  "2024-04-01T00:00:00Z",
				},
				"anon_key": map[string]any{
					"id":         "key-anon-1",
					"project_id": "proj-new",
					"name":       "anon",
					"key_prefix":  "anon_xyzabc",
					"role":       "anon",
					"created_at": "2024-04-01T00:00:00Z",
					"raw_key":    "eyJhbGciOiJIUzI1NiJ9.anon-jwt",
				},
				"service_role_key": map[string]any{
					"id":         "key-sr-1",
					"project_id": "proj-new",
					"name":       "service_role",
					"key_prefix":  "sr_xyzabc",
					"role":       "service_role",
					"created_at": "2024-04-01T00:00:00Z",
					"raw_key":    "eyJhbGciOiJIUzI1NiJ9.service-role-jwt",
				},
				"db_connection_string": "postgres://user:pass@db.mimdb.dev:5432/postgres",
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-create-proj"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{AdminSecret: "test-secret"})
	result, err := client.Platform().CreateProject(context.Background(), CreateProjectRequest{
		Name:  "New Project",
		OrgID: "org-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify request body.
	var reqBody map[string]string
	if err := json.Unmarshal(capturedBody, &reqBody); err != nil {
		t.Fatalf("request body is not valid JSON: %v", err)
	}
	if reqBody["name"] != "New Project" {
		t.Errorf("request body name = %q, want %q", reqBody["name"], "New Project")
	}
	if reqBody["org_id"] != "org-1" {
		t.Errorf("request body org_id = %q, want %q", reqBody["org_id"], "org-1")
	}

	// Verify response.
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Project.ID != "proj-new" {
		t.Errorf("Project.ID = %q, want %q", result.Project.ID, "proj-new")
	}
	if result.Project.Name != "New Project" {
		t.Errorf("Project.Name = %q, want %q", result.Project.Name, "New Project")
	}
	if result.Project.OrgID != "org-1" {
		t.Errorf("Project.OrgID = %q, want %q", result.Project.OrgID, "org-1")
	}
	if result.AnonKey == nil {
		t.Fatal("expected non-nil AnonKey")
	}
	if result.AnonKey.RawKey != "eyJhbGciOiJIUzI1NiJ9.anon-jwt" {
		t.Errorf("AnonKey.RawKey = %q, want %q", result.AnonKey.RawKey, "eyJhbGciOiJIUzI1NiJ9.anon-jwt")
	}
	if result.ServiceRoleKey == nil {
		t.Fatal("expected non-nil ServiceRoleKey")
	}
	if result.ServiceRoleKey.RawKey != "eyJhbGciOiJIUzI1NiJ9.service-role-jwt" {
		t.Errorf("ServiceRoleKey.RawKey = %q, want %q", result.ServiceRoleKey.RawKey, "eyJhbGciOiJIUzI1NiJ9.service-role-jwt")
	}
	if result.DBConnectionString != "postgres://user:pass@db.mimdb.dev:5432/postgres" {
		t.Errorf("DBConnectionString = %q, want %q", result.DBConnectionString, "postgres://user:pass@db.mimdb.dev:5432/postgres")
	}
}

// TestPlatform_GetProject verifies that GetProject sends a GET to
// /v1/platform/projects/{projectID} and returns the *Project.
func TestPlatform_GetProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/v1/platform/projects/proj-42" {
			t.Errorf("path = %q, want /v1/platform/projects/proj-42", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":          "proj-42",
				"org_id":      "org-7",
				"ref":         "ref42",
				"name":        "My Project",
				"schema_name": "my_project",
				"region":      "eu-west-1",
				"status":      "active",
				"settings":    map[string]any{"feature_x": true},
				"created_at":  "2024-05-01T00:00:00Z",
				"updated_at":  "2024-05-01T00:00:00Z",
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-get-proj"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{AdminSecret: "test-secret"})
	proj, err := client.Platform().GetProject(context.Background(), "proj-42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if proj == nil {
		t.Fatal("expected non-nil project")
	}
	if proj.ID != "proj-42" {
		t.Errorf("proj.ID = %q, want %q", proj.ID, "proj-42")
	}
	if proj.OrgID != "org-7" {
		t.Errorf("proj.OrgID = %q, want %q", proj.OrgID, "org-7")
	}
	if proj.Ref != "ref42" {
		t.Errorf("proj.Ref = %q, want %q", proj.Ref, "ref42")
	}
	if proj.Name != "My Project" {
		t.Errorf("proj.Name = %q, want %q", proj.Name, "My Project")
	}
	if proj.Region != "eu-west-1" {
		t.Errorf("proj.Region = %q, want %q", proj.Region, "eu-west-1")
	}
	if proj.Status != "active" {
		t.Errorf("proj.Status = %q, want %q", proj.Status, "active")
	}
}

// TestPlatform_RotateDBCredential verifies that RotateDBCredential sends a POST
// to /v1/platform/projects/{projectID}/rotate-db-credential and extracts the
// password string from the response.
func TestPlatform_RotateDBCredential(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/platform/projects/proj-42/rotate-db-credential" {
			t.Errorf("path = %q, want /v1/platform/projects/proj-42/rotate-db-credential", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"db_connection_string": "postgres://user:new-password-hex-abc123@db.mimdb.dev:5432/postgres",
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-rotate"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{AdminSecret: "test-secret"})
	connStr, err := client.Platform().RotateDBCredential(context.Background(), "proj-42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "postgres://user:new-password-hex-abc123@db.mimdb.dev:5432/postgres"
	if connStr != expected {
		t.Errorf("connStr = %q, want %q", connStr, expected)
	}
}

// TestPlatform_GetAPIKeys verifies that GetAPIKeys sends a GET to
// /v1/platform/projects/{projectID}/api-keys and returns *APIKeys with
// anon_key and service_role_key.
func TestPlatform_GetAPIKeys(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/v1/platform/projects/proj-42/api-keys" {
			t.Errorf("path = %q, want /v1/platform/projects/proj-42/api-keys", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"anon_key":         "eyJ.anon-key-value",
				"service_role_key": "eyJ.service-role-key-value",
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-get-keys"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{AdminSecret: "test-secret"})
	keys, err := client.Platform().GetAPIKeys(context.Background(), "proj-42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if keys == nil {
		t.Fatal("expected non-nil keys")
	}
	if keys.AnonKey != "eyJ.anon-key-value" {
		t.Errorf("AnonKey = %q, want %q", keys.AnonKey, "eyJ.anon-key-value")
	}
	if keys.ServiceRoleKey != "eyJ.service-role-key-value" {
		t.Errorf("ServiceRoleKey = %q, want %q", keys.ServiceRoleKey, "eyJ.service-role-key-value")
	}
}

// TestPlatform_RegenerateAPIKeys verifies that RegenerateAPIKeys sends a POST to
// /v1/platform/projects/{projectID}/api-keys/regenerate and returns *APIKeys.
func TestPlatform_RegenerateAPIKeys(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/platform/projects/proj-42/api-keys/regenerate" {
			t.Errorf("path = %q, want /v1/platform/projects/proj-42/api-keys/regenerate", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"anon_key":         "eyJ.new-anon-key",
				"service_role_key": "eyJ.new-service-role-key",
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-regen-keys"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{AdminSecret: "test-secret"})
	keys, err := client.Platform().RegenerateAPIKeys(context.Background(), "proj-42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if keys == nil {
		t.Fatal("expected non-nil keys")
	}
	if keys.AnonKey != "eyJ.new-anon-key" {
		t.Errorf("AnonKey = %q, want %q", keys.AnonKey, "eyJ.new-anon-key")
	}
	if keys.ServiceRoleKey != "eyJ.new-service-role-key" {
		t.Errorf("ServiceRoleKey = %q, want %q", keys.ServiceRoleKey, "eyJ.new-service-role-key")
	}
}

// TestPlatform_GetConnectionInfo verifies that GetConnectionInfo sends a GET to
// /v1/platform/projects/{projectID}/connection-info and returns *ConnectionInfo
// with nested DBPooled and DBDirect connections.
func TestPlatform_GetConnectionInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/v1/platform/projects/proj-42/connection-info" {
			t.Errorf("path = %q, want /v1/platform/projects/proj-42/connection-info", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"project_ref": "ref42",
				"schema_name": "my_project",
				"db_pooled": map[string]any{
					"host":     "pooled.db.mimdb.dev",
					"port":     6543,
					"database": "postgres",
					"user":     "postgres.ref42",
				},
				"db_direct": map[string]any{
					"host":     "direct.db.mimdb.dev",
					"port":     5432,
					"database": "postgres",
					"user":     "postgres",
				},
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-conn-info"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{AdminSecret: "test-secret"})
	info, err := client.Platform().GetConnectionInfo(context.Background(), "proj-42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info == nil {
		t.Fatal("expected non-nil connection info")
	}
	if info.ProjectRef != "ref42" {
		t.Errorf("ProjectRef = %q, want %q", info.ProjectRef, "ref42")
	}
	if info.SchemaName != "my_project" {
		t.Errorf("SchemaName = %q, want %q", info.SchemaName, "my_project")
	}

	// Verify pooled connection.
	if info.DBPooled == nil {
		t.Fatal("expected non-nil DBPooled")
	}
	if info.DBPooled.Host != "pooled.db.mimdb.dev" {
		t.Errorf("DBPooled.Host = %q, want %q", info.DBPooled.Host, "pooled.db.mimdb.dev")
	}
	if info.DBPooled.Port != 6543 {
		t.Errorf("DBPooled.Port = %d, want %d", info.DBPooled.Port, 6543)
	}
	if info.DBPooled.Database != "postgres" {
		t.Errorf("DBPooled.Database = %q, want %q", info.DBPooled.Database, "postgres")
	}
	if info.DBPooled.User != "postgres.ref42" {
		t.Errorf("DBPooled.User = %q, want %q", info.DBPooled.User, "postgres.ref42")
	}

	// Verify direct connection.
	if info.DBDirect == nil {
		t.Fatal("expected non-nil DBDirect")
	}
	if info.DBDirect.Host != "direct.db.mimdb.dev" {
		t.Errorf("DBDirect.Host = %q, want %q", info.DBDirect.Host, "direct.db.mimdb.dev")
	}
	if info.DBDirect.Port != 5432 {
		t.Errorf("DBDirect.Port = %d, want %d", info.DBDirect.Port, 5432)
	}
	if info.DBDirect.Database != "postgres" {
		t.Errorf("DBDirect.Database = %q, want %q", info.DBDirect.Database, "postgres")
	}
	if info.DBDirect.User != "postgres" {
		t.Errorf("DBDirect.User = %q, want %q", info.DBDirect.User, "postgres")
	}
}

// TestPlatform_ListExtensions verifies that ListExtensions sends a GET to
// /v1/platform/extensions and deserializes the envelope response into
// []Extension with all 10 fields populated.
func TestPlatform_ListExtensions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/v1/platform/extensions" {
			t.Errorf("path = %q, want /v1/platform/extensions", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"name":             "pgvector",
					"display_name":     "pgvector",
					"description":      "Vector similarity search for PostgreSQL",
					"pg_name":          "vector",
					"installed":        true,
					"available":        true,
					"preloaded":        false,
					"requires_preload": false,
					"api_enabled":      true,
					"version":          "0.7.0",
				},
				{
					"name":             "pg_cron",
					"display_name":     "pg_cron",
					"description":      "Job scheduler for PostgreSQL",
					"pg_name":          "pg_cron",
					"installed":        false,
					"available":        true,
					"preloaded":        true,
					"requires_preload": true,
					"api_enabled":      false,
					"version":          "1.6.0",
				},
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-list-ext"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{AdminSecret: "test-secret"})
	exts, err := client.Platform().ListExtensions(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(exts) != 2 {
		t.Fatalf("len(exts) = %d, want 2", len(exts))
	}

	// Verify first extension with all 10 fields.
	if exts[0].Name != "pgvector" {
		t.Errorf("exts[0].Name = %q, want %q", exts[0].Name, "pgvector")
	}
	if exts[0].DisplayName != "pgvector" {
		t.Errorf("exts[0].DisplayName = %q, want %q", exts[0].DisplayName, "pgvector")
	}
	if exts[0].Description != "Vector similarity search for PostgreSQL" {
		t.Errorf("exts[0].Description = %q, want %q", exts[0].Description, "Vector similarity search for PostgreSQL")
	}
	if exts[0].PGName != "vector" {
		t.Errorf("exts[0].PGName = %q, want %q", exts[0].PGName, "vector")
	}
	if !exts[0].Installed {
		t.Error("exts[0].Installed = false, want true")
	}
	if !exts[0].Available {
		t.Error("exts[0].Available = false, want true")
	}
	if exts[0].Preloaded {
		t.Error("exts[0].Preloaded = true, want false")
	}
	if exts[0].RequiresPreload {
		t.Error("exts[0].RequiresPreload = true, want false")
	}
	if !exts[0].APIEnabled {
		t.Error("exts[0].APIEnabled = false, want true")
	}
	if exts[0].Version != "0.7.0" {
		t.Errorf("exts[0].Version = %q, want %q", exts[0].Version, "0.7.0")
	}

	// Spot-check second extension.
	if exts[1].Name != "pg_cron" {
		t.Errorf("exts[1].Name = %q, want %q", exts[1].Name, "pg_cron")
	}
	if exts[1].Installed {
		t.Error("exts[1].Installed = true, want false")
	}
	if !exts[1].RequiresPreload {
		t.Error("exts[1].RequiresPreload = false, want true")
	}
}

// TestPlatform_ToggleExtension verifies that ToggleExtension sends a POST to
// /v1/platform/extensions/{name}/toggle with {"enable": true} in the body
// and returns the updated *Extension.
func TestPlatform_ToggleExtension(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/platform/extensions/pgvector/toggle" {
			t.Errorf("path = %q, want /v1/platform/extensions/pgvector/toggle", r.URL.Path)
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
				"name":   "pgvector",
				"status": "enabled",
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-toggle-ext"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{AdminSecret: "test-secret"})
	result, err := client.Platform().ToggleExtension(context.Background(), "pgvector", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the request body contains {"enable": true}.
	var reqBody map[string]any
	if err := json.Unmarshal(capturedBody, &reqBody); err != nil {
		t.Fatalf("request body is not valid JSON: %v", err)
	}
	enable, ok := reqBody["enable"]
	if !ok {
		t.Fatal("request body missing \"enable\" field")
	}
	if enable != true {
		t.Errorf("request body enable = %v, want true", enable)
	}

	// Verify response.
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Name != "pgvector" {
		t.Errorf("result.Name = %q, want %q", result.Name, "pgvector")
	}
	if result.Status != "enabled" {
		t.Errorf("result.Status = %q, want %q", result.Status, "enabled")
	}
}

// TestPlatform_CreateAuthProvider verifies that CreateAuthProvider sends a POST
// to /v1/platform/projects/{projectID}/auth/providers with the correct JSON
// body and returns the created *AuthProvider.
func TestPlatform_CreateAuthProvider(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/platform/projects/proj-42/auth/providers" {
			t.Errorf("path = %q, want /v1/platform/projects/proj-42/auth/providers", r.URL.Path)
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
				"id":                    "prov-1",
				"project_id":            "proj-42",
				"provider":              "github",
				"client_id":             "gh-client-id",
				"scopes":                []string{"read:user", "user:email"},
				"allowed_redirect_urls": []string{"https://app.example.com/callback"},
				"enabled":               true,
				"created_at":            "2024-05-01T00:00:00Z",
				"updated_at":            "2024-05-01T00:00:00Z",
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-create-prov"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{AdminSecret: "test-secret"})
	provider, err := client.Platform().CreateAuthProvider(context.Background(), "proj-42", CreateProviderRequest{
		Provider:            "github",
		ClientID:            "gh-client-id",
		ClientSecret:        "gh-secret",
		Scopes:              []string{"read:user", "user:email"},
		AllowedRedirectURLs: []string{"https://app.example.com/callback"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the request body.
	var reqBody map[string]any
	if err := json.Unmarshal(capturedBody, &reqBody); err != nil {
		t.Fatalf("request body is not valid JSON: %v", err)
	}
	if reqBody["provider"] != "github" {
		t.Errorf("request body provider = %v, want %q", reqBody["provider"], "github")
	}
	if reqBody["client_id"] != "gh-client-id" {
		t.Errorf("request body client_id = %v, want %q", reqBody["client_id"], "gh-client-id")
	}
	if reqBody["client_secret"] != "gh-secret" {
		t.Errorf("request body client_secret = %v, want %q", reqBody["client_secret"], "gh-secret")
	}

	// Verify response.
	if provider == nil {
		t.Fatal("expected non-nil provider")
	}
	if provider.ID != "prov-1" {
		t.Errorf("provider.ID = %q, want %q", provider.ID, "prov-1")
	}
	if provider.ProjectID != "proj-42" {
		t.Errorf("provider.ProjectID = %q, want %q", provider.ProjectID, "proj-42")
	}
	if provider.Provider != "github" {
		t.Errorf("provider.Provider = %q, want %q", provider.Provider, "github")
	}
	if provider.ClientID != "gh-client-id" {
		t.Errorf("provider.ClientID = %q, want %q", provider.ClientID, "gh-client-id")
	}
	if !provider.Enabled {
		t.Error("provider.Enabled = false, want true")
	}
	if len(provider.Scopes) != 2 {
		t.Fatalf("len(provider.Scopes) = %d, want 2", len(provider.Scopes))
	}
	if provider.Scopes[0] != "read:user" {
		t.Errorf("provider.Scopes[0] = %q, want %q", provider.Scopes[0], "read:user")
	}
	if len(provider.AllowedRedirectURLs) != 1 {
		t.Fatalf("len(provider.AllowedRedirectURLs) = %d, want 1", len(provider.AllowedRedirectURLs))
	}
	if provider.AllowedRedirectURLs[0] != "https://app.example.com/callback" {
		t.Errorf("provider.AllowedRedirectURLs[0] = %q, want %q", provider.AllowedRedirectURLs[0], "https://app.example.com/callback")
	}
}

// TestPlatform_ListAuthProviders verifies that ListAuthProviders sends a GET to
// /v1/platform/projects/{projectID}/auth/providers and returns []AuthProvider.
func TestPlatform_ListAuthProviders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/v1/platform/projects/proj-42/auth/providers" {
			t.Errorf("path = %q, want /v1/platform/projects/proj-42/auth/providers", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id":                    "prov-1",
					"project_id":            "proj-42",
					"provider":              "github",
					"client_id":             "gh-client-id",
					"scopes":                []string{"read:user"},
					"allowed_redirect_urls": []string{"https://app.example.com/callback"},
					"enabled":               true,
					"created_at":            "2024-05-01T00:00:00Z",
					"updated_at":            "2024-05-01T00:00:00Z",
				},
				{
					"id":                    "prov-2",
					"project_id":            "proj-42",
					"provider":              "google",
					"client_id":             "goog-client-id",
					"scopes":                []string{"openid", "email"},
					"allowed_redirect_urls": []string{},
					"enabled":               false,
					"created_at":            "2024-06-01T00:00:00Z",
					"updated_at":            "2024-06-01T00:00:00Z",
				},
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-list-prov"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{AdminSecret: "test-secret"})
	providers, err := client.Platform().ListAuthProviders(context.Background(), "proj-42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(providers) != 2 {
		t.Fatalf("len(providers) = %d, want 2", len(providers))
	}
	if providers[0].ID != "prov-1" {
		t.Errorf("providers[0].ID = %q, want %q", providers[0].ID, "prov-1")
	}
	if providers[0].Provider != "github" {
		t.Errorf("providers[0].Provider = %q, want %q", providers[0].Provider, "github")
	}
	if providers[1].ID != "prov-2" {
		t.Errorf("providers[1].ID = %q, want %q", providers[1].ID, "prov-2")
	}
	if providers[1].Provider != "google" {
		t.Errorf("providers[1].Provider = %q, want %q", providers[1].Provider, "google")
	}
	if providers[1].Enabled {
		t.Error("providers[1].Enabled = true, want false")
	}
}

// TestPlatform_DeleteAuthProvider verifies that DeleteAuthProvider sends a
// DELETE to /v1/platform/projects/{projectID}/auth/providers/{provider} and
// returns nil on a 204 No Content response.
func TestPlatform_DeleteAuthProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		if r.URL.Path != "/v1/platform/projects/proj-42/auth/providers/github" {
			t.Errorf("path = %q, want /v1/platform/projects/proj-42/auth/providers/github", r.URL.Path)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{AdminSecret: "test-secret"})
	err := client.Platform().DeleteAuthProvider(context.Background(), "proj-42", "github")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestPlatform_GetInfo verifies that GetInfo sends a GET to /v1/platform/info
// and returns PlatformInfo with initialization status and auth ref.
func TestPlatform_GetInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/v1/platform/info" {
			t.Errorf("path = %q, want /v1/platform/info", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"initialized": true,
				"auth_ref":    "ref-abc123",
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-info"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{AdminSecret: "test-secret"})
	info, err := client.Platform().GetInfo(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info == nil {
		t.Fatal("expected non-nil info")
	}
	if !info.Initialized {
		t.Error("info.Initialized = false, want true")
	}
	if info.AuthRef != "ref-abc123" {
		t.Errorf("info.AuthRef = %q, want %q", info.AuthRef, "ref-abc123")
	}
}

// TestPlatform_Setup verifies that Setup sends a POST to /v1/platform/setup
// with email and password and returns a SetupResponse containing auth_ref,
// user, and tokens.
func TestPlatform_Setup(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/platform/setup" {
			t.Errorf("path = %q, want /v1/platform/setup", r.URL.Path)
		}

		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		email := "admin@example.com"
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"auth_ref": "ref-setup-abc",
				"user": map[string]any{
					"id":              "user-1",
					"email":           &email,
					"email_confirmed": true,
					"token_version":   1,
					"app_metadata":    map[string]any{"role": "admin"},
					"user_metadata":   map[string]any{},
					"created_at":      "2024-01-01T00:00:00Z",
					"updated_at":      "2024-01-01T00:00:00Z",
				},
				"access_token":  "eyJ.access-token",
				"refresh_token": "eyJ.refresh-token",
				"expires_in":    3600,
			},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-setup"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{})
	resp, err := client.Platform().Setup(context.Background(), SetupRequest{
		Email:    "admin@example.com",
		Password: "secure-password",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify request body.
	var reqBody map[string]string
	if err := json.Unmarshal(capturedBody, &reqBody); err != nil {
		t.Fatalf("request body is not valid JSON: %v", err)
	}
	if reqBody["email"] != "admin@example.com" {
		t.Errorf("request body email = %q, want %q", reqBody["email"], "admin@example.com")
	}
	if reqBody["password"] != "secure-password" {
		t.Errorf("request body password = %q, want %q", reqBody["password"], "secure-password")
	}

	// Verify response.
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.AuthRef != "ref-setup-abc" {
		t.Errorf("resp.AuthRef = %q, want %q", resp.AuthRef, "ref-setup-abc")
	}
	if resp.User == nil {
		t.Fatal("expected non-nil User")
	}
	if resp.User.ID != "user-1" {
		t.Errorf("resp.User.ID = %q, want %q", resp.User.ID, "user-1")
	}
	if resp.User.Email == nil || *resp.User.Email != "admin@example.com" {
		t.Errorf("resp.User.Email = %v, want %q", resp.User.Email, "admin@example.com")
	}
	if !resp.User.EmailConfirmed {
		t.Error("resp.User.EmailConfirmed = false, want true")
	}
	if resp.AccessToken != "eyJ.access-token" {
		t.Errorf("resp.AccessToken = %q, want %q", resp.AccessToken, "eyJ.access-token")
	}
	if resp.RefreshToken != "eyJ.refresh-token" {
		t.Errorf("resp.RefreshToken = %q, want %q", resp.RefreshToken, "eyJ.refresh-token")
	}
	if resp.ExpiresIn != 3600 {
		t.Errorf("resp.ExpiresIn = %d, want %d", resp.ExpiresIn, 3600)
	}
}

// TestPlatform_ListOrganizations_AuthHeader verifies that the AdminSecret is
// sent as a Bearer token in the Authorization header on platform requests.
func TestPlatform_ListOrganizations_AuthHeader(t *testing.T) {
	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":  []map[string]any{},
			"error": nil,
			"meta":  map[string]string{"request_id": "req-auth"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{AdminSecret: "my-admin-secret"})
	_, err := client.Platform().ListOrganizations(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "Bearer my-admin-secret"
	if capturedAuth != expected {
		t.Errorf("Authorization = %q, want %q", capturedAuth, expected)
	}
}
