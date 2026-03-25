import { useEffect, useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import {
  MimDBRealtimeClient,
  type RealtimeEvent,
  type SubscriptionStatus,
} from '@mimdb/realtime'
import { useClient } from './context'

/**
 * Options for the {@link useRealtime} hook.
 *
 * @typeParam T - Expected row type for realtime events.
 */
export interface UseRealtimeOptions<T = Record<string, unknown>> {
  /** Event type filter. Defaults to `'*'` (all events). */
  event?: '*' | 'INSERT' | 'UPDATE' | 'DELETE'
  /** Row filter expression (e.g. `'user_id=eq.42'`). */
  filter?: string
  /** Called for each matching realtime event. */
  onEvent?: (event: RealtimeEvent<T>) => void
  /**
   * Whether to automatically invalidate the table's TanStack Query cache
   * when an event is received. Defaults to `true`.
   */
  invalidateQueries?: boolean
}

/**
 * Subscribe to realtime database changes for a table via WebSocket.
 *
 * Creates a `MimDBRealtimeClient` using the connection config from
 * `MimDBClient.getConfig()` and subscribes to the specified table.
 * On each event, the table's TanStack Query cache is invalidated
 * (unless opted out) so queries refetch automatically.
 *
 * The subscription is cleaned up when the component unmounts or when
 * the `table`, `event`, or `filter` options change.
 *
 * @typeParam T - Expected row type for realtime events.
 * @param table   - Database table to subscribe to.
 * @param options - Event filters and callbacks.
 * @returns An object containing the current subscription status.
 *
 * @example
 * ```tsx
 * const { status } = useRealtime<Message>('messages', {
 *   event: 'INSERT',
 *   onEvent: (e) => console.log('New message:', e.new),
 * })
 * ```
 */
export function useRealtime<T = Record<string, unknown>>(
  table: string,
  options?: UseRealtimeOptions<T>,
): { status: SubscriptionStatus } {
  const client = useClient()
  const queryClient = useQueryClient()
  const [status, setStatus] = useState<SubscriptionStatus>('pending')
  const realtimeRef = useRef<MimDBRealtimeClient | null>(null)

  useEffect(() => {
    // Skip WebSocket connections during SSR
    if (typeof window === 'undefined') return

    const config = client.getConfig()

    if (!realtimeRef.current) {
      realtimeRef.current = new MimDBRealtimeClient({
        url: config.url,
        projectRef: config.ref,
        apiKey: config.apiKey,
      })
    }

    const sub = realtimeRef.current.subscribe<T>(table, {
      event: options?.event ?? '*',
      filter: options?.filter,
      onEvent(event) {
        options?.onEvent?.(event)
        if (options?.invalidateQueries !== false) {
          queryClient.invalidateQueries({ queryKey: ['mimdb', table] })
        }
      },
      onSubscribed() {
        setStatus('active')
      },
      onError() {
        setStatus('error')
      },
    })

    setStatus('pending')

    return () => {
      sub.unsubscribe()
      setStatus('closed')
    }
  }, [table, options?.event, options?.filter]) // eslint-disable-line react-hooks/exhaustive-deps

  return { status }
}
