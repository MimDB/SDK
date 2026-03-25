import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { AuthClient } from '../src/auth'
import { InMemoryTokenStore } from '../src/auth-store'
import { envelope, mockFetch } from './helpers'

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

/**
 * Build a minimal JWT with a given `exp` claim (seconds since epoch).
 */
function buildJwt(exp: number): string {
  const header = btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' }))
  const payload = btoa(JSON.stringify({ sub: 'user-1', exp }))
  return `${header}.${payload}.fake-signature`
}

function createAuth(
  fetchFn: typeof fetch,
  store?: InMemoryTokenStore,
  autoRefresh?: boolean,
): AuthClient {
  return new AuthClient(URL, REF, fetchFn, { ...HEADERS }, store ?? new InMemoryTokenStore(), autoRefresh)
}

describe('AuthClient auto-refresh', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('schedules a refresh before the token expires', async () => {
    // Token expires 60 seconds from now
    const nowSec = Math.floor(Date.now() / 1000)
    const exp = nowSec + 60
    const accessToken = buildJwt(exp)

    const newTokens = {
      access_token: buildJwt(nowSec + 3660),
      refresh_token: 'new-refresh',
      expires_in: 3600,
    }

    const fetchFn = mockFetch(200, envelope({ user: MOCK_USER, access_token: accessToken, refresh_token: 'refresh-1', expires_in: 60 }))
    const store = new InMemoryTokenStore()
    const auth = createAuth(fetchFn, store, true)

    await auth.signIn('test@example.com', 'password')

    // Replace fetch for the refresh call
    vi.mocked(fetchFn).mockResolvedValue(
      new Response(JSON.stringify(envelope(newTokens)), {
        status: 200,
        statusText: 'OK',
        headers: { 'content-type': 'application/json' },
      }),
    )

    // Advance to 30s before expiry (should trigger refresh)
    // exp - 30s from now = 30s delay
    await vi.advanceTimersByTimeAsync(31_000)

    // The refresh should have been called
    const calls = vi.mocked(fetchFn).mock.calls
    const refreshCall = calls.find(([url]) =>
      (url as string).includes('grant_type=refresh_token'),
    )
    expect(refreshCall).toBeDefined()
  })

  it('clears the refresh timer on signOut', async () => {
    const nowSec = Math.floor(Date.now() / 1000)
    const accessToken = buildJwt(nowSec + 120)

    const fetchFn = mockFetch(200, envelope({ user: MOCK_USER, access_token: accessToken, refresh_token: 'refresh-1', expires_in: 120 }))
    const store = new InMemoryTokenStore()
    const auth = createAuth(fetchFn, store, true)

    await auth.signIn('test@example.com', 'password')

    // Mock sign-out response
    vi.mocked(fetchFn).mockResolvedValue(
      new Response(null, { status: 204, statusText: 'No Content' }),
    )

    await auth.signOut()

    // Advance past when the refresh would have fired
    vi.mocked(fetchFn).mockClear()
    await vi.advanceTimersByTimeAsync(120_000)

    // No refresh calls should have been made
    const calls = vi.mocked(fetchFn).mock.calls
    const refreshCall = calls.find(([url]) =>
      (url as string).includes('grant_type=refresh_token'),
    )
    expect(refreshCall).toBeUndefined()
  })

  it('emits TOKEN_REFRESH_FAILED and clears session on failure', async () => {
    const nowSec = Math.floor(Date.now() / 1000)
    const accessToken = buildJwt(nowSec + 60)

    const fetchFn = mockFetch(200, envelope({ user: MOCK_USER, access_token: accessToken, refresh_token: 'refresh-1', expires_in: 60 }))
    const store = new InMemoryTokenStore()
    const auth = createAuth(fetchFn, store, true)

    const events: string[] = []
    auth.onAuthStateChange((event) => events.push(event))

    await auth.signIn('test@example.com', 'password')

    // Make the refresh fail
    vi.mocked(fetchFn).mockResolvedValue(
      new Response(JSON.stringify({ error: { code: 'AUTH-0100', message: 'Token expired' } }), {
        status: 401,
        statusText: 'Unauthorized',
        headers: { 'content-type': 'application/json' },
      }),
    )

    // Advance past the refresh point
    await vi.advanceTimersByTimeAsync(31_000)

    expect(events).toContain('TOKEN_REFRESH_FAILED')
    expect(store.get()).toBeNull()
  })

  it('does not schedule refresh when autoRefresh is false', async () => {
    const nowSec = Math.floor(Date.now() / 1000)
    const accessToken = buildJwt(nowSec + 60)

    const fetchFn = mockFetch(200, envelope({ user: MOCK_USER, access_token: accessToken, refresh_token: 'refresh-1', expires_in: 60 }))
    const store = new InMemoryTokenStore()
    const auth = createAuth(fetchFn, store, false)

    await auth.signIn('test@example.com', 'password')

    vi.mocked(fetchFn).mockClear()

    // Advance well past the refresh point
    await vi.advanceTimersByTimeAsync(120_000)

    const calls = vi.mocked(fetchFn).mock.calls
    const refreshCall = calls.find(([url]) =>
      (url as string).includes('grant_type=refresh_token'),
    )
    expect(refreshCall).toBeUndefined()
  })

  it('handles invalid JWT gracefully (no crash)', async () => {
    const fetchFn = mockFetch(200, envelope({ user: MOCK_USER, access_token: 'not-a-jwt', refresh_token: 'refresh-1', expires_in: 60 }))
    const store = new InMemoryTokenStore()
    const auth = createAuth(fetchFn, store, true)

    // Should not throw
    await auth.signIn('test@example.com', 'password')
    expect(store.get()).not.toBeNull()
  })
})
