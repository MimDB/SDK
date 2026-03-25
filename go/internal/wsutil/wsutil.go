// Package wsutil provides shared utilities for the MimDB Realtime WebSocket
// client. It contains two focused components:
//
//   - Exponential backoff delay calculation for reconnection attempts.
//   - A thread-safe message buffer for queuing outbound messages while the
//     WebSocket connection is being established.
package wsutil

import (
	"sync"
	"time"
)

// BackoffDelay calculates the delay before a reconnection attempt using
// exponential backoff with a ceiling.
//
// The formula is: min(baseDelay * 2^(attempt-1), maxDelay)
//
// Attempt numbering starts at 1. Values of 0 or below are clamped to 1,
// returning baseDelay as the minimum possible delay.
//
//	BackoffDelay(1, 1*time.Second, 30*time.Second) // 1s
//	BackoffDelay(3, 1*time.Second, 30*time.Second) // 4s
//	BackoffDelay(6, 1*time.Second, 30*time.Second) // 30s (capped)
func BackoffDelay(attempt int, baseDelay, maxDelay time.Duration) time.Duration {
	if attempt < 1 {
		attempt = 1
	}

	delay := baseDelay
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay >= maxDelay {
			return maxDelay
		}
	}

	return delay
}

// MessageBuffer is a thread-safe FIFO queue for raw WebSocket messages. It is
// used to hold outbound messages while the connection is in the "connecting"
// state. Once the connection opens, the caller flushes the buffer and sends
// each message over the wire.
type MessageBuffer struct {
	mu       sync.Mutex
	messages [][]byte
}

// Enqueue appends a message to the end of the buffer. It is safe for
// concurrent use.
func (b *MessageBuffer) Enqueue(msg []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.messages = append(b.messages, msg)
}

// Flush returns all buffered messages in FIFO order and resets the buffer to
// empty. The returned slice is owned by the caller. If the buffer is empty,
// nil is returned.
func (b *MessageBuffer) Flush() [][]byte {
	b.mu.Lock()
	defer b.mu.Unlock()

	msgs := b.messages
	b.messages = nil
	return msgs
}

// Len returns the number of messages currently in the buffer. It is safe for
// concurrent use.
func (b *MessageBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	return len(b.messages)
}
