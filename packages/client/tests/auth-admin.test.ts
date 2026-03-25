import { describe, expect, it, vi } from 'vitest'
import { AuthAdminClient } from '../src/auth-admin'
import { mockFetch } from './helpers'

const URL = 'https://api.mimdb.dev'
const REF = 'abc123'
const HEADERS = {
  'Content-Type': 'application/json',
  'Authorization': 'Bearer service-role-key',
  'apikey': 'service-role-key',
}

const MOCK_USER = {
  id: 'user-uuid-1',
  email: 'test@example.com',
  email_confirmed: true,
  phone_confirmed: false,
  app_metadata: {},
  user_metadata: {},
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
}

function createAdmin(fetchFn: typeof fetch): AuthAdminClient {
  return new AuthAdminClient(URL, REF, fetchFn, { ...HEADERS })
}

describe('AuthAdminClient', () => {
  describe('listUsers', () => {
    it('sends GET to /users', async () => {
      const fetchFn = mockFetch(200, [MOCK_USER])
      const admin = createAdmin(fetchFn)

      const users = await admin.listUsers()

      expect(users).toEqual([MOCK_USER])

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = call[0] as string
      const init = call[1] as RequestInit
      expect(url).toBe(`${URL}/v1/auth/${REF}/users`)
      expect(init.method).toBe('GET')
    })

    it('sends limit and offset query params', async () => {
      const fetchFn = mockFetch(200, [MOCK_USER])
      const admin = createAdmin(fetchFn)

      await admin.listUsers({ limit: 10, offset: 20 })

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = new globalThis.URL(call[0] as string)
      expect(url.searchParams.get('limit')).toBe('10')
      expect(url.searchParams.get('offset')).toBe('20')
    })

    it('omits params when not provided', async () => {
      const fetchFn = mockFetch(200, [])
      const admin = createAdmin(fetchFn)

      await admin.listUsers()

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = call[0] as string
      expect(url).not.toContain('?')
    })

    it('sends service_role Authorization header', async () => {
      const fetchFn = mockFetch(200, [])
      const admin = createAdmin(fetchFn)

      await admin.listUsers()

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const init = call[1] as RequestInit
      const headers = init.headers as Record<string, string>
      expect(headers['Authorization']).toBe('Bearer service-role-key')
    })

    it('throws on API error', async () => {
      const fetchFn = mockFetch(403, { message: 'Forbidden', code: 'AUTH_FORBIDDEN' })
      const admin = createAdmin(fetchFn)

      await expect(admin.listUsers()).rejects.toThrow('Forbidden')
    })
  })

  describe('getUserByEmail', () => {
    it('sends GET with email query param', async () => {
      const fetchFn = mockFetch(200, [MOCK_USER])
      const admin = createAdmin(fetchFn)

      const user = await admin.getUserByEmail('test@example.com')

      expect(user).toEqual(MOCK_USER)

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = new globalThis.URL(call[0] as string)
      expect(url.searchParams.get('email')).toBe('test@example.com')
      expect(url.pathname).toBe(`/v1/auth/${REF}/users`)
    })

    it('returns null when no user is found', async () => {
      const fetchFn = mockFetch(200, [])
      const admin = createAdmin(fetchFn)

      const user = await admin.getUserByEmail('missing@example.com')

      expect(user).toBeNull()
    })

    it('throws on API error', async () => {
      const fetchFn = mockFetch(500, { message: 'Internal error', code: 'INTERNAL' })
      const admin = createAdmin(fetchFn)

      await expect(admin.getUserByEmail('test@example.com'))
        .rejects
        .toThrow('Internal error')
    })
  })

  describe('updateUserById', () => {
    it('sends PATCH with app_metadata', async () => {
      const updatedUser = { ...MOCK_USER, app_metadata: { role: 'admin' } }
      const fetchFn = mockFetch(200, updatedUser)
      const admin = createAdmin(fetchFn)

      const result = await admin.updateUserById('user-uuid-1', {
        appMetadata: { role: 'admin' },
      })

      expect(result).toEqual(updatedUser)

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = call[0] as string
      const init = call[1] as RequestInit
      expect(url).toBe(`${URL}/v1/auth/${REF}/users/user-uuid-1`)
      expect(init.method).toBe('PATCH')
      expect(JSON.parse(init.body as string)).toEqual({
        app_metadata: { role: 'admin' },
      })
    })

    it('sends PATCH with user_metadata', async () => {
      const fetchFn = mockFetch(200, MOCK_USER)
      const admin = createAdmin(fetchFn)

      await admin.updateUserById('user-uuid-1', {
        userMetadata: { theme: 'dark' },
      })

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const body = JSON.parse((call[1] as RequestInit).body as string)
      expect(body.user_metadata).toEqual({ theme: 'dark' })
    })

    it('sends both app_metadata and user_metadata', async () => {
      const fetchFn = mockFetch(200, MOCK_USER)
      const admin = createAdmin(fetchFn)

      await admin.updateUserById('user-uuid-1', {
        appMetadata: { role: 'moderator' },
        userMetadata: { name: 'Bob' },
      })

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const body = JSON.parse((call[1] as RequestInit).body as string)
      expect(body.app_metadata).toEqual({ role: 'moderator' })
      expect(body.user_metadata).toEqual({ name: 'Bob' })
    })

    it('throws on API error', async () => {
      const fetchFn = mockFetch(404, { message: 'User not found', code: 'AUTH_NOT_FOUND' })
      const admin = createAdmin(fetchFn)

      await expect(admin.updateUserById('missing-id', { appMetadata: {} }))
        .rejects
        .toThrow('User not found')
    })
  })
})
