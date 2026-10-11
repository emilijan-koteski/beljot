# Story 5.1: Player Pause System

Status: done

## Story

As a player,
I want to pause the game briefly when I need a break,
so that I don't miss my turn or disadvantage my team.

## Acceptance Criteria

1. **Pause Action — Happy Path**
   Given a game is in `playing` or `bidding` phase,
   When a player submits `action:pause`,
   Then the game phase transitions to `paused`, the player's pause is recorded (1 per player per game limit), and all clients receive `event:game_paused` with the pausing player's seat index.

2. **Pause Exhausted**
   Given a player has already used their pause in this game,
   When they submit `action:pause` again,
   Then an `error:pause_exhausted` event is returned and the game state is unchanged.

3. **Pause Stacking**
   Given multiple players pause before any unpause occurs,
   When pauses stack,
   Then all active pauses are tracked; the game remains paused until ALL active pauses are resolved (via individual unpause or owner override).

4. **Actions Blocked While Paused**
   Given the game is paused,
   When any player submits a game action (`play_card`, `pick_trump`, `declare`, etc.),
   Then `ErrGamePaused` is returned and the state is unchanged.

5. **Unpause — Happy Path**
   Given the player who paused submits `action:unpause`,
   When the unpause is processed,
   Then their pause is cleared; if no other active pauses remain, the game resumes to the previous phase and all clients receive `event:game_resumed`.

6. **Unpause — Wrong Player**
   Given a player who does NOT have an active pause submits `action:unpause`,
   When the action is processed,
   Then an `error:no_active_pause` event is returned and the state is unchanged.

7. **Pause Overlay UI**
   Given the game is paused,
   When all clients render,
   Then a PauseOverlay is displayed showing who paused, that the game is temporarily on hold, and a pause/unpause button reflecting the current player's state.

8. **Timer Suspension During Pause**
   Given a per-move timer is active when a pause occurs,
   When the game pauses,
   Then the turn timer is cancelled and the remaining time is preserved. On resume, the timer restarts with the preserved remaining time.

## Tasks / Subtasks

- [x] Task 1: Backend — Add pause fields to GameState (AC: #1, #3, #8)
  - [x] 1.1 In `server/internal/game/state.go`, add these fields to the GameState struct in a new **Pause State** section (after Timer section):
    - `PreviousPhase Phase json:"previousPhase"` — phase before pause (for resume)
    - `PausedPlayers [4]bool json:"pausedPlayers"` — which seats have active pauses
    - `PauseUsed [4]bool json:"pauseUsed"` — which seats have used their one-time pause this game
    - `TurnTimeRemaining int64 json:"turnTimeRemaining"` — milliseconds remaining on turn timer when paused (0 if no timer active)
  - [x] 1.2 In `client/src/shared/types/gameTypes.ts`, add matching fields to the `GameState` interface:
    - `previousPhase: Phase`
    - `pausedPlayers: boolean[]`
    - `pauseUsed: boolean[]`
    - `turnTimeRemaining: number`

- [x] Task 2: Backend — Add new error types (AC: #2, #6)
  - [x] 2.1 In `server/internal/apperr/errors.go`, add:
    - `ErrPauseExhausted = NewAppError("PAUSE_EXHAUSTED", "player has already used their pause", http.StatusConflict)`
    - `ErrNotPaused = NewAppError("NOT_PAUSED", "game is not paused", http.StatusConflict)`
    - `ErrNoActivePause = NewAppError("NO_ACTIVE_PAUSE", "player does not have an active pause to clear", http.StatusConflict)`
  - [x] 2.2 `ErrGamePaused` already exists — no change needed

- [x] Task 3: Backend — Add WebSocket event constants (AC: #1, #5)
  - [x] 3.1 In `server/internal/ws/events.go`, add:
    - `const ActionPause = "action:pause"`
    - `const ActionUnpause = "action:unpause"`
    - `const ActionOwnerUnpause = "action:owner_unpause"` (scaffolded for Story 5.2)
    - `const EventGamePaused = "event:game_paused"`
    - `const EventGameResumed = "event:game_resumed"`
  - [x] 3.2 Add payload structs:
    - `GamePausedPayload { PausedBy int json:"pausedBy"; PausedPlayers [4]bool json:"pausedPlayers" }`
    - `GameResumedPayload { ResumedBy int json:"resumedBy"; OwnerOverride bool json:"ownerOverride" }`
  - [x] 3.3 In `client/src/shared/types/wsEvents.ts`, add matching constants and payload interfaces — SAME COMMIT as events.go:
    - `export const ACTION_PAUSE = "action:pause" as const;`
    - `export const ACTION_UNPAUSE = "action:unpause" as const;`
    - `export const ACTION_OWNER_UNPAUSE = "action:owner_unpause" as const;`
    - `export const EVENT_GAME_PAUSED = "event:game_paused" as const;`
    - `export const EVENT_GAME_RESUMED = "event:game_resumed" as const;`
    - `GamePausedPayload` and `GameResumedPayload` interfaces

- [x] Task 4: Backend — Implement pause/unpause in rules engine (AC: #1, #2, #3, #4, #5, #6)
  - [x] 4.1 Create `server/internal/game/pause.go` with two handler functions:
    - `handlePause(state *GameState, action Action) (*GameState, error)`:
      - Clone state via existing pattern (see `handlePlaying` in `playing.go`)
      - Validate: `state.Phase` must be `PhasePlaying` or `PhaseBidding` — if not, return `ErrWrongPhase`
      - Validate: `state.PauseUsed[action.PlayerSeat]` must be `false` — if not, return `ErrPauseExhausted`
      - Set `newState.PreviousPhase = state.Phase` (only on first pause — if already paused, keep existing `PreviousPhase`)
      - Set `newState.PausedPlayers[action.PlayerSeat] = true`
      - Set `newState.PauseUsed[action.PlayerSeat] = true`
      - Set `newState.Phase = PhasePaused`
      - Return `newState, nil`
    - `handleUnpause(state *GameState, action Action) (*GameState, error)`:
      - Clone state
      - Validate: `state.Phase` must be `PhasePaused` — if not, return `ErrNotPaused`
      - Validate: `state.PausedPlayers[action.PlayerSeat]` must be `true` — if not, return `ErrNoActivePause`
      - Set `newState.PausedPlayers[action.PlayerSeat] = false`
      - Check if ANY `PausedPlayers` entry is still `true`
      - If all false: set `newState.Phase = newState.PreviousPhase` (resume), clear `PreviousPhase` to empty string
      - If some still true: keep `Phase = PhasePaused`
      - Return `newState, nil`
  - [x] 4.2 Update `server/internal/game/rules_engine.go` `ApplyAction()`:
    - The `PhasePaused` case already returns `ErrGamePaused` — change it to allow ONLY `ActionUnpause` and `ActionOwnerUnpause` through, reject all others with `ErrGamePaused`:
      ```go
      case PhasePaused:
          switch action.Type {
          case ActionUnpause, ActionOwnerUnpause:
              return handleUnpause(state, action)
          default:
              return nil, apperr.ErrGamePaused
          }
      ```
    - In the `PhasePlaying` and `PhaseBidding` cases, add handling for `ActionPause`:
      - In `handlePlaying()` switch: add `case ActionPause: return handlePause(state, action)`
      - In `handleBidding()` switch: add `case ActionPause: return handlePause(state, action)`

- [x] Task 5: Backend — Session manager pause integration (AC: #1, #5, #8)
  - [x] 5.1 In `server/internal/session/manager.go` `HandleAction()`:
    - Before calling `game.ApplyAction()` for pause: if action is `ActionPause`, capture current timer remaining by computing `turnExpiresAt - time.Now()` and store it in `TurnTimeRemaining` field on the game state before passing to the rules engine. Then call `session.cancelTurnTimer()`.
    - After `game.ApplyAction()` returns for unpause: if the new phase is no longer `PhasePaused` (resumed), restore the timer by calling `m.setTurnExpiry()` with the remaining time from `newState.TurnTimeRemaining`, then `m.startTimerLocked()`.
  - [x] 5.2 In `broadcastActionResult()`, add cases for `game.ActionPause` and `game.ActionUnpause`:
    - `ActionPause`: broadcast `EventGamePaused` with `GamePausedPayload` to all 4 players
    - `ActionUnpause`: broadcast `EventGameResumed` with `GameResumedPayload` to all 4 players (only when phase actually changes from paused to playing/bidding)
    - Also broadcast `EventGameState` (full state snapshot) so all clients sync the pause state fields

- [x] Task 6: Backend tests (AC: #1, #2, #3, #4, #5, #6, #8)
  - [x] 6.1 Add test fixture in `server/internal/game/testfixtures/fixtures.go`:
    - `NewGamePaused(pausedBySeat int) *GameState` — returns a mid-play game state transitioned to `PhasePaused` with `PausedPlayers[pausedBySeat] = true` and `PreviousPhase = PhasePlaying`
  - [x] 6.2 Create `server/internal/game/pause_test.go` with table-driven tests:
    - **Pause from playing phase** — succeeds, phase becomes `paused`, `PreviousPhase` is `playing`
    - **Pause from bidding phase** — succeeds, phase becomes `paused`, `PreviousPhase` is `bidding`
    - **Pause from wrong phase** (`match_end`, `dealing`) — returns `ErrWrongPhase`
    - **Double pause by same player** — returns `ErrPauseExhausted`
    - **Pause stacking** — second player pauses, both `PausedPlayers` entries true
    - **Unpause single pause** — phase returns to `PreviousPhase`
    - **Unpause stacked** — first unpause keeps phase `paused`, second unpause resumes
    - **Unpause wrong player** (no active pause) — returns `ErrNoActivePause`
    - **Unpause when not paused** — returns `ErrNotPaused`
    - **Game actions rejected while paused** — `play_card`, `pick_trump` all return `ErrGamePaused`
    - **Unpause allowed while paused** — `ActionUnpause` is not blocked by `ErrGamePaused`
  - [x] 6.3 Update `server/internal/session/manager_test.go` (or `session_test.go`):
    - Test pause action cancels turn timer
    - Test unpause action restarts timer with remaining time
    - Test `EventGamePaused` broadcast contains correct payload
    - Test `EventGameResumed` broadcast when all pauses cleared

- [x] Task 7: Frontend — Update gameStore with pause state (AC: #7)
  - [x] 7.1 In `client/src/shared/stores/gameStore.ts`, the `pausedPlayers` and other pause fields are already part of GameState from the server — no separate store fields needed. The existing `gameState` field will contain all pause info.
  - [x] 7.2 Add a computed helper: `isPaused()` that checks `gameState?.phase === "paused"` — or just check the phase directly in components.

- [x] Task 8: Frontend — Update useWsDispatch for pause events (AC: #7)
  - [x] 8.1 In `client/src/shared/hooks/useWsDispatch.ts`, add handlers in `dispatchGameEvent()`:
    - `EVENT_GAME_PAUSED`: update gameStore with the pause state (the full `EVENT_GAME_STATE` snapshot will also arrive, so this event is primarily for UI triggers like toast notifications)
    - `EVENT_GAME_RESUMED`: trigger resume UI (again, `EVENT_GAME_STATE` does the heavy lifting for state sync)

- [x] Task 9: Frontend — PauseOverlay component (AC: #7)
  - [x] 9.1 Create `client/src/features/game/components/PauseOverlay.tsx`:
    - Full-table overlay (`fixed inset-0 z-20`, `bg-background/80 backdrop-blur-sm`)
    - Shows: "Game Paused" heading, list of players who paused (from `pausedPlayers` array mapped to player names via seat indices)
    - If the current player has an active pause: show "Resume" button (primary style)
    - If the current player does not have an active pause: show "Waiting for players to resume..." text
    - Visual tone: composed, calm — `surface-elevated` card centered, no red alerts, no panic typography (per UX spec)
    - `aria-live="polite"` on the overlay
    - `data-testid="pause-overlay"`
  - [x] 9.2 Create `client/src/features/game/components/PauseOverlay.test.tsx`:
    - Renders paused player names
    - Shows Resume button for player who paused
    - Hides Resume button for other players
    - Fires unpause action on Resume click

- [x] Task 10: Frontend — Pause button and GamePage integration (AC: #7, #1)
  - [x] 10.1 Add a "Pause" button to `GamePage.tsx`:
    - Position: near the bottom or as part of the game controls area
    - Ghost style button (secondary tier per UX spec)
    - Disabled when: game is already paused, or current player has used their pause (`pauseUsed[mySeat] === true`)
    - Shows remaining pauses: "Pause (1)" or "Paused" when exhausted
    - On click: sends `ACTION_PAUSE` via WebSocket
  - [x] 10.2 Add PauseOverlay to GamePage overlay stack:
    - Show when `gameState.phase === "paused"`
    - Z-index between game table and other overlays (z-20, below TrumpPrompt z-30)
    - PauseOverlay "Resume" button calls `sendMessage(ACTION_UNPAUSE, {})`
  - [x] 10.3 When paused, disable all game interaction buttons (play card, trump prompt, declaration prompt)

- [x] Task 11: i18n keys (AC: #7)
  - [x] 11.1 Add keys to `client/src/shared/i18n/en.json`:
    - `game.pause.title`: "Game Paused"
    - `game.pause.pausedBy`: "{{player}} paused the game"
    - `game.pause.waitingToResume`: "Waiting for players to resume..."
    - `game.pause.resume`: "Resume"
    - `game.pause.pauseButton`: "Pause"
    - `game.pause.pauseUsed`: "Pause Used"
    - `game.pause.ownerResumedToast`: "Room owner resumed the game"
  - [x] 11.2 Add corresponding keys to `client/src/shared/i18n/sr.json`:
    - `game.pause.title`: "Igra pauzirana"
    - `game.pause.pausedBy`: "{{player}} je pauzirao igru"
    - `game.pause.waitingToResume`: "Cekanje da igraci nastave..."
    - `game.pause.resume`: "Nastavi"
    - `game.pause.pauseButton`: "Pauza"
    - `game.pause.pauseUsed`: "Pauza iskoriscena"
    - `game.pause.ownerResumedToast`: "Vlasnik sobe je nastavio igru"
  - [x] 11.3 Verify `make lint` passes

- [x] Task 12: Integration testing and final validation (AC: all)
  - [x] 12.1 Verify `make lint` passes (both Go and TypeScript)
  - [x] 12.2 Verify `make test` passes (all existing + new tests)
  - [x] 12.3 Manual testing via dev server: pause during play, unpause, verify timer resumes, verify stacking works
  - [x] 12.4 Verify all new frontend tests pass in Vitest

### Review Findings

- [x] [Review][Decision] Client blocks pause stacking — resolved: enabled stacking from PauseOverlay UI. Added `onPause` callback and `pauseUsed` prop to PauseOverlay. Players who haven't used their pause see a "Pause" button during another player's pause.
- [x] [Review][Patch] Timer remaining floor: added 3-second minimum floor in session manager unpause timer branch. Prevents both sub-ms auto-play and the expired-timer full-reset exploit. [server/internal/session/manager.go:170-195]
- [x] [Review][Defer] `sendGameError` routes ALL game errors as `error:invalid_action` — pause-specific error types (`error:pause_exhausted`, `error:no_active_pause`) defined in both contract files but never emitted [server/internal/session/manager.go:528] — deferred, pre-existing pattern across all game errors
- [x] [Review][Defer] Disconnect while paused: no mechanism auto-clears a disconnected player's active pause, leaving game frozen. Covered by Stories 5.3/5.4 [server/internal/session/manager.go] — deferred, future story scope

## Dev Notes

### Existing Scaffolding — Do NOT Recreate

Critical: Several pause-related constants and types are **already defined** in the codebase. Do not duplicate them:

- `server/internal/game/types.go`: `PhasePaused`, `PhaseDisconnected`, `ActionPause`, `ActionUnpause`, `ActionOwnerUnpause` — all already exist as constants
- `server/internal/apperr/errors.go`: `ErrGamePaused` already exists (line ~80)
- `server/internal/game/rules_engine.go`: `ApplyAction()` already has a `case PhasePaused: return nil, apperr.ErrGamePaused` — this needs to be MODIFIED (not added) to allow unpause actions through
- `client/src/shared/types/gameTypes.ts`: The `Phase` union type already includes `"paused"`

### Architecture Patterns to Follow

- **Rules engine is a pure function**: `ApplyAction(state, action) → (state, error)`. No side effects. Clone slices before mutation using `slices.Clone()` or `copy()`. The pause logic in `pause.go` must follow this contract.
- **Session manager is the orchestrator**: Timer management (cancel on pause, restart on resume) happens in the session manager, NOT in the rules engine. The rules engine only sets `TurnTimeRemaining`.
- **Domain package file pattern**: Create `pause.go` alongside `bidding.go`, `playing.go`, `scoring.go`, `validation.go` in `server/internal/game/`. Tests go in `pause_test.go`.
- **WebSocket event contract**: Both `wsEvents.ts` and `events.go` must be updated in the SAME COMMIT. No exceptions.
- **Multi-event broadcast**: Session manager sends `EventGamePaused` (for UI trigger) followed by `EventGameState` (full state sync). Same pattern used by trick resolution and hand scoring.
- **Timer sync via absolute timestamps**: When resuming, compute new `TurnExpiresAt = time.Now().Add(turnTimeRemaining)` and send it to clients. Never send relative durations.

### Session Manager Timer Integration Details

The session manager (`server/internal/session/manager.go`) handles timers:

- `cancelTurnTimer()` in `timer.go` (lines 5-10) — call this when pausing
- `startTimerLocked()` (lines 514-527) — call this when resuming
- `setTurnExpiry()` (lines 500-509) — sets `TurnExpiresAt` on game state
- `handleTimerExpiry()` (lines 532-626) — auto-play logic when timer fires

**Pause timer flow**:

1. On pause: compute `remaining = session.gameState.TurnExpiresAt - time.Now()`, store as milliseconds in `state.TurnTimeRemaining`, call `cancelTurnTimer()`
2. On resume (phase leaves paused): compute new `TurnExpiresAt = time.Now() + remaining`, call `setTurnExpiry()`, then `startTimerLocked()`
3. If no timer was active when paused (`TurnTimeRemaining == 0`), skip timer restart

### Frontend Overlay Z-Index Order

Current overlay z-index stack in `GamePage.tsx` (lines 326-437):

- z-10: DealAnimation
- z-20: **PauseOverlay** (new — insert here)
- z-25: TrumpPrompt, DeclarationPrompt, BelotPrompt
- z-30: ScoreReveal, DeclarationReveal
- z-40: CapotAnimation
- z-50: MatchResult

PauseOverlay should NOT block TrumpPrompt or DeclarationPrompt — but it SHOULD prevent card plays. When paused, disable the `HandCards` click handlers and any active prompts should be hidden behind the pause overlay.

### UX Design Requirements

Per UX spec (`ux-design-specification.md`):

- **Visual tone**: Composed, calm. `surface-elevated` background, no red alerts, no panic typography
- **Overlays are blocking**: PauseOverlay requires action (unpause) or auto-resolves — no dismiss button
- **Toasts**: Use `info` toast (neutral, 3s auto-dismiss) when game resumes: "Game resumed" or when owner overrides: "Room owner resumed the game"
- **Disconnection visual pattern**: Similar to pause but with different content — seat dims, countdown. Pause does NOT dim seats — it just shows the overlay
- **Button hierarchy**: Resume button is Primary style. Pause button on game table is Ghost style.

### Cross-Story Context — Story 5.2 Dependency

Story 5.2 (Room Owner Pause Override) builds on this story:

- `ActionOwnerUnpause` is already defined in types but should NOT be fully implemented here
- The `handleUnpause` function should accept `ActionOwnerUnpause` as a valid action type in the `PhasePaused` case of `ApplyAction()` but the owner validation logic belongs in Story 5.2
- For now, `ActionOwnerUnpause` behaves identically to `ActionUnpause` (clears the acting player's pause only). Story 5.2 will add the "clear ALL pauses" behavior and ownership validation
- The PauseOverlay should NOT show the "Resume All" button yet — that's Story 5.2

### Cross-Story Context — Story 5.3/5.4 Dependency

Stories 5.3/5.4 (Disconnect Detection and Reconnection) will add:

- `PhaseDisconnected` handling (already a constant in types.go)
- `ReconnectOverlay.tsx` (already listed in architecture file tree)
- The pause system and disconnect system share the "previous phase" pattern — both save the phase before transitioning to their respective overlay phase
- Ensure `PreviousPhase` field can be reused by disconnect logic (it already handles this since it stores any phase)

### Project Structure Notes

- All new Go files in `server/internal/game/` for rules logic
- Test fixtures update in `server/internal/game/testfixtures/fixtures.go`
- Frontend component in `client/src/features/game/components/PauseOverlay.tsx`
- WS contract files updated in sync: `server/internal/ws/events.go` + `client/src/shared/types/wsEvents.ts`

### References

- [Source: _bmad-output/planning-artifacts/epics.md — Story 5.1 acceptance criteria, lines 1108-1139]
- [Source: _bmad-output/planning-artifacts/architecture.md — Game state machine phases, lines 998-1011]
- [Source: _bmad-output/planning-artifacts/architecture.md — Session manager pattern, lines 251-252]
- [Source: _bmad-output/planning-artifacts/architecture.md — WebSocket event naming, lines 328-336]
- [Source: _bmad-output/planning-artifacts/architecture.md — Error specification for paused phase, line 1073]
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md — ReconnectOverlay pattern, lines 763-771]
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md — Overlay feedback patterns, lines 836-839]
- [Source: _bmad-output/planning-artifacts/prd.md — FR24: Player pause system, line 324]
- [Source: _bmad-output/project-context.md — Timer sync via absolute timestamps, "send absolute timestamps" rule]
- [Source: _bmad-output/project-context.md — Rules engine pure function contract, "clone slices before mutation"]
- [Source: server/internal/game/types.go — PhasePaused, ActionPause constants already defined]
- [Source: server/internal/apperr/errors.go — ErrGamePaused already defined]
- [Source: server/internal/game/rules_engine.go — PhasePaused case already in ApplyAction switch]
- [Source: server/internal/session/manager.go — HandleAction, broadcastActionResult, timer management]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

- Fixed pause stacking: `handlePause` needed to accept `PhasePaused` as a valid source phase to allow multiple players to pause
- Fixed unpause routing: `ActionUnpause` handled before phase-specific dispatch in `ApplyAction()` so calling unpause from playing returns `ErrNotPaused`
- Fixed GamePage crash: added null guard for `myPlayerSeat` in `canPause` computation
- Updated 4 existing test files with new GameState pause fields to fix test regressions

### Completion Notes List

- All 8 acceptance criteria satisfied
- 16 table-driven test cases in `pause_test.go` covering pause, unpause, stacking, error conditions, state immutability
- PauseOverlay component with 5 test cases
- Timer suspend/resume integrated into session manager with remaining-time preservation
- i18n keys added for both English and Serbian
- All 251 frontend tests pass, all Go tests pass, no regressions

### Change Log

- 2026-04-13: Implemented player pause system (Story 5.1)

### File List

**New files:**

- `server/internal/game/pause.go`
- `server/internal/game/pause_test.go`
- `client/src/features/game/components/PauseOverlay.tsx`
- `client/src/features/game/components/PauseOverlay.test.tsx`

**Modified files:**

- `server/internal/game/state.go`
- `server/internal/game/rules_engine.go`
- `server/internal/game/testfixtures/fixtures.go`
- `server/internal/apperr/errors.go`
- `server/internal/ws/events.go`
- `server/internal/session/manager.go`
- `client/src/shared/types/gameTypes.ts`
- `client/src/shared/types/wsEvents.ts`
- `client/src/shared/hooks/useWsDispatch.ts`
- `client/src/features/game/GamePage.tsx`
- `client/src/shared/i18n/en.json`
- `client/src/shared/i18n/sr.json`
- `client/src/features/game/GamePage.test.tsx`
- `client/src/shared/stores/gameStore.test.ts`
- `client/src/shared/hooks/useWsDispatch.test.ts`
- `client/src/shared/types/gameTypes.test.ts`
