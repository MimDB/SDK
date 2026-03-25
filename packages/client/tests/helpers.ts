import { vi } from 'vitest'

/**
 * Create a mock fetch function that resolves with the given status and body.
 *
 * @param status - HTTP status code.
 * @param body   - Response body (will be JSON-serialized).
 * @param headers - Optional response headers.
 * @returns A vi.fn() mock typed as `typeof fetch`.
 */
export function mockFetch(
  status: number,
  body: unknown,
  headers?: Record<string, string>,
): typeof fetch {
  return vi.fn().mockResolvedValue(
    new Response(JSON.stringify(body), {
      status,
      statusText: statusTextFor(status),
      headers: {
        'content-type': 'application/json',
        ...headers,
      },
    }),
  ) as unknown as typeof fetch
}

/**
 * Return a conventional status text for common HTTP codes.
 */
function statusTextFor(status: number): string {
  const map: Record<number, string> = {
    200: 'OK',
    201: 'Created',
    204: 'No Content',
    400: 'Bad Request',
    401: 'Unauthorized',
    404: 'Not Found',
    406: 'Not Acceptable',
    409: 'Conflict',
    500: 'Internal Server Error',
  }
  return map[status] ?? 'Unknown'
}
