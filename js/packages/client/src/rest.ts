import { MimDBError } from './errors'
import { FilterBuilder } from './filters'
import { fetchWithRetry } from './retry'
import type { CountMethod, OrderOptions, QueryResult, RequestInterceptor, ResponseInterceptor, RetryOptions } from './types'

/**
 * Builds and executes a PostgREST-compatible query against a single table.
 *
 * Supports SELECT, INSERT, UPDATE, UPSERT, and DELETE operations with
 * fluent filter chaining and PostgREST modifiers (order, limit, offset, etc.).
 *
 * The builder is **thenable** - it can be awaited directly without calling
 * `.execute()` explicitly.
 *
 * @typeParam T - Row type of the table being queried.
 *
 * @example
 * ```ts
 * const { data, error } = await queryBuilder
 *   .select('id, name')
 *   .eq('active', 'true')
 *   .order('name')
 *   .limit(10)
 * ```
 */
/**
 * Middleware configuration passed to each `QueryBuilder` so it can apply
 * interceptors and retry logic during request execution.
 */
export interface QueryMiddleware {
  /** Retry configuration. Undefined means no retries. */
  retry?: RetryOptions
  /** Interceptor applied before each outgoing request. */
  onRequest?: RequestInterceptor
  /** Interceptor applied after each response. */
  onResponse?: ResponseInterceptor
}

export class QueryBuilder<T> extends FilterBuilder<T> {
  private readonly baseUrl: string
  private readonly table: string
  private readonly fetchFn: typeof fetch
  private readonly middleware: QueryMiddleware

  private method: string = 'GET'
  private body: unknown = undefined
  private isMaybeSingle: boolean = false
  private hasSelect: boolean = false
  private isMutation: boolean = false

  /**
   * @param baseUrl        - Full base URL including `/v1/rest/{ref}`.
   * @param table          - Table name to query.
   * @param fetchFn        - Fetch implementation.
   * @param defaultHeaders - Headers applied to every request (auth, content-type, etc.).
   * @param middleware      - Optional retry and interceptor configuration.
   */
  constructor(
    baseUrl: string,
    table: string,
    fetchFn: typeof fetch,
    defaultHeaders: Record<string, string>,
    middleware?: QueryMiddleware,
  ) {
    super(new URLSearchParams(), { ...defaultHeaders })
    this.baseUrl = baseUrl
    this.table = table
    this.fetchFn = fetchFn
    this.middleware = middleware ?? {}
  }

  // ---------------------------------------------------------------------------
  // Query methods
  // ---------------------------------------------------------------------------

  /**
   * Build a SELECT query, or request representation after a mutation.
   *
   * When called on a fresh builder, starts a SELECT (GET) query.
   * When chained after `insert()`, `update()`, `upsert()`, or `delete()`,
   * it requests the affected rows back via `Prefer: return=representation`.
   *
   * @param columns - Comma-separated column list. Defaults to `'*'`.
   * @returns This builder for chaining filters and modifiers.
   */
  select(columns: string = '*'): this {
    // Only set GET when select() is the primary operation, not after a mutation
    if (!this.isMutation) {
      this.method = 'GET'
    }
    this.params.set('select', columns)
    this.hasSelect = true
    return this
  }

  /**
   * Build an INSERT query.
   *
   * The inserted rows are returned only when `.select()` is chained after.
   *
   * @param data - Row or rows to insert.
   * @returns This builder for chaining.
   */
  insert(data: Partial<T> | Partial<T>[]): this {
    this.method = 'POST'
    this.body = data
    this.isMutation = true
    return this
  }

  /**
   * Build an UPDATE query.
   *
   * Always combine with at least one filter to avoid updating every row.
   * Updated rows are returned only when `.select()` is chained after.
   *
   * @param data - Fields to update.
   * @returns This builder for chaining.
   */
  update(data: Partial<T>): this {
    this.method = 'PATCH'
    this.body = data
    this.isMutation = true
    return this
  }

  /**
   * Build an UPSERT query (insert-or-update).
   *
   * Uses PostgREST `resolution=merge-duplicates` to merge on conflict.
   * Upserted rows are returned only when `.select()` is chained after.
   *
   * @param data - Row or rows to upsert.
   * @returns This builder for chaining.
   */
  upsert(data: Partial<T> | Partial<T>[]): this {
    this.method = 'POST'
    this.body = data
    this.isMutation = true
    this.appendPrefer('resolution=merge-duplicates')
    return this
  }

  /**
   * Build a DELETE query.
   *
   * Always combine with at least one filter to avoid deleting every row.
   *
   * @returns This builder for chaining.
   */
  delete(): this {
    this.method = 'DELETE'
    this.isMutation = true
    return this
  }

  // ---------------------------------------------------------------------------
  // Modifier methods
  // ---------------------------------------------------------------------------

  /**
   * Order the result set by a column.
   *
   * @param column  - Column to order by.
   * @param options - Direction and null placement options.
   * @returns This builder for chaining.
   */
  order(column: string, options?: OrderOptions): this {
    const direction = options?.ascending === false ? 'desc' : 'asc'
    const nulls = options?.nullsFirst === true ? '.nullsfirst' : options?.nullsFirst === false ? '.nullslast' : ''

    const value = `${column}.${direction}${nulls}`

    if (options?.foreignTable) {
      this.params.append(`${options.foreignTable}.order`, value)
    } else {
      this.params.append('order', value)
    }

    return this
  }

  /**
   * Limit the number of rows returned.
   *
   * @param count - Maximum number of rows.
   * @returns This builder for chaining.
   */
  limit(count: number): this {
    this.params.set('limit', String(count))
    return this
  }

  /**
   * Skip the first `count` rows.
   *
   * @param count - Number of rows to skip.
   * @returns This builder for chaining.
   */
  offset(count: number): this {
    this.params.set('offset', String(count))
    return this
  }

  /**
   * Restrict the result to rows within the given index range (inclusive).
   *
   * Sets the `Range` header for PostgREST/MimREST keyset pagination.
   * The range is zero-based and inclusive on both ends.
   *
   * @param from - Start index (inclusive).
   * @param to   - End index (inclusive).
   * @returns This builder for chaining.
   *
   * @example
   * ```ts
   * // Get rows 0 through 49 (first 50)
   * const { data } = await mimdb.from('players').select('*').range(0, 49)
   * ```
   */
  range(from: number, to: number): this {
    this.headers['Range'] = `${from}-${to}`
    this.headers['Range-Unit'] = 'items'
    return this
  }

  /**
   * Expect exactly one row in the response.
   *
   * Sets the `Accept` header to `application/vnd.pgrst.object+json` so
   * PostgREST returns a single object instead of an array. Returns an error
   * if zero or multiple rows match.
   *
   * @returns This builder for chaining.
   */
  single(): this {
    this.headers['Accept'] = 'application/vnd.pgrst.object+json'
    return this
  }

  /**
   * Expect at most one row in the response.
   *
   * Like `single()`, but returns `null` data instead of an error when
   * zero rows match.
   *
   * @returns This builder for chaining.
   */
  maybeSingle(): this {
    this.isMaybeSingle = true
    this.headers['Accept'] = 'application/vnd.pgrst.object+json'
    return this
  }

  /**
   * Request a row count alongside the data.
   *
   * @param method - Count strategy: 'exact', 'planned', or 'estimated'.
   * @returns This builder for chaining.
   */
  count(method: CountMethod = 'exact'): this {
    this.appendPrefer(`count=${method}`)
    return this
  }

  // ---------------------------------------------------------------------------
  // Thenable interface
  // ---------------------------------------------------------------------------

  /**
   * Makes the builder thenable so it can be awaited directly.
   *
   * @param onfulfilled - Callback invoked with the `QueryResult`.
   * @param onrejected  - Callback invoked if execution fails unexpectedly.
   * @returns A promise that resolves with the query result.
   */
  then<TResult1 = QueryResult<T>, TResult2 = never>(
    onfulfilled?: ((value: QueryResult<T>) => TResult1 | PromiseLike<TResult1>) | null,
    onrejected?: ((reason: unknown) => TResult2 | PromiseLike<TResult2>) | null,
  ): Promise<TResult1 | TResult2> {
    return this.execute().then(onfulfilled, onrejected)
  }

  // ---------------------------------------------------------------------------
  // Execution
  // ---------------------------------------------------------------------------

  /**
   * Build the URL, execute the fetch call, and parse the response.
   *
   * @returns The query result containing data, error, count, and HTTP status.
   */
  private async execute(): Promise<QueryResult<T>> {
    // For mutations with a chained select, request representation back
    if (this.isMutation && this.hasSelect) {
      this.appendPrefer('return=representation')
    }

    const queryString = this.params.toString()
    const url = `${this.baseUrl}/${this.table}${queryString ? `?${queryString}` : ''}`

    let init: RequestInit = {
      method: this.method,
      headers: { ...this.headers },
    }

    if (this.body !== undefined) {
      init.body = JSON.stringify(this.body)
    }

    // Apply request interceptor
    if (this.middleware.onRequest) {
      init = await this.middleware.onRequest(url, init)
    }

    try {
      let response: Response

      if (this.middleware.retry) {
        response = await fetchWithRetry(this.fetchFn, url, init, this.middleware.retry)
      } else {
        response = await this.fetchFn(url, init)
      }

      // Apply response interceptor
      if (this.middleware.onResponse) {
        response = await this.middleware.onResponse(response)
      }

      if (!response.ok) {
        // For maybeSingle, PostgREST returns 406 for both zero-row and
        // multi-row results. Only suppress the zero-row case; a multi-row
        // result is a real error the caller needs to know about.
        if (this.isMaybeSingle && response.status === 406) {
          const error = await MimDBError.fromResponse(response)
          const combined = `${error.code} ${error.message} ${error.detail ?? ''}`.toLowerCase()
          if (error.code === 'PGRST106' || combined.includes('0 rows') || combined.includes('zero')) {
            return {
              data: null,
              error: null,
              count: null,
              status: response.status,
              statusText: response.statusText,
            }
          }
          // Multi-row 406 - fall through to return the error
          return {
            data: null,
            error,
            count: null,
            status: response.status,
            statusText: response.statusText,
          }
        }

        const error = await MimDBError.fromResponse(response)
        return {
          data: null,
          error,
          count: null,
          status: response.status,
          statusText: response.statusText,
        }
      }

      // Parse count from content-range header (format: "0-24/100" or "*/100")
      let count: number | null = null
      const contentRange = response.headers.get('content-range')
      if (contentRange) {
        const match = /\/(\d+)/.exec(contentRange)
        if (match?.[1]) {
          count = parseInt(match[1], 10)
        }
      }

      // Parse response body
      let data: T | null = null
      const text = await response.text()
      if (text) {
        data = JSON.parse(text) as T
      }

      return {
        data,
        error: null,
        count,
        status: response.status,
        statusText: response.statusText,
      }
    } catch (err) {
      const error = err instanceof MimDBError
        ? err
        : new MimDBError(
            err instanceof Error ? err.message : 'Unknown error',
            'FETCH_ERROR',
            0,
          )

      return {
        data: null,
        error,
        count: null,
        status: 0,
        statusText: 'Fetch Error',
      }
    }
  }

  // ---------------------------------------------------------------------------
  // Internal helpers
  // ---------------------------------------------------------------------------

  /**
   * Append a value to the `Prefer` header, comma-separating multiple values.
   *
   * @param value - Prefer directive to add (e.g. 'return=representation').
   */
  private appendPrefer(value: string): void {
    const existing = this.headers['Prefer']
    this.headers['Prefer'] = existing ? `${existing}, ${value}` : value
  }
}
