import { createContext, useContext } from 'react'
import type { MimDBClient } from '@mimdb/client'

/**
 * React context that holds the MimDB client instance.
 *
 * Consumers should use the {@link useClient} hook rather than accessing
 * this context directly.
 *
 * @internal
 */
const MimDBContext = createContext<MimDBClient | null>(null)

/**
 * Retrieve the MimDB client from the nearest `<MimDBProvider>`.
 *
 * @returns The `MimDBClient` instance provided by the enclosing provider.
 * @throws If called outside of a `<MimDBProvider>` tree.
 *
 * @example
 * ```tsx
 * function MyComponent() {
 *   const client = useClient()
 *   // Use client.from(), client.auth, etc.
 * }
 * ```
 */
export function useClient(): MimDBClient {
  const client = useContext(MimDBContext)
  if (!client) {
    throw new Error('useClient must be used within <MimDBProvider>')
  }
  return client
}

export { MimDBContext }
