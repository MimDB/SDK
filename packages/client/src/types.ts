import type { TokenStore } from './auth-store'
import type { MimDBError } from './errors'

/**
 * Result returned from executing a query against the MimDB API.
 *
 * @typeParam T - The expected shape of the returned data.
 */
export interface QueryResult<T> {
  /** The response data, or null if the query produced an error. */
  data: T | null
  /** A structured error, or null if the query succeeded. */
  error: MimDBError | null
  /** Row count from the `content-range` header when a count method is requested. */
  count: number | null
  /** HTTP status code of the response. */
  status: number
  /** HTTP status text of the response. */
  statusText: string
}

/**
 * Options for the `order()` query modifier.
 */
export interface OrderOptions {
  /** Sort ascending (true) or descending (false). Defaults to true. */
  ascending?: boolean
  /** Place nulls first (true) or last (false). */
  nullsFirst?: boolean
  /** Apply ordering to a foreign table (for resource embedding). */
  foreignTable?: string
}

/**
 * Strategy PostgREST uses to compute the total row count.
 *
 * - `exact`     - precise count via `COUNT(*)` (slower on large tables)
 * - `planned`   - estimated count from the query planner
 * - `estimated` - uses exact for small counts, planned for large
 */
export type CountMethod = 'exact' | 'planned' | 'estimated'

/**
 * Text search type for PostgREST full-text search operators.
 *
 * - `plain`   - `plfts` (plain to tsquery)
 * - `phrase`  - `phfts` (phrase to tsquery)
 * - `web`     - `wfts`  (websearch to tsquery)
 * - `fts`     - `fts`   (to_tsquery - default)
 */
export type TextSearchType = 'plain' | 'phrase' | 'web' | 'fts'

/**
 * Options for the `textSearch()` filter method.
 */
export interface TextSearchOptions {
  /** The type of text search to perform. Defaults to 'fts'. */
  type?: TextSearchType
  /** Text search configuration (e.g. 'english'). */
  config?: string
}

/**
 * Options accepted by `createClient` and `MimDBClient`.
 */
export interface ClientOptions {
  /** Custom fetch implementation. Defaults to `globalThis.fetch`. */
  fetch?: typeof fetch
  /** Additional headers sent with every request. */
  headers?: Record<string, string>
  /** Custom token store for auth session persistence. Defaults to `InMemoryTokenStore`. */
  tokenStore?: TokenStore
}

// ---------------------------------------------------------------------------
// Auth types
// ---------------------------------------------------------------------------

/**
 * A user record returned by the MimDB auth API.
 */
export interface User {
  /** Unique user identifier (UUID). */
  id: string
  /** User's email address. */
  email: string
  /** Whether the user's email has been confirmed. */
  email_confirmed: boolean
  /** Whether the user's phone has been confirmed. */
  phone_confirmed: boolean
  /** Application-level metadata (only writable by service_role). */
  app_metadata: Record<string, unknown>
  /** User-level metadata (writable by the user). */
  user_metadata: Record<string, unknown>
  /** ISO 8601 timestamp of when the user was created. */
  created_at: string
  /** ISO 8601 timestamp of when the user was last updated. */
  updated_at: string
}

/**
 * Token set returned after successful authentication.
 */
export interface Tokens {
  /** JWT access token for authenticating API requests. */
  access_token: string
  /** Opaque token used to obtain a new access token. */
  refresh_token: string
  /** Number of seconds until the access token expires. */
  expires_in: number
}

// ---------------------------------------------------------------------------
// Storage types
// ---------------------------------------------------------------------------

/**
 * A storage bucket managed by the MimDB Storage API.
 */
export interface Bucket {
  /** Unique bucket name. */
  name: string
  /** Whether the bucket allows unauthenticated read access. */
  public: boolean
  /** Maximum file size in bytes, or null for unlimited. */
  file_size_limit: number | null
  /** Allowed MIME types for uploads, or null for unrestricted. */
  allowed_mime_types: string[] | null
  /** ISO 8601 timestamp of when the bucket was created. */
  created_at: string
  /** ISO 8601 timestamp of when the bucket was last updated. */
  updated_at: string
}

/**
 * Options for file upload operations.
 */
export interface UploadOptions {
  /** MIME type of the file. Defaults to `application/octet-stream`. */
  contentType?: string
  /** When true, overwrites an existing file at the same path. */
  upsert?: boolean
}
