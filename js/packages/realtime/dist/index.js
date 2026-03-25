// src/subscription.ts
var Subscription = class {
  /** Internal subscription ID matching the server-side registration. */
  id;
  /** The subscribed table name. */
  table;
  _status = "pending";
  options;
  sendFn;
  /**
   * @param id - Unique subscription identifier (e.g. `'sub-1'`).
   * @param table - Database table to subscribe to.
   * @param options - Event callbacks and optional filter/event settings.
   * @param sendFn - Function used to write raw JSON strings to the transport.
   */
  constructor(id, table, options, sendFn) {
    this.id = id;
    this.table = table;
    this.options = options;
    this.sendFn = sendFn;
  }
  /**
   * Current state of this subscription.
   *
   * State machine:
   * - `pending`  - created, subscribe message not yet acknowledged
   * - `active`   - server confirmed subscription
   * - `error`    - server returned an error; no further events will arrive
   * - `closed`   - unsubscribed by the client
   */
  get status() {
    return this._status;
  }
  /**
   * Build the subscribe message to send to the server.
   * Defaults `event` to `'*'` when not specified in options.
   *
   * @returns A `SubscribeMessage` ready to be JSON-serialised and sent.
   */
  buildSubscribeMessage() {
    const msg = {
      type: "subscribe",
      id: this.id,
      table: this.table,
      event: this.options.event ?? "*"
    };
    if (this.options.filter) {
      msg.filter = this.options.filter;
    }
    return msg;
  }
  /**
   * Handle server confirmation that this subscription is now active.
   * Transitions status to `active` and invokes `onSubscribed` if provided.
   */
  handleSubscribed() {
    this._status = "active";
    this.options.onSubscribed?.();
  }
  /**
   * Handle an incoming event message from the server.
   * No-ops if the subscription is `closed` (guards against late delivery).
   *
   * @param msg - The raw server event message.
   */
  handleEvent(msg) {
    if (this._status === "closed") return;
    const event = {
      type: msg.event,
      table: msg.table,
      new: msg.new,
      old: msg.old
    };
    this.options.onEvent(event);
  }
  /**
   * Handle a server error targeting this subscription.
   * Transitions status to `error` and invokes `onError` if provided.
   *
   * @param msg - The raw server error message.
   */
  handleError(msg) {
    this._status = "error";
    this.options.onError?.({ code: msg.error_code, message: msg.message });
  }
  /**
   * Unsubscribe from the server and mark this subscription as closed.
   * Sends an `unsubscribe` message over the transport.
   * Idempotent: subsequent calls after the first are no-ops.
   */
  unsubscribe() {
    if (this._status === "closed") return;
    this._status = "closed";
    this.sendFn(JSON.stringify({ type: "unsubscribe", id: this.id }));
  }
  /**
   * Reset status to `pending` to allow resubscription after a reconnect.
   * No-ops if the subscription has been explicitly closed by the client.
   */
  resetForReconnect() {
    if (this._status === "closed") return;
    this._status = "pending";
  }
};

// src/client.ts
var DEFAULT_HEARTBEAT_INTERVAL = 3e4;
var DEFAULT_HEARTBEAT_TIMEOUT = 1e4;
var DEFAULT_MAX_RECONNECT_DELAY = 3e4;
var INITIAL_RECONNECT_DELAY = 1e3;
var MimDBRealtimeClient = class {
  url;
  projectRef;
  apiKey;
  autoConnect;
  heartbeatInterval;
  heartbeatTimeout;
  maxReconnectDelay;
  maxRetries;
  WSConstructor;
  ws = null;
  _state = "disconnected";
  intentionalClose = false;
  retryCount = 0;
  subCounter = 0;
  subscriptions = /* @__PURE__ */ new Map();
  pendingMessages = [];
  listeners = /* @__PURE__ */ new Map();
  heartbeatTimer = null;
  heartbeatAckTimer = null;
  reconnectTimer = null;
  /**
   * @param options - Client configuration including URL, project ref, and API key.
   */
  constructor(options) {
    this.url = options.url;
    this.projectRef = options.projectRef;
    this.apiKey = options.apiKey;
    this.autoConnect = options.autoConnect ?? true;
    this.heartbeatInterval = options.heartbeatInterval ?? DEFAULT_HEARTBEAT_INTERVAL;
    this.heartbeatTimeout = options.heartbeatTimeout ?? DEFAULT_HEARTBEAT_TIMEOUT;
    this.maxReconnectDelay = options.maxReconnectDelay ?? DEFAULT_MAX_RECONNECT_DELAY;
    this.maxRetries = options.maxRetries ?? Infinity;
    this.WSConstructor = options.WebSocket ?? globalThis.WebSocket;
  }
  /** Current connection state. */
  get state() {
    return this._state;
  }
  /**
   * Register a connection event listener.
   *
   * @param event - Event name to listen for.
   * @param cb - Callback invoked when the event fires.
   * @returns An unsubscribe function that removes this listener.
   */
  on(event, cb) {
    if (!this.listeners.has(event)) this.listeners.set(event, /* @__PURE__ */ new Set());
    this.listeners.get(event).add(cb);
    return () => {
      this.listeners.get(event)?.delete(cb);
    };
  }
  /**
   * Emit a connection event to all registered listeners.
   *
   * @param event - Event name.
   * @param args - Arguments forwarded to the listener callbacks.
   */
  emit(event, ...args) {
    for (const cb of this.listeners.get(event) ?? []) {
      ;
      cb(...args);
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
  setToken(token) {
    this.apiKey = token;
    if (!token) {
      this.disconnect();
      return;
    }
    if (this._state === "connected" || this._state === "connecting" || this._state === "reconnecting") {
      this.stopHeartbeat();
      if (this.reconnectTimer) {
        clearTimeout(this.reconnectTimer);
        this.reconnectTimer = null;
      }
      if (this.ws) {
        this.ws.onclose = null;
        this.ws.close();
        this.ws = null;
      }
      this.doConnect();
    }
  }
  /**
   * Open the WebSocket connection.
   * No-op if already connecting or connected.
   */
  connect() {
    if (this._state === "connecting" || this._state === "connected" || this._state === "reconnecting") return;
    this.intentionalClose = false;
    this.doConnect();
  }
  /**
   * Internal connection logic. Builds the WSS URL and attaches
   * WebSocket event handlers.
   */
  doConnect() {
    const protocol = this.url.startsWith("https") ? "wss" : "ws";
    const host = this.url.replace(/^https?:\/\//, "");
    const wsUrl = `${protocol}://${host}/v1/realtime/${this.projectRef}?apikey=${this.apiKey}`;
    this._state = "connecting";
    this.ws = new this.WSConstructor(wsUrl);
    this.ws.onopen = () => {
      this._state = "connected";
      this.retryCount = 0;
      this.startHeartbeat();
      this.flushPendingMessages();
      this.resubscribeAll();
      this.emit("connected");
    };
    this.ws.onclose = () => {
      this.stopHeartbeat();
      if (this.intentionalClose) {
        this._state = "disconnected";
        this.emit("disconnected", "intentional");
        return;
      }
      this.scheduleReconnect();
    };
    this.ws.onerror = () => {
      this.emit("error", new Error("WebSocket error"));
    };
    this.ws.onmessage = (ev) => {
      this.clearHeartbeatAckTimer();
      this.handleMessage(typeof ev.data === "string" ? ev.data : String(ev.data));
    };
  }
  /**
   * Close the connection and stop all activity.
   * The client is reusable after disconnect - calling {@link connect}
   * or {@link subscribe} will establish a fresh connection.
   * Idempotent: safe to call multiple times.
   */
  disconnect() {
    this.intentionalClose = true;
    this.stopHeartbeat();
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    for (const sub of this.subscriptions.values()) {
      if (sub.status !== "closed") {
        ;
        sub._status = "closed";
      }
    }
    this.subscriptions.clear();
    this.emit("disconnected", "intentional");
    if (this.ws) {
      this.ws.onclose = null;
      this.ws.close();
      this.ws = null;
    }
    this._state = "disconnected";
  }
  /**
   * Subscribe to table changes. Automatically connects if
   * `autoConnect` is enabled and the client is disconnected.
   *
   * @param table - Database table name to subscribe to.
   * @param options - Event callbacks and optional filter/event settings.
   * @returns A handle for checking status and unsubscribing.
   */
  subscribe(table, options) {
    const id = `sub-${++this.subCounter}`;
    const sub = new Subscription(id, table, options, (data) => this.send(data));
    this.subscriptions.set(id, sub);
    if (this.autoConnect && this._state === "disconnected") {
      this.connect();
    }
    if (this._state === "connected") {
      this.send(JSON.stringify(sub.buildSubscribeMessage()));
    }
    return sub;
  }
  /**
   * Send a raw JSON string over the WebSocket if it is open.
   *
   * @param data - Serialised JSON message.
   */
  send(data) {
    if (this.ws?.readyState === 1) {
      this.ws.send(data);
    }
  }
  /** Flush all messages that were queued while the socket was not open. */
  flushPendingMessages() {
    while (this.pendingMessages.length > 0) {
      this.send(this.pendingMessages.shift());
    }
  }
  /** Re-send subscribe messages for all active subscriptions after reconnect. */
  resubscribeAll() {
    for (const sub of this.subscriptions.values()) {
      if (sub.status === "closed" || sub.status === "error") continue;
      sub.resetForReconnect();
      this.send(JSON.stringify(sub.buildSubscribeMessage()));
    }
  }
  /**
   * Parse and route an incoming server message to the appropriate handler.
   *
   * @param data - Raw JSON string from the server.
   */
  handleMessage(data) {
    let msg;
    try {
      msg = JSON.parse(data);
    } catch {
      return;
    }
    switch (msg.type) {
      case "subscribed": {
        const sub = this.subscriptions.get(msg.id);
        sub?.handleSubscribed();
        break;
      }
      case "unsubscribed": {
        this.subscriptions.delete(msg.id);
        break;
      }
      case "event": {
        const sub = this.subscriptions.get(msg.id);
        sub?.handleEvent(msg);
        break;
      }
      case "error": {
        const errMsg = msg;
        if (errMsg.id) {
          const sub = this.subscriptions.get(errMsg.id);
          sub?.handleError(errMsg);
        } else {
          this.emit("error", new Error(`[${errMsg.error_code}] ${errMsg.message}`));
        }
        break;
      }
      case "heartbeat_ack":
        break;
    }
  }
  /** Start the heartbeat interval timer. */
  startHeartbeat() {
    this.heartbeatTimer = setInterval(() => {
      this.clearHeartbeatAckTimer();
      this.send(JSON.stringify({ type: "heartbeat" }));
      this.heartbeatAckTimer = setTimeout(() => {
        this.ws?.close();
      }, this.heartbeatTimeout);
    }, this.heartbeatInterval);
  }
  /** Stop the heartbeat interval and any pending ack timeout. */
  stopHeartbeat() {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
    this.clearHeartbeatAckTimer();
  }
  /** Cancel a pending heartbeat ack timeout if one exists. */
  clearHeartbeatAckTimer() {
    if (this.heartbeatAckTimer) {
      clearTimeout(this.heartbeatAckTimer);
      this.heartbeatAckTimer = null;
    }
  }
  /**
   * Schedule a reconnection attempt with exponential backoff.
   * Emits `reconnecting` before each attempt and `disconnected`
   * if max retries are exceeded.
   */
  scheduleReconnect() {
    this.retryCount++;
    if (this.retryCount > this.maxRetries) {
      this._state = "disconnected";
      this.emit("disconnected", "max retries exceeded");
      return;
    }
    this._state = "reconnecting";
    this.emit("reconnecting", this.retryCount);
    const delay = Math.min(
      INITIAL_RECONNECT_DELAY * Math.pow(2, this.retryCount - 1),
      this.maxReconnectDelay
    );
    this.reconnectTimer = setTimeout(() => {
      this.doConnect();
    }, delay);
  }
};
export {
  MimDBRealtimeClient
};
//# sourceMappingURL=index.js.map