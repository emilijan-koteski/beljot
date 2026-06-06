# Story 3.6: Match Completion and Special Conditions

Status: done

## Story

As a player,
I want the match to end correctly when a team reaches 1001 points, and instant-win to be detected,
So that match outcomes are resolved accurately.

## Acceptance Criteria

1. **Given** a hand has been scored and one team's match total reaches or exceeds 1001 points
   **When** the match-end check runs
   **Then** the phase transitions to `match_end` with the winning team recorded in `WinnerTeam`
   **And** both teams' final scores are preserved in the game state

2. **Given** both teams exceed 1001 points in the same hand
   **When** the match-end check runs
   **Then** the team with the higher score wins; if scores are equal, the taker's team (trump picker) wins
   **And** `WinnerTeam` is set to the winning team index

3. **Given** a player holds all 8 trump cards (7, 8, 9, T, J, Q, K, A of the trump suit) after dealing completes
   **When** hands are evaluated at the start of the bidding phase
   **Then** the instant-win condition is detected, `WinnerTeam` is set to that player's team, and `Phase` transitions to `match_end`

4. **Given** the game is in `match_end` phase
   **When** any player submits a game action (`play_card`, `pick_trump`, `pass_trump`, etc.)
   **Then** `ErrWrongPhase` is returned and state is unchanged

5. **Given** the game is in `paused` phase
   **When** any player submits a game action other than `unpause` or `owner_unpause`
   **Then** `ErrGamePaused` is returned and state is unchanged

6. **Given** test fixtures
   **When** I inspect `testfixtures/`
   **Then** `NewGameNearEnd(teamAScore, teamBScore int)` exists for testing match completion thresholds
   **And** all 7 minimum fixture factory functions from the Architecture spec are present

## Tasks / Subtasks

- [x] Task 1: Add `WinnerTeam *int` field to `GameState` struct (AC: 1, 2, 3)
  - [x] Add field in the Scoring section of `state.go`, after `BelotAnnounced`
  - [x] Add JSON tag `json:"winnerTeam"`
  - [x] Update `cloneGameState` in `bidding.go` to deep-copy the new pointer field
  - [x] Reset `WinnerTeam = nil` in `startNewHand()` in `scoring.go` (defensive — should already be nil if match continues)

- [x] Task 2: Implement tiebreaker logic in `scoreHand()` (AC: 1, 2)
  - [x] Replace simple `>=` check (step 6 in `scoring.go:48-52`) with tiebreaker-aware logic
  - [x] If only one team >= target: that team wins, set `WinnerTeam`
  - [x] If both teams >= target: higher score wins; if tied, the taker's team wins
  - [x] Set `state.WinnerTeam` before setting `state.Phase = PhaseMatchEnd`

- [x] Task 3: Implement instant-win detection (AC: 3)
  - [x] Add `checkInstantWin(state *GameState) *int` function in `scoring.go` — returns winning team index or nil
  - [x] Logic: after dealing, check if any player holds all 8 cards of the trump suit (7-8-9-T-J-Q-K-A)
  - [x] Call from `startNewHand()` after `dealCards()` completes (before returning to `PhaseBidding`)
  - [x] Also call from `NewGame()` in `state.go` for the very first hand
  - [x] If instant-win detected: set `Phase = PhaseMatchEnd`, set `WinnerTeam` to that player's team

- [x] Task 4: Add `PhaseMatchEnd` and `PhasePaused` handling to `ApplyAction` (AC: 4, 5)
  - [x] Add `case PhaseMatchEnd` to the switch in `rules_engine.go` — return `nil, apperr.ErrWrongPhase`
  - [x] Add `case PhasePaused` to the switch in `rules_engine.go` — return `nil, apperr.ErrGamePaused`

- [x] Task 5: Create `NewGameNearEnd` test fixture (AC: 6)
  - [x] Add `NewGameNearEnd(teamAScore, teamBScore int)` to `testfixtures/fixtures.go`
  - [x] Returns a `GameState` in `PhasePlaying` at trick 8 (like `NewGameLastTrick`)
  - [x] Sets `TeamScores[TeamA] = teamAScore`, `TeamScores[TeamB] = teamBScore`
  - [x] Use deterministic card distribution matching existing fixture patterns
  - [x] Validate in `fixtures_test.go`

- [x] Task 6: Write comprehensive tests (AC: 1-6)
  - [x] `scoring_test.go` — match-end tiebreaker tests:
    - TestMatchEnd_SingleTeamReaches1001
    - TestMatchEnd_BothTeamsExceed1001_HigherScoreWins
    - TestMatchEnd_BothTeamsExceed1001_TiedScore_ContractingTeamWins
    - TestMatchEnd_WinnerTeamFieldSet
    - TestMatchEnd_501Mode
  - [x] `scoring_test.go` and `scoring_internal_test.go` — instant-win tests:
    - TestCheckInstantWin_AllTrumpCards (internal)
    - TestCheckInstantWin_NoInstantWin (internal)
    - TestCheckInstantWin_NilTrumpCandidate (internal)
    - TestCheckInstantWin_Seat0TeamA (internal)
    - TestInstantWin_NotTriggered_PartialTrump
    - TestInstantWin_FirstHand (through `NewGame`)
  - [x] `rules_engine_test.go` — phase error tests:
    - TestApplyAction_MatchEndPhase_ReturnsErrWrongPhase
    - TestApplyAction_PausedPhase_ReturnsErrGamePaused
  - [x] Verify all existing tests still pass

- [x] Task 7: Run `go test ./server/internal/game/...` and `go vet ./server/internal/game/...` (AC: all)
  - [x] Ensure 0 failures, 0 vet warnings
  - [x] Verify >90% coverage on `internal/game/` (achieved: 94.6%)

### Review Findings

- [x] [Review][Patch] Missing instant-win check in `reshuffleAndRedeal()` after re-deal [server/internal/game/bidding.go:109] — FIXED: added `checkInstantWin()` call after `dealCards()` in `reshuffleAndRedeal()`
- [x] [Review][Defer] Nil TrickWinnerSeat dereference in `scoreHand()` [server/internal/game/scoring.go:9] — deferred, pre-existing from Story 3.5; safe through current call chain (resolveTrick always sets it at trick 8)

## Dev Notes

### Architecture Compliance

- **Pure function rules engine**: All logic flows through `ApplyAction(state, action) → (state, error)`. No side effects (no I/O, no broadcasts, no DB writes) inside the engine.
- **State cloning**: `handlePlayCard()` in `playing.go` already calls `cloneGameState(state)` at the top. All scoring mutations (including match-end) operate on the cloned state. Do NOT add additional cloning in `scoreHand`.
- **Automatic phases**: `PhaseHandScoring` is automatic (no player actions), processed atomically within the same `ApplyAction` call triggered by the 4th card of the 8th trick. The returned state lands directly in a stable phase (`PhaseBidding` or `PhaseMatchEnd`). Do NOT create a separate `ApplyAction` handler for `PhaseHandScoring`.
- **Do NOT modify `ApplyAction`'s switch for `PhaseHandScoring`** — hand scoring is triggered automatically within `handlePlaying` → `resolveTrickWithDeclarations` → `scoreHand`. Only add `PhaseMatchEnd` and `PhasePaused` cases.
- **Randomness accepted**: `startNewHand` uses `ShuffleDeck` (random). This is the accepted pattern, same as `NewGame()` and `reshuffleAndRedeal()`.

### Key Integration Point — scoreHand Step 6 Modification

The current match-end check in `scoring.go:48-52` is:

```go
target := matchTarget(state.MatchMode)
if state.TeamScores[TeamA] >= target || state.TeamScores[TeamB] >= target {
    state.Phase = PhaseMatchEnd
    return
}
```

Replace this with tiebreaker-aware logic:

```go
target := matchTarget(state.MatchMode)
teamAOver := state.TeamScores[TeamA] >= target
teamBOver := state.TeamScores[TeamB] >= target

if teamAOver || teamBOver {
    winner := determineMatchWinner(state, teamAOver, teamBOver)
    state.WinnerTeam = &winner
    state.Phase = PhaseMatchEnd
    return
}
```

Where `determineMatchWinner` implements:

1. If only one team crossed → that team
2. If both crossed → team with higher `TeamScores`
3. If both crossed AND tied → `TeamForSeat(*state.TrumpCallerSeat)` (the taker's team)

### Key Integration Point — Instant-Win in startNewHand

After `dealCards(state, deck)` in `startNewHand()`, before setting `Phase = PhaseBidding`:

```go
if winnerTeam := checkInstantWin(state); winnerTeam != nil {
    state.WinnerTeam = winnerTeam
    state.Phase = PhaseMatchEnd
    return
}
```

Also add the same check at the end of `NewGame()` in `state.go`, after `dealCards(gs, deck)`:

```go
if winnerTeam := checkInstantWin(gs); winnerTeam != nil {
    gs.WinnerTeam = winnerTeam
    gs.Phase = PhaseMatchEnd
}
```

### Instant-Win Detection Logic

`checkInstantWin` checks if any player holds all 8 cards of the trump suit. The trump suit is determined by `TrumpCandidate.Suit` (set during dealing):

```go
func checkInstantWin(state *GameState) *int {
    if state.TrumpCandidate == nil {
        return nil
    }
    trumpSuit := state.TrumpCandidate.Suit
    for i := range state.Players {
        trumpCount := 0
        for _, card := range state.Players[i].Hand {
            if card.Suit == trumpSuit {
                trumpCount++
            }
        }
        if trumpCount == 8 {
            team := TeamForSeat(state.Players[i].Seat)
            return &team
        }
    }
    return nil
}
```

Note: Since the standard deck has exactly 8 cards per suit, holding 8 cards of the trump suit means holding ALL trump cards. The check is simply count == 8.

### Existing Code to Reuse — DO NOT Reinvent

| Function / Constant            | Location           | Purpose                                        |
| ------------------------------ | ------------------ | ---------------------------------------------- |
| `TeamForSeat(seat int) int`    | `state.go`         | Maps seat to team index (seat % 2)             |
| `TeamA = 0`, `TeamB = 1`       | `state.go`         | Team index constants                           |
| `matchTarget(mode string) int` | `scoring.go`       | Returns 1001 or 501 based on MatchMode         |
| `cloneGameState(state)`        | `bidding.go`       | Deep clones GameState including pointer fields |
| `PhaseMatchEnd`, `PhasePaused` | `types.go`         | Phase constants (already defined)              |
| `apperr.ErrWrongPhase`         | `apperr/errors.go` | Error for invalid phase actions                |
| `apperr.ErrGamePaused`         | `apperr/errors.go` | Error for paused game actions                  |
| All error sentinels            | `apperr/errors.go` | `ErrWrongPhase`, `ErrNotYourTurn`, etc.        |

### Files to Create

| File | Purpose                            |
| ---- | ---------------------------------- |
| None | All logic goes into existing files |

### Files to Modify

| File                                                 | Changes                                                                                                                                                                                             |
| ---------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `server/internal/game/state.go`                      | Add `WinnerTeam *int` field to `GameState` struct (Scoring section). Add instant-win check after `dealCards` in `NewGame()`                                                                         |
| `server/internal/game/scoring.go`                    | Replace match-end check with tiebreaker logic. Add `determineMatchWinner` helper. Add `checkInstantWin` function. Add instant-win check in `startNewHand()`. Reset `WinnerTeam` in `startNewHand()` |
| `server/internal/game/rules_engine.go`               | Add `case PhaseMatchEnd` → `ErrWrongPhase` and `case PhasePaused` → `ErrGamePaused` to switch                                                                                                       |
| `server/internal/game/bidding.go`                    | Add `WinnerTeam` deep-copy to `cloneGameState`                                                                                                                                                      |
| `server/internal/game/scoring_test.go`               | Add tiebreaker tests, instant-win tests, WinnerTeam field tests                                                                                                                                     |
| `server/internal/game/rules_engine_test.go`          | Add PhaseMatchEnd and PhasePaused error tests                                                                                                                                                       |
| `server/internal/game/testfixtures/fixtures.go`      | Add `NewGameNearEnd(teamAScore, teamBScore int)`                                                                                                                                                    |
| `server/internal/game/testfixtures/fixtures_test.go` | Add validation test for `NewGameNearEnd`                                                                                                                                                            |

### Testing Patterns — MANDATORY

- **All tests through `ApplyAction`** — never call `scoreHand`, `checkInstantWin`, or `determineMatchWinner` directly
- **Use `testfixtures` factory functions** exclusively — no raw `GameState{}` struct literals
- **Table-driven tests**: `[]struct{ name string; ... }` with `t.Run`
- **External test package**: `package game_test`
- **Imports**: `game "github.com/emilijan/beljot/server/internal/game"` and `testfixtures "github.com/emilijan/beljot/server/internal/game/testfixtures"`
- **Assertions**: `testify/assert`
- **Test instant-win through `NewGame`**: Create a test that constructs specific player IDs, then verify the returned state has `Phase == PhaseMatchEnd` — but since `NewGame` shuffles randomly, instant-win tests for `startNewHand` should use `NewGameNearEnd` with scores just below target, play out the hand to trigger `startNewHand`, then verify the new hand's state
- **For instant-win unit testing**: Consider adding a deterministic variant of the fixture or testing `checkInstantWin` through a carefully set up `NewGameNearEnd` where the next hand triggers the condition

### Testing Approach for Instant-Win

Since `startNewHand` and `NewGame` use random shuffles, you cannot deterministically test instant-win through the normal flow. Options:

1. **Test `checkInstantWin` directly** (exception to the "only through ApplyAction" rule) — this is a pure function with no side effects, making direct testing acceptable for this edge case
2. **Create `NewGameInstantWin(trumpSuit Suit)` fixture** that returns a state where one player holds all 8 cards of the trump suit, already in `PhaseBidding`, then verify the function detects it
3. The fixture approach is preferred — set up a state just before the check would run

### Previous Story Intelligence (from 3.5)

**Critical learnings to apply:**

- `TurnExpiresAt` must be reset in `startNewHand` — already done in 3.5, but verify `WinnerTeam` follows the same nil-reset pattern
- Test edge cases with multiple point sources (declarations + card points + bonuses)
- When changing phase transitions, update dependent tests — check if any existing tests assert `Phase == PhaseMatchEnd` without checking `WinnerTeam`
- Existing `TestHandScoring_MatchEndTriggered` in `scoring_test.go` already tests basic match-end — update it to verify `WinnerTeam` is set

**Deferred items from 3.5 — DO NOT address in 3.6:**

- D35: `legalCards` nil-guard dereferences (pre-existing, session manager validates)
- D36: Negative seat modulo (pre-existing)
- D37: `action.PlayerSeat` bounds check (Epic 4)
- `scoreHand` nil-guard for `TrickWinnerSeat` and `TrumpCallerSeat` (pre-existing pattern)

### Git Intelligence (from recent commits)

Recent commit pattern: `feat(game): implement <feature> with code review fixes`

- Each story ships as a single commit with code review fixes included
- All game engine stories (3.1-3.5) follow this pattern
- Backend tests run via `go test ./server/internal/game/...`
- Frontend tests run via `npx vitest run` (109/109 tests pass as of 3.5)
- Linting via `go vet ./server/internal/game/...`

### Project Structure Notes

- All game engine code in `server/internal/game/`
- Test fixtures in `server/internal/game/testfixtures/`
- Centralized errors in `server/internal/apperr/errors.go`
- This story is **backend only** — no frontend changes, no WebSocket events, no UI
- Epic 4 will handle WebSocket broadcasting of `event:match_end` and frontend `gameStore` lifecycle

### References

- [Source: _bmad-output/planning-artifacts/epics.md, Epic 3, Story 3.6, lines 826-862]
- [Source: _bmad-output/planning-artifacts/architecture.md — Game state machine, scoring sub-system, test fixtures]
- [Source: _bmad-output/planning-artifacts/prd.md — FR13 (instant-win), FR14 (1001-point match mode)]
- [Source: _bmad-output/implementation-artifacts/3-5-hand-scoring-failed-hands-and-capot.md — scoreHand implementation, startNewHand, match-end check pattern]
- [Source: server/internal/game/scoring.go — Current match-end check at lines 48-52]
- [Source: server/internal/game/rules_engine.go — Current ApplyAction switch with only PhaseBidding and PhasePlaying]
- [Source: server/internal/game/state.go — GameState struct, no WinnerTeam field yet]
- [Source: server/internal/apperr/errors.go — ErrWrongPhase, ErrGamePaused already defined]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

None — all tests passed on first run after implementation.

### Completion Notes List

- Added `WinnerTeam *int` field to `GameState` struct in Scoring section with `json:"winnerTeam"` tag
- Updated `cloneGameState` to deep-copy `WinnerTeam` pointer field
- Replaced simple `>= target` match-end check in `scoreHand()` with tiebreaker-aware logic via `determineMatchWinner` helper
- Tiebreaker rules: single team over → that team wins; both over → higher score; tied scores → the taker's team
- Implemented `checkInstantWin()` — detects player holding all 8 trump cards, returns winning team
- Integrated instant-win check into both `startNewHand()` and `NewGame()` after dealing
- Added `PhaseMatchEnd` → `ErrWrongPhase` and `PhasePaused` → `ErrGamePaused` cases to `ApplyAction`
- Created `NewGameNearEnd(teamAScore, teamBScore int)` fixture delegating to `NewGameLastTrick` with configurable scores
- Added `TestMatchEnd_TeamBWins` as additional coverage for Team B winning scenario
- Updated 3 existing tests to verify `WinnerTeam` field: `TestHandScoring_MatchEndTriggered`, `TestHandScoring_NewHandStateReset`, `TestHandScoring_501MatchMode`
- Total new tests: 18 (6 tiebreaker + 6 instant-win + 3 phase error + 2 fixture validation + 1 additional)
- Test coverage: 94.6% on `internal/game/` (threshold: >90%)
- Full regression: backend all packages OK, frontend 109/109 pass, go vet clean

### Change Log

- 2026-04-12: Implemented match completion with tiebreaker logic, instant-win detection, and phase error handling (Story 3.6)

### File List

- `server/internal/game/state.go` — Added `WinnerTeam *int` field, instant-win check in `NewGame()`
- `server/internal/game/scoring.go` — Added `checkInstantWin()`, `determineMatchWinner()`, tiebreaker logic in `scoreHand()`, instant-win check in `startNewHand()`, `WinnerTeam` reset in `startNewHand()`
- `server/internal/game/rules_engine.go` — Added `PhaseMatchEnd` and `PhasePaused` cases to `ApplyAction`
- `server/internal/game/bidding.go` — Added `WinnerTeam` deep-copy to `cloneGameState`
- `server/internal/game/scoring_test.go` — Added 8 tiebreaker/match-end tests, 2 instant-win tests, updated 3 existing tests
- `server/internal/game/scoring_internal_test.go` — NEW: 4 internal tests for `checkInstantWin` with deterministic fixtures
- `server/internal/game/rules_engine_test.go` — Added `TestApplyAction_MatchEndPhase_ReturnsErrWrongPhase`, `TestApplyAction_PausedPhase_ReturnsErrGamePaused`, updated existing test
- `server/internal/game/testfixtures/fixtures.go` — Added `NewGameNearEnd(teamAScore, teamBScore int)`
- `server/internal/game/testfixtures/fixtures_test.go` — Added `TestNewGameNearEnd` validation
