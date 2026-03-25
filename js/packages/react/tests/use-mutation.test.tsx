import { describe, it, expect, vi } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { type ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MimDBProvider } from '../src/provider'
import { useInsert, useUpdate, useDelete } from '../src/use-mutation'
import type { MimDBClient } from '@mimdb/client'

interface Todo {
  id: string
  task: string
  done: boolean
}

/**
 * Build a mock MimDBClient with configurable mutation responses.
 * The mutationFn is deliberately slow so we can observe the optimistic
 * cache state before the server responds.
 */
function createMockClient(): MimDBClient {
  // Use a deferred promise so we can control when the mutation resolves
  let resolveInsert: (() => void) | null = null
  let resolveUpdate: (() => void) | null = null
  let resolveDelete: (() => void) | null = null

  const makeQuery = (resolver: { get: () => (() => void) | null, set: (fn: () => void) => void }, result: unknown) => {
    const single = () => new Promise<{ data: unknown; error: null }>((resolve) => {
      resolver.set(() => resolve({ data: result, error: null }))
    })
    return {
      single,
      select: () => ({ single }),
      eq: (col: string, val: string) => makeQuery(resolver, result),
    }
  }

  const insertResolver = {
    get: () => resolveInsert,
    set: (fn: () => void) => { resolveInsert = fn },
  }
  const updateResolver = {
    get: () => resolveUpdate,
    set: (fn: () => void) => { resolveUpdate = fn },
  }

  return {
    from: vi.fn().mockImplementation(() => ({
      insert: () => ({
        select: () => ({
          single: () =>
            new Promise((resolve) => {
              resolveInsert = () =>
                resolve({ data: { id: 'new', task: 'New task', done: false }, error: null })
            }),
        }),
      }),
      update: () => {
        const chain: Record<string, unknown> = {}
        chain.eq = () => ({
          select: () => ({
            single: () =>
              new Promise((resolve) => {
                resolveUpdate = () =>
                  resolve({ data: { id: '1', task: 'Existing task', done: true }, error: null })
              }),
          }),
        })
        return chain
      },
      delete: () => {
        const chain: Record<string, unknown> = {}
        chain.eq = () => ({
          then: (onfulfilled: Function) =>
            new Promise((resolve) => {
              resolveDelete = () => resolve(onfulfilled({ data: null, error: null }))
            }),
        })
        return chain
      },
    })),
    auth: {
      getSession: vi.fn().mockReturnValue(null),
      onAuthStateChange: vi.fn().mockReturnValue(() => {}),
    },
    storage: { from: vi.fn() },
    getConfig: vi.fn().mockReturnValue({ url: 'http://test', ref: 'ref', apiKey: 'key' }),
  } as unknown as MimDBClient
}

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  // Prepopulate cache using the full key format produced by useQuery
  queryClient.setQueryData(['mimdb', 'todos', undefined], [
    { id: '1', task: 'Existing task', done: false },
    { id: '2', task: 'Another task', done: true },
  ])

  const mockClient = createMockClient()

  return {
    queryClient,
    mockClient,
    Wrapper: function Wrapper({ children }: { children: ReactNode }) {
      return (
        <QueryClientProvider client={queryClient}>
          <MimDBProvider client={mockClient}>{children}</MimDBProvider>
        </QueryClientProvider>
      )
    },
  }
}

describe('useInsert optimistic', () => {
  it('adds item to cache immediately when optimistic is true', async () => {
    const { queryClient, Wrapper } = createWrapper()

    const { result } = renderHook(
      () => useInsert<Todo>('todos', { optimistic: true }),
      { wrapper: Wrapper },
    )

    await act(async () => {
      result.current.mutate({ task: 'New task', done: false })
      // Allow microtasks (onMutate) to run
      await new Promise((r) => setTimeout(r, 0))
    })

    // The cache should have been updated optimistically
    const cached = queryClient.getQueryData<Todo[]>(['mimdb', 'todos', undefined])
    expect(cached?.length).toBe(3)
    expect(cached?.[2]?.task).toBe('New task')
  })
})

describe('useUpdate optimistic', () => {
  it('updates matching row in cache when optimistic is true', async () => {
    const { queryClient, Wrapper } = createWrapper()

    const { result } = renderHook(
      () => useUpdate<Todo>('todos', { optimistic: true }),
      { wrapper: Wrapper },
    )

    await act(async () => {
      result.current.mutate({ data: { done: true }, eq: { id: '1' } })
      await new Promise((r) => setTimeout(r, 0))
    })

    const cached = queryClient.getQueryData<Todo[]>(['mimdb', 'todos', undefined])
    const row = cached?.find((r) => r.id === '1')
    expect(row?.done).toBe(true)
  })
})

describe('useDelete optimistic', () => {
  it('removes matching row from cache when optimistic is true', async () => {
    const { queryClient, Wrapper } = createWrapper()

    const { result } = renderHook(
      () => useDelete('todos', { optimistic: true }),
      { wrapper: Wrapper },
    )

    await act(async () => {
      result.current.mutate({ id: '1' })
      await new Promise((r) => setTimeout(r, 0))
    })

    const cached = queryClient.getQueryData<Record<string, unknown>[]>(['mimdb', 'todos', undefined])
    expect(cached?.length).toBe(1)
    expect(cached?.[0]?.id).toBe('2')
  })
})
