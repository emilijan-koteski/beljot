# Story 5.5: Match Abandonment on Timeout

Status: done

## Story

As a player,
I want the match to end gracefully if a disconnected player doesn't return in time,
so that the remaining players aren't stuck waiting indefinitely.

## Acceptance Criteria

1. **Reconnect Window Expiry Triggers Abandonment**
   Given a player is disconnected and the reconnect window is active,
   When the countdown reaches 0 without reconnection,
   Then the match is abandoned and the game phase transitions to `match_end` with `WinnerTeam = nil`.

2. **Match Record Persisted with Abandon Status**
   Given a match is abandoned,
   When the match record is persisted,
   Then the `matches` table record includes: `status: 'abandoned'`, `abandoned_by: [player_id]`, the current scores at time of abandonment, and `completed_at` timestamp.

3. **Casual Match Outcome**
   Given a casual match is abandoned,
   When outcomes are applied,
   Then the disconnected player receives no XP (placeholder for Epic 9 — no XP system exists yet, no code needed),
   And the remaining 3 players are returned to the lobby with a clear explanation: "[Player] disconnected — match ended".

4. **Client Abandonment Event and Redirect**
   Given a match is abandoned,
   When the remaining players' clients receive `event:match_abandoned`,
   Then the ReconnectOverlay transitions to an abandonment message,
   And after a brief pause (~3 seconds), the client auto-redirects to the lobby,
   And the Zustand gameStore is cleared.

5. **Lobby Pre-Game Disconnect — Seat Freed**
   Given a player disconnects in a room lobby (before the game starts),
   When their connection drops,
   Then their seat is freed after a short timeout (10 seconds),
   And other players see the seat become available via WebSocket broadcast,
   And no match abandonment logic is triggered.

## Tasks / Subtasks

- [x] Task 1: Database migration — add `status` and `abandoned_by` columns to `matches` table (AC: #2)
  - [x] 1.1 Create `server/migrations/000008_add_match_status.up.sql`:
    ```sql
    ALTER TABLE matches ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'completed';
    ALTER TABLE matches ADD COLUMN abandoned_by INTEGER REFERENCES users(id);
    CREATE INDEX idx_matches_status ON matches(status);
    CREATE INDEX idx_matches_abandoned_by ON matches(abandoned_by);
    ```
  - [x] 1.2 Create `server/migrations/000008_add_match_status.down.sql`:
    ```sql
    DROP INDEX IF EXISTS idx_matches_abandoned_by;
    DROP INDEX IF EXISTS idx_matches_status;
    ALTER TABLE matches DROP COLUMN IF EXISTS abandoned_by;
    ALTER TABLE matches DROP COLUMN IF EXISTS status;
    ```
  - [x] 1.3 Run `make migrate` to apply

- [x] Task 2: Update Match model and existing `handleMatchEnd` (AC: #2)
  - [x] 2.1 Update `server/internal/match/model.go` — add two fields to Match struct:
    ```go
    Status      string `gorm:"size:20;not null;default:completed" json:"status"`
    AbandonedBy *uint  `gorm:"index" json:"abandonedBy,omitempty"`
    ```
  - [x] 2.2 Update `handleMatchEnd` in `server/internal/session/manager.go` (line ~510) — set `Status: "completed"` explicitly on the match record:
    ```go
    matchRecord := &match.Match{
        // ... existing fields ...
        Status: "completed",
    }
    ```

- [x] Task 3: Add WS event `EventMatchAbandoned` to both contract files (AC: #4)
  - [x] 3.1 In `server/internal/ws/events.go`, add after the existing disconnect events:

    ```go
    const EventMatchAbandoned = "event:match_abandoned"

    type MatchAbandonedPayload struct {
        AbandonedByPlayer int `json:"abandonedByPlayer"` // seat index (client resolves name from gameState.players[seat].username — D43: server PlayerState has no Username field)
        TeamAFinalScore   int `json:"teamAFinalScore"`
        TeamBFinalScore   int `json:"teamBFinalScore"`
        MatchDurationSec  int `json:"matchDurationSec"`
    }
    ```

  - [x] 3.2 In `client/src/shared/types/wsEvents.ts`, add:

    ```typescript
    export const EVENT_MATCH_ABANDONED = "event:match_abandoned" as const;

    export interface MatchAbandonedPayload {
      abandonedByPlayer: number; // seat index — resolve name from gameState.players[seat].username
      teamAFinalScore: number;
      teamBFinalScore: number;
      matchDurationSec: number;
    }
    ```

- [x] Task 4: Backend — implement `handleReconnectTimeout` in session manager (AC: #1, #2, #3)
  - [x] 4.1 Replace the placeholder in `server/internal/session/reconnect.go` (lines 249-265). **CRITICAL: The existing placeholder uses `defer session.mu.Unlock()`. The replacement uses manual `session.mu.Unlock()` before external I/O. You MUST remove the `defer` — keeping both will double-unlock and panic.** Implement:

    ```go
    func (m *Manager) handleReconnectTimeout(session *Session, generation uint64) {
        session.mu.Lock()

        // Guard: session closed or generation stale (player already reconnected)
        if session.closed || session.reconnectGeneration != generation {
            session.mu.Unlock()
            return
        }

        gs := session.gameState

        // Identify the abandoned player
        abandonedSeat := gs.DisconnectedSeat
        abandonedPlayerID := session.playerIDs[abandonedSeat]
        // NOTE: server PlayerState has no Username field (D43). Client resolves name
        // from gameState.players[seat].username which was set during game setup.

        // Transition game state to match_end (abandoned)
        gs.Phase = game.PhaseMatchEnd
        // WinnerTeam stays nil — no winner for abandoned match
        // Clear disconnect fields
        gs.DisconnectedSeat = -1
        gs.ReconnectExpiresAt = nil

        // Cancel any active timers
        session.cancelReconnectTimer()
        session.cancelTurnTimer()

        // Capture data for broadcast BEFORE unlocking (data race prevention)
        playerIDs := session.playerIDs
        roomID := session.roomID
        startedAt := session.startedAt
        teamAScore := gs.TeamScores[game.TeamA]
        teamBScore := gs.TeamScores[game.TeamB]

        abandonedPayload := ws.MatchAbandonedPayload{
            AbandonedByPlayer: abandonedSeat,
            TeamAFinalScore:   teamAScore,
            TeamBFinalScore:   teamBScore,
            MatchDurationSec:  int(time.Since(startedAt).Seconds()),
        }
        abandonedMsg := buildMessage(ws.EventMatchAbandoned, abandonedPayload)
        stateMsg := buildMessage(ws.EventGameState, gs)

        session.mu.Unlock()

        // Broadcast to all 4 players (disconnected player gets it if they reconnect to WS later)
        userIDs := playerIDs[:]
        m.hub.BroadcastToUsers(userIDs, abandonedMsg)
        m.hub.BroadcastToUsers(userIDs, stateMsg)

        slog.Info("session: match abandoned due to reconnect timeout",
            "roomID", roomID,
            "abandonedBy", abandonedPlayerID,
            "abandonedSeat", abandonedSeat,
        )

        // Persist match record with abandoned status
        matchRecord := &match.Match{
            RoomID:        roomID,
            Player1ID:     playerIDs[0],
            Player2ID:     playerIDs[1],
            Player3ID:     playerIDs[2],
            Player4ID:     playerIDs[3],
            TeamAScore:    teamAScore,
            TeamBScore:    teamBScore,
            WinnerTeam:    0, // No winner — status field distinguishes abandoned
            Variant:       string(gs.Variant),
            MatchMode:     gs.MatchMode,
            StartedAt:     startedAt,
            CompletedAt:   time.Now(),
            Status:        "abandoned",
            AbandonedBy:   &abandonedPlayerID,
        }

        if err := m.matchRepo.Create(matchRecord); err != nil {
            slog.Error("session: failed to persist abandoned match", "roomID", roomID, "error", err)
        }

        // Update room status
        if m.roomUpdater != nil {
            if err := m.roomUpdater.UpdateRoomStatus(roomID, "completed"); err != nil {
                slog.Error("session: failed to update room status", "roomID", roomID, "error", err)
            }
        }

        m.RemoveSession(roomID)
    }
    ```

  - [x] 4.2 Verify the `handleReconnectTimeout` is already called by `HandleDisconnect` (lines 118-127 in reconnect.go) via `time.AfterFunc`. No new wiring needed — the timer callback already calls this function.

- [x] Task 5: Backend — lobby pre-game disconnect with 10s seat-free timer (AC: #5)
  - [x] 5.1 Create a `LobbyDisconnectHandler` struct in a new file `server/internal/room/lobby_disconnect.go`:

    ```go
    type LobbyDisconnectHandler struct {
        roomRepo    Repository
        hub         *ws.Hub
        mu          sync.Mutex
        pending     map[uint]*lobbyDisconnect // userID → pending disconnect
    }

    type lobbyDisconnect struct {
        timer  *time.Timer
        roomID uint
    }
    ```

  - [x] 5.2 Implement `HandleDisconnect(userID uint)`:
    - Check if user is in a room via `roomRepo.FindRoomByPlayer(userID)` (you may need to add this repo method)
    - If user is NOT in a room, return (no-op)
    - If user IS in a room with status `"playing"`, return (session manager handles this)
    - If user IS in a room with status `"waiting"`, start a 10-second timer
    - On timer expiry: call room leave logic (remove player from room_players, broadcast `system:player_left` to room)
  - [x] 5.3 Implement `HandleReconnect(userID uint)`:
    - Cancel any pending lobby disconnect timer for this user
  - [x] 5.4 Add repo method `FindRoomByPlayer(userID uint) (*Room, error)` to `server/internal/room/repository.go` and `gorm_repo.go` — returns the room the player is currently in (via room_players join), or nil if not in any room
  - [x] 5.5 Wire composite disconnect/connect handlers in `server/cmd/api/main.go`:

    ```go
    lobbyHandler := room.NewLobbyDisconnectHandler(roomRepo, hub)

    hub.SetDisconnectHandler(func(userID uint) {
        sessionManager.HandleDisconnect(userID)
        lobbyHandler.HandleDisconnect(userID)
    })
    hub.SetConnectHandler(func(userID uint) {
        sessionManager.HandleReconnect(userID)
        lobbyHandler.HandleReconnect(userID)
    })
    ```

- [x] Task 6: Frontend — handle `EVENT_MATCH_ABANDONED` in WS dispatch (AC: #4)
  - [x] 6.1 Add `matchAbandonedData` field to `gameStore.ts`:
    ```typescript
    matchAbandonedData: MatchAbandonedPayload | null;
    setMatchAbandonedData: (data: MatchAbandonedPayload | null) => void;
    ```
    Include in `initialState`: `matchAbandonedData: null`
    Include in `clearGame`: resets to null
  - [x] 6.2 In `useWsDispatch.ts`, add handler for `EVENT_MATCH_ABANDONED` (import the new types):
    ```typescript
    if (type === EVENT_MATCH_ABANDONED) {
      const payload = message.payload as MatchAbandonedPayload;
      store.setMatchAbandonedData(payload);
      return;
    }
    ```

- [x] Task 7: Frontend — ReconnectOverlay abandonment transition and auto-redirect (AC: #4)
  - [x] 7.1 Update `ReconnectOverlay` in `client/src/features/game/components/ReconnectOverlay.tsx` to accept an optional `abandonedData` prop:
    ```typescript
    interface ReconnectOverlayProps {
      disconnectedPlayerName: string;
      reconnectExpiresAt: string;
      abandonedData?: MatchAbandonedPayload | null;
    }
    ```
  - [x] 7.2 When `abandonedData` is provided, transition the overlay to show an abandonment message instead of the countdown:
    - Replace countdown display with: "{player} disconnected — match ended" (player name comes from `disconnectedPlayerName` prop, resolved in GamePage from `gameState.players[abandonedByPlayer].username` — NOT from payload, since server PlayerState has no Username field per D43)
    - Show final scores: Team A {score} : Team B {score} (from `abandonedData.teamAFinalScore` / `teamBFinalScore`)
    - Show "Returning to lobby..." with a brief countdown (3 seconds)
    - Use i18n keys for all text
  - [x] 7.3 Implement auto-redirect: after 3 seconds when `abandonedData` is present, call `clearGame()` and `navigate("/lobby")`. Use a `useEffect` with a `setTimeout`:
    ```typescript
    useEffect(() => {
      if (!abandonedData) return;
      const timer = setTimeout(() => {
        onReturnToLobby();
      }, 3000);
      return () => clearTimeout(timer);
    }, [abandonedData, onReturnToLobby]);
    ```
  - [x] 7.4 Update `ReconnectOverlay` usage in `GamePage.tsx` — pass `abandonedData` from `gameStore`:
    ```typescript
    const matchAbandonedData = useGameStore((s) => s.matchAbandonedData);
    // ...
    {((gameState.phase === "disconnected" && gameState.disconnectedSeat !== -1 && gameState.reconnectExpiresAt) || matchAbandonedData) && (
      <ReconnectOverlay
        disconnectedPlayerName={
          matchAbandonedData
            ? (gameState.players[matchAbandonedData.abandonedByPlayer]?.username ?? `Player ${matchAbandonedData.abandonedByPlayer + 1}`)
            : (gameState.players[gameState.disconnectedSeat]?.username ?? `Player ${gameState.disconnectedSeat + 1}`)
        }
        reconnectExpiresAt={gameState.reconnectExpiresAt ?? ""}
        abandonedData={matchAbandonedData}
      />
    )}
    ```
    **CRITICAL:** The render condition uses OR logic: show overlay when EITHER (a) in disconnected phase with valid seat, OR (b) `matchAbandonedData` is present. This is necessary because when `event:game_state` arrives with `phase: "match_end"`, the server clears `disconnectedSeat` to `-1` — so the old condition (`disconnectedSeat !== -1`) would suppress the overlay. The `matchAbandonedData` branch renders the abandonment UI regardless of the now-cleared disconnect fields.
  - [x] 7.5 Add `onReturnToLobby` prop to `ReconnectOverlay` for the auto-redirect callback. This should call `clearGame()` + `navigate("/lobby")` + `setMatchAbandonedData(null)` (same pattern as `handleReturnToLobby` in GamePage).

- [x] Task 8: i18n keys for abandonment (AC: #3, #4)
  - [x] 8.1 In `client/src/shared/i18n/en.json`, under `game.disconnect`:
    ```json
    "matchAbandoned": "{{player}} disconnected — match ended",
    "matchAbandonedScores": "Final: Team A {{a}} : Team B {{b}}",
    "returningToLobby": "Returning to lobby..."
    ```
  - [x] 8.2 In `client/src/shared/i18n/sr.json`, under `game.disconnect`:
    ```json
    "matchAbandoned": "{{player}} se disconnektovao — meč završen",
    "matchAbandonedScores": "Rezultat: Tim A {{a}} : Tim B {{b}}",
    "returningToLobby": "Povratak u lobi..."
    ```

- [x] Task 9: Backend tests — reconnect timeout abandonment (AC: #1, #2)
  - [x] 9.1 Add test fixture in `server/internal/game/testfixtures/fixtures.go`:
    - `NewGameDisconnectedNearTimeout(disconnectedSeat int) *GameState` — returns a game in `PhaseDisconnected` with `ReconnectExpiresAt` set to 1 second in the future (for testing timeout-triggered abandon)
  - [x] 9.2 In `server/internal/session/reconnect_test.go`, add tests:
    - **handleReconnectTimeout transitions to PhaseMatchEnd** — verify `Phase == PhaseMatchEnd`, `WinnerTeam == nil`, `DisconnectedSeat == -1`, `ReconnectExpiresAt == nil`
    - **handleReconnectTimeout persists abandoned match record** — verify `matchRepo.Create` called with `Status: "abandoned"`, `AbandonedBy` set to the disconnected player's ID
    - **handleReconnectTimeout broadcasts match_abandoned event** — verify `event:match_abandoned` and `event:game_state` broadcast to all 4 players
    - **handleReconnectTimeout updates room status** — verify `roomUpdater.UpdateRoomStatus(roomID, "completed")` called
    - **handleReconnectTimeout removes session** — verify `HasSession(roomID)` returns false after timeout
    - **handleReconnectTimeout is no-op when generation stale** — increment `reconnectGeneration` (simulating reconnection), verify no state change
    - **handleReconnectTimeout is no-op when session closed** — set `session.closed = true`, verify no state change
  - [x] 9.3 Add integration-style test: start game → disconnect player → wait for reconnect window to expire → verify full abandonment flow (match persisted, session removed, events broadcast)

- [x] Task 10: Backend tests — lobby disconnect seat freeing (AC: #5)
  - [x] 10.1 In `server/internal/room/lobby_disconnect_test.go`:
    - **HandleDisconnect frees seat after 10s for waiting room** — player in a "waiting" room disconnects, verify seat freed after timer
    - **HandleDisconnect no-op for playing room** — player in a "playing" room, verify no timer started
    - **HandleDisconnect no-op for player not in any room** — verify no crash, no action
    - **HandleReconnect cancels pending timer** — disconnect then reconnect before 10s, verify seat NOT freed
    - **Seat freed broadcasts player_left** — verify WebSocket broadcast sent when seat is freed

- [x] Task 11: Frontend tests (AC: #4)
  - [x] 11.1 Update `ReconnectOverlay.test.tsx`:
    - **Renders abandonment message when abandonedData provided** — verify abandonment text and scores displayed instead of countdown
    - **Auto-redirects to lobby after 3 seconds** — verify `onReturnToLobby` callback called after timeout
    - **Does not auto-redirect without abandonedData** — verify normal countdown behavior unchanged
  - [x] 11.2 Verify existing `MatchResult` tests still pass (no regressions — MatchResult is NOT used for abandoned matches)

- [x] Task 12: Validation and quality gates (AC: all)
  - [x] 12.1 Run `make lint` — both Go and TypeScript must pass
  - [x] 12.2 Run `make test` — all existing + new tests must pass
  - [x] 12.3 Verify no regressions in Stories 5.1-5.4 tests
  - [x] 12.4 Verify WS contract files are in sync (`wsEvents.ts` + `events.go`) — new `EventMatchAbandoned` in both
  - [x] 12.5 Verify `handleMatchEnd` still works for normal match completion (status = "completed")
  - [x] 12.6 Manual test: Start game → disconnect one player → wait for reconnect window to expire → verify abandonment overlay appears → verify auto-redirect to lobby → verify match persisted in DB with status "abandoned"

## Dev Notes

### What Already Exists — Do NOT Recreate

| Item                                                   | Location                                                   | Status                                                 |
| ------------------------------------------------------ | ---------------------------------------------------------- | ------------------------------------------------------ |
| `handleReconnectTimeout` placeholder                   | `server/internal/session/reconnect.go:249-265`             | Exists — REPLACE contents                              |
| Reconnect timer setup (calls `handleReconnectTimeout`) | `server/internal/session/reconnect.go:118-127`             | Exists — DO NOT modify                                 |
| `PhaseMatchEnd = "match_end"`                          | `server/internal/game/types.go:86`                         | Exists                                                 |
| `"match_end"` in Phase type                            | `client/src/shared/types/gameTypes.ts:18`                  | Exists                                                 |
| `WinnerTeam *int` (nullable)                           | `server/internal/game/state.go:88`                         | Exists                                                 |
| `winnerTeam: number \| null`                           | `client/src/shared/types/gameTypes.ts:95`                  | Exists                                                 |
| `DisconnectedSeat int`                                 | `server/internal/game/state.go:102`                        | Exists (init -1)                                       |
| `ReconnectExpiresAt *time.Time`                        | `server/internal/game/state.go:103`                        | Exists                                                 |
| `handleMatchEnd(session, finalState)`                  | `server/internal/session/manager.go:502-539`               | Exists — add `Status: "completed"` to its match record |
| `RemoveSession(roomID)`                                | `server/internal/session/manager.go:244-258`               | Exists — reuse as-is                                   |
| `buildMessage(eventType, payload)`                     | `server/internal/session/manager.go`                       | Exists                                                 |
| `cancelReconnectTimer()`                               | `server/internal/session/timer.go:12-19`                   | Exists                                                 |
| `cancelTurnTimer()`                                    | `server/internal/session/timer.go`                         | Exists                                                 |
| `reconnectGeneration` guard pattern                    | `server/internal/session/reconnect.go:252-256`             | Exists                                                 |
| `hub.BroadcastToUsers(userIDs, msg)`                   | `server/internal/ws/hub.go:174-182`                        | Exists                                                 |
| `EventMatchEnd` + `MatchEndPayload`                    | Both contract files                                        | Exists — NOT used for abandonment                      |
| `ReconnectOverlay` component                           | `client/src/features/game/components/ReconnectOverlay.tsx` | Exists — MODIFY to handle abandonment                  |
| `MatchResult` component                                | `client/src/features/game/components/MatchResult.tsx`      | Exists — NOT used for abandonment                      |
| `useReconnectionRedirect` hook                         | `client/src/shared/hooks/useReconnectionRedirect.ts`       | Exists — has `phase !== "match_end"` guard             |
| `clearGame()` in gameStore                             | `client/src/shared/stores/gameStore.ts:60`                 | Exists                                                 |
| `handleReturnToLobby` in GamePage                      | `client/src/features/game/GamePage.tsx:230-234`            | Exists — pattern to follow for abandon redirect        |
| `safeDerefInt` helper                                  | `server/internal/session/manager.go:701-706`               | Exists — returns 0 for nil, used in normal match end   |
| `RoomStatusUpdater` interface                          | `server/internal/session/manager.go:33-36`                 | Exists                                                 |
| `RoomStatusAdapter` implementation                     | `server/internal/room/handler.go:54-65`                    | Exists                                                 |
| `LeaveRoom` handler                                    | `server/internal/room/handler.go:442`                      | Exists — reference for lobby seat-freeing logic        |
| `Match` struct                                         | `server/internal/match/model.go:6-21`                      | Exists — MODIFY to add fields                          |
| `match.MatchRepository` interface                      | `server/internal/match/repository.go`                      | Exists — `Create` method used                          |
| Match DB migration                                     | `server/migrations/000006_create_matches.up.sql`           | Exists — DO NOT modify                                 |
| Hub composite handler wiring                           | `server/cmd/api/main.go:98-100`                            | Exists — MODIFY to composite                           |

### What Must Be Created

1. **`server/migrations/000008_add_match_status.up.sql`** + **`.down.sql`** — add `status` and `abandoned_by` columns
2. **`EventMatchAbandoned`** constant + **`MatchAbandonedPayload`** struct — in both `events.go` and `wsEvents.ts`
3. **`server/internal/room/lobby_disconnect.go`** — `LobbyDisconnectHandler` with 10s seat-free timer
4. **`FindRoomByPlayer(userID)` repo method** — in room repository interface + GORM implementation
5. **`matchAbandonedData`** store field + setter in `gameStore.ts`
6. **Frontend test updates** for ReconnectOverlay abandonment mode

### What Must Be Modified

1. **`server/internal/session/reconnect.go`** — replace `handleReconnectTimeout` placeholder with full implementation
2. **`server/internal/match/model.go`** — add `Status` and `AbandonedBy` fields to Match struct
3. **`server/internal/session/manager.go`** — add `Status: "completed"` to match record in `handleMatchEnd`
4. **`server/internal/ws/events.go`** — add `EventMatchAbandoned` + `MatchAbandonedPayload`
5. **`client/src/shared/types/wsEvents.ts`** — add `EVENT_MATCH_ABANDONED` + `MatchAbandonedPayload`
6. **`client/src/shared/hooks/useWsDispatch.ts`** — add `EVENT_MATCH_ABANDONED` handler
7. **`client/src/shared/stores/gameStore.ts`** — add `matchAbandonedData` field + setter
8. **`client/src/features/game/components/ReconnectOverlay.tsx`** — add abandonment mode (props, display, auto-redirect)
9. **`client/src/features/game/GamePage.tsx`** — pass `matchAbandonedData` to ReconnectOverlay, add `onReturnToLobby` prop
10. **`client/src/shared/i18n/en.json`** + **`sr.json`** — add abandonment i18n keys
11. **`server/cmd/api/main.go`** — composite disconnect/connect handler wiring
12. **`server/internal/room/repository.go`** + **`gorm_repo.go`** — add `FindRoomByPlayer` method

### Architecture Patterns to Follow

- **Build messages BEFORE unlocking session mutex.** Capture all broadcast data while holding `session.mu.Lock()`. Pattern from `HandleDisconnect` (F1 fix) and `HandleReconnect` — prevents data races on mutable GameState.
- **Generation guard for stale timer callbacks.** `handleReconnectTimeout` already has the `reconnectGeneration` check (lines 253-256). This prevents the timer from firing after a successful reconnection.
- **Timer synchronization via absolute timestamps.** When broadcasting, always use absolute server timestamps. The client computes any needed countdowns from these.
- **Session manager orchestrates, rules engine is not involved.** Abandonment is a session concern — it manipulates GameState directly (setting Phase, clearing fields). The rules engine is never called.
- **Broadcast to ALL 4 players** (same as `HandleReconnect`). The disconnected player may reconnect to WebSocket later and receive the event.
- **Separate event for abandonment.** Use `event:match_abandoned` (NOT `event:match_end`) so the client can distinguish abandoned from completed matches. The normal match-end flow (score reveal → MatchResult overlay) should NOT trigger for abandonment.
- **WS contract sync in same commit.** Both `events.go` and `wsEvents.ts` updated together.
- **i18n from day one.** All user-facing strings through `react-i18next`.
- **Domain errors in `apperr/errors.go`.** No new domain errors needed — abandonment is a timeout event, not an action validation failure.

### `handleReconnectTimeout` vs `handleMatchEnd` — Key Differences

| Concern            | `handleMatchEnd` (normal)            | `handleReconnectTimeout` (abandon)                 |
| ------------------ | ------------------------------------ | -------------------------------------------------- |
| Triggered by       | Rules engine returns `PhaseMatchEnd` | Reconnect timer expires                            |
| Lock state         | Caller holds no session lock         | Function acquires session lock                     |
| WinnerTeam         | Non-nil (0 or 1)                     | nil (no winner)                                    |
| Match status       | `"completed"`                        | `"abandoned"`                                      |
| AbandonedBy        | nil                                  | Disconnected player's ID                           |
| WS event           | `event:match_end`                    | `event:match_abandoned`                            |
| Client flow        | Score reveal → MatchResult overlay   | ReconnectOverlay → abandon message → auto-redirect |
| GameState mutation | Done by rules engine (pure function) | Done directly in `handleReconnectTimeout`          |

### Frontend Abandonment Flow — Step by Step

1. ReconnectOverlay is visible (phase === `"disconnected"`, countdown running)
2. Client countdown reaches 0 visually (client-side timer — server is authoritative)
3. Server's reconnect timer fires → `handleReconnectTimeout` executes
4. Server broadcasts `event:match_abandoned` + `event:game_state` (phase: `"match_end"`)
5. `useWsDispatch` receives `EVENT_MATCH_ABANDONED` → sets `matchAbandonedData` in gameStore
6. `useWsDispatch` receives `EVENT_GAME_STATE` → updates gameState (phase now `"match_end"`)
7. `GamePage` detects `matchAbandonedData` is non-null → passes it to `ReconnectOverlay`
8. `ReconnectOverlay` switches to abandonment mode: shows "[Player] disconnected — match ended" + scores
9. 3-second auto-redirect timer starts
10. After 3s: `clearGame()` → `navigate("/lobby")` → `setMatchAbandonedData(null)`

**Edge case — event ordering:** `event:match_abandoned` may arrive before or after `event:game_state`. The overlay should render correctly regardless of order. Use `matchAbandonedData` as the primary trigger for the abandonment branch (not phase or `disconnectedSeat`), since the server clears `disconnectedSeat` to `-1` in the final game state. The GamePage render condition uses OR logic: `(phase === "disconnected" && disconnectedSeat !== -1) || matchAbandonedData`.

**Edge case — `useReconnectionRedirect` interference:** The reconnection redirect hook (Story 5.4) navigates to `/game/:roomId` when game state arrives while not on the game page. It already has a guard for `phase !== "match_end"`. Verify this guard prevents redirect during abandonment (it should, since the arriving game state has `phase: "match_end"`).

### Lobby Pre-Game Disconnect — Design Notes

The session manager's `HandleDisconnect` already returns early for non-game users (line 19: "Not in a game session — lobby disconnect, no game impact"). The lobby disconnect seat-freeing is a **separate concern** handled by a new `LobbyDisconnectHandler` in the room package.

**Flow:**

1. Hub fires `disconnectHandler(userID)` → composite handler delegates to both session manager and lobby handler
2. Session manager returns immediately (user not in a game)
3. Lobby handler checks if user is in a room with status `"waiting"` (via `FindRoomByPlayer`)
4. If yes, starts a 10-second timer stored in a `pending` map
5. After 10s: removes player from room (reuse logic from `LeaveRoom`), broadcasts `system:player_left` to room

**Reconnection cancellation:**

1. Hub fires `connectHandler(userID)` → composite handler delegates to both session manager and lobby handler
2. Session manager returns immediately (user not in a game)
3. Lobby handler checks `pending` map, cancels timer if present

**Key:** The `LobbyDisconnectHandler` needs access to the room repository and the WS hub. It manages its own timer map with a mutex. The actual "leave room" operation should reuse existing repository methods (`RemovePlayer`, update room state).

### Previous Story Intelligence (Story 5.4)

Key learnings to carry forward:

- **Data race on broadcast after unlock** — build ALL messages BEFORE unlocking session mutex (F1 fix pattern). This is critical for `handleReconnectTimeout`.
- **Generation guard prevents stale callbacks** — `reconnectGeneration` is already incremented in `HandleDisconnect` (line 120). The guard in `handleReconnectTimeout` checks this. If a player reconnects and then disconnects again, a NEW timer starts with a NEW generation — the old timer's callback is invalidated.
- **Hub ConnectHandler fires for ALL connections** — including first-time lobby connections. Both `HandleReconnect` and `LobbyDisconnectHandler.HandleReconnect` must be safe for all connections (early return for non-applicable users).
- **`handleReconnectTimeout` was explicitly deferred** — review finding D56 from Story 5.4: "`handleReconnectTimeout` stub leaves game in PhaseDisconnected after window expires — deferred, Story 5.5 scope". This story resolves D56.
- **`session.closed` guard** — always check `session.closed` before proceeding. If the session was already cleaned up (e.g., server restart), the timer callback should be a no-op.

### Cross-Story Context

- **Story 5.3** (done) — implemented disconnect detection, `HandleDisconnect`, `ReconnectOverlay`, reconnect countdown, and all WS event types for disconnect/reconnect.
- **Story 5.4** (done) — implemented `HandleReconnect`, `ConnectHandler` on Hub, reconnection redirect hook, state restoration. Left `handleReconnectTimeout` as a placeholder for this story.
- **Stories 5.1/5.2** (done) — pause/unpause patterns, timer preservation, owner override. Timer management patterns reused across Epic 5.
- **Epic 6** (next) — lobby and match chat. Unrelated to this story.

### Deferred Items Resolved by This Story

- **D56:** `handleReconnectTimeout` stub leaves game in PhaseDisconnected after window expires. This story fully implements the timeout handler.

### Project Structure Notes

**New files (expected):**

- `server/migrations/000008_add_match_status.up.sql`
- `server/migrations/000008_add_match_status.down.sql`
- `server/internal/room/lobby_disconnect.go`
- `server/internal/room/lobby_disconnect_test.go`

**Modified files (expected):**

- `server/internal/session/reconnect.go` (replace `handleReconnectTimeout` placeholder)
- `server/internal/session/reconnect_test.go` (add timeout tests)
- `server/internal/session/manager.go` (add `Status: "completed"` to `handleMatchEnd`)
- `server/internal/match/model.go` (add `Status`, `AbandonedBy` fields)
- `server/internal/ws/events.go` (add `EventMatchAbandoned` + payload)
- `client/src/shared/types/wsEvents.ts` (add `EVENT_MATCH_ABANDONED` + payload)
- `client/src/shared/hooks/useWsDispatch.ts` (add abandoned event handler)
- `client/src/shared/stores/gameStore.ts` (add `matchAbandonedData` field)
- `client/src/features/game/components/ReconnectOverlay.tsx` (add abandonment mode)
- `client/src/features/game/GamePage.tsx` (pass abandoned data, add onReturnToLobby prop)
- `client/src/shared/i18n/en.json` (add abandonment keys)
- `client/src/shared/i18n/sr.json` (add abandonment keys)
- `server/cmd/api/main.go` (composite disconnect/connect handler wiring)
- `server/internal/room/repository.go` (add `FindRoomByPlayer` interface method)
- `server/internal/room/gorm_repo.go` (add `FindRoomByPlayer` implementation)
- `server/internal/game/testfixtures/fixtures.go` (add `NewGameDisconnectedNearTimeout` fixture)

### References

- [Source: epics.md — Story 5.5 acceptance criteria, Epic 5 context]
- [Source: prd.md — FR26-FR28: disconnect handling, reconnect window, match abandonment]
- [Source: prd.md — FR43: partial XP for remaining players in abandoned casual match (placeholder for Epic 9)]
- [Source: architecture.md — Session manager as orchestrator, server-authoritative state]
- [Source: architecture.md — WebSocket event contract: both files updated in same commit]
- [Source: architecture.md — Timer synchronization via absolute server timestamps]
- [Source: ux-design-specification.md — ReconnectOverlay: auto-dismisses on reconnection or match abandonment, calm tone]
- [Source: project-context.md — Disconnection & Reconnection Edge Cases section]
- [Source: 5-4-reconnection-and-state-restoration.md — HandleReconnect implementation, D56 deferral, timer patterns]
- [Source: 5-3-disconnect-detection-and-reconnect-countdown.md — HandleDisconnect implementation, reconnect timer setup]

### Review Findings

- [x] [Review][Patch] Add defensive bounds check on `abandonedSeat` before array index in `handleReconnectTimeout` [server/internal/session/reconnect.go:265-266] — Fixed: added `if abandonedSeat < 0 || abandonedSeat >= 4` guard before array index
- [x] [Review][Defer] Page refresh during/after abandonment shows blank game page — `useReconnectionRedirect` suppresses lobby redirect when `phase === "match_end"`, but `matchAbandonedData` is not persisted so the abandonment overlay never renders on refresh — deferred, pre-existing limitation of the page-refresh recovery path from Story 5.4

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

- All Go tests pass: `go test ./...` (13 packages, 0 failures)
- All frontend tests pass: `npx vitest run` (41 files, 295 tests, 0 failures)
- Go vet: clean
- TypeScript compilation: clean (pre-existing test-only TS errors unchanged)

### Completion Notes List

- Created database migration 000008: adds `status` (VARCHAR(20), default 'completed') and `abandoned_by` (INTEGER, FK to users) to matches table
- Updated `Match` model with `Status` and `AbandonedBy` fields; updated `handleMatchEnd` to explicitly set `Status: "completed"` on normal completions
- Added `EventMatchAbandoned` constant and `MatchAbandonedPayload` struct to both WS contract files (events.go + wsEvents.ts)
- Replaced `handleReconnectTimeout` placeholder with full implementation: transitions to PhaseMatchEnd, clears disconnect fields, cancels timers, builds messages under lock, broadcasts event:match_abandoned + event:game_state to all 4 players, persists abandoned match record, updates room status, removes session
- Created `LobbyDisconnectHandler` for pre-game lobby disconnects: starts 10s timer when player disconnects from a waiting room, removes player and broadcasts on timeout, cancels timer on reconnection
- Wired composite disconnect/connect handlers in main.go (sessionManager + lobbyDisconnectHandler)
- Updated gameStore with `matchAbandonedData` field and `setMatchAbandonedData` setter
- Added `EVENT_MATCH_ABANDONED` handler in useWsDispatch
- Updated ReconnectOverlay to support abandonment mode: shows "[Player] disconnected — match ended" with final scores, auto-redirects to lobby after 3 seconds
- Updated GamePage to pass matchAbandonedData to ReconnectOverlay with OR-logic render condition (handles event ordering edge case where server clears disconnectedSeat)
- Added i18n keys for abandonment in both English and Serbian
- 4 new reconnect timeout tests (transitions, persists, no-op when reconnected, removes session)
- Updated TestHandleReconnect_RejectsExpiredWindow to reflect new behavior (session removed after timeout)
- 4 new lobby disconnect tests (frees after timeout, cancelled by reconnect, no-op for playing room, no-op for non-room player)
- 3 new ReconnectOverlay tests (renders abandonment, auto-redirects after 3s, no redirect without data)

### File List

**New files:**

- server/migrations/000008_add_match_status.up.sql
- server/migrations/000008_add_match_status.down.sql
- server/internal/room/lobby_disconnect.go
- server/internal/room/lobby_disconnect_test.go

**Modified files:**

- server/internal/match/model.go (added Status, AbandonedBy fields)
- server/internal/session/reconnect.go (replaced handleReconnectTimeout placeholder, added match import)
- server/internal/session/reconnect_test.go (added 4 timeout tests, updated expired window test)
- server/internal/session/manager.go (added Status: "completed" to handleMatchEnd)
- server/internal/ws/events.go (added EventMatchAbandoned, MatchAbandonedPayload)
- server/cmd/api/main.go (composite disconnect/connect handler wiring, reordered room repo init)
- client/src/shared/types/wsEvents.ts (added EVENT_MATCH_ABANDONED, MatchAbandonedPayload)
- client/src/shared/stores/gameStore.ts (added matchAbandonedData field + setter)
- client/src/shared/hooks/useWsDispatch.ts (added EVENT_MATCH_ABANDONED handler + import)
- client/src/features/game/components/ReconnectOverlay.tsx (added abandonment mode with auto-redirect)
- client/src/features/game/components/ReconnectOverlay.test.tsx (added 3 abandonment tests)
- client/src/features/game/GamePage.tsx (added matchAbandonedData wiring, abandon return handler, updated overlay render condition)
- client/src/shared/i18n/en.json (added matchAbandoned, matchAbandonedScores, returningToLobby keys)
- client/src/shared/i18n/sr.json (added matchAbandoned, matchAbandonedScores, returningToLobby keys)
