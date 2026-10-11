# Story 2.2: Browse & Search Rooms

Status: done

## Story

As a player,
I want to browse and search available game rooms,
So that I can find a game to join.

## Acceptance Criteria

1. **Lobby Layout (Direction 5)** — When the lobby page loads, the Direction 5 layout is displayed: play options column on the left (Quick Play, Browse Rooms, Create Room as equal-weight cards) and a placeholder leaderboard panel on the right. Quick Play card has `accent-glow` background to signal recommended default action.

2. **Room List Loading** — When a player selects Browse Rooms, `GET /api/v1/rooms?status=waiting` returns all open rooms. Each room is displayed as a RoomCard showing: room name, variant, mode, player count (e.g., "2/4"), timer style.

3. **Live Search Filtering** — When a player types in the search bar, the room list live-filters by room name and room code without requiring a submit action. Results update as the player types.

4. **Empty State** — When no rooms match the search query, an empty state is shown: "No rooms match '[query]' — Clear search" with a clickable clear link.

5. **Real-Time Room Updates** — When rooms are created or filled by other players, the room list updates in real-time via WebSocket `system:room_updated` events without requiring page refresh.

## Tasks / Subtasks

- [x] **Task 1: Backend — Add `FindByStatus` to room repository** (AC: #2)
  - [x] Add `FindByStatus(status string) ([]Room, error)` to `RoomRepository` interface in `server/internal/room/repository.go`
  - [x] Implement in `server/internal/room/gorm_repo.go` — query `rooms` table filtered by `status`, ordered by `created_at DESC`

- [x] **Task 2: Backend — Add `ListRooms` handler** (AC: #2)
  - [x] Add `ListRoomsRequest` struct with `Status` query param
  - [x] Create `ListRooms` handler in `server/internal/room/handler.go` — read `status` from query string, default to `waiting`, call `repo.FindByStatus()`
  - [x] Return `{ "data": [...rooms] }` with HTTP 200
  - [x] Wire `api.GET("/rooms", roomHandler.ListRooms)` in `server/cmd/api/main.go` immediately after the existing `api.POST("/rooms", roomHandler.CreateRoom)` line

- [x] **Task 3: Backend — Add `system:room_updated` WebSocket event** (AC: #5)
  - [x] Add `SystemRoomUpdated = "system:room_updated"` to `server/internal/ws/events.go`
  - [x] Add `SYSTEM_ROOM_UPDATED` and `RoomUpdatedPayload` to `client/src/shared/types/wsEvents.ts`
  - [x] **Both files in the same commit**
  - [x] Note: Actual WS broadcast from handler is stubbed with TODO (WS hub not yet wired). The frontend will listen for this event type for when the hub is connected in a future story. Include the event type constant so the contract is established.

- [x] **Task 4: Frontend — Add `getRooms` API client function** (AC: #2)
  - [x] Add `getRooms(status?: string): Promise<Room[]>` to `client/src/shared/api/rooms.ts`
  - [x] Calls `GET /api/v1/rooms?status={status}` via `fetchClient`

- [x] **Task 5: Frontend — Expand lobbyStore with search state** (AC: #2, #3)
  - [x] Add `searchQuery: string` and `setSearchQuery(query: string)` to `lobbyStore`
  - [x] Add `filteredRooms` computed getter (or derive in component) that filters `rooms` by `searchQuery` matching room `name` or `code` (case-insensitive)

- [x] **Task 6: Frontend — Create RoomCard component** (AC: #2)
  - [x] Create `client/src/features/lobby/RoomCard.tsx`
  - [x] Displays: room name, variant badge, mode label, player count (e.g., "2/4" with dot indicators), timer style icon/label
  - [x] Ghost-style card matching existing lobby card design: `rounded-xl border border-border bg-surface p-4`
  - [x] Includes a "Join" button (primary style) — **non-functional in this story** (wired in Story 2.3)
  - [x] Use `data-testid="room-card"` and `data-testid="room-card-join"` for test selection

- [x] **Task 7: Frontend — Create RoomList component with search** (AC: #2, #3, #4)
  - [x] Create `client/src/features/lobby/RoomList.tsx`
  - [x] Search bar at top: shadcn `Input` component (`data-testid="room-list-search"`), placeholder text, live-filtering with no submit button
  - [x] Renders list of `RoomCard` components from filtered rooms
  - [x] Loading state: 3 skeleton placeholder rows (pulsing animation)
  - [x] Empty state when search has no results: "No rooms match '[query]' — Clear search" with clickable clear link that resets `searchQuery`
  - [x] Empty state when no rooms exist at all (no search active): "No open rooms — Create one or try Quick Play"

- [x] **Task 8: Frontend — Update LobbyPage with Browse Rooms view** (AC: #1, #2)
  - [x] Add `activeView: 'options' | 'browse'` state to `LobbyPage`
  - [x] Browse Rooms card click sets `activeView` to `'browse'` and fetches rooms via `getRooms('waiting')`
  - [x] When `activeView === 'browse'`: replace the play options column with `RoomList` component, add a back arrow/button (`data-testid="back-to-options"`) to return to options view
  - [x] When `activeView === 'options'`: show the current play option cards (existing behavior)
  - [x] On fetch: set `lobbyStore.setLoading(true)`, call `getRooms`, then `setRooms(result)` and `setLoading(false)`
  - [x] On fetch error: catch `FetchError`, call `setLoading(false)`, display error in a toast or inline message (e.g., "Failed to load rooms — try again"). Follow the error handling pattern from `CreateRoomModal.tsx`

- [x] **Task 9: Frontend — WebSocket room update listener** (AC: #5)
  - [x] Create `client/src/features/lobby/useRoomUpdates.ts` hook
  - [x] Import `SYSTEM_ROOM_CREATED` and `RoomCreatedPayload` from existing `shared/types/wsEvents.ts` — do NOT redefine these
  - [x] Import the new `SYSTEM_ROOM_UPDATED` and `RoomUpdatedPayload` from `shared/types/wsEvents.ts`
  - [x] Listen for `system:room_created`: call `useLobbyStore.getState().addRoom(payload)` (use `.getState()` to avoid stale closures)
  - [x] Listen for `system:room_updated`: call `useLobbyStore.getState().updateRoom(payload)`. If the room status is no longer `waiting`, call `useLobbyStore.getState().removeRoom(payload.id)` instead
  - [x] Call this hook from `LobbyPage` when `activeView === 'browse'`
  - [x] **Note:** The WS hub is not yet wired end-to-end. Create the hook with correct event handler logic but document in a code comment that it requires WS infrastructure from a future story. Use `useEffect` cleanup to unsubscribe.

- [x] **Task 10: i18n translations** (AC: #2, #3, #4)
  - [x] Add browse/search room strings to `client/src/shared/i18n/en.json` under `lobby.browseRooms.*`
  - [x] Add matching strings to `client/src/shared/i18n/sr.json`

- [x] **Task 11: Backend tests** (AC: #2)
  - [x] Add test cases to `server/internal/room/handler_test.go`:
    - ListRooms returns only rooms with matching status
    - ListRooms defaults to `status=waiting` when no query param
    - ListRooms returns empty array `[]` (not `null`) when no rooms match — verify JSON body
    - ListRooms requires authentication (401 without token)
  - [x] Extend existing mock repository with `FindByStatus` method — use an in-memory `[]*Room` slice and filter by status. **Critical:** return `[]Room{}` (not `nil`) when no rooms match, otherwise JSON serializes as `null`:

    ```go
    func (m *mockRoomRepo) FindByStatus(status string) ([]Room, error) {
        var result []Room
        for _, r := range m.rooms {
            if r.Status == status {
                result = append(result, *r)
            }
        }
        if result == nil {
            result = []Room{}
        }
        return result, m.findByStatusErr
    }
    ```

- [x] **Task 12: Frontend tests** (AC: #1, #2, #3, #4)
  - [x] Create `client/src/features/lobby/RoomCard.test.tsx` — renders room data correctly, displays all fields
  - [x] Create `client/src/features/lobby/RoomList.test.tsx` — renders room cards, search filters results, empty states display correctly, loading skeleton shown
  - [x] Update `client/src/features/lobby/LobbyPage.test.tsx` — Browse Rooms card click switches to browse view, back button returns to options

- [x] **Task 13: Lint and test** (AC: all)
  - [x] Run `make lint` — pass with zero new errors
  - [x] Run `make test` — all existing + new tests pass

## Dev Notes

### Architecture Compliance

**This story is primarily a read endpoint + frontend UI.** The backend work is minimal (one new GET handler + repository method). The bulk of the work is frontend: RoomCard, RoomList, search filtering, and LobbyPage state management.

**Backend domain stays in `room/` package** — the `GET /api/v1/rooms` endpoint belongs in the same domain package as `POST /api/v1/rooms` (Story 2.1). Do NOT create a separate `lobby/` backend package for this. The `lobby/` package is reserved for future matchmaking logic (Story 2.5).

**File organization:**

```
server/internal/room/
├── model.go          # Room GORM struct (existing, no changes)
├── repository.go     # Add FindByStatus to interface
├── gorm_repo.go      # Add FindByStatus implementation
├── handler.go        # Add ListRooms handler
└── handler_test.go   # Add ListRooms tests

client/src/features/lobby/
├── LobbyPage.tsx         # Modify: add browse view state switching
├── LobbyPage.test.tsx    # Modify: add browse view tests
├── CreateRoomModal.tsx   # Existing, no changes
├── CreateRoomModal.test.tsx  # Existing, no changes
├── RoomLobby.tsx         # Existing placeholder, no changes
├── RoomCard.tsx          # NEW
├── RoomCard.test.tsx     # NEW
├── RoomList.tsx          # NEW
├── RoomList.test.tsx     # NEW
└── useRoomUpdates.ts     # NEW (WS listener hook)
```

### Backend Implementation Details

**Repository method — `FindByStatus`:**

```go
func (r *GormRepository) FindByStatus(status string) ([]Room, error) {
    var rooms []Room
    if err := r.db.Where("status = ?", status).Order("created_at DESC").Find(&rooms).Error; err != nil {
        return nil, fmt.Errorf("finding rooms by status: %w", err)
    }
    return rooms, nil
}
```

- GORM `Find` returns an empty slice (not nil) when no records match — this is correct behavior
- GORM soft-delete scope automatically excludes `deleted_at IS NOT NULL` rows
- Order by `created_at DESC` so newest rooms appear first

**Handler — `ListRooms`:**

```go
func (h *RoomHandler) ListRooms(c echo.Context) error {
    status := c.QueryParam("status")
    if status == "" {
        status = "waiting"
    }

    rooms, err := h.repo.FindByStatus(status)
    if err != nil {
        return fmt.Errorf("listing rooms: %w", err)
    }

    return c.JSON(http.StatusOK, map[string]interface{}{"data": rooms})
}
```

- Requires authentication (route is under the `api` group which has auth middleware)
- Default filter is `waiting` — only shows rooms that are open for joining
- Response wraps in `{ "data": [...] }` per project API convention
- Returns empty array `[]` not `null` when no rooms exist

**Route wiring in `main.go`** (add after line 85):

```go
api.GET("/rooms", roomHandler.ListRooms)
```

### Frontend Implementation Details

**API client — `getRooms`:**

```typescript
export function getRooms(status: string = "waiting"): Promise<Room[]> {
  return fetchClient<Room[]>(`/rooms?status=${encodeURIComponent(status)}`);
}
```

- `fetchClient` auto-unwraps `{ "data": T }` — the function returns `Room[]` directly. Confirmed: `fetchClient` handles arrays in `data` the same as objects (see `fetchClient.ts:124` — `body.data` works for any JSON type)
- Default param `"waiting"` matches backend default

**RoomCard component:**

```
┌─────────────────────────────────────────┐
│  Room Name                    Join  →   │
│  Bitola · 1001 · 2/4 · Relaxed         │
└─────────────────────────────────────────┘
```

- Room name as primary text (`text-text-primary`, `font-semibold`)
- Meta info row: variant, mode, player count, timer style — `text-sm text-text-secondary`
- Player count format: `"2/4"` (current/max)
- Join button: Primary style (`bg-accent`) — non-functional placeholder for Story 2.3
- Card padding: `p-4` with `16px` internal spacing per UX spec
- Card gap: `8px` between cards per UX spec

**RoomList component:**

```
┌──────────────────────────────────────────┐
│  🔍 Search rooms by name or code...     │
├──────────────────────────────────────────┤
│  [RoomCard]                              │
│  [RoomCard]                              │
│  [RoomCard]                              │
└──────────────────────────────────────────┘
```

- Search input: shadcn `Input`, no submit button, filters as user types
- Filtering is **client-side** on the already-fetched room list — no additional API calls per keystroke
- Filter logic: case-insensitive match on `room.name` OR `room.code` containing the search query
- Use `useMemo` to derive filtered list from `rooms` and `searchQuery` for performance

**LobbyPage state management:**

- Add `activeView` local state: `'options' | 'browse'`
- Default: `'options'` (current behavior)
- Browse Rooms card click: set `activeView = 'browse'`, fetch rooms
- Back navigation: reset to `activeView = 'options'`, optionally clear search
- Do NOT use React Router for this — it's a single-page view toggle within the lobby, not a route change

**Loading state:**

- Use `lobbyStore.isLoading` for skeleton display
- Show 3 skeleton rows using a pulsing `animate-pulse` placeholder matching RoomCard dimensions
- Each store manages its own `isLoading` boolean — per project convention

### WebSocket Event Contract

**Add to `server/internal/ws/events.go`:**

```go
const SystemRoomUpdated = "system:room_updated"
```

**Add to `client/src/shared/types/wsEvents.ts`:**

```typescript
export const SYSTEM_ROOM_UPDATED = "system:room_updated" as const;

export interface RoomUpdatedPayload {
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
```

**Important:** The WS hub is not yet wired end-to-end. The handler TODO at `handler.go:137` remains a TODO. The frontend WS listener hook should be created with the correct event types so the contract is established, but it won't receive events until the WS infrastructure is connected in a later story. Document this clearly in code comments.

### i18n Keys

**Add to `en.json` under `lobby`:**

```json
{
  "lobby": {
    "browseRooms": "Browse Rooms", // already exists
    "browseRoomsDesc": "Find an open game to join", // already exists
    "roomList": {
      "searchPlaceholder": "Search rooms by name or code...",
      "backToOptions": "Back",
      "variant": "Variant",
      "mode": "Mode",
      "players": "{{current}}/{{max}}",
      "timerRelaxed": "Relaxed",
      "timerPerMove": "{{seconds}}s timer",
      "join": "Join",
      "loading": "Loading rooms...",
      "emptyNoRooms": "No open rooms \u2014 Create one or try Quick Play",
      "emptyNoMatch": "No rooms match '{{query}}' \u2014",
      "clearSearch": "Clear search"
    }
  }
}
```

**Add matching keys to `sr.json`:**

```json
{
  "lobby": {
    "roomList": {
      "searchPlaceholder": "Pretra\u017ei sobe po imenu ili kodu...",
      "backToOptions": "Nazad",
      "variant": "Varijanta",
      "mode": "Re\u017eim",
      "players": "{{current}}/{{max}}",
      "timerRelaxed": "Opu\u0161teno",
      "timerPerMove": "{{seconds}}s tajmer",
      "join": "Pridru\u017ei se",
      "loading": "U\u010ditavanje soba...",
      "emptyNoRooms": "Nema otvorenih soba \u2014 Napravi sobu ili probaj Brzu igru",
      "emptyNoMatch": "Nijedna soba ne odgovara '{{query}}' \u2014",
      "clearSearch": "Obri\u0161i pretragu"
    }
  }
}
```

### Testing Strategy

**Backend (Go):**

- External test package: `package room_test`
- Use mock repository from existing `handler_test.go` pattern — add `FindByStatus(status string) ([]Room, error)` to mock
- Test cases for ListRooms:
  - Returns rooms with status `waiting` by default
  - Respects `?status=` query parameter
  - Returns empty `[]` array when no rooms match (verify JSON is `[]` not `null`)
  - Requires authentication (401 without token)
  - Returns correct response envelope `{ "data": [...] }`

**Frontend (Vitest):**

- Co-locate all test files with their components
- Use `@testing-library/react` with `BrowserRouter` wrapper
- Use `data-testid` attributes for element selection — never CSS classes
- Mock `shared/api/rooms` module for API calls
- Test descriptions in present tense: `it('renders room list')` not `it('should render room list')`
- Test RoomCard: renders room name, variant, mode, player count, timer style
- Test RoomList: renders cards from data, search filters correctly, empty states shown, loading skeleton
- Test LobbyPage: browse card click triggers view switch, back button returns to options

### Previous Story Intelligence (from Story 2.1)

**Critical learnings to apply:**

- `fetchClient<T>()` auto-unwraps `{ data: T }` — do NOT add a second unwrap layer in `getRooms()`
- Use `auth.GetUserID(c)` from `auth/middleware.go` for user ID extraction (canonical public helper)
- Use external test packages (`room_test` not `room`) to prevent import cycles
- Mock repos return `(nil, nil)` for not-found cases — maintain this pattern for `FindByStatus` returning empty slice
- `verbatimModuleSyntax` in tsconfig requires `import type` for type-only imports
- Pre-existing GORM integration test failures exist (2 in `user/user_test.go`) — not caused by this story
- Base UI Dialog (not Radix) — the shadcn components use `@base-ui/react` under the hood. Input component should work the same way
- Stale closure prevention: use `useLobbyStore.getState()` in event handlers, not dependency closures

**Patterns from Story 2.1 to reuse:**

- Card component styling: `rounded-xl border border-border bg-surface p-6 text-left transition-colors hover:bg-surface/80`
- Primary CTA: `bg-accent-glow` for the prominent action
- i18n key convention: `lobby.{section}.{element}`
- API error handling: catch `FetchError` and check `.code` for specific error codes
- Use `useNavigate()` from React Router for navigation

**Deferred items from Story 2.1 review:**

- D1: `FindByID` returns `(nil, nil)` for missing rooms — this is the established pattern, follow it for `FindByStatus` returning empty slice
- D2: No rate limiting on room endpoints — still deferred, infrastructure concern
- D3: No per-user active room count cap — not in scope

### Git Intelligence

**Recent commit pattern:**

```
eead853 feat(room): implement room creation and configuration with code review fixes
6f3e1af feat(profile): implement player profile, navigation shell, and language selector with code review fixes
46a5949 feat(auth): implement user login and session persistence with code review fixes
```

- Format: `{type}({scope}): {description}`
- Scope for this story: `room` (same domain as 2.1)
- Expected commit: `feat(room): implement room browsing and search`

**Branch naming:** `feat/E2-S2-browse-search-rooms`

### Project Structure Notes

- Frontend lobby components go in `client/src/features/lobby/` — the folder already exists
- Backend room domain stays in `server/internal/room/` — no new packages needed
- API client file `client/src/shared/api/rooms.ts` already exists — add `getRooms` to it
- Lobby store `client/src/shared/stores/lobbyStore.ts` already has `rooms`, `setRooms`, `addRoom`, `updateRoom`, `removeRoom` — add only `searchQuery` and `setSearchQuery`
- No new shadcn components needed — `Input` is already installed
- No database migration needed — `rooms` table schema from Story 2.1 is sufficient

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic-2, Story 2.2]
- [Source: _bmad-output/planning-artifacts/architecture.md#Lobby-Domain, #Frontend-Feature-Organization, #State-Store-Partitioning]
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md#Direction-5, #Journey-3-Room-Join, #Loading-Empty-States, #Search-Patterns]
- [Source: _bmad-output/planning-artifacts/prd.md#FR17, #FR18]
- [Source: _bmad-output/project-context.md#Framework-Rules, #Testing-Rules, #Naming-Conventions]
- [Source: _bmad-output/archive/implementation-artifacts/2-1-create-room-and-room-configuration.md#Dev-Notes, #Review-Findings]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

- No blockers or halts encountered during implementation.

### Completion Notes List

- **Task 1:** Added `FindByStatus(status string) ([]Room, error)` to `RoomRepository` interface and implemented in `GormRepository` with status filter + `created_at DESC` ordering.
- **Task 2:** Added `ListRooms` handler reading `status` query param (default: `waiting`), returning `{ "data": [...] }` envelope. Wired `GET /rooms` route in main.go under auth middleware.
- **Task 3:** Added `SystemRoomUpdated` constant to Go `events.go` and `SYSTEM_ROOM_UPDATED` + `RoomUpdatedPayload` to TS `wsEvents.ts`. Both contract files updated together.
- **Task 4:** Added `getRooms(status)` to `rooms.ts` API client using `fetchClient` auto-unwrap.
- **Task 5:** Added `searchQuery` and `setSearchQuery` to `lobbyStore`. Filtering derived in `RoomList` component via `useMemo`.
- **Task 6:** Created `RoomCard.tsx` — displays room name, variant, mode, player count (N/4), timer style, and non-functional Join button.
- **Task 7:** Created `RoomList.tsx` — search input with live filtering by name/code, loading skeleton (3 pulsing rows), empty states for no-match and no-rooms.
- **Task 8:** Updated `LobbyPage.tsx` with `activeView` state toggle (`options`/`browse`), fetch on browse click, back button, error handling via `FetchError`.
- **Task 9:** Created `useRoomUpdates.ts` hook with exported `handleWsMessage` function. Event handlers for `system:room_created` (addRoom) and `system:room_updated` (updateRoom/removeRoom) using `getState()` for stale closure prevention. WS not yet wired — documented in code comments.
- **Task 10:** Added `lobby.roomList.*` i18n keys to both `en.json` and `sr.json`.
- **Task 11:** Added 4 backend tests: `TestListRooms_DefaultsToWaiting`, `TestListRooms_RespectsStatusParam`, `TestListRooms_EmptyArrayNotNull`, `TestListRooms_Unauthorized`. Extended mock repo with `FindByStatus`.
- **Task 12:** Created `RoomCard.test.tsx` (6 tests), `RoomList.test.tsx` (7 tests), updated `LobbyPage.test.tsx` (5 tests total, +3 new browse view tests).
- **Task 13:** All 21 Go room tests pass. All 68 frontend tests pass. Zero new ESLint errors. Pre-existing `user/user_test.go` GORM failures (2) unrelated to this story.

### Review Findings

- [x] [Review][Defer] F2: `handleWsMessage` uses unsafe `as` casts on raw `JSON.parse` output — violates project WS dispatch rule. Deferred to Story 4-1 (WS Gateway) where the central typed dispatch function will be built. [client/src/features/lobby/useRoomUpdates.ts:28-36]
- [x] [Review][Patch] F1: `ListRooms` accepts arbitrary unvalidated `status` parameter — added allowlist validation + `ErrInvalidRoomStatus` error + test [server/internal/room/handler.go]
- [x] [Review][Patch] F3: `useRoomUpdates` called unconditionally in LobbyPage — moved into `RoomList` component (only mounts during browse view) [client/src/features/lobby/RoomList.tsx]
- [x] [Review][Patch] F4: Stale rooms and search query not cleared on browse re-entry — added `setRooms([])` and `setSearchQuery("")` at start of `handleBrowseRooms` and in `handleBackToOptions` [client/src/features/lobby/LobbyPage.tsx]
- [x] [Review][Patch] F5: `handleBrowseRooms` not re-entrant-safe — added `isLoading` guard at function entry [client/src/features/lobby/LobbyPage.tsx]
- [x] [Review][Patch] F6: `RoomCard` renders `"null s timer"` when `timerDurationSeconds` is null — added `?? "?"` fallback [client/src/features/lobby/RoomCard.tsx]

### Change Log

- 2026-04-11: Implemented Story 2.2 — Browse & Search Rooms (all 13 tasks complete)

### File List

**New files:**

- client/src/features/lobby/RoomCard.tsx
- client/src/features/lobby/RoomCard.test.tsx
- client/src/features/lobby/RoomList.tsx
- client/src/features/lobby/RoomList.test.tsx
- client/src/features/lobby/useRoomUpdates.ts

**Modified files:**

- server/internal/room/repository.go (added FindByStatus to interface)
- server/internal/room/gorm_repo.go (added FindByStatus implementation)
- server/internal/room/handler.go (added ListRooms handler)
- server/internal/room/handler_test.go (added ListRooms tests + FindByStatus mock)
- server/cmd/api/main.go (added GET /rooms route)
- server/internal/ws/events.go (added SystemRoomUpdated constant)
- client/src/shared/types/wsEvents.ts (added SYSTEM_ROOM_UPDATED + RoomUpdatedPayload)
- client/src/shared/api/rooms.ts (added getRooms function)
- client/src/shared/stores/lobbyStore.ts (added searchQuery + setSearchQuery)
- client/src/features/lobby/LobbyPage.tsx (added browse view with state toggle)
- client/src/features/lobby/LobbyPage.test.tsx (added browse view tests)
- client/src/shared/i18n/en.json (added lobby.roomList.\* keys)
- client/src/shared/i18n/sr.json (added lobby.roomList.\* keys)
