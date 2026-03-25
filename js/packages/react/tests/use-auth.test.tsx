import { describe, it, expect, vi } from 'vitest'
import { renderHook, waitFor, act } from '@testing-library/react'
import { type ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MimDBProvider, useAuth } from '../src/index'
import type { MimDBClient } from '@mimdb/client'
import type { User } from '@mimdb/client'

const mockUser: User = {
  id: 'user-1',
  email: 'test@example.com',
  email_confirmed: true,
  phone_confirmed: false,
  app_metadata: {},
  user_metadata: {},
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
}

function createMockClient(hasSession = false): MimDBClient {
  return {
    getConfig: vi.fn().mockReturnValue({ url: 'https://api.test', ref: 'abc', apiKey: 'key' }),
    from: vi.fn(),
    auth: {
      signIn: vi.fn().mockResolvedValue({ user: mockUser, tokens: {} }),
      signUp: vi.fn().mockResolvedValue({ user: mockUser, tokens: {} }),
      signOut: vi.fn().mockResolvedValue(undefined),
      getSession: vi.fn().mockReturnValue(
        hasSession ? { accessToken: 'tok', refreshToken: 'ref' } : null,
      ),
      getUser: vi.fn().mockResolvedValue(mockUser),
      onAuthStateChange: vi.fn().mockReturnValue(() => {}),
      signInWithOAuth: vi.fn().mockReturnValue('https://oauth.example.com'),
    },
    storage: { from: vi.fn() },
  } as unknown as MimDBClient
}

function createWrapper(client: MimDBClient) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        <MimDBProvider client={client}>{children}</MimDBProvider>
      </QueryClientProvider>
    )
  }
}

describe('useAuth', () => {
  it('starts with null user and loading state when no session', async () => {
    const mockClient = createMockClient(false)
    const { result } = renderHook(() => useAuth(), {
      wrapper: createWrapper(mockClient),
    })

    await waitFor(() => expect(result.current.isLoading).toBe(false))

    expect(result.current.user).toBeNull()
  })

  it('fetches user when session exists', async () => {
    const mockClient = createMockClient(true)
    const { result } = renderHook(() => useAuth(), {
      wrapper: createWrapper(mockClient),
    })

    await waitFor(() => expect(result.current.isLoading).toBe(false))

    expect(result.current.user).toEqual(mockUser)
    expect(mockClient.auth.getUser).toHaveBeenCalled()
  })

  it('provides signIn, signUp, signOut functions', async () => {
    const mockClient = createMockClient(false)
    const { result } = renderHook(() => useAuth(), {
      wrapper: createWrapper(mockClient),
    })

    await waitFor(() => expect(result.current.isLoading).toBe(false))

    expect(typeof result.current.signIn).toBe('function')
    expect(typeof result.current.signUp).toBe('function')
    expect(typeof result.current.signOut).toBe('function')
    expect(typeof result.current.signInWithOAuth).toBe('function')
  })

  it('calls client.auth.signIn when signIn is invoked', async () => {
    const mockClient = createMockClient(false)
    const { result } = renderHook(() => useAuth(), {
      wrapper: createWrapper(mockClient),
    })

    await waitFor(() => expect(result.current.isLoading).toBe(false))

    await act(async () => {
      await result.current.signIn('test@example.com', 'password')
    })

    expect(mockClient.auth.signIn).toHaveBeenCalledWith('test@example.com', 'password')
  })

  it('subscribes to auth state changes', () => {
    const mockClient = createMockClient(false)
    renderHook(() => useAuth(), {
      wrapper: createWrapper(mockClient),
    })

    expect(mockClient.auth.onAuthStateChange).toHaveBeenCalled()
  })
})
