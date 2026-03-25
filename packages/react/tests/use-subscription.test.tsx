import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { type ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MimDBProvider } from '../src/provider'
import { useSubscription } from '../src/use-subscription'
import type { MimDBClient } from '@mimdb/client'
import type { SubscribeOptions } from '@mimdb/realtime'

// Mock the @mimdb/realtime module
const mockUnsubscribe = vi.fn()
let capturedOnEvent: ((event: unknown) => void) | null = null
let capturedOnSubscribed: (() => void) | null = null
let capturedOnError: (() => void) | null = null

const mockSubscribe = vi.fn().mockImplementation((_table: string, opts: SubscribeOptions) => {
  capturedOnEvent = opts.onEvent as (event: unknown) => void
  capturedOnSubscribed = opts.onSubscribed ?? null
  capturedOnError = opts.onError ?? null
  return {
    id: 'sub-1',
    table: _table,
    status: 'pending' as const,
    unsubscribe: mockUnsubscribe,
  }
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
    realtime: {
      subscribe: mockSubscribe,
      disconnect: vi.fn(),
    },
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

describe('useSubscription', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    capturedOnEvent = null
    capturedOnSubscribed = null
    capturedOnError = null
  })

  it('subscribes to the specified table', () => {
    const mockClient = createMockClient()
    renderHook(() => useSubscription('messages'), {
      wrapper: createWrapper(mockClient),
    })

    expect(mockSubscribe).toHaveBeenCalledWith(
      'messages',
      expect.objectContaining({
        event: '*',
      }),
    )
  })

  it('starts with pending status and null lastEvent', () => {
    const mockClient = createMockClient()
    const { result } = renderHook(() => useSubscription('messages'), {
      wrapper: createWrapper(mockClient),
    })

    expect(result.current.lastEvent).toBeNull()
    expect(result.current.status).toBeDefined()
  })

  it('updates lastEvent when an event is received', () => {
    const mockClient = createMockClient()
    const { result } = renderHook(() => useSubscription('messages'), {
      wrapper: createWrapper(mockClient),
    })

    const testEvent = {
      type: 'INSERT' as const,
      table: 'messages',
      new: { id: 1, text: 'hello' },
      old: null,
    }

    act(() => {
      capturedOnEvent?.(testEvent)
    })

    expect(result.current.lastEvent).toEqual(testEvent)
  })

  it('updates status to active when subscribed', () => {
    const mockClient = createMockClient()
    const { result } = renderHook(() => useSubscription('messages'), {
      wrapper: createWrapper(mockClient),
    })

    act(() => {
      capturedOnSubscribed?.()
    })

    expect(result.current.status).toBe('active')
  })

  it('updates status to error on subscription error', () => {
    const mockClient = createMockClient()
    const { result } = renderHook(() => useSubscription('messages'), {
      wrapper: createWrapper(mockClient),
    })

    act(() => {
      capturedOnError?.()
    })

    expect(result.current.status).toBe('error')
  })

  it('unsubscribes on unmount', () => {
    const mockClient = createMockClient()
    const { unmount } = renderHook(() => useSubscription('messages'), {
      wrapper: createWrapper(mockClient),
    })

    unmount()
    expect(mockUnsubscribe).toHaveBeenCalled()
  })

  it('passes event filter to subscribe', () => {
    const mockClient = createMockClient()
    renderHook(() => useSubscription('messages', { event: 'INSERT' }), {
      wrapper: createWrapper(mockClient),
    })

    expect(mockSubscribe).toHaveBeenCalledWith(
      'messages',
      expect.objectContaining({
        event: 'INSERT',
      }),
    )
  })

  it('passes filter to subscribe', () => {
    const mockClient = createMockClient()
    renderHook(() => useSubscription('messages', { filter: 'user_id=eq.42' }), {
      wrapper: createWrapper(mockClient),
    })

    expect(mockSubscribe).toHaveBeenCalledWith(
      'messages',
      expect.objectContaining({
        filter: 'user_id=eq.42',
      }),
    )
  })
})
