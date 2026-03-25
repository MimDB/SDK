/** Current state of the WebSocket connection. */
type ConnectionState = 'disconnected' | 'connecting' | 'connected' | 'reconnecting';
/** Database change event received from the server. */
interface RealtimeEvent<T = Record<string, unknown>> {
    /** The type of database operation. */
    type: 'INSERT' | 'UPDATE' | 'DELETE';
    /** The table that changed. */
    table: string;
    /** The new row state. Null on DELETE. */
    new: T | null;
    /**
     * Previous row state.
     * - INSERT: null
     * - UPDATE: null (server does not track pre-update state)
     * - DELETE: primary key columns only, all values stringified
     */
    old: Record<string, string> | null;
}
/** Error from the server for a specific subscription. */
interface RealtimeError {
    /**
     * Error code from the server (e.g., 'RT-0001').
     * Mapped from the wire format field `error_code`.
     */
    code: string;
    /** Human-readable error message. */
    message: string;
}
/** Current state of a subscription. */
type SubscriptionStatus = 'pending' | 'active' | 'error' | 'closed';
/** Options for subscribing to a table. */
interface SubscribeOptions<T = Record<string, unknown>> {
    /** Event filter. Default: '*' (all events). */
    event?: '*' | 'INSERT' | 'UPDATE' | 'DELETE';
    /**
     * Row filter expression. Format: `column=eq.value` (equality only).
     * Only the `eq` operator is supported. Values are compared as strings.
     * Example: `'id=eq.lobby_smp'`
     */
    filter?: string;
    /** Called for each matching database change event. */
    onEvent: (event: RealtimeEvent<T>) => void;
    /** Called when the server reports a subscription error. */
    onError?: (error: RealtimeError) => void;
    /** Called when the server confirms the subscription is active. */
    onSubscribed?: () => void;
}
/** Handle for an active subscription. */
interface SubscriptionHandle {
    /** Internal subscription ID (e.g., 'sub-1'). */
    readonly id: string;
    /** The subscribed table name. */
    readonly table: string;
    /** Current subscription state. */
    readonly status: SubscriptionStatus;
    /** Unsubscribe from this table. */
    unsubscribe(): void;
}
/** Connection event callbacks. */
type ConnectionEventMap = {
    connected: () => void;
    disconnected: (reason: string) => void;
    reconnecting: (attempt: number) => void;
    error: (err: Error) => void;
};
/** Constructor options for MimDBRealtimeClient. */
interface RealtimeClientOptions {
    /** Mimisbrunnr instance URL (HTTPS). Converted to WSS internally. */
    url: string;
    /** Project reference (8-char hex). */
    projectRef: string;
    /** API key JWT (anon or service_role). */
    apiKey: string;
    /** Connect on first subscribe() call. Default: true. */
    autoConnect?: boolean;
    /** Interval between heartbeat pings in ms. Default: 30000. */
    heartbeatInterval?: number;
    /** Time to wait for heartbeat_ack before triggering reconnect. Default: 10000. */
    heartbeatTimeout?: number;
    /** Maximum delay between reconnect attempts in ms. Default: 30000. */
    maxReconnectDelay?: number;
    /** Maximum consecutive reconnect attempts. Default: Infinity. */
    maxRetries?: number;
    /** Custom WebSocket constructor for Node.js (pass `ws` package). */
    WebSocket?: {
        new (url: string | URL): WebSocket;
    };
}

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
declare class MimDBRealtimeClient {
    private readonly url;
    private readonly projectRef;
    private apiKey;
    private readonly autoConnect;
    private readonly heartbeatInterval;
    private readonly heartbeatTimeout;
    private readonly maxReconnectDelay;
    private readonly maxRetries;
    private readonly WSConstructor;
    private ws;
    private _state;
    private intentionalClose;
    private retryCount;
    private subCounter;
    private readonly subscriptions;
    private readonly pendingMessages;
    private readonly listeners;
    private heartbeatTimer;
    private heartbeatAckTimer;
    private reconnectTimer;
    /**
     * @param options - Client configuration including URL, project ref, and API key.
     */
    constructor(options: RealtimeClientOptions);
    /** Current connection state. */
    get state(): ConnectionState;
    /**
     * Register a connection event listener.
     *
     * @param event - Event name to listen for.
     * @param cb - Callback invoked when the event fires.
     * @returns An unsubscribe function that removes this listener.
     */
    on<K extends keyof ConnectionEventMap>(event: K, cb: ConnectionEventMap[K]): () => void;
    /**
     * Emit a connection event to all registered listeners.
     *
     * @param event - Event name.
     * @param args - Arguments forwarded to the listener callbacks.
     */
    private emit;
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
    setToken(token: string): void;
    /**
     * Open the WebSocket connection.
     * No-op if already connecting or connected.
     */
    connect(): void;
    /**
     * Internal connection logic. Builds the WSS URL and attaches
     * WebSocket event handlers.
     */
    private doConnect;
    /**
     * Close the connection and stop all activity.
     * The client is reusable after disconnect - calling {@link connect}
     * or {@link subscribe} will establish a fresh connection.
     * Idempotent: safe to call multiple times.
     */
    disconnect(): void;
    /**
     * Subscribe to table changes. Automatically connects if
     * `autoConnect` is enabled and the client is disconnected.
     *
     * @param table - Database table name to subscribe to.
     * @param options - Event callbacks and optional filter/event settings.
     * @returns A handle for checking status and unsubscribing.
     */
    subscribe<T = Record<string, unknown>>(table: string, options: SubscribeOptions<T>): SubscriptionHandle;
    /**
     * Send a raw JSON string over the WebSocket if it is open.
     *
     * @param data - Serialised JSON message.
     */
    private send;
    /** Flush all messages that were queued while the socket was not open. */
    private flushPendingMessages;
    /** Re-send subscribe messages for all active subscriptions after reconnect. */
    private resubscribeAll;
    /**
     * Parse and route an incoming server message to the appropriate handler.
     *
     * @param data - Raw JSON string from the server.
     */
    private handleMessage;
    /** Start the heartbeat interval timer. */
    private startHeartbeat;
    /** Stop the heartbeat interval and any pending ack timeout. */
    private stopHeartbeat;
    /** Cancel a pending heartbeat ack timeout if one exists. */
    private clearHeartbeatAckTimer;
    /**
     * Schedule a reconnection attempt with exponential backoff.
     * Emits `reconnecting` before each attempt and `disconnected`
     * if max retries are exceeded.
     */
    private scheduleReconnect;
}

export { type ConnectionEventMap, type ConnectionState, MimDBRealtimeClient, type RealtimeClientOptions, type RealtimeError, type RealtimeEvent, type SubscribeOptions, type SubscriptionHandle, type SubscriptionStatus };
