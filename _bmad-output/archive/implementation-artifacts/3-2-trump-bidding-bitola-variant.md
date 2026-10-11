# Story 3.2: Trump Bidding (Bitola Variant)

Status: done

## Story

As a player in a Belot game,
I want the trump bidding to follow authentic Bitola variant rules,
so that the game feels correct and familiar to experienced players.

## Acceptance Criteria

1. **Given** a game is in `bidding` phase with a trump candidate revealed
   **When** the active bidder submits `pick_trump`
   **Then** the trump suit is locked to the candidate's suit, `TrumpCallerSeat` is set to the bidder's seat, the phase transitions to `playing`, and `ActivePlayerSeat` is set to `(DealerSeat + 1) % 4` (player after the dealer, counter-clockwise)

2. **Given** a game is in `bidding` phase
   **When** the active bidder submits `pass_trump`
   **Then** `BiddingPassCount` increments by 1, `ActivePlayerSeat` advances counter-clockwise to `(ActivePlayerSeat + 1) % 4`

3. **Given** all 4 players pass in round 1 of bidding (Bitola variant)
   **When** the 4th pass is processed (`BiddingPassCount` reaches 4)
   **Then** `BiddingRound` transitions to 2, `BiddingPassCount` resets to 0, bidding restarts from `(DealerSeat + 1) % 4`
   **And** in round 2 players can pick any suit as trump (not just the candidate suit)

4. **Given** a player submits `pick_trump` in round 2 with a `Suit` field specified in the action
   **When** `ApplyAction` processes the action
   **Then** the trump suit is locked to the specified suit (any of S/H/D/C), `TrumpCallerSeat` is set, the phase transitions to `playing`, and `ActivePlayerSeat` is set to `(DealerSeat + 1) % 4`

5. **Given** all 4 players pass in round 2 of bidding (Bitola variant)
   **When** the 8th total pass is processed (`BiddingPassCount` reaches 4 in round 2)
   **Then** the deck reshuffles, `DealerSeat` rotates counter-clockwise to `(DealerSeat + 1) % 4`, cards are re-dealt in 3+2+3 sequence, a new trump candidate is revealed
   **And** the phase remains `bidding` with `BiddingRound` reset to 1, `BiddingPassCount` reset to 0, `ActivePlayerSeat` set to `(new DealerSeat + 1) % 4`

6. **Given** a player who is NOT the active bidder submits `pick_trump` or `pass_trump`
   **When** `ApplyAction` processes the action
   **Then** `ErrNotYourTurn` is returned and the state is unchanged

7. **Given** the game is NOT in `bidding` phase
   **When** any player submits `pick_trump` or `pass_trump`
   **Then** `ErrWrongPhase` is returned and the state is unchanged

8. **Given** a player submits `pick_trump` in round 2 without specifying a `Suit` in the action
   **When** `ApplyAction` processes the action
   **Then** `ErrInvalidBid` is returned and the state is unchanged

9. **Given** test fixtures
   **When** I inspect `testfixtures/`
   **Then** `NewGameMidBidding(passCount int)` exists, returning a game state with the specified number of passes already recorded, correctly tracking `BiddingRound` (round 2 if passCount >= 4) and `ActivePlayerSeat`

## Tasks / Subtasks

- [x] Task 1: Create `bidding.go` with bidding logic (AC: #1-5, #8)
  - [x] 1.1: Create `server/internal/game/bidding.go` with `handleBidding(state *GameState, action Action) (*GameState, error)`
  - [x] 1.2: Implement phase check -- return `ErrWrongPhase` if `state.Phase != PhaseBidding`
  - [x] 1.3: Implement turn check -- return `ErrNotYourTurn` if `action.PlayerSeat != state.ActivePlayerSeat`
  - [x] 1.4: Implement `ActionPassTrump` handling:
    - Clone state before mutation
    - Increment `BiddingPassCount`
    - Advance `ActivePlayerSeat` to `(ActivePlayerSeat + 1) % 4`
    - If `BiddingPassCount == 4` and `BiddingRound == 1`: transition to round 2 (reset `BiddingPassCount` to 0, set `BiddingRound` to 2, set `ActivePlayerSeat` to `(DealerSeat + 1) % 4`)
    - If `BiddingPassCount == 4` and `BiddingRound == 2`: reshuffle and re-deal (call `reshuffleAndRedeal`)
  - [x] 1.5: Implement `ActionPickTrump` handling:
    - Clone state before mutation
    - Round 1: lock `TrumpSuit` to `TrumpCandidate.Suit` (ignore `action.Suit`)
    - Round 2: lock `TrumpSuit` to `action.Suit` (MUST be non-nil, return `ErrInvalidBid` if nil)
    - Validate `action.Suit` is a valid suit (S/H/D/C) in round 2
    - Set `TrumpCallerSeat` to `action.PlayerSeat`
    - Transition `Phase` to `PhasePlaying`
    - Set `ActivePlayerSeat` to `(DealerSeat + 1) % 4`
    - Set `TrickNumber` to 1
    - Reset `CurrentTrick` to empty slice
  - [x] 1.6: Implement `reshuffleAndRedeal(state *GameState) *GameState`:
    - Collect all 32 cards from all players' hands
    - Shuffle using `ShuffleDeck()`
    - Rotate dealer: `DealerSeat = (DealerSeat + 1) % 4`
    - Re-deal using `dealCards(state, deck)` -- reuse existing deal logic
    - Reset: `BiddingRound = 1`, `BiddingPassCount = 0`, `TrumpSuit = nil`, `TrumpCallerSeat = nil`
    - Set `ActivePlayerSeat = (DealerSeat + 1) % 4`
    - Keep `HandNumber` unchanged (same hand, just re-dealt)

- [x] Task 2: Add `ErrInvalidBid` error to apperr (AC: #8)
  - [x] 2.1: Add `ErrInvalidBid = NewAppError("INVALID_BID", "invalid bid action", 400)` to `server/internal/apperr/errors.go`

- [x] Task 3: Wire bidding into `ApplyAction` in `rules_engine.go` (AC: #1-8)
  - [x] 3.1: Replace the stub in `ApplyAction` with phase-based dispatch
  - [x] 3.2: For `PhaseBidding` phase with `ActionPickTrump` or `ActionPassTrump` actions, delegate to `handleBidding`
  - [x] 3.3: Return `ErrWrongPhase` for any other action type in bidding phase
  - [x] 3.4: Keep returning `ErrWrongPhase` for all other phases (they remain stubs for Stories 3.3-3.6)

- [x] Task 4: Create `NewGameMidBidding` test fixture (AC: #9)
  - [x] 4.1: Add `NewGameMidBidding(passCount int) *GameState` to `testfixtures/fixtures.go`
  - [x] 4.2: Factory logic: start from `NewGameJustDealt()` base state, apply `passCount` passes tracking correct `ActivePlayerSeat` rotation, transition to round 2 if `passCount >= 4`
  - [x] 4.3: Ensure `ActivePlayerSeat` is correctly calculated: seat `(1 + passCount) % 4` for round 1 (dealer=0, first bidder=1), reset to seat 1 for round 2 start (passCount=4), then `(1 + (passCount-4)) % 4` for round 2
  - [x] 4.4: Write fixture validation tests: verify passCount=0 matches `NewGameJustDealt()`, passCount=3 has round 1 with 3 passes, passCount=4 has round 2 with 0 passes, passCount=7 has round 2 with 3 passes

- [x] Task 5: Write table-driven bidding tests in `bidding_test.go` (AC: #1-8)
  - [x] 5.1: Create `server/internal/game/bidding_test.go` with `package game_test`
  - [x] 5.2: Test `pick_trump` in round 1 -- verify trump locked to candidate suit, phase transition, active player, trick number
  - [x] 5.3: Test `pass_trump` sequence -- verify pass count increments, active player rotates correctly for 1-3 passes
  - [x] 5.4: Test round 1 to round 2 transition -- 4 passes trigger round 2 with reset state
  - [x] 5.5: Test `pick_trump` in round 2 -- verify any suit accepted, `action.Suit` used (not candidate)
  - [x] 5.6: Test round 2 `pick_trump` without suit -- verify `ErrInvalidBid`
  - [x] 5.7: Test round 2 full pass -- 8 total passes trigger reshuffle, dealer rotates, new trump candidate, round resets
  - [x] 5.8: Test `ErrNotYourTurn` -- wrong player submitting bid
  - [x] 5.9: Test `ErrWrongPhase` -- bidding actions in non-bidding phase
  - [x] 5.10: Test state immutability -- original state unchanged after `ApplyAction`
  - [x] 5.11: Test multiple reshuffles -- pass through 2 complete rounds twice, verify dealer rotates each time

- [x] Task 6: Verify full regression suite passes
  - [x] 6.1: Run `go test ./...` -- all existing tests (types, state, rules_engine, fixtures) must still pass
  - [x] 6.2: Run `npx vitest run` -- all frontend tests must still pass
  - [x] 6.3: Run `make lint` -- no linting errors

### Review Findings

- [x] [Review][Patch] TrumpCandidate nil dereference in handlePickTrump round 1 — FIXED: added nil guard returning ErrWrongPhase before dereference [bidding.go:64]
- [x] [Review][Patch] NewGameMidBidding accepts passCount >= 8 producing invalid states — FIXED: clamped passCount to max 7 [testfixtures/fixtures.go:102]
- [x] [Review][Defer] cloneGameState does not deep-copy pointer fields (TrumpCandidate, TrumpSuit, TrumpCallerSeat, etc.) — deferred, latent aliasing risk for future stories 3.3-3.6; current code paths are safe because they always assign new pointers rather than writing through existing ones

## Dev Notes

### Architecture Requirements

**Pure Function Contract:** `handleBidding` MUST be a pure function called from `ApplyAction`. No side effects (no broadcasting, no timer management, no DB writes). The session manager (Epic 4) will handle all side effects. `handleBidding` takes state + action, returns new state or error.

**State Cloning:** Clone the `GameState` before any mutation. Use `slices.Clone()` for slice fields (player hands). Go slices share underlying arrays -- modifying without cloning violates the pure function contract. The returned `*GameState` must be a new object; the input state must remain unchanged.

**Phase-Based Dispatch in ApplyAction:** Story 3.2 converts the `ApplyAction` stub into a real dispatcher. The pattern:

```go
func ApplyAction(state *GameState, action Action) (*GameState, error) {
    switch state.Phase {
    case PhaseBidding:
        return handleBidding(state, action)
    // Future stories add cases here
    default:
        return nil, apperr.ErrWrongPhase
    }
}
```

Inside `handleBidding`, further dispatch on `action.Type` (`ActionPickTrump` / `ActionPassTrump`). Return `ErrWrongPhase` for any other action type within bidding phase.

**Counter-Clockwise Turn Order:** All rotation uses `(seat + 1) % 4`. Seats: 0->1->2->3->0. Dealer rotates counter-clockwise on reshuffle. First bidder is always `(DealerSeat + 1) % 4`. First player to act after trump is picked is `(DealerSeat + 1) % 4`.

**Bitola Variant Bidding Rules (Complete Specification):**

1. **Round 1:** Trump candidate card (`TrumpCandidate`) is revealed (already set by `NewGame`/`dealCards`). Starting from `(DealerSeat + 1) % 4`, each player can PICK (accept candidate suit as trump) or PASS. If anyone picks, bidding ends immediately.
2. **Round 2:** If all 4 pass in round 1, round 2 begins. Starting from `(DealerSeat + 1) % 4` again, each player can PICK any of the 4 suits as trump, or PASS. If anyone picks, bidding ends immediately.
3. **Reshuffle:** If all 4 pass in round 2 (8 total passes), the deck is reshuffled, dealer rotates to `(DealerSeat + 1) % 4`, cards are re-dealt in 3+2+3 sequence, new trump candidate is revealed, and round 1 restarts with the new dealer.
4. **No limit on reshuffles** -- the process repeats until someone picks trump.

**Action.Suit Field Usage:**

- Round 1 `pick_trump`: `Action.Suit` is IGNORED -- trump is always the candidate card's suit
- Round 2 `pick_trump`: `Action.Suit` is REQUIRED -- player chooses any suit. Return `ErrInvalidBid` if nil
- `pass_trump` (any round): `Action.Suit` is ignored

### Existing Code to Reuse

**DO NOT DUPLICATE -- reuse these existing functions and types:**

- `ShuffleDeck(deck []Card)` in `state.go` -- shuffles in place using `math/rand/v2`
- `dealCards(gs *GameState, deck []Card)` in `state.go` -- implements 3+2+3 dealing, sets `TrumpCandidate`
- `NewDeck()` in `types.go` -- generates fresh 32-card deck
- All existing type definitions (`Card`, `Suit`, `Rank`, `Phase`, `Action`, `Variant`)
- All existing `GameState` fields (`BiddingRound`, `BiddingPassCount`, `TrumpCandidate`, `TrumpSuit`, `TrumpCallerSeat`, `ActivePlayerSeat`, `DealerSeat`)
- `TeamForSeat(seat int) int` in `state.go`

**IMPORTANT:** `dealCards` is currently unexported (lowercase). The `reshuffleAndRedeal` function is in the same package (`game`) so it can call `dealCards` directly. Do NOT export `dealCards` -- keep it internal.

**IMPORTANT:** `ShuffleDeck` IS exported (uppercase S). It can be called directly from `bidding.go`.

### File Structure

```
server/internal/game/
  bidding.go             -- NEW: handleBidding() + reshuffleAndRedeal()
  bidding_test.go        -- NEW: Table-driven bidding tests
  types.go               -- UNCHANGED (all types already defined)
  state.go               -- UNCHANGED (NewGame, dealCards, ShuffleDeck already exist)
  rules_engine.go        -- MODIFIED: Replace stub with phase-based dispatch
  rules_engine_test.go   -- UNCHANGED (existing stub tests may need update if stub behavior changes)
  testfixtures/
    fixtures.go          -- MODIFIED: Add NewGameMidBidding()
    fixtures_test.go     -- MODIFIED: Add NewGameMidBidding validation tests
```

**Files to modify:**

- `server/internal/game/rules_engine.go` -- Replace stub with phase dispatch
- `server/internal/game/testfixtures/fixtures.go` -- Add `NewGameMidBidding`
- `server/internal/game/testfixtures/fixtures_test.go` -- Add fixture tests
- `server/internal/apperr/errors.go` -- Add `ErrInvalidBid`

**New files:**

- `server/internal/game/bidding.go`
- `server/internal/game/bidding_test.go`

**No frontend changes required.** The frontend `gameTypes.ts` already has `pick_trump` and `pass_trump` action types, `biddingRound` and `biddingPassCount` fields in `GameState`, and `trumpSuit`/`trumpCallerSeat`/`trumpCandidate` fields. No new types needed.

**No DB migration needed.** Game state is in-memory.

### Testing Standards

- **Table-driven tests:** `[]struct{ name string; ... }` with `t.Run` -- every test function uses this pattern
- **External test package:** `package game_test` in `bidding_test.go`
- **testify assertions:** `assert.Equal()`, `require.NoError()`, `assert.ErrorIs()`, `require.NotNil()`
- **Factory functions ONLY:** Use `testfixtures.NewGameJustDealt()` and the new `testfixtures.NewGameMidBidding()` -- NEVER raw `GameState{}` struct literals
- **Test through `ApplyAction` ONLY:** Do not test `handleBidding` directly -- test indirectly through `ApplyAction` to preserve refactoring freedom (per project-context.md)
- **State immutability tests:** After every `ApplyAction` call, verify the original state was not modified (compare key fields before/after)
- **>90% coverage target** for `internal/game/` package

### Card Point Values Reference

| Card | Trump Points | Non-Trump Points |
| ---- | ------------ | ---------------- |
| J    | 20           | 2                |
| 9    | 14           | 0                |
| A    | 11           | 11               |
| T    | 10           | 10               |
| K    | 4            | 4                |
| Q    | 3            | 3                |
| 8    | 0            | 0                |
| 7    | 0            | 0                |

### Game Phase State Machine Reference

| Phase             | Valid Actions                          | Transitions To                            |
| ----------------- | -------------------------------------- | ----------------------------------------- |
| `dealing`         | (automatic)                            | `bidding`                                 |
| `bidding`         | `pick_trump`, `pass_trump`             | `playing` or `dealing` (Bitola reshuffle) |
| `playing`         | `play_card`, `declare`, `skip_declare` | `trick_resolving`                         |
| `trick_resolving` | (automatic)                            | `playing` or `hand_scoring`               |
| `hand_scoring`    | (automatic)                            | `dealing` or `match_end`                  |
| `match_end`       | (none)                                 | --                                        |
| `paused`          | `unpause`, `owner_unpause`             | (previous phase)                          |
| `disconnected`    | `reconnect`                            | (previous phase)                          |

### Project Structure Notes

- All game logic in `server/internal/game/` -- rules engine is pure, no side effects
- `bidding.go` follows architecture spec: separate file for bidding logic, distinct from `rules_engine.go` dispatcher
- `bidding_test.go` co-located with source per project conventions
- Domain errors centralized in `server/internal/apperr/errors.go`
- No new dependencies needed -- uses only stdlib (`slices`, `math/rand/v2`) and existing project deps (`testify`)

### Previous Story Intelligence (from 3-1)

- **Dealing sequence is 3+2+3** (not 3+2 as originally specified in epics). Story 3.1 clarified: 3 cards, trump candidate revealed at position 12, then 2 cards (trump candidate goes to a player's hand), then 3 cards. Each player ends with 8 cards.
- **Trump candidate exists in a player's hand** -- it is both stored in `GameState.TrumpCandidate` AND dealt to a player. The card at deck index 12 is the trump candidate.
- **`NewGameJustDealt()` fixture uses deterministic hands** -- Seat 0: all Spades, Seat 1: all Hearts, Seat 2: all Diamonds, Seat 3: all Clubs. Trump candidate: 7H. Use this for predictable test assertions.
- **External test package pattern:** `package game_test` -- follow this for `bidding_test.go`
- **Review found `ActivePlayerSeat` was in wrong struct section** -- it was moved to Timer state section per Architecture spec. Make sure not to re-introduce ordering issues.
- **Review deferred duplicate playerID validation** -- session manager responsibility, not game layer. Don't add player validation in bidding logic.

### Git Intelligence

Recent commits show the project uses conventional commit format: `feat(game): ...`, `fix(i18n): ...`. For this story, commit as: `feat(game): implement trump bidding for Bitola variant`.

### References

- [Source: _bmad-output/planning-artifacts/epics.md - Epic 3, Story 3.2 (lines 673-711)]
- [Source: _bmad-output/planning-artifacts/architecture.md - Game Domain Specifications (lines 994-1011)]
- [Source: _bmad-output/planning-artifacts/architecture.md - Game Phase Error Specification (lines 1060-1075)]
- [Source: _bmad-output/planning-artifacts/architecture.md - Testing & Quality Gates, Test Fixtures (lines 1090-1103)]
- [Source: _bmad-output/planning-artifacts/architecture.md - Rules Engine Design (line 250)]
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md - Trump Selection (lines 262-267)]
- [Source: _bmad-output/project-context.md - Game Rules Critical Correctness, Trump Bidding]
- [Source: _bmad-output/project-context.md - Anti-Patterns: "Implement generic trump bidding" -> "Branch by variant from the start"]
- [Source: _bmad-output/archive/implementation-artifacts/3-1-game-state-types-card-encoding-and-deck.md - Previous Story Intelligence]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

- Task 2 (ErrInvalidBid) was implemented before Task 1 since `bidding.go` depends on it for compilation. No functional impact.
- Pre-existing `TestApplyActionStub` test was updated to `TestApplyActionPhaseBidding` since `ApplyAction` is no longer a stub for bidding phase actions.
- Pre-existing gofmt alignment issue on `ErrGameNotStartable` was fixed while modifying `errors.go`.
- Pre-existing Prettier formatting issue in `gameTypes.test.ts` is unrelated to this story (not modified).

### Completion Notes List

- Created `bidding.go` with `handleBidding`, `handlePassTrump`, `handlePickTrump`, `reshuffleAndRedeal`, and `cloneGameState` — all pure functions, no side effects
- Implemented full Bitola variant bidding: Round 1 (pick candidate suit or pass), Round 2 (pick any suit or pass), Reshuffle-and-rotate-dealer on double-pass
- Added `ErrInvalidBid` to centralized error definitions in `apperr/errors.go`
- Replaced `ApplyAction` stub with phase-based dispatcher routing `PhaseBidding` to `handleBidding`
- Created `NewGameMidBidding(passCount int)` test fixture factory supporting passCount 0-7
- Wrote 11 test functions with 30+ test cases covering: round 1 pick, pass sequence, round 1→2 transition, round 2 pick (any suit), round 2 pick without suit (ErrInvalidBid), reshuffle on 8 passes, ErrNotYourTurn, ErrWrongPhase, state immutability, multiple reshuffles, round 1 ignores Action.Suit
- Full regression: all Go tests pass (all packages green), all 100 frontend tests pass
- Go vet and gofmt clean for all modified files

### File List

**New files:**

- `server/internal/game/bidding.go`
- `server/internal/game/bidding_test.go`

**Modified files:**

- `server/internal/apperr/errors.go` (added ErrInvalidBid, fixed gofmt alignment)
- `server/internal/game/rules_engine.go` (replaced stub with phase-based dispatch)
- `server/internal/game/rules_engine_test.go` (updated stub test to reflect new bidding behavior)
- `server/internal/game/testfixtures/fixtures.go` (added NewGameMidBidding)
- `server/internal/game/testfixtures/fixtures_test.go` (added NewGameMidBidding validation tests)

### Change Log

- 2026-04-12: Story 3.2 implementation complete — trump bidding for Bitola variant with full test suite
