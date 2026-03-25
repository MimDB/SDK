/**
 * Abstraction for storing and retrieving authentication tokens.
 *
 * Implementations handle the persistence mechanism (memory, localStorage, etc.)
 * so the auth client remains decoupled from any specific storage backend.
 */
export interface TokenStore {
  /**
   * Retrieve the current token pair.
   *
   * @returns The stored access and refresh tokens, or null if none are stored.
   */
  get(): { accessToken: string; refreshToken: string } | null

  /**
   * Persist a token pair.
   *
   * @param accessToken  - JWT access token.
   * @param refreshToken - Opaque refresh token.
   */
  set(accessToken: string, refreshToken: string): void

  /** Remove all stored tokens. */
  clear(): void
}

/**
 * In-memory token store suitable for server-side or short-lived clients.
 *
 * Tokens are lost when the process exits or the instance is garbage-collected.
 */
export class InMemoryTokenStore implements TokenStore {
  private tokens: { accessToken: string; refreshToken: string } | null = null

  get(): { accessToken: string; refreshToken: string } | null {
    return this.tokens
  }

  set(accessToken: string, refreshToken: string): void {
    this.tokens = { accessToken, refreshToken }
  }

  clear(): void {
    this.tokens = null
  }
}

/** @internal localStorage key for the access token. */
const LS_ACCESS_KEY = 'mimdb-access-token'
/** @internal localStorage key for the refresh token. */
const LS_REFRESH_KEY = 'mimdb-refresh-token'

/**
 * Token store backed by the browser's `localStorage`.
 *
 * Gracefully degrades to a no-op when `localStorage` is unavailable
 * (e.g. server-side rendering or restricted browser contexts).
 */
export class LocalStorageTokenStore implements TokenStore {
  /**
   * Whether `localStorage` is available in the current environment.
   *
   * @returns true if reads/writes will succeed, false otherwise.
   */
  private isAvailable(): boolean {
    try {
      return typeof localStorage !== 'undefined'
    } catch {
      return false
    }
  }

  get(): { accessToken: string; refreshToken: string } | null {
    if (!this.isAvailable()) return null

    const accessToken = localStorage.getItem(LS_ACCESS_KEY)
    const refreshToken = localStorage.getItem(LS_REFRESH_KEY)

    if (accessToken && refreshToken) {
      return { accessToken, refreshToken }
    }

    return null
  }

  set(accessToken: string, refreshToken: string): void {
    if (!this.isAvailable()) return

    localStorage.setItem(LS_ACCESS_KEY, accessToken)
    localStorage.setItem(LS_REFRESH_KEY, refreshToken)
  }

  clear(): void {
    if (!this.isAvailable()) return

    localStorage.removeItem(LS_ACCESS_KEY)
    localStorage.removeItem(LS_REFRESH_KEY)
  }
}
