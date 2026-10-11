# Story 2.1: Create Room & Room Configuration

Status: done

## Story

As a player,
I want to create a game room and configure its settings,
So that I can set up a Belot game exactly how my group wants to play.

## Acceptance Criteria

1. **Room Creation Modal** — Clicking "Create Room" in the lobby opens a modal with 4 configuration controls:
   - Room Name (text input, required)
   - Variant (dropdown: "Bitola" only for Phase 1)
   - Match Mode (dropdown: "1001" only for Phase 1)
   - Timer Style (dropdown: "Per-move" with duration selector, or "Relaxed")
   - Defaults pre-filled: Bitola / 1001 / Relaxed

2. **Create Room API** — `POST /api/v1/rooms` creates a room record with the authenticated player as owner, returns the created room wrapped in `{ "data": {...} }`, and responds with HTTP 201.

3. **Database Schema** — The `rooms` table contains: `id`, `name` (unique among active rooms), `code` (unique, auto-generated 6-char uppercase), `owner_id` (FK to `users`), `variant`, `match_mode`, `timer_style`, `timer_duration_seconds` (nullable), `status` (waiting/in_progress/completed), `created_at`, `updated_at`, `deleted_at` (GORM soft delete).

4. **Post-Creation Redirect** — After successful creation, the player is redirected to the room lobby view at `/rooms/:id`.

5. **WebSocket Broadcast** — On room creation, a `system:room_created` event is broadcast to all connected lobby clients so the new room appears in the browse list without page refresh.

6. **Form Validation** — Create button remains disabled until a non-empty room name is entered. Modal closes via backdrop click or Cancel button without creating a room.

7. **Error Handling** — Server validates: empty name returns 400 `ROOM_NAME_REQUIRED`, duplicate active room name returns 409 `ROOM_NAME_TAKEN`. Errors display as inline messages below the room name input.

## Tasks / Subtasks

- [x] **Task 1: Database migration for rooms table** (AC: #3)
  - [x] Create `server/migrations/000003_create_rooms.up.sql` with rooms table schema
  - [x] Create `server/migrations/000003_create_rooms.down.sql` with `DROP TABLE rooms`
  - [x] Run `make migrate` to apply

- [x] **Task 2: Backend room domain package** (AC: #2, #3, #7)
  - [x] Create `server/internal/room/model.go` — Room GORM struct
  - [x] Create `server/internal/room/repository.go` — RoomRepository interface
  - [x] Create `server/internal/room/gorm_repo.go` — GORM implementation
  - [x] Create `server/internal/room/handler.go` — CreateRoom handler
  - [x] Add room errors to `server/internal/apperr/errors.go`
  - [x] Wire routes in `server/cmd/api/main.go`

- [x] **Task 3: Room code generation** (AC: #3)
  - [x] Implement 6-character uppercase alphanumeric code generator in room package
  - [x] Ensure uniqueness via retry loop on DB unique constraint violation

- [x] **Task 4: WebSocket room event** (AC: #5)
  - [x] Add `system:room_created` event type to `server/internal/ws/events.go`
  - [x] Add matching type to `client/src/shared/types/wsEvents.ts`
  - [x] Broadcast from handler after successful room creation (if WS hub is wired; otherwise stub the broadcast call and document it for Story 2.2)

- [x] **Task 5: Frontend API client** (AC: #2)
  - [x] Create `client/src/shared/api/rooms.ts` with `createRoom()` function using `fetchClient`

- [x] **Task 6: Install shadcn Select component** (AC: #1)
  - [x] Run `npx shadcn@latest add select` — needed for Variant, Match Mode, and Timer Style dropdowns

- [x] **Task 7: Lobby page update with Create Room card** (AC: #1)
  - [x] Update `client/src/features/lobby/LobbyPage.tsx` to show play option cards (Quick Play, Browse Rooms, Create Room) per Direction 5 layout
  - [x] Quick Play card gets `accent-glow` background; Browse Rooms and Create Room are ghost-style cards
  - [x] Create Room card opens the CreateRoomModal

- [x] **Task 8: CreateRoomModal component** (AC: #1, #6, #7)
  - [x] Create `client/src/features/lobby/CreateRoomModal.tsx` using shadcn Dialog
  - [x] 4 form controls: Room Name (Input), Variant (Select, Bitola only), Match Mode (Select, 1001 only), Timer Style (Select with conditional duration input)
  - [x] Pre-fill defaults: Bitola / 1001 / Relaxed
  - [x] Disable Create button when name is empty
  - [x] Show inline error below name input on API error
  - [x] On success: close modal and navigate to `/rooms/:id`

- [x] **Task 9: Room lobby placeholder page** (AC: #4)
  - [x] Create `client/src/features/lobby/RoomLobby.tsx` as placeholder (details in Story 2.3)
  - [x] Add route `/rooms/:id` to `App.tsx` (protected, inside AppLayout)

- [x] **Task 10: Lobby store expansion** (AC: #1, #5)
  - [x] Expand `client/src/shared/stores/lobbyStore.ts` with `rooms: Room[]`, `addRoom()`, `setRooms()` actions
  - [x] Add Room type to `client/src/shared/types/apiTypes.ts`

- [x] **Task 11: i18n translations** (AC: #1, #7)
  - [x] Add room creation strings to `client/src/shared/i18n/en.json`
  - [x] Add room creation strings to `client/src/shared/i18n/sr.json`

- [x] **Task 12: Backend tests** (AC: #2, #3, #7)
  - [x] Create `server/internal/room/handler_test.go` — test valid creation, missing name, duplicate name, owner assignment, room code generation, default values
  - [x] Use external test package (`room_test`) to avoid import cycles
  - [x] Use mock repository (in-memory) following the pattern from `auth/handler_test.go`

- [x] **Task 13: Frontend tests** (AC: #1, #6, #7, #8)
  - [x] Create `client/src/features/lobby/CreateRoomModal.test.tsx`
  - [x] Test: renders with defaults, create button disabled when name empty, submit calls API, error display, cancel closes modal
  - [x] Create `client/src/features/lobby/LobbyPage.test.tsx` (or update existing)
  - [x] Test: renders play option cards, Create Room card opens modal

- [x] **Task 14: Lint and test** (AC: all)
  - [x] Run `make lint` — pass with zero errors (new files clean; pre-existing import-sort issues in other files)
  - [x] Run `make test` — all existing + new tests pass (51 frontend, 10 room backend; 2 pre-existing user integration failures unrelated)

## Dev Notes

### Architecture Compliance

**Backend domain package shape** — follow exactly:

```
server/internal/room/
├── model.go          # Room GORM struct
├── repository.go     # RoomRepository interface
├── gorm_repo.go      # GORM implementation
├── handler.go        # Echo HTTP handlers
└── handler_test.go   # Tests (external package: room_test)
```

**Important:** The architecture document references `internal/lobby/` for this domain. A `lobby/` directory already exists but is currently empty/minimal. Use `internal/room/` as the domain package name for this story since it aligns with the resource name (`/api/v1/rooms`). The lobby-level orchestration (browsing, matchmaking) can use `lobby/` in later stories (2.2, 2.5). This avoids a monolithic package and follows the established 1:1 mapping between API resources and backend packages (like `auth/` → `/api/v1/auth/`, `user/` → `/api/v1/users/`).

### Room GORM Model

```go
// server/internal/room/model.go
package room

import (
    "time"
    "gorm.io/gorm"
)

type Room struct {
    ID                   uint           `gorm:"primaryKey" json:"id"`
    Name                 string         `gorm:"size:100;not null" json:"name"`
    Code                 string         `gorm:"size:6;uniqueIndex;not null" json:"code"`
    OwnerID              uint           `gorm:"not null;index" json:"ownerId"`
    Variant              string         `gorm:"size:20;not null;default:bitola" json:"variant"`
    MatchMode            string         `gorm:"size:10;not null;default:1001" json:"matchMode"`
    TimerStyle           string         `gorm:"size:20;not null;default:relaxed" json:"timerStyle"`
    TimerDurationSeconds *int           `json:"timerDurationSeconds"`
    Status               string         `gorm:"size:20;not null;default:waiting;index" json:"status"`
    PlayerCount          int            `gorm:"not null;default:1" json:"playerCount"`
    CreatedAt            time.Time      `json:"createdAt"`
    UpdatedAt            time.Time      `json:"updatedAt"`
    DeletedAt            gorm.DeletedAt `gorm:"index" json:"-"`
}
```

**Key conventions:**

- All JSON tags use `camelCase` (matches project standard)
- `TimerDurationSeconds` is `*int` (pointer for nullable — only set when `timerStyle` is `per-move`)
- `DeletedAt` uses GORM soft delete, excluded from JSON with `json:"-"`
- `OwnerID` has an index for lookups; `Status` indexed for room browsing
- `Code` has a unique index for room code lookups
- `PlayerCount` starts at 1 (the creator)

### Database Migration

```sql
-- server/migrations/000003_create_rooms.up.sql
CREATE TABLE rooms (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    code VARCHAR(6) NOT NULL,
    owner_id INTEGER NOT NULL REFERENCES users(id),
    variant VARCHAR(20) NOT NULL DEFAULT 'bitola',
    match_mode VARCHAR(10) NOT NULL DEFAULT '1001',
    timer_style VARCHAR(20) NOT NULL DEFAULT 'relaxed',
    timer_duration_seconds INTEGER,
    status VARCHAR(20) NOT NULL DEFAULT 'waiting',
    player_count INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_rooms_code ON rooms(code);
CREATE INDEX idx_rooms_owner_id ON rooms(owner_id);
CREATE INDEX idx_rooms_status ON rooms(status) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX idx_rooms_name_active ON rooms(name) WHERE deleted_at IS NULL AND status != 'completed';
```

**Notes:**

- The unique constraint on `name` is partial — only among active (non-deleted, non-completed) rooms. Players can reuse names after rooms close.
- `deleted_at IS NULL` partial index on status speeds up the most common query (browsing active rooms).
- Migration numbering: `000003` — after existing `000001_init` and `000002_create_users`.

```sql
-- server/migrations/000003_create_rooms.down.sql
DROP TABLE IF EXISTS rooms;
```

### Room Code Generation

Generate a 6-character uppercase alphanumeric code (A-Z, 0-9, excluding ambiguous chars I/O/0/1):

```go
const codeChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
const codeLength = 6
```

Use `crypto/rand` for generation. On unique constraint violation (extremely rare), retry up to 3 times.

### API Endpoint

```
POST /api/v1/rooms
Authorization: Bearer {accessToken}
Content-Type: application/json

Request:
{
  "name": "Zagreb Ekipa",
  "variant": "bitola",
  "matchMode": "1001",
  "timerStyle": "relaxed",
  "timerDurationSeconds": null
}

Response (201):
{
  "data": {
    "id": 1,
    "name": "Zagreb Ekipa",
    "code": "X7KM3P",
    "ownerId": 5,
    "variant": "bitola",
    "matchMode": "1001",
    "timerStyle": "relaxed",
    "timerDurationSeconds": null,
    "status": "waiting",
    "playerCount": 1,
    "createdAt": "2026-04-11T14:30:00Z",
    "updatedAt": "2026-04-11T14:30:00Z"
  }
}
```

**Error responses:**

- 400 `ROOM_NAME_REQUIRED` — empty or whitespace-only name
- 409 `ROOM_NAME_TAKEN` — active room with this name already exists
- 401 — missing/invalid auth token (handled by auth middleware)

### Error Definitions

Add to `server/internal/apperr/errors.go`:

```go
// Room errors
var ErrRoomNameRequired = NewAppError("ROOM_NAME_REQUIRED", "room name is required", http.StatusBadRequest)
var ErrRoomNameTaken    = NewAppError("ROOM_NAME_TAKEN", "a room with this name already exists", http.StatusConflict)
var ErrRoomNotFound     = NewAppError("ROOM_NOT_FOUND", "room not found", http.StatusNotFound)
var ErrRoomFull         = NewAppError("ROOM_FULL", "room is full", http.StatusConflict)
var ErrNotRoomOwner     = NewAppError("NOT_ROOM_OWNER", "only the room owner can perform this action", http.StatusForbidden)
```

Define all room errors now even though some are used in later stories (2.3, 2.4). This prevents future import cycles and keeps the error catalogue consistent.

### Handler Pattern

Follow the exact pattern from `server/internal/auth/handler.go` and `server/internal/user/handler.go`:

```go
type RoomHandler struct {
    repo RoomRepository
}

func NewRoomHandler(repo RoomRepository) *RoomHandler {
    return &RoomHandler{repo: repo}
}

func (h *RoomHandler) CreateRoom(c echo.Context) error {
    // 1. Extract userID via auth helper (safe to import auth from room — no reverse dep)
    userID, err := auth.GetUserID(c)
    if err != nil {
        return apperr.ErrUnauthorized
    }

    // 2. Bind and validate request
    var req CreateRoomRequest
    if err := c.Bind(&req); err != nil {
        return apperr.ErrBadRequest
    }

    // 3. Validate name
    name := strings.TrimSpace(req.Name)
    if name == "" {
        return apperr.ErrRoomNameRequired
    }

    // 4. Build Room model with defaults
    // 5. Generate room code
    // 6. Call repo.Create()
    // 7. Handle duplicate name error → ErrRoomNameTaken (see gorm_repo.go pattern below)
    // 8. Return 201 with wrapped response
    return c.JSON(http.StatusCreated, map[string]interface{}{"data": room})
}
```

**Duplicate name detection in `gorm_repo.go`** — detect PostgreSQL unique constraint violation and translate to domain error:

```go
import "github.com/jackc/pgx/v5/pgconn"

func (r *GormRepository) Create(room *Room) error {
    result := r.db.Create(room)
    if result.Error != nil {
        var pgErr *pgconn.PgError
        if errors.As(result.Error, &pgErr) && pgErr.Code == "23505" {
            if strings.Contains(pgErr.ConstraintName, "idx_rooms_name_active") {
                return apperr.ErrRoomNameTaken
            }
        }
        return fmt.Errorf("creating room: %w", result.Error)
    }
    return nil
}
```

**User ID extraction:** Use `auth.GetUserID(c)` from `server/internal/auth/middleware.go` (line 52). Importing `auth` from `room` is safe — there is no reverse dependency. The `user/handler.go` has a private duplicate (`getUserID`) as a legacy workaround, but the room handler should use the canonical public helper.

### Route Wiring

In `server/cmd/api/main.go`, add to the protected API group:

```go
// Room routes
roomRepo := room.NewGormRepository(db)
roomHandler := room.NewRoomHandler(roomRepo)
api.POST("/rooms", roomHandler.CreateRoom)
```

The `api` group already has auth middleware applied.

### WebSocket Event Contract

**Both files must be updated in the same commit.**

Add to `server/internal/ws/events.go`:

```go
// Room events
const SystemRoomCreated = "system:room_created"
```

Add to `client/src/shared/types/wsEvents.ts`:

```typescript
// Room events
export const SYSTEM_ROOM_CREATED = "system:room_created" as const;

export interface RoomCreatedPayload {
  id: number;
  name: string;
  code: string;
  ownerId: number;
  variant: string;
  matchMode: string;
  timerStyle: string;
  timerDurationSeconds: number | null;
  status: string;
  playerCount: number;
}
```

**Note on broadcast implementation:** The WS hub (`internal/ws/hub.go`) may not be fully wired yet. If a hub broadcast function is not available, create a stub/no-op call in the handler with a `// TODO: broadcast via WS hub when available (Story 2.2)` comment. Do not block the room creation flow on WS infrastructure. The room MUST be created via HTTP regardless of WS availability.

### Frontend Implementation

#### Room Type Definition

Add to `client/src/shared/types/apiTypes.ts`:

```typescript
export interface Room {
  id: number;
  name: string;
  code: string;
  ownerId: number;
  variant: string;
  matchMode: string;
  timerStyle: string;
  timerDurationSeconds: number | null;
  status: string;
  playerCount: number;
  createdAt: string;
  updatedAt: string;
}

export interface CreateRoomRequest {
  name: string;
  variant: string;
  matchMode: string;
  timerStyle: string;
  timerDurationSeconds: number | null;
}
```

#### API Client

Create `client/src/shared/api/rooms.ts`:

```typescript
import { fetchClient } from "@/shared/api/fetchClient";
import { Room, CreateRoomRequest } from "@/shared/types/apiTypes";

export function createRoom(req: CreateRoomRequest): Promise<Room> {
  return fetchClient<Room>("/rooms", {
    method: "POST",
    body: JSON.stringify(req),
  });
}
```

Follow the pattern from `profile.ts` — use `fetchClient` which handles auth headers and 401 refresh cycle. Use `@/` import alias (project standard — see `profile.ts` line 1).

**Important:** `fetchClient` automatically unwraps the `{ "data": T }` response envelope. `createRoom()` receives a `Room` directly, not `{ data: Room }`. Do not add a second unwrap layer.

#### Lobby Store Expansion

Expand `client/src/shared/stores/lobbyStore.ts`. The existing store has `isLoading: boolean` and `setLoading()` — preserve these and add room list management:

```typescript
import { create } from "zustand";
import { Room } from "@/shared/types/apiTypes";

interface LobbyState {
  // Existing state (do not remove)
  isLoading: boolean;
  setLoading: (loading: boolean) => void;

  // New room state
  rooms: Room[];
  setRooms: (rooms: Room[]) => void;
  addRoom: (room: Room) => void;
  removeRoom: (roomId: number) => void;
  updateRoom: (room: Room) => void;
}
```

Use immutable updates — replace the `rooms` array, never mutate it directly.

#### CreateRoomModal Component

Use the existing `Dialog` component (already installed at `shared/components/ui/dialog.tsx`). This is a **Base UI** (`@base-ui/react/dialog`) component, not Radix. The API is similar but note:

- `Dialog` accepts `open` and `onOpenChange` props (passed through to `DialogPrimitive.Root`)
- `DialogContent` accepts an optional `showCloseButton` prop (defaults to `true`)
- Backdrop click closes the dialog automatically (Base UI handles this — no manual wiring needed)
- `DialogContent` default max-width is `sm:max-w-sm` — override with className for wider modals

Install shadcn `Select` component: `npx shadcn@latest add select`

**Component structure:**

```
<Dialog open={showCreateModal} onOpenChange={setShowCreateModal}>
  <DialogContent className="sm:max-w-md">
    <DialogHeader>
      <DialogTitle>        {t('lobby.createRoomModal.title')}
    </DialogHeader>
    <form>
      <Input>              Room Name (required)
      <Select>             Variant (Bitola only — disabled/single option)
      <Select>             Match Mode (1001 only — disabled/single option)
      <Select>             Timer Style (Relaxed / Per-move)
      {timerStyle === 'per-move' && <Input type="number"> Timer Duration (seconds)}
      <error message>      Inline below name input
    </form>
    <DialogFooter>
      <Button variant="ghost">  Cancel
      <Button disabled={!name}>  Create Room
    </DialogFooter>
  </DialogContent>
</Dialog>
```

**Phase 1 simplification:** Variant and Match Mode selects only have one option each (Bitola, 1001). Render them as read-only or disabled selects to show future extensibility without confusing users.

**Timer duration:** When "Per-move" is selected, show a number input defaulting to 30 seconds. Valid range: 10-120 seconds. When "Relaxed" is selected, hide duration and send `null`.

#### LobbyPage Update

Update `client/src/features/lobby/LobbyPage.tsx` to render Direction 5 layout:

```
<div className="max-w-5xl mx-auto px-8 py-8">
  <div className="grid grid-cols-[280px_1fr] gap-6">
    {/* Left: Play Options */}
    <div className="flex flex-col gap-4">
      <PlayOptionCard                     Quick Play (accent-glow bg)
        title={t('lobby.quickPlay')}
        description={t('lobby.quickPlayDesc')}
        className="bg-accent-glow"
        onClick={...}                     (placeholder for Story 2.5)
      />
      <PlayOptionCard                     Browse Rooms (ghost style)
        title={t('lobby.browseRooms')}
        onClick={...}                     (placeholder for Story 2.2)
      />
      <PlayOptionCard                     Create Room (ghost style)
        title={t('lobby.createRoom')}
        onClick={() => setShowCreateModal(true)}
      />
    </div>
    {/* Right: Leaderboard placeholder */}
    <div className="bg-surface rounded-lg border border-border p-6">
      <p className="text-text-secondary">Leaderboard coming soon</p>
    </div>
  </div>
  <CreateRoomModal
    open={showCreateModal}
    onOpenChange={setShowCreateModal}
  />
</div>
```

#### RoomLobby Placeholder

Create `client/src/features/lobby/RoomLobby.tsx` — simple placeholder page showing room name, code, and "Waiting for players..." message. Full implementation in Story 2.3.

Add route to `App.tsx` — nest inside the existing `<Route element={<ProtectedRoute />}>` → `<Route element={<AppLayout />}>` group (do NOT wrap inline — follow the existing nesting pattern):

```tsx
{
  /* Inside the existing <Route element={<AppLayout />}> group, after /rules */
}
<Route path="/rooms/:id" element={<RoomLobby />} />;
```

#### Navigation After Creation

On successful `createRoom()` API call, use `useNavigate()` from React Router to navigate to `/rooms/${room.id}`.

### i18n Keys

**English (`en.json`):**

```json
{
  "lobby": {
    "title": "Lobby",
    "quickPlay": "Quick Play",
    "quickPlayDesc": "Jump into a game instantly",
    "browseRooms": "Browse Rooms",
    "browseRoomsDesc": "Find an open game to join",
    "createRoom": "Create Room",
    "createRoomDesc": "Set up a game your way",
    "leaderboardPlaceholder": "Leaderboard coming soon",
    "createRoomModal": {
      "title": "Create a Game Room",
      "roomName": "Room Name",
      "roomNamePlaceholder": "Enter room name...",
      "variant": "Game Variant",
      "variantBitola": "Bitola",
      "matchMode": "Match Mode",
      "matchMode1001": "1001 points",
      "timerStyle": "Timer Style",
      "timerRelaxed": "Relaxed (no timer)",
      "timerPerMove": "Per-move timer",
      "timerDuration": "Timer Duration (seconds)",
      "create": "Create Room",
      "cancel": "Cancel",
      "errors": {
        "nameRequired": "Room name is required",
        "nameTaken": "A room with this name already exists"
      }
    }
  }
}
```

**Serbian (`sr.json`):**

```json
{
  "lobby": {
    "title": "Lobi",
    "quickPlay": "Brza igra",
    "quickPlayDesc": "Odmah uskoči u igru",
    "browseRooms": "Pregledaj sobe",
    "browseRoomsDesc": "Pronađi otvorenu igru",
    "createRoom": "Napravi sobu",
    "createRoomDesc": "Podesi igru po svom",
    "leaderboardPlaceholder": "Lista lidera uskoro",
    "createRoomModal": {
      "title": "Napravi sobu za igru",
      "roomName": "Ime sobe",
      "roomNamePlaceholder": "Unesi ime sobe...",
      "variant": "Varijanta igre",
      "variantBitola": "Bitola",
      "matchMode": "Rezim igre",
      "matchMode1001": "1001 poen",
      "timerStyle": "Stil tajmera",
      "timerRelaxed": "Opušteno (bez tajmera)",
      "timerPerMove": "Tajmer po potezu",
      "timerDuration": "Trajanje tajmera (sekunde)",
      "create": "Napravi sobu",
      "cancel": "Otkaži",
      "errors": {
        "nameRequired": "Ime sobe je obavezno",
        "nameTaken": "Soba sa ovim imenom već postoji"
      }
    }
  }
}
```

### Project Structure Notes

- **Backend `room/` package:** New package at `server/internal/room/`. Follows the established domain package shape from `auth/` and `user/`.
- **Frontend lobby feature:** Components go in `client/src/features/lobby/` which already exists with `LobbyPage.tsx`.
- **Shared types:** Room type added to existing `apiTypes.ts` — no new type file needed.
- **API client:** New file `rooms.ts` in `shared/api/` — follows 1:1 mapping convention (rooms.ts ↔ room package).
- **Store:** Expand existing `lobbyStore.ts` — do not create a separate room store.
- **shadcn Select:** Must be installed before use: `npx shadcn@latest add select`. Dialog is already available.

### Testing Strategy

**Backend (Go):**

- External test package: `package room_test`
- Mock repository with in-memory `[]Room` slice — same pattern as `auth/handler_test.go`
- Test via real HTTP requests using `httptest.NewRecorder()` + `echo.New()`
- Test cases:
  - Valid room creation with all fields → 201 + room in response
  - Valid creation with defaults (only name provided) → 201, variant=bitola, matchMode=1001, timerStyle=relaxed
  - Empty name → 400 `ROOM_NAME_REQUIRED`
  - Duplicate active room name → 409 `ROOM_NAME_TAKEN`
  - Room code is 6 chars, uppercase alphanumeric
  - Owner ID extracted from auth context
  - Timer duration set when timerStyle is per-move, null when relaxed

**Frontend (Vitest):**

- Co-located: `CreateRoomModal.test.tsx` next to `CreateRoomModal.tsx`
- Use `@testing-library/react` with `BrowserRouter` wrapper
- Use `data-testid` for element selection
- Mock `shared/api/rooms` module
- Test cases:
  - Renders modal with default values (Bitola, 1001, Relaxed)
  - Create button disabled when name input is empty
  - Create button enabled when name has text
  - Submitting form calls `createRoom()` with correct payload
  - API error `ROOM_NAME_TAKEN` shows inline error message
  - Cancel button closes modal
  - Timer duration input appears when "Per-move" selected, hidden when "Relaxed"

### Previous Story Intelligence

**From Story 1.4 (Profile & Navigation Shell):**

- `fetchClient<T>()` handles 401 refresh-retry and `{ data: T }` unwrapping automatically — use it for all authenticated API calls.
- Auth calls in `shared/api/auth.ts` use raw `fetch()` (pre-auth), but all other API calls must use `fetchClient`.
- User ID extraction: use `auth.GetUserID(c)` from `auth/middleware.go`. The `user/handler.go` has a private duplicate as a legacy workaround — do not replicate; use the canonical public helper.
- Use external test packages (`room_test` not `room`) to prevent import cycles. Note: `auth/handler_test.go` uses `package auth` (internal), not external — use the mock repository pattern from it, but declare your package as `room_test`.
- Mock repos return `(nil, nil)` for not-found cases, not `(nil, error)`.
- Stale closure prevention: use `useAuthStore.getState()` in event handlers, not dependency closures.
- Pre-existing GORM integration test failures exist — do not be alarmed by 2 failing integration tests unrelated to this story.

**From Git History:**

- Commit format: `feat(room): implement room creation and configuration`
- Scope matches domain: `(room)` for this story
- ~20-25 files per feature story is typical
- Co-Author tag required for Claude-authored work

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic-2, Story 2.1]
- [Source: _bmad-output/planning-artifacts/architecture.md#Lobby-Domain, #Database-Schemas, #WebSocket-Events]
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md#Direction-5, #Room-Creation-Journey, #Design-Tokens]
- [Source: _bmad-output/planning-artifacts/prd.md#FR16, #FR17]
- [Source: _bmad-output/project-context.md#Framework-Rules, #Testing-Rules, #Naming-Conventions]
- [Source: _bmad-output/archive/implementation-artifacts/1-4-basic-player-profile-and-navigation-shell.md#Dev-Notes]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

- Base UI Select `onValueChange` signature is `(value: string | null, eventDetails) => void` — required wrapping setter to handle null
- `verbatimModuleSyntax` enforced in tsconfig — all type-only imports require `import type` syntax
- Pre-existing App.test.tsx expected `"Lobby"` text which was removed in LobbyPage redesign — updated test to check for `data-testid` instead
- `golang-migrate` CLI not installed locally — applied migration directly via `docker compose exec postgres psql`
- 2 pre-existing GORM integration test failures in `user/user_test.go` (FindByEmail_NotFound, FindByUsername_NotFound) — unrelated to this story

### Completion Notes List

- Implemented full room creation flow: database migration, backend API, frontend modal with form validation
- Backend follows exact domain package shape: `model.go`, `repository.go`, `gorm_repo.go`, `handler.go`, `handler_test.go`
- Room code generation uses `crypto/rand` with retry loop for uniqueness
- WS broadcast stubbed with TODO comment (hub not yet wired — Story 2.2)
- LobbyPage redesigned with Direction 5 layout: Quick Play (accent-glow), Browse Rooms (ghost), Create Room (ghost) cards
- CreateRoomModal uses Dialog + Select components from shadcn/base-ui, with conditional timer duration input
- Variant and Match Mode selects are disabled (single option for Phase 1)
- All room errors pre-defined in `apperr/errors.go` for future stories
- 10 backend tests (external `room_test` package) covering all ACs
- 7 CreateRoomModal tests + 2 LobbyPage tests + updated App routing test
- i18n strings added for both English and Serbian
- All new/modified files pass ESLint with zero errors

### Review Findings

_Code review performed 2026-04-11 by Blind Hunter + Edge Case Hunter + Acceptance Auditor (3 parallel layers)._

**Patch (10 findings):**

- [x] [Review][Patch] **P1 (critical): Retry loop variable shadowing — returns 201 for unpersisted room** [handler.go:91-110] — FIXED: separate `createErr` variable, typed `ErrRoomCodeTaken` via `errors.Is()`.
- [x] [Review][Patch] **P2 (high): No server-side validation of variant/matchMode/timerStyle values** [handler.go:54-68] — FIXED: whitelist maps + `ErrInvalidVariant`, `ErrInvalidMatchMode`, `ErrInvalidTimerStyle` in apperr.
- [x] [Review][Patch] **P3 (high): No server-side room name length validation** [handler.go:49-52] — FIXED: explicit `len(name) > 100` check with `ErrRoomNameTooLong`.
- [x] [Review][Patch] **P4 (high): No server-side timerDurationSeconds validation** [handler.go:69-72] — FIXED: require duration when per-move, range 10-120, `ErrTimerDurationRequired` / `ErrTimerDurationOutOfRange`.
- [x] [Review][Patch] **P5 (medium): gorm_repo.go returns unwrapped errors** [gorm_repo.go:29] — FIXED: `return fmt.Errorf("creating room: %w", err)`.
- [x] [Review][Patch] **P6 (medium): Code collision should return typed error from repository** [gorm_repo.go:22-29] — FIXED: detect `idx_rooms_code` constraint, return `apperr.ErrRoomCodeTaken`.
- [x] [Review][Patch] **P7 (medium): RoomCreatedPayload missing createdAt/updatedAt** [wsEvents.ts:14-24] — FIXED: added `createdAt` and `updatedAt` fields.
- [x] [Review][Patch] **P8 (medium): handleOpenChange doesn't reset isSubmitting** [CreateRoomModal.tsx:81-89] — FIXED: added `setIsSubmitting(false)` in reset block.
- [x] [Review][Patch] **P9 (low): Hardcoded English strings bypass i18n** [CreateRoomModal.tsx:74, RoomLobby.tsx:13] — FIXED: added i18n keys `lobby.createRoomModal.errors.unexpected` and `lobby.roomLobby.waitingForPlayers` in both en.json and sr.json.
- [x] [Review][Patch] **P10 (low): Missing test — timer duration visibility toggle** [CreateRoomModal.test.tsx] — FIXED: added test case verifying input appears for per-move and is absent for relaxed.

**Deferred (3 findings):**

- [x] [Review][Defer] **D1: FindByID returns (nil, nil) for missing rooms** [gorm_repo.go:34-43] — deferred, not called by any handler in this diff; follows established project pattern
- [x] [Review][Defer] **D2: No rate limiting on room creation** [main.go:84] — deferred, infrastructure concern outside story scope
- [x] [Review][Defer] **D3: No per-user active room count cap** [handler.go:38-114] — deferred, business rule not specified in story requirements

**Dismissed (6 findings):** Partial index semantics (spec-intentional), GORM uniqueIndex tag vs migration naming (project uses CLI migrations), QuickPlay/BrowseRooms dead buttons (spec placeholders for 2.2/2.5), fetchClient data unwrapping (confirmed working at fetchClient.ts:124), double-submit race (theoretical, button disabled provides protection), room code index on soft-deleted rows (887M code space, non-issue).

### Change Log

- 2026-04-11: Story 2.1 implemented — room creation API, migration, frontend modal, lobby redesign, tests, i18n
- 2026-04-11: Code review — 10 patches applied (1 critical, 3 high, 4 medium, 2 low), 3 deferred, 6 dismissed. 7 new backend validation tests added.

### File List

**New files:**

- server/migrations/000003_create_rooms.up.sql
- server/migrations/000003_create_rooms.down.sql
- server/internal/room/model.go
- server/internal/room/repository.go
- server/internal/room/gorm_repo.go
- server/internal/room/handler.go
- server/internal/room/handler_test.go
- client/src/shared/api/rooms.ts
- client/src/shared/components/ui/select.tsx (shadcn install)
- client/src/features/lobby/CreateRoomModal.tsx
- client/src/features/lobby/CreateRoomModal.test.tsx
- client/src/features/lobby/RoomLobby.tsx
- client/src/features/lobby/LobbyPage.test.tsx

**Modified files:**

- server/internal/apperr/errors.go (added room errors)
- server/internal/ws/events.go (added SystemRoomCreated constant)
- server/cmd/api/main.go (wired room routes)
- client/src/shared/types/apiTypes.ts (added Room, CreateRoomRequest)
- client/src/shared/types/wsEvents.ts (added SYSTEM_ROOM_CREATED, RoomCreatedPayload)
- client/src/shared/stores/lobbyStore.ts (added rooms state + actions)
- client/src/shared/i18n/en.json (added lobby.\* keys)
- client/src/shared/i18n/sr.json (added lobby.\* keys)
- client/src/features/lobby/LobbyPage.tsx (redesigned with play option cards)
- client/src/App.tsx (added RoomLobby route)
- client/src/App.test.tsx (updated lobby assertion)
