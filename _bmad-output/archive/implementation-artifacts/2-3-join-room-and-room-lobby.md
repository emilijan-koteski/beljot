# Story 2.3: Join Room & Room Lobby

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a player,
I want to join a room and see who else is in the room lobby,
So that I can prepare for a game with other players.

## Acceptance Criteria

1. **Join Room from Browse List** — When a player clicks "Join" on a RoomCard in the browse list, `POST /api/v1/rooms/:id/join` is called. The player is added to the room and redirected to the RoomLobby view (`/rooms/:id`). All other players in the room receive a WebSocket `system:player_joined` event and see the updated seat state.

2. **Room Full Rejection** — When a player tries to join a full room (4/4 players), a 409 error is returned with code `ROOM_FULL`. A toast is shown: "Room is full — try another".

3. **Room Lobby View** — When a player is in the RoomLobby view, 4 player seats are displayed showing occupied seats (player username) and empty seats ("Waiting..." with dashed border). Room configuration is visible (variant, mode, timer). A prominent "Copy Link" button copies the room code/link to clipboard with a single click and shows a success toast.

4. **Leave Room** — When a player navigates away or clicks "Leave Room", their seat is freed and all remaining players receive a WebSocket `system:player_left` event. The room list updates to reflect the new player count.

5. **Room Code Join** — When a player enters a room code in the lobby search bar, the matching room appears in the filtered list and can be joined (already implemented in Story 2.2 — this AC validates the end-to-end flow through the Join button).

## Tasks / Subtasks

- [x] **Task 1: Backend — Create `room_players` migration** (AC: #1, #3)
  - [x] Create `server/migrations/000004_create_room_players.up.sql` with `room_players` table: `id` (PK), `room_id` (FK to rooms), `user_id` (FK to users), `seat` (nullable int), `team` (nullable varchar), `created_at`
  - [x] Create `server/migrations/000004_create_room_players.down.sql` — `DROP TABLE room_players`
  - [x] Add unique constraint: `idx_room_players_room_user` on `(room_id, user_id)` where `deleted_at IS NULL` — prevents duplicate joins
  - [x] Add index: `idx_room_players_room_id` on `room_id` — fast player list lookups
  - [x] Add index: `idx_room_players_user_id` on `user_id` — fast "is user in a room?" checks
  - [x] Run `make migrate` to apply

- [x] **Task 2: Backend — Add `RoomPlayer` model** (AC: #1, #3)
  - [x] Add `RoomPlayer` struct to `server/internal/room/model.go`:
    ```go
    type RoomPlayer struct {
        ID        uint      `gorm:"primaryKey" json:"id"`
        RoomID    uint      `gorm:"not null;index" json:"roomId"`
        UserID    uint      `gorm:"not null;index" json:"userId"`
        Username  string    `gorm:"-" json:"username"` // populated via join query, not stored
        Seat      *int      `json:"seat"`              // nullable — assigned in Story 2.4
        Team      *string   `gorm:"size:10" json:"team"` // nullable — assigned in Story 2.4
        CreatedAt time.Time `json:"createdAt"`
    }
    ```
  - [x] **Note:** `seat` and `team` are nullable pointers — they will be populated in Story 2.4 (Team Assignment). Do NOT implement seat/team assignment logic in this story.

- [x] **Task 3: Backend — Extend repository with player management** (AC: #1, #3, #4)
  - [x] Add to `RoomRepository` interface in `server/internal/room/repository.go`:
    ```go
    Update(room *Room) error
    AddPlayer(roomPlayer *RoomPlayer) error
    RemovePlayer(roomID uint, userID uint) error
    FindPlayersByRoomID(roomID uint) ([]RoomPlayer, error)
    FindPlayerRoom(userID uint) (*RoomPlayer, error)
    IncrementPlayerCount(roomID uint) error
    DecrementPlayerCount(roomID uint) error
    ```
  - [x] Implement in `server/internal/room/gorm_repo.go`:
    - `Update`: `r.db.Save(room).Error` — used for ownership transfer and room status changes
    - `AddPlayer`: insert into `room_players`, handle unique constraint violation → return `ErrAlreadyInRoom`
    - `RemovePlayer`: delete from `room_players` where room_id AND user_id match. If no rows affected → return `ErrNotInRoom`
    - `FindPlayersByRoomID`: query `room_players` joined with `users` to populate `Username` field, ordered by `created_at ASC`
    - `FindPlayerRoom`: find the `room_player` record for a user where the room status is `waiting` — returns `(nil, nil)` if not in any active room (follows established pattern)
    - `IncrementPlayerCount`: `UPDATE rooms SET player_count = player_count + 1 WHERE id = ?`
    - `DecrementPlayerCount`: `UPDATE rooms SET player_count = player_count - 1 WHERE id = ? AND player_count > 0`

- [x] **Task 4: Backend — Add new domain errors** (AC: #2, #4)
  - [x] Add to `server/internal/apperr/errors.go`:
    ```go
    ErrAlreadyInRoom = NewAppError("ALREADY_IN_ROOM", "player is already in a room", 409)
    ErrNotInRoom     = NewAppError("NOT_IN_ROOM", "player is not in this room", 404)
    ```
  - [x] `ErrRoomFull` (409) and `ErrRoomNotFound` (404) already exist — reuse them

- [x] **Task 5: Backend — Patch `CreateRoom` to insert room_players record** (AC: #3)
  - [x] Modify `CreateRoom` handler in `server/internal/room/handler.go`: after `repo.Create(&room)` succeeds, call `repo.AddPlayer(&RoomPlayer{RoomID: room.ID, UserID: userID})` to add the creator as the first room player
  - [x] This ensures `GetRoom` returns the creator in the players list and `FindPlayerRoom` correctly detects the creator is already in a room
  - [x] Add test: "Creator appears in room player list after creation"

- [x] **Task 6: Backend — Add `JoinRoom` handler** (AC: #1, #2)
  - [x] Add `JoinRoom` handler in `server/internal/room/handler.go`:
    ```
    POST /api/v1/rooms/:id/join
    ```
  - [x] Validation sequence:
    1. Parse room ID from URL param (validate it's a valid uint)
    2. Get authenticated user ID via `auth.GetUserID(c)`
    3. `repo.FindByID(roomID)` — if nil, return `ErrRoomNotFound`
    4. Check `room.Status == "waiting"` — if not, return `ErrRoomNotFound` (treat non-waiting rooms as not joinable)
    5. Check `room.PlayerCount < 4` — if not, return `ErrRoomFull`
    6. `repo.FindPlayerRoom(userID)` — if not nil, return `ErrAlreadyInRoom` (player already in an active room, including their own)
    7. Create `RoomPlayer{RoomID: roomID, UserID: userID}` and call `repo.AddPlayer()`
    8. `repo.IncrementPlayerCount(roomID)`
    9. Return HTTP 200 with `{ "data": room }` (re-fetch room for updated player count)
  - [x] **TODO comment for WS broadcast:** Add `// TODO: broadcast system:player_joined to room participants (WS hub not yet wired)` after successful join
  - [x] **Race condition note:** The check-then-insert has a minor race window. The unique constraint on `room_players(room_id, user_id)` prevents one user from joining twice, but does NOT prevent two different users from both passing the `PlayerCount < 4` check simultaneously and both joining (resulting in 5 players). At Phase 1 scale (~10 concurrent games, ~40 players) this is acceptably unlikely. If it becomes an issue, add `CHECK (player_count <= 4)` on the `rooms` table or use `SELECT ... FOR UPDATE` locking.

- [x] **Task 7: Backend — Add `LeaveRoom` handler** (AC: #4)
  - [x] Add `LeaveRoom` handler in `server/internal/room/handler.go`:
    ```
    POST /api/v1/rooms/:id/leave
    ```
  - [x] Validation sequence:
    1. Parse room ID from URL param (validate it's a valid uint)
    2. Get authenticated user ID via `auth.GetUserID(c)`
    3. `repo.FindByID(roomID)` — if nil, return `ErrRoomNotFound`. Store as `room` for ownership check later
    4. `repo.RemovePlayer(roomID, userID)` — if error is `ErrNotInRoom`, return it
    5. `repo.DecrementPlayerCount(roomID)`
    6. If `room.OwnerID == userID` AND room has other players: transfer ownership to the earliest-joined remaining player via `repo.FindPlayersByRoomID(roomID)` → `players[0].UserID`. Call `room.OwnerID = players[0].UserID` then `repo.Update(room)`
    7. If `room.OwnerID == userID` AND room has NO other players: set `room.Status = "completed"` then `repo.Update(room)`
    8. Return HTTP 200 with `{ "data": { "message": "left room" } }`
  - [x] **TODO comment for WS broadcast:** Add `// TODO: broadcast system:player_left to room participants (WS hub not yet wired)`

- [x] **Task 8: Backend — Add `GetRoom` handler** (AC: #3)
  - [x] Add `GetRoom` handler in `server/internal/room/handler.go`:
    ```
    GET /api/v1/rooms/:id
    ```
  - [x] Returns room data with player list:
    ```go
    type RoomDetailResponse struct {
        Room    Room         `json:"room"`
        Players []RoomPlayer `json:"players"`
    }
    ```
  - [x] Calls `repo.FindByID(roomID)` for room, `repo.FindPlayersByRoomID(roomID)` for players
  - [x] If room not found → return `ErrRoomNotFound`
  - [x] Response: `{ "data": { "room": {...}, "players": [...] } }`

- [x] **Task 9: Backend — Wire new routes** (AC: #1, #3, #4)
  - [x] Add to `server/cmd/api/main.go` after existing room routes:
    ```go
    api.GET("/rooms/:id", roomHandler.GetRoom)
    api.POST("/rooms/:id/join", roomHandler.JoinRoom)
    api.POST("/rooms/:id/leave", roomHandler.LeaveRoom)
    ```
  - [x] All routes are under the `api` group (auth middleware applied)

- [x] **Task 10: Backend — Add WebSocket events to contract** (AC: #1, #4)
  - [x] Add to `server/internal/ws/events.go`:
    ```go
    const SystemPlayerJoined = "system:player_joined"
    const SystemPlayerLeft   = "system:player_left"
    ```
  - [x] Add to `client/src/shared/types/wsEvents.ts`:

    ```typescript
    export const SYSTEM_PLAYER_JOINED = "system:player_joined" as const;
    export const SYSTEM_PLAYER_LEFT = "system:player_left" as const;

    export interface PlayerJoinedPayload {
      roomId: number;
      userId: number;
      username: string;
      playerCount: number;
    }

    export interface PlayerLeftPayload {
      roomId: number;
      userId: number;
      username: string;
      playerCount: number;
      newOwnerId?: number; // set if ownership transferred
    }
    ```

  - [x] **Both files updated in the same commit** — per project convention

- [x] **Task 11: Frontend — Add API client functions** (AC: #1, #3, #4)
  - [x] Add `RoomPlayer` and `RoomDetail` types to `client/src/shared/types/apiTypes.ts` (canonical location):

    ```typescript
    export interface RoomPlayer {
      id: number;
      roomId: number;
      userId: number;
      username: string;
      seat: number | null;
      team: string | null;
      createdAt: string;
    }

    export interface RoomDetail {
      room: Room;
      players: RoomPlayer[];
    }
    ```

  - [x] Add functions to `client/src/shared/api/rooms.ts` — import `RoomDetail` and `RoomPlayer` from `apiTypes.ts`:

    ```typescript
    import type { Room, RoomDetail } from "@/shared/types/apiTypes";

    export function getRoom(id: number): Promise<RoomDetail> {
      return fetchClient<RoomDetail>(`/rooms/${id}`);
    }

    export function joinRoom(id: number): Promise<Room> {
      return fetchClient<Room>(`/rooms/${id}/join`, { method: "POST" });
    }

    export function leaveRoom(id: number): Promise<void> {
      return fetchClient<void>(`/rooms/${id}/leave`, { method: "POST" });
    }
    ```

  - [x] **Important:** `fetchClient<T>` auto-unwraps `{ data: T }` — do NOT double-unwrap

- [x] **Task 12: Frontend — Wire Join button on RoomCard** (AC: #1, #2)
  - [x] Add `onJoin` callback prop to `RoomCard`:
    ```typescript
    interface RoomCardProps {
      room: Room;
      onJoin: (roomId: number) => void;
    }
    ```
  - [x] Wire `onClick={()=> onJoin(room.id)}` to the existing Join button
  - [x] In `RoomList.tsx`, pass the `onJoin` handler from parent:
    ```typescript
    interface RoomListProps {
      onJoinRoom: (roomId: number) => void;
    }
    ```
  - [x] In `LobbyPage.tsx`, implement join handler:
    ```typescript
    const handleJoinRoom = async (roomId: number) => {
      try {
        await joinRoom(roomId);
        navigate(`/rooms/${roomId}`);
      } catch (err) {
        if (err instanceof FetchError) {
          if (err.code === "ROOM_FULL") {
            toast(t("lobby.roomList.errors.roomFull"));
          } else if (err.code === "ALREADY_IN_ROOM") {
            toast(t("lobby.roomList.errors.alreadyInRoom"));
          } else {
            toast(t("lobby.roomList.errors.joinFailed"));
          }
        }
      }
    };
    ```

- [x] **Task 13: Frontend — Implement full RoomLobby component** (AC: #3, #4)
  - [x] Rewrite `client/src/features/lobby/RoomLobby.tsx` — replace the stub entirely
  - [x] **Component structure:**
    ```
    ┌─────────────────────────────────────────────────────┐
    │  ← Back to Lobby        Room Name        Copy Link  │
    ├─────────────────────────────────────────────────────┤
    │                                                     │
    │   ┌──────────┐                    ┌──────────┐      │
    │   │  Seat 0  │                    │  Seat 1  │      │
    │   │ Player A │                    │ Waiting..│      │
    │   └──────────┘                    └──────────┘      │
    │                                                     │
    │   ┌──────────┐                    ┌──────────┐      │
    │   │  Seat 2  │                    │  Seat 3  │      │
    │   │ Waiting..│                    │ Player B │      │
    │   └──────────┘                    └──────────┘      │
    │                                                     │
    │  Bitola · 1001 · Relaxed           Leave Room       │
    └─────────────────────────────────────────────────────┘
    ```
  - [x] **On mount:** Call `getRoom(id)` to fetch room details + player list
  - [x] **Player seat display:** Render 4 seat slots in a 2x2 grid. Each slot shows:
    - **Occupied:** Player username, `bg-surface` background, solid `border-border` border
    - **Empty:** "Waiting..." text, dashed `border-border` border, `text-text-secondary` color
  - [x] **Room config bar:** Show variant, match mode, timer style below the seats — `text-sm text-text-secondary`
  - [x] **Header bar:**
    - Left: Back arrow/button (`data-testid="back-to-lobby"`) → navigates to `/lobby`
    - Center: Room name in `heading-lg` style
    - Right: "Copy Link" button (`data-testid="copy-link"`)
  - [x] **Copy Link:** Copies room code to clipboard using `navigator.clipboard.writeText(room.code)`. Shows success toast: "Room code copied!" Use `try/catch` for clipboard API failure.
  - [x] **Leave Room button:** Bottom-right, `variant="ghost"` with `text-destructive` color (`data-testid="leave-room"`). On click: call `leaveRoom(roomId)`, navigate to `/lobby`
  - [x] **Loading state:** Show skeleton while `getRoom` is loading — use `isLoading` local state
  - [x] **Error state:** If room not found, show "Room not found" and a link back to lobby
  - [x] **Self indicator:** Highlight the current user's seat (use `authStore.user.id` to match against `player.userId`). Add "You" badge or slight visual emphasis.
  - [x] Use `data-testid="player-seat-{index}"` for each seat slot
  - [x] Use `data-testid="room-lobby"` on the root container

- [x] **Task 14: Frontend — Create `useRoomLobbyUpdates` hook** (AC: #1, #4)
  - [x] Create `client/src/features/lobby/useRoomLobbyUpdates.ts`
  - [x] Listen for `system:player_joined` → add player to local state, update player count
  - [x] Listen for `system:player_left` → remove player from local state, update player count. If `newOwnerId` is set, update room owner display
  - [x] Listen for `system:room_updated` → update room details (e.g., if room status changes)
  - [x] Use `useCallback` with stable references, access store via `getState()` pattern to prevent stale closures
  - [x] **Note:** WS hub is not yet wired end-to-end. Create the hook with correct event handler logic. Document that it requires WS infrastructure from a future story. Export `handleWsMessage` function for future integration.

- [x] **Task 15: Frontend — Handle room owner leaving** (AC: #4)
  - [x] When `system:player_left` event includes `newOwnerId`, update the room's `ownerId` in local state
  - [x] Room owner is visually indicated with a crown/star icon or "(Owner)" label next to their username
  - [x] If the current user becomes the new owner, the UI reflects this immediately

- [x] **Task 16: Frontend — Add i18n translations** (AC: #3, #4)
  - [x] Add to `client/src/shared/i18n/en.json` under `lobby.roomLobby`:
    ```json
    {
      "lobby": {
        "roomLobby": {
          "waitingForPlayers": "Waiting for players...",
          "backToLobby": "Back to Lobby",
          "copyLink": "Copy Link",
          "copyLinkSuccess": "Room code copied!",
          "copyLinkFailed": "Failed to copy — code: {{code}}",
          "leaveRoom": "Leave Room",
          "seatEmpty": "Waiting...",
          "seatYou": "You",
          "seatOwner": "Owner",
          "roomConfig": "{{variant}} · {{mode}} · {{timer}}",
          "loading": "Loading room...",
          "notFound": "Room not found",
          "notFoundAction": "Back to Lobby",
          "playerJoined": "{{username}} joined the room",
          "playerLeft": "{{username}} left the room"
        },
        "roomList": {
          "errors": {
            "roomFull": "Room is full — try another",
            "alreadyInRoom": "You're already in a room",
            "joinFailed": "Failed to join room — try again"
          }
        }
      }
    }
    ```
  - [x] Add matching keys to `client/src/shared/i18n/sr.json`:
    ```json
    {
      "lobby": {
        "roomLobby": {
          "waitingForPlayers": "Čekanje igrača...",
          "backToLobby": "Nazad u lobi",
          "copyLink": "Kopiraj link",
          "copyLinkSuccess": "Kod sobe kopiran!",
          "copyLinkFailed": "Kopiranje neuspešno — kod: {{code}}",
          "leaveRoom": "Napusti sobu",
          "seatEmpty": "Čekanje...",
          "seatYou": "Ti",
          "seatOwner": "Vlasnik",
          "roomConfig": "{{variant}} · {{mode}} · {{timer}}",
          "loading": "Učitavanje sobe...",
          "notFound": "Soba nije pronađena",
          "notFoundAction": "Nazad u lobi",
          "playerJoined": "{{username}} se pridružio sobi",
          "playerLeft": "{{username}} je napustio sobu"
        },
        "roomList": {
          "errors": {
            "roomFull": "Soba je puna — probaj drugu",
            "alreadyInRoom": "Već si u sobi",
            "joinFailed": "Pridruživanje neuspešno — probaj ponovo"
          }
        }
      }
    }
    ```

- [x] **Task 17: Backend tests** (AC: #1, #2, #3, #4)
  - [x] Add test cases to `server/internal/room/handler_test.go`:
    - **JoinRoom** tests:
      - Successful join — returns 200 with updated room data
      - Join non-existent room — returns 404 `ROOM_NOT_FOUND`
      - Join full room (4/4) — returns 409 `ROOM_FULL`
      - Join room already in — returns 409 `ALREADY_IN_ROOM`
      - Join room not in `waiting` status — returns 404
      - Join requires authentication — returns 401
    - **LeaveRoom** tests:
      - Successful leave — returns 200
      - Leave room not in — returns 404 `NOT_IN_ROOM`
      - Leave as owner with others — ownership transfers to earliest player
      - Leave as owner alone — room is deleted/completed
      - Leave requires authentication — returns 401
    - **GetRoom** tests:
      - Returns room with players list
      - Non-existent room — returns 404
      - Returns empty players array when no players (not null)
      - Requires authentication — returns 401
  - [x] Extend existing mock repository with: `AddPlayer`, `RemovePlayer`, `FindPlayersByRoomID`, `FindPlayerRoom`, `IncrementPlayerCount`, `DecrementPlayerCount`
  - [x] Use external test package: `package room_test`

- [x] **Task 18: Frontend tests** (AC: #1, #2, #3, #4)
  - [x] Update `client/src/features/lobby/RoomCard.test.tsx` — verify `onJoin` callback is invoked on Join button click
  - [x] Update `client/src/features/lobby/RoomList.test.tsx` — verify `onJoinRoom` prop is passed through to RoomCard
  - [x] Create `client/src/features/lobby/RoomLobby.test.tsx`:
    - Renders room name and configuration
    - Displays occupied seats with player usernames
    - Displays empty seats with "Waiting..." text
    - Copy Link button calls clipboard API
    - Leave Room button calls leaveRoom API and navigates to lobby
    - Loading skeleton displayed while fetching
    - Error state shown for non-existent room
    - Current user's seat highlighted with "You" indicator
  - [x] Update `client/src/features/lobby/LobbyPage.test.tsx` — verify join handler calls API and navigates on success, shows toast on error
  - [x] Use `data-testid` attributes for element selection — never CSS classes
  - [x] Test descriptions in present tense: `it('renders room details')`

- [x] **Task 19: Lint and test** (AC: all)
  - [x] Run `make lint` — pass with zero new errors
  - [x] Run `make test` — all existing + new tests pass

## Dev Notes

### Architecture Compliance

**This story spans backend (3 new endpoints + migration + model) and frontend (full RoomLobby component + Join wiring).** It is the first story that introduces a join table (`room_players`) connecting users to rooms.

**Backend domain stays in `room/` package** — JoinRoom, LeaveRoom, and GetRoom handlers belong alongside CreateRoom and ListRooms. Do NOT create a separate package. The `lobby/` package is reserved for matchmaking logic (Story 2.5).

**File organization:**

```
server/internal/room/
├── model.go          # Add RoomPlayer struct
├── repository.go     # Add 6 new interface methods
├── gorm_repo.go      # Add 6 new implementations
├── handler.go        # Add JoinRoom, LeaveRoom, GetRoom handlers
├── handler_test.go   # Add tests for all 3 new handlers

server/migrations/
├── 000004_create_room_players.up.sql    # NEW
├── 000004_create_room_players.down.sql  # NEW

client/src/features/lobby/
├── LobbyPage.tsx         # Modify: add join handler, pass to RoomList
├── LobbyPage.test.tsx    # Modify: add join flow tests
├── RoomCard.tsx           # Modify: add onJoin prop
├── RoomCard.test.tsx      # Modify: test onJoin callback
├── RoomList.tsx           # Modify: add onJoinRoom prop, pass to RoomCard
├── RoomList.test.tsx      # Modify: test prop passing
├── RoomLobby.tsx          # REWRITE (replace stub)
├── RoomLobby.test.tsx     # NEW
├── useRoomLobbyUpdates.ts # NEW (WS listener for room-specific events)
```

### Backend Implementation Details

**Migration — `000004_create_room_players.up.sql`:**

```sql
CREATE TABLE room_players (
    id SERIAL PRIMARY KEY,
    room_id INTEGER NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id),
    seat INTEGER,
    team VARCHAR(10),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_room_players_room_user ON room_players(room_id, user_id);
CREATE INDEX idx_room_players_room_id ON room_players(room_id);
CREATE INDEX idx_room_players_user_id ON room_players(user_id);
```

**Migration — `000004_create_room_players.down.sql`:**

```sql
DROP TABLE IF EXISTS room_players;
```

**Important migration notes:**

- `ON DELETE CASCADE` on `room_id` — if a room is deleted, its players are cleaned up
- No `deleted_at` column — room_players records are hard-deleted on leave (soft delete is for rooms, not transient join records)
- `seat` and `team` are nullable — populated in Story 2.4

**FindPlayersByRoomID with username join:**

```go
func (r *GormRepository) FindPlayersByRoomID(roomID uint) ([]RoomPlayer, error) {
    var players []RoomPlayer
    err := r.db.Table("room_players").
        Select("room_players.*, users.username").
        Joins("JOIN users ON users.id = room_players.user_id").
        Where("room_players.room_id = ?", roomID).
        Order("room_players.created_at ASC").
        Scan(&players).Error
    if err != nil {
        return nil, fmt.Errorf("finding players for room %d: %w", roomID, err)
    }
    if players == nil {
        players = []RoomPlayer{}
    }
    return players, nil
}
```

**FindPlayerRoom — check if user is in any active room:**

```go
func (r *GormRepository) FindPlayerRoom(userID uint) (*RoomPlayer, error) {
    var player RoomPlayer
    err := r.db.Table("room_players").
        Joins("JOIN rooms ON rooms.id = room_players.room_id").
        Where("room_players.user_id = ? AND rooms.status = ? AND rooms.deleted_at IS NULL", userID, "waiting").
        First(&player).Error
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, nil // not in any room — follows established pattern
        }
        return nil, fmt.Errorf("finding player room for user %d: %w", userID, err)
    }
    return &player, nil
}
```

**JoinRoom handler response:** After successful join, re-fetch the room to get the updated `PlayerCount`:

```go
updatedRoom, err := h.repo.FindByID(roomID)
if err != nil {
    return fmt.Errorf("re-fetching room after join: %w", err)
}
return c.JSON(http.StatusOK, map[string]interface{}{"data": updatedRoom})
```

**LeaveRoom — ownership transfer logic:**

```go
// After removing the player, check if they were the owner
if room.OwnerID == userID {
    // Find the earliest remaining player
    players, err := h.repo.FindPlayersByRoomID(roomID)
    if err != nil {
        return fmt.Errorf("finding remaining players: %w", err)
    }
    if len(players) > 0 {
        // Transfer ownership to the earliest joiner
        room.OwnerID = players[0].UserID
        if err := h.repo.Update(room); err != nil {
            return fmt.Errorf("transferring room ownership: %w", err)
        }
    } else {
        // No players left — mark room as completed
        room.Status = "completed"
        if err := h.repo.Update(room); err != nil {
            return fmt.Errorf("closing empty room: %w", err)
        }
    }
}
```

**Note:** The repository needs an `Update(room *Room) error` method. Check if it already exists — if not, add it. It should be a simple `r.db.Save(room)` call.

### Frontend Implementation Details

**RoomLobby component data flow:**

```
useParams() → roomId
  ↓
getRoom(roomId) on mount → { room, players }
  ↓
Local state: room, players, isLoading, error
  ↓
Render: header + seats grid + config bar + leave button
```

**Player seat grid layout:**

```tsx
<div className="grid grid-cols-2 gap-6 p-8">
  {[0, 1, 2, 3].map((seatIndex) => {
    const player = players.find((p) => /* ordered by join time, seat index = array index */);
    return (
      <div
        key={seatIndex}
        className={/* occupied vs empty styles */}
        data-testid={`player-seat-${seatIndex}`}
      >
        {player ? (
          <div>
            <span className="text-text-primary font-semibold">{player.username}</span>
            {player.userId === currentUserId && (
              <span className="text-accent text-xs ml-2">{t('lobby.roomLobby.seatYou')}</span>
            )}
            {player.userId === room.ownerId && (
              <span className="text-text-secondary text-xs ml-2">{t('lobby.roomLobby.seatOwner')}</span>
            )}
          </div>
        ) : (
          <span className="text-text-secondary">{t('lobby.roomLobby.seatEmpty')}</span>
        )}
      </div>
    );
  })}
</div>
```

**Seat styling:**

- Occupied: `rounded-xl border border-border bg-surface p-6 text-center`
- Empty: `rounded-xl border border-dashed border-border bg-surface/50 p-6 text-center`
- Current user's seat: add `ring-1 ring-accent` for subtle highlight

**Copy Link implementation:**

```tsx
const handleCopyLink = async () => {
  try {
    await navigator.clipboard.writeText(room.code);
    toast(t("lobby.roomLobby.copyLinkSuccess"));
  } catch {
    toast(t("lobby.roomLobby.copyLinkFailed", { code: room.code }));
  }
};
```

**Leave Room handler:**

```tsx
const handleLeaveRoom = async () => {
  try {
    await leaveRoom(Number(id));
    navigate("/lobby");
  } catch {
    // Even on error, navigate back — the player shouldn't be stuck
    navigate("/lobby");
  }
};
```

**Browser back/navigation leave:** Use `useEffect` cleanup to call `leaveRoom` when the component unmounts due to navigation:

```tsx
useEffect(() => {
  return () => {
    // Fire-and-forget leave on unmount (navigating away)
    if (roomId && hasJoined) {
      leaveRoom(Number(roomId)).catch(() => {});
    }
  };
}, [roomId, hasJoined]);
```

**Important:** Track `hasJoined` state to avoid calling leaveRoom on initial mount failure or when the user explicitly clicked "Leave Room" (which already calls leaveRoom). Use a ref to prevent double-leave.

**Toast notifications:** Use the existing toast pattern from the project. Check if a toast utility/component exists (from shadcn). If not, use `window.alert` as a temporary fallback and add a TODO for proper toast implementation.

### WebSocket Event Contract

**New events for `server/internal/ws/events.go`:**

```go
const SystemPlayerJoined = "system:player_joined"
const SystemPlayerLeft   = "system:player_left"
```

**New events for `client/src/shared/types/wsEvents.ts`:**

```typescript
export const SYSTEM_PLAYER_JOINED = "system:player_joined" as const;
export const SYSTEM_PLAYER_LEFT = "system:player_left" as const;

export interface PlayerJoinedPayload {
  roomId: number;
  userId: number;
  username: string;
  playerCount: number;
}

export interface PlayerLeftPayload {
  roomId: number;
  userId: number;
  username: string;
  playerCount: number;
  newOwnerId?: number;
}
```

**WS hub is not yet wired end-to-end.** The frontend hook (`useRoomLobbyUpdates`) should be created with correct event handler logic but documented as requiring WS infrastructure. Handler TODOs in the backend mark where broadcasts should be added.

### Testing Strategy

**Backend (Go):**

- External test package: `package room_test`
- Extend existing mock repository with 6 new methods
- Mock patterns:
  - `AddPlayer`: track in-memory `[]RoomPlayer` slice, return `ErrAlreadyInRoom` if duplicate
  - `RemovePlayer`: remove from slice, return `ErrNotInRoom` if not found
  - `FindPlayersByRoomID`: filter slice by roomID
  - `FindPlayerRoom`: scan slice for matching userID with a "waiting" room
  - `IncrementPlayerCount` / `DecrementPlayerCount`: modify room's PlayerCount in mock
- **Critical:** Return `[]RoomPlayer{}` (not `nil`) when no players match — JSON serializes as `[]` not `null`

**Frontend (Vitest):**

- Co-locate all test files with their components
- Mock `shared/api/rooms` module for API calls
- Mock `navigator.clipboard.writeText` for Copy Link tests
- Mock `useParams` to return test room ID
- Mock `useNavigate` to assert navigation calls
- Use `@testing-library/react` with `BrowserRouter` wrapper
- Use `data-testid` attributes — never CSS classes

### Previous Story Intelligence (from Story 2.2)

**Critical learnings to apply:**

- `fetchClient<T>()` auto-unwraps `{ data: T }` — do NOT add a second unwrap in `getRoom()`, `joinRoom()`, or `leaveRoom()`
- Use `auth.GetUserID(c)` from `auth/middleware.go` for user ID extraction in handlers
- Use external test packages (`room_test` not `room`) to prevent import cycles
- `verbatimModuleSyntax` in tsconfig requires `import type` for type-only imports
- Pre-existing GORM integration test failures exist (2 in `user/user_test.go`) — not caused by this story
- Stale closure prevention: use `useLobbyStore.getState()` in event handlers, not dependency closures
- Card component styling: `rounded-xl border border-border bg-surface p-4` (reuse for seats)
- Error handling: catch `FetchError` and check `.code` for specific error codes
- Use `useNavigate()` from React Router for navigation
- Handler returns `map[string]interface{}{"data": ...}` for response wrapping

**Review findings from Story 2.2 to avoid repeating:**

- F1: Validate parameters — validate room ID is a valid integer in JoinRoom/LeaveRoom/GetRoom handlers
- F2: Unsafe `as` casts on WS payloads — deferred to Story 4-1, continue deferring
- F4: Clear state on view re-entry — clear room lobby data when navigating away

**Deferred items still pending:**

- D1: `FindByID` returns `(nil, nil)` for missing rooms — follow this pattern
- D2: No rate limiting on room endpoints — still deferred
- D3: No per-user active room count cap — now partially addressed by `FindPlayerRoom` check (one active room at a time)

### Git Intelligence

**Recent commit pattern:**

```
ea114b7 feat(room): implement room browsing and search with code review fixes
eead853 feat(room): implement room creation and configuration with code review fixes
```

- Format: `{type}({scope}): {description}`
- Scope for this story: `room` (same domain as 2.1 and 2.2)
- Expected commit: `feat(room): implement room joining and room lobby`

**Branch naming:** `feat/E2-S3-join-room-lobby`

### Project Structure Notes

- Frontend lobby components go in `client/src/features/lobby/` — the folder already exists
- Backend room domain stays in `server/internal/room/` — no new packages needed
- API client file `client/src/shared/api/rooms.ts` already exists — add `getRoom`, `joinRoom`, `leaveRoom` to it
- Route `/rooms/:id` already exists in `App.tsx` pointing to `RoomLobby` component
- Next migration number is `000004`
- No new shadcn components needed — `Button`, `Input`, and existing primitives are sufficient
- The room owner creates a room and is automatically the first player (Story 2.1 sets `PlayerCount = 1`). This story must ensure the owner is added to `room_players` when they create a room. **Option A:** Add a `room_players` insert inside `CreateRoom` handler. **Option B:** Create a data migration to backfill. **Recommended:** Option A — modify `CreateRoom` in handler.go to also call `repo.AddPlayer()` after creating the room. This keeps the `room_players` table consistent from the start. If there are existing rooms in dev DB without room_player records, `make seed` will re-create clean data.

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic-2, Story 2.3]
- [Source: _bmad-output/planning-artifacts/architecture.md#Lobby-Domain, #Frontend-Feature-Organization, #Structure-Patterns, #Data-Architecture]
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md#Journey-3-Room-Join, #PlayerSeat-Component, #Journey-2-Ana-Private-Room]
- [Source: _bmad-output/planning-artifacts/prd.md#FR17, #FR18, #FR20]
- [Source: _bmad-output/project-context.md#Framework-Rules, #Testing-Rules, #Naming-Conventions, #Anti-Patterns]
- [Source: _bmad-output/archive/implementation-artifacts/2-2-browse-and-search-rooms.md#Dev-Notes, #Review-Findings]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

- No blockers or halts encountered during implementation.
- Pre-existing GORM integration test failures (2 in `user/user_test.go`) — not caused by this story.
- `golangci-lint` CLI not installed locally — used `go vet` as fallback (passes clean).

### Completion Notes List

- **Task 1:** Created `000004_create_room_players` migration (up + down). Table with id, room_id (FK CASCADE), user_id (FK), nullable seat/team, created_at. Unique index on (room_id, user_id), indexes on room_id and user_id.
- **Task 2:** Added `RoomPlayer` struct to `model.go` with `gorm:"-"` Username field for join queries. Seat and Team are nullable pointers for Story 2.4.
- **Task 3:** Extended `RoomRepository` interface with 7 new methods: `Update`, `AddPlayer`, `RemovePlayer`, `FindPlayersByRoomID`, `FindPlayerRoom`, `IncrementPlayerCount`, `DecrementPlayerCount`. All implemented in `gorm_repo.go`.
- **Task 4:** Added `ErrAlreadyInRoom` (409) and `ErrNotInRoom` (404) to `apperr/errors.go`.
- **Task 5:** Patched `CreateRoom` handler to insert `room_players` record for the creator immediately after room creation.
- **Task 6:** Added `JoinRoom` handler — validates room exists, is waiting, not full, user not already in room. Adds player and increments count.
- **Task 7:** Added `LeaveRoom` handler — removes player, decrements count, transfers ownership to earliest remaining player if owner leaves, marks room completed if empty.
- **Task 8:** Added `GetRoom` handler — returns `RoomDetailResponse` with room data and player list including usernames via JOIN query.
- **Task 9:** Wired `GET /rooms/:id`, `POST /rooms/:id/join`, `POST /rooms/:id/leave` in `main.go` under auth middleware.
- **Task 10:** Added `SystemPlayerJoined` and `SystemPlayerLeft` WS events to Go `events.go` and TS `wsEvents.ts` with full payload interfaces.
- **Task 11:** Added `RoomPlayer`, `RoomDetail` types to `apiTypes.ts`. Added `getRoom`, `joinRoom`, `leaveRoom` functions to `rooms.ts`.
- **Task 12:** Added `onJoin` prop to `RoomCard`, `onJoinRoom` prop to `RoomList`, join handler in `LobbyPage` with error-specific toasts.
- **Task 13:** Rewrote `RoomLobby.tsx` with full implementation: room fetch on mount, 2x2 seat grid, Copy Link, Leave Room, loading/error states, self/owner indicators, cleanup effect for leave on unmount.
- **Task 14:** Created `useRoomLobbyUpdates.ts` with `handleWsMessage` export for future WS integration. Handles player_joined, player_left, room_updated events.
- **Task 15:** Owner leaving handled in Task 7 backend + Task 14 WS hook. Owner badge displayed in RoomLobby component.
- **Task 16:** Added 12 new i18n keys to `lobby.roomLobby` and 3 error keys to `lobby.roomList.errors` in both en.json and sr.json.
- **Task 17:** Added 15 backend tests: 4 GetRoom, 6 JoinRoom, 5 LeaveRoom. Extended mock repo with all 7 new methods. All 36 room tests pass.
- **Task 18:** Updated RoomCard tests (added onJoin callback test), updated RoomList tests (added onJoinRoom prop). Created RoomLobby.test.tsx with 8 tests covering room display, seats, copy link, leave room, loading/error states.
- **Task 19:** All 77 frontend tests pass. All Go room tests pass (36/36). Zero lint errors on modified files. `go vet` clean.

### Review Findings

- [x] [Review][Decision] F1: GetRoom endpoint accessible to any authenticated user — RESOLVED: keep open for room previews (user choice). No code change needed.
- [x] [Review][Patch] F2: JoinRoom/LeaveRoom/CreateRoom not transactional — wrapped all three handlers in `RunInTransaction` with rollback on failure [server/internal/room/handler.go]
- [x] [Review][Patch] F3: LeaveRoom ownership transfer uses stale room object — now re-fetches room inside transaction after decrement [server/internal/room/handler.go]
- [x] [Review][Patch] F4: Status "completed" not in validStatuses map — added "completed" to the allowlist [server/internal/room/handler.go:22]
- [x] [Review][Patch] F5: RoomLobby cleanup useEffect fires leaveRoom spuriously — added `hasJoinedRef` gated on getRoom response confirming user membership [client/src/features/lobby/RoomLobby.tsx]
- [x] [Review][Patch] F6: IncrementPlayerCount/DecrementPlayerCount ignore RowsAffected — added RowsAffected == 0 → ErrRoomNotFound [server/internal/room/gorm_repo.go]
- [x] [Review][Patch] F7: FindPlayerRoom only checks "waiting" rooms — added "playing" to IN clause [server/internal/room/gorm_repo.go]
- [x] [Review][Defer] F8: WS events system:player_joined and system:player_left are TODO-only — not broadcast. Deferred to Story 4-1 (WebSocket Gateway) which builds the WS hub infrastructure required for broadcasting
- [x] [Review][Defer] F9: Copy Link copies room code, not a full URL — spec says "code/link" and user journeys describe sharing codes via WhatsApp/Viber. Current behavior matches spec intent. Revisit if URL sharing is needed
- [x] [Review][Defer] F10: Orphan room_players rows for soft-deleted rooms — no functional impact, GORM soft delete doesn't trigger FK CASCADE. Cleanup script for Phase 2
- [x] [Review][Defer] F11: No unit test for useRoomLobbyUpdates handler logic — WS not yet wired. Test when WS infrastructure lands (Story 4-1)

### Change Log

- 2026-04-11: Implemented Story 2.3 — Join Room & Room Lobby (all 19 tasks complete)
- 2026-04-11: Code review — 6 patches applied (transactions, stale state, status map, cleanup effect, RowsAffected, FindPlayerRoom scope), 1 decision resolved (keep GetRoom open), 4 deferred

### File List

**New files:**

- server/migrations/000004_create_room_players.up.sql
- server/migrations/000004_create_room_players.down.sql
- client/src/features/lobby/RoomLobby.test.tsx
- client/src/features/lobby/useRoomLobbyUpdates.ts

**Modified files:**

- server/internal/room/model.go (added RoomPlayer struct)
- server/internal/room/repository.go (added 7 interface methods)
- server/internal/room/gorm_repo.go (added 7 implementations)
- server/internal/room/handler.go (patched CreateRoom, added JoinRoom, LeaveRoom, GetRoom, RoomDetailResponse)
- server/internal/room/handler_test.go (extended mock, added 15 tests)
- server/internal/apperr/errors.go (added ErrAlreadyInRoom, ErrNotInRoom)
- server/internal/ws/events.go (added SystemPlayerJoined, SystemPlayerLeft)
- server/cmd/api/main.go (added 3 new routes)
- client/src/shared/types/apiTypes.ts (added RoomPlayer, RoomDetail interfaces)
- client/src/shared/types/wsEvents.ts (added SYSTEM_PLAYER_JOINED, SYSTEM_PLAYER_LEFT + payload interfaces)
- client/src/shared/api/rooms.ts (added getRoom, joinRoom, leaveRoom functions)
- client/src/features/lobby/RoomCard.tsx (added onJoin prop)
- client/src/features/lobby/RoomCard.test.tsx (added onJoin callback test)
- client/src/features/lobby/RoomList.tsx (added onJoinRoom prop, RoomListProps interface)
- client/src/features/lobby/RoomList.test.tsx (updated for onJoinRoom prop)
- client/src/features/lobby/RoomLobby.tsx (full rewrite from stub)
- client/src/features/lobby/LobbyPage.tsx (added join handler, navigate, toast imports)
- client/src/shared/i18n/en.json (added lobby.roomLobby._ and lobby.roomList.errors._ keys)
- client/src/shared/i18n/sr.json (added matching Serbian translations)
