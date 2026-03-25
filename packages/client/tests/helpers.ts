import { vi } from 'vitest'

/**
 * Wrap raw data in the backend's API envelope format.
 *
 * @param data - The payload to place in the `data` field.
 * @returns An envelope object matching `{ data, error, meta }`.
 */
export function envelope<T>(data: T): { data: T; error: null; meta: { request_id: string } } {
  return { data, error: null, meta: { request_id: 'test-req-id' } }
}

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
 * Create a mock fetch for 204 No Content responses (no body).
 *
 * @returns A vi.fn() mock typed as `typeof fetch`.
 */
export function mockFetchNoContent(): typeof fetch {
  return vi.fn().mockResolvedValue(
    new Response(null, {
      status: 204,
      statusText: 'No Content',
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
