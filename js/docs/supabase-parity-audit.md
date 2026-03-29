# MimDB vs Supabase JavaScript SDK: Feature Gap Audit

**Audit Date:** March 28, 2026  
**Scope:** Public API surface comparison  
**MimDB SDKs Audited:**
- `@mimdb/client` (main client SDK)
- `@mimdb/realtime` (WebSocket realtime)
- `@mimdb/react` (React hooks)

**Supabase SDKs Compared Against:**
- `@supabase/supabase-js` v2.58.0 (comprehensive client)
- `@supabase/auth-js` (authentication)
- `@supabase/storage-js` (file storage)
- `@supabase/realtime-js` (WebSocket subscriptions)

---

## Executive Summary

MimDB provides a well-structured SDK with solid coverage of core database and realtime functionality. The implementation is lean, focusing on PostgREST-compatible REST APIs and WebSocket subscriptions. However, there are notable gaps compared to Supabase's feature breadth:

- **Auth:** Missing OTP/magic link, phone authentication, anonymous sign-in, social sign-in with scopes/metadata
- **Storage:** Missing file move/copy, signed URLs batch creation, image transformations
- **Realtime:** No presence tracking or broadcast channels (MimDB is table-only)
- **Database:** Missing full-text search, complex joins, recursive queries
- **Admin/Management:** Limited admin APIs compared to Supabase's comprehensive admin suite

---

## 1. Authentication (Auth)

### Email / Password Authentication

| Feature | Supabase | MimDB | Status |
|---------|----------|-------|--------|
| `signUp(email, password, metadata)` | ✓ | `signUp(email, password, opts)` | ✓ IMPLEMENTED |
| `signInWithPassword(email, password)` | ✓ | `signIn(email, password)` | ✓ IMPLEMENTED (different naming) |
| `signOut()` / `signOut({ scope: 'global' })` | ✓ | `signOut()` | PARTIAL (no global signout option) |
| `refreshSession(token?)` | ✓ | `refreshSession(token?)` | ✓ IMPLEMENTED |
| `getUser()` | ✓ | `getUser()` | ✓ IMPLEMENTED |
| `updateUser(data)` | ✓ | `updateUser(data)` | ✓ IMPLEMENTED |
| `getSession()` | ✓ | `getSession()` | ✓ IMPLEMENTED |
| `setSession(session)` | ✓ | `setSession(session)` | ✓ IMPLEMENTED |

### OAuth / Social Sign-In

| Feature | Supabase | MimDB | Status |
|---------|----------|-------|--------|
| `signInWithOAuth(provider, options)` | ✓ Full featured | `signInWithOAuth(provider, opts)` | PARTIAL |
| OAuth provider scopes | ✓ `.options.scopes` | MISSING | **P1 - GAP** |
| OAuth query parameters | ✓ `.options.queryParams` | MISSING | **P1 - GAP** |
| OAuth flow types (PKCE, implicit) | ✓ `auth.flowType` config | MISSING | **P2 - NICE TO HAVE** |
| `handleOAuthCallback(hash)` | ✗ (implicit parsing) | ✓ Explicit method | ✓ MimDB ADVANTAGE |

### Passwordless Authentication

| Feature | Supabase | MimDB | Status |
|---------|----------|-------|--------|
| `signInWithOtp(email/phone)` | ✓ | MISSING | **P1 - BLOCKING** |
| `verifyOtp(phone, token, type)` | ✓ | MISSING | **P1 - BLOCKING** |
| Magic link (email OTP) | ✓ | MISSING | **P1 - BLOCKING** |
| SMS OTP | ✓ | MISSING | **P1 - BLOCKING** |
| WhatsApp OTP | ✓ | MISSING | **P2 - BLOCKING** |

### Anonymous Authentication

| Feature | Supabase | MimDB | Status |
|---------|----------|-------|--------|
| `signInAnonymously(options)` | ✓ | MISSING | **P2 - NICE TO HAVE** |

### State Management & Events

| Feature | Supabase | MimDB | Status |
|---------|----------|-------|--------|
| `onAuthStateChange(callback)` | ✓ Returns subscription | ✓ `onAuthStateChange()` | ✓ IMPLEMENTED |
| Event types | `SIGNED_IN, SIGNED_OUT, TOKEN_REFRESHED, PASSWORD_RECOVERY` | `SIGNED_IN, SIGNED_OUT, TOKEN_REFRESHED, TOKEN_REFRESH_FAILED` | PARTIAL (different event set) |
| Auto token refresh | ✓ Configurable via `auth.autoRefreshToken` | ✓ `autoRefresh` option | ✓ IMPLEMENTED |
| Session persistence | ✓ `auth.persistSession` | ✓ `tokenStore` (pluggable) | ✓ IMPLEMENTED |

### Admin User Management

| Feature | Supabase | MimDB | Status |
|---------|----------|-------|--------|
| `admin.listUsers(filters)` | ✓ Full pagination | ✓ `listUsers(limit, offset)` | ✓ IMPLEMENTED |
| `admin.getUserByEmail(email)` | ✓ | ✓ | ✓ IMPLEMENTED |
| `admin.updateUserById(id, data)` | ✓ Full updates | ✓ `updateUserById(id, {appMetadata})` | PARTIAL (app_metadata only) |
| `admin.createUser()` | ✓ | MISSING | **P1 - BLOCKING** |
| `admin.deleteUser(id)` | ✓ | MISSING | **P1 - BLOCKING** |
| `admin.sendInvitationEmail()` | ✓ | MISSING | **P2 - NICE TO HAVE** |
| MFA management | ✓ | MISSING | **P2 - BLOCKING** |

### Known Issues in MimDB Auth
- **Sign-out doesn't support global scope** - only signs out current session
- **No API for password reset flows** - missing `resetPasswordForEmail()`, `getPasswordRecoveryError()`
- **No phone/SMS authentication** - significant feature gap for developers needing SMS OTP
- **Limited OAuth configuration** - scopes and advanced options not supported

---

## 2. Database (REST / PostgREST)

### CRUD Operations

| Feature | Supabase | MimDB | Status |
|---------|----------|-------|--------|
| `.select(columns)` | ✓ | ✓ | ✓ IMPLEMENTED |
| `.insert(rows)` | ✓ | ✓ | ✓ IMPLEMENTED |
| `.update(data)` | ✓ | ✓ | ✓ IMPLEMENTED |
| `.delete()` | ✓ | ✓ | ✓ IMPLEMENTED |
| `.upsert(data)` | ✓ | ✓ | ✓ IMPLEMENTED |
| `.single()` (single row) | ✓ | MISSING | **P1 - IMPORTANT** |
| `.maybeSingle()` (0 or 1) | ✓ | MISSING | **P1 - IMPORTANT** |

### Filtering / Where Clauses

| Filter Operator | Supabase | MimDB | Status |
|-----------------|----------|-------|--------|
| `.eq(col, val)` | ✓ | ✓ | ✓ IMPLEMENTED |
| `.neq(col, val)` | ✓ | ✓ | ✓ IMPLEMENTED |
| `.gt(col, val)` | ✓ | ✓ | ✓ IMPLEMENTED |
| `.gte(col, val)` | ✓ | ✓ | ✓ IMPLEMENTED |
| `.lt(col, val)` | ✓ | ✓ | ✓ IMPLEMENTED |
| `.lte(col, val)` | ✓ | ✓ | ✓ IMPLEMENTED |
| `.like(col, pattern)` | ✓ | ✓ | ✓ IMPLEMENTED |
| `.ilike(col, pattern)` | ✓ | MISSING | **P1 - IMPORTANT** |
| `.in(col, array)` | ✓ | MISSING | **P1 - IMPORTANT** |
| `.is(col, value)` | ✓ | MISSING | **P1 - IMPORTANT** |
| `.contains(col, value)` | ✓ (JSON/arrays) | MISSING | **P2 - NICE TO HAVE** |
| `.containedBy(col, value)` | ✓ (JSON/arrays) | MISSING | **P2 - NICE TO HAVE** |
| `.filter(col, op, val)` | ✓ (low-level) | MISSING | **P2 - NICE TO HAVE** |
| `.or(filter_string)` | ✓ | MISSING | **P1 - IMPORTANT** |
| `.not(col, op, val)` | ✓ | MISSING | **P1 - IMPORTANT** |

### Query Modifiers

| Modifier | Supabase | MimDB | Status |
|----------|----------|-------|--------|
| `.order(col, options)` | ✓ | ✓ | ✓ IMPLEMENTED |
| `.limit(n)` | ✓ | ✓ | ✓ IMPLEMENTED |
| `.offset(n)` | ✓ | ✓ | ✓ IMPLEMENTED |
| `.range(start, end)` | ✓ | MISSING | **P1 - IMPORTANT** |
| `.textSearch(col, query, options)` | ✓ Full-text search | MISSING | **P1 - BLOCKING** |
| Count modes: `exact`, `planned`, `estimated` | ✓ | ✓ `exact/planned/estimated` | ✓ IMPLEMENTED |

### Relational Queries (Joins)

| Feature | Supabase | MimDB | Status |
|---------|----------|-------|--------|
| Foreign key joins in select | ✓ Nested notation | MISSING | **P0 - BLOCKING** |
| Recursive select | ✓ | MISSING | **P2 - NICE TO HAVE** |
| Aggregate functions | ✓ | MISSING | **P2 - NICE TO HAVE** |

### RPC / Function Calls

| Feature | Supabase | MimDB | Status |
|---------|----------|-------|--------|
| `.rpc(fn_name, params)` | ✓ | ✓ `rpc()` | ✓ IMPLEMENTED |
| Return type inference | ✓ Generic via typing | ✓ Generic `<T>` | ✓ IMPLEMENTED |

### Known Issues in MimDB Database
- **No full-text search** - significant feature gap for search-heavy applications
- **No complex relational joins** - limited to single-table queries
- **Missing filter operators** - `ilike`, `in`, `is`, `or`, `not` operators unavailable
- **No `single()` / `maybeSingle()`** - developers must manually handle array destructuring
- **No range() modifier** - must use limit+offset pattern instead

---

## 3. Storage (File Management)

### Bucket Operations

| Feature | Supabase | MimDB | Status |
|---------|----------|-------|--------|
| `.createBucket(name, opts)` | ✓ Full options | ✓ | ✓ IMPLEMENTED |
| `.listBuckets()` | ✓ | ✓ | ✓ IMPLEMENTED |
| `.getBucket(name)` | ✓ | MISSING | **P1 - IMPORTANT** |
| `.updateBucket(name, opts)` | ✓ | ✓ | ✓ IMPLEMENTED |
| `.deleteBucket(name)` | ✓ | ✓ | ✓ IMPLEMENTED |
| `.emptyBucket(name)` | ✓ | MISSING | **P2 - NICE TO HAVE** |

### File Operations

| Feature | Supabase | MimDB | Status |
|---------|----------|-------|--------|
| `.upload(path, body, opts)` | ✓ | ✓ | ✓ IMPLEMENTED |
| `.download(path)` | ✓ | ✓ | ✓ IMPLEMENTED |
| `.remove(paths[])` | ✓ | ✓ | ✓ IMPLEMENTED |
| `.move(from, to)` | ✓ | MISSING | **P1 - IMPORTANT** |
| `.copy(from, to)` | ✓ | MISSING | **P1 - IMPORTANT** |
| `.list(path, options)` | ✓ | MISSING | **P1 - IMPORTANT** |
| `.update(path, body, opts)` | ✓ (upsert) | MISSING | **P1 - IMPORTANT** |

### URL Generation

| Feature | Supabase | MimDB | Status |
|---------|----------|-------|--------|
| `.getPublicUrl(path)` | ✓ | ✓ | ✓ IMPLEMENTED |
| Public URL with transforms | ✓ Image resize, quality, etc. | MISSING | **P2 - NICE TO HAVE** |
| `.createSignedUrl(path, expiresIn)` | ✓ | ✓ | ✓ IMPLEMENTED |
| `.createSignedUrls(paths[], expiresIn)` | ✓ Batch operation | MISSING | **P1 - IMPORTANT** |
| Signed URL options (download name) | ✓ | MISSING | **P2 - NICE TO HAVE** |

### Known Issues in MimDB Storage
- **No file move/copy** - forces delete+upload pattern for reorganization
- **No batch signed URL creation** - must call in a loop
- **No list operation** - developers can't browse bucket contents
- **No get bucket details** - can't inspect bucket configuration
- **No image transformations** - public URLs lack resize/quality parameters
- **Missing update() method** - upsert pattern not supported for files
- **Signed URL construction quirk** - returns relative path initially, documentation notes backend returns relative paths that need manual prepending

---

## 4. Realtime Subscriptions

### Table Change Subscriptions

| Feature | Supabase | MimDB | Status |
|---------|----------|-------|--------|
| Subscribe to table | ✓ `channel().on('postgres_changes')` | ✓ `.subscribe()` | ✓ IMPLEMENTED |
| Event filter (INSERT/UPDATE/DELETE) | ✓ | ✓ | ✓ IMPLEMENTED |
| Schema filter | ✓ | MISSING | **P2 - NICE TO HAVE** |
| Row filter (eq only) | ✓ Simple | ✓ `filter: "col=eq.val"` | PARTIAL (limited operators) |
| Complex row filters | ✓ Multiple operators | MISSING | **P1 - IMPORTANT** |
| `onEvent` callback | ✓ | ✓ | ✓ IMPLEMENTED |
| `onError` callback | ✓ | ✓ | ✓ IMPLEMENTED |
| `onSubscribed` callback | ✓ | ✓ | ✓ IMPLEMENTED |

### Presence Tracking

| Feature | Supabase | MimDB | Status |
|---------|----------|-------|--------|
| `.track(state)` | ✓ Full presence | MISSING | **P1 - BLOCKING** |
| `.untrack()` | ✓ | MISSING | **P1 - BLOCKING** |
| `presence.sync` event | ✓ | MISSING | **P1 - BLOCKING** |
| `presence.join` event | ✓ | MISSING | **P1 - BLOCKING** |
| `presence.leave` event | ✓ | MISSING | **P1 - BLOCKING** |
| `.presenceState()` | ✓ Get all presences | MISSING | **P1 - BLOCKING** |

### Broadcast Channels

| Feature | Supabase | MimDB | Status |
|---------|----------|-------|--------|
| Broadcast messaging | ✓ `.on('broadcast')` | MISSING | **P1 - BLOCKING** |
| `.send()` (broadcast) | ✓ | MISSING | **P1 - BLOCKING** |
| Acknowledgment option | ✓ `config.ack` | MISSING | **P1 - BLOCKING** |
| Self-delivery option | ✓ `config.self` | MISSING | **P1 - BLOCKING** |

### Connection Management

| Feature | Supabase | MimDB | Status |
|---------|----------|-------|--------|
| `.connect()` | ✓ | ✓ | ✓ IMPLEMENTED |
| `.disconnect()` | ✓ | ✓ | ✓ IMPLEMENTED |
| `on('connected')` event | ✓ | ✓ | ✓ IMPLEMENTED |
| `on('disconnected')` event | ✓ | ✓ | ✓ IMPLEMENTED |
| `on('error')` event | ✓ | ✓ | ✓ IMPLEMENTED |
| Reconnection with backoff | ✓ | ✓ | ✓ IMPLEMENTED |
| `.state` property | ✓ | ✓ | ✓ IMPLEMENTED |
| `.setToken(jwt)` | ✓ | ✓ | ✓ IMPLEMENTED |

### Known Issues in MimDB Realtime
- **No presence tracking** - can't track online users or shared state
- **No broadcast channels** - limited to table subscriptions only
- **Row filters limited to equality** - no `gt`, `lt`, `in`, `like` operators for filtering
- **No schema filtering** - must subscribe to all schemas

---

## 5. React Bindings

### Query Hooks

| Hook | Supabase | MimDB | Status |
|------|----------|-------|--------|
| `useQuery(table, options)` | N/A (library agnostic) | ✓ TanStack Query backed | ✓ IMPLEMENTED |
| Options: select, filters, order | ✓ | ✓ Partial (eq/neq only) | PARTIAL |
| Options: limit, offset, enabled | ✓ | ✓ | ✓ IMPLEMENTED |
| Options: staleTime, refetchInterval | ✓ TanStack Query | ✓ | ✓ IMPLEMENTED |

### Mutation Hooks

| Hook | Supabase | MimDB | Status |
|------|----------|-------|--------|
| `useInsert(table, options)` | N/A | ✓ | ✓ IMPLEMENTED |
| Optimistic updates | ✓ Pattern | ✓ | ✓ IMPLEMENTED |
| `useUpdate(table, options)` | N/A | ✓ | ✓ IMPLEMENTED |
| `useDelete(table, options)` | N/A | ✓ | ✓ IMPLEMENTED |
| Error handling | ✓ | ✓ MimDBError | ✓ IMPLEMENTED |
| Cache invalidation | ✓ Manual | ✓ Automatic | ✓ MimDB ADVANTAGE |

### Auth Hooks

| Hook | Supabase | MimDB | Status |
|------|----------|-------|--------|
| `useAuth()` | N/A | ✓ | ✓ IMPLEMENTED |
| `.user` / `.isLoading` | ✓ Pattern | ✓ | ✓ IMPLEMENTED |
| `.signIn()` | ✓ Pattern | ✓ | ✓ IMPLEMENTED |
| `.signUp()` | ✓ Pattern | ✓ | ✓ IMPLEMENTED |
| `.signOut()` | ✓ Pattern | ✓ | ✓ IMPLEMENTED |
| `.signInWithOAuth()` | ✓ Pattern | ✓ | ✓ IMPLEMENTED |
| Phone/OTP methods | ✓ Pattern | MISSING | **P1 - BLOCKING** |

### Realtime Hooks

| Hook | Supabase | MimDB | Status |
|------|----------|-------|--------|
| `useRealtime(table, options)` | N/A | ✓ Subscription-based | ✓ IMPLEMENTED |
| `useSubscription(table, options)` | N/A | ✓ Alternative to useRealtime | ✓ IMPLEMENTED |
| Presence hooks | ✓ Pattern | MISSING | **P1 - BLOCKING** |
| Broadcast hooks | ✓ Pattern | MISSING | **P1 - BLOCKING** |

### Upload Hooks

| Hook | Supabase | MimDB | Status |
|------|----------|-------|--------|
| `useUpload(bucket)` | N/A | ✓ | ✓ IMPLEMENTED |
| Progress callback | ✓ Pattern | MISSING | **P1 - IMPORTANT** |
| Pause/resume | ✓ Pattern | MISSING | **P2 - NICE TO HAVE** |

### Known Issues in React Bindings
- **Upload hook missing progress tracking** - can't show upload progress bar
- **No presence/broadcast hooks** - limited realtime capabilities
- **useQuery filters only support eq/neq** - advanced filters require direct client usage
- **No built-in pagination helpers** - developers must manage offset/limit manually

---

## 6. Edge Functions (Server-Side Functions)

| Feature | Supabase | MimDB | Status |
|---------|----------|-------|--------|
| `.functions.invoke(name, options)` | ✓ | MISSING | **P1 - BLOCKING** |
| Custom function execution | ✓ | MISSING | **P1 - BLOCKING** |
| Function headers/auth | ✓ | MISSING | **P1 - BLOCKING** |

MimDB has no Edge Functions equivalent in the client SDKs (would require separate infrastructure).

---

## 7. Client Configuration & Advanced Features

### Client Options

| Feature | Supabase | MimDB | Status |
|---------|----------|-------|--------|
| Custom fetch | ✓ | ✓ | ✓ IMPLEMENTED |
| Custom headers | ✓ | ✓ | ✓ IMPLEMENTED |
| Request interceptors | ✓ | ✓ `onRequest` | ✓ IMPLEMENTED |
| Response interceptors | ✓ | ✓ `onResponse` | ✓ IMPLEMENTED |
| Retry configuration | ✓ | ✓ Full exponential backoff | ✓ IMPLEMENTED |
| Token persistence | ✓ LocalStorage, custom | ✓ `tokenStore` (pluggable) | ✓ IMPLEMENTED (better) |

### Error Handling

| Feature | Supabase | MimDB | Status |
|---------|----------|-------|--------|
| Structured errors | ✓ | ✓ `MimDBError` class | ✓ IMPLEMENTED |
| Error codes | ✓ | ✓ Includes detail field | ✓ IMPLEMENTED |
| Error parsing | ✓ | ✓ From response | ✓ IMPLEMENTED |
| HTTP status codes | ✓ | ✓ Included in result | ✓ IMPLEMENTED |

---

## 8. Type Safety & Developer Experience

### TypeScript Support

| Feature | Supabase | MimDB | Status |
|---------|----------|-------|--------|
| Generic type parameters | ✓ `Database` type | ✓ `<T>` parameters | ✓ IMPLEMENTED |
| Exported types | ✓ Comprehensive | ✓ Good coverage | ✓ IMPLEMENTED |
| Type inference | ✓ Excellent | ✓ Good | ✓ COMPARABLE |
| IntelliSense | ✓ Excellent | ✓ Good | ✓ COMPARABLE |

### Server-Side Rendering

| Feature | Supabase | MimDB | Status |
|---------|----------|-------|--------|
| `createServerClient()` | ✓ | ✓ | ✓ IMPLEMENTED |
| Session restoration | ✓ | ✓ | ✓ IMPLEMENTED |
| No localStorage | ✓ | ✓ InMemoryTokenStore | ✓ IMPLEMENTED |
| SSR + ISR support | ✓ | ✓ Via tokenStore | ✓ IMPLEMENTED |

---

## Priority-Ranked Feature Gaps

### P0: Blocking for Migrations (Show-Stoppers)

1. **Relational Queries / Foreign Key Joins** - Supabase can fetch related data; MimDB is single-table only
2. **Full-Text Search** - No text search in MimDB; Supabase has `textSearch()`
3. **OTP / Magic Link Authentication** - Critical for passwordless auth flows
4. **Presence Tracking** - No online user tracking; essential for collaborative apps
5. **Broadcast Channels** - No pub/sub messaging beyond table subscriptions

### P1: Important Features (High Priority)

1. **Phone/SMS OTP** - Growing authentication standard
2. **Admin User Create/Delete** - Essential for user management
3. **Advanced Filter Operators** - `ilike`, `in`, `is`, `or`, `not`
4. **File Move/Copy** - Prevents costly delete+upload workarounds
5. **Single/MaybeSingle Query Modifiers** - Cleaner single-row fetching
6. **Batch Signed URLs** - Efficiency for multi-file operations
7. **OAuth Scopes & Metadata** - Advanced social login configuration
8. **Upload Progress Tracking** - Better UX for file uploads
9. **File List/Browse** - Bucket introspection essential for storage management
10. **Complex Row Filters** - Beyond equality for realtime subscriptions

### P2: Nice-to-Have Features (Lower Priority)

1. **Anonymous Sign-In** - Guest user flows
2. **Image Transformations** - Public URL image resizing
3. **Global Sign-Out** - Sign out across devices
4. **Batch Operations** - Performance optimization
5. **File Download Naming** - UX improvement
6. **Aggregate Functions** - Analytics queries
7. **Recursive Queries** - Hierarchical data
8. **WhatsApp OTP** - Alternative auth channel
9. **Upload Pause/Resume** - Large file optimization

---

## Implementation Quality Notes

### MimDB Strengths
1. **Pluggable TokenStore** - Better than Supabase's localStorage assumption
2. **Explicit retry configuration** - Developers have fine-grained control
3. **Dual interceptor pattern** - Request AND response interceptors
4. **Lean codebase** - Focused on core functionality (no bloat)
5. **React hook cache invalidation** - Automatic without manual invalidation
6. **Simple OAuth callback parsing** - Explicit `handleOAuthCallback()` method

### MimDB Implementation Issues
1. **Fetch binding issue** (NOTED IN AUDIT COMMENTS)
   - Client constructor: `this.fetchFn = options?.fetch ?? globalThis.fetch.bind(globalThis)`
   - This works but the `.bind(globalThis)` is a workaround for potential context loss
   - **Severity:** Low (works but not idiomatic)

2. **Signed URL relative path quirk** (NOTED IN AUDIT COMMENTS)
   - Backend returns relative path; SDK manually prepends baseUrl
   - Documentation acknowledges this: "The backend returns a relative path"
   - **Severity:** Low (documented and handled)

3. **Missing OAuth parameter validation**
   - `signInWithOAuth()` doesn't validate provider names or redirectTo
   - **Severity:** Low (backend validates)

4. **Auth state change event naming inconsistency**
   - Supabase: `PASSWORD_RECOVERY`
   - MimDB: `TOKEN_REFRESH_FAILED`
   - Different event sets for different error scenarios
   - **Severity:** Low (documented)

5. **TokenStore null return handling**
   - `tokenStore.get()` returns null if no session
   - Several places check `?.accessToken` - good null-safety
   - **Severity:** None (good practice)

---

## Migration Considerations

### For Supabase → MimDB Users

**Can migrate easily:**
- Basic auth (email/password)
- Simple table queries (SELECT, INSERT, UPDATE, DELETE)
- File storage (upload, download, public URLs)
- Realtime table subscriptions
- React hooks (with some filter limitations)

**Will require refactoring:**
- OTP/magic link flows → Need backend-side implementation
- Complex queries with joins → Fetch related data separately or use RPC
- Presence/broadcast → Build custom WebSocket solution
- Full-text search → Use RPC to PostgreSQL native FTS
- Advanced storage operations → Use REST API directly for move/copy/list

### For MimDB → Supabase Users

**Should be straightforward:**
- Most APIs map 1:1 with minor naming differences
- Supabase is a superset; more features available
- TypeScript support equivalent
- React integration via `@supabase/supabase-js` or libraries like `supabase-js-toolkit`

---

## Recommendations

### For MimDB Roadmap

**High-Impact, Medium-Effort (Quick Wins)**
1. Add `.single()` and `.maybeSingle()` query modifiers
2. Add remaining filter operators: `ilike`, `in`, `is`, `or`, `not`
3. Add storage `.list()`, `.move()`, `.copy()` methods
4. Add storage `.getBucket()` method

**Medium-Impact, Medium-Effort (Important)**
1. Implement OTP/magic link endpoints (may require backend changes)
2. Add phone authentication flow
3. Implement batch `.createSignedUrls()`
4. Add upload progress callbacks via `FileList` API

**High-Impact, High-Effort (Long-Term)**
1. Add full-text search support
2. Implement presence tracking for realtime
3. Add broadcast channels
4. Implement admin user create/delete/update

### For MimDB Users

1. **For advanced queries:** Use `.rpc()` to call PostgreSQL functions with joins/FTS
2. **For presence:** Implement custom WebSocket or use Supabase's realtime-js alongside MimDB
3. **For OTP flows:** Implement backend endpoint to issue OTP tokens via email/SMS
4. **For file operations:** Use MimDB for basic upload/download; consider adding direct REST calls for move/copy

---

## Conclusion

MimDB provides a solid, focused SDK for core database and realtime functionality. It's an excellent choice for:
- Projects needing simple PostgreSQL CRUD with WebSocket updates
- Developers who prefer a lean SDK without extra features
- React applications with simple auth flows (email/password)
- Teams building MVP products with straightforward requirements

However, it's not a drop-in Supabase replacement for:
- Applications requiring passwordless authentication (OTP, magic links)
- Complex queries with relational joins
- Full-text search features
- Collaborative features (presence, broadcast)
- Advanced storage operations (move, copy, list, transformations)

The SDK is well-engineered with good TypeScript support, but the feature gap is significant in several critical areas. The recommended approach is to use MimDB for its strengths (simple REST, realtime, auth) and supplement with backend RPC calls for advanced queries or custom WebSocket implementations for presence/broadcast.

