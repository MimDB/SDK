import { describe, expect, it, vi } from 'vitest'
import { MimDBClient } from '../src/client'
import { StorageClient, BucketClient } from '../src/storage'
import { envelope, mockFetch, mockFetchNoContent } from './helpers'

const URL = 'https://api.mimdb.dev'
const REF = 'abc123'
const KEY = 'test-api-key'

const MOCK_BUCKET = {
  id: 'bucket-uuid-1',
  project_id: 'project-uuid-1',
  name: 'avatars',
  public: true,
  file_size_limit: 1024,
  allowed_types: ['image/png'],
  created_at: '2025-01-01T00:00:00Z',
}

const MOCK_OBJECT = {
  id: 'obj-uuid-1',
  bucket_id: 'bucket-uuid-1',
  path: 'photo.png',
  size: 12345,
  content_type: 'image/png',
  etag: 'abc123',
  status: 'active',
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
}

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
    it('sends POST with name and options, parses envelope', async () => {
      const fetchFn = mockFetch(201, envelope(MOCK_BUCKET))
      const storage = createStorage(fetchFn)

      const bucket = await storage.createBucket('avatars', {
        public: true,
      })

      expect(bucket.name).toBe('avatars')
      expect(bucket.public).toBe(true)
      expect(bucket.id).toBe('bucket-uuid-1')

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = call[0] as string
      const init = call[1] as RequestInit
      expect(url).toBe(`${URL}/v1/storage/${REF}/buckets`)
      expect(init.method).toBe('POST')
      // Backend create only accepts name + public (not fileSizeLimit/allowedTypes)
      expect(JSON.parse(init.body as string)).toEqual({
        name: 'avatars',
        public: true,
      })
    })
  })

  describe('.listBuckets()', () => {
    it('sends GET and parses envelope', async () => {
      const fetchFn = mockFetch(200, envelope([MOCK_BUCKET]))
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
    it('sends DELETE and accepts 204 No Content', async () => {
      const fetchFn = mockFetchNoContent()
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
    it('sends PATCH and accepts 204 No Content', async () => {
      const fetchFn = mockFetchNoContent()
      const storage = createStorage(fetchFn)

      await storage.updateBucket('avatars', {
        public: false,
        fileSizeLimit: 2048,
      })

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
    it('sends POST with correct path and Content-Type, parses envelope', async () => {
      const fetchFn = mockFetch(201, envelope(MOCK_OBJECT))
      const bucket = createBucket(fetchFn)

      const result = await bucket.upload('photo.png', 'file-contents', {
        contentType: 'image/png',
      })

      expect(result.path).toBe('photo.png')
      expect(result.id).toBe('obj-uuid-1')

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = call[0] as string
      const init = call[1] as RequestInit
      expect(url).toBe(`${URL}/v1/storage/${REF}/object/avatars/photo.png`)
      expect(init.method).toBe('POST')

      const headers = init.headers as Record<string, string>
      expect(headers['Content-Type']).toBe('image/png')
    })

    it('uses application/octet-stream as default Content-Type', async () => {
      const fetchFn = mockFetch(201, envelope(MOCK_OBJECT))
      const bucket = createBucket(fetchFn)

      await bucket.upload('data.bin', 'binary-data')

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const init = call[1] as RequestInit
      const headers = init.headers as Record<string, string>
      expect(headers['Content-Type']).toBe('application/octet-stream')
    })

    it('does not send x-upsert header', async () => {
      const fetchFn = mockFetch(201, envelope(MOCK_OBJECT))
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
      const fetchFn = mockFetchNoContent()
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
    it('sends POST and parses signedURL from envelope', async () => {
      const fetchFn = mockFetch(200, envelope({ signedURL: '/v1/storage/abc123/object/avatars/photo.png?token=tok' }))
      const bucket = createBucket(fetchFn)

      const result = await bucket.createSignedUrl('photo.png')

      expect(result.signedUrl).toBe('/v1/storage/abc123/object/avatars/photo.png?token=tok')

      const call = vi.mocked(fetchFn).mock.calls[0]!
      const url = call[0] as string
      const init = call[1] as RequestInit
      expect(url).toBe(`${URL}/v1/storage/${REF}/sign/avatars/photo.png`)
      expect(init.method).toBe('POST')
      // No body - TTL is configured server-side
      expect(init.body).toBeUndefined()
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
