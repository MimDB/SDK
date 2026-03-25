import { describe, expect, it, vi } from 'vitest'
import { MimDBClient } from '../src/client'
import { QueryBuilder } from '../src/rest'
import { createClient } from '../src/index'
import { mockFetch } from './helpers'

const URL = 'https://api.mimdb.dev'
const REF = 'abc123'
const KEY = 'test-api-key'

describe('createClient', () => {
  it('returns an instance of MimDBClient', () => {
    const client = createClient(URL, REF, KEY, { fetch: mockFetch(200, []) })
    expect(client).toBeInstanceOf(MimDBClient)
  })
})

describe('MimDBClient', () => {
  it('.from() returns a QueryBuilder', () => {
    const client = new MimDBClient(URL, REF, KEY, { fetch: mockFetch(200, []) })
    const builder = client.from('todos')
    expect(builder).toBeInstanceOf(QueryBuilder)
  })

  it('.from().select() calls the correct URL', async () => {
    const fetchFn = mockFetch(200, [{ id: 1 }])
    const client = new MimDBClient(URL, REF, KEY, { fetch: fetchFn })

    await client.from('todos').select('*')

    const call = vi.mocked(fetchFn).mock.calls[0]!
    const url = call[0] as string
    expect(url).toMatch(new RegExp(`^${URL}/v1/rest/${REF}/todos\\?`))
    expect(url).toContain('select=*')
  })

  it('sends API key as both Authorization and apikey headers', async () => {
    const fetchFn = mockFetch(200, [])
    const client = new MimDBClient(URL, REF, KEY, { fetch: fetchFn })

    await client.from('todos').select('*')

    const call = vi.mocked(fetchFn).mock.calls[0]!
    const init = call[1] as RequestInit
    const headers = init.headers as Record<string, string>
    expect(headers['Authorization']).toBe(`Bearer ${KEY}`)
    expect(headers['apikey']).toBe(KEY)
  })

  it('sends custom headers alongside defaults', async () => {
    const fetchFn = mockFetch(200, [])
    const client = new MimDBClient(URL, REF, KEY, {
      fetch: fetchFn,
      headers: { 'X-Custom': 'value' },
    })

    await client.from('todos').select('*')

    const call = vi.mocked(fetchFn).mock.calls[0]!
    const init = call[1] as RequestInit
    const headers = init.headers as Record<string, string>
    expect(headers['X-Custom']).toBe('value')
    expect(headers['Authorization']).toBe(`Bearer ${KEY}`)
  })

  it('strips trailing slash from base URL', async () => {
    const fetchFn = mockFetch(200, [])
    const client = new MimDBClient(`${URL}/`, REF, KEY, { fetch: fetchFn })

    await client.from('todos').select('*')

    const call = vi.mocked(fetchFn).mock.calls[0]!
    const url = call[0] as string
    // Ensure no double slashes after the protocol
    const pathPortion = url.replace('https://', '')
    expect(pathPortion).not.toContain('//')
    expect(url).toMatch(new RegExp(`^${URL}/v1/rest/${REF}/todos`))
  })

  describe('.rpc()', () => {
    it('calls the correct RPC endpoint with POST', async () => {
      const fetchFn = mockFetch(200, { result: 42 })
      const client = new MimDBClient(URL, REF, KEY, { fetch: fetchFn })

      const { data, error } = await client.rpc('my_function', { arg: 'value' })

      expect(error).toBeNull()
      expect(data).toEqual({ result: 42 })

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = call[0] as string
      const init = call[1] as RequestInit
      expect(url).toBe(`${URL}/v1/rest/${REF}/rpc/my_function`)
      expect(init.method).toBe('POST')
      expect(init.body).toBe(JSON.stringify({ arg: 'value' }))
    })

    it('sends empty object when no params provided', async () => {
      const fetchFn = mockFetch(200, 'ok')
      const client = new MimDBClient(URL, REF, KEY, { fetch: fetchFn })

      await client.rpc('ping')

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const init = call[1] as RequestInit
      expect(init.body).toBe(JSON.stringify({}))
    })

    it('returns error for non-OK response', async () => {
      const fetchFn = mockFetch(404, { message: 'function not found', code: 'PGRST202' })
      const client = new MimDBClient(URL, REF, KEY, { fetch: fetchFn })

      const { data, error, status } = await client.rpc('missing_fn')

      expect(data).toBeNull()
      expect(error).not.toBeNull()
      expect(error!.code).toBe('PGRST202')
      expect(status).toBe(404)
    })

    it('handles fetch failure gracefully', async () => {
      const fetchFn = vi.fn().mockRejectedValue(new Error('Offline')) as unknown as typeof fetch
      const client = new MimDBClient(URL, REF, KEY, { fetch: fetchFn })

      const { data, error, status } = await client.rpc('my_function')

      expect(data).toBeNull()
      expect(error).not.toBeNull()
      expect(error!.code).toBe('FETCH_ERROR')
      expect(status).toBe(0)
    })

    it('sends auth headers on RPC calls', async () => {
      const fetchFn = mockFetch(200, null)
      const client = new MimDBClient(URL, REF, KEY, { fetch: fetchFn })

      await client.rpc('my_function')

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const init = call[1] as RequestInit
      const headers = init.headers as Record<string, string>
      expect(headers['Authorization']).toBe(`Bearer ${KEY}`)
      expect(headers['apikey']).toBe(KEY)
    })
  })
})
