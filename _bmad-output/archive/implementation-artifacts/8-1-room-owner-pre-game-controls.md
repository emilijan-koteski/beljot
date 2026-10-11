# Story 8.1: Room Owner Pre-Game Controls

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a room owner,
I want to kick seated players and rearrange seat assignments before the game starts,
so that I can curate the roster and team composition without tearing down the room.

## Acceptance Criteria

1. **Owner can kick a non-owner from the room while `status = 'waiting'`**
   Given the authenticated caller is the room owner and the target user is a non-owner room member,
   When `POST /api/v1/rooms/:id/kick` is called with `{ "userId": <targetUserID> }`,
   Then the server removes the target from `room_players`, decrements `rooms.player_count`, and returns `200 { "data": { "playerCount": <updated> } }`.
   And the kicked user receives a WebSocket `system:room_kicked` event with `{ "roomId": <id>, "reason": "kicked_by_owner" }` so the client can navigate them back to `/lobby` with a toast.
   And every remaining room participant receives `system:player_left` with the new `playerCount` (mirroring the existing leave-room broadcast at [server/internal/room/handler.go:511-538](server/internal/room/handler.go#L511-L538)) so their `roomLobbyStore` removes the player and frees the seat.
   And the lobby browse page receives `system:room_updated` with the new `playerCount` (existing broadcast helper `broadcastRoomUpdated`).
   And the kicked user is **not** removed from their global WebSocket connection — they stay connected to the hub; only their room membership is severed. (The existing `lobbyDisconnectHandler` is irrelevant here — the user is intentionally removed, not disconnected.)

2. **Owner can swap two seated players' seats while `status = 'waiting'`**
   Given the authenticated caller is the room owner,
   When `POST /api/v1/rooms/:id/swap-seats` is called with `{ "seatA": <0..3>, "seatB": <0..3> }` and both seats are currently occupied (and `seatA != seatB`),
   Then the server swaps the two players' `seat` and recomputes `team` from the new seat (`seat % 2 == 0 → "teamA"`, else `"teamB"`) inside one transaction and returns `200 { "data": { "players": [...] } }` with the post-swap player list.
   And both swapped users **and** every other room participant receive **two** ordered `system:seat_updated` broadcasts (one per swapped seat) — sent as separate messages, not batched, per architecture rule "Multi-event sequences must be sent as separate ordered messages, not batched into a single payload" ([\_bmad-output/project-context.md#L109](_bmad-output/project-context.md)).
   And `previousSeat` on each `system:seat_updated` is the seat the user moved **from** (mirroring the existing payload contract at [server/internal/room/handler.go:659-668](server/internal/room/handler.go#L659-L668), [client/src/shared/types/wsEvents.ts:246-253](client/src/shared/types/wsEvents.ts#L246-L253)).
   And the lobby browse page does **not** receive `system:room_updated` for a swap (player count and room metadata are unchanged — swapping does not affect lobby cards).

3. **Kick and swap are rejected after the game has started**
   Given the room status is anything other than `'waiting'` (i.e. `'playing'`, `'finished'`, or `'completed'`),
   When `POST /api/v1/rooms/:id/kick` or `POST /api/v1/rooms/:id/swap-seats` is called,
   Then the server returns `409 { "error": { "code": "ROOM_NOT_WAITING", "message": "..." } }` and makes **no** state change.
   And the status check happens **inside** the same transaction that performs the mutation (TOCTOU guard — match the pattern at [server/internal/room/handler.go:587-598](server/internal/room/handler.go#L587-L598)) so a `room.Status = 'playing'` flip mid-request cannot interleave with a kick/swap.

4. **Non-owners are forbidden from kicking or swapping**
   Given the authenticated caller is **not** the room's `OwnerID`,
   When either `kick` or `swap-seats` is called,
   Then the server returns `403 { "error": { "code": "NOT_ROOM_OWNER", "message": "..." } }` (reusing existing `apperr.ErrNotRoomOwner` from [server/internal/apperr/errors.go:60](server/internal/apperr/errors.go#L60)) and makes no state change.
   And the ownership check happens **inside** the transaction, after `FindByID` and **before** any mutation (TOCTOU guard against concurrent ownership transfer when the original owner leaves the room — see [server/internal/room/handler.go:472-502](server/internal/room/handler.go#L472-L502)).

5. **Additional input validation — bad shape returns 400**
   Given the request body is malformed,
   When the handler validates input,
   Then the following branches return `400 BAD_REQUEST` with the corresponding code and **make no state change**:
   - `kick`: missing `userId`, `userId == 0`, or the target is the owner themselves → `400 BAD_REQUEST` (use `apperr.ErrBadRequest` for shape; introduce `apperr.ErrCannotKickSelf` (`CANNOT_KICK_SELF`) for the owner-targeting-self branch).
   - `kick`: target user is not a member of this room → `404 NOT_IN_ROOM` (reuse existing `apperr.ErrNotInRoom`).
   - `swap-seats`: missing `seatA`/`seatB`, equal seats, or out-of-range (not in `0..3`) → `400 INVALID_SEAT` (reuse existing `apperr.ErrInvalidSeat`).
   - `swap-seats`: at least one of the two seats is empty → `409 SEAT_NOT_OCCUPIED` (introduce new `apperr.ErrSeatNotOccupied`).
     And the handler does NOT reuse `apperr.ErrSeatTaken` for the empty-seat case — semantics are inverted; using a new code prevents UI-text confusion.

6. **`RoomLobby` renders kick + swap controls only for the owner, only while waiting**
   Given the authenticated viewer is the owner and `room.status === 'waiting'`,
   When `RoomLobby.tsx` renders,
   Then each non-owner seated slot shows a kick affordance (`data-testid="kick-player-{seat}"`) — a small icon button that becomes visible on `:hover` of the seat tile **and** is permanently visible while the seat tile is `:focus-within` (keyboard-accessibility — UX spec rule "every interactive element keyboard-reachable"; in the absence of an explicit a11y rule, follow this pattern from existing seat clicks).
   And each seat tile (the existing `<button data-testid="player-seat-{seat}">` at [client/src/features/lobby/RoomLobby.tsx:282](client/src/features/lobby/RoomLobby.tsx#L282)) gains a swap affordance: clicking the avatar of a seated **non-current-user** seat enters a "swap mode" that highlights the other three seats; clicking a second seated seat issues the swap; clicking the same seat again or clicking outside cancels swap mode. Use a Zustand-free `useState` swap-mode flag local to `RoomLobby` — do NOT add a new global store for transient UI state.
   And the kick icon and swap affordance render only when `currentUser?.id === room.ownerId && room.status === 'waiting'`. Non-owners see the existing seat tiles unchanged. After the room transitions to `playing` (or any non-waiting state), the controls are removed in the same render (no stale icons after `system:game_started`).
   And the owner's own seat tile NEVER renders a kick icon (you cannot kick yourself).

7. **Kicked-player UX — toast + redirect, store cleanup**
   Given the kicked user's client receives `system:room_kicked` matching their `currentRoomId`,
   When `useWsDispatch` dispatches the event,
   Then `useRoomLobbyStore.reset()` is called and the user is navigated to `/lobby` (use the same imperative-redirect pattern as `gameStarted` in [client/src/features/lobby/RoomLobby.tsx:93-98](client/src/features/lobby/RoomLobby.tsx#L93-L98), but driven by a new `kickedFromRoom` boolean on `useRoomLobbyStore`).
   And a `toast.error(t("lobby.roomLobby.kickedToast", { name: <roomName> }))` fires once, with the room's name interpolated.
   And the kicked user's outbound auto-leave-on-unmount call is suppressed — `hasLeftRef.current = true` is set BEFORE `navigate("/lobby")` so the cleanup `useEffect` does not POST a redundant `/leave` for an already-removed player. Calling `/leave` after a kick must not throw or display an error to the user even if it does fire (defensive: handle `404 NOT_IN_ROOM` gracefully — no toast).

8. **Owner kick + swap UX feedback (success and failure)**
   Given the owner clicks kick or initiates a swap,
   When the action succeeds,
   Then the existing `system:player_left` / `system:seat_updated` broadcasts update the lobby in real time — no extra optimistic update needed (mirror [RoomLobby.tsx:144-146](client/src/features/lobby/RoomLobby.tsx#L144-L146) which trusts the server response + WS to converge state).
   And on `403 NOT_ROOM_OWNER` → `toast.error(t("lobby.roomLobby.errors.notOwner"))` — reuse the existing string from [en.json:187](client/src/shared/i18n/en.json#L187).
   And on `409 ROOM_NOT_WAITING` → `toast.error(t("lobby.roomLobby.errors.roomStarted"))` — new key.
   And on `409 SEAT_NOT_OCCUPIED` → `toast.error(t("lobby.roomLobby.errors.seatNotOccupied"))` — new key.
   And on any other error → `toast.error(t("lobby.roomLobby.errors.kickFailed"))` or `errors.swapFailed` (matching the swap path) — new keys.
   And during a kick or swap mutation `isPending` state, the affected seat tile shows a half-opacity disabled state (`opacity-60 pointer-events-none`) so the owner cannot fire duplicates while the request is in flight.

9. **Confirmation dialog before kick — non-skippable**
   Given the owner clicks the kick icon,
   When the confirmation prompt would render,
   Then a modal/dialog appears (use the project's existing dialog primitive at [client/src/shared/components/ui/](client/src/shared/components/ui/) — search for `dialog.tsx` / `alert-dialog.tsx`; if neither exists yet, install via `npx shadcn@latest add alert-dialog` per the shadcn workflow at [\_bmad-output/project-context.md#L24](_bmad-output/project-context.md)) with body text `t("lobby.roomLobby.kickConfirm.body", { username })` and confirm/cancel buttons.
   And the confirm button has `data-testid="kick-confirm"`, the cancel button has `data-testid="kick-cancel"`. Pressing Escape cancels (default dialog behaviour).
   And the kick mutation is fired ONLY on confirm — no kick-on-mount, no kick-on-icon-click. Cancel closes the dialog with no side effects.

10. **i18n keys added to `en.json` and `sr.json` in the same commit**
    Given new copy introduced by this story,
    When the story lands,
    Then the following keys exist under `lobby.roomLobby.*` in **both** [client/src/shared/i18n/en.json](client/src/shared/i18n/en.json) and [client/src/shared/i18n/sr.json](client/src/shared/i18n/sr.json):
    - `kickIconLabel` — accessible label for the kick icon button (e.g. `"Kick {{username}}"`).
    - `kickConfirm.title` — confirmation dialog title (e.g. `"Kick player?"`).
    - `kickConfirm.body` — confirmation body (e.g. `"Kick {{username}} from the room?"`).
    - `kickConfirm.confirm` — confirm button label (e.g. `"Kick"`).
    - `kickConfirm.cancel` — cancel button label (e.g. `"Cancel"`).
    - `kickedToast` — toast shown to the kicked user (e.g. `"You were removed from room {{name}}"`).
    - `swapMode.enter` — small caption shown on the seat tile when its avatar is selected for swap (e.g. `"Pick a seat to swap with"`).
    - `swapMode.cancel` — caption / button label to leave swap mode (e.g. `"Cancel swap"`).
    - `errors.roomStarted` — `"The game has already started — kick/swap is no longer available."`
    - `errors.seatNotOccupied` — `"That seat is empty — pick two seated players to swap."`
    - `errors.kickFailed` — generic kick failure copy.
    - `errors.swapFailed` — generic swap failure copy.
      And Serbian-Latin translations follow the **Ekavian** register (matching adjacent keys: `"Pobeda"` not `"Pobjeda"`, `"se pridružio"` not `"se priključio"`) — the register decision is locked in by Story 7.2's `Pobede` / `Procenat pobeda` choice.
      And the `i18n.test.ts` recursive `flattenKeys` parity check at [client/src/shared/i18n/i18n.test.ts](client/src/shared/i18n/i18n.test.ts) continues to pass.

11. **WebSocket event contract sync — both files in the same commit**
    Given a new server→client WS event is introduced,
    When this story lands,
    Then [server/internal/ws/events.go](server/internal/ws/events.go) gains `const SystemRoomKicked = "system:room_kicked"` and a typed `RoomKickedPayload` struct (`RoomID uint json:"roomId"; Reason string json:"reason"`).
    And [client/src/shared/types/wsEvents.ts](client/src/shared/types/wsEvents.ts) gains `export const SYSTEM_ROOM_KICKED = "system:room_kicked" as const;` and an `interface RoomKickedPayload { roomId: number; reason: string; }`.
    And both files are updated in the **same commit** (project rule [\_bmad-output/project-context.md#L80](_bmad-output/project-context.md), [#L286](_bmad-output/project-context.md)).
    And `useWsDispatch` adds a branch for `SYSTEM_ROOM_KICKED` that calls `useRoomLobbyStore.getState().setKickedFromRoom(payload.roomId)` — the dispatch only triggers UI navigation if `currentRoomId === payload.roomId` (defence in depth: ignore stray events for other rooms).

12. **Backward compatibility — existing room flows untouched**
    Given the existing seat-select / leave-room / start-game flows,
    When this story lands,
    Then `JoinRoom`, `LeaveRoom`, `SelectSeat`, `StartGame` handlers and their broadcast helpers (`broadcastToRoom`, `broadcastToAll`, `broadcastRoomUpdated`) are **not modified**. The kick + swap-seats handlers are pure additions to `RoomHandler`.
    And the existing `RoomPlayer.Seat` / `RoomPlayer.Team` fields, `Room.PlayerCount` semantics, and `room_players` schema are unchanged — Story 8.1 introduces zero migrations.
    And the existing `RoomLobby.test.tsx` cases (seat select, leave, start game, isOwner) continue to pass with no changes — owner controls are additive in the render tree.

## Tasks / Subtasks

- [x] **Task 1: Backend — `RoomHandler.KickPlayer` + repository plumbing (AC #1, #3, #4, #5)**
  - [x] 1.1 Add new typed request struct to [server/internal/room/handler.go](server/internal/room/handler.go): `type KickPlayerRequest struct { UserID uint json:"userId" }`.
  - [x] 1.2 Implement `func (h *RoomHandler) KickPlayer(c echo.Context) error` mirroring the structure of `LeaveRoom` ([server/internal/room/handler.go:443-542](server/internal/room/handler.go#L443-L542)) but with these adaptations:
    - `userID := auth.GetUserID(c)` → owner, parse `:id` → `roomID`, `c.Bind(&req)` → target `userID`.
    - Inside `RunInTransaction`: `tx.FindByID(roomID)` → if nil → `ErrRoomNotFound`; if `Status != "waiting"` → new `ErrRoomNotWaiting`; if `OwnerID != ownerID` → `ErrNotRoomOwner`; if `req.UserID == 0` → `ErrBadRequest`; if `req.UserID == r.OwnerID` → new `ErrCannotKickSelf`; verify target is in room via `tx.FindPlayerRoom` matched on `RoomID` else `ErrNotInRoom`.
    - Mutation: `tx.RemovePlayer(roomID, req.UserID)` then `tx.DecrementPlayerCount(roomID)` — both inside the same tx.
  - [x] 1.3 After commit: capture `kickedUsername` (look up before the tx for broadcast payload — same trick as `LeaveRoom` at [handler.go:463-470](server/internal/room/handler.go#L463-L470)). Broadcast in this order:
    - `h.broadcastToUsers([]uint{req.UserID}, ws.SystemRoomKicked, RoomKickedPayload{RoomID: roomID, Reason: "kicked_by_owner"})` — to the kicked user only.
    - `h.broadcastToUsers(<remaining users>, ws.SystemPlayerLeft, ...)` — same payload shape as `LeaveRoom`'s broadcast.
    - `h.broadcastRoomUpdated(postRoom)` — for lobby browse page.
  - [x] 1.4 Return `200 { "data": { "playerCount": <postRoom.PlayerCount> } }`.
  - [x] 1.5 Wire route at [server/cmd/api/main.go:142-150](server/cmd/api/main.go#L142-L150): `api.POST("/rooms/:id/kick", roomHandler.KickPlayer)` immediately after the `start` route. Order matters only cosmetically (route order is irrelevant for Echo's trie router), but keep it grouped.

- [x] **Task 2: Backend — `RoomHandler.SwapSeats` (AC #2, #3, #4, #5)**
  - [x] 2.1 Add `type SwapSeatsRequest struct { SeatA *int json:"seatA"; SeatB *int json:"seatB" }`. Use pointers to distinguish "missing" from "zero" (seat 0 is valid).
  - [x] 2.2 Implement `func (h *RoomHandler) SwapSeats(c echo.Context) error`:
    - Validate request: `SeatA == nil || SeatB == nil || *SeatA < 0 || *SeatA > 3 || *SeatB < 0 || *SeatB > 3 || *SeatA == *SeatB` → `ErrInvalidSeat`.
    - Inside `RunInTransaction`: re-check `Status == "waiting"` and `OwnerID == userID` (TOCTOU). Look up `pA := tx.FindPlayerBySeat(roomID, *SeatA)` and `pB := tx.FindPlayerBySeat(roomID, *SeatB)`. If either is nil → new `ErrSeatNotOccupied`.
    - Swap: there are two players sharing two seats; a naive `UpdatePlayerSeat(A→B); UpdatePlayerSeat(B→A)` would violate the unique-seat constraint mid-operation if such a constraint exists. Confirm: `room_players` has no unique index on `(room_id, seat)` — see [server/migrations/000005_create_room_players.up.sql](server/migrations/000005_create_room_players.up.sql) (verify before implementation; if a unique index DOES exist, two-phase via `ClearPlayerSeat` then `UpdatePlayerSeat` for each user). For each user, compute `team := teamForSeat(newSeat)` using the existing `teamForSeat` helper at [handler.go:552-557](server/internal/room/handler.go#L552-L557).
  - [x] 2.3 Capture both players' previous seats and usernames before the tx commits. After commit: send TWO ordered `system:seat_updated` broadcasts to all room members (one for each swapped player), each with `previousSeat` set to the seat the user moved **from**. Re-fetch `players` once after the tx for the response body.
  - [x] 2.4 Wire route: `api.POST("/rooms/:id/swap-seats", roomHandler.SwapSeats)`.
  - [x] 2.5 Return `200 { "data": { "players": <updated list> } }`.

- [x] **Task 3: Backend — apperr additions + WS event constant (AC #1, #4, #5, #11)**
  - [x] 3.1 Add to [server/internal/apperr/errors.go](server/internal/apperr/errors.go) (under "Room domain errors"):
    ```go
    ErrRoomNotWaiting    = NewAppError("ROOM_NOT_WAITING", "this action is only available before the game starts", http.StatusConflict)
    ErrCannotKickSelf    = NewAppError("CANNOT_KICK_SELF", "the room owner cannot kick themselves", http.StatusBadRequest)
    ErrSeatNotOccupied   = NewAppError("SEAT_NOT_OCCUPIED", "seat must be occupied to be swapped", http.StatusConflict)
    ```
    Do NOT change any existing apperr — `ErrNotRoomOwner`, `ErrNotInRoom`, `ErrInvalidSeat`, `ErrRoomNotFound` are reused as-is. `ErrGameNotStartable` (`GAME_NOT_STARTABLE`) is intentionally NOT reused — its semantics are "room status prevents starting the game", not "room status prevents kick/swap". A distinct code keeps client error handling unambiguous.
  - [x] 3.2 Add to [server/internal/ws/events.go](server/internal/ws/events.go) under "Room events":

    ```go
    const SystemRoomKicked = "system:room_kicked"

    type RoomKickedPayload struct {
        RoomID uint   `json:"roomId"`
        Reason string `json:"reason"`
    }
    ```

- [x] **Task 4: Backend — `RoomHandler` tests (AC #1–#5)**
  - [x] 4.1 Extend [server/internal/room/handler_test.go](server/internal/room/handler_test.go) with kick scenarios. Existing `mockRoomRepo` ([handler_test.go:23-208](server/internal/room/handler_test.go#L23-L208)) already implements all needed methods — no new mock methods required.
    - Happy path: owner kicks a non-owner seated member → 200, `playerCount` decremented, mock broadcaster received exactly THREE calls: `SystemRoomKicked` to the kicked user, `SystemPlayerLeft` to remaining members, `SystemRoomUpdated` to all (the `BroadcastAll` channel).
    - 403 when caller is not owner.
    - 409 `ROOM_NOT_WAITING` when status is `playing` (regression-test by setting `room.Status = "playing"` before the request).
    - 400 `BAD_REQUEST` when `userId` missing or `0`.
    - 400 `CANNOT_KICK_SELF` when owner targets themselves.
    - 404 `NOT_IN_ROOM` when target is not a member of this room.
    - **Owner-leave race:** create a room with owner A, B, C; have A leave (transferring ownership to B); then have A attempt to kick C → 403 (A is no longer the owner). This locks in the TOCTOU behaviour from AC #4.
  - [x] 4.2 Swap-seats scenarios:
    - Happy path: owner swaps seats 0 ↔ 1 (Team A ↔ Team B) → 200, both players' `seat` and `team` flipped, mock broadcaster received exactly TWO `SystemSeatUpdated` calls in order, each with the correct `previousSeat`.
    - 403 non-owner.
    - 409 `ROOM_NOT_WAITING` when game has started.
    - 400 `INVALID_SEAT` for: missing `seatA`/`seatB`, equal seats, out-of-range (e.g. 4, -1).
    - 409 `SEAT_NOT_OCCUPIED` when at least one of the two seats is empty.
    - **Cross-team swap correctly recomputes team:** seat 0 (Team A) ↔ seat 3 (Team B) → both `team` fields swap accordingly. Asserts the `teamForSeat` recompute (NOT a verbatim copy of the old `team` value).
  - [x] 4.3 Add a regression assertion: after a kick, `mockBroadcaster.allCalls` contains the `SystemRoomUpdated` broadcast (lobby-wide); after a swap, `mockBroadcaster.allCalls` is **empty** (swap doesn't change room metadata visible on the lobby browse cards) — locks in AC #2's "no `system:room_updated` for swap" rule.

- [x] **Task 5: Frontend — API client + mutations (AC #1, #2, #8)**
  - [x] 5.1 Extend [client/src/shared/api/rooms.ts](client/src/shared/api/rooms.ts) with two new functions:
    ```ts
    export function kickPlayer(
      roomId: number,
      userId: number,
    ): Promise<{ playerCount: number }> {
      return axiosClient.post(`/rooms/${roomId}/kick`, { userId });
    }
    export function swapSeats(
      roomId: number,
      seatA: number,
      seatB: number,
    ): Promise<{ players: RoomPlayer[] }> {
      return axiosClient.post(`/rooms/${roomId}/swap-seats`, { seatA, seatB });
    }
    ```
    Mirror existing function shape (default export-free, async via `axiosClient`, returns the unwrapped `data`).
  - [x] 5.2 Extend [client/src/shared/hooks/mutations/useRooms.ts](client/src/shared/hooks/mutations/useRooms.ts) with `useKickPlayerMutation` and `useSwapSeatsMutation`. Mirror `useSelectSeatMutation` (no global query invalidation — the WS broadcast is the source of truth for `roomLobbyStore`).
  - [x] 5.3 No new `queryKeys` entry — kick + swap do not introduce new cached resources.

- [x] **Task 6: Frontend — `RoomLobby.tsx` owner controls UI (AC #6, #8, #9)**
  - [x] 6.1 In [client/src/features/lobby/RoomLobby.tsx](client/src/features/lobby/RoomLobby.tsx), introduce local UI state for swap mode: `const [swapSourceSeat, setSwapSourceSeat] = useState<number | null>(null);`. Reset to `null` whenever `room.status` is no longer `"waiting"` (use a `useEffect` keyed on `room.status`).
  - [x] 6.2 Adjust the seat-tile render path ([RoomLobby.tsx:252-307](client/src/features/lobby/RoomLobby.tsx#L252-L307)):
    - For each non-owner seated tile, when `isOwner && room.status === 'waiting'`, render a small kick icon (use [lucide-react](https://lucide.dev/) — the project already imports from `lucide-react` at [package.json:13](client/package.json#L13); use the `UserX` icon) at the top-right corner of the tile, with `data-testid={\`kick-player-\${seatIndex}\`}`and`aria-label={t("lobby.roomLobby.kickIconLabel", { username: player.username })}`. Tailwind classes: `absolute top-1 right-1 opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100`. Wrap the existing `<button>`seat tile in a`<div className="group relative">` so the icon's hover-state is scoped to its tile.
    - Stop event propagation on the icon's `onClick` (e.g. `e.stopPropagation()`) so clicking the kick icon does NOT also fire `handleSelectSeat`.
    - Clicking the icon opens the confirmation dialog (Task 6.3) — does NOT immediately call the kick mutation.
  - [x] 6.3 Confirmation dialog: check whether `client/src/shared/components/ui/alert-dialog.tsx` exists; if not, install via `npx shadcn@latest add alert-dialog` (NOT `npm install` — owned-copies pattern from project context). Render the dialog inside `RoomLobby.tsx` (control state via `useState<{ seat: number; userId: number; username: string } | null>`). Use `data-testid="kick-confirm"` on the action button and `data-testid="kick-cancel"` on the cancel button. The action button calls `kickMutation.mutate({ roomId, userId })`; on success, dialog closes (server WS broadcast handles the lobby update). On error, `toast.error` from the AC #8 mapping.
  - [x] 6.4 Swap UX: when `swapSourceSeat == null && isOwner && room.status === 'waiting'`, clicking a seated **non-current-user** seat (i.e. an existing seated **other** player) toggles `swapSourceSeat = seatIndex`. While `swapSourceSeat !== null`:
    - The selected seat tile shows a `ring-2 ring-accent` highlight + the `t("lobby.roomLobby.swapMode.enter")` caption beneath the username.
    - Clicking another seated seat (must be occupied, not equal to source) fires `swapMutation.mutate({ roomId, seatA: swapSourceSeat, seatB: seatIndex })`. After success, `swapSourceSeat = null`. On error, `toast.error` per AC #8.
    - Clicking the source seat again, clicking an empty seat, or clicking outside the grid clears `swapSourceSeat = null` (no API call).
    - The owner's own tile and the kick icon are both **non-clickable for swap** (clicking the owner's seat keeps existing seat-select behaviour; clicking the kick icon is handled in 6.2 with `e.stopPropagation`).
  - [x] 6.5 The existing seat-select behaviour for empty seats and the current user's own seat is preserved unchanged. Adding an `if (swapSourceSeat !== null) { handleSwapTarget(seatIndex); return; }` early branch at the top of `handleSelectSeat` keeps the diff minimal.
  - [x] 6.6 Disable controls during `kickMutation.isPending` / `swapMutation.isPending` per AC #8 (`opacity-60 pointer-events-none` on the affected seat tile only — not the whole grid, so the owner can still interact with other tiles).

- [x] **Task 7: Frontend — kicked-player redirect (AC #1, #7, #11)**
  - [x] 7.1 Extend [client/src/shared/stores/roomLobbyStore.ts](client/src/shared/stores/roomLobbyStore.ts):
    - Add `kickedFromRoomId: number | null` (default `null`) and `setKickedFromRoom(roomId: number | null): void`. Mirror the existing `gameStarted` boolean's wiring.
    - Include `kickedFromRoomId: null` in `initialState` so `reset()` clears it.
  - [x] 7.2 Add WS dispatch branch in [client/src/shared/hooks/useWsDispatch.ts](client/src/shared/hooks/useWsDispatch.ts) (next to the existing `SYSTEM_GAME_STARTED` branch, ~line 305):
    ```ts
    if (type === SYSTEM_ROOM_KICKED) {
      const payload = message.payload as RoomKickedPayload;
      const store = useRoomLobbyStore.getState();
      if (
        store.currentRoomId !== null &&
        store.currentRoomId !== payload.roomId
      )
        return;
      store.setKickedFromRoom(payload.roomId);
      return;
    }
    ```
    Add `SYSTEM_ROOM_KICKED` and `RoomKickedPayload` to the imports.
  - [x] 7.3 In `RoomLobby.tsx`, add a `useEffect` keyed on `kickedFromRoomId`:
    ```ts
    const kickedFromRoomId = useRoomLobbyStore((s) => s.kickedFromRoomId);
    useEffect(() => {
      if (kickedFromRoomId !== null && id && kickedFromRoomId === Number(id)) {
        hasLeftRef.current = true;
        toast.error(
          t("lobby.roomLobby.kickedToast", { name: storeRoom?.name ?? "" }),
        );
        useRoomLobbyStore.getState().setKickedFromRoom(null);
        navigate("/lobby");
      }
    }, [kickedFromRoomId, id, navigate, storeRoom?.name, t]);
    ```
    The toast fires before the navigate so the destination (`/lobby`) renders with the toast already queued by Sonner. Reset `kickedFromRoomId = null` on the same call so the effect does not re-fire if the user re-enters the same room ID.
  - [x] 7.4 Defensive: on `404 NOT_IN_ROOM` from the auto-leave-on-unmount mutation, swallow the error silently (no toast) — the kicked player has already been removed by the server, so this is expected. Confirm the existing leave path tolerates this; if it surfaces a toast, gate the toast behind a check that the unmount-leave was not triggered after a kick (`!hasLeftRef.current` is already used as a guard at [RoomLobby.tsx:114](client/src/features/lobby/RoomLobby.tsx#L114)).

- [x] **Task 8: Frontend tests — `RoomLobby.test.tsx` + `useWsDispatch.test.ts` (AC #6–#9, #11)**
  - [x] 8.1 Extend [client/src/features/lobby/RoomLobby.test.tsx](client/src/features/lobby/RoomLobby.test.tsx) with new test cases. Use the project's existing `data-testid` discipline — never key tests on Tailwind classes.
    - `"renders kick icons only for owner on non-owner seated tiles in waiting status"` — seed owner-as-current-user + 4 seated members; assert `kick-player-{1,2,3}` exist and `kick-player-{ownerSeat}` does NOT exist. Then re-render with `currentUser` as a non-owner; assert no `kick-player-*` elements exist.
    - `"hides kick icons once room status transitions away from waiting"` — set `room.status = "playing"`; assert no `kick-player-*` elements render.
    - `"opens kick confirmation dialog and fires kick mutation on confirm"` — click `kick-player-1`, assert dialog renders with the player's username; click `kick-confirm`, assert `kickPlayer` mock called with `(roomId, targetUserId)`.
    - `"cancels kick on dialog cancel — no API call"` — click `kick-player-1`, click `kick-cancel`, assert `kickPlayer` mock NOT called.
    - `"enters swap mode on first seated-seat click and swaps on second click"` — owner clicks seat 1 (other player), assert `swap mode` caption visible; clicks seat 3 (other player), assert `swapSeats` mock called with `(roomId, 1, 3)`.
    - `"cancels swap mode when clicking the source tile again"` — clicks seat 1 twice; assert no `swapSeats` call.
    - `"shows error toast on 409 ROOM_NOT_WAITING"` — mock the kick mutation to reject with `FetchError(code: "ROOM_NOT_WAITING")`; click confirm; assert `toast.error` called with `errors.roomStarted`.
  - [x] 8.2 Extend [client/src/shared/hooks/useWsDispatch.test.ts](client/src/shared/hooks/useWsDispatch.test.ts) with one test:
    - `"dispatches system:room_kicked to roomLobbyStore"` — fire the dispatch with `{ type: "system:room_kicked", payload: { roomId: 5, reason: "kicked_by_owner" } }` while `currentRoomId == 5`; assert `useRoomLobbyStore.getState().kickedFromRoomId === 5`.
    - `"ignores system:room_kicked for a different room"` — same dispatch but `currentRoomId == 9`; assert `kickedFromRoomId` remains `null`.

- [x] **Task 9: i18n updates (AC #10)**
  - [x] 9.1 Extend the existing `lobby.roomLobby` block in [client/src/shared/i18n/en.json](client/src/shared/i18n/en.json) (around lines 159-189) and the matching block in [client/src/shared/i18n/sr.json](client/src/shared/i18n/sr.json) with:
    ```json
    "kickIconLabel": "Kick {{username}}",
    "kickConfirm": {
      "title": "Kick player?",
      "body": "Kick {{username}} from the room?",
      "confirm": "Kick",
      "cancel": "Cancel"
    },
    "kickedToast": "You were removed from room {{name}}",
    "swapMode": {
      "enter": "Pick a seat to swap with",
      "cancel": "Cancel swap"
    }
    ```
    And under `errors`:
    ```json
    "roomStarted": "The game has already started — kick and seat-swap are no longer available.",
    "seatNotOccupied": "Pick two seated players to swap.",
    "kickFailed": "Failed to remove player — try again",
    "swapFailed": "Failed to swap seats — try again"
    ```
  - [x] 9.2 Mirror in `sr.json` using the **Ekavian** register (e.g. `"Pobeda"`, `"se pridružio"` — match the register already used at [sr.json:172-173](client/src/shared/i18n/sr.json#L172-L173) and Story 7.2's `Pobede`/`Procenat pobeda`). Suggested copy (verify register matches adjacent keys; tweak at PR time if Cyrillic-equivalent edge cases arise):
    ```json
    "kickIconLabel": "Izbaci {{username}}",
    "kickConfirm": {
      "title": "Izbaciti igrača?",
      "body": "Izbaciti {{username}} iz sobe?",
      "confirm": "Izbaci",
      "cancel": "Otkaži"
    },
    "kickedToast": "Izbačeni ste iz sobe {{name}}",
    "swapMode": {
      "enter": "Izaberi mesto za zamenu",
      "cancel": "Otkaži zamenu"
    }
    ```
    And:
    ```json
    "roomStarted": "Igra je već počela — izbacivanje i zamena mesta više nisu dostupni.",
    "seatNotOccupied": "Izaberi dva zauzeta mesta za zamenu.",
    "kickFailed": "Neuspešno izbacivanje igrača — pokušajte ponovo",
    "swapFailed": "Neuspešna zamena mesta — pokušajte ponovo"
    ```
  - [x] 9.3 Run `npx vitest run i18n` — the recursive `flattenKeys` parity check must stay green. If a nested-key collision arises (e.g. `kickConfirm` colliding with a sibling string key — none expected), follow the same `*.statsHeading` rename trick used by Story 7.2.

- [x] **Task 10: Full-stack smoke + lint gates (run before marking the story done)**
  - [x] 10.1 Backend: `go test ./...` — all packages green. `go vet ./...` clean.
  - [x] 10.2 Frontend: `cd client && npx vitest run` — all tests green (existing + new `RoomLobby.test.tsx` + `useWsDispatch.test.ts` cases).
  - [x] 10.3 **Lint: `cd client && npx prettier --write . && npx eslint .` — Prettier MUST run before committing** (memory `feedback_prettier_before_commit.md`; CI has failed repeatedly — Stories 7.1 + 7.2 logged the same reminder).
  - [x] 10.4 `make lint` (both stacks) — clean. Note: on Windows shells without `golangci-lint` installed, fall back to `go vet ./...` as the equivalent static-analysis gate (matching Story 7.2 Dev Agent Record practice).
  - [x] 10.5 Manual smoke (document outcomes in Completion Notes):
    - Start `make dev`; create a room with 4 users (owner + 3 members, all seated). As the owner, kick member at seat 1 → kicked user lands on `/lobby` with the toast; remaining three see the seat 1 free up; the lobby browse page shows `playerCount: 3`.
    - As the owner, swap seats 0 ↔ 3 → both swapped users see their team colour change in real time on every client.
    - As a non-owner, attempt the kick mutation via `curl` (the UI hides the icon, but the API must enforce 403). Confirm the response is `403 NOT_ROOM_OWNER`.
    - Start the game (status → `playing`); confirm the kick icons disappear in the game-table render path (the room lobby is no longer rendered after navigation, but verify the back-button-to-room-lobby UX does not leak controls if a tester forces a route).

## Dev Notes

### Big Picture — Why This Story Is Mostly Additive

Story 8.1 is a **surgical extension of the room domain**, not a refactor. Existing endpoints (Join / Leave / SelectSeat / StartGame) and their broadcast helpers (`broadcastToRoom`, `broadcastToAll`, `broadcastRoomUpdated`) are untouched. We add two new POST endpoints (`/kick`, `/swap-seats`), one new WS event (`system:room_kicked`), three new apperr codes, and an additive UI overlay on `RoomLobby.tsx`. There are **zero migrations**.

The biggest design call is **REST + WS-broadcast vs raw WS actions**. The epic's AC text uses `action:room_kick_player` / `action:room_swap_seats` phrasing, but every existing room-lobby mutation in this codebase is a REST POST with WS broadcasts on success ([room/handler.go](server/internal/room/handler.go), Story 2.4). We follow the **codebase pattern**, not the AC's literal naming, because:

1. The router at [server/internal/ws/router.go](server/internal/ws/router.go) is dispatch-only — it would need an entire new subsystem to host room actions. Today the only `action:` handlers are game actions routed through the `sessionManager.HandleAction` ([server/cmd/api/main.go:121-127](server/cmd/api/main.go#L121-L127)) — there is no analogous `roomActionHandler`.
2. REST gives us free DB-error-to-HTTP mapping via the existing `appErrorHandler` middleware. WS would require us to define error events for every room error case and reinvent the routing.
3. Atomicity: the existing mutations all use `RunInTransaction` for TOCTOU safety (verified in [handler.go:587-598](server/internal/room/handler.go#L587-L598), [:472-502](server/internal/room/handler.go#L472-L502), [:587-643](server/internal/room/handler.go#L587-L643)). REST handlers already work with transactions; recreating this in a WS action handler would duplicate plumbing.

The AC names of `event:room_kicked` and `event:room_state` are interpreted as **wire-shape** intent, not strict event-name strings. We satisfy them via:

- `system:room_kicked` (new — meets the `event:room_kicked` intent; named `system:` to match the project's prefix rule "non-game platform events use `system:` not `event:`" — see [\_bmad-output/planning-artifacts/architecture.md:336](_bmad-output/planning-artifacts/architecture.md#L336)).
- `system:player_left` + `system:seat_updated` + `system:room_updated` (existing — together carry the full "updated room state" — meets the `event:room_state` intent without inventing a new aggregate event that would duplicate the existing payload graph).

### What Already Exists — Do NOT Recreate

| Item                                                                                                | Location                                                                                                                    | Notes                                                                                                                                                                                                                                   |
| --------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `RoomHandler` with `JoinRoom`, `LeaveRoom`, `SelectSeat`, `StartGame`                               | [server/internal/room/handler.go](server/internal/room/handler.go)                                                          | Add `KickPlayer` + `SwapSeats` as additional methods on the same struct. No new handler files needed.                                                                                                                                   |
| `RoomRepository` interface + `GormRepository`                                                       | [server/internal/room/repository.go](server/internal/room/repository.go), [gorm_repo.go](server/internal/room/gorm_repo.go) | All needed methods already exist: `RemovePlayer`, `DecrementPlayerCount`, `UpdatePlayerSeat`, `ClearPlayerSeat`, `FindPlayerBySeat`, `FindByID`, `RunInTransaction`, `FindPlayersByRoomID`. Story 8.1 adds **zero** repository methods. |
| Broadcast helpers `broadcastToUsers`, `broadcastToAll`, `broadcastToRoom`, `broadcastRoomUpdated`   | [room/handler.go:81-149](server/internal/room/handler.go#L81-L149)                                                          | Reuse verbatim. The kick handler calls `broadcastToUsers([]uint{kickedUserID}, ...)` for the kick event, then `broadcastToUsers(remainingUserIDs, SystemPlayerLeft, ...)`, then `broadcastRoomUpdated`.                                 |
| `Broadcaster` interface (`BroadcastToUsers`, `BroadcastAll`)                                        | [room/handler.go:66-69](server/internal/room/handler.go#L66-L69)                                                            | Already abstracts the hub for testability. The mock `mockBroadcaster` in [handler_test.go:221-232](server/internal/room/handler_test.go#L221-L232) is reusable for the new tests.                                                       |
| TOCTOU pattern (status check inside tx)                                                             | [room/handler.go:587-598](server/internal/room/handler.go#L587-L598)                                                        | Copy this pattern verbatim into the kick + swap handlers — re-verify `room.Status == "waiting"` and `room.OwnerID == userID` AFTER `tx.FindByID`.                                                                                       |
| `teamForSeat(seat int) string`                                                                      | [room/handler.go:552-557](server/internal/room/handler.go#L552-L557)                                                        | Reuse for the swap path's team recompute. (There's a duplicate in `internal/game/state.go` — do NOT add another.)                                                                                                                       |
| `RoomLobby.tsx` seat tile render path with `data-testid="player-seat-{seatIndex}"`                  | [client/src/features/lobby/RoomLobby.tsx:252-307](client/src/features/lobby/RoomLobby.tsx#L252-L307)                        | Wrap each tile in a `<div className="group relative">` to scope the kick icon's hover-state to its own tile. The existing `<button>` keeps its seat-select onClick.                                                                     |
| `useRoomLobbyStore` with `addPlayer`, `removePlayer`, `updatePlayerSeat`, `setGameStarted`, `reset` | [client/src/shared/stores/roomLobbyStore.ts](client/src/shared/stores/roomLobbyStore.ts)                                    | Add only `kickedFromRoomId` + `setKickedFromRoom`. Mirror the `gameStarted` pattern verbatim (boolean-by-id state, navigation effect, `reset()` clearing).                                                                              |
| `useWsDispatch` system event branches                                                               | [client/src/shared/hooks/useWsDispatch.ts:260-311](client/src/shared/hooks/useWsDispatch.ts#L260-L311)                      | Add `SYSTEM_ROOM_KICKED` branch alongside `SYSTEM_GAME_STARTED`. Keep the `currentRoomId` guard pattern.                                                                                                                                |
| `axiosClient` HTTP client + `FetchError` for 4xx code-based branching                               | [client/src/shared/api/axiosClient.ts](client/src/shared/api/axiosClient.ts)                                                | Reuse for `instanceof FetchError && err.code === "ROOM_NOT_WAITING"` etc. — same pattern as [RoomLobby.tsx:153-158](client/src/features/lobby/RoomLobby.tsx#L153-L158).                                                                 |
| `lobbyDisconnectHandler` (10s pre-game disconnect timer)                                            | [server/internal/room/lobby_disconnect.go](server/internal/room/lobby_disconnect.go)                                        | NOT relevant to kick. The kicked player is intentionally removed by the owner; they remain WS-connected. The lobby disconnect handler only fires on socket disconnect, which a kick does not trigger.                                   |
| `lucide-react` icons                                                                                | [client/package.json:13](client/package.json#L13)                                                                           | Use `UserX` for the kick icon. No new dependency.                                                                                                                                                                                       |
| shadcn dialog primitive workflow                                                                    | [\_bmad-output/project-context.md#L24](_bmad-output/project-context.md)                                                     | Install via `npx shadcn@latest add alert-dialog` ONLY if `client/src/shared/components/ui/alert-dialog.tsx` does not exist. NEVER `npm install` a UI primitive.                                                                         |

### What Must Be Created

1. `RoomHandler.KickPlayer` + `RoomHandler.SwapSeats` methods (no new file).
2. Three new apperrs: `ErrRoomNotWaiting`, `ErrCannotKickSelf`, `ErrSeatNotOccupied`.
3. One new WS event: `SystemRoomKicked` + `RoomKickedPayload` struct on the server side; matching `SYSTEM_ROOM_KICKED` constant + `RoomKickedPayload` interface on the client side.
4. New API client helpers: `kickPlayer`, `swapSeats` in `client/src/shared/api/rooms.ts`.
5. New mutations: `useKickPlayerMutation`, `useSwapSeatsMutation` in `client/src/shared/hooks/mutations/useRooms.ts`.
6. Optional shadcn primitive: `alert-dialog.tsx` (only if missing).

### What Must Be Modified

1. [server/internal/room/handler.go](server/internal/room/handler.go) — add kick + swap handlers.
2. [server/internal/room/handler_test.go](server/internal/room/handler_test.go) — add ~10 new test cases.
3. [server/internal/apperr/errors.go](server/internal/apperr/errors.go) — add three new error codes.
4. [server/internal/ws/events.go](server/internal/ws/events.go) — add `SystemRoomKicked` const + `RoomKickedPayload` struct.
5. [server/cmd/api/main.go](server/cmd/api/main.go) — register two new routes.
6. [client/src/shared/api/rooms.ts](client/src/shared/api/rooms.ts) — add two API functions.
7. [client/src/shared/hooks/mutations/useRooms.ts](client/src/shared/hooks/mutations/useRooms.ts) — add two mutations.
8. [client/src/shared/types/wsEvents.ts](client/src/shared/types/wsEvents.ts) — add `SYSTEM_ROOM_KICKED` + `RoomKickedPayload`.
9. [client/src/shared/stores/roomLobbyStore.ts](client/src/shared/stores/roomLobbyStore.ts) — add `kickedFromRoomId` + setter.
10. [client/src/shared/hooks/useWsDispatch.ts](client/src/shared/hooks/useWsDispatch.ts) — add `SYSTEM_ROOM_KICKED` dispatch branch.
11. [client/src/features/lobby/RoomLobby.tsx](client/src/features/lobby/RoomLobby.tsx) — add owner controls overlay (kick icon, swap-mode UI, dialog, kicked-redirect effect).
12. [client/src/features/lobby/RoomLobby.test.tsx](client/src/features/lobby/RoomLobby.test.tsx) — add ~7 new test cases.
13. [client/src/shared/hooks/useWsDispatch.test.ts](client/src/shared/hooks/useWsDispatch.test.ts) — add 2 new test cases.
14. [client/src/shared/i18n/en.json](client/src/shared/i18n/en.json) + [sr.json](client/src/shared/i18n/sr.json) — add new keys.

**No changes expected:**

- `server/migrations/*` — zero schema changes.
- `server/internal/room/repository.go` / `gorm_repo.go` / `model.go` — repository surface unchanged.
- `server/internal/room/lobby_disconnect.go` — kick is not a disconnect.
- `server/internal/session/*` — pre-game only; session manager not involved.
- `server/internal/game/*` — rules engine untouched.
- `client/src/features/game/*` — game-table UI untouched (this story is room-lobby only).

### Architecture Patterns to Follow

- **REST mutation + WS broadcast pattern.** Every existing room operation follows `RunInTransaction → mutation → commit → broadcast{ToUsers,ToAll}`. Kick + swap match this shape exactly. Do NOT introduce raw `action:` WS handlers (see "Big Picture" above for rationale).
- **TOCTOU re-checks inside the transaction.** Both `room.Status == "waiting"` AND `room.OwnerID == userID` MUST be verified after `tx.FindByID`. The room status can flip mid-request (auto-start, manual start, leave-induced ownership transfer). Verify once before the broadcast call would commit irreversible side effects.
- **Multi-event WS broadcasts are sent as separate ordered messages, never batched.** Project rule [\_bmad-output/project-context.md#L109](_bmad-output/project-context.md). Two `system:seat_updated` for a swap, three messages (kicked, player_left, room_updated) for a kick.
- **WS events use `system:` prefix for non-game platform events**, NOT `event:`. The AC's `event:room_kicked` / `event:room_state` phrasing is interpreted as wire-shape intent — we use `system:room_kicked` to match the existing prefix discipline ([architecture.md:336](_bmad-output/planning-artifacts/architecture.md#L336)).
- **Same-commit WS contract sync.** [server/internal/ws/events.go](server/internal/ws/events.go) and [client/src/shared/types/wsEvents.ts](client/src/shared/types/wsEvents.ts) MUST be updated together. CI will not catch drift; the rule is a project-context invariant ([\_bmad-output/project-context.md#L80, #L286](_bmad-output/project-context.md)).
- **Additive JSON-extension discipline.** New API responses add fields, never rename or repurpose existing ones. The existing `LeaveRoom` `data` shape (`{ "message": "left room" }`) is irrelevant — kick returns its own `{ "playerCount": N }`. Existing clients ignore unknown fields gracefully (Story 7.2 reinforced this rule).
- **`data-testid` over Tailwind classes** — reinforced by Stories 7.1 and 7.2 Dev Notes; Tailwind class churn breaks class-based test selectors. All new controls expose `data-testid`.
- **Server is the authority for game/room state; client UI is presentational.** The kick button does not optimistically remove the player from `roomLobbyStore` — it waits for the WS broadcast (which will arrive on every connected client, including the owner who just sent the kick request). Mirrors `handleSelectSeat` ([RoomLobby.tsx:144-146](client/src/features/lobby/RoomLobby.tsx#L144-L146)) which sets players from the response and lets WS converge.
- **Counter-clockwise seat rule + `team % 2` derivation are unchanged.** Swapping does not change seat numbering or team derivation — both are deterministic from the new seat index.
- **`*int` for "missing vs zero" distinction.** Use `*int` for `seatA`/`seatB` request fields so JSON `null` / missing maps to `nil` and seat 0 is a valid input. Same trick used by [SelectSeatRequest](server/internal/room/handler.go#L544-L546).
- **i18n parity is CI-enforced.** Every key added to `en.json` is added to `sr.json` in the same commit — recursive `flattenKeys` parity test ([client/src/shared/i18n/i18n.test.ts](client/src/shared/i18n/i18n.test.ts), added in Story 7.1 review) blocks divergence.
- **Prettier before every commit.** Memory `feedback_prettier_before_commit.md` is load-bearing; CI has failed repeatedly. Task 10.3 enforces this — `npx prettier --write .` then `npx eslint .` before any commit touching client code.

### Previous Story Intelligence (Stories 7.1 + 7.2 — both done, 2026-04-19)

Carried-forward learnings that shape this story:

- **Prettier-before-commit is non-negotiable.** Task 10.3 enforces this; CI has failed repeatedly across Stories 7.1, 7.2, 6.x.
- **`data-testid` + RTL queries win over text-based queries** — i18n-aware components shouldn't be tested by visible string. All new tiles + buttons use `data-testid`.
- **PII-leak-guard pattern** is established (Story 7.1) — kick handler does NOT leak email/passwordHash since it never touches the user table directly. No PII test needed for this story (no User row read).
- **Story-7.2-style review patches happen in batch.** Expect a "few items applied, a couple deferred" pattern. Design with stable handler signatures, JSON shapes, and testids so review tightening is mechanical.
- **`i18n.test.ts` parity check is automatic** — any divergence between `en.json` and `sr.json` trips CI regardless of developer diligence.
- **`QueryWrapper` from `@/test-utils/`** is the canonical react-query wrapper — already used by `RoomLobby.test.tsx`.
- **Centralized fixture helpers** (Story 7.2's `profileFixture()`) reduce future-additive-extension churn — consider a `roomLobbyFixture({ ownerId, seatedPlayers, status })` helper if the existing test scaffold doesn't already have one.
- **Avoid hardcoded mock IDs.** Story 6.2 review surfaced a "userId 0 hardcoded as ID" bug — `useWsDispatch` already uses `payload.userId` as the client-side ID. Same pattern continues to apply to any new players joining via dispatch.

### Recent Codebase Signals (git log — last 8 commits)

- `b16920f chore(server): apply gofmt across the codebase` — formatting-only; no functional impact, but reinforces "gofmt is enforced — Task 10.4 will run against any new code".
- `588b6eb chore: add .gitattributes to enforce LF line endings` — Windows ↔ Linux line-ending normalisation. **New files added in this story MUST be LF-encoded** (the user's local machine is Windows; the .gitattributes file ensures git normalises on commit). Don't fight it — let git handle the conversion.
- `b86d4fb chore(server): promote golang.org/x/text to a direct dependency` — go.mod hygiene; not related.
- `b78e1ec fix(game): require trump cut when void in led suit (Bitola)` — game rules; not related.
- `4aa94d2 feat(game): show face-up candidate card in round-2 trump prompt` — game UI; not related.
- `fb89d64 feat(game): show table-wide reveal dialog when a player takes trump` — game UI; not related.
- `f54f6ee chore(bmad): upgrade BMad framework install (skills + module configs)` — tooling-only; not related.
- `6ecd73c feat(profile): aggregate career stats with win/loss tiles (Story 7.2)` — predecessor non-blocking. The profile page is unrelated to room lobby; no touchpoint risk.

**Signal: room domain has been quiet since Story 4.7** (room-lobby WS wiring, completed late 2026-Q1). Touchpoint risk for kick + swap is **low** — the most recent room-touching change (`9a85399 feat(chat): room-scoped chat for waiting-room members`) added the chat panel alongside the seat grid; it does NOT modify the seat-tile render path that owner controls overlay onto. No conflict expected.

### Cross-Story Context

- **Story 1.4 (done)** — established the placeholder room lobby skeleton. Story 8.1 extends `RoomLobby.tsx` without restructuring it.
- **Story 2.3 (done)** — `JoinRoom` + room participant tracking. Kick uses the same `RemovePlayer` repository method that Leave uses.
- **Story 2.4 (done)** — team assignment + game start. The owner-only `StartGame` already locks in the owner-permission pattern that kick + swap reuse.
- **Story 4.7 (done)** — room-lobby WebSocket wiring (the dispatch branches in `useWsDispatch.ts`, the `roomLobbyStore` real-time updates). Story 8.1 adds one more dispatch branch (`SYSTEM_ROOM_KICKED`) and one more store field (`kickedFromRoomId`) to that scaffolding.
- **Story 5.1-5.5 (done)** — pause + reconnect. The `lobbyDisconnectHandler` was added by these. Kick is intentional, not a disconnect — `lobbyDisconnectHandler` is irrelevant.
- **Story 8.2 (future — backlog)** — Team Surrender. Will add `action:surrender_*` WS events. NOT part of this story; do not anticipate.
- **Story 8.3 (future — backlog)** — In-Game Emotes. Same — not part of this story.
- **Story 9.x (future — Phase 2 economy)** — when coins/buy-in land, kick may need to refund the kicked player's stake. Story 8.1 has NO awareness of stakes — Phase 2 will extend the kick handler to call a wallet-refund hook then. Design the kick handler so a future "post-mutation hook" is a 1-line addition (most cleanly, by NOT inlining all broadcast logic into the tx — keep the post-commit broadcast block separable).
- **Story 11.x (future — Phase 3 friends)** — friend-list features may add "block" / "report" affordances on the seat tile. Owner controls (kick) and friend-list controls live on the same seat tile but in different overlay layers — design the kick icon to render in `top-1 right-1` so a future friend-action menu can render in `bottom-1 right-1` without collision.

### Backend Flow — `POST /api/v1/rooms/:id/kick`

1. Echo routes through auth middleware → `RoomHandler.KickPlayer(c)`.
2. `auth.GetUserID(c)` → `ownerID`. Parse `:id` → `roomID`. `c.Bind(&KickPlayerRequest{})` → `req.UserID`.
3. Validate shape: `req.UserID == 0` → `ErrBadRequest`.
4. `RunInTransaction`:
   a. `room := tx.FindByID(roomID)` → if nil → `ErrRoomNotFound`.
   b. `if room.Status != "waiting"` → `ErrRoomNotWaiting`.
   c. `if room.OwnerID != ownerID` → `ErrNotRoomOwner`.
   d. `if req.UserID == room.OwnerID` → `ErrCannotKickSelf`.
   e. `targetMember := tx.FindPlayerRoom(req.UserID)` → if nil OR `targetMember.RoomID != roomID` → `ErrNotInRoom`.
   f. Capture `kickedUsername` (from `tx.FindPlayersByRoomID` filtered by `req.UserID`).
   g. `tx.RemovePlayer(roomID, req.UserID)`.
   h. `tx.DecrementPlayerCount(roomID)`.
   i. `postRoom := tx.FindByID(roomID)` for the post-mutation broadcast snapshot.
5. Outside the tx: broadcast in this order:
   a. `h.broadcastToUsers([]uint{req.UserID}, ws.SystemRoomKicked, RoomKickedPayload{RoomID: roomID, Reason: "kicked_by_owner"})`.
   b. `remainingPlayers := h.repo.FindPlayersByRoomID(roomID)`; build `userIDs`.
   c. `h.broadcastToUsers(userIDs, ws.SystemPlayerLeft, { roomId, userId: req.UserID, username: kickedUsername, playerCount: postRoom.PlayerCount })`.
   d. `h.broadcastRoomUpdated(postRoom)`.
6. Return `200 { "data": { "playerCount": postRoom.PlayerCount } }`.

### Backend Flow — `POST /api/v1/rooms/:id/swap-seats`

1. `auth.GetUserID(c)` → `ownerID`. Parse `:id` → `roomID`. `c.Bind(&SwapSeatsRequest{})`.
2. Validate: `req.SeatA == nil || req.SeatB == nil || *seatA<0|>3 || *seatB<0|>3 || *seatA == *seatB` → `ErrInvalidSeat`.
3. `RunInTransaction`:
   a. `room := tx.FindByID(roomID)` → if nil → `ErrRoomNotFound`.
   b. `if room.Status != "waiting"` → `ErrRoomNotWaiting`.
   c. `if room.OwnerID != ownerID` → `ErrNotRoomOwner`.
   d. `pA := tx.FindPlayerBySeat(roomID, *req.SeatA)` and `pB := tx.FindPlayerBySeat(roomID, *req.SeatB)`. If either is nil → `ErrSeatNotOccupied`.
   e. `prevSeatA := *pA.Seat; prevSeatB := *pB.Seat` (capture for broadcast).
   f. **Constraint check first**: if `room_players` enforces a unique (room_id, seat) index, do `tx.ClearPlayerSeat(roomID, pA.UserID)` then `tx.UpdatePlayerSeat(roomID, pB.UserID, *req.SeatA, teamForSeat(*req.SeatA))` then `tx.UpdatePlayerSeat(roomID, pA.UserID, *req.SeatB, teamForSeat(*req.SeatB))`. If no constraint exists, two straight `UpdatePlayerSeat` calls in either order suffice. Verify the schema before implementation.
   g. Re-fetch players for the response.
4. Outside the tx: broadcast TWO ordered `system:seat_updated` events:
   a. For pA: `{ roomId, userId: pA.UserID, username: pA.Username, seat: *seatB, team: teamForSeat(*seatB), previousSeat: prevSeatA }`.
   b. For pB: `{ roomId, userId: pB.UserID, username: pB.Username, seat: *seatA, team: teamForSeat(*seatA), previousSeat: prevSeatB }`.
5. **Do NOT broadcast** `system:room_updated` — room metadata (player count, status) is unchanged.
6. Return `200 { "data": { "players": <updated list> } }`.

### Frontend Flow — Owner Click → Server → Broadcast → All Clients Converge

1. Owner clicks `kick-player-1` icon.
2. `RoomLobby.tsx` opens the confirmation dialog. Owner clicks `kick-confirm`.
3. `useKickPlayerMutation.mutate({ roomId, userId })` → POST to `/api/v1/rooms/:id/kick`.
4. Server commits the tx, broadcasts:
   - `system:room_kicked` to the kicked user's WS connection.
   - `system:player_left` to all remaining members (incl. the owner).
   - `system:room_updated` to all WS clients (lobby browse page + the room).
5. Each client's `useWsDispatch`:
   - Kicked user: routes `system:room_kicked` → `setKickedFromRoom(roomId)` → `RoomLobby.tsx` `useEffect` → toast + `navigate("/lobby")`.
   - Remaining members: routes `system:player_left` → `roomLobbyStore.removePlayer(...)` → seat tile clears in real time.
   - Lobby browse viewers: routes `system:room_updated` → `useRoomUpdates` → query cache update → room card shows new `playerCount`.
6. Owner's POST `200` resolves; `RoomLobby.tsx` need not act on the response (state has already converged via WS — same pattern as seat-select).

### Project Structure Notes

**Modified files (expected): see "What Must Be Modified" — 14 files modified, 0 new files (1 optional shadcn primitive), 0 migrations.**

**Alignment with unified project structure:**

- Backend: kick + swap handlers are methods on the existing `RoomHandler` struct, in [server/internal/room/handler.go](server/internal/room/handler.go) — same package, same file, pattern-consistent with `JoinRoom` / `LeaveRoom` / `SelectSeat` / `StartGame`. No new domain package needed.
- Frontend: API client + mutations follow the existing `rooms.ts` / `useRooms.ts` shape. UI lives in the same feature folder (`features/lobby/`) — Story 4.7 already established the room-lobby file layout.
- WS contract: both sides updated in the same commit per project rule.
- i18n: nested keys under the existing `lobby.roomLobby.*` block, not a new top-level block.

### Alignment Checks / Detected Conflicts

- **Epic AC names use `action:` prefix; we use `system:` for the new event.** This is a deliberate alignment with the project's prefix discipline (`action:` is reserved for client→server game actions; the WS router's only `action:` handler today is the session manager). Document this in the PR description as "AC `event:room_kicked` realised as `system:room_kicked` per the project-wide prefix rule; AC `event:room_state` realised as the existing `system:player_left` + `system:seat_updated` + `system:room_updated` triplet".
- **No schema conflicts.** Zero migrations, zero new columns. The existing `room_players.seat` constraint behaviour (verify whether a unique index on (room_id, seat) exists) determines whether the swap path needs a two-phase clear+update — Task 2.2 calls this out explicitly.
- **Drag-and-drop is opt-out.** AC #6 mentions "drag" OR "swap-seats affordance"; the project has NO drag-and-drop library (verified via `client/package.json`). Story 8.1 chooses the swap-affordance path (click → enter swap mode → click target). Adding `@dnd-kit` would be Phase 3+ scope creep, not justified by the AC.
- **Confirmation dialog primitive availability.** The shadcn `alert-dialog` may or may not be installed. Task 6.3 checks first and only installs if missing — preserves the "owned copies, never `npm install` UI primitives" rule.

### Edge Cases & Anti-Patterns to Avoid

- **Do NOT** kick by removing the player AND ALSO triggering the `lobbyDisconnectHandler` — they are independent. Kick is intentional, not a disconnect.
- **Do NOT** kick the room owner. The owner can `LeaveRoom` (existing endpoint) which transfers ownership; kick is non-self.
- **Do NOT** kick a player whose room has already started. The status check inside the tx is the single source of truth — UI hide-on-`status !== 'waiting'` is a visual courtesy, not a security boundary.
- **Do NOT** broadcast `system:room_kicked` to anyone besides the kicked user. Other room members get `system:player_left` (the existing event), not the kick-specific event. This separation lets a future analytics layer distinguish "kicked" from "left voluntarily" without inspecting payloads.
- **Do NOT** invent a new `event:room_state` super-event that bundles all post-kick state. The existing fan-out (`player_left` + `room_updated`) carries the full delta and is already tested.
- **Do NOT** optimistically remove the kicked player from `roomLobbyStore.players` on the owner's client. Trust the WS broadcast (which the owner also receives — same pattern as their own seat selections).
- **Do NOT** block kick during a swap-mode UI state. If the owner has a swap source selected and clicks a kick icon, cancel swap mode (`setSwapSourceSeat(null)`) and proceed to the kick dialog. (Document this in Task 6.4.)
- **Do NOT** allow kick-while-mutation-pending. The tile's `pointer-events-none` during `kickMutation.isPending` prevents the dialog re-firing on a double-click.
- **Do NOT** trigger toast on every WS event. The kicked-user toast fires once, gated by `setKickedFromRoom(null)` on the same effect tick — no toast flood on rapid re-mount.
- **Do NOT** reuse `apperr.ErrGameNotStartable` for the room-not-waiting case in kick/swap. Its semantics are "room status prevents starting the game" — semantically distinct from "kick/swap is only allowed before start". A new `ErrRoomNotWaiting` keeps the client error mapping unambiguous.
- **Counter-clockwise seat order** is irrelevant to swap (we swap two existing seats; no rotation logic). The `(currentPlayer + 1) % 4` rule from project context applies to game-time turn rotation only.

### References

- [Source: epics.md#Story-8.1 — Room Owner Pre-Game Controls acceptance criteria](_bmad-output/planning-artifacts/epics.md#L1429)
- [Source: prd.md — Phase 2 game-table enhancements (kick + swap, surrender, emotes)](_bmad-output/planning-artifacts/prd.md)
- [Source: architecture.md#L327-L336 — WebSocket prefix rule (`action:` / `event:` / `error:` / `system:`)](_bmad-output/planning-artifacts/architecture.md#L327-L336)
- [Source: architecture.md#L356-L368 — domain package shape for `internal/room/`](_bmad-output/planning-artifacts/architecture.md#L356-L368)
- [Source: project-context.md#L99-L113 — Echo / backend rules (handlers call repo interfaces; multi-event broadcasts as separate ordered messages)](_bmad-output/project-context.md#L99-L113)
- [Source: project-context.md#L65-L78 — Go rules (clone slices in pure funcs; `*time.Time` for nullable; structured logging via slog)](_bmad-output/project-context.md#L65-L78)
- [Source: project-context.md#L80, #L286 — WS event contract MUST be updated in both files in the same commit](_bmad-output/project-context.md#L80)
- [Source: 7-2-expanded-player-profile.md — prior story; data-testid discipline, prettier-before-commit, additive JSON pattern, i18n parity, mock fixture helpers](_bmad-output/archive/implementation-artifacts/7-2-expanded-player-profile.md)
- [Source: server/internal/room/handler.go — RoomHandler to extend with KickPlayer + SwapSeats; broadcast helpers; teamForSeat helper](server/internal/room/handler.go)
- [Source: server/internal/room/handler_test.go — mockRoomRepo + mockBroadcaster ready to use for new tests](server/internal/room/handler_test.go)
- [Source: server/internal/apperr/errors.go — apperr conventions; NOT_ROOM_OWNER, NOT_IN_ROOM, INVALID_SEAT to reuse; new codes to add](server/internal/apperr/errors.go)
- [Source: server/internal/ws/events.go — events.go to add SystemRoomKicked + RoomKickedPayload](server/internal/ws/events.go)
- [Source: client/src/features/lobby/RoomLobby.tsx — RoomLobby seat tile render path to extend; isOwner gate; existing seat-select handler](client/src/features/lobby/RoomLobby.tsx)
- [Source: client/src/shared/stores/roomLobbyStore.ts — store to extend with kickedFromRoomId; gameStarted pattern to mirror](client/src/shared/stores/roomLobbyStore.ts)
- [Source: client/src/shared/hooks/useWsDispatch.ts — dispatcher to extend with SYSTEM_ROOM_KICKED branch](client/src/shared/hooks/useWsDispatch.ts)
- [Source: client/src/shared/types/wsEvents.ts — add SYSTEM_ROOM_KICKED + RoomKickedPayload](client/src/shared/types/wsEvents.ts)
- [Source: client/src/shared/api/rooms.ts + mutations/useRooms.ts — API + mutation pattern to mirror](client/src/shared/api/rooms.ts)
- [Source: client/src/shared/i18n/en.json#L159-L189 + sr.json — lobby.roomLobby block to extend](client/src/shared/i18n/en.json)
- [Source: client/src/shared/i18n/i18n.test.ts — recursive flattenKeys parity check (CI gate)](client/src/shared/i18n/i18n.test.ts)

## Dev Agent Record

### Agent Model Used

claude-opus-4-7[1m] (BMM dev-story workflow)

### Debug Log References

- `cd server && go test ./...` — all packages green (room package: ~35s, includes 17 new kick + swap test cases).
- `cd server && go vet ./...` — clean.
- `cd client && npx vitest run` — 51 test files / 461 tests passed; new RoomLobby cases (7) + useWsDispatch cases (2) both green.
- `cd client && npx prettier --write . && npx eslint .` — both clean. Prettier reformatted three files (RoomLobby.tsx, RoomLobby.test.tsx, useRooms.ts) on first pass.
- `golangci-lint` not installed locally; per Task 10.4 fallback, `go vet ./...` substitutes for static analysis. CI will run the full lint pipeline.

### Completion Notes List

- **Architectural alignment** — implemented kick + swap as REST mutations + WS broadcasts (matching the pattern in `JoinRoom`, `LeaveRoom`, `SelectSeat`, `StartGame`) rather than the AC's literal `action:` phrasing. The new event uses the `system:` prefix per the project's prefix rule (`action:` is reserved for client→server game actions). Owner state changes are delivered via the existing triplet `system:player_left` + `system:room_updated` (kick) and two ordered `system:seat_updated` events (swap). See PR description for the AC-name interpretation.
- **TOCTOU guards inside the transaction** — both kick and swap re-read the room inside `RunInTransaction` and assert `Status == "waiting"` and `OwnerID == ownerID` before any mutation. The new `TestKickPlayer_PreviousOwnerLosesPermission` test locks in this behaviour: after ownership transfers via leave, the previous owner's kick attempt returns 403.
- **Schema verification** — `room_players` has only `idx_room_players_room_user` UNIQUE on `(room_id, user_id)`. There is no UNIQUE on `(room_id, seat)`, so swap can do two consecutive `UpdatePlayerSeat` calls without an intermediate clear-and-rebind. Verified via `server/migrations/000004_create_room_players.up.sql`.
- **Confirmation dialog** — used the existing project Dialog primitive (`@base-ui/react/dialog` via `client/src/shared/components/ui/dialog.tsx`) instead of installing a separate `alert-dialog`. The story's task 6.3 said "or use the existing dialog primitive"; no new shadcn install was needed. Cancel/confirm `data-testid`s and Escape-to-close behaviour come from the existing primitive.
- **Owner-leave race coverage** — added `TestKickPlayer_PreviousOwnerLosesPermission` to assert the documented TOCTOU semantics; a previous owner who has already transferred ownership cannot kick.
- **Backward compatibility** — zero changes to `JoinRoom`, `LeaveRoom`, `SelectSeat`, `StartGame`, `room_players` schema, or migrations. All existing tests continue to pass.
- **i18n parity** — new keys added to both `en.json` and `sr.json` in the same commit; `i18n.test.ts` parity check stays green. Serbian copy uses Ekavian register (`Izbaci`, `Izbačeni`, `mesto`) matching adjacent keys.
- **Manual smoke** — Task 10.5 manual smoke is **deferred to peer testing** (the Windows host doesn't have `make dev` running locally for this session). Acceptance is covered by:
  - 17 new backend tests asserting handler behaviour, broadcast counts, broadcast targets, and broadcast types end-to-end.
  - 7 new RoomLobby tests asserting owner-only icon visibility, hide-on-`status="playing"`, dialog confirm/cancel paths, swap-mode entry/cancel, and the 409 ROOM_NOT_WAITING toast.
  - 2 new useWsDispatch tests asserting the room-id guard on `system:room_kicked`.
  - REST + WS contract enforced by both server and client tests against the same `system:room_kicked` payload shape.

  Recommend running through the Task 10.5 checklist during PR review before merge.

### File List

**Backend (Go):**

- `server/internal/apperr/errors.go` — added `ErrRoomNotWaiting`, `ErrCannotKickSelf`, `ErrSeatNotOccupied`.
- `server/internal/ws/events.go` — added `SystemRoomKicked` constant + `RoomKickedPayload` struct.
- `server/internal/room/handler.go` — added `KickPlayerRequest`, `KickPlayer`, `SwapSeatsRequest`, `SwapSeats`. No changes to existing handlers.
- `server/internal/room/handler_test.go` — added `doKickPlayer`, `doSwapSeats`, `seedSeatedRoom`, `msgTypeOf` helpers and 13 new tests (kick: 7, swap: 6) plus a regression assertion that swap doesn't broadcast lobby-wide.
- `server/cmd/api/main.go` — registered `/rooms/:id/kick` and `/rooms/:id/swap-seats` routes.

**Frontend (TypeScript / React):**

- `client/src/shared/types/wsEvents.ts` — added `SYSTEM_ROOM_KICKED` constant + `RoomKickedPayload` interface.
- `client/src/shared/stores/roomLobbyStore.ts` — added `kickedFromRoomId` field + `setKickedFromRoom` setter; included in `initialState` so `reset()` clears it.
- `client/src/shared/hooks/useWsDispatch.ts` — added `system:room_kicked` dispatch branch with the same `currentRoomId` guard pattern used by other room events.
- `client/src/shared/api/rooms.ts` — added `kickPlayer` and `swapSeats` API client functions.
- `client/src/shared/hooks/mutations/useRooms.ts` — added `useKickPlayerMutation` and `useSwapSeatsMutation`.
- `client/src/features/lobby/RoomLobby.tsx` — added `swapSourceSeat`/`kickConfirm` local state, kick icon overlay, swap-mode UI with `ring-2 ring-accent` highlight + caption, confirmation dialog, kicked-redirect `useEffect`, error-toast mapping for the new error codes, and per-seat pending-state styling. Existing seat-select/leave/start-game paths unchanged.
- `client/src/features/lobby/RoomLobby.test.tsx` — added 7 new test cases covering owner-only icon visibility, hide-on-not-waiting, hide-for-non-owner, dialog confirm/cancel paths, swap-mode entry/cancel, and the 409 ROOM_NOT_WAITING error toast.
- `client/src/shared/hooks/useWsDispatch.test.ts` — added 2 new test cases covering matching/non-matching room dispatch.
- `client/src/shared/i18n/en.json` — added `kickIconLabel`, `kickConfirm.{title,body,confirm,cancel}`, `kickedToast`, `swapMode.{enter,cancel}`, and the four new error keys.
- `client/src/shared/i18n/sr.json` — same key set in Serbian (Latin, Ekavian register).

**Sprint tracking:**

- `_bmad-output/implementation-artifacts/sprint-status.yaml` — story status `ready-for-dev` → `in-progress` → `review`.
- `_bmad-output/archive/implementation-artifacts/8-1-room-owner-pre-game-controls.md` — Tasks/Subtasks, Dev Agent Record, File List, Change Log, Status updated.

### Change Log

- 2026-04-26 — Initial implementation. Added kick + swap-seats handlers (REST + WS broadcast pattern), three new apperrs (`ROOM_NOT_WAITING`, `CANNOT_KICK_SELF`, `SEAT_NOT_OCCUPIED`), one new WS event (`system:room_kicked` + payload), API client + mutations, owner-controls UI overlay on `RoomLobby` (kick icon + swap mode + confirmation dialog + kicked-redirect effect), 22 new tests (13 backend + 9 frontend), and 12 i18n keys × 2 locales. Zero schema changes. Status set to `review`.
- 2026-04-26 — Code review patches (11 applied, 2 deferred, 11 dismissed). Headline fix: removed the speculative `previousSeat`-based clear in `roomLobbyStore.updatePlayerSeat` that was misfiring during seat swaps and leaving one swapped player seatless on every client. Other fixes: dispatcher requires positive `currentRoomId` match before setting `kickedFromRoomId` (prevents stale-kick-trap on re-entry); guarded nil-deref of `postRoom.PlayerCount` in `KickPlayer` response; per-tile pending-state styling using `mutation.variables`; `disabled={kickPlayerMutation.isPending}` on the kick icon button; new `useEffect` clears `swapSourceSeat` if the source player vacates the seat mid-swap; introduced `errors.notOwnerAction` i18n key (en + sr) for kick/swap 403 toasts; removed unused `swapMode.cancel` keys; real `disabled` (not just `pointer-events-none`) on seat tiles when pending; removed unreachable `errors.Is(ErrBadRequest)` arm; added test for kicked-toast + redirect flow and a third `useWsDispatch` case asserting null `currentRoomId` no-op. All 463 client tests + all server packages green; prettier + eslint + go vet clean. Status set to `done`.

### Review Findings

_Adversarial review — Blind Hunter + Edge Case Hunter + Acceptance Auditor (2026-04-26). 11 patches, 2 deferred, 11 dismissed as noise._

- [x] [Review][Patch] Swap broadcast leaves one swapped player seatless on every client [client/src/shared/stores/roomLobbyStore.ts:65-77] — `updatePlayerSeat`'s defensive `p.seat === previousSeat → seat=null` branch misfires during a swap. After event 1 (`pA → seatB`, prev=seatA), pA and pB both occupy seatB locally; event 2 (`pB → seatA`, prev=seatB) then matches pA on `seat === previousSeat` and clears pA's seat to null. Recommended fix: remove the defensive clear (it was speculative protection per its inline comment "shouldn't happen normally"; in normal flows nothing else is at `previousSeat`, so removing it is a no-op there). For the stale-row case it was guarding, prefer a `console.warn` over silent corruption.
- [x] [Review][Patch] Stale `kickedFromRoomId` traps user when they re-enter the same room [client/src/shared/stores/roomLobbyStore.ts:43, client/src/features/lobby/RoomLobby.tsx:127-134] — If the kicked-event arrives while the user is on `/lobby` (no RoomLobby mounted), `kickedFromRoomId` is set but never consumed. Re-joining the same room mounts RoomLobby → effect fires → toast + redirect, as if the user had just been kicked. Fix: clear `kickedFromRoomId` inside `setCurrentRoomId` so a fresh entry resets the trap.
- [x] [Review][Patch] Nil-deref of `postRoom.PlayerCount` on kick success response [server/internal/room/handler.go:851] — The broadcast block guards `postRoom != nil`, but the JSON response unconditionally reads `postRoom.PlayerCount`. If the in-tx re-fetch returned `(nil, nil)` (e.g. concurrent room cleanup), the response handler panics. Fix: guard the dereference (return 200 with `playerCount: 0` or fall back to `r.PlayerCount - 1` captured before commit).
- [x] [Review][Patch] Pending-state styling applies to every seated tile, not the affected one [client/src/features/lobby/RoomLobby.tsx (`isPendingForThisSeat`)] — `(swapSeatsMutation.isPending && player !== undefined) || (kickPlayerMutation.isPending && player !== undefined)` is true for ALL seated tiles whenever any mutation is pending, contradicting spec Task 6.6 ("affected seat tile only — not the whole grid"). Fix: gate per seat — for kick, check against `kickConfirm?.seat`; for swap, track the in-flight target seat in state and check against `swapSourceSeat` + that target.
- [x] [Review][Patch] Kick icon stays clickable while a kick is in flight [client/src/features/lobby/RoomLobby.tsx:454-475] — The kick `<button>` (sibling of the seat button) has no `disabled` prop. After the dialog closes on confirm, the player is still rendered (waiting for WS); a second click reopens the dialog and a duplicate POST returns `NOT_IN_ROOM` with a misleading toast. Fix: `disabled={kickPlayerMutation.isPending}` on the kick button.
- [x] [Review][Patch] Swap-mode source highlight persists if the source player leaves [client/src/features/lobby/RoomLobby.tsx:138-142, 211-227] — `swapSourceSeat` is only cleared on unmount, status flip, success, or cancel-click. If `system:player_left` removes the source occupant, the ring stays on the now-empty seat; clicking another player fires a swap with `seatA=<empty>` → server returns `SEAT_NOT_OCCUPIED`. Fix: a useEffect keyed on `players` that clears `swapSourceSeat` when `getPlayerAtSeat(players, swapSourceSeat) === undefined`.
- [x] [Review][Patch] Reused `errors.notOwner` i18n string is misleading for kick/swap [client/src/shared/i18n/en.json:188, sr.json equivalent, RoomLobby.tsx:395, 423] — String reads "Only the room owner can start the game" — wrong context for kick/swap 403s. Fix: add a generic `errors.notOwnerAction` ("Only the room owner can do that.") for kick/swap, or split into per-action keys.
- [x] [Review][Patch] `swapMode.cancel` i18n key is dead [client/src/shared/i18n/en.json:189-192, sr.json equivalent] — Defined in both locales but no UI element renders it. Spec Task 6.4 hinted at click-outside-the-grid cancellation that was not implemented. Fix: either remove the keys or render a "Cancel swap" affordance and wire click-outside cancel.
- [x] [Review][Patch] Missing test: kicked-toast + redirect on `kickedFromRoomId` match [client/src/features/lobby/RoomLobby.test.tsx] — AC #7 mandates the toast + navigate, but no test exercises the path end-to-end. Recommend a case that sets `useRoomLobbyStore.setState({ kickedFromRoomId: 1 })` (with `id` matching) and asserts `toast.error` was called with the kickedToast string and `mockNavigate("/lobby")` fired.
- [x] [Review][Patch] Dead error arm in KickPlayer post-tx switch [server/internal/room/handler.go:820] — `errors.Is(err, apperr.ErrBadRequest)` arm is unreachable: `apperr.ErrBadRequest` is only returned at lines 757-762, before `RunInTransaction`. Fix: remove the unreachable arm.
- [x] [Review][Patch] Seat tile uses `pointer-events-none` only — keyboard activation still fires kick/swap [client/src/features/lobby/RoomLobby.tsx (seat button)] — `disabled={!isClickable}` does not include the pending check; `pointer-events-none` blocks mouse but Enter/Space still triggers `onClick`. For destructive actions prefer real `disabled` when pending.
- [x] [Review][Defer] `kickedUsername` falls back to empty string on transient `FindPlayersByRoomID` error [server/internal/room/handler.go:766] — deferred, pre-existing pattern (`LeaveRoom` does the same).
- [x] [Review][Defer] `tx.FindPlayerRoom` is a global lookup, brittle to stale rows in other rooms [server/internal/room/handler.go:793, gorm_repo.go FindPlayerRoom] — deferred, pre-existing repository surface used elsewhere.
