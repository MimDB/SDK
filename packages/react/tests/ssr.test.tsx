import { describe, it, expect, vi } from 'vitest'
import { createServerClient } from '@mimdb/client'

// Mock @mimdb/realtime to avoid real WebSocket creation
vi.mock('@mimdb/realtime', () => {
  class MockRealtimeClient {
    subscribe = vi.fn().mockReturnValue({
      id: 'sub-1',
      table: 'test',
      status: 'pending' as const,
      unsubscribe: vi.fn(),
    })
    disconnect = vi.fn()
  }
  return { MimDBRealtimeClient: MockRealtimeClient }
})

describe('SSR support', () => {
  it('createServerClient creates a client with in-memory store', () => {
    const client = createServerClient('https://api.test', 'ref', 'service-key')
    expect(client).toBeDefined()
    expect(client.auth).toBeDefined()
    // Should have no session stored
    expect(client.auth.getSession()).toBeNull()
  })

  it('createServerClient disables auto-refresh', async () => {
    vi.useFakeTimers()

    // Build a JWT that expires in 60s
    const nowSec = Math.floor(Date.now() / 1000)
    const payload = btoa(JSON.stringify({ sub: 'user-1', exp: nowSec + 60 }))
    const accessToken = `${btoa('{"alg":"HS256"}')}.${payload}.sig`

    // Create a fetch that returns a sign-in response with our JWT
    const mockFetch = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({
        data: {
          user: { id: '1', email: 'a@b.com', email_confirmed: true, phone_confirmed: false, app_metadata: {}, user_metadata: {}, created_at: '', updated_at: '' },
          access_token: accessToken,
          refresh_token: 'refresh-1',
          expires_in: 60,
        },
        error: null,
        meta: { request_id: 'test' },
      }), {
        status: 200,
        statusText: 'OK',
        headers: { 'content-type': 'application/json' },
      }),
    ) as unknown as typeof fetch

    const client = createServerClient('https://api.test', 'ref', 'service-key', {
      fetch: mockFetch,
    })

    await client.auth.signIn('a@b.com', 'password')

    // Clear mock to track any refresh calls
    mockFetch.mockClear()

    // Advance past when auto-refresh would fire
    await vi.advanceTimersByTimeAsync(120_000)

    // No refresh calls should have been made
    expect(mockFetch).not.toHaveBeenCalled()

    vi.useRealTimers()
  })

  it('createServerClient accepts custom fetch', async () => {
    const customFetch = vi.fn().mockResolvedValue(
      new Response(JSON.stringify([{ id: 1 }]), {
        status: 200,
        statusText: 'OK',
        headers: { 'content-type': 'application/json' },
      }),
    ) as unknown as typeof fetch

    const client = createServerClient('https://api.test', 'ref', 'service-key', {
      fetch: customFetch,
    })

    await client.from('todos').select('*')

    expect(customFetch).toHaveBeenCalled()
  })
})
