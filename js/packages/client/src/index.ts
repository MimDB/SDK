import { MimDBClient } from './client'
import { InMemoryTokenStore } from './auth-store'
import type { ClientOptions } from './types'

export { MimDBClient } from './client'
export { MimDBError } from './errors'
export { QueryBuilder } from './rest'
export type { QueryMiddleware } from './rest'
export { FilterBuilder } from './filters'
export { AuthClient } from './auth'
export type { AuthChangeEvent } from './auth'
export { AuthAdminClient } from './auth-admin'
export { InMemoryTokenStore, LocalStorageTokenStore } from './auth-store'
export type { TokenStore } from './auth-store'
export { StorageClient, BucketClient } from './storage'
export type { BucketOptions } from './storage'
export { fetchWithRetry } from './retry'
export type {
  ApiEnvelope,
  QueryResult,
  OrderOptions,
  CountMethod,
  TextSearchType,
  TextSearchOptions,
  ClientOptions,
  RetryOptions,
  RequestInterceptor,
  ResponseInterceptor,
  User,
  Tokens,
  Bucket,
  StorageObject,
  UploadOptions,
} from './types'

// Re-export realtime types for convenience
export { MimDBRealtimeClient } from '@mimdb/realtime'
export type {
  RealtimeEvent,
  RealtimeError,
  SubscribeOptions,
  SubscriptionHandle,
  SubscriptionStatus,
  ConnectionState,
  ConnectionEventMap,
  RealtimeClientOptions,
} from '@mimdb/realtime'

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

/**
 * Create a MimDB client suitable for server-side data fetching.
 *
 * Uses an in-memory token store (no localStorage) and disables
 * auto-refresh since server-side code typically uses a service_role key
 * that does not expire.
 *
 * @param url        - Base URL of the MimDB API.
 * @param projectRef - Short project reference ID.
 * @param serviceKey - Service-role API key for privileged access.
 * @param options    - Optional additional client configuration.
 * @returns A configured `MimDBClient` for server-side use.
 *
 * @example
 * ```ts
 * // In a Next.js API route or server component
 * import { createServerClient } from '@mimdb/client'
 *
 * const mimdb = createServerClient('https://api.mimdb.dev', 'ref', 'service-key')
 * const { data } = await mimdb.from('users').select('*')
 * ```
 */
export function createServerClient(
  url: string,
  projectRef: string,
  serviceKey: string,
  options?: Omit<ClientOptions, 'tokenStore' | 'autoRefresh'>,
): MimDBClient {
  return new MimDBClient(url, projectRef, serviceKey, {
    ...options,
    tokenStore: new InMemoryTokenStore(),
    autoRefresh: false,
  })
}
