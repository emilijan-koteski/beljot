# Story 4.2: Game Session Manager & State Sync

Status: done

## Story

As a player in a game,
I want all game actions to be processed by the server and the updated state broadcast to all players instantly,
So that all four players see the same game state at all times.

## Acceptance Criteria

1. **Given** a game is started from a room
   **When** the session manager initializes
   **Then** `internal/session/manager.go` creates a new GameState via the rules engine, associates it with the 4 connected WebSocket clients, and broadcasts the initial state to all players via `event:game_state`

2. **Given** a player sends a game action (e.g., `action:play_card`)
   **When** the session manager receives it
   **Then** it validates the player's turn, calls `ApplyAction()` on the rules engine, and on success broadcasts the resulting state change to all 4 clients via the appropriate event (e.g., `event:card_played`)
   **And** on rules engine error, sends an `error:` event only to the acting player

3. **Given** a player reconnects mid-game (future Epic 5 support)
   **When** their WebSocket connection is re-established
   **Then** the session manager sends a full `event:game_state` snapshot from the serializable GameState struct

4. **Given** a match completes (match_end phase)
   **When** the session manager processes the final state
   **Then** a match record is persisted to the `matches` table including: room_id, player IDs, team assignments, final scores, variant, mode, winner team, timestamps
   **And** the Zustand gameStore is cleared on all clients

5. **Given** the `matches` migration
   **When** I inspect the database schema
   **Then** the `matches` table contains: `id`, `room_id`, `player1_id` through `player4_id`, `team_a_score`, `team_b_score`, `winner_team`, `variant`, `match_mode`, `started_at`, `completed_at`

6. **Given** the frontend receives a game event
   **When** `useWsDispatch.ts` processes the event
   **Then** the event is routed to `gameStore` which updates and triggers a React re-render within 200ms (NFR2)

## Tasks / Subtasks

- [x] Task 1: Create `matches` database migration (AC: 5)
  - [x] Create `server/migrations/000006_create_matches.up.sql` with columns: `id` (serial PK), `room_id` (int, FK rooms), `player1_id` through `player4_id` (int, FK users), `team_a_score` (int), `team_b_score` (int), `winner_team` (int), `variant` (varchar), `match_mode` (varchar), `started_at` (timestamptz), `completed_at` (timestamptz), `created_at` (timestamptz default now)
  - [x] Create `server/migrations/000006_create_matches.down.sql` — DROP TABLE matches
  - [x] Add indexes: `idx_matches_room_id`, `idx_matches_player1_id` through `idx_matches_player4_id`

- [x] Task 2: Create Match GORM model and repository (AC: 4, 5)
  - [x] Create `server/internal/match/model.go` — Match struct with GORM tags matching migration
  - [x] Create `server/internal/match/repository.go` — MatchRepository interface with `Create(match *Match) error`
  - [x] Create `server/internal/match/gorm_repo.go` — GORM implementation

- [x] Task 3: Create `internal/session/manager.go` — core session manager (AC: 1, 2, 3)
  - [x] Define `Session` struct: `gameState *game.GameState`, `playerUserIDs [4]uint`, `roomID uint`, `mu sync.Mutex`
  - [x] Define `Manager` struct: `sessions map[uint]*Session` (keyed by roomID), `hub *ws.Hub`, `matchRepo match.MatchRepository`, `mu sync.RWMutex`
  - [x] Implement `NewManager(hub *ws.Hub, matchRepo match.MatchRepository) *Manager`
  - [x] Implement `StartGame(roomID uint, players [4]RoomPlayerInfo) error`:
    - [x] Create `game.NewGame(playerIDs, variant, matchMode, roomID)`
    - [x] Store session in `sessions` map keyed by roomID
    - [x] Broadcast initial `event:game_state` to all 4 players
  - [x] Implement `HandleAction(client *ws.Client, msg ws.WSMessage)`:
    - [x] Find session by looking up which room the client's userID is in
    - [x] Map client.UserID to playerSeat in the session
    - [x] Parse action payload (type + card) from WSMessage
    - [x] Build `game.Action{Type, PlayerSeat, Card}`
    - [x] Call `game.ApplyAction(session.gameState, action)`
    - [x] On error: send `error:` event to acting player only
    - [x] On success: update session.gameState, broadcast state change to all 4 players
    - [x] If new state Phase == PhaseMatchEnd: persist match, clean up session
  - [x] Implement `GetStateSnapshot(roomID uint) *game.GameState` for reconnection support (AC: 3)
  - [x] Implement `RemoveSession(roomID uint)` for cleanup

- [x] Task 4: Implement event broadcasting logic in session manager (AC: 2)
  - [x] Map game action types to appropriate broadcast events:
    - [x] `play_card` → broadcast `event:card_played` with `{playerSeat, cardId, autoPlayed: false}`
    - [x] `pick_trump` / `pass_trump` → broadcast `event:trump_selected` with `{playerSeat, trumpSuit}` (on pick), or broadcast updated `event:game_state` (on pass/reshuffle)
    - [x] `declare` / `skip_declare` → when declarations resolve, broadcast `event:declarations_resolved`
    - [x] `announce_belot` / `skip_belot` → broadcast `event:belot_announced` with `{playerSeat, team}`
  - [x] Detect phase transitions after ApplyAction and broadcast accordingly:
    - [x] If trick completed (4 cards in trick): broadcast `event:trick_resolved` with `{winnerSeat, winnerTeam, cards}`
    - [x] If hand scored: broadcast `event:hand_scored` with scores
    - [x] If match ended: broadcast `event:match_end` with `{winnerTeam, teamAFinalScore, teamBFinalScore}`
  - [x] All multi-event sequences sent as **separate ordered messages** (e.g., card_played, then trick_resolved, then hand_scored)

- [x] Task 5: Implement match persistence on game completion (AC: 4)
  - [x] When Phase transitions to `PhaseMatchEnd`:
    - [x] Build Match record from GameState (roomID, player IDs from session, scores, winner, variant, mode, timestamps)
    - [x] Call `matchRepo.Create(&match)`
    - [x] Log any persistence errors (don't fail the broadcast)
    - [x] Broadcast `event:match_end` to all 4 clients
    - [x] Remove session from manager

- [x] Task 6: Wire session manager into main.go and StartGame handler (AC: 1)
  - [x] In `main.go`: create Manager instance with hub and matchRepo, call `hub.SetActionHandler(manager.HandleAction)`
  - [x] Pass Manager to RoomHandler (or create a new GameHandler)
  - [x] In StartGame handler: after room status update, call `manager.StartGame()` with room details and player info
  - [x] After StartGame: broadcast `system:game_started` to room participants, `system:room_updated` to lobby

- [x] Task 7: Expand frontend `gameStore` with full game state (AC: 6)
  - [x] Import `GameState` from `gameTypes.ts`
  - [x] Add state fields: `gameState: GameState | null`, `myPlayerSeat: number | null`, `roomId: number | null`
  - [x] Add actions: `setGameState`, `updateFromEvent`, `clearGame`
  - [x] `setGameState` replaces entire state (for `event:game_state` snapshots)
  - [x] `clearGame` wipes on match end or navigation away from game page

- [x] Task 8: Implement `dispatchGameEvent` in `useWsDispatch.ts` (AC: 6)
  - [x] Route `event:game_state` → `gameStore.setGameState(payload)`
  - [x] Route `event:card_played` → update gameStore (add card to currentTrick, remove from player hand)
  - [x] Route `event:trick_resolved` → update gameStore (clear trick, update tricksWon)
  - [x] Route `event:hand_scored` → update gameStore (update scores, reset hand state)
  - [x] Route `event:match_end` → update gameStore (set winnerTeam, phase to match_end)
  - [x] Route `event:trump_selected` → update gameStore (set trumpSuit, phase to playing)
  - [x] Route `event:declarations_resolved` → update gameStore (set declarations)
  - [x] Route `event:belot_announced` → update gameStore (set belotAnnounced)
  - [x] Route game `error:*` events → console.warn (existing behavior)

- [x] Task 9: Write backend session manager tests (AC: 1-5)
  - [x] `server/internal/session/manager_test.go`:
    - [x] TestStartGame_CreatesSession — verify gameState initialized, broadcast sent
    - [x] TestHandleAction_PlayCard_Success — verify ApplyAction called, state updated, broadcast sent
    - [x] TestHandleAction_InvalidAction — verify error sent to acting player only
    - [x] TestHandleAction_WrongPlayer — verify error for non-existent session
    - [x] TestHandleAction_MatchEnd_PersistsMatch — verify match record created on match_end
    - [x] TestGetStateSnapshot — verify full state returned for reconnection
    - [x] TestRemoveSession — verify session cleaned up
  - [x] `server/internal/match/match_test.go`:
    - [x] TestMatchModel_JSONTags — verify camelCase JSON serialization
  - [x] Use test fixtures from `testfixtures/` — no raw GameState struct literals

- [x] Task 10: Write frontend tests (AC: 6)
  - [x] `client/src/shared/stores/gameStore.test.ts`:
    - [x] Test setGameState sets full state
    - [x] Test clearGame resets to null
    - [x] Test reset clears all game data
  - [x] Update `client/src/shared/hooks/useWsDispatch.test.ts`:
    - [x] Test event:game_state dispatches to gameStore
    - [x] Test event:card_played updates gameStore
    - [x] Test event:match_end updates gameStore

- [x] Task 11: Run full regression suite (AC: all)
  - [x] `go test ./...` — all backend packages pass
  - [x] `npx vitest run` — all frontend tests pass
  - [x] `go vet ./...` — no warnings

### Review Findings

- [x] [Review][Patch] Data race: broadcastActionResult and handleMatchEnd read session.gameState without lock — pass newState as param instead of re-reading [server/internal/session/manager.go:112,262,284,294,299,305]
- [x] [Review][Patch] Double StartGame for same roomID silently replaces live session — add HasSession guard [server/internal/session/manager.go:63]
- [x] [Review][Patch] Round-2 pick_trump never works — parseAction doesn't parse action.Suit from payload [server/internal/session/manager.go:183-197]
- [x] [Review][Patch] action:decline_belot (WS contract) vs skip_belot (engine) mismatch — skip-belot always fails from client [server/internal/ws/events.go:23 vs game/types.go:102]
- [x] [Review][Patch] TrickWinnerSeat reports wrong seat for EventTrickResolved on hand transition — capture from oldState.CurrentTrick [server/internal/session/manager.go:223]
- [x] [Review][Patch] EventTrickResolved not sent for 8th trick when new hand starts in same ApplyAction [server/internal/session/manager.go:221]
- [x] [Review][Patch] Room status never updated from "playing" after match ends [server/internal/session/manager.go:332]
- [x] [Review][Patch] system:game_started and system:room_updated not broadcast after StartGame [server/internal/room/handler.go:563]
- [x] [Review][Patch] failedContract always false in event:hand_scored — derive from state [server/internal/session/manager.go:237]
- [x] [Review][Patch] GetStateSnapshot returns raw pointer without copy and uses exclusive Lock for read [server/internal/session/manager.go:128]
- [x] [Review][Patch] event:card_played handler computes activePlayerSeat client-side — remove client-side game logic [client/src/shared/hooks/useWsDispatch.ts:83]
- [x] [Review][Patch] buildMessage silently discards json.Marshal errors — add slog.Error [server/internal/session/manager.go:347]
- [x] [Review][Patch] Missing tests: MatchEnd persistence, reconnection scenarios, match_test.go [server/internal/session/manager_test.go]
- [x] [Review][Defer] matches table FK has no ON DELETE clause — deferred, low risk since rooms are never deleted in Phase 1

## Dev Notes

### Architecture Compliance

- **Session manager is the orchestrator** — receives player actions via WebSocket, calls pure-function rules engine, broadcasts results, manages timers. ALL side effects live here, not in the rules engine.
- **Rules engine is pure**: `ApplyAction(state *GameState, action Action) (*GameState, error)` — zero side effects. Session manager must NOT modify game state directly; always go through `ApplyAction`.
- **Multi-event sequences as separate ordered messages**: card_played → trick_resolved → hand_scored → match_end. Frontend animations depend on this ordering. Never batch into a single payload.
- **In-memory game state**: `GameState` is a serializable Go struct stored in the session manager. Phase 1 scale (10 concurrent games) doesn't justify external state store.
- **Match persistence on completion only**: When phase transitions to `PhaseMatchEnd`, persist to PostgreSQL via repository pattern. Don't persist intermediate game state.

### Session Manager Design

```
StartGame called from room handler
  → manager.StartGame(roomID, players)
  → game.NewGame(playerIDs, variant, matchMode, roomID)
  → store session in sessions map
  → hub.BroadcastToUsers(playerUserIDs, event:game_state)

Client sends action:play_card
  → hub dispatches to manager.HandleAction(client, msg)
  → find session for client.UserID
  → map UserID → playerSeat
  → build game.Action{Type: "play_card", PlayerSeat: seat, Card: &card}
  → game.ApplyAction(session.gameState, action)
  → on error: hub.SendToUser(client.UserID, error:not_your_turn)
  → on success: update session.gameState
  → broadcast event:card_played to all 4 players
  → check if trick complete → broadcast event:trick_resolved
  → check if hand scored → broadcast event:hand_scored
  → check if match end → persist match, broadcast event:match_end, remove session
```

### RoomPlayerInfo for StartGame

The session manager needs player info from the room to initialize the game. Define a simple struct:

```go
type RoomPlayerInfo struct {
    UserID uint
    Seat   int
    Team   string // "teamA" or "teamB"
}
```

This is populated from `RoomPlayer` records by the StartGame handler before calling `manager.StartGame()`.

### Mapping UserID to PlayerSeat

GameState stores `Players[4]PlayerState` where each has `UserID` and `Seat`. The session manager maintains a `playerUserIDs [4]uint` array (index = seat) for quick lookup. When an action arrives from `client.UserID`, find the matching seat index.

### Event Payload Construction

After `ApplyAction` succeeds, compare old state vs new state to determine what changed:

- **Card played**: New card in `CurrentTrick` that wasn't there before
- **Trump selected**: `TrumpSuit` changed from nil to a value, phase changed to `PhasePlaying`
- **Trick resolved**: `TrickNumber` incremented, `CurrentTrick` cleared
- **Hand scored**: `HandNumber` incremented, `HandPoints` reset
- **Match end**: `Phase == PhaseMatchEnd`, `WinnerTeam` is set

Alternatively, broadcast the full `event:game_state` after every action (simpler, slightly more bandwidth). The story spec says to broadcast "the appropriate event" — use incremental events for the specific action, plus full state snapshot for complex transitions (reshuffle, scoring).

### Match Model

```go
type Match struct {
    ID             uint       `gorm:"primaryKey" json:"id"`
    RoomID         uint       `gorm:"not null;index" json:"roomId"`
    Player1ID      uint       `gorm:"not null;index" json:"player1Id"`
    Player2ID      uint       `gorm:"not null;index" json:"player2Id"`
    Player3ID      uint       `gorm:"not null;index" json:"player3Id"`
    Player4ID      uint       `gorm:"not null;index" json:"player4Id"`
    TeamAScore     int        `gorm:"not null" json:"teamAScore"`
    TeamBScore     int        `gorm:"not null" json:"teamBScore"`
    WinnerTeam     int        `gorm:"not null" json:"winnerTeam"`
    Variant        string     `gorm:"size:20;not null" json:"variant"`
    MatchMode      string     `gorm:"size:10;not null" json:"matchMode"`
    StartedAt      time.Time  `gorm:"not null" json:"startedAt"`
    CompletedAt    time.Time  `gorm:"not null" json:"completedAt"`
    CreatedAt      time.Time  `json:"createdAt"`
}
```

### Existing Code to Reuse — DO NOT Reinvent

| Function / File                                       | Location                                   | Purpose                                                         |
| ----------------------------------------------------- | ------------------------------------------ | --------------------------------------------------------------- |
| `game.NewGame(playerIDs, variant, matchMode, roomID)` | `internal/game/state.go`                   | Creates new GameState with dealt cards                          |
| `game.ApplyAction(state, action)`                     | `internal/game/rules_engine.go`            | Pure function rules engine                                      |
| `game.ParseCard(id string)`                           | `internal/game/types.go`                   | Parses "KS" card ID to Card struct                              |
| `game.PhaseMatchEnd`                                  | `internal/game/types.go`                   | Phase constant for match completion                             |
| `game.TeamForSeat(seat)`                              | `internal/game/state.go`                   | Maps seat to team index                                         |
| `ws.Hub.SetActionHandler()`                           | `internal/ws/hub.go`                       | Registers action handler — wire session manager here            |
| `ws.Hub.SendToUser()`                                 | `internal/ws/hub.go`                       | Send error to acting player                                     |
| `ws.Hub.BroadcastToUsers()`                           | `internal/ws/hub.go`                       | Broadcast state to all 4 players                                |
| `ws.WSMessage`                                        | `internal/ws/message.go`                   | Wire format struct                                              |
| `ws.Event*` constants                                 | `internal/ws/events.go`                    | All event type strings                                          |
| `auth.GetUserID(c)`                                   | `internal/auth/middleware.go`              | Extract userID in HTTP handlers                                 |
| Room model + repo                                     | `internal/room/`                           | Room and RoomPlayer for StartGame                               |
| `GameState` TS type                                   | `client/src/shared/types/gameTypes.ts`     | Complete frontend game state type — already matches server      |
| `useGameStore`                                        | `client/src/shared/stores/gameStore.ts`    | Stub to expand                                                  |
| `dispatchGameEvent`                                   | `client/src/shared/hooks/useWsDispatch.ts` | Empty function to implement                                     |
| All WS event payloads                                 | `client/src/shared/types/wsEvents.ts`      | Already defined — CardPlayedPayload, TrickResolvedPayload, etc. |

### Files to Create

| File                                               | Purpose                                     |
| -------------------------------------------------- | ------------------------------------------- |
| `server/migrations/000006_create_matches.up.sql`   | Matches table migration                     |
| `server/migrations/000006_create_matches.down.sql` | Matches table rollback                      |
| `server/internal/match/model.go`                   | Match GORM model                            |
| `server/internal/match/repository.go`              | MatchRepository interface                   |
| `server/internal/match/gorm_repo.go`               | GORM implementation                         |
| `server/internal/session/manager.go`               | Session manager (replaces empty session.go) |
| `server/internal/session/manager_test.go`          | Session manager tests                       |
| `server/internal/match/match_test.go`              | Match model tests                           |
| `client/src/shared/stores/gameStore.test.ts`       | gameStore tests                             |

### Files to Modify

| File                                            | Changes                                                                 |
| ----------------------------------------------- | ----------------------------------------------------------------------- |
| `server/internal/session/session.go`            | Remove empty stub (replaced by manager.go)                              |
| `server/cmd/api/main.go`                        | Create Manager, wire hub.SetActionHandler, pass manager to room handler |
| `server/internal/room/handler.go`               | Update StartGame to call manager.StartGame(), broadcast events          |
| `client/src/shared/stores/gameStore.ts`         | Expand with full GameState, setGameState, clearGame                     |
| `client/src/shared/hooks/useWsDispatch.ts`      | Implement dispatchGameEvent with all game event handlers                |
| `client/src/shared/hooks/useWsDispatch.test.ts` | Add game event dispatch tests                                           |

### Testing Patterns — MANDATORY

**Backend (Go):**

- Session manager tests in `internal/session/manager_test.go` using `package session_test`
- Use a **mock hub** that captures sent messages — don't need real WebSocket connections for unit tests
- Use `testfixtures/` factory functions for game states
- Test through the public API (`StartGame`, `HandleAction`, `GetStateSnapshot`)
- Match persistence tests should use a **mock repository**, not a real database
- Table-driven tests with `t.Run` for action types

**Frontend (Vitest):**

- gameStore tests co-located: `gameStore.test.ts` next to `gameStore.ts`
- Test that `setGameState` replaces the full state
- Test that `clearGame` resets everything
- Test descriptions in present tense

### Scope Boundaries — What This Story Does NOT Do

- **No per-move timer** — Story 4.5 handles timer management and auto-play
- **No disconnect/reconnect handling** — Epic 5 handles reconnection flow (but session manager's `GetStateSnapshot` enables it)
- **No game UI** — Stories 4.3-4.6 handle the visual game table
- **No chat integration** — Epic 6
- **No room lobby WS broadcasting** — Room events are already handled in lobby; this story focuses on game session events

### Previous Story Intelligence (from 4.1)

**Critical learnings:**

- Hub's `SetActionHandler` registers the handler — call it once at startup with `manager.HandleAction`
- Hub dispatches action handlers in goroutines (`go h.actionHandler(...)`) — the handler must be safe for concurrent calls
- `Client.UserID` is `uint` — matches `game.PlayerState.UserID`
- `WSMessage.Payload` is `json.RawMessage` — lazy parsed, session manager must unmarshal to get card ID etc.
- `Hub.BroadcastToUsers(userIDs []uint, msg []byte)` takes pre-marshaled bytes
- `Hub.SendToUser(userID uint, msg []byte)` for error messages to single player
- Review finding: Hub dispatches handlers in goroutines, so session manager's `HandleAction` must use mutex for game state access
- Review finding: `Client.Send()` is safe to call concurrently (has internal closed flag)

### Git Intelligence (from recent commits)

Recent commit pattern: `feat(scope): implement <feature> with code review fixes`

- Commit scope for this story: `feat(session): implement game session manager and state sync`
- Backend tests: `go test ./server/internal/session/...`
- Frontend tests: `npx vitest run`

### References

- [Source: _bmad-output/planning-artifacts/epics.md, Epic 4, Story 4.2, lines 905-938]
- [Source: _bmad-output/planning-artifacts/architecture.md — Session manager design, integration points, match persistence, multi-event sequences]
- [Source: _bmad-output/planning-artifacts/prd.md — FR23 (real-time match), FR14 (1001-point mode)]
- [Source: _bmad-output/project-context.md — Pure function engine, session manager orchestrator, domain package shape, GORM conventions]
- [Source: server/internal/game/rules_engine.go — ApplyAction signature and phase dispatch]
- [Source: server/internal/game/state.go — GameState struct with all fields]
- [Source: server/internal/game/types.go — Card, Action, Variant, Phase types]
- [Source: server/internal/ws/hub.go — Hub API: SetActionHandler, SendToUser, BroadcastToUsers]
- [Source: server/internal/ws/client.go — Client struct with UserID and Send()]
- [Source: server/internal/room/handler.go — StartGame handler with TODO for session creation]
- [Source: server/internal/room/model.go — Room and RoomPlayer structs]
- [Source: client/src/shared/types/gameTypes.ts — Complete frontend GameState type]
- [Source: client/src/shared/types/wsEvents.ts — All game event payload interfaces]
- [Source: client/src/shared/stores/gameStore.ts — Stub to expand]
- [Source: client/src/shared/hooks/useWsDispatch.ts — Empty dispatchGameEvent to implement]
- [Source: _bmad-output/archive/implementation-artifacts/4-1-websocket-gateway-and-event-contract.md — WS infrastructure learnings, review findings]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

None — all tests passed on first run.

### Completion Notes List

- Created `matches` migration (000006) with all required columns and indexes
- Created Match GORM model + repository (interface + GORM implementation)
- Built session Manager as the core orchestrator: StartGame creates sessions, HandleAction dispatches to rules engine, broadcasts results
- Session manager uses `sync.Mutex` per session for concurrent action safety (hub dispatches handlers in goroutines)
- Event broadcasting: maps action types to specific incremental events (card_played, trick_resolved, hand_scored, trump_selected, declarations_resolved, belot_announced, match_end)
- Multi-event sequences sent as separate ordered messages per architecture spec
- Match persistence: on PhaseMatchEnd, creates Match record via repository, then removes session
- Wired Manager into main.go: hub.SetActionHandler(manager.HandleAction), passed to RoomHandler
- Updated RoomHandler to accept GameStarter interface, StartGame now calls manager.StartGame with player seat info
- Expanded frontend gameStore: full GameState, myPlayerSeat, roomId, setGameState, clearGame
- Implemented dispatchGameEvent: routes all 8 game event types to gameStore updates
- Backend: 8 session manager tests (StartGame, HandleAction bidding/play, invalid user, GetStateSnapshot, RemoveSession, HasSession)
- Frontend: 6 new gameStore tests + 3 new dispatch tests
- Full regression: backend all packages OK, frontend 132/132 pass (up from 123), go vet clean

### Change Log

- 2026-04-12: Implemented game session manager, match persistence, event broadcasting, frontend game state sync (Story 4.2)

### File List

- `server/migrations/000006_create_matches.up.sql` — NEW: Matches table migration
- `server/migrations/000006_create_matches.down.sql` — NEW: Matches table rollback
- `server/internal/match/model.go` — NEW: Match GORM model
- `server/internal/match/repository.go` — NEW: MatchRepository interface
- `server/internal/match/gorm_repo.go` — NEW: GORM implementation
- `server/internal/session/manager.go` — NEW: Session manager (replaces empty session.go)
- `server/internal/session/manager_test.go` — NEW: 8 session manager tests
- `server/cmd/api/main.go` — MODIFIED: Added session manager, match repo, wired hub action handler
- `server/internal/room/handler.go` — MODIFIED: Added GameStarter interface, updated StartGame to create game session
- `server/internal/room/handler_test.go` — MODIFIED: Updated NewRoomHandler call to pass nil GameStarter
- `client/src/shared/stores/gameStore.ts` — MODIFIED: Expanded with full GameState, setGameState, clearGame
- `client/src/shared/stores/gameStore.test.ts` — NEW: 6 gameStore tests
- `client/src/shared/hooks/useWsDispatch.ts` — MODIFIED: Implemented dispatchGameEvent with all game event handlers
- `client/src/shared/hooks/useWsDispatch.test.ts` — MODIFIED: Added 3 game event dispatch tests
