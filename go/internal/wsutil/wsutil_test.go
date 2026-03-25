package wsutil

import (
	"sync"
	"testing"
	"time"
)

// TestBackoffDelay verifies the exponential backoff formula using a
// table-driven approach. Formula: min(baseDelay * 2^(attempt-1), maxDelay).
func TestBackoffDelay(t *testing.T) {
	tests := []struct {
		name     string
		attempt  int
		base     time.Duration
		max      time.Duration
		expected time.Duration
	}{
		{name: "attempt 1", attempt: 1, base: time.Second, max: 30 * time.Second, expected: time.Second},
		{name: "attempt 2", attempt: 2, base: time.Second, max: 30 * time.Second, expected: 2 * time.Second},
		{name: "attempt 3", attempt: 3, base: time.Second, max: 30 * time.Second, expected: 4 * time.Second},
		{name: "attempt 4", attempt: 4, base: time.Second, max: 30 * time.Second, expected: 8 * time.Second},
		{name: "attempt 5", attempt: 5, base: time.Second, max: 30 * time.Second, expected: 16 * time.Second},
		{name: "attempt 6 capped", attempt: 6, base: time.Second, max: 30 * time.Second, expected: 30 * time.Second},
		{name: "attempt 7 still capped", attempt: 7, base: time.Second, max: 30 * time.Second, expected: 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BackoffDelay(tt.attempt, tt.base, tt.max)
			if got != tt.expected {
				t.Errorf("BackoffDelay(%d, %v, %v) = %v, want %v",
					tt.attempt, tt.base, tt.max, got, tt.expected)
			}
		})
	}
}

// TestBackoffDelay_ZeroAttempt verifies that attempt=0 is treated as a
// minimum-effort reconnect and returns baseDelay.
func TestBackoffDelay_ZeroAttempt(t *testing.T) {
	got := BackoffDelay(0, time.Second, 30*time.Second)
	if got != time.Second {
		t.Errorf("BackoffDelay(0, 1s, 30s) = %v, want %v", got, time.Second)
	}
}

// TestMessageBuffer_EnqueueFlush verifies that enqueued messages are returned
// in FIFO order and the buffer is empty after flushing.
func TestMessageBuffer_EnqueueFlush(t *testing.T) {
	var buf MessageBuffer

	buf.Enqueue([]byte("msg1"))
	buf.Enqueue([]byte("msg2"))
	buf.Enqueue([]byte("msg3"))

	msgs := buf.Flush()
	if len(msgs) != 3 {
		t.Fatalf("Flush() returned %d messages, want 3", len(msgs))
	}
	if string(msgs[0]) != "msg1" {
		t.Errorf("msgs[0] = %q, want %q", string(msgs[0]), "msg1")
	}
	if string(msgs[1]) != "msg2" {
		t.Errorf("msgs[1] = %q, want %q", string(msgs[1]), "msg2")
	}
	if string(msgs[2]) != "msg3" {
		t.Errorf("msgs[2] = %q, want %q", string(msgs[2]), "msg3")
	}

	// Buffer should be empty after flush.
	remaining := buf.Flush()
	if len(remaining) != 0 {
		t.Errorf("Flush() after flush returned %d messages, want 0", len(remaining))
	}
}

// TestMessageBuffer_FlushEmpty verifies that flushing an empty buffer returns
// nil or an empty slice without panicking.
func TestMessageBuffer_FlushEmpty(t *testing.T) {
	var buf MessageBuffer

	msgs := buf.Flush()
	if len(msgs) != 0 {
		t.Errorf("Flush() on empty buffer returned %d messages, want 0", len(msgs))
	}
}

// TestMessageBuffer_Len verifies the buffered message count.
func TestMessageBuffer_Len(t *testing.T) {
	var buf MessageBuffer

	if buf.Len() != 0 {
		t.Errorf("Len() on empty buffer = %d, want 0", buf.Len())
	}

	buf.Enqueue([]byte("a"))
	buf.Enqueue([]byte("b"))

	if buf.Len() != 2 {
		t.Errorf("Len() after 2 enqueues = %d, want 2", buf.Len())
	}

	buf.Flush()

	if buf.Len() != 0 {
		t.Errorf("Len() after flush = %d, want 0", buf.Len())
	}
}

// TestMessageBuffer_ConcurrentAccess verifies that the buffer is safe for
// concurrent use by multiple goroutines. The test enqueues messages from many
// goroutines simultaneously and checks that nothing is lost or corrupted.
func TestMessageBuffer_ConcurrentAccess(t *testing.T) {
	var buf MessageBuffer

	const goroutines = 50
	const msgsPerGoroutine = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for j := range msgsPerGoroutine {
				buf.Enqueue([]byte{byte(id), byte(j)})
			}
		}(i)
	}

	wg.Wait()

	msgs := buf.Flush()
	expected := goroutines * msgsPerGoroutine
	if len(msgs) != expected {
		t.Errorf("Flush() returned %d messages, want %d", len(msgs), expected)
	}

	// Buffer should be empty after flush.
	if buf.Len() != 0 {
		t.Errorf("Len() after flush = %d, want 0", buf.Len())
	}
}
