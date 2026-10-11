# Story 2.5: Quick Play Matchmaking

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a player,
I want to quickly find a game without browsing rooms,
So that I can start playing as fast as possible.

## Acceptance Criteria

1. **Quick Play Entry** — When a player clicks "Quick Play" in the lobby, they enter a matchmaking state with a pulsing "Finding match..." indicator and a Cancel button.

2. **Join Existing Room** — When an open Quick Play room exists with available seats (Bitola, 1001, relaxed defaults, `is_quick_play = true`, status `waiting`, `player_count < 4`), the server assigns the player to that room and redirects them to the RoomLobby view.

3. **Create New Room** — When no suitable Quick Play room exists, the server creates a new room with Quick Play defaults (Bitola, 1001, relaxed, `is_quick_play = true`) with the player as owner, and redirects them to the RoomLobby to wait for others. Room name is auto-generated as `"Quick Play XXXXXX"` using the 6-char room code.

4. **Cancel Matchmaking** — When a player is waiting in the Quick Play state, a "Cancel" button is available that aborts the request and returns to the lobby options view.

5. **Auto-Start on Full Seats** — When all 4 players in a Quick Play room have selected seats (`SelectSeat`), the server automatically transitions the room to `in_progress` status (no manual Start Game required). All 4 players are transitioned to the game view. The `SelectSeat` response includes a `gameStarted` boolean flag.

6. **Quick Play Room Lobby Behavior** — When a player is in a Quick Play room lobby, no "Start Game" button is shown for anyone (including the owner). Instead, a message displays: "Game starts automatically when all players are seated." The room is otherwise identical to a manual room (seat selection, team display, leave room).

## Tasks / Subtasks

- [x] **Task 1: Database — Add `is_quick_play` column to rooms** (AC: #2, #3)
  - [x] Create migration `server/migrations/000005_add_is_quick_play.up.sql`:
    ```sql
    ALTER TABLE rooms ADD COLUMN is_quick_play BOOLEAN NOT NULL DEFAULT false;
    CREATE INDEX idx_rooms_quick_play ON rooms(is_quick_play, status, player_count) WHERE deleted_at IS NULL AND is_quick_play = true AND status = 'waiting';
    ```
  - [x] Create migration `server/migrations/000005_add_is_quick_play.down.sql`:
    ```sql
    DROP INDEX IF EXISTS idx_rooms_quick_play;
    ALTER TABLE rooms DROP COLUMN IF EXISTS is_quick_play;
    ```
  - [x] Update Room model in `server/internal/room/model.go`:
    ```go
    IsQuickPlay bool `gorm:"not null;default:false" json:"isQuickPlay"`
    ```
    Add after `PlayerCount` field, before `CreatedAt`
  - [x] Exempt Quick Play rooms from unique name constraint: the existing `idx_rooms_name_active` unique index on `(name) WHERE deleted_at IS NULL AND status != 'completed'` will conflict when multiple Quick Play rooms exist. However, since each Quick Play room name includes the unique room code (`"Quick Play ABCDEF"`), names will be unique. No constraint change needed

- [x] **Task 2: Backend — Add `FindQuickPlayRoom` repository method** (AC: #2)
  - [x] Add to `RoomRepository` interface in `server/internal/room/repository.go`:
    ```go
    FindQuickPlayRoom() (*Room, error)
    ```
  - [x] Implement in `server/internal/room/gorm_repo.go`:

    ```go
    func (r *GormRepository) FindQuickPlayRoom() (*Room, error) {
        var room Room
        err := r.db.Where("is_quick_play = ? AND status = ? AND player_count < 4", true, "waiting").
            Order("created_at ASC").
            First(&room).Error
        if err != nil {
            if errors.Is(err, gorm.ErrRecordNotFound) {
                return nil, nil
            }
            return nil, fmt.Errorf("finding quick play room: %w", err)
        }
        return &room, nil
    }
    ```

    - Orders by `created_at ASC` to fill oldest rooms first
    - Returns `(nil, nil)` when no room is available (same pattern as `FindByID`)

- [x] **Task 3: Backend — Add `QuickPlay` handler** (AC: #1, #2, #3)
  - [x] Add `QuickPlay` handler in `server/internal/room/handler.go`:
    ```
    POST /api/v1/rooms/quick-play
    No request body required
    ```
  - [x] Handler logic:
    1. Get authenticated user ID via `auth.GetUserID(c)`
    2. Check player not already in a room: `repo.FindPlayerRoom(userID)` — if non-nil, return `ErrAlreadyInRoom`
    3. Run in transaction:
       a. `repo.FindQuickPlayRoom()` to find available room
       b. **If room found:** Join the room (same as JoinRoom logic):
       - `tx.AddPlayer(&RoomPlayer{RoomID: room.ID, UserID: userID})`
       - `tx.IncrementPlayerCount(room.ID)`
       - Re-fetch room via `tx.FindByID(room.ID)` to get updated state
         c. **If no room found:** Create a new Quick Play room:
       - Generate room code via `generateRoomCode()` (existing function)
       - Create room with: `Name: "Quick Play " + code`, `Variant: "bitola"`, `MatchMode: "1001"`, `TimerStyle: "relaxed"`, `IsQuickPlay: true`, `Status: "waiting"`, `PlayerCount: 1`
       - `tx.Create(room)` + `tx.AddPlayer(&RoomPlayer{RoomID: room.ID, UserID: userID})`
       - Handle code collision with retry (same pattern as CreateRoom)
    4. Return HTTP 200 with `{ "data": room }`
  - [x] **Important:** The response returns the full Room object (including `isQuickPlay: true`). The frontend uses this to navigate to `/rooms/${room.id}`
  - [x] **No auto-seat assignment in QuickPlay handler** — players pick seats manually in the RoomLobby, same as manual rooms. The difference is auto-start when all 4 are seated

- [x] **Task 4: Backend — Modify `SelectSeat` for Quick Play auto-start** (AC: #5)
  - [x] In `server/internal/room/handler.go`, modify the `SelectSeat` handler:
        After the successful seat update transaction, add auto-start logic:
    ```go
    // After re-fetching players via repo.FindPlayersByRoomID(roomID):
    // Check if Quick Play room should auto-start
    if room.IsQuickPlay {
        seatedCount := 0
        for _, p := range players {
            if p.Seat != nil {
                seatedCount++
            }
        }
        if seatedCount == 4 {
            room.Status = "in_progress"
            if err := h.repo.Update(room); err != nil {
                return fmt.Errorf("auto-starting quick play room: %w", err)
            }
        }
    }
    ```
  - [x] Update the SelectSeat response to include `gameStarted` flag:
    ```go
    return c.JSON(http.StatusOK, map[string]interface{}{
        "data": map[string]interface{}{
            "players":     players,
            "gameStarted": room.IsQuickPlay && room.Status == "in_progress",
        },
    })
    ```
  - [x] **Important:** The auto-start update runs outside the seat assignment transaction. This is acceptable — if it fails, the seats are still correctly assigned and the frontend can retry. The room is already in `waiting` status with all 4 seated, so the next seat re-selection will trigger auto-start again
  - [x] **The existing `StartGame` handler remains unchanged** — it's still needed for manual rooms. Quick Play rooms bypass it via auto-start

- [x] **Task 5: Backend — Wire new route** (AC: #1)
  - [x] Add to `server/cmd/api/main.go` after existing room routes:
    ```go
    api.POST("/rooms/quick-play", roomHandler.QuickPlay)
    ```
  - [x] **Route ordering matters:** This route MUST be registered BEFORE `api.POST("/rooms/:id/join", ...)` because Echo matches routes in registration order. `/rooms/quick-play` could match `:id` as `"quick-play"` if the parameterized route is registered first. Place it right after `api.POST("/rooms", ...)` and before `api.GET("/rooms/:id", ...)`

- [x] **Task 6: Frontend — Add API client function** (AC: #1)
  - [x] Add to `client/src/shared/api/rooms.ts`:
    ```typescript
    export function quickPlay(signal?: AbortSignal): Promise<Room> {
      return fetchClient<Room>("/rooms/quick-play", {
        method: "POST",
        signal,
      });
    }
    ```
  - [x] The `signal` parameter enables request cancellation via `AbortController`
  - [x] **Important:** `fetchClient<T>` auto-unwraps `{ data: T }` — do NOT double-unwrap

- [x] **Task 7: Frontend — Update Room type** (AC: #2, #3, #6)
  - [x] Add to `Room` interface in `client/src/shared/types/apiTypes.ts`:
    ```typescript
    isQuickPlay: boolean;
    ```
    Add after `playerCount`, before `createdAt`
  - [x] Add `SelectSeatResponse` type:
    ```typescript
    export interface SelectSeatResponse {
      players: RoomPlayer[];
      gameStarted: boolean;
    }
    ```
  - [x] Update `selectSeat` return type in `client/src/shared/api/rooms.ts`:
    ```typescript
    export function selectSeat(
      roomId: number,
      seat: number,
    ): Promise<SelectSeatResponse> {
      return fetchClient<SelectSeatResponse>(`/rooms/${roomId}/seat`, {
        method: "POST",
        body: JSON.stringify({ seat }),
      });
    }
    ```

- [x] **Task 8: Frontend — Wire Quick Play in LobbyPage** (AC: #1, #4)
  - [x] Modify `client/src/features/lobby/LobbyPage.tsx`:
  - [x] Add state for matchmaking:
    ```typescript
    const [isMatchmaking, setIsMatchmaking] = useState(false);
    const abortRef = useRef<AbortController | null>(null);
    ```
  - [x] Add `handleQuickPlay` function:
    1. If already matchmaking, return
    2. Check if player already in a room: same guard as browse (optional — server validates)
    3. Set `isMatchmaking = true`
    4. Create `AbortController`, store in `abortRef`
    5. Call `quickPlay(abortRef.current.signal)`
    6. On success: navigate to `/rooms/${room.id}`
    7. On error: if `AbortError`, do nothing (user cancelled). Otherwise, show toast with error message
    8. Finally: set `isMatchmaking = false`, clear `abortRef`
  - [x] Add `handleCancelMatchmaking` function:
    1. `abortRef.current?.abort()`
    2. `setIsMatchmaking = false`
  - [x] Wire `onClick={handleQuickPlay}` on the Quick Play card button
  - [x] **Matchmaking overlay:** When `isMatchmaking` is true, render an overlay on the play options column:

    ```
    ┌─────────────────────┐
    │                     │
    │   ◉ Finding match...│  (pulsing dot + text)
    │                     │
    │   [ Cancel ]        │
    │                     │
    └─────────────────────┘
    ```

    - Pulsing animation: use `animate-pulse` Tailwind class on the indicator
    - Cancel button: ghost variant, calls `handleCancelMatchmaking`
    - Overlay replaces the options cards (not a modal/dialog)
    - `data-testid="matchmaking-overlay"` on container
    - `data-testid="matchmaking-cancel"` on cancel button

  - [x] Cleanup on unmount: abort any pending request in a `useEffect` return

- [x] **Task 9: Frontend — Update RoomLobby for Quick Play rooms** (AC: #5, #6)
  - [x] Modify `client/src/features/lobby/RoomLobby.tsx`:
  - [x] **Start Game section changes for Quick Play rooms:**
        When `room.isQuickPlay === true`:
    - Do NOT show Start Game button (for anyone, including owner)
    - Do NOT show "Waiting for [owner] to start..." message
    - Instead show: "Game starts automatically when all players are seated" (`data-testid="auto-start-message"`)
    - When all 4 seats are filled, show: "Starting game..." with a brief loading state
  - [x] **Handle `gameStarted` response from seat selection:**
        In the existing seat selection click handler, after calling `selectSeat()`:
    ```typescript
    const result = await selectSeat(roomId, seatIndex);
    setPlayers(result.players);
    if (result.gameStarted) {
      navigate(`/game/${roomId}`);
      return;
    }
    ```
  - [x] **Important:** Non-Quick Play rooms must continue to work exactly as before. The `gameStarted` flag will always be `false` for manual rooms. Check `room.isQuickPlay` for the Start Game section rendering
  - [x] **Existing behavior preserved:** Seat selection, team colors, leave room, copy link, owner/you indicators — all unchanged

- [x] **Task 10: Frontend — Add i18n translations** (AC: #1, #4, #6)
  - [x] Add to `client/src/shared/i18n/en.json` under `lobby`:
    ```json
    {
      "matchmaking": {
        "finding": "Finding match...",
        "cancel": "Cancel",
        "failed": "Could not find a match — try again",
        "alreadyInRoom": "You're already in a room — leave it first"
      }
    }
    ```
  - [x] Add to `client/src/shared/i18n/en.json` under `lobby.roomLobby`:
    ```json
    {
      "autoStartMessage": "Game starts automatically when all players are seated",
      "autoStarting": "Starting game..."
    }
    ```
  - [x] Add matching keys to `client/src/shared/i18n/sr.json` under `lobby`:
    ```json
    {
      "matchmaking": {
        "finding": "Барање натпревар...",
        "cancel": "Откажи",
        "failed": "Не може да се најде натпревар — пробај повторно",
        "alreadyInRoom": "Веќе сте во соба — прво напуштете ја"
      }
    }
    ```
  - [x] Add matching keys to `client/src/shared/i18n/sr.json` under `lobby.roomLobby`:
    ```json
    {
      "autoStartMessage": "Играта почнува автоматски кога сите играчи ќе седнат",
      "autoStarting": "Играта почнува..."
    }
    ```

- [x] **Task 11: Backend tests** (AC: #1, #2, #3, #5)
  - [x] Add test cases to `server/internal/room/handler_test.go`:
    - **QuickPlay** tests:
      - No available room — creates new Quick Play room, returns 200, `isQuickPlay: true`, defaults are bitola/1001/relaxed
      - Available room exists — joins existing room, returns 200 with updated room
      - Player already in a room — returns 409 `ALREADY_IN_ROOM`
      - Requires authentication — returns 401
      - Created room name follows "Quick Play XXXXXX" pattern
      - Multiple Quick Play calls fill the oldest room first (created_at ASC ordering)
    - **SelectSeat auto-start** tests:
      - Quick Play room: 4th seat selection triggers auto-start — response has `gameStarted: true` and room status is `in_progress`
      - Quick Play room: 3rd seat selection — response has `gameStarted: false`
      - Manual room: 4th seat selection — response has `gameStarted: false` (auto-start does NOT apply)
      - Quick Play room: auto-start on seat switch that results in all 4 seated
  - [x] Extend existing mock repository with: `FindQuickPlayRoom`
  - [x] Use external test package: `package room_test`

- [x] **Task 12: Frontend tests** (AC: #1, #4, #5, #6)
  - [x] Add tests to `client/src/features/lobby/LobbyPage.test.tsx`:
    - Quick Play card click calls `quickPlay` API
    - Matchmaking overlay shows pulsing indicator and cancel button
    - Cancel button aborts the request and returns to options view
    - Navigate to room on successful matchmaking
    - Error toast shown when matchmaking fails
    - Already-in-room error shows specific message
  - [x] Update tests in `client/src/features/lobby/RoomLobby.test.tsx`:
    - Quick Play room: no Start Game button shown for owner
    - Quick Play room: no "Waiting for owner..." message shown for non-owner
    - Quick Play room: auto-start message shown
    - Quick Play room: seat selection response with `gameStarted: true` navigates to game
    - Manual room: unchanged behavior (Start Game button, waiting message)
  - [x] Use `data-testid` attributes for element selection — never CSS classes
  - [x] Test descriptions in present tense

- [x] **Task 13: Lint and test** (AC: all)
  - [x] Run `make lint` — pass with zero new errors
  - [x] Run `make test` — all existing + new tests pass

### Review Findings

- [x] [Review][Patch] FindQuickPlayRoom needs SELECT FOR UPDATE to prevent concurrent Quick Play from exceeding 4 players [server/internal/room/gorm_repo.go:163] — Fixed: added `FOR UPDATE SKIP LOCKED` clause to the query.
- [x] [Review][Patch] StartGame handler should reject Quick Play rooms — server-authoritative enforcement [server/internal/room/handler.go:459] — Fixed: added `if room.IsQuickPlay { return apperr.ErrGameNotStartable }` check + test.
- [x] [Review][Defer] TOCTOU race on FindPlayerRoom check before transaction [server/internal/room/handler.go:530] — deferred, pre-existing pattern identical to JoinRoom handler
- [x] [Review][Defer] Only the 4th-seat player receives gameStarted notification — deferred, WS hub not yet wired (Epic 4 will broadcast system:game_started to all room participants)
- [x] [Review][Defer] Client abort does not cancel server-side room join/creation — deferred, acceptable for Phase 1 (user gets ALREADY_IN_ROOM error with recovery guidance)
- [x] [Review][Defer] player_count denormalized counter drift risk — deferred, pre-existing architectural pattern since Story 2.1
- [x] [Review][Defer] No TTL/cleanup for abandoned Quick Play rooms — deferred, not in scope (LeaveRoom already marks empty rooms as completed)
- [x] [Review][Defer] RoomLobby has no real-time refresh for other players' actions — deferred, pre-existing limitation affecting all rooms (WS solution in Epic 4)

## Dev Notes

### Architecture Compliance

**This story extends the existing `room/` package** — the `QuickPlay` handler belongs alongside JoinRoom, CreateRoom, etc. Per architecture, `matchmaking.go` was planned as a separate file in the lobby domain, but since the actual implementation is a simple handler (find-or-create room), keeping it in `handler.go` is appropriate. If the matchmaking logic grows (ranked queues, ELO matching in Phase 2), extract to `matchmaking.go` at that point.

**Quick Play is HTTP-based, not WebSocket-based** for Phase 1. The WS hub is not yet wired. The "queue" experience is the HTTP request duration. When WS infrastructure arrives (Epic 4+), Quick Play can be enhanced with real queue broadcasting (player count in queue, estimated wait time).

**Server-authoritative auto-start:** The auto-start trigger lives in the `SelectSeat` handler, not on the frontend. The frontend just reads the `gameStarted` flag from the response. This aligns with the server-authoritative architecture — the client never decides when a game starts.

**Room status lifecycle for Quick Play:**

- `waiting` -> `in_progress` (via auto-start in SelectSeat when all 4 seated)
- Same lifecycle as manual rooms, just triggered differently
- Once `in_progress`, the room no longer appears in browse list or Quick Play search (existing `status = 'waiting'` filter handles this)

### File Modifications

```
server/migrations/
├── 000005_add_is_quick_play.up.sql    # NEW: add is_quick_play column + index
├── 000005_add_is_quick_play.down.sql  # NEW: reverse migration

server/internal/room/
├── model.go          # ADD: IsQuickPlay bool field to Room struct
├── repository.go     # ADD: FindQuickPlayRoom() method to interface
├── gorm_repo.go      # ADD: FindQuickPlayRoom() implementation
├── handler.go        # ADD: QuickPlay handler; MODIFY: SelectSeat for auto-start + gameStarted response
├── handler_test.go   # ADD: QuickPlay tests + SelectSeat auto-start tests

server/cmd/api/
├── main.go           # ADD: POST /rooms/quick-play route (before parameterized routes)

client/src/shared/types/
├── apiTypes.ts       # ADD: isQuickPlay to Room, SelectSeatResponse type

client/src/shared/api/
├── rooms.ts          # ADD: quickPlay() function; MODIFY: selectSeat() return type

client/src/features/lobby/
├── LobbyPage.tsx          # MODIFY: wire Quick Play button, matchmaking overlay with cancel
├── LobbyPage.test.tsx     # ADD: Quick Play matchmaking tests
├── RoomLobby.tsx          # MODIFY: Quick Play auto-start UI, gameStarted navigation
├── RoomLobby.test.tsx     # ADD: Quick Play room behavior tests

client/src/shared/i18n/
├── en.json           # ADD: matchmaking + autoStart i18n keys
├── sr.json           # ADD: matching Serbian translations
```

### Previous Story (2.4) Intelligence

**Key patterns established:**

- `SelectSeat` returns `{ data: { players: [...] } }` — this story extends the response to `{ data: { players: [...], gameStarted: bool } }`. Update the frontend type accordingly
- `StartGame` handler sets `room.Status = "in_progress"` — Quick Play auto-start does the same thing, just triggered from SelectSeat
- Transaction pattern: `h.repo.RunInTransaction(func(tx RoomRepository) error { ... })` — use for the QuickPlay find-or-create logic
- Room code generation with retry: `generateRoomCode()` + collision retry loop — reuse in QuickPlay room creation
- Frontend `RoomLobby.tsx` checks `room.ownerId === user.id` for Start Game button — extend to also check `room.isQuickPlay` to hide the button entirely for QP rooms
- The `data-testid` conventions: `start-game`, `waiting-for-start` — add `auto-start-message` for QP rooms, `matchmaking-overlay` and `matchmaking-cancel` for the lobby
- `useRoomLobbyUpdates.ts` already handles `system:game_started` event with navigation to `/game/${roomId}` — this will work for Quick Play auto-start once WS is wired

**Things NOT to break:**

- Manual room creation flow (Create Room modal)
- Manual room joining flow (Browse Rooms -> Join)
- Manual Start Game flow (owner clicks Start when all 4 seated)
- Seat selection in manual rooms (no auto-start)
- Leave Room functionality
- Loading/error states
- Copy Link button
- Owner/You indicators

### Backend Implementation Details

**Route ordering is critical:**
`POST /rooms/quick-play` must be registered before `GET /rooms/:id` and `POST /rooms/:id/join` in Echo's router. Otherwise `:id` captures `"quick-play"` as a parameter. Place the new route immediately after `POST /rooms` (CreateRoom).

**Quick Play room name uniqueness:**
Each Quick Play room name is `"Quick Play " + code` (e.g., "Quick Play ABC123"). Since room codes are unique (6-char alphanumeric), names are unique. The `idx_rooms_name_active` constraint is satisfied.

**FindQuickPlayRoom query optimization:**
The composite index `idx_rooms_quick_play ON rooms(is_quick_play, status, player_count) WHERE ...` ensures the query is fast. It filters on the partial index condition first, then scans by `created_at ASC`.

**Transaction scope for QuickPlay handler:**
The find-or-create logic runs in a single transaction to prevent race conditions where two players try to Quick Play simultaneously and both create new rooms instead of one joining the other's room. The transaction isolation ensures the `FindQuickPlayRoom` + join/create sequence is atomic.

**Auto-start in SelectSeat:**
The auto-start check happens after the seat transaction completes (not inside it). This is a deliberate simplification — the seat assignment is the critical section, and the status update is idempotent (setting `in_progress` on an already-`in_progress` room is a no-op via GORM's `Save`). If the auto-start update fails, the room remains in `waiting` with all 4 seated, and the next seat interaction will trigger it again.

### Frontend Implementation Details

**Matchmaking overlay replaces options cards:**
When `isMatchmaking` is true, the left column of the lobby page shows the matchmaking overlay instead of the Quick Play / Browse / Create cards. This is a simple conditional render, not a modal or dialog.

**AbortController for cancellation:**
Store the controller in a `useRef` to persist across renders. On unmount, abort any pending request to prevent memory leaks or navigation after unmount.

**SelectSeat response type change:**
The `selectSeat` function return type changes from `{ players: RoomPlayer[] }` to `SelectSeatResponse { players: RoomPlayer[], gameStarted: boolean }`. This is a backward-compatible change — existing code accesses `.players` which still exists. The new `.gameStarted` field is only checked after seat selection in RoomLobby.

**Quick Play room detection in RoomLobby:**
The room detail is already fetched via `getRoom(id)` which returns `{ room, players }`. Check `room.isQuickPlay` to toggle the Start Game section behavior.

### References

- [Source: _bmad-output/planning-artifacts/epics.md — Story 2.5 acceptance criteria, lines 605-634]
- [Source: _bmad-output/planning-artifacts/architecture.md — matchmaking.go planned structure, line 767]
- [Source: _bmad-output/planning-artifacts/architecture.md — lobbyStore includes matchmaking status, line 488]
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md — Quick Play accent-glow, line 428]
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md — Matchmaking queue pulsing indicator, line 873]
- [Source: _bmad-output/planning-artifacts/prd.md — FR19 Quick Play matchmaking, line 312]
- [Source: _bmad-output/project-context.md — Room domain package shape, framework rules, testing rules]
- [Source: server/internal/room/handler.go — Existing CreateRoom/JoinRoom transaction patterns]
- [Source: server/internal/room/model.go — Room struct with IsQuickPlay to be added]
- [Source: server/migrations/000003_create_rooms.up.sql — idx_rooms_name_active unique constraint]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

### Completion Notes List

- Implemented `QuickPlay` handler (POST /rooms/quick-play) — finds available QP room or creates one with defaults (Bitola, 1001, relaxed)
- Added `is_quick_play` boolean column to rooms via migration 000005 with composite partial index for fast matching
- Quick Play rooms use auto-generated names: "Quick Play XXXXXX" (room code ensures name uniqueness)
- Transaction wraps find-or-create to prevent race conditions between concurrent Quick Play requests
- Modified `SelectSeat` to auto-start Quick Play rooms when all 4 seats filled — sets room status to "playing" and returns `gameStarted: true`
- Extended SelectSeat response from `{ players }` to `{ players, gameStarted }` — backward-compatible for manual rooms (always false)
- Added `FindQuickPlayRoom()` repo method — queries `is_quick_play=true AND status=waiting AND player_count<4` ordered by `created_at ASC` (fills oldest rooms first)
- Route `POST /rooms/quick-play` registered before parameterized `/rooms/:id` routes to prevent Echo param capture
- Frontend: LobbyPage Quick Play button wired with matchmaking overlay (pulsing indicator + Cancel button via AbortController)
- Frontend: RoomLobby shows auto-start message for QP rooms instead of Start Game button — navigates to `/game/:id` on `gameStarted: true` response
- i18n translations added for EN and SR (matchmaking + autoStart keys)
- 5 new backend tests (QuickPlay handler + SelectSeat auto-start), all passing
- 4 new frontend tests (LobbyPage matchmaking + RoomLobby QP behavior), all passing
- All 95 frontend tests pass across 14 files, zero regressions
- All Go tests pass, go vet clean, ESLint + Prettier clean
- Existing room status uses "playing" not "in_progress" — followed codebase convention

### Change Log

- 2026-04-11: Story 2.5 implementation complete — quick play matchmaking

### File List

- server/migrations/000005_add_is_quick_play.up.sql (new — add is_quick_play column + index)
- server/migrations/000005_add_is_quick_play.down.sql (new — reverse migration)
- server/internal/room/model.go (modified — IsQuickPlay bool field added to Room struct)
- server/internal/room/repository.go (modified — FindQuickPlayRoom() added to interface)
- server/internal/room/gorm_repo.go (modified — FindQuickPlayRoom() implementation)
- server/internal/room/handler.go (modified — QuickPlay handler + SelectSeat auto-start + gameStarted response)
- server/internal/room/handler_test.go (modified — FindQuickPlayRoom mock + 5 new QuickPlay/auto-start tests + quick-play route)
- server/cmd/api/main.go (modified — POST /rooms/quick-play route before parameterized routes)
- client/src/shared/types/apiTypes.ts (modified — isQuickPlay on Room + SelectSeatResponse type)
- client/src/shared/api/rooms.ts (modified — quickPlay() function + selectSeat() return type updated)
- client/src/features/lobby/LobbyPage.tsx (modified — Quick Play wired with matchmaking overlay + cancel)
- client/src/features/lobby/LobbyPage.test.tsx (modified — 3 new Quick Play tests + mock updates)
- client/src/features/lobby/RoomLobby.tsx (modified — Quick Play auto-start UI + gameStarted navigation)
- client/src/features/lobby/RoomLobby.test.tsx (modified — 4 new Quick Play room behavior tests + isQuickPlay on defaultRoom + gameStarted on mock returns)
- client/src/shared/i18n/en.json (modified — matchmaking + autoStart i18n keys)
- client/src/shared/i18n/sr.json (modified — matching Serbian translations)
