import type { TokenStore } from './auth-store'
import type { MimDBError } from './errors'

/**
 * Unified API response envelope returned by the Mimisbrunnr backend.
 *
 * Success responses populate `data` and set `error` to null.
 * Error responses set `data` to null and populate `error`.
 *
 * @typeParam T - The shape of the data payload.
 */
export interface ApiEnvelope<T> {
  /** The response payload, or null on error. */
  data: T
  /** Structured error object, or null on success. */
  error: { code: string; message: string; detail?: string } | null
  /** Response metadata. Always contains a request_id. */
  meta: { request_id: string; next_cursor?: string; has_more?: boolean }
}

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
  /** Unique bucket identifier (UUID). */
  id: string
  /** Project this bucket belongs to. */
  project_id: string
  /** Unique bucket name. */
  name: string
  /** Whether the bucket allows unauthenticated read access. */
  public: boolean
  /** Maximum file size in bytes. Absent when unlimited. */
  file_size_limit?: number
  /** Allowed MIME types for uploads. Absent when unrestricted. */
  allowed_types?: string[]
  /** ISO 8601 timestamp of when the bucket was created. */
  created_at: string
}

/**
 * A storage object returned by upload and list operations.
 */
export interface StorageObject {
  /** Unique object identifier (UUID). */
  id: string
  /** ID of the bucket this object belongs to. */
  bucket_id: string
  /** Object path within the bucket. */
  path: string
  /** File size in bytes. */
  size: number
  /** MIME type of the file content. */
  content_type: string
  /** Content hash for cache validation. */
  etag: string
  /** Object lifecycle status. */
  status: string
  /** ISO 8601 timestamp of when the object was created. */
  created_at: string
  /** ISO 8601 timestamp of when the object was last updated. */
  updated_at: string
  /** UUID of the user who owns this object, if applicable. */
  owner_id?: string
}

/**
 * Options for file upload operations.
 */
export interface UploadOptions {
  /** MIME type of the file. Defaults to `application/octet-stream`. */
  contentType?: string
}
