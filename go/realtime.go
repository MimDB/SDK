package mimdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/MimDB/SDK/go/internal/wsutil"
	"github.com/coder/websocket"
)

// wsMessage is the JSON envelope used for all client-server communication over
// the realtime WebSocket connection. Both outbound (subscribe, unsubscribe)
// and inbound (subscribed, event, error) messages share this structure.
type wsMessage struct {
	Type      string         `json:"type"`
	ID        string         `json:"id,omitempty"`
	Table     string         `json:"table,omitempty"`
	Event     string         `json:"event,omitempty"`
	Filter    string         `json:"filter,omitempty"`
	New       map[string]any `json:"new,omitempty"`
	Old       map[string]any `json:"old,omitempty"`
	ErrorCode string         `json:"error_code,omitempty"`
	Message   string         `json:"message,omitempty"`
}

// SubscribeOptions configures a realtime table subscription.
//
// OnEvent is required and receives every data change that matches the
// subscription filter. OnError and OnSubscribed are optional lifecycle
// callbacks.
type SubscribeOptions struct {
	// Event selects which mutation types to listen for. Use [EventAll] ("*")
	// to receive inserts, updates, and deletes.
	Event EventType

	// Filter narrows the subscription to rows matching a PostgREST-style
	// filter expression, e.g. "id=eq.lobby_smp".
	Filter string

	// OnEvent is called for each data change event that matches the
	// subscription. It must not be nil.
	OnEvent func(RealtimeEvent)

	// OnError is called when the server sends an error scoped to this
	// subscription. If nil, subscription errors are silently dropped.
	OnError func(RealtimeError)

	// OnSubscribed is called once the server confirms the subscription is
	// active. If nil, the confirmation is silently consumed.
	OnSubscribed func()
}

// RealtimeOptions configures heartbeat, reconnect, and retry behavior for the
// realtime WebSocket client.
type RealtimeOptions struct {
	// HeartbeatInterval is how often the client sends a heartbeat ping to the
	// server. Default: 30s.
	HeartbeatInterval time.Duration

	// HeartbeatTimeout is how long the client waits for a heartbeat_ack before
	// considering the connection dead. Default: 10s.
	HeartbeatTimeout time.Duration

	// BaseReconnectDelay is the initial delay before the first reconnect
	// attempt. Subsequent attempts double this value up to MaxReconnectDelay.
	// Default: 1s.
	BaseReconnectDelay time.Duration

	// MaxReconnectDelay caps the exponential backoff delay between reconnect
	// attempts. Default: 30s.
	MaxReconnectDelay time.Duration

	// MaxRetries limits the number of consecutive reconnect attempts. Zero
	// means infinite retries. Default: 0 (infinite).
	MaxRetries int

	// AutoReconnect enables automatic reconnection when the connection drops
	// unexpectedly. Default: true.
	AutoReconnect bool
}

// defaultRealtimeOptions returns the default heartbeat and reconnect settings.
func defaultRealtimeOptions() RealtimeOptions {
	return RealtimeOptions{
		HeartbeatInterval:  30 * time.Second,
		HeartbeatTimeout:   10 * time.Second,
		BaseReconnectDelay: 1 * time.Second,
		MaxReconnectDelay:  30 * time.Second,
		MaxRetries:         0,
		AutoReconnect:      true,
	}
}

// Subscription represents an active or pending realtime subscription. Use
// [Subscription.Status] to query the current lifecycle state and
// [Subscription.Unsubscribe] to cancel the subscription.
type Subscription struct {
	id    string
	table string
	opts  SubscribeOptions
	rt    *RealtimeClient

	mu     sync.RWMutex
	status string // "pending", "active", "closed", "error"
}

// Status returns the current lifecycle state of the subscription. Possible
// values are "pending", "active", "closed", and "error". It is safe for
// concurrent use.
func (s *Subscription) Status() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// setStatus updates the subscription status under the write lock.
func (s *Subscription) setStatus(status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
}

// Unsubscribe cancels the subscription by sending an unsubscribe message to
// the server, marking the subscription as closed, and removing it from the
// client's subscription map.
func (s *Subscription) Unsubscribe() {
	s.setStatus("closed")

	s.rt.mu.Lock()
	delete(s.rt.subs, s.id)
	conn := s.rt.conn
	s.rt.mu.Unlock()

	if conn != nil {
		msg := wsMessage{Type: "unsubscribe", ID: s.id}
		data, err := json.Marshal(msg)
		if err == nil {
			_ = conn.Write(context.Background(), websocket.MessageText, data)
		}
	}
}

// RealtimeClient manages a WebSocket connection to the MimDB realtime service.
// It provides a state machine that tracks connection lifecycle, event listeners
// for connection state changes, heartbeat keep-alive, automatic reconnection
// with exponential backoff, and a pending message buffer for subscriptions
// created during reconnection.
//
// Obtain an instance through [Client.Realtime]; do not construct directly.
type RealtimeClient struct {
	client *Client
	opts   RealtimeOptions

	mu             sync.RWMutex
	state          ConnectionState
	conn           *websocket.Conn
	listeners      map[string][]listener
	subs           map[string]*Subscription // keyed by subscription ID
	cancelFn       context.CancelFunc       // cancels the read loop + heartbeat goroutines
	nextListenerID int                      // monotonic counter for unique listener IDs
	nextSubID      int                      // monotonic counter for subscription IDs
	stopCh         chan struct{}             // closed by Disconnect to suppress reconnect
	ackCh          chan struct{}             // heartbeat_ack signals from read loop
	pendingBuf     wsutil.MessageBuffer     // buffers outbound messages during reconnect
}

// listener pairs a callback with a unique ID so it can be unsubscribed.
type listener struct {
	id int
	fn func(any)
}

// SetOptions configures heartbeat, reconnect, and retry behavior. Call before
// [RealtimeClient.Connect] to apply settings for the initial connection. It is
// safe to call between reconnect cycles as well.
func (r *RealtimeClient) SetOptions(opts RealtimeOptions) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.opts = opts
}

// getOpts returns a snapshot of the current options under the read lock.
func (r *RealtimeClient) getOpts() RealtimeOptions {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.opts
}

// Connect establishes a WebSocket connection to the MimDB realtime service.
//
// The provided context is used only for the initial WebSocket dial handshake.
// Once connected, background goroutines (read loop, heartbeat) run
// independently of ctx. To stop the client and all background goroutines,
// call [RealtimeClient.Disconnect].
//
// It transitions the state to [StateConnecting], dials the server, and on
// success moves to [StateConnected]. On failure the state returns to
// [StateDisconnected] and the error is returned.
//
// A background read loop and heartbeat goroutine are started on successful
// connection. Any messages buffered while reconnecting are flushed immediately.
func (r *RealtimeClient) Connect(ctx context.Context) error {
	if err := r.client.requireProjectRef(); err != nil {
		return err
	}

	r.setState(StateConnecting)
	r.emit("connecting", nil)

	conn, err := r.dial(ctx)
	if err != nil {
		r.setState(StateDisconnected)
		r.emit("disconnected", fmt.Sprintf("dial failed: %v", err))
		return fmt.Errorf("realtime connect: %w", err)
	}

	stopCh := make(chan struct{})
	ackCh := make(chan struct{}, 1)

	r.mu.Lock()
	r.conn = conn
	r.stopCh = stopCh
	r.ackCh = ackCh
	r.mu.Unlock()

	r.setState(StateConnected)
	r.emit("connected", nil)

	// Flush any messages that were buffered while connecting.
	r.flushPendingMessages(conn)

	// Start the read loop and heartbeat goroutines.
	loopCtx, cancel := context.WithCancel(context.Background())
	r.mu.Lock()
	r.cancelFn = cancel
	r.mu.Unlock()

	go r.readLoop(loopCtx, stopCh)
	go r.heartbeatLoop(loopCtx, conn, ackCh)

	return nil
}

// Disconnect closes the WebSocket connection and transitions the state to
// [StateDisconnected]. It signals the stop channel to suppress automatic
// reconnection. It is safe to call even if not connected.
func (r *RealtimeClient) Disconnect() {
	r.mu.Lock()
	conn := r.conn
	cancel := r.cancelFn
	stopCh := r.stopCh
	r.conn = nil
	r.cancelFn = nil
	r.stopCh = nil
	r.ackCh = nil
	r.mu.Unlock()

	// Signal intentional disconnect to suppress reconnect.
	if stopCh != nil {
		close(stopCh)
	}

	if cancel != nil {
		cancel()
	}

	if conn != nil {
		conn.Close(websocket.StatusNormalClosure, "client disconnect")
	}

	r.setState(StateDisconnected)
	r.emit("disconnected", "client disconnect")
}

// State returns the current [ConnectionState]. It is safe for concurrent use.
func (r *RealtimeClient) State() ConnectionState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state
}

// On registers a callback for a connection lifecycle event. Supported event
// names are "connected", "disconnected", "reconnecting", and "error".
//
// The callback receives event-specific data:
//   - "connected"    -> nil
//   - "disconnected" -> reason string
//   - "reconnecting" -> attempt number (int)
//   - "error"        -> error
//
// On returns an unsubscribe function that removes the callback. Both On and
// the returned unsubscribe function are safe for concurrent use.
func (r *RealtimeClient) On(event string, cb func(any)) func() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nextListenerID++
	id := r.nextListenerID

	r.listeners[event] = append(r.listeners[event], listener{id: id, fn: cb})

	return func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.removeListenerLocked(event, id)
	}
}

// Subscribe creates a new subscription for the given table. If the connection
// is established, it sends the subscribe message immediately. If the
// connection is in the connecting or reconnecting state, the message is
// buffered and will be sent once the connection opens.
//
// The provided [SubscribeOptions.OnEvent] callback must not be nil.
func (r *RealtimeClient) Subscribe(ctx context.Context, table string, opts SubscribeOptions) (*Subscription, error) {
	if opts.OnEvent == nil {
		return nil, fmt.Errorf("realtime subscribe: OnEvent callback is required")
	}

	r.mu.Lock()
	r.nextSubID++
	id := fmt.Sprintf("sub-%d", r.nextSubID)

	sub := &Subscription{
		id:     id,
		table:  table,
		opts:   opts,
		rt:     r,
		status: "pending",
	}

	if r.subs == nil {
		r.subs = make(map[string]*Subscription)
	}
	r.subs[id] = sub

	conn := r.conn
	state := r.state
	r.mu.Unlock()

	msg := wsMessage{
		Type:   "subscribe",
		ID:     id,
		Table:  table,
		Event:  string(opts.Event),
		Filter: opts.Filter,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("realtime subscribe: marshal: %w", err)
	}

	// If the connection is not yet open, buffer the message for later.
	if conn == nil || state == StateConnecting || state == StateReconnecting {
		r.pendingBuf.Enqueue(data)
		return sub, nil
	}

	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		r.mu.Lock()
		delete(r.subs, id)
		r.mu.Unlock()
		return nil, fmt.Errorf("realtime subscribe: write: %w", err)
	}

	return sub, nil
}

// ---------- Internal helpers ----------

// setState updates the connection state under the write lock.
func (r *RealtimeClient) setState(s ConnectionState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state = s
}

// emit dispatches an event to all registered listeners for the given event
// name. Callbacks are invoked synchronously; the listener slice is copied
// before release to prevent deadlocks if a callback calls On/unsubscribe.
func (r *RealtimeClient) emit(event string, data any) {
	r.mu.RLock()
	cbs := make([]func(any), len(r.listeners[event]))
	for i, l := range r.listeners[event] {
		cbs[i] = l.fn
	}
	r.mu.RUnlock()

	for _, fn := range cbs {
		fn(data)
	}
}

// removeListenerLocked removes a listener by ID from the given event's list.
// Caller must hold r.mu write lock.
func (r *RealtimeClient) removeListenerLocked(event string, id int) {
	listeners := r.listeners[event]
	for i, l := range listeners {
		if l.id == id {
			r.listeners[event] = append(listeners[:i], listeners[i+1:]...)
			return
		}
	}
}

// dial performs the WebSocket handshake and returns the connection. It is used
// by both Connect and the reconnect loop.
func (r *RealtimeClient) dial(ctx context.Context) (*websocket.Conn, error) {
	wsURL, headers := r.buildDialParams()

	opts := &websocket.DialOptions{}
	if len(headers) > 0 {
		opts.HTTPHeader = headers
	}

	conn, _, err := websocket.Dial(ctx, wsURL, opts)
	return conn, err
}

// buildDialParams constructs the WebSocket URL and optional HTTP headers for
// the dial handshake. If the parent client has an access token set, it is sent
// as an Authorization header; otherwise the API key is appended as a query
// parameter.
func (r *RealtimeClient) buildDialParams() (string, http.Header) {
	base := r.client.baseURL
	ref := r.client.projectRef

	// Convert HTTP(S) scheme to WS(S).
	wsURL := base
	if strings.HasPrefix(wsURL, "https://") {
		wsURL = "wss://" + wsURL[len("https://"):]
	} else if strings.HasPrefix(wsURL, "http://") {
		wsURL = "ws://" + wsURL[len("http://"):]
	}

	wsURL += "/v1/realtime/" + ref

	headers := make(http.Header)

	accessToken := r.client.getAccessToken()
	if accessToken != "" {
		headers.Set("Authorization", "Bearer "+accessToken)
	} else {
		wsURL += "?apikey=" + r.client.options.APIKey
	}

	return wsURL, headers
}

// ---------- Read loop ----------

// readLoop reads messages from the WebSocket connection in a loop. It runs in
// a dedicated goroutine started by Connect. When the connection drops or the
// context is cancelled, it either triggers reconnection (if auto-reconnect is
// enabled) or emits disconnected events.
//
// Inbound messages are JSON-decoded into [wsMessage] and dispatched based on
// type:
//   - "subscribed"    -> mark subscription active, call OnSubscribed
//   - "unsubscribed"  -> mark subscription closed
//   - "event"         -> construct [RealtimeEvent], call OnEvent
//   - "error" with ID -> call subscription OnError
//   - "error" no ID   -> emit to connection-level "error" listeners
//   - "heartbeat_ack" -> signal the heartbeat goroutine
func (r *RealtimeClient) readLoop(ctx context.Context, stopCh chan struct{}) {
	for {
		r.mu.RLock()
		conn := r.conn
		r.mu.RUnlock()

		if conn == nil {
			return
		}

		_, data, err := conn.Read(ctx)
		if err != nil {
			// Deliberate close (context cancelled) - exit silently.
			if ctx.Err() != nil {
				return
			}

			r.emit("error", err)

			// Check if this is an intentional disconnect.
			select {
			case <-stopCh:
				// Disconnect() was called; do not reconnect.
				return
			default:
			}

			// Unexpected disconnect - attempt reconnect if enabled.
			opts := r.getOpts()
			if opts.AutoReconnect {
				go r.reconnectLoop(stopCh)
			} else {
				r.setState(StateDisconnected)
				r.emit("disconnected", fmt.Sprintf("read error: %v", err))
			}
			return
		}

		var msg wsMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		r.dispatchMessage(msg)
	}
}

// ---------- Heartbeat ----------

// heartbeatLoop sends periodic heartbeat messages and expects acks within the
// configured timeout. If an ack is not received in time, the connection is
// closed to trigger reconnection via the read loop.
func (r *RealtimeClient) heartbeatLoop(ctx context.Context, conn *websocket.Conn, ackCh chan struct{}) {
	opts := r.getOpts()
	if opts.HeartbeatInterval <= 0 {
		return
	}

	ticker := time.NewTicker(opts.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Send heartbeat.
			hb, _ := json.Marshal(wsMessage{Type: "heartbeat"})
			writeCtx, writeCancel := context.WithTimeout(ctx, opts.HeartbeatTimeout)
			err := conn.Write(writeCtx, websocket.MessageText, hb)
			writeCancel()

			if err != nil {
				// Write failed - connection is dead, close to trigger reconnect.
				conn.Close(websocket.StatusGoingAway, "heartbeat write failed")
				return
			}

			// Wait for ack within timeout.
			timer := time.NewTimer(opts.HeartbeatTimeout)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-ackCh:
				timer.Stop()
				// Ack received, continue.
			case <-timer.C:
				// Timeout - close the connection to trigger reconnect.
				conn.Close(websocket.StatusGoingAway, "heartbeat timeout")
				return
			}
		}
	}
}

// ---------- Reconnect ----------

// reconnectLoop attempts to re-establish the WebSocket connection using
// exponential backoff. On success it resubscribes all active subscriptions and
// flushes the pending message buffer.
func (r *RealtimeClient) reconnectLoop(stopCh chan struct{}) {
	opts := r.getOpts()

	// Clear the old connection before reconnecting.
	r.mu.Lock()
	if r.conn != nil {
		r.conn.Close(websocket.StatusGoingAway, "reconnecting")
		r.conn = nil
	}
	if r.cancelFn != nil {
		r.cancelFn()
		r.cancelFn = nil
	}
	r.mu.Unlock()

	r.setState(StateReconnecting)

	for attempt := 1; ; attempt++ {
		// Check if the user called Disconnect while we were waiting.
		select {
		case <-stopCh:
			r.setState(StateDisconnected)
			r.emit("disconnected", "client disconnect during reconnect")
			return
		default:
		}

		r.emit("reconnecting", attempt)

		// Check max retries.
		if opts.MaxRetries > 0 && attempt > opts.MaxRetries {
			r.setState(StateDisconnected)
			r.emit("disconnected", "max retries exceeded")
			return
		}

		// Backoff delay.
		baseDelay := opts.BaseReconnectDelay
		if baseDelay <= 0 {
			baseDelay = time.Second
		}
		delay := wsutil.BackoffDelay(attempt, baseDelay, opts.MaxReconnectDelay)
		select {
		case <-stopCh:
			r.setState(StateDisconnected)
			r.emit("disconnected", "client disconnect during reconnect")
			return
		case <-time.After(delay):
		}

		// Attempt to dial.
		dialCtx, dialCancel := context.WithTimeout(context.Background(), 10*time.Second)
		conn, err := r.dial(dialCtx)
		dialCancel()

		if err != nil {
			r.emit("error", fmt.Errorf("reconnect attempt %d: %w", attempt, err))
			continue
		}

		// Reconnected successfully.
		ackCh := make(chan struct{}, 1)
		loopCtx, cancel := context.WithCancel(context.Background())

		r.mu.Lock()
		r.conn = conn
		r.cancelFn = cancel
		r.ackCh = ackCh
		r.mu.Unlock()

		r.setState(StateConnected)
		r.emit("connected", nil)

		// Resubscribe all active/pending subscriptions.
		r.resubscribeAll(conn)

		// Flush any messages buffered during reconnection.
		r.flushPendingMessages(conn)

		// Restart the read loop and heartbeat.
		go r.readLoop(loopCtx, stopCh)
		go r.heartbeatLoop(loopCtx, conn, ackCh)

		return
	}
}

// resubscribeAll re-sends subscribe messages for all non-closed subscriptions
// after a successful reconnect. Each subscription is reset to "pending" until
// the server confirms it.
func (r *RealtimeClient) resubscribeAll(conn *websocket.Conn) {
	r.mu.RLock()
	subs := make([]*Subscription, 0, len(r.subs))
	for _, sub := range r.subs {
		subs = append(subs, sub)
	}
	r.mu.RUnlock()

	for _, sub := range subs {
		status := sub.Status()
		if status == "closed" {
			continue
		}

		sub.setStatus("pending")

		msg := wsMessage{
			Type:   "subscribe",
			ID:     sub.id,
			Table:  sub.table,
			Event:  string(sub.opts.Event),
			Filter: sub.opts.Filter,
		}

		data, err := json.Marshal(msg)
		if err != nil {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = conn.Write(ctx, websocket.MessageText, data)
		cancel()
	}
}

// flushPendingMessages sends all buffered messages over the given connection.
// Messages that fail to send are silently dropped.
func (r *RealtimeClient) flushPendingMessages(conn *websocket.Conn) {
	msgs := r.pendingBuf.Flush()
	for _, data := range msgs {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = conn.Write(ctx, websocket.MessageText, data)
		cancel()
	}
}

// ---------- Message dispatch ----------

// dispatchMessage routes a parsed WebSocket message to the appropriate
// subscription callback or connection-level listener based on message type.
func (r *RealtimeClient) dispatchMessage(msg wsMessage) {
	switch msg.Type {
	case "subscribed":
		sub := r.getSub(msg.ID)
		if sub == nil {
			return
		}
		sub.setStatus("active")
		if sub.opts.OnSubscribed != nil {
			sub.opts.OnSubscribed()
		}

	case "unsubscribed":
		sub := r.getSub(msg.ID)
		if sub == nil {
			return
		}
		sub.setStatus("closed")

	case "event":
		sub := r.getSub(msg.ID)
		if sub == nil {
			return
		}
		evt := RealtimeEvent{
			Type:  EventType(msg.Event),
			Table: msg.Table,
			New:   msg.New,
			Old:   msg.Old,
		}
		sub.opts.OnEvent(evt)

	case "error":
		if msg.ID != "" {
			sub := r.getSub(msg.ID)
			if sub == nil {
				return
			}
			sub.setStatus("error")
			if sub.opts.OnError != nil {
				sub.opts.OnError(RealtimeError{
					Code:    msg.ErrorCode,
					Message: msg.Message,
				})
			}
		} else {
			r.emit("error", fmt.Errorf("%s: %s", msg.ErrorCode, msg.Message))
		}

	case "heartbeat_ack":
		r.mu.RLock()
		ackCh := r.ackCh
		r.mu.RUnlock()

		if ackCh != nil {
			select {
			case ackCh <- struct{}{}:
			default:
			}
		}

	default:
		// Unknown message types are silently dropped.
	}
}

// getSub retrieves a subscription by ID from the map. Returns nil if not found.
func (r *RealtimeClient) getSub(id string) *Subscription {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.subs[id]
}
