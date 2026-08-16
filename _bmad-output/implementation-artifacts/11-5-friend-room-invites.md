---
baseline_commit: a0db52f3f58999771efbb8706b1a0af2f03075da
---

# Story 11.5: Friend Room Invites

Status: done

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

- [x] **Task 0: Branch setup — whole-epic-on-one-branch**
  - [x] **Continue on the current branch `feat/11-3-public-player-profiles`** (do NOT cut a new branch). Per user direction, Epic 11 (11-1/11-2/11-4/11-5) ships as ONE feature/PR on this branch (same pattern as 9.7+9.8). **Hard dependency: Story 11.2** (friend list = the invitable roster; the "Invite to Room" hook it leaves). This story is the **last** of Epic 11 — do it after 11.2 (and after 11.4) in the on-branch order.

- [x] **Task 1: Backend — one-time invite-grant registry** (AC: #2, #3)
  - [x] Create an in-memory grant registry (mirror `room.PresenceRegistry` at `server/internal/room/presence.go:19-77` — a `map` guarded by `sync.Mutex`, best-effort, non-durable): `InviteRegistry` keyed by `(roomID, inviteeUserID)` → `{ inviterID, isHostInvite bool, expiresAt time.Time }`. Methods: `Issue(roomID, inviteeID, inviterID, isHost)`, `ConsumeHostGrant(roomID, inviteeID) bool` (one-time: returns true and deletes iff a non-expired **host** grant exists), `Void(roomID, inviteeID)`, `VoidRoom(roomID)` (on fill/close). Add a `time.Timer`-based TTL auto-void (reuse the `LobbyDisconnectHandler` timer discipline, `room/lobby_disconnect.go:18-42`). Construct it in `main.go` (like `room.NewPresenceRegistry()` at `main.go:236`) and inject into `RoomHandler` (`NewRoomHandler`, `main.go:254`).
  - [x] **The grant bypasses ONLY the password, ONLY for host invites.** It is looked up server-side by the authenticated `userID` + `roomID` — the client sends NO bypass flag (`JoinRoomRequest` gains no field).

- [x] **Task 2: Backend — invite endpoint + presence-based availability** (AC: #1, #2, #6)
  - [x] Add `InviteToRoom(c)` handler + route `api.POST("/rooms/:id/invite", roomHandler.InviteToRoom)` (co-located with the room action routes ~`main.go:255-272`). Flow:
    1. Auth (`getUserID`), parse room id, bind `{ friendUserId }`.
    2. `FindByID(roomID)`; nil or status != `waiting` → `ErrRoomNotFound`.
    3. Caller must be in the room: `FindPlayerRoom(callerID).RoomID == roomID` (owner or seated member), else `ErrForbidden`/not-in-room.
    4. Target must be the caller's **friend** (`friend.Repository.AreFriends(callerID, friendUserId)`, Story 11.2) and **available** (online + not-in-match + not-in-room — the presence trio). Not-available → a clear apperr (e.g. `ErrFriendNotAvailable`, 409).
    5. `isHost := room.OwnerID == callerID`. `inviteRegistry.Issue(roomID, friendUserId, callerID, isHost)` (host grant only bypasses password later).
    6. Push `system:room_invite` to the friend via `hub.SendToUser` with `{ inviteId, roomId, roomName, inviterUsername, coinBuyIn, isPrivate: room.PasswordHash != nil, isHostInvite }`. Return `200`.
  - [x] Reuse the `lobby.GetStats` availability recipe (`server/internal/lobby/lobby.go:73-136`): online via `hub.ConnectedUserIDs`/`IsConnected`, in-match via `match.Manager.IsUserInMatch`, in-room via `room.FindPlayerRoom`/`FindUserIDsByRoomStatus("waiting")`. Inject the same narrow trackers the lobby handler uses (`ConnectionTracker`/`SessionTracker` interfaces, `lobby.go:18-26`) rather than concrete types.
  - [x] Add `system:room_invite` const + `RoomInvitePayload` struct in `server/internal/ws/events.go` and the mirror const + interface in `client/src/shared/types/wsEvents.ts` (SAME commit). **`system:` ⇒ ZERO drift-gate touchpoints** (verified: room-lifecycle/eject `system:*` events have no golden/Zod/contract rows). Do NOT add to `events_contract_test.go`, `testdata/events/`, `wsEvents.schemas.ts`, or `wsEvents.contract.test.ts`.

- [x] **Task 3: Backend — hook the grant into `JoinRoom` (password bypass only)** (AC: #3, #4, #5, #6)
  - [x] In `JoinRoom` (`server/internal/room/handler.go:830-1001`), at the password block (`:864-868`), consult `inviteRegistry.ConsumeHostGrant(roomID, userID)` — if a valid host grant exists, **skip the `PasswordHash` check** and proceed. **Leave every other gate intact and in order:** status-waiting (:854), capacity (:870-884), already-in-room (:886-892), coin (:899-907), **honor (:918-928)**. The honor gate, `allow_new_players`, and capacity run for the invitee exactly as for any joiner (AC6). Consuming the grant is one-time (deleted on use).
  - [x] Auto-void the grant + invite when the room fills or closes: call `inviteRegistry.VoidRoom(roomID)` where the room transitions out of `waiting` / reaches capacity (e.g. after a successful join that fills it, and in the room-close/StartMatch paths). On invitee disconnect from the lobby, the existing `LobbyDisconnectHandler` is the hook to also void their outstanding grants.

- [x] **Task 4: Backend tests** (AC: #1, #2, #3, #4, #5, #6, #7)
  - [x] Grant-registry unit tests: issue/consume-once (second consume false), expiry, `Void`/`VoidRoom`, host vs non-host (non-host issues NO consumable host grant).
  - [x] `InviteToRoom` handler tests (DB-backed via `getRoomTestDB`, mirroring `honor_handler_test.go`/`privacy_handler_test.go` + a notifier spy): caller-not-in-room → forbidden; room not `waiting` → 404; non-friend target → error; unavailable friend (in room/match/offline) → 409; happy path issues a grant + pushes `system:room_invite` (host flag correct for owner vs member).
  - [x] `JoinRoom` grant tests: **host grant bypasses the password** (join a private room with NO password + a valid grant → 200) **but honor still enforced** (grant present + honor too low → `HONOR_TOO_LOW` 409, not seated); **capacity still enforced** (grant + full room → `ROOM_FULL`); **no grant on a private room** → `WRONG_ROOM_PASSWORD` (unchanged 9.6 behavior); non-host + private → password required (no grant). Assert the 9.6/9.8 gate tests still pass unedited (regression net).

- [x] **Task 5: Frontend — invite popup (WS → store → always-mounted modal)** (AC: #2, #3, #4, #5, #7)
  - [x] Add a `SYSTEM_ROOM_INVITE` case in `client/src/shared/hooks/useWsDispatch.ts` `dispatchSystemEvent` (~`:558`), modeled on the `SYSTEM_HONOR_EJECTED` per-user push (`:757-777`) with `typeof === "number"` validation for numeric fields (buy-in of `0` is legitimate). Write the invite into a store field (a new `roomInvite` on `roomStore.ts`, mirroring the `roomEjection` field pattern `:19-30,43,70`).
  - [x] Create `RoomInviteModal.tsx` (store-driven, always-mounted like `RoomEjectionModal` — `open = roomInvite !== null`, mounted in `LobbyPage.tsx` alongside `RoomEjectionModal` at `:282`). Shows inviter username, room name, buy-in, private badge, Accept/Decline. On Accept:
    - **host invite** (`isHostInvite === true`) → call `joinRoom(roomId)` (no password); the server grant handles the bypass. Navigate to `/rooms/:id`.
    - **non-host + private** (`isPrivate && !isHostInvite`) → open the Story 9.6 `PasswordPromptDialog`; submit password via `joinRoom(roomId, password)`; `WRONG_ROOM_PASSWORD` keeps it open.
    - **non-host + public** → `joinRoom(roomId)` directly.
  - [x] **Route the accept-join through the SAME failure-message mapping as every other join path** — the invite-accept is a NEW (4th+) join call path; it must reuse `joinFailureMessage(code, room)` (`RoomPage.tsx:246-263`) / the lobby toast mapping so `HONOR_TOO_LOW`/`NEW_PLAYER_NOT_ALLOWED`/`ROOM_FULL`/`ROOM_NOT_FOUND` (full/closed/expired) render correctly (AC6, AC7). Do not invent a parallel mapping.

- [x] **Task 6: Frontend — inviter-side invite panel (from the waiting room)** (AC: #1)
  - [x] Add an "Invite friends" panel/affordance in `client/src/features/room/RoomPage.tsx` (the waiting room), mounted near the room-info card / copy-code chip (`:995`) or between the info card (`:1256`) and the action bar (`:1263`). It lists the viewer's friends with availability (from a new query hitting the invite-availability data — either the friend list enriched with availability, or a dedicated endpoint) and an "Invite to Room" button per available friend (disabled + reason for unavailable). Wire the button to `POST /rooms/:id/invite`. This completes the Story 11.2 "Invite to Room" hook (11.2 left the affordance; 11.5 makes it deliver).
  - [x] Reuse the shared `Dialog` (controlled pattern, like the owner dialogs at `RoomPage.tsx:1567-1621`) for the panel; testids kebab-case per friend (`invite-friend-<userId>`), mirroring `in-room-list-item-<userId>`.

- [x] **Task 7: i18n** (AC: #8)
  - [x] Add a `roomInvite.*` block (invite panel labels, availability reasons, the popup title/body with `{{inviter}}`/`{{roomName}}`/`{{buyIn}}` interpolation, Accept/Decline, and a "friend not available" reason) to all four locales `client/src/shared/i18n/{en,sr,mk,hr}.json`. Reuse the existing `room.errors.*` keys for join failures (`wrongPassword`, `honorTooLow`, `newPlayerNotAllowed`, `roomFull`/not-found). `mk` all-Cyrillic; NO em dash in `mk`/`sr`/`hr`. `i18n.parity.test.ts` green.

- [x] **Task 8: Full validation gates** (AC: #8)
  - [x] `make lint` + `make test` green (Go grant-registry + invite handler + JoinRoom-grant tests RUN against dev DB `:5433`; the 9.6/9.8 regression tests pass unedited; client vitest for the modal, availability panel, and dispatch). Recommend a **manual E2E pass** (two accounts + a private honor-gated room) — 9.6/9.8 both found real bugs in manual E2E after review passed, and this touches the join spine. Update File List + Completion Notes.

### Review Findings

_Code review 2026-08-16 — three parallel adversarial layers (Blind Hunter, Edge Case Hunter, Acceptance Auditor) over the working-tree diff vs `a0db52f`. All quality gates independently re-verified green (`go build` / `go vet` / `golangci-lint` / `eslint` / `prettier` / `go test ./...` 20 pkgs / `vitest` 110 files 1192 tests / `tsc -p tsconfig.build.json`). Findings below are post-triage; severity is the reviewer's own, not the subagents'._

**Decision needed — all four RESOLVED by Emilijan 2026-08-16, converted to patches below**

- [x] [Review][Decision] **Invite popup renders only on `/lobby`, but server-side availability admits any online, room-free user** — `availabilityReason` (`invite_handler.go:297-312`) requires only online && !in-match && !in-room, so a friend on `/profile`, `/players/:id`, `/rules` or `/terms` is "available" and gets invited, but `RoomInviteModal` is mounted solely at `LobbyPage.tsx:277`. The push lands, `roomStore.roomInvite` is set, nothing renders; the 60s TTL burns down and the auto-dismiss effect drops it before it ever paints. The inviter's panel meanwhile shows "Invited". Note AC1 literally says "online AND **in the lobby** AND not in any room or match" — the implementation dropped the in-the-lobby clause. → **RESOLVED: mount the modal app-wide** (above the authenticated route tree); availability stays as the presence trio, no server change.
- [x] [Review][Decision] **Availability ignores the honor gate and `allow_new_players`, so guaranteed-to-fail friends are advertised as invitable** — `invite_handler.go:297-315`. In an honor-gated room a below-floor friend shows enabled, the invite is issued and the popup appears; accepting returns `HONOR_TOO_LOW`/`NEW_PLAYER_NOT_ALLOWED`. Because `minHonor` is absent from `RoomInvitePayload`, `joinFailureMessage(code, { coinBuyIn })` falls back to `room.errors.honorTooLowGeneric` — the invitee gets a numberless message, unlike every other join path. → **RESOLVED: carry `minHonor` in `RoomInvitePayload`** so the failure message is specific; keep the presence trio as AC1 defines it, do NOT pre-filter (avoids disclosing a friend's honor standing to the inviter).
- [x] [Review][Decision] **Declining is purely client-side — the grant stays consumable for the full TTL** — `RoomInviteModal.close()` only calls `setRoomInvite(null)`. `InviteRegistry.Void` is exported and unit-tested but has **zero non-test callers** (grep-verified). A friend who declines an invite into a private room can still walk past the password for up to 60s by clicking the room card, and the host's panel keeps saying "Invited". → **RESOLVED: add a decline endpoint** that calls `Void`, giving the method its caller and making decline mean what it says.
- [x] [Review][Decision] **No cooldown, throttle, or already-pending short-circuit on invites** — `invite_handler.go:205-240`. `Issue` re-arms on every POST and the client holds exactly one `roomInvite` slot, so any seated member (not only the owner) can repeatedly replace a friend's popup and reset its expiry as fast as they can send requests. Harassment vector. → **RESOLVED: reject a re-invite while a non-expired grant exists for the pair** — kills the flood vector and fixes the host-grant downgrade at the same root.

**Patch**

- [x] [Review][Patch] DECISION D1 — Mount `RoomInviteModal` above the authenticated route tree so an invite renders wherever the player is, not only on `/lobby` [client/src/App.tsx; client/src/features/lobby/LobbyPage.tsx:277]
- [x] [Review][Patch] DECISION D2 — Add `minHonor` to `RoomInvitePayload` (both contract files) and pass it into `joinFailureMessage` so the invite popup renders the specific honor message [server/internal/ws/events.go; client/src/shared/types/wsEvents.ts; client/src/features/lobby/components/RoomInviteModal.tsx:94]
- [x] [Review][Patch] DECISION D3 — Add a decline endpoint that calls `InviteRegistry.Void`, and wire the modal's Decline button to it [server/internal/room/invite_handler.go; server/cmd/api/main.go; client/src/features/lobby/components/RoomInviteModal.tsx:50-54]
- [x] [Review][Patch] DECISION D4 — Reject a re-invite while a non-expired grant exists for the (room, invitee) pair [server/internal/room/invite_handler.go:205-240; server/internal/room/invite_registry.go]

- [x] [Review][Patch] HIGH — Invite-accept swallows `WRONG_ROOM_PASSWORD` into a state nothing renders: permanent silent dead end (direct AC7 violation) [client/src/features/lobby/components/RoomInviteModal.tsx:84-88]
- [x] [Review][Patch] HIGH — Host grant is consumed before capacity/already-in-room/coin/honor, so any transient rejection burns it permanently [server/internal/room/handler.go:905-911]
- [x] [Review][Patch] MEDIUM — A later non-host `Issue` silently downgrades an outstanding host grant to `isHost:false` [server/internal/room/invite_registry.go:82-97]
- [x] [Review][Patch] MEDIUM — A second invite arriving while the password prompt is open submits the typed password to the new room [client/src/features/lobby/components/RoomInviteModal.tsx:192-201]
- [x] [Review][Patch] MEDIUM — `logout()` calls `roomStore.reset()`, which preserves `roomInvite`: the previous account's invite renders for the next user in the same tab [client/src/shared/stores/roomStore.ts:210-215]
- [x] [Review][Patch] MEDIUM — Auto-dismiss timer fires unconditionally mid-accept and mid-password-typing, tearing down both dialogs with no message [client/src/features/lobby/components/RoomInviteModal.tsx:60-71]
- [x] [Review][Patch] MEDIUM — `invited[userId]` is never cleared, so a declined or expired invite can never be re-sent without a page reload [client/src/features/room/components/InviteFriendsDialog.tsx:44,115]
- [x] [Review][Patch] MEDIUM — `ALREADY_IN_ROOM` on invite accept toasts with no navigation, unlike the two `RoomPage` join paths that resync [client/src/features/lobby/components/RoomInviteModal.tsx:93-94]
- [x] [Review][Patch] MEDIUM — `InviteToRoom` has no `IsQuickPlay` guard, unlike `AddBot`/`LeaveSeat`/`StartMatch`; a friend can be seated in a bracket they never qualified for [server/internal/room/invite_handler.go:254-285]
- [x] [Review][Patch] MEDIUM — Grants outlive the inviter's membership and host status (`LeaveRoom`, `TransferOwnership`, `KickPlayer` never void) [server/internal/room/handler.go:1823,2669,2798]
- [x] [Review][Patch] MEDIUM — i18n terminology drift: the new block calls the room a "table"/маса/stol/sto while the app-wide term is room/соба/soba, and `roomInvite.errors.roomFull` duplicates `lobby.errors.roomFull` with different wording on the same surface [client/src/shared/i18n/{en,mk,hr,sr}.json]
- [x] [Review][Patch] MEDIUM — i18n register mixing: mk mixes informal ти-forms with formal Вие-forms inside one block; hr/sr are consistently formal but drop to informal for `popup.accept` [client/src/shared/i18n/{mk,hr,sr}.json]
- [x] [Review][Patch] LOW — Lobby-disconnect room close never calls `VoidRoom`; the registry is not injected into `LobbyDisconnectHandler`, contradicting the completion note's "beside every `presence.Clear` site" claim [server/internal/room/lobby_disconnect.go:147]
- [x] [Review][Patch] LOW — `expiresAt` is silently coerced to `""` on a malformed payload while every other field is validated, disabling auto-dismiss entirely [client/src/shared/hooks/useWsDispatch.ts]
- [x] [Review][Patch] LOW — `"room_full"` is a magic string while its three sibling reasons are constants, and `reasonLabel` falls through to `""` — a disabled button with a blank explanation [server/internal/room/invite_handler.go:167; client/src/features/room/components/InviteFriendsDialog.tsx:47-60]
- [x] [Review][Patch] LOW — `ROOM_NOT_FOUND`/`NOT_IN_ROOM` from the invite endpoint render as a retryable "please try again" on a panel that can never succeed [client/src/features/room/components/InviteFriendsDialog.tsx:84-85]
- [x] [Review][Patch] LOW — Ejection modal and invite modal can be open simultaneously; the second traps focus and hides the first [client/src/features/lobby/LobbyPage.tsx:273-277]
- [x] [Review][Patch] LOW — hr/sr `popup.body` uses ASCII `"` instead of the documented `„…“` convention that mk's own new string follows [client/src/shared/i18n/{hr,sr}.json]
- [x] [Review][Patch] LOW — Completion note "All 1169 pre-existing client tests still pass, proving no message changed for any existing path" is unsupported: the shared mapping adds `ROOM_NOT_FOUND` and `ALREADY_IN_ROOM` branches that `LobbyPage`/`RoomPage` lacked. The change is an improvement; the claim is wrong and should be corrected [_bmad-output/implementation-artifacts/11-5-friend-room-invites.md]

**Deferred**

- [x] [Review][Defer] N+1 presence query per friend on the invite panel, plus redundant room re-reads in `InviteToRoom` [server/internal/room/invite_handler.go:143-150,212,228,304] — deferred, perf-only; the `FindPlayerRoom`-over-`FindUserIDsByRoomStatus` choice is correctness-justified in the completion notes
- [x] [Review][Defer] ~30 pre-existing `tsc --noEmit` errors in client test files, invisible to CI [client/src/features/**/*.test.tsx] — deferred, pre-existing (Stories 9.7/9.8 fields), not caused by this change

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

Claude Opus 5 (`claude-opus-5[1m]`) via bmad-dev-story.

### Debug Log References

- `go test ./... -count=1` — all 21 packages green (DB-backed room/privacy/honor tests confirmed RUNNING, not skipping, against dev DB `:5433`).
- `npx vitest run` — 110 files / 1192 tests green (1169 pre-existing + 23 new).
- `make lint` — ESLint + Prettier + `golangci-lint` all clean.
- `go test ./internal/room/ -race` — the NEW invite code is race-clean (`TestInviteRegistry_ConcurrentAccess`, 50 goroutines). One PRE-EXISTING race in `TestLobbyDisconnect_FreesAfterTimeout` reproduces identically on baseline `a0db52f` via `git stash -u`; filed to `deferred-work.md`, untouched here.

### Completion Notes List

- Ultimate context engine analysis completed — comprehensive developer guide created.
- **D1 honoured — `system:room_invite`, zero drift-gate touchpoints.** Const + `RoomInvitePayload` added to `server/internal/ws/events.go` and `client/src/shared/types/wsEvents.ts` in the same change. Verified by grep that the event appears in NO golden (`ws/testdata/events/`), NO `events_contract_test.go` row, NO `wsEvents.schemas.ts`, and NO `wsEvents.contract.test.ts` row. The 20 contract tests pass unchanged.
- **D2/D3 honoured — the grant bypasses ONLY the password, ONLY for host invites, and is never client-supplied.** `InviteRegistry.ConsumeHostGrant` is consulted *inside* the `room.PasswordHash != nil` branch of `JoinRoom`, so a grant is spent only when there is actually a password to bypass; every other gate (status-waiting → capacity → already-in-room → coin → honor) runs untouched and in its original order. `JoinRoomRequest` gained **no field**. Five backend tests pin the negative: a host grant does NOT bypass `HONOR_TOO_LOW`, `NEW_PLAYER_NOT_ALLOWED`, `ROOM_FULL` (human capacity), or bot-covered capacity, and a **non-host** grant still yields `WRONG_ROOM_PASSWORD`.
- **The 9.6 / 9.8 regression net is intact.** `privacy_handler_test.go` is byte-for-byte untouched. `honor_handler_test.go`, `coin_handler_test.go` and `handler_test.go` changed **only** in `NewRoomHandler` call arity (a trailing `, nil`) — zero assertions, fixtures, or expectations altered (verifiable in the diff: 8 changed lines, all the same mechanical shape). This follows the project's own precedent, where Story 9.8 added `honorService` as a positional parameter the same way.
- **D4 honoured, and the duplication that caused it was removed at the root.** The invite-accept is a 4th join entry point, so rather than copy the mapping a 4th time the shared `client/src/shared/lib/joinFailure.ts` was extracted, and **all four** paths now route through it: `LobbyPage.joinRoomFlow`, `JoinByCodeTile.toastError`, `RoomPage.joinFailureMessage` (kept as a thin wrapper so its call sites are unchanged), and `RoomInviteModal`. Its `JoinFailureContext` takes `coinBuyIn` / `minHonor` independently. **Correction (code review 2026-08-16):** the original note claimed "All 1169 pre-existing client tests still pass, proving no message changed for any existing path" — that inference is wrong. The shared mapping adds `ROOM_NOT_FOUND` and `ALREADY_IN_ROOM` branches that `LobbyPage.joinRoomFlow` and `RoomPage.joinFailureMessage` did not have (both previously fell through to `lobby.errors.joinFailed`), so two messages DID change on pre-existing paths. The change is an improvement and consistent with D4's intent; the green suite proves only that those branches were untested. Also superseded: the invite popup now DOES know the room's honor floor — `minHonor` was added to `RoomInvitePayload` in review, so the invite path renders the *specific* honor message like every other join entry point.
- **Auto-void coverage (AC2) is wired at every transition, keyed off an existing invariant.** `VoidRoom` is called beside **every** `presence.Clear(roomID)` site — those are exactly the "match started / room closed" transitions — plus a new `voidInvitesIfFull` helper on `JoinRoom`, `ReturnToRoom` and `AddBot` (a bot can take the last seat too). `VoidUser` fires on `JoinRoom`/`ReturnToRoom` (the invitee is now in a room) and in `main.go`'s `hub.SetDisconnectHandler`. **Note:** the disconnect hook is deliberately in `main.go`, NOT inside `LobbyDisconnectHandler.HandleDisconnect` as Task 3 suggested — that method early-returns for users not in a room, which is precisely every invitee, so wiring it there would have been dead code.
- **Scope note — two additions beyond the literal task list, both implied by AC2/AC7.** (1) `InviteToRoom` rejects an invite into an already-full room with `ROOM_FULL`, since AC2 says a filled room voids invites and issuing one into a full room creates a grant that is born void. (2) `ListInvitableFriends` disables every row with reason `room_full` in that case. Both are surfaced rather than silent.
- **Architecture note — `InviteHandler` is a separate handler from `RoomHandler`.** The invite endpoints need four dependencies (`FriendDirectory`, `ConnectionTracker`, `SessionTracker`, `InviteNotifier`) that nothing else in `RoomHandler` uses; threading them through its already-six-argument constructor would have touched every room handler test for no benefit. `RoomHandler` gained only the one thing it actually uses — the shared `*InviteRegistry`. Import direction stays one-way (`room` never imports `friend`): the `inviteFriendDirectory` adapter in `main.go` does the friend-rows → usernames join, mirroring how `chatRoomMembership` / `whisperPresenceLocator` already bridge domains.
- **Story 11.2's parked "Invite to Room" hook is resolved, not left dangling.** An invite is issued INTO a room, and a player reading the lobby friend list is by definition in none — so the working affordance lives in the waiting room (`InviteFriendsDialog`, opened from `RoomPage`'s action bar), and the lobby button now says where to find it instead of being a dead no-op. The 11.2 button and its `friend-invite-room` testid are preserved, so `FriendList.test.tsx` passes unedited.
- **Availability is server-computed and never trusted from the client (AC1/AC6).** The presence trio uses `hub.IsConnected` → `match.Manager.IsUserInMatch` → `repo.FindPlayerRoom`. `FindPlayerRoom` (waiting OR playing) was chosen over the lobby's batched `FindUserIDsByRoomStatus("waiting")` deliberately: it is the *same predicate* `JoinRoom`'s already-in-room gate uses, so the panel can never advertise a friend whose join would then be rejected. It is re-checked at invite time, and the join gates catch anything still stale.
- **Recommended before merge: a manual E2E pass** with two accounts and a private, honor-gated room — 9.6 and 9.8 each had a real bug survive review and surface only in manual E2E, and this story touches the join spine.

### File List

**Backend — new**

- `server/internal/room/invite_registry.go` — the one-time host-invite grant registry (mutex + TTL timers, `Issue` / `ConsumeHostGrant` / `Pending` / `Void` / `VoidRoom` / `VoidUser`)
- `server/internal/room/invite_registry_test.go` — issue/consume-once, non-host never consumable, room+invitee scoping, expiry, re-issue, the three void paths, 50-goroutine race test
- `server/internal/room/invite_handler.go` — `InviteHandler` (`InviteToRoom`, `ListInvitableFriends`), the `FriendDirectory` / `ConnectionTracker` / `SessionTracker` / `InviteNotifier` narrow interfaces, the presence-trio availability rule
- `server/internal/room/invite_handler_test.go` — 19 tests: invite gates, availability trio, and the JoinRoom-under-grant matrix (password bypassed, honor/new-player/capacity/bot-capacity NOT bypassed)

**Backend — modified**

- `server/internal/room/handler.go` — `RoomHandler.invites` field + `NewRoomHandler` parameter; the host-grant consult in `JoinRoom`'s password block; `voidInvitesIfFull` helper; `VoidRoom` beside all five `presence.Clear` sites; `VoidUser` on join/return; `voidInvitesIfFull` on join/return/add-bot
- `server/internal/ws/events.go` — `SystemRoomInvite` const + `RoomInvitePayload`
- `server/internal/apperr/errors.go` — `ErrNotFriends`, `ErrFriendNotAvailable`
- `server/cmd/api/main.go` — `inviteRegistry` construction + injection, the two invite routes, `inviteFriendDirectory` adapter, `VoidUser` on WS disconnect
- `server/internal/room/handler_test.go`, `honor_handler_test.go`, `coin_handler_test.go` — `NewRoomHandler` call arity only (trailing `, nil`); no assertion changed

**Frontend — new**

- `client/src/features/lobby/components/RoomInviteModal.tsx` — the store-driven incoming-invite popup (host / non-host-private / non-host-public accept paths, expiry auto-dismiss)
- `client/src/features/lobby/components/RoomInviteModal.test.tsx` — 13 tests covering all three accept paths + the shared failure mapping
- `client/src/features/room/components/InviteFriendsDialog.tsx` — the inviter-side friend panel with availability reasons and inline per-row errors
- `client/src/features/room/components/InviteFriendsDialog.test.tsx` — 6 tests
- `client/src/shared/hooks/queries/useInvitableFriends.ts` — the invite-panel roster query
- `client/src/shared/lib/joinFailure.ts` — THE shared join-failure → message mapping (D4)

**Frontend — modified**

- `client/src/shared/types/wsEvents.ts` — `SYSTEM_ROOM_INVITE` + `RoomInvitePayload` (mirror of events.go)
- `client/src/shared/types/apiTypes.ts` — `InvitableFriend`
- `client/src/shared/hooks/useWsDispatch.ts` — the `SYSTEM_ROOM_INVITE` dispatch case (typeof validation, no truthiness)
- `client/src/shared/stores/roomStore.ts` — `RoomInvite` type, `roomInvite` field, `setRoomInvite`, reset preservation
- `client/src/shared/api/rooms.ts` — `listInvitableFriends`, `inviteToRoom`
- `client/src/shared/api/queryKeys.ts` — `rooms.invitableFriends(roomId)`
- `client/src/shared/hooks/mutations/useRooms.ts` — `useInviteToRoomMutation`
- `client/src/features/room/RoomPage.tsx` — the "Invite friends" action-bar button + dialog mount; `joinFailureMessage` delegates to the shared helper
- `client/src/features/lobby/LobbyPage.tsx` — `RoomInviteModal` mount; `joinRoomFlow` uses the shared mapping
- `client/src/features/lobby/components/JoinByCodeTile.tsx` — `toastError` uses the shared mapping
- `client/src/features/friends/FriendList.tsx` — the Story 11.2 invite hook resolved (points to the room panel)
- `client/src/shared/i18n/{en,sr,mk,hr}.json` — the `roomInvite.*` block (26 leaves × 4 locales)
- `client/src/shared/hooks/useWsDispatch.test.ts`, `client/src/shared/stores/roomStore.test.ts` — new invite tests appended

**Docs**

- `_bmad-output/implementation-artifacts/deferred-work.md` — filed the pre-existing `TestLobbyDisconnect_FreesAfterTimeout` `-race` failure

### Code Review Fixes (2026-08-16)

All 23 patch findings applied; the 4 decision items were resolved by Emilijan and implemented. Gates re-verified after the fixes: `go build` / `go vet` / `golangci-lint` clean, `go test ./...` 20 packages ok, `eslint` + `prettier` clean, `tsc -p tsconfig.build.json` clean, `vitest` **110 files / 1193 tests** green.

**The two that mattered most, and how they compounded.** `JoinRoom` consumed the host grant inside the password block — the *first* of six gates — so a rejection from capacity, already-in-room, coin or honor burned it permanently. The invitee's retry then hit bcrypt for a password they were never given, and `RoomInviteModal` wrote that `WRONG_ROOM_PASSWORD` into a `PasswordPromptDialog` rendered with `open={false}`: no toast, no close, no navigation, Accept re-enabled. A recoverable "the room was briefly full" turned into a silent, permanent dead end — the exact failure AC7 exists to prevent. Fixed at both ends: the password gate now PEEKS (`HasHostGrant`) and the grant is spent by the post-success `VoidUser`, and the modal dismisses with a clear message whenever no prompt is open to render into.

**Backend**
- `invite_registry.go` — `HasHostGrant` (non-consuming peek); `Issue` never downgrades a live host grant; new `VoidInviter(roomID, inviterID)`.
- `handler.go` — `JoinRoom` peeks instead of consuming; `VoidInviter` on `LeaveRoom`, `KickPlayer`, `TransferOwnership` so a departed or demoted ex-owner's grant stops bypassing the *current* owner's password.
- `invite_handler.go` — `IsQuickPlay` rejected in `requireRoomMember` (JoinRoom has no bracket check, so an invite would seat a friend in a bracket they never qualified for); `INVITE_ALREADY_PENDING` throttle; new `DeclineInvite` endpoint; `room_full` promoted to `inviteReasonRoomFull`; `MinHonor` in the push.
- `lobby_disconnect.go` — registry injected; `VoidInviter` + `VoidRoom` on the timeout close, the one room-close transition that had neither.
- `apperr/errors.go` — `ErrInviteAlreadyPending` (409). `ws/events.go` + `wsEvents.ts` — `MinHonor`, same change both files.
- 10 new tests in `invite_handler_test.go` pinning: grant survives a rejected join and is spent by a successful one, non-host never downgrades, `VoidInviter` scoping, transfer-ownership void, pending throttle, quick-play rejection, decline (void + idempotent), honor floor in the payload.

**Frontend**
- `RoomInviteModal.tsx` — the dead-end fix; prompt state reset on `inviteId` change (a second invite could otherwise submit room A's typed password to room B); auto-dismiss inert while joining or typing; `ALREADY_IN_ROOM` navigates like every other join path; Decline calls the new endpoint; suppressed while `roomEjection` is live so two always-mounted dialogs cannot fight over focus.
- **Mount moved from `LobbyPage` to `AppLayout`** — the server marks a friend invitable whenever they are online, not in a match and not in a room, which includes anyone on `/profile`, `/players/:id` or `/rules`. Lobby-only mounting made every such invite a silent black hole: pushed, stored, never rendered, expired, while the inviter's panel read "Invited".
- `roomStore.ts` / `authStore.ts` — new `clearSessionNotices()` used by logout; `reset()` preserves `roomInvite` by design, but logout is a session boundary and was leaking the previous account's invite (and inviter's username) to the next user in the same tab.
- `useWsDispatch.ts` — `expiresAt` validated rather than coerced to `""`; `minHonor` carried through.
- `InviteFriendsDialog.tsx` — per-row state cleared on open (a declined/expired invite left the button dead as "Invited" until a page reload); unknown reason slugs get a real label instead of a blank line; `ROOM_NOT_FOUND`/`NOT_IN_ROOM` close the panel instead of inviting a doomed retry.
- `AppLayout.test.tsx` — wrapped in `QueryClientProvider`, which the layout has in production via `App`'s `QueryProvider`.

**i18n (all four locales rewritten).** Terminology: the block called the room a "table"/маса/stol/sto against the app-wide room/соба/soba, so mk showed „Оваа маса е полна" in the panel and „Собата е полна" in the toast on the same surface. Register: corrected to **informal** in mk/hr/sr — measured against each file's own dominant usage (`Обиди се` 23 vs `Обидете се` 6; hr `Pokušaj` 22 vs `Pokušajte` 6, `Unesi` 10 vs `Unesite` 0), which means hr/sr needed the *opposite* correction from what a first read suggested. Quotes: hr/sr `popup.body` moved from ASCII `"` to `„…“`. Three new keys (`errors.expired`, `errors.roomGone`, `reasons.unavailable`). Verified: 958 leaves × 4 with zero missing/extra, mk values all-Cyrillic, no em dash in mk/hr/sr.

**Deferred (see `deferred-work.md`):** the N+1 presence query on the invite panel (perf; the `FindPlayerRoom` predicate choice is correctness-justified), and ~30 pre-existing `tsc` errors in client test files that CI structurally cannot see (`make lint` runs no `tsc`; `tsconfig.build.json` excludes tests).

### Manual E2E Verification (2026-08-16)

The pass the story itself recommended, run against `make dev` (Vite :5173, Go :8080, Postgres :5433) with Playwright driving a real browser as the **invitee** and REST/WS bots as the **inviter** — two accounts in one browser would collide on the httpOnly refresh cookie, so the sides were split. Zero server panics; no unexpected ERROR lines.

| # | Scenario | Result |
|---|---|---|
| 1 | AC1 — invite panel lists friends with server-computed availability; unavailable shown disabled + reason | PASS |
| 2 | AC3 — host invite into a private room → accept → straight to `/rooms/:id`, **no password prompt** | PASS |
| 3 | AC4 — non-host member invite into a private room → accept → password prompt, badge reads "password required" | PASS |
| 4 | **P1 (Critical)** — host invite, room fills (grant voided), then accept → modal closes + toast "That invite is no longer valid." | PASS |
| 5 | **AC6** — host invite into a private **veterans-only** room → password bypassed, but rejected `NEW_PLAYER_NOT_ALLOWED` with the specific message; DB confirms invitee **not seated** | PASS |
| 6 | **D1** — invite delivered while the invitee is on `/profile` → popup renders (was a silent black hole) | PASS |
| 7 | **D3** — decline → `POST /invite/decline` 200 → grant voided → re-entry now demands the password | PASS |
| 8 | **D4** — repeat invites → `INVITE_ALREADY_PENDING`; a member invite cannot overwrite the owner's live host grant | PASS |
| 9 | **P4** — second invite arriving while the password prompt is open with text typed → prompt resets to the new invite, typed password discarded | PASS |
| 10 | **P5** — unanswered invite + logout → `roomInvite` cleared; next account in the same tab sees nothing | PASS |
| 11 | **P7** — invite a friend, close and reopen the panel → button resets to "Invite" (was stuck "Invited" until reload) | PASS |
| 12 | AC2 — TTL auto-dismiss observed; `system:room_invite` push carries `minHonor` + `expiresAt` over a real socket | PASS |

Not reachable as written: the `roomGone` panel branch (P14). When the owner leaves, the existing room-lifecycle handling navigates them to the lobby and unmounts the panel first, so `ROOM_NOT_FOUND` never renders there. The branch stays as defence for the narrower race (kicked in another tab).

#### Out-of-scope bug found during E2E: every error response was logged as HTTP 200

Spotted while auditing the server log for the invite runs: `POST /rooms/undefined/invite` was logged `status: 200` while the client had plainly received `404`. Reproduced generally — a bad-credentials login returns `401` to the client and logs `200`, as does every 4xx/5xx in the app. **No error rate of any kind was visible in the server logs.**

Cause: Echo calls `HTTPErrorHandler` *after* the middleware chain unwinds, so `RequestLogger` read `c.Response().Status` before the error handler had written anything and recorded Echo's default 200. The documented remedy is `HandleError: true`, which routes the error through `appErrorHandler` inside the chain so the logged status is the one the client actually got.

Fixed in `server/cmd/api/main.go`: added `HandleError: true` + `LogError: true`, and split the log level so failures surface — 5xx → `slog.Error`, 4xx → `slog.Warn`, else `slog.Info`, with the error message attached. Verified after restart: `404 room not found` and `401 invalid email or password` now log at WARN with the correct status.

**This is PRE-EXISTING and unrelated to Story 11.5**, and the project rule is "if you discover a bug while working on a story, file it separately — don't fix it in the current branch unless it directly blocks the story." It does not block 11.5. The fix is 15 lines in `main.go` and touches no story code, but it is on this branch and needs Emilijan's call: keep it here, or revert and re-land as its own `fix/` branch.

## Change Log

## Change Log

- 2026-08-16 — Story 11.5 implemented on `feat/11-3-public-player-profiles` (whole-epic-on-one-branch, Task 0). Server-authorized one-time host-invite grant registry + `POST /rooms/:id/invite` + `GET /rooms/:id/invitable-friends`; `system:room_invite` added to both contract files (zero drift-gate touchpoints, D1); the grant bypasses the room password and nothing else (D2/D3), pinned by tests asserting honor/new-player/capacity still fire under a grant; incoming-invite popup + inviter-side panel on the client; the join-failure mapping extracted to `shared/lib/joinFailure.ts` and adopted by all four join entry points (D4); `roomInvite.*` in all four locales. `make lint` + `make test` green (Go: 21 packages; client: 1192 tests).
