---
title: 'Bot: secure the take when forced over a partner, and stop smearing unprotected boss cards'
type: 'bugfix'
created: '2026-07-15'
status: 'done'
baseline_commit: 'b20fdae99131de20240b6f41476fae21bdcbd1a3'
review_loop_iteration: 2
context: ['{project-root}/_bmad-output/project-context.md']
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Two trick-play leaks in the bot heuristics (`bot.go`). (1) Taking a trick with an opponent still to play, the bot picks the cheapest overtaker with no regard for unseen higher cards: trump led, partner Q-trump winning, opp 8-trump, bot 3rd forced to overplay plays the K while the 10 is unseen — donating the trick despite holding sure winners (A/9/J). (2) Smearing onto a partner's won trick, `highestPointNonOvertaking` maximizes points blindly and throws an unprotected boss (a side-suit Ace without its Ten), surrendering future control of that suit.

**Approach:** Add a "secure winner" notion — a winning card no card an opponent could still play (`threats`) beats, or any winner when the bot closes the trick. When taking a trick (partner-unsafe fall-through AND the cheapest-winning path), prefer the cheapest secure winner; fall back to current behavior when none exists. **Material-stake gate (human-added 2026-07-15):** the secure upgrade applies only when there are points at stake — current trick points plus the cheapest winner's own points > 0; on pointless tricks the bot contests cheaply as before and never burns a master to secure nothing. A secure winner is also never spent over a partner who is GUARANTEED to take the trick (extends the existing `partnerTakesTrick` duck to trump-led tricks, where the overplay rule forces a partner's known threat-proof trump to win). Guard all smear sites: never smear a boss/master unless another held card of its suit is also a boss after it leaves (Ace-backed-by-Ten, generalized); smear the highest-point boss-safe card instead. **Endgame exception (human-added 2026-07-15):** at the second-to-last trick (2 cards in hand), a non-trump boss is guarded ONLY when it is provably uncuttable (opponents out of trump); if an opponent may still hold a trump, the boss is smeared now — hoarding it just donates its points to the trick-8 ruff. Pure heuristics change in `bot.Decide` only, plus tests.

## Boundaries & Constraints

**Always:** Keep `bot.Decide` pure/deterministic. Every returned card stays within `game.LegalCards`. Reuse existing memory machinery (`threats`, `cardIsBoss`, `isSuitBoss`, `isTrumpMaster`) — no new memory subsystem. The simulation regression guard (heuristic team share ≥60%) stays green.

**Ask First:** Any change beyond `bot.go`/`bot_test.go`. Narrowing the boss-guard to Aces only (see Design Notes — the general boss rule flips 5 existing test expectations). Weakening any existing rule (draw gates, Rule 8 trigger conditions, retention).

**Never:** Touch the rules engine, validation, or `Memory`. Change `chooseLead` or `retainLastTrickWinner`. Add search/simulation-based play.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Forced overplay, secure winner held | Trump led, partner Q-trump winning, opp 8-trump; bot 3rd holds K,A,9,J of trump; 10-trump unseen | Play A (cheapest card the unseen 10 cannot beat), never K | N/A |
| Forced overplay, no secure winner | Same shape, but bot's trumps are all below the unseen top trumps | Cheapest overtaker (minimal donation) — current behavior | N/A |
| Opponent winning, secure winner exists | Bot 2nd/3rd, points at stake (trick + cheap winner > 0), cheapest winner beatable but a secure winner is legal | Cheapest secure winner, not the beatable cheap one | N/A |
| Opponent winning, nothing secure | Ruff threat or higher unseen card covers every winner | Current `cheapestWinning` gamble unchanged | N/A |
| Junk trick, secure winner exists | 0 points in the trick and on the cheapest winner (e.g. forced ruff of a 7-lead holding J+8 of trump) | Cheapest (beatable) winner — the master is not burnt to secure nothing | N/A |
| Partner forced to take (trump led) | Opponent leads trump and winning; partner yet to play with a declared threat-proof higher trump (overplay rule forces it) | Dump/smear low — never spend a secure winner over the partner's guaranteed take | N/A |
| Smear: unprotected boss | Partner wins for sure, bot last, void in led+trump, holds boss A♠ without T♠ | Smear the highest-point NON-boss card; keep the Ace | N/A |
| Smear: protected boss | Same, but bot holds A♠ AND T♠ | Smear the A♠ (Ten becomes the new boss) — unchanged | N/A |
| Smear: only unprotected bosses legal | Every non-overtaking card is an unprotected boss | Fall back to highest-point (current behavior) | N/A |
| Smear: endgame cuttable boss | Trick 7 (2 cards in hand), partner wins for sure, unprotected boss A♠, an opponent may still hold a trump | Smear the A♠ — bank the 11 pts; hoarding donates them to the trick-8 ruff | N/A |
| Smear: endgame uncuttable boss | Same, but opponents provably out of trump | Guard holds — keep the A♠ to win trick 8 | N/A |
| Duck stays a duck | Partner contestable, bot holds non-overtaking cards, no secure winner | `lowestValue` duck unchanged | N/A |

</frozen-after-approval>

## Code Map

- `server/internal/bot/bot.go` -- `chooseFollow` (partner-unsafe fall-through `bot.go:425`, `cheapestWinning` call `bot.go:454`); smear call sites `bot.go:399/418/432`; helpers `threats`, `cardIsBoss`, `beatsCard`, `seatsYetToPlay`, `highestPointNonOvertaking`, `cheapestWinning`. All changes live here.
- `server/internal/bot/bot_test.go` -- table-driven card-play tests; six fixtures redesigned point-differentiated under the boss-guard (listed in Tasks), new cases added.
- `server/internal/bot/simulation_test.go` -- statistical ≥60% regression guard; must stay green, no edits expected.
- `server/internal/bot/memory.go`, `view.go` -- read-only reference (`PlayedCards`, `KnownVoids`, `KnownCards`).

## Tasks & Acceptance

**Execution:**
- [x] `server/internal/bot/bot.go` -- Add `securelyWins(v, c, trump) bool`, **seat-aware** like `partnerWinIsSafe`: false when `len(v.CurrentTrick) == 0` (defensive) or unless `wouldOvertake(c, ...)`; true when the bot closes the trick (`len(v.CurrentTrick) == 3`); else true iff for every OPPONENT seat in `seatsYetToPlay(v)`, no card that seat could still play beats `c` (via `beatsCard` with the led suit). "Could still play" for a seat = cards in `knownHeldBy(v, seat)`, plus every card in `unseenCards(v)` whose suit the seat is NOT known void in (`v.KnownVoids`). Cards known-held by a seat that already acted this trick, and unseen suits a remaining opponent is provably void in, are NOT threats. A seat whose `knownHeldBy` contains a led-suit card is FORCED to follow suit — only its led-suit cards count as threats (its trumps are unplayable this trick). Add `cheapestSecureWinning(v, legal, trump) *game.Card`: like `cheapestWinning` but only over cards where `securelyWins` holds.
- [x] `server/internal/bot/bot.go` -- In `chooseFollow`, partner-winning-unsafe fall-through (`bot.go:425`): only when EVERY legal card overtakes (`highestPointNonOvertaking == nil` — makes the code match its forced-overplay rationale and stays correct if a future variant relaxes the iber rule) AND the material stake is positive, return `cheapestSecureWinning` when non-nil; else `lowestValue`. In the opponent-winning path, try `cheapestSecureWinning` before `cheapestWinning` only when the stake is positive. **Stake** = summed `cardPoints` of `v.CurrentTrick` + `cardPoints` of the `cheapestWinning` candidate (in the partner branch: of the `lowestValue` pick); stake 0 → old behavior. Extend `partnerTakesTrick` to trump-led tricks: when `led == trump`, drop the void-in-led requirement — the partner must follow trump and the overplay rule forces its known beater out — keeping the existing threat-proof check; its branch already precedes the secure-take, so a guaranteed partner win ducks/smears instead of burning a secure winner. In that branch, when `bestSmear` is nil (every legal card overtakes but the partner still takes the trick), fall back to `strongestPreservingBoss` — the card lands in the team's trick, so bank the high points while preserving a boss/master — not `lowestValue`.
- [x] `server/internal/bot/bot.go` -- Add `bestSmear(v, legal, trump) *game.Card` (nil on an empty trick, defensive): among non-overtaking legal cards, pick the highest-point card that is boss-safe — NOT `cardIsBoss`, or another held card of the same suit is also `cardIsBoss` once it leaves (with `c` in our hand, `cardIsBoss(d)` already ignores `c` as a threat, so the check is: exists `d != c` in `v.Hand`, same suit, `cardIsBoss(v, d, trump)`). Ties on points smear the WEAKER rank — keep the stronger card, consistent with `cheapestSecureWinning`. The unprotected-boss skip does NOT apply when the boss cannot convert to a future trick: **(a) endgame** — `len(v.Hand) == 2`, non-trump boss, `!opponentsOutOfTrump(v, trump)`; **(b) dead suit** — some opponent seat is known void in the boss's suit (`v.KnownVoids`) AND `opponentMayHoldTrump(v, seat, trump)` (the near-certain-ruff test `shouldSmearOntoPartnerBoss` already uses). Trump masters stay guarded (they cannot be ruffed). When every candidate is an unprotected boss, return the plain `highestPointNonOvertaking` pick (PO-confirmed 2026-07-15: highest points, unchanged old behavior); nil when no candidate. Replace `highestPointNonOvertaking` with `bestSmear` at the three smear sites (safe smear, Rule 8 risk-smear, `partnerTakesTrick` smear).
- [x] `server/internal/bot/bot_test.go` -- Every smear fixture must be DISCRIMINATING the honest way: the expected card must differ from the duck (`lowestValue`) output via POINT-differentiated candidates (e.g. a J or Q among 0-pt cards), never via a tie-break bent for observability. Flipped/redesigned fixtures (trump Hearts): "smear keeps the unprotected ace boss when partner trumped" hand AS,QS,7S → QS (duck 7S); "smear low and keep the promoted ten when partner cashes the ace" hand TS,JS,8S → JS (duck 8S); "risk-smear keeps the promoted ten" hand TS,JS,7S,8C → JS (duck 7S); "smear a boss-safe card from another suit when void in led and trump" hand AD,QC,7C → QC (duck 7C); "safe partner win smears the high boss-safe card" hand TS,JS,9S → JS (duck 9S); "smear into a void opponent that cannot ruff" hand KS,7S,8C → KS. New cases: Q/8 forced overplay holding K,A,9,J → A; forced overplay, nothing secure → cheapest; secure ruff with stake (KS led, JH seen) → 9H over 8H; **seat-aware**: forced overplay holding K,A,J with the remaining opponent void in trump → K; a declared JH held by the ALREADY-ACTED leader does not veto the 9H ruff; **stake gate**: forced ruff of a pointless 7D lead holding JH,8H → 8H (master not burnt); **partner forced take (trump led)**: opponent leads KH winning, partner declared JH yet to play, bot holds 9H,TH → TH (dump, never the 14-pt 9H); **trump-master smear**: under partner's closed JH-ruff trick holding 9H,QH,8H → QH (master 9H kept; duck 8H); holding 9H,AH → 9H (both protected, canonical 9-under-Jack); **partnerTakesTrick boss-guard**: hand TD,JD,7D under a led AD → JD (promoted TD kept; duck 7D); A smeared when its Ten is held; all-unprotected fallback → highest-point boss; **endgame**: 2 cards AS+8C, partner safely won, trumps possibly with opponents → AS (banked); same with both opponents void in trump → 8C (AS kept for trick 8; pairs with the previous case as the branch discriminator); **dead suit**: hand AD,QC,7C with both opponents void in diamonds and trump outstanding → AD (banked, not hoarded); **follow-suit security**: bot ruffs a non-trump lead with 9H,8H while the remaining opponent's reveal shows it holds a led-suit card (forced to follow) plus a higher trump → 8H (the cheap ruff is provably secure; without the follow-suit filter the bot burns the 9H); **negative trump-led partner take**: opponent leads KH, partner declared only AH with JH/9H unseen (beater not threat-proof) → `partnerTakesTrick` false, normal take path; **forced overtake under guaranteed partner take**: KH,TH forced overplays under the partner's threat-proof declared JH → TH (bank 10 via `strongestPreservingBoss`, not the 4-pt KH); relocate/rename the 9H,TH dump case so its coverage is attributed to the `partnerTakesTrick` branch, not the secure-winner test.

**Acceptance Criteria:**
- Given the bot must or chooses to take a contested trick with an opponent still to play, when it holds a card no remaining opponent card can beat, then it plays the cheapest such card and never a cheaper beatable one.
- Given no legal card is a secure winner, when taking or forced to overplay, then behavior is unchanged from today (cheapest winner / lowest value).
- Given a smear opportunity, when the highest-point candidate is a boss without a same-suit boss backup in hand, then the bot smears the best non-boss candidate instead and keeps the boss.
- Given every decision point, the chosen card stays within `game.LegalCards`, and the simulation share stays ≥60%.

## Spec Change Log

- **2026-07-15, loop 1 (bad_spec):** Both review hunters found (one with an executable probe) that the specified `securelyWins` was seat-blind: scanning raw `threats(v)` counts cards known-held by an opponent who already acted this trick and unseen suits the only remaining opponent is provably void in — so the bot burned the Jack where the King was already guaranteed (opponent void in trump), missing exactly the donation this spec exists to prevent. The frozen intent says "no card an opponent could STILL play"; the task now requires seat-awareness via `seatsYetToPlay` + `knownHeldBy` + `KnownVoids`, mirroring `partnerWinIsSafe`. Known-bad state avoided: seat-blind threat scan. Also folded in (coverage findings, would otherwise be patches): two flipped fixtures were vacuous (expected card equal to the duck output — fixtures must discriminate), trump-master smear behavior had zero coverage, and the `partnerTakesTrick` smear site had no boss-guard test. **KEEP:** helper names and call-site placement (`securelyWins`/`cheapestSecureWinning` before `lowestValue` and `cheapestWinning`; `bestSmear` at all three smear sites); `bestSmear` points-then-higher-rank tie-break and unprotected-boss fallback to `highestPointNonOvertaking`; the AS→QS, AD→8C, TS→9S flips and the KS adaptation of "smear into a void opponent that cannot ruff"; the three secure-take cases (AH forced overplay, QH minimal donation, 9H after JH seen) and both boss-smear cases (A protected by T, all-unprotected fallback); game-reasoning comment style.

- **2026-07-15, loop 2 (human intent update):** Emilijan pulled the deferred boss-guard refinement into scope: at the second-to-last trick, an unprotected non-trump boss that an opponent could still ruff must be smeared, not hoarded — hoarding donates its points on the forced trick 8. Implemented as: guard skipped when `len(v.Hand) == 2 && c.Suit != trump && !opponentsOutOfTrump(v, trump)` (reuses the `isUncuttableBoss` machinery's core check; "provably retains a trump" read conservatively as "not provably out of trump" so the uncertain case banks the points too). Frozen block amended by human instruction. The matching deferred-work entry is retired. **KEEP:** everything from the loop-1 KEEP list plus the seat-aware `securelyWins` and all loop-1 test additions.

- **2026-07-15, loop 2 (bad_spec, review round 2 + PO answers):** Round-2 hunters (validation-aware) found: (1) the secure-take burns master trumps to win pointless tricks (proven: J spent ruffing a 7-lead) — PO chose a material-stake gate (trick points + cheap winner points > 0); (2) on trump-led tricks a partner with a declared threat-proof trump is FORCED by the overplay rule to win, yet the bot spent its 9 "securely" — fixed by extending `partnerTakesTrick` to trump-led; (3) the boss-guard hoarded provably dead bosses (suit void with a ruff-capable opponent) — dead-suit skip added alongside the PO's endgame rule; (4) the points-tie smear tie-break favored the stronger card only to keep tests observable — inverted to keep-stronger, with fixtures redesigned point-differentiated; (5) three fixtures could not detect the smear branch being skipped (expected = duck output) — redesigned; (6) missing empty-trick defensive guards — added; (7) the reviewer's "give the cheapest boss in the all-unprotected fallback" was REJECTED by the PO — highest-point stands. Known-bad states avoided: master-burn on junk tricks; 9-under-forced-J waste; dead-boss hoarding; observability-driven heuristics. **KEEP:** loop-1 KEEP list; seat-aware `securelyWins`; endgame exception; all verification gates.

- **2026-07-16, patch round (review round 3, no loopback):** Both hunters confirmed the loop-2 architecture (trump-led take extension sound, stake gate intentional, homogeneity proxy valid). Surviving patches: (1) `securelyWins` gains a follow-suit filter — a seat revealed to hold a led-suit card must follow, so only its led-suit cards threaten; (2) the `partnerTakesTrick` all-overtake fallback banks via `strongestPreservingBoss` instead of `lowestValue`; (3) negative test for the non-threat-proof declared beater on a trump lead; (4) test-attribution and comment fixes (securelyWins doc no longer claims parity with `partnerWinIsSafe`'s raw trump scan; single-suit-class caller invariant documented). REJECTED as re-litigating approved intent: boss-guard keeping a promoted Ten (checkpoint-1 approval), ">0" stake gate (PO decision 2026-07-15), all-unprotected fallback (PO), bestSmear guard/fallback discontinuity. DEFERRED (one entry, simulation-gated tuning): stake cost/benefit threshold, trick-8 leader nuance in the endgame guard, seat-aware `partnerWinIsSafe`, arbitration with last-trick retention.

## Design Notes

Scenario-1 root: the comment at `bot.go:423` assumes the overplay rule "auto-promotes us when we hold the boss" — false: `lowestValue` picks by points, so a forced overplay plays K (4 pts) while the unseen T (trump order Q<K<T<A<9<J) still wins. `securelyWins` must be seat-aware, mirroring `partnerWinIsSafe`: only cards a yet-to-play OPPONENT could still play count — per remaining opponent seat, its `knownHeldBy` cards plus unseen cards of suits it is not known void in. Ruff threats are covered automatically (a trump beats any non-trump candidate via `beatsCard`). Still conservative for genuinely unknown hands — when nothing is provably secure it falls back to today's behavior, never worse.

Scenario-2 root: `highestPointNonOvertaking` has no notion of control. The boss-guard generalizes the existing Ace+Ten exceptions (`chooseLead`, `strongestPreservingBoss`): a boss is smearable iff a same-suit held card is also a boss once it leaves. This intentionally also protects a Ten whose Ace falls in the current trick, and a promoted King — that is what flips the 5 listed test expectations. The narrower "Ace-only, needs the Ten" variant would flip only 2; flagged under Ask First for the human call.

## Verification

**Commands:**
- `cd server && go test ./internal/bot/...` -- expected: all pass incl. `TestDecide_AlwaysLegal` and the ≥60% simulation guard.
- `cd server && go test ./internal/game/...` -- expected: green (no engine change).
- `make lint` -- expected: gofmt + golangci-lint clean.

## Suggested Review Order

**Secure take (scenario 1: never donate a winnable trick)**

- Entry point: the opponent-winning take — stake gate, then cheapest secure winner, else the old cheap gamble.
  [`bot.go:471`](../../server/internal/bot/bot.go#L471)

- Partner-winning-unsafe fall-through: secure-take only when every legal card overtakes AND points are at stake.
  [`bot.go:428`](../../server/internal/bot/bot.go#L428)

- `securelyWins`: seat-aware per remaining opponent — known holdings, void filtering, and the follow-suit pin.
  [`bot.go:994`](../../server/internal/bot/bot.go#L994)

- `seatKnownHoldsSuit`: a seat revealed to hold the led suit must follow — its trumps aren't threats.
  [`bot.go:1039`](../../server/internal/bot/bot.go#L1039)

- `cheapestSecureWinning` + `trickPoints`: selection and the material-stake input.
  [`bot.go:1057`](../../server/internal/bot/bot.go#L1057)

**Guaranteed partner take (never fight a forced partner win)**

- `partnerTakesTrick` trump-led relaxation: overplay rule forces the partner's declared threat-proof beater out.
  [`bot.go:505`](../../server/internal/bot/bot.go#L505)

- Its branch: smear boss-safe, else bank high via `strongestPreservingBoss` — never dump minimum into a team trick.
  [`bot.go:447`](../../server/internal/bot/bot.go#L447)

**Boss-preserving smear (scenario 2: stop throwing lone Aces)**

- `bestSmear`: highest-point boss-safe candidate; keep-stronger tie-break; PO-confirmed highest-point fallback.
  [`bot.go:1108`](../../server/internal/bot/bot.go#L1108)

- `bossWorthGuarding`: trump masters always; non-trump bosses unless endgame-cuttable or the suit is dead.
  [`bot.go:1166`](../../server/internal/bot/bot.go#L1166)

- `heldSameSuitBoss`: the Ace-backed-by-Ten backup test, generalized.
  [`bot.go:1145`](../../server/internal/bot/bot.go#L1145)

- The three smear call sites: safe smear, Rule 8 risk-smear, partner-takes-trick.
  [`bot.go:400`](../../server/internal/bot/bot.go#L400)

**Tests**

- Secure-take table: forced overplay, stake gate, seat-aware and follow-suit cases.
  [`bot_test.go:1165`](../../server/internal/bot/bot_test.go#L1165)

- Trump-led partner-take: positive, negative (non-threat-proof), and forced-overtake banking cases.
  [`bot_test.go:1282`](../../server/internal/bot/bot_test.go#L1282)

- Boss-guard table: protected/unprotected, trump master, endgame pair, dead suit, fallback.
  [`bot_test.go:1339`](../../server/internal/bot/bot_test.go#L1339)

- Redesigned point-differentiated smear fixtures (discriminate from the duck path honestly).
  [`bot_test.go:325`](../../server/internal/bot/bot_test.go#L325)
