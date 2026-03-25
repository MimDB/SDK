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
}
