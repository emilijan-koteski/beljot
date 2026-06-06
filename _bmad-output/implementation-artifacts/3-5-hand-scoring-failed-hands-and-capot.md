# Story 3.5: Hand Scoring, Failed Hands & Capot

Status: done

## Story

As a player,
I want hand scoring to correctly calculate points including failed hands and Capot,
So that the competitive integrity of the game is maintained.

## Acceptance Criteria

1. **Given** all 8 tricks in a hand have been resolved
   **When** hand scoring is calculated
   **Then** each team's hand score includes: sum of card points from won tricks + declaration points (if applicable)
   **And** the team winning the last trick receives +10 bonus points
   **And** the phase transitions to `hand_scoring`

2. **Given** one team wins all 8 tricks (Capot)
   **When** hand scoring is calculated
   **Then** the winning team receives +100 bonus points (replacing the +10 last-trick bonus, not in addition to it)

3. **Given** the team that picked trump (the taker's team) scores fewer points than, or exactly ties, the opposing team
   **When** failed hand logic is applied
   **Then** the taker's team scores 0 points for the hand and ALL points (both teams' card points + declarations + bonuses) are awarded to the opposing team

4. **Given** the team that picked trump scores **strictly more** points than the opposing team
   **When** normal scoring is applied
   **Then** each team keeps their own card points + declaration points + applicable bonuses

5. **Given** hand scoring is complete
   **When** scores are added to the match total
   **Then** the match score updates for both teams and the phase transitions to `bidding` (next hand dealt and ready for trump selection) or `match_end` (if a team has reached the target)
   **Note:** The architecture defines `dealing` and `hand_scoring` as automatic phases with no player actions. In the implementation, both are processed atomically within the same `ApplyAction` call — the returned state lands directly in `PhaseBidding` (or `PhaseMatchEnd`). `PhaseDealing` has no handler in `ApplyAction` and must NOT be set as a stable phase.

6. **Given** test fixtures
   **When** I inspect `testfixtures/`
   **Then** `NewGameLastTrick()` exists, returning a game state at the 8th trick for testing hand-end scoring
   **And** `NewGameCapotInProgress()` exists, returning a state where one team has won all tricks so far

## Tasks / Subtasks

- [x] Task 1: Create `scoring.go` with hand scoring logic (AC: #1, #2, #3, #4)
  - [x] 1.1: Create `server/internal/game/scoring.go`
  - [x] 1.2: Implement `scoreHand(state *GameState)` — the orchestrator function that applies all scoring steps in order. Mutates an already-cloned state (called from within `resolveTrickWithDeclarations`)
  - [x] 1.3: Implement last-trick bonus: add +10 to `HandPoints[TeamForSeat(*state.TrickWinnerSeat)]` — the team that won trick 8 gets the bonus
  - [x] 1.4: Implement Capot detection: if `TricksWon[team] == 8` for either team, award +100 to that team's `HandPoints` **instead of** the +10 last-trick bonus (Capot replaces, does not stack)
  - [x] 1.5: Calculate each team's total hand score: `teamTotal = HandPoints[team] + DeclarationPoints[team]`
  - [x] 1.6: Implement failed hand check: identify the taker's team via `TeamForSeat(*state.TrumpCallerSeat)`. If `contractingTeamTotal <= opposingTeamTotal` (a tie counts as failure), the taker's team gets 0 and opponent's `TeamScores` increases by `contractingTeamTotal + opposingTeamTotal` (ALL points). If the taker's team total is strictly greater than the opposing team total, each team's `TeamScores` increases by their own total (normal scoring)
  - [x] 1.7: Implement match-end check: determine target from `MatchMode` ("501" → 501, default → 1001). If either team's `TeamScores >= target`, set `Phase = PhaseMatchEnd` and return. Story 3.6 will refine this with tiebreaker logic for simultaneous threshold crossing
  - [x] 1.8: If match not over, call `startNewHand(state)` to set up the next hand

- [x] Task 2: Implement `startNewHand` for new hand setup (AC: #5)
  - [x] 2.1: In `scoring.go`, implement `startNewHand(state *GameState)` that fully resets all per-hand state for a new deal
  - [x] 2.2: Increment `HandNumber`, rotate dealer: `DealerSeat = (DealerSeat + 1) % 4`
  - [x] 2.3: Reset bidding state: `TrumpSuit = nil`, `TrumpCallerSeat = nil`, `TrumpCandidate = nil`, `BiddingRound = 1`, `BiddingPassCount = 0`
  - [x] 2.4: Reset trick state: `TrickNumber = 0`, `CurrentTrick = []TrickCard{}`, `LeadSuit = nil`, `TrickWinnerSeat = nil`
  - [x] 2.5: Reset declaration/Belote state (fields from Story 3.4 that must be reset between hands): `AwaitingDeclaration = false`, `DeclarationsResolved = false`, `PendingBelotSeat = nil`, `BelotAnnounced = false`
  - [x] 2.6: Reset per-hand scoring: `HandPoints = [2]int{0, 0}`, `DeclarationPoints = [2]int{0, 0}`, `TricksWon = [2]int{0, 0}`
  - [x] 2.7: Clear player state: `Players[i].Hand = []Card{}`, `Players[i].Declarations = nil` for all 4 players
  - [x] 2.8: Generate fresh deck via `NewDeck()`, shuffle via `ShuffleDeck()`, deal via existing `dealCards()` function
  - [x] 2.9: Set `ActivePlayerSeat = (DealerSeat + 1) % 4`, `Phase = PhaseBidding`

- [x] Task 3: Integrate scoring into game flow (AC: all)
  - [x] 3.1: In `declarations.go`, inside `resolveTrickWithDeclarations`, add a call to `scoreHand(state)` after the existing `PhaseHandScoring` declaration edge-case check. Guard with `if state.Phase == PhaseHandScoring`. This guard is safe because `resolveTrick` only sets `PhaseHandScoring` when `TrickNumber == 8` — it cannot fire on non-trick-8 resolutions. This follows the same pattern as trick resolution (automatic phases processed within the same `ApplyAction` call)
  - [x] 3.2: Verify the call order: `resolveTrick()` → `resolveDeclarationsForHand()` (if needed) → `scoreHand()` — scoring must run AFTER declarations are resolved since it reads `DeclarationPoints`

- [x] Task 4: Create test fixtures (AC: #6)
      **Note:** `NewGameMidPlay(8)` already provides TrickNumber=8 with 1 card per player, but it has arbitrary HandPoints/TricksWon and no controlled TrumpCallerSeat for failed-hand testing. The new fixtures below provide deterministic scoring state needed for hand-scoring tests.
  - [x] 4.1: Add `NewGameLastTrick() *GameState` to `testfixtures/fixtures.go`
  - [x] 4.2: Add `NewGameCapotInProgress() *GameState` to `testfixtures/fixtures.go`
  - [x] 4.3: Write fixture validation tests in `testfixtures/fixtures_test.go`

- [x] Task 5: Write hand scoring tests in `scoring_test.go` (AC: #1-#5)
  - [x] 5.1: Create `server/internal/game/scoring_test.go` with `package game_test`
  - [x] 5.2: Test last-trick bonus (TestHandScoring_LastTrickBonus)
  - [x] 5.3: Test Capot scoring (TestHandScoring_CapotScoring)
  - [x] 5.4: Test Capot broken (TestHandScoring_CapotBroken)
  - [x] 5.5: Test failed hand (TestHandScoring_FailedContract)
  - [x] 5.6: Test equal points is failure (TestHandScoring_EqualPointsIsFailure) and explicit 81:81 tie (TestHandScoring_TieFailsTrumpCaller)
  - [x] 5.7: Test normal scoring (TestHandScoring_NormalScoring)
  - [x] 5.8: Test match end triggered (TestHandScoring_MatchEndTriggered)
  - [x] 5.9: Test match continues (TestHandScoring_MatchContinues)
  - [x] 5.10: Test new hand state reset (TestHandScoring_NewHandStateReset)
  - [x] 5.11: Test declarations included (TestHandScoring_DeclarationsIncluded)
  - [x] 5.12: Test Belote bonus included (TestHandScoring_BelotBonusIncluded)
  - [x] 5.13: Test 501 match mode (TestHandScoring_501MatchMode)
  - [x] 5.14: Test state immutability (TestHandScoring_StateImmutability)

- [x] Task 6: Verify full regression suite passes
  - [x] 6.1: Run `go test ./server/internal/game/...` — all packages OK (0 failures)
  - [x] 6.2: Run `npx vitest run` — 109/109 frontend tests pass
  - [x] 6.3: Run `go vet ./internal/game/...` — clean (make lint not available in this environment)

## Dev Notes

### Architecture Requirements

**Pure Function Contract:** `scoreHand` and `startNewHand` are internal helper functions called within the `resolveTrickWithDeclarations` flow, which is called within `handlePlayCard`, which is called within `ApplyAction`. The entire chain remains a pure function from the caller's perspective — `ApplyAction(state, action) → (newState, error)`. The `scoreHand` function mutates an already-cloned state (cloned at the top of `handlePlayCard`), so no side effects leak.

**Exception: `startNewHand` uses randomness** (shuffle). This is the same pattern as `NewGame()` in `state.go` and `reshuffleAndRedeal()` in `bidding.go`. Randomness in card dealing is accepted within the game package. The "pure function" contract means no I/O, no broadcasting, no DB writes — not determinism.

**Phase Flow:** The `PhaseHandScoring` phase is automatic (no player actions). Following the precedent of `PhaseTrickResolving` (also automatic), it is processed immediately within the same `ApplyAction` call. The returned state will be either `PhaseBidding` (new hand) or `PhaseMatchEnd` (game over). The session manager (Epic 4) will detect the phase transitions from the returned state to broadcast score reveals and new hand events.

**Do NOT modify `ApplyAction` in `rules_engine.go`** — hand scoring is triggered automatically within the playing phase flow, not as a new phase dispatch case.

### Scoring Rules — Complete Specification

**Total Card Points:** A full 32-card Belot deck contains exactly **152 card points** (verified by existing test in `playing_test.go:783`). With the last-trick bonus, the baseline distributable points per hand = 162. With Capot, it's 252.

**Card Points Per Suit:**

- Trump suit: J(20) + 9(14) + A(11) + T(10) + K(4) + Q(3) + 8(0) + 7(0) = 62
- Non-trump suit: A(11) + T(10) + K(4) + Q(3) + J(2) + 9(0) + 8(0) + 7(0) = 30
- Total: 62 + 3×30 = 152

**Hand Score Components (per team):**

| Component               | Source                            | Where Stored                                                  |
| ----------------------- | --------------------------------- | ------------------------------------------------------------- |
| Card points from tricks | Sum of card values in won tricks  | `HandPoints[team]` (set by `resolveTrick` in playing.go)      |
| Belote bonus            | +20 if announced during hand      | `HandPoints[team]` (set by Story 3.4 in declarations.go)      |
| Last-trick bonus        | +10 to team winning trick 8       | `HandPoints[team]` (set by `scoreHand` — this story)          |
| Capot bonus             | +100 if one team won all 8 tricks | `HandPoints[team]` (set by `scoreHand` — replaces last-trick) |
| Declaration points      | From trick 1 resolution           | `DeclarationPoints[team]` (set by Story 3.4)                  |

**Team Total:** `HandPoints[team] + DeclarationPoints[team]`

**Scoring Steps (in order):**

1. Add last-trick bonus (+10) OR Capot bonus (+100) to `HandPoints`
2. Calculate each team's total: `teamTotal = HandPoints[team] + DeclarationPoints[team]`
3. Check failed hand: is the taker's team's total **less than or equal to** opponent's total (i.e., NOT strictly more)?
4. If failed: `TeamScores[opponent] += contractingTotal + opponentTotal` (opponent gets ALL)
5. If not failed: `TeamScores[team] += teamTotal` for each team
6. Check match-end condition

**Failed hand — Precise Rule:**

- The the taker's team = the team of the player who picked trump: `TeamForSeat(*state.TrumpCallerSeat)`
- **Less than or equal** triggers failure: `contractingTotal <= opposingTotal`
- **A tie IS a failure** — the taker's team must score strictly more than the opponents to succeed (e.g. 81:81 → failed for the caller)
- On failure: the taker's team adds **0** to `TeamScores`, opponent adds the **sum of both totals**

**Capot Rule:**

- Capot = one team won all 8 tricks: `TricksWon[team] == 8`
- Capot bonus is +100 to `HandPoints`, **replacing** the +10 last-trick bonus (not additive)
- The last-trick bonus and Capot bonus are mutually exclusive — check Capot first, only apply +10 if not Capot

**Match-End Check:**

- Target: 1001 for MatchMode "1001", 501 for MatchMode "501"
- If either team's `TeamScores >= target` → `Phase = PhaseMatchEnd`
- Story 3.6 will add tiebreaker logic (both teams cross threshold simultaneously → the taker's team wins)
- For this story, the simple check is sufficient — just detect if any team crossed the target

### `scoreHand` Function Specification

```
scoreHand(state *GameState):
  1. lastTrickTeam = TeamForSeat(*state.TrickWinnerSeat)
  2. IF TricksWon[0] == 8 OR TricksWon[1] == 8:
       capotTeam = team with 8 tricks
       HandPoints[capotTeam] += 100    // Capot bonus
     ELSE:
       HandPoints[lastTrickTeam] += 10  // Last-trick bonus
  3. teamATotal = HandPoints[TeamA] + DeclarationPoints[TeamA]
     teamBTotal = HandPoints[TeamB] + DeclarationPoints[TeamB]
  4. contractingTeam = TeamForSeat(*state.TrumpCallerSeat)
     opposingTeam = 1 - contractingTeam
     contractingTotal = IF contractingTeam == TeamA THEN teamATotal ELSE teamBTotal
     opposingTotal = IF opposingTeam == TeamA THEN teamATotal ELSE teamBTotal
  5. IF contractingTotal <= opposingTotal:    // a tie counts as failure
       TeamScores[opposingTeam] += teamATotal + teamBTotal  // ALL points to opponent
       // contractingTeam gets 0 — no addition to TeamScores
     ELSE:
       TeamScores[TeamA] += teamATotal
       TeamScores[TeamB] += teamBTotal
  6. target = matchTarget(state.MatchMode)
     IF TeamScores[0] >= target OR TeamScores[1] >= target:
       Phase = PhaseMatchEnd
       RETURN
  7. startNewHand(state)
```

### `startNewHand` Function Specification

This resets ALL per-hand state. Reference: `reshuffleAndRedeal()` in `bidding.go` for the pattern, but this function resets more fields (scoring, declarations, Belote).

**Fields to reset:**

| Field                   | Reset Value                        | Section                  |
| ----------------------- | ---------------------------------- | ------------------------ |
| HandNumber              | increment by 1                     | Hand state               |
| DealerSeat              | `(DealerSeat + 1) % 4`             | Hand state               |
| TrumpSuit               | `nil`                              | Hand state               |
| TrumpCallerSeat         | `nil`                              | Hand state               |
| TrumpCandidate          | `nil` (set by dealCards)           | Hand state               |
| BiddingRound            | `1`                                | Hand state               |
| BiddingPassCount        | `0`                                | Hand state               |
| TrickNumber             | `0` (becomes 1 in handlePickTrump) | Trick state              |
| CurrentTrick            | `[]TrickCard{}`                    | Trick state              |
| LeadSuit                | `nil`                              | Trick state              |
| TrickWinnerSeat         | `nil`                              | Trick state              |
| AwaitingDeclaration     | `false`                            | Trick state (Story 3.4)  |
| DeclarationsResolved    | `false`                            | Trick state (Story 3.4)  |
| HandPoints              | `[2]int{0, 0}`                     | Scoring                  |
| DeclarationPoints       | `[2]int{0, 0}`                     | Scoring                  |
| TricksWon               | `[2]int{0, 0}`                     | Scoring                  |
| PendingBelotSeat        | `nil`                              | Scoring (Story 3.4)      |
| BelotAnnounced          | `false`                            | Scoring (Story 3.4)      |
| Players[i].Hand         | `[]Card{}` (refilled by dealCards) | Player state             |
| Players[i].Declarations | `nil`                              | Player state (Story 3.4) |
| ActivePlayerSeat        | `(DealerSeat + 1) % 4`             | Timer state              |
| Phase                   | `PhaseBidding`                     | Metadata                 |

**Do NOT reset:** `TeamScores` (cumulative across hands), `ID`, `RoomID`, `Variant`, `MatchMode`, `Players[i].Seat`, `Players[i].UserID`, `Players[i].Team`, `Players[i].Connected`

### Integration Point

The only code modification outside `scoring.go` is in `declarations.go`:

```go
// In resolveTrickWithDeclarations, after the existing edge case check:
if state.Phase == PhaseHandScoring {
    scoreHand(state)
}
```

This must come AFTER the `resolveDeclarationsForHand` call (which is already guarded by the same `PhaseHandScoring` check) so that `DeclarationPoints` are set before `scoreHand` reads them. The existing code structure already ensures this ordering.

### Existing Code to Reuse

**DO NOT DUPLICATE — reuse these existing functions and types:**

- `TeamForSeat(seat int) int` in `state.go` — 0=Team A (seats 0,2), 1=Team B (seats 1,3)
- `TeamA = 0`, `TeamB = 1` constants in `state.go`
- `NewDeck() []Card` in `types.go` — generates fresh 32-card deck
- `ShuffleDeck(deck []Card)` in `state.go` — in-place shuffle
- `dealCards(gs *GameState, deck []Card)` in `state.go` — deals using 3+2+3 Bitola sequence
- `TrumpCardPoints`, `NonTrumpCardPoints` maps in `types.go` — for card point reference in tests
- `cloneGameState(state *GameState) *GameState` in `bidding.go` — already called by `handlePlayCard` before any mutation
- All error sentinels in `apperr/errors.go`
- `PhaseHandScoring`, `PhaseMatchEnd`, `PhaseBidding`, `PhasePlaying` constants in `types.go`

### Forward Compatibility — Story 3.6 (Match Completion)

Story 3.6 will:

1. Refine the match-end check with tiebreaker logic: if both teams cross 1001 in the same hand, the team with the higher score wins; if tied, the taker's team wins
2. Add instant-win detection: player holds all 8 trump in sequence → immediate match end after dealing
3. Add `ErrWrongPhase` handling for `PhaseMatchEnd` and `ErrGamePaused` for `PhasePaused`
4. Potentially add a `WinnerTeam *int` field to `GameState` for PhaseMatchEnd

For this story: implement the basic match-end check (`TeamScores >= target → PhaseMatchEnd`). Story 3.6 will wrap or replace this check with the tiebreaker-aware version.

### Existing Code Patterns to Follow

**Test pattern** (from `declarations_test.go`, `playing_test.go`):

- Package: `package game_test` (external test package)
- Import: `game "github.com/emilijan/beljot/server/internal/game"` and `testfixtures "github.com/emilijan/beljot/server/internal/game/testfixtures"`
- Use table-driven tests: `[]struct{ name string; ... }` with `t.Run`
- Test through `game.ApplyAction` — never call internal functions directly
- Use `testfixtures` factory functions exclusively — no raw `GameState{}` struct literals
- Use `testify/assert` for assertions

**Fixture pattern** (from `testfixtures/fixtures.go`):

- Return `*game.GameState` pointer
- Set all fields explicitly — no reliance on zero values for fields that should be a specific value
- Design hands carefully so all 32 cards are accounted for (8 per player for full hands, or 1 per player for last trick)
- Include comments explaining what each player's hand contains and why

### Deferred Items from Story 3.4

These items documented in Story 3.4 are now relevant to this story:

- **State reset between hands**: Story 3.4 documented which fields must be reset. This story implements that reset in `startNewHand`. All fields listed in the "State Reset Between Hands" section of Story 3.4 MUST be reset
- **`BelotAnnounced`/`DeclarationsResolved`/`DeclarationPoints` not reset between hands**: This was explicitly deferred to Story 3.5. Implement the reset
- **`TrumpSuit` nil dereference in declaration functions**: Pre-existing pattern, still deferred. `TrumpCallerSeat` nil dereference in `scoreHand` is safe because `PhaseHandScoring` is only reached after trump is picked

### Deferred Review Items (Carry Forward)

These items remain deferred and are NOT addressed by this story:

- `legalCards` nil-guard on `TrumpSuit`/`LeadSuit` — safe through normal call paths (Story 3.3)
- `currentTrickWinnerSeat` returns -1 for empty trick — unreachable (Story 3.3)
- No bounds check on `action.PlayerSeat` — deferred to session manager (Epic 4)
- `checkDeclarationPrompt` cannot distinguish skip from never-asked — safe in current flow (Story 3.4)

### Project Structure Notes

**New files:**

```
server/internal/game/
  scoring.go          -- NEW: scoreHand, startNewHand, matchTarget helper
  scoring_test.go     -- NEW: hand scoring tests through ApplyAction
```

**Modified files:**

```
server/internal/game/
  declarations.go     -- MODIFIED: Add scoreHand call in resolveTrickWithDeclarations
  testfixtures/
    fixtures.go       -- MODIFIED: Add NewGameLastTrick(), NewGameCapotInProgress()
    fixtures_test.go  -- MODIFIED: Add fixture validation tests
```

**Unchanged files:**

```
  rules_engine.go     -- UNCHANGED (no new phase case needed)
  playing.go          -- UNCHANGED (resolveTrick already transitions to PhaseHandScoring)
  state.go            -- UNCHANGED (no new fields needed)
  types.go            -- UNCHANGED
  bidding.go          -- UNCHANGED
  validation.go       -- UNCHANGED
```

**Error file:**

```
server/internal/apperr/
  errors.go           -- UNCHANGED (no new error sentinels needed for scoring)
```

### Testing Approach

- All tests go through `ApplyAction` — test the public interface, not `scoreHand` directly
- Use `testfixtures/` factory functions exclusively — no raw `GameState{}` struct literals
- Use Go table-driven test pattern (`[]struct{ name string; ... }` with `t.Run`)
- Tests co-located: `scoring_test.go` next to `scoring.go`
- Package: `package game_test` (external test package)
- Import testfixtures via `"github.com/emilijan/beljot/server/internal/game/testfixtures"`
- To test hand scoring, play the 8th trick by calling `ApplyAction` with `play_card` actions on a `NewGameLastTrick()` or `NewGameCapotInProgress()` state
- After the 4th card of trick 8, the returned state should reflect completed scoring (Phase = PhaseBidding or PhaseMatchEnd)

### References

- [Source: _bmad-output/planning-artifacts/epics.md — Story 3.5: Hand Scoring, Failed Hands & Capot]
- [Source: _bmad-output/planning-artifacts/architecture.md — Game State Machine Phases, hand_scoring phase, Minimum Test Fixture Set]
- [Source: _bmad-output/planning-artifacts/prd.md — FR11 (failed hand scoring), FR12 (last-trick bonus, Capot)]
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md — ScorePanel, CapotAnimation, score reveal moment]
- [Source: _bmad-output/project-context.md — Two-phase scoring, failed hand rule, Capot replaces last-trick, match target tiebreaker]
- [Source: _bmad-output/implementation-artifacts/3-4-declarations-and-belot-bonus.md — State Reset Between Hands, Forward Compatibility note for Story 3.5, deferred review items]

### Review Findings

- [x] [Review][Patch] `TurnExpiresAt` not reset in `startNewHand` — FIXED: added `state.TurnExpiresAt = nil` in `startNewHand` and assertion in `TestHandScoring_NewHandStateReset` [scoring.go:86, scoring_test.go:191]
- [x] [Review][Patch] Missing test: failed hand where both teams have non-zero DeclarationPoints — FIXED: added `TestHandScoring_FailedContractBothTeamsHaveDeclarations` verifying opponent absorbs the taker's team's declarations [scoring_test.go]
- [x] [Review][Defer] `scoreHand` dereferences `*state.TrickWinnerSeat` without nil guard — safe because `resolveTrick` always sets it at trick 8, but implicit invariant with no defensive check. Pre-existing pattern (Story 3.3, 3.4) [scoring.go:9]
- [x] [Review][Defer] `scoreHand` dereferences `*state.TrumpCallerSeat` without nil guard — safe because PhaseHandScoring only reachable after trump picked. Pre-existing pattern deferred to session manager (Epic 4) [scoring.go:25]
- [x] [Review][Defer] `matchTarget` returns 1001 for any unrecognized MatchMode string — upstream validation at room creation prevents invalid modes reaching the engine. Defensive validation deferred [scoring.go:104-108]
- [x] [Review][Defer] `resolveDeclarationsForHand` dereferences `*state.TrumpSuit` without nil check — pre-existing pattern noted in Story 3.3 and 3.4 [declarations.go:393]

### Rule Correction — 2026-06-06

- **Tie now fails the trump-calling team.** This story originally shipped "equal total is NOT a failure" (AC #4, Dev Notes, `TestHandScoring_EqualPointsNotFailure`). On user direction the rule was corrected: the trump-calling team must score **strictly more** than the opponents; an equal total (e.g. 81:81) is a failed hand and all points transfer to the opponents. Changed `contractingTotal < opposingTotal` → `<=` in `scoreHand` [scoring.go:64]; inverted the equal-points test to `TestHandScoring_EqualPointsIsFailure` and added `TestHandScoring_TieFailsTrumpCaller`. ACs #3/#4 and Dev Notes above updated to match. Tracked by `spec-fix-tie-fails-trump-caller.md`.
- **Variant divergence (deferred to Epic 12).** The corrected "tie → all points to the opponents" rule is the **Croatian-variant** rule, applied to all variants for now. The **Bitola variant** should eventually use **hanging points (carry-over)** on a tie (points held over, nobody scores, carried to the next decisive hand). This needs cross-hand state and is deferred to Epic 12 (Variant Expansion, Phase 3). See `deferred-work.md` and the Epic 12 entry in `planning-artifacts/epics.md`. Until then, do not treat the Bitola tie behavior as a bug.

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

### Completion Notes List

- Implemented `scoreHand()` in `scoring.go` — orchestrates all hand scoring: last-trick bonus (+10), Capot detection (+100 replaces +10), failed hand check (the taker's team <= opponent, i.e. a tie also fails → all points to opponent), normal scoring, match-end check (1001/501), and new hand setup.
- Implemented `startNewHand()` in `scoring.go` — resets all 20+ per-hand state fields (bidding, trick, declaration/Belote, scoring, player hands), rotates dealer, shuffles and deals fresh deck via existing `dealCards()`.
- Implemented `matchTarget()` helper — returns 1001 or 501 based on MatchMode.
- Integrated scoring into game flow via single `if state.Phase == PhaseHandScoring` guard in `resolveTrickWithDeclarations()` in `declarations.go`. Scoring runs atomically within the same `ApplyAction` call after trick 8 resolves.
- Created `NewGameLastTrick()` fixture — trick 8 state with controlled card distribution (AS, 8D, TD, 7H), TrumpCallerSeat=1 (Team B), TricksWon=[4,3], HandPoints=[70,61] (sum=131, matching 152-21 remaining).
- Created `NewGameCapotInProgress()` fixture — trick 8 state with Team A winning all 7 prior tricks, TricksWon=[7,0], HandPoints=[121,0], Team A holds JH+AH (guaranteed to win).
- All 13 new scoring tests pass through `ApplyAction` — covering last-trick bonus, Capot, Capot broken, failed hand, equal points, normal scoring, match end, match continues, state reset, declarations in scoring, Belote in scoring, 501 mode, and state immutability.
- Updated 2 existing tests in `playing_test.go` (TestEighthTrickTransition, TestFull8TrickHand) that expected `PhaseHandScoring` as final state — now expect `PhaseBidding` or `PhaseMatchEnd` since scoring processes atomically.
- Full backend suite: all packages OK. Frontend: 109/109 tests pass. Zero regressions.

### File List

**New files:**

- `server/internal/game/scoring.go` — scoreHand, startNewHand, matchTarget
- `server/internal/game/scoring_test.go` — 13 test cases through ApplyAction

**Modified files:**

- `server/internal/game/declarations.go` — Added scoreHand call in resolveTrickWithDeclarations
- `server/internal/game/testfixtures/fixtures.go` — Added NewGameLastTrick(), NewGameCapotInProgress()
- `server/internal/game/testfixtures/fixtures_test.go` — Added fixture validation tests
- `server/internal/game/playing_test.go` — Updated 2 tests for atomic scoring behavior
