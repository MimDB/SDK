import { Subscription } from './subscription'
import type {
  RealtimeClientOptions,
  ConnectionState,
  ConnectionEventMap,
  SubscribeOptions,
  SubscriptionHandle,
  ServerEventMessage,
  ServerErrorMessage,
} from './types'

const DEFAULT_HEARTBEAT_INTERVAL = 30_000
const DEFAULT_HEARTBEAT_TIMEOUT = 10_000
const DEFAULT_MAX_RECONNECT_DELAY = 30_000
const INITIAL_RECONNECT_DELAY = 1_000

/**
 * WebSocket client for Mimisbrunnr realtime table change subscriptions.
 *
 * Manages connection lifecycle, heartbeat keep-alive, reconnection with
 * exponential backoff, and routes server messages to individual
 * {@link Subscription} instances.
 *
 * @example
 * ```ts
 * const client = new MimDBRealtimeClient({
 *   url: 'https://api.mimdb.dev',
 *   projectRef: '40891b0d',
 *   apiKey: 'my-api-key',
 * })
 *
 * client.on('connected', () => console.log('Connected'))
 *
 * const handle = client.subscribe('users', {
 *   event: 'INSERT',
 *   onEvent: (e) => console.log('New user:', e.new),
 * })
 *
 * // Later: handle.unsubscribe() and client.disconnect()
 * ```
 */
export class MimDBRealtimeClient {
  private readonly url: string
  private readonly projectRef: string
  private readonly anonKey: string
  private apiKey: string
  private readonly autoConnect: boolean
  private readonly heartbeatInterval: number
  private readonly heartbeatTimeout: number
  private readonly maxReconnectDelay: number
  private readonly maxRetries: number
  private readonly WSConstructor: { new (url: string | URL): WebSocket }

  private ws: WebSocket | null = null
  private _state: ConnectionState = 'disconnected'
  private intentionalClose = false
  private retryCount = 0
  private subCounter = 0

  private readonly subscriptions = new Map<string, Subscription>()
  private readonly pendingMessages: string[] = []
  private readonly listeners = new Map<keyof ConnectionEventMap, Set<Function>>()

  private heartbeatTimer: ReturnType<typeof setInterval> | null = null
  private heartbeatAckTimer: ReturnType<typeof setTimeout> | null = null
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null

  /**
   * @param options - Client configuration including URL, project ref, and API key.
   */
  constructor(options: RealtimeClientOptions) {
    this.url = options.url
    this.projectRef = options.projectRef
    this.anonKey = options.apiKey
    this.apiKey = options.apiKey
    this.autoConnect = options.autoConnect ?? true
    this.heartbeatInterval = options.heartbeatInterval ?? DEFAULT_HEARTBEAT_INTERVAL
    this.heartbeatTimeout = options.heartbeatTimeout ?? DEFAULT_HEARTBEAT_TIMEOUT
    this.maxReconnectDelay = options.maxReconnectDelay ?? DEFAULT_MAX_RECONNECT_DELAY
    this.maxRetries = options.maxRetries ?? Infinity
    this.WSConstructor = options.WebSocket ?? globalThis.WebSocket

    // Reconnect immediately when a background tab becomes visible.
    // Browsers throttle setTimeout in background tabs, which kills
    // the heartbeat ping and causes the server to drop the connection.
    // This listener bypasses the throttled reconnect timer.
    if (typeof document !== 'undefined') {
      document.addEventListener('visibilitychange', () => {
        if (document.visibilityState === 'visible' && this._state !== 'connected' && this._state !== 'connecting') {
          if (!this.intentionalClose && this.subscriptions.size > 0) {
            this.retryCount = 0
            this.doConnect()
          }
        }
      })
    }
  }

  /** Current connection state. */
  get state(): ConnectionState {
    return this._state
  }

  /**
   * Register a connection event listener.
   *
   * @param event - Event name to listen for.
   * @param cb - Callback invoked when the event fires.
   * @returns An unsubscribe function that removes this listener.
   */
  on<K extends keyof ConnectionEventMap>(event: K, cb: ConnectionEventMap[K]): () => void {
    if (!this.listeners.has(event)) this.listeners.set(event, new Set())
    this.listeners.get(event)!.add(cb)
    return () => {
      this.listeners.get(event)?.delete(cb)
    }
  }

  /**
   * Emit a connection event to all registered listeners.
   *
   * @param event - Event name.
   * @param args - Arguments forwarded to the listener callbacks.
   */
  private emit<K extends keyof ConnectionEventMap>(
    event: K,
    ...args: Parameters<ConnectionEventMap[K]>
  ) {
    for (const cb of this.listeners.get(event) ?? []) {
      ;(cb as Function)(...args)
    }
  }

  /**
   * Replace the API key (JWT) used for authentication.
   *
   * If the client is currently connected, the WebSocket is closed and
   * a fresh connection is opened with the new token. All active
   * subscriptions are automatically re-established on reconnect.
   *
   * Pass an empty string to clear the token without reconnecting
   * (used on logout).
   *
   * @param token - New API key / JWT to authenticate with.
   */
  setToken(token: string): void {
    this.apiKey = token

    // Empty token means logout - disconnect without reconnect
    if (!token) {
      this.disconnect()
      return
    }

    // If connected, reconnect with the new token
    if (this._state === 'connected' || this._state === 'connecting' || this._state === 'reconnecting') {
      // Close current connection without triggering auto-reconnect
      this.stopHeartbeat()
      if (this.reconnectTimer) {
        clearTimeout(this.reconnectTimer)
        this.reconnectTimer = null
      }
      if (this.ws) {
        this.ws.onclose = null
        this.ws.close()
        this.ws = null
      }
      // Reconnect with the new token
      this.doConnect()
    }
  }

  /**
   * Open the WebSocket connection.
   * No-op if already connecting or connected.
   */
  connect(): void {
    if (this._state === 'connecting' || this._state === 'connected' || this._state === 'reconnecting') return
    this.intentionalClose = false
    this.doConnect()
  }

  /**
   * Internal connection logic. Builds the WSS URL and attaches
   * WebSocket event handlers.
   */
  private doConnect(): void {
    const protocol = this.url.startsWith('https') ? 'wss' : 'ws'
    const host = this.url.replace(/^https?:\/\//, '')
    // Use the anon key for project identification in the URL. The user's
    // access token (set via setToken) is sent separately so the server can
    // verify it as a JWT. Browser WebSocket API cannot set custom headers,
    // so both values go in query params.
    const params = new URLSearchParams({ apikey: this.anonKey })
    if (this.apiKey !== this.anonKey) {
      params.set('token', this.apiKey)
    }
    const wsUrl = `${protocol}://${host}/v1/realtime/${this.projectRef}?${params}`

    this._state = 'connecting'
    this.ws = new this.WSConstructor(wsUrl)

    this.ws.onopen = () => {
      this._state = 'connected'
      this.retryCount = 0
      this.startHeartbeat()
      this.flushPendingMessages()
      this.resubscribeAll()
      this.emit('connected')
    }

    this.ws.onclose = () => {
      this.stopHeartbeat()
      if (this.intentionalClose) {
        this._state = 'disconnected'
        this.emit('disconnected', 'intentional')
        return
      }
      this.scheduleReconnect()
    }

    this.ws.onerror = () => {
      this.emit('error', new Error('WebSocket error'))
    }

    this.ws.onmessage = (ev) => {
      this.clearHeartbeatAckTimer()
      this.handleMessage(typeof ev.data === 'string' ? ev.data : String(ev.data))
    }
  }

  /**
   * Close the connection and stop all activity.
   * The client is reusable after disconnect - calling {@link connect}
   * or {@link subscribe} will establish a fresh connection.
   * Idempotent: safe to call multiple times.
   */
  disconnect(): void {
    this.intentionalClose = true
    this.stopHeartbeat()
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    for (const sub of this.subscriptions.values()) {
      if (sub.status !== 'closed') {
        ;(sub as unknown as { _status: string })._status = 'closed'
      }
    }
    this.subscriptions.clear()
    this.emit('disconnected', 'intentional')
    if (this.ws) {
      this.ws.onclose = null
      this.ws.close()
      this.ws = null
    }
    this._state = 'disconnected'
  }

  /**
   * Subscribe to table changes. Automatically connects if
   * `autoConnect` is enabled and the client is disconnected.
   *
   * @param table - Database table name to subscribe to.
   * @param options - Event callbacks and optional filter/event settings.
   * @returns A handle for checking status and unsubscribing.
   */
  subscribe<T = Record<string, unknown>>(
    table: string,
    options: SubscribeOptions<T>,
  ): SubscriptionHandle {
    const id = `sub-${++this.subCounter}`
    const sub = new Subscription<T>(id, table, options, (data) => this.send(data))
    this.subscriptions.set(id, sub as Subscription)

    if (this.autoConnect && this._state === 'disconnected') {
      this.connect()
    }

    if (this._state === 'connected') {
      this.send(JSON.stringify(sub.buildSubscribeMessage()))
    }
    // When not yet connected, resubscribeAll() on open handles sending.

    return sub
  }

  /**
   * Send a raw JSON string over the WebSocket if it is open.
   *
   * @param data - Serialised JSON message.
   */
  private send(data: string): void {
    if (this.ws?.readyState === 1) {
      this.ws.send(data)
    }
  }

  /** Flush all messages that were queued while the socket was not open. */
  private flushPendingMessages(): void {
    while (this.pendingMessages.length > 0) {
      this.send(this.pendingMessages.shift()!)
    }
  }

  /** Re-send subscribe messages for all active subscriptions after reconnect. */
  private resubscribeAll(): void {
    for (const sub of this.subscriptions.values()) {
      if (sub.status === 'closed' || sub.status === 'error') continue
      sub.resetForReconnect()
      this.send(JSON.stringify(sub.buildSubscribeMessage()))
    }
  }

  /**
   * Parse and route an incoming server message to the appropriate handler.
   *
   * @param data - Raw JSON string from the server.
   */
  private handleMessage(data: string): void {
    let msg: Record<string, unknown>
    try {
      msg = JSON.parse(data)
    } catch {
      return
    }

    switch (msg.type) {
      case 'subscribed': {
        const sub = this.subscriptions.get(msg.id as string)
        sub?.handleSubscribed()
        break
      }
      case 'unsubscribed': {
        this.subscriptions.delete(msg.id as string)
        break
      }
      case 'event': {
        const sub = this.subscriptions.get(msg.id as string)
        sub?.handleEvent(msg as unknown as ServerEventMessage)
        break
      }
      case 'error': {
        const errMsg = msg as unknown as ServerErrorMessage
        if (errMsg.id) {
          const sub = this.subscriptions.get(errMsg.id)
          sub?.handleError(errMsg)
        } else {
          this.emit('error', new Error(`[${errMsg.error_code}] ${errMsg.message}`))
        }
        break
      }
      case 'heartbeat_ack':
        break
    }
  }

  /** Start the heartbeat interval timer. */
  private startHeartbeat(): void {
    this.heartbeatTimer = setInterval(() => {
      this.clearHeartbeatAckTimer()
      this.send(JSON.stringify({ type: 'heartbeat' }))
      this.heartbeatAckTimer = setTimeout(() => {
        this.ws?.close()
      }, this.heartbeatTimeout)
    }, this.heartbeatInterval)
  }

  /** Stop the heartbeat interval and any pending ack timeout. */
  private stopHeartbeat(): void {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer)
      this.heartbeatTimer = null
    }
    this.clearHeartbeatAckTimer()
  }

  /** Cancel a pending heartbeat ack timeout if one exists. */
  private clearHeartbeatAckTimer(): void {
    if (this.heartbeatAckTimer) {
      clearTimeout(this.heartbeatAckTimer)
      this.heartbeatAckTimer = null
    }
  }

  /**
   * Schedule a reconnection attempt with exponential backoff.
   * Emits `reconnecting` before each attempt and `disconnected`
   * if max retries are exceeded.
   */
  private scheduleReconnect(): void {
    this.retryCount++
    if (this.retryCount > this.maxRetries) {
      this._state = 'disconnected'
      this.emit('disconnected', 'max retries exceeded')
      return
    }
    this._state = 'reconnecting'
    this.emit('reconnecting', this.retryCount)
    const delay = Math.min(
      INITIAL_RECONNECT_DELAY * Math.pow(2, this.retryCount - 1),
      this.maxReconnectDelay,
    )
    this.reconnectTimer = setTimeout(() => {
      this.doConnect()
    }, delay)
  }
}
