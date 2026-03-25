import { describe, it, expect, vi } from 'vitest'
import { renderHook } from '@testing-library/react'
import { type ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MimDBProvider, useClient } from '../src/index'
import type { MimDBClient } from '@mimdb/client'

function createMockClient(): MimDBClient {
  return {
    getConfig: vi.fn().mockReturnValue({ url: 'https://api.test', ref: 'abc', apiKey: 'key' }),
    from: vi.fn(),
    rpc: vi.fn(),
    auth: {
      signIn: vi.fn(),
      signUp: vi.fn(),
      signOut: vi.fn(),
      getSession: vi.fn().mockReturnValue(null),
      getUser: vi.fn(),
      onAuthStateChange: vi.fn().mockReturnValue(() => {}),
      signInWithOAuth: vi.fn(),
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

describe('MimDBProvider', () => {
  it('provides the client via useClient', () => {
    const mockClient = createMockClient()
    const { result } = renderHook(() => useClient(), {
      wrapper: createWrapper(mockClient),
    })
    expect(result.current).toBe(mockClient)
  })

  it('throws when useClient is used outside MimDBProvider', () => {
    const queryClient = new QueryClient()
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    )

    expect(() =>
      renderHook(() => useClient(), { wrapper }),
    ).toThrow('useClient must be used within <MimDBProvider>')
  })
})
