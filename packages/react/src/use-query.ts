import {
  useQuery as useTanstackQuery,
  type UseQueryResult,
} from '@tanstack/react-query'
import { useClient } from './context'

/**
 * Options for the {@link useQuery} hook.
 */
export interface UseQueryOptions {
  /** Column selection string (PostgREST format). Defaults to `'*'`. */
  select?: string
  /** Equality filters applied as `query.eq(column, value)`. */
  eq?: Record<string, string>
  /** Not-equal filters applied as `query.neq(column, value)`. */
  neq?: Record<string, string>
  /** Column ordering configuration. */
  order?: { column: string; ascending?: boolean }
  /** Maximum number of rows to return. */
  limit?: number
  /** Number of rows to skip before returning results. */
  offset?: number
  /** Whether the query should execute. Maps to TanStack Query's `enabled`. */
  enabled?: boolean
  /** Duration in ms before cached data is considered stale. */
  staleTime?: number
  /** Polling interval in ms, or `false` to disable. */
  refetchInterval?: number | false
}

/**
 * Fetch rows from a MimDB table using the REST API, backed by TanStack Query
 * for caching, deduplication, and background refetching.
 *
 * Automatically derives a stable query key from the table name and options
 * so cache invalidation works out of the box with the mutation hooks.
 *
 * @typeParam T - Expected row type. Defaults to a generic record.
 * @param table   - Name of the database table to query.
 * @param options - Query filters, modifiers, and TanStack Query settings.
 * @returns A TanStack `UseQueryResult` containing the row array and status flags.
 *
 * @example
 * ```tsx
 * const { data, isLoading } = useQuery<Todo>('todos', {
 *   eq: { done: 'false' },
 *   order: { column: 'created_at', ascending: false },
 *   limit: 20,
 * })
 * ```
 */
export function useQuery<T = Record<string, unknown>>(
  table: string,
  options?: UseQueryOptions,
): UseQueryResult<T[], Error> {
  const client = useClient()

  return useTanstackQuery({
    queryKey: ['mimdb', table, options],
    queryFn: async () => {
      let query = client.from<T>(table).select(options?.select ?? '*')

      if (options?.eq) {
        for (const [col, val] of Object.entries(options.eq)) {
          query = query.eq(col, val)
        }
      }
      if (options?.neq) {
        for (const [col, val] of Object.entries(options.neq)) {
          query = query.neq(col, val)
        }
      }
      if (options?.order) {
        query = query.order(options.order.column, {
          ascending: options.order.ascending,
        })
      }
      if (options?.limit) query = query.limit(options.limit)
      if (options?.offset) query = query.offset(options.offset)

      const { data, error } = await query
      if (error) throw error
      return (data ?? []) as T[]
    },
    enabled: options?.enabled,
    staleTime: options?.staleTime,
    refetchInterval: options?.refetchInterval,
  })
}
