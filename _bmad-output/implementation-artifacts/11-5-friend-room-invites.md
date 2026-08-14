# Story 11.5: Friend Room Invites

Status: ready-for-dev

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a room member,
I want to invite my available friends into my room,
so that we can play together quickly — with the host able to pull friends past the room password, while other members' invitees still enter it.

## Acceptance Criteria

**Epic source:** `_bmad-output/planning-artifacts/epics.md#Story-11.5-Friend-Room-Invites` (FR62) + `sprint-change-proposal-2026-08-14.md §4e` (with the confirmed decision: invites bypass **only** the password for **host** invites; the honor gate is never bypassed for anyone). The seven epic ACs are expanded below. The WS event **prefix** decision (`system:room_invite`, not the epic's literal `event:room_invite`) is recorded in Dev Notes → **Design Decision D1** (**PO-confirmed 2026-08-14**).

1. **AC1 — Invite panel lists available friends from a waiting room.** A player who is the owner of, or seated in, a room with `status == "waiting"` can open a friend/invite panel. Each **available** friend — **online AND in the lobby AND not in any room or match** — shows an "Invite to Room" action (extends the Story 11.2 hook). Friends who are in a room/match or offline are shown as **not invitable** (disabled + reason). "Available" is computed server-side from the presence trio (online = `hub.IsConnected`, not-in-match = `match.Manager.IsUserInMatch`, not-in-room = `room.FindPlayerRoom == nil`), applied to the viewer's accepted-friend set (Story 11.2).
2. **AC2 — Sending an invite pushes a popup; it expires / auto-voids.** `POST /api/v1/rooms/:id/invite` `{ friendUserId }` (caller must be in the room, room `waiting`, friend available) delivers a `system:room_invite` push to the friend: `{ inviteId, roomId, roomName, inviterUsername, coinBuyIn, isPrivate, isHostInvite }`. The invite **expires after a timeout** and is **auto-voided** if the room fills (`PlayerCount + bots >= 4`), closes (status != `waiting`), or the friend leaves the lobby (disconnect). The push is best-effort (offline friend → silent no-op; nothing to deliver, invite simply never actioned).
3. **AC3 — Host invite bypasses the password (server-authorized one-time grant).** When the inviter **is the room owner** (`room.OwnerID`), confirming the popup auto-joins the room **bypassing the room password even if one is set**, via a **server-issued one-time grant** consulted inside `JoinRoom` — the bypass is server-authorized (looked up by the authenticated `userId` + `roomId`), **never a client-supplied flag**. No `PasswordPromptDialog` is shown for a host invite.
4. **AC4 — Non-host invite + private room → password prompt.** When the inviter is a **non-host** member AND the room has a password (Story 9.6), confirming shows the Story 9.6 `PasswordPromptDialog`; the friend must enter the correct room password to join. A wrong password → `error`/409 `WRONG_ROOM_PASSWORD` (reuses the existing `JoinRoom` bcrypt gate, no grant).
5. **AC5 — Non-host invite + no password → direct join.** When the inviter is a non-host member AND the room has no password, confirming joins directly with no password step (normal `JoinRoom`, no grant needed).
6. **AC6 — Honor gate, `allow_new_players`, and capacity ALWAYS apply.** For **every** invitee (host or non-host), the room's honor gate + `allow_new_players` rule (Story 9.8) and seat capacity still apply — **only the password is bypassed, and only for host invites**. The invite-accept path runs the exact same `JoinRoom` gate sequence (capacity → already-in-room → coin → honor); only the `PasswordHash` block is skipped when a valid host grant exists. An invitee who fails the honor gate gets the standard `HONOR_TOO_LOW` / `NEW_PLAYER_NOT_ALLOWED` 409 (non-disclosing message), and is NOT seated.
7. **AC7 — Graceful failure on full / closed / expired.** If an invited friend confirms but the room is now full, closed, or the invite has expired, the join fails gracefully with a clear localized message and the friend remains in the lobby (no dead-end, no stuck modal). Reuses the existing join error → toast/message mapping.
8. **AC8 — WS contract (D1) + i18n + quality gates.** The invite push is `system:room_invite` (D1), added to **both** contract files in the same commit (zero drift-gate touchpoints — verified against the room-lifecycle/eject `system:*` precedent). All new user-facing strings are in all four locales (`en`/`sr`/`mk`/`hr`), `mk` all-Cyrillic, no em dash in `mk`/`sr`/`hr`, parity green. `make lint` + `make test` pass.

## Tasks / Subtasks

- [ ] **Task 0: Branch setup — whole-epic-on-one-branch**
  - [ ] **Continue on the current branch `feat/11-3-public-player-profiles`** (do NOT cut a new branch). Per user direction, Epic 11 (11-1/11-2/11-4/11-5) ships as ONE feature/PR on this branch (same pattern as 9.7+9.8). **Hard dependency: Story 11.2** (friend list = the invitable roster; the "Invite to Room" hook it leaves). This story is the **last** of Epic 11 — do it after 11.2 (and after 11.4) in the on-branch order.

- [ ] **Task 1: Backend — one-time invite-grant registry** (AC: #2, #3)
  - [ ] Create an in-memory grant registry (mirror `room.PresenceRegistry` at `server/internal/room/presence.go:19-77` — a `map` guarded by `sync.Mutex`, best-effort, non-durable): `InviteRegistry` keyed by `(roomID, inviteeUserID)` → `{ inviterID, isHostInvite bool, expiresAt time.Time }`. Methods: `Issue(roomID, inviteeID, inviterID, isHost)`, `ConsumeHostGrant(roomID, inviteeID) bool` (one-time: returns true and deletes iff a non-expired **host** grant exists), `Void(roomID, inviteeID)`, `VoidRoom(roomID)` (on fill/close). Add a `time.Timer`-based TTL auto-void (reuse the `LobbyDisconnectHandler` timer discipline, `room/lobby_disconnect.go:18-42`). Construct it in `main.go` (like `room.NewPresenceRegistry()` at `main.go:236`) and inject into `RoomHandler` (`NewRoomHandler`, `main.go:254`).
  - [ ] **The grant bypasses ONLY the password, ONLY for host invites.** It is looked up server-side by the authenticated `userID` + `roomID` — the client sends NO bypass flag (`JoinRoomRequest` gains no field).

- [ ] **Task 2: Backend — invite endpoint + presence-based availability** (AC: #1, #2, #6)
  - [ ] Add `InviteToRoom(c)` handler + route `api.POST("/rooms/:id/invite", roomHandler.InviteToRoom)` (co-located with the room action routes ~`main.go:255-272`). Flow:
    1. Auth (`getUserID`), parse room id, bind `{ friendUserId }`.
    2. `FindByID(roomID)`; nil or status != `waiting` → `ErrRoomNotFound`.
    3. Caller must be in the room: `FindPlayerRoom(callerID).RoomID == roomID` (owner or seated member), else `ErrForbidden`/not-in-room.
    4. Target must be the caller's **friend** (`friend.Repository.AreFriends(callerID, friendUserId)`, Story 11.2) and **available** (online + not-in-match + not-in-room — the presence trio). Not-available → a clear apperr (e.g. `ErrFriendNotAvailable`, 409).
    5. `isHost := room.OwnerID == callerID`. `inviteRegistry.Issue(roomID, friendUserId, callerID, isHost)` (host grant only bypasses password later).
    6. Push `system:room_invite` to the friend via `hub.SendToUser` with `{ inviteId, roomId, roomName, inviterUsername, coinBuyIn, isPrivate: room.PasswordHash != nil, isHostInvite }`. Return `200`.
  - [ ] Reuse the `lobby.GetStats` availability recipe (`server/internal/lobby/lobby.go:73-136`): online via `hub.ConnectedUserIDs`/`IsConnected`, in-match via `match.Manager.IsUserInMatch`, in-room via `room.FindPlayerRoom`/`FindUserIDsByRoomStatus("waiting")`. Inject the same narrow trackers the lobby handler uses (`ConnectionTracker`/`SessionTracker` interfaces, `lobby.go:18-26`) rather than concrete types.
  - [ ] Add `system:room_invite` const + `RoomInvitePayload` struct in `server/internal/ws/events.go` and the mirror const + interface in `client/src/shared/types/wsEvents.ts` (SAME commit). **`system:` ⇒ ZERO drift-gate touchpoints** (verified: room-lifecycle/eject `system:*` events have no golden/Zod/contract rows). Do NOT add to `events_contract_test.go`, `testdata/events/`, `wsEvents.schemas.ts`, or `wsEvents.contract.test.ts`.

- [ ] **Task 3: Backend — hook the grant into `JoinRoom` (password bypass only)** (AC: #3, #4, #5, #6)
  - [ ] In `JoinRoom` (`server/internal/room/handler.go:830-1001`), at the password block (`:864-868`), consult `inviteRegistry.ConsumeHostGrant(roomID, userID)` — if a valid host grant exists, **skip the `PasswordHash` check** and proceed. **Leave every other gate intact and in order:** status-waiting (:854), capacity (:870-884), already-in-room (:886-892), coin (:899-907), **honor (:918-928)**. The honor gate, `allow_new_players`, and capacity run for the invitee exactly as for any joiner (AC6). Consuming the grant is one-time (deleted on use).
  - [ ] Auto-void the grant + invite when the room fills or closes: call `inviteRegistry.VoidRoom(roomID)` where the room transitions out of `waiting` / reaches capacity (e.g. after a successful join that fills it, and in the room-close/StartMatch paths). On invitee disconnect from the lobby, the existing `LobbyDisconnectHandler` is the hook to also void their outstanding grants.

- [ ] **Task 4: Backend tests** (AC: #1, #2, #3, #4, #5, #6, #7)
  - [ ] Grant-registry unit tests: issue/consume-once (second consume false), expiry, `Void`/`VoidRoom`, host vs non-host (non-host issues NO consumable host grant).
  - [ ] `InviteToRoom` handler tests (DB-backed via `getRoomTestDB`, mirroring `honor_handler_test.go`/`privacy_handler_test.go` + a notifier spy): caller-not-in-room → forbidden; room not `waiting` → 404; non-friend target → error; unavailable friend (in room/match/offline) → 409; happy path issues a grant + pushes `system:room_invite` (host flag correct for owner vs member).
  - [ ] `JoinRoom` grant tests: **host grant bypasses the password** (join a private room with NO password + a valid grant → 200) **but honor still enforced** (grant present + honor too low → `HONOR_TOO_LOW` 409, not seated); **capacity still enforced** (grant + full room → `ROOM_FULL`); **no grant on a private room** → `WRONG_ROOM_PASSWORD` (unchanged 9.6 behavior); non-host + private → password required (no grant). Assert the 9.6/9.8 gate tests still pass unedited (regression net).

- [ ] **Task 5: Frontend — invite popup (WS → store → always-mounted modal)** (AC: #2, #3, #4, #5, #7)
  - [ ] Add a `SYSTEM_ROOM_INVITE` case in `client/src/shared/hooks/useWsDispatch.ts` `dispatchSystemEvent` (~`:558`), modeled on the `SYSTEM_HONOR_EJECTED` per-user push (`:757-777`) with `typeof === "number"` validation for numeric fields (buy-in of `0` is legitimate). Write the invite into a store field (a new `roomInvite` on `roomStore.ts`, mirroring the `roomEjection` field pattern `:19-30,43,70`).
  - [ ] Create `RoomInviteModal.tsx` (store-driven, always-mounted like `RoomEjectionModal` — `open = roomInvite !== null`, mounted in `LobbyPage.tsx` alongside `RoomEjectionModal` at `:282`). Shows inviter username, room name, buy-in, private badge, Accept/Decline. On Accept:
    - **host invite** (`isHostInvite === true`) → call `joinRoom(roomId)` (no password); the server grant handles the bypass. Navigate to `/rooms/:id`.
    - **non-host + private** (`isPrivate && !isHostInvite`) → open the Story 9.6 `PasswordPromptDialog`; submit password via `joinRoom(roomId, password)`; `WRONG_ROOM_PASSWORD` keeps it open.
    - **non-host + public** → `joinRoom(roomId)` directly.
  - [ ] **Route the accept-join through the SAME failure-message mapping as every other join path** — the invite-accept is a NEW (4th+) join call path; it must reuse `joinFailureMessage(code, room)` (`RoomPage.tsx:246-263`) / the lobby toast mapping so `HONOR_TOO_LOW`/`NEW_PLAYER_NOT_ALLOWED`/`ROOM_FULL`/`ROOM_NOT_FOUND` (full/closed/expired) render correctly (AC6, AC7). Do not invent a parallel mapping.

- [ ] **Task 6: Frontend — inviter-side invite panel (from the waiting room)** (AC: #1)
  - [ ] Add an "Invite friends" panel/affordance in `client/src/features/room/RoomPage.tsx` (the waiting room), mounted near the room-info card / copy-code chip (`:995`) or between the info card (`:1256`) and the action bar (`:1263`). It lists the viewer's friends with availability (from a new query hitting the invite-availability data — either the friend list enriched with availability, or a dedicated endpoint) and an "Invite to Room" button per available friend (disabled + reason for unavailable). Wire the button to `POST /rooms/:id/invite`. This completes the Story 11.2 "Invite to Room" hook (11.2 left the affordance; 11.5 makes it deliver).
  - [ ] Reuse the shared `Dialog` (controlled pattern, like the owner dialogs at `RoomPage.tsx:1567-1621`) for the panel; testids kebab-case per friend (`invite-friend-<userId>`), mirroring `in-room-list-item-<userId>`.

- [ ] **Task 7: i18n** (AC: #8)
  - [ ] Add a `roomInvite.*` block (invite panel labels, availability reasons, the popup title/body with `{{inviter}}`/`{{roomName}}`/`{{buyIn}}` interpolation, Accept/Decline, and a "friend not available" reason) to all four locales `client/src/shared/i18n/{en,sr,mk,hr}.json`. Reuse the existing `room.errors.*` keys for join failures (`wrongPassword`, `honorTooLow`, `newPlayerNotAllowed`, `roomFull`/not-found). `mk` all-Cyrillic; NO em dash in `mk`/`sr`/`hr`. `i18n.parity.test.ts` green.

- [ ] **Task 8: Full validation gates** (AC: #8)
  - [ ] `make lint` + `make test` green (Go grant-registry + invite handler + JoinRoom-grant tests RUN against dev DB `:5433`; the 9.6/9.8 regression tests pass unedited; client vitest for the modal, availability panel, and dispatch). Recommend a **manual E2E pass** (two accounts + a private honor-gated room) — 9.6/9.8 both found real bugs in manual E2E after review passed, and this touches the join spine. Update File List + Completion Notes.

## Dev Notes

### Design Decisions (READ FIRST)

- **D1 — WS prefix: `system:room_invite`, NOT `event:room_invite` (deviation, justified).** The epic AC prescribes `event:room_invite`, but a room invite is a **pre-match, room-level, per-user platform push** — structurally identical to `system:honor_ejected` / `system:room_updated`, which are all `system:*`. `event:*` is reserved for in-match game state and is the ONLY prefix inside the WS drift gate. Story 9.8's D4 set this exact precedent (an honor eject is `system:*`, not `event:*`, for zero drift-gate cost). **Decision:** use `system:room_invite` — 2-file diff, outside the gate, consistent with all other room lifecycle pushes. **PO-confirmed 2026-08-14: `system:room_invite` accepted** — proceed with the 2-file diff (zero drift-gate touchpoints). The literal `event:room_invite` is NOT used.
- **D2 — The grant bypasses ONLY the password, ONLY for host invites; the honor gate is NEVER bypassed** (confirmed decision, `sprint-change-proposal-2026-08-14.md §4e`). The invite-accept path lands at the single authoritative `JoinRoom` gate and runs capacity → already-in-room → coin → honor unchanged. Only `room.PasswordHash` verification is skipped, and only when a valid **host** grant exists. This is the 9.6-D5 / 9.8 "one gate at the seat grant" discipline — do not add a second, weaker gate on the invite path.
- **D3 — Server-authorized grant, never a client flag.** The bypass is a server-side one-time grant looked up by the authenticated `userId` + `roomId`. `JoinRoomRequest` gains NO field. A client cannot self-authorize a bypass. Storage is an in-memory registry (like `PresenceRegistry` / the lobby-disconnect timers) — best-effort and non-durable is correct for an ephemeral invite.
- **D4 — Invite-accept is a NEW join call path — reuse the shared failure mapping.** 9.6/9.8's hardest-won lesson: there are multiple client join entry points (`LobbyPage.joinRoomFlow`, `JoinByCodeTile`, and BOTH `RoomPage` deep-link paths) unified through `joinFailureMessage`, and a new error code added to one and forgotten on another is a real bug caught only in manual E2E. The invite-accept is another such path — it MUST reuse `joinFailureMessage`/the lobby toast mapping, not a parallel one, or a full/closed/gated invite will render a wrong or missing message (AC7).
- **D5 — Friend + availability depend on Story 11.2 + presence.** The invitable roster is the viewer's accepted friends (11.2), filtered to "available" via the presence trio (`lobby.GetStats` recipe). 11.2 must land first. The "Invite to Room" affordance itself is the hook 11.2 leaves and 11.5 completes.

### Backend implementation notes

- **JoinRoom gate order (preserve exactly), `handler.go:830-1001`:** auth → parse id → **status `waiting`** (:854) → **password** (:864-868, the only bypass point) → **capacity** (:870-884) → **already-in-room** (:886-892) → **coin affordability** (:899-907) → **honor** (:918-928) → tx add + `presence.Add` + broadcasts. `JoinRoom` adds a seatless `room_players` row; seat is a separate `SelectSeat`. `honorGated()` (`:81-83`) short-circuits ungated rooms; `honorGateError` (`:109-120`) is the pure gate (New Player never score-checked — D1 of 9.8). Snapshots via `honorSnapshotFor` read the authoritative recomputed score, never `users.honor_score`.
- **apperr codes (reuse):** `ErrWrongRoomPassword` (409 `WRONG_ROOM_PASSWORD`), `ErrInsufficientCoins` (409), `ErrHonorTooLow`/`ErrNewPlayerNotAllowed` (409), `ErrRoomFull`/`ErrAlreadyInRoom` (409), `ErrRoomNotFound` (404). New: `ErrFriendNotAvailable` (409) for the invite endpoint (and reuse `ErrForbidden` for caller-not-in-room). All join-gate rejections are HTTP, not WS events.
- **Presence trio (availability):** online `hub.IsConnected`/`ConnectedUserIDs` (`hub.go:216-232`); in-match `match.Manager.IsUserInMatch`/`InMatchUserIDs` (`live_match.go:615-633`); in-room `room.FindPlayerRoom` (waiting OR playing, `gorm_repo.go:164-177`) / `FindUserIDsByRoomStatus("waiting")`. `lobby.GetStats` (`lobby.go:73-136`) is the exact bucketing precedent; invariant `online == inLobby + inRoom + inMatch`, so "available" = online AND not-in-match AND not-in-room.
- **Grant registry precedent:** `room.PresenceRegistry` (`presence.go:19-77`) = `sync.Mutex` + `map`, constructed `main.go:236`, injected `main.go:254`. `LobbyDisconnectHandler` (`lobby_disconnect.go:18-42`) = in-memory map + `time.Timer` expiry — reuse both patterns for the invite TTL/auto-void.
- **Per-user push:** `hub.SendToUser` / `BroadcastToUsers` — silent no-op for offline (no offline inbox). The invite popup uses the same per-recipient push as the eject events.
- **Hand-built-map trap (only if you add a Room field):** `roomLifecyclePayload` (`handler.go:359-411`) is a hand-built `map[string]any`; a new room column must be added there (and the separate QuickPlay `room_created` map + `lobby_disconnect.go:222-236` map). This story adds **no new Room column** (the invite is transient), so the trap is only a caution — the invite payload itself is `system:*` and exempt from the golden/Zod gate.

### Frontend implementation notes

- **Store:** `client/src/shared/stores/roomStore.ts` (NOT `roomLobbyStore` — that name does not exist) holds `currentRoomId`, live seat/player state, and `roomEjection`. Add a `roomInvite` field mirroring `roomEjection` (set by the WS dispatch, cleared on modal close). `reset()` should preserve `roomInvite` the way it preserves `roomEjection` (`:176`) so a navigation doesn't drop an in-flight invite.
- **Modal patterns:** store-driven always-mounted (`RoomEjectionModal`, `open = roomEjection !== null`, mounted `LobbyPage.tsx:282`) for the incoming invite popup; controlled `useState(open)` + shared `Dialog` (owner dialogs `RoomPage.tsx:1567-1621`) for the inviter-side friend panel. `PasswordPromptDialog` (`features/lobby/components/PasswordPromptDialog.tsx`, props `{open, roomName, pending, errorKey, onSubmit, onClose}`) is reused verbatim for the non-host private path.
- **Join API:** `client/src/shared/api/rooms.ts` `joinRoom(id, password?)` (`:29`) already sends `{password}` only when defined — host-invite accept calls it with no password; non-host private calls it with the entered password. `useJoinRoomMutation` in `hooks/queries/useRooms.ts`.
- **Failure mapping:** `joinFailureMessage(code, room)` (`RoomPage.tsx:246-263`) + the lobby `joinRoomFlow` toast mapping (`LobbyPage.tsx:187-231`) — reuse for the accept path (D4). Use the param-less `*Generic` message variants when no `room` object is in hand (as `JoinByCodeTile` does).
- **Never JS-truthiness on Go numerics/bools** — validate `system:room_invite` payload with `typeof === "number"`/`=== true`; a `coinBuyIn` of `0` and `isPrivate: false` are legitimate real values.

### i18n notes

- Four locales in `client/src/shared/i18n/` (`["en","hr","sr","mk"]`, fallback `en`); `i18n.parity.test.ts` enforces 1:1 leaf parity + non-empty. Add a `roomInvite.*` block to all four; reuse existing `room.errors.*` for join failures. `mk` all-Cyrillic; no em dash (`—`) in `mk`/`sr`/`hr`. Interpolation `{{inviter}}`/`{{roomName}}`/`{{buyIn}}`.

### Testing standards summary

- **Go:** DB-backed room handler tests via `getRoomTestDB` (templates: `room/honor_handler_test.go`, `privacy_handler_test.go`, `coin_handler_test.go`); grant-registry pure-unit tests; real-websocket test (`ws/ws_test.go`) if asserting end-to-end push delivery. **Critically: assert the 9.6 password tests and 9.8 honor/eject tests still pass UNEDITED** — they are the regression net proving the grant bypasses only the password. testify `require`/`assert`.
- **Client:** Vitest + Testing Library; precedents `RoomEjectionModal.test.tsx`, `PasswordPromptDialog.test.tsx`, `RoomPage.test.tsx`, `LobbyPage.test.tsx`, `useWsDispatch.test.ts`, `roomStore.test.ts`. Feed synthetic `system:room_invite`; assert the store field + modal open; assert host-accept calls `joinRoom` with no password and non-host-private opens `PasswordPromptDialog`; assert the failure mapping renders `HONOR_TOO_LOW`/`ROOM_FULL` correctly.

### Known Traps

- **Invite must NOT bypass honor/capacity/coin/already-in-room** (D2) — only the `PasswordHash` block, only for host grants. Assert honor+capacity still fire under a grant.
- **Server-authorized grant, never a client flag** (D3) — `JoinRoomRequest` gains no field; grant looked up by authed `userId`+`roomId`.
- **New join call path → reuse `joinFailureMessage`** (D4) — a full/closed/gated invite must render the right message; don't fork the mapping.
- **Expiry / auto-void races** — `sync.Mutex` + `time.Timer`; void on timeout, room-fill (`PlayerCount+bots >= 4`), room-close (status != `waiting`), friend-leaves-lobby (disconnect). Offline `SendToUser` is a silent no-op — no offline inbox.
- **`system:` not `event:`** (D1) — no drift-gate goldens/Zod/contract rows; both contract files same commit.
- **Availability is server-computed** — do not trust a client "friend is available" claim; recompute via the presence trio at invite time and (implicitly) again at `JoinRoom` (capacity/already-in-room gates catch stale state).
- **9.6/9.8 regression tests must pass unedited** — the proof the bypass is password-only.
- **One story = one branch is overridden** for this epic (whole epic on one branch, Task 0) — still file unrelated bugs to `deferred-work.md`.

### Project Structure Notes

- **New backend:** an invite-grant registry (`server/internal/room/invite_registry.go` or similar, + test); `InviteToRoom` handler + route in `server/internal/room/handler.go` + `cmd/api/main.go`; grant consult in `JoinRoom`; `system:room_invite` const + payload in `server/internal/ws/events.go`; `ErrFriendNotAvailable` in `apperr/errors.go`. NO new Room column, NO migration (invite is transient/in-memory).
- **New frontend:** `RoomInviteModal.tsx` (store-driven) + an inviter-side friend/invite panel in `RoomPage.tsx`; a `roomInvite` field on `roomStore.ts`; a `SYSTEM_ROOM_INVITE` dispatch case in `useWsDispatch.ts`; the `system:room_invite` const/type in `wsEvents.ts`; `POST /rooms/:id/invite` in `api/rooms.ts` + a query for invitable-friend availability; `roomInvite.*` in four i18n JSONs. Conforms to feature-folder + `shared/` conventions.

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-11.5-Friend-Room-Invites] — user story + 7 ACs (FR62).
- [Source: _bmad-output/planning-artifacts/sprint-change-proposal-2026-08-14.md §4e] — FR62; host-only password bypass via server-authorized one-time grant; honor gate never bypassed (confirmed decision).
- [Source: _bmad-output/implementation-artifacts/11-2-friend-requests-and-friend-list.md] — friend list + `AreFriends`; the "Invite to Room" hook this story completes.
- [Source: _bmad-output/implementation-artifacts/9-6-private-rooms.md] — `PasswordPromptDialog`, `WRONG_ROOM_PASSWORD`, the multiple join entry points, "verify only at the seat grant" (D5).
- [Source: _bmad-output/implementation-artifacts/9-8-honor-gated-rooms.md] — honor gate sites, `honorGateError`/`honorGated`, `system:honor_ejected` (`system:*` outside gate, D4), the "check every entry point" lesson.
- [Source: server/internal/room/handler.go:81-120,359-411,830-1001,1399-1435,1461,1617] — `honorGated`/`honorGateError`, `roomLifecyclePayload` (hand-built-map trap), `JoinRoom` gate order + password block, `ejectNotice`/`ejectReturner`/`ejectSeatsAtStart`.
- [Source: server/internal/room/gorm_repo.go:164-177,251-262] — `FindPlayerRoom` (waiting/playing presence), `FindUserIDsByRoomStatus`.
- [Source: server/internal/room/presence.go:19-77 + lobby_disconnect.go:18-42] — in-memory registry + timer-based auto-void precedents.
- [Source: server/internal/lobby/lobby.go:18-26,73-136] — availability bucketing (online/in-match/in-room), narrow tracker interfaces.
- [Source: server/internal/ws/hub.go:178-232] — `SendToUser`/`BroadcastToUsers` (silent no-op offline), `IsConnected`, `ConnectedUserIDs`.
- [Source: server/internal/ws/events.go:280-411,335-349] — room-lifecycle `system:*` events; `system:*` outside the drift gate.
- [Source: server/internal/apperr/errors.go] — join-gate error codes + statuses.
- [Source: server/cmd/api/main.go:236,254,255-272] — `PresenceRegistry` construction/injection, room action-route registration.
- [Source: client/src/shared/hooks/useWsDispatch.ts:558,757-777] — `dispatchSystemEvent`; `SYSTEM_HONOR_EJECTED` per-user push template.
- [Source: client/src/shared/stores/roomStore.ts:19-30,43,70,176] — `roomEjection` field + `reset()` preservation (template for `roomInvite`).
- [Source: client/src/features/lobby/components/PasswordPromptDialog.tsx + RoomEjectionModal.tsx] — reused dialog + store-driven modal.
- [Source: client/src/features/lobby/LobbyPage.tsx:187-231,282 + components/JoinByCodeTile.tsx:38-75] — `joinRoomFlow` toast mapping, generic-variant messages, modal mount point.
- [Source: client/src/features/room/RoomPage.tsx:246-263,265-348,995,1256-1263,1567-1621] — `joinFailureMessage`, the two deep-link join paths, waiting-room UI mount points, owner-dialog pattern.
- [Source: client/src/shared/api/rooms.ts:29 + hooks/queries/useRooms.ts] — `joinRoom(id, password?)`, `useJoinRoomMutation`.
- [Source: client/src/shared/i18n/*.json + i18n.parity.test.ts] — locales + parity; existing `room.errors.*` keys.
- [Source: _bmad-output/project-context.md] — WS contract rule, server-authoritative security, "never JS-truthiness on Go zero values", i18n rules, migration rules.

## Dev Agent Record

### Agent Model Used

_TBD by dev-story_

### Debug Log References

### Completion Notes List

- Ultimate context engine analysis completed — comprehensive developer guide created.

### File List
