import { describe, it, expect, vi } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { type ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MimDBProvider, useQuery } from '../src/index'
import type { MimDBClient } from '@mimdb/client'

interface Todo {
  id: string
  task: string
  done: boolean
}

const mockRows: Todo[] = [
  { id: '1', task: 'Buy milk', done: false },
  { id: '2', task: 'Walk dog', done: true },
]

function createMockClient(data: unknown[] = mockRows): MimDBClient {
  const mockQueryBuilder = {
    select: vi.fn().mockReturnThis(),
    eq: vi.fn().mockReturnThis(),
    neq: vi.fn().mockReturnThis(),
    order: vi.fn().mockReturnThis(),
    limit: vi.fn().mockReturnThis(),
    offset: vi.fn().mockReturnThis(),
    then: vi.fn((resolve: Function) =>
      resolve({ data, error: null, count: null, status: 200, statusText: 'OK' }),
    ),
  }

  return {
    getConfig: vi.fn().mockReturnValue({ url: 'https://api.test', ref: 'abc', apiKey: 'key' }),
    from: vi.fn().mockReturnValue(mockQueryBuilder),
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

describe('useQuery', () => {
  it('fetches data from the specified table', async () => {
    const mockClient = createMockClient()
    const { result } = renderHook(() => useQuery<Todo>('todos'), {
      wrapper: createWrapper(mockClient),
    })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(result.current.data).toEqual(mockRows)
    expect(mockClient.from).toHaveBeenCalledWith('todos')
  })

  it('applies eq filters', async () => {
    const mockClient = createMockClient()
    const queryBuilder = (mockClient.from as ReturnType<typeof vi.fn>)()

    renderHook(
      () => useQuery<Todo>('todos', { eq: { done: 'false' } }),
      { wrapper: createWrapper(mockClient) },
    )

    await waitFor(() =>
      expect(queryBuilder.eq).toHaveBeenCalledWith('done', 'false'),
    )
  })

  it('applies limit and offset', async () => {
    const mockClient = createMockClient()
    const queryBuilder = (mockClient.from as ReturnType<typeof vi.fn>)()

    renderHook(
      () => useQuery<Todo>('todos', { limit: 10, offset: 5 }),
      { wrapper: createWrapper(mockClient) },
    )

    await waitFor(() => {
      expect(queryBuilder.limit).toHaveBeenCalledWith(10)
      expect(queryBuilder.offset).toHaveBeenCalledWith(5)
    })
  })
})
