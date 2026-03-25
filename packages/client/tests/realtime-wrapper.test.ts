import { describe, expect, it, vi } from 'vitest'
import { MimDBClient } from '../src/client'
import { mockFetch } from './helpers'

// Mock the @mimdb/realtime module
vi.mock('@mimdb/realtime', () => {
  class MockRealtimeClient {
    options: unknown
    constructor(opts: unknown) {
      this.options = opts
    }
    subscribe = vi.fn()
    disconnect = vi.fn()
  }
  return { MimDBRealtimeClient: MockRealtimeClient }
})

const URL = 'https://api.mimdb.dev'
const REF = 'abc123'
const KEY = 'test-api-key'

describe('MimDBClient.realtime', () => {
  it('returns a MimDBRealtimeClient instance', () => {
    const client = new MimDBClient(URL, REF, KEY, { fetch: mockFetch(200, []) })
    const rt = client.realtime
    expect(rt).toBeDefined()
    expect(rt.subscribe).toBeDefined()
  })

  it('lazily creates the realtime client (same instance on repeated access)', () => {
    const client = new MimDBClient(URL, REF, KEY, { fetch: mockFetch(200, []) })
    const rt1 = client.realtime
    const rt2 = client.realtime
    expect(rt1).toBe(rt2)
  })

  it('forwards url, projectRef, and apiKey to MimDBRealtimeClient', () => {
    const client = new MimDBClient(URL, REF, KEY, { fetch: mockFetch(200, []) })
    const rt = client.realtime as unknown as { options: Record<string, unknown> }
    expect(rt.options).toEqual({
      url: URL,
      projectRef: REF,
      apiKey: KEY,
    })
  })
})
