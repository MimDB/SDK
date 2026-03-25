import { describe, it, expect, vi } from 'vitest'
import { Subscription } from '../src/subscription'
import type { SubscribeOptions } from '../src/types'

function makeOptions(overrides: Partial<SubscribeOptions> = {}): SubscribeOptions {
  return { onEvent: vi.fn(), ...overrides }
}

describe('Subscription', () => {
  it('starts in pending status', () => {
    const sub = new Subscription('sub-1', 'users', makeOptions(), vi.fn())
    expect(sub.status).toBe('pending')
    expect(sub.id).toBe('sub-1')
    expect(sub.table).toBe('users')
  })

  it('builds correct subscribe message', () => {
    const sub = new Subscription('sub-1', 'users', makeOptions({ event: 'INSERT' }), vi.fn())
    const msg = sub.buildSubscribeMessage()
    expect(msg).toEqual({
      type: 'subscribe',
      id: 'sub-1',
      table: 'users',
      event: 'INSERT',
    })
  })

  it('includes filter in subscribe message when provided', () => {
    const sub = new Subscription('sub-1', 'users', makeOptions({ filter: 'id=eq.123' }), vi.fn())
    const msg = sub.buildSubscribeMessage()
    expect(msg.filter).toBe('id=eq.123')
  })

  it('defaults event to * when not specified', () => {
    const sub = new Subscription('sub-1', 'users', makeOptions(), vi.fn())
    const msg = sub.buildSubscribeMessage()
    expect(msg.event).toBe('*')
  })

  it('transitions to active on handleSubscribed', () => {
    const onSubscribed = vi.fn()
    const sub = new Subscription('sub-1', 'users', makeOptions({ onSubscribed }), vi.fn())
    sub.handleSubscribed()
    expect(sub.status).toBe('active')
    expect(onSubscribed).toHaveBeenCalledOnce()
  })

  it('dispatches events via onEvent callback', () => {
    const onEvent = vi.fn()
    const sub = new Subscription('sub-1', 'users', makeOptions({ onEvent }), vi.fn())
    sub.handleSubscribed()
    sub.handleEvent({
      type: 'event', id: 'sub-1', event: 'INSERT', table: 'users',
      new: { id: '1', name: 'Test' }, old: null,
    })
    expect(onEvent).toHaveBeenCalledWith({
      type: 'INSERT', table: 'users',
      new: { id: '1', name: 'Test' }, old: null,
    })
  })

  it('dispatches errors via onError callback', () => {
    const onError = vi.fn()
    const sub = new Subscription('sub-1', 'users', makeOptions({ onError }), vi.fn())
    sub.handleError({ type: 'error', error_code: 'RT-0001', message: 'fail', id: 'sub-1' })
    expect(sub.status).toBe('error')
    expect(onError).toHaveBeenCalledWith({ code: 'RT-0001', message: 'fail' })
  })

  it('sends unsubscribe message and transitions to closed', () => {
    const sendFn = vi.fn()
    const sub = new Subscription('sub-1', 'users', makeOptions(), sendFn)
    sub.handleSubscribed()
    sub.unsubscribe()
    expect(sub.status).toBe('closed')
    expect(sendFn).toHaveBeenCalledWith(JSON.stringify({ type: 'unsubscribe', id: 'sub-1' }))
  })

  it('does not dispatch events after unsubscribe', () => {
    const onEvent = vi.fn()
    const sub = new Subscription('sub-1', 'users', makeOptions({ onEvent }), vi.fn())
    sub.handleSubscribed()
    sub.unsubscribe()
    sub.handleEvent({
      type: 'event', id: 'sub-1', event: 'INSERT', table: 'users',
      new: { id: '1' }, old: null,
    })
    expect(onEvent).not.toHaveBeenCalled()
  })

  it('unsubscribe is idempotent', () => {
    const sendFn = vi.fn()
    const sub = new Subscription('sub-1', 'users', makeOptions(), sendFn)
    sub.handleSubscribed()
    sub.unsubscribe()
    sub.unsubscribe()
    expect(sendFn).toHaveBeenCalledTimes(1)
  })
})
