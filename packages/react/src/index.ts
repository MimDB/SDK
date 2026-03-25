export { MimDBProvider, type MimDBProviderProps } from './provider'
export { useClient } from './context'
export { useQuery, type UseQueryOptions } from './use-query'
export {
  useInsert,
  useUpdate,
  useDelete,
  type UpdateInput,
  type UseInsertOptions,
  type UseUpdateOptions,
  type UseDeleteOptions,
} from './use-mutation'
export { useRealtime, type UseRealtimeOptions } from './use-realtime'
export { useSubscription, type UseSubscriptionResult } from './use-subscription'
export { useAuth, type UseAuthResult } from './use-auth'
export { useUpload, type UseUploadResult } from './use-upload'

// Re-export createServerClient for convenience
export { createServerClient } from '@mimdb/client'
