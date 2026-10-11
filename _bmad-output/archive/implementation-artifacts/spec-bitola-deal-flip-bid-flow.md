---
title: "Bitola dealing & trump-selection: deal-flip-bid-finish flow"
type: "refactor"
created: "2026-04-25"
status: "done"
baseline_commit: "d04a1f0"
context:
  - _bmad-output/archive/implementation-artifacts/3-2-trump-bidding-bitola-variant.md
  - _bmad-output/planning-artifacts/project-context.md
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The current Bitola flow deals all 8 cards before bidding, with the trump candidate already living inside one player's hand (the "candidate is also dealt" trick). The authentic Bitola variant deals only 5 cards, flips a public candidate card on the table, runs bidding, and finishes the deal based on who took trump — meaning the candidate is held aside and only enters someone's hand once a picker accepts.

**Approach:** Split dealing into two stages around bidding. Stage 1 (`dealCards`): deal 3+2 = 5 to each player, lift the next deck card into a public `TrumpCandidate`, and store the remaining 11 cards in a new `GameState.Deck` field. Stage 2 (inside `handlePickTrump`): give the picker `TrumpCandidate` + 2 cards from `Deck`; give every other seat 3 cards from `Deck` in dealing order from `(DealerSeat+1) % 4`; clear `Deck` and `TrumpCandidate`; only then check instant-win and transition to `PhasePlaying`. `reshuffleAndRedeal` now collects from hands + `Deck` + `TrumpCandidate` to rebuild a 32-card deck before re-dealing.

## Boundaries & Constraints

**Always:**

- Initial deal stores 5 cards per seat in `Players[i].Hand`, sets `TrumpCandidate` (next card), assigns the remaining 11 cards to `GameState.Deck` in deck order.
- Round 1 `pick_trump`: `TrumpSuit = TrumpCandidate.Suit` (action.Suit ignored). Stage-2 distribution mimics a real-table deal: walk the seats in turn order from `(DealerSeat+1) % 4`, taking the next slice off the front of `Deck` — 3 cards for each non-picker seat, **2 cards for the picker (in their natural rotation slot, not first)**. After the rotation, append `TrumpCandidate` to the picker's hand. Final hand size = 8 for everyone, `Deck` fully consumed (11 cards distributed).
- Round 2 `pick_trump`: requires `action.Suit` (else `ErrInvalidBid`); same real-table distribution rule as round 1 — picker waits their slot for 2 cards, others get 3 each in turn order; candidate appended to picker's hand last.
- Reshuffle on 8 total passes: pool hands + `Deck` + `TrumpCandidate` (must total 32 cards), shuffle, rotate dealer to `(DealerSeat+1) % 4`, run stage-1 deal again, leave `Phase = PhaseDealing` (session manager auto-promotes to `PhaseBidding`).
- `checkInstantWin` runs only after stage-2 distribution completes (final 8-card hands), never on the 5-card mid-bidding state.
- Pure-function rules engine preserved; `cloneGameState` deep-clones the new `Deck` slice.

**Ask First:**

- If the test corpus uses `NewGameJustDealt` to assert 8-card hands during bidding outside the bidding-flow tests themselves, flag the affected files before mass-editing — the fixture's contract is changing.

**Never:**

- Don't deal all 8 cards upfront, and don't write the candidate into a hand during stage 1.
- Don't change scoring, declaration, trick-resolution, pause/disconnect, or play-phase rules.
- Don't add hand or deck privacy filtering on the WebSocket broadcast — existing all-hands-visible exposure is a separate concern and out of scope.
- Don't split the post-pick distribution into a second `ApplyAction` round trip; it must complete atomically inside `handlePickTrump`.

## I/O & Edge-Case Matrix

| Scenario                                            | Input / State                                                  | Expected Output / Behavior                                                                                                                                                                                                         | Error Handling  |
| --------------------------------------------------- | -------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------- |
| Stage-1 deal                                        | fresh `NewGame`                                                | each `Players[i].Hand` has 5 cards, `TrumpCandidate` non-nil, `len(Deck) == 11`, `Phase == PhaseDealing`                                                                                                                           | n/a             |
| Round 1 pick by first bidder (seat 1)               | dealer=0, active=1, candidate=7H, Deck has 11 cards            | rotation order (1,2,3,0) draws (2,3,3,3) from Deck → seat 1 gets Deck[0:2]+candidate, seat 2 gets Deck[2:5], seat 3 gets Deck[5:8], seat 0 gets Deck[8:11]; all hands = 8, Deck=[], TrumpCandidate=nil, TrumpSuit=H, Phase=playing | n/a             |
| Round 1 pick by 4th bidder (seat 0, 3 prior passes) | dealer=0, active=0                                             | rotation (1,2,3,0) draws (3,3,3,2) → seat 1 gets Deck[0:3], seat 2 gets Deck[3:6], seat 3 gets Deck[6:9], seat 0 gets Deck[9:11]+candidate; all hands = 8                                                                          | n/a             |
| Round 2 pick with action.Suit=S (seat 1)            | dealer=0, active=1, BiddingRound=2                             | same rotation as round-1-seat-1 row, but TrumpSuit=S                                                                                                                                                                               | n/a             |
| Round 2 pick without action.Suit                    | nil action.Suit                                                | state unchanged                                                                                                                                                                                                                    | `ErrInvalidBid` |
| 8 total passes                                      | round 2, 3 prior passes, picker passes                         | hands + Deck + TrumpCandidate pooled (32), shuffled, dealer +1, stage-1 re-deal: 5 per player + new candidate + 11-card Deck, BiddingRound=1, BiddingPassCount=0, Phase=PhaseDealing                                               | n/a             |
| Pick when TrumpCandidate is nil (defensive)         | inconsistent state                                             | unchanged                                                                                                                                                                                                                          | `ErrWrongPhase` |
| Instant-win after pick                              | picker ends up with all 8 of trump suit (e.g., contrived deck) | `WinnerTeam` set, `Phase = PhaseMatchEnd`                                                                                                                                                                                          | n/a             |

</frozen-after-approval>

## Code Map

- `server/internal/game/state.go` — `GameState` (add `Deck []Card`), `dealCards` (rewrite to stage-1 only), `NewGame` (no longer runs `checkInstantWin`).
- `server/internal/game/bidding.go` — `handlePickTrump` runs stage-2 distribution + `checkInstantWin` before `Phase=PhasePlaying`; `reshuffleAndRedeal` collects from hands + `Deck` + `TrumpCandidate`; `cloneGameState` deep-clones `Deck`.
- `server/internal/game/testfixtures/fixtures.go` — `NewGameJustDealt` and `NewGameMidBidding` rebuilt around 5-card hands + 11-card Deck + visible candidate. Playing fixtures (`NewGameMidPlay`, `NewGameFirstTrick`, `NewGameLastTrick`, …) keep 8-card hands and explicitly set `Deck = nil`.
- `server/internal/game/state_test.go`, `bidding_test.go`, `testfixtures/fixtures_test.go`, `scoring_internal_test.go` — update assertions for stage-1 sizes and stage-2 distribution; add post-pick hand-size + Deck-empty checks.
- `server/internal/session/manager.go` — no logic change; verify `Deck` survives the `GetStateSnapshot` shallow copy (it does, slices are shared but immutable from session view).
- `client/src/shared/types/gameTypes.ts` — add `deck: Card[]` to `GameState`.
- `client/src/features/game/components/HandCards.tsx`, `DealAnimation.tsx`, `GamePage.tsx` — verify dynamic 5→8 hand rendering is fine (HandCards already iterates the array); the existing `ReshuffleAnimation` still triggers off `Phase === "dealing"` after pass-twice. No new component required.

## Tasks & Acceptance

**Execution:**

- [x] `server/internal/game/state.go` — Add `Deck []Card json:"deck"` to `GameState`. Rewrite `dealCards` to: deal 3 then 2 cards per seat (counter-clockwise from `(Dealer+1) % 4`), set `TrumpCandidate = &deck[20]`, assign `gs.Deck = slices.Clone(deck[21:32])`. Drop the `checkInstantWin` call from `NewGame` (move responsibility to post-pick).
- [x] `server/internal/game/bidding.go` — In `handlePickTrump`: after suit is locked, do a real-table rotation — for `i` in 0..3 take seat `s = (Dealer+1+i) % 4` and slice off `n` cards from the front of `newState.Deck` where `n = 2` if `s == picker` else `3`; append slice to `Players[s].Hand`. After the rotation append `*TrumpCandidate` to the picker's hand; assert `len(newState.Deck) == 0` after distribution; set `newState.Deck = nil` and `newState.TrumpCandidate = nil`; run `checkInstantWin` (set `WinnerTeam`/`Phase=PhaseMatchEnd` on positive result, otherwise `Phase=PhasePlaying`). In `reshuffleAndRedeal`: build the 32-card pool from hands + `state.Deck` + (if non-nil) `*state.TrumpCandidate` before shuffling, then call `dealCards`. In `cloneGameState`: `newState.Deck = slices.Clone(state.Deck)`.
- [x] `server/internal/game/scoring.go` — `checkInstantWin` now prefers `TrumpSuit` (the locked trump after a round 2 pick that differs from the candidate) and falls back to `TrumpCandidate.Suit` for legacy/internal-test states.
- [x] `server/internal/game/testfixtures/fixtures.go` — Rewrote `NewGameJustDealt` so each seat holds 5 low-rank cards of one suit, `TrumpCandidate = AH`, and `Deck` holds 11 high-rank cards engineered so picking trump (round 1 or 2, by any seat) never accidentally triggers instant-win. `NewGameMidBidding` inherits the new state. Playing-phase fixtures naturally have nil `Deck`.
- [x] `server/internal/game/bidding_test.go` — Updated post-pick tests; added `TestPickTrumpStage2Rotation` covering round-1 picks by every seat and round-2 picks by seat 1 (spades) and seat 0 (clubs); added `TestPickTrumpRound1_AppendsCandidateAndDealsCorrectCards` for slice-level math; updated reshuffle tests to expect 5+11+candidate.
- [x] `server/internal/game/state_test.go` — Updated `TestNewGame` assertions: 5 cards per seat after stage-1, 11-card Deck, candidate not in any hand or deck. Added `deck` to camelCase JSON key check.
- [x] `server/internal/game/testfixtures/fixtures_test.go` — Updated `TestNewGameMidBidding` and `TestNewGameJustDealt` to expect 5-card hands + 11-card Deck + candidate, with full 32-card conservation.
- [x] `server/internal/game/scoring_test.go` — `TestInstantWin_NotTriggered_PartialTrump` now relies on the safe-by-design fixture; new `TestInstantWin_TriggeredOnPick` builds a contrived stage-1 state where seat 1's stage-2 slice yields all hearts → instant-win. `TestInstantWin_FirstHand` rewritten: NewGame always returns PhaseDealing now. `TestHandScoring_MatchContinues` updated for stage-1 re-deal sizes.
- [x] `server/internal/game/scoring_internal_test.go` — Existing tests still pass: they set `TrumpCandidate` directly without `TrumpSuit`, exercising the fallback branch.
- [x] `server/internal/game/auto_play_test.go` — Updated stale 8-card "all spades" comment to reflect the new 5-card stage-1 layout.
- [x] `client/src/shared/types/gameTypes.ts` — Added `deck: Card[]` to the `GameState` interface.
- [x] `client/src/shared/stores/gameStore.ts` — `normalizeGameState` coerces a missing/null `deck` to `[]` (matches existing `currentTrick`/hand handling for Go nil-slice JSON output).
- [x] Frontend test fixtures — Added `deck: []` to the default-state literals in `gameStore.test.ts`, `gameTypes.test.ts`, `useWsDispatch.test.ts`, `useReconnectionRedirect.test.tsx`, `legalCards.test.ts`, and `GamePage.test.tsx`.

**Acceptance Criteria:**

- Given a fresh game, when `NewGame` runs, then every seat holds exactly 5 cards, `TrumpCandidate` is set, `Deck` has 11 cards, and no card appears twice across hands ∪ Deck ∪ candidate.
- Given the bidding phase with the candidate revealed, when the active bidder submits `pick_trump` in round 1, then stage-2 distribution walks the seats from `(Dealer+1) % 4` taking the next slice off `Deck` — 2 cards in the picker's slot, 3 elsewhere — and finally appends the candidate to the picker's hand; every seat ends with 8 cards, `Deck` is empty, `TrumpCandidate` is nil, `TrumpSuit` equals the candidate's suit, `Phase` is `playing`, `ActivePlayerSeat = (Dealer+1) % 4`, `TrickNumber = 1`.
- Given bidding round 2, when the active bidder submits `pick_trump` with `action.Suit = SuitSpades`, then the same distribution rule applies and `TrumpSuit = SuitSpades`.
- Given 8 total passes, when the 8th pass is processed, then all 32 cards are pooled (no duplicates, no losses), shuffled, the dealer rotates by +1, stage-1 deal repopulates 5-card hands and `Deck` of 11, `BiddingRound=1`, `BiddingPassCount=0`, `Phase=PhaseDealing`.
- Given a player ends up with all 8 trump-suit cards after stage-2, when the pick resolves, then `Phase = PhaseMatchEnd` and `WinnerTeam` is set to that player's team.
- Given the frontend receives a bidding-phase state, when it renders, then the local seat's `HandCards` shows 5 cards and the `TrumpPrompt` displays the candidate; once `pick_trump` resolves and the new state arrives, hands show 8 cards.

## Spec Change Log

- **2026-04-26 — Step-04 review (iteration 1, patch-only)**
  - Defensive guards added in `handlePickTrump` (`len(state.Deck) != 11` → `ErrWrongPhase`) and `reshuffleAndRedeal` (rebuild from `NewDeck` if pooled `len(deck) != 32`). Avoids slice-index panics on malformed states without changing the happy path.
  - `checkInstantWin` comment rewritten — the candidate fallback is reachable in stage-1 production states, not just internal tests.
  - `TestRound2FullPassReshuffle` refactored to use the shared `collectCards` helper through a new `assertCardsAreFullDeck` assertion — removes the silent-duplicate-candidate gap flagged by the auditor.
  - Added round-2 same-suit-as-candidate test case to `TestPickTrumpStage2Rotation` (picker=1, action.Suit=Hearts).
  - `TestInstantWin_NotTriggered_PartialTrump` strengthened: now constructs a state with exactly 7 trump cards in the picker's hand post-stage-2 and asserts the count, instead of relying on the fixture's safe-by-design layout.
  - Added `TestNewGameJustDealt_NoInstantWinOnAnyPick` — exhaustively exercises every (round, picker, suit) combination and asserts no instant-win trigger, locking the fixture-docstring promise.
  - Rejected: WS broadcast exposing `Deck` (explicitly out of scope per Boundaries "Never").
  - Deferred: `reshuffleAndRedeal` doesn't reset `BelotAnnounced` / `DeclarationsResolved` (pre-existing behavior, unrelated to this story) — appended to deferred-work.md.

## Design Notes

Stage-2 dealing pattern (real-table rotation, canonical implementation in `bidding.go`):

```go
deck := newState.Deck
picker := action.PlayerSeat
idx := 0
// Walk seats in turn order from (Dealer+1)%4. Each seat receives 3 cards
// from the front of the deck — except the picker, who receives 2 in their
// own rotation slot (not first).
for i := 0; i < 4; i++ {
    seat := (newState.DealerSeat + 1 + i) % 4
    n := 3
    if seat == picker {
        n = 2
    }
    newState.Players[seat].Hand = append(newState.Players[seat].Hand, deck[idx:idx+n]...)
    idx += n
}
// After the rotation, the picker takes the public candidate as their 8th card.
newState.Players[picker].Hand = append(newState.Players[picker].Hand, *newState.TrumpCandidate)
// idx must equal 11; clear Deck + candidate
newState.Deck = nil
newState.TrumpCandidate = nil
```

Card-conservation invariant for tests: across `Players[*].Hand` ∪ `Deck` ∪ `{*TrumpCandidate}`, every Bitola card appears exactly once at every step (stage-1 done, mid-bidding, post-pick, post-reshuffle). Add a helper assertion in `bidding_test.go` and reuse.

## Verification

**Commands:**

- `cd server && go test ./...` — all packages green
- `cd server && go vet ./...` — clean
- `cd client && npx vitest run` — all frontend tests green
- `cd client && npx prettier --write .` — formatting normalized (per memory rule before commit)
- `make lint` — no errors
