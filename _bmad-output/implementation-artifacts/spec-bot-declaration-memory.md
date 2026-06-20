---
title: 'Declaration memory for bots — stop fighting a partner who holds the revealed winners'
type: 'feature'
created: '2026-06-20'
status: 'done'
baseline_commit: '6a161a37e4a87be854157e383f668c554840a33f'
context: ['{project-root}/_bmad-output/project-context.md']
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** When declarations resolve, the winning team's exact cards become public, but bots ignore them. So a bot still leads/overtrumps into a partner who provably holds the top cards — e.g. opponents void of trump, partner declared a trump run, the bot holds the leftover trumps and drains them instead of letting the partner cash. Goal A's master-trump lead is also over-conservative: it won't draw while the master is "unseen," even when the *partner* declared it, because the redacted view can't tell a partner's Jack from an opponent's.

**Approach:** Add a per-hand "known holdings by seat" memory dimension fed from revealed declarations, expose it on `bot.View`, and let the lead/follow heuristics treat a known card as no longer unseen — distinguishing a *partner's* known card (not a threat) from an *opponent's* (still a threat). This lets the bot stop drawing/overtrumping a partner's winners and relaxes the master-trump rule once the partner has declared the top.

## Boundaries & Constraints

**Always:** Keep `bot.Decide` pure/deterministic — all side effects (recording revealed cards into `Memory`) stay in the match layer. Respect Bitola no-peeking: a bot may learn a seat's exact declared cards **only after** `DeclarationsResolved` is true, and the engine has already nil'd the losing team's `Declarations` by then — so reading post-resolution `Players[seat].Declarations` exposes only the publicly-revealed winning team's cards. Every returned card stays within `game.LegalCards`. Existing behavior must be unchanged whenever no declaration has been revealed (empty known-holdings ⇒ identical to today).

**Ask First:** Any change beyond `bot/memory.go`, `bot/view.go`, `bot/bot.go`, `match/bot_driver.go`, and their tests. Changing the engine, validation, or declaration resolution.

**Never:** Touch the rules engine / `declarations.go` / validation (read-only source of truth). Read pre-resolution declarations into the view. Add Monte-Carlo/ISMCTS search. Change bidding (`wantsTrump`/`decideBid`). Implement items C or D from the deferred-work note.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Pre-reveal | `DeclarationsResolved == false` | `View.KnownCards` empty; every decision identical to today | N/A |
| Bot team won | resolved, partner declared | `KnownCards[partner]` = partner's exact cards; opponent entries empty | N/A |
| Opponents won | resolved, opponents declared | `KnownCards[oppSeats]` = their cards; bot-team entries empty | N/A |
| Don't drain partner | own team called, opponents void of trump, partner declared the high trumps, bot holds leftover trumps + a side card | `trumpsRemainUnseen` false ⇒ no trump-draw lead; cash side boss / lead lowest non-trump | N/A |
| Relaxed draw | opponents hold lower trumps, partner declared the master (J), bot holds 2nd-best trump | `holdsMasterTrump` true vs threats ⇒ bot draws with its best trump | N/A |
| No false boss | opponents declared A♠; bot holds K♠ | `isSuitBoss(K♠)` stays false (A♠ is a known opponent threat); bot won't lead it as a boss | N/A |
| Played declared card | a declared card already played this hand | excluded by `knownHeldBy`; not re-counted as a holding or threat | N/A |
| Don't overtrump partner's win | opponent winning; bot plays before its partner (last seat); partner known to hold a threat-proof beater | bot does not overtake — smears highest-point non-overtaking, else lowest | N/A |

</frozen-after-approval>

## Code Map

- `server/internal/bot/memory.go` -- add `declared [4][]game.Card`; `ObserveDeclarations(players [4]game.PlayerState)`; `KnownCards() [4][]game.Card`; clear `declared` in `SyncHand`.
- `server/internal/bot/view.go` -- add `KnownCards [4][]game.Card` to `View` (cards known held per seat; only winning team populated, post-reveal).
- `server/internal/bot/bot.go` -- consume known holdings: exclude them from `unseenCards`; add `threats` (unseen ∪ known-opponent-held) and route `isSuitBoss`/`holdsMasterTrump`/`trumpsRemainUnseen`/`partnerWinIsSafe` through it; add the partner-will-win follow branch.
- `server/internal/match/bot_driver.go` -- `buildBotView` sets `v.KnownCards = mem.KnownCards()`; `observeBotMemory` calls `mem.ObserveDeclarations(newState.Players)` when `newState.DeclarationsResolved`.
- `server/internal/bot/bot_test.go` -- mirror `KnownCards` in `viewFromState`; add declaration-memory card-play cases.
- `server/internal/bot/simulation_test.go` -- `playOneHand` must call `mem.ObserveDeclarations(gs.Players)` once resolved, else the new memory never activates in the ≥60% guard.

## Tasks & Acceptance

**Execution:**
- [x] `server/internal/bot/memory.go` -- Add `declared [4][]game.Card`. `ObserveDeclarations(players [4]game.PlayerState)`: for each seat, set `declared[seat]` to the flattened `Players[seat].Declarations[*].Cards` (clone). `KnownCards() [4][]game.Card`: return a clone. `SyncHand` resets `declared` alongside `played`/`voids`.
- [x] `server/internal/bot/view.go` -- Add `KnownCards [4][]game.Card` with a doc comment noting it carries only publicly-revealed (winning-team) cards, indexed by seat.
- [x] `server/internal/bot/bot.go` -- Add `knownHeldBy(v, seat)` = `KnownCards[seat]` minus already-played and current-trick cards. Exclude all known-held cards from `unseenCards`. Add `threats(v)` = `unseenCards(v)` ∪ known-held cards of opponent seats. Switch `isSuitBoss`, `holdsMasterTrump`, `trumpsRemainUnseen`, and `partnerWinIsSafe` to scan `threats(v)` instead of `unseenCards(v)`. In `chooseFollow`'s opponent-winning branch, before `cheapestWinning`: if the bot's partner is yet to play, is provably void in the led suit (`KnownVoids`), and `knownHeldBy(v, partner)` holds a **trump** that beats the current winner and is unbeatable by any `threats` card, smear `highestPointNonOvertaking` (else `lowestValue`) instead of taking. (The partner is always last in this geometry; the void+trump guard makes the ruff *legal and certain* — a mere led-suit beater is unsound, since the opponent between them could ruff and force the suit-bound partner under.)
- [x] `server/internal/match/bot_driver.go` -- `buildBotView`: `v.KnownCards = mem.KnownCards()` under the `mem != nil` guard. `observeBotMemory`: after `SyncHand`, `if newState.DeclarationsResolved { mem.ObserveDeclarations(newState.Players) }`.
- [x] `server/internal/bot/bot_test.go` -- Mirror `KnownCards: mem.KnownCards()` in `viewFromState`. Add `TestDecide_CardPlay` cases (set `gs.Players[seat].Declarations` + call `mem.ObserveDeclarations(gs.Players)`): don't-drain-partner, relaxed-draw, no-false-boss-vs-opponent-declaration, partner-will-win follow duck, played-declared-card-not-double-counted.
- [x] `server/internal/bot/simulation_test.go` -- In `playOneHand`, after each `ApplyAction`, `if gs.DeclarationsResolved { mem.ObserveDeclarations(gs.Players) }` so the heuristic team exercises declaration memory.

**Acceptance Criteria:** (system-level; per-scenario behaviors live in the I/O matrix)
- Given no declaration has been revealed, when any decision is made, then it is byte-identical to current behavior (`KnownCards` empty ⇒ `threats == unseenCards`), so the existing Goal A test suite stays green unchanged.
- Given the losing team's declarations, then they never reach the View (engine nils them pre-`KnownCards`); the bot only ever knows the publicly-revealed winning team's cards.
- Given any decision, the chosen card stays within `game.LegalCards`, and `TestSimulation_HeuristicBeatsRandomBaseline` stays ≥60%.

## Spec Change Log

- **Review (adversarial + edge-case hunters), iteration 1 — `partnerTakesTrick` was unsound (HIGH).** Triggering finding: the follow-side duck counted any declared partner card that *beats* the current winner, ignoring whether the partner could *legally play it*. A declared off-suit/trump beater is unplayable when the partner still holds the led suit (must follow), so the bot would smear a high card straight into the opponent's trick (feeding points). Amendment (patch, code-local — the other five files passed all three reviews clean, so no full re-derive): the duck now fires only when the partner is provably void in the led suit (`KnownVoids`) and holds a threat-proof **trump** — provably sound because the partner is always last in this geometry. Added a defensive empty-trick guard. Known-bad state avoided: ducking a trick the partner cannot actually take. **KEEP:** the lead-side work (threats refactor, `trumpsRemainUnseen`/`holdsMasterTrump`/`isSuitBoss` routing, don't-drain + relaxed-draw + no-false-boss behaviors) was confirmed correct and must survive — only `partnerTakesTrick` and its tests changed.

## Design Notes

The engine already does the Bitola redaction: `resolveDeclarationsForHand` nils the *losing* team's `Players[seat].Declarations` and sets `DeclarationsResolved = true` (declarations.go:457-465). So `ObserveDeclarations(newState.Players)`, called only when resolved, snapshots exactly the public reveal — no team-relationship logic in the plumbing. `startNewHand` resets the fields (scoring.go:176/200) and `SyncHand` clears `declared`, so nothing leaks across hands.

The crux in `bot.go` is the **two senses of "unseen"**: `unseenCards` = cards in an *unknown* hand; `threats` = cards an *opponent* could play = `unseenCards ∪ known-opponent-held`. A partner's known card leaves both threat scans (don't fight it); an opponent's stays a threat (no false boss/master). When no card is known, `threats == unseenCards`, so the refactor is a no-op on today's tests.

The follow-side duck (`partnerTakesTrick`) is sound only because *knowing a card beats the winner ≠ the partner can play it*. The partner (seat+2) is yet to play only when the bot sits at the leader's left, and then the partner is **last** with one opponent between them. The duck therefore fires only when the partner is provably void in the led suit (forced to ruff, not to follow into a loss) and holds a threat-proof **trump** — anything weaker risks feeding the smear straight to the opponent.

## Verification

**Commands:**
- `cd server && go test ./internal/bot/... ./internal/game/...` -- expected: all green, incl. `TestSimulation_HeuristicBeatsRandomBaseline` (logged share ≥60%) and `TestDecide_AlwaysLegal`.
- `cd server && gofmt -l ./internal/bot ./internal/match && go vet ./internal/bot/... ./internal/match/...` -- expected: no output / no findings.
- `make lint` -- expected: golangci-lint clean.

## Suggested Review Order

**Design crux — the two senses of "unseen"**

- Entry point: the partner-vs-opponent split that makes the whole feature sound.
  [`bot.go:369`](../../server/internal/bot/bot.go#L369)

- Known cards are no longer in an "unknown" hand; drops them from the deck scan.
  [`bot.go:315`](../../server/internal/bot/bot.go#L315)

- Cards still in a seat's hand: revealed declarations minus already-played.
  [`bot.go:344`](../../server/internal/bot/bot.go#L344)

**Known-holdings memory (plumbing)**

- Snapshots only the public reveal — engine pre-nils the losing team.
  [`memory.go:59`](../../server/internal/bot/memory.go#L59)

- New per-seat known-cards dimension, cleared on hand advance.
  [`memory.go:19`](../../server/internal/bot/memory.go#L19)

- Exposed on the redacted View (winning team only).
  [`view.go:52`](../../server/internal/bot/view.go#L52)

- Records once `DeclarationsResolved`; the engine has already redacted losers.
  [`bot_driver.go:256`](../../server/internal/match/bot_driver.go#L256)

- Populates `View.KnownCards` from memory.
  [`bot_driver.go:232`](../../server/internal/match/bot_driver.go#L232)

**Lead heuristics (don't drain the partner / relaxed master)**

- Opponents void of trump ⇒ nothing to draw ⇒ stop leading trump at the partner.
  [`bot.go:303`](../../server/internal/bot/bot.go#L303)

- Relaxed: a partner's declared high trump no longer blocks the bot from drawing.
  [`bot.go:387`](../../server/internal/bot/bot.go#L387)

- A known opponent Ace stays a threat — no false boss.
  [`bot.go:404`](../../server/internal/bot/bot.go#L404)

**Follow heuristic (highest-risk — review fix lives here)**

- Don't overtake a trick the partner is guaranteed to take.
  [`bot.go:232`](../../server/internal/bot/bot.go#L232)

- Sound only when the partner is provably void in led + holds a threat-proof trump.
  [`bot.go:257`](../../server/internal/bot/bot.go#L257)

**Tests (peripherals)**

- Test mirror of `buildBotView` gains `KnownCards`.
  [`bot_test.go:40`](../../server/internal/bot/bot_test.go#L40)

- Sound/negative/stale cases for the follow duck.
  [`bot_test.go:421`](../../server/internal/bot/bot_test.go#L421)

- The ≥60% guard now exercises declaration memory.
  [`simulation_test.go:87`](../../server/internal/bot/simulation_test.go#L87)
