# MimDB JavaScript SDK - Supabase Feature Parity Implementation Plan

**Date:** March 28, 2026
**Based on:** `supabase-parity-audit.md` (corrected for inaccuracies)

## Audit Corrections

The audit report contains several inaccuracies. The following features are **already implemented** but were marked as MISSING:

- `ilike()`, `in()`, `is()`, `contains()`, `containedBy()`, `not()`, `or()` - in `filters.ts`
- `textSearch()` - in `filters.ts`
- `single()`, `maybeSingle()` - in `rest.ts`
- FK joins via `select('*, posts(*)')` - PostgREST handles natively, SDK passes through verbatim

This significantly reduces the actual gap.

---

## Phase 1: Quick Wins - SDK-Only (1-2 weeks)

No backend changes needed. Ship in a single minor release.

| # | Task | Package | Size | Type |
|---|------|---------|------|------|
| 1.1 | Add `range(from, to)` convenience method to QueryBuilder | client | S | SDK |
| 1.2 | Add `filter(column, operator, value)` low-level filter method | client | S | SDK |
| 1.3 | Add `resetPasswordForEmail(email)` - backend `/forgot-password` exists | client | S | SDK |
| 1.4 | Add `PASSWORD_RECOVERY` auth event type | client | S | SDK |
| 1.5 | Add OAuth `scopes` and `queryParams` to `signInWithOAuth()` | client | S | SDK |
| 1.6 | Add `createSignedUrls(paths[])` batch method (parallel Promise.all) | client | S | SDK |
| 1.7 | Add `list(prefix?, options?)` to BucketClient - backend handler exists | client | S | SDK |
| 1.8 | Add `update(path, body, opts)` to BucketClient (upsert semantics) | client | S | SDK |
| 1.9 | Extend `useQuery` hook to support all filter types | react | M | SDK |
| 1.10 | Add `signOut({ scope })` parameter (local scope now, global later) | client | S | SDK |

---

## Phase 2: Storage Completeness (2-3 weeks)

Backend handlers needed for move, copy, getBucket, emptyBucket.

| # | Task | Package | Size | Type | Depends |
|---|------|---------|------|------|---------|
| 2.1 | Add `GET /storage/{ref}/buckets/{name}` handler (repo method exists) | Backend | S | Backend | - |
| 2.2 | Add `getBucket(name)` to StorageClient | client | S | SDK | 2.1 |
| 2.3 | Add `MoveObject` service + `POST /storage/{ref}/object/move` handler | Backend | M | Backend | - |
| 2.4 | Add `CopyObject` service + `POST /storage/{ref}/object/copy` handler | Backend | M | Backend | - |
| 2.5 | Add `move(from, to)` and `copy(from, to)` to BucketClient | client | S | SDK | 2.3, 2.4 |
| 2.6 | Add upload progress tracking via `onProgress` callback | client | M | SDK | - |
| 2.7 | Add `progress` callback to `useUpload` hook | react | S | SDK | 2.6 |
| 2.8 | Add `emptyBucket(name)` service + handler | Backend | M | Backend | - |
| 2.9 | Add `emptyBucket(name)` to StorageClient | client | S | SDK | 2.8 |

---

## Phase 3: Auth Completeness (3-4 weeks)

Admin CRUD, password reset flow, email OTP/magic link.

| # | Task | Package | Size | Type | Depends |
|---|------|---------|------|------|---------|
| 3.1 | Add `POST /auth/{ref}/users` (create user, service_role) | Backend | M | Backend | - |
| 3.2 | Add `DELETE /auth/{ref}/users/{id}` (delete user, service_role) | Backend | M | Backend | - |
| 3.3 | Add `admin.createUser(data)` to AuthAdminClient | client | S | SDK | 3.1 |
| 3.4 | Add `admin.deleteUser(id)` to AuthAdminClient | client | S | SDK | 3.2 |
| 3.5 | Expand `admin.updateUserById` to support more fields | Both | M | Both | - |
| 3.6 | Add `resetPasswordForEmail` + `resetPassword(token, newPassword)` | client | S | SDK | - |
| 3.7 | Add email OTP/magic link: `POST /auth/{ref}/otp` endpoint | Backend | L | Backend | - |
| 3.8 | Add `signInWithOtp({ email })` and `verifyOtp()` to AuthClient | client | S | SDK | 3.7 |
| 3.9 | Add OTP methods to `useAuth` hook | react | S | SDK | 3.8 |
| 3.10 | Add `sendInvitationEmail(email)` to AuthAdminClient | Both | M | Both | 3.7 |

---

## Phase 4: Realtime - Broadcast Channels (3-4 weeks)

Pub/sub messaging without shared state. In-memory relay through the Hub.

| # | Task | Package | Size | Type | Depends |
|---|------|---------|------|------|---------|
| 4.1 | Extend backend realtime protocol with broadcast message types | Backend | L | Backend | - |
| 4.2 | Add `channel(name)` to MimDBRealtimeClient returning Channel | realtime | M | SDK | 4.1 |
| 4.3 | Implement `channel.on('broadcast', { event }, callback)` | realtime | M | SDK | 4.1 |
| 4.4 | Implement `channel.send({ type: 'broadcast', event, payload })` | realtime | S | SDK | 4.2 |
| 4.5 | Add broadcast `ack` and `self` config options | realtime | S | SDK | 4.2 |
| 4.6 | Add `useBroadcast(channelName, event)` hook | react | M | SDK | 4.2-4.4 |

---

## Phase 5: Realtime - Presence Tracking (4-6 weeks)

CRDT-based presence state. Builds on Phase 4 channel abstraction.

| # | Task | Package | Size | Type | Depends |
|---|------|---------|------|------|---------|
| 5.1 | Add per-channel presence state map to backend Hub | Backend | XL | Backend | 4.1 |
| 5.2 | Add `channel.track(state)` and `channel.untrack()` | realtime | M | SDK | 5.1 |
| 5.3 | Implement client-side presence reconciliation (sync/join/leave) | realtime | L | SDK | 5.1 |
| 5.4 | Add `channel.presenceState()` for full state map | realtime | S | SDK | 5.3 |
| 5.5 | Add `usePresence(channelName)` hook | react | M | SDK | 5.2-5.4 |
| 5.6 | Extend realtime row filter operators (gt, lt, in, like) | Backend | M | Backend | - |

---

## Phase 6: Edge Functions & Advanced Auth (5-7 weeks)

SDK function invocation, phone OTP, anonymous auth, PKCE.

| # | Task | Package | Size | Type | Depends |
|---|------|---------|------|------|---------|
| 6.1 | Add `FunctionsClient` with `invoke(name, opts)` | client | M | SDK | - |
| 6.2 | Wire `client.functions` lazy accessor | client | S | SDK | 6.1 |
| 6.3 | Integrate SMS provider (Twilio) into backend auth | Backend | L | External | - |
| 6.4 | Add phone OTP to `POST /auth/{ref}/otp` | Backend | M | Backend | 6.3 |
| 6.5 | Add phone OTP to `signInWithOtp({ phone })` | client | S | SDK | 6.4 |
| 6.6 | Add anonymous sign-in backend endpoint | Backend | M | Backend | - |
| 6.7 | Add `signInAnonymously(options?)` | client | S | SDK | 6.6 |
| 6.8 | Add PKCE OAuth flow support | Both | L | Both | - |

---

## Phase 7: Polish & Long-Tail (5-8 weeks, low priority)

| # | Task | Package | Size | Type |
|---|------|---------|------|------|
| 7.1 | Image transformation params on `getPublicUrl()` | client | S | SDK |
| 7.2 | `usePagination` hook with nextPage/prevPage | react | M | SDK |
| 7.3 | Upload pause/resume via tus protocol | client | L | SDK |
| 7.4 | Global signout backend support (revoke all tokens) | Backend | M | Backend |
| 7.5 | MFA enrollment and verification | Both | XL | Both |
| 7.6 | Signed URL download filename (Content-Disposition) | Both | S | Both |
| 7.7 | Schema filter for realtime subscriptions | Both | M | Both |
| 7.8 | **VERIFY:** FK joins already work via PostgREST passthrough | client | S | Investigation |

---

## Critical Path

```
Phase 1 (SDK quick wins) -- start immediately, no blockers
    |
Phase 2 (Storage) -- backend handlers first
    |
Phase 3 (Auth) -- backend endpoints first
    |
Phase 4 (Broadcast) -- backend protocol first
    |
Phase 5 (Presence) -- depends on Phase 4
    |
Phase 6 (Functions + Advanced Auth) -- SMS provider is external dep
    |
Phase 7 (Polish) -- independent, interleave anytime
```

## True Remaining P0 Gaps

After correcting audit inaccuracies:

1. **FK joins** - Likely already works. Verify (task 7.8).
2. **Presence tracking** - Phase 5, significant backend work.
3. **Broadcast channels** - Phase 4, backend protocol extension.
4. **OTP / Magic link** - Phase 3 (email), Phase 6 (phone).

Full-text search and advanced filter operators are already implemented.
