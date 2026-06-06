---
title: "Fix: Declaration comparison uses strongest single meld, not team sum"
type: "bugfix"
created: "2026-04-25"
status: "done"
baseline_commit: "d62ab06f2be84b3354689e663843099ff849f614"
context:
  - _bmad-output/implementation-artifacts/3-4-declarations-and-belot-bonus.md
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The declaration-round winner must be decided by each team's _strongest single_ meld (per Beljot rules), not by the sum of each team's melds. Today the only test that exercises the user-reported scenario (Team A: 4 Queens 100 + 4 Kings 100 = 200; Team B: 4 Nines 150) does not actually call the resolution function — it only asserts on input fixture values. So whether `resolveDeclarations` produces the wrong answer is unverified, and the user reports observing the wrong team awarded.

**Approach:** Lock in the rule with a regression test that drives the full trick-1 flow for the canonical scenario and asserts Team B wins with exactly 150 points and Team A gets 0. If the test fails, fix the comparison logic in `resolveDeclarations` so it picks each team's strongest individual declaration first, then sums _only the winning team's_ declarations. Belote remains excluded (it never enters `Players[*].Declarations`; it is awarded directly to `HandPoints` on announcement).

## Boundaries & Constraints

**Always:**

- Comparison is between **each team's single strongest declaration**. Tiebreakers stay in this order: higher Value → FoaK over Sequence at equal Value → higher top card (sequences) → trump-suit sequence → earlier seat in play order from trick 1 leader.
- After resolution, only the winning team's declarations are kept on `state.Players[*].Declarations`; the losing team's slice is set to `nil`.
- `state.DeclarationPoints[winner] = sum(winner's decls)`; `state.DeclarationPoints[loser] = 0`.
- Belote (King + Queen of trump, 20 pts) is awarded via `handleAnnounceBelot` to `HandPoints[team]` — it does not flow through `resolveDeclarations` and must remain unaffected.

**Ask First:**

- Any change to the tiebreaker chain itself (rules 3–5) — the user's bug report only names the strongest-vs-sum rule.
- Any change to `Declaration.Value` mapping (rarity ordering between Quint 100 and Kare 100, etc.).

**Never:**

- Don't compare team meld sums to decide the winner.
- Don't preserve the losing team's declarations on the broadcast payload (the manager already iterates all `Players[*].Declarations`, which after resolution should only contain the winner's).
- Don't touch Belote logic, Bitola dedup, or the prompt total displayed in `DeclarationPrompt` (which is per-player, not per-team).

## I/O & Edge-Case Matrix

| Scenario                                                               | Input / State                                                                                          | Expected Output / Behavior                                                                                  | Error Handling |
| ---------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------- | -------------- |
| Strongest-single beats higher sum                                      | Team A seat 0: Kare-Q (100); Team A seat 2: Kare-K (100); Team B seat 1: Kare-9 (150). Trump = Hearts. | `winningTeam = 1` (Team B); `DeclarationPoints = [0, 150]`; Team A `Declarations` cleared, Team B retained. | N/A            |
| Two melds on one team, none on the other                               | Team A seat 0: Tierce (20); Team A seat 2: Kare-T (100); Team B: none.                                 | `winningTeam = 0`; `DeclarationPoints = [120, 0]`.                                                          | N/A            |
| Both teams have melds, equal strongest by Value+Type → seat tiebreaker | Trick 1 leader = seat 1. Team A seat 0: Kare-Q (100); Team B seat 1: Kare-K (100).                     | Team B wins (seat 1 distance 0 < seat 0 distance 3 from leader); `DeclarationPoints[1] = 100`.              | N/A            |
| No declarations at all                                                 | All players' `Declarations` empty.                                                                     | `winningTeam = -1`; both `DeclarationPoints` stay 0.                                                        | N/A            |
| Belote announced + losing-side melds                                   | Team A holds K+Q trump and announces Belote during trick 2; Team B wins meld round.                    | Team B's meld total in `DeclarationPoints[1]`; Belote's 20 pts in `HandPoints[0]`; both recorded.           | N/A            |

</frozen-after-approval>

## Code Map

- `server/internal/game/declarations.go:172-211` — `resolveDeclarations`: builds best-per-team via `declarationBeats`, returns winner + sum of winner's melds. Verify the loop and the final compare-best-vs-best step do exactly that; do not switch to `teamDeclarationTotal` for the comparison.
- `server/internal/game/declarations.go:213-248` — `declarationBeats`: tiebreaker chain. Untouched unless step-03 finds it diverging from the rule order.
- `server/internal/game/declarations.go:436-453` — `resolveDeclarationsForHand`: writes `DeclarationPoints[winner]`, clears losing team's per-player declarations. Untouched.
- `server/internal/game/declarations.go:323-377` — `hasBelot` / `handleAnnounceBelot`: Belote is awarded to `HandPoints[team]`, never enters `Declarations`. Belote path must remain independent.
- `server/internal/game/declarations_test.go:221-235` — `TestDeclarationResolution / only winning team declarations scored`: asserts only on input fixture values, never invokes resolution. Tighten or replace.
- `server/internal/game/testfixtures/fixtures.go:316` — `NewGameWithDeclarations`: helper to seed trick-1 state with pre-set `Declarations`. Reuse.

## Tasks & Acceptance

**Execution:**

- [x] `server/internal/game/declarations_test.go` -- Added `TestDeclarationResolution / strongest single meld wins over higher team sum (Q+K vs 9)` driving the full trick-1 flow with Team A seats 0/2 = Kare-Q/Kare-K (sum 200), Team B seat 1 = Kare-9 (150). Asserts `DeclarationPoints == [0, 150]`, Team A declarations cleared, Team B preserved. Reused `NewGameWithDeclarations` + new `completeTrick1` helper; `BelotAnnounced = true` skips the K/Q-trump prompt fixture-side. Note: seat 3 must play TD (not 7D) due to the Bitola must-overplay-led-suit rule.
- [x] `server/internal/game/declarations_test.go` -- Strengthened `only winning team declarations scored`: now drives the resolution flow via `completeTrick1` and asserts on `state.DeclarationPoints` (Team A 0 / Team B 50) and per-player declaration retention. Old assertion (`decls[2].Value == 50`) was a no-op.
- [x] `server/internal/game/declarations.go` -- Tests pass against current logic. The bug as described ("comparing team sums") is **not** present in `resolveDeclarations`; it already picks each team's strongest meld via `declarationBeats` and only sums the winning team's. No code change to the comparison was needed; the regression tests now lock the behavior in.
- [x] `server/internal/game/declarations.go:162-180` -- Updated the doc comment on `resolveDeclarations` to state explicitly that the winner is the team holding the single strongest declaration (never the team with the larger sum) and that Belote is awarded separately. Rule list now numbered 1-6 mirroring the canonical Beljot ordering.

**Acceptance Criteria:**

- Given Team A holds Kare-Q (100) and Kare-K (100) and Team B holds Kare-9 (150), when trick 1 completes, then `DeclarationPoints` is `[0, 150]` and Team A's per-player `Declarations` are cleared.
- Given only Team A has declarations totalling any amount, when trick 1 completes, then Team A wins with their full sum and Team B gets 0.
- Given Belote is announced by a player on the losing-meld team, when the hand scores, then Belote's 20 pts appear in `HandPoints` independent of `DeclarationPoints`.
- Given no player has any declarations, when trick 1 completes, then both `DeclarationPoints` entries remain 0 and `DeclarationsResolved` is true.

## Verification

**Commands:**

- `cd server && go test -run TestDeclarationResolution -v ./internal/game/` -- expected: all subtests pass, including the new strongest-vs-sum scenario.
- `cd server && go test ./...` -- expected: full suite green; no regression in Belote, scoring, or fixtures.
- `cd server && go vet ./...` -- expected: clean.

## Suggested Review Order

**Behavioural invariant (start here)**

- Updated function-doc nails down "strongest single meld wins, never team sum" plus Belote independence — read this first to grasp the rule the diff defends.
  [`declarations.go:162`](../../server/internal/game/declarations.go#L162)

**Regression coverage (the meat of the change)**

- The headline regression: Q+K (sum 200) loses to lone Kare-9 (150). Asserts winner, sum, and per-player declaration retention.
  [`declarations_test.go:235`](../../server/internal/game/declarations_test.go#L235)

- Strengthened sibling test — used to assert on input fixture only; now drives the resolver and asserts on output `DeclarationPoints`.
  [`declarations_test.go:221`](../../server/internal/game/declarations_test.go#L221)

- Added per acceptance audit: the "only one team declared" path, asserting full sum award.
  [`declarations_test.go:236`](../../server/internal/game/declarations_test.go#L236)

**Test infrastructure**

- New `completeTrick1` helper drives a legal trick-1 sequence end-to-end so resolution actually fires; note seat 3 must play TD due to Bitola's must-overplay rule.
  [`declarations_test.go:267`](../../server/internal/game/declarations_test.go#L267)
