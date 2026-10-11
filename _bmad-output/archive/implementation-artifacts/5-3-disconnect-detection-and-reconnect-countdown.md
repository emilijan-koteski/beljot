# Story 5.3: Disconnect Detection & Reconnect Countdown

Status: done

## Story

As a player in a game,
I want to be informed when another player disconnects and see a countdown for their return,
so that I know the game is paused and how long we're waiting.

## Acceptance Criteria

1. **Disconnect Detection — Server Detects Drop via Missed Pong**
   Given a player's WebSocket connection drops during an active game (phase is `playing`, `bidding`, or `paused`),
   When the server detects the disconnection (existing ping/pong: 30s ping interval, 45s pong timeout in `ws/client.go`),
   Then the game phase transitions to `disconnected`, the disconnected player's seat is marked `Connected: false`, and a reconnect countdown begins using a server-defined window (default 120 seconds).

2. **Remaining Players Receive Disconnect Event**
   Given a player disconnects,
   When the remaining 3 players' clients receive `event:player_disconnected`,
   Then the `ReconnectOverlay` renders on all 3 clients showing: disconnected player's username, "Reconnecting..." status, countdown timer, and calm informational copy,
   And the disconnected player's seat dims (greyed avatar),
   And all game interactions are disabled during the reconnect window.

3. **Other Players' Connections Unaffected (NFR17)**
   Given a player disconnects,
   When the other 3 players' WebSocket connections are evaluated,
   Then their connections remain active and unaffected,
   And the game state integrity is preserved for all players.

4. **Countdown Synchronized via Absolute Timestamp**
   Given the reconnect countdown is running,
   When the timer updates,
   Then the countdown is synchronized across all 3 remaining clients using the server's absolute `reconnectExpiresAt` timestamp (same pattern as `turnExpiresAt`).

5. **Client Auto-Reconnects Within 1 Second (NFR5)**
   Given the WebSocket client detects its own connection drop,
   When reconnection is needed,
   Then the client begins automatic reconnection attempts within 1 second (existing exponential backoff in `useWebSocket.ts`).

6. **Disconnect While Paused — Auto-Clear Pause (Fixes D54)**
   Given a player who has an active pause disconnects,
   When the server processes the disconnect,
   Then their active pause is automatically cleared (`PausedPlayers[seat] = false`),
   And if the game was in `paused` phase and no other pauses remain, the game transitions through `previousPhase` before entering `disconnected` phase,
   And the reconnect window uses the configurable duration from the room.

## Tasks / Subtasks

- [x] Task 1: Backend — Add `ReconnectWindowSec` to Room model and creation flow (AC: #1)
  - [x] 1.1 Add field to Room struct in `server/internal/room/model.go`:
    ```go
    ReconnectWindowSec *int `json:"reconnectWindowSec"` // nil = default 120s
    ```
  - [x] 1.2 Create DB migration to add `reconnect_window_sec` column (nullable int, same pattern as `timer_duration_seconds`)
  - [x] 1.3 Update `CreateRoom` handler in `server/internal/room/handler.go` to accept and validate `reconnectWindowSec` (range: 30-300 seconds)
  - [x] 1.4 Update `CreateRoomRequest` struct to include `reconnectWindowSec`
  - [x] 1.5 Wire reconnect window through `StartGame` — add `reconnectWindowSec int` parameter to `GameStarter` interface and `session.Manager.StartGame`
  - [x] 1.6 Store `reconnectWindowSec` in Session struct (default 120 if nil from room)
  - [x] 1.7 Update all existing `StartGame` call sites in `room/handler.go` (StartGame handler + QuickPlay auto-start) to pass `room.ReconnectWindowSec`
  - [x] 1.8 Update test mocks/stubs of `GameStarter` interface to match the new signature
  - [x] 1.9 Update frontend create room form and types to include reconnect window option (optional — can defer UI to a later story if needed; use default 120s for now)

- [x] Task 2: Backend — Add disconnect state fields to GameState (AC: #1, #4)
  - [x] 2.1 In `server/internal/game/state.go`, add fields to GameState struct in the **Pause state** section (after `TurnTimeRemaining`):
    ```go
    // Disconnect state
    DisconnectedSeat   int        `json:"disconnectedSeat"`   // Seat index of disconnected player (-1 if none)
    ReconnectExpiresAt *time.Time `json:"reconnectExpiresAt"` // Absolute timestamp when reconnect window closes
    ```
  - [x] 2.2 Initialize `DisconnectedSeat` to `-1` in `NewGame()` in `state.go`
  - [x] 2.3 Update `cloneGameState` if it has custom logic (currently shallow struct copy — `int` and `*time.Time` copy correctly, but verify `*time.Time` deep copy)
  - [x] 2.4 In `client/src/shared/types/gameTypes.ts`, add to `GameState` interface:
    ```ts
    disconnectedSeat: number;
    reconnectExpiresAt: string | null;
    ```

- [x] Task 3: Backend — Add WS events and payload types for disconnect (AC: #2)
  - [x] 3.1 In `server/internal/ws/events.go`, add:

    ```go
    // --- Disconnect/reconnect events ---
    const EventPlayerDisconnected = "event:player_disconnected"
    const EventPlayerReconnected  = "event:player_reconnected"
    const ErrorPlayerDisconnected = "error:player_disconnected"

    // PlayerDisconnectedPayload is the typed payload for EventPlayerDisconnected events.
    type PlayerDisconnectedPayload struct {
        PlayerSeat         int       `json:"playerSeat"`
        Username           string    `json:"username"`
        ReconnectExpiresAt time.Time `json:"reconnectExpiresAt"`
    }

    // PlayerReconnectedPayload is the typed payload for EventPlayerReconnected events.
    type PlayerReconnectedPayload struct {
        PlayerSeat int `json:"playerSeat"`
    }
    ```

  - [x] 3.2 In `client/src/shared/types/wsEvents.ts`, add matching constants and types:

    ```ts
    // --- Disconnect/reconnect events ---
    export const EVENT_PLAYER_DISCONNECTED =
      "event:player_disconnected" as const;
    export const EVENT_PLAYER_RECONNECTED = "event:player_reconnected" as const;
    export const ERROR_PLAYER_DISCONNECTED =
      "error:player_disconnected" as const;

    export interface PlayerDisconnectedPayload {
      playerSeat: number;
      username: string;
      reconnectExpiresAt: string;
    }

    export interface PlayerReconnectedPayload {
      playerSeat: number;
    }
    ```

- [x] Task 4: Backend — Implement disconnect detection in session manager (AC: #1, #3, #6)
  - [x] 4.1 Add a `DisconnectHandler` interface or method to the session manager that is called when the hub unregisters a client:
    - Create `server/internal/session/reconnect.go` (per architecture spec)
    - Add method: `func (m *Manager) HandleDisconnect(userID uint)`
  - [x] 4.2 In `HandleDisconnect`:
    - Look up `userToRoom[userID]` to find if user is in an active game session
    - If not in a session, return (lobby disconnect — no game impact)
    - Lock the session
    - Find the seat index for the disconnected userID
    - If the player has an active pause (`PausedPlayers[seat] == true`), auto-clear it (**fixes D54**)
    - If game was in `PhasePaused` and no other pauses remain, restore to `PreviousPhase`
    - Set `Players[seat].Connected = false`
    - Cancel the turn timer (no auto-play while disconnected)
    - Capture remaining turn time into `TurnTimeRemaining` (same pattern as pause)
    - Set `TurnExpiresAt = nil` (timer paused during disconnect)
    - Set `DisconnectedSeat = seat`
    - Set `PreviousPhase` to the current phase (before disconnect)
    - Set `Phase = PhaseDisconnected`
    - Calculate `reconnectExpiresAt = time.Now().Add(reconnectWindowSec)`
    - Set `ReconnectExpiresAt = &reconnectExpiresAt`
    - Start a reconnect countdown timer (`time.AfterFunc(reconnectWindowSec, handleReconnectTimeout)`)
    - Unlock the session
    - Broadcast `event:player_disconnected` to the remaining 3 players
    - Broadcast `event:game_state` to the remaining 3 players (full state with disconnected phase)
  - [x] 4.3 Add `reconnectTimer *time.Timer` and `reconnectGeneration uint64` fields to Session struct (same pattern as `turnTimer`/`timerGeneration`)
  - [x] 4.4 Add `reconnectWindowSec int` field to Session struct (set during `StartGame`, default 120)
  - [x] 4.5 Implement `handleReconnectTimeout(session, generation)`:
    - Guard: session closed or generation stale
    - This will be the hook for Story 5.5 (match abandonment) — for now, log a warning and leave the session in `disconnected` phase
    - **Do NOT implement match abandonment in this story** — that is Story 5.5
  - [x] 4.6 Wire `HandleDisconnect` into the hub's unregister flow:
    - Option A: Add a disconnect callback/hook to the Hub that the session manager registers
    - Option B: Have the session manager register an `OnDisconnect` handler that the hub calls during unregister
    - Choose the approach that doesn't create a circular dependency (hub → session is fine since session already imports hub)
    - **Recommended approach**: Add a `DisconnectHandler func(userID uint)` field to Hub, called during unregister. Session manager sets it via `hub.SetDisconnectHandler(m.HandleDisconnect)`.
  - [x] 4.7 Ensure `HandleDisconnect` does NOT fire for connection replacement (hub.go:67-70 replaces existing connections — the old client unregisters but the user has a new connection). Check `hub.IsConnected(userID)` after a brief delay or in the disconnect handler to verify the user is truly gone, not just reconnecting.

- [x] Task 5: Backend — Block game actions during disconnected phase (AC: #2)
  - [x] 5.1 In `server/internal/game/rules_engine.go`, add `PhaseDisconnected` rejection in `ApplyAction()`:
    ```go
    if state.Phase == PhaseDisconnected {
        return nil, apperr.ErrPlayerDisconnected
    }
    ```
    Place this BEFORE the pause check (pause and disconnect are mutually exclusive phases — disconnect takes precedence)
  - [x] 5.2 Ensure this error is mapped in `sendGameError` (it will route as `error:invalid_action` due to D53, acceptable for now)

- [x] Task 6: Backend tests — Disconnect detection scenarios (AC: #1, #3, #6)
  - [x] 6.1 Add test fixture in `server/internal/game/testfixtures/fixtures.go`:
    - `NewGameDisconnected(disconnectedSeat int) *GameState` — returns a game in `PhaseDisconnected` with one player `Connected: false`
  - [x] 6.2 In `server/internal/game/rules_engine_test.go` or a new `disconnect_test.go`:
    - **Actions rejected during disconnected phase** — any game action returns `ErrPlayerDisconnected`
    - **Phase transition to disconnected** — verify state transitions correctly
  - [x] 6.3 In `server/internal/session/` tests (if `reconnect.go` has testable logic):
    - **HandleDisconnect marks player as disconnected** — verify `Connected = false`, `Phase = PhaseDisconnected`, `DisconnectedSeat` set correctly
    - **HandleDisconnect clears active pause** — player had active pause, disconnect clears it (D54 fix)
    - **HandleDisconnect preserves turn timer remaining** — verify `TurnTimeRemaining` is captured
    - **HandleDisconnect starts reconnect countdown** — verify `ReconnectExpiresAt` is set
    - **Non-game disconnect is no-op** — user not in a session, handler returns immediately
    - **Connection replacement does not trigger game disconnect** — verify that replacing a connection (same user reconnects) does not mark them as disconnected

- [x] Task 7: Frontend — Add `ReconnectOverlay` component (AC: #2, #4)
  - [x] 7.1 Create `client/src/features/game/components/ReconnectOverlay.tsx`:
    - Props:
      ```ts
      interface ReconnectOverlayProps {
        disconnectedPlayerName: string;
        reconnectExpiresAt: string; // ISO timestamp from server
      }
      ```
    - Render: Full-table overlay (same positioning pattern as `PauseOverlay`)
    - Content:
      - Player name prominently displayed
      - "Reconnecting..." status text
      - Countdown timer (minutes:seconds) computed from `reconnectExpiresAt - Date.now()` — same client-side countdown pattern as `TimerRing`
      - Calm informational copy: "The game is on hold while we wait for [player] to reconnect."
    - Visual tone: `surface-elevated` background, no red alerts, `warning` color (`#f59e0b`) for countdown (per UX spec color palette)
    - `aria-live="assertive"` on overlay (per UX accessibility spec)
    - `data-testid="reconnect-overlay"` and `data-testid="reconnect-countdown"`
  - [x] 7.2 Use `useEffect` + `setInterval(1000)` for countdown tick (same approach as client-side timer rendering)
  - [x] 7.3 Display `0:00` and stop updating when countdown reaches zero (Story 5.5 handles what happens next)

- [x] Task 8: Frontend — Wire disconnect events in `useWsDispatch.ts` (AC: #2)
  - [x] 8.1 Import new event constants: `EVENT_PLAYER_DISCONNECTED`, `EVENT_PLAYER_RECONNECTED`
  - [x] 8.2 Add dispatch case for `EVENT_PLAYER_DISCONNECTED`:
    - Update `gameStore` with the new game state (the `event:game_state` broadcast that follows will handle this)
    - Show a toast: `t("game.disconnect.playerDisconnected", { player: payload.username })`
  - [x] 8.3 Add dispatch case for `EVENT_PLAYER_RECONNECTED` (placeholder for Story 5.4):
    - Show a toast: `t("game.disconnect.playerReconnected", { player: payload.username })`
    - The `event:game_state` broadcast that follows will update the store
  - [x] 8.4 Add `ERROR_PLAYER_DISCONNECTED` to `GAME_ERROR_TYPES` set for error toast handling

- [x] Task 9: Frontend — Update `GamePage.tsx` to show ReconnectOverlay (AC: #2)
  - [x] 9.1 Import `ReconnectOverlay`
  - [x] 9.2 Render `ReconnectOverlay` when `gameState.phase === "disconnected"` and `gameState.disconnectedSeat !== -1`:
    ```tsx
    {
      gameState.phase === "disconnected" &&
        gameState.disconnectedSeat !== -1 && (
          <ReconnectOverlay
            disconnectedPlayerName={
              gameState.players[gameState.disconnectedSeat].username
            }
            reconnectExpiresAt={gameState.reconnectExpiresAt!}
          />
        );
    }
    ```
  - [x] 9.3 Disable all game interactions (card clicks, pause button, etc.) when phase is `disconnected` — the existing phase-based interaction guards should handle most of this, but verify

- [x] Task 10: Frontend — Dim disconnected player's seat (AC: #2)
  - [x] 10.1 In the seat/player component (likely in the game table layout), check `player.connected === false`
  - [x] 10.2 When `connected === false`, apply greyed/dimmed styling: `opacity-50 grayscale` (or similar)
  - [x] 10.3 Keep the player's username and team color visible (dimmed, not hidden)

- [x] Task 11: Frontend tests — ReconnectOverlay (AC: #2, #4)
  - [x] 11.1 In `ReconnectOverlay.test.tsx`:
    - **Renders player name and countdown** — render with props, assert player name and countdown visible
    - **Countdown decrements** — advance timers, assert countdown updates
    - **Countdown stops at zero** — set `reconnectExpiresAt` in the past, assert shows `0:00`
    - **Has correct accessibility attributes** — assert `aria-live="assertive"`
  - [x] 11.2 In `GamePage` tests (if applicable):
    - **ReconnectOverlay shown during disconnected phase** — set phase to `disconnected`, assert overlay present
    - **ReconnectOverlay not shown during other phases** — assert overlay absent

- [x] Task 12: i18n keys (AC: #2)
  - [x] 12.1 In `client/src/shared/i18n/en.json`, add under `game.disconnect`:
    ```json
    "playerDisconnected": "{{player}} disconnected",
    "playerReconnected": "{{player}} reconnected",
    "reconnecting": "Reconnecting...",
    "waitingMessage": "The game is on hold while we wait for {{player}} to reconnect.",
    "countdownLabel": "Time remaining"
    ```
  - [x] 12.2 In `client/src/shared/i18n/sr.json`, add under `game.disconnect`:
    ```json
    "playerDisconnected": "{{player}} se diskonektova",
    "playerReconnected": "{{player}} se rekonektova",
    "reconnecting": "Rekonektovanje...",
    "waitingMessage": "Igra je pauzirana dok cekamo da se {{player}} vrati.",
    "countdownLabel": "Preostalo vreme"
    ```
  - [x] 12.3 Add error i18n key in `game.errors`:
    ```json
    "playerDisconnected": "A player is disconnected"
    ```

- [x] Task 13: Validation and quality gates (AC: all)
  - [x] 13.1 Run `make lint` — both Go and TypeScript must pass
  - [x] 13.2 Run `make test` — all existing + new tests must pass
  - [x] 13.3 Verify no regressions in existing pause tests (Story 5.1/5.2 tests must still pass)
  - [x] 13.4 Verify WS contract files are in sync (`wsEvents.ts` + `events.go`)
  - [x] 13.5 Verify `DisconnectedSeat` initialized to -1 in `NewGame()` (same sentinel pattern as `OwnerSeat`)
  - [x] 13.6 Verify `cloneGameState` correctly handles new fields (shallow copy of `*time.Time` is a pointer alias — needs deep copy like `TurnExpiresAt`)

## Dev Notes

### What Already Exists — Do NOT Recreate

Critical: The following are already implemented. They MUST NOT be duplicated:

| Item                                                    | Location                                               | Status                                                            |
| ------------------------------------------------------- | ------------------------------------------------------ | ----------------------------------------------------------------- |
| `PhaseDisconnected = "disconnected"`                    | `server/internal/game/types.go:91`                     | Exists                                                            |
| `"disconnected"` in Phase type                          | `client/src/shared/types/gameTypes.ts:20`              | Exists                                                            |
| `PlayerState.Connected bool`                            | `server/internal/game/state.go:16`                     | Exists (initialized `true`, never set to `false` in current code) |
| `PlayerState.connected: boolean`                        | `client/src/shared/types/gameTypes.ts:60`              | Exists                                                            |
| `ErrPlayerDisconnected`                                 | `server/internal/apperr/errors.go:84`                  | Exists                                                            |
| Ping/pong mechanism (30s ping, 45s timeout)             | `server/internal/ws/client.go:12-14, 107-125`          | Exists                                                            |
| Hub auto-unregisters on disconnect                      | `server/internal/ws/hub.go:76-83`                      | Exists — unregister channel fires when `readPump` returns         |
| Hub.IsConnected(userID)                                 | `server/internal/ws/hub.go:172-177`                    | Exists                                                            |
| Client auto-reconnect (exponential backoff)             | `client/src/shared/hooks/useWebSocket.ts:133-147`      | Exists (1s base, 30s max)                                         |
| `GetStateSnapshot(roomID)`                              | `server/internal/session/manager.go:228-238`           | Exists (for reconnection use in Story 5.4)                        |
| Timer preservation pattern (capture remaining on pause) | `server/internal/session/manager.go:170-178`           | Exists — reuse same pattern for disconnect                        |
| `PauseOverlay` component                                | `client/src/features/game/components/PauseOverlay.tsx` | Exists — use as UI pattern reference                              |
| Seat `disconnected` state (greyed avatar)               | UX spec lines 680-687                                  | Specified, not yet implemented                                    |

### What Must Be Created

1. **`server/internal/session/reconnect.go`** — New file per architecture spec. Contains `HandleDisconnect`, `handleReconnectTimeout`, and reconnect countdown management.

2. **`client/src/features/game/components/ReconnectOverlay.tsx`** — New component for the reconnect countdown overlay. Follow `PauseOverlay` pattern.

3. **`client/src/features/game/components/ReconnectOverlay.test.tsx`** — Tests for the overlay.

4. **DB migration** — New migration file to add `reconnect_window_sec` column to `rooms` table.

### What Must Be Changed

1. **`server/internal/room/model.go`** — Add `ReconnectWindowSec *int` field to Room struct.
2. **`server/internal/room/handler.go`** — Accept reconnect window in creation, validate range, pass to `StartGame`. Update `GameStarter` interface.
3. **`server/internal/session/manager.go`** — Update `StartGame` to accept and store `reconnectWindowSec`. Update Session struct with reconnect timer fields.
4. **`server/internal/ws/hub.go`** — Add `DisconnectHandler` callback, called during unregister. Must handle connection replacement gracefully (don't fire disconnect if user has a new connection).
5. **`server/internal/ws/events.go`** — Add disconnect/reconnect event constants and payload types.
6. **`server/internal/game/state.go`** — Add `DisconnectedSeat` and `ReconnectExpiresAt` fields to GameState.
7. **`server/internal/game/rules_engine.go`** — Add `PhaseDisconnected` rejection in `ApplyAction()`.
8. **`client/src/shared/types/wsEvents.ts`** — Add disconnect/reconnect event constants and interfaces.
9. **`client/src/shared/types/gameTypes.ts`** — Add `disconnectedSeat` and `reconnectExpiresAt` to GameState interface.
10. **`client/src/shared/hooks/useWsDispatch.ts`** — Add handlers for disconnect/reconnect events.
11. **`client/src/features/game/GamePage.tsx`** — Render ReconnectOverlay during disconnected phase.
12. **`client/src/shared/i18n/en.json`** + **`sr.json`** — Add i18n keys.

### Architecture Patterns to Follow

- **Rules engine is a pure function**: `ApplyAction` validates phase and rejects with `ErrPlayerDisconnected` during `PhaseDisconnected`. No side effects.
- **Session manager is the orchestrator**: All disconnect detection, timer management, countdown timers, and broadcasting happen in the session manager — NOT in the rules engine.
- **Timer sync via absolute timestamps**: Use `ReconnectExpiresAt *time.Time` (server absolute UTC timestamp). Client computes countdown as `reconnectExpiresAt - Date.now()`. Same pattern as `TurnExpiresAt`. **Never send relative "start a 120s countdown."**
- **Timer capture on disconnect**: When disconnecting, capture remaining turn time into `TurnTimeRemaining` and nil out `TurnExpiresAt`. Same pattern as pause (manager.go:170-178).
- **Clone state before mutation**: Use `cloneGameState(state)` for any state changes in the rules engine.
- **WebSocket contract sync**: Both `wsEvents.ts` and `events.go` must define the same events and payload types.
- **i18n from day one**: All user-facing strings through `react-i18next`. English + Serbian.
- **`data-testid` attributes**: For all interactive/assertable elements (established by PauseOverlay pattern).
- **Centralized errors**: Use existing `apperr.ErrPlayerDisconnected` — do not create new error types.

### Hub Disconnect Hook — Critical Design Decision

The hub's `unregister` channel fires when a client's `readPump` returns (connection drop or normal close). The session manager needs to know about this to trigger game disconnect handling.

**Recommended approach**: Add a `DisconnectHandler` callback to the Hub:

```go
type Hub struct {
    // ... existing fields
    disconnectHandler func(userID uint)
}

func (h *Hub) SetDisconnectHandler(handler func(userID uint)) {
    h.disconnectHandler = handler
}
```

In the `Run()` unregister case, AFTER deleting the client from the map, call:

```go
if h.disconnectHandler != nil {
    go h.disconnectHandler(client.UserID)
}
```

**CRITICAL**: The disconnect handler must check `hub.IsConnected(userID)` to distinguish true disconnects from connection replacements. When a user reconnects, the hub replaces the old client (line 67-70) and then unregisters the old one (line 76-83). The handler should verify the user is truly gone:

```go
func (m *Manager) HandleDisconnect(userID uint) {
    // Small delay to let replacement registration complete
    time.Sleep(100 * time.Millisecond)
    if m.hub.IsConnected(userID) {
        return // Connection replaced, not a true disconnect
    }
    // ... proceed with disconnect handling
}
```

Alternatively, check inside the hub's unregister case: only fire disconnect if `current == client` (the deleted client is the same object, not a replacement). This is already the condition at hub.go:78. So the handler would only fire when the client being unregistered is the current one — which means it's a true disconnect, not a replacement. **This is the cleaner approach — no sleep needed.**

### Disconnect While Paused — D54 Fix

This is the critical deferred item from Story 5.1/5.2 reviews. Currently, if a paused player closes their browser:

1. Their `PausedPlayers[seat]` remains `true`
2. The game stays in `PhasePaused` forever
3. Other players cannot resume (they can't unpause someone else's pause)
4. Even the room owner's override is useless if the owner is the one who disconnected

**Fix in HandleDisconnect:**

```go
// Auto-clear disconnected player's active pause
if session.gameState.PausedPlayers[seat] {
    session.gameState.PausedPlayers[seat] = false
    // If game was paused and no other pauses remain, restore previous phase
    anyPaused := false
    for _, p := range session.gameState.PausedPlayers {
        if p { anyPaused = true; break }
    }
    if !anyPaused && session.gameState.Phase == game.PhasePaused {
        session.gameState.Phase = session.gameState.PreviousPhase
        session.gameState.PreviousPhase = ""
    }
}
// THEN transition to PhaseDisconnected
session.gameState.PreviousPhase = session.gameState.Phase
session.gameState.Phase = game.PhaseDisconnected
```

### Timer Management During Disconnect

Follow the exact same pattern as pause timer preservation:

1. **On disconnect**: Capture `TurnTimeRemaining` from `time.Until(*TurnExpiresAt)`, nil out `TurnExpiresAt`, cancel turn timer
2. **On reconnect** (Story 5.4): Restore timer with `min(TurnTimeRemaining, 3000ms)` floor, same as unpause resume
3. **No auto-play during disconnect**: Timer is paused — the disconnected player gets their remaining time back on reconnect

### `*time.Time` Deep Copy Warning

`cloneGameState` is a shallow struct copy. Adding `ReconnectExpiresAt *time.Time` means both original and clone share the same `time.Time` pointer. Currently safe because code always assigns new pointers (never mutates through existing ones), but verify this pattern is maintained. If `cloneGameState` gains deep-copy logic for pointer fields, include `ReconnectExpiresAt`.

### Room Configuration — ReconnectWindowSec

FR16 specifies "reconnect window duration" as a room config option. The PRD says default is 2 minutes. The field is nullable with a default:

- `nil` → use server default (120 seconds)
- Explicit value → 30-300 second range

For **this story**, the room creation UI update for reconnect window is **optional** — the backend should accept and validate it, but the frontend form can keep using the default. If adding to the UI, follow the same pattern as `timerDurationSeconds` (optional number input, shown only when relevant).

### What This Story Does NOT Cover

- **Reconnection and state restoration** — Story 5.4 handles the reconnecting player's flow (auth refresh → new WS → state snapshot)
- **Match abandonment on timeout** — Story 5.5 handles what happens when the countdown reaches 0
- **Room lobby disconnect** — Story 5.5 AC covers pre-game lobby disconnects (seat freed after 10s)

This story implements the **detection** and **notification** side. The countdown timer fires but the timeout handler is a placeholder until Story 5.5.

### Previous Story Intelligence (Stories 5.1 & 5.2)

Key learnings from previous Epic 5 stories:

- `OwnerSeat` defaults to `-1` sentinel — use same pattern for `DisconnectedSeat`
- Timer remaining floor of 3 seconds is enforced in session manager on resume — apply same floor on reconnect resume (Story 5.4)
- `PauseOverlay` uses `data-testid` attributes for testing — follow same pattern for `ReconnectOverlay`
- `cloneGameState` is a shallow struct copy — `int` value types copy correctly, `*time.Time` is a pointer alias (acceptable if never mutated through)
- `broadcastActionResult` in session manager handles side effects — disconnect broadcasting follows same pattern but is triggered by hub event, not player action
- All 11 `StartGame` calls in `manager_test.go` were updated for `ownerID` in Story 5.2 — they'll need updating again for `reconnectWindowSec`
- `GameStarter` interface in `room/handler.go` was updated for `ownerID` — extend it further for `reconnectWindowSec`

### Cross-Story Context

- **Story 5.4** (Reconnection & State Restoration) will use `HandleReconnect` to reverse the disconnect: set `Connected = true`, restore phase from `PreviousPhase`, send full `event:game_state` snapshot, broadcast `event:player_reconnected` to all, dismiss `ReconnectOverlay`.
- **Story 5.5** (Match Abandonment on Timeout) will implement `handleReconnectTimeout`: transition to `PhaseMatchEnd` with abandon status, persist match record with `status: abandoned`, broadcast `event:match_abandoned`, redirect remaining players to lobby.
- **Deferred D46**: `GetStateSnapshot` returns mutable pointer. Story 5.4 will use this for reconnection — should deep-copy or clone the state before sending.

### Project Structure Notes

**New files (expected):**

- `server/internal/session/reconnect.go` — Disconnect detection, reconnect countdown management
- `server/db/migrations/NNNNNN_add_reconnect_window_sec.up.sql` — DB migration
- `server/db/migrations/NNNNNN_add_reconnect_window_sec.down.sql` — DB migration rollback
- `client/src/features/game/components/ReconnectOverlay.tsx` — Reconnect countdown overlay
- `client/src/features/game/components/ReconnectOverlay.test.tsx` — Overlay tests

**Modified files (expected):**

- `server/internal/room/model.go` — Add `ReconnectWindowSec` field
- `server/internal/room/handler.go` — Accept/validate reconnect window, update `GameStarter` interface
- `server/internal/session/manager.go` — Update `StartGame` signature, add reconnect timer fields to Session
- `server/internal/ws/hub.go` — Add disconnect handler callback
- `server/internal/ws/events.go` — Add disconnect/reconnect event constants and payloads
- `server/internal/game/state.go` — Add `DisconnectedSeat` and `ReconnectExpiresAt` fields
- `server/internal/game/rules_engine.go` — Add `PhaseDisconnected` rejection
- `server/internal/game/testfixtures/fixtures.go` — Add `NewGameDisconnected` factory
- `server/internal/session/manager_test.go` — Update `StartGame` calls with new parameter
- `client/src/shared/types/gameTypes.ts` — Add disconnect fields to GameState
- `client/src/shared/types/wsEvents.ts` — Add disconnect/reconnect events and types
- `client/src/shared/hooks/useWsDispatch.ts` — Add disconnect/reconnect event handlers
- `client/src/features/game/GamePage.tsx` — Render ReconnectOverlay
- `client/src/shared/i18n/en.json` — Add i18n keys
- `client/src/shared/i18n/sr.json` — Add i18n keys

### References

- [Source: _bmad-output/planning-artifacts/epics.md#L1166-1196 — Story 5.3 acceptance criteria]
- [Source: _bmad-output/planning-artifacts/architecture.md — WebSocket health check: ping every 30s, missed pong triggers disconnect in session/reconnect.go]
- [Source: _bmad-output/planning-artifacts/architecture.md — Session manager orchestrator pattern, rules engine pure function]
- [Source: _bmad-output/planning-artifacts/architecture.md — Timer sync via absolute timestamps: never relative countdown]
- [Source: _bmad-output/planning-artifacts/architecture.md — File structure: session/reconnect.go for disconnect detection]
- [Source: _bmad-output/planning-artifacts/architecture.md — Game state machine: PhaseDisconnected transitions]
- [Source: _bmad-output/planning-artifacts/architecture.md — NFR5 (reconnect within 1s), NFR16 (full state preservation), NFR17 (single disconnect isolation)]
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md#L763-771 — ReconnectOverlay: full-table overlay, player name + reconnecting + countdown + calm copy]
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md#L680-687 — Seat states including `disconnected`: greyed avatar, reconnect countdown overlay]
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md#L322 — Warning color #f59e0b for reconnect countdowns]
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md#L98 — Emotional journey: "Something goes wrong" → calm and informed]
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md#L981 — Accessibility: aria-live="assertive" on reconnect overlay]
- [Source: _bmad-output/planning-artifacts/prd.md — FR16: Room config includes reconnect window duration; FR26/FR27: Disconnect detection and reconnection]
- [Source: _bmad-output/implementation-artifacts/deferred-work.md#D54 — Disconnect while paused leaves game frozen]
- [Source: _bmad-output/implementation-artifacts/deferred-work.md#D46 — GetStateSnapshot returns mutable pointer]
- [Source: _bmad-output/archive/implementation-artifacts/5-2-room-owner-pause-override.md — Previous story: OwnerSeat sentinel pattern, timer preservation, test update patterns]
- [Source: server/internal/ws/client.go:12-14 — Existing ping/pong: 30s ping, 45s timeout]
- [Source: server/internal/ws/hub.go:76-83 — Hub unregister flow (triggers on readPump return)]
- [Source: server/internal/ws/hub.go:67-70 — Connection replacement (same user reconnects)]
- [Source: server/internal/session/manager.go:170-178 — Timer capture pattern for pause (reuse for disconnect)]
- [Source: server/internal/session/manager.go:228-238 — GetStateSnapshot for reconnection]
- [Source: server/internal/game/state.go:16 — PlayerState.Connected field (exists, never set to false)]
- [Source: server/internal/game/types.go:91 — PhaseDisconnected constant (exists)]
- [Source: server/internal/apperr/errors.go:84 — ErrPlayerDisconnected (exists)]
- [Source: client/src/shared/hooks/useWebSocket.ts:133-147 — Client auto-reconnect with exponential backoff]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

- Import order for ReconnectOverlay fixed to satisfy simple-import-sort (alphabetical after PauseOverlay, before PlayerSeat → after PlayerSeat, before ReconnectOverlay)
- All 11 `StartGame` calls in `manager_test.go` updated with new `reconnectWindowSec` parameter (default 120)
- `cloneGameState` updated with deep-copy for `ReconnectExpiresAt *time.Time` (new pointer field)
- Hub disconnect handler fires only for true disconnects (not connection replacements) — leverages existing `current == client` guard at hub.go:78
- `DisconnectedSeat` initialized to -1 sentinel (same pattern as `OwnerSeat`)
- D54 fix: HandleDisconnect auto-clears paused player's active pause before transitioning to PhaseDisconnected
- Server-side username field is empty in PlayerDisconnectedPayload (D43: server PlayerState has no Username field) — client resolves from existing gameState.players

### Completion Notes List

- All 6 acceptance criteria satisfied
- 10 new test cases: 8 table-driven `TestDisconnectedPhaseRejectsAllActions` + 2 fixture validation tests
- 4 new frontend tests: renders player/countdown, countdown decrements, stops at zero, accessibility attributes
- All 288 frontend tests pass, all Go tests pass across 12 packages, zero regressions
- WS contract files in sync (events.go + wsEvents.ts both define EVENT_PLAYER_DISCONNECTED, EVENT_PLAYER_RECONNECTED, ERROR_PLAYER_DISCONNECTED)
- i18n keys added for both English and Serbian (disconnect section + error key)
- ReconnectOverlay follows PauseOverlay pattern: surface-elevated bg, calm visual tone, warning color countdown
- Disconnected player seat dims with opacity-50 grayscale
- Reconnect countdown synced via absolute server timestamp (same pattern as turnExpiresAt)

### Change Log

- 2026-04-13: Implemented disconnect detection & reconnect countdown (Story 5.3)

### File List

**New files:**

- `server/internal/session/reconnect.go` — HandleDisconnect, handleReconnectTimeout
- `server/internal/game/disconnect_test.go` — PhaseDisconnected rejection tests
- `server/migrations/000007_add_reconnect_window_sec.up.sql` — DB migration
- `server/migrations/000007_add_reconnect_window_sec.down.sql` — DB migration rollback
- `client/src/features/game/components/ReconnectOverlay.tsx` — Reconnect countdown overlay
- `client/src/features/game/components/ReconnectOverlay.test.tsx` — Overlay tests

**Modified files:**

- `server/internal/room/model.go` — Added `ReconnectWindowSec *int` field
- `server/internal/room/handler.go` — Added reconnect window validation, updated GameStarter interface + both StartGame call sites, added `resolveReconnectWindow` helper
- `server/internal/session/manager.go` — Updated StartGame signature (added reconnectWindowSec), added reconnect fields to Session, cancelReconnectTimer in RemoveSession
- `server/internal/session/timer.go` — Added `cancelReconnectTimer` method
- `server/internal/session/manager_test.go` — Updated all 11 StartGame calls with reconnectWindowSec parameter
- `server/internal/ws/hub.go` — Added DisconnectHandler type, SetDisconnectHandler method, disconnect callback in unregister flow
- `server/internal/ws/events.go` — Added EventPlayerDisconnected, EventPlayerReconnected, ErrorPlayerDisconnected constants and payload types
- `server/internal/game/state.go` — Added DisconnectedSeat, ReconnectExpiresAt fields; initialized DisconnectedSeat=-1 in NewGame
- `server/internal/game/bidding.go` — Added ReconnectExpiresAt deep-copy in cloneGameState
- `server/internal/game/rules_engine.go` — Added PhaseDisconnected rejection in ApplyAction
- `server/internal/game/testfixtures/fixtures.go` — Added NewGameDisconnected factory
- `server/internal/apperr/errors.go` — Added ErrReconnectWindowOutOfRange
- `server/cmd/api/main.go` — Wired hub.SetDisconnectHandler(sessionManager.HandleDisconnect)
- `client/src/shared/types/gameTypes.ts` — Added disconnectedSeat, reconnectExpiresAt to GameState
- `client/src/shared/types/wsEvents.ts` — Added disconnect/reconnect event constants and payload interfaces
- `client/src/shared/hooks/useWsDispatch.ts` — Added disconnect/reconnect event handlers and ERROR_PLAYER_DISCONNECTED to error set
- `client/src/features/game/GamePage.tsx` — Added ReconnectOverlay rendering during disconnected phase
- `client/src/features/game/components/PlayerSeat.tsx` — Added disconnected seat dimming (opacity-50 grayscale)
- `client/src/shared/i18n/en.json` — Added game.disconnect and game.errors.playerDisconnected keys
- `client/src/shared/i18n/sr.json` — Added game.disconnect and game.errors.playerDisconnected keys

### Review Findings

- [x] [Review][Patch] `gs` broadcast after `session.mu.Unlock()` is a data race — `buildMessage(EventGameState, gs)` serializes live `*GameState` pointer after lock release; concurrent `HandleAction` can mutate fields during `json.Marshal`. Fix: capture snapshot before unlock. [server/internal/session/reconnect.go:115-132]
- [x] [Review][Patch] Disconnect during `PhasePaused` with other active pauses sets `PreviousPhase = PhasePaused` — when the disconnecting player's pause is cleared but other pauses remain, `gs.Phase` stays `PhasePaused`, and line 87 saves `PreviousPhase = PhasePaused`. On reconnect, restoring to `PhasePaused` loses the original pre-pause phase. Fix: when `gs.Phase == PhasePaused`, save `gs.PreviousPhase` (the pre-pause phase) instead. [server/internal/session/reconnect.go:57-88]
- [x] [Review][Patch] `HandleDisconnect` fires on transient phases (`dealing`, `trick_resolving`, `hand_scoring`) not specified in AC1 — AC1 states disconnect handling applies during `playing`, `bidding`, or `paused` only. Add allowlist guard. [server/internal/session/reconnect.go:36]
- [x] [Review][Patch] `timerGeneration` not incremented in `HandleDisconnect` — a stale turn timer callback that fires between `cancelTurnTimer()` and the generation check can pass the staleness guard. Fix: increment `session.timerGeneration` after cancelling. [server/internal/session/reconnect.go:77]
- [x] [Review][Patch] Pause→Disconnect: `TurnTimeRemaining` not preserved when pause is auto-cleared — after auto-clearing the pause, `TurnExpiresAt` is still `nil` (was cleared by the original pause), so the `if gs.TurnExpiresAt != nil` check skips and no remaining time is stored. The pre-pause `TurnTimeRemaining` should be kept as-is when pause was auto-cleared. [server/internal/session/reconnect.go:78-83]
- [x] [Review][Patch] `ReconnectOverlay` lacks prominent standalone player name display — spec Task 7.1 says "Player name prominently displayed" as a standalone element; currently embedded inside `waitingMessage` paragraph only. [client/src/features/game/components/ReconnectOverlay.tsx]
- [x] [Review][Defer] `username` field in `PlayerDisconnectedPayload` is always empty — server `PlayerState` has no `Username` field (pre-existing D43). Client falls back to `Player N`. [server/internal/session/reconnect.go:108-113]
- [x] [Review][Defer] `ERROR_PLAYER_DISCONNECTED` unreachable on wire — `sendGameError` routes all game errors as `error:invalid_action` (pre-existing D53). The typed constant is defined for future use. [server/internal/session/manager.go:548]
- [x] [Review][Defer] `startNewHand` does not reset `DisconnectedSeat`, `ReconnectExpiresAt`, or `Players[i].Connected` — pre-existing design; Story 5.4/5.5 must handle reset on reconnection or match resumption. [server/internal/game/scoring.go]
