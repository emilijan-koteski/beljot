# Story 2.4: Team Assignment & Game Start

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a player in a room lobby,
I want to pick my team and have the room owner start the game when everyone is ready,
So that we can begin playing with the teams arranged how we want.

## Acceptance Criteria

1. **Seat Selection** — When a player in the RoomLobby clicks an empty seat on Team A or Team B, they are assigned to that team and seat. All players in the room see the updated seat assignment via WebSocket. Partners face each other (seats 0+2 = Team A, seats 1+3 = Team B per Architecture seat mapping).

2. **Seat Switching** — When a player who is already seated clicks a different empty seat, they move to the new seat and their old seat becomes empty. All players see the update in real-time.

3. **Start Game Button Active** — When all 4 seats are filled, the room owner sees the "Start Game" button become active (accent style, previously disabled at 40% opacity).

4. **Start Game Button Disabled** — When fewer than 4 seats are filled, the room owner sees the "Start Game" button disabled with `cursor-not-allowed` and a tooltip: "All 4 players must be seated".

5. **Game Start** — When the room owner clicks "Start Game" with 4 players seated, the room status changes to `in_progress`. All 4 players are transitioned to the game view via WebSocket `system:game_started` event. The room is removed from the browse list.

6. **Non-Owner Waiting Message** — When a non-owner player views the lobby with 4 players, they see a message "Waiting for [owner username] to start the game..." instead of a Start button.

## Tasks / Subtasks

- [x] **Task 1: Backend — Add `UpdatePlayerSeat` and `ClearPlayerSeat` repository methods** (AC: #1, #2)
  - [x] Add to `RoomRepository` interface in `server/internal/room/repository.go`:
    ```go
    UpdatePlayerSeat(roomID uint, userID uint, seat int, team string) error
    ClearPlayerSeat(roomID uint, userID uint) error
    FindPlayerBySeat(roomID uint, seat int) (*RoomPlayer, error)
    ```
  - [x] Implement in `server/internal/room/gorm_repo.go`:
    - `UpdatePlayerSeat`: `UPDATE room_players SET seat = ?, team = ? WHERE room_id = ? AND user_id = ?`. If no rows affected -> return `ErrNotInRoom`
    - `ClearPlayerSeat`: `UPDATE room_players SET seat = NULL, team = NULL WHERE room_id = ? AND user_id = ?`. If no rows affected -> return `ErrNotInRoom`
    - `FindPlayerBySeat`: `SELECT rp.*, u.username FROM room_players rp JOIN users u ON u.id = rp.user_id WHERE rp.room_id = ? AND rp.seat = ?`. Returns `(nil, nil)` if no player at that seat
  - [x] **Seat-to-team derivation**: The team is deterministic from seat index — seats 0,2 = `"teamA"`, seats 1,3 = `"teamB"`. The handler computes the team from the seat, NOT the client. Never trust client-submitted team values

- [x] **Task 2: Backend — Add new domain errors** (AC: #1, #2, #3, #5)
  - [x] Add to `server/internal/apperr/errors.go`:
    ```go
    ErrSeatTaken       = NewAppError("SEAT_TAKEN", "seat is already occupied", http.StatusConflict)
    ErrInvalidSeat     = NewAppError("INVALID_SEAT", "seat must be 0, 1, 2, or 3", http.StatusBadRequest)
    ErrNotAllSeated    = NewAppError("NOT_ALL_SEATED", "all 4 players must be seated to start", http.StatusBadRequest)
    ErrGameNotStartable = NewAppError("GAME_NOT_STARTABLE", "room is not in waiting status", http.StatusConflict)
    ```

- [x] **Task 3: Backend — Add `SelectSeat` handler** (AC: #1, #2)
  - [x] Add `SelectSeat` handler in `server/internal/room/handler.go`:
    ```
    POST /api/v1/rooms/:id/seat
    Request body: { "seat": 0 }
    ```
  - [x] Add request struct:
    ```go
    type SelectSeatRequest struct {
        Seat int `json:"seat"`
    }
    ```
  - [x] Validation sequence:
    1. Get authenticated user ID via `auth.GetUserID(c)`
    2. Parse room ID from URL param
    3. Bind JSON body to `SelectSeatRequest`
    4. Validate `seat` is 0, 1, 2, or 3 — if not, return `ErrInvalidSeat`
    5. `repo.FindByID(roomID)` — if nil, return `ErrRoomNotFound`
    6. Check `room.Status == "waiting"` — if not, return `ErrGameNotStartable`
    7. Derive team: `if seat % 2 == 0 { team = "teamA" } else { team = "teamB" }`
    8. Run in transaction:
       a. `repo.FindPlayerBySeat(roomID, seat)` — if occupied by another user, return `ErrSeatTaken`. If occupied by same user, return 200 (no-op)
       b. Find current player record: call `repo.FindPlayerRoom(userID)` within the transaction — this returns the player's existing `RoomPlayer` record. If `player.Seat != nil`, call `repo.ClearPlayerSeat(roomID, userID)` first (seat switching per AC #2). This leverages the existing method from Story 2.3 rather than scanning `FindPlayersByRoomID`
       c. `repo.UpdatePlayerSeat(roomID, userID, seat, team)`
    9. Re-fetch players via `repo.FindPlayersByRoomID(roomID)` for response
    10. Return HTTP 200 with `{ "data": { "players": [...] } }`
  - [x] **TODO comment for WS broadcast:** Add `// TODO: broadcast system:seat_updated to room participants (WS hub not yet wired)` after successful update
  - [x] **Important: seat validation is integer-based** — the request body uses `int`, not a string. Validate that the bound value is exactly 0, 1, 2, or 3. Go's JSON decoder will set unset int fields to 0 which is a valid seat — use a pointer `*int` in the request struct to distinguish missing from zero:
    ```go
    type SelectSeatRequest struct {
        Seat *int `json:"seat"`
    }
    ```
    Check `req.Seat == nil` -> return `ErrInvalidSeat`. Then dereference: `seat := *req.Seat`

- [x] **Task 4: Backend — Add `StartGame` handler** (AC: #5)
  - [x] Add `StartGame` handler in `server/internal/room/handler.go`:
    ```
    POST /api/v1/rooms/:id/start
    ```
  - [x] Validation sequence:
    1. Get authenticated user ID via `auth.GetUserID(c)`
    2. Parse room ID from URL param
    3. `repo.FindByID(roomID)` — if nil, return `ErrRoomNotFound`
    4. Check `room.Status == "waiting"` — if not, return `ErrGameNotStartable`
    5. Check `room.OwnerID == userID` — if not, return `ErrNotRoomOwner`
    6. `repo.FindPlayersByRoomID(roomID)` — count players with non-nil seats. If < 4, return `ErrNotAllSeated`
    7. Run in transaction:
       a. Set `room.Status = "in_progress"`
       b. `repo.Update(room)`
    8. Return HTTP 200 with `{ "data": room }`
  - [x] **TODO comment for WS broadcast:** Add `// TODO: broadcast system:game_started to room participants + system:room_updated to lobby (WS hub not yet wired)`
  - [x] **Note:** This story only changes the room status. Actual game session creation (dealing cards, initializing GameState) is in Epic 3/4. This endpoint is the trigger point — future stories will hook game initialization here

- [x] **Task 5: Backend — Wire new routes** (AC: #1, #5)
  - [x] Add to `server/cmd/api/main.go` after existing room routes:
    ```go
    api.POST("/rooms/:id/seat", roomHandler.SelectSeat)
    api.POST("/rooms/:id/start", roomHandler.StartGame)
    ```
  - [x] Both routes are under the `api` group (auth middleware applied)

- [x] **Task 6: Backend — Add WebSocket events to contract** (AC: #1, #2, #5)
  - [x] Add to `server/internal/ws/events.go`:
    ```go
    const SystemSeatUpdated = "system:seat_updated"
    const SystemGameStarted = "system:game_started"
    ```
  - [x] Add to `client/src/shared/types/wsEvents.ts`:

    ```typescript
    export const SYSTEM_SEAT_UPDATED = "system:seat_updated" as const;
    export const SYSTEM_GAME_STARTED = "system:game_started" as const;

    export interface SeatUpdatedPayload {
      roomId: number;
      userId: number;
      username: string;
      seat: number;
      team: string;
      previousSeat: number | null; // null if player was not previously seated (first-time seat selection)
    }

    export interface GameStartedPayload {
      roomId: number;
    }
    ```

  - [x] **Both files updated in the same commit** — per project convention

- [x] **Task 7: Frontend — Add API client functions** (AC: #1, #5)
  - [x] Add to `client/src/shared/api/rooms.ts`:

    ```typescript
    export function selectSeat(
      roomId: number,
      seat: number,
    ): Promise<{ players: RoomPlayer[] }> {
      return fetchClient<{ players: RoomPlayer[] }>(`/rooms/${roomId}/seat`, {
        method: "POST",
        body: JSON.stringify({ seat }),
      });
    }

    export function startGame(roomId: number): Promise<Room> {
      return fetchClient<Room>(`/rooms/${roomId}/start`, { method: "POST" });
    }
    ```

  - [x] Import `RoomPlayer` from `apiTypes.ts` (already imported via `RoomDetail`)
  - [x] **Important:** `fetchClient<T>` auto-unwraps `{ data: T }` — do NOT double-unwrap

- [x] **Task 8: Frontend — Modify RoomLobby for clickable team seats** (AC: #1, #2, #3, #4, #6)
  - [x] Modify `client/src/features/lobby/RoomLobby.tsx`:
  - [x] **Replace simple grid with team-labeled seat layout:**
    ```
    ┌─────────────────────────────────────────────────────┐
    │  <- Back to Lobby        Room Name        Copy Link  │
    ├─────────────────────────────────────────────────────┤
    │                                                     │
    │  TEAM A                            TEAM B           │
    │  ┌──────────┐                    ┌──────────┐       │
    │  │  Seat 0  │                    │  Seat 1  │       │
    │  │ Player A │                    │ Waiting..│       │
    │  │ (Team A) │                    │ (Team B) │       │
    │  └──────────┘                    └──────────┘       │
    │                                                     │
    │  ┌──────────┐                    ┌──────────┐       │
    │  │  Seat 2  │                    │  Seat 3  │       │
    │  │ Waiting..│                    │ Player B │       │
    │  │ (Team A) │                    │ (Team B) │       │
    │  └──────────┘                    └──────────┘       │
    │                                                     │
    │  Bitola · 1001 · Relaxed                            │
    │                                                     │
    │  [Start Game] / "Waiting for X to start..."         │
    │                              Leave Room             │
    └─────────────────────────────────────────────────────┘
    ```
  - [x] **Seat rendering logic:** Render seats as a 2-column layout. Left column = Team A (seats 0, 2), right column = Team B (seats 1, 3). Each seat is clickable if empty or occupied by the current user (for switching). Partner seats are vertically aligned (0 above 2, 1 above 3) — partners face each other
  - [x] **Team color indicators:** Team A seats use `border-team-a` (or fallback `border-red-500`), Team B seats use `border-team-b` (or fallback `border-blue-500`). Team color label text above each column
  - [x] **Click handler on empty seats:** Call `selectSeat(roomId, seatIndex)`. On success, update local `players` state from response. On error, show toast with error message (SEAT_TAKEN, INVALID_SEAT, etc.)
  - [x] **Click handler on own seat:** Same as empty — the backend handles seat switching (clears old seat, assigns new one)
  - [x] **Occupied seats (other players):** Show player username, team color border, NOT clickable. Use `cursor-default`
  - [x] **Self indicator:** Keep the `ring-1 ring-accent` highlight + "You" badge on the current user's seat
  - [x] **Owner indicator:** Keep the "Owner" label next to the owner's username
  - [x] **Start Game section (bottom of seats area):**
    - If current user IS the room owner:
      - All 4 seats filled -> show `<Button>` with accent style: "Start Game" (`data-testid="start-game"`)
      - Fewer than 4 seats -> show disabled `<Button>` at 40% opacity with `cursor-not-allowed` and `title="All 4 players must be seated"` (`data-testid="start-game"`)
    - If current user is NOT the room owner:
      - All 4 seats filled -> show text: "Waiting for {ownerUsername} to start the game..." (`data-testid="waiting-for-start"`)
      - Fewer than 4 seats -> show nothing extra (the empty seats communicate the state)
  - [x] **Start Game click handler:** Call `startGame(roomId)`. On success, navigate to `/game/${roomId}` (the game view route — even if it doesn't exist yet, wire the navigation). On error, show toast
  - [x] **Owner username resolution:** The owner's username is available from the `players` array by matching `player.userId === room.ownerId`. If owner is not yet seated (no matching player with that userId in players with a seat), fall back to showing "the room owner"
  - [x] Use `data-testid="player-seat-{index}"` for each seat slot (preserve existing convention)

- [x] **Task 9: Frontend — Update `useRoomLobbyUpdates` hook** (AC: #1, #2, #5)
  - [x] Modify `client/src/features/lobby/useRoomLobbyUpdates.ts`:
  - [x] Add handler for `system:seat_updated`:
    - Update the player's seat and team in local state
    - If `previousSeat` is not null, clear that seat assignment from local state
  - [x] Add handler for `system:game_started`:
    - Navigate to `/game/${roomId}` when received (all players transition together)
  - [x] **Note:** WS hub is not yet wired end-to-end. Create/update the handler logic for correctness. Document that it requires WS infrastructure from a future story

- [x] **Task 10: Frontend — Add i18n translations** (AC: #1, #3, #4, #6)
  - [x] Add to `client/src/shared/i18n/en.json` under `lobby.roomLobby`:
    ```json
    {
      "teamA": "Team A",
      "teamB": "Team B",
      "startGame": "Start Game",
      "startGameDisabled": "All 4 players must be seated",
      "waitingForOwner": "Waiting for {{owner}} to start the game...",
      "seatTaken": "That seat is already taken",
      "seatUpdated": "Seat updated",
      "gameStarting": "Game starting...",
      "errors": {
        "seatFailed": "Failed to select seat — try again",
        "startFailed": "Failed to start game — try again",
        "notOwner": "Only the room owner can start the game",
        "notAllSeated": "All 4 players must be seated to start"
      }
    }
    ```
  - [x] Add matching keys to `client/src/shared/i18n/sr.json`:
    ```json
    {
      "teamA": "Tim A",
      "teamB": "Tim B",
      "startGame": "Započni igra",
      "startGameDisabled": "Site 4 igrači mora da se sednat",
      "waitingForOwner": "Čekanje {{owner}} da ja započne igrata...",
      "seatTaken": "Toa mesto e zazemeno",
      "seatUpdated": "Mesto ažurirano",
      "gameStarting": "Igrata počnuva...",
      "errors": {
        "seatFailed": "Neuspešen izbor na mesto — probaj povtorno",
        "startFailed": "Neuspešno započnuvanje — probaj povtorno",
        "notOwner": "Samo sopstvenikot na sobata može da započne",
        "notAllSeated": "Site 4 igrači mora da se sednat za da počne"
      }
    }
    ```

- [x] **Task 11: Backend tests** (AC: #1, #2, #3, #5)
  - [x] Add test cases to `server/internal/room/handler_test.go`:
    - **SelectSeat** tests:
      - Successful seat selection — returns 200 with updated players list, correct seat and team
      - Select seat 0 -> team is "teamA"; seat 1 -> team is "teamB"; seat 2 -> "teamA"; seat 3 -> "teamB"
      - Seat switching — player moves from seat 0 to seat 3, old seat freed
      - Seat already occupied by another player — returns 409 `SEAT_TAKEN`
      - Invalid seat number (5, -1) — returns 400 `INVALID_SEAT`
      - Missing seat in body (null) — returns 400 `INVALID_SEAT`
      - Player not in room — returns 404 `NOT_IN_ROOM`
      - Room not in `waiting` status — returns 409 `GAME_NOT_STARTABLE`
      - Select own current seat — returns 200 (no-op)
      - Requires authentication — returns 401
    - **StartGame** tests:
      - Successful start — returns 200, room status is `in_progress`
      - Non-owner tries to start — returns 403 `NOT_ROOM_OWNER`
      - Fewer than 4 players seated — returns 400 `NOT_ALL_SEATED`
      - Room not in `waiting` status — returns 409 `GAME_NOT_STARTABLE`
      - Room not found — returns 404
      - Requires authentication — returns 401
  - [x] Extend existing mock repository with: `UpdatePlayerSeat`, `ClearPlayerSeat`, `FindPlayerBySeat`
  - [x] Use external test package: `package room_test`

- [x] **Task 12: Frontend tests** (AC: #1, #2, #3, #4, #5, #6)
  - [x] Update `client/src/features/lobby/RoomLobby.test.tsx`:
    - Empty seats are clickable and call `selectSeat` API on click
    - Occupied seats (other players) are NOT clickable
    - Own seat is clickable (for switching)
    - Team A seats (0, 2) show Team A color indicator
    - Team B seats (1, 3) show Team B color indicator
    - Room owner sees Start Game button
    - Start Game button disabled when fewer than 4 players seated
    - Start Game button enabled when all 4 seated, calls `startGame` API on click
    - Non-owner sees "Waiting for [owner] to start..." message when 4 seated
    - Non-owner does NOT see Start Game button
    - Error toast shown when seat selection fails
    - Error toast shown when game start fails
  - [x] Use `data-testid` attributes for element selection — never CSS classes
  - [x] Test descriptions in present tense: `it('renders Start Game button for room owner')`

- [x] **Task 13: Lint and test** (AC: all)
  - [x] Run `make lint` — pass with zero new errors
  - [x] Run `make test` — all existing + new tests pass

## Dev Notes

### Architecture Compliance

**This story extends the existing `room/` package** — `SelectSeat` and `StartGame` handlers belong alongside JoinRoom, LeaveRoom, GetRoom. Do NOT create a separate package.

**Seat mapping is canonical per Architecture:**

- Seats 0+2 = Team A (partners)
- Seats 1+3 = Team B (partners)
- Team is derived server-side from seat index: `seat % 2 == 0 -> "teamA"`, `seat % 2 != 0 -> "teamB"`
- The client NEVER sends a team value — it sends only the seat number. Server computes team

**Room status lifecycle for this story:**

- `waiting` -> `in_progress` (via StartGame endpoint)
- Only the room owner can trigger StartGame
- Once `in_progress`, the room no longer appears in browse list (existing `GET /rooms?status=waiting` filter handles this)

**No game session creation in this story.** StartGame only changes room status. The actual game initialization (dealing, GameState creation) belongs to Epic 3 (rules engine) and Epic 4 (real-time session). This story establishes the transition trigger.

### File Modifications

```
server/internal/room/
├── model.go          # NO changes (RoomPlayer already has Seat *int, Team *string)
├── repository.go     # ADD: UpdatePlayerSeat, ClearPlayerSeat, FindPlayerBySeat
├── gorm_repo.go      # ADD: implementations for 3 new methods
├── handler.go        # ADD: SelectSeat, StartGame handlers + SelectSeatRequest struct
├── handler_test.go   # ADD: tests for SelectSeat and StartGame

server/internal/apperr/
├── errors.go         # ADD: ErrSeatTaken, ErrInvalidSeat, ErrNotAllSeated, ErrGameNotStartable

server/internal/ws/
├── events.go         # ADD: SystemSeatUpdated, SystemGameStarted

server/cmd/api/
├── main.go           # ADD: 2 new route registrations

client/src/shared/types/
├── wsEvents.ts       # ADD: SYSTEM_SEAT_UPDATED, SYSTEM_GAME_STARTED + payload types

client/src/shared/api/
├── rooms.ts          # ADD: selectSeat(), startGame() functions

client/src/features/lobby/
├── RoomLobby.tsx          # MODIFY: clickable team seats, Start Game button, owner/non-owner logic
├── RoomLobby.test.tsx     # MODIFY: add seat selection + game start tests
├── useRoomLobbyUpdates.ts # MODIFY: add system:seat_updated + system:game_started handlers

client/src/shared/i18n/
├── en.json           # ADD: team/seat/start i18n keys under lobby.roomLobby
├── sr.json           # ADD: matching Serbian translations
```

### Previous Story (2.3) Intelligence

**Key patterns established:**

- `RoomPlayer` model already has nullable `Seat *int` and `Team *string` fields — ready for this story
- Transaction pattern: `h.repo.RunInTransaction(func(tx RoomRepository) error { ... })` — use this for SelectSeat to prevent race conditions on seat assignment
- Response pattern: `c.JSON(http.StatusOK, map[string]interface{}{"data": ...})`
- Error pattern: domain errors from `apperr` package, handler checks via `errors.Is()`
- Frontend uses local state (`useState`) for room/players, not a Zustand store — continue this pattern
- `getRoom(id)` returns `{ room, players }` — use this for initial load, then update `players` locally on seat changes
- `hasLeftRef` / `hasJoinedRef` pattern for cleanup on unmount — preserve this
- Existing `data-testid` conventions: `player-seat-{index}`, `room-lobby`, `copy-link`, `leave-room`, `back-to-lobby`

**Things NOT to break:**

- Leave Room functionality (navigate away cleanup via `useEffect` return)
- Copy Link button
- Loading/error states
- Owner/You indicators
- Room config display bar

### Backend Implementation Details

**SelectSeat request uses `*int` pointer for seat:**
Go's JSON decoder defaults `int` to `0` for missing fields. Since seat 0 is a valid seat, use `*int` in the request struct to distinguish "not provided" (`nil`) from "seat 0" (`&0`).

**Transaction is critical for SelectSeat:**
Two players clicking the same seat simultaneously could both pass the "seat empty" check. The transaction + the check-then-update within a single transaction prevents this. The unique constraint approach (unique on `room_id + seat WHERE seat IS NOT NULL`) could also help as a safety net, but is NOT required for Phase 1 — the transaction is sufficient.

**LeaveRoom interaction:** When a player leaves the room (Story 2.3), their `RoomPlayer` record is deleted entirely via `RemovePlayer`. This automatically frees their seat — no additional cleanup needed in this story.

### Frontend Implementation Details

**Seat layout structure:**
The 2x2 grid should visually group seats by team. Left column = Team A (seats 0, 2), right column = Team B (seats 1, 3). Each column has a team label header. This maps to the Architecture seat mapping where partners face each other.

**Game route navigation:**
On `StartGame` success (owner) or `system:game_started` event (all players), navigate to `/game/${roomId}`. This route may not exist yet — wire the navigation anyway. React Router will show a 404 or blank page until Epic 4 implements the game view.

**Existing CSS tokens available:**

- `team-a` (#ff4d4d) and `team-b` (#4d9fff) are defined in the Tailwind design tokens (Story 1.1)
- Use `border-team-a`, `text-team-a`, `border-team-b`, `text-team-b` classes
- If tokens are not yet wired as Tailwind classes, fall back to `border-red-400`, `text-red-400`, `border-blue-400`, `text-blue-400` and add a TODO to align with design tokens

**Disabled button styling:**
Use `opacity-40 cursor-not-allowed` on the disabled Start Game button. Add a `title` attribute for the tooltip text (native browser tooltip is acceptable for Phase 1).

### References

- [Source: _bmad-output/planning-artifacts/epics.md — Story 2.4 acceptance criteria, lines 568-603]
- [Source: _bmad-output/planning-artifacts/architecture.md — Player Seat Mapping, lines 1043-1058]
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md — Journey 2 Ana's Private Room, lines 460-489]
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md — PlayerSeat component spec, lines 676-690]
- [Source: _bmad-output/project-context.md — Room domain package shape, framework rules, testing rules]
- [Source: server/internal/room/handler.go — Existing JoinRoom/LeaveRoom transaction patterns]
- [Source: server/internal/room/model.go — RoomPlayer with Seat *int, Team *string already defined]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

### Completion Notes List

- Implemented `SelectSeat` handler (POST /rooms/:id/seat) with `*int` pointer for seat to distinguish missing from seat 0
- Team derived server-side: seats 0,2 = "teamA", seats 1,3 = "teamB" — client never sends team
- Seat switching handled atomically in transaction: clear old seat, assign new seat
- Implemented `StartGame` handler (POST /rooms/:id/start) — owner-only, requires all 4 seated
- StartGame only changes room status to `in_progress` — no game session creation (Epic 3/4)
- RoomLobby rewritten with team-colored 2-column seat layout (Team A left, Team B right)
- Empty seats clickable, own seat clickable (for switching), other players' seats disabled
- Owner sees Start Game button (enabled when all 4 seated, disabled at 40% opacity otherwise)
- Non-owner sees "Waiting for [owner] to start the game..." when all 4 seated
- WebSocket events contract updated in both Go and TypeScript files
- i18n translations added for EN and SR
- 17 new backend tests (11 SelectSeat + 6 StartGame), all passing
- 12 new/updated frontend tests (19 total in RoomLobby.test.tsx), all passing
- All 88 frontend tests pass across 14 files, zero regressions
- All Go tests pass (pre-existing user integration test failures unrelated to this story)

### Change Log

- 2026-04-11: Story 2.4 implementation complete — team assignment and game start

### File List

- server/internal/room/repository.go (modified — 3 new interface methods)
- server/internal/room/gorm_repo.go (modified — 3 new implementations)
- server/internal/room/handler.go (modified — SelectSeat, StartGame handlers + types)
- server/internal/room/handler_test.go (modified — 3 new mock methods + 17 new tests)
- server/internal/apperr/errors.go (modified — 4 new error types)
- server/internal/ws/events.go (modified — 2 new event constants)
- server/cmd/api/main.go (modified — 2 new route registrations)
- client/src/shared/types/wsEvents.ts (modified — 2 new events + payload types)
- client/src/shared/types/apiTypes.ts (no changes — types already existed)
- client/src/shared/api/rooms.ts (modified — selectSeat, startGame functions)
- client/src/features/lobby/RoomLobby.tsx (modified — team seats, start game, owner logic)
- client/src/features/lobby/RoomLobby.test.tsx (modified — 19 tests covering all ACs)
- client/src/features/lobby/useRoomLobbyUpdates.ts (modified — seat_updated, game_started handlers)
- client/src/shared/i18n/en.json (modified — team/seat/start i18n keys)
- client/src/shared/i18n/sr.json (modified — matching Serbian translations)
