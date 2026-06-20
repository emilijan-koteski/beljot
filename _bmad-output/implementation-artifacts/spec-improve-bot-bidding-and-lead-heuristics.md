---
title: 'Strengthen bot trump-bidding thresholds and memory-aware lead play'
type: 'feature'
created: '2026-06-20'
status: 'done'
baseline_commit: '00446401dfb5a6a85000ec8300bece12d07144fb'
context: ['{project-root}/_bmad-output/project-context.md']
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The bot (Story 10.3) over-bids weak trump and leaks high cards when leading. Bidding accepts 3-trump hands of `J+7+8` and a `9+A` trump pair with no off-suit support. Leading, it leaks a naked 10/King into an unseen Ace, and — when it called trump without the Jack — it leads a high trump while the master (J) sits in an unknown hand. The played-card memory to avoid this already exists (`memory.go`, `unseenCards`, `isSuitBoss`, `trumpsRemainUnseen`); the lead heuristics just don't use it tightly enough.

**Approach:** Tighten the `wantsTrump` evaluator (used by both bidding rounds) and reshape `chooseLead` to consult existing card memory before committing a high card. No new memory subsystem and no engine change — only the pure `bot.Decide` heuristics in `bot.go`, plus tests.

## Boundaries & Constraints

**Always:** Keep `bot.Decide` pure/deterministic (humanization stays in the match layer). Every returned card stays within `game.LegalCards`. One `wantsTrump` evaluator governs both bidding rounds. Use only `View` data (own hand, public state, `PlayedCards`/`KnownVoids`). Bidding runs on the 5-card stage-1 hand.

**Ask First:** Any change beyond `bot.go`/`bot_test.go`. Loosening the master-trump or safe-lead rules below.

**Never:** Touch the rules engine, validation, or `Memory`. Add ISMCTS/Monte-Carlo search. Change follow-suit play (`chooseFollow`) — only `chooseLead` and `wantsTrump` change.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Bid: length | ≥4 cards of candidate suit | Pick (take trump) | N/A |
| Bid: J + decent | 3 trumps incl. J, other two NOT both {7,8} (e.g. J,9,7) | Pick | N/A |
| Bid: J weak | exactly J,7,8 of the suit | Pass | N/A |
| Bid: 9+A backed | 3 trumps incl. 9 & A + ≥1 Ace of another suit | Pick | N/A |
| Bid: 9+A naked | 3 trumps incl. 9 & A, no off-suit Ace | Pass | N/A |
| Bid: junk | 3 trumps without J and without 9+A pair | Pass | N/A |
| Lead: draw master | Own team called trump, trumps unseen, bot holds the master (top remaining) trump | Lead that master trump (draw) | N/A |
| Lead: no master | Own team called trump but master trump is unseen; bot holds a side-suit Ace | Lead the side Ace (cash boss), NOT a trump | N/A |
| Lead: no master, no boss | Own team called trump, master unseen, no boss in hand | Lead lowest point-free card, NOT a high trump | N/A |
| Lead: 10 leak | Leading, no boss, holds a 10 whose Ace is still unseen | Lead a 0-point card, NOT the 10 | N/A |
| Lead: boss held | Holds a side-suit Ace (or master once superiors are gone) | Lead the boss (Aces first) — unchanged | N/A |

</frozen-after-approval>

## Code Map

- `server/internal/bot/bot.go` -- `wantsTrump` (bidding gate, both rounds via `decideBid`); `chooseLead` (trump-draw branch + side-boss + no-boss fallback). All changes live here.
- `server/internal/bot/bot_test.go` -- table-driven `TestDecide_Bidding` and `TestDecide_CardPlay`; cases asserting the old thresholds/leak must be updated and new ones added.
- `server/internal/bot/memory.go`, `view.go` -- existing memory the heuristics read (`PlayedCards`, `KnownVoids`); reference only, do not modify.
- `server/internal/bot/simulation_test.go` -- statistical ≥60% regression guard; must stay green (no edits expected).

## Tasks & Acceptance

**Execution:**
- [x] `server/internal/bot/bot.go` -- Rewrite `wantsTrump(hand, suit)`: keep `count >= 4 → true`; for `count == 3`, accept the Jack branch only when the other two trumps are NOT both 7 and 8 (`hasJack && !(has7 && has8)`), and accept the 9+A branch only when the hand also holds an Ace of some other suit (`hasNine && hasAce && hasSideAce`). Scan the whole hand for the off-suit Ace.
- [x] `server/internal/bot/bot.go` -- Add pure helper `holdsMasterTrump(v, trump)`: true iff the bot's highest trump is not outranked by any unseen trump (false with no trump). Gate the trump-draw branch in `chooseLead` on `myTeamCalledTrump && trumpsRemainUnseen && holdsMasterTrump`; lead the bot's highest trump (the master) when it fires.
- [x] `server/internal/bot/bot.go` -- Replace the no-boss fallback in `chooseLead` (currently "lead the strongest side card") with "lead the lowest point-value non-trump card (tie-break weakest rank)". Keep the side-suit-boss branch (Aces first) ahead of it and the only-trumps-left branch after it.
- [x] `server/internal/bot/bot_test.go` -- Update `TestDecide_Bidding`: the `J,7,8` case now expects Pass (rename/repoint), the `9,A` case with no side Ace now expects Pass; add cases for J+9+7 picks, 9+A+side-Ace picks. Update `TestDecide_CardPlay` "promoted king" case to mark both A and 10 of the suit played (so the King is a true boss); add cases for caller-without-master cashing a side Ace, caller-without-master leading safe-low, and no-boss-with-a-10 leading a 0-point card.

**Acceptance Criteria:**
- Given a stage-1 hand whose only 3 candidate-suit trumps are J,7,8, when bidding (round 1 or 2), then the bot passes; given any stronger third trump alongside the Jack, it picks.
- Given a 3-trump hand holding the 9 and Ace of trump, when the hand has no off-suit Ace, then the bot passes; with an off-suit Ace, it picks.
- Given the bot called trump and does not hold the master trump while it is still unseen, when it leads, then it never leads a trump — it cashes a side-suit Ace if held, otherwise leads its lowest point-free card.
- Given the bot leads with no boss card and holds a 10 whose Ace is unseen, then it leads a 0-point card, never the 10.
- Given the bot called trump and holds the master trump while trumps remain unseen, when it leads, then it leads that master trump to draw.
- Given any decision point, the chosen action stays within `game.LegalCards`, and the heuristic team's simulation share stays ≥60%.

## Design Notes

Off-trump order puts 10 (`NonTrumpRankOrder` 6) above K(5)/Q(4)/J(3), so a King is a boss only once BOTH the Ace and 10 of its suit are gone — hence the "promoted king" test must mark A *and* 10 played. `isSuitBoss` already rejects a 10 while its Ace is unseen; the leak was only in the fallback that led by raw rank.

Master-trump rule: `holdsMasterTrump` is true when no *unseen* trump outranks the bot's best trump — a caller holding the J (or 9 once the J is gone) keeps drawing; a caller without the top trump stops and cashes elsewhere. Reuse `lowestValue` on the non-trump subset for the no-boss lead:

```go
var nt []game.Card
for _, c := range legal { if c.Suit != trump { nt = append(nt, c) } }
if len(nt) > 0 { return lowestValue(nt, trump) } // lowest-value non-trump; never a naked 10
```

## Verification

**Commands:**
- `cd server && go test ./internal/bot/...` -- expected: all unit tests pass, including `TestSimulation_HeuristicBeatsRandomBaseline` (share logged ≥60%) and `TestDecide_AlwaysLegal`.
- `cd server && go test ./internal/game/...` -- expected: green (no engine change; confirms nothing leaked across the boundary).
- `make lint` -- expected: gofmt + golangci-lint clean.

## Suggested Review Order

**Bidding thresholds (Error fixes 1)**

- Entry point: the tightened evaluator — J+7+8 excluded, 9+A needs a side Ace; one gate for both rounds.
  [`bot.go:44`](../../server/internal/bot/bot.go#L44)

- The Jack-weakness exception — passes only when the other two trumps are exactly 7 and 8.
  [`bot.go:75`](../../server/internal/bot/bot.go#L75)

- The 9+Ace pair now requires an off-suit Ace (`hasSideAce` scanned over the whole hand).
  [`bot.go:79`](../../server/internal/bot/bot.go#L79)

**Memory-aware leads (Error fixes 2 & 1)**

- The trump-draw gate now also requires holding the master trump — fixes leaking a high trump while the J is out.
  [`bot.go:156`](../../server/internal/bot/bot.go#L156)

- New helper: master = highest trump still in play; true only when no unseen trump outranks the bot's best.
  [`bot.go:280`](../../server/internal/bot/bot.go#L280)

- No-boss fallback rewritten — lead the lowest-value non-trump instead of the strongest side card.
  [`bot.go:178`](../../server/internal/bot/bot.go#L178)

**Tests**

- Bidding cases: J+7+8 passes / J+9 picks; 9+A without vs. with a side Ace.
  [`bot_test.go:93`](../../server/internal/bot/bot_test.go#L93)

- Lead cases: no-boss-with-a-10, caller-without-master cashes side Ace vs. leads safe-low; promoted-king needs A+10 gone.
  [`bot_test.go:313`](../../server/internal/bot/bot_test.go#L313)
