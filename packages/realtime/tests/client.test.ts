import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createMockWSFactory } from './mock-ws'
import { MimDBRealtimeClient } from '../src/client'

/**
 * Helper to create a client with a mock WebSocket factory.
 * Defaults to autoConnect: false so tests control connection timing.
 */
function createClient(overrides = {}) {
  const { factory, getInstance } = createMockWSFactory()
  const client = new MimDBRealtimeClient({
    url: 'https://api.mimdb.dev',
    projectRef: '40891b0d',
    apiKey: 'test-key',
    autoConnect: false,
    WebSocket: factory,
    ...overrides,
  })
  return { client, factory, getInstance }
}

describe('MimDBRealtimeClient', () => {
  // ── Connection tests ───────────────────────────────────────────────

  describe('connection', () => {
    it('constructs correct WSS URL from HTTPS', () => {
      const { client, factory, getInstance } = createClient()
      client.connect()
      expect(getInstance().url).toBe(
        'wss://api.mimdb.dev/v1/realtime/40891b0d?apikey=test-key',
      )
      expect(factory).toHaveBeenCalledOnce()
    })

    it('converts http:// to ws://', () => {
      const { client, getInstance } = createClient({ url: 'http://localhost:9000' })
      client.connect()
      expect(getInstance().url).toBe(
        'ws://localhost:9000/v1/realtime/40891b0d?apikey=test-key',
      )
    })

    it('transitions disconnected -> connecting -> connected', () => {
      const { client, getInstance } = createClient()
      expect(client.state).toBe('disconnected')

      client.connect()
      expect(client.state).toBe('connecting')

      getInstance().simulateOpen()
      expect(client.state).toBe('connected')
    })

    it('emits connected event', () => {
      const { client, getInstance } = createClient()
      const onConnected = vi.fn()
      client.on('connected', onConnected)

      client.connect()
      getInstance().simulateOpen()

      expect(onConnected).toHaveBeenCalledOnce()
    })

    it('on() returns unsubscribe function that removes listener', () => {
      const { client, getInstance } = createClient()
      const onConnected = vi.fn()
      const unsub = client.on('connected', onConnected)

      unsub()

      client.connect()
      getInstance().simulateOpen()
      expect(onConnected).not.toHaveBeenCalled()
    })

    it('disconnect() closes WebSocket and transitions to disconnected', () => {
      const { client, getInstance } = createClient()
      client.connect()
      const ws = getInstance()
      ws.simulateOpen()
      expect(client.state).toBe('connected')

      client.disconnect()
      expect(client.state).toBe('disconnected')
      expect(ws.readyState).toBe(3) // CLOSED
    })

    it('disconnect() emits disconnected event', () => {
      const { client, getInstance } = createClient()
      const cb = vi.fn()
      client.on('disconnected', cb)
      client.connect()
      getInstance().simulateOpen()
      client.disconnect()
      expect(cb).toHaveBeenCalledWith('intentional')
    })

    it('disconnect() is idempotent', () => {
      const { client, getInstance } = createClient()
      client.connect()
      getInstance().simulateOpen()

      client.disconnect()
      expect(client.state).toBe('disconnected')

      // Second disconnect should not throw
      client.disconnect()
      expect(client.state).toBe('disconnected')
    })

    it('client is reusable after disconnect', () => {
      const { client, factory, getInstance } = createClient()
      client.connect()
      getInstance().simulateOpen()
      expect(client.state).toBe('connected')

      client.disconnect()
      expect(client.state).toBe('disconnected')

      client.connect()
      expect(client.state).toBe('connecting')
      getInstance().simulateOpen()
      expect(client.state).toBe('connected')
      expect(factory).toHaveBeenCalledTimes(2)
    })
  })

  // ── Heartbeat tests ────────────────────────────────────────────────

  describe('heartbeat', () => {
    beforeEach(() => vi.useFakeTimers())
    afterEach(() => vi.useRealTimers())

    it('sends heartbeat JSON after heartbeatInterval', () => {
      const { client, getInstance } = createClient({ heartbeatInterval: 5000 })
      client.connect()
      getInstance().simulateOpen()

      vi.advanceTimersByTime(5000)

      const ws = getInstance()
      const heartbeatSent = ws.sent.some(
        (msg) => JSON.parse(msg).type === 'heartbeat',
      )
      expect(heartbeatSent).toBe(true)
    })

    it('triggers reconnect when heartbeat_ack not received within timeout', () => {
      const { client, factory, getInstance } = createClient({
        heartbeatInterval: 5000,
        heartbeatTimeout: 3000,
      })
      client.connect()
      getInstance().simulateOpen()

      // Advance past heartbeat interval to send heartbeat
      vi.advanceTimersByTime(5000)

      // Advance past heartbeat timeout without sending ack
      vi.advanceTimersByTime(3000)

      // The ws.close() was called, which fires onclose, which schedules reconnect
      // Advance past reconnect delay (1000ms for first attempt)
      vi.advanceTimersByTime(1000)

      // A new WebSocket should have been created for reconnect
      expect(factory).toHaveBeenCalledTimes(2)
    })

    it('does NOT reconnect if heartbeat_ack received in time', () => {
      const { client, factory, getInstance } = createClient({
        heartbeatInterval: 5000,
        heartbeatTimeout: 3000,
      })
      client.connect()
      getInstance().simulateOpen()

      // Advance past heartbeat interval to send heartbeat
      vi.advanceTimersByTime(5000)

      // Server sends heartbeat_ack before timeout - clears the ack timer
      getInstance().simulateMessage({ type: 'heartbeat_ack' })

      // Advance past the timeout period - should NOT trigger reconnect
      // because the ack timer was cleared
      vi.advanceTimersByTime(3000)

      // Should still be on the same connection - no reconnect
      expect(factory).toHaveBeenCalledTimes(1)
      expect(client.state).toBe('connected')
    })

    it('clears previous ack timer before sending new heartbeat', () => {
      const { client, getInstance } = createClient({
        heartbeatInterval: 100,
        heartbeatTimeout: 200,
      })
      client.connect()
      getInstance().simulateOpen()

      // First heartbeat at 100ms
      vi.advanceTimersByTime(100)
      // Ack arrives
      getInstance().simulateMessage({ type: 'heartbeat_ack', server_time: '2026-01-01T00:00:00Z' })
      // Second heartbeat at 200ms
      vi.advanceTimersByTime(100)
      // First ack timer would fire at 300ms if not cleared
      vi.advanceTimersByTime(100)
      // Connection should still be alive (old timer was cleared)
      expect(client.state).toBe('connected')
    })
  })

  // ── Reconnect tests ────────────────────────────────────────────────

  describe('reconnect', () => {
    beforeEach(() => vi.useFakeTimers())
    afterEach(() => vi.useRealTimers())

    it('reconnects with exponential backoff on abnormal close', () => {
      const { client, factory, getInstance } = createClient()
      client.connect()
      getInstance().simulateOpen()

      // Abnormal close
      getInstance().simulateClose(1006)
      expect(client.state).toBe('reconnecting')

      // First retry delay: 1000ms
      vi.advanceTimersByTime(1000)
      expect(factory).toHaveBeenCalledTimes(2)

      // Open and close again
      getInstance().simulateOpen()
      getInstance().simulateClose(1006)

      // Second retry delay: 2000ms
      vi.advanceTimersByTime(2000)
      expect(factory).toHaveBeenCalledTimes(3)
    })

    it('emits reconnecting event with attempt number', () => {
      const { client, getInstance } = createClient()
      const onReconnecting = vi.fn()
      client.on('reconnecting', onReconnecting)

      client.connect()
      getInstance().simulateOpen()

      // First abnormal close -> attempt 1
      getInstance().simulateClose(1006)
      expect(onReconnecting).toHaveBeenCalledWith(1)

      // Advance past first retry delay (1000ms), factory creates new WS
      vi.advanceTimersByTime(1000)

      // Close without opening - retryCount not reset, so next is attempt 2
      getInstance().simulateClose(1006)
      expect(onReconnecting).toHaveBeenCalledWith(2)
    })

    it('connect() during reconnect does not create duplicate sockets', () => {
      const { client, factory, getInstance } = createClient()
      client.connect()
      getInstance().simulateOpen()

      // Abnormal close -> reconnecting
      getInstance().simulateClose(1006)
      expect(client.state).toBe('reconnecting')

      // Calling connect() while reconnecting should be a no-op
      client.connect()

      // Only the original connect + the reconnect timer should create sockets
      vi.advanceTimersByTime(1000)
      expect(factory).toHaveBeenCalledTimes(2) // not 3
    })

    it('does NOT reconnect after intentional disconnect()', () => {
      const { client, factory, getInstance } = createClient()
      client.connect()
      getInstance().simulateOpen()

      client.disconnect()

      // Advance plenty of time - no reconnect should happen
      vi.advanceTimersByTime(60_000)
      expect(factory).toHaveBeenCalledTimes(1)
      expect(client.state).toBe('disconnected')
    })

    it('stops after maxRetries and emits disconnected with reason', () => {
      const { client, factory, getInstance } = createClient({ maxRetries: 2 })
      const onDisconnected = vi.fn()
      client.on('disconnected', onDisconnected)

      client.connect()
      getInstance().simulateOpen()

      // Abnormal close -> retry 1 (retryCount = 1)
      getInstance().simulateClose(1006)
      expect(client.state).toBe('reconnecting')
      vi.advanceTimersByTime(1000) // factory call #2
      expect(factory).toHaveBeenCalledTimes(2)

      // Close without open -> retry 2 (retryCount = 2)
      getInstance().simulateClose(1006)
      expect(client.state).toBe('reconnecting')
      vi.advanceTimersByTime(2000) // factory call #3
      expect(factory).toHaveBeenCalledTimes(3)

      // Close without open -> retry 3 exceeds maxRetries (2)
      getInstance().simulateClose(1006)

      expect(client.state).toBe('disconnected')
      expect(onDisconnected).toHaveBeenCalledWith('max retries exceeded')
      // No additional WebSocket created beyond the 3 already made
      expect(factory).toHaveBeenCalledTimes(3)
    })
  })

  // ── Subscribe tests ────────────────────────────────────────────────

  describe('subscribe', () => {
    beforeEach(() => vi.useFakeTimers())
    afterEach(() => vi.useRealTimers())

    it('auto-connects on first subscribe when autoConnect: true', () => {
      const { client, factory, getInstance } = createClient({ autoConnect: true })
      expect(client.state).toBe('disconnected')

      client.subscribe('users', { onEvent: vi.fn() })
      expect(client.state).toBe('connecting')
      expect(factory).toHaveBeenCalledOnce()
    })

    it('sends subscribe message after connection opens', () => {
      const { client, getInstance } = createClient()
      client.connect()
      getInstance().simulateOpen()

      client.subscribe('users', { onEvent: vi.fn(), event: 'INSERT' })

      const ws = getInstance()
      const subMsg = ws.sent.find((msg) => JSON.parse(msg).type === 'subscribe')
      expect(subMsg).toBeDefined()
      const parsed = JSON.parse(subMsg!)
      expect(parsed).toMatchObject({
        type: 'subscribe',
        table: 'users',
        event: 'INSERT',
      })
    })

    it('queues subscribe messages sent before socket opens, flushes on open', () => {
      const { client, getInstance } = createClient()
      client.connect()
      // Socket is still connecting (not open yet)

      client.subscribe('users', { onEvent: vi.fn() })
      client.subscribe('posts', { onEvent: vi.fn() })

      // No messages sent yet since socket is not open
      const ws = getInstance()
      expect(ws.sent).toHaveLength(0)

      // Open the socket - resubscribeAll sends subscribe messages
      ws.simulateOpen()

      const subscribeMsgs = ws.sent.filter(
        (msg: string) => JSON.parse(msg).type === 'subscribe',
      )
      expect(subscribeMsgs).toHaveLength(2)

      const tables = subscribeMsgs.map((msg: string) => JSON.parse(msg).table)
      expect(tables).toContain('users')
      expect(tables).toContain('posts')
    })

    it('routes events to correct subscription by ID', () => {
      const { client, getInstance } = createClient()
      client.connect()
      getInstance().simulateOpen()

      const onEventA = vi.fn()
      const onEventB = vi.fn()
      client.subscribe('users', { onEvent: onEventA })
      client.subscribe('posts', { onEvent: onEventB })

      // Route an event to sub-1 (users)
      getInstance().simulateMessage({
        type: 'event',
        id: 'sub-1',
        event: 'INSERT',
        table: 'users',
        new: { id: '1', name: 'Alice' },
        old: null,
      })

      expect(onEventA).toHaveBeenCalledOnce()
      expect(onEventB).not.toHaveBeenCalled()
      expect(onEventA).toHaveBeenCalledWith({
        type: 'INSERT',
        table: 'users',
        new: { id: '1', name: 'Alice' },
        old: null,
      })
    })

    it('handles subscribed confirmation from server', () => {
      const { client, getInstance } = createClient()
      client.connect()
      getInstance().simulateOpen()

      const onSubscribed = vi.fn()
      const handle = client.subscribe('users', {
        onEvent: vi.fn(),
        onSubscribed,
      })
      expect(handle.status).toBe('pending')

      getInstance().simulateMessage({ type: 'subscribed', id: handle.id })

      expect(handle.status).toBe('active')
      expect(onSubscribed).toHaveBeenCalledOnce()
    })

    it('handles server error for a subscription', () => {
      const { client, getInstance } = createClient()
      client.connect()
      getInstance().simulateOpen()

      const onError = vi.fn()
      const handle = client.subscribe('users', {
        onEvent: vi.fn(),
        onError,
      })

      getInstance().simulateMessage({
        type: 'error',
        id: handle.id,
        error_code: 'RT-0001',
        message: 'table not found',
      })

      expect(handle.status).toBe('error')
      expect(onError).toHaveBeenCalledWith({
        code: 'RT-0001',
        message: 'table not found',
      })
    })

    it('resubscribes all active subscriptions after reconnect', () => {
      const { client, factory, getInstance } = createClient()
      client.connect()
      getInstance().simulateOpen()

      const onEventA = vi.fn()
      const onEventB = vi.fn()
      client.subscribe('users', { onEvent: onEventA })
      client.subscribe('posts', { onEvent: onEventB })

      // Confirm both subscribed
      getInstance().simulateMessage({ type: 'subscribed', id: 'sub-1' })
      getInstance().simulateMessage({ type: 'subscribed', id: 'sub-2' })

      // Simulate abnormal close -> reconnect
      getInstance().simulateClose(1006)
      vi.advanceTimersByTime(1000)

      // A new WS was created
      expect(factory).toHaveBeenCalledTimes(2)
      const newWs = getInstance()
      newWs.simulateOpen()

      // The new WS should have resubscribe messages
      const subscribeMsgs = newWs.sent.filter(
        (msg) => JSON.parse(msg).type === 'subscribe',
      )
      expect(subscribeMsgs).toHaveLength(2)

      const tables = subscribeMsgs.map((msg) => JSON.parse(msg).table)
      expect(tables).toContain('users')
      expect(tables).toContain('posts')
    })

    it('does not resubscribe errored subscriptions after reconnect', () => {
      const { client, factory, getInstance } = createClient()
      client.connect()
      getInstance().simulateOpen()

      const sub = client.subscribe('users', { onEvent: vi.fn(), onError: vi.fn() })

      // Error the subscription
      getInstance().simulateMessage({
        type: 'error',
        error_code: 'RT-0001',
        message: 'fail',
        id: 'sub-1',
      })
      expect(sub.status).toBe('error')

      // Reconnect
      getInstance().simulateClose(1006)
      vi.advanceTimersByTime(1000)
      getInstance().simulateOpen()

      // Should NOT have resubscribed the errored sub
      const sent = getInstance().sent.map((s: string) => JSON.parse(s))
      const subscribes = sent.filter((m: Record<string, unknown>) => m.type === 'subscribe')
      expect(subscribes).toHaveLength(0)
    })

    it('unsubscribed subscription ignores further events', () => {
      const { client, getInstance } = createClient()
      client.connect()
      getInstance().simulateOpen()

      const onEvent = vi.fn()
      const handle = client.subscribe('users', { onEvent })

      getInstance().simulateMessage({ type: 'subscribed', id: handle.id })
      handle.unsubscribe()

      // Send an event after unsubscribe - should be ignored
      getInstance().simulateMessage({
        type: 'event',
        id: handle.id,
        event: 'INSERT',
        table: 'users',
        new: { id: '1' },
        old: null,
      })

      expect(onEvent).not.toHaveBeenCalled()
    })
  })
})
