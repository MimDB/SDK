import type { TextSearchOptions } from './types'

/**
 * Base class providing PostgREST filter methods.
 *
 * Each filter appends the appropriate query parameter to an internal
 * `URLSearchParams` instance and returns `this` for fluent chaining.
 *
 * @typeParam T - Row type of the table being queried.
 */
export class FilterBuilder<_T = unknown> {
  /** @internal Accumulated query-string parameters. */
  protected readonly params: URLSearchParams

  /** @internal Headers specific to this query. */
  protected readonly headers: Record<string, string>

  constructor(params: URLSearchParams, headers: Record<string, string>) {
    this.params = params
    this.headers = headers
  }

  /**
   * Filter rows where `column` equals `value`.
   *
   * @param column - Column name.
   * @param value  - Value to compare against.
   * @returns This builder for chaining.
   */
  eq(column: string, value: string): this {
    this.params.append(column, `eq.${value}`)
    return this
  }

  /**
   * Filter rows where `column` does not equal `value`.
   *
   * @param column - Column name.
   * @param value  - Value to compare against.
   * @returns This builder for chaining.
   */
  neq(column: string, value: string): this {
    this.params.append(column, `neq.${value}`)
    return this
  }

  /**
   * Filter rows where `column` is greater than `value`.
   *
   * @param column - Column name.
   * @param value  - Value to compare against.
   * @returns This builder for chaining.
   */
  gt(column: string, value: string): this {
    this.params.append(column, `gt.${value}`)
    return this
  }

  /**
   * Filter rows where `column` is greater than or equal to `value`.
   *
   * @param column - Column name.
   * @param value  - Value to compare against.
   * @returns This builder for chaining.
   */
  gte(column: string, value: string): this {
    this.params.append(column, `gte.${value}`)
    return this
  }

  /**
   * Filter rows where `column` is less than `value`.
   *
   * @param column - Column name.
   * @param value  - Value to compare against.
   * @returns This builder for chaining.
   */
  lt(column: string, value: string): this {
    this.params.append(column, `lt.${value}`)
    return this
  }

  /**
   * Filter rows where `column` is less than or equal to `value`.
   *
   * @param column - Column name.
   * @param value  - Value to compare against.
   * @returns This builder for chaining.
   */
  lte(column: string, value: string): this {
    this.params.append(column, `lte.${value}`)
    return this
  }

  /**
   * Filter rows where `column` matches a case-sensitive `LIKE` pattern.
   *
   * @param column  - Column name.
   * @param pattern - SQL LIKE pattern (use `%` for wildcards).
   * @returns This builder for chaining.
   */
  like(column: string, pattern: string): this {
    this.params.append(column, `like.${pattern}`)
    return this
  }

  /**
   * Filter rows where `column` matches a case-insensitive `ILIKE` pattern.
   *
   * @param column  - Column name.
   * @param pattern - SQL ILIKE pattern (use `%` for wildcards).
   * @returns This builder for chaining.
   */
  ilike(column: string, pattern: string): this {
    this.params.append(column, `ilike.${pattern}`)
    return this
  }

  /**
   * Filter rows where `column` IS a specific value.
   *
   * Use for `null`, `true`, or `false` comparisons.
   *
   * @param column - Column name.
   * @param value  - One of 'null', 'true', or 'false'.
   * @returns This builder for chaining.
   */
  is(column: string, value: 'null' | 'true' | 'false'): this {
    this.params.append(column, `is.${value}`)
    return this
  }

  /**
   * Filter rows where `column` is one of the provided values.
   *
   * @param column - Column name.
   * @param values - Array of values to match against.
   * @returns This builder for chaining.
   */
  in(column: string, values: string[]): this {
    this.params.append(column, `in.(${values.join(',')})`)
    return this
  }

  /**
   * Filter rows where `column` contains (is a superset of) `value`.
   *
   * Works with arrays, ranges, and JSON columns.
   *
   * @param column - Column name.
   * @param value  - Value to check containment against.
   * @returns This builder for chaining.
   */
  contains(column: string, value: string): this {
    this.params.append(column, `cs.${value}`)
    return this
  }

  /**
   * Filter rows where `column` is contained by (is a subset of) `value`.
   *
   * Works with arrays, ranges, and JSON columns.
   *
   * @param column - Column name.
   * @param value  - Value to check containment against.
   * @returns This builder for chaining.
   */
  containedBy(column: string, value: string): this {
    this.params.append(column, `cd.${value}`)
    return this
  }

  /**
   * Negate a filter operator on `column`.
   *
   * @example
   * ```ts
   * builder.not('status', 'eq', 'deleted')
   * // produces: status=not.eq.deleted
   * ```
   *
   * @param column - Column name.
   * @param op     - PostgREST operator to negate (e.g. 'eq', 'in').
   * @param value  - Value for the operator.
   * @returns This builder for chaining.
   */
  not(column: string, op: string, value: string): this {
    this.params.append(column, `not.${op}.${value}`)
    return this
  }

  /**
   * Combine multiple filters with a logical OR.
   *
   * @example
   * ```ts
   * builder.or('status.eq.active,status.eq.pending')
   * // produces: or=(status.eq.active,status.eq.pending)
   * ```
   *
   * @param filterString - Comma-separated PostgREST filter expressions.
   * @returns This builder for chaining.
   */
  or(filterString: string): this {
    this.params.append('or', `(${filterString})`)
    return this
  }

  /**
   * Perform a full-text search on `column`.
   *
   * @param column  - Column name (or a tsvector column).
   * @param query   - The search query string.
   * @param options - Optional search type and configuration.
   * @returns This builder for chaining.
   */
  textSearch(column: string, query: string, options?: TextSearchOptions): this {
    const type = options?.type ?? 'fts'
    const operatorMap: Record<string, string> = {
      fts: 'fts',
      plain: 'plfts',
      phrase: 'phfts',
      web: 'wfts',
    }
    const op = operatorMap[type] ?? 'fts'
    const config = options?.config ? `(${options.config})` : ''
    this.params.append(column, `${op}${config}.${query}`)
    return this
  }
}
