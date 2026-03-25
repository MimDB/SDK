/**
 * Structured error from the MimDB API.
 *
 * Captures the machine-readable error code, HTTP status, and optional hint
 * returned by PostgREST / Mimisbrunnr so callers can programmatically
 * distinguish error types.
 */
export class MimDBError extends Error {
  /** Machine-readable error code (e.g. `PGRST116`, `HTTP-404`). */
  readonly code: string

  /** HTTP status code of the response that produced this error. */
  readonly status: number

  /** Optional hint from PostgREST suggesting how to fix the issue. */
  readonly hint?: string

  /**
   * @param message - Human-readable error description.
   * @param code    - Machine-readable error code.
   * @param status  - HTTP status code.
   * @param hint    - Optional remediation hint from PostgREST.
   */
  constructor(message: string, code: string, status: number, hint?: string) {
    super(message)
    this.name = 'MimDBError'
    this.code = code
    this.status = status
    this.hint = hint
  }

  /**
   * Parse an HTTP error response into a `MimDBError`.
   *
   * Attempts to read the response body as JSON and extract structured
   * error fields. Falls back to the HTTP status text when the body
   * cannot be parsed.
   *
   * @param response - The fetch `Response` object.
   * @returns A `MimDBError` representing the failure.
   */
  static async fromResponse(response: Response): Promise<MimDBError> {
    try {
      const body = await response.json() as Record<string, unknown>
      // The backend always returns { data, error, meta }.
      // PostgREST responses are flat objects with code/message at root.
      const err = (body.error ?? body) as Record<string, unknown>
      return new MimDBError(
        (err.message as string | undefined) ?? response.statusText,
        (err.code as string | undefined) ?? (err.error_code as string | undefined) ?? `HTTP-${response.status}`,
        response.status,
        (err.hint as string | undefined) ?? (err.detail as string | undefined),
      )
    } catch {
      return new MimDBError(
        response.statusText,
        `HTTP-${response.status}`,
        response.status,
      )
    }
  }
}
