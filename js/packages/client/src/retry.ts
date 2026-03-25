import type { RetryOptions } from './types'

/**
 * Execute a fetch request with automatic retry and exponential backoff
 * for transient server errors.
 *
 * Retries occur when the response status is in `retryOn` (default:
 * 502, 503, 504) or when the fetch itself throws (e.g. network failure).
 * Delay between attempts grows exponentially: `baseDelay * 2^attempt`,
 * capped at `maxDelay`.
 *
 * @param fetchFn - The fetch implementation to use.
 * @param url     - Request URL.
 * @param init    - Request init options.
 * @param options - Retry configuration.
 * @returns The successful `Response`, or throws the last error after exhausting retries.
 */
export async function fetchWithRetry(
  fetchFn: typeof fetch,
  url: string,
  init: RequestInit,
  options?: RetryOptions,
): Promise<Response> {
  const maxRetries = options?.maxRetries ?? 3
  const baseDelay = options?.baseDelay ?? 1000
  const maxDelay = options?.maxDelay ?? 10_000
  const retryOn = options?.retryOn ?? [502, 503, 504]

  let lastError: Error | null = null

  for (let attempt = 0; attempt <= maxRetries; attempt++) {
    try {
      const response = await fetchFn(url, init)

      if (attempt < maxRetries && retryOn.includes(response.status)) {
        const delay = Math.min(baseDelay * Math.pow(2, attempt), maxDelay)
        await new Promise((resolve) => setTimeout(resolve, delay))
        continue
      }

      return response
    } catch (err) {
      lastError = err instanceof Error ? err : new Error(String(err))

      if (attempt < maxRetries) {
        const delay = Math.min(baseDelay * Math.pow(2, attempt), maxDelay)
        await new Promise((resolve) => setTimeout(resolve, delay))
        continue
      }
    }
  }

  throw lastError ?? new Error('Retry exhausted')
}
