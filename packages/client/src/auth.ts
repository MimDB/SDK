import { AuthAdminClient } from './auth-admin'
import type { TokenStore } from './auth-store'
import { MimDBError } from './errors'
import type { ApiEnvelope, Tokens, User } from './types'

/**
 * Event types emitted when the authentication state changes.
 *
 * - `SIGNED_IN`       - A user signed in or signed up successfully.
 * - `SIGNED_OUT`      - The current user signed out.
 * - `TOKEN_REFRESHED` - The access token was refreshed.
 */
export type AuthChangeEvent = 'SIGNED_IN' | 'SIGNED_OUT' | 'TOKEN_REFRESHED'

/** @internal Callback signature for auth state change listeners. */
type AuthChangeCallback = (
  event: AuthChangeEvent,
  session: { accessToken: string; refreshToken: string } | null,
) => void

/**
 * Authentication client for email/password sign-in, OAuth, session
 * management, and admin user operations.
 *
 * Manages token storage, fires auth state change events, and provides
 * an admin sub-client for service_role operations.
 *
 * @example
 * ```ts
 * // Accessed via the main MimDB client
 * const { user, tokens } = await mimdb.auth.signIn('user@example.com', 's3cret')
 * const session = mimdb.auth.getSession()
 * ```
 */
export class AuthClient {
  private readonly baseUrl: string
  private readonly ref: string
  private readonly fetchFn: typeof fetch
  private readonly defaultHeaders: Record<string, string>
  private readonly tokenStore: TokenStore
  private readonly listeners: Set<AuthChangeCallback> = new Set()

  private _admin: AuthAdminClient | null = null

  /**
   * Callback invoked whenever the stored access token changes.
   * The MimDBClient uses this to keep the REST Authorization header in sync.
   *
   * @internal
   */
  onTokenChange: ((accessToken: string | null) => void) | null = null

  /**
   * @param baseUrl        - Base URL of the MimDB API.
   * @param ref            - Short project reference ID.
   * @param fetchFn        - Fetch implementation.
   * @param defaultHeaders - Default headers (includes apikey and Authorization with the API key).
   * @param tokenStore     - Storage backend for access/refresh tokens.
   */
  constructor(
    baseUrl: string,
    ref: string,
    fetchFn: typeof fetch,
    defaultHeaders: Record<string, string>,
    tokenStore: TokenStore,
  ) {
    this.baseUrl = baseUrl
    this.ref = ref
    this.fetchFn = fetchFn
    this.defaultHeaders = defaultHeaders
    this.tokenStore = tokenStore
  }

  // ---------------------------------------------------------------------------
  // Email / password
  // ---------------------------------------------------------------------------

  /**
   * Create a new user account with email and password.
   *
   * On success the tokens are stored and a `SIGNED_IN` event is fired.
   *
   * @param email    - User's email address.
   * @param password - Desired password.
   * @param opts     - Optional additional fields.
   * @param opts.userMetadata - Arbitrary metadata to attach to the user profile.
   * @returns The newly created user and their auth tokens.
   * @throws {MimDBError} If the API rejects the sign-up request.
   */
  async signUp(
    email: string,
    password: string,
    opts?: { userMetadata?: Record<string, unknown> },
  ): Promise<{ user: User; tokens: Tokens }> {
    const url = `${this.baseUrl}/v1/auth/${this.ref}/signup`

    const body: Record<string, unknown> = { email, password }
    if (opts?.userMetadata !== undefined) {
      body.user_metadata = opts.userMetadata
    }

    const response = await this.fetchFn(url, {
      method: 'POST',
      headers: { ...this.defaultHeaders },
      body: JSON.stringify(body),
    })

    if (!response.ok) {
      throw await MimDBError.fromResponse(response)
    }

    const envelope = (await response.json()) as ApiEnvelope<{ user: User } & Tokens>
    const result = envelope.data
    const tokens: Tokens = {
      access_token: result.access_token,
      refresh_token: result.refresh_token,
      expires_in: result.expires_in,
    }

    this.setSession(tokens)
    this.emit('SIGNED_IN')
    return { user: result.user, tokens }
  }

  /**
   * Sign in an existing user with email and password.
   *
   * On success the tokens are stored and a `SIGNED_IN` event is fired.
   *
   * @param email    - User's email address.
   * @param password - User's password.
   * @returns The authenticated user and their auth tokens.
   * @throws {MimDBError} If credentials are invalid or the API returns an error.
   */
  async signIn(
    email: string,
    password: string,
  ): Promise<{ user: User; tokens: Tokens }> {
    const url = `${this.baseUrl}/v1/auth/${this.ref}/token?grant_type=password`

    const response = await this.fetchFn(url, {
      method: 'POST',
      headers: { ...this.defaultHeaders },
      body: JSON.stringify({ email, password }),
    })

    if (!response.ok) {
      throw await MimDBError.fromResponse(response)
    }

    const envelope = (await response.json()) as ApiEnvelope<{ user: User } & Tokens>
    const result = envelope.data
    const tokens: Tokens = {
      access_token: result.access_token,
      refresh_token: result.refresh_token,
      expires_in: result.expires_in,
    }

    this.setSession(tokens)
    this.emit('SIGNED_IN')
    return { user: result.user, tokens }
  }

  /**
   * Sign out the current user.
   *
   * Sends a logout request to the API with the stored refresh token
   * so the server can revoke it. Clears stored tokens and fires
   * a `SIGNED_OUT` event.
   *
   * @throws {MimDBError} If the API returns an error response.
   */
  async signOut(): Promise<void> {
    const url = `${this.baseUrl}/v1/auth/${this.ref}/logout`

    const session = this.tokenStore.get()
    const headers: Record<string, string> = { ...this.defaultHeaders }
    if (session) {
      headers['Authorization'] = `Bearer ${session.accessToken}`
    }

    const response = await this.fetchFn(url, {
      method: 'POST',
      headers,
      body: JSON.stringify({
        refresh_token: session?.refreshToken ?? undefined,
      }),
    })

    // 204 No Content is the expected success response
    if (!response.ok && response.status !== 204) {
      throw await MimDBError.fromResponse(response)
    }

    this.clearSession()
    this.emit('SIGNED_OUT')
  }

  /**
   * Refresh the current session using a refresh token.
   *
   * On success the new tokens are stored and a `TOKEN_REFRESHED` event is fired.
   *
   * @param refreshToken - Explicit refresh token. Falls back to the stored token.
   * @returns The new auth tokens.
   * @throws {MimDBError} If the refresh request fails or no refresh token is available.
   */
  async refreshSession(refreshToken?: string): Promise<Tokens> {
    const token = refreshToken ?? this.tokenStore.get()?.refreshToken
    if (!token) {
      throw new MimDBError('No refresh token available', 'AUTH_NO_TOKEN', 0)
    }

    const url = `${this.baseUrl}/v1/auth/${this.ref}/token?grant_type=refresh_token`

    const response = await this.fetchFn(url, {
      method: 'POST',
      headers: { ...this.defaultHeaders },
      body: JSON.stringify({ refresh_token: token }),
    })

    if (!response.ok) {
      throw await MimDBError.fromResponse(response)
    }

    const envelope = (await response.json()) as ApiEnvelope<Tokens>
    const tokens = envelope.data

    this.setSession(tokens)
    this.emit('TOKEN_REFRESHED')
    return tokens
  }

  // ---------------------------------------------------------------------------
  // User
  // ---------------------------------------------------------------------------

  /**
   * Get the currently authenticated user.
   *
   * Sends the stored access token to the API to retrieve the user profile.
   *
   * @returns The current user's profile.
   * @throws {MimDBError} If no session exists or the API returns an error.
   */
  async getUser(): Promise<User> {
    const url = `${this.baseUrl}/v1/auth/${this.ref}/user`
    const session = this.tokenStore.get()

    const headers: Record<string, string> = { ...this.defaultHeaders }
    if (session) {
      headers['Authorization'] = `Bearer ${session.accessToken}`
    }

    const response = await this.fetchFn(url, {
      method: 'GET',
      headers,
    })

    if (!response.ok) {
      throw await MimDBError.fromResponse(response)
    }

    const envelope = (await response.json()) as ApiEnvelope<User>
    return envelope.data
  }

  /**
   * Update the currently authenticated user's profile.
   *
   * @param data - Fields to update.
   * @param data.userMetadata - User-level metadata to merge into the profile.
   * @returns The updated user profile.
   * @throws {MimDBError} If no session exists or the API returns an error.
   */
  async updateUser(
    data: { userMetadata?: Record<string, unknown> },
  ): Promise<User> {
    const url = `${this.baseUrl}/v1/auth/${this.ref}/user`
    const session = this.tokenStore.get()

    const headers: Record<string, string> = { ...this.defaultHeaders }
    if (session) {
      headers['Authorization'] = `Bearer ${session.accessToken}`
    }

    const body: Record<string, unknown> = {}
    if (data.userMetadata !== undefined) {
      body.user_metadata = data.userMetadata
    }

    const response = await this.fetchFn(url, {
      method: 'PUT',
      headers,
      body: JSON.stringify(body),
    })

    if (!response.ok) {
      throw await MimDBError.fromResponse(response)
    }

    const envelope = (await response.json()) as ApiEnvelope<User>
    return envelope.data
  }

  // ---------------------------------------------------------------------------
  // OAuth
  // ---------------------------------------------------------------------------

  /**
   * Build the OAuth sign-in URL for a given provider.
   *
   * The caller should redirect the user to this URL. After authentication
   * the provider will redirect back to `opts.redirectTo` with token
   * fragments in the URL hash.
   *
   * @param provider - OAuth provider name (e.g. 'google', 'github').
   * @param opts     - OAuth options.
   * @param opts.redirectTo - URL the provider should redirect to after auth.
   * @returns The full OAuth authorization URL.
   */
  signInWithOAuth(provider: string, opts: { redirectTo: string }): string {
    return `${this.baseUrl}/v1/auth/${this.ref}/oauth/${provider}?redirect_to=${encodeURIComponent(opts.redirectTo)}`
  }

  /**
   * Parse tokens from a URL hash fragment returned by an OAuth callback.
   *
   * Checks for error fragments first and returns an error result if present.
   *
   * @param urlFragment - The URL hash (e.g. `#access_token=...&refresh_token=...&expires_in=3600`).
   * @returns Parsed tokens on success, an error object if the fragment contains
   *          an error, or null if the required fields are missing.
   */
  handleOAuthCallback(
    urlFragment: string,
  ): { accessToken: string; refreshToken: string; expiresIn: number } | { error: string; errorDescription?: string } | null {
    const hash = urlFragment.startsWith('#') ? urlFragment.slice(1) : urlFragment
    const params = new URLSearchParams(hash)

    // Check for error fragments from the OAuth provider
    const errorParam = params.get('error')
    if (errorParam) {
      return {
        error: errorParam,
        errorDescription: params.get('error_description') ?? undefined,
      }
    }

    const accessToken = params.get('access_token')
    const refreshToken = params.get('refresh_token')
    const expiresInRaw = params.get('expires_in')

    if (!accessToken || !refreshToken || !expiresInRaw) {
      return null
    }

    const expiresIn = parseInt(expiresInRaw, 10)
    if (isNaN(expiresIn)) {
      return null
    }

    this.tokenStore.set(accessToken, refreshToken)
    this.onTokenChange?.(accessToken)
    this.emit('SIGNED_IN')

    return { accessToken, refreshToken, expiresIn }
  }

  // ---------------------------------------------------------------------------
  // Session
  // ---------------------------------------------------------------------------

  /**
   * Get the current session tokens from the store without making a network request.
   *
   * @returns The stored access and refresh tokens, or null if no session exists.
   */
  getSession(): { accessToken: string; refreshToken: string } | null {
    return this.tokenStore.get()
  }

  // ---------------------------------------------------------------------------
  // State change
  // ---------------------------------------------------------------------------

  /**
   * Register a callback to be notified when the auth state changes.
   *
   * @param callback - Function called with the event type and current session.
   * @returns An unsubscribe function that removes the listener.
   *
   * @example
   * ```ts
   * const unsubscribe = mimdb.auth.onAuthStateChange((event, session) => {
   *   if (event === 'SIGNED_OUT') {
   *     router.push('/login')
   *   }
   * })
   *
   * // Later, stop listening
   * unsubscribe()
   * ```
   */
  onAuthStateChange(callback: AuthChangeCallback): () => void {
    this.listeners.add(callback)
    return () => {
      this.listeners.delete(callback)
    }
  }

  // ---------------------------------------------------------------------------
  // Admin
  // ---------------------------------------------------------------------------

  /**
   * Access the admin user management client.
   *
   * Uses the service_role API key (the key passed to `createClient`) for
   * Authorization. Only use this on trusted server-side code.
   */
  get admin(): AuthAdminClient {
    if (!this._admin) {
      this._admin = new AuthAdminClient(
        this.baseUrl,
        this.ref,
        this.fetchFn,
        { ...this.defaultHeaders },
      )
    }
    return this._admin
  }

  // ---------------------------------------------------------------------------
  // Internal helpers
  // ---------------------------------------------------------------------------

  /**
   * Store tokens and notify the parent client about the new access token.
   *
   * @internal
   */
  private setSession(tokens: Tokens): void {
    this.tokenStore.set(tokens.access_token, tokens.refresh_token)
    this.onTokenChange?.(tokens.access_token)
  }

  /**
   * Clear stored tokens and notify the parent client.
   *
   * @internal
   */
  private clearSession(): void {
    this.tokenStore.clear()
    this.onTokenChange?.(null)
  }

  /**
   * Fire an auth state change event to all registered listeners.
   *
   * @internal
   */
  private emit(event: AuthChangeEvent): void {
    const session = this.tokenStore.get()
    for (const callback of this.listeners) {
      callback(event, session)
    }
  }
}
