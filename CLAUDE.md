# MimDB JavaScript SDKs

## Monorepo

- pnpm workspaces, packages in `packages/`
- Build all: `pnpm build`
- Test all: `pnpm test`

## @mimdb/realtime

- `packages/realtime/` - TypeScript WebSocket client for Mimisbrunnr realtime
- Zero runtime deps in browsers, optional `ws` peer dep for Node.js
- Dual ESM/CJS output via tsup
- 42 tests via Vitest
- Published: `npm publish --access public --otp=<code>` (requires @mimdb npm org)
- GitHub: `MimDB/SDK`

## Key patterns

- `MockWebSocket` in `tests/mock-ws.ts` for deterministic WebSocket testing
- `createMockWSFactory()` returns `{ factory, getInstance }` for capturing WS instances
- Use `vi.useFakeTimers()` for heartbeat/reconnect tests
- `tsconfig.base.json` has `ignoreDeprecations: "6.0"` for tsup/TS6 compat
