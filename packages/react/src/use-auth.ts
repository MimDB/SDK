import { useState, useEffect, useCallback } from 'react'
import { useClient } from './context'
import type { User } from '@mimdb/client'

/**
 * Return type of the {@link useAuth} hook.
 */
export interface UseAuthResult {
  /** The currently authenticated user, or null if signed out. */
  user: User | null
  /** True while the initial session check is in progress. */
  isLoading: boolean
  /** Sign in with email and password. */
  signIn: (email: string, password: string) => Promise<void>
  /** Create a new account with email and password. */
  signUp: (email: string, password: string) => Promise<void>
  /** Sign out the current user. */
  signOut: () => Promise<void>
  /** Redirect to an OAuth provider's authorization page. */
  signInWithOAuth: (provider: string, opts: { redirectTo: string }) => void
}

/**
 * React hook for authentication state management.
 *
 * On mount, checks for an existing session and fetches the current user.
 * Subscribes to auth state changes so the returned `user` stays in sync
 * with sign-in, sign-out, and token refresh events.
 *
 * @returns An object with the current user, loading state, and auth methods.
 *
 * @example
 * ```tsx
 * function LoginPage() {
 *   const { user, isLoading, signIn, signOut } = useAuth()
 *
 *   if (isLoading) return <p>Loading...</p>
 *   if (user) return <button onClick={signOut}>Sign Out</button>
 *
 *   return (
 *     <button onClick={() => signIn('user@example.com', 'password')}>
 *       Sign In
 *     </button>
 *   )
 * }
 * ```
 */
export function useAuth(): UseAuthResult {
  const client = useClient()
  const [user, setUser] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    const session = client.auth.getSession()
    if (session) {
      client.auth
        .getUser()
        .then(setUser)
        .catch(() => setUser(null))
        .finally(() => setIsLoading(false))
    } else {
      setIsLoading(false)
    }

    const unsub = client.auth.onAuthStateChange((event, _session) => {
      if (event === 'SIGNED_IN' || event === 'TOKEN_REFRESHED') {
        client.auth
          .getUser()
          .then(setUser)
          .catch(() => setUser(null))
      } else if (event === 'SIGNED_OUT' || event === 'TOKEN_REFRESH_FAILED') {
        setUser(null)
      }
    })

    return unsub
  }, [client])

  const signIn = useCallback(
    async (email: string, password: string) => {
      await client.auth.signIn(email, password)
    },
    [client],
  )

  const signUp = useCallback(
    async (email: string, password: string) => {
      await client.auth.signUp(email, password)
    },
    [client],
  )

  const signOut = useCallback(async () => {
    await client.auth.signOut()
  }, [client])

  const signInWithOAuth = useCallback(
    (provider: string, opts: { redirectTo: string }) => {
      const url = client.auth.signInWithOAuth(provider, opts)
      if (typeof window !== 'undefined') {
        window.location.href = url
      }
    },
    [client],
  )

  return { user, isLoading, signIn, signUp, signOut, signInWithOAuth }
}
