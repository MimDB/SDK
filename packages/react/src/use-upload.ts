import { useState, useCallback } from 'react'
import { useClient } from './context'
import type { UploadOptions } from '@mimdb/client'

/**
 * Return type of the {@link useUpload} hook.
 */
export interface UseUploadResult {
  /** Upload a file to the bucket. Re-throws on failure after setting `error`. */
  upload: (path: string, file: Blob | File, opts?: UploadOptions) => Promise<void>
  /** True while an upload is in progress. */
  isUploading: boolean
  /** The error from the most recent failed upload, or null. */
  error: Error | null
}

/**
 * React hook for uploading files to a MimDB storage bucket.
 *
 * Tracks the upload's loading and error state so components can show
 * progress indicators or error messages without manual state management.
 *
 * @param bucket - Name of the storage bucket to upload to.
 * @returns An object with the `upload` function and status flags.
 *
 * @example
 * ```tsx
 * function AvatarUpload() {
 *   const { upload, isUploading, error } = useUpload('avatars')
 *
 *   const handleFile = (file: File) => {
 *     upload(`users/${userId}/avatar.png`, file, { contentType: 'image/png' })
 *   }
 *
 *   return (
 *     <>
 *       <input type="file" onChange={(e) => handleFile(e.target.files![0])} />
 *       {isUploading && <p>Uploading...</p>}
 *       {error && <p>Error: {error.message}</p>}
 *     </>
 *   )
 * }
 * ```
 */
export function useUpload(bucket: string): UseUploadResult {
  const client = useClient()
  const [isUploading, setIsUploading] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const upload = useCallback(
    async (path: string, file: Blob | File, opts?: UploadOptions) => {
      setIsUploading(true)
      setError(null)
      try {
        await client.storage.from(bucket).upload(path, file, opts)
      } catch (err) {
        const uploadError =
          err instanceof Error ? err : new Error(String(err))
        setError(uploadError)
        throw uploadError
      } finally {
        setIsUploading(false)
      }
    },
    [client, bucket],
  )

  return { upload, isUploading, error }
}
