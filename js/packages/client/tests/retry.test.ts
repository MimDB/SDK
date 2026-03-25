import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { fetchWithRetry } from '../src/retry'

describe('fetchWithRetry', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('returns the response on success without retrying', async () => {
    const fetchFn = vi.fn().mockResolvedValue(
      new Response('ok', { status: 200 }),
    ) as unknown as typeof fetch

    const promise = fetchWithRetry(fetchFn, 'https://api.test/data', {})
    const response = await promise

    expect(response.status).toBe(200)
    expect(fetchFn).toHaveBeenCalledTimes(1)
  })

  it('retries on 502 and eventually succeeds', async () => {
    const fetchFn = vi.fn()
      .mockResolvedValueOnce(new Response('bad', { status: 502 }))
      .mockResolvedValueOnce(new Response('ok', { status: 200 })) as unknown as typeof fetch

    const promise = fetchWithRetry(fetchFn, 'https://api.test/data', {}, {
      maxRetries: 3,
      baseDelay: 100,
    })

    // First call returns 502 immediately, then waits baseDelay * 2^0 = 100ms
    await vi.advanceTimersByTimeAsync(150)

    const response = await promise
    expect(response.status).toBe(200)
    expect(fetchFn).toHaveBeenCalledTimes(2)
  })

  it('retries on 503 and 504', async () => {
    const fetchFn = vi.fn()
      .mockResolvedValueOnce(new Response('bad', { status: 503 }))
      .mockResolvedValueOnce(new Response('bad', { status: 504 }))
      .mockResolvedValueOnce(new Response('ok', { status: 200 })) as unknown as typeof fetch

    const promise = fetchWithRetry(fetchFn, 'https://api.test/data', {}, {
      maxRetries: 3,
      baseDelay: 50,
    })

    // First retry: 50ms
    await vi.advanceTimersByTimeAsync(60)
    // Second retry: 100ms
    await vi.advanceTimersByTimeAsync(110)

    const response = await promise
    expect(response.status).toBe(200)
    expect(fetchFn).toHaveBeenCalledTimes(3)
  })

  it('does not retry on 4xx errors', async () => {
    const fetchFn = vi.fn().mockResolvedValue(
      new Response('not found', { status: 404 }),
    ) as unknown as typeof fetch

    const promise = fetchWithRetry(fetchFn, 'https://api.test/data', {}, {
      maxRetries: 3,
      baseDelay: 100,
    })

    const response = await promise
    expect(response.status).toBe(404)
    expect(fetchFn).toHaveBeenCalledTimes(1)
  })

  it('respects maxRetries and returns the last response', async () => {
    const fetchFn = vi.fn().mockResolvedValue(
      new Response('bad gateway', { status: 502 }),
    ) as unknown as typeof fetch

    const promise = fetchWithRetry(fetchFn, 'https://api.test/data', {}, {
      maxRetries: 2,
      baseDelay: 50,
    })

    // Attempt 0 -> wait 50ms
    await vi.advanceTimersByTimeAsync(60)
    // Attempt 1 -> wait 100ms
    await vi.advanceTimersByTimeAsync(110)

    const response = await promise
    expect(response.status).toBe(502)
    // 1 initial + 2 retries = 3 total
    expect(fetchFn).toHaveBeenCalledTimes(3)
  })

  it('retries on network errors and throws after exhausting retries', async () => {
    vi.useRealTimers()

    const fetchFn = vi.fn().mockRejectedValue(
      new Error('Network failure'),
    ) as unknown as typeof fetch

    await expect(
      fetchWithRetry(fetchFn, 'https://api.test/data', {}, {
        maxRetries: 1,
        baseDelay: 1,
        maxDelay: 1,
      }),
    ).rejects.toThrow('Network failure')

    expect(fetchFn).toHaveBeenCalledTimes(2)

    vi.useFakeTimers()
  })

  it('caps delay at maxDelay', async () => {
    const fetchFn = vi.fn()
      .mockResolvedValueOnce(new Response('bad', { status: 502 }))
      .mockResolvedValueOnce(new Response('bad', { status: 502 }))
      .mockResolvedValueOnce(new Response('ok', { status: 200 })) as unknown as typeof fetch

    const promise = fetchWithRetry(fetchFn, 'https://api.test/data', {}, {
      maxRetries: 3,
      baseDelay: 1000,
      maxDelay: 500,
    })

    // Both delays should be capped at 500ms
    await vi.advanceTimersByTimeAsync(510)
    await vi.advanceTimersByTimeAsync(510)

    const response = await promise
    expect(response.status).toBe(200)
  })

  it('supports custom retryOn status codes', async () => {
    const fetchFn = vi.fn()
      .mockResolvedValueOnce(new Response('rate limited', { status: 429 }))
      .mockResolvedValueOnce(new Response('ok', { status: 200 })) as unknown as typeof fetch

    const promise = fetchWithRetry(fetchFn, 'https://api.test/data', {}, {
      maxRetries: 2,
      baseDelay: 50,
      retryOn: [429],
    })

    await vi.advanceTimersByTimeAsync(60)

    const response = await promise
    expect(response.status).toBe(200)
    expect(fetchFn).toHaveBeenCalledTimes(2)
  })
})
