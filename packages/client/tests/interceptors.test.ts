import { describe, expect, it, vi } from 'vitest'
import { MimDBClient } from '../src/client'

const URL = 'https://api.mimdb.dev'
const REF = 'abc123'
const KEY = 'test-api-key'

/**
 * Create a mock fetch that tracks calls and returns a fixed response.
 */
function createMockFetch(status: number, body: unknown): typeof fetch {
  return vi.fn().mockResolvedValue(
    new Response(JSON.stringify(body), {
      status,
      statusText: status === 200 ? 'OK' : 'Error',
      headers: { 'content-type': 'application/json' },
    }),
  ) as unknown as typeof fetch
}

describe('Request interceptors', () => {
  it('onRequest modifies headers before the fetch call', async () => {
    const fetchFn = createMockFetch(200, [{ id: 1 }])

    const client = new MimDBClient(URL, REF, KEY, {
      fetch: fetchFn,
      onRequest: (_url, init) => ({
        ...init,
        headers: {
          ...(init.headers as Record<string, string>),
          'X-Custom-Header': 'injected-value',
        },
      }),
    })

    await client.from('todos').select('*')

    const call = vi.mocked(fetchFn).mock.calls[0]!
    const init = call[1] as RequestInit
    const headers = init.headers as Record<string, string>
    expect(headers['X-Custom-Header']).toBe('injected-value')
  })

  it('onRequest works with rpc calls', async () => {
    const fetchFn = createMockFetch(200, { result: 42 })

    const client = new MimDBClient(URL, REF, KEY, {
      fetch: fetchFn,
      onRequest: (_url, init) => ({
        ...init,
        headers: {
          ...(init.headers as Record<string, string>),
          'X-Trace-Id': 'trace-123',
        },
      }),
    })

    await client.rpc('my_function', { arg: 'value' })

    const call = vi.mocked(fetchFn).mock.calls[0]!
    const init = call[1] as RequestInit
    const headers = init.headers as Record<string, string>
    expect(headers['X-Trace-Id']).toBe('trace-123')
  })

  it('onRequest supports async interceptors', async () => {
    const fetchFn = createMockFetch(200, [])

    const client = new MimDBClient(URL, REF, KEY, {
      fetch: fetchFn,
      onRequest: async (_url, init) => {
        // Simulate async token lookup
        await Promise.resolve()
        return {
          ...init,
          headers: {
            ...(init.headers as Record<string, string>),
            'X-Async': 'true',
          },
        }
      },
    })

    await client.from('todos').select('*')

    const call = vi.mocked(fetchFn).mock.calls[0]!
    const headers = (call[1] as RequestInit).headers as Record<string, string>
    expect(headers['X-Async']).toBe('true')
  })
})

describe('Response interceptors', () => {
  it('onResponse can inspect and pass through the response', async () => {
    const fetchFn = createMockFetch(200, [{ id: 1, task: 'hello' }])
    const intercepted: number[] = []

    const client = new MimDBClient(URL, REF, KEY, {
      fetch: fetchFn,
      onResponse: (response) => {
        intercepted.push(response.status)
        return response
      },
    })

    const { data } = await client.from('todos').select('*')

    expect(intercepted).toEqual([200])
    expect(data).toEqual([{ id: 1, task: 'hello' }])
  })

  it('onResponse works with rpc calls', async () => {
    const fetchFn = createMockFetch(200, { result: 42 })
    const intercepted: string[] = []

    const client = new MimDBClient(URL, REF, KEY, {
      fetch: fetchFn,
      onResponse: (response) => {
        intercepted.push('intercepted')
        return response
      },
    })

    await client.rpc('my_function')

    expect(intercepted).toEqual(['intercepted'])
  })
})
