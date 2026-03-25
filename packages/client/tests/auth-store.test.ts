import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { InMemoryTokenStore, LocalStorageTokenStore } from '../src/auth-store'

describe('InMemoryTokenStore', () => {
  it('returns null when no tokens have been set', () => {
    const store = new InMemoryTokenStore()
    expect(store.get()).toBeNull()
  })

  it('stores and retrieves tokens', () => {
    const store = new InMemoryTokenStore()
    store.set('access-123', 'refresh-456')

    const result = store.get()
    expect(result).toEqual({ accessToken: 'access-123', refreshToken: 'refresh-456' })
  })

  it('overwrites previously stored tokens', () => {
    const store = new InMemoryTokenStore()
    store.set('old-access', 'old-refresh')
    store.set('new-access', 'new-refresh')

    expect(store.get()).toEqual({ accessToken: 'new-access', refreshToken: 'new-refresh' })
  })

  it('clear removes stored tokens', () => {
    const store = new InMemoryTokenStore()
    store.set('access', 'refresh')
    store.clear()

    expect(store.get()).toBeNull()
  })

  it('clear is safe to call when already empty', () => {
    const store = new InMemoryTokenStore()
    store.clear()
    expect(store.get()).toBeNull()
  })
})

describe('LocalStorageTokenStore', () => {
  let mockStorage: Record<string, string>

  beforeEach(() => {
    mockStorage = {}

    const storage = {
      getItem: vi.fn((key: string) => mockStorage[key] ?? null),
      setItem: vi.fn((key: string, value: string) => {
        mockStorage[key] = value
      }),
      removeItem: vi.fn((key: string) => {
        delete mockStorage[key]
      }),
      clear: vi.fn(),
      length: 0,
      key: vi.fn(),
    } as unknown as Storage

    vi.stubGlobal('localStorage', storage)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('returns null when no tokens are stored', () => {
    const store = new LocalStorageTokenStore()
    expect(store.get()).toBeNull()
  })

  it('stores and retrieves tokens via localStorage', () => {
    const store = new LocalStorageTokenStore()
    store.set('access-abc', 'refresh-xyz')

    expect(localStorage.setItem).toHaveBeenCalledWith('mimdb-access-token', 'access-abc')
    expect(localStorage.setItem).toHaveBeenCalledWith('mimdb-refresh-token', 'refresh-xyz')

    const result = store.get()
    expect(result).toEqual({ accessToken: 'access-abc', refreshToken: 'refresh-xyz' })
  })

  it('clear removes tokens from localStorage', () => {
    const store = new LocalStorageTokenStore()
    store.set('access', 'refresh')
    store.clear()

    expect(localStorage.removeItem).toHaveBeenCalledWith('mimdb-access-token')
    expect(localStorage.removeItem).toHaveBeenCalledWith('mimdb-refresh-token')
  })

  it('returns null when only access token is stored', () => {
    mockStorage['mimdb-access-token'] = 'access-only'
    const store = new LocalStorageTokenStore()

    expect(store.get()).toBeNull()
  })

  it('returns null when only refresh token is stored', () => {
    mockStorage['mimdb-refresh-token'] = 'refresh-only'
    const store = new LocalStorageTokenStore()

    expect(store.get()).toBeNull()
  })

  it('degrades gracefully when localStorage is undefined', () => {
    vi.unstubAllGlobals()
    // In Node.js, localStorage is not defined natively; simulate that
    // by deleting the global we just stubbed
    const desc = Object.getOwnPropertyDescriptor(globalThis, 'localStorage')
    if (desc) {
      Object.defineProperty(globalThis, 'localStorage', {
        get() {
          throw new ReferenceError('localStorage is not defined')
        },
        configurable: true,
      })
    }

    const store = new LocalStorageTokenStore()

    expect(store.get()).toBeNull()
    expect(() => store.set('a', 'b')).not.toThrow()
    expect(() => store.clear()).not.toThrow()

    // Restore
    if (desc) {
      Object.defineProperty(globalThis, 'localStorage', desc)
    }
  })
})
