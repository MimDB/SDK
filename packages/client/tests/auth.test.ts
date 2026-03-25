import { describe, expect, it, vi } from 'vitest'
import { AuthClient } from '../src/auth'
import { InMemoryTokenStore } from '../src/auth-store'
import { mockFetch } from './helpers'

const URL = 'https://api.mimdb.dev'
const REF = 'abc123'
const HEADERS = {
  'Content-Type': 'application/json',
  'Authorization': 'Bearer test-api-key',
  'apikey': 'test-api-key',
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

const MOCK_TOKENS = {
  access_token: 'access-token-123',
  refresh_token: 'refresh-token-456',
  expires_in: 3600,
}

function createAuth(fetchFn: typeof fetch, store?: InMemoryTokenStore): AuthClient {
  return new AuthClient(URL, REF, fetchFn, { ...HEADERS }, store ?? new InMemoryTokenStore())
}

describe('AuthClient', () => {
  describe('signUp', () => {
    it('sends correct POST to /signup and stores tokens', async () => {
      const fetchFn = mockFetch(200, { user: MOCK_USER, ...MOCK_TOKENS })
      const store = new InMemoryTokenStore()
      const auth = createAuth(fetchFn, store)

      const result = await auth.signUp('test@example.com', 'password123')

      expect(result.user).toEqual(MOCK_USER)
      expect(result.tokens).toEqual(MOCK_TOKENS)

      // Verify fetch call
      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = call[0] as string
      const init = call[1] as RequestInit
      expect(url).toBe(`${URL}/v1/auth/${REF}/signup`)
      expect(init.method).toBe('POST')
      expect(JSON.parse(init.body as string)).toEqual({
        email: 'test@example.com',
        password: 'password123',
      })

      // Tokens should be stored
      expect(store.get()).toEqual({
        accessToken: 'access-token-123',
        refreshToken: 'refresh-token-456',
      })
    })

    it('sends user_metadata when provided', async () => {
      const fetchFn = mockFetch(200, { user: MOCK_USER, ...MOCK_TOKENS })
      const auth = createAuth(fetchFn)

      await auth.signUp('test@example.com', 'password', {
        userMetadata: { name: 'Alice' },
      })

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const body = JSON.parse((call[1] as RequestInit).body as string)
      expect(body.user_metadata).toEqual({ name: 'Alice' })
    })

    it('fires SIGNED_IN event', async () => {
      const fetchFn = mockFetch(200, { user: MOCK_USER, ...MOCK_TOKENS })
      const auth = createAuth(fetchFn)
      const listener = vi.fn()
      auth.onAuthStateChange(listener)

      await auth.signUp('test@example.com', 'password')

      expect(listener).toHaveBeenCalledWith('SIGNED_IN', {
        accessToken: 'access-token-123',
        refreshToken: 'refresh-token-456',
      })
    })
  })

  describe('signIn', () => {
    it('sends correct POST to /token with grant_type password', async () => {
      const fetchFn = mockFetch(200, { user: MOCK_USER, ...MOCK_TOKENS })
      const store = new InMemoryTokenStore()
      const auth = createAuth(fetchFn, store)

      const result = await auth.signIn('test@example.com', 'password123')

      expect(result.user).toEqual(MOCK_USER)
      expect(result.tokens).toEqual(MOCK_TOKENS)

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = call[0] as string
      const init = call[1] as RequestInit
      expect(url).toBe(`${URL}/v1/auth/${REF}/token`)
      expect(init.method).toBe('POST')
      expect(JSON.parse(init.body as string)).toEqual({
        email: 'test@example.com',
        password: 'password123',
        grant_type: 'password',
      })

      expect(store.get()).toEqual({
        accessToken: 'access-token-123',
        refreshToken: 'refresh-token-456',
      })
    })

    it('fires SIGNED_IN event', async () => {
      const fetchFn = mockFetch(200, { user: MOCK_USER, ...MOCK_TOKENS })
      const auth = createAuth(fetchFn)
      const listener = vi.fn()
      auth.onAuthStateChange(listener)

      await auth.signIn('test@example.com', 'password')

      expect(listener).toHaveBeenCalledWith('SIGNED_IN', {
        accessToken: 'access-token-123',
        refreshToken: 'refresh-token-456',
      })
    })

    it('throws MimDBError on failed sign-in', async () => {
      const fetchFn = mockFetch(401, { message: 'Invalid credentials', code: 'AUTH_INVALID' })
      const auth = createAuth(fetchFn)

      await expect(auth.signIn('bad@example.com', 'wrong'))
        .rejects
        .toThrow('Invalid credentials')
    })
  })

  describe('signOut', () => {
    it('sends POST to /logout and clears tokens', async () => {
      const signInFetch = mockFetch(200, { user: MOCK_USER, ...MOCK_TOKENS })
      const store = new InMemoryTokenStore()
      const auth = createAuth(signInFetch, store)

      await auth.signIn('test@example.com', 'password')
      expect(store.get()).not.toBeNull()

      // Replace fetch for sign out
      const signOutFetch = mockFetch(200, null)
      const auth2 = createAuth(signOutFetch, store)

      await auth2.signOut()

      const call = vi.mocked(signOutFetch).mock.calls[0]!
      const url = call[0] as string
      const init = call[1] as RequestInit
      expect(url).toBe(`${URL}/v1/auth/${REF}/logout`)
      expect(init.method).toBe('POST')

      expect(store.get()).toBeNull()
    })

    it('fires SIGNED_OUT event', async () => {
      const fetchFn = mockFetch(200, null)
      const auth = createAuth(fetchFn)
      const listener = vi.fn()
      auth.onAuthStateChange(listener)

      await auth.signOut()

      expect(listener).toHaveBeenCalledWith('SIGNED_OUT', null)
    })

    it('sends current access token in Authorization header', async () => {
      const store = new InMemoryTokenStore()
      store.set('user-access-token', 'user-refresh-token')

      const fetchFn = mockFetch(200, null)
      const auth = createAuth(fetchFn, store)

      await auth.signOut()

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const init = call[1] as RequestInit
      const headers = init.headers as Record<string, string>
      expect(headers['Authorization']).toBe('Bearer user-access-token')
    })
  })

  describe('refreshSession', () => {
    it('sends correct POST with refresh_token and grant_type', async () => {
      const store = new InMemoryTokenStore()
      store.set('old-access', 'old-refresh')

      const newTokens = {
        access_token: 'new-access',
        refresh_token: 'new-refresh',
        expires_in: 7200,
      }
      const fetchFn = mockFetch(200, newTokens)
      const auth = createAuth(fetchFn, store)

      const result = await auth.refreshSession()

      expect(result).toEqual(newTokens)

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = call[0] as string
      const init = call[1] as RequestInit
      expect(url).toBe(`${URL}/v1/auth/${REF}/token`)
      expect(JSON.parse(init.body as string)).toEqual({
        refresh_token: 'old-refresh',
        grant_type: 'refresh_token',
      })

      expect(store.get()).toEqual({
        accessToken: 'new-access',
        refreshToken: 'new-refresh',
      })
    })

    it('uses explicit refresh token when provided', async () => {
      const fetchFn = mockFetch(200, MOCK_TOKENS)
      const auth = createAuth(fetchFn)

      await auth.refreshSession('explicit-refresh-token')

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const body = JSON.parse((call[1] as RequestInit).body as string)
      expect(body.refresh_token).toBe('explicit-refresh-token')
    })

    it('fires TOKEN_REFRESHED event', async () => {
      const store = new InMemoryTokenStore()
      store.set('old-access', 'old-refresh')

      const fetchFn = mockFetch(200, MOCK_TOKENS)
      const auth = createAuth(fetchFn, store)
      const listener = vi.fn()
      auth.onAuthStateChange(listener)

      await auth.refreshSession()

      expect(listener).toHaveBeenCalledWith('TOKEN_REFRESHED', {
        accessToken: 'access-token-123',
        refreshToken: 'refresh-token-456',
      })
    })

    it('throws when no refresh token is available', async () => {
      const fetchFn = mockFetch(200, MOCK_TOKENS)
      const auth = createAuth(fetchFn)

      await expect(auth.refreshSession())
        .rejects
        .toThrow('No refresh token available')
    })
  })

  describe('getUser', () => {
    it('sends GET with access token in Authorization header', async () => {
      const store = new InMemoryTokenStore()
      store.set('my-access-token', 'my-refresh')

      const fetchFn = mockFetch(200, MOCK_USER)
      const auth = createAuth(fetchFn, store)

      const user = await auth.getUser()

      expect(user).toEqual(MOCK_USER)

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = call[0] as string
      const init = call[1] as RequestInit
      expect(url).toBe(`${URL}/v1/auth/${REF}/user`)
      expect(init.method).toBe('GET')
      const headers = init.headers as Record<string, string>
      expect(headers['Authorization']).toBe('Bearer my-access-token')
    })

    it('throws on API error', async () => {
      const fetchFn = mockFetch(401, { message: 'Unauthorized', code: 'AUTH01' })
      const auth = createAuth(fetchFn)

      await expect(auth.getUser()).rejects.toThrow('Unauthorized')
    })
  })

  describe('updateUser', () => {
    it('sends PUT with user_metadata', async () => {
      const store = new InMemoryTokenStore()
      store.set('my-access', 'my-refresh')

      const updatedUser = { ...MOCK_USER, user_metadata: { theme: 'dark' } }
      const fetchFn = mockFetch(200, updatedUser)
      const auth = createAuth(fetchFn, store)

      const result = await auth.updateUser({ userMetadata: { theme: 'dark' } })

      expect(result).toEqual(updatedUser)

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = call[0] as string
      const init = call[1] as RequestInit
      expect(url).toBe(`${URL}/v1/auth/${REF}/user`)
      expect(init.method).toBe('PUT')
      expect(JSON.parse(init.body as string)).toEqual({
        user_metadata: { theme: 'dark' },
      })
      const headers = init.headers as Record<string, string>
      expect(headers['Authorization']).toBe('Bearer my-access')
    })
  })

  describe('signInWithOAuth', () => {
    it('returns correct OAuth URL string', () => {
      const auth = createAuth(mockFetch(200, null))

      const url = auth.signInWithOAuth('google', {
        redirectTo: 'https://myapp.com/callback',
      })

      expect(url).toBe(
        `${URL}/v1/auth/${REF}/oauth/google?redirect_to=${encodeURIComponent('https://myapp.com/callback')}`,
      )
    })

    it('encodes special characters in redirectTo', () => {
      const auth = createAuth(mockFetch(200, null))

      const url = auth.signInWithOAuth('github', {
        redirectTo: 'https://myapp.com/auth?foo=bar&baz=qux',
      })

      expect(url).toContain('redirect_to=')
      expect(url).toContain(encodeURIComponent('https://myapp.com/auth?foo=bar&baz=qux'))
    })
  })

  describe('handleOAuthCallback', () => {
    it('parses URL fragment correctly', () => {
      const store = new InMemoryTokenStore()
      const auth = createAuth(mockFetch(200, null), store)

      const result = auth.handleOAuthCallback(
        '#access_token=abc&refresh_token=xyz&expires_in=3600',
      )

      expect(result).toEqual({
        accessToken: 'abc',
        refreshToken: 'xyz',
        expiresIn: 3600,
      })

      expect(store.get()).toEqual({
        accessToken: 'abc',
        refreshToken: 'xyz',
      })
    })

    it('handles fragment without leading hash', () => {
      const auth = createAuth(mockFetch(200, null))

      const result = auth.handleOAuthCallback(
        'access_token=abc&refresh_token=xyz&expires_in=3600',
      )

      expect(result).toEqual({
        accessToken: 'abc',
        refreshToken: 'xyz',
        expiresIn: 3600,
      })
    })

    it('returns null for missing access_token', () => {
      const auth = createAuth(mockFetch(200, null))

      const result = auth.handleOAuthCallback('#refresh_token=xyz&expires_in=3600')

      expect(result).toBeNull()
    })

    it('returns null for missing refresh_token', () => {
      const auth = createAuth(mockFetch(200, null))

      const result = auth.handleOAuthCallback('#access_token=abc&expires_in=3600')

      expect(result).toBeNull()
    })

    it('returns null for missing expires_in', () => {
      const auth = createAuth(mockFetch(200, null))

      const result = auth.handleOAuthCallback('#access_token=abc&refresh_token=xyz')

      expect(result).toBeNull()
    })

    it('returns null for non-numeric expires_in', () => {
      const auth = createAuth(mockFetch(200, null))

      const result = auth.handleOAuthCallback(
        '#access_token=abc&refresh_token=xyz&expires_in=notanumber',
      )

      expect(result).toBeNull()
    })

    it('fires SIGNED_IN event on success', () => {
      const auth = createAuth(mockFetch(200, null))
      const listener = vi.fn()
      auth.onAuthStateChange(listener)

      auth.handleOAuthCallback('#access_token=abc&refresh_token=xyz&expires_in=3600')

      expect(listener).toHaveBeenCalledWith('SIGNED_IN', {
        accessToken: 'abc',
        refreshToken: 'xyz',
      })
    })
  })

  describe('getSession', () => {
    it('returns cached tokens from store', () => {
      const store = new InMemoryTokenStore()
      store.set('cached-access', 'cached-refresh')

      const auth = createAuth(mockFetch(200, null), store)
      const session = auth.getSession()

      expect(session).toEqual({
        accessToken: 'cached-access',
        refreshToken: 'cached-refresh',
      })
    })

    it('returns null when no tokens are stored', () => {
      const auth = createAuth(mockFetch(200, null))
      expect(auth.getSession()).toBeNull()
    })
  })

  describe('onAuthStateChange', () => {
    it('fires on signIn', async () => {
      const fetchFn = mockFetch(200, { user: MOCK_USER, ...MOCK_TOKENS })
      const auth = createAuth(fetchFn)
      const events: string[] = []
      auth.onAuthStateChange((event) => events.push(event))

      await auth.signIn('test@example.com', 'password')

      expect(events).toEqual(['SIGNED_IN'])
    })

    it('fires on signOut', async () => {
      const fetchFn = mockFetch(200, null)
      const auth = createAuth(fetchFn)
      const events: string[] = []
      auth.onAuthStateChange((event) => events.push(event))

      await auth.signOut()

      expect(events).toEqual(['SIGNED_OUT'])
    })

    it('fires on refreshSession', async () => {
      const store = new InMemoryTokenStore()
      store.set('old', 'old-refresh')

      const fetchFn = mockFetch(200, MOCK_TOKENS)
      const auth = createAuth(fetchFn, store)
      const events: string[] = []
      auth.onAuthStateChange((event) => events.push(event))

      await auth.refreshSession()

      expect(events).toEqual(['TOKEN_REFRESHED'])
    })

    it('unsubscribe function stops notifications', async () => {
      const fetchFn = mockFetch(200, { user: MOCK_USER, ...MOCK_TOKENS })
      const auth = createAuth(fetchFn)
      const events: string[] = []

      const unsubscribe = auth.onAuthStateChange((event) => events.push(event))
      unsubscribe()

      await auth.signIn('test@example.com', 'password')

      expect(events).toEqual([])
    })

    it('supports multiple listeners', async () => {
      const fetchFn = mockFetch(200, { user: MOCK_USER, ...MOCK_TOKENS })
      const auth = createAuth(fetchFn)
      const events1: string[] = []
      const events2: string[] = []

      auth.onAuthStateChange((event) => events1.push(event))
      auth.onAuthStateChange((event) => events2.push(event))

      await auth.signIn('test@example.com', 'password')

      expect(events1).toEqual(['SIGNED_IN'])
      expect(events2).toEqual(['SIGNED_IN'])
    })
  })

  describe('admin accessor', () => {
    it('returns an AuthAdminClient', () => {
      const auth = createAuth(mockFetch(200, null))
      const admin = auth.admin
      expect(admin).toBeDefined()
    })

    it('returns the same instance on repeated access', () => {
      const auth = createAuth(mockFetch(200, null))
      expect(auth.admin).toBe(auth.admin)
    })
  })
})
