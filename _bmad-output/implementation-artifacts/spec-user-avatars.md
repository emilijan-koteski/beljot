---
title: 'User avatars on object storage'
type: 'feature'
created: '2026-09-28'
status: 'in-review'
route: 'dispatch'
baseline_commit: '120059402735f5afc3be98b4e37e1c28c7d20da8'
review_loop_iteration: 2
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
- Decode memory, predicted from the header before any pixel is decoded (format, progressive scan, component count, chroma subsampling, PNG bit depth and interlace, plus the resampler buffer) and calibrated against measured peaks: at most 150 MiB → else 413 `AVATAR_DIMENSIONS_TOO_LARGE`. Baseline JPEG, 8-bit PNG and WebP keep the full 16 MP; progressive, CMYK and 16-bit sources get a smaller effective cap.
- Admission: at most 1 upload per user and 3 per process in flight, checked before the body is read → else 503 `AVATAR_BUSY` at once; the upload body has a 30 s read deadline.
- Processing: at most 1 image decode in flight per process (semaphore); a request that waits more than 10 s for the slot → 503. Each S3 call runs under a 15 s timeout.
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
| Heavy encoding | 4096×3906 progressive 4:4:4 or CMYK JPEG, or a 16 MP 16-bit PNG, predicted over 150 MiB | 413 `AVATAR_DIMENSIONS_TOO_LARGE`, before full decode; a 16 MP baseline 4:2:0 JPEG is still accepted | |
| Too many in flight | a second concurrent upload from the same user, or a 4th process-wide | 503 `AVATAR_BUSY` before the body is read; no budget spent | |
| Portrait phone JPEG | EXIF orientation 6 | both derivatives upright | |
| Non-square | 1000×600 PNG | centre 600×600 crop, then 256 and 128 | |
| Rate limit | 11th upload within 60 min | 429 `AVATAR_UPLOAD_RATE_LIMITED` | |
| Busy | 1 decode in flight and the slot not free within 10 s | 503 `AVATAR_BUSY` | |
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
- `<Avatar name=…>` call sites: `features/friends/FriendList.tsx`, `features/friends/FriendRequests.tsx`, `features/lobby/components/PlayerSearch.tsx`, `features/profile/components/IdentityHero.tsx`, `Rivalries.tsx`, `PartnerSpotlight.tsx`, `features/room/components/SeatTile.tsx`, `features/room/RoomPage.tsx` (roster), `features/room/components/InviteFriendsDialog.tsx`, `features/match/components/TrumpReveal.tsx`, `shared/components/matchStats/SeatChip.tsx`, `shared/components/matchStats/MatchPlayerActions.tsx`, `features/lobby/components/MatchmakingDiagram.tsx` (self disc l.169, orbit occupants l.204), `features/room/OwnerConfirmDialogs.tsx` (kick/transfer targets l.295, 515, 570).
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
- [x] `server/migrations/000030_add_avatar_key_to_users.up.sql` / `.down.sql` -- `ALTER TABLE users ADD COLUMN avatar_key text NULL;` / `ALTER TABLE users DROP COLUMN avatar_key;`.
- [x] `server/internal/config/config.go` -- `S3Endpoint`, `S3Region`, `S3AccessKey`, `S3SecretKey`, `S3PublicBucket`, `PublicAssetsURL` (trailing slash trimmed); `AvatarStorageConfigured()`; non-development fail-fast naming the missing variable.
- [x] `server/internal/apperr/errors.go` -- the sentinels named in the matrix (400 ×2, 413 ×2, 415, 429, 503 ×2).
- [x] `server/internal/user/model.go`, `gorm_repo.go`, new `avatar_url.go`, `user_test.go` -- `AvatarKey *string` (`column:avatar_key`, `json:"-"`); `UpdateAvatarKey(id uint, key *string) (previous *string, err error)` in one transaction (`SELECT … FOR UPDATE`, then update); `SetPublicAssetsURL(base)` + `AvatarURL(key *string, size int) *string` returning `<base>/<key>/<size>.webp`, nil when base or key is empty; repository test.
- [x] `server/internal/avatar/` (new: `store.go`, `image.go`, `handler.go`, `image_test.go`, `handler_test.go`) -- `Store` interface (`Put(ctx, key, contentType, cacheControl string, body []byte) error`, `Delete(ctx, keys ...string) error`) with the S3 implementation (static credentials, region, `BaseEndpoint`, `UsePathStyle: true`, 15 s per call); `Process(r io.Reader) (large, small []byte, err error)` = sniff → `DecodeConfig` limits → decode → EXIF orientation → centre-crop → `draw.CatmullRom` to 256 → `draw.CatmullRom` 256→128 → encode both; handler with `http.MaxBytesReader`, field `avatar`, semaphore of 1 with a 10 s wait cap, per-user window, upload both → DB → delete old pair, and its own narrow users interface; tests cover every matrix row with a fake store and generated JPEG/PNG/WebP fixtures.
- [x] `server/internal/avatar/image.go`, `handler.go`, tests, `server/Dockerfile`, `docs/DEPLOYMENT.md` (review iteration 2) -- `Inspect` predicts peak decode memory from the header (JPEG SOF type, components and sampling factors; PNG bit depth, colour type and interlace; WebP lossy/lossless) plus the 256 × crop-side resampler buffer, calibrated so each fixture's measured peak Go memory stays at or below its prediction, and rejects above 150 MiB with 413 before the slot; admission gate (1 per user, 3 per process, before `readAvatarPart`, 503 `AVATAR_BUSY`, no budget) and a 30 s read deadline on the body via `http.ResponseController`; `ENV GOMEMLIMIT=200MiB` in the runtime image; memory paragraph rewritten with the measured table.
- [x] `server/cmd/api/main.go` -- build the S3 client only when `AvatarStorageConfigured()`, else inject nil so the handler answers 503; call `user.SetPublicAssetsURL`; register both routes beside the username route.
- [x] Server DTO sweep -- `AvatarURL *string \`json:"avatarUrl"\`` (128) on `RegisterResponseData`, `ProfileResponse`, `PublicProfileResponse`, `PlayerSearchResult`, `MatchPlayer`, `PartnerStat`, `RivalStat`, `FriendDTO`, `PendingRequestDTO`, `LeaderboardEntry` + `LeaderboardRowView`, `Room.OwnerAvatarURL`, `RoomPlayer.AvatarURL` (JOINs scan `users.avatar_key`), `FriendSummary` + `InvitableFriendDTO`, `PlayerSeatInfo`, `game.PlayerState` (stamped in `StartMatch`), and every hand-built map in the Code Map; `AvatarLargeURL \`json:"avatarLargeUrl"\`` (256) on `ProfileResponse` and `PublicProfileResponse` only; regenerate goldens.
- [x] `client/src/shared/components/ui/avatar.tsx` + `avatar.test.tsx` -- `avatarUrl?: string | null`, `eager?: boolean`; render `<img alt="" width={size} height={size} decoding="async" loading={eager ? "eager" : "lazy"} className="size-full rounded-full object-cover">` inside the disc, `onError` state falls back to the initial; ring, halo and `icon` unchanged.
- [x] Client types -- `avatarUrl: string | null` on every type in the Code Map, plus `avatarLargeUrl: string | null` on `ProfileResponse` and `PublicProfileResponse`; `PlayerStateSchema` gains `avatarUrl: z.string().nullable()`; both `User` builders copy it.
- [x] Client call-site sweep -- pass `avatarUrl` at every `<Avatar>` site in the Code Map (fourteen files) (`IdentityHero` passes `avatarLargeUrl` with `eager`); convert the five hand-rolled discs (`LeaderboardRow` gets an `avatarUrl` prop, the self row reading `authStore.user`); adjust `PlayerSeat.test.tsx` if the markup changes.
- [x] `client/src/shared/api/profile.ts` -- `uploadAvatar(userId, file, onProgress)` (`FormData`, `Content-Type: multipart/form-data` override, 60 s timeout, axios `onUploadProgress`) returning both URLs, and `removeAvatar(userId)`.
- [x] `client/src/shared/hooks/mutations/useProfile.ts` -- `useUploadAvatarMutation`, `useRemoveAvatarMutation`: patch `profile.detail` (both URLs), `setUser({ ...user, avatarUrl })`, invalidate `publicProfile.detail`.
- [x] `client/src/features/profile/components/AvatarDialog.tsx` + `.test.tsx`, `IdentityHero.tsx` -- edit button over the hero disc only when `userId` is passed; dialog with `<input type="file" accept="image/jpeg,image/png,image/webp">`, `file.size > 2_097_152` check with an inline message, `URL.createObjectURL` preview (revoked on close), progress bar, Upload, Remove (only when an avatar exists), not dismissible while pending; `FetchError.status` 400/413/415/429/503 → `profile.avatar.errors.*`; `toast.success` on completion.
- [x] `client/src/shared/i18n/en.json`, `mk.json`, `hr.json`, `sr.json` -- `profile.avatar.*` (title, pick, upload, remove, uploading, success, errors.tooLarge, errors.tooSmall, errors.unsupported, errors.rateLimited, errors.busy, errors.unavailable).
- [x] `docs/DEPLOYMENT.md` -- six rows in the backend table (Required yes; Secret yes for key and secret) and an "Object storage" paragraph (bucket, `avatars/<uuid>/{128,256}.webp` layout, public by design, immutable cache headers).
- [x] PR description -- the report in the brief's order: migration number and SQL; endpoints, shapes and limits; encoder and resulting dependency list; confirmation that the production variables suffice; local-dev steps; assumptions made. (Drafted against the diff; becomes the PR body when the PR is opened.)

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

- **Season cannot import `user`** (user already imports season), so `season.Service` gets `SetAvatarURLResolver(func(*string) *string)`, wired to `user.SmallAvatarURL` in `main.go`; `LeaderboardEntry` scans `avatar_key`, `LeaderboardRowView` carries the URL. Unwired (tests) serialises `null`. Match needs no derivation: `PlayerSeatInfo.AvatarURL` is copied onto `PlayerState` in `StartMatch`.
- **Key naming shared:** `user.AvatarObjectKey(prefix, size)` is used by both the writer (avatar package) and `user.AvatarURL`; sizes are `user.AvatarSizeLarge/Small`.
- **Room payload keys:** `Room.ownerAvatarUrl` (next to `ownerUsername`) on the struct, `roomLifecyclePayload` and the lobby-disconnect `room_updated`; `system:room_owner_changed` gains `newOwnerAvatarUrl` (next to `newOwnerUsername`); `player_joined` / `seat_updated` gain `avatarUrl`. Transfer-ownership now hydrates the owner fields once before its two broadcasts (FindPlayerRoom reads no user columns). Leave-seat takes the mover's avatar from the post-write roster for the same reason.
- **Encoder:** `gen2brain/webp` v0.6.4 is now libwebp transpiled to Go with wasm2go (no wazero), plus an optional purego path to a system libwebp. That purego path turns a `CGO_ENABLED=0` Linux build into a *dynamically linked* binary, so the Dockerfile, `make build/dev/test-server` build with `-tags nodynamic` (static binary, pure-Go encoder everywhere). Verified in `golang:1.26-alpine`: tagged binary is static, avatar tests pass.
- **Decode slots vs memory:** measured on the production image, one 16 MP PNG upload peaks at ~110 MiB and four concurrent at ~415 MiB, above the documented 256 MiB backend limit. Resolved by the owner's amendment (see Spec Change Log): `decodeSlots = 1`, so the worst single decode (16-bit 16 MP PNG, roughly 170 MiB container peak) stays inside 256 MiB; `docs/DEPLOYMENT.md` says so. The upload budget is now spent only after a successful render, so a 503 BUSY costs nothing.
- **Rate limit accounting:** budget is peeked before the body is read (fast 429) and spent only once the file passes the cheap checks, so rejected files cost no budget; DELETE is never limited.
- **Client errors:** size/type pre-checks answer inline; server failures (incl. the dev 503) are `toast.error`, the dialog stays open. Extra i18n keys beyond the list: `edit`, `hint`, `previewAlt`, `progressLabel`, `removing`, `cancel`, `removed`, `errors.generic`.
- **Shared fallback:** `AvatarImage` (exported from `avatar.tsx`) is the img-with-initial-fallback used by `Avatar` and the five hand-rolled discs.
- `go.mod` directive normalised to `go 1.26.0` by the toolchain (`golang.org/x/image` v0.46.0 requires it).

## Spec Change Log

- 2026-09-28 (owner request, pre-implementation): limits made explicit and stricter (min 128 px, max 4096 px / 16 MP, decode semaphore, per-call S3 timeout); one 256 image replaced by a 128 + 256 pair under a shared prefix, `avatarLargeUrl` added to the two profile responses; local Garage compose moved from a task to existing infra. KEEP: route and casing decisions, upload → DB → delete-old ordering, narrow-interface rule.
- 2026-09-29 (owner decision after review iteration 1, triage row 1): decode concurrency lowered from 4 to 1 in the frozen limits and the Busy matrix row. Trigger: measured ~110 MiB per 16 MP PNG upload (a 16-bit PNG decodes to 128 MiB plus a 32 MiB resampler buffer), so three concurrent max-size uploads exceed the 256 MiB backend limit and one user could OOM-kill every live match. Known-bad state avoided: any configuration where concurrent decodes can sum past the container limit. Also amended (triage row 2): the Code Map's `<Avatar>` inventory gains `MatchmakingDiagram.tsx` and `OwnerConfirmDialogs.tsx`. Owner chose to restore the reviewed implementation and fix on top rather than re-derive. KEEP: everything in Implementation Notes, the Inspect/Render split (cheap rejection before the slot), `-tags nodynamic`, the season avatar resolver, all passing tests.
- 2026-09-29 (owner decisions after review iteration 2, triage rows 31 and 32): added a predicted decode-memory cap (150 MiB, calibrated against measured peaks, 413) and an admission cap (1 upload per user, 3 per process, before the body is read, 503; 30 s body read deadline) to the frozen limits, with two new matrix rows. Trigger: measured peak Go memory for accepted 16 MP sources reached 286 MiB (progressive 4:4:4 JPEG, 282 KB file) and 431 MiB (progressive CMYK, 251 KB file) with one decode slot, and unbounded parallel uploads could each buffer ~2.5 MiB. Known-bad states avoided: any accepted upload whose single decode can exceed the container limit; unbounded buffered bodies. Also: `GOMEMLIMIT` in the backend image. Owner again chose to fix on top. KEEP: one decode slot, the 16 MP / 4096 px limits for ordinary photos, row 33's reserve-then-refund budget rule.

## Review Triage Log

Review iteration 1 (2026-09-29). Layers: blind-hunter (B), edge-case-hunter (E), verification-gap (V). Grouped entries share one root cause; routes: intent_gap > bad_spec > patch > defer; `reject` rows are dismissed on the stated evidence.

| # | Source | Location | Finding | Verdict | Evidence | Route |
|---|---|---|---|---|---|---|
| 1 | B8 | `avatar/handler.go:42`, `docs/DEPLOYMENT.md:92,154` | Concurrent max-size decodes exceed the backend's 256 MiB limit | high | Implementer measured ~110 MiB per 16 MP PNG upload, ~415 MiB for four; three in flight already exceed 256 MiB, and a 16-bit PNG decodes to 128 MiB before the 32 MiB resampler buffer. A single user can send 3 to 4 parallel requests (the per-user limit allows 10) with solid-colour 16 MP PNGs of a few KB, OOM-killing the backend and every live match. The frozen limits (4 slots, 16 MP, matrix "4 decodes in flight") plus "never change Dokploy" leave no fix inside the intent. | intent_gap |
| 2 | B4, V7, E10 | `lobby/components/MatchmakingDiagram.tsx:169,204`, `room/OwnerConfirmDialogs.tsx:295,515,570` | Five `<Avatar>` sites still pass no `avatarUrl` | low | Verified: the Quick Play orbit (self disc and occupants, `RoomPlayer.avatarUrl` in hand) and the kick/transfer confirmations render initials while the roster behind them shows the picture; met in everyday Quick Play. The Code Map's "twelve sites" inventory missed them; the intent does not exclude them. The ReconnectOverlay and CreateRoomModal parts of V7/E10 are rejected: the intent's Never list names both. | bad_spec |
| 3 | B1, E1, V-other | `avatar/handler.go:119-128` | Hourly budget is spent before render, so 503 busy, undecodable pixels and 500s cost uploads | low | `limiter.take` runs before `h.render`, `putPair` and `UpdateAvatarKey`; ten busy answers lock a user out without a stored picture. Fix is moving `take` after a successful render. | patch |
| 4 | B2, E3 | `game/state.go:28-30`, `avatar/handler.go:144` | Replaced pair is deleted at once, so stale URLs in live match state and caches 404 | low | Design part rejected: the frozen intent mandates delete-after-commit and accepts "at worst an initial shows briefly". The comment "a picture changed mid-match shows from the next match on" is wrong (uncached discs fall back to the initial mid-match); comment fix. | patch |
| 5 | B5d | `client/src/shared/types/wsEvents.ts` (`RoomUpdatedPayload.ownerAvatarUrl`) | Doc comment copy-pasted from `RoomCreatedPayload` mentions the Quick Play room_created map | low | Verified at the cited comment; direct correction. | patch |
| 6 | B6, V3 | `room/gorm_repo.go:153,212,304,333`, `season/gorm_repo.go:311,345` | No DB-backed test reads `users.avatar_key` back through the room JOINs, owner hydration or leaderboard selects | medium | Pre-verified gap: every avatar assertion on these reads goes through `mockRoomRepo` / season `mockRepo`; DB suites (`getRoomTestDB`, `season/gorm_repo_test.go`) exist and never assert the key. Dropping the column from a select turns every roster/seat/leaderboard URL null with all tests green. | patch |
| 7 | B7 | `avatar/handler.go:96` | Comment claims "the bucket collects no orphans" | low | Best-effort delete failures and a crash between put and key update leak objects; reword the comment. Reconciliation tooling rejected: the frozen intent chose best-effort delete with logging. | patch |
| 8 | B11 | `client/src/shared/i18n/{en,mk,hr,sr}.json` `profile.avatar.errors.tooLarge` | Copy omits the 16 MP cap, so a 4096x4000 image is told a limit it meets | low | `Inspect` rejects >16,000,000 px with sides <=4096; message names only 2 MB and 4096 px. Text edit in four locales. | patch |
| 9 | B12 | `features/profile/components/AvatarDialog.tsx:240` | Inline error live region mounts together with its text | low | `{error && <p aria-live>}` is often not announced; keep the region mounted, render text conditionally (repo pattern: `PlayerSeat.tsx:487`). Client-side dimension pre-check rejected: the intent has the client mirror only size and type. | patch |
| 10 | B14, E4 | `avatar/handler.go:330-348` | Limiter comment says the map only holds live users; stale entries persist until that user returns | low | Verified; memory is negligible (<=10 timestamps per past uploader, reset per deploy), so the sweep is rejected; comment fix only. | patch |
| 11 | B15 | `room/avatar_payload_test.go` `TestJoinRoom_PlayerJoinedCarriesAvatarKey` | Test name says key, asserts URL; only the null case is covered | low | Rename and add the non-null case (JoinRoom sources `FindPlayersByRoomID`, which JOINs users, so no live bug). The other untested paths B15 lists (prefix guard, small leading field, Remove failure) are rejected: no demonstrated defect, new tests beyond a correction. | patch |
| 12 | V1 | `client/src/shared/api/profile.ts:191-206` | No test pins the upload request shape (field `avatar`, multipart header, 60 s timeout, progress, DELETE path) | medium | Pre-verified: the only caller test mocks the module; dropping the header override makes axios JSON-serialise FormData and every upload answers `AVATAR_MISSING` with both suites green. | patch |
| 13 | V2 | `hooks/mutations/useAuth.ts:61`, `api/axiosClient.ts:171` | No test that login/refresh copy `avatarUrl` into the auth store | medium | Pre-verified: no client test imports the builders; deleting either copy line blanks the nav pill picture on reload or login with all tests green. | patch |
| 14 | V4 | `room/handler.go:2468` | No test that auto-started (Quick Play) matches carry seat avatars | medium | Pre-verified: the only `lastPlayers[].AvatarURL` assertion uses the manual-start path; quick-fill tests check `UserID`/`IsBot` only. | patch |
| 15 | V5 | `cmd/api/main.go:646` `inviteFriendDirectory.ListFriends` | Adapter's avatar resolution untested | low | Pre-verified: the room test stubs the directory. Adapter unit test in `cmd/api`. The `SetPublicAssetsURL` / `SetAvatarURLResolver` wiring part is rejected: needs a startup harness, more than a correction, and E2E exercised it. | patch |
| 16 | V6 | `ProfilePage.tsx:124`, `PublicPlayerProfilePage.tsx:135`, `LeaderboardPage.tsx:240`, `SeatTile.tsx:161` | Page-level call sites have no non-null avatar assertion | medium | Pre-verified: page tests only gained `avatarUrl: null` fixtures; passing `avatarUrl` (128) to the hero instead of `avatarLargeUrl` passes every test. Representative assertions only; the remaining list rows are rejected as low, one-line-pattern duplicates. | patch |
| 17 | V-other | `room/handler.go:2951,2967` | `newOwnerUsername` is always empty in production | low | Pre-existing: `target` comes from `FindPlayerRoom` (`gorm_repo.go:168`), which never JOINs users; no client reads the field. | defer |
| 18 | B3 | `hooks/mutations/useProfile.ts:56` | Uploader's own leaderboard/history/lobby caches keep the old URL | low | Queries go stale after 30 s (`queryClient.ts:6`) and the frozen intent accepts "at worst an initial shows briefly"; uploads are rare and the fix adds invalidations beyond the spec's task. | reject |
| 19 | B5a | `features/lobby/useRoomUpdates.ts:138` | Lobby seat_updated ignores `avatarUrl` | low | The entry keeps the URL from the list/join payload; only a mid-lobby avatar change is missed, and the fix adds a branch. | reject |
| 20 | B5b | `wsEvents.ts` `newOwnerAvatarUrl` | Sent but unread by the client | false | Nothing renders the owner from room_owner_changed; the field follows the spec's hand-built-map rule and harms nothing. | reject |
| 21 | B5c | `room/handler.go:3973` | Quick Play room_created lacks the owner avatar | false | That map carries no `ownerUsername` either, so the card's host disc shows "?" regardless; the spec's rule covers maps that carry a roster username. | reject |
| 22 | B9 | spec task list, `docs/DEPLOYMENT.md` | PR description not in the diff; docs lack prod values, Garage in the Flow diagram, a pre-merge variable warning | low | PR body is drafted outside the repo by design (false). The six variables are already provisioned per the intent (false). Prod values are not in the repo to copy. The Flow diagram shows the deploy path, not runtime reads. | reject |
| 23 | B10, E8 | `config/config.go:122-127` | URL variables are not validated | low | Set once and already provisioned; the startup log prints both values; the fix adds parse/scheme guards. | reject |
| 24 | B13 | `avatar/image.go:Render` | ICC profiles dropped; non-JPEG EXIF orientation ignored | low | Intent specifies JPEG EXIF only; the P3 shift is negligible at 24 to 96 px and needs colour management. | reject |
| 25 | E2 | `avatar/handler.go:125-146` | Server worst case (10 s wait + four 15 s S3 calls) can exceed the client's 60 s timeout | low | Needs a degraded Garage on several consecutive calls; the fix (async delete plus a time cap) adds complexity. | reject |
| 26 | E5 | `avatar/image.go:84` | APNG accepted and flattened to its default frame | low | Benign outcome (a still avatar); intent names only animated WebP; the fix adds a chunk walk. | reject |
| 27 | E6 | `AvatarDialog.tsx:125-135` | Empty-MIME undecodable pick shows a broken preview and allows Upload | low | The picker's accept filter makes it rare; the server answers 415 with the right toast; the fix adds a handler. | reject |
| 28 | E7 | `AvatarDialog.tsx:117` | Zero-byte file gets the generic toast | low | Rare; the fix adds a branch. | reject |
| 29 | E9 | `ui/avatar.tsx:30-44` | One transient load failure pins the initial until remount | low | Rare; the fix adds retry logic. | reject |
| 30 | E11 | `avatar/image.go:130-138` | Crop can sit one source pixel off-centre after rotation with an odd margin | low | Invisible after downscaling to 256/128. | reject |

Review iteration 2 (2026-09-29, after the owner's decode-concurrency amendment and the row 3-16 patches). Same layers and routes.

| # | Source | Location | Finding | Verdict | Evidence | Route |
|---|---|---|---|---|---|---|
| 31 | B, E | `avatar/image.go:33`, `docs/DEPLOYMENT.md:92`, `server/Dockerfile` | A single decode of an accepted 16 MP JPEG exceeds the 256 MiB limit even with one slot; no `GOMEMLIMIT` | high | Measured with the real `Inspect`+`Render` (peak Go runtime memory): baseline 4:2:0 JPEG 74 MiB, 8-bit PNG 110, baseline CMYK 174, 16-bit PNG 182, progressive 4:4:4 JPEG 286 (282 KB file), progressive CMYK 431 (251 KB file). Go keeps every DCT coefficient for progressive scans (`image/jpeg/scan.go:156`, 256 B per block per component). The frozen 16 MP acceptance admits these; row 1's decision assumed a ~170 MiB worst case. | intent_gap |
| 32 | B, E | `avatar/handler.go:113-117`, `cmd/api/main.go:506` | Nothing bounds uploads in flight: each buffers up to ~2.5 MiB while queuing 10 s for the slot, and the server has no read deadline | high | `allow()` spends nothing, so one user can open dozens of parallel uploads; ~50 queued bodies (~150 MiB) plus one ordinary decode exceed 256 MiB; slow-drip bodies stay resident (`e.Start` sets no read timeouts; this is the first 2 MiB body route). The spec bounds decodes, not admission, and does not settle the cap. | intent_gap |
| 33 | B, E | `avatar/handler.go:113,131` | Budget accounting: charging only after a successful render (row 3's patch) makes failed decodes free, and failures after `take` still burn budget | medium | A valid header with pixels truncated at the end decodes almost fully, holds the single slot, returns 415 and costs nothing, so one account can keep everyone else at 503; conversely a store or DB 500 after `take` spends an upload. Fix: reserve before the slot, refund only on server-side outcomes (busy, 5xx), never on a 415. | patch |
| 34 | B, V | `avatar/store.go` | `S3Store` (path-style, checksum setting, headers, per-key delete with joined errors) is never exercised by a test | medium | Pre-verified: only `main.go` references `NewS3Store`; handler tests use `fakeStore`. Dropping `UsePathStyle` or `CacheControl` stays green. `httptest` S3 stub. | patch |
| 35 | V | `room/handler.go:4164-4187` | Quick Play join broadcasts (`player_joined`, `seat_updated` from `broadcastQuickPlayerSeated`) carry `avatarUrl` untested | medium | Pre-verified: the only source of a later joiner's picture in the matchmaking orbit; no room test decodes a Quick Play broadcast. | patch |
| 36 | V | `avatar/handler.go:113` | The pre-body `allow()` check is not pinned | low | Pre-verified: every 429 test runs with the slot free, so deleting the peek stays green. Test: exhausted budget plus a held slot answers 429, not 503. | patch |
| 37 | V | `avatar/handler.go:63-65` | Upload response keys are checked by neither side | medium | Pre-verified: the server test decodes through the same struct; the client stubs the response. A renamed tag leaves the hero and nav pill on stale URLs. Exact-key assertion like `TestGetLeaderboard_WirePayloadKeysAreExact`. | patch |
| 38 | B | `avatar/image_test.go` | Pipeline tested only with opaque 8-bit RGB inputs | low | The alpha path (premultiplied scale, then NRGBA for the encoder) is the one with real logic and has no case. Add a transparent PNG (alpha survives) and a grayscale JPEG. The heavy-encoding fixtures come with row 31. | patch |
| 39 | B | `docs/DEPLOYMENT.md` (Object storage) | No way to take down an abusive picture shown to strangers | medium | Only the owner can DELETE. A runbook entry (NULL the key, delete both objects, note that immutable caching keeps already-fetched copies) is a doc addition; an operator endpoint is rejected (new public surface, not in the intent). | patch |
| 40 | B | spec Implementation Notes ("Rate limit accounting") | Note contradicts the code after row 3's patch | low | Fix edits this build's spec. | reject |
| 41 | B | `docs/DEPLOYMENT.md` (Upload limits) | Says "JPEG EXIF orientation is applied first"; `Render` applies it after the crop and resample | low | Output is equivalent (row 30); reword to say portraits are stored upright. | patch |
| 42 | B, V | `avatar/handler.go` `render` (`ctx.Done()`) | A client that hangs up while queued gets a 500 and an error-level log | low | The client that left never sees it; the budget part is covered by row 33; quieting the log adds a branch. | reject |
| 43 | B | `cmd/api/main.go:117-130` | Storage misconfiguration only surfaces on the first upload | low | Variables are provisioned per the intent; a boot-time HeadBucket adds a network dependency at startup (compare row 23). | reject |
| 44 | B | `AvatarDialog.tsx`, `api/profile.ts:198` | Progress bar reads 100% while the server decodes and stores | low | Normal server work is under a second; a busy wait ends in a toast. | reject |
| 45 | B | `avatar/handler.go`, `apperr/errors.go` | 429/503 carry no `Retry-After` | low | New headers and client copy, not in the intent. | reject |
| 46 | E | `avatar/handler.go:144-153` | Two overlapping uploads by one user: the earlier tab adopts URLs the later upload deleted | low | Needs concurrent uploads from one account; row 32's per-user cap would prevent it; the fix adds 409 logic. | reject |
| 47 | E | `avatar/handler.go:172-177`, `api/profile.ts:205` | DELETE's two object deletes can outlast the client's 15 s timeout | low | Same root as row 25 (synchronous best-effort deletes need a degraded Garage). | reject |
| 48 | E | `AvatarDialog.tsx:125` | A JPEG reported as `image/jpg` or `image/pjpeg` is rejected inline | low | Browsers report `image/jpeg` for JPEG files; the accept list is fixed by the task. | reject |
| 49 | E | `avatar/handler.go:125-146` | Server worst case can exceed the client's 60 s timeout | low | carried: row 25; code unchanged there. | reject |
| 50 | E | `config/config.go:122-127` | URL variables not validated | low | carried: row 23; code unchanged. | reject |
| 51 | E | `avatar/image.go:130-138` | One-pixel crop shift after rotation | low | carried: row 30; code unchanged. | reject |
| 52 | E | `ReconnectOverlay.tsx:503`, `CreateRoomModal.tsx:960` | Discs still initial-only | low | carried: row 2's rejected part; the intent's Never list names both. | reject |

Review iteration 3 (2026-09-29): the three layers reported, then the owner asked to commit, merge to master and push before triage. These findings are OPEN (not yet verified or routed); resume here. None reopens the OOM risk: the worst, the RGB-JPEG underpricing, was measured by a reviewer at ~115 MiB peak heap, inside the 256 MiB limit.

- `decodemem.go` `jpegDecodeBytes`: 3-component JPEGs that image/jpeg decodes as RGB (Adobe APP14 transform 0 or R/G/B ids, no JFIF) get an extra 4 B/px `*image.RGBA` the model omits; `jpegHeaderOnly` fixtures carry that marker, so the "light" JPEG tests and one calibration row are really RGB files.
- `handler.go` `serverFault`: `context.Canceled` from `putPair` after the render is refunded, so a client that hangs up after sending the body gets free decodes.
- `handler.go` admission: drip-fed or aborted bodies hold a place at no cost (3 accounts can keep every upload at 503); no early `Content-Length > maxBodyBytes` rejection before admission.
- `handler.go` body read deadline: 30 s vs the client's 60 s, surfaces as 400 BAD_REQUEST, never runs under test (recorder has no `SetReadDeadline`); per-call S3 timeout untested.
- Early 429/503 answered before reading a large body may arrive as a connection reset through the proxy (net/http closes with unread body).
- Tests: the pre-body guarantees (budget peek, admission) are not proven by a body that fails if read; WebP ALPH branch and 3 of the 10 calibration rows untested; calibration compares against literals, no real-decode allocation check.
- Client copy: 413 `AVATAR_DIMENSIONS_TOO_LARGE` for a heavy encoding (e.g. 16 MP progressive 4:2:0 JPEG, interlaced PNG) shows `tooLarge`, which names limits the file meets; frozen wording "Baseline JPEG, 8-bit PNG and WebP keep the full 16 MP" overstates it (interlaced PNG, some lossless/alpha WebP are refused) and the docs table omits those rows.
- `readJPEGFrame` / `jpegOrientation` stop at stray bytes between segments that image/jpeg skips (worst-case pricing, EXIF ignored).
- Takedown runbook: a cleared avatar can be re-uploaded at once or overwritten by an in-flight upload; the `aws s3 rm` example does not show forcing path-style.
- `TransferOwnership` still sends the empty `newOwnerUsername` (deferred entry) though `postRoom.OwnerUsername` is now hydrated.
- "Stays inside 256 MiB" rests on isolated decode measurements, not a container-level peak under a production-like baseline.
- Carried/rejected shapes seen again: EXIF crop 1 px (row 30), ReconnectOverlay/CreateRoomModal (row 2), own-cache staleness (row 18), lobby seat_updated (row 19), post-commit deletes vs client timeouts (rows 25, 47).

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
