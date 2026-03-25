package mimdb

import (
	"context"
	"fmt"
	"net/http"
)

// PlatformClient provides access to the MimDB Platform API for managing
// organizations, projects, API keys, and infrastructure-level resources.
//
// Obtain a PlatformClient via [Client.Platform]. It requires an AdminSecret
// to be configured on the parent Client.
type PlatformClient struct {
	client *Client
}

// CreateOrgRequest holds the parameters for creating a new organization.
type CreateOrgRequest struct {
	// Name is the display name of the organization.
	Name string `json:"name"`

	// Slug is a URL-friendly identifier for the organization. It must be unique
	// across the platform.
	Slug string `json:"slug"`
}

// CreateProjectRequest holds the parameters for creating a new project.
type CreateProjectRequest struct {
	// Name is the display name of the project.
	Name string `json:"name"`

	// OrgID is the ID of the organization that will own the project.
	OrgID string `json:"org_id"`
}

// CreateProviderRequest holds the parameters for configuring an OAuth or social
// auth provider on a project.
type CreateProviderRequest struct {
	// Provider is the provider identifier (e.g. "github", "google").
	Provider string `json:"provider"`

	// ClientID is the OAuth client ID obtained from the provider.
	ClientID string `json:"client_id"`

	// ClientSecret is the OAuth client secret obtained from the provider.
	ClientSecret string `json:"client_secret"`

	// Scopes lists the OAuth scopes to request during authentication.
	Scopes []string `json:"scopes,omitempty"`

	// AllowedRedirectURLs lists the URLs that the provider may redirect to
	// after authentication.
	AllowedRedirectURLs []string `json:"allowed_redirect_urls,omitempty"`
}

// PlatformInfo describes the current state of a MimDB platform instance,
// including whether initial setup has been completed.
type PlatformInfo struct {
	// Initialized indicates whether the platform has completed first-time
	// setup.
	Initialized bool `json:"initialized"`

	// AuthRef is the project ref of the auth project, present only after
	// initialization.
	AuthRef string `json:"auth_ref,omitempty"`
}

// SetupRequest holds the credentials for the initial platform setup.
type SetupRequest struct {
	// Email is the email address for the initial admin user.
	Email string `json:"email"`

	// Password is the password for the initial admin user.
	Password string `json:"password"`
}

// SetupResponse contains the result of a successful platform setup, including
// the auth project ref, the created admin user, and session tokens. The backend
// returns token fields at the top level (not nested under a "tokens" key).
type SetupResponse struct {
	// AuthRef is the project ref assigned to the auth project.
	AuthRef string `json:"auth_ref"`

	// User is the newly created admin user.
	User *User `json:"user"`

	// AccessToken is the JWT access token for the admin session.
	AccessToken string `json:"access_token"`

	// RefreshToken is the refresh token for obtaining new access tokens.
	RefreshToken string `json:"refresh_token"`

	// ExpiresIn is the number of seconds until the access token expires.
	ExpiresIn int `json:"expires_in"`
}

// toggleExtensionRequest is the internal request body for enabling/disabling
// an extension.
type toggleExtensionRequest struct {
	Enable bool `json:"enable"`
}

// orgBasePath is the URL prefix for all organization endpoints.
const orgBasePath = "/v1/platform/organizations"

// projectBasePath is the URL prefix for project-level platform endpoints.
const projectBasePath = "/v1/platform/projects"

// extensionBasePath is the URL prefix for platform-wide extension endpoints.
const extensionBasePath = "/v1/platform/extensions"

// platformBasePath is the URL prefix for platform info and setup endpoints.
const platformBasePath = "/v1/platform"

// ListOrganizations retrieves all organizations accessible to the authenticated
// admin.
//
// NOTE: This endpoint requires platform JWT authentication. It may not work
// with AdminSecret-only clients. Use [PlatformClient.GetOrganization] with a
// known org ID as an alternative.
//
//	orgs, err := client.Platform().ListOrganizations(ctx)
func (p *PlatformClient) ListOrganizations(ctx context.Context) ([]Organization, error) {
	var orgs []Organization
	err := p.client.transport.Do(ctx, http.MethodGet, orgBasePath, nil, &orgs)
	if err != nil {
		return nil, wrapTransportError(err)
	}
	return orgs, nil
}

// CreateOrganization creates a new organization with the given name and slug.
// Returns the newly created organization.
//
//	org, err := client.Platform().CreateOrganization(ctx, mimdb.CreateOrgRequest{
//	    Name: "My Org",
//	    Slug: "my-org",
//	})
func (p *PlatformClient) CreateOrganization(ctx context.Context, req CreateOrgRequest) (*Organization, error) {
	var org Organization
	err := p.client.transport.Do(ctx, http.MethodPost, orgBasePath, req, &org)
	if err != nil {
		return nil, wrapTransportError(err)
	}
	return &org, nil
}

// GetOrganization retrieves a single organization by its ID.
//
//	org, err := client.Platform().GetOrganization(ctx, "org-abc123")
func (p *PlatformClient) GetOrganization(ctx context.Context, orgID string) (*Organization, error) {
	path := fmt.Sprintf("%s/%s", orgBasePath, orgID)
	var org Organization
	err := p.client.transport.Do(ctx, http.MethodGet, path, nil, &org)
	if err != nil {
		return nil, wrapTransportError(err)
	}
	return &org, nil
}

// ListProjects retrieves all projects belonging to the specified organization.
//
//	projects, err := client.Platform().ListProjects(ctx, "org-abc123")
func (p *PlatformClient) ListProjects(ctx context.Context, orgID string) ([]Project, error) {
	path := fmt.Sprintf("%s/%s/projects", orgBasePath, orgID)
	var projects []Project
	err := p.client.transport.Do(ctx, http.MethodGet, path, nil, &projects)
	if err != nil {
		return nil, wrapTransportError(err)
	}
	return projects, nil
}

// CreateProject creates a new project within the specified organization. The
// response includes the project along with its auto-generated API keys and
// database connection string.
//
//	result, err := client.Platform().CreateProject(ctx, mimdb.CreateProjectRequest{
//	    Name:  "My Project",
//	    OrgID: "org-abc123",
//	})
func (p *PlatformClient) CreateProject(ctx context.Context, req CreateProjectRequest) (*ProjectWithKeys, error) {
	var result ProjectWithKeys
	err := p.client.transport.Do(ctx, http.MethodPost, projectBasePath, req, &result)
	if err != nil {
		return nil, wrapTransportError(err)
	}
	return &result, nil
}

// GetProject retrieves a single project by its ID.
//
//	proj, err := client.Platform().GetProject(ctx, "proj-abc123")
func (p *PlatformClient) GetProject(ctx context.Context, projectID string) (*Project, error) {
	path := fmt.Sprintf("%s/%s", projectBasePath, projectID)
	var proj Project
	err := p.client.transport.Do(ctx, http.MethodGet, path, nil, &proj)
	if err != nil {
		return nil, wrapTransportError(err)
	}
	return &proj, nil
}

// RotateDBCredential rotates the database credentials for the specified project
// and returns the new connection string. The backend responds with
// {"db_connection_string": "postgres://..."}.
//
//	connStr, err := client.Platform().RotateDBCredential(ctx, "proj-abc123")
func (p *PlatformClient) RotateDBCredential(ctx context.Context, projectID string) (string, error) {
	path := fmt.Sprintf("%s/%s/rotate-db-credential", projectBasePath, projectID)
	var data map[string]string
	err := p.client.transport.Do(ctx, http.MethodPost, path, nil, &data)
	if err != nil {
		return "", wrapTransportError(err)
	}
	connStr, ok := data["db_connection_string"]
	if !ok {
		return "", fmt.Errorf("mimdb: rotate-db-credential response missing \"db_connection_string\" field")
	}
	return connStr, nil
}

// GetAPIKeys retrieves the current anon and service-role API key strings for
// the specified project.
//
//	keys, err := client.Platform().GetAPIKeys(ctx, "proj-abc123")
func (p *PlatformClient) GetAPIKeys(ctx context.Context, projectID string) (*APIKeys, error) {
	path := fmt.Sprintf("%s/%s/api-keys", projectBasePath, projectID)
	var keys APIKeys
	err := p.client.transport.Do(ctx, http.MethodGet, path, nil, &keys)
	if err != nil {
		return nil, wrapTransportError(err)
	}
	return &keys, nil
}

// RegenerateAPIKeys regenerates the anon and service-role API keys for the
// specified project and returns the new key strings.
//
//	keys, err := client.Platform().RegenerateAPIKeys(ctx, "proj-abc123")
func (p *PlatformClient) RegenerateAPIKeys(ctx context.Context, projectID string) (*APIKeys, error) {
	path := fmt.Sprintf("%s/%s/api-keys/regenerate", projectBasePath, projectID)
	var keys APIKeys
	err := p.client.transport.Do(ctx, http.MethodPost, path, nil, &keys)
	if err != nil {
		return nil, wrapTransportError(err)
	}
	return &keys, nil
}

// GetConnectionInfo retrieves database connection endpoints for the specified
// project, including both pooled and direct connection details.
//
//	info, err := client.Platform().GetConnectionInfo(ctx, "proj-abc123")
func (p *PlatformClient) GetConnectionInfo(ctx context.Context, projectID string) (*ConnectionInfo, error) {
	path := fmt.Sprintf("%s/%s/connection-info", projectBasePath, projectID)
	var info ConnectionInfo
	err := p.client.transport.Do(ctx, http.MethodGet, path, nil, &info)
	if err != nil {
		return nil, wrapTransportError(err)
	}
	return &info, nil
}

// ---------- Extensions (server-wide) ----------

// ListExtensions retrieves all available PostgreSQL extensions and their
// installation status across the platform.
//
//	exts, err := client.Platform().ListExtensions(ctx)
func (p *PlatformClient) ListExtensions(ctx context.Context) ([]Extension, error) {
	var exts []Extension
	err := p.client.transport.Do(ctx, http.MethodGet, extensionBasePath, nil, &exts)
	if err != nil {
		return nil, wrapTransportError(err)
	}
	return exts, nil
}

// ToggleExtensionResult holds the response from toggling a PostgreSQL extension.
// The backend returns only the extension name and its new status (e.g. "enabled"
// or "disabled"), not the full Extension struct.
type ToggleExtensionResult struct {
	// Name is the identifier of the toggled extension.
	Name string `json:"name"`

	// Status is the new state of the extension (e.g. "enabled", "disabled").
	Status string `json:"status"`
}

// ToggleExtension enables or disables a PostgreSQL extension by name. The
// enable parameter controls whether the extension should be installed (true)
// or uninstalled (false). Returns the extension name and its new status.
//
//	result, err := client.Platform().ToggleExtension(ctx, "pgvector", true)
func (p *PlatformClient) ToggleExtension(ctx context.Context, name string, enable bool) (*ToggleExtensionResult, error) {
	path := fmt.Sprintf("%s/%s/toggle", extensionBasePath, name)
	var result ToggleExtensionResult
	err := p.client.transport.Do(ctx, http.MethodPost, path, toggleExtensionRequest{Enable: enable}, &result)
	if err != nil {
		return nil, wrapTransportError(err)
	}
	return &result, nil
}

// ---------- Auth Providers (per-project) ----------

// CreateAuthProvider configures a new OAuth or social auth provider for the
// specified project. The request must include the provider name, client
// credentials, and optionally scopes and redirect URLs.
//
//	provider, err := client.Platform().CreateAuthProvider(ctx, "proj-abc123", mimdb.CreateProviderRequest{
//	    Provider:     "github",
//	    ClientID:     "gh-client-id",
//	    ClientSecret: "gh-secret",
//	})
func (p *PlatformClient) CreateAuthProvider(ctx context.Context, projectID string, req CreateProviderRequest) (*AuthProvider, error) {
	path := fmt.Sprintf("%s/%s/auth/providers", projectBasePath, projectID)
	var provider AuthProvider
	err := p.client.transport.Do(ctx, http.MethodPost, path, req, &provider)
	if err != nil {
		return nil, wrapTransportError(err)
	}
	return &provider, nil
}

// ListAuthProviders retrieves all configured auth providers for the specified
// project.
//
//	providers, err := client.Platform().ListAuthProviders(ctx, "proj-abc123")
func (p *PlatformClient) ListAuthProviders(ctx context.Context, projectID string) ([]AuthProvider, error) {
	path := fmt.Sprintf("%s/%s/auth/providers", projectBasePath, projectID)
	var providers []AuthProvider
	err := p.client.transport.Do(ctx, http.MethodGet, path, nil, &providers)
	if err != nil {
		return nil, wrapTransportError(err)
	}
	return providers, nil
}

// DeleteAuthProvider removes an auth provider configuration from the specified
// project. The server responds with 204 No Content on success.
//
//	err := client.Platform().DeleteAuthProvider(ctx, "proj-abc123", "github")
func (p *PlatformClient) DeleteAuthProvider(ctx context.Context, projectID string, provider string) error {
	path := fmt.Sprintf("%s/%s/auth/providers/%s", projectBasePath, projectID, provider)
	err := p.client.transport.Do(ctx, http.MethodDelete, path, nil, nil)
	if err != nil {
		return wrapTransportError(err)
	}
	return nil
}

// ---------- Platform Info & Setup ----------

// GetInfo retrieves the current state of the MimDB platform instance,
// including whether initial setup has been completed and the auth project ref.
//
//	info, err := client.Platform().GetInfo(ctx)
func (p *PlatformClient) GetInfo(ctx context.Context) (*PlatformInfo, error) {
	path := fmt.Sprintf("%s/info", platformBasePath)
	var info PlatformInfo
	err := p.client.transport.Do(ctx, http.MethodGet, path, nil, &info)
	if err != nil {
		return nil, wrapTransportError(err)
	}
	return &info, nil
}

// Setup performs first-time platform initialization by creating the initial
// admin user with the provided email and password. Returns the auth project
// ref, the created user, and session tokens.
//
//	resp, err := client.Platform().Setup(ctx, mimdb.SetupRequest{
//	    Email:    "admin@example.com",
//	    Password: "secure-password",
//	})
func (p *PlatformClient) Setup(ctx context.Context, req SetupRequest) (*SetupResponse, error) {
	path := fmt.Sprintf("%s/setup", platformBasePath)
	var resp SetupResponse
	err := p.client.transport.Do(ctx, http.MethodPost, path, req, &resp)
	if err != nil {
		return nil, wrapTransportError(err)
	}
	return &resp, nil
}
