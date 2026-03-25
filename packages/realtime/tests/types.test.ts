import { describe, it, expect } from 'vitest'
import type {
  RealtimeEvent,
  RealtimeError,
  SubscribeOptions,
  ConnectionState,
  SubscriptionStatus,
} from '../src/types'

describe('types', () => {
  it('RealtimeEvent accepts INSERT with typed payload', () => {
    interface Npc { id: string; name: string }
    const event: RealtimeEvent<Npc> = {
      type: 'INSERT',
      table: 'npcs',
      new: { id: '1', name: 'Guard' },
      old: null,
    }
    expect(event.new?.name).toBe('Guard')
    expect(event.old).toBeNull()
  })

  it('RealtimeEvent accepts DELETE with string PK old', () => {
    const event: RealtimeEvent = {
      type: 'DELETE',
      table: 'npcs',
      new: null,
      old: { id: '123' },
    }
    expect(event.old?.id).toBe('123')
    expect(event.new).toBeNull()
  })

  it('RealtimeError maps error_code to code', () => {
    const err: RealtimeError = { code: 'RT-0001', message: 'not found' }
    expect(err.code).toBe('RT-0001')
  })

  it('ConnectionState covers all values', () => {
    const states: ConnectionState[] = ['disconnected', 'connecting', 'connected', 'reconnecting']
    expect(states).toHaveLength(4)
  })

  it('SubscriptionStatus covers all values', () => {
    const statuses: SubscriptionStatus[] = ['pending', 'active', 'error', 'closed']
    expect(statuses).toHaveLength(4)
  })
})
