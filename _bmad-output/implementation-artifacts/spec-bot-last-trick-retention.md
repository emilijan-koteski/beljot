---
title: 'Last-trick retention for bots — keep the master trump for the +10 (dix de der)'
type: 'feature'
created: '2026-06-20'
status: 'done'
baseline_commit: 'ecf39515ae9b9c7fde9487c2e236f90b817130fd'
context: ['{project-root}/_bmad-output/project-context.md']
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Entering the second-to-last trick the bot holds two cards — a card that would win the forced 8th trick (its master trump) plus a junk card. It spends the winner now to grab a cheap or already-decided trick 7, then is forced to play junk on trick 8 and hands the +10 last-trick bonus to the opponents.

**Approach:** At the endgame decision point (exactly two cards in hand) recognise when the bot holds the **master trump** — a guaranteed trick-8 winner because no opponent-reachable card outranks it — and, when the engine lets it legally hold that card back, play the OTHER card now and retain the master for trick 8. Whether the spare wins trick 7 or loses it, the master then takes the forced trick 8 and banks the +10. A pure `bot.go` refinement reusing existing memory (`threats`, `holdsMasterTrump`, `knownHeldBy`).

## Boundaries & Constraints

**Always:** Keep `bot.Decide` pure/deterministic. Every returned card stays within `game.LegalCards` (the chosen spare must be in the engine's legal set). Use `len(v.Hand)` as the endgame signal (no `TrickNumber` on the View): retention reasons only at `len(v.Hand) == 2`. Treat a card as a guaranteed trick-8 winner ONLY when the engine rules cannot force it out before trick 8 — verified by requiring the spare to be legal *this* trick, never by assumption. Existing behaviour must be byte-identical whenever no master trump is retainable.

**Ask First:** Any change beyond `bot/bot.go` and `bot/bot_test.go`. Extending retention to side-suit bosses, to `len(v.Hand) >= 3`, or adding Capot-pursuit special-casing.

**Never:** Touch the rules engine, `validation.go`, `scoring.go`, `Memory`, or `View` (no new field). Add ISMCTS/Monte-Carlo search. Change bidding (`wantsTrump`/`decideBid`) or declaration plumbing. Implement item C (Ace/10 suit-return signalling).

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Lead, retain | 2 cards = master trump + point-free junk; own team called trump | Lead the junk; retain the master (don't draw it on trick 7) | N/A |
| Non-absolute master | 2 cards = 9H + junk; every higher trump already played | `holdsMasterTrump` true ⇒ retain the 9H, play the junk | N/A |
| Forced out | 2 cards = master trump + off-suit junk; void in led ⇒ must cut | engine legal set = {master} only ⇒ `len(legal)==1` plays it; retention never overrides a forced play | N/A |
| Cut-and-keep | 2 cards = master JH + lower trump; void in led, opponent winning | play the lower trump (spare): it cuts/wins now AND keeps JH for trick 8 | N/A |
| Partner controls trick 8 | partner declared a trump above the bot's master | retention defers (returns nil) — don't fight a partner who already secures the last trick | N/A |
| No master | 2 cards, neither is the top live trump (or no trump held) | retention defers; existing lead/follow heuristics unchanged | N/A |
| Not endgame | `len(v.Hand) != 2` | retention defers; every decision identical to today | N/A |

</frozen-after-approval>

## Code Map

- `server/internal/bot/bot.go` -- add `retainLastTrickWinner(v, legal, trump) *game.Card`; call it in `chooseCard` after the `len(legal)==1` early return and before the lead/follow split. Reuses `highestOfSuit`, `holdsMasterTrump`, `knownHeldBy`. All changes live here.
- `server/internal/bot/bot_test.go` -- add `TestDecide_LastTrickRetention` (table-driven) covering the I/O matrix; reuses `viewFromState`/`NewGameMidPlay(1)` with a 2-card hand override.
- `server/internal/bot/memory.go`, `view.go` -- existing memory the helper reads (`KnownCards`, `PlayedCards`); reference only, do not modify.
- `server/internal/bot/simulation_test.go` -- ≥60% regression guard; must stay green (no edits expected — retention is pure in `bot.go`).
- `server/internal/game/{scoring.go,validation.go}` -- read-only source of truth: +10/Capot bonus, forced-card/over-trump legality.

## Tasks & Acceptance

**Execution:**
- [x] `server/internal/bot/bot.go` -- Add `retainLastTrickWinner`. Fire only when `len(v.Hand) == 2`; identify `master := highestOfSuit(v.Hand, trump, TrumpRankOrder)` and require `master != nil && holdsMasterTrump(v, trump)` (no threat trump outranks it ⇒ guaranteed trick-8 winner). Let `spare` be the other held card; require `spare` ∈ `legal` (soundness — engine may force the master out). Return nil if `knownHeldBy(partner)` holds a trump outranking `master` (team already controls trick 8). Otherwise return `spare`. Wire the call into `chooseCard`.
- [x] `server/internal/bot/bot_test.go` -- Add `TestDecide_LastTrickRetention` table covering: lead-retain (master JH + junk ⇒ play junk), non-absolute master retained after higher trumps played, forced-out (`len(legal)==1` plays the master), cut-and-keep (lower trump wins now, JH retained), partner-controls-trick-8 defer (partner declared a higher trump ⇒ existing draw behaviour), no-master/not-endgame no-op.

**Acceptance Criteria:**
- Given the bot leads the second-to-last trick holding the master trump plus a junk card, when it decides, then it plays the junk and retains the master (it does NOT draw the master on trick 7).
- Given the engine forces the master out (the spare is not legal this trick), then the bot plays the forced card and retention does not interfere.
- Given the partner is known (from the reveal) to hold a trump above the bot's master, then retention defers and the prior heuristics are unchanged.
- Given no master is retainable or `len(v.Hand) != 2`, then every decision is byte-identical to current behaviour, so the existing suite stays green.
- Given any decision point, the chosen card is within `game.LegalCards`, and `TestSimulation_HeuristicBeatsRandomBaseline` stays ≥60%.

## Spec Change Log

## Design Notes

**Master trump only, not side bosses.** Trick 8 is forced (one legal card each), so a retained card's only job is to win that one trick. A master trump wins under any lead (trump beats non-trump; nothing outranks it). An off-suit boss wins trick 8 only if it is the led suit there — which the bot cannot guarantee at trick 7 — so it is "forced to lose" when another suit leads. Side bosses are therefore excluded; the existing "cash the boss now" lead already handles them. Literal application of the carried lesson: *would win ≠ can be played to win.*

**Returning the spare is always safe.** If the spare wins trick 7 (e.g. a low trump cutting), the bot leads trick 8 and the master takes it; if the spare loses, an opponent leads trick 8 and the master (forced) still beats everything. Both branches bank the +10 — so no "current trick worth more than +10" comparison is needed: a spare that can win the current trick already grabs those points too, and a bare lead has no trick to grab. The lone risk — leading junk may bleed a partner's forced high card on trick 7 — is second-order against a guaranteed +10; left for adversarial review.

**Composition with B.** `holdsMasterTrump` and `knownHeldBy` already scan `threats` (unseen ∪ known-opponent), so a partner's declared trumps don't block the master check and the partner-controls-trick-8 guard reuses the same data. With no reveal `threats == unseenCards`, so retention degrades to played-card memory.

## Verification

**Commands:**
- `cd server && go test ./internal/bot/... ./internal/game/...` -- expected: all green, incl. `TestSimulation_HeuristicBeatsRandomBaseline` (≥60%) and `TestDecide_AlwaysLegal`.
- `cd server && gofmt -l ./internal/bot && go vet ./internal/bot/...` -- expected: no output / no findings.
- `make lint` -- expected: golangci-lint clean.

## Suggested Review Order

**Design crux — what "guaranteed trick-8 winner" means**

- Entry point: the helper's contract — master trump only, why side bosses are excluded, the +10-banked-either-way argument.
  [`bot.go:156`](../../server/internal/bot/bot.go#L156)

- The master check: the bot's best trump is the top live trump (no opponent-reachable trump outranks it).
  [`bot.go:184`](../../server/internal/bot/bot.go#L184)

**Soundness gates (highest-risk — the carried lesson)**

- Never assume a card the engine would force out: the spare must be legal this trick (belt-and-suspenders vs `len(legal)==1`).
  [`bot.go:204`](../../server/internal/bot/bot.go#L204)

- Don't fight a partner who already secures the last trick (known higher trump ⇒ defer).
  [`bot.go:207`](../../server/internal/bot/bot.go#L207)

**Wiring**

- Endgame guard sits between the `len(legal)==1` early return and the lead/follow split; deferring (nil) keeps prior behavior byte-identical.
  [`bot.go:146`](../../server/internal/bot/bot.go#L146)

**Tests (peripherals)**

- Table covers the full I/O matrix: lead-retain, non-absolute master, forced-cut, cut-and-keep, partner-defer, follow-over-partner, not-endgame, no-master.
  [`bot_test.go:482`](../../server/internal/bot/bot_test.go#L482)
