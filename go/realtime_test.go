package mimdb

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// isConnClosedErr reports whether err is a connection-closed error that is
// expected during reconnection tests (e.g. "connection reset by peer" or
// "broken pipe"). These occur when the mock server tries to write to a
// WebSocket that the client has already torn down.
func isConnClosedErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "connection reset by peer") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "use of closed network connection")
}

// tryWriteWSMessage is like writeWSMessage but tolerates connection-closed
// errors that are expected in reconnection tests. It returns false when the
// write failed so the caller can break out of its read loop.
func tryWriteWSMessage(t *testing.T, c *websocket.Conn, msg wsMessage) bool {
	t.Helper()
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("tryWriteWSMessage marshal: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, data); err != nil {
		if isConnClosedErr(err) {
			t.Logf("tryWriteWSMessage: ignoring expected write error: %v", err)
			return false
		}
		t.Fatalf("tryWriteWSMessage write: %v", err)
	}
	return true
}

// newTestWSServer creates an httptest.Server that upgrades HTTP requests to
// WebSocket connections. The optional handler is called with each accepted
// connection; if nil the server simply accepts and holds the connection open
// until the client disconnects.
func newTestWSServer(t *testing.T, handler func(*websocket.Conn)) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			t.Logf("ws accept error: %v", err)
			return
		}
		defer c.CloseNow()

		if handler != nil {
			handler(c)
			return
		}

		// Default: hold open until the client closes.
		ctx := context.Background()
		for {
			_, _, err := c.Read(ctx)
			if err != nil {
				return
			}
		}
	}))
}

// TestRealtime_Connect verifies that Connect dials a mock WebSocket server and
// transitions the state to StateConnected.
func TestRealtime_Connect(t *testing.T) {
	srv := newTestWSServer(t, nil)
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	rt := client.Realtime()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer rt.Disconnect()

	if got := rt.State(); got != StateConnected {
		t.Errorf("State() = %q, want %q", got, StateConnected)
	}
}

// TestRealtime_Disconnect verifies that Disconnect closes the connection and
// sets the state to StateDisconnected.
func TestRealtime_Disconnect(t *testing.T) {
	srv := newTestWSServer(t, nil)
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	rt := client.Realtime()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}

	rt.Disconnect()

	if got := rt.State(); got != StateDisconnected {
		t.Errorf("State() after Disconnect = %q, want %q", got, StateDisconnected)
	}
}

// TestRealtime_State verifies state transitions through the full connect and
// disconnect lifecycle.
func TestRealtime_State(t *testing.T) {
	srv := newTestWSServer(t, nil)
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	rt := client.Realtime()

	// Initial state is disconnected.
	if got := rt.State(); got != StateDisconnected {
		t.Errorf("initial State() = %q, want %q", got, StateDisconnected)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}

	if got := rt.State(); got != StateConnected {
		t.Errorf("State() after Connect = %q, want %q", got, StateConnected)
	}

	rt.Disconnect()

	if got := rt.State(); got != StateDisconnected {
		t.Errorf("State() after Disconnect = %q, want %q", got, StateDisconnected)
	}
}

// TestRealtime_OnConnected verifies that a listener registered for the
// "connected" event fires when the connection is established.
func TestRealtime_OnConnected(t *testing.T) {
	srv := newTestWSServer(t, nil)
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	rt := client.Realtime()

	var mu sync.Mutex
	called := false

	rt.On("connected", func(data any) {
		mu.Lock()
		defer mu.Unlock()
		called = true
		if data != nil {
			t.Errorf("connected callback data = %v, want nil", data)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer rt.Disconnect()

	mu.Lock()
	defer mu.Unlock()
	if !called {
		t.Error("connected callback was not called")
	}
}

// TestRealtime_OnDisconnected verifies that a listener registered for the
// "disconnected" event fires with a reason string when the client disconnects.
func TestRealtime_OnDisconnected(t *testing.T) {
	srv := newTestWSServer(t, nil)
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	rt := client.Realtime()

	ch := make(chan any, 1)

	rt.On("disconnected", func(data any) {
		ch <- data
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}

	rt.Disconnect()

	select {
	case data := <-ch:
		reason, ok := data.(string)
		if !ok {
			t.Fatalf("disconnected data type = %T, want string", data)
		}
		if reason == "" {
			t.Error("disconnected reason is empty")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("disconnected callback was not called within timeout")
	}
}

// TestRealtime_OnUnsubscribe verifies that calling the unsubscribe function
// returned by On removes the callback so it does not fire on subsequent events.
func TestRealtime_OnUnsubscribe(t *testing.T) {
	srv := newTestWSServer(t, nil)
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	rt := client.Realtime()

	var mu sync.Mutex
	callCount := 0

	unsub := rt.On("connected", func(data any) {
		mu.Lock()
		defer mu.Unlock()
		callCount++
	})

	// Unsubscribe before connecting.
	unsub()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer rt.Disconnect()

	mu.Lock()
	defer mu.Unlock()
	if callCount != 0 {
		t.Errorf("callback called %d times after unsubscribe, want 0", callCount)
	}
}

// TestRealtime_RequiresProjectRef verifies that Connect returns an error when
// the client has no ProjectRef configured.
func TestRealtime_RequiresProjectRef(t *testing.T) {
	client := NewClient("https://api.mimdb.dev", Options{
		AdminSecret: "admin-secret",
	})

	rt := client.Realtime()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := rt.Connect(ctx)
	if err == nil {
		rt.Disconnect()
		t.Fatal("Connect() should return error without ProjectRef")
	}
	if !strings.Contains(err.Error(), "ProjectRef") {
		t.Errorf("error = %q, should mention ProjectRef", err.Error())
	}
}

// TestRealtime_URLConstruction verifies that the WebSocket URL is correctly
// assembled from the client's base URL, project ref, and API key. It inspects
// the request received by the mock server to validate the URL and headers.
func TestRealtime_URLConstruction(t *testing.T) {
	var capturedURL string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()

		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			t.Logf("ws accept error: %v", err)
			return
		}
		defer c.CloseNow()

		// Hold open until client disconnects.
		ctx := context.Background()
		for {
			_, _, err := c.Read(ctx)
			if err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "myref",
		APIKey:     "my-api-key",
	})

	rt := client.Realtime()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer rt.Disconnect()

	// Verify path.
	if !strings.HasPrefix(capturedURL, "/v1/realtime/myref") {
		t.Errorf("URL path = %q, want prefix /v1/realtime/myref", capturedURL)
	}

	// Verify apikey query parameter.
	if !strings.Contains(capturedURL, "apikey=my-api-key") {
		t.Errorf("URL = %q, should contain apikey=my-api-key", capturedURL)
	}
}

// TestRealtime_URLConstruction_AccessToken verifies that when an access token
// is set, it is sent as a Bearer token header instead of using the apikey query
// parameter.
func TestRealtime_URLConstruction_AccessToken(t *testing.T) {
	var capturedURL string
	var capturedHeaders http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		capturedHeaders = r.Header.Clone()

		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			t.Logf("ws accept error: %v", err)
			return
		}
		defer c.CloseNow()

		ctx := context.Background()
		for {
			_, _, err := c.Read(ctx)
			if err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "myref",
		APIKey:     "my-api-key",
	})
	client.SetAccessToken("user-jwt-token")

	rt := client.Realtime()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer rt.Disconnect()

	// With access token, should use Authorization header instead of apikey param.
	authHeader := capturedHeaders.Get("Authorization")
	if authHeader != "Bearer user-jwt-token" {
		t.Errorf("Authorization header = %q, want %q", authHeader, "Bearer user-jwt-token")
	}

	// Should NOT have apikey in the URL when using access token.
	if strings.Contains(capturedURL, "apikey=") {
		t.Errorf("URL = %q, should NOT contain apikey param when access token is set", capturedURL)
	}
}

// TestRealtime_LazyInit verifies that Realtime() returns the same instance on
// repeated calls (lazy singleton pattern).
func TestRealtime_LazyInit(t *testing.T) {
	client := NewClient("https://api.mimdb.dev", Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	rt1 := client.Realtime()
	rt2 := client.Realtime()

	if rt1 != rt2 {
		t.Error("Realtime() should return the same instance on repeated calls")
	}
}

// TestRealtime_ConnectFailure verifies that Connect returns an error and sets
// state to disconnected when the dial fails.
func TestRealtime_ConnectFailure(t *testing.T) {
	// Point at a URL that will refuse connections.
	client := NewClient("http://127.0.0.1:1", Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	rt := client.Realtime()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := rt.Connect(ctx)
	if err == nil {
		rt.Disconnect()
		t.Fatal("Connect() should return error for unreachable server")
	}

	if got := rt.State(); got != StateDisconnected {
		t.Errorf("State() after failed Connect = %q, want %q", got, StateDisconnected)
	}
}

// TestRealtime_OnError verifies that the "error" event fires with an error
// value when the server drops the connection unexpectedly.
func TestRealtime_OnError(t *testing.T) {
	srv := newTestWSServer(t, func(c *websocket.Conn) {
		// Close immediately with an abnormal reason to trigger read error.
		c.Close(websocket.StatusInternalError, "server error")
	})
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	rt := client.Realtime()

	errCh := make(chan any, 1)
	rt.On("error", func(data any) {
		errCh <- data
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer rt.Disconnect()

	select {
	case data := <-errCh:
		if _, ok := data.(error); !ok {
			t.Errorf("error event data type = %T, want error", data)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("error callback was not called within timeout")
	}
}

// ---------- Subscription Tests ----------

// readWSMessage reads and unmarshals one JSON message from the WebSocket.
func readWSMessage(t *testing.T, c *websocket.Conn) wsMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("readWSMessage: %v", err)
	}

	var msg wsMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("readWSMessage unmarshal: %v (raw: %s)", err, data)
	}
	return msg
}

// writeWSMessage marshals and sends a JSON message over the WebSocket.
func writeWSMessage(t *testing.T, c *websocket.Conn, msg wsMessage) {
	t.Helper()
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("writeWSMessage marshal: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("writeWSMessage write: %v", err)
	}
}

// TestRealtime_Subscribe verifies that Subscribe sends a subscribe message,
// and when the server confirms with "subscribed", the subscription status
// transitions to "active".
func TestRealtime_Subscribe(t *testing.T) {
	srv := newTestWSServer(t, func(c *websocket.Conn) {
		// Read the subscribe message from the client.
		msg := readWSMessage(t, c)
		if msg.Type != "subscribe" {
			t.Errorf("expected type=subscribe, got %q", msg.Type)
		}
		if msg.Table != "users" {
			t.Errorf("expected table=users, got %q", msg.Table)
		}
		if msg.Event != string(EventAll) {
			t.Errorf("expected event=*, got %q", msg.Event)
		}

		// Send confirmation.
		writeWSMessage(t, c, wsMessage{Type: "subscribed", ID: msg.ID})

		// Hold open.
		ctx := context.Background()
		for {
			if _, _, err := c.Read(ctx); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	rt := client.Realtime()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer rt.Disconnect()

	sub, err := rt.Subscribe(ctx, "users", SubscribeOptions{
		Event:   EventAll,
		OnEvent: func(RealtimeEvent) {},
	})
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}

	// Wait for the subscription to become active.
	deadline := time.After(3 * time.Second)
	for sub.Status() != "active" {
		select {
		case <-deadline:
			t.Fatalf("subscription status = %q, want %q", sub.Status(), "active")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// TestRealtime_Subscribe_ReceiveEvent verifies that when the server sends an
// INSERT event, the OnEvent callback fires with the correct RealtimeEvent.
func TestRealtime_Subscribe_ReceiveEvent(t *testing.T) {
	eventCh := make(chan RealtimeEvent, 1)

	srv := newTestWSServer(t, func(c *websocket.Conn) {
		msg := readWSMessage(t, c)

		// Confirm subscription.
		writeWSMessage(t, c, wsMessage{Type: "subscribed", ID: msg.ID})

		// Send an INSERT event.
		writeWSMessage(t, c, wsMessage{
			Type:  "event",
			ID:    msg.ID,
			Event: "INSERT",
			Table: "users",
			New:   map[string]any{"id": "u1", "name": "Alice"},
		})

		// Hold open.
		ctx := context.Background()
		for {
			if _, _, err := c.Read(ctx); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	rt := client.Realtime()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer rt.Disconnect()

	_, err := rt.Subscribe(ctx, "users", SubscribeOptions{
		Event: EventAll,
		OnEvent: func(evt RealtimeEvent) {
			eventCh <- evt
		},
	})
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}

	select {
	case evt := <-eventCh:
		if evt.Type != EventInsert {
			t.Errorf("event Type = %q, want %q", evt.Type, EventInsert)
		}
		if evt.Table != "users" {
			t.Errorf("event Table = %q, want %q", evt.Table, "users")
		}
		if evt.New["id"] != "u1" {
			t.Errorf("event New[id] = %v, want %q", evt.New["id"], "u1")
		}
		if evt.New["name"] != "Alice" {
			t.Errorf("event New[name] = %v, want %q", evt.New["name"], "Alice")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("OnEvent callback was not called within timeout")
	}
}

// TestRealtime_Subscribe_UpdateEvent verifies that an UPDATE event carries
// both New and Old row data.
func TestRealtime_Subscribe_UpdateEvent(t *testing.T) {
	eventCh := make(chan RealtimeEvent, 1)

	srv := newTestWSServer(t, func(c *websocket.Conn) {
		msg := readWSMessage(t, c)
		writeWSMessage(t, c, wsMessage{Type: "subscribed", ID: msg.ID})

		writeWSMessage(t, c, wsMessage{
			Type:  "event",
			ID:    msg.ID,
			Event: "UPDATE",
			Table: "users",
			New:   map[string]any{"id": "u1", "name": "Bob"},
			Old:   map[string]any{"id": "u1", "name": "Alice"},
		})

		ctx := context.Background()
		for {
			if _, _, err := c.Read(ctx); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	rt := client.Realtime()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer rt.Disconnect()

	_, err := rt.Subscribe(ctx, "users", SubscribeOptions{
		Event: EventAll,
		OnEvent: func(evt RealtimeEvent) {
			eventCh <- evt
		},
	})
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}

	select {
	case evt := <-eventCh:
		if evt.Type != EventUpdate {
			t.Errorf("event Type = %q, want %q", evt.Type, EventUpdate)
		}
		if evt.New["name"] != "Bob" {
			t.Errorf("event New[name] = %v, want %q", evt.New["name"], "Bob")
		}
		if evt.Old["name"] != "Alice" {
			t.Errorf("event Old[name] = %v, want %q", evt.Old["name"], "Alice")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("OnEvent callback was not called within timeout")
	}
}

// TestRealtime_Subscribe_DeleteEvent verifies that a DELETE event carries
// the Old row data with at least the primary key.
func TestRealtime_Subscribe_DeleteEvent(t *testing.T) {
	eventCh := make(chan RealtimeEvent, 1)

	srv := newTestWSServer(t, func(c *websocket.Conn) {
		msg := readWSMessage(t, c)
		writeWSMessage(t, c, wsMessage{Type: "subscribed", ID: msg.ID})

		writeWSMessage(t, c, wsMessage{
			Type:  "event",
			ID:    msg.ID,
			Event: "DELETE",
			Table: "users",
			Old:   map[string]any{"id": "u1"},
		})

		ctx := context.Background()
		for {
			if _, _, err := c.Read(ctx); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	rt := client.Realtime()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer rt.Disconnect()

	_, err := rt.Subscribe(ctx, "users", SubscribeOptions{
		Event: EventAll,
		OnEvent: func(evt RealtimeEvent) {
			eventCh <- evt
		},
	})
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}

	select {
	case evt := <-eventCh:
		if evt.Type != EventDelete {
			t.Errorf("event Type = %q, want %q", evt.Type, EventDelete)
		}
		if evt.Old["id"] != "u1" {
			t.Errorf("event Old[id] = %v, want %q", evt.Old["id"], "u1")
		}
		if evt.New != nil {
			t.Errorf("event New = %v, want nil for DELETE", evt.New)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("OnEvent callback was not called within timeout")
	}
}

// TestRealtime_Unsubscribe verifies that Unsubscribe sends the "unsubscribe"
// message over the wire and sets the subscription status to "closed".
func TestRealtime_Unsubscribe(t *testing.T) {
	unsubMsgCh := make(chan wsMessage, 1)

	srv := newTestWSServer(t, func(c *websocket.Conn) {
		// Read subscribe message.
		msg := readWSMessage(t, c)
		writeWSMessage(t, c, wsMessage{Type: "subscribed", ID: msg.ID})

		// Read unsubscribe message.
		unsub := readWSMessage(t, c)
		unsubMsgCh <- unsub

		// Send unsubscribed confirmation.
		writeWSMessage(t, c, wsMessage{Type: "unsubscribed", ID: unsub.ID})

		ctx := context.Background()
		for {
			if _, _, err := c.Read(ctx); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	rt := client.Realtime()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer rt.Disconnect()

	sub, err := rt.Subscribe(ctx, "users", SubscribeOptions{
		Event:   EventAll,
		OnEvent: func(RealtimeEvent) {},
	})
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}

	// Wait for active status.
	deadline := time.After(3 * time.Second)
	for sub.Status() != "active" {
		select {
		case <-deadline:
			t.Fatalf("subscription never became active, status = %q", sub.Status())
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	sub.Unsubscribe()

	// Verify the unsubscribe message was sent.
	select {
	case msg := <-unsubMsgCh:
		if msg.Type != "unsubscribe" {
			t.Errorf("unsubscribe message type = %q, want %q", msg.Type, "unsubscribe")
		}
		if msg.ID != sub.id {
			t.Errorf("unsubscribe message ID = %q, want %q", msg.ID, sub.id)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("unsubscribe message was not received within timeout")
	}

	if got := sub.Status(); got != "closed" {
		t.Errorf("subscription status after Unsubscribe = %q, want %q", got, "closed")
	}
}

// TestRealtime_Subscribe_Error verifies that when the server sends an error
// message with a subscription ID, the OnError callback fires with the correct
// RealtimeError.
func TestRealtime_Subscribe_Error(t *testing.T) {
	errCh := make(chan RealtimeError, 1)

	srv := newTestWSServer(t, func(c *websocket.Conn) {
		msg := readWSMessage(t, c)

		// Send a subscription-level error.
		writeWSMessage(t, c, wsMessage{
			Type:      "error",
			ID:        msg.ID,
			ErrorCode: "RT-0001",
			Message:   "table not found",
		})

		ctx := context.Background()
		for {
			if _, _, err := c.Read(ctx); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	rt := client.Realtime()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer rt.Disconnect()

	_, err := rt.Subscribe(ctx, "nonexistent", SubscribeOptions{
		Event:   EventAll,
		OnEvent: func(RealtimeEvent) {},
		OnError: func(re RealtimeError) {
			errCh <- re
		},
	})
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}

	select {
	case re := <-errCh:
		if re.Code != "RT-0001" {
			t.Errorf("error Code = %q, want %q", re.Code, "RT-0001")
		}
		if re.Message != "table not found" {
			t.Errorf("error Message = %q, want %q", re.Message, "table not found")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("OnError callback was not called within timeout")
	}
}

// TestRealtime_Subscribe_OnSubscribed verifies that the OnSubscribed callback
// fires when the server confirms the subscription.
func TestRealtime_Subscribe_OnSubscribed(t *testing.T) {
	subscribedCh := make(chan struct{}, 1)

	srv := newTestWSServer(t, func(c *websocket.Conn) {
		msg := readWSMessage(t, c)
		writeWSMessage(t, c, wsMessage{Type: "subscribed", ID: msg.ID})

		ctx := context.Background()
		for {
			if _, _, err := c.Read(ctx); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	rt := client.Realtime()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer rt.Disconnect()

	_, err := rt.Subscribe(ctx, "users", SubscribeOptions{
		Event:   EventAll,
		OnEvent: func(RealtimeEvent) {},
		OnSubscribed: func() {
			subscribedCh <- struct{}{}
		},
	})
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}

	select {
	case <-subscribedCh:
		// Success.
	case <-time.After(3 * time.Second):
		t.Fatal("OnSubscribed callback was not called within timeout")
	}
}

// TestRealtime_MultipleSubscriptions verifies that events from different
// subscriptions are routed to the correct OnEvent callbacks.
func TestRealtime_MultipleSubscriptions(t *testing.T) {
	usersCh := make(chan RealtimeEvent, 1)
	ordersCh := make(chan RealtimeEvent, 1)

	srv := newTestWSServer(t, func(c *websocket.Conn) {
		// Read first subscribe (users).
		sub1 := readWSMessage(t, c)
		writeWSMessage(t, c, wsMessage{Type: "subscribed", ID: sub1.ID})

		// Read second subscribe (orders).
		sub2 := readWSMessage(t, c)
		writeWSMessage(t, c, wsMessage{Type: "subscribed", ID: sub2.ID})

		// Send an event to users.
		writeWSMessage(t, c, wsMessage{
			Type:  "event",
			ID:    sub1.ID,
			Event: "INSERT",
			Table: "users",
			New:   map[string]any{"id": "u1"},
		})

		// Send an event to orders.
		writeWSMessage(t, c, wsMessage{
			Type:  "event",
			ID:    sub2.ID,
			Event: "INSERT",
			Table: "orders",
			New:   map[string]any{"id": "o1"},
		})

		ctx := context.Background()
		for {
			if _, _, err := c.Read(ctx); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	rt := client.Realtime()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer rt.Disconnect()

	_, err := rt.Subscribe(ctx, "users", SubscribeOptions{
		Event: EventAll,
		OnEvent: func(evt RealtimeEvent) {
			usersCh <- evt
		},
	})
	if err != nil {
		t.Fatalf("Subscribe(users) error: %v", err)
	}

	_, err = rt.Subscribe(ctx, "orders", SubscribeOptions{
		Event: EventAll,
		OnEvent: func(evt RealtimeEvent) {
			ordersCh <- evt
		},
	})
	if err != nil {
		t.Fatalf("Subscribe(orders) error: %v", err)
	}

	// Verify users event routed correctly.
	select {
	case evt := <-usersCh:
		if evt.Table != "users" {
			t.Errorf("users event Table = %q, want %q", evt.Table, "users")
		}
		if evt.New["id"] != "u1" {
			t.Errorf("users event New[id] = %v, want %q", evt.New["id"], "u1")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("users OnEvent callback was not called within timeout")
	}

	// Verify orders event routed correctly.
	select {
	case evt := <-ordersCh:
		if evt.Table != "orders" {
			t.Errorf("orders event Table = %q, want %q", evt.Table, "orders")
		}
		if evt.New["id"] != "o1" {
			t.Errorf("orders event New[id] = %v, want %q", evt.New["id"], "o1")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("orders OnEvent callback was not called within timeout")
	}

	// Verify no cross-routing (users should not have gotten orders' event).
	select {
	case evt := <-usersCh:
		t.Errorf("unexpected second event on users channel: %+v", evt)
	case evt := <-ordersCh:
		t.Errorf("unexpected second event on orders channel: %+v", evt)
	case <-time.After(200 * time.Millisecond):
		// Good - no cross-routing.
	}
}

// TestRealtime_Subscribe_WithFilter verifies that the filter is included in
// the subscribe message sent to the server.
func TestRealtime_Subscribe_WithFilter(t *testing.T) {
	filterCh := make(chan string, 1)

	srv := newTestWSServer(t, func(c *websocket.Conn) {
		msg := readWSMessage(t, c)
		filterCh <- msg.Filter
		writeWSMessage(t, c, wsMessage{Type: "subscribed", ID: msg.ID})

		ctx := context.Background()
		for {
			if _, _, err := c.Read(ctx); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	rt := client.Realtime()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer rt.Disconnect()

	_, err := rt.Subscribe(ctx, "servers", SubscribeOptions{
		Event:   EventInsert,
		Filter:  "id=eq.lobby_smp",
		OnEvent: func(RealtimeEvent) {},
	})
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}

	select {
	case filter := <-filterCh:
		if filter != "id=eq.lobby_smp" {
			t.Errorf("filter = %q, want %q", filter, "id=eq.lobby_smp")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("subscribe message was not received within timeout")
	}
}

// ---------- Heartbeat, Reconnect, Resubscribe Tests ----------

// TestRealtime_Heartbeat verifies that the client sends a heartbeat message
// within the configured interval, and that replying with a heartbeat_ack keeps
// the connection alive.
func TestRealtime_Heartbeat(t *testing.T) {
	heartbeatCh := make(chan struct{}, 1)

	srv := newTestWSServer(t, func(c *websocket.Conn) {
		ctx := context.Background()
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var msg wsMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			if msg.Type == "heartbeat" {
				// Signal that heartbeat was received.
				select {
				case heartbeatCh <- struct{}{}:
				default:
				}
				// Send ack back.
				ack, _ := json.Marshal(wsMessage{Type: "heartbeat_ack"})
				if err := c.Write(ctx, websocket.MessageText, ack); err != nil {
					return
				}
			}
		}
	})
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	rt := client.Realtime()
	rt.SetOptions(RealtimeOptions{
		HeartbeatInterval: 50 * time.Millisecond,
		HeartbeatTimeout:  200 * time.Millisecond,
		MaxReconnectDelay: 100 * time.Millisecond,
		AutoReconnect:     true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer rt.Disconnect()

	// We should receive a heartbeat within the interval + some slack.
	select {
	case <-heartbeatCh:
		// Heartbeat sent and ack replied - connection should remain connected.
		time.Sleep(100 * time.Millisecond)
		if got := rt.State(); got != StateConnected {
			t.Errorf("State() after heartbeat exchange = %q, want %q", got, StateConnected)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("heartbeat message was not received within timeout")
	}
}

// TestRealtime_HeartbeatTimeout verifies that when the server does not reply
// with heartbeat_ack, the client closes the connection and enters the
// reconnecting state.
func TestRealtime_HeartbeatTimeout(t *testing.T) {
	var connCount int32
	var mu sync.Mutex

	srv := newTestWSServer(t, func(c *websocket.Conn) {
		mu.Lock()
		connCount++
		mu.Unlock()

		// Read messages but never reply with heartbeat_ack.
		ctx := context.Background()
		for {
			_, _, err := c.Read(ctx)
			if err != nil {
				return
			}
		}
	})
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	rt := client.Realtime()
	rt.SetOptions(RealtimeOptions{
		HeartbeatInterval: 50 * time.Millisecond,
		HeartbeatTimeout:  30 * time.Millisecond,
		MaxReconnectDelay: 50 * time.Millisecond,
		AutoReconnect:     true,
	})

	reconnectCh := make(chan int, 5)
	rt.On("reconnecting", func(data any) {
		if attempt, ok := data.(int); ok {
			reconnectCh <- attempt
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer rt.Disconnect()

	// Wait for at least one reconnect attempt triggered by heartbeat timeout.
	select {
	case attempt := <-reconnectCh:
		if attempt < 1 {
			t.Errorf("reconnect attempt = %d, want >= 1", attempt)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("reconnecting event was not emitted within timeout")
	}
}

// TestRealtime_Reconnect verifies that when the server closes the connection,
// the client automatically reconnects and transitions back to connected state.
func TestRealtime_Reconnect(t *testing.T) {
	var connCount int32
	var mu sync.Mutex

	srv := newTestWSServer(t, func(c *websocket.Conn) {
		mu.Lock()
		n := connCount
		connCount++
		mu.Unlock()

		if n == 0 {
			// First connection: close immediately to trigger reconnect.
			c.Close(websocket.StatusInternalError, "server restart")
			return
		}

		// Second+ connection: hold open.
		ctx := context.Background()
		for {
			if _, _, err := c.Read(ctx); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	rt := client.Realtime()
	rt.SetOptions(RealtimeOptions{
		HeartbeatInterval: 10 * time.Second, // high to avoid heartbeat interference
		HeartbeatTimeout:  5 * time.Second,
		MaxReconnectDelay: 50 * time.Millisecond,
		AutoReconnect:     true,
	})

	reconnectCh := make(chan int, 5)
	rt.On("reconnecting", func(data any) {
		if attempt, ok := data.(int); ok {
			reconnectCh <- attempt
		}
	})

	connectedCh := make(chan struct{}, 5)
	rt.On("connected", func(data any) {
		connectedCh <- struct{}{}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer rt.Disconnect()

	// Wait for first "connected" (from initial connect).
	select {
	case <-connectedCh:
	case <-time.After(2 * time.Second):
		t.Fatal("initial connected event not received")
	}

	// Wait for reconnecting event.
	select {
	case attempt := <-reconnectCh:
		if attempt != 1 {
			t.Errorf("first reconnect attempt = %d, want 1", attempt)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("reconnecting event was not emitted within timeout")
	}

	// Wait for re-connected.
	select {
	case <-connectedCh:
		if got := rt.State(); got != StateConnected {
			t.Errorf("State() after reconnect = %q, want %q", got, StateConnected)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("connected event was not emitted after reconnect")
	}
}

// TestRealtime_ReconnectBackoff verifies that consecutive reconnect attempts
// use increasing delays via exponential backoff. The server rejects WebSocket
// upgrades to keep the reconnect loop in its retry cycle with incrementing
// attempt numbers.
func TestRealtime_ReconnectBackoff(t *testing.T) {
	var connCount int32
	var mu sync.Mutex

	// First connection succeeds, subsequent connections are rejected via HTTP
	// 500 to force the reconnect loop to keep retrying (dial fails).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		n := connCount
		connCount++
		mu.Unlock()

		if n == 0 {
			// First connection: accept, then close to trigger reconnect.
			c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
				InsecureSkipVerify: true,
			})
			if err != nil {
				return
			}
			// Close immediately to trigger reconnect.
			c.Close(websocket.StatusInternalError, "initial drop")
			return
		}

		// All subsequent connections: reject at HTTP level so dial fails.
		http.Error(w, "server unavailable", http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	rt := client.Realtime()
	rt.SetOptions(RealtimeOptions{
		HeartbeatInterval:  10 * time.Second,
		HeartbeatTimeout:   5 * time.Second,
		BaseReconnectDelay: 50 * time.Millisecond, // 50ms, 100ms, 200ms, ...
		MaxReconnectDelay:  2 * time.Second,
		MaxRetries:         5,
		AutoReconnect:      true,
	})

	type attemptRecord struct {
		attempt int
		at      time.Time
	}

	recordCh := make(chan attemptRecord, 10)
	rt.On("reconnecting", func(data any) {
		if attempt, ok := data.(int); ok {
			recordCh <- attemptRecord{attempt: attempt, at: time.Now()}
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := rt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer rt.Disconnect()

	// Collect first 3 attempts. With base delay of 50ms the expected delays
	// are 50ms, 100ms, 200ms (before each respective attempt).
	var records []attemptRecord
	deadline := time.After(5 * time.Second)
	for len(records) < 3 {
		select {
		case rec := <-recordCh:
			records = append(records, rec)
		case <-deadline:
			t.Fatalf("only received %d reconnect attempts, want at least 3", len(records))
		}
	}

	// Verify attempts are numbered sequentially.
	for i, rec := range records {
		expected := i + 1
		if rec.attempt != expected {
			t.Errorf("records[%d].attempt = %d, want %d", i, rec.attempt, expected)
		}
	}

	// Verify that the gap between attempt 2 and 3 is larger than between 1
	// and 2, confirming exponential backoff. Allow some timing tolerance.
	if len(records) >= 3 {
		gap1 := records[1].at.Sub(records[0].at)
		gap2 := records[2].at.Sub(records[1].at)
		// gap2 should be roughly 2x gap1. Check gap2 >= gap1 * 0.8 to
		// account for scheduling jitter.
		if gap2 < time.Duration(float64(gap1)*0.8) {
			t.Errorf("backoff not increasing: gap1=%v, gap2=%v", gap1, gap2)
		}
	}
}

// TestRealtime_ResubscribeAfterReconnect verifies that after a successful
// reconnect, the client re-sends subscribe messages for all active
// subscriptions.
func TestRealtime_ResubscribeAfterReconnect(t *testing.T) {
	// Track subscribe messages across all connections.
	subscribeMsgCh := make(chan wsMessage, 10)

	srv := newTestWSServer(t, func(c *websocket.Conn) {
		ctx := context.Background()
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var msg wsMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}

			if msg.Type == "subscribe" {
				subscribeMsgCh <- msg
				// Confirm subscription. Use tryWriteWSMessage because the
				// client may close the connection (simulated drop) at any time.
				if !tryWriteWSMessage(t, c, wsMessage{Type: "subscribed", ID: msg.ID}) {
					return
				}
			}

			if msg.Type == "heartbeat" {
				ack, _ := json.Marshal(wsMessage{Type: "heartbeat_ack"})
				if err := c.Write(ctx, websocket.MessageText, ack); err != nil {
					if isConnClosedErr(err) {
						return
					}
				}
			}
		}
	})
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	rt := client.Realtime()
	rt.SetOptions(RealtimeOptions{
		HeartbeatInterval: 10 * time.Second,
		HeartbeatTimeout:  5 * time.Second,
		MaxReconnectDelay: 50 * time.Millisecond,
		AutoReconnect:     true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := rt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer rt.Disconnect()

	// Subscribe to "users" table.
	sub, err := rt.Subscribe(ctx, "users", SubscribeOptions{
		Event:   EventAll,
		OnEvent: func(RealtimeEvent) {},
	})
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}

	// Wait for the initial subscribe message.
	select {
	case msg := <-subscribeMsgCh:
		if msg.Table != "users" {
			t.Errorf("initial subscribe table = %q, want %q", msg.Table, "users")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("initial subscribe message not received")
	}

	// Wait for subscription to become active.
	deadline := time.After(3 * time.Second)
	for sub.Status() != "active" {
		select {
		case <-deadline:
			t.Fatalf("subscription never became active, status = %q", sub.Status())
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Now forcibly close the server-side connection to trigger reconnect.
	// We do this by getting the connection and closing it from server side.
	// Instead, we'll close the client's internal connection to simulate a drop.
	rt.mu.Lock()
	conn := rt.conn
	rt.mu.Unlock()
	conn.Close(websocket.StatusInternalError, "simulate drop")

	// Wait for the re-subscribe message after reconnect.
	select {
	case msg := <-subscribeMsgCh:
		if msg.Table != "users" {
			t.Errorf("resubscribe table = %q, want %q", msg.Table, "users")
		}
		if msg.ID != sub.id {
			t.Errorf("resubscribe ID = %q, want %q", msg.ID, sub.id)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("resubscribe message not received after reconnect")
	}
}

// TestRealtime_PendingMessageBuffer verifies that subscribe messages sent while
// the connection is in the reconnecting state are buffered and delivered once
// the connection re-opens.
func TestRealtime_PendingMessageBuffer(t *testing.T) {
	var connCount int32
	var mu sync.Mutex

	subscribeMsgCh := make(chan wsMessage, 10)

	srv := newTestWSServer(t, func(c *websocket.Conn) {
		mu.Lock()
		n := connCount
		connCount++
		mu.Unlock()

		if n == 0 {
			// First connection: close immediately to trigger reconnect.
			c.Close(websocket.StatusInternalError, "server restart")
			return
		}

		// Second connection: read messages normally.
		ctx := context.Background()
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var msg wsMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			if msg.Type == "subscribe" {
				subscribeMsgCh <- msg
				if !tryWriteWSMessage(t, c, wsMessage{Type: "subscribed", ID: msg.ID}) {
					return
				}
			}
			if msg.Type == "heartbeat" {
				ack, _ := json.Marshal(wsMessage{Type: "heartbeat_ack"})
				if err := c.Write(ctx, websocket.MessageText, ack); err != nil {
					if isConnClosedErr(err) {
						return
					}
				}
			}
		}
	})
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	rt := client.Realtime()
	rt.SetOptions(RealtimeOptions{
		HeartbeatInterval: 10 * time.Second,
		HeartbeatTimeout:  5 * time.Second,
		MaxReconnectDelay: 50 * time.Millisecond,
		AutoReconnect:     true,
	})

	reconnectingCh := make(chan struct{}, 5)
	rt.On("reconnecting", func(data any) {
		select {
		case reconnectingCh <- struct{}{}:
		default:
		}
	})

	connectedCh := make(chan struct{}, 5)
	rt.On("connected", func(data any) {
		select {
		case connectedCh <- struct{}{}:
		default:
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := rt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer rt.Disconnect()

	// Wait for initial connected.
	select {
	case <-connectedCh:
	case <-time.After(2 * time.Second):
		t.Fatal("initial connected event not received")
	}

	// Wait for reconnecting state (first connection dropped).
	select {
	case <-reconnectingCh:
	case <-time.After(3 * time.Second):
		t.Fatal("reconnecting event not received")
	}

	// Subscribe while in reconnecting state - should be buffered.
	sub, err := rt.Subscribe(ctx, "orders", SubscribeOptions{
		Event:   EventAll,
		OnEvent: func(RealtimeEvent) {},
	})
	if err != nil {
		t.Fatalf("Subscribe() during reconnect error: %v", err)
	}

	// Wait for reconnection to complete.
	select {
	case <-connectedCh:
	case <-time.After(5 * time.Second):
		t.Fatal("reconnected event not received")
	}

	// The buffered subscribe message should now be sent. We may see
	// resubscribes from zero existing subs first, but the "orders" subscribe
	// from the buffer should arrive.
	deadline := time.After(3 * time.Second)
	found := false
	for !found {
		select {
		case msg := <-subscribeMsgCh:
			if msg.Table == "orders" && msg.ID == sub.id {
				found = true
			}
		case <-deadline:
			t.Fatal("buffered subscribe message for 'orders' not received after reconnect")
		}
	}
}

// TestRealtime_MaxRetriesExceeded verifies that when max retries is exceeded,
// the client transitions to disconnected state with a "max retries exceeded"
// reason. The server accepts the first connection then rejects all subsequent
// ones at the HTTP level to force dial failures in the reconnect loop.
func TestRealtime_MaxRetriesExceeded(t *testing.T) {
	var connCount int32
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		n := connCount
		connCount++
		mu.Unlock()

		if n == 0 {
			c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
				InsecureSkipVerify: true,
			})
			if err != nil {
				return
			}
			c.Close(websocket.StatusInternalError, "initial drop")
			return
		}

		// Reject all reconnect attempts at HTTP level.
		http.Error(w, "unavailable", http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	rt := client.Realtime()
	rt.SetOptions(RealtimeOptions{
		HeartbeatInterval: 10 * time.Second,
		HeartbeatTimeout:  5 * time.Second,
		MaxReconnectDelay: 20 * time.Millisecond,
		MaxRetries:        2,
		AutoReconnect:     true,
	})

	disconnectedCh := make(chan string, 5)
	rt.On("disconnected", func(data any) {
		if reason, ok := data.(string); ok {
			disconnectedCh <- reason
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := rt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer rt.Disconnect()

	// Wait for max retries exceeded disconnect.
	deadline := time.After(8 * time.Second)
	for {
		select {
		case reason := <-disconnectedCh:
			if reason == "max retries exceeded" {
				// Verify final state.
				if got := rt.State(); got != StateDisconnected {
					t.Errorf("State() = %q, want %q", got, StateDisconnected)
				}
				return
			}
		case <-deadline:
			t.Fatal("did not receive 'max retries exceeded' disconnect within timeout")
		}
	}
}

// TestRealtime_AutoReconnectDisabled verifies that when AutoReconnect is false,
// the client does not attempt to reconnect after a connection drop.
func TestRealtime_AutoReconnectDisabled(t *testing.T) {
	srv := newTestWSServer(t, func(c *websocket.Conn) {
		// Close immediately.
		c.Close(websocket.StatusInternalError, "server error")
	})
	defer srv.Close()

	client := NewClient(srv.URL, Options{
		ProjectRef: "testref",
		APIKey:     "test-key",
	})

	rt := client.Realtime()
	rt.SetOptions(RealtimeOptions{
		HeartbeatInterval: 10 * time.Second,
		HeartbeatTimeout:  5 * time.Second,
		MaxReconnectDelay: 50 * time.Millisecond,
		AutoReconnect:     false,
	})

	reconnectCalled := false
	var rmu sync.Mutex
	rt.On("reconnecting", func(data any) {
		rmu.Lock()
		reconnectCalled = true
		rmu.Unlock()
	})

	disconnectedCh := make(chan string, 5)
	rt.On("disconnected", func(data any) {
		if reason, ok := data.(string); ok {
			disconnectedCh <- reason
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer rt.Disconnect()

	// Should get disconnected without any reconnect attempts.
	select {
	case reason := <-disconnectedCh:
		if !strings.Contains(reason, "read error") {
			t.Logf("disconnected reason = %q (unexpected but not necessarily wrong)", reason)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("disconnected event was not emitted within timeout")
	}

	// Give a moment for any spurious reconnect attempt.
	time.Sleep(200 * time.Millisecond)

	rmu.Lock()
	if reconnectCalled {
		t.Error("reconnecting event should not be emitted when AutoReconnect=false")
	}
	rmu.Unlock()
}
