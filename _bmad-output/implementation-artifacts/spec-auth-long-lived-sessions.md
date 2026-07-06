---
title: "Long-lived sessions: refresh-token rotation, reuse detection, and silent renewal"
type: "feature"
created: "2026-07-06"
status: "done"
baseline_commit: "577bcb2b59e86581710b5ec8b2d52abb4c2a2c08"
context:
  - "{project-root}/_bmad-output/project-context.md"
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Sessions die a hard 7 days after login no matter how active the user is: `/auth/refresh` reissues only the access token and never re-sets the stateless 7-day refresh JWT. And because refresh tokens are stateless (no DB record), the server cannot rotate, revoke, or detect a stolen/replayed token.

**Approach:** Replace the stateless refresh JWT with a DB-backed **opaque** refresh token (random value, only its SHA-256 hash stored — mirroring `password_reset_tokens`). Rotate it on every refresh with a **sliding 30-day idle window** and a **180-day absolute cap**, so an active user effectively stays logged in ("forget about it"). Group a login's rotations into a **family** (one session/device). A replayed (already-rotated) token outside a short grace window is the compromise signal → **revoke that family only**. Add **proactive client-side silent renewal** (refresh shortly before the access token lapses, only while the tab is visible), keeping today's reactive 401 path as a safety net. Access token stays a 15-minute JWT — unchanged validation.

## Boundaries & Constraints

**Always:**
- Only the SHA-256 hash of the refresh token is persisted; the raw value lives only in the httpOnly cookie. Reuse the `password_reset_tokens` opaque-token+hash+repository-interface pattern.
- Every successful refresh rotates the token: mark the presented token consumed, mint a successor in the **same family**, set a new cookie. Exactly one live (un-rotated, un-revoked) token per family at a time.
- Sliding idle = each issue sets `expires_at = min(now + idleTTL, family_expires_at)`; absolute cap `family_expires_at` is fixed at login and copied unchanged across rotations. Cookie `Max-Age` tracks the idle TTL.
- Reuse detection revokes **only the presented token's family** (per the chosen policy); other families/devices for the same user keep working. Logout revokes only the current family.
- A short reuse **grace window** (~20s) tolerates benign races (multi-tab, retried request) without revoking — heal the client to a live token instead of nuking the session.
- Access-token JWT format, `"access"`-audience validation (middleware + WS handshake), and the `RegisterResponseData` response envelope are unchanged. Refresh cookie attributes stay `HttpOnly`, `Secure` (non-dev), `SameSite=Strict`, `Path=/api/v1/auth`.
- TTLs are config-driven (env), not hardcoded across two files as today.

**Ask First:**
- Adding immediate WebSocket force-disconnect on family revocation (today WS validates only at connect; a revoked session's socket lingers until its 15-min access token would next refresh — treated as out of scope below).
- Adding a "your sessions" management/listing UI or a device label column.

**Never:**
- No change to access-token TTL semantics beyond making it configurable (stays 15 min default) or to `"access"`-audience checks.
- No refresh-token persistence on the client (access token stays in-memory Zustand; refresh stays httpOnly cookie).
- No revoking *all* the user's sessions on reuse (chosen: affected family only).
- No new heavyweight dependency for client-side JWT decoding — decode the `exp` claim manually.
- Do not batch multi-event WS sequences or touch game/session code.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Login / register | Valid credentials | New family created; opaque refresh token minted, hash stored (`expires_at`=now+30d, `family_expires_at`=now+180d); cookie set (Max-Age 30d); access JWT returned | N/A |
| Active refresh | Cookie holds the live token | Token rotated; successor cookie set; idle window slid to now+30d (capped at family cap); new access JWT returned | N/A |
| Idle > 30 days | Live token past `expires_at` | Family revoked; cookie cleared; 401 → client re-logs in | 401 UNAUTHORIZED |
| Absolute cap reached | `family_expires_at` in the past | Family revoked; cookie cleared; 401 | 401 UNAUTHORIZED |
| Token replay (attack) | Already-rotated token presented after grace | Family revoked; cookie cleared; 401. Next refresh with the (now-revoked) live token also 401s → that session logged out | 401 UNAUTHORIZED |
| Benign race within grace | Rotated token re-presented within ~20s (multi-tab / retry) | No revocation; a fresh access JWT returned and client left holding a live token | N/A |
| Logout | Valid cookie | Presented token's family revoked + cookie cleared; other devices unaffected | N/A |
| Multi-device | Two logins for one user | Two independent families; rotating/revoking one never affects the other | N/A |
| Stale JWT cookie (post-deploy) | Old 7-day refresh JWT in cookie | Hash not found in DB → cookie cleared, 401 → one-time re-login | 401 UNAUTHORIZED |
| Proactive renewal | Tab visible, access token near expiry | Silent refresh fires before expiry; no request ever 401s; backgrounded/idle tabs do not spin refreshes | On failure → fall through to reactive 401 / logout |

</frozen-after-approval>

## Code Map

**New — backend**
- `server/migrations/000013_create_refresh_tokens.up.sql` / `.down.sql` -- `refresh_tokens` table (mirror 000008 shape) + indexes
- `server/internal/refreshtoken/model.go` -- `RefreshToken` struct (id, user_id, family_id, token_hash, expires_at, family_expires_at, rotated_at, revoked_at, created_at)
- `server/internal/refreshtoken/repository.go` -- `Repository` interface: `Create`, `FindByHash`, `Rotate(id)`, `FindLiveByFamily(familyID)`, `RevokeFamily(familyID)`
- `server/internal/refreshtoken/gorm_repo.go` -- GORM impl; `Rotate` uses `WHERE id=? AND rotated_at IS NULL` for atomicity (mirror `MarkUsed`)

**Modified — backend**
- `server/internal/auth/service.go` -- drop `GenerateRefreshToken` (JWT); add opaque `mintRefreshToken()` (raw + SHA-256 hash) and `newFamilyID()`; give `GenerateAccessToken` a configurable TTL
- `server/internal/auth/handler.go` -- `AuthHandler` gains `refreshRepo` + TTL fields; `setRefreshCookie` takes a `maxAge`; Register/Login create a family; **Refresh** does rotation + reuse detection + sliding + grace; **Logout** revokes the family
- `server/internal/config/config.go` -- add `AccessTokenTTL` (15m), `RefreshIdleTTL` (720h), `RefreshAbsoluteTTL` (4320h) from env via a duration helper
- `server/cmd/api/main.go` -- construct `refreshtoken.NewGormRepository(db)`; pass repo + TTLs into `NewAuthHandler`
- `.env.example` -- document `BELJOT_ACCESS_TOKEN_TTL`, `BELJOT_REFRESH_IDLE_TTL`, `BELJOT_REFRESH_ABSOLUTE_TTL`

**Modified — frontend**
- `client/src/shared/hooks/useTokenRefresh.ts` -- **NEW** hook: decode access-token `exp`, schedule a refresh at ~75% of lifetime, gate on `document.visibilityState`; coordinate across tabs via `BroadcastChannel` so only one tab refreshes and shares the new token
- `client/src/App.tsx` -- call `useTokenRefresh()` alongside `useAuthInit()`
- `client/src/shared/api/axiosClient.ts` -- broadcast the new access token from `doRefresh` on success (feeds cross-tab coordination); no change to the 401 cycle itself
- `client/src/shared/hooks/useWebSocket.ts` -- route `handleAuthFailure` refresh through the shared coordinated refresh (avoid a second concurrent refresh path)

## Tasks & Acceptance

**Execution:**
- [x] `server/migrations/000013_create_refresh_tokens.up.sql` / `.down.sql` -- create `refresh_tokens` + unique index on `token_hash`, indexes on `family_id` and `user_id`; down drops the table -- durable session store
- [x] `server/internal/refreshtoken/{model,repository,gorm_repo}.go` -- new package mirroring `passwordreset`; atomic `Rotate` and family-scoped `RevokeFamily` -- persistence boundary for rotation/reuse/revocation
- [x] `server/internal/auth/service.go` -- opaque `mintRefreshToken` + `newFamilyID`, configurable access-token TTL; remove refresh JWT generation -- token minting
- [x] `server/internal/auth/handler.go` -- family creation on Register/Login; rotation + reuse-detection + sliding + grace in Refresh; family revoke + cookie clear in Logout; `setRefreshCookie(maxAge)` -- core policy
- [x] `server/internal/config/config.go` -- add the three TTL settings with defaults + env duration parsing -- config-driven TTLs
- [x] `server/cmd/api/main.go` -- wire the refresh-token repo and TTLs into `NewAuthHandler` -- DI
- [x] `.env.example` -- add the three new vars with default values -- ops docs
- [x] `server/internal/auth/handler_test.go` (+ `refreshtoken` repo tests) -- table-driven cases for every I/O Matrix row: rotation, sliding, idle/absolute expiry, reuse→family-revoke, grace race, logout revoke, multi-family isolation -- correctness
- [x] `client/src/shared/hooks/useTokenRefresh.ts` + `.test.tsx` -- proactive scheduler with visibility gating + cross-tab coordination; tests assert scheduling from a decoded `exp` and no-fire when hidden -- silent renewal
- [x] `client/src/App.tsx`, `client/src/shared/api/axiosClient.ts`, `client/src/shared/hooks/useWebSocket.ts` -- wire the hook, broadcast on refresh, single coordinated refresh path -- integration

**Acceptance Criteria:**
- Given an active user, when they keep using the app past 7 days (refreshing within each 30-day window), then they are never forced to log in until the 180-day cap.
- Given a captured refresh token that is replayed after it has already been rotated (outside grace), when it hits `/auth/refresh`, then that family is revoked and both the attacker and the legitimate client are logged out — while the user's other devices stay logged in.
- Given two devices logged into one account, when one logs out, then the other's session keeps refreshing normally.
- Given a visible tab whose access token is about to expire, when the renewal timer fires, then a refresh completes silently with no failed (401) request; given the tab is hidden, then no proactive refresh fires.
- Given a user holding a pre-deploy 7-day JWT cookie, when the app boots, then they are cleanly logged out once and can log back in.

## Spec Change Log

### 2026-07-06 — Review round 1 (bad_spec correction; Design Notes only, frozen block unchanged)

**Triggering findings (step-04 adversarial review):** (1) grace-path *heal-by-rotation* consumed the winning refresh's token, so under multi-tab last-write-wins the shared cookie could land on a consumed token and trip reuse detection ~1 cycle later → recurring spurious logout; (2) a logout/reuse-revoke racing a refresh still issued an access JWT (stale snapshot, no re-check); (3) `Rotate`+`Create` were not atomic (a failed insert stranded the family with no live token); (4) idle-expiry keyed off the presented (possibly consumed) token's frozen `expires_at`; (5) deleted-user refresh minted orphan tokens and never revoked the family.

**Amended (Design Notes only; frozen Intent/Boundaries/I-O matrix/ACs unchanged and still satisfied):** grace heal now serves access-only (no re-rotation, no cookie write); idle-expiry moved to the live-token branch; rotation made atomic via `RotateAndReplace`; user loaded (and family revoked if gone) before any rotation; the lost-rotation-race path re-reads and rejects a revoked family. Client: proactive-refresh failures re-arm a short retry instead of being swallowed.

**Known-bad state avoided:** multi-tab users force-logged-out every ~20 min; revoked/logged-out sessions retaining API+WS access for the access-token TTL; a transient DB error permanently killing a session.

**KEEP (must survive re-derivation):** opaque-token + SHA-256-hash storage mirroring `passwordreset`; one-live-token-per-family invariant; family-only revocation on reuse; 15-min access JWT + `"access"`-audience checks unchanged; config-driven TTLs; full I/O-matrix coverage in `refresh_test.go`.

### 2026-07-06 — Review round 2 (focused re-review of the round-1 fixes)

**Confirmed:** all six round-1 findings resolved and test-backed. **New finding fixed:** `RevokeFamily` was not serialized against a concurrent winning `RotateAndReplace` — under Postgres READ COMMITTED a revoke could miss a just-minted successor, leaving a "revoked" family with one live token (violating the "both attacker and legitimate client are logged out" AC). Fix: both operations now take a per-family transaction-scoped advisory lock (`pg_advisory_xact_lock(hashtextextended(family_id, 0))`), so rotate and revoke for a family run one-at-a-time. Also capped `startSession`'s first-token `expires_at` at the absolute deadline (consistency with `newSuccessor`). **Accepted (no change):** a within-grace consumed replay may yield one access-only token (no cookie) off a stale snapshot — inherent to stateless access JWTs, which live their 15-min TTL regardless of family state.

## Design Notes

**Refresh algorithm (server, `/auth/refresh`):** hash the cookie value → `FindByHash`.
1. Not found, or `revoked_at != nil` → clear cookie, 401.
2. `now > family_expires_at` (absolute cap) → `RevokeFamily`, clear cookie, 401.
3. Load the user; if gone (deleted after the token was issued) → `RevokeFamily`, clear cookie, 401. Never rotate/mint for a vanished user.
4. `rotated_at != nil` (a **consumed** token was presented) → within grace (`now - rotated_at <= GRACE`) it is a benign race (multi-tab / retried request / bootstrap): return a fresh access JWT **without rotating and without touching the cookie**, so the live successor set by the request that won the rotation stays the browser's cookie. Past grace → **reuse detected** → `RevokeFamily`, clear cookie, 401.
5. `rotated_at == nil` (**live** token): idle check applies HERE (`now > expires_at` → `RevokeFamily`, clear, 401 — idle is a property of the live token, not a spent sibling). Then `RotateAndReplace(id, successor)` — one transaction that stamps `rotated_at` (guarded `rotated_at IS NULL AND revoked_at IS NULL`) and inserts the successor (`expires_at = min(now+idle, family_expires_at)`, `family_expires_at` copied unchanged), so a failed insert rolls back the rotation and leaves the old token live. Won → set cookie(successor), return access JWT. Lost the race → re-read by hash; if `revoked_at != nil`/gone → 401; else the token is now consumed → apply the case-4 grace/reuse logic on the fresh row.

Heal-without-rotation is deliberate: only the raw token (never the stored hash) can be placed in a cookie, so re-rotating on a benign race would consume the winner's fresh token and — under last-write-wins on the shared cookie — could strand the browser on a consumed token that trips reuse detection a cycle later. Serving access-only keeps the winner's live token as the cookie and, as a bonus, denies a within-grace replayer any refresh cookie. Residual: a genuinely lost rotation response (server rotated, `Set-Cookie` never reached the browser) leaves the browser on a consumed token → one reuse-triggered logout on the next cycle — a rare partial-failure trade-off accepted for this game. Client-side, the tab that refreshes broadcasts the new token so siblings adopt it; concurrent refreshes stay safe regardless. Row growth is bounded per family; pruning long-rotated rows is deferred (see `deferred-work.md`).

**Client scheduler:** decode the JWT payload (`atob` the middle segment, JSON-parse, read `exp`) — no dependency. Schedule `refresh()` at ~75% of `(exp - iat)`; reschedule on each new token; skip/defer when `document.hidden`; on `visibilitychange` back to visible, refresh immediately if past the threshold. A `BroadcastChannel('auth')` message carries the freshly obtained access token so sibling tabs adopt it and reset their timers instead of each hitting `/auth/refresh`.

**Migration impact:** existing refresh cookies are JWTs whose hash is absent from `refresh_tokens` → they 401 on first refresh → one-time re-login for everyone (accepted).

## Verification

**Commands:**
- `make migrate` (with `BELJOT_DB_URL` → dev DB on port 6433) -- expected: `000013` applies and rolls back cleanly
- `cd server && go test ./internal/auth/... ./internal/refreshtoken/...` -- expected: all pass, including reuse/rotation/grace cases
- `make lint` -- expected: `golangci-lint` + ESLint/Prettier clean
- `cd client && npx vitest run` -- expected: all pass incl. `useTokenRefresh.test.tsx`

**Manual checks:**
- DevTools → Application → Cookies: `refresh_token` value changes on each `/auth/refresh` (rotation); Network shows a silent refresh firing ~11 min in on an active tab with no 401.
- Copy the refresh cookie, refresh once (rotates), then replay the copied value via a second request after ~30s → 401 and the live session is logged out on its next refresh; a second logged-in device stays active.

## Suggested Review Order

**Core refresh policy (entry point)**

- The whole state machine: live → rotate, consumed → grace/reuse, expiry/revoke/gone-user gates — read this first
  [`handler.go:300`](../../server/internal/auth/handler.go#L300)
- Grace window that separates a benign race from a replay attack
  [`handler.go:26`](../../server/internal/auth/handler.go#L26)
- Consumed-token handling: in-grace = access-only (no rotate/no cookie), past-grace = revoke family
  [`handler.go:387`](../../server/internal/auth/handler.go#L387)

**Rotation atomicity & cross-family concurrency (highest risk)**

- Rotate + create-successor in one transaction; a failed insert rolls back the rotation
  [`gorm_repo.go:34`](../../server/internal/refreshtoken/gorm_repo.go#L34)
- Per-family advisory lock serializing rotate-vs-revoke so a revoke can't miss a fresh successor
  [`gorm_repo.go:91`](../../server/internal/refreshtoken/gorm_repo.go#L91)
- Family-scoped revocation under the same lock
  [`gorm_repo.go:74`](../../server/internal/refreshtoken/gorm_repo.go#L74)

**Session lifecycle**

- New family on login/register; sliding idle deadline capped at the absolute cap
  [`handler.go:113`](../../server/internal/auth/handler.go#L113)
- Logout revokes the presented session's family (other devices survive)
  [`handler.go:441`](../../server/internal/auth/handler.go#L441)

**Token minting & schema**

- Opaque token (base64) + SHA-256 hash; only the hash is ever stored
  [`service.go:75`](../../server/internal/auth/service.go#L75)
- Table + unique(token_hash) + family/user indexes + `ON DELETE CASCADE`
  [`000013…up.sql:1`](../../server/migrations/000013_create_refresh_tokens.up.sql#L1)
- Model: hash-only storage, sliding + absolute deadlines, rotated/revoked stamps
  [`model.go:19`](../../server/internal/refreshtoken/model.go#L19)

**Config & wiring**

- Config-driven TTLs (access 15m, idle 30d, absolute 180d)
  [`config.go:56`](../../server/internal/config/config.go#L56)
- Duration env parsing that rejects non-positive values
  [`config.go:118`](../../server/internal/config/config.go#L118)
- DI: refresh-token repo + TTLs into the auth handler
  [`main.go:57`](../../server/cmd/api/main.go#L57)

**Client silent renewal**

- Proactive scheduler: visibility-gated, reschedules on token change, retries on failure
  [`useTokenRefresh.ts:57`](../../client/src/shared/hooks/useTokenRefresh.ts#L57)
- 75%-of-lifetime scheduling math (manual `exp` decode, no dependency)
  [`useTokenRefresh.ts:44`](../../client/src/shared/hooks/useTokenRefresh.ts#L44)
- Coordinated single refresh + cross-tab token broadcast
  [`axiosClient.ts:100`](../../client/src/shared/api/axiosClient.ts#L100)
- WS re-auth routed through the same coordinated refresh
  [`useWebSocket.ts:151`](../../client/src/shared/hooks/useWebSocket.ts#L151)

**Tests (peripherals)**

- Reuse → family revoke, and in-grace race → access-only (no false logout)
  [`refresh_test.go:237`](../../server/internal/auth/refresh_test.go#L237)
- Atomic rotate: winner creates successor, loser creates nothing
  [`gorm_repo_test.go:79`](../../server/internal/refreshtoken/gorm_repo_test.go#L79)
- Proactive-failure retry re-arm
  [`useTokenRefresh.test.tsx:114`](../../client/src/shared/hooks/useTokenRefresh.test.tsx#L114)
