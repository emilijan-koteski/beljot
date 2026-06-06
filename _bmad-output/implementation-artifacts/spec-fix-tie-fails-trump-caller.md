---
title: 'Hand tie (e.g. 81:81) fails the trump-calling team'
type: 'bugfix'
created: '2026-06-06'
status: 'done'
baseline_commit: 'd012d5fda8dc756a5e61ac7e4b3bf80363e683cf'
context:
  - '{project-root}/_bmad-output/project-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** When the trump-calling team's hand total equals the opponents' (a tie, e.g. 81:81 of the 162 base points — the threshold is higher when declarations/bonuses inflate the total), the engine currently treats it as a *success* and lets each team keep its own points. Per the rules, a tie is a **failed hand** for the trump-calling team: they must score *strictly more* than the opponents to succeed.

**Approach:** Change the failed-hand test in `scoreHand` so a tie counts as failure (`contractingTotal <= opposingTotal`). The existing failed-hand branch already transfers all points to the opponents, so only the comparison operator changes. Invert the stale test that encoded the old rule, add an explicit 81:81 regression test, and correct the rule wording in the docs that state the opposite.

## Boundaries & Constraints

**Always:**
- Compare *team totals* (`HandPoints + DeclarationPoints`, i.e. card points + last-trick/Capot + declarations + Belote), not raw card points — the comparison site already uses totals.
- On failure the trump-calling team scores 0 and the opponents receive the sum of both totals (existing behavior — do not duplicate or alter it).
- Keep `scoreHand` a pure mutator of the already-cloned state; no new fields, no signature change.

**Ask First:**
- Whether to also exempt an announced Belote (+20) from the failed-hand transfer, or apply a "hanging points" carry-over on ties — these are real Belot variants the user did **not** request. Default: do neither (tie → all points to opponents).

**Never:**
- Do not re-derive the failed-hand rule anywhere else — `scoreHand` is the single source of truth; every other site only reads the resulting `FailedContract` boolean.
- Do not change match-end tiebreaker logic (`determineMatchWinner`) — that handles *match-score* ties, a separate concern.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Caller wins | contractingTotal > opposingTotal | Normal scoring: each team keeps its own total | N/A |
| Exact tie (THE FIX) | contractingTotal == opposingTotal (e.g. 81:81) | Failed hand: caller +0, opponents += both totals | N/A |
| Caller loses | contractingTotal < opposingTotal | Failed hand: caller +0, opponents += both totals | N/A |
| Tie with declarations | totals equal after declarations/Belote inflate them | Failed hand (compare totals, not card points) | N/A |
| Capot by caller | caller wins all 8 tricks (252 vs 0) | Success — not a tie, unaffected | N/A |

</frozen-after-approval>

## Code Map

- `server/internal/game/scoring.go:60` -- `failedContract := contractingTotal < opposingTotal` — the one operator to change; line 63 comment also describes the rule
- `server/internal/game/scoring_test.go:111` -- `TestHandScoring_EqualPointsNotFailure` encodes the OLD rule (80:80 → not failure); must be inverted; add explicit 81:81 case
- `server/internal/game/state.go:39` -- `HandScore.FailedContract` field — populated by `scoreHand`, only propagated downstream (no change)
- `client/src/features/match/components/ScoreReveal.tsx`, `client/src/features/profile/MatchHistory.tsx` -- render the `failedContract` boolean (no change)
- `_bmad-output/project-context.md` -- "Failed hand" rule bullet — add tie clarification
- `_bmad-output/implementation-artifacts/3-5-hand-scoring-failed-hands-and-capot.md` -- AC #3/#4, Dev Notes "Failed hand — Precise Rule" and function spec state the old rule — correct + add a dated correction note

## Tasks & Acceptance

**Execution:**
- [x] `server/internal/game/scoring.go` -- changed `<` to `<=` (now line 63) and updated the inline comment to state a tie fails the trump-calling team
- [x] `server/internal/game/scoring_test.go` -- renamed `TestHandScoring_EqualPointsNotFailure` → `TestHandScoring_EqualPointsIsFailure`, flipped assertions (caller → 0, opponents → all 160); added `TestHandScoring_TieFailsTrumpCaller` exercising an explicit 81:81 split
- [x] `_bmad-output/project-context.md` -- clarified the failed-hand bullet: trump-calling team must score strictly more than opponents; an equal total is a failed hand; noted `scoreHand` is the single decision site
- [x] `_bmad-output/implementation-artifacts/3-5-hand-scoring-failed-hands-and-capot.md` -- corrected AC #3/#4 + Dev Notes/function-spec rule wording (`<` → `<=`, "a tie IS a failure") and appended a dated Rule Correction note

**Acceptance Criteria:**
- Given the trump-calling team and opponents finish a hand with equal totals (e.g. 81:81), when scoring runs, then the calling team adds 0 and the opponents add the sum of both totals to `TeamScores`, and `LastHandResult.FailedContract` is `true`.
- Given the calling team's total is strictly greater than the opponents', when scoring runs, then normal scoring is unchanged (each team keeps its own total).
- Given the full game-package test suite, when `go test ./internal/game/...` runs, then all tests pass with no stale "equal = not failure" assertions remaining.

## Design Notes

The fix is a single operator, but it reverses a rule that story 3-5 shipped intentionally ("Equal is NOT a failure"). The risk is not the code — it's leaving contradicting tests and docs behind. The `grep` sweep confirmed `scoreHand` is the only place the rule is decided; the `FailedContract` field flows unchanged through `match/`, `user/handler.go`, `live_match.go`, and the React `ScoreReveal`/`MatchHistory` components, all of which merely render the boolean. So no downstream logic changes — only the one comparison, plus the test/doc cleanup that keeps the rule statement honest everywhere it appears.

## Verification

**Commands:**
- `cd server && go test ./internal/game/...` -- expected: ok, 0 failures (new tie test passes; inverted equal-points test passes)
- `cd server && go vet ./internal/game/...` -- expected: clean
- `cd server && grep -rn "contractingTotal" internal/game/scoring.go` -- expected: shows `<=` (sweep-back confirmation)

## Suggested Review Order

**The rule decision (start here)**

- The one-line fix: a tie now fails the trump-calling team (`<` → `<=`).
  [`scoring.go:63`](../../server/internal/game/scoring.go#L63)

**Regression tests**

- Inverted from the old "equal = not failure" rule: 80:80 now sends all 160 to the opponents.
  [`scoring_test.go:111`](../../server/internal/game/scoring_test.go#L111)

- Explicit canonical 81:81 case: caller scores 0, opponents win all 162.
  [`scoring_test.go:128`](../../server/internal/game/scoring_test.go#L128)

**Documentation alignment**

- Agents' source of truth: failed-hand rule clarified (strictly-more; tie = failure).
  [`project-context.md:327`](../project-context.md#L327)

- Dated correction note explaining the reversed rule from story 3-5.
  [`3-5-hand-scoring-failed-hands-and-capot.md:360`](3-5-hand-scoring-failed-hands-and-capot.md#L360)

## Follow-up (deferred)

- **Bitola hanging-points (carry-over) tie rule.** The tie behavior implemented here (all points to the opponents) is the **Croatian-variant** rule, applied to all variants as an interim stand-in. The **Bitola variant** should instead carry the tied hand's points over to the next decisive hand (nobody scores on the tie). Deferred to Epic 12 (Variant Expansion, Phase 3) because it needs cross-hand carry-over state and match-end interaction. Recorded in `deferred-work.md`, Epic 12 in `planning-artifacts/epics.md`, and the `project-context.md` failed-hand rule.
