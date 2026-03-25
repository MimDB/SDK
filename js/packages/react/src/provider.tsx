import { type ReactNode } from 'react'
import { MimDBContext } from './context'
import type { MimDBClient } from '@mimdb/client'

/**
 * Props for the {@link MimDBProvider} component.
 */
export interface MimDBProviderProps {
  /** A configured `MimDBClient` instance to make available to child components. */
  client: MimDBClient
  /** The React subtree that will have access to the MimDB client. */
  children: ReactNode
}

/**
 * Context provider that makes a `MimDBClient` available to all descendant
 * components via the {@link useClient} hook.
 *
 * Wrap your application (or a subtree) with this provider and pass a
 * pre-configured client instance.
 *
 * @param props - Provider props containing the client and children.
 *
 * @example
 * ```tsx
 * import { createClient } from '@mimdb/client'
 * import { MimDBProvider } from '@mimdb/react'
 *
 * const client = createClient('https://api.mimdb.dev', 'ref', 'key')
 *
 * function App() {
 *   return (
 *     <MimDBProvider client={client}>
 *       <MyApp />
 *     </MimDBProvider>
 *   )
 * }
 * ```
 */
export function MimDBProvider({ client, children }: MimDBProviderProps) {
  return <MimDBContext.Provider value={client}>{children}</MimDBContext.Provider>
}
