# Story 3.4: Declarations & Belote Bonus

Status: done

## Story

As a player,
I want declarations to be detected and scored correctly at the first trick, and Belote bonus to work when I hold K+Q of trump,
So that these important scoring mechanics are authentic.

## Acceptance Criteria

1. **Given** the game is at the first trick and a player holds a declarable combination
   **When** it is their turn to play
   **Then** the game state indicates a pending declaration decision for that player (declare/skip)
   **And** the player can submit `declare` or `skip_declare` before playing their card

2. **Given** declarations are submitted at the first trick
   **When** the first trick resolves (all 4 cards played)
   **Then** declarations are compared: tierce (20pts), quarte (50pts), quint (100pts), carré (4xJ=200pts, 4x9=150pts, 4xA/T/K/Q=100pts)
   **And** the highest-value declaration wins; on tie, the team that declared first in play order wins
   **And** only the winning team's declarations are scored — the losing team's declarations are discarded

3. **Given** a player holds K and Q of the trump suit
   **When** they play either the K or Q during any trick
   **Then** the game state indicates a pending Belote announcement for that player
   **And** if the player submits `announce_belot`, 20 points are added to their team's hand score
   **And** if the player submits `skip_belot`, no bonus is awarded

4. **Given** a player tries to declare after the first trick
   **When** `ApplyAction` processes a `declare` action
   **Then** `ErrWrongPhase` is returned (declarations only valid at first trick)

5. **Given** a player submits `skip_declare`
   **When** the action is processed
   **Then** their declaration opportunity is waived and they proceed to play a card

6. **Given** test fixtures
   **When** I inspect `testfixtures/`
   **Then** `NewGameFirstTrick(trump Suit)` exists with configurable hands that include declarable combinations
   **And** `NewGameWithDeclarations(decls []Declaration)` exists for testing declaration resolution

## Tasks / Subtasks

- [x] Task 1: Add new GameState fields for declaration/Belote tracking (AC: all)
  - [x] 1.1: In `server/internal/game/state.go`, add `AwaitingDeclaration bool` to GameState — true when the active player at trick 1 has declarable combos and has not yet declared/skipped
  - [x] 1.2: Add `DeclarationsResolved bool` to GameState — true after trick 1 declarations have been compared and points awarded. Reset to false each new hand
  - [x] 1.3: Add `PendingBelotSeat *int` to GameState — set when a player plays trump K or Q while holding the other; must be resolved via `announce_belot`/`skip_belot` before turn advances
  - [x] 1.4: Add `BelotAnnounced bool` to GameState — true if Belote has been announced this hand (at most once per hand since only one player can hold both trump K+Q)
  - [x] 1.5: Update `cloneGameState()` in `bidding.go` to deep-copy the new pointer field (`PendingBelotSeat`)
  - [x] 1.6: Ensure new fields have correct JSON tags (`json:"awaitingDeclaration"`, `json:"declarationsResolved"`, `json:"pendingBelotSeat"`, `json:"belotAnnounced"`) and are placed in the correct GameState sections (declaration fields in "Current trick state" or "Player states" section, Belote fields near scoring)

- [x] Task 2: Add new action type constants and error sentinels (AC: #3, #4)
  - [x] 2.1: In `server/internal/game/types.go`, add `ActionAnnounceBelot = "announce_belot"` and `ActionSkipBelot = "skip_belot"` action type constants
  - [x] 2.2: In `server/internal/apperr/errors.go`, add `ErrDeclarationNotAvailable = NewAppError("DECLARATION_NOT_AVAILABLE", "no declarable combinations in hand", 400)` — returned when a player submits `declare` but has no valid declarations
  - [x] 2.3: In `server/internal/apperr/errors.go`, add `ErrBelotNotAvailable = NewAppError("BELOT_NOT_AVAILABLE", "belot announcement not available", 400)` — returned when a player submits `announce_belot` without holding K+Q of trump or without playing K/Q of trump
  - [x] 2.4: In `server/internal/apperr/errors.go`, add `ErrActionRequired = NewAppError("ACTION_REQUIRED", "pending action must be resolved first", 400)` — returned when a player tries to play a card while `AwaitingDeclaration` or `PendingBelotSeat` is set

- [x] Task 3: Create `declarations.go` with declaration detection and resolution (AC: #1, #2, #4, #5)
  - [x] 3.1: Create `server/internal/game/declarations.go`
  - [x] 3.2: Implement `detectDeclarations(hand []Card, trumpSuit Suit) []Declaration` — scans a player's hand for all valid declarations:
    - **Sequences**: Find consecutive ranks of the same suit using the natural rank order (7,8,9,T,J,Q,K,A). A tierce = 20pts, quarte = 50pts, quint = 100pts. A longer sequence subsumes shorter subsequences (e.g., a quint does NOT also count a tierce within it)
    - **Carré**: Find ranks where the player holds all 4 suits. 4×J = 200pts, 4×9 = 150pts, 4×A/T/K/Q = 100pts. 4×8 and 4×7 are NOT declarable (0-point cards)
    - Return all valid declarations with their type, cards, and point value
  - [x] 3.3: Implement `resolveDeclarations(players [4]PlayerState, trumpSuit Suit) (winningTeam int, totalPoints int)` — compares all players' declarations after trick 1:
    - Compare highest individual declaration from each team
    - Higher value wins. On equal value: sequence beats carré if somehow equal (in practice they can't be); for equal sequences, the one with the higher top card wins; for still-equal sequences, the one in the trump suit wins; for still-equal, the team that declared first in counter-clockwise play order from the trick leader wins
    - Return winning team index (0=Team A, 1=Team B) and total declaration points for the winning team (sum of ALL their declarations, not just the highest)
    - If no declarations at all, return -1 for winningTeam and 0 points
  - [x] 3.4: Implement `hasDeclarableCombinations(hand []Card, trumpSuit Suit) bool` — quick check returning true if `detectDeclarations` would return any results. Used by `handlePlaying` to decide whether to set `AwaitingDeclaration`
  - [x] 3.5: Implement `handleDeclare(state *GameState, action Action) (*GameState, error)`:
    - Validate: `TrickNumber == 1` and `AwaitingDeclaration == true` and `action.PlayerSeat == ActivePlayerSeat`, else return appropriate error
    - Auto-detect declarations from player's hand via `detectDeclarations`
    - If no valid declarations found, return `ErrDeclarationNotAvailable`
    - Store declarations in `Players[seat].Declarations`
    - Set `AwaitingDeclaration = false`
    - Return new state (player now needs to play a card — do NOT advance ActivePlayerSeat)
  - [x] 3.6: Implement `handleSkipDeclare(state *GameState, action Action) (*GameState, error)`:
    - Validate: `TrickNumber == 1` and `AwaitingDeclaration == true` and `action.PlayerSeat == ActivePlayerSeat`
    - Set `AwaitingDeclaration = false`
    - Return new state (player now needs to play a card)

- [x] Task 4: Implement Belote bonus detection and announcement (AC: #3)
  - [x] 4.1: In `declarations.go`, implement `hasBelot(hand []Card, trumpSuit Suit) bool` — returns true if player holds both K and Q of trump suit
  - [x] 4.2: Implement `shouldPromptBelot(state *GameState, seat int, playedCard Card) bool` — returns true if:
    - The played card is K or Q of the trump suit
    - The player's hand (BEFORE the card was removed) contained both K and Q of trump
    - `BelotAnnounced` is false for this hand (can only be announced once)
  - [x] 4.3: Implement `handleAnnounceBelot(state *GameState, action Action) (*GameState, error)`:
    - Validate: `PendingBelotSeat != nil` and `*PendingBelotSeat == action.PlayerSeat`
    - Award 20 points to the player's team: `HandPoints[TeamForSeat(seat)] += 20`
    - Set `BelotAnnounced = true`, `PendingBelotSeat = nil`
    - Resume normal turn flow (advance ActivePlayerSeat, or resolve trick if 4 cards played)
  - [x] 4.4: Implement `handleSkipBelot(state *GameState, action Action) (*GameState, error)`:
    - Validate: `PendingBelotSeat != nil` and `*PendingBelotSeat == action.PlayerSeat`
    - Set `PendingBelotSeat = nil` (no bonus)
    - Resume normal turn flow

- [x] Task 5: Modify `handlePlaying` to integrate declarations and Belote (AC: #1-5)
  - [x] 5.1: In `playing.go`, update `handlePlaying` to route new action types:
    ```go
    case ActionDeclare:     return handleDeclare(state, action)
    case ActionSkipDeclare: return handleSkipDeclare(state, action)
    case ActionAnnounceBelot: return handleAnnounceBelot(state, action)
    case ActionSkipBelot:     return handleSkipBelot(state, action)
    ```
  - [x] 5.2: In `handlePlayCard`, add guard at the top: if `AwaitingDeclaration == true`, return `ErrActionRequired` ("must declare or skip first")
  - [x] 5.3: In `handlePlayCard`, add guard: if `PendingBelotSeat != nil`, return `ErrActionRequired` ("must announce or skip belot first")
  - [x] 5.4: After a card is successfully played (after removing card from hand, before advancing active player), check `shouldPromptBelot()`. If true:
    - Set `PendingBelotSeat` to the playing seat
    - Do NOT advance ActivePlayerSeat yet — the same player must resolve Belote first
    - Do NOT call `resolveTrick` yet even if 4 cards played — Belote must resolve first
  - [x] 5.5: In the Belote handlers (`handleAnnounceBelot`/`handleSkipBelot`), after clearing `PendingBelotSeat`:
    - Advance `ActivePlayerSeat = (seat + 1) % 4` (the normal post-play advance that was deferred)
    - If `len(CurrentTrick) == 4`, call `resolveTrick`
  - [x] 5.6: When trick 1 begins (after a player's card is played as the first card, OR when a new player's turn starts at trick 1), check if the newly-active player has declarable combos via `hasDeclarableCombinations()`:
    - If yes and `TrickNumber == 1` and player hasn't declared/skipped yet → set `AwaitingDeclaration = true`
    - If no → leave `AwaitingDeclaration = false`, player proceeds to play card
  - [x] 5.7: After trick 1 resolves (in `resolveTrick` when TrickNumber == 1), call `resolveDeclarations`:
    - Award points: `DeclarationPoints[winningTeam] = totalPoints`
    - Clear losing team's declarations: set `Players[seat].Declarations = nil` for losing team players
    - Set `DeclarationsResolved = true`

- [x] Task 6: Create test fixtures (AC: #6)
  - [x] 6.1: Add `NewGameFirstTrick(trump Suit) *GameState` to `testfixtures/fixtures.go`:
    - Phase = PhasePlaying, TrickNumber = 1, CurrentTrick = nil, LeadSuit = nil
    - Trump set to the given suit, TrumpCallerSeat set
    - **Critical**: Player hands must contain declarable combinations for testing:
      - Seat 0: A tierce in one suit (e.g., 7H, 8H, 9H among other cards) → 20 pts
      - Seat 1: A quarte in one suit (e.g., JD, QD, KD, AD among other cards) → 50 pts
      - Seat 2: Carré of Jacks (JS, JH, JD, JC among other cards) → 200 pts (but this requires all 4 Jacks — may conflict with other seats; adjust to a valid 32-card distribution)
      - Seat 3: No declarable combinations
    - Design hands carefully so all 32 cards are distributed (8 per player) and declarations are testable
    - Include at least one player with K+Q of trump for Belote testing
  - [x] 6.2: Add `NewGameWithDeclarations(decls []Declaration) *GameState` to `testfixtures/fixtures.go`:
    - Phase = PhasePlaying, TrickNumber = 1, some cards played in CurrentTrick (simulating mid-trick-1)
    - Pre-populated `Players[seat].Declarations` from the provided `decls` parameter
    - Used for testing `resolveDeclarations` without going through the full declare flow
    - DeclarationsResolved = false (so resolution can be tested)
  - [x] 6.3: Write fixture validation tests in `fixtures_test.go`:
    - `NewGameFirstTrick`: verify 8 cards per player, phase is PhasePlaying, TrickNumber is 1, trump is set, at least 2 players have declarable combos, at least 1 player has no declarable combos
    - `NewGameWithDeclarations`: verify declarations are set on correct seats

- [x] Task 7: Write declaration detection tests in `declarations_test.go` (AC: #1, #2)
  - [x] 7.1: Create `server/internal/game/declarations_test.go` with `package game_test`
  - [x] 7.2: Test `detectDeclarations` through `ApplyAction` — player at trick 1 submits `declare`, verify correct declarations stored:
    - Tierce (e.g., 7H, 8H, 9H) = 20 pts
    - Quarte (e.g., TH, JH, QH, KH) = 50 pts
    - Quint = 100 pts
    - Carré of Jacks = 200 pts
    - Carré of Nines = 150 pts
    - Carré of A/T/K/Q = 100 pts
    - No declaration for 4×8 or 4×7
    - Multiple declarations from same player (sequence + carré)
  - [x] 7.3: Test declaration resolution through full trick 1 play:
    - Higher-value declaration wins (50 beats 20)
    - On tie (e.g., two tierces), higher top card wins
    - On still-tie, trump suit sequence wins
    - On still-tie, first in play order (counter-clockwise from trick leader) wins
    - Only winning team's declarations are scored — losing team gets 0
    - Both teammates' declarations sum for the winning team
  - [x] 7.4: Test no-declaration scenarios:
    - Player with no declarable combos → `AwaitingDeclaration` stays false, goes straight to play_card
    - Player submits `skip_declare` → no declarations stored, proceed to play_card
    - Player submits `declare` with no combos → `ErrDeclarationNotAvailable`
  - [x] 7.5: Test error cases:
    - `declare` at trick 2+ → `ErrWrongPhase`
    - `declare` when not active player → `ErrNotYourTurn`
    - `play_card` while `AwaitingDeclaration = true` → `ErrActionRequired`

- [x] Task 8: Write Belote bonus tests (AC: #3)
  - [x] 8.1: Test Belote detection — play trump K while holding trump Q → `PendingBelotSeat` is set
  - [x] 8.2: Test Belote detection — play trump Q while holding trump K → `PendingBelotSeat` is set
  - [x] 8.3: Test `announce_belot` → 20 points added to team's HandPoints, turn advances normally
  - [x] 8.4: Test `skip_belot` → no points, turn advances normally
  - [x] 8.5: Test Belote at trick 1 with declarations — declaration resolved first, then card play with Belote prompt
  - [x] 8.6: Test Belote at later tricks (trick 2-8) — no declaration prompt, just Belote
  - [x] 8.7: Test no Belote when playing K of trump without Q in hand → no prompt
  - [x] 8.8: Test no Belote when playing non-trump K or Q → no prompt
  - [x] 8.9: Test `announce_belot` when `PendingBelotSeat` is nil → error
  - [x] 8.10: Test `play_card` while `PendingBelotSeat` is set → `ErrActionRequired`

- [x] Task 9: Write integration test — full trick 1 with declarations + Belote (AC: all)
  - [x] 9.1: Test full trick 1 flow: player declares → plays card (with Belote) → announces Belote → next player skips declare → plays card → ... → trick resolves → declarations compared → points awarded
  - [x] 9.2: Verify declaration points appear in `DeclarationPoints[team]` after trick 1
  - [x] 9.3: Verify Belote points appear in `HandPoints[team]` (20 pts added)
  - [x] 9.4: Verify tricks 2-8 have no declaration prompts (only Belote possible)
  - [x] 9.5: Verify state immutability — original state unchanged after all ApplyAction calls

- [x] Task 10: Verify full regression suite passes
  - [x] 10.1: Run `go test ./server/internal/game/...` — all existing tests (types, state, bidding, playing, validation, fixtures) must still pass
  - [x] 10.2: Run `npx vitest run` — all frontend tests must still pass
  - [x] 10.3: Run `make lint` — no linting errors

## Dev Notes

### Architecture Requirements

**Pure Function Contract:** All new functions (`handleDeclare`, `handleSkipDeclare`, `handleAnnounceBelot`, `handleSkipBelot`, `detectDeclarations`, `resolveDeclarations`) MUST be pure functions with no side effects. No broadcasting, no timer management, no DB writes. The session manager (Epic 4) handles all side effects.

**State Cloning:** All `handle*` functions must clone the `GameState` via `cloneGameState()` before any mutation. After Story 3.3, `cloneGameState` properly deep-copies all pointer fields and slices.

**Phase-Based Dispatch:** Extend the existing `handlePlaying` function's action type switch (from Story 3.3) to route `ActionDeclare`, `ActionSkipDeclare`, `ActionAnnounceBelot`, `ActionSkipBelot`. Do NOT modify `ApplyAction` in `rules_engine.go` — all declaration and Belote logic lives within the `PhasePlaying` phase handler.

### Declaration Rules — Complete Specification

**When Declarations Occur:**

- Only during trick 1 of each hand
- Before each player plays their card, they get a chance to declare (if they have declarable combos)
- Players without declarable combos skip directly to play_card (no prompt)

**Declaration Types and Values:**

| Type        | Cards                           | Points |
| ----------- | ------------------------------- | ------ |
| Tierce      | 3 consecutive ranks, same suit  | 20     |
| Quarte      | 4 consecutive ranks, same suit  | 50     |
| Quint       | 5+ consecutive ranks, same suit | 100    |
| Four Jacks  | J of all 4 suits                | 200    |
| Four Nines  | 9 of all 4 suits                | 150    |
| Four Aces   | A of all 4 suits                | 100    |
| Four Tens   | T of all 4 suits                | 100    |
| Four Kings  | K of all 4 suits                | 100    |
| Four Queens | Q of all 4 suits                | 100    |

**Rank Order for Sequences:** 7 < 8 < 9 < T < J < Q < K < A. Consecutive means adjacent in this order. Example: T-J-Q is a valid tierce; 9-J-Q is NOT (skips T).

**Subsumption Rule:** A longer sequence subsumes shorter ones within it. A player holding 7-8-9-T-J of hearts has ONE declaration (quint = 100pts), NOT a quint + two tierces.

**Carré Exclusions:** 4×8 and 4×7 are NOT valid declarations (they have 0 card points).

**Declaration Resolution (after all 4 trick-1 cards played):**

1. Each team's highest individual declaration is compared
2. Higher point value wins
3. On equal point value (e.g., two tierces at 20pts each): the declaration with the higher top card wins (using non-trump rank order: A > T > K > Q > J > 9 > 8 > 7)
4. On still-equal (same length, same top card): declaration in trump suit wins
5. On still-equal (both trump or both non-trump): the team whose declaring player is earlier in counter-clockwise play order from the trick-1 leader wins
6. The **winning team** scores the sum of ALL their declarations (not just the highest). The losing team scores 0
7. Points go into `DeclarationPoints[team]`, separate from `HandPoints[team]`

**Auto-Detection:** When a player submits `declare`, the server auto-detects ALL valid declarations from their hand. The player does NOT specify which declarations — the server finds them all. The `declare` action is a simple "yes, I want to declare" signal.

### Belote Bonus — Complete Specification

**What:** King + Queen of the trump suit held by the same player = 20 bonus points.

**When:** The player plays either the K or Q of trump during ANY trick (not just trick 1). The other card must still be in their hand at the time of play.

**Flow:**

1. Player plays trump K or Q via `play_card`
2. Server detects player held both trump K+Q (checking hand BEFORE the card was removed)
3. `PendingBelotSeat` is set → turn does NOT advance
4. Player submits `announce_belot` (20 pts to team's `HandPoints`) or `skip_belot` (no bonus)
5. After Belote resolved → turn advances normally (or trick resolves if 4th card)

**Constraints:**

- At most one Belote per hand (only one player can hold both trump K+Q)
- Belote points go into `HandPoints[team]`, NOT `DeclarationPoints[team]` — they are a hand bonus, not a declaration
- Belote can be announced at ANY trick, not just trick 1
- If the player does not announce (skips), the bonus is lost permanently for that hand

### Action Flow at Trick 1 (Per Player)

```
Player's turn begins at trick 1
  │
  ├─ Has declarable combos? ──No──→ AwaitingDeclaration = false → play_card
  │
  └─ Yes → AwaitingDeclaration = true
            │
            ├─ Player submits `declare` → auto-detect, store declarations → AwaitingDeclaration = false
            │
            └─ Player submits `skip_declare` → no declarations → AwaitingDeclaration = false
                    │
                    └─ Player submits `play_card`
                        │
                        ├─ Card is trump K/Q and player holds the other? ──Yes──→ PendingBelotSeat = seat
                        │                                                          │
                        │                                                    Player submits announce/skip_belot
                        │                                                          │
                        └─ No → advance turn (or resolve trick if 4 cards)  ←──────┘
```

### Action Flow at Tricks 2-8 (Per Player)

```
Player's turn begins
  │
  └─ Player submits `play_card`
      │
      ├─ Card is trump K/Q and player holds the other? ──Yes──→ PendingBelotSeat = seat
      │                                                          │
      │                                                    Player submits announce/skip_belot
      │                                                          │
      └─ No → advance turn (or resolve trick if 4 cards)  ←──────┘
```

### Existing Code to Reuse

**DO NOT DUPLICATE — reuse these existing functions and types:**

- `cloneGameState(state *GameState) *GameState` in `bidding.go` — clone before mutation
- `TeamForSeat(seat int) int` in `state.go` — 0=Team A (seats 0,2), 1=Team B (seats 1,3)
- `TrumpCardPoints`, `NonTrumpCardPoints` maps in `types.go`
- `TrumpRankOrder`, `NonTrumpRankOrder` maps in `types.go`
- `Declaration` struct in `types.go` — already has Type, Cards, PlayerSeat, Value fields
- `DeclarationType` constants: `DeclarationSequence`, `DeclarationFourOfAKind` in `types.go`
- `ActionDeclare`, `ActionSkipDeclare` action constants in `types.go` — already defined
- `DeclarationPoints [2]int` field in GameState — already exists
- `Players[i].Declarations []Declaration` field in PlayerState — already exists
- `legalCards()`, `isCardLegal()` in `validation.go` — reuse for card play validation
- `resolveTrick()` in `playing.go` — call after Belote resolution at trick end
- All error sentinels: `ErrWrongPhase`, `ErrNotYourTurn`, `ErrInvalidCard`, `ErrIllegalPlay` in `apperr/errors.go`
- `slices.Clone()` for slice deep-copies

### Forward Compatibility — Story 3.5 (Hand Scoring)

Story 3.5 will implement hand scoring after all 8 tricks. It will read `DeclarationPoints[team]` (set by this story) and `HandPoints[team]` (card points + Belote bonus set by this story and Story 3.3) to calculate final hand scores including last-trick bonus, Capot, and failed hands. Ensure `DeclarationPoints` and `HandPoints` are correctly populated so Story 3.5 can consume them directly.

### State Reset Between Hands

When a new hand begins (dealing phase — Story 3.5/3.6 territory), the following fields set by this story must be reset:

- `AwaitingDeclaration = false`
- `DeclarationsResolved = false`
- `PendingBelotSeat = nil`
- `BelotAnnounced = false`
- `DeclarationPoints = [2]int{0, 0}`
- `Players[i].Declarations = nil` for all players

This reset logic belongs in Story 3.5/3.6 (hand/match scoring transitions), NOT in this story. But document it here so the dev agent for 3.5 knows what to reset.

### Project Structure Notes

**New files:**

```
server/internal/game/
  declarations.go          -- NEW: declaration detection, resolution, Belote bonus logic
  declarations_test.go     -- NEW: declaration + Belote tests through ApplyAction
```

**Modified files:**

```
server/internal/game/
  state.go                 -- MODIFIED: Add AwaitingDeclaration, DeclarationsResolved, PendingBelotSeat, BelotAnnounced fields
  types.go                 -- MODIFIED: Add ActionAnnounceBelot, ActionSkipBelot constants
  playing.go               -- MODIFIED: Route new action types, integrate declaration/Belote guards into handlePlayCard
  bidding.go               -- MODIFIED: Update cloneGameState for new pointer field (PendingBelotSeat)
  testfixtures/
    fixtures.go            -- MODIFIED: Add NewGameFirstTrick(), NewGameWithDeclarations()
    fixtures_test.go       -- MODIFIED: Add fixture validation tests
```

**Unchanged files:**

```
  rules_engine.go          -- UNCHANGED (declarations handled within PhasePlaying, no new phase case needed)
  validation.go            -- UNCHANGED (card legality rules don't change)
  playing_test.go          -- UNCHANGED (existing tests should still pass)
  validation_test.go       -- UNCHANGED
  bidding_test.go          -- UNCHANGED
```

**Error file:**

```
server/internal/apperr/
  errors.go                -- MODIFIED: Add ErrDeclarationNotAvailable, ErrBelotNotAvailable, ErrActionRequired
```

### Testing Approach

- All tests go through `ApplyAction` — test the public interface, not internal functions directly
- Use `testfixtures/` factory functions exclusively — no raw `GameState{}` struct literals
- Use Go table-driven test pattern (`[]struct{ name string; ... }` with `t.Run`)
- Tests co-located: `declarations_test.go` next to `declarations.go`
- Package: `package game_test` (external test package)
- Import testfixtures via `"github.com/.../game/testfixtures"`

### Deferred Review Items from Story 3.3

These deferred items from the 3.3 code review are NOT addressed by this story but remain relevant:

- `legalCards` nil-guard on `TrumpSuit`/`LeadSuit` — latent panic on corrupted state, safe through normal call paths
- `currentTrickWinnerSeat` returns -1 for empty trick — unreachable through current code
- No bounds check on `action.PlayerSeat` — deferred to session manager (Epic 4)

### References

- [Source: _bmad-output/planning-artifacts/epics.md — Story 3.4: Declarations & Belote Bonus]
- [Source: _bmad-output/planning-artifacts/architecture.md — Game Domain Specifications, phase table, test fixture minimum set]
- [Source: _bmad-output/planning-artifacts/prd.md — FR9 (declarations), FR10 (Belote bonus)]
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md — DeclarationPrompt component]
- [Source: _bmad-output/project-context.md — Anti-pattern: require player announcement for Belote]
- [Source: _bmad-output/implementation-artifacts/3-3-card-play-and-trick-resolution.md — Forward compatibility note for Story 3.4]

### Review Findings

- [x] [Review][Patch] Carré (100pts) vs quint (100pts) tiebreaker — FIXED: carré beats equal-value sequence — `declarationBeats` falls through to play-order when a FourOfAKind ties a quint at 100pts. Standard Belot rules say carré beats equal-value sequence; story spec says "in practice they can't be equal" but they CAN (100 vs 100). Which should win? [declarations.go:167-195]
- [x] [Review][Patch] `Declaration.Cards` shallow-cloned in `cloneGameState` — `slices.Clone` copies the outer `[]Declaration` but each `Declaration.Cards []Card` shares backing array with original. Deep-clone Cards to preserve pure-function contract [bidding.go:162]
- [x] [Review][Patch] `handBeforePlay` aliases cloned hand — correct today but if `removeCard` ever mutates in-place, Belote detection breaks. Add explicit `slices.Clone` [playing.go:65]
- [x] [Review][Patch] `NewGameFirstTrick(trump)` ignores `trump` parameter — always uses Hearts. Spec says configurable [testfixtures/fixtures.go:234]
- [x] [Review][Patch] Fixture comment for seat 2 says "tierce QS-KS-AS" but seat 2 has no QS — correct comment to "tierce 9H-TH-JH (trump)" [testfixtures/fixtures.go:217-229]
- [x] [Review][Patch] `detectDeclarations` accepts unused `trumpSuit` parameter — remove or document [declarations.go:42]
- [x] [Review][Patch] Dead code: `trickPointsForTeam` helper always returns 0 and is unused — remove [declarations_test.go:514-519]
- [x] [Review][Patch] Missing tests: tiebreaker resolution (top card, trump suit, play-order), carré declarations (4xJ=200, 4x9=150, 4xA/T/K/Q=100, no 4x8/7), quint (5+ sequence = 100pts), Belote at tricks 2-8, skip_declare at trick 2, fixture validation tests for NewGameFirstTrick/NewGameWithDeclarations [declarations_test.go]
- [x] [Review][Defer] `checkDeclarationPrompt` cannot distinguish skip from never-asked — safe in current linear 4-player flow but fragile for future changes; no re-prompt possible today [declarations.go:404]
- [x] [Review][Defer] `TrumpSuit` nil dereference in `resolveDeclarationsForHand` and `checkDeclarationPrompt` — pre-existing pattern deferred in Story 3.3; current call paths guarantee non-nil [declarations.go:382,407]
- [x] [Review][Defer] `BelotAnnounced`/`DeclarationsResolved`/`DeclarationPoints` not reset between hands — Story 3.5/3.6 responsibility per story Dev Notes [state.go]
- [x] [Review][Defer] No `action.PlayerSeat` bounds check before array access — deferred to session manager (Epic 4) per Story 3.3 precedent [declarations.go, playing.go]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

### Completion Notes List

- Implemented declaration detection (`detectDeclarations`) supporting sequences (tierce 20, quarte 50, quint 100) and carré (4xJ=200, 4x9=150, 4xA/T/K/Q=100). Longer sequences subsume shorter subsequences.
- Implemented declaration resolution (`resolveDeclarations`) with full tiebreaker chain: value > top card rank > trump suit > play order. Only winning team's declarations scored.
- Implemented Belote bonus: plays trump K or Q when holding the other triggers pending announcement. `announce_belot` awards 20pts to team's HandPoints; `skip_belot` awards nothing.
- Integrated declarations and Belote into `handlePlaying` — new action types routed, guards added for pending actions (`ErrActionRequired`).
- `handlePickTrump` now calls `checkDeclarationPrompt` for the first player when transitioning to playing phase.
- `NewGameMidPlay` fixture updated with `DeclarationsResolved=true` and `BelotAnnounced=true` to preserve backward compatibility for existing card play tests.
- All 26 new test cases pass. All existing tests pass (zero regressions). Full backend suite: all packages OK. Frontend: 109/109 tests pass.

### File List

**New files:**

- `server/internal/game/declarations.go` — Declaration detection, resolution, Belote bonus, handlers
- `server/internal/game/declarations_test.go` — 26 test cases covering declarations, Belote, integration flow

**Modified files:**

- `server/internal/game/state.go` — Added `AwaitingDeclaration`, `DeclarationsResolved`, `PendingBelotSeat`, `BelotAnnounced` fields
- `server/internal/game/types.go` — Added `ActionAnnounceBelot`, `ActionSkipBelot` constants
- `server/internal/game/playing.go` — Routed new action types, added declaration/Belote guards, integrated Belote detection into card play flow
- `server/internal/game/bidding.go` — Updated `cloneGameState` for `PendingBelotSeat`, added `checkDeclarationPrompt` call in `handlePickTrump`
- `server/internal/game/testfixtures/fixtures.go` — Added `NewGameFirstTrick`, `NewGameWithDeclarations` fixtures; updated `NewGameMidPlay` with declaration/Belote flags
- `server/internal/apperr/errors.go` — Added `ErrDeclarationNotAvailable`, `ErrBelotNotAvailable`, `ErrActionRequired`
