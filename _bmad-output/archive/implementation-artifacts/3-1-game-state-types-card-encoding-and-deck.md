# Story 3.1: Game State Types, Card Encoding & Deck

Status: done

## Story

As a developer,
I want well-defined game state types, card encoding, and dealing logic,
so that the rules engine has a solid, testable foundation for all game operations.

## Acceptance Criteria

1. **Given** the game types are defined in `server/internal/game/types.go`
   **When** I inspect the type definitions
   **Then** the following types exist: `Card` (Rank + Suit), `Suit` (S/H/D/C), `Rank` (7/8/9/T/J/Q/K/A), `Declaration`, `Action`, `Variant` (Bitola), `Phase` (dealing/bidding/playing/trick_resolving/hand_scoring/match_end/paused/disconnected)
   **And** cards use 2-character encoding: `{Rank}{Suit}` (e.g., KS, TH, 7D, AC)

2. **Given** the GameState struct is defined in `server/internal/game/state.go`
   **When** I inspect the struct
   **Then** fields are ordered per Architecture spec: match metadata, current hand state, current trick state, player states, scoring, timer state
   **And** the struct is serializable with JSON tags using camelCase

3. **Given** a new game is initialized
   **When** `NewGame()` is called with 4 player IDs and Bitola variant
   **Then** a 32-card deck is generated (7-A in all 4 suits), shuffled, and dealt in 3+2 sequence (3 cards to each player, then 2 cards to each player, counter-clockwise from dealer)
   **And** each player holds exactly 8 cards
   **And** a trump candidate card is set (first undealt card or per Bitola convention)
   **And** the phase is set to `bidding`
   **And** the dealer is seat 0 for the first hand

   > **SUPERSEDED (2026-08-18)** — the fixed dealer-0 start no longer holds. `NewGame` now draws the first-hand dealer uniformly at random from seats 0-3, with the opening bidder derived as `(DealerSeat + 1) % 4`. This affects the AC above and tasks 3.5 and 3.6 below, which still record `DealerSeat=0` / `ActivePlayerSeat=1` as this story implemented them. The ACs are left as the historical record of Story 3.1; current behavior is defined by `spec-randomize-first-dealer.md` and `epics.md`. Do not "restore" dealer 0 from this file.

4. **Given** test fixtures are needed
   **When** I inspect `server/internal/game/testfixtures/fixtures.go`
   **Then** factory function `NewGameJustDealt()` exists and returns a valid GameState in `bidding` phase with all 4 players holding 8 cards

5. **Given** the `ApplyAction` function signature is defined in `server/internal/game/rules_engine.go`
   **When** I inspect the function
   **Then** the signature is `ApplyAction(state *GameState, action Action) (*GameState, error)` -- pure function, no side effects

## Tasks / Subtasks

- [x] Task 1: Define core game types in `types.go` (AC: #1)
  - [x] 1.1: Define `Suit` type as string constants: `S`, `H`, `D`, `C`
  - [x] 1.2: Define `Rank` type as string constants: `7`, `8`, `9`, `T`, `J`, `Q`, `K`, `A`
  - [x] 1.3: Define `Card` struct with `Rank` and `Suit` fields + `String()` method returning 2-char ID (e.g., `"KS"`)
  - [x] 1.4: Define `ParseCard(id string) (Card, error)` function to parse 2-char card IDs
  - [x] 1.5: Define `Variant` type with `VariantBitola` constant
  - [x] 1.6: Define `Phase` type with all 8 phase constants: `PhaseDealing`, `PhaseBidding`, `PhasePlaying`, `PhaseTrickResolving`, `PhaseHandScoring`, `PhaseMatchEnd`, `PhasePaused`, `PhaseDisconnected`
  - [x] 1.7: Define `Action` struct with `Type` (string), `PlayerSeat` (int), `Card` (*Card, optional), `Suit` (*Suit, optional for trump pick)
  - [x] 1.8: Define action type constants: `ActionPlayCard`, `ActionPickTrump`, `ActionPassTrump`, `ActionDeclare`, `ActionSkipDeclare`, `ActionPause`, `ActionUnpause`, `ActionOwnerUnpause`
  - [x] 1.9: Define `Declaration` struct placeholder (type, cards, player seat, value)
  - [x] 1.10: Define card point value lookup maps: `TrumpCardPoints` (J=20, 9=14, A=11, T=10, K=4, Q=3, 8=0, 7=0) and `NonTrumpCardPoints` (A=11, T=10, K=4, Q=3, J=2, 9=0, 8=0, 7=0)
  - [x] 1.11: Define `NewDeck()` function returning all 32 cards (7-A in all 4 suits)
  - [x] 1.12: Write table-driven tests for `types.go`: card parsing, card string representation, deck generation (32 unique cards, correct suits/ranks)

- [x] Task 2: Define `GameState` struct in `state.go` (AC: #2)
  - [x] 2.1: Define `PlayerState` struct: `Hand []Card`, `Seat int`, `UserID uint`, `Team string` (teamA/teamB), `Declarations []Declaration`, `Connected bool`
  - [x] 2.2: Define `TrickCard` struct: `Card Card`, `PlayerSeat int`
  - [x] 2.3: Define `GameState` struct with ordered field sections
  - [x] 2.4: Add JSON tags (camelCase) to all exported fields
  - [x] 2.5: Write tests verifying GameState JSON serialization round-trips correctly (marshal then unmarshal, verify all fields preserved)

- [x] Task 3: Implement `NewGame()` initialization in `state.go` (AC: #3)
  - [x] 3.1: Implement `NewGame(playerIDs [4]uint, variant Variant, matchMode string, roomID uint) *GameState`
  - [x] 3.2: Implement `ShuffleDeck(deck []Card)` using `math/rand/v2` (Go 1.22+ automatically seeded)
  - [x] 3.3: Implement 3+2+3 dealing sequence: deal 3 cards, then 2 cards, then 3 cards to each player counter-clockwise from dealer
  - [x] 3.4: Set trump candidate from deck (card at position 12, revealed after round 1)
  - [x] 3.5: Set initial state: Phase=bidding, DealerSeat=0, HandNumber=1, ActivePlayerSeat=1 (player after dealer, counter-clockwise), teams assigned (seats 0,2=Team A, seats 1,3=Team B)
  - [x] 3.6: Write table-driven tests: each player gets 8 cards, no duplicate cards across hands, all 32 cards accounted for, phase is bidding, dealer is seat 0, active player is seat 1, trump candidate is set, teams correct

- [x] Task 4: Define `ApplyAction` signature in `rules_engine.go` (AC: #5)
  - [x] 4.1: Create `rules_engine.go` with `ApplyAction(state *GameState, action Action) (*GameState, error)` -- stub returning `ErrWrongPhase` for now
  - [x] 4.2: Add game domain errors to `server/internal/apperr/errors.go`: `ErrWrongPhase`, `ErrNotYourTurn`, `ErrInvalidCard`, `ErrIllegalPlay`, `ErrGamePaused`, `ErrPlayerDisconnected`
  - [x] 4.3: Write test verifying ApplyAction returns error for stub (confirms function exists and is callable)

- [x] Task 5: Create test fixture factory in `testfixtures/fixtures.go` (AC: #4)
  - [x] 5.1: Implement `NewGameJustDealt()` returning a valid GameState in bidding phase with 4 players each holding 8 cards, deterministic hands (no random shuffle) for reproducible tests
  - [x] 5.2: Write tests validating the fixture: 4 players with 8 cards each, phase is bidding, all 32 cards accounted for, teams correct, dealer and active player correct

- [x] Task 6: Update frontend types to align with backend (AC: #1)
  - [x] 6.1: Extend `client/src/shared/types/gameTypes.ts` with `Phase`, `Variant`, `ActionType`, `Declaration`, `PlayerState`, `TrickCard`, `GameState` types matching backend structs
  - [x] 6.2: Write co-located test `gameTypes.test.ts` verifying type consistency (e.g., all 32 CardId values are valid, Phase and Variant unions cover all values)

### Review Findings

- [x] [Review][Decision] Counter-clockwise dealing formula — RESOLVED: `+1` is correct, seats numbered CCW. Fixed `project-context.md` formula from `(currentPlayer - 1 + 4) % 4` to `(currentPlayer + 1) % 4`. [state.go:143]
- [x] [Review][Patch] Trump candidate card overlap — FIXED: added explanatory comment in dealCards() and test asserting trump candidate exists in a player's hand [state.go:151-157]
- [x] [Review][Patch] ShuffleDeck card preservation — FIXED: added "preserves all 32 cards" subtest verifying no duplicates/drops after shuffle [state_test.go:257-270]
- [x] [Review][Patch] ActivePlayerSeat field ordering — FIXED: moved from Current hand state to Timer state section per Architecture spec [state.go:66]
- [x] [Review][Defer] Duplicate playerID validation missing in `NewGame` — accepts `[4]uint{10, 10, 20, 30}` silently. Validation is session manager's responsibility (Epic 4), not the pure game layer. Pre-existing design pattern.
- [x] [Review][Defer] Zero UserID (uint 0) accepted silently in `NewGame` — GORM auto-increment starts at 1 so DB ID=0 is impossible, but game layer doesn't enforce. Session manager will validate. Pre-existing pattern.

## Dev Notes

### Architecture Requirements

**Pure Function Design:** The rules engine is `ApplyAction(state, action) -> (state, error)` with ZERO side effects. The session manager (Epic 4) handles broadcasting, persistence, timers. This story establishes the signature only -- actual game logic comes in Stories 3.2-3.6.

**GameState Struct Field Ordering:** Fields MUST be ordered in these sections:

1. Match metadata (ID, variant, mode, room, players)
2. Current hand state (hand number, dealer, trump suit)
3. Current trick state (trick number, cards played, leading suit)
4. Player states (hands held, declarations, connection status)
5. Scoring (team scores, hand points, declarations points)
6. Timer state (active player, turn expiry timestamp)

**In-Memory State:** Game state lives in memory as serializable Go structs (not in DB). Phase 1 accepts state loss on crash. The serializable struct design enables Redis-backed persistence as a Phase 2 drop-in upgrade.

**Counter-Clockwise Turn Order:** ALL game operations (dealing, bidding, playing) are counter-clockwise. Turn advancement formula: `(currentPlayer + 1) % 4` where seats go 0->1->2->3->0 counter-clockwise. Seat mapping: 0+2 = Team A (partners), 1+3 = Team B (partners).

**Card ID Format:** Two-character strings `{Rank}{Suit}`. Rank: `7 8 9 T J Q K A`. Suit: `S H D C`. Used everywhere: GameState, WebSocket payloads, frontend rendering. Defined once in `game/types.go` and `gameTypes.ts`.

**3+2 Dealing Sequence (Bitola):** Deal 3 cards to each player counter-clockwise from dealer position, then 2 cards to each player counter-clockwise. Each player ends with 8 cards. Total 32 cards in deck (7 through Ace in 4 suits).

### Technical Constraints

- **JSON tags:** All struct fields use `camelCase` JSON tags (e.g., `json:"trumpSuit"`). All exported (PascalCase) Go fields must have explicit JSON tags -- unexported fields silently vanish from JSON.
- **Optional fields:** Use pointer types for optional/nullable fields (`*Suit`, `*Card`, `*int`, `*time.Time`). Go zero values serialize as real values, not null.
- **Slice cloning:** When manipulating cards or players within any function, clone slices first with `slices.Clone()` or `copy()`. Go slices share underlying arrays.
- **Error wrapping:** Wrap errors with `%w` verb (`fmt.Errorf("...: %w", err)`), never `%v`. Use `errors.Is()` against `apperr` errors.
- **No DB migration needed:** Game state is in-memory. No new database tables for this story.
- **`math/rand/v2`:** Use Go 1.22+ `math/rand/v2` for shuffling -- automatically seeded, no manual seed needed. Do NOT use the older `math/rand` package.

### File Structure (Architecture-Specified)

```
server/internal/game/
  types.go              -- Card, Suit, Rank, Declaration, Action, Variant, Phase types + NewDeck()
  state.go              -- GameState struct + NewGame() initialization
  rules_engine.go       -- ApplyAction() stub (pure function signature)
  types_test.go         -- Table-driven tests for types
  state_test.go         -- Tests for GameState serialization + NewGame()
  rules_engine_test.go  -- Test for ApplyAction stub
  testfixtures/
    fixtures.go         -- NewGameJustDealt() factory function
    fixtures_test.go    -- Fixture validation tests
```

**Existing files to modify:**

- `server/internal/apperr/errors.go` -- Add game domain errors
- `client/src/shared/types/gameTypes.ts` -- Extend with Phase, Variant, GameState types

**Existing scaffolding (empty files):**

- `server/internal/game/game.go` -- Contains only `package game`. Can be deleted or left; new files use the same package.
- `server/internal/game/testfixtures/fixtures.go` -- Contains only `package testfixtures`. Will be replaced with implementation.

### Testing Standards

- **Table-driven tests:** `[]struct{ name string; ... }` with `t.Run` for all test functions
- **testify assertions:** `assert.Equal()`, `require.NoError()`, etc. from `github.com/stretchr/testify`
- **Factory functions only:** All rules engine tests use `testfixtures/` factories, never raw `GameState{}` struct literals
- **Test package:** External test package `package game_test` for tests in `server/internal/game/`, `package testfixtures_test` for fixture tests
- **>90% coverage target** for `internal/game/` -- rule correctness is priority #1
- **Co-located tests:** Test files next to source files

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

Trump total: 62 points per suit. Non-trump total: 30 points per suit. Game total: 62 + 30\*3 = 152 card points per hand.

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

### Previous Story Intelligence (from 2-5)

- **External test package pattern:** `package room_test` -- follow this with `package game_test`
- **Mock repositories:** Not needed for this story (game state is in-memory, no DB layer)
- **testify usage:** `assert.Equal`, `require.NoError`, standard across the project
- **Go module:** v1.26, all dependencies already available -- `testify` imported, no new deps needed
- **Frontend test pattern:** `data-testid` attributes for selection, present tense descriptions

### References

- [Source: _bmad-output/planning-artifacts/epics.md - Epic 3, Story 3.1]
- [Source: _bmad-output/planning-artifacts/architecture.md - Game Domain Specifications]
- [Source: _bmad-output/planning-artifacts/architecture.md - Structure Patterns, GameState]
- [Source: _bmad-output/planning-artifacts/architecture.md - Testing & Quality Gates]
- [Source: _bmad-output/project-context.md - Game Rules Critical Correctness]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

- Dealing sequence clarification: Story AC specified "3+2 sequence" but required 8 cards per player (32 total). Implemented as 3+2+3 dealing pattern (traditional Bitola) to satisfy both constraints. Trump candidate revealed after round 1 (deck position 12).

### Completion Notes List

- Implemented all core game types in `types.go`: Suit, Rank, Card (with String/ParseCard), Variant, Phase (8 phases), Action, Declaration, DeclarationType, card point value maps, NewDeck()
- Implemented `GameState` struct in `state.go` with architecture-ordered field sections (metadata, hand state, trick state, player states, scoring, timer) and camelCase JSON tags
- Implemented `NewGame()` with 3+2+3 dealing sequence, ShuffleDeck using math/rand/v2, counter-clockwise dealing from dealer, team assignment (0,2=Team A, 1,3=Team B)
- Implemented `ApplyAction` stub in `rules_engine.go` returning ErrWrongPhase (actual logic deferred to Stories 3.2-3.6)
- Added 6 game domain errors to `apperr/errors.go`: ErrWrongPhase, ErrNotYourTurn, ErrInvalidCard, ErrIllegalPlay, ErrGamePaused, ErrPlayerDisconnected
- Created `NewGameJustDealt()` test fixture with deterministic hands (each player gets one full suit) for reproducible tests
- Extended frontend `gameTypes.ts` with Phase, Variant, ActionType, Card, Declaration, TrickCard, PlayerState, GameState types matching backend
- 70 new Go tests across 3 test files (types_test.go, state_test.go, rules_engine_test.go) + 11 fixture tests, all passing
- 5 new frontend tests in gameTypes.test.ts, all passing
- Full regression suite: all Go tests pass (go vet clean), all 100 frontend tests pass across 15 files
- ESLint + Prettier clean

### File List

**New files:**

- `server/internal/game/types.go`
- `server/internal/game/types_test.go`
- `server/internal/game/state.go`
- `server/internal/game/state_test.go`
- `server/internal/game/rules_engine.go`
- `server/internal/game/rules_engine_test.go`
- `server/internal/game/testfixtures/fixtures.go` (replaced empty scaffold)
- `server/internal/game/testfixtures/fixtures_test.go`
- `client/src/shared/types/gameTypes.test.ts`

**Modified files:**

- `server/internal/apperr/errors.go` (added 6 game domain errors)
- `client/src/shared/types/gameTypes.ts` (extended with Phase, Variant, GameState types)

### Change Log

- 2026-04-11: Story 3.1 implementation complete -- game state types, card encoding, deck, and dealing logic
