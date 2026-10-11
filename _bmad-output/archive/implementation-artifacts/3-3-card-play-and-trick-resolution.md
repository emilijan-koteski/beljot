# Story 3.3: Card Play & Trick Resolution

Status: done

## Story

As a player in a Belot game,
I want card play to enforce correct suit-following and trump rules,
so that every trick resolves fairly according to Bitola rules.

## Acceptance Criteria

1. **Given** a game is in `playing` phase and it is a player's turn
   **When** the player submits `play_card` with a card in their hand that is a legal play
   **Then** the card is removed from their hand, added to the current trick, and the active player advances counter-clockwise `(ActivePlayerSeat + 1) % 4`

2. **Given** a card has been led in the current trick
   **When** a player must follow suit
   **Then** only cards matching the led suit are legal; if the player has no cards of the led suit, the trump obligation rule (AC #3) or free play (AC #4) applies; if trump has been led, trump must be played if held AND the player must over-trump (play higher trump than the current highest trump in the trick) if possible

3. **Given** a non-trump suit has been led and the player cannot follow suit
   **When** the opponent team is currently winning the trick
   **Then** the player must play trump if they hold any (trump obligation); if playing trump, they must over-trump (play a higher trump than the current highest trump in the trick) if possible; if they have no trump, any card is legal

4. **Given** a non-trump suit has been led and the player cannot follow suit
   **When** the player's team (partner) is currently winning the trick
   **Then** any card in the player's hand is legal (partner exemption -- no trump obligation)

5. **Given** 4 cards have been played in a trick
   **When** the trick resolves
   **Then** the winner is determined: highest trump wins if any trump was played; otherwise highest card of the led suit wins
   **And** card point values are calculated and added to the winning team's `HandPoints`
   **And** `TricksWon` increments for the winning team
   **And** the trick winner leads the next trick (`ActivePlayerSeat` = winner seat)

6. **Given** the 8th trick has been resolved
   **When** all cards have been played in the hand
   **Then** the phase transitions to `PhaseHandScoring` (scoring logic is Story 3.5)

7. **Given** a player submits `play_card` with a card NOT in their hand
   **When** `ApplyAction` processes the action
   **Then** `ErrInvalidCard` is returned and the state is unchanged

8. **Given** a player submits `play_card` that violates suit-following or trump obligations
   **When** `ApplyAction` processes the action
   **Then** `ErrIllegalPlay` is returned and the state is unchanged

9. **Given** it is NOT the player's turn
   **When** they submit `play_card`
   **Then** `ErrNotYourTurn` is returned and the state is unchanged

10. **Given** the game is NOT in `playing` phase
    **When** any player submits `play_card`
    **Then** `ErrWrongPhase` is returned and the state is unchanged

11. **Given** test fixtures
    **When** I inspect `testfixtures/`
    **Then** `NewGameMidPlay(trickNum int)` exists, returning a game in `playing` phase at the specified trick number with trump set and players holding appropriate cards

## Tasks / Subtasks

- [x] Task 1: Fix `cloneGameState` to deep-copy pointer fields (AC: all -- prerequisite)
  - [x] 1.1: In `server/internal/game/bidding.go`, update `cloneGameState` to deep-copy all pointer fields: `TrumpSuit *Suit`, `TrumpCallerSeat *int`, `TrumpCandidate *Card`, `LeadSuit *Suit`, `TrickWinnerSeat *int`, `TurnExpiresAt *time.Time`
  - [x] 1.2: For each pointer field, if non-nil, allocate a new value and copy: `v := *original.Field; clone.Field = &v`
  - [x] 1.3: Deep-copy `CurrentTrick` slice: `clone.CurrentTrick = slices.Clone(original.CurrentTrick)` (TrickCard is a value type, no pointer fields)
  - [x] 1.4: Verify existing bidding tests still pass after the change (`go test ./server/internal/game/...`)
  - [x] 1.5: Add a targeted immutability test: clone state, mutate a pointer field on the clone, assert original is unchanged

- [x] Task 2: Add card rank ordering maps to `types.go` (AC: #5)
  - [x] 2.1: Add `TrumpRankOrder map[Rank]int` in `server/internal/game/types.go`: `{RankJack: 7, Rank9: 6, RankAce: 5, RankTen: 4, RankKing: 3, RankQueen: 2, Rank8: 1, Rank7: 0}`
  - [x] 2.2: Add `NonTrumpRankOrder map[Rank]int`: `{RankAce: 7, RankTen: 6, RankKing: 5, RankQueen: 4, RankJack: 3, Rank9: 2, Rank8: 1, Rank7: 0}`
  - [x] 2.3: These maps are used for trick winner comparison (higher value wins). They differ from point maps because 8 and 7 have distinct ranks despite both having 0 points

- [x] Task 3: Create `validation.go` with card legality logic (AC: #2-4, #7-8)
  - [x] 3.1: Create `server/internal/game/validation.go` with `legalCards(state *GameState, seat int) []Card`
  - [x] 3.2: Implement leading card rule: if `len(state.CurrentTrick) == 0`, return all cards in hand (any card legal when leading)
  - [x] 3.3: Implement follow-suit rule: filter hand for cards matching `state.LeadSuit`. If any exist, return them. Special case: if led suit is trump, apply over-trump rule (see 3.6)
  - [x] 3.4: Implement trump obligation (opponent winning): if player cannot follow suit AND opponent team is winning, filter hand for trump cards. If any exist, apply over-trump rule (see 3.6). If no trump cards, return entire hand
  - [x] 3.5: Implement partner exemption: if player cannot follow suit AND player's team is winning (or tied), return entire hand (no obligation)
  - [x] 3.6: Implement over-trump sub-rule: when a player must play trump, find highest trump rank in current trick. Filter player's trump cards for those with higher `TrumpRankOrder` value. If any exist, return only those. If none, return all trump cards (obligation to trump still holds, but not to over-trump)
  - [x] 3.7: Add helper `currentTrickWinnerSeat(trick []TrickCard, trumpSuit Suit) int` -- determines who is currently winning among cards played so far
  - [x] 3.8: Add helper `isCardLegal(state *GameState, seat int, card Card) bool` -- checks if specific card is in `legalCards` result
  - [x] 3.9: Helper `highestTrumpInTrick(trick []TrickCard, trumpSuit Suit) *Rank` -- returns highest trump rank currently in trick, or nil if no trump played

- [x] Task 4: Create `playing.go` with card play and trick resolution (AC: #1, #5-6)
  - [x] 4.1: Create `server/internal/game/playing.go` with `handlePlaying(state *GameState, action Action) (*GameState, error)`
  - [x] 4.2: Validate phase: return `apperr.ErrWrongPhase` if `state.Phase != PhasePlaying`
  - [x] 4.3: Validate action type: only `ActionPlayCard` is supported (return `ErrWrongPhase` for declare/skip_declare actions -- declarations are Story 3.4)
  - [x] 4.4: Validate turn: return `apperr.ErrNotYourTurn` if `action.PlayerSeat != state.ActivePlayerSeat`
  - [x] 4.5: Validate card in hand: return `apperr.ErrInvalidCard` if `action.Card` is nil or card is not in `state.Players[action.PlayerSeat].Hand`
  - [x] 4.6: Validate legal play: call `isCardLegal(state, action.PlayerSeat, *action.Card)`. Return `apperr.ErrIllegalPlay` if false
  - [x] 4.7: Clone state via `cloneGameState` before any mutation
  - [x] 4.8: Play the card: remove card from player's hand (filter by card match), append `TrickCard{Card: *action.Card, PlayerSeat: action.PlayerSeat}` to `CurrentTrick`
  - [x] 4.9: Set `LeadSuit` on first card of trick: if `len(clone.CurrentTrick) == 1`, set `clone.LeadSuit = &card.Suit`
  - [x] 4.10: Advance active player: `clone.ActivePlayerSeat = (clone.ActivePlayerSeat + 1) % 4`
  - [x] 4.11: Check trick completion: if `len(clone.CurrentTrick) == 4`, call `resolveTrick(clone)`
  - [x] 4.12: Return new state

- [x] Task 5: Implement trick resolution in `playing.go` (AC: #5-6)
  - [x] 5.1: Create `resolveTrick(state *GameState)` (mutates the already-cloned state in place)
  - [x] 5.2: Determine trick winner using `determineTrickWinner(state.CurrentTrick, *state.TrumpSuit) int` -- returns winning player's seat index
  - [x] 5.3: Calculate trick points using `calculateTrickPoints(state.CurrentTrick, *state.TrumpSuit) int` -- sums card point values for all 4 cards
  - [x] 5.4: Award points: `state.HandPoints[TeamForSeat(winnerSeat)] += trickPoints`
  - [x] 5.5: Increment tricks won: `state.TricksWon[TeamForSeat(winnerSeat)]++`
  - [x] 5.6: Set `state.TrickWinnerSeat = &winnerSeat`
  - [x] 5.7: Check trick number BEFORE incrementing: if `state.TrickNumber == 8` (this was the last trick), transition `state.Phase = PhaseHandScoring` (scoring logic in Story 3.5). Do NOT increment TrickNumber past 8
  - [x] 5.8: If `state.TrickNumber < 8`: set up next trick -- `state.TrickNumber++`, `state.CurrentTrick = nil`, `state.LeadSuit = nil`, `state.TrickWinnerSeat = nil`, `state.ActivePlayerSeat = winnerSeat` (winner leads)

- [x] Task 6: Implement `determineTrickWinner` and `calculateTrickPoints` in `playing.go` (AC: #5)
  - [x] 6.1: `determineTrickWinner(trick []TrickCard, trumpSuit Suit) int`:
    - Find all trump cards in trick (card.Suit == trumpSuit)
    - If any trump: highest trump wins (compare using `TrumpRankOrder`)
    - If no trump: highest card of led suit wins (compare using `NonTrumpRankOrder`, only consider cards matching led suit = trick[0].Card.Suit)
    - Return winning player's seat index
  - [x] 6.2: `calculateTrickPoints(trick []TrickCard, trumpSuit Suit) int`:
    - Sum points for all 4 cards: if `card.Suit == trumpSuit` use `TrumpCardPoints[card.Rank]`, else use `NonTrumpCardPoints[card.Rank]`
    - Return total points

- [x] Task 7: Wire playing into `ApplyAction` in `rules_engine.go` (AC: #1-10)
  - [x] 7.1: Add `case PhasePlaying:` to the switch in `ApplyAction`, delegating to `handlePlaying`
  - [x] 7.2: Keep `PhaseTrickResolving` and `PhaseHandScoring` in the default case (return `ErrWrongPhase`) -- they are stub phases for Stories 3.5-3.6

- [x] Task 8: Create `NewGameMidPlay(trickNum int)` test fixture (AC: #11)
  - [x] 8.1: Add `NewGameMidPlay(trickNum int) *GameState` to `testfixtures/fixtures.go`
  - [x] 8.2: Start from a base state: Phase=PhasePlaying, trump set (e.g., Hearts), TrumpCallerSeat set (e.g., seat 1), TrickNumber=trickNum
  - [x] 8.3: Deal deterministic hands with mixed suits so suit-following rules can be tested (NOT the all-one-suit distribution of NewGameJustDealt)
  - [x] 8.4: For trickNum > 1: simulate previous tricks by reducing player hand sizes appropriately (8 - (trickNum-1) cards each), accumulate some HandPoints and TricksWon
  - [x] 8.5: Set ActivePlayerSeat to seat 0 (or configurable via the trick winner chain)
  - [x] 8.6: Set LeadSuit to nil, CurrentTrick to empty (beginning of a new trick)
  - [x] 8.7: Accept trickNum 1-8. Clamp to valid range (1 min, 8 max)
  - [x] 8.8: Write fixture validation tests in `fixtures_test.go`: verify trickNum=1 has 8 cards per player, trickNum=4 has 5 cards per player, trickNum=8 has 1 card per player, phase is PhasePlaying, trump is set

- [x] Task 9: Write card play tests in `playing_test.go` (AC: #1-10)
  - [x] 9.1: Create `server/internal/game/playing_test.go` with `package game_test`
  - [x] 9.2: Test leading card -- any card legal, card removed from hand, added to CurrentTrick, LeadSuit set, ActivePlayerSeat advances
  - [x] 9.3: Test follow suit -- player with led suit cards must play one; other suit cards rejected with ErrIllegalPlay
  - [x] 9.4: Test trump led -- must play trump, must over-trump if possible; lower trump rejected if higher available
  - [x] 9.5: Test trump obligation (opponent winning) -- player void in led suit, opponent winning, must play trump; non-trump rejected with ErrIllegalPlay
  - [x] 9.6: Test over-trump obligation -- when obligated to trump and trick has a trump card, must play higher trump if available
  - [x] 9.7: Test partner exemption -- player void in led suit, partner winning, any card legal (including non-trump)
  - [x] 9.8: Test can't follow suit, no trump held -- any card is legal
  - [x] 9.9: Test trick resolution -- 4th card triggers resolution: winner determined, points calculated, TricksWon incremented, winner leads next trick
  - [x] 9.10: Test trick winner determination -- trump beats non-trump, higher trump beats lower trump, highest of led suit wins when no trump
  - [x] 9.11: Test point calculation -- verify correct card point accumulation per trick using both TrumpCardPoints and NonTrumpCardPoints
  - [x] 9.12: Test 8th trick → PhaseHandScoring transition
  - [x] 9.13: Test ErrInvalidCard -- card not in hand
  - [x] 9.14: Test ErrNotYourTurn -- wrong player
  - [x] 9.15: Test ErrWrongPhase -- play_card in non-playing phase
  - [x] 9.16: Test state immutability -- original state unchanged after ApplyAction
  - [x] 9.17: Test full 8-trick hand -- play all 32 cards through 8 tricks, verify total card points (HandPoints[0] + HandPoints[1]) sum to 152 (62 trump + 30\*3 non-trump = 152 total card points in a hand, before last-trick bonus)

- [x] Task 10: Write validation-focused tests in `validation_test.go` (AC: #2-4)
  - [x] 10.1: Create `server/internal/game/validation_test.go` with `package game_test`
  - [x] 10.2: Test `legalCards` through constructed game states (use fixtures, then modify hands for edge cases via test helper if needed)
  - [x] 10.3: Edge case: all 4 hand cards are trump, non-trump led, opponent winning -- all 4 are legal (all are trump, over-trump applies only among them)
  - [x] 10.4: Edge case: player has only one card left -- always legal regardless of rules
  - [x] 10.5: Edge case: trick has 3 cards, player is 4th -- all rules still apply before the 4th card
  - [x] 10.6: Edge case: partner played trump and is winning, player void in led suit -- any card legal (partner exemption includes when partner trumped)

- [x] Task 11: Verify full regression suite passes
  - [x] 11.1: Run `go test ./...` -- all existing tests (types, state, bidding, fixtures) must still pass
  - [x] 11.2: Run `npx vitest run` -- all frontend tests must still pass
  - [x] 11.3: Run `make lint` -- no linting errors

### Review Findings

- [x] [Review][Patch] `removeCard` panics on empty hand input — `make([]Card, 0, -1)` causes runtime panic; add capacity guard [playing.go:138] — FIXED: safe capacity calculation
- [x] [Review][Patch] Test "trump led - lower trump rejected" plays non-trump JS instead of a lower trump card — does not exercise over-trump rejection [playing_test.go:134] — FIXED: now plays 7H (lower trump) when JH/9H available
- [x] [Review][Patch] Missing negative test for over-trump obligation — no test verifies lower trump is rejected when higher trump available (trump obligation path) [playing_test.go:199] — FIXED: added "lower trump rejected when higher available" sub-test
- [x] [Review][Patch] Test 10.6 partner-trumped exemption scenario doesn't test the actual edge case — all players follow suit instead of testing void-in-led-suit with partner trumped [validation_test.go:147] — FIXED: proper 4-card scenario where partner trumped and is winning
- [x] [Review][Defer] `legalCards` dereferences `TrumpSuit`/`LeadSuit` without nil guards — deferred, latent panic on corrupted/deserialized state; current call chain is safe because `PhasePlaying` invariant guarantees non-nil and `LeadSuit` is set before trick cards accumulate
- [x] [Review][Defer] `currentTrickWinnerSeat` returns -1 for empty trick → `TeamForSeat(-1)` returns -1 in Go (not 1) — deferred, unreachable through current call paths; `isOpponentWinning` only called when `CurrentTrick` is non-empty
- [x] [Review][Defer] No bounds check on `action.PlayerSeat` before array index — deferred, session manager (Epic 4) validates seat range before calling rules engine

## Dev Notes

### Architecture Requirements

**Pure Function Contract:** `handlePlaying` MUST be a pure function called from `ApplyAction`. No side effects (no broadcasting, no timer management, no DB writes). The session manager (Epic 4) handles all side effects. `handlePlaying` takes state + action, returns new state or error.

**State Cloning:** Clone the `GameState` before any mutation. Use `cloneGameState()` from `bidding.go` (same package, directly callable). After fixing D34, this function properly deep-copies pointer fields and slices.

**Phase-Based Dispatch in ApplyAction:** Extend the existing dispatcher (from Story 3.2):

```go
func ApplyAction(state *GameState, action Action) (*GameState, error) {
    switch state.Phase {
    case PhaseBidding:
        return handleBidding(state, action)
    case PhasePlaying:
        return handlePlaying(state, action)
    default:
        return nil, apperr.ErrWrongPhase
    }
}
```

**Trick Resolution is Atomic:** When the 4th card is played, the rules engine resolves the trick within the same `ApplyAction` call. The returned state contains: resolved trick data (`TrickWinnerSeat`, updated `HandPoints`, `TricksWon`) AND the next trick setup (or `PhaseHandScoring` transition). The session manager handles animation delays client-side -- the rules engine does NOT set `PhaseTrickResolving` as an intermediate state. `PhaseTrickResolving` exists as a phase constant for the session manager to use when orchestrating client-side animations (Epic 4), but the pure rules engine transitions atomically.

**Counter-Clockwise Turn Order:** Active player advances via `(seat + 1) % 4`. Trick winner resets the lead for the next trick. Seats: 0->1->2->3->0.

### Bitola Variant Card Play Rules (Complete Specification)

**Three-Layer Validation in `validation.go`:**

1. **Follow Suit:** If you hold cards of the led suit, you MUST play one. If the led suit is trump, you must also over-trump if possible.

2. **Trump Obligation (opponent winning):** If you cannot follow suit AND the opponent team currently holds the winning card in the trick, you MUST play trump if you hold any. If trumping, you must over-trump (play a higher trump than the highest trump already in the trick) if possible. If you cannot over-trump, you must still play a trump card (any trump). If you hold no trump, play any card.

3. **Partner Exemption:** If you cannot follow suit AND your team (partner or yourself from an earlier play) currently holds the winning card in the trick, play ANY card from your hand. No trump obligation, no over-trump obligation.

**Over-Trump Rule (applies when playing a trump card under obligation):**

- Find the highest-ranked trump card already played in the current trick (using `TrumpRankOrder`: J > 9 > A > T > K > Q > 8 > 7)
- If the player holds a trump that outranks it, they MUST play one of those higher trumps
- If the player's only trump cards are lower-ranked, they may play any of their trump cards
- Over-trump only applies when the player is OBLIGATED to play trump (suit-following trump or trump obligation). When a player voluntarily plays trump (partner exemption, free choice), any card is legal including lower trumps

**Leading a Trick:** Any card in hand is legal when leading (first card of a trick).

**Determining Current Trick Winner (for obligation checks):**

- Among cards played so far (1-3 cards):
  - If any trump card(s) were played: the highest trump wins (using `TrumpRankOrder`)
  - Otherwise: the highest card of the led suit wins (using `NonTrumpRankOrder`)
- Compare winner's team (`TeamForSeat(winnerSeat)`) to current player's team to determine opponent vs. partner winning

### Trick Winner Determination

After 4 cards are played:

- If any trump card(s) were played: the highest trump (by `TrumpRankOrder`) wins
- If no trump played: the highest card of the led suit (by `NonTrumpRankOrder`) wins
- Cards of non-led, non-trump suits NEVER win a trick

**Card Ranking (higher value = stronger):**

| Rank | Trump Order | Non-Trump Order |
| ---- | ----------- | --------------- |
| J    | 7 (highest) | 3               |
| 9    | 6           | 2               |
| A    | 5           | 7 (highest)     |
| T    | 4           | 6               |
| K    | 3           | 5               |
| Q    | 2           | 4               |
| 8    | 1           | 1               |
| 7    | 0 (lowest)  | 0 (lowest)      |

### Card Point Values (already defined in `types.go`)

| Rank | Trump Points | Non-Trump Points |
| ---- | ------------ | ---------------- |
| J    | 20           | 2                |
| 9    | 14           | 0                |
| A    | 11           | 11               |
| T    | 10           | 10               |
| K    | 4            | 4                |
| Q    | 3            | 3                |
| 8    | 0            | 0                |
| 7    | 0            | 0                |

**Total card points in a hand: always 152** (trump suit: J(20)+9(14)+A(11)+T(10)+K(4)+Q(3)+8(0)+7(0) = 62 points; each non-trump suit: A(11)+T(10)+K(4)+Q(3)+J(2)+9(0)+8(0)+7(0) = 30 points; total = 62 + 30 + 30 + 30 = 152). This is BEFORE last-trick bonus (+10) or Capot (+100) -- those are added in Story 3.5.

### Existing Code to Reuse

**DO NOT DUPLICATE -- reuse these existing functions and types:**

- `cloneGameState(state *GameState) *GameState` in `bidding.go` -- clone before mutation
- `TeamForSeat(seat int) int` in `state.go` -- returns team index (0=Team A for seats 0,2; 1=Team B for seats 1,3)
- `TrumpCardPoints` map in `types.go` -- point values for trump suit cards
- `NonTrumpCardPoints` map in `types.go` -- point values for non-trump suit cards
- All existing type definitions: `Card`, `Suit`, `Rank`, `Phase`, `Action`, `TrickCard`, `PlayerState`, `GameState`
- Existing error sentinels: `ErrWrongPhase`, `ErrNotYourTurn`, `ErrInvalidCard`, `ErrIllegalPlay` (all already defined in `apperr/errors.go`)
- `slices.Clone()` for slice deep-copies

**IMPORTANT:** `legalCards`, `isCardLegal`, `determineTrickWinner`, `calculateTrickPoints` and helpers are all in the same `game` package. They are unexported (lowercase) and testable indirectly through `ApplyAction`.

**SHARED COMPARISON LOGIC:** `currentTrickWinnerSeat` (validation, 1-3 cards) and `determineTrickWinner` (resolution, 4 cards) use the same card comparison logic. Extract a shared helper `compareCards(a, b Card, trumpSuit Suit, ledSuit Suit) int` or similar to avoid duplication. Both need: trump beats non-trump, higher trump rank beats lower, higher non-trump rank of led suit beats lower.

**STORY 3.4 FORWARD-COMPATIBILITY:** `handlePlaying` currently only handles `ActionPlayCard`. Story 3.4 will add `ActionDeclare` and `ActionSkipDeclare` as additional action types within the `PhasePlaying` phase (first trick only). Design `handlePlaying` as a switch on `action.Type` so Story 3.4 can add cases without restructuring. Return `ErrWrongPhase` for unrecognized action types (follows `handleBidding` pattern).

### Deferred Work Resolution

**D34 (cloneGameState pointer aliasing):** Task 1 of this story resolves the deferred issue from Story 3.2 code review. After fix, pointer fields (`TrumpSuit`, `TrumpCallerSeat`, `TrumpCandidate`, `LeadSuit`, `TrickWinnerSeat`, `TurnExpiresAt`) are deep-copied. This is a prerequisite for safe card play handling.

### File Structure

```
server/internal/game/
  validation.go          -- NEW: legalCards(), isCardLegal(), helpers
  playing.go             -- NEW: handlePlaying(), resolveTrick(), determineTrickWinner(), calculateTrickPoints()
  playing_test.go        -- NEW: Card play + trick resolution tests through ApplyAction
  validation_test.go     -- NEW: Validation edge case tests through ApplyAction
  types.go               -- MODIFIED: Add TrumpRankOrder, NonTrumpRankOrder maps
  bidding.go             -- MODIFIED: Fix cloneGameState pointer deep-copy
  rules_engine.go        -- MODIFIED: Add PhasePlaying case
  state.go               -- UNCHANGED
  rules_engine_test.go   -- UNCHANGED (existing bidding tests should still pass)
  testfixtures/
    fixtures.go          -- MODIFIED: Add NewGameMidPlay()
    fixtures_test.go     -- MODIFIED: Add NewGameMidPlay validation tests
```

**New files:**

- `server/internal/game/validation.go`
- `server/internal/game/playing.go`
- `server/internal/game/playing_test.go`
- `server/internal/game/validation_test.go`

**Modified files:**

- `server/internal/game/types.go` (add rank order maps)
- `server/internal/game/bidding.go` (fix cloneGameState D34)
- `server/internal/game/rules_engine.go` (add PhasePlaying dispatch)
- `server/internal/game/testfixtures/fixtures.go` (add NewGameMidPlay)
- `server/internal/game/testfixtures/fixtures_test.go` (add fixture validation tests)

**No new error definitions needed.** `ErrInvalidCard` and `ErrIllegalPlay` already exist in `apperr/errors.go`.

**No frontend changes required.** The frontend `gameTypes.ts` already has `play_card` action type, `TrickCard` type, `currentTrick`, `leadSuit`, `trickWinnerSeat`, `trickNumber`, `handPoints`, `tricksWon` fields. No new types needed.

**No DB migration needed.** Game state is in-memory.

### Testing Standards

- **Table-driven tests:** `[]struct{ name string; ... }` with `t.Run` -- every test function uses this pattern
- **External test package:** `package game_test` in all test files
- **testify assertions:** `assert.Equal()`, `require.NoError()`, `assert.ErrorIs()`, `require.NotNil()`
- **Factory functions ONLY:** Use `testfixtures.NewGameJustDealt()`, `testfixtures.NewGameMidBidding()`, and the new `testfixtures.NewGameMidPlay()` -- NEVER raw `GameState{}` struct literals
- **Test through `ApplyAction` ONLY:** Do not test `handlePlaying`, `legalCards`, `determineTrickWinner` etc. directly -- test indirectly through `ApplyAction` to preserve refactoring freedom (per project-context.md)
- **State immutability tests:** After every `ApplyAction` call, verify the original state was not modified
- **>90% coverage target** for `internal/game/` package

### NewGameMidPlay Fixture Design Notes

The fixture must provide **mixed-suit hands** (unlike `NewGameJustDealt` which gives each player all one suit). For card play validation testing, players need:

- Cards of the led suit (to test follow-suit)
- Trump cards (to test trump obligation and over-trump)
- Non-trump, non-led-suit cards (to test free play)

Suggested deterministic hand distribution for `NewGameMidPlay(1)` with trump=Hearts:

- Seat 0: `AS, TS, KS, QS, AH, TH, KD, 7C` (4 spades, 2 trump hearts, 1 diamond, 1 club)
- Seat 1: `JS, 9S, 8S, 7S, JH, 9H, QD, 8C` (4 spades, 2 trump hearts, 1 diamond, 1 club)
- Seat 2: `AD, TD, KH, QH, 8H, 7H, AC, TC` (2 diamonds, 4 trump hearts, 2 clubs)
- Seat 3: `JD, 9D, 8D, 7D, KC, QC, JC, 9C` (4 diamonds, 0 trump hearts, 4 clubs)

This allows testing:

- Seat 0 or 1 leading spades → both can follow suit
- Seat 2 or 3 facing spade lead → void in spades, must check trump obligation
- Seat 3 has NO trump → tests "no trump available" branch
- Seat 2 has 4 trumps → tests over-trump scenarios
- Multiple suits per player → realistic game scenarios

For `trickNum > 1`, remove cards proportionally from hands (1 card per player per past trick), accumulate HandPoints from imaginary resolved tricks.

### Game Phase State Machine Reference

| Phase             | Valid Actions                                                 | Transitions To                                             |
| ----------------- | ------------------------------------------------------------- | ---------------------------------------------------------- |
| `dealing`         | (automatic)                                                   | `bidding`                                                  |
| `bidding`         | `pick_trump`, `pass_trump`                                    | `playing` or `dealing` (Bitola reshuffle)                  |
| `playing`         | `play_card` (Story 3.3), `declare`/`skip_declare` (Story 3.4) | next trick (stays `playing`) or `hand_scoring` (8th trick) |
| `trick_resolving` | (session manager concern, Epic 4)                             | `playing` or `hand_scoring`                                |
| `hand_scoring`    | (automatic -- Story 3.5)                                      | `dealing` or `match_end`                                   |
| `match_end`       | (none -- Story 3.6)                                           | --                                                         |

### Previous Story Intelligence (from 3-2)

- **Pure function pattern is established:** `handleBidding` in `bidding.go` is the reference implementation. Follow the same structure: phase check, turn check, clone state, apply logic, return new state
- **`cloneGameState` exists in `bidding.go`** and is callable from any file in the `game` package. Fix it (D34) before using it for card play
- **`reshuffleAndRedeal` collects all 32 cards** from all players -- useful reference for how to iterate over player hands
- **TrickNumber is set to 1 and CurrentTrick is reset** when bidding transitions to playing (already done in `handlePickTrump`). Do not re-initialize these in `handlePlaying`
- **ActivePlayerSeat after trump pick is `(DealerSeat + 1) % 4`** -- this is the player who leads the first trick
- **Code review found TrumpCandidate nil dereference risk** -- be careful with nil pointer fields. Always check before dereferencing
- **Code review deferred D34 (pointer aliasing)** -- this story resolves it
- **External test package pattern:** `package game_test` -- maintain this for all new test files
- **Conventional commit format:** `feat(game): implement card play and trick resolution`

### Git Intelligence

Recent commits follow conventional format: `feat(game): implement trump bidding for Bitola variant with code review fixes`. For this story, commit as: `feat(game): implement card play and trick resolution`.

Files changed in previous story (3.2): `bidding.go`, `bidding_test.go`, `rules_engine.go`, `rules_engine_test.go`, `apperr/errors.go`, `testfixtures/fixtures.go`, `testfixtures/fixtures_test.go`. Story 3.3 modifies some of the same files (`rules_engine.go`, `types.go`, `bidding.go`, `testfixtures/`).

### Project Structure Notes

- All game logic in `server/internal/game/` -- rules engine is pure, no side effects
- `validation.go` follows architecture spec: separate file for legal move checking
- `playing.go` follows `bidding.go` convention: phase-specific handler in its own file
- Test files co-located with source per project conventions
- Domain errors centralized in `server/internal/apperr/errors.go` -- no new errors needed for this story
- No new dependencies needed -- uses only stdlib (`slices`) and existing project deps (`testify`)

### UX Context (for dev awareness -- no frontend changes in this story)

The session manager (Epic 4) and frontend (Epic 4, Story 4-3) will use the state returned by the rules engine to drive these interactions:

- **Playable cards glow** -- determined by `legalCards()` result sent to client
- **Single click plays immediately** -- 150ms animation to trick area
- **Trick resolution pause** -- ~1 second after 4th card, all 4 cards visible, winning card highlights, then cards sweep to winner's pile
- **Active player seat indicator** -- based on `ActivePlayerSeat`
- **Score panel updates** -- based on `HandPoints` changes

### References

- [Source: _bmad-output/planning-artifacts/epics.md - Epic 3, Story 3.3 (lines 712-753)]
- [Source: _bmad-output/planning-artifacts/architecture.md - Game Package Structure (lines 739-754)]
- [Source: _bmad-output/planning-artifacts/architecture.md - Game Phase State Machine (lines 996-1010)]
- [Source: _bmad-output/planning-artifacts/architecture.md - Card Play Error Specification (lines 1064-1075)]
- [Source: _bmad-output/planning-artifacts/architecture.md - Test Fixtures (lines 1090-1102)]
- [Source: _bmad-output/planning-artifacts/architecture.md - Card Encoding Convention (lines 1013-1041)]
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md - Card Play Phase (lines 269-289)]
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md - PlayingCard Component (lines 658-674)]
- [Source: _bmad-output/project-context.md - Three-layer card validation]
- [Source: _bmad-output/project-context.md - Anti-Patterns: "Continue rotation after trick resolves" -> "Trick winner leads the next trick"]
- [Source: _bmad-output/project-context.md - Game Rules Critical Correctness]
- [Source: _bmad-output/archive/implementation-artifacts/3-2-trump-bidding-bitola-variant.md - Previous Story Intelligence]
- [Source: _bmad-output/implementation-artifacts/deferred-work.md - D34: cloneGameState pointer aliasing]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

- Pre-existing `TestApplyActionPhaseBidding` test had a case using `PhasePlaying` phase which now routes to `handlePlaying` instead of the default `ErrWrongPhase`. Updated test to use `PhaseMatchEnd` (a truly unhandled phase).
- `gofmt` alignment issues in `playing_test.go` comment alignment — auto-formatted.
- `determineTrickWinner` delegates to shared `currentTrickWinnerSeat` helper to avoid duplicating card comparison logic between validation (1-3 cards) and resolution (4 cards).

### Completion Notes List

- Fixed `cloneGameState` to deep-copy all 6 pointer fields (D34 resolution): `TrumpSuit`, `TrumpCallerSeat`, `TrumpCandidate`, `LeadSuit`, `TrickWinnerSeat`, `TurnExpiresAt`
- Added `TrumpRankOrder` and `NonTrumpRankOrder` maps to `types.go` for trick winner comparison
- Created `validation.go` with three-layer Bitola card play validation: follow suit, trump obligation (opponent winning), partner exemption, over-trump sub-rule
- Created `playing.go` with `handlePlaying` (pure function), `resolveTrick`, `determineTrickWinner`, `calculateTrickPoints`, card removal helpers
- Wired `PhasePlaying` into `ApplyAction` dispatcher in `rules_engine.go`
- Created `NewGameMidPlay(trickNum int)` fixture with mixed-suit hands (seat 3 has no trump for edge case testing)
- Wrote 17 test functions in `playing_test.go` covering: leading, follow suit, trump led/over-trump, trump obligation, partner exemption, no trump held, trick resolution, point calculation, 8th trick transition, errors, state immutability, full 8-trick hand (152 points verification)
- Wrote 4 test functions in `validation_test.go` covering edge cases: all-trump hand, one card left, 4th player rules, partner-trumped exemption
- Full regression: all Go tests pass (all packages), all 109 frontend tests pass, gofmt clean

### File List

**New files:**

- `server/internal/game/validation.go`
- `server/internal/game/playing.go`
- `server/internal/game/playing_test.go`
- `server/internal/game/validation_test.go`

**Modified files:**

- `server/internal/game/types.go` (added TrumpRankOrder, NonTrumpRankOrder)
- `server/internal/game/bidding.go` (fixed cloneGameState D34 — deep-copy pointer fields)
- `server/internal/game/rules_engine.go` (added PhasePlaying dispatch)
- `server/internal/game/rules_engine_test.go` (updated test to use PhaseMatchEnd instead of PhasePlaying)
- `server/internal/game/testfixtures/fixtures.go` (added NewGameMidPlay)
- `server/internal/game/testfixtures/fixtures_test.go` (added NewGameMidPlay validation tests)

### Change Log

- 2026-04-12: Story 3.3 implementation complete — card play and trick resolution for Bitola variant with full test suite
