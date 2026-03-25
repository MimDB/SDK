import { MimDBClient } from './client'
import type { ClientOptions } from './types'

export { MimDBClient } from './client'
export { MimDBError } from './errors'
export { QueryBuilder } from './rest'
export { FilterBuilder } from './filters'
export { AuthClient } from './auth'
export type { AuthChangeEvent } from './auth'
export { AuthAdminClient } from './auth-admin'
export { InMemoryTokenStore, LocalStorageTokenStore } from './auth-store'
export type { TokenStore } from './auth-store'
export { StorageClient, BucketClient } from './storage'
export type { BucketOptions } from './storage'
export type {
  QueryResult,
  OrderOptions,
  CountMethod,
  TextSearchType,
  TextSearchOptions,
  ClientOptions,
  User,
  Tokens,
  Bucket,
  UploadOptions,
} from './types'

/**
 * Create a new MimDB client.
 *
 * This is the primary entry point for the SDK. It returns a configured
 * `MimDBClient` ready to query tables, call RPCs, and more.
 *
 * @param url        - Base URL of the MimDB API (e.g. `https://api.mimdb.dev`).
 * @param projectRef - Short project reference ID.
 * @param apiKey     - API key for authentication.
 * @param options    - Optional client configuration (custom fetch, headers).
 * @returns A configured `MimDBClient` instance.
 *
 * @example
 * ```ts
 * import { createClient } from '@mimdb/client'
 *
 * const mimdb = createClient('https://api.mimdb.dev', '40891b0d', 'eyJ...')
 * const { data } = await mimdb.from('todos').select('*')
 * ```
 */
export function createClient(
  url: string,
  projectRef: string,
  apiKey: string,
  options?: ClientOptions,
): MimDBClient {
  return new MimDBClient(url, projectRef, apiKey, options)
}
