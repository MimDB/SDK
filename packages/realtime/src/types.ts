/** Current state of the WebSocket connection. */
export type ConnectionState = 'disconnected' | 'connecting' | 'connected' | 'reconnecting'

/** Database change event received from the server. */
export interface RealtimeEvent<T = Record<string, unknown>> {
  /** The type of database operation. */
  type: 'INSERT' | 'UPDATE' | 'DELETE'
  /** The table that changed. */
  table: string
  /** The new row state. Null on DELETE. */
  new: T | null
  /**
   * Previous row state.
   * - INSERT: null
   * - UPDATE: null (server does not track pre-update state)
   * - DELETE: primary key columns only, all values stringified
   */
  old: Record<string, string> | null
}

/** Error from the server for a specific subscription. */
export interface RealtimeError {
  /**
   * Error code from the server (e.g., 'RT-0001').
   * Mapped from the wire format field `error_code`.
   */
  code: string
  /** Human-readable error message. */
  message: string
}

/** Current state of a subscription. */
export type SubscriptionStatus = 'pending' | 'active' | 'error' | 'closed'

/** Options for subscribing to a table. */
export interface SubscribeOptions<T = Record<string, unknown>> {
  /** Event filter. Default: '*' (all events). */
  event?: '*' | 'INSERT' | 'UPDATE' | 'DELETE'
  /**
   * Row filter expression. Format: `column=eq.value` (equality only).
   * Only the `eq` operator is supported. Values are compared as strings.
   * Example: `'id=eq.lobby_smp'`
   */
  filter?: string
  /** Called for each matching database change event. */
  onEvent: (event: RealtimeEvent<T>) => void
  /** Called when the server reports a subscription error. */
  onError?: (error: RealtimeError) => void
  /** Called when the server confirms the subscription is active. */
  onSubscribed?: () => void
}

/** Handle for an active subscription. */
export interface SubscriptionHandle {
  /** Internal subscription ID (e.g., 'sub-1'). */
  readonly id: string
  /** The subscribed table name. */
  readonly table: string
  /** Current subscription state. */
  readonly status: SubscriptionStatus
  /** Unsubscribe from this table. */
  unsubscribe(): void
}

/** Connection event callbacks. */
export type ConnectionEventMap = {
  connected: () => void
  disconnected: (reason: string) => void
  reconnecting: (attempt: number) => void
  error: (err: Error) => void
}

/** Constructor options for MimDBRealtimeClient. */
export interface RealtimeClientOptions {
  /** Mimisbrunnr instance URL (HTTPS). Converted to WSS internally. */
  url: string
  /** Project reference (8-char hex). */
  projectRef: string
  /** API key JWT (anon or service_role). */
  apiKey: string
  /** Connect on first subscribe() call. Default: true. */
  autoConnect?: boolean
  /** Interval between heartbeat pings in ms. Default: 30000. */
  heartbeatInterval?: number
  /** Time to wait for heartbeat_ack before triggering reconnect. Default: 10000. */
  heartbeatTimeout?: number
  /** Maximum delay between reconnect attempts in ms. Default: 30000. */
  maxReconnectDelay?: number
  /** Maximum consecutive reconnect attempts. Default: Infinity. */
  maxRetries?: number
  /** Custom WebSocket constructor for Node.js (pass `ws` package). */
  WebSocket?: { new (url: string | URL): WebSocket }
}

// --- Wire protocol types (internal, not exported from index.ts) ---

/** Client-to-server message types. */
export type ClientMessageType = 'subscribe' | 'unsubscribe' | 'heartbeat'

/** Server-to-client message types. */
export type ServerMessageType = 'subscribed' | 'unsubscribed' | 'event' | 'heartbeat_ack' | 'error'

/** Subscribe message sent to the server. */
export interface SubscribeMessage {
  type: 'subscribe'
  id: string
  table: string
  event: string
  filter?: string
}

/** Unsubscribe message sent to the server. */
export interface UnsubscribeMessage {
  type: 'unsubscribe'
  id: string
}

/** Server event message. */
export interface ServerEventMessage {
  type: 'event'
  id: string
  event: 'INSERT' | 'UPDATE' | 'DELETE'
  table: string
  new: Record<string, unknown> | null
  old: Record<string, string> | null
}

/** Server error message. */
export interface ServerErrorMessage {
  type: 'error'
  error_code: string
  message: string
  id?: string
}
