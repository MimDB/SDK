import {
  useMutation as useTanstackMutation,
  useQueryClient,
  type UseMutationResult,
} from '@tanstack/react-query'
import { useClient } from './context'
import type { MimDBError } from '@mimdb/client'

/**
 * React hook for inserting a row into a MimDB table.
 *
 * On success, all `useQuery` caches for the same table are automatically
 * invalidated so lists stay in sync.
 *
 * @typeParam T - Expected row type. Defaults to a generic record.
 * @param table - Name of the database table.
 * @returns A TanStack `UseMutationResult` whose `mutate` / `mutateAsync`
 *          accepts a partial row to insert.
 *
 * @example
 * ```tsx
 * const insert = useInsert<Todo>('todos')
 * insert.mutate({ task: 'Buy milk', done: false })
 * ```
 */
export function useInsert<T = Record<string, unknown>>(
  table: string,
): UseMutationResult<T, MimDBError, Partial<T>> {
  const client = useClient()
  const queryClient = useQueryClient()

  return useTanstackMutation({
    mutationFn: async (data: Partial<T>) => {
      const { data: result, error } = await client
        .from<T>(table)
        .insert(data)
        .select()
        .single()
      if (error) throw error
      return result as T
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['mimdb', table] })
    },
  })
}

/**
 * Input shape for the {@link useUpdate} mutation.
 *
 * @typeParam T - Expected row type.
 */
export interface UpdateInput<T> {
  /** Fields to update. */
  data: Partial<T>
  /** Equality filters identifying which rows to update. */
  eq: Record<string, string>
}

/**
 * React hook for updating rows in a MimDB table.
 *
 * The mutation function accepts an object with `data` (fields to set) and
 * `eq` (equality filters to target specific rows). On success, all
 * `useQuery` caches for the same table are invalidated.
 *
 * @typeParam T - Expected row type. Defaults to a generic record.
 * @param table - Name of the database table.
 * @returns A TanStack `UseMutationResult`.
 *
 * @example
 * ```tsx
 * const update = useUpdate<Todo>('todos')
 * update.mutate({ data: { done: true }, eq: { id: '42' } })
 * ```
 */
export function useUpdate<T = Record<string, unknown>>(
  table: string,
): UseMutationResult<T, MimDBError, UpdateInput<T>> {
  const client = useClient()
  const queryClient = useQueryClient()

  return useTanstackMutation({
    mutationFn: async ({ data, eq }: UpdateInput<T>) => {
      let query = client.from<T>(table).update(data)
      for (const [col, val] of Object.entries(eq)) {
        query = query.eq(col, val)
      }
      const { data: result, error } = await query.select().single()
      if (error) throw error
      return result as T
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['mimdb', table] })
    },
  })
}

/**
 * React hook for deleting rows from a MimDB table.
 *
 * The mutation function accepts equality filters identifying which rows
 * to delete. On success, all `useQuery` caches for the same table are
 * invalidated.
 *
 * @param table - Name of the database table.
 * @returns A TanStack `UseMutationResult` whose `mutate` accepts equality filters.
 *
 * @example
 * ```tsx
 * const del = useDelete('todos')
 * del.mutate({ id: '42' })
 * ```
 */
export function useDelete(
  table: string,
): UseMutationResult<void, MimDBError, Record<string, string>> {
  const client = useClient()
  const queryClient = useQueryClient()

  return useTanstackMutation({
    mutationFn: async (eq: Record<string, string>) => {
      let query = client.from(table).delete()
      for (const [col, val] of Object.entries(eq)) {
        query = query.eq(col, val)
      }
      const { error } = await query
      if (error) throw error
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['mimdb', table] })
    },
  })
}
