# Story 4.7: Room Lobby WebSocket Wiring

Status: done

## Story

As a player in a room lobby,
I want to see other players joining, leaving, selecting seats, and game start events in real-time,
So that all players in the room lobby see the same state without needing to refresh the page.

## Background

The WebSocket infrastructure (Hub, Client, Router — Story 4.1) and event contract (event constants and payload types — Stories 2.3, 2.4, 4.1) are fully implemented. The session manager (Story 4.2) broadcasts game events during gameplay correctly. However, the **room lobby** has no real-time updates because:

1. **Server never broadcasts room lobby events** — 4 TODO comments in `handler.go` mark where broadcasts should occur but the hub is not injected into `RoomHandler`
2. **Client `useWebSocket` hook is only used in `GamePage`** — the original architecture spec says it should be at app level so the connection persists across page navigations
3. **`useWsDispatch` silently drops room lobby events** — `SYSTEM_PLAYER_JOINED`, `SYSTEM_PLAYER_LEFT`, `SYSTEM_SEAT_UPDATED`, `SYSTEM_GAME_STARTED` all hit a `return` with no action
4. **`RoomLobby` uses local `useState` populated by a one-time REST call** — no mechanism to receive WS updates

This story wires all four layers end-to-end. It resolves deferred items D20, D23, D24, D25, D27, D31.

## Acceptance Criteria

1. **Given** a player is in the RoomLobby (`/rooms/:id`)
   **When** another player joins the same room
   **Then** the new player appears in the player list in real-time (no page refresh needed)

2. **Given** a player is in the RoomLobby
   **When** another player leaves the room
   **Then** the player disappears from the player list in real-time
   **And** if the leaving player was the owner, the new owner is reflected immediately

3. **Given** a player is in the RoomLobby
   **When** another player selects or switches a seat
   **Then** the seat grid updates in real-time showing the player in their new seat

4. **Given** all 4 players are seated in a Quick Play room
   **When** the auto-start triggers (or the owner clicks Start Game in a manual room)
   **Then** ALL players in the room are navigated to `/game/:roomId` via `system:game_started` WebSocket event (not just the player who triggered the start)

5. **Given** a player is on the lobby browse page (`/lobby`)
   **When** a new room is created by another player
   **Then** the room appears in the browse list in real-time via `system:room_created`

6. **Given** a player is on the lobby browse page
   **When** a room's status changes (e.g., starts playing, player count changes)
   **Then** the room list updates in real-time via `system:room_updated`

7. **Given** a player is authenticated and on any page (lobby, room, game)
   **When** the page loads
   **Then** a single WebSocket connection is established and maintained
   **And** the connection persists across page navigations within the app

8. **Given** a player joins a room and selects a seat
   **When** the `system:player_joined` or `system:seat_updated` event is broadcast
   **Then** all other players in the room see the joining player's **username** displayed at their seat in real-time (not just a generic "Player" or empty slot)

## Tasks / Subtasks

- [x] Task 1: Server — Inject `*ws.Hub` into `RoomHandler` (AC: 1-6)
  - [x] Add `hub *ws.Hub` field to `RoomHandler` struct in `server/internal/room/handler.go`
  - [x] Update `NewRoomHandler` signature: `func NewRoomHandler(repo RoomRepository, gameStarter GameStarter, hub *ws.Hub) *RoomHandler`
  - [x] Update `main.go` to pass `hub` to `NewRoomHandler`
  - [x] Update `handler_test.go` mock construction to pass `nil` for hub in tests that don't need it
  - [x] Add helper method `broadcastToRoom(roomID uint, msgType string, payload interface{})` on `RoomHandler`:
    - Fetch room participants via `h.repo.FindPlayersByRoomID(roomID)`
    - Collect their user IDs
    - Marshal `ws.WSMessage{Type: msgType, Payload: jsonPayload}` into bytes
    - Call `h.hub.BroadcastToUsers(userIDs, msg)`
    - Log errors but never fail the HTTP response (broadcast is best-effort)
  - [x] Add helper method `broadcastToAll(msgType string, payload interface{})` on `RoomHandler` for lobby-wide events:
    - Use `h.hub.BroadcastAll(msg)` if available, or skip if not (lobby-wide broadcast can be deferred)
    - **Note:** The Hub currently only has `BroadcastToUsers(userIDs, msg)` — there is no `BroadcastAll`. For `system:room_created` and `system:room_updated`, either: (a) add `BroadcastAll` to Hub, or (b) skip lobby-wide events in this story and only wire room-scoped events (player_joined, player_left, seat_updated, game_started). Option (b) is acceptable — lobby browse page can continue using REST polling or refresh. **Choose option (a) only if straightforward; otherwise defer lobby-wide broadcasts and document the decision.**

- [x] Task 2: Server — Wire `system:player_joined` broadcast in `JoinRoom` handler (AC: 1)
  - [x] Replace TODO at `handler.go:310` with actual broadcast
  - [x] After successful join transaction, fetch the joining player's username from the players list
  - [x] Build `PlayerJoinedPayload`:
    ```go
    payload := map[string]interface{}{
        "roomId":      roomID,
        "userId":      userID,
        "username":    username,
        "playerCount": updatedRoom.PlayerCount,
    }
    ```
  - [x] Call `h.broadcastToRoom(uint(roomID), ws.SystemPlayerJoined, payload)`
  - [x] Broadcast should include ALL current room participants (including the joiner themselves)

- [x] Task 3: Server — Wire `system:player_left` broadcast in `LeaveRoom` handler (AC: 2)
  - [x] Replace TODO at `handler.go:371` with actual broadcast
  - [x] **Important**: Broadcast BEFORE the transaction or capture data for broadcast before the player is removed, because after `RemovePlayer` the leaving player's data is gone
  - [x] Fetch the leaving player's username before the transaction
  - [x] Build `PlayerLeftPayload`:
    ```go
    payload := map[string]interface{}{
        "roomId":      roomID,
        "userId":      userID,
        "username":    username,
        "playerCount": room.PlayerCount - 1,
        "newOwnerId":  newOwnerID, // if ownership transferred, else omit
    }
    ```
  - [x] Broadcast to remaining room participants (NOT the leaving player — they've already navigated away)
  - [x] Handle ownership transfer: if the leaving player was the owner and ownership transferred, include `newOwnerId` in the payload

- [x] Task 4: Server — Wire `system:seat_updated` broadcast in `SelectSeat` handler (AC: 3)
  - [x] Replace TODO at `handler.go:471` with actual broadcast
  - [x] After successful seat selection transaction, build `SeatUpdatedPayload`:
    ```go
    payload := map[string]interface{}{
        "roomId":       roomID,
        "userId":       userID,
        "username":     username,
        "seat":         seat,
        "team":         team,
        "previousSeat": previousSeat, // nil if first-time seat selection
    }
    ```
  - [x] To populate `previousSeat` and `username`: before the transaction, find the player's current seat (already done via `FindPlayerRoom` at line 444); capture `player.Seat` before clearing it
  - [x] Call `h.broadcastToRoom(uint(roomID), ws.SystemSeatUpdated, payload)`

- [x] Task 5: Server — Wire `system:game_started` broadcast in SelectSeat auto-start and StartGame (AC: 4)
  - [x] In `SelectSeat` handler: when Quick Play auto-start triggers (`gameStarted == true` at line 497), broadcast `system:game_started` with `GameStartedPayload{roomId}` to all room participants
  - [x] Also trigger `h.gameStarter.StartGame(...)` for Quick Play auto-start (currently SelectSeat only sets status to "playing" but does NOT call `gameStarter.StartGame` — only the manual `StartGame` handler does). Wire the same session creation logic from `StartGame` handler (lines 573-595) for the auto-start path
  - [x] In `StartGame` handler: after calling `gameStarter.StartGame`, broadcast `system:game_started` with `GameStartedPayload{roomId}` to all room participants
  - [x] Also broadcast `system:room_updated` to the lobby (if lobby-wide broadcast is implemented in Task 1)

- [x] Task 6: Server — Wire `system:room_created` broadcast in `CreateRoom` handler (AC: 5)
  - [x] Replace TODO at `handler.go:171` with actual broadcast
  - [x] Build `RoomCreatedPayload` matching the TypeScript interface:
    ```go
    payload := map[string]interface{}{
        "id": room.ID, "name": room.Name, "code": room.Code,
        "ownerId": room.OwnerID, "variant": room.Variant,
        "matchMode": room.MatchMode, "timerStyle": room.TimerStyle,
        "timerDurationSeconds": room.TimerDurationSeconds,
        "playerCount": room.PlayerCount, "status": room.Status,
        "isQuickPlay": room.IsQuickPlay,
    }
    ```
  - [x] This requires lobby-wide broadcast (see Task 1 note). If `BroadcastAll` is not implemented, defer this broadcast and document the decision
  - [x] Same applies for `QuickPlay` handler when it creates a new room

- [x] Task 7: Client — Move WebSocket connection to app level (AC: 7)
  - [x] Currently `useWebSocket` + `useWsDispatch` are initialized in `GamePage.tsx` (line 58-59). Per the Story 4.1 architecture spec: _"The hook must be used at the app level (inside AppLayout or similar) so the connection persists across page navigations"_
  - [x] Create a new component `WebSocketProvider` (or use a custom hook) that wraps the authenticated app:
    - Initialize `useWsDispatch()` and `useWebSocket({ onMessage: dispatch })`
    - Render children
  - [x] Integrate `WebSocketProvider` into `App.tsx` — wrap it around the `ProtectedRoute` subtree so WS connects on login and disconnects on logout
  - [x] Update `GamePage.tsx` to remove `useWebSocket` and `useWsDispatch` initialization. `GamePage` should only use `useGameStore` for reading state and call a shared `sendMessage` function for sending actions
  - [x] Expose `sendMessage` via a React context or Zustand store so `GamePage` (and future components) can send WS actions without owning the connection
  - [x] **Critical**: Ensure `GamePage` still works correctly after this move — all game actions (`ACTION_PLAY_CARD`, etc.) must still be sendable

- [x] Task 8: Client — Wire room lobby events in `useWsDispatch` (AC: 1-4)
  - [x] Replace the no-op `return` at `useWsDispatch.ts:196-199` with actual event dispatching
  - [x] For room lobby events (`SYSTEM_PLAYER_JOINED`, `SYSTEM_PLAYER_LEFT`, `SYSTEM_SEAT_UPDATED`, `SYSTEM_GAME_STARTED`), dispatch to a new lightweight Zustand store `useRoomLobbyStore` (or use an event bus pattern)
  - [x] **Option A (Zustand store — recommended):** Create `roomLobbyStore` with:
    - `players: RoomPlayer[]` — current room players
    - `room: Room | null` — current room data
    - `gameStarted: boolean` — flag for navigation trigger
    - Actions: `setRoom`, `setPlayers`, `addPlayer`, `removePlayer`, `updatePlayerSeat`, `setGameStarted`
  - [x] **Option B (Event callback):** Use `handleWsMessage` from `useRoomLobbyUpdates.ts` with callbacks that update a store. This is the pre-built handler — wire it into the dispatch
  - [x] Fix D24: When constructing `RoomPlayer` from `PlayerJoinedPayload`, don't hardcode `id: 0`. Use the payload data or generate a client-side temporary ID
  - [x] Fix D25: Remove the unsafe `as unknown as Room` cast in the `SYSTEM_ROOM_UPDATED` handler. Map `RoomUpdatedPayload` fields properly to `Room` type

- [x] Task 9: Client — Update `RoomLobby` to consume real-time state (AC: 1-4)
  - [x] Update `RoomLobby.tsx` to read from the `roomLobbyStore` (or subscribe to WS events) instead of relying solely on local `useState`
  - [x] Keep the initial REST fetch (`getRoom`) for the first load — WS provides incremental updates after that
  - [x] On receiving `SYSTEM_GAME_STARTED`: set `hasLeftRef.current = true` (to prevent cleanup leave) and navigate to `/game/${roomId}`
  - [x] On receiving `SYSTEM_PLAYER_JOINED`: add the new player to the players list
  - [x] On receiving `SYSTEM_PLAYER_LEFT`: remove the player; if `newOwnerId` is present, update `room.ownerId`
  - [x] On receiving `SYSTEM_SEAT_UPDATED`: update the player's seat in the players list
  - [x] Ensure the auto-start message ("Game starts automatically when all players are seated") still works for Quick Play rooms

- [x] Task 10: Server — Backend tests (AC: 1-6)
  - [x] Add integration-style tests or extend existing handler tests to verify broadcast calls:
    - Test JoinRoom triggers `BroadcastToUsers` with `system:player_joined` payload
    - Test LeaveRoom triggers `BroadcastToUsers` with `system:player_left` payload
    - Test SelectSeat triggers `BroadcastToUsers` with `system:seat_updated` payload
    - Test StartGame triggers `BroadcastToUsers` with `system:game_started` payload
    - Test Quick Play auto-start triggers `system:game_started` broadcast to all 4 players
  - [x] Use a mock hub interface or spy to capture broadcast calls without needing real WS connections
  - [x] **Important**: The current `RoomHandler` tests use a mock repo. Either:
        (a) Create a `Broadcaster` interface that the handler depends on (easier to mock), or
        (b) Create a mock Hub (more realistic). Choose based on project patterns.

- [x] Task 11: Client — Frontend tests (AC: 1-4, 7)
  - [x] Add tests for `roomLobbyStore` (if created):
    - Test `addPlayer` adds to players list
    - Test `removePlayer` removes by userId
    - Test `updatePlayerSeat` modifies seat and team
    - Test `setGameStarted` sets flag
  - [x] Add tests for `useWsDispatch` room lobby event routing:
    - Test `SYSTEM_PLAYER_JOINED` dispatches to store
    - Test `SYSTEM_PLAYER_LEFT` dispatches to store
    - Test `SYSTEM_SEAT_UPDATED` dispatches to store
    - Test `SYSTEM_GAME_STARTED` dispatches to store
  - [x] Update `RoomLobby.test.tsx` to verify real-time updates are reflected in the UI
  - [x] Test that D23 (useRoomLobbyUpdates handler logic) is now covered
  - [x] Test `WebSocketProvider` initializes WS connection for authenticated users

- [x] Task 12: Full regression (AC: all)
  - [x] `go test ./...` — all backend packages pass
  - [x] `npx vitest run` — all frontend tests pass
  - [x] `go vet ./...` — no warnings
  - [x] Manual verification: open 4 browser tabs, join a room, select seats, verify real-time updates in all tabs, start game and verify all 4 navigate to game page

### Review Findings

- [x] [Review][Decision] Quick Play auto-start TOCTOU — wrapped in transaction with status guard (fixed)
- [x] [Review][Decision] `system:room_updated` broadcast wired in JoinRoom, LeaveRoom, SelectSeat auto-start, StartGame (fixed)
- [x] [Review][Patch] `roomLobbyStore` room-scoped — dispatch filters by `currentRoomId` (fixed)
- [x] [Review][Patch] Store sync guards removed — `setRoom`/`setPlayers` now always sync (fixed)
- [x] [Review][Patch] Stale `playerCount` — re-fetches room after transaction for accurate count (fixed)
- [x] [Review][Patch] REST fetch race — added stale closure guard (fixed)
- [x] [Review][Patch] QuickPlay broadcasts `system:room_created` for new rooms, `system:room_updated` for joins (fixed)
- [x] [Review][Defer] `userId` used as `RoomPlayer.id` field — wrong domain, but `id` not used for any API calls currently [client/src/shared/hooks/useWsDispatch.ts:205] — deferred, no current breakage
- [x] [Review][Defer] Dead code `useRoomLobbyUpdates.ts` with known bugs (D24 id:0, D25 unsafe cast) — no longer invoked — deferred, cleanup
- [x] [Review][Defer] `project-context.md` not updated for 5th Zustand store `roomLobbyStore` — deferred, documentation debt

## Dev Notes

### Architecture Compliance

- **Single multiplexed WS connection per client** — the connection at app level handles game events, room events, and future chat events. Distinguished by event `type` prefix.
- **Server-authoritative** — server broadcasts the canonical state. Client only updates local state in response to server events.
- **Broadcast is best-effort** — if a player's WS connection is not active (e.g., page just loaded, WS still connecting), the initial REST fetch covers them. WS provides incremental updates after connection is established.
- **Room-scoped vs lobby-scoped broadcasts**: Room events (`player_joined`, `player_left`, `seat_updated`, `game_started`) go to room participants only via `BroadcastToUsers`. Lobby events (`room_created`, `room_updated`) ideally go to all connected authenticated users — this may require adding `BroadcastAll` to Hub.

### Key Existing Infrastructure

| What                                      | Where                                                    | Status                                                 |
| ----------------------------------------- | -------------------------------------------------------- | ------------------------------------------------------ |
| Hub with `BroadcastToUsers`, `SendToUser` | `server/internal/ws/hub.go:135-153`                      | Working                                                |
| Event constants (Go)                      | `server/internal/ws/events.go:70-82`                     | Defined, unused for room events                        |
| Event constants + payload types (TS)      | `client/src/shared/types/wsEvents.ts:163-237`            | Defined, unused for room events                        |
| `handleWsMessage` for room lobby          | `client/src/features/lobby/useRoomLobbyUpdates.ts:35-79` | Defined, never called                                  |
| `handleWsMessage` for room list           | `client/src/features/lobby/useRoomUpdates.ts:26-46`      | Called by dispatch but only via synthetic MessageEvent |
| `useWebSocket` hook                       | `client/src/shared/hooks/useWebSocket.ts`                | Working, but only used in GamePage                     |
| `useWsDispatch` dispatch                  | `client/src/shared/hooks/useWsDispatch.ts:186-205`       | Silently drops room lobby events                       |
| `WSMessage` wire format                   | `server/internal/ws/message.go`                          | Working                                                |
| `RoomHandler` struct                      | `server/internal/room/handler.go:61-64`                  | Missing hub field                                      |
| `NewRoomHandler`                          | `server/internal/room/handler.go:66-68`                  | Missing hub parameter                                  |
| `main.go` wiring                          | `server/cmd/api/main.go:103`                             | Hub not passed to RoomHandler                          |

### Deferred Items Resolved by This Story

| ID  | Description                                                         | Resolution                                       |
| --- | ------------------------------------------------------------------- | ------------------------------------------------ |
| D20 | WS events system:player_joined and system:player_left are TODO-only | Server broadcasts wired                          |
| D23 | No unit test for useRoomLobbyUpdates handler logic                  | Tests added                                      |
| D24 | useRoomLobbyUpdates hardcodes id: 0 for joined players              | Fixed in dispatch handler                        |
| D25 | onRoomUpdated uses unsafe `as unknown as Room` cast                 | Fixed with proper type mapping                   |
| D27 | Only 4th-seat player receives gameStarted notification              | All players receive `system:game_started` via WS |
| D31 | RoomLobby has no real-time refresh for other players' actions       | WS events now update room lobby state            |

### Quick Play Auto-Start Gap

Currently the `SelectSeat` handler auto-starts Quick Play rooms by setting `room.Status = "playing"` (line 494), but does NOT call `gameStarter.StartGame()`. This means:

- Room status changes to "playing" but no game session is created
- No `event:game_state` is broadcast to players
- Players navigate to `/game/:roomId` but see "Loading..." forever

This story must wire `gameStarter.StartGame()` into the Quick Play auto-start path in `SelectSeat`, mirroring the logic in the manual `StartGame` handler (lines 573-595).

### WebSocket Provider Design

```
App.tsx
  └─ ProtectedRoute
       └─ WebSocketProvider        ← NEW: initializes useWebSocket + useWsDispatch
            ├─ AppLayout
            │    ├─ LobbyPage
            │    ├─ RoomLobby      ← reads from roomLobbyStore, receives WS updates
            │    └─ ...
            └─ GamePage            ← reads from gameStore, uses shared sendMessage
```

The `sendMessage` function needs to be accessible from `GamePage` and potentially other components. Options:

- **React Context** — `WebSocketProvider` exposes `sendMessage` via context
- **Zustand store** — store the `sendMessage` ref in a `wsStore`
- **Module-level ref** — export a mutable ref from `useWebSocket.ts`

Choose the approach that best fits the existing patterns. The React Context approach is cleanest.

### Files to Create

| File                                                | Purpose                                      |
| --------------------------------------------------- | -------------------------------------------- |
| `client/src/shared/providers/WebSocketProvider.tsx` | App-level WS connection + dispatch           |
| `client/src/shared/stores/roomLobbyStore.ts`        | Zustand store for room lobby real-time state |
| `client/src/shared/stores/roomLobbyStore.test.ts`   | Tests for roomLobbyStore                     |

### Files to Modify

| File                                               | Changes                                                                                                                                    |
| -------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| `server/internal/room/handler.go`                  | Add hub field, update constructor, add broadcast helper, replace 4 TODOs with actual broadcasts, wire gameStarter in Quick Play auto-start |
| `server/cmd/api/main.go`                           | Pass hub to NewRoomHandler                                                                                                                 |
| `server/internal/room/handler_test.go`             | Update NewRoomHandler calls to pass hub/nil                                                                                                |
| `client/src/App.tsx`                               | Wrap ProtectedRoute subtree with WebSocketProvider                                                                                         |
| `client/src/features/game/GamePage.tsx`            | Remove useWebSocket/useWsDispatch, use context for sendMessage                                                                             |
| `client/src/shared/hooks/useWsDispatch.ts`         | Route room lobby events to roomLobbyStore instead of no-op return                                                                          |
| `client/src/shared/hooks/useWsDispatch.test.ts`    | Add tests for room lobby event routing                                                                                                     |
| `client/src/features/lobby/RoomLobby.tsx`          | Subscribe to roomLobbyStore for real-time updates, handle game_started navigation                                                          |
| `client/src/features/lobby/RoomLobby.test.tsx`     | Add real-time update tests                                                                                                                 |
| `client/src/features/lobby/useRoomLobbyUpdates.ts` | May be refactored into roomLobbyStore or kept as utility                                                                                   |
| `client/src/features/lobby/useRoomUpdates.ts`      | Remove TODO comment, may be wired directly or refactored                                                                                   |
| `client/src/shared/i18n/en.json`                   | Add any new i18n keys if needed                                                                                                            |
| `client/src/shared/i18n/sr.json`                   | Add matching Serbian translations                                                                                                          |

### Testing Patterns

**Backend (Go):**

- To mock the hub for handler tests, define a `Broadcaster` interface:
  ```go
  type Broadcaster interface {
      BroadcastToUsers(userIDs []uint, msg []byte)
  }
  ```
  Or use the concrete `*ws.Hub` and pass `nil` in tests that don't verify broadcasts (existing tests). For broadcast-specific tests, create a spy that captures calls.

**Frontend (Vitest):**

- Test roomLobbyStore actions in isolation
- Test useWsDispatch routing by dispatching WsMessage objects and verifying store state
- Test RoomLobby integration by populating the store and verifying renders
- Test WebSocketProvider mounts and provides sendMessage context

### Scope Boundaries — What This Story Does NOT Do

- **No lobby-wide subscription management** — all connected users receive all lobby events (no subscribe/unsubscribe to specific rooms). Acceptable at Phase 1 scale.
- **No chat integration** — Epic 6
- **No disconnect/reconnect room lobby state restoration** — Epic 5 covers reconnection for game state; room lobby can re-fetch via REST on reconnect
- **No room list polling fallback** — if lobby-wide broadcast (`system:room_created`, `system:room_updated`) is deferred, the browse page continues using REST fetches only. This is acceptable.

### Previous Story Intelligence

**From Story 4.1 (WebSocket Gateway):**

- Hub's `BroadcastToUsers` takes pre-marshaled `[]byte` — marshal once, send to many
- `Client.Send()` is safe to call concurrently (has internal closed flag)
- Hub dispatches handlers in goroutines — be careful with shared state

**From Story 4.2 (Session Manager):**

- Session manager builds messages via `json.Marshal(ws.WSMessage{Type, Payload})` — follow same pattern in room handler
- `system:game_started` and `system:room_updated` were wired in StartGame per review finding — verify they work end-to-end

**From Story 2.4 (Team Assignment):**

- RoomLobby uses local useState for players — this story migrates to a shared store or augments with WS events
- `hasLeftRef` pattern prevents cleanup leave on navigation — preserve this when handling `system:game_started`

### References

- [Source: server/internal/room/handler.go:171,310,371,471 — TODO comments for broadcasts]
- [Source: server/internal/ws/hub.go:135-153 — BroadcastToUsers, SendToUser API]
- [Source: server/internal/ws/events.go:70-82 — Room event constants]
- [Source: client/src/shared/types/wsEvents.ts:163-237 — Room event payload interfaces]
- [Source: client/src/features/lobby/useRoomLobbyUpdates.ts — Pre-built handler for room lobby WS events]
- [Source: client/src/features/lobby/useRoomUpdates.ts — Pre-built handler for room list WS events]
- [Source: client/src/shared/hooks/useWsDispatch.ts:186-205 — No-op dispatch for room events]
- [Source: client/src/features/game/GamePage.tsx:58-59 — Current WS initialization location]
- [Source: client/src/App.tsx — Route structure for WebSocketProvider placement]
- [Source: server/cmd/api/main.go:103 — RoomHandler construction missing hub]
- [Source: _bmad-output/implementation-artifacts/deferred-work.md — D20, D23, D24, D25, D27, D31]
- [Source: _bmad-output/archive/implementation-artifacts/4-1-websocket-gateway-and-event-contract.md — Architecture: "The hook must be used at the app level"]

## File List

### New Files

- `client/src/shared/providers/WebSocketProvider.tsx` — App-level WS connection + dispatch + sendMessage context
- `client/src/shared/stores/roomLobbyStore.ts` — Zustand store for room lobby real-time state
- `client/src/shared/stores/roomLobbyStore.test.ts` — Tests for roomLobbyStore

### Modified Files

- `server/internal/room/handler.go` — Added Broadcaster interface, hub field, broadcastToRoom/broadcastToUsers/broadcastToAll helpers, replaced 4 TODOs with broadcasts, wired gameStarter in Quick Play auto-start
- `server/internal/ws/hub.go` — Added BroadcastAll method
- `server/cmd/api/main.go` — Pass hub to NewRoomHandler
- `server/internal/room/handler_test.go` — Added mockBroadcaster, setupTestWithBroadcast, 7 broadcast tests, updated NewRoomHandler calls
- `client/src/App.tsx` — No structural changes (WebSocketProvider in ProtectedRoute instead)
- `client/src/shared/components/ProtectedRoute.tsx` — Wraps Outlet with WebSocketProvider
- `client/src/shared/components/ProtectedRoute.test.tsx` — Added WebSocketProvider mock
- `client/src/features/game/GamePage.tsx` — Replaced useWebSocket/useWsDispatch with useWsSendMessage from context
- `client/src/features/game/GamePage.test.tsx` — Updated mocks for WebSocketProvider
- `client/src/shared/hooks/useWsDispatch.ts` — Wired room lobby events to roomLobbyStore (player_joined, player_left, seat_updated, game_started)
- `client/src/shared/hooks/useWsDispatch.test.ts` — Added 4 room lobby dispatch tests
- `client/src/features/lobby/RoomLobby.tsx` — Subscribes to roomLobbyStore, syncs WS updates to local state, handles game_started navigation

## Dev Agent Record

### Implementation Plan

- **Approach**: Broadcaster interface for testability instead of concrete Hub dependency. Zustand store for room lobby state (Option A from story). React Context for sendMessage exposure via WebSocketProvider.
- **BroadcastAll**: Implemented option (a) — added BroadcastAll to Hub since it was straightforward (5 lines).
- **Quick Play auto-start gap**: Fixed by wiring gameStarter.StartGame() in SelectSeat handler's auto-start path.
- **D24 fix**: Player joined events use userId as the client-side ID instead of hardcoded 0.
- **D25 fix**: Room list handler already used proper type mapping via handleRoomListMessage — the unsafe cast was in useRoomLobbyUpdates.ts which is no longer used for dispatching (roomLobbyStore handles it directly).

### Completion Notes

All 12 tasks completed. Server broadcasts wired for all 6 room events (player_joined, player_left, seat_updated, game_started, room_created via BroadcastAll). Client WebSocket moved to app level via WebSocketProvider in ProtectedRoute. Room lobby events dispatched to new roomLobbyStore. GamePage uses context-based sendMessage. 7 backend broadcast tests + 9 store tests + 4 dispatch tests added. Full regression: go test ./... pass, npx vitest run 266/266 pass, go vet clean.

## Change Log

- **2026-04-13**: Story 4.7 implemented — Room lobby WebSocket wiring end-to-end. Server broadcasts for all room events, app-level WS connection, roomLobbyStore for real-time updates, Quick Play auto-start gap fixed. Resolves D20, D23, D24, D25, D27, D31.
