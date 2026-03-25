import { describe, expect, it, vi } from 'vitest'
import { QueryBuilder } from '../src/rest'
import { mockFetch } from './helpers'

const BASE_URL = 'https://api.mimdb.dev/v1/rest/abc123'
const DEFAULT_HEADERS = {
  'Content-Type': 'application/json',
  'Authorization': 'Bearer test-key',
  'apikey': 'test-key',
}

function createBuilder<T = Record<string, unknown>>(
  fetchFn: typeof fetch = mockFetch(200, []),
  table: string = 'todos',
): QueryBuilder<T> {
  return new QueryBuilder<T>(BASE_URL, table, fetchFn, { ...DEFAULT_HEADERS })
}

describe('QueryBuilder', () => {
  describe('SELECT', () => {
    it('builds correct URL with select param', async () => {
      const fetchFn = mockFetch(200, [{ id: 1, task: 'test' }])
      const builder = createBuilder(fetchFn)

      const { data, error } = await builder.select('id, task')

      expect(error).toBeNull()
      expect(data).toEqual([{ id: 1, task: 'test' }])

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = call[0] as string
      expect(url).toContain(`${BASE_URL}/todos`)
      expect(url).toContain('select=id%2C+task')
    })

    it('defaults to * when no columns specified', async () => {
      const fetchFn = mockFetch(200, [])
      const builder = createBuilder(fetchFn)

      await builder.select()

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = call[0] as string
      expect(url).toContain('select=*')
    })

    it('applies filters, order, limit, and offset', async () => {
      const fetchFn = mockFetch(200, [])
      const builder = createBuilder(fetchFn)

      await builder
        .select('id, task')
        .eq('done', 'false')
        .order('created_at', { ascending: false })
        .limit(10)
        .offset(20)

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = new URL(call[0] as string)
      expect(url.searchParams.get('select')).toBe('id, task')
      expect(url.searchParams.get('done')).toBe('eq.false')
      expect(url.searchParams.get('order')).toBe('created_at.desc')
      expect(url.searchParams.get('limit')).toBe('10')
      expect(url.searchParams.get('offset')).toBe('20')
    })

    it('sends GET request', async () => {
      const fetchFn = mockFetch(200, [])
      const builder = createBuilder(fetchFn)

      await builder.select('*')

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const init = call[1] as RequestInit
      expect(init.method).toBe('GET')
    })
  })

  describe('INSERT', () => {
    it('sends POST with JSON body', async () => {
      const fetchFn = mockFetch(201, { id: 1, task: 'New' })
      const builder = createBuilder(fetchFn)

      await builder.insert({ task: 'New', done: false }).select()

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const init = call[1] as RequestInit
      expect(init.method).toBe('POST')
      expect(init.body).toBe(JSON.stringify({ task: 'New', done: false }))
    })

    it('adds return=representation when select is chained', async () => {
      const fetchFn = mockFetch(201, { id: 1 })
      const builder = createBuilder(fetchFn)

      await builder.insert({ task: 'New' }).select()

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const init = call[1] as RequestInit
      const headers = init.headers as Record<string, string>
      expect(headers['Prefer']).toContain('return=representation')
    })
  })

  describe('UPDATE', () => {
    it('sends PATCH with JSON body and filters', async () => {
      const fetchFn = mockFetch(200, { id: 42, done: true })
      const builder = createBuilder(fetchFn)

      await builder.update({ done: true }).eq('id', '42').select()

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = new URL(call[0] as string)
      const init = call[1] as RequestInit
      expect(init.method).toBe('PATCH')
      expect(init.body).toBe(JSON.stringify({ done: true }))
      expect(url.searchParams.get('id')).toBe('eq.42')
    })
  })

  describe('DELETE', () => {
    it('sends DELETE with filters', async () => {
      const fetchFn = mockFetch(200, null)
      const builder = createBuilder(fetchFn)

      await builder.delete().eq('id', '42')

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = new URL(call[0] as string)
      const init = call[1] as RequestInit
      expect(init.method).toBe('DELETE')
      expect(url.searchParams.get('id')).toBe('eq.42')
    })
  })

  describe('UPSERT', () => {
    it('sends POST with merge-duplicates prefer header', async () => {
      const fetchFn = mockFetch(200, { id: 1, task: 'Updated' })
      const builder = createBuilder(fetchFn)

      await builder.upsert({ id: 1, task: 'Updated', done: true }).select()

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const init = call[1] as RequestInit
      expect(init.method).toBe('POST')
      const headers = init.headers as Record<string, string>
      expect(headers['Prefer']).toContain('resolution=merge-duplicates')
      expect(headers['Prefer']).toContain('return=representation')
    })
  })

  describe('Modifiers', () => {
    it('single() adds correct Accept header', async () => {
      const fetchFn = mockFetch(200, { id: 1 })
      const builder = createBuilder(fetchFn)

      await builder.select('*').eq('id', '1').single()

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const init = call[1] as RequestInit
      const headers = init.headers as Record<string, string>
      expect(headers['Accept']).toBe('application/vnd.pgrst.object+json')
    })

    it('maybeSingle() returns null data on 406 with zero-row message', async () => {
      const fetchFn = mockFetch(406, { message: 'JSON object requested, multiple (or no) rows returned. The result contains 0 rows', code: 'PGRST116' })
      const builder = createBuilder(fetchFn)

      const { data, error } = await builder.select('*').maybeSingle()

      expect(data).toBeNull()
      expect(error).toBeNull()
    })

    it('maybeSingle() returns error on 406 with multiple-row message', async () => {
      const fetchFn = mockFetch(406, { message: 'JSON object requested, multiple (or no) rows returned. The result contains 3 rows', code: 'PGRST116' })
      const builder = createBuilder(fetchFn)

      const { data, error } = await builder.select('*').maybeSingle()

      expect(data).toBeNull()
      expect(error).not.toBeNull()
      expect(error!.status).toBe(406)
    })

    it('count() adds correct Prefer header', async () => {
      const fetchFn = mockFetch(200, [], { 'content-range': '0-9/42' })
      const builder = createBuilder(fetchFn)

      const { count } = await builder.select('*').count('exact')

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const init = call[1] as RequestInit
      const headers = init.headers as Record<string, string>
      expect(headers['Prefer']).toContain('count=exact')
      expect(count).toBe(42)
    })

    it('order() with ascending false produces desc', async () => {
      const fetchFn = mockFetch(200, [])
      const builder = createBuilder(fetchFn)

      await builder.select('*').order('name', { ascending: false })

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = new URL(call[0] as string)
      expect(url.searchParams.get('order')).toBe('name.desc')
    })

    it('order() with nullsFirst produces nullsfirst suffix', async () => {
      const fetchFn = mockFetch(200, [])
      const builder = createBuilder(fetchFn)

      await builder.select('*').order('name', { nullsFirst: true })

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = new URL(call[0] as string)
      expect(url.searchParams.get('order')).toBe('name.asc.nullsfirst')
    })

    it('order() with foreignTable scopes to that table', async () => {
      const fetchFn = mockFetch(200, [])
      const builder = createBuilder(fetchFn)

      await builder.select('*').order('name', { foreignTable: 'profiles' })

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = new URL(call[0] as string)
      expect(url.searchParams.get('profiles.order')).toBe('name.asc')
    })
  })

  describe('Error handling', () => {
    it('returns MimDBError for non-OK responses', async () => {
      const fetchFn = mockFetch(404, {
        message: 'relation "missing" not found',
        code: 'PGRST204',
      })
      const builder = createBuilder(fetchFn)

      const { data, error, status } = await builder.select('*')

      expect(data).toBeNull()
      expect(error).not.toBeNull()
      expect(error!.code).toBe('PGRST204')
      expect(status).toBe(404)
    })

    it('handles fetch failures gracefully', async () => {
      const fetchFn = vi.fn().mockRejectedValue(new Error('Network error')) as unknown as typeof fetch
      const builder = createBuilder(fetchFn)

      const { data, error, status } = await builder.select('*')

      expect(data).toBeNull()
      expect(error).not.toBeNull()
      expect(error!.code).toBe('FETCH_ERROR')
      expect(error!.message).toBe('Network error')
      expect(status).toBe(0)
    })
  })

  describe('Content-Range parsing', () => {
    it('parses count from content-range header', async () => {
      const fetchFn = mockFetch(200, [], { 'content-range': '0-24/100' })
      const builder = createBuilder(fetchFn)

      const { count } = await builder.select('*').count()

      expect(count).toBe(100)
    })

    it('parses count from */N format', async () => {
      const fetchFn = mockFetch(200, [], { 'content-range': '*/50' })
      const builder = createBuilder(fetchFn)

      const { count } = await builder.select('*').count()

      expect(count).toBe(50)
    })

    it('returns null count when header is absent', async () => {
      const fetchFn = mockFetch(200, [])
      const builder = createBuilder(fetchFn)

      const { count } = await builder.select('*')

      expect(count).toBeNull()
    })
  })
})
