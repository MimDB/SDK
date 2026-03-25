import { vi } from 'vitest'

/**
 * Minimal mock WebSocket for unit testing.
 * Simulates server responses without a real connection.
 */
export class MockWebSocket {
  static CONNECTING = 0 as const
  static OPEN = 1 as const
  static CLOSING = 2 as const
  static CLOSED = 3 as const

  readyState = MockWebSocket.CONNECTING
  url: string

  onopen: ((ev: Event) => void) | null = null
  onclose: ((ev: CloseEvent) => void) | null = null
  onmessage: ((ev: MessageEvent) => void) | null = null
  onerror: ((ev: Event) => void) | null = null

  readonly sent: string[] = []

  constructor(url: string | URL) {
    this.url = typeof url === 'string' ? url : url.toString()
  }

  /** Simulate the connection opening. Call after construction. */
  simulateOpen(): void {
    this.readyState = MockWebSocket.OPEN
    this.onopen?.(new Event('open'))
  }

  /** Simulate receiving a message from the server. */
  simulateMessage(data: Record<string, unknown>): void {
    this.onmessage?.(new MessageEvent('message', { data: JSON.stringify(data) }))
  }

  /** Simulate the connection closing. */
  simulateClose(code = 1000, reason = ''): void {
    this.readyState = MockWebSocket.CLOSED
    this.onclose?.({ code, reason, wasClean: code === 1000 } as CloseEvent)
  }

  /** Simulate a connection error. */
  simulateError(): void {
    this.onerror?.(new Event('error'))
  }

  send(data: string): void {
    this.sent.push(data)
  }

  close(code = 1000, reason = ''): void {
    if (this.readyState === MockWebSocket.CLOSED) return
    this.readyState = MockWebSocket.CLOSED
    this.onclose?.({ code, reason, wasClean: code === 1000 } as CloseEvent)
  }
}

/**
 * Creates a MockWebSocket factory that captures the last instance.
 * Use: `const { factory, getInstance } = createMockWSFactory()`
 */
export function createMockWSFactory() {
  let instance: MockWebSocket | null = null
  const factory = vi.fn(function (this: unknown, url: string | URL) {
    instance = new MockWebSocket(url)
    return instance
  })
  return {
    factory,
    getInstance: () => instance!,
  }
}
