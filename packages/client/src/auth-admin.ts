import { MimDBError } from './errors'
import type { User } from './types'

/**
 * Admin client for managing users with elevated (service_role) privileges.
 *
 * All requests use the service_role API key in the Authorization header,
 * bypassing row-level security. This client should only be used in
 * trusted server-side environments.
 *
 * @example
 * ```ts
 * // Accessed via the main auth client
 * const users = await mimdb.auth.admin.listUsers({ limit: 50 })
 * ```
 */
export class AuthAdminClient {
  private readonly baseUrl: string
  private readonly ref: string
  private readonly fetchFn: typeof fetch
  private readonly headers: Record<string, string>

  /**
   * @param baseUrl - Base URL of the MimDB API.
   * @param ref     - Short project reference ID.
   * @param fetchFn - Fetch implementation.
   * @param headers - Default headers including service_role Authorization.
   */
  constructor(
    baseUrl: string,
    ref: string,
    fetchFn: typeof fetch,
    headers: Record<string, string>,
  ) {
    this.baseUrl = baseUrl
    this.ref = ref
    this.fetchFn = fetchFn
    this.headers = headers
  }

  /**
   * List users with optional pagination.
   *
   * @param opts - Pagination options.
   * @param opts.limit  - Maximum number of users to return.
   * @param opts.offset - Number of users to skip.
   * @returns Array of user records.
   * @throws {MimDBError} If the API returns an error response.
   */
  async listUsers(opts?: { limit?: number; offset?: number }): Promise<User[]> {
    const params = new URLSearchParams()
    if (opts?.limit !== undefined) params.set('limit', String(opts.limit))
    if (opts?.offset !== undefined) params.set('offset', String(opts.offset))

    const query = params.toString()
    const url = `${this.baseUrl}/v1/auth/${this.ref}/users${query ? `?${query}` : ''}`

    const response = await this.fetchFn(url, {
      method: 'GET',
      headers: { ...this.headers },
    })

    if (!response.ok) {
      throw await MimDBError.fromResponse(response)
    }

    return (await response.json()) as User[]
  }

  /**
   * Look up a user by their email address.
   *
   * @param email - Email address to search for.
   * @returns The matching user, or null if no user was found.
   * @throws {MimDBError} If the API returns an error response other than empty results.
   */
  async getUserByEmail(email: string): Promise<User | null> {
    const params = new URLSearchParams({ email })
    const url = `${this.baseUrl}/v1/auth/${this.ref}/users?${params.toString()}`

    const response = await this.fetchFn(url, {
      method: 'GET',
      headers: { ...this.headers },
    })

    if (!response.ok) {
      throw await MimDBError.fromResponse(response)
    }

    const users = (await response.json()) as User[]
    return users[0] ?? null
  }

  /**
   * Update a user by their ID with admin-level metadata.
   *
   * @param id   - UUID of the user to update.
   * @param data - Fields to update.
   * @param data.appMetadata  - Application-level metadata (only settable by admins).
   * @param data.userMetadata - User-level metadata.
   * @returns The updated user record.
   * @throws {MimDBError} If the API returns an error response.
   */
  async updateUserById(
    id: string,
    data: { appMetadata?: Record<string, unknown>; userMetadata?: Record<string, unknown> },
  ): Promise<User> {
    const url = `${this.baseUrl}/v1/auth/${this.ref}/users/${id}`

    const body: Record<string, unknown> = {}
    if (data.appMetadata !== undefined) body.app_metadata = data.appMetadata
    if (data.userMetadata !== undefined) body.user_metadata = data.userMetadata

    const response = await this.fetchFn(url, {
      method: 'PATCH',
      headers: { ...this.headers },
      body: JSON.stringify(body),
    })

    if (!response.ok) {
      throw await MimDBError.fromResponse(response)
    }

    return (await response.json()) as User
  }
}
