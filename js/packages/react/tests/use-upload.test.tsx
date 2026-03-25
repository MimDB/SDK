import { describe, it, expect, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { type ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MimDBProvider, useUpload } from '../src/index'
import type { MimDBClient } from '@mimdb/client'

const mockUpload = vi.fn().mockResolvedValue({ path: 'avatars/photo.png' })

function createMockClient(): MimDBClient {
  return {
    getConfig: vi.fn().mockReturnValue({ url: 'https://api.test', ref: 'abc', apiKey: 'key' }),
    from: vi.fn(),
    auth: {
      getSession: vi.fn().mockReturnValue(null),
      onAuthStateChange: vi.fn().mockReturnValue(() => {}),
    },
    storage: {
      from: vi.fn().mockReturnValue({
        upload: mockUpload,
      }),
    },
  } as unknown as MimDBClient
}

function createWrapper(client: MimDBClient) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        <MimDBProvider client={client}>{children}</MimDBProvider>
      </QueryClientProvider>
    )
  }
}

describe('useUpload', () => {
  it('starts with isUploading=false and error=null', () => {
    const mockClient = createMockClient()
    const { result } = renderHook(() => useUpload('avatars'), {
      wrapper: createWrapper(mockClient),
    })

    expect(result.current.isUploading).toBe(false)
    expect(result.current.error).toBeNull()
  })

  it('tracks uploading state during upload', async () => {
    const mockClient = createMockClient()
    const { result } = renderHook(() => useUpload('avatars'), {
      wrapper: createWrapper(mockClient),
    })

    const file = new Blob(['test'], { type: 'text/plain' })

    await act(async () => {
      await result.current.upload('test.txt', file)
    })

    expect(mockUpload).toHaveBeenCalledWith('test.txt', file, undefined)
    expect(result.current.isUploading).toBe(false)
    expect(result.current.error).toBeNull()
  })

  it('sets error state on upload failure', async () => {
    const failingUpload = vi.fn().mockRejectedValue(new Error('Upload failed'))
    const mockClient = {
      ...createMockClient(),
      storage: {
        from: vi.fn().mockReturnValue({
          upload: failingUpload,
        }),
      },
    } as unknown as MimDBClient

    const { result } = renderHook(() => useUpload('avatars'), {
      wrapper: createWrapper(mockClient),
    })

    const file = new Blob(['test'], { type: 'text/plain' })

    await act(async () => {
      try {
        await result.current.upload('test.txt', file)
      } catch {
        // Expected
      }
    })

    expect(result.current.isUploading).toBe(false)
    expect(result.current.error).toBeInstanceOf(Error)
    expect(result.current.error!.message).toBe('Upload failed')
  })

  it('passes upload options through', async () => {
    const mockClient = createMockClient()
    const { result } = renderHook(() => useUpload('avatars'), {
      wrapper: createWrapper(mockClient),
    })

    const file = new Blob(['test'], { type: 'image/png' })

    await act(async () => {
      await result.current.upload('photo.png', file, {
        contentType: 'image/png',
        upsert: true,
      })
    })

    expect(mockUpload).toHaveBeenCalledWith('photo.png', file, {
      contentType: 'image/png',
      upsert: true,
    })
  })
})
