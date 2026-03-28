package mimdb

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/MimDB/SDK/go/internal/transport"
)

// AuthClient provides access to the MimDB authentication API for user sign-up,
// sign-in, token refresh, and session logout.
//
// Obtain an AuthClient via [Client.Auth]. It requires a ProjectRef to be
// configured on the parent Client, since all auth endpoints are scoped to a
// specific project.
type AuthClient struct {
	client *Client
}

// authResponse is the internal representation of the backend's auth token
// response. Both SignUp and SignIn return this shape; the public API
// destructures it into separate *User and *Tokens values.
type authResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	User         User   `json:"user"`
}

// authCredentials is the request body for email/password authentication.
type authCredentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// refreshRequest is the request body for token refresh.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// logoutRequest is the request body for session logout.
type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// authBasePath returns the URL prefix for auth endpoints scoped to the given
// project ref.
func authBasePath(ref string) string {
	return fmt.Sprintf("/v1/auth/%s", ref)
}

// SignUp creates a new user account with the given email and password. On
// success it returns the created User and the initial session Tokens.
//
//	user, tokens, err := client.Auth().SignUp(ctx, "user@example.com", "password")
func (a *AuthClient) SignUp(ctx context.Context, email, password string) (*User, *Tokens, error) {
	if err := a.client.requireProjectRef(); err != nil {
		return nil, nil, err
	}

	path := fmt.Sprintf("%s/signup", authBasePath(a.client.projectRef))
	body := authCredentials{Email: email, Password: password}

	var resp authResponse
	if err := a.client.transport.Do(ctx, http.MethodPost, path, body, &resp); err != nil {
		return nil, nil, wrapTransportError(err)
	}

	return a.splitResponse(&resp)
}

// SignIn authenticates an existing user with email and password. On success it
// returns the authenticated User and session Tokens.
//
// This hits the /v1/auth/{ref}/token endpoint with grant_type=password.
//
//	user, tokens, err := client.Auth().SignIn(ctx, "user@example.com", "password")
func (a *AuthClient) SignIn(ctx context.Context, email, password string) (*User, *Tokens, error) {
	if err := a.client.requireProjectRef(); err != nil {
		return nil, nil, err
	}

	path := fmt.Sprintf("%s/token?grant_type=password", authBasePath(a.client.projectRef))
	body := authCredentials{Email: email, Password: password}

	var resp authResponse
	if err := a.client.transport.Do(ctx, http.MethodPost, path, body, &resp); err != nil {
		return nil, nil, wrapTransportError(err)
	}

	return a.splitResponse(&resp)
}

// Refresh exchanges a valid refresh token for a new access/refresh token pair.
// The user object in the response is discarded; only the new Tokens are
// returned.
//
// This hits the /v1/auth/{ref}/token endpoint with grant_type=refresh_token.
//
//	tokens, err := client.Auth().Refresh(ctx, refreshToken)
func (a *AuthClient) Refresh(ctx context.Context, refreshToken string) (*Tokens, error) {
	if err := a.client.requireProjectRef(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("%s/token?grant_type=refresh_token", authBasePath(a.client.projectRef))
	body := refreshRequest{RefreshToken: refreshToken}

	var resp authResponse
	if err := a.client.transport.Do(ctx, http.MethodPost, path, body, &resp); err != nil {
		return nil, wrapTransportError(err)
	}

	return &Tokens{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresIn:    resp.ExpiresIn,
	}, nil
}

// Logout revokes the session associated with the given refresh token. The
// server responds with 204 No Content on success.
//
//	err := client.Auth().Logout(ctx, refreshToken)
func (a *AuthClient) Logout(ctx context.Context, refreshToken string) error {
	if err := a.client.requireProjectRef(); err != nil {
		return err
	}

	path := fmt.Sprintf("%s/logout", authBasePath(a.client.projectRef))
	body := logoutRequest{RefreshToken: refreshToken}

	if err := a.client.transport.Do(ctx, http.MethodPost, path, body, nil); err != nil {
		return wrapTransportError(err)
	}
	return nil
}

// SetSession restores a previously saved session by setting both the access
// token and refresh token on the client. This allows applications to persist
// tokens (e.g. to disk or secure storage) and restore them on restart without
// making a network call.
//
// The access token is applied to the parent Client so subsequent API calls
// authenticate as the restored user. The refresh token is returned alongside
// the access token in the resulting Tokens value, enabling callers to later
// call [AuthClient.Refresh] when the access token expires.
//
// SetSession is the Go equivalent of the JS SDK's auth.setSession().
//
//	err := client.Auth().SetSession("eyJhbGci...", "v1.refresh-token-abc")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Subsequent calls use the restored session.
//	user, err := client.Auth().GetUser(ctx)
func (a *AuthClient) SetSession(accessToken, refreshToken string) (*Tokens, error) {
	if accessToken == "" {
		return nil, fmt.Errorf("accessToken must not be empty")
	}
	if refreshToken == "" {
		return nil, fmt.Errorf("refreshToken must not be empty")
	}

	a.client.SetAccessToken(accessToken)

	return &Tokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// splitResponse destructures an authResponse into separate *User and *Tokens
// values. This is shared by SignUp and SignIn.
func (a *AuthClient) splitResponse(resp *authResponse) (*User, *Tokens, error) {
	user := resp.User
	tokens := Tokens{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresIn:    resp.ExpiresIn,
	}
	return &user, &tokens, nil
}

// userRequestOptions returns transport.RequestOptions that include the current
// user's access token in the Authorization header. This is used for endpoints
// that operate on behalf of an authenticated end-user.
func (a *AuthClient) userRequestOptions() transport.RequestOptions {
	return transport.RequestOptions{
		AccessToken: a.client.getAccessToken(),
	}
}

// adminRequestOptions returns transport.RequestOptions that send the API key
// as a Bearer token in the Authorization header. Admin auth endpoints (e.g.
// /v1/auth/{ref}/users) are protected by RequireProjectJWT middleware which
// expects "Authorization: Bearer {jwt}". The service_role API key is itself a
// JWT, so passing it as a Bearer token satisfies the middleware.
func (a *AuthClient) adminRequestOptions() transport.RequestOptions {
	return transport.RequestOptions{
		AccessToken: a.client.options.APIKey,
	}
}

// ---------- User Management ----------

// UpdateUserRequest is the request body for updating the authenticated user's
// profile. Only user_metadata is updatable by the user themselves.
type UpdateUserRequest struct {
	UserMetadata map[string]any `json:"user_metadata"`
}

// GetUser retrieves the profile of the currently authenticated user. The
// user's access token (set via [Client.SetAccessToken]) is sent as the
// Authorization header.
//
//	user, err := client.Auth().GetUser(ctx)
func (a *AuthClient) GetUser(ctx context.Context) (*User, error) {
	if err := a.client.requireProjectRef(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("%s/user", authBasePath(a.client.projectRef))

	var user User
	if err := a.client.transport.Do(ctx, http.MethodGet, path, nil, &user, a.userRequestOptions()); err != nil {
		return nil, wrapTransportError(err)
	}
	return &user, nil
}

// UpdateUser updates the authenticated user's profile. Only user_metadata
// can be modified by the user. The user's access token is sent as the
// Authorization header.
//
//	user, err := client.Auth().UpdateUser(ctx, mimdb.UpdateUserRequest{
//	    UserMetadata: map[string]any{"theme": "dark"},
//	})
func (a *AuthClient) UpdateUser(ctx context.Context, req UpdateUserRequest) (*User, error) {
	if err := a.client.requireProjectRef(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("%s/user", authBasePath(a.client.projectRef))

	var user User
	if err := a.client.transport.Do(ctx, http.MethodPut, path, req, &user, a.userRequestOptions()); err != nil {
		return nil, wrapTransportError(err)
	}
	return &user, nil
}

// ListSessions returns all active sessions for the currently authenticated
// user. The user's access token is sent as the Authorization header.
//
//	sessions, err := client.Auth().ListSessions(ctx)
func (a *AuthClient) ListSessions(ctx context.Context) ([]Session, error) {
	if err := a.client.requireProjectRef(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("%s/sessions", authBasePath(a.client.projectRef))

	var sessions []Session
	if err := a.client.transport.Do(ctx, http.MethodGet, path, nil, &sessions, a.userRequestOptions()); err != nil {
		return nil, wrapTransportError(err)
	}
	return sessions, nil
}

// ---------- Email Verification ----------

// verifyEmailRequest is the request body for email verification.
type verifyEmailRequest struct {
	Token string `json:"token"`
}

// VerifyEmail confirms a user's email address using the verification token
// sent via email. The server responds with 204 No Content on success.
//
//	err := client.Auth().VerifyEmail(ctx, token)
func (a *AuthClient) VerifyEmail(ctx context.Context, token string) error {
	if err := a.client.requireProjectRef(); err != nil {
		return err
	}

	path := fmt.Sprintf("%s/verify", authBasePath(a.client.projectRef))
	body := verifyEmailRequest{Token: token}

	if err := a.client.transport.Do(ctx, http.MethodPost, path, body, nil); err != nil {
		return wrapTransportError(err)
	}
	return nil
}

// ---------- OAuth ----------

// OAuthURL constructs the OAuth authorization URL for the given provider. No
// HTTP call is made; the URL is built from the client's base URL, project ref,
// provider name, and the desired redirect URL.
//
//	url := client.Auth().OAuthURL("github", "https://myapp.com/callback")
func (a *AuthClient) OAuthURL(provider, redirectURL string) string {
	return fmt.Sprintf(
		"%s/v1/auth/%s/oauth/%s?redirect_to=%s",
		a.client.baseURL,
		a.client.projectRef,
		provider,
		url.QueryEscape(redirectURL),
	)
}

// ---------- Admin ----------

// AdminUpdateUserRequest is the request body for admin-level user updates.
// Only app_metadata is patchable via the admin endpoint.
type AdminUpdateUserRequest struct {
	AppMetadata map[string]any `json:"app_metadata"`
}

// AdminListUsersOptions configures optional filtering and pagination for the
// admin user listing endpoint.
type AdminListUsersOptions struct {
	// Email filters users by email address (exact match).
	Email string

	// Limit controls the maximum number of users returned.
	Limit int

	// Offset skips the first N users for pagination.
	Offset int
}

// AdminListUsers returns a list of users in the project. This requires a
// service_role API key. Optional filtering and pagination can be applied via
// AdminListUsersOptions.
//
// The endpoint is /v1/auth/{ref}/users - the service_role key provides admin
// access without a separate /admin/ prefix.
//
//	users, err := client.Auth().AdminListUsers(ctx)
//	users, err := client.Auth().AdminListUsers(ctx, mimdb.AdminListUsersOptions{Limit: 50})
func (a *AuthClient) AdminListUsers(ctx context.Context, opts ...AdminListUsersOptions) ([]User, error) {
	if err := a.client.requireProjectRef(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("%s/users", authBasePath(a.client.projectRef))

	// Apply optional query parameters.
	if len(opts) > 0 {
		opt := opts[0]
		params := url.Values{}
		if opt.Email != "" {
			params.Set("email", opt.Email)
		}
		if opt.Limit > 0 {
			params.Set("limit", strconv.Itoa(opt.Limit))
		}
		if opt.Offset > 0 {
			params.Set("offset", strconv.Itoa(opt.Offset))
		}
		if encoded := params.Encode(); encoded != "" {
			path += "?" + encoded
		}
	}

	var users []User
	if err := a.client.transport.Do(ctx, http.MethodGet, path, nil, &users, a.adminRequestOptions()); err != nil {
		return nil, wrapTransportError(err)
	}
	return users, nil
}

// AdminUpdateUser updates a user's app_metadata by user ID. This requires a
// service_role API key. Only app_metadata is patchable.
//
//	user, err := client.Auth().AdminUpdateUser(ctx, userID, mimdb.AdminUpdateUserRequest{
//	    AppMetadata: map[string]any{"role": "admin"},
//	})
func (a *AuthClient) AdminUpdateUser(ctx context.Context, userID string, req AdminUpdateUserRequest) (*User, error) {
	if err := a.client.requireProjectRef(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("%s/users/%s", authBasePath(a.client.projectRef), userID)

	var user User
	if err := a.client.transport.Do(ctx, http.MethodPatch, path, req, &user, a.adminRequestOptions()); err != nil {
		return nil, wrapTransportError(err)
	}
	return &user, nil
}

// ---------- Password Reset ----------

// forgotPasswordRequest is the request body for initiating a password reset.
type forgotPasswordRequest struct {
	Email string `json:"email"`
}

// resetPasswordRequest is the request body for completing a password reset.
type resetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

// ForgotPassword sends a password reset email to the given address. The server
// responds with 204 No Content on success regardless of whether the email
// exists, to prevent user enumeration.
//
//	err := client.Auth().ForgotPassword(ctx, "user@example.com")
func (a *AuthClient) ForgotPassword(ctx context.Context, email string) error {
	if err := a.client.requireProjectRef(); err != nil {
		return err
	}

	path := fmt.Sprintf("%s/forgot-password", authBasePath(a.client.projectRef))
	body := forgotPasswordRequest{Email: email}

	if err := a.client.transport.Do(ctx, http.MethodPost, path, body, nil); err != nil {
		return wrapTransportError(err)
	}
	return nil
}

// ResetPassword completes a password reset using the token sent via email and
// the user's chosen new password. The server responds with 204 No Content on
// success.
//
//	err := client.Auth().ResetPassword(ctx, token, "newSecurePassword")
func (a *AuthClient) ResetPassword(ctx context.Context, token, newPassword string) error {
	if err := a.client.requireProjectRef(); err != nil {
		return err
	}

	path := fmt.Sprintf("%s/reset-password", authBasePath(a.client.projectRef))
	body := resetPasswordRequest{Token: token, NewPassword: newPassword}

	if err := a.client.transport.Do(ctx, http.MethodPost, path, body, nil); err != nil {
		return wrapTransportError(err)
	}
	return nil
}
