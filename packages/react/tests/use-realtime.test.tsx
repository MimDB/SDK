import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook } from '@testing-library/react'
import { type ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MimDBProvider, useRealtime } from '../src/index'
import type { MimDBClient } from '@mimdb/client'

// Mock the @mimdb/realtime module
const mockUnsubscribe = vi.fn()
const mockSubscribe = vi.fn().mockReturnValue({
  id: 'sub-1',
  table: 'messages',
  status: 'pending' as const,
  unsubscribe: mockUnsubscribe,
})

vi.mock('@mimdb/realtime', () => {
  class MockRealtimeClient {
    subscribe = mockSubscribe
    disconnect = vi.fn()
  }
  return { MimDBRealtimeClient: MockRealtimeClient }
})

function createMockClient(): MimDBClient {
  return {
    getConfig: vi.fn().mockReturnValue({
      url: 'https://api.test',
      ref: 'abc',
      apiKey: 'key',
    }),
    from: vi.fn(),
    auth: {
      getSession: vi.fn().mockReturnValue(null),
      onAuthStateChange: vi.fn().mockReturnValue(() => {}),
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

describe('useRealtime', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('subscribes to the specified table on mount', () => {
    const mockClient = createMockClient()
    renderHook(() => useRealtime('messages'), {
      wrapper: createWrapper(mockClient),
    })

    expect(mockSubscribe).toHaveBeenCalledWith(
      'messages',
      expect.objectContaining({
        event: '*',
      }),
    )
  })

  it('passes event filter to subscribe', () => {
    const mockClient = createMockClient()
    renderHook(() => useRealtime('messages', { event: 'INSERT' }), {
      wrapper: createWrapper(mockClient),
    })

    expect(mockSubscribe).toHaveBeenCalledWith(
      'messages',
      expect.objectContaining({
        event: 'INSERT',
      }),
    )
  })

  it('unsubscribes on unmount', () => {
    const mockClient = createMockClient()
    const { unmount } = renderHook(() => useRealtime('messages'), {
      wrapper: createWrapper(mockClient),
    })

    unmount()

    expect(mockUnsubscribe).toHaveBeenCalled()
  })

  it('returns subscription status', () => {
    const mockClient = createMockClient()
    const { result } = renderHook(() => useRealtime('messages'), {
      wrapper: createWrapper(mockClient),
    })

    expect(result.current.status).toBeDefined()
  })
})
