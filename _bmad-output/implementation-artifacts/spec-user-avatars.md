---
title: 'User avatars on object storage'
type: 'feature'
created: '2026-09-28'
status: 'ready-for-dev'
route: 'dispatch'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Every player surface (match seats, room rosters, friends, leaderboard, profile, nav pill) identifies a user only by a first-letter disc. There is no way to set a picture and no object-storage integration exists, although the public Garage bucket `beljot-public` and the six `BELJOT_S3_*` / `BELJOT_PUBLIC_ASSETS_URL` production variables are already provisioned. Local development already has Garage in `docker-compose.yml` (service `garage`, `garage.toml`, `.env.example` groups, 2026-09-28).

**Approach:** A user uploads, replaces or removes their own avatar from their profile page. The backend sniffs and decodes the image once, centre-crops it to a square, and writes **two WebP derivatives** under one unguessable prefix with the AWS SDK for Go v2 (path-style): `256.webp` for the profile hero and `128.webp` for every other disc. Only the prefix is stored in `users.avatar_key`. Every DTO that carries a user's identity gains a derived `avatarUrl` (the 128 image); the two profile responses also carry `avatarLargeUrl` (the 256 image). The shared client `Avatar` disc renders the image with the initial as fallback. Objects are immutable and cached for a year, so a repeat view costs no request.

**Resolved against the codebase (deviations from the brief, approved with this spec):**
- Routes are `PUT` and `DELETE /api/v1/users/:id/avatar` with the existing "`:id` must be the caller" check, mirroring `PATCH /users/:id/username`; the API has no `/users/me/*` routes, so the brief's `me` form is not introduced.
- The wire field is `avatarUrl` (`string | null`), camelCase like every other JSON tag; `avatar_url` would be the API's only snake_case key.
- JPEG EXIF orientation is applied before cropping so phone portraits are not stored sideways; the output still carries no metadata.
- Two derivatives replace the brief's single 256 image (owner request 2026-09-28): list and seat discs are 24 to 64 CSS px, so 128 px covers them at 2x for a few KB each, and the 96 px hero gets 256.

## Boundaries & Constraints

**Limits (server-enforced; the client mirrors the first two before sending):**
- File: at most 2 MiB = 2,097,152 bytes. Request body capped at 2 MiB + 16 KiB by `http.MaxBytesReader`, the file part itself at 2 MiB → 413.
- Types: JPEG, PNG, WebP, decided by `http.DetectContentType` on the bytes; filename and declared type are ignored. Animated WebP and anything else → 415.
- Source dimensions, checked with `image.DecodeConfig` before any pixel is decoded: shorter side ≥ 128 px → else 400; each side ≤ 4096 px and ≤ 16 megapixels → else 413.
- Processing: at most 4 image decodes in flight per process (semaphore); a request that waits more than 10 s for a slot → 503. Each S3 call runs under a 15 s timeout.
- Uploads: 10 per user per rolling hour (process-local limiter, single-replica deploy) → 429. `DELETE` is not limited.
- Output: exactly two lossy WebP files, 256×256 and 128×128, quality 80, method 4; no metadata.

**Always:**
- Pure Go only (`CGO_ENABLED=0` image): `github.com/aws/aws-sdk-go-v2` (`config`, `credentials`, `service/s3`), `github.com/gen2brain/webp` (wazero) to encode, `golang.org/x/image` (`draw` to resize, `webp` to decode). If gen2brain/webp cannot build or run with `CGO_ENABLED=0`, encode JPEG q85 with the stdlib, use `.jpg`, and say so in the PR report.
- Keys `avatars/<uuid v4>/256.webp` and `avatars/<uuid v4>/128.webp`; `users.avatar_key` stores the prefix `avatars/<uuid v4>`. `PutObject` with `Content-Type: image/webp` and `Cache-Control: public, max-age=31536000, immutable`. Every upload gets a new prefix; the previous pair is deleted best-effort only after the DB update commits (log on failure, never fail the request).
- Config: all six variables required when `BELJOT_ENV != development` (`slog.Error` + `os.Exit(1)`, the JWT-secret pattern). In development an empty `BELJOT_S3_ENDPOINT` disables the feature: both endpoints answer 503 and everything else runs without Garage.
- The access key, secret and endpoint never reach the client, build args, or any committed file other than `.env.example` and the compose defaults.
- Client images carry explicit `width` and `height`, `decoding="async"`, and `loading="lazy"` everywhere except the profile hero, so lists never shift layout and off-screen discs cost nothing.
- i18n in all four locales: mk all-Cyrillic and idiomatic, hr/sr natural rather than literal; em-dashes only in en.

**Never:**
- Presigned URLs, proxying avatar reads, storing a full URL, guessable keys, new buckets, or any change to bucket permissions, Dokploy or Garage.
- Changing an existing endpoint beyond adding `avatarUrl` / `avatarLargeUrl`.
- Extending text-only payloads: chat and whisper messages, WS friend-request / room-invite / surrender / disconnect notices, `ReconnectOverlay`, `CreateRoomModal` preview, landing mocks. Rosters re-read from REST, so at worst an initial shows briefly.
- Adding `UpdateAvatarKey` to the `UserRepository` interface (seven test mocks would break); the avatar package declares its own narrow interface.
- Hiding the upload UI in development when storage is disabled; the 503 surfaces as a toast.
- Client-side resizing before upload; the server is the only place that decodes.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Happy upload | `PUT`, multipart field `avatar`, 1.2 MiB 1600×1200 JPEG, own id | 200 `{"avatarUrl":"<base>/avatars/<uuid>/128.webp","avatarLargeUrl":"<base>/avatars/<uuid>/256.webp"}`; `avatar_key` = `avatars/<uuid>`; both objects exist, square, WebP | N/A |
| Replace | user already has prefix P1 | P2 pair uploaded, DB updated, then P1 pair deleted | P1 delete failure logged, still 200 |
| Remove | `DELETE`, key set or NULL | 204 either way; column NULL; pair deleted best-effort | N/A |
| Someone else's id | `:id` ≠ token user | 403 `FORBIDDEN` | existing sentinel |
| No file part | field `avatar` absent | 400 `AVATAR_MISSING` | |
| Too many bytes | file part > 2 MiB, or body over the cap | 413 `AVATAR_TOO_LARGE` (detect `*http.MaxBytesError`) | |
| Wrong type | GIF, SVG, PDF, text, animated WebP, or undecodable bytes named `.jpg` | 415 `AVATAR_UNSUPPORTED_TYPE` | |
| Too small | 64×64 PNG | 400 `AVATAR_TOO_SMALL` | |
| Oversized dimensions | `DecodeConfig` reports 5000 px on a side, or 4000×4001 (> 16 MP) | 413 `AVATAR_DIMENSIONS_TOO_LARGE`, before full decode | |
| Portrait phone JPEG | EXIF orientation 6 | both derivatives upright | |
| Non-square | 1000×600 PNG | centre 600×600 crop, then 256 and 128 | |
| Rate limit | 11th upload within 60 min | 429 `AVATAR_UPLOAD_RATE_LIMITED` | |
| Busy | 4 decodes in flight and no slot within 10 s | 503 `AVATAR_BUSY` | |
| Storage disabled (dev) | `BELJOT_S3_ENDPOINT` empty | 503 `AVATAR_STORAGE_UNAVAILABLE` on PUT and DELETE | |
| Storage or DB failure | either `PutObject` fails, or the UPDATE fails after upload | 500 `INTERNAL_ERROR`; column unchanged; any freshly uploaded object deleted best-effort | logged |
| Client: oversize pick | file > 2,097,152 bytes chosen | inline message, no request | |
| Client: image fails to load | `<img>` `onError` | initial disc renders instead | |

</frozen-after-approval>

## Code Map

**Backend (`server/`)**
- `internal/config/config.go` -- `Config`, `Load()`, JWT fail-fast (l.80-86), `SMTPConfigured()` predicate shape.
- `cmd/api/main.go` -- constructor injection; mailer real/log fallback; `/users/:id/*` routes l.190-200; `inviteFriendDirectory` adapter; `appErrorHandler`.
- `internal/apperr/errors.go` -- `NewAppError`; `ErrUsernameChangeTooSoon` is the 429 example.
- `internal/user/model.go` (`User`), `gorm_repo.go` (`UpdateUsername` update style, inline `r.db.Transaction`), `handler.go` (self-check l.759-771, `getUserID`; DTOs `ProfileResponse`, `PublicProfileResponse`, `MatchPlayer`, `PartnerStat`, `RivalStat`, `PlayerSearchResult`; `loadUsernamesForAggregates` and `loadUsernamesForMatches` return `map[uint]string`, widen to name + avatar key).
- `internal/auth/handler.go` -- `RegisterResponseData` via `authResponseData` (login, register, refresh, SSO); this becomes the client `User`.
- `internal/friend/handler.go` -- `PendingRequestDTO`, `FriendDTO`, `usernamesFor`.
- `internal/season/gorm_repo.go` raw selects l.310 and l.343 (add `users.avatar_key`); `model.go` `LeaderboardEntry`; `handler.go` `LeaderboardRowView`; `service.go` mapping.
- `internal/room/model.go` -- `Room.OwnerUsername`, `RoomPlayer.Username` (`gorm:"-"`); `gorm_repo.go` `roomPlayerRow` scanned by the JOINs in `FindPlayersByRoomID` / `FindPlayerBySeat` / `FindPlayersByRoomIDs`, `LoadOwnerUsernames`.
- `internal/room/handler.go` hand-built WS maps carrying a roster `username`: `system:player_joined` l.1100, 4147; `system:seat_updated` l.2424, 3177, 3474; `roomLifecyclePayload` l.391; `system:room_owner_changed` l.2990; `PlayerSeatInfo` built l.2461, 3600; also `lobby_disconnect.go` l.254-265.
- `internal/room/invite_handler.go` -- `InvitableFriendDTO` via `room.FriendSummary`.
- `internal/match/model.go` `PlayerSeatInfo`; `live_match.go` `StartMatch` stamps `Level` at l.383 (copy for `AvatarURL`); `internal/game/state.go` `PlayerState`.
- `internal/ws/events_contract_test.go` + `testdata/events/event_match_state.json` -- `UPDATE_GOLDENS=1 go test ./internal/ws/...` regenerates.
- `internal/emote/handler.go` `acceptRateLimited` -- mutex + map limiter shape.
- `migrations/` -- highest is `000029_*`; `embed_test.go` requires a matching down file.
- Tests to mirror: `user/handler_test.go` (`setupUserHandlerWithSeason`, `TestUpdateUsername_*`, httptest through `auth.AuthMiddleware`); `user/user_test.go` `getTestDB` (skips without `BELJOT_DB_URL`).

**Client (`client/src/`)**
- `shared/components/ui/avatar.tsx` -- `Avatar({name,size,team,owner,you,halo,icon})`, initial disc, `aria-hidden`; sizes in use: 24, 28, 42, 44, 56, the 64 (44 compact) match-seat disc, and 96 for the hero.
- `<Avatar name=…>` call sites: `features/friends/FriendList.tsx`, `features/friends/FriendRequests.tsx`, `features/lobby/components/PlayerSearch.tsx`, `features/profile/components/IdentityHero.tsx`, `Rivalries.tsx`, `PartnerSpotlight.tsx`, `features/room/components/SeatTile.tsx`, `features/room/RoomPage.tsx` (roster), `features/room/components/InviteFriendsDialog.tsx`, `features/match/components/TrumpReveal.tsx`, `shared/components/matchStats/SeatChip.tsx`, `shared/components/matchStats/MatchPlayerActions.tsx`.
- Hand-rolled initial discs: `features/match/components/PlayerSeat.tsx` (l.272 initial, l.337-375 conic team frame, `AVATAR_DISC_PX` constants; keep `data-testid="player-seat-avatar"`, queried by `useDealController.ts`), `shared/components/TopBar.tsx` nav pill (l.259-264), `features/lobby/components/SeatChip.tsx`, `features/lobby/components/RoomCard.tsx` owner circle, `shared/components/season/LeaderboardRow.tsx` (no disc today; `username` is a plain prop, the self row's name comes from the auth store).
- Types (hand-written, camelCase wire): `shared/types/apiTypes.ts` (`User`, `PlayerSearchResult`, `Friend`, `FriendRequest`, `InvitableFriend`, `Room`, `RoomPlayer`, `LeaderboardRow`), `shared/api/profile.ts` (`ProfileResponse`, `PublicProfileResponse`), `shared/api/career.ts` (`PartnerStat`, `RivalStat`), `shared/api/matches.ts` (`MatchPlayer`), `shared/api/auth.ts` (`RegisterResponse`, `RefreshResponse`), `shared/types/matchTypes.ts` `PlayerState`, `shared/types/wsEvents.ts` (`SeatUpdated`, `PlayerJoined`), `shared/types/wsEvents.schemas.ts` `PlayerStateSchema` (`z.strictObject`: the Go field and this entry must land together or `wsEvents.contract.test.ts` fails). `User` is built field by field in `shared/hooks/mutations/useAuth.ts` and `shared/api/axiosClient.ts`.
- `shared/api/axiosClient.ts` -- default `Content-Type: application/json` makes axios serialise `FormData` as JSON; 15 s timeout; errors are `FetchError{status, code, message}`.
- `shared/hooks/mutations/useProfile.ts` -- `useUpdateUsernameMutation` does `setQueryData(profile.detail)` + `useAuthStore.getState().setUser(...)` (pattern to copy); keys in `shared/api/queryKeys.ts`.
- `features/profile/components/EditableUsername.tsx` (+ `.test.tsx`) -- owner-only gate via `userId`, `FetchError` → message mapping; `UnlinkAccountDialog.tsx` -- not dismissible while pending; `IdentityHero.tsx` also serves `PublicPlayerProfilePage.tsx` without `userId`.
- `shared/i18n/{en,mk,hr,sr}.json` (single namespace, nested camelCase keys) + `i18n.parity.test.ts`; `src/test-utils.tsx` (`makeUser`, `QueryWrapper`).

**Infra (already in place, do not redo)**
- `docker-compose.yml` service `garage` (`dxflrs/garage:v2.3.0`, `--single-node --default-bucket`, ports 3900 and 3902, `garagedata` volume, defaults for `GARAGE_RPC_SECRET` and the `GARAGE_DEFAULT_*` trio), `garage.toml` at the repo root (comment-free), `.env.example` groups `Garage (Docker Compose)` and `Avatar storage` with the local values, `Makefile` `dev` echo, `docs/DEPLOYMENT.md` local-development block including the one-time `docker compose exec garage /garage bucket website --allow beljot-public`. Runtime-verified 2026-09-28: `docker compose up -d garage` boots, the default bucket and key exist, website mode enabled, a sigv4 path-style `PutObject` with `Cache-Control: public, max-age=31536000, immutable` is served back at `http://beljot-public.web.garage.localhost:3902/<key>` with both headers intact, the bucket root answers 404, and `DeleteObject` works.
- `docs/DEPLOYMENT.md` backend env table (l.49-68, `Variable | Required | Secret | Description`) still lacks the six rows. No CSP exists anywhere, so no header change is needed.

## Tasks & Acceptance

**Execution:**
- [ ] `server/migrations/000030_add_avatar_key_to_users.up.sql` / `.down.sql` -- `ALTER TABLE users ADD COLUMN avatar_key text NULL;` / `ALTER TABLE users DROP COLUMN avatar_key;`.
- [ ] `server/internal/config/config.go` -- `S3Endpoint`, `S3Region`, `S3AccessKey`, `S3SecretKey`, `S3PublicBucket`, `PublicAssetsURL` (trailing slash trimmed); `AvatarStorageConfigured()`; non-development fail-fast naming the missing variable.
- [ ] `server/internal/apperr/errors.go` -- the sentinels named in the matrix (400 ×2, 413 ×2, 415, 429, 503 ×2).
- [ ] `server/internal/user/model.go`, `gorm_repo.go`, new `avatar_url.go`, `user_test.go` -- `AvatarKey *string` (`column:avatar_key`, `json:"-"`); `UpdateAvatarKey(id uint, key *string) (previous *string, err error)` in one transaction (`SELECT … FOR UPDATE`, then update); `SetPublicAssetsURL(base)` + `AvatarURL(key *string, size int) *string` returning `<base>/<key>/<size>.webp`, nil when base or key is empty; repository test.
- [ ] `server/internal/avatar/` (new: `store.go`, `image.go`, `handler.go`, `image_test.go`, `handler_test.go`) -- `Store` interface (`Put(ctx, key, contentType, cacheControl string, body []byte) error`, `Delete(ctx, keys ...string) error`) with the S3 implementation (static credentials, region, `BaseEndpoint`, `UsePathStyle: true`, 15 s per call); `Process(r io.Reader) (large, small []byte, err error)` = sniff → `DecodeConfig` limits → decode → EXIF orientation → centre-crop → `draw.CatmullRom` to 256 → `draw.CatmullRom` 256→128 → encode both; handler with `http.MaxBytesReader`, field `avatar`, semaphore of 4 with a 10 s wait cap, per-user window, upload both → DB → delete old pair, and its own narrow users interface; tests cover every matrix row with a fake store and generated JPEG/PNG/WebP fixtures.
- [ ] `server/cmd/api/main.go` -- build the S3 client only when `AvatarStorageConfigured()`, else inject nil so the handler answers 503; call `user.SetPublicAssetsURL`; register both routes beside the username route.
- [ ] Server DTO sweep -- `AvatarURL *string \`json:"avatarUrl"\`` (128) on `RegisterResponseData`, `ProfileResponse`, `PublicProfileResponse`, `PlayerSearchResult`, `MatchPlayer`, `PartnerStat`, `RivalStat`, `FriendDTO`, `PendingRequestDTO`, `LeaderboardEntry` + `LeaderboardRowView`, `Room.OwnerAvatarURL`, `RoomPlayer.AvatarURL` (JOINs scan `users.avatar_key`), `FriendSummary` + `InvitableFriendDTO`, `PlayerSeatInfo`, `game.PlayerState` (stamped in `StartMatch`), and every hand-built map in the Code Map; `AvatarLargeURL \`json:"avatarLargeUrl"\`` (256) on `ProfileResponse` and `PublicProfileResponse` only; regenerate goldens.
- [ ] `client/src/shared/components/ui/avatar.tsx` + `avatar.test.tsx` -- `avatarUrl?: string | null`, `eager?: boolean`; render `<img alt="" width={size} height={size} decoding="async" loading={eager ? "eager" : "lazy"} className="size-full rounded-full object-cover">` inside the disc, `onError` state falls back to the initial; ring, halo and `icon` unchanged.
- [ ] Client types -- `avatarUrl: string | null` on every type in the Code Map, plus `avatarLargeUrl: string | null` on `ProfileResponse` and `PublicProfileResponse`; `PlayerStateSchema` gains `avatarUrl: z.string().nullable()`; both `User` builders copy it.
- [ ] Client call-site sweep -- pass `avatarUrl` at all twelve `<Avatar>` sites (`IdentityHero` passes `avatarLargeUrl` with `eager`); convert the five hand-rolled discs (`LeaderboardRow` gets an `avatarUrl` prop, the self row reading `authStore.user`); adjust `PlayerSeat.test.tsx` if the markup changes.
- [ ] `client/src/shared/api/profile.ts` -- `uploadAvatar(userId, file, onProgress)` (`FormData`, `Content-Type: multipart/form-data` override, 60 s timeout, axios `onUploadProgress`) returning both URLs, and `removeAvatar(userId)`.
- [ ] `client/src/shared/hooks/mutations/useProfile.ts` -- `useUploadAvatarMutation`, `useRemoveAvatarMutation`: patch `profile.detail` (both URLs), `setUser({ ...user, avatarUrl })`, invalidate `publicProfile.detail`.
- [ ] `client/src/features/profile/components/AvatarDialog.tsx` + `.test.tsx`, `IdentityHero.tsx` -- edit button over the hero disc only when `userId` is passed; dialog with `<input type="file" accept="image/jpeg,image/png,image/webp">`, `file.size > 2_097_152` check with an inline message, `URL.createObjectURL` preview (revoked on close), progress bar, Upload, Remove (only when an avatar exists), not dismissible while pending; `FetchError.status` 400/413/415/429/503 → `profile.avatar.errors.*`; `toast.success` on completion.
- [ ] `client/src/shared/i18n/en.json`, `mk.json`, `hr.json`, `sr.json` -- `profile.avatar.*` (title, pick, upload, remove, uploading, success, errors.tooLarge, errors.tooSmall, errors.unsupported, errors.rateLimited, errors.busy, errors.unavailable).
- [ ] `docs/DEPLOYMENT.md` -- six rows in the backend table (Required yes; Secret yes for key and secret) and an "Object storage" paragraph (bucket, `avatars/<uuid>/{128,256}.webp` layout, public by design, immutable cache headers).
- [ ] PR description -- the report in the brief's order: migration number and SQL; endpoints, shapes and limits; encoder and resulting dependency list; confirmation that the production variables suffice; local-dev steps; assumptions made.

**Acceptance Criteria:**
- Given a database at version 29, when the server starts, then migration 30 applies and `users.avatar_key` exists as nullable text.
- Given `BELJOT_ENV=production` with any of the six variables unset, when the server starts, then it logs an error naming the variable and exits with status 1.
- Given `BELJOT_ENV=development` and `BELJOT_S3_ENDPOINT` empty, when the server starts, then every other feature works and both avatar endpoints answer 503.
- Given a user with an avatar, when any endpoint or WS payload listed in the Code Map returns that user, then it carries `avatarUrl` equal to `<BELJOT_PUBLIC_ASSETS_URL>/<avatar_key>/128.webp`, users without one carry `null`, and only the two profile responses also carry `avatarLargeUrl`.
- Given that user seated in a match, when `event:match_state` is emitted, then each `players[]` entry carries `avatarUrl` and `wsEvents.contract.test.ts` passes against the regenerated golden.
- Given the signed-in user's own profile, when they upload a valid image, then the hero disc shows the 256 image, the nav pill the 128 image, and the profile query updates without a reload.
- Given another user's public profile, when it is viewed, then no avatar edit affordance is rendered.
- Given the four locale files, when `i18n.parity.test.ts` runs, then it passes with the new keys.
- Given `make lint`, `make test` and `CGO_ENABLED=0 go build ./cmd/api`, when run, then all pass.

## Implementation Notes

## Spec Change Log

- 2026-09-28 (owner request, pre-implementation): limits made explicit and stricter (min 128 px, max 4096 px / 16 MP, decode semaphore, per-call S3 timeout); one 256 image replaced by a 128 + 256 pair under a shared prefix, `avatarLargeUrl` added to the two profile responses; local Garage compose moved from a task to existing infra. KEEP: route and casing decisions, upload → DB → delete-old ordering, narrow-interface rule.

## Review Triage Log

## Design Notes

- **Two sizes, one decode:** the source is decoded once, cropped, resampled to 256, and the 128 is resampled from the 256 (CatmullRom both times). Total work is dominated by the single decode and two WebP encodes; expect roughly 10 to 20 KB for 256 and 3 to 6 KB for 128 at quality 80.
- **Why 128 and 256:** list and seat discs are at most 64 CSS px, so 128 covers 2x displays exactly; the 96 px hero gets 256 (2.67x). No third size: the marginal sharpness on 3x phones does not justify a third object per user.
- **URL derivation:** a package-level base in `user`, set once from `main.go`, avoids threading a config string through eight handler constructors; `AvatarURL` returns nil when base or key is empty, so tests and disabled dev mode serialise `null`.
- **Upload order:** process → `PutObject` 256 → `PutObject` 128 → `UpdateAvatarKey` (returns the previous prefix from inside the transaction) → delete the old pair. Nothing old is removed before the new pair is durable; a failure after the first put deletes what was just written so the bucket collects no orphans.
- **Body cap:** `MaxBytesReader` at 2 MiB + 16 KiB so a file of exactly 2 MiB, which the client check allows, is not rejected for multipart overhead; the file part itself is capped at 2 MiB.
- **Cheap rejection first:** size, sniffed type and `DecodeConfig` bounds are all checked before the semaphore is taken, so bad uploads never occupy a decode slot.
- **EXIF:** orientation comes from the JPEG APP1 segment (tiny in-house reader or a pure-Go EXIF package, implementer's choice).

## Verification

**Commands:**
- `cd server && CGO_ENABLED=0 go build ./cmd/api && go vet ./...` -- expected: clean.
- `cd server && go test ./...` -- expected: pass (repository tests need `BELJOT_DB_URL`, otherwise they skip).
- `cd server && UPDATE_GOLDENS=1 go test ./internal/ws/... && git diff --stat internal/ws/testdata` -- expected: only `avatarUrl` additions.
- `make lint` -- expected: golangci-lint, tsc, eslint and prettier clean.
- `cd client && npx vitest run` -- expected: pass, including `wsEvents.contract.test.ts` and `i18n.parity.test.ts`.
- `docker compose up -d && docker compose exec garage /garage bucket website --allow beljot-public && make dev`, upload on `/profile`, then `curl -sI -H "Host: beljot-public.web.garage.localhost" http://localhost:3902/avatars/<uuid>/128.webp` -- expected: 200, `content-type: image/webp`, `cache-control: public, max-age=31536000, immutable`; the same for `256.webp`.

**Manual checks (if no CLI):**
- Two browsers in one room: the uploading player's new picture appears in the other browser's roster and at the match table seat without a refresh.
- DevTools network tab on the leaderboard: avatar requests are 128 px WebP, a few KB each, and a second visit serves them from cache.
