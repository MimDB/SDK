import { MimDBRealtimeClient } from '@mimdb/realtime'
import { AuthClient } from './auth'
import { InMemoryTokenStore } from './auth-store'
import type { TokenStore } from './auth-store'
import { MimDBError } from './errors'
import { fetchWithRetry } from './retry'
import { QueryBuilder } from './rest'
import type { QueryMiddleware } from './rest'
import { StorageClient } from './storage'
import type { ClientOptions, QueryResult, RetryOptions } from './types'

/**
 * MimDB client - the main entry point for interacting with a Mimisbrunnr project.
 *
 * Provides access to the PostgREST-compatible REST API for querying and
 * mutating database tables, and for calling PostgreSQL functions via RPC.
 *
 * @example
 * ```ts
 * import { createClient } from '@mimdb/client'
 *
 * const mimdb = createClient('https://api.mimdb.dev', '40891b0d', 'eyJ...')
 *
 * const { data, error } = await mimdb
 *   .from('todos')
 *   .select('id, task, done')
 *   .eq('done', 'false')
 *   .limit(10)
 * ```
 */
export class MimDBClient {
  private readonly baseUrl: string
  private readonly ref: string
  private readonly apiKey: string
  private readonly fetchFn: typeof fetch
  private readonly defaultHeaders: Record<string, string>
  private _auth: AuthClient | null = null
  private _storage: StorageClient | null = null
  private _realtime: MimDBRealtimeClient | null = null
  private readonly tokenStore: TokenStore
  private readonly options?: ClientOptions
  private readonly middleware: QueryMiddleware

  /**
   * @param url        - Base URL of the MimDB API (e.g. `https://api.mimdb.dev`).
   * @param projectRef - Short project reference ID.
   * @param apiKey     - API key for authentication.
   * @param options    - Optional client configuration.
   */
  constructor(
    url: string,
    projectRef: string,
    apiKey: string,
    options?: ClientOptions,
  ) {
    // Strip trailing slash for consistent URL construction
    this.baseUrl = url.replace(/\/+$/, '')
    this.ref = projectRef
    this.apiKey = apiKey
    this.fetchFn = options?.fetch ?? globalThis.fetch.bind(globalThis)

    this.defaultHeaders = {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${this.apiKey}`,
      'apikey': this.apiKey,
      ...options?.headers,
    }

    this.tokenStore = options?.tokenStore ?? new InMemoryTokenStore()
    this.options = options

    // Build middleware from options
    const retryOpts = options?.retry
    const resolvedRetry: RetryOptions | undefined =
      retryOpts === true ? {} : retryOpts === false ? undefined : retryOpts ?? undefined

    this.middleware = {
      retry: resolvedRetry,
      onRequest: options?.onRequest,
      onResponse: options?.onResponse,
    }
  }

  /**
   * Return the core configuration values used to construct this client.
   *
   * Useful when companion packages (e.g. `@mimdb/react`) need access to
   * the URL, project reference, and API key to create their own service
   * clients such as `MimDBRealtimeClient`.
   *
   * @returns An object containing the base URL, project ref, and API key.
   */
  getConfig(): { url: string; ref: string; apiKey: string } {
    return { url: this.baseUrl, ref: this.ref, apiKey: this.apiKey }
  }

  /**
   * Access the authentication client for sign-up, sign-in, OAuth, and
   * session management.
   *
   * The auth client is lazily instantiated on first access. When a user
   * signs in or refreshes their session, the access token is automatically
   * applied to subsequent REST queries.
   *
   * @example
   * ```ts
   * const { user } = await client.auth.signIn('user@example.com', 'password')
   * // REST queries now use the authenticated user's token
   * const { data } = await client.from('todos').select('*')
   * ```
   */
  get auth(): AuthClient {
    if (!this._auth) {
      this._auth = new AuthClient(
        this.baseUrl,
        this.ref,
        this.fetchFn,
        { ...this.defaultHeaders },
        this.tokenStore,
        this.options?.autoRefresh,
      )

      // Keep the REST Authorization header in sync with auth token changes
      this._auth.onTokenChange = (accessToken) => {
        if (accessToken) {
          this.defaultHeaders['Authorization'] = `Bearer ${accessToken}`
        } else {
          this.defaultHeaders['Authorization'] = `Bearer ${this.apiKey}`
        }
      }
    }

    return this._auth
  }

  /**
   * Access the storage client for file uploads, downloads, signed URLs,
   * and bucket management.
   *
   * The storage client is lazily instantiated on first access and shares
   * the same auth headers as the REST client.
   *
   * @example
   * ```ts
   * // Upload a file
   * const { path } = await client.storage.from('avatars').upload('photo.png', blob)
   *
   * // Get a public URL
   * const url = client.storage.from('avatars').getPublicUrl('photo.png')
   * ```
   */
  /**
   * Access the realtime client for subscribing to live database changes
   * via WebSocket.
   *
   * The realtime client is lazily instantiated on first access using
   * the same URL, project ref, and API key as the main client.
   *
   * @example
   * ```ts
   * const sub = client.realtime.subscribe('messages', {
   *   event: 'INSERT',
   *   onEvent: (e) => console.log('New message:', e.new),
   * })
   * ```
   */
  get realtime(): MimDBRealtimeClient {
    if (!this._realtime) {
      this._realtime = new MimDBRealtimeClient({
        url: this.baseUrl,
        projectRef: this.ref,
        apiKey: this.apiKey,
      })
    }
    return this._realtime
  }

  get storage(): StorageClient {
    if (!this._storage) {
      this._storage = new StorageClient(
        this.baseUrl,
        this.ref,
        this.fetchFn,
        this.defaultHeaders,
      )
    }
    return this._storage
  }

  /**
   * Start building a query against a database table.
   *
   * Returns a `QueryBuilder` that supports PostgREST filter, modifier, and
   * mutation methods. The builder is thenable and can be awaited directly.
   *
   * @typeParam T - Expected row type. Defaults to a generic record.
   * @param table - Name of the table to query.
   * @returns A new `QueryBuilder` scoped to the given table.
   *
   * @example
   * ```ts
   * const { data } = await client.from('users').select('id, name').limit(5)
   * ```
   */
  from<T = Record<string, unknown>>(table: string): QueryBuilder<T> {
    const restUrl = `${this.baseUrl}/v1/rest/${this.ref}`
    return new QueryBuilder<T>(restUrl, table, this.fetchFn, { ...this.defaultHeaders }, this.middleware)
  }

  /**
   * Call a PostgreSQL function via PostgREST RPC.
   *
   * @typeParam T - Expected return type of the function.
   * @param fn     - Name of the PostgreSQL function.
   * @param params - Key-value arguments to pass to the function.
   * @returns The query result containing the function's return value.
   *
   * @example
   * ```ts
   * const { data } = await client.rpc('get_user_stats', { user_id: '42' })
   * ```
   */
  async rpc<T = unknown>(
    fn: string,
    params?: Record<string, unknown>,
  ): Promise<QueryResult<T>> {
    const url = `${this.baseUrl}/v1/rest/${this.ref}/rpc/${fn}`

    let init: RequestInit = {
      method: 'POST',
      headers: { ...this.defaultHeaders },
      body: JSON.stringify(params ?? {}),
    }

    // Apply request interceptor
    if (this.middleware.onRequest) {
      init = await this.middleware.onRequest(url, init)
    }

    try {
      let response: Response

      if (this.middleware.retry) {
        response = await fetchWithRetry(this.fetchFn, url, init, this.middleware.retry)
      } else {
        response = await this.fetchFn(url, init)
      }

      // Apply response interceptor
      if (this.middleware.onResponse) {
        response = await this.middleware.onResponse(response)
      }

      if (!response.ok) {
        const error = await MimDBError.fromResponse(response)
        return {
          data: null,
          error,
          count: null,
          status: response.status,
          statusText: response.statusText,
        }
      }

      let data: T | null = null
      const text = await response.text()
      if (text) {
        data = JSON.parse(text) as T
      }

      return {
        data,
        error: null,
        count: null,
        status: response.status,
        statusText: response.statusText,
      }
    } catch (err) {
      const error = err instanceof MimDBError
        ? err
        : new MimDBError(
            err instanceof Error ? err.message : 'Unknown error',
            'FETCH_ERROR',
            0,
          )

      return {
        data: null,
        error,
        count: null,
        status: 0,
        statusText: 'Fetch Error',
      }
    }
  }
}
