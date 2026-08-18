---
title: 'Bot secures the take when a trump lead forces the overplay'
type: 'bugfix'
created: '2026-08-18'
baseline_commit: 'd88d56317a345553aa672fdc7d7933f5f58275f1'
status: 'done'
review_loop_iteration: 3
context:
  - '{project-root}/docs/bot/BOT_RULES.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** On a **trump lead** the bot throws a losing trump instead of the guaranteed winner it holds. Reported: trump ♥, the bot sits third, its partner led Q♥ and an opponent played 7♥; holding J♥ 9♥ T♥ the bot plays **T♥** and the last opponent takes the trick with the unseen A♥ — 24 points to the opponents. Cause: `trumpEconomyTake` reserves the control trumps {J, 9} unless the pot alone clears their card value (the 9 needs >= 14; only 3 was showing), and its cheaper-alternative branch is gated to non-trump leads — so on a trump lead there is nothing to fall back on and the bot ducks into a loss.

**Approach:** On a trump lead the overplay rule forces a trump out no matter what, so declining to secure does not *preserve* the control trump — it donates a **different** trump into a trick the opponents take. Secure the take when the trick has points on the table (`trickPoints > 0`); duck when it does not, so a master is never burnt to win nothing. The ruff economy is left exactly as specced.

**Explicitly NOT a bug (measured, do not "fix"):** the same shape at **exactly two cards** (trick 7) is handled by `retainLastTrickWinner`, which banks the master and spends the spare. That is **correct play**: the master wins exactly one trick either way, so it should win **trick 8**, which carries the +10 "dix de der" and collects the opponents' last cards. Three independent measurements agree — double-dummy over 5,374 information sets (−2.54 points per changed decision, and every contest threshold worse than never contesting), minimax over 200k fully legal hands (11 better / 23 worse), and a two-trick playout (retain 44/24 vs contest 34/34). An earlier attempt to "fix" trick 7 was reverted for this reason.

## Boundaries & Constraints

**Always:**
- Change behavior only when the led suit **is** trump, and only in `trumpEconomyTake`. The four documented ruff cases in `TestDecide_TrumpEconomyTake` keep their current expected cards.
- **No inert clauses.** Every clause of the new guard must change at least one real decision — prove it by deleting each clause individually and seeing a test go red.
- The `trickPoints > 0` test is load-bearing and the callers' material-stake gate does **not** substitute for it: that gate prices the *cheapest legal card*, so a cheap-but-scoring fallback opens it on a worthless trick.
- Update `docs/bot/BOT_RULES.md` in the same change; state only claims verified by a test.
- `bot.go` stays a pure decision layer: no engine, session, or state changes.

**Ask First:**
- Any change to `controlTrumpValue`, `isControlTrump`, or the ruff `preserve` branch.
- Any change to `retainLastTrickWinner` — including "improving" it. Its known gaps (capot, leading trick 7, non-trump over-ruff) are logged in `deferred-work.md`.

**Never:**
- Do not touch `server/internal/game/validation.go` — `LegalCards` is correct.
- Do not add probability or score-state reasoning.
- Do not claim a behavior is prevented by an existing mechanism without a test that fails when that mechanism is removed.
- Do not assert a codebase-wide invariant the change does not actually establish.

## I/O & Edge-Case Matrix

Trump = ♥, bot at seat 0. Hands of 3+ cards throughout, so `retainLastTrickWinner` is gated out.

| Scenario | Input / State | Expected Behavior |
|----------|--------------|-------------------|
| Reported bug | Partner led Q♥, seat 3 played 7♥; hand J♥ 9♥ T♥ +side; A♥ unseen | **9♥** (was T♥) |
| Opponent winning, trump led | Partner Q♥, seat 3 K♥; hand J♥ 9♥ T♥ +side | **9♥** (was T♥) |
| Pointless trick, scoring fallback | Seat 3 led 8♥ (0 pts); hand J♥ Q♥ +side | **Q♥** — master kept |
| Fat pot, cheap trump available | Partner 8♥, seat 3 discards A♠ (11 pts); hand J♥ Q♥ +side | **J♥** — pot is real |
| Partner still to play behind us | Seat 3 led Q♥; hand J♥ K♥ +side | **J♥** — partner is never a threat |
| Secure winner is not a control trump | Forced over Q♥ holding K♥ A♥ J♥; seat 1 known void in trump | **K♥** — unchanged |
| Nothing securely wins | Forced over 8♥ holding K♥ Q♥; A♥ T♥ 9♥ J♥ unseen | cheapest donation — unchanged |
| No higher trump held | Overplay set degrades to all trumps, none overtaking | Nothing `securelyWins`; guard inert |
| Void in trump on a trump lead | Legal set is the whole all-non-trump hand | Nothing can beat a trump; guard inert |
| Any non-trump lead (ruff) | The four pre-existing ruff cases | Byte-identical |
| **Trick 7, two cards, trump lead** | Hand J♥ T♥, partner Q♥ + 7♥ | **T♥** — retention banks the master; verified over both tricks as the better line |

</frozen-after-approval>

## Code Map

- `server/internal/bot/bot.go` -- `trumpEconomyTake` (~line 1288): the single fix site. The new branch nests under a shared `secure != nil` with the pre-existing non-control early return, so the nil check is structural rather than an inert clause; reaching the second branch implies `secure` is a control trump.
- `server/internal/bot/bot.go` -- `chooseFollow` (~line 389): both call sites. Their stake gate is `trickPoints + cardPoints(fallback) > 0` — it prices the cheapest legal card, NOT the pot. Read-only.
- `server/internal/bot/bot.go` -- `retainLastTrickWinner` (~line 219): **read-only.** Runs before `chooseFollow` at exactly two cards, which is why the guard is unreachable there on a trump lead.
- `server/internal/bot/bot_test.go` -- `TestDecide_TrumpEconomyTake` (~1302), `runPlayTweakCases` (~746; its case struct uses `observes`, not `played`).
- `docs/bot/BOT_RULES.md` -- "Trump economy on the take" (~359), endgame retention B1 (~187), tuning lever 5, blind spots, quick map.

## Tasks & Acceptance

**Execution:**
- [x] `server/internal/bot/bot.go` -- in `trumpEconomyTake`, nest the pre-existing non-control early return and a new branch under one `secure != nil`; the new branch returns `secure` when the led suit is trump **and** `trickPoints(v, trump) > 0`.
- [x] `server/internal/bot/bot_test.go` -- cover every matrix row in `TestDecide_TrumpEconomyTake`.
- [x] `server/internal/bot/bot_test.go` -- add an end-to-end `game.ApplyAction` regression asserting hand points, with two sub-tests: the reported mid-hand position (bot secures, team banks 28), and the trick-7 position played through **both** tricks, showing retention's line banks more (44/24) than contesting would (34/34). The second exists to stop a future agent "fixing" trick 7 again.
- [x] `docs/bot/BOT_RULES.md` -- document the trump-lead threshold; note in the retention section that trick 7 is deliberately left alone and why; keep lever 5's "ruff-only" wording consistent with the step list; do not restate the pointless-trick behavior as a codebase-wide invariant.

**Acceptance Criteria:**
- Given the reported position at 3+ cards, when the bot plays third, then it plays a card no yet-to-play opponent can beat, and the end-to-end test confirms its team banks the trick points.
- Given a trump-led trick with `trickPoints == 0`, when the bot could secure with a J or 9, then it does not.
- Given a trump-led trick at exactly two cards, then behavior is **identical to before this change** at every `atRisk` value.
- Given any non-trump lead, then the chosen card is identical to before this change.
- Given each clause of the new guard is deleted individually, when the suite runs, then at least one test fails for every clause.
- Given `make test` and `make lint`, when run, then both pass with no new failures.

## Spec Change Log

- **Loop 1 (intent_gap):** the approved approach secured on a trump lead unconditionally, burning a master on zero-point tricks. **Amended:** added `trickPoints > 0`. **Rejected:** pricing the donation into the ruff gate (`3 + 10 = 13 < 14` leaves the reported bug open).
- **Loop 2 (intent_gap):** scope was widened to `retainLastTrickWinner`, delegating its decision to `chooseFollow`. Unsound — `trumpEconomyTake`'s non-control early return has no points gate, so a non-control master was spent on a zero-point trick (14-point swing). Two of the guard's four clauses were also provable no-ops. **Amended:** retention priced the +10 itself.
- **Loop 3 (intent_gap) — scope REDUCED:** measurement showed the whole trick-7 premise was wrong. The master wins one trick either way, so banking it for the +10 trick is correct; every contest threshold scored below never contesting. **Amended:** `retainLastTrickWinner` is out of scope and explicitly protected by an `Ask First` boundary plus a two-trick regression test. Guard 1 survives unchanged.
  - **KEEP:** guard 1 and its nested position, verified across three review passes — aggregate 92.35 with it vs 88.18 without; no inert clauses; non-trump leads byte-identical across 3,862 information sets; the nested restructure byte-identical to the flat form across 9,236 sets. Keep the three-shapes legal-set argument (strictly-higher trumps / all trumps none overtaking / void-in-trump whole hand), which matches `legalCards` exactly.
  - **Known-bad avoided:** any guard inside `retainLastTrickWinner`; any single shared threshold across the two sites; any clause that reads protective but never discriminates.

## Design Notes

Reaching the new branch implies `secure` is a control trump, because the shared `secure != nil` block returns any non-control secure winner first. That early return has **no points gate**, which is precisely why the trick-7 path must not be routed through this function.

```go
if secure != nil {
	if !isControlTrump(*secure, trump) {
		return secure
	}
	if v.CurrentTrick[0].Card.Suit == trump && trickPoints(v, trump) > 0 {
		return secure
	}
}
```

## Verification

**Commands:**
- `go test ./internal/bot/ -run 'TestDecide' -v` (from `server/`, mise-shimmed go) -- expected: all pass, four ruff cases unmodified
- Clause mutation: delete `led == trump`, then `trickPoints > 0` -- expected: a red test for each
- `go test ./...`, `make test`, `make lint` -- expected: all green

## Suggested Review Order

**The fix**

- Entry point: the whole behavior change, 5 net lines. Reaching the inner branch implies a control trump.
  [`bot.go:1273`](../../server/internal/bot/bot.go#L1273)

- The threshold itself. Without `trickPoints > 0` a Jack is burnt on a worthless trick.
  [`bot.go:1286`](../../server/internal/bot/bot.go#L1286)

**What was deliberately NOT changed**

- Why trick 7 is left alone; two attempts to "fix" it were measured worse and reverted.
  [`bot.go:220`](../../server/internal/bot/bot.go#L220)

- Retention's executable code is byte-identical to baseline — comment only.
  [`bot.go:227`](../../server/internal/bot/bot.go#L227)

**Tests**

- The reported position, end to end, asserting banked points rather than tricks won.
  [`bot_test.go:1842`](../../server/internal/bot/bot_test.go#L1842)

- Two-trick playout pinning retention as correct (44/24 vs 34/34) — blocks a future "fix".
  [`bot_test.go:1889`](../../server/internal/bot/bot_test.go#L1889)

- The reported bug as a table case.
  [`bot_test.go:1372`](../../server/internal/bot/bot_test.go#L1372)

- Guards the threshold: 0-point trick with a scoring fallback keeps the master.
  [`bot_test.go:1399`](../../server/internal/bot/bot_test.go#L1399)

**Docs**

- The trump-lead rule, and why the callers' stake gate cannot substitute for it.
  [`BOT_RULES.md:419`](../../docs/bot/BOT_RULES.md#L419)

- Corrects which steps are ruff-gated — step (5) is what holds the master on a pointless trick.
  [`BOT_RULES.md:415`](../../docs/bot/BOT_RULES.md#L415)

- Records the measurement that trick 7 is correct as-is, so it is not "fixed" again.
  [`BOT_RULES.md:199`](../../docs/bot/BOT_RULES.md#L199)
