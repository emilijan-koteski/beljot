# Story 11.2: Friend Requests & Friend List

Status: ready-for-dev

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a player,
I want to send friend requests and maintain a friend list,
so that I can easily find and play with people I know.

## Acceptance Criteria

**Epic source:** `_bmad-output/planning-artifacts/epics.md#Story-11.2-Friend-Requests-&-Friend-List` (FR6). The four epic ACs are expanded below into testable form. Cross-story scope calls (Add-Friend button reuses the 11.3 seam; the "Invite to Room" action is a hook completed by 11.5) are recorded in Dev Notes → **Scope & Sequencing Decisions**.

1. **AC1 — Send friend request.** An authenticated `POST /api/v1/friends/request` with body `{ "userId": <uint> }` creates a **pending, directional** friendship row (`user_id` = requester = the authenticated caller, `friend_id` = `userId` = recipient). Returns `201` with `{ "data": { "id", "userId", "friendId", "status": "pending", "createdAt" } }`. **Guards:** self-request (`userId == authUserID`) → `400 SELF_FRIEND_REQUEST`; unknown/soft-deleted target → `404 USER_NOT_FOUND`; an existing pending or accepted relationship in **either direction** → `409` (`FRIEND_REQUEST_EXISTS` if pending, `ALREADY_FRIENDS` if accepted). The requester id is **always** taken from `getUserID(c)`, never from the request body.
2. **AC2 — Recipient notification (best-effort, online-only).** On a successful request, if the recipient is currently connected, the server pushes a `system:friend_request` WS message (`{ requestId, fromUserId, fromUsername }`) via `hub.SendToUser`. Because `SendToUser` is an unqueued no-op for offline users (no offline inbox — same model as chat/honor), this push is **best-effort**; the durable delivery path is the recipient seeing the request in their pending-requests list (AC4) on next load. The endpoint must **not** fail if the recipient is offline.
3. **AC3 — Friendship status for a subject (drives the profile button).** `GET /api/v1/friends/status/:id` returns the relationship between the authenticated viewer and subject `:id`: `{ "data": { "status": "none" | "pending_outgoing" | "pending_incoming" | "friends", "requestId": <uint|null> } }`. `pending_outgoing` = viewer sent it; `pending_incoming` = subject sent it (viewer can accept); `requestId` is the row id when a request/friendship exists. Self (`:id == authUserID`) → `status: "none"` (or `400`; pick one and pin it — the profile button is never shown on your own page).
4. **AC4 — Pending requests list.** `GET /api/v1/friends/requests` returns the viewer's **incoming** pending requests: `{ "data": [ { "id", "fromUserId", "fromUsername", "createdAt" }, ... ] }` (empty → `[]`, never `null`). Each row renders in the UI with the sender's username and **Accept** + **Decline** actions.
5. **AC5 — Accept / decline (recipient-only, atomic).** `POST /api/v1/friends/:id/accept` transitions a `pending` row to `accepted` **only when the caller is the recipient** (`friend_id == authUserID` and `status == 'pending'`), done as a single atomic conditional `UPDATE` (rows-affected check, not read-then-write); a caller who is not the recipient, or a non-pending/unknown row → `404 FRIEND_REQUEST_NOT_FOUND` (do not leak existence/authorization detail). `POST /api/v1/friends/:id/decline` removes the pending row under the same recipient-only guard. After accept, both players appear in each other's friend list (AC6).
6. **AC6 — Friend list with online status.** `GET /api/v1/friends` returns the viewer's accepted friends: `{ "data": [ { "id", "username", "online": <bool> }, ... ] }` (empty → `[]`). `online` is derived server-side from `hub.IsConnected(friendID)`. Friends resolve through **live** users only (a soft-deleted user is omitted). The list is symmetric — a friend appears whether the viewer was the requester or the recipient of the original request.
7. **AC7 — Profile "Add Friend" button (fills the 11.3 seam).** On the public profile `/players/:id`, at the documented insertion point in `PublicPlayerProfilePage.tsx` (under `<IdentityHero>`, lines ~155-159), render a friendship action driven by `GET /friends/status/:id`: **"Add Friend"** when `none` (→ `POST /friends/request`), **"Request sent"** (disabled) when `pending_outgoing`, **"Accept request"** when `pending_incoming` (→ accept), **"Friends ✓"** (or an unfriend affordance) when `friends`. The button is **never** dead/placeholder — every state maps to a real action or a real disabled affordance. Optimistic/invalidating mutation updates the button and the requests/list queries.
8. **AC8 — Friend list & requests UI + "Invite to Room" hook.** A friends surface in the lobby (`data-testid="friend-list"`) shows accepted friends with an online/offline indicator, and a pending-requests surface (`data-testid="friend-requests"`) with Accept/Decline. Each **online** friend row shows an **"Invite to Room"** affordance (`data-testid="friend-invite-room"`); the actual invite delivery (`event:room_invite`, availability computation, room context) is **owned by Story 11.5** — this story renders the affordance and leaves a documented hook, exactly mirroring how 11.3 left the Add-Friend hook for this story. Empty states for both surfaces are localized.
9. **AC9 — i18n & quality gates.** All new user-facing strings are added to **all four** locale files (`en`, `sr`, `mk`, `hr`) with `mk` all-Cyrillic and no em dash (`—`) in `mk`/`sr`/`hr`. `make lint` and `make test` pass (Go: `go vet` + `golangci-lint` + `gofmt`; client: `tsc` build + `vitest` + `eslint` + `prettier`; i18n parity test green).

## Tasks / Subtasks

- [ ] **Task 0: Branch setup — whole-epic-on-one-branch**
  - [ ] **Continue on the current branch `feat/11-3-public-player-profiles`** (do NOT cut a new per-story branch). Per user direction, the remaining Epic 11 stories (11-1, 11-2, 11-4, 11-5) ship as ONE feature/PR on this branch — the same whole-scope-on-one-branch pattern used for 9.7+9.8. The branch already contains Epic 9 (honor, merged `dc14308`) + Story 11.3, so the public-profile Add-Friend seam and `PublicProfileResponse` are present. Note: this overrides the "one story = one branch = one PR" default for this epic only, by explicit instruction.

- [ ] **Task 1: Migration `000019_create_friendships`** (AC: #1, #5, #6)
  - [ ] Create `server/migrations/000019_create_friendships.up.sql` and `.down.sql` (next number after `000018`; every up needs a matching down — project rule). Schema (mirrors the two-user-columns precedent `000014_create_user_identities` + the state-column idiom of `000013_create_refresh_tokens`):
    ```sql
    -- up
    CREATE TABLE friendships (
        id         SERIAL PRIMARY KEY,
        user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,  -- requester
        friend_id  INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,  -- recipient
        status     VARCHAR(20) NOT NULL DEFAULT 'pending',                    -- 'pending' | 'accepted'
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        CONSTRAINT chk_friendships_not_self CHECK (user_id <> friend_id)
    );
    CREATE UNIQUE INDEX idx_friendships_pair ON friendships (user_id, friend_id);
    CREATE INDEX idx_friendships_friend_id ON friendships (friend_id);
    CREATE INDEX idx_friendships_user_id_status ON friendships (user_id, status);
    -- down
    DROP TABLE IF EXISTS friendships;
    ```
  - [ ] The DB unique index only guards **one direction** `(user_id, friend_id)`. The **reverse-duplicate** (B→A while A→B exists) is NOT catchable by this index — enforce it in the repo/handler with a direction-agnostic pair lookup (Task 2). No soft-delete column (hard-delete on decline/unfriend, like `identity`/`refreshtoken`).
  - [ ] Verify the up/down roundtrip against dev DB `:5433` (`docker compose up -d postgres`, `make migrate`), then down, then up — column + CHECK + index inspection at each step.

- [ ] **Task 2: Backend — new `internal/friend` package (model + repo)** (AC: #1, #5, #6)
  - [ ] Create `server/internal/friend/model.go`: `Friendship` struct with GORM/JSON tag bridge (`ID uint gorm:"primaryKey" json:"id"`, `UserID uint gorm:"column:user_id" json:"userId"`, `FriendID uint gorm:"column:friend_id" json:"friendId"`, `Status string gorm:"column:status" json:"status"`, `CreatedAt`/`UpdatedAt time.Time`). Add `const ( FriendStatusPending = "pending"; FriendStatusAccepted = "accepted" )`. GORM auto-pluralizes to `friendships` — no `TableName()` override needed.
  - [ ] Create `server/internal/friend/repository.go` — the interface ONLY (handlers depend on it, never GORM directly):
    ```go
    type Repository interface {
        Create(f *Friendship) error
        FindByID(id uint) (*Friendship, error)                 // (nil,nil) on not-found
        FindByPair(a, b uint) (*Friendship, error)             // direction-agnostic: (a→b) OR (b→a); (nil,nil) if none
        Accept(id, recipientID uint) (int64, error)            // atomic: UPDATE ... WHERE id=? AND friend_id=? AND status='pending'; returns rows affected
        Delete(id, recipientID uint) (int64, error)            // recipient-only decline; returns rows affected
        ListAccepted(userID uint) ([]Friendship, error)        // rows where (user_id=? OR friend_id=?) AND status='accepted'
        ListIncomingPending(userID uint) ([]Friendship, error) // friend_id=? AND status='pending'
        AreFriends(a, b uint) (bool, error)                    // convenience for 11.4/11.5 consumers
    }
    ```
    (Return non-nil empty slices for the list methods — `[]Friendship{}` not `nil`.)
  - [ ] Create `server/internal/friend/gorm_repo.go`: `GormRepository{ db *gorm.DB }` + `NewGormRepository(db)`. `Create` maps the pg unique-violation (`pgconn.PgError` code `"23505"`) → `apperr.ErrFriendRequestExists` exactly like `identity/gorm_repo.go:23-27`. `FindByPair` uses `Where("(user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?)", a, b, b, a)`. `Accept`/`Delete` use conditional `UPDATE`/`DELETE` with a rows-affected return (atomic recipient-only guard — model the atomicity on `user/gorm_repo.go:138-160` `UpdateUsername`). GORM `ErrRecordNotFound` → `(nil, nil)`.
  - [ ] Add friend-domain errors to `server/internal/apperr/errors.go` (new grouped block, following the `INSUFFICIENT_COINS`/`WRONG_ROOM_PASSWORD` 409 precedent): `ErrSelfFriendRequest` (400, `SELF_FRIEND_REQUEST`), `ErrFriendRequestExists` (409, `FRIEND_REQUEST_EXISTS`), `ErrAlreadyFriends` (409, `ALREADY_FRIENDS`), `ErrFriendRequestNotFound` (404, `FRIEND_REQUEST_NOT_FOUND`). Reuse `ErrUserNotFound` (404) for an unknown request target.

- [ ] **Task 3: Backend — friend handler, routes, notification push** (AC: #1, #2, #3, #4, #5, #6)
  - [ ] Create `server/internal/friend/handler.go`: `Handler` holding the friend `Repository`, the `user.UserRepository` (for `FindByID` target-existence + `FindManyByIDs` username resolution — reuse, do NOT add methods to `UserRepository`), and a **narrow** connection/notify interface so the handler stays unit-testable without a real hub:
    ```go
    type Notifier interface {
        IsConnected(userID uint) bool
        SendToUser(userID uint, msg []byte)
    }
    ```
    (`*ws.Hub` satisfies this already — `hub.go:227`/`:178`.)
  - [ ] Handlers (all read caller via `getUserID`-equivalent; requester is always `authUserID`):
    - `SendRequest(c)` — bind `{userId}`; guard self (400) → target `FindByID` nil (404) → `FindByPair` existing pending (409 exists) / accepted (409 already-friends) → `Create` → build & `SendToUser(recipientID, system:friend_request)` **only if `IsConnected`** → `201`.
    - `GetStatus(c)` — parse `:id`; `FindByPair(viewer, subject)`; map to `none`/`pending_outgoing`/`pending_incoming`/`friends` + `requestId`.
    - `ListRequests(c)` — `ListIncomingPending(viewer)` → resolve sender usernames via `userRepo.FindManyByIDs` → DTO list.
    - `Accept(c)` / `Decline(c)` — parse `:id`; call `repo.Accept(id, viewer)` / `repo.Delete(id, viewer)`; rows-affected `0` → `404 FRIEND_REQUEST_NOT_FOUND`.
    - `ListFriends(c)` — `ListAccepted(viewer)` → collapse each row to the OTHER user id → `FindManyByIDs` for usernames → set `online` from `notifier.IsConnected(id)` → DTO list.
  - [ ] Register routes on the authenticated `api` group in `server/cmd/api/main.go`, placed **after** the `hub := ws.NewHub()` line (`~main.go:159`) so `hub` is in scope (co-locate near the room/session route block, ~`main.go:254`). Wire `friendRepo := friend.NewGormRepository(db)` + `friendHandler := friend.NewHandler(friendRepo, userRepo, hub)`:
    ```go
    api.POST("/friends/request",    friendHandler.SendRequest)
    api.GET("/friends",             friendHandler.ListFriends)
    api.GET("/friends/requests",    friendHandler.ListRequests)
    api.GET("/friends/status/:id",  friendHandler.GetStatus)
    api.POST("/friends/:id/accept", friendHandler.Accept)
    api.POST("/friends/:id/decline",friendHandler.Decline)
    ```
    Echo resolves the static `/friends/requests` and `/friends/status/:id` segments ahead of `/friends/:id/accept` — no collision (static > param). Success envelope: `c.JSON(2xx, map[string]interface{}{"data": <dto>})`; errors `return apperr.ErrXxx`.
  - [ ] Add `system:friend_request` to the WS contract: const + `FriendRequestPayload{ RequestID uint json:"requestId"; FromUserID uint json:"fromUserId"; FromUsername string json:"fromUsername" }` in `server/internal/ws/events.go`, and the mirror const + TS interface in `client/src/shared/types/wsEvents.ts`. **`system:` prefix ⇒ ZERO drift-gate touchpoints** (no golden, no Zod schema, no contract-test row — verified: chat/emote `system:*` events have none). The diff is exactly those two files (+ the client dispatch case in Task 7). Both contract files in the SAME commit (project rule).

- [ ] **Task 4: Backend tests** (AC: #1, #3, #4, #5, #6)
  - [ ] DB-backed `server/internal/friend/gorm_repo_test.go` via `getTestDB`-style tx-rollback (DSN dev DB `:5433`, skips if unavailable): create; `FindByPair` matches BOTH directions; unique-violation on exact-duplicate → `ErrFriendRequestExists`; `Accept` atomic (only recipient + only pending flips; wrong caller / already-accepted → 0 rows); `Delete` recipient-only; `ListAccepted` symmetric (returns friend whether viewer was requester or recipient); `ListIncomingPending`; `AreFriends` true/false; `chk_friendships_not_self` rejects self-row.
  - [ ] Handler tests `server/internal/friend/handler_test.go` (mock friend `Repository` + a `notifierSpy` capturing `IsConnected`/`SendToUser` + mock/stub `UserRepository`; real JWT via `auth.GenerateAccessToken`, `testErrorHandler`, `ServeHTTP`): self-request → 400; unknown target → 404; pending-dup → 409; accepted-dup → 409; happy path → 201 **and** asserts `SendToUser` called for an online recipient **and NOT called** when `IsConnected` is false (endpoint still 201); `GetStatus` all four states; `Accept`/`Decline` non-recipient → 404; `ListFriends` sets `online` from the spy; empty lists serialize `[]`.

- [ ] **Task 5: Frontend — API client, types, query & mutation hooks** (AC: #1, #3, #4, #5, #6)
  - [ ] New `client/src/shared/api/friends.ts` (maps 1:1 to the `friend` backend domain): `sendFriendRequest(userId: number)`, `getFriendshipStatus(id: number)`, `listFriendRequests()`, `acceptFriendRequest(id: number)`, `declineFriendRequest(id: number)`, `listFriends()`. Return the unwrapped payload (the axios response interceptor already unwraps `{data}` — never `.data.data`).
  - [ ] Types in `client/src/shared/types/apiTypes.ts` (named exports): `Friend { id: number; username: string; online: boolean }`, `FriendRequest { id: number; fromUserId: number; fromUsername: string; createdAt: string }`, `FriendshipStatus { status: "none" | "pending_outgoing" | "pending_incoming" | "friends"; requestId: number | null }`.
  - [ ] Extend `client/src/shared/api/queryKeys.ts`: `friends: { list: () => ["friends","list"] as const, requests: () => ["friends","requests"] as const, status: (id: number) => ["friends","status",id] as const }`.
  - [ ] Query hooks `client/src/shared/hooks/queries/`: `useFriends()`, `useFriendRequests()`, `useFriendshipStatus(id)` (`enabled: Number.isInteger(id) && id > 0`). Mutation hooks `client/src/shared/hooks/mutations/useFriendMutations.ts`: send/accept/decline, each `invalidateQueries` on `friends.status(target)` + `friends.requests()` + `friends.list()` as appropriate (mirror the existing mutation idiom in `hooks/mutations/useProfile.ts`).

- [ ] **Task 6: Frontend — Add-Friend button on the public profile** (AC: #7)
  - [ ] Create `client/src/features/profile/components/FriendButton.tsx`, mounted at the **Story-11.2 insertion point** in `client/src/features/profile/PublicPlayerProfilePage.tsx` (~lines 155-159, under `<IdentityHero>`). Props: `userId` (the validated subject id already in scope as `validId`). Drives its label/action from `useFriendshipStatus(userId)`: `Add Friend` / `Request sent` (disabled) / `Accept request` / `Friends ✓`. Never renders a dead button. testids: `friend-button-add` / `-pending` / `-accept` / `-friends`. Replace the placeholder comment with the real component.
  - [ ] Do NOT show the button on the viewer's own id (the public page never renders for self in practice, but guard defensively — `status: "none"` self case or hide if `userId === authUser.id`).

- [ ] **Task 7: Frontend — friend list + requests UI + WS notification** (AC: #2, #4, #6, #8)
  - [ ] New feature folder `client/src/features/friends/`: `FriendList.tsx` (`data-testid="friend-list"`; each row = username + online/offline dot + an "Invite to Room" affordance `data-testid="friend-invite-room"` for online friends — **stub: onClick is a documented no-op/handler owned by Story 11.5**; localized empty state) and `FriendRequests.tsx` (`data-testid="friend-requests"`; Accept/Decline buttons wired to the mutations; localized empty state). Mount both in `client/src/features/lobby/LobbyPage.tsx` as a "Friends" panel (sibling to the room grid / `FilterRail`).
  - [ ] Add a `SYSTEM_FRIEND_REQUEST` dispatch case in `client/src/shared/hooks/useWsDispatch.ts` (`dispatchSystemEvent` switch, ~`:558`): validate the payload (`typeof requestId === "number"`, etc. — never JS-truthiness on Go numerics), then `queryClient.invalidateQueries({ queryKey: queryKeys.friends.requests() })` and optionally a subtle `toast`/unseen badge. Add the const to `wsEvents.ts` (done in Task 3).

- [ ] **Task 8: i18n** (AC: #9)
  - [ ] Add a `friends.*` block to all four locales `client/src/shared/i18n/{en,sr,mk,hr}.json`: button labels (`addFriend`, `requestSent`, `acceptRequest`, `friends`), list/requests headings + empty states, Accept/Decline labels, the online/offline label, the "Invite to Room" label, and a notification toast string (`{{username}}` interpolation). `mk` all-Cyrillic; NO em dash in `mk`/`sr`/`hr`. `i18n.parity.test.ts` must stay green (1:1 leaf parity + non-empty).

- [ ] **Task 9: Full validation gates** (AC: #9)
  - [ ] `make lint` (Go + client) and `make test` (`go test ./...` + `npx vitest run`) green. Confirm the DB-backed friend repo test RAN (not skipped) against dev DB `:5433`, migrated to v19; note in Completion Notes if the DB was unavailable and it skipped. Update the File List and Completion Notes.

## Dev Notes

### Scope & Sequencing Decisions (READ FIRST)

- **Greenfield friend backend — new `internal/friend` package.** There is **zero** friend model, table, endpoint, WS event, or client module on this branch today (grep confirms only doc references). Build a self-contained `friend` package (the idiomatic shape — cf. `identity`/`refreshtoken`), NOT an extension of `user`. Rationale: adding query methods to the shared `user.UserRepository` interface forces every mock of it across the tree to be updated (the "mock blast radius" trap 11.3 called out for `MatchRepository`). A separate `friend.Repository` touches zero existing mocks. Where the handler needs user data (target existence, username resolution), reuse the existing `userRepo.FindByID` / `FindManyByIDs` — no `UserRepository` change.
- **The Add-Friend button fills the 11.3 seam.** Story 11.3 deliberately left a documented insertion point in `PublicPlayerProfilePage.tsx` (under `<IdentityHero>`, ~155-159) and a `deferred-work.md` entry (D4/AC6). This story mounts the real `FriendButton` there. It needs friendship-state to render correctly — hence the dedicated `GET /friends/status/:id` endpoint rather than widening `PublicProfileResponse` (keeps 11.3's private-field whitelist + leak tests untouched).
- **"Invite to Room" is a hook completed by Story 11.5 (PO-confirmed 2026-08-14).** The epic 11.2 AC says an online friend row exposes an "Invite to Room" action. The *mechanism* (available-friend computation = online + in-lobby + not-in-room/match, the `event:room_invite` push, the one-time password-bypass grant, room context) is Story 11.5's scope and depends on being inside a `waiting` room. This story renders the affordance for online friends and leaves a clean, documented hook — exactly the pattern 11.3 used for Add-Friend. Do NOT build the invite delivery here.
- **Notification is best-effort, online-only.** `hub.SendToUser` is an unqueued no-op for offline users; there is no offline inbox anywhere in the system (chat, honor, coins all share this). The durable path for a friend request is the recipient's pending-requests list on next load (AC4). The `POST /friends/request` must succeed (201) regardless of recipient connection state.
- **Whole epic on one branch.** Per user direction, 11-1/11-2/11-4/11-5 ship together on `feat/11-3-public-player-profiles` as one PR (see Task 0). `AreFriends` on the friend repo is exposed now because Stories 11.4 (whisper friend-check) and 11.5 (available-friend list) both consume it — this avoids those stories re-touching the friend package interface.

### Backend implementation notes

- **Domain package shape** (from `user`/`identity`/`refreshtoken`): `model.go` (GORM struct, tag bridge), `repository.go` (interface only), `gorm_repo.go` (`GormRepository{db}` + `NewGormRepository`), `handler.go` (`Handler` + `NewHandler`), `gorm_repo_test.go`, `handler_test.go`. Optional `service.go` — not needed here; accept/decline logic is small enough to live in the handler with atomic repo methods.
- **Auth-user extraction:** `getUserID(c echo.Context) (uint, error)` reads `c.Get("userID").(uint)` (see `user/handler.go:242-252`; there is also exported `auth.GetUserID(c)`). The requester/viewer is ALWAYS this value — never a client-supplied id (11.3's D1 subject-vs-viewer lesson: never trust a body-supplied actor id).
- **Response/error envelopes are inlined** (no helper fns). Success: `c.JSON(2xx, map[string]interface{}{"data": items})`. Errors: `return apperr.ErrXxx` — the central `appErrorHandler` (`main.go:335-379`) renders `{ "error": { "code", "message" } }`. Path id via `strconv.ParseUint(c.Param("id"), 10, 64)`, rejecting `0`. Wrap unexpected repo errors with `%w`.
- **pg unique-violation mapping** is the standard idiom (`identity/gorm_repo.go:20-31`, `user/gorm_repo.go:25-38`): `var pgErr *pgconn.PgError; if errors.As(err, &pgErr) && pgErr.Code == "23505" { return apperr.ErrFriendRequestExists }`. GORM record-not-found is swallowed to `(nil, nil)` (`if errors.Is(err, gorm.ErrRecordNotFound)`).
- **Direction-agnostic duplicate check is mandatory** — the DB unique index only covers `(user_id, friend_id)`; a reverse request (B→A while A→B pending) passes the index. `FindByPair` (both orderings) in the handler is what blocks it (→ 409). Do NOT rely on the unique index alone.
- **Atomic accept/decline** — use a single conditional statement with a rows-affected check (`UPDATE friendships SET status='accepted', updated_at=NOW() WHERE id=? AND friend_id=? AND status='pending'`), modeled on the atomic `UpdateUsername` (`user/gorm_repo.go:138-160`). Read-then-write invites a double-accept race. `rows == 0` → `404 FRIEND_REQUEST_NOT_FOUND` (uniform response whether the row is missing, already accepted, or the caller isn't the recipient — don't leak which).
- **Online status source:** `hub.IsConnected(userID) bool` (`ws/hub.go:227-232`); batch snapshot `ConnectedUserIDs()` (`:216-224`). Inject via the narrow `Notifier` interface so tests use a spy. `room.PresenceRegistry` is room-scoped, NOT a global online registry — do not use it here.
- **WS notification:** build `ws.WSMessage{Type: ws.SystemFriendRequest, Payload: <json>}`, `hub.SendToUser(recipientID, bytes)`. Per-user pushes are established (`event:coin_settlement`, `system:honor_ejected`). Use the `system:` prefix (platform push, not in-match game state) → outside the drift gate.
- **Soft-deleted users:** `FindManyByIDs` excludes them, so a friend row can outlive its (soft-deleted) user — the list query naturally omits such friends; treat a missing user as "not shown," don't 500.

### Frontend implementation notes

- **Stack facts (verified):** axios wrapper `client/src/shared/api/axiosClient.ts` (base `/api/v1`, `{data}` unwrap so api fns return the inner payload, Bearer header, 401→refresh→retry-once central). TanStack Query v5 (queries in `shared/hooks/queries/`, mutations in `shared/hooks/mutations/`, keys centralized in `queryKeys.ts`). Named exports only; `@/` → `client/src/`. `react-router` `useParams`/`useNavigate`.
- **Friend data lives in the Query cache, not Zustand.** Server-collection data (rooms, profile, matches) is TanStack-cached; only session/live UI state (auth token, live game, live room, chat, level-up) is in the 5 Zustand stores. Friend list + requests = query hooks; the WS `system:friend_request` push just triggers `invalidateQueries(friends.requests())`. If you add an unseen-request badge, a tiny piece of state is fine but not required.
- **Add-Friend button** is a small state machine over `useFriendshipStatus`. Use the mutations to flip state and invalidate. Never render a placeholder — every status maps to a concrete affordance.
- **`useDebounce` (from Story 11.1) is not needed here** — friend requests are initiated from the profile button (reached via 11.1 search or direct URL), not a separate in-list search. If an add-by-search UX is desired, reuse 11.1's `PlayerSearch` (same branch) rather than duplicating; do not build a second search box.
- **Never JS-truthiness on the `online` bool** (Go-origin) — use `friend.online === true` / `typeof === "boolean"`. `[]` (not `null`) is guaranteed by the backend for empty lists.

### i18n notes

- Locale files `client/src/shared/i18n/{en,sr,mk,hr}.json`; four locales wired in `i18n.ts` (`["en","hr","sr","mk"]`, fallback `en`). Add a new top-level `friends.*` block to all four. `i18n.parity.test.ts` fails on any missing/extra/empty leaf. `mk` all-Cyrillic (proper nouns like "Beljot" stay Latin); no em dash (`—`) in `mk`/`sr`/`hr` (use `…`/`–`). `{{username}}` interpolation style.

### Testing standards summary

- **Go:** `testing` + `testify`. DB-backed repo tests via `getTestDB` (per-test `tx.Begin()` + `t.Cleanup(tx.Rollback)`, DSN dev DB `:5433`, skips if no DB) for the SQL-level pair/atomic/symmetry behavior; mock-repo handler tests via `httptest` `ServeHTTP` (hand-written mock implementing `friend.Repository` + a `notifierSpy` + a `UserRepository` stub; real JWT via `auth.GenerateAccessToken`; `testErrorHandler` mirrors prod). Name DB tests `TestGormRepository_Method_Case`.
- **Client:** Vitest + Testing Library. `vi.mock("@/shared/api/friends")`; render with `QueryWrapper`/`TestProviders` + `BrowserRouter`/`MemoryRouter`; `makeUser` fixtures; assert by `data-testid` in `waitFor`; present-tense `it(...)`. For the WS dispatch case, feed a synthetic `system:friend_request` message and assert `invalidateQueries` (spy the query client).

### Known Traps

- **Mock blast radius:** keep friend queries on the NEW `friend.Repository` — do NOT add methods to `user.UserRepository` (would break every user-repo mock in the tree). Reuse `FindByID`/`FindManyByIDs`.
- **Reverse-duplicate:** the single-direction unique index does not catch B→A vs A→B — enforce with `FindByPair`.
- **Self-request:** guard `userId == authUserID` at the handler (400) in addition to the `chk_friendships_not_self` CHECK constraint (defense in depth).
- **Recipient-only accept/decline, atomically:** conditional `WHERE ... AND friend_id=authUser AND status='pending'` with rows-affected; uniform 404 on any miss (don't leak authz/existence).
- **Never trust a body-supplied requester id** — requester = `getUserID(c)` always.
- **Return `[]` not `null`** for both list endpoints; **never JS-truthiness** on the Go `online` bool; **`*time.Time`** for any nullable timestamp.
- **Offline recipient must not fail the request** — gate the push on `IsConnected`, keep the 201.
- **Both WS contract files (events.go + wsEvents.ts) in the same commit** even though `system:` incurs no drift-gate golden/schema/contract-test changes.
- **One story = one branch is overridden** for this epic (whole epic on one branch, Task 0) — but still: if you find an unrelated bug, file it in `deferred-work.md`, don't fix it here.

### Project Structure Notes

- **New backend:** `server/internal/friend/{model,repository,gorm_repo,handler,gorm_repo_test,handler_test}.go`; `server/migrations/000019_create_friendships.{up,down}.sql`; additions to `server/internal/apperr/errors.go`, `server/cmd/api/main.go` (DI + routes), `server/internal/ws/events.go` (`system:friend_request`).
- **New frontend:** `client/src/shared/api/friends.ts`; `client/src/features/profile/components/FriendButton.tsx` (+ test); `client/src/features/friends/{FriendList,FriendRequests}.tsx` (+ tests); `client/src/shared/hooks/queries/{useFriends,useFriendRequests,useFriendshipStatus}.ts`; `client/src/shared/hooks/mutations/useFriendMutations.ts`. Additions to `apiTypes.ts`, `queryKeys.ts`, `wsEvents.ts` (`system:friend_request` const/type), `useWsDispatch.ts` (dispatch case), `PublicPlayerProfilePage.tsx` (mount FriendButton), `LobbyPage.tsx` (mount Friends panel), four i18n JSONs. Conforms to the feature-folder + `shared/` conventions; frontend api files map 1:1 to backend domains (`friends.ts` ↔ `friend`).

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-11.2-Friend-Requests-&-Friend-List] — user story + 4 ACs (FR6).
- [Source: _bmad-output/planning-artifacts/epics.md#Epic-11] — Epic objectives, FRs (FR5/FR6/FR47/FR61/FR62), Phase 3.
- [Source: _bmad-output/planning-artifacts/prd.md] — FR6 "Friend requests and friend list".
- [Source: _bmad-output/planning-artifacts/sprint-change-proposal-2026-08-14.md] — Epic 11 order 11-1→11-5; 11.4/11.5 depend on the 11-2 friend model + presence.
- [Source: _bmad-output/implementation-artifacts/11-3-public-player-profiles.md] — AC6/D4 Add-Friend deferral to 11.2; `PublicProfileResponse` shape; the `PublicPlayerProfilePage.tsx` insertion seam; `/players/:id` route.
- [Source: _bmad-output/implementation-artifacts/11-1-player-search.md] — search navigates result-click to `/players/:id`; `useDebounce`/`api/users.ts` introduced there (same branch).
- [Source: server/internal/user/{model,repository,gorm_repo,handler}.go] — domain package shape, `getUserID`, envelopes, `FindByID`/`FindByUsername`/`FindManyByIDs`, atomic `UpdateUsername` pattern.
- [Source: server/internal/identity/{model,repository,gorm_repo}.go] — closest new-package precedent (two-user-column table, hard delete, pg 23505 mapping).
- [Source: server/migrations/000013_create_refresh_tokens.*, 000014_create_user_identities.*] — migration format, FK/index idioms, next number = 000019.
- [Source: server/internal/apperr/errors.go] — error-code + status conventions (409 for conflict, 404, 400).
- [Source: server/cmd/api/main.go:131-159,240-254] — authenticated `api` group, `userRepo`/`hub` construction, route registration site.
- [Source: server/internal/ws/hub.go:178-232] — `SendToUser` (unqueued no-op), `IsConnected`, `ConnectedUserIDs`.
- [Source: server/internal/ws/events.go:5-9,336-349,383-448] — prefix conventions; `system:*` outside the drift gate; chat/emote payload precedent.
- [Source: client/src/shared/types/wsEvents.ts] — WS const/type mirror; `system:*` outside gate.
- [Source: client/src/shared/hooks/useWsDispatch.ts:558] — `dispatchSystemEvent` switch (add `system:friend_request` case).
- [Source: client/src/shared/api/{axiosClient,queryKeys,profile}.ts + hooks/{queries,mutations}/] — api-fn idiom, key factory, query/mutation hook patterns.
- [Source: client/src/features/profile/PublicPlayerProfilePage.tsx:155-159] — the Add-Friend insertion point.
- [Source: client/src/shared/i18n/*.json + i18n.parity.test.ts] — locales + parity/quality enforcement.
- [Source: _bmad-output/project-context.md] — Go/TS language rules, domain package shape, GORM tag bridge, WS contract rule, testing rules, i18n rules, "never JS-truthiness on Go zero values", "return [] not null".

## Dev Agent Record

### Agent Model Used

_TBD by dev-story_

### Debug Log References

### Completion Notes List

- Ultimate context engine analysis completed — comprehensive developer guide created.

### File List
