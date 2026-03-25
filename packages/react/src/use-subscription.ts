import { useState, useEffect } from 'react'
import type { RealtimeEvent, SubscriptionStatus } from '@mimdb/realtime'
import { useClient } from './context'

/**
 * Return type of the {@link useSubscription} hook.
 *
 * @typeParam T - Expected row type for realtime events.
 */
export interface UseSubscriptionResult<T> {
  /** The most recently received realtime event, or null if none yet. */
  lastEvent: RealtimeEvent<T> | null
  /** Current subscription lifecycle status. */
  status: SubscriptionStatus
}

/**
 * Subscribe to a table and maintain the latest event in React state.
 *
 * Unlike {@link useRealtime} (which fires callbacks), this hook returns
 * the most recent event directly as component state, making it ideal
 * for rendering the latest change inline.
 *
 * Uses `client.realtime` (the lazy realtime accessor on MimDBClient)
 * so no separate realtime client setup is needed.
 *
 * The subscription is cleaned up when the component unmounts or when
 * `table`, `event`, or `filter` change.
 *
 * @typeParam T - Expected row type for realtime events.
 * @param table   - Database table to subscribe to.
 * @param options - Event type and filter configuration.
 * @returns The latest event and subscription status.
 *
 * @example
 * ```tsx
 * const { lastEvent, status } = useSubscription<Message>('messages', {
 *   event: 'INSERT',
 * })
 *
 * if (lastEvent) {
 *   console.log('Latest insert:', lastEvent.new)
 * }
 * ```
 */
export function useSubscription<T = Record<string, unknown>>(
  table: string,
  options?: {
    /** Event type filter. Defaults to `'*'` (all events). */
    event?: '*' | 'INSERT' | 'UPDATE' | 'DELETE'
    /** Row filter expression (e.g. `'user_id=eq.42'`). */
    filter?: string
  },
): UseSubscriptionResult<T> {
  const client = useClient()
  const [lastEvent, setLastEvent] = useState<RealtimeEvent<T> | null>(null)
  const [status, setStatus] = useState<SubscriptionStatus>('pending')

  useEffect(() => {
    // Skip WebSocket connections during SSR
    if (typeof window === 'undefined') return

    const sub = client.realtime.subscribe<T>(table, {
      event: options?.event ?? '*',
      filter: options?.filter,
      onEvent(event) {
        setLastEvent(event)
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

  return { lastEvent, status }
}
