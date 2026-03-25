import { describe, expect, it, vi } from 'vitest'
import { MimDBClient } from '../src/client'
import { StorageClient, BucketClient } from '../src/storage'
import { mockFetch } from './helpers'

const URL = 'https://api.mimdb.dev'
const REF = 'abc123'
const KEY = 'test-api-key'

describe('MimDBClient.storage', () => {
  it('returns a StorageClient', () => {
    const client = new MimDBClient(URL, REF, KEY, { fetch: mockFetch(200, []) })
    expect(client.storage).toBeInstanceOf(StorageClient)
  })

  it('returns the same instance on repeated access', () => {
    const client = new MimDBClient(URL, REF, KEY, { fetch: mockFetch(200, []) })
    expect(client.storage).toBe(client.storage)
  })
})

describe('StorageClient', () => {
  function createStorage(fetchFn: typeof fetch) {
    return new StorageClient(URL, REF, fetchFn, {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${KEY}`,
      'apikey': KEY,
    })
  }

  describe('.from()', () => {
    it('returns a BucketClient with the correct bucket name', () => {
      const storage = createStorage(mockFetch(200, []))
      const bucket = storage.from('avatars')
      expect(bucket).toBeInstanceOf(BucketClient)
    })
  })

  describe('.createBucket()', () => {
    it('sends POST with name and options', async () => {
      const fetchFn = mockFetch(201, {
        name: 'avatars',
        public: true,
        file_size_limit: 1024,
        allowed_mime_types: ['image/png'],
        created_at: '2025-01-01T00:00:00Z',
        updated_at: '2025-01-01T00:00:00Z',
      })
      const storage = createStorage(fetchFn)

      const bucket = await storage.createBucket('avatars', {
        public: true,
        fileSizeLimit: 1024,
        allowedMimeTypes: ['image/png'],
      })

      expect(bucket.name).toBe('avatars')
      expect(bucket.public).toBe(true)

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = call[0] as string
      const init = call[1] as RequestInit
      expect(url).toBe(`${URL}/v1/storage/${REF}/buckets`)
      expect(init.method).toBe('POST')
      expect(JSON.parse(init.body as string)).toEqual({
        name: 'avatars',
        public: true,
        file_size_limit: 1024,
        allowed_mime_types: ['image/png'],
      })
    })
  })

  describe('.listBuckets()', () => {
    it('sends GET to the buckets endpoint', async () => {
      const fetchFn = mockFetch(200, [
        { name: 'avatars', public: true, file_size_limit: null, allowed_mime_types: null, created_at: '', updated_at: '' },
      ])
      const storage = createStorage(fetchFn)

      const buckets = await storage.listBuckets()

      expect(buckets).toHaveLength(1)
      expect(buckets[0]!.name).toBe('avatars')

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = call[0] as string
      const init = call[1] as RequestInit
      expect(url).toBe(`${URL}/v1/storage/${REF}/buckets`)
      expect(init.method).toBe('GET')
    })
  })

  describe('.deleteBucket()', () => {
    it('sends DELETE to the bucket endpoint', async () => {
      const fetchFn = mockFetch(200, {})
      const storage = createStorage(fetchFn)

      await storage.deleteBucket('avatars')

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = call[0] as string
      const init = call[1] as RequestInit
      expect(url).toBe(`${URL}/v1/storage/${REF}/buckets/avatars`)
      expect(init.method).toBe('DELETE')
    })
  })

  describe('.updateBucket()', () => {
    it('sends PATCH with updated options', async () => {
      const fetchFn = mockFetch(200, {
        name: 'avatars',
        public: false,
        file_size_limit: 2048,
        allowed_mime_types: null,
        created_at: '',
        updated_at: '',
      })
      const storage = createStorage(fetchFn)

      const bucket = await storage.updateBucket('avatars', {
        public: false,
        fileSizeLimit: 2048,
      })

      expect(bucket.public).toBe(false)

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = call[0] as string
      const init = call[1] as RequestInit
      expect(url).toBe(`${URL}/v1/storage/${REF}/buckets/avatars`)
      expect(init.method).toBe('PATCH')
      expect(JSON.parse(init.body as string)).toEqual({
        public: false,
        file_size_limit: 2048,
      })
    })
  })
})

describe('BucketClient', () => {
  const defaultHeaders: Record<string, string> = {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${KEY}`,
    'apikey': KEY,
  }

  function createBucket(fetchFn: typeof fetch) {
    return new BucketClient('avatars', URL, REF, fetchFn, { ...defaultHeaders })
  }

  describe('.upload()', () => {
    it('sends POST with correct path and Content-Type', async () => {
      const fetchFn = mockFetch(200, { path: 'photo.png' })
      const bucket = createBucket(fetchFn)

      const result = await bucket.upload('photo.png', 'file-contents', {
        contentType: 'image/png',
      })

      expect(result.path).toBe('photo.png')

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = call[0] as string
      const init = call[1] as RequestInit
      expect(url).toBe(`${URL}/v1/storage/${REF}/object/avatars/photo.png`)
      expect(init.method).toBe('POST')

      const headers = init.headers as Record<string, string>
      expect(headers['Content-Type']).toBe('image/png')
    })

    it('uses application/octet-stream as default Content-Type', async () => {
      const fetchFn = mockFetch(200, { path: 'data.bin' })
      const bucket = createBucket(fetchFn)

      await bucket.upload('data.bin', 'binary-data')

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const init = call[1] as RequestInit
      const headers = init.headers as Record<string, string>
      expect(headers['Content-Type']).toBe('application/octet-stream')
    })

    it('sends x-upsert header when upsert is true', async () => {
      const fetchFn = mockFetch(200, { path: 'photo.png' })
      const bucket = createBucket(fetchFn)

      await bucket.upload('photo.png', 'file-contents', { upsert: true })

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const init = call[1] as RequestInit
      const headers = init.headers as Record<string, string>
      expect(headers['x-upsert']).toBe('true')
    })

    it('does not send x-upsert header when upsert is false or omitted', async () => {
      const fetchFn = mockFetch(200, { path: 'photo.png' })
      const bucket = createBucket(fetchFn)

      await bucket.upload('photo.png', 'file-contents')

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const init = call[1] as RequestInit
      const headers = init.headers as Record<string, string>
      expect(headers['x-upsert']).toBeUndefined()
    })
  })

  describe('.download()', () => {
    it('sends GET and returns blob', async () => {
      const blobContent = 'file-data'
      const fetchFn = vi.fn().mockResolvedValue(
        new Response(blobContent, {
          status: 200,
          statusText: 'OK',
          headers: { 'content-type': 'application/octet-stream' },
        }),
      ) as unknown as typeof fetch
      const bucket = createBucket(fetchFn)

      const blob = await bucket.download('photo.png')

      expect(blob).toBeInstanceOf(Blob)
      const text = await blob.text()
      expect(text).toBe('file-data')

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = call[0] as string
      const init = call[1] as RequestInit
      expect(url).toBe(`${URL}/v1/storage/${REF}/object/avatars/photo.png`)
      expect(init.method).toBe('GET')
    })
  })

  describe('.remove()', () => {
    it('sends DELETE for each path', async () => {
      const fetchFn = mockFetch(200, {})
      const bucket = createBucket(fetchFn)

      await bucket.remove(['a.png', 'b.png', 'c.png'])

      const calls = vi.mocked(fetchFn).mock.calls
      expect(calls).toHaveLength(3)

      expect((calls[0]![0] as string)).toBe(`${URL}/v1/storage/${REF}/object/avatars/a.png`)
      expect((calls[1]![0] as string)).toBe(`${URL}/v1/storage/${REF}/object/avatars/b.png`)
      expect((calls[2]![0] as string)).toBe(`${URL}/v1/storage/${REF}/object/avatars/c.png`)

      for (const call of calls) {
        const init = call[1] as RequestInit
        expect(init.method).toBe('DELETE')
      }
    })
  })

  describe('.createSignedUrl()', () => {
    it('sends POST with expiresIn', async () => {
      const fetchFn = mockFetch(200, { signedUrl: 'https://signed.example.com/photo.png?token=abc' })
      const bucket = createBucket(fetchFn)

      const result = await bucket.createSignedUrl('photo.png', 3600)

      expect(result.signedUrl).toBe('https://signed.example.com/photo.png?token=abc')

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = call[0] as string
      const init = call[1] as RequestInit
      expect(url).toBe(`${URL}/v1/storage/${REF}/sign/avatars/photo.png`)
      expect(init.method).toBe('POST')
      expect(JSON.parse(init.body as string)).toEqual({ expiresIn: 3600 })
    })
  })

  describe('.getPublicUrl()', () => {
    it('constructs URL without network call', () => {
      const fetchFn = vi.fn() as unknown as typeof fetch
      const bucket = createBucket(fetchFn)

      const url = bucket.getPublicUrl('photo.png')

      expect(url).toBe(`${URL}/v1/storage/${REF}/public/avatars/photo.png`)
      expect(fetchFn).not.toHaveBeenCalled()
    })
  })
})
