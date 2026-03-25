import { describe, expect, it } from 'vitest'
import { MimDBError } from '../src/errors'

describe('MimDBError', () => {
  it('sets name, message, code, status, and hint', () => {
    const err = new MimDBError('not found', 'PGRST116', 404, 'Check the table name')

    expect(err).toBeInstanceOf(Error)
    expect(err.name).toBe('MimDBError')
    expect(err.message).toBe('not found')
    expect(err.code).toBe('PGRST116')
    expect(err.status).toBe(404)
    expect(err.hint).toBe('Check the table name')
  })

  it('hint is optional', () => {
    const err = new MimDBError('bad request', 'HTTP-400', 400)

    expect(err.hint).toBeUndefined()
  })

  describe('fromResponse', () => {
    it('parses a valid API error JSON body', async () => {
      const body = {
        message: 'relation "missing" does not exist',
        code: 'PGRST204',
        hint: 'Check the schema cache',
      }

      const response = new Response(JSON.stringify(body), {
        status: 404,
        statusText: 'Not Found',
        headers: { 'content-type': 'application/json' },
      })

      const err = await MimDBError.fromResponse(response)

      expect(err.message).toBe('relation "missing" does not exist')
      expect(err.code).toBe('PGRST204')
      expect(err.status).toBe(404)
      expect(err.hint).toBe('Check the schema cache')
    })

    it('parses the backend envelope error format', async () => {
      const body = {
        data: null,
        error: {
          code: 'AUTH-0300',
          message: 'Invalid credentials',
          detail: 'Email or password is incorrect',
        },
        meta: { request_id: 'req-123' },
      }

      const response = new Response(JSON.stringify(body), {
        status: 401,
        statusText: 'Unauthorized',
      })

      const err = await MimDBError.fromResponse(response)

      expect(err.message).toBe('Invalid credentials')
      expect(err.code).toBe('AUTH-0300')
      expect(err.status).toBe(401)
      expect(err.hint).toBe('Email or password is incorrect')
    })

    it('parses a nested error object', async () => {
      const body = {
        error: {
          message: 'JWT expired',
          code: 'AUTH01',
        },
      }

      const response = new Response(JSON.stringify(body), {
        status: 401,
        statusText: 'Unauthorized',
      })

      const err = await MimDBError.fromResponse(response)

      expect(err.message).toBe('JWT expired')
      expect(err.code).toBe('AUTH01')
      expect(err.status).toBe(401)
    })

    it('uses error_code when code is absent', async () => {
      const body = {
        message: 'Rate limited',
        error_code: 'RATE_LIMIT',
      }

      const response = new Response(JSON.stringify(body), {
        status: 429,
        statusText: 'Too Many Requests',
      })

      const err = await MimDBError.fromResponse(response)

      expect(err.code).toBe('RATE_LIMIT')
    })

    it('falls back to statusText when body is not JSON', async () => {
      const response = new Response('Internal error occurred', {
        status: 500,
        statusText: 'Internal Server Error',
      })

      const err = await MimDBError.fromResponse(response)

      expect(err.message).toBe('Internal Server Error')
      expect(err.code).toBe('HTTP-500')
      expect(err.status).toBe(500)
      expect(err.hint).toBeUndefined()
    })

    it('falls back gracefully when JSON has missing fields', async () => {
      const body = {}

      const response = new Response(JSON.stringify(body), {
        status: 503,
        statusText: 'Service Unavailable',
      })

      const err = await MimDBError.fromResponse(response)

      expect(err.message).toBe('Service Unavailable')
      expect(err.code).toBe('HTTP-503')
      expect(err.status).toBe(503)
    })
  })
})
