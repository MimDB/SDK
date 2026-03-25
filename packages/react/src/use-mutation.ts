import {
  useMutation as useTanstackMutation,
  useQueryClient,
  type UseMutationResult,
} from '@tanstack/react-query'
import { useClient } from './context'
import type { MimDBError } from '@mimdb/client'

/**
 * Options for the {@link useInsert} hook.
 */
export interface UseInsertOptions {
  /**
   * Enable optimistic updates. When true, the new row is appended to
   * the query cache immediately and rolled back on error.
   */
  optimistic?: boolean
}

/**
 * React hook for inserting a row into a MimDB table.
 *
 * On success, all `useQuery` caches for the same table are automatically
 * invalidated so lists stay in sync.
 *
 * When `optimistic` is enabled, the new row is appended to the cache
 * before the server responds. On error the cache is rolled back.
 *
 * @typeParam T - Expected row type. Defaults to a generic record.
 * @param table   - Name of the database table.
 * @param options - Insert hook options.
 * @returns A TanStack `UseMutationResult` whose `mutate` / `mutateAsync`
 *          accepts a partial row to insert.
 *
 * @example
 * ```tsx
 * const insert = useInsert<Todo>('todos', { optimistic: true })
 * insert.mutate({ task: 'Buy milk', done: false })
 * ```
 */
export function useInsert<T = Record<string, unknown>>(
  table: string,
  options?: UseInsertOptions,
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
    onMutate: options?.optimistic
      ? async (newData) => {
          await queryClient.cancelQueries({ queryKey: ['mimdb', table] })
          const previous = queryClient.getQueryData<T[]>(['mimdb', table])
          queryClient.setQueryData<T[]>(['mimdb', table], (old) => [
            ...(old ?? []),
            newData as T,
          ])
          return { previous }
        }
      : undefined,
    onError: options?.optimistic
      ? (_err, _data, context: { previous?: T[] } | undefined) => {
          if (context?.previous !== undefined) {
            queryClient.setQueryData(['mimdb', table], context.previous)
          }
        }
      : undefined,
    onSettled: () => {
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
 * Options for the {@link useUpdate} hook.
 */
export interface UseUpdateOptions {
  /**
   * Enable optimistic updates. When true, matching rows in the cache
   * are updated immediately and rolled back on error.
   */
  optimistic?: boolean
}

/**
 * React hook for updating rows in a MimDB table.
 *
 * The mutation function accepts an object with `data` (fields to set) and
 * `eq` (equality filters to target specific rows). On success, all
 * `useQuery` caches for the same table are invalidated.
 *
 * When `optimistic` is enabled, matching rows in the cache are updated
 * in-place before the server responds and rolled back on error.
 *
 * @typeParam T - Expected row type. Defaults to a generic record.
 * @param table   - Name of the database table.
 * @param options - Update hook options.
 * @returns A TanStack `UseMutationResult`.
 *
 * @example
 * ```tsx
 * const update = useUpdate<Todo>('todos', { optimistic: true })
 * update.mutate({ data: { done: true }, eq: { id: '42' } })
 * ```
 */
export function useUpdate<T = Record<string, unknown>>(
  table: string,
  options?: UseUpdateOptions,
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
    onMutate: options?.optimistic
      ? async ({ data, eq }) => {
          await queryClient.cancelQueries({ queryKey: ['mimdb', table] })
          const previous = queryClient.getQueryData<T[]>(['mimdb', table])
          queryClient.setQueryData<T[]>(['mimdb', table], (old) =>
            (old ?? []).map((row) => {
              const record = row as Record<string, unknown>
              const matches = Object.entries(eq).every(
                ([col, val]) => String(record[col]) === val,
              )
              return matches ? { ...row, ...data } : row
            }),
          )
          return { previous }
        }
      : undefined,
    onError: options?.optimistic
      ? (_err, _data, context: { previous?: T[] } | undefined) => {
          if (context?.previous !== undefined) {
            queryClient.setQueryData(['mimdb', table], context.previous)
          }
        }
      : undefined,
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['mimdb', table] })
    },
  })
}

/**
 * Options for the {@link useDelete} hook.
 */
export interface UseDeleteOptions {
  /**
   * Enable optimistic updates. When true, matching rows are removed
   * from the cache immediately and restored on error.
   */
  optimistic?: boolean
}

/**
 * React hook for deleting rows from a MimDB table.
 *
 * The mutation function accepts equality filters identifying which rows
 * to delete. On success, all `useQuery` caches for the same table are
 * invalidated.
 *
 * When `optimistic` is enabled, matching rows are removed from the cache
 * before the server responds and restored on error.
 *
 * @param table   - Name of the database table.
 * @param options - Delete hook options.
 * @returns A TanStack `UseMutationResult` whose `mutate` accepts equality filters.
 *
 * @example
 * ```tsx
 * const del = useDelete('todos', { optimistic: true })
 * del.mutate({ id: '42' })
 * ```
 */
export function useDelete(
  table: string,
  options?: UseDeleteOptions,
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
    onMutate: options?.optimistic
      ? async (eq) => {
          await queryClient.cancelQueries({ queryKey: ['mimdb', table] })
          const previous = queryClient.getQueryData<Record<string, unknown>[]>(['mimdb', table])
          queryClient.setQueryData<Record<string, unknown>[]>(['mimdb', table], (old) =>
            (old ?? []).filter((row) =>
              !Object.entries(eq).every(
                ([col, val]) => String(row[col]) === val,
              ),
            ),
          )
          return { previous }
        }
      : undefined,
    onError: options?.optimistic
      ? (_err, _data, context: { previous?: Record<string, unknown>[] } | undefined) => {
          if (context?.previous !== undefined) {
            queryClient.setQueryData(['mimdb', table], context.previous)
          }
        }
      : undefined,
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['mimdb', table] })
    },
  })
}
