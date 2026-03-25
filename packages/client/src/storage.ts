import { MimDBError } from './errors'
import type { Bucket, UploadOptions } from './types'

/**
 * Options for creating or updating a storage bucket.
 */
export interface BucketOptions {
  /** Whether the bucket allows unauthenticated read access. */
  public?: boolean
  /** Maximum file size in bytes. */
  fileSizeLimit?: number
  /** Allowed MIME types for uploads. */
  allowedMimeTypes?: string[]
}

/**
 * Client for managing storage buckets in a MimDB project.
 *
 * Provides methods for creating, listing, updating, and deleting buckets,
 * as well as targeting a specific bucket for file operations via `.from()`.
 *
 * @example
 * ```ts
 * // Upload a file
 * const { path } = await mimdb.storage.from('avatars').upload('user/photo.png', blob)
 *
 * // Get a public URL
 * const url = mimdb.storage.from('avatars').getPublicUrl('user/photo.png')
 * ```
 */
export class StorageClient {
  private readonly baseUrl: string
  private readonly ref: string
  private readonly fetchFn: typeof fetch
  private readonly defaultHeaders: Record<string, string>

  /**
   * @param baseUrl        - Base URL of the MimDB API.
   * @param ref            - Short project reference ID.
   * @param fetchFn        - Fetch implementation.
   * @param defaultHeaders - Default headers (includes apikey and Authorization).
   */
  constructor(
    baseUrl: string,
    ref: string,
    fetchFn: typeof fetch,
    defaultHeaders: Record<string, string>,
  ) {
    this.baseUrl = baseUrl
    this.ref = ref
    this.fetchFn = fetchFn
    this.defaultHeaders = defaultHeaders
  }

  /**
   * Target a specific bucket for file operations.
   *
   * @param bucket - Name of the bucket to operate on.
   * @returns A `BucketClient` scoped to the given bucket.
   *
   * @example
   * ```ts
   * const bucketClient = mimdb.storage.from('avatars')
   * await bucketClient.upload('photo.png', fileBlob)
   * ```
   */
  from(bucket: string): BucketClient {
    return new BucketClient(
      bucket,
      this.baseUrl,
      this.ref,
      this.fetchFn,
      this.defaultHeaders,
    )
  }

  /**
   * Create a new storage bucket.
   *
   * Requires a service_role API key.
   *
   * @param name - Unique name for the bucket.
   * @param opts - Optional bucket configuration.
   * @returns The newly created bucket.
   * @throws {MimDBError} If the API returns an error.
   */
  async createBucket(name: string, opts?: BucketOptions): Promise<Bucket> {
    const url = `${this.baseUrl}/v1/storage/${this.ref}/buckets`

    const body: Record<string, unknown> = { name }
    if (opts?.public !== undefined) body.public = opts.public
    if (opts?.fileSizeLimit !== undefined) body.file_size_limit = opts.fileSizeLimit
    if (opts?.allowedMimeTypes !== undefined) body.allowed_mime_types = opts.allowedMimeTypes

    const response = await this.fetchFn(url, {
      method: 'POST',
      headers: { ...this.defaultHeaders },
      body: JSON.stringify(body),
    })

    if (!response.ok) {
      throw await MimDBError.fromResponse(response)
    }

    return (await response.json()) as Bucket
  }

  /**
   * List all storage buckets in the project.
   *
   * Requires a service_role API key.
   *
   * @returns An array of all buckets.
   * @throws {MimDBError} If the API returns an error.
   */
  async listBuckets(): Promise<Bucket[]> {
    const url = `${this.baseUrl}/v1/storage/${this.ref}/buckets`

    const response = await this.fetchFn(url, {
      method: 'GET',
      headers: { ...this.defaultHeaders },
    })

    if (!response.ok) {
      throw await MimDBError.fromResponse(response)
    }

    return (await response.json()) as Bucket[]
  }

  /**
   * Delete a storage bucket.
   *
   * The bucket must be empty before it can be deleted.
   * Requires a service_role API key.
   *
   * @param name - Name of the bucket to delete.
   * @throws {MimDBError} If the API returns an error.
   */
  async deleteBucket(name: string): Promise<void> {
    const url = `${this.baseUrl}/v1/storage/${this.ref}/buckets/${name}`

    const response = await this.fetchFn(url, {
      method: 'DELETE',
      headers: { ...this.defaultHeaders },
    })

    if (!response.ok) {
      throw await MimDBError.fromResponse(response)
    }
  }

  /**
   * Update a storage bucket's configuration.
   *
   * Requires a service_role API key.
   *
   * @param name - Name of the bucket to update.
   * @param opts - Bucket configuration fields to update.
   * @returns The updated bucket.
   * @throws {MimDBError} If the API returns an error.
   */
  async updateBucket(name: string, opts: BucketOptions): Promise<Bucket> {
    const url = `${this.baseUrl}/v1/storage/${this.ref}/buckets/${name}`

    const body: Record<string, unknown> = {}
    if (opts.public !== undefined) body.public = opts.public
    if (opts.fileSizeLimit !== undefined) body.file_size_limit = opts.fileSizeLimit
    if (opts.allowedMimeTypes !== undefined) body.allowed_mime_types = opts.allowedMimeTypes

    const response = await this.fetchFn(url, {
      method: 'PATCH',
      headers: { ...this.defaultHeaders },
      body: JSON.stringify(body),
    })

    if (!response.ok) {
      throw await MimDBError.fromResponse(response)
    }

    return (await response.json()) as Bucket
  }
}

/**
 * Client for performing file operations within a specific storage bucket.
 *
 * Obtained by calling `StorageClient.from(bucketName)`. Provides upload,
 * download, delete, signed URL, and public URL operations.
 *
 * @example
 * ```ts
 * const bucket = mimdb.storage.from('avatars')
 *
 * // Upload
 * const { path } = await bucket.upload('users/123/avatar.png', imageBlob, {
 *   contentType: 'image/png',
 *   upsert: true,
 * })
 *
 * // Get a signed URL valid for 1 hour
 * const { signedUrl } = await bucket.createSignedUrl('users/123/avatar.png', 3600)
 * ```
 */
export class BucketClient {
  private readonly bucket: string
  private readonly baseUrl: string
  private readonly ref: string
  private readonly fetchFn: typeof fetch
  private readonly defaultHeaders: Record<string, string>

  /**
   * @param bucket         - Name of the bucket this client operates on.
   * @param baseUrl        - Base URL of the MimDB API.
   * @param ref            - Short project reference ID.
   * @param fetchFn        - Fetch implementation.
   * @param defaultHeaders - Default headers (includes apikey and Authorization).
   */
  constructor(
    bucket: string,
    baseUrl: string,
    ref: string,
    fetchFn: typeof fetch,
    defaultHeaders: Record<string, string>,
  ) {
    this.bucket = bucket
    this.baseUrl = baseUrl
    this.ref = ref
    this.fetchFn = fetchFn
    this.defaultHeaders = defaultHeaders
  }

  /**
   * Upload a file to the bucket.
   *
   * @param path - Object path within the bucket (e.g. `folder/file.png`).
   * @param body - File content as a Blob, ArrayBuffer, string, or ReadableStream.
   * @param opts - Optional upload configuration.
   * @returns An object containing the uploaded file path.
   * @throws {MimDBError} If the API returns an error.
   *
   * @example
   * ```ts
   * const { path } = await bucket.upload('docs/readme.md', markdownBlob, {
   *   contentType: 'text/markdown',
   * })
   * ```
   */
  async upload(
    path: string,
    body: Blob | ArrayBuffer | string | ReadableStream,
    opts?: UploadOptions,
  ): Promise<{ path: string }> {
    const url = `${this.baseUrl}/v1/storage/${this.ref}/object/${this.bucket}/${path}`

    const headers: Record<string, string> = { ...this.defaultHeaders }
    headers['Content-Type'] = opts?.contentType ?? 'application/octet-stream'
    if (opts?.upsert) {
      headers['x-upsert'] = 'true'
    }

    const response = await this.fetchFn(url, {
      method: 'POST',
      headers,
      body: body as BodyInit,
    })

    if (!response.ok) {
      throw await MimDBError.fromResponse(response)
    }

    return (await response.json()) as { path: string }
  }

  /**
   * Download a file from the bucket.
   *
   * @param path - Object path within the bucket.
   * @returns The file content as a Blob.
   * @throws {MimDBError} If the API returns an error.
   *
   * @example
   * ```ts
   * const blob = await bucket.download('docs/readme.md')
   * const text = await blob.text()
   * ```
   */
  async download(path: string): Promise<Blob> {
    const url = `${this.baseUrl}/v1/storage/${this.ref}/object/${this.bucket}/${path}`

    const response = await this.fetchFn(url, {
      method: 'GET',
      headers: { ...this.defaultHeaders },
    })

    if (!response.ok) {
      throw await MimDBError.fromResponse(response)
    }

    return await response.blob()
  }

  /**
   * Delete one or more files from the bucket.
   *
   * Sends a separate DELETE request for each path.
   *
   * @param paths - Array of object paths to delete.
   * @throws {MimDBError} If any deletion fails.
   *
   * @example
   * ```ts
   * await bucket.remove(['old/file1.png', 'old/file2.png'])
   * ```
   */
  async remove(paths: string[]): Promise<void> {
    for (const path of paths) {
      const url = `${this.baseUrl}/v1/storage/${this.ref}/object/${this.bucket}/${path}`

      const response = await this.fetchFn(url, {
        method: 'DELETE',
        headers: { ...this.defaultHeaders },
      })

      if (!response.ok) {
        throw await MimDBError.fromResponse(response)
      }
    }
  }

  /**
   * Create a time-limited signed URL for downloading a file.
   *
   * The signed URL can be shared with users who do not have direct
   * API credentials.
   *
   * @param path      - Object path within the bucket.
   * @param expiresIn - Number of seconds until the URL expires.
   * @returns An object containing the signed download URL.
   * @throws {MimDBError} If the API returns an error.
   *
   * @example
   * ```ts
   * // URL valid for 1 hour
   * const { signedUrl } = await bucket.createSignedUrl('report.pdf', 3600)
   * ```
   */
  async createSignedUrl(
    path: string,
    expiresIn: number,
  ): Promise<{ signedUrl: string }> {
    const url = `${this.baseUrl}/v1/storage/${this.ref}/sign/${this.bucket}/${path}`

    const response = await this.fetchFn(url, {
      method: 'POST',
      headers: { ...this.defaultHeaders },
      body: JSON.stringify({ expiresIn }),
    })

    if (!response.ok) {
      throw await MimDBError.fromResponse(response)
    }

    return (await response.json()) as { signedUrl: string }
  }

  /**
   * Construct a public URL for a file in a public bucket.
   *
   * This is a client-side operation and does not make a network request.
   * The file must be in a bucket with `public: true` for the URL to work.
   *
   * @param path - Object path within the bucket.
   * @returns The public URL string.
   *
   * @example
   * ```ts
   * const url = bucket.getPublicUrl('images/hero.jpg')
   * // => "https://api.mimdb.dev/v1/storage/abc123/public/avatars/images/hero.jpg"
   * ```
   */
  getPublicUrl(path: string): string {
    return `${this.baseUrl}/v1/storage/${this.ref}/public/${this.bucket}/${path}`
  }
}
