# MimDB SDK

Official client SDKs for [MimDB](https://mimdb.dev) - a self-hosted Backend-as-a-Service platform.

## Packages

| Package | Version | Description |
|---------|---------|-------------|
| [`@mimdb/realtime`](./packages/realtime) | [![npm](https://img.shields.io/npm/v/@mimdb/realtime)](https://www.npmjs.com/package/@mimdb/realtime) | WebSocket client for realtime table change subscriptions |
| [`@mimdb/client`](./packages/client) | [![npm](https://img.shields.io/npm/v/@mimdb/client)](https://www.npmjs.com/package/@mimdb/client) | Unified SDK (REST + Auth + Storage + Realtime) |
| [`@mimdb/react`](./packages/react) | [![npm](https://img.shields.io/npm/v/@mimdb/react)](https://www.npmjs.com/package/@mimdb/react) | React hooks for MimDB |

## Quick Start

```bash
npm install @mimdb/client
```

```typescript
import { createClient } from '@mimdb/client'

const mimdb = createClient('https://api.mimdb.dev', '40891b0d', 'eyJ...')

// REST queries
const { data } = await mimdb.from('todos').select('*').eq('done', 'false')

// Auth
const { user } = await mimdb.auth.signIn('user@example.com', 'password')

// Storage
await mimdb.storage.from('avatars').upload('photo.png', file)

// Realtime
mimdb.realtime.subscribe('todos', {
  event: '*',
  onEvent: (e) => console.log(e.type, e.new),
})
```

### React

```bash
npm install @mimdb/client @mimdb/react @tanstack/react-query
```

```tsx
import { createClient } from '@mimdb/client'
import { MimDBProvider, useQuery, useRealtime, useAuth } from '@mimdb/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

const mimdb = createClient('https://api.mimdb.dev', '40891b0d', 'eyJ...')
const queryClient = new QueryClient()

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <MimDBProvider client={mimdb}>
        <TodoList />
      </MimDBProvider>
    </QueryClientProvider>
  )
}

function TodoList() {
  const { data: todos } = useQuery('todos', { eq: { done: 'false' } })
  useRealtime('todos') // auto-invalidates cache on changes
  return <ul>{todos?.map(t => <li key={t.id}>{t.task}</li>)}</ul>
}
```

### Realtime Only

```bash
npm install @mimdb/realtime
```

```typescript
import { MimDBRealtimeClient } from '@mimdb/realtime'

const realtime = new MimDBRealtimeClient({
  url: 'https://api.mimdb.dev',
  projectRef: '40891b0d',
  apiKey: 'eyJ...',
})

realtime.subscribe('todos', {
  event: '*',
  onEvent(event) {
    console.log(event.type, event.new)
  },
})
```

## Packages

### @mimdb/realtime

Zero-dependency TypeScript WebSocket client for Mimisbrunnr's native realtime protocol.

- **Type-safe** - Generic `subscribe<T>()` for typed event payloads
- **Auto-reconnect** - Exponential backoff (1s-30s) with automatic resubscription
- **Heartbeat** - Dead connection detection with configurable intervals
- **Zero deps** - Native `WebSocket` in browsers, optional `ws` for Node.js
- **Dual output** - ESM and CJS builds

```typescript
interface Player {
  uuid: string
  name: string
  world: string
  x: number
  y: number
  z: number
}

const sub = realtime.subscribe<Player>('player_positions', {
  event: 'UPDATE',
  filter: 'uuid=eq.a27f9a4c-7ae6-452a-b7b3-e0cb9bc58f9c',
  onEvent(event) {
    console.log(`${event.new!.name} moved to ${event.new!.x}, ${event.new!.y}, ${event.new!.z}`)
  },
  onSubscribed() {
    console.log('Tracking player position')
  },
})

// Connection lifecycle
realtime.on('connected', () => console.log('Connected'))
realtime.on('reconnecting', (n) => console.log(`Reconnecting (attempt ${n})`))
realtime.on('disconnected', (reason) => console.log('Disconnected:', reason))

// Cleanup
sub.unsubscribe()
realtime.disconnect()
```

### Node.js

```typescript
import { MimDBRealtimeClient } from '@mimdb/realtime'
import WebSocket from 'ws'

const realtime = new MimDBRealtimeClient({
  url: 'https://api.mimdb.dev',
  projectRef: '40891b0d',
  apiKey: 'eyJ...',
  WebSocket,
})
```

## Development

```bash
# Install dependencies
pnpm install

# Build all packages
pnpm build

# Run tests
pnpm test

# Watch mode
cd packages/realtime && pnpm test:watch
```

## Using with @supabase/supabase-js

MimDB's `/rest/v1/*` compatibility route means `@supabase/supabase-js` works for REST queries. Use `@mimdb/realtime` for realtime subscriptions (MimDB uses a different WebSocket protocol than Supabase).

```typescript
import { createClient } from '@supabase/supabase-js'
import { MimDBRealtimeClient } from '@mimdb/realtime'

// REST queries via supabase-js
const supabase = createClient('https://api.mimdb.dev', 'eyJ...')
const { data } = await supabase.from('todos').select('*')

// Realtime via @mimdb/realtime
const realtime = new MimDBRealtimeClient({
  url: 'https://api.mimdb.dev',
  projectRef: '40891b0d',
  apiKey: 'eyJ...',
})
realtime.subscribe('todos', {
  event: '*',
  onEvent(event) { console.log(event) },
})
```

## Documentation

- [SDK Documentation](https://docs.mimdb.dev/client-integration/mimdb-realtime)
- [Realtime Protocol Reference](https://docs.mimdb.dev/realtime/protocol)
- [WebSocket Examples](https://docs.mimdb.dev/client-integration/websocket-examples)

## License

MIT
