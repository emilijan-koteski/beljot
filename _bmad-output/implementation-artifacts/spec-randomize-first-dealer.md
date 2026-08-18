---
title: 'Randomize the first-hand dealer at match start'
type: 'feature'
created: '2026-08-18'
status: 'done'
review_loop_iteration: 0
baseline_commit: 'bf27d8cba86c0164333dab18a20fb2acc7c54e87'
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `NewGame` hardcodes `DealerSeat: 0`, so the first hand of every match is always dealt by the same seat and opened by the seat after it. Per-hand rotation works, but the starting point never varies — unfair across matches, and unlike a real table where the first dealer is drawn at random.

**Approach:** Pick the first-hand dealer uniformly at random (0-3) inside `NewGame` and derive the opening bidder from it. Everything downstream already reads `DealerSeat`, so no rotation, dealing, timer, bot, or client logic changes — only the seed value and the tests that hardcoded it.

## Boundaries & Constraints

**Always:**
- Randomize inside `NewGame` only; derive `ActivePlayerSeat` as `(DealerSeat + 1) % 4`, never a literal.
- Use the existing randomness source (`math/rand/v2` global, as `ShuffleDeck` does at `state.go:183-188`). No new seeding infrastructure.
- Every seat must be a possible first dealer, with roughly equal probability.
- Tests that assumed dealer 0 must become dealer-derived or explicitly pinned — never deleted to make the suite pass, never left vacuously passing.

**Ask First:**
- Changing `NewGame`'s signature, or adding a test-only hook/field to production code to control the dealer.
- Changing how the dealer is displayed to players.

**Never:**
- Do not change per-hand rotation (`scoring.go:161`), the Bitola round-2 reshuffle-and-rotate (`bidding.go:164`), or the 3+2 dealing sequence.
- Do not touch client code; `dealerSeat` already flows through with exactly one consumer.
- Do not fix pre-existing cosmetic issues found nearby (`DealAnimation.tsx` always fanning from south; un-i18n'd `title="Dealer this hand"` at `PlayerSeat.tsx:311`; vacuous dealer test at `MatchPage.test.tsx:961-973`). Report them only.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior |
|----------|--------------|---------------------------|
| Match start | `NewGame(...)` called | `DealerSeat` in 0..3; `ActivePlayerSeat == (DealerSeat+1)%4`; 5 cards per seat; `Deck` holds 11 |
| Distribution | `NewGame` called 100x | All four seats appear as dealer at least once |
| Hand 2 | Hand 1 scored, dealer D | Dealer becomes `(D+1)%4`; bidding reopens at `(D+2)%4` |
| Round-2 all pass | Nobody picks trump, dealer D | Deck reshuffles; dealer becomes `(D+1)%4` |
| Rematch same room | Match ends, `RemoveSession`, room back to `waiting`, owner restarts | Fresh `NewGame`, newly randomized dealer, independent of prior match |
| Reconnect mid-match | Player rejoins | Snapshot carries the actual `DealerSeat`; dealer chip renders on the correct seat |

</frozen-after-approval>

## Code Map

- `server/internal/game/state.go:194-234` -- `NewGame`; the ONLY place the first-hand dealer is set. Sole production change site.
- `server/internal/game/state.go:245-265` -- `dealCards`; already deals from `(dealer+1+i)%4`. No change.
- `server/internal/match/live_match.go:233` -- only production `NewGame` caller; lines 300-310 arm timers/bots off `gs.ActivePlayerSeat`. No change.
- `server/internal/game/state_test.go:142-163` -- `TestNewGame`; asserts dealer 0 and active seat 1. Breaks.
- `server/internal/match/bot_driver_test.go:135-150, 208-221, 225-236, 460-475` -- four tests premised on "seat 1 opens bidding". One breaks outright; three go vacuous.
- `server/internal/bot/simulation_test.go:26-54` -- 200-hand sim with a `>= 0.60` threshold; calls `NewGame` per hand.
- `server/internal/match/export_test.go` -- existing seams to reuse: `SetBotDelayForTest`, `SetGameStateForTest`, `BotSchedule`.
- `server/internal/game/testfixtures/fixtures.go` -- builds `GameState` literals, never calls `NewGame`. Unaffected.
- `_bmad-output/planning-artifacts/epics.md:739` -- AC "the dealer is seat 0 for the first hand"; now contradicted.

## Tasks & Acceptance

**Execution:**
- [x] `server/internal/game/state.go` -- replace `DealerSeat: 0` with a uniform random seat and derive `ActivePlayerSeat`; comment the fairness rationale. The one behavioral change.
- [x] `server/internal/game/state_test.go` -- rewrite the "dealer is seat 0" / "active player is seat 1" subtests to assert the 0..3 range and the derived relationship; add a subtest for the distribution row above.
- [x] `server/internal/match/bot_driver_test.go` -- make the four listed tests deterministic via `SetGameStateForTest` + `BotSchedule` (inject a known just-dealt state so the bot seat really is the opening bidder) and derive expected seats from `DealerSeat`, so none silently stops testing anything.
- [x] `server/internal/bot/simulation_test.go` -- pin the dealer after each `NewGame` so the experiment stays deterministic and comparable to its baseline; comment that the sim measures heuristics, not seat luck.
- [x] `_bmad-output/planning-artifacts/epics.md` -- update the Story 3.1 AC at line 739 to say the first-hand dealer is randomized, so the epic stops contradicting the code.

**Acceptance Criteria:**
- Given many matches are started, when their first hands are compared, then the dealer varies across all four seats rather than always seat 0.
- Given a match has started, when a player reconnects or the client renders the table, then the dealer chip appears on the seat the server actually chose.
- Given the suite is run repeatedly, when `go test -count=5 ./...` completes, then nothing is flaky and no bot-driver test passes vacuously.

## Spec Change Log

## Design Notes

Randomization lives in `NewGame` rather than a new parameter: `NewGame` already owns non-deterministic setup (`ShuffleDeck`, same global rand), and `live_match.go:233` is its only production caller, so a signature change would spread the concern for no gain.

Test determinism comes from existing seams, not new production surface. Adding `internal/game/export_test.go` would NOT help: an `export_test.go` compiles only into its own package's test binary, so it can never reach the failing tests in `internal/match`. Those inject state instead:

```go
require.NoError(t, mgr.StartMatch(100, "bitola", "1001", mixedPlayers(1), ...))
mgr.SetGameStateForTest(100, markBots(testfixtures.NewGameJustDealt(), 1)) // dealer 0, seat 1 opens
mgr.BotSchedule(100)                                                       // arm the think delay deterministically
```

`testfixtures` builds `GameState` literals directly, so fixture-based rotation tests in `bidding_test.go` and `scoring_test.go` keep their fixed dealer and stay green.

## Verification

**Commands:**
- `cd server && go test ./...` -- expected: all packages ok (baseline before this change was fully green).
- `cd server && go test -count=5 ./internal/match ./internal/bot ./internal/game` -- expected: stable across all 5 runs, proving no dealer-dependent flakiness.
- `cd server && golangci-lint run ./...` -- expected: clean.
- `cd client && npx vitest run` -- expected: unchanged pass (no client edits).

## Suggested Review Order

**The behavior change**

- Start here: the whole feature is these two lines; everything else reacts to them.
  [`state.go:200`](../../server/internal/game/state.go#L200)

**Proving it is really random**

- Share-based, not "each seat appears once" -- catches a 70/10/10/10 skew.
  [`state_test.go:161`](../../server/internal/game/state_test.go#L161)

- Pins the derived relationship so the opening bidder can never drift from the dealer.
  [`state_test.go:157`](../../server/internal/game/state_test.go#L157)

**Keeping the bot tests honest (highest-risk area)**

- Two arming paths now exist; the comment explains why both are valid.
  [`bot_driver_test.go:129`](../../server/internal/match/bot_driver_test.go#L129)

- Delay raised to 300ms so the pause reliably wins the race; broadcast check defeats fixture-restating.
  [`bot_driver_test.go:225`](../../server/internal/match/bot_driver_test.go#L225)

- Control pass first, then teardown mid-flight; assertion relabelled to claim only what it proves.
  [`bot_driver_test.go:292`](../../server/internal/match/bot_driver_test.go#L292)

- Shared arm helper carrying the non-vacuity preconditions.
  [`bot_driver_test.go:304`](../../server/internal/match/bot_driver_test.go#L304)

- Was a literal `1`; now derived from the dealer.
  [`bot_driver_test.go:579`](../../server/internal/match/bot_driver_test.go#L579)

**Deliberate compromise**

- Dealer pinned to keep the bot-quality experiment comparable; comment states what the pin does NOT guarantee.
  [`simulation_test.go:52`](../../server/internal/bot/simulation_test.go#L52)

**Documentation truth**

- The living AC this change overrides.
  [`epics.md:739`](../planning-artifacts/epics.md#L739)

- Story 3.1 is `done`, so its ACs are annotated as superseded rather than rewritten.
  [`3-1-game-state...md:31`](3-1-game-state-types-card-encoding-and-deck.md#L31)

- Six review findings kept out of scope, each with measured evidence.
  [`deferred-work.md:645`](deferred-work.md#L645)
