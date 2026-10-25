# Beljot bot v2: analysis and proposal

Status: proposal, revision 4 (signed off after three rounds of adversarial review), 2026-10-25. Phase 1 (§4.7) is implemented on branch `feat/bot-v2-phase1` (2026-10-25); the lab measured that package at +3.9 ± 0.7 (Bitola) / +5.2 ± 0.7 (Croatian) before implementation. Nothing else in this document is implemented. It is the output of a measurement-driven review of `server/internal/bot` for both
variants (Bitola and Croatian). The numbers and the method are in `BOT_V2_LAB_RESULTS.md`; the
code that produced them is in `docs/bot/lab/`; the review and what changed because of it is in
`BOT_V2_REVIEW_LOG.md`.

Constraints the proposal respects, as set by the owner:

- **Fair.** A bot may only use what a human in its seat could know: its own hand, the cards
  played and by whom, the bidding, revealed declarations, announced Belote, the candidate card.
  Bots at the same table never share hands.
- **Mathematical, not learned.** Decisions come from explicit probability reasoning and exact
  game evaluation. There is no neural network and no model trained on play. The bidding table is
  a list of hand shapes with their simulated value, readable and vetoable cell by cell.
- **Deterministic.** Every decision is reproducible from the seat's information state. The
  randomness inside the algorithm is seeded from a canonical hash of that state.
- **Rules are the engine's.** Nothing here changes a game rule. The bot only chooses among the
  cards the engine already declared legal.

---

## 1. Summary: the proposals that matter, in order of measured value

Measured in duplicate self-play against the current bot (points per dealt hand; see §2 for the
method and its limits):

| # | Proposal | Bitola | Croatian |
|---|---|---|---|
| 1 | **Card play by consistent-deal sampling + exact solving (PIMC).** Sample deals of the unseen cards that are consistent with everything public, solve each exactly, play the card with the best expected banked result. | **+8.8 ± 1.9** (12 samples on every trick, pooled over three seeds) to **+15 ± 4** (30 samples). The affordable step, rule layer on tricks 1–2 and the search from trick 3, is worth **+3.6 ± 1.1** (pooled; +3 to +5) at a tenth of the CPU. | **+18 ± 5** (every trick) |
| 2 | **Bidding by a calibrated hand-shape table** (per variant, one position adjustment in Croatian) instead of `wantsTrump`. | **+4.0 ± 0.7** (the regression formula: +2.3 ± 0.7); all-bot pass-outs 31% → 13% | **+3.9 ± 0.7** (formula: +4.1 ± 0.8) |
| 3 | **Rule fixes that stand on their own**: take with the cheapest card that no unseen led-suit card can beat (3rd hand with A♠ Q♠ plays the Ace, not the Queen); cash the Ace before the Ten. | +0.5 ± 0.3, +0.5 ± 0.1 | +1.3 ± 0.4, +0.5 ± 0.1 |
| 4 | **Information model**: every exact inference the rules allow but the bot ignores today (a side-suit discard proves void in trump too; under-play proves no higher card; under-trump proves no higher trump; the candidate card sits with the caller; declared runs locate cards and exclude their neighbours for every seat; Belote locates the pair). | free and exact; effect **not resolved** (run inference +1.9 ± 1.8; trump-void rule in the rule layer −0.1 ± 0.1 / +0.1 ± 0.1) | |
| 5 | **Correct objective**: value a line by what the hand banks under the engine's settlement (taker strictly ahead or loses everything; capot takes everything; the declaration contest; the stop-at-target race; the match rule near the target), not by raw card points. | **not resolved** (+1.4 ± 1.8 with a capot bug; re-measured in the lab results) | |
| 6 | **Conventions for human partners** (Ten under the Ace; return the signalled suit), applied as a small fixed bonus. | cost ≤ 0.02 in the rule layer; **not measured** inside the search | |

Packages, each measured as one policy against the current bot: phase 1, rules + table + memory
(#2 + #3 + #4): **+3.9 ± 0.7 / +5.2 ± 0.7**; phase 2 as it ships, phase 1 plus the search from
trick 3: **+8.0 ± 2.5 / +13.1 ± 2.8**; search on every trick + the bidding formula (#1 + #2):
+17.8 ± 5.4 / +19.4 ± 6.2.

What this is **not**: a collection of new special-case rules. Single rule tweaks are worth half a
point each; the current bot leaves 10–18 points per hand on the table through decisions no fixed
rule makes well, because they depend on where the unseen cards probably are.

What the two review rounds changed (details in the review log): the bidding ships as a table
with a stated inclusion criterion, not a formula; the search is "anytime" with a statistical
stopping rule, and the proposal states plainly that at the affordable budget the early tricks
stay seed-sensitive until the solver is faster; the hand award follows the engine's settlement
exactly, including the both-cross rule the rules file states differently, the stop-at-target
race (which needs its own objective inside the solver) and a per-sample declaration contest; the
bidding-consistency inference is deferred and, when it comes, derived from a hypergeometric
visibility factor; a production path that skips the bot memory update is listed as a phase-1
fix; audit verdicts are per variant with standard errors.

---

## 2. How the current bot was measured

A throwaway lab (`docs/bot/lab/`) was built around the real engine:

- **Double-dummy solver**: exact minimax over the four hands for the remainder of a hand, using
  the engine's legality rules (follow, must-overplay while the led suit can win, must-cut,
  must-over-trump, no partner exemption) and scoring (card points, +10 last trick, ±100 capot).
  Verified against brute force on 300 small endgames and, memo on vs off, on 150 mid-hand
  positions of real deals: 0 mismatches. Cost on one quiet core, unoptimised: ~0.3 s per solve
  from trick 1, ~0.1 s from trick 2, 15 ms from trick 3, 6 ms from trick 4, ≤1 ms later.
- **Duplicate self-play**: every deal is played twice with the two policies swapping teams, same
  dealer, same cards, so the luck of the deal cancels. Both policies see only the `View` the
  production bot gets, built from one shared public `Memory`.
- **Bidding rollouts**: at every bidding decision point of 60 000 (Bitola) and 30 000 (Croatian)
  deals, force the call (each allowed suit) and force the pass, play both out, record the
  difference. 830 000 rows; every cell quoted below carries its standard error.

Caveats, so the numbers are read correctly:

- Self-play measures bot vs bot. Against humans the absolute numbers move; the ordering of the
  proposals should not.
- The bidding calibration used the **current** bot to play out the hands, so its cells describe
  the game as the current bot plays it. The Croatian regression even values the trump Ace below
  the trump Seven, because the current rules misplay a non-master trump Ace. The table must be
  re-run under the new play engine before it ships (a five-minute lab job), and the cells
  re-checked by a human.
- Double-dummy "regret" (distance from the perfect-information optimum) does not rank
  imperfect-information policies: PIMC and the current bot have the same regret (4.3 per
  decision) and PIMC wins by 10–15 per hand. Regret still locates *where* decisions are hard:
  **68% of it is in the lead**, 8–12 points per lead decision; following costs 0.2–4.
- Sample-count and schedule comparisons against the current bot are under-powered at 120–250
  deals (SE 2–3 on a 2–4 point question); the review log lists which of them reproduce and which
  do not. Direct A/B runs (both sides searching) are the reliable form and are used for the
  early-trick question. The PIMC-vs-current effect itself reproduces across seeds (+7.6 to +10
  per deal).

---

## 3. Audit of the current rules

Verdicts: **keep**, **fix**, **remove**, or **superseded** (correct idea; the search in §4 does it
exactly). Bidding verdicts are per variant and quote the standard error of the gain
(EV(call) − EV(pass), points per hand, current-bot rollouts).

### 3.1 Bidding (`wantsTrump`, `trumpSuitScore`, `decideBid`)

| Rule | Bitola | Croatian | Verdict |
|---|---|---|---|
| Four or more trumps: always call | four small trumps, no side Ace: −4.2 ± 5.4; with a side Ace +34 ± 10 | +5.3 ± 9.3; +33 ± 15 | **fix**: a coin flip without a side Ace, a call with one |
| Three trumps with the Jack, no 7/8 unless a side Ace | J + any two: **+29.8 ± 1.5**, bot calls 30%; J+8+7 alone +5.6 ± 4.7 | **+22.7 ± 2.5**; J+8+7 +14 ± 8 | **fix**: too strict; J with three trumps always calls |
| 9 + A + side Ace (three trumps) | **+7.9 ± 3.0** | **−19.9 ± 4.7** | **keep in Bitola, remove in Croatian** |
| J + 9 doubleton only with a side Ace | without one: **+16.9 ± 2.3** (bot passes) | **+22.6 ± 3.7** | **fix**: J+9 always calls |
| J + other doubleton with two side Aces | **+22.3 ± 3.3** (bot passes) | −1.6 ± 5.0 | **add in Bitola** |
| Round-2 suit choice by `trumpSuitScore` | | | superseded by the table's ranking |
| No awareness of position, round or score | round 2 costs ~4 more than round 1 for the same hand | the dealer's partner (second to speak) gains **+16 ± 3** from calling with a J doubleton and a side Ace where the other seats lose 13–27, because its pass risks the dealer's forced pick | **fix** |
| Net effect | **31% of deals pass out** at an all-bot table; 86–92% of contracts made: only near-certain hands are called | | |

### 3.2 Leading (`chooseLead`)

| Rule | Verdict | Evidence |
|---|---|---|
| Draw trumps with the master when we called | superseded | right idea; the search decides when it pays |
| Draw trumps for the partner with Q→K→T→A | keep if rules remain / superseded | removing it costs −0.65 ± 0.31; leading the lowest trump instead changes nothing (+0.06 ± 0.10) |
| **Cash the Ten before the Ace** from A+T | **remove** | With A+T and nothing else known an opponent can ruff the **second round 59%** of the time (three cards: 75%); the first round is safe 88%. Leading the Ten exposes the 11-point Ace to that ruff instead of the 10-point Ten, and as a signal it is sent where no signal is needed (the owner's position; the Ten-under-Ace discard in §4.4 is the convention). Ace first: **+0.48 ± 0.12** (Bitola), **+0.53 ± 0.13** (Croatian). |
| Cash a boss, then the next boss in the same suit | **fix** | the second round of a suit needs a ruff-risk estimate; a bare threshold on the sampled ruff probability measured ±0 because the fallback lead is no better, so the decision has to compare the alternatives' expected values, which is what the search does |
| Lead into the partner's known void | keep / generalise | correct; the sampler turns "proven void" into "probably void and probably holds trump" |
| Lead safe low; only-trumps rule | keep as fallback | |
| No defender logic beyond "cash a boss / lead low" | **add** (§6) | |

### 3.3 Following (`chooseFollow` and helpers)

| Rule | Verdict | Evidence |
|---|---|---|
| **Take "as cheaply as possible" over an opponent** with an opponent still to play (3rd hand with A♠ Q♠ over a low lead plays the Queen) | **fix** | the "secure take" that should override it counts every unseen trump as a threat even when the last player must follow suit, so early in a hand nothing is ever secure and the bot gambles the Queen; the last player wins with the Ten or King and the kept Ace is ruffed later. Fix: when a plain suit is led, take with the cheapest card that **no unseen card of the led suit** can beat (a ruff beats the Ace too, so it is no argument for the Queen); that banks the Ace's 11 points and the trick. **+0.52 ± 0.27** (Bitola), **+1.31 ± 0.35** (Croatian). |
| `partnerWinIsSafe` scans unseen cards without per-seat void filtering | superseded | |
| forced-overplay "secure take", control-trump economy (`trumpEconomyTake`) | superseded | what the solver computes without special cases |

Measured regret while following is small (0.2–4 per decision); the rules stay as the fallback
layer.

### 3.4 Endgame (`retainLastTrickWinner`)

Superseded exactly: with two cards left the solve over sampled deals is the precise version of
this rule, including the capot case the rule ignores.

### 3.5 Memory (`memory.go`)

| Fact available to a human | Current bot | Verdict |
|---|---|---|
| A player who does not follow the led suit is void in it | yes | keep |
| A player who plays neither the led suit nor a trump is **also void in trump** (the engine forces a cut; no partner exemption) | **no** | **fix** (free, exact) |
| A player who follows suit below the highest led-suit card **on the table at that moment**, while the led suit could still win, **has no higher card of that suit** | no | **fix** (free, exact; needs the trick prefix passed to `ObservePlay`) |
| A player who trumps below the highest trump on the table at that moment **has no higher trump** | no | **fix** (same) |
| The candidate card is in the caller's hand (Bitola, both rounds) | no | **fix** (free, exact) |
| Revealed declaration cards are in that hand | yes | keep |
| A revealed run is maximal, so its neighbours are absent | partner + trump suit only | **fix**: every seat, every suit (sound in both variants) |
| Belote announced ⇒ the other card of the K/Q pair is in that hand | no | **fix** (free, exact; only the announcement is public, the prompt is masked) |
| A seat that declared during Bitola trick 1 (public `event:player_declared`) holds a declarable combination; a **bot** seat that did not declare holds none (bots always declare); after the reveal, `contested` bounds the losing team's best meld below the winner's | no | **fix** (free, exact) |
| Who called, who passed, in which round, on which suit | no | record; use only as the §4.1 rule 10 inference, later |
| Partner discarded the Ten of a side suit on our trick | no | add as a soft inference (§4.1 rule 11) |
| **The memory is updated on every state change** | **no**: `handleDeclarationTimeout` in `live_match.go` replaces the state after a Croatian declaration window expires without calling `observeBotMemory`, so the first decision of trick 1 can run without the revealed declarations | **fix** (one line; phase 1) |

### 3.6 Declarations, Belote, surrender

Always declare, accept the partner's surrender: **keep**. Always announce Belote: **keep for
now**; once the search exposes P(make), gate it as caller (announcing while the contract is
already lost hands 20 more points to the opponents).

---

## 4. The proposed algorithm

```
View (public facts) ─► Information model ─► Belief sampler ─► Decision
                       hard + soft facts     consistent deals   bidding table / anytime PIMC / conventions
```

### 4.1 Information model

**Hard constraints** (exact, derived from the engine's legality rules; each is one bit-set
operation on a per-seat "cannot hold" mask):

1. Cards played are gone; the current trick's cards are gone.
2. Hand sizes: every seat holds `len(ownHand)` cards, minus one for seats that have already
   played in the current trick.
3. Void in the led suit when a seat did not follow it.
4. Void in trump as well when a seat played neither the led suit nor a trump.
5. No card of the led suit above the highest led-suit card that was on the table **when that
   seat played**, if the seat followed below it while the led suit could still win (no trump had
   cut, or trump was led).
6. No trump above the highest trump that was on the table when that seat trumped below it.
7. Cards located in a hand: revealed declaration cards (until played), the candidate card in the
   caller's hand (Bitola, either round), the Belote pair once announced.
8. Neighbours of a revealed run are absent from that hand. (A run shown is maximal in the hand;
   in Bitola a run dropped by the one-card-one-group rule is not shown, so nothing false is
   inferred; Croatian overlap does not change maximality.)
9. Declaration events: in Bitola the `event:player_declared` message tells every seat which seats
   declared during trick 1, and the engine rejects a declare from a meldless hand, so a seat that
   declared holds a declarable combination (a sampled hand without one is rejected); a bot seat
   that did not declare holds none (bots always declare; for a human seat "skipped" proves
   nothing). After the reveal, `contested` says whether the losing team declared at all, and if so
   their best meld ranks below the winner's. Without this the per-sample declaration contest in
   4.3 under-prices the opponents' melds exactly when they are known to exist.

Rules 5 and 6 need the trick prefix at the moment of the play, so `Memory.ObservePlay` takes the
pre-play trick, not just the lead suit.

**Soft likelihoods** (a weight in (0, 1] per sampled deal; none in the first delivery):

10. **Bidding consistency.** Deferred. A naive version (weight 0.2 on any deal where a passer
   holds J+9, four of the suit, or J with three) measured **−1.9 ± 1.9**, and the review showed
   why: the pass was made on 5 of the 8 cards (Bitola) or 6 (Croatian), so a final hand that shows
   the pattern often did not show it at bid time. The probability that it did is a hypergeometric
   constant: J+9 both visible 0.36 / 0.54; all four visible 0.07 / 0.21; J with two others 0.18 /
   0.36 (Bitola / Croatian). If the inference is ever used, the weight is
   `w = 1 − P(visible) · (1 − ε)`, with a bidder model that matches the seats at the table (for bot
   seats, their own table; for humans, nothing until there is data). Every soft weight stays
   ≥ 0.5 until a mixed-table measurement exists.
11. **Ten-under-Ace signal.** Under these rules a side-suit discard is legal only when the player
    holds no trump at all (rule 4), so a discard never means "emptying a suit to ruff it". The
    partner's discard of the **Ten** of suit X on a trick our side is winning means, by
    convention, "I hold the Ace of X, return it": samples in which the partner lacks the Ace of X
    get weight 0.5. Only the Ten carries the signal (a discarded King is just a worthless King;
    the current bot's smear would throw it with no meaning). Whether the partner seat is a v2 bot,
    a v1 bot or a human is public at the table; for a v1 or human partner the weight is a prior
    until measured. The absence of the Ten is **not** used as evidence: a v2 sender only throws it
    when its own search allows, which the receiver cannot reconstruct.

### 4.2 Belief sampler

Input: the constraints for one seat. Output: deals of the unseen cards, each with a weight, drawn
until the decision layer stops asking (see 4.3).

```
unseen  = deck − played − ownHand − currentTrick − located cards
need[s] = handSize[s] − |located[s]|            for the three other seats
draw:
    shuffle unseen; deal need[s] cards to each seat in order
    reject and redraw if a card lands in a seat that cannot hold it (rules 3–6, 8)
    (after 200 rejections fall back to a constructive fill that places each card in a random
     seat still allowed to take it)
    weight = product of the soft likelihoods (rule 11; rule 10 when enabled)
```

Determinism: the RNG is seeded from a **canonical hash of the information state** (own hand as a
mask, played set as a mask, trick so far, void and cannot-hold masks, located cards, scores,
trump, caller), so two situations with the same information produce the same samples and the same
card, regardless of the order in which cards were observed. The final choice among cards with
exactly equal value is made on the legal set in canonical order (hand order depends on the deal
and on `removeCard`, so it cannot be the tie-break). Bidding bookkeeping (who passed what) must be
reset by the Bitola round-2 reshuffle, which keeps the hand number and so does not trigger
`SyncHand`. The lab verified: same seed ⇒ same card, 55/55; 2 750 samples with 0 constraint
violations.

### 4.3 Decisions

#### Bidding: a hand-shape table

Call when the hand (in Bitola including the candidate card) matches a row; otherwise pass. When
several suits match (Bitola round 2, Croatian), take the one with the most trump points plus
length; when the pick is forced (Croatian dealer), take that suit regardless of rows.

Inclusion criterion, applied to every cell of the lab's class table: the cell's gain is at least
twice its standard error and at least +5 per hand, and the shape is one a human partner would
accept as a call. Cells that fail either test are out; two borderline cells (rows 4 in Croatian
and 7 in Bitola) are included by the owner's decision and flagged for re-check.

| Row | Call with | Bitola (gain ± SE) | Croatian (gain ± SE) |
|---|---|---|---|
| 1 | J and 9 of the suit, two or more trumps | +17 ± 2 (two), +62 ± 2 (three), +119 ± 3 (four) | +23 ± 4, +64 ± 3, +101 ± 6 |
| 2 | J with three or more trumps | +30 ± 2 (three, no side Ace), +49 ± 2 (with one) | +23 ± 3, +47 ± 4 |
| 3 | four or more trumps including the 9 | +38 ± 4 | +33 ± 7 |
| 4 | four or more trumps including the Ace (no J/9) | +9 ± 4 | +10 ± 7 (1.4 SE; included by the owner's decision, re-checked after recalibration) |
| 5 | four or more trumps with a side Ace | +28 ± 11 (exactly one side Ace) | +33 ± 15 (exactly one) |
| 6 | five or more trumps | +107 to +185 | (no cell with n ≥ 150) |
| 7 | Bitola only: 9 and A with three trumps and a side Ace | +5 ± 3 (exactly one side Ace), +8 ± 3 (one or more); 1.7–2.6 SE, included by the owner's decision, re-checked after recalibration | −19 ± 5: out |
| 8 | Bitola only: J and A doubleton with a side Ace | +13 ± 3 | −16 ± 5: out |
| 9 | Bitola only: J doubleton with two side Aces | +22 ± 3 | −2 ± 5: out |
| 10 | Bitola only: 9 with two other trumps (no J, no A) and a side Ace | +7 ± 2 | −7 ± 4: out |
| 11 | Bitola only, third to speak: J doubleton with one side Ace | +13 ± 2 at that position | — |
| 12 | Croatian only, dealer's partner (second to speak): J doubleton with a side Ace | — | **+16 ± 3** at that position (−27 ± 3 and −13 ± 4 at the other two: passing as the dealer's partner risks the forced pick; the value shrinks against humans who call more in third seat) |

Not rows (the data and a human partner agree): J + one other doubleton without side honours
(−28 ± 2 / −28 ± 3), A + two small with a side Ace (−17 ± 2 / −38 ± 4), four small trumps with
no side Ace (−4 ± 5 / +5 ± 9), any other two-trump hand. Round 2 in Bitola costs about 4 points
more than round 1 for the same hand; the table ignores that (acceptable: no row is within 4 of
the threshold except the owner's-choice ones).

Measured as a policy in duplicate play against the current bot, with every row above
included: **+4.0 ± 0.7** (Bitola), **+3.9 ± 0.7** (Croatian). On the lab's own rows, per
decision row: +5.71 / +3.90 against the current rules' +4.96 / +3.06 and the regression
formula's +5.77 / +3.93.

The regression fit (20 coefficients, published in the lab results) stays a lab cross-check. Its
coefficients are statistics of the **current bot's play**, not Belot knowledge, and are not to be
read as "bidding points". Both the table and the fit must be re-derived under v2 play before the
table ships, and every row re-checked by a human.

Score situation (not yet measured, §6): opponents within one hand of the target: call anything
playable; own side within one hand: call only what makes it alone; `StopAtTarget` tables: melds
count the moment they are scored.

#### Card play: anytime PIMC

For every legal card, the value is the weighted average over sampled deals of the exact
remainder after playing it, mapped to what the hand banks:

```
value(card) = Σ_d w_d · award(v_d(card)) / Σ_d w_d
```

`award` is the engine's own settlement (`scoring.go`, not the prose rules file) applied to the
points already banked by both teams, the declarations and Belote, and the remainder:

- the taker's team must finish strictly ahead or every point of the hand goes to the opponents;
- a capot gives the capot team everything, the opponents' melds included;
- **declaration contest**: in Bitola during trick 1 nothing is scored yet and the bot's own melds
  only count if its side wins the contest, which depends on hidden melds. Each sample is a full
  deal, so each sample settles its own contest with the engine's detection and tie-break rules
  (the trick-1 leader is known); this also prices the opponents' possible melds;
- **match rule near the target**: when both teams would cross the target in the same hand, the
  engine gives the match to the **higher total**, and only an exact tie to the caller
  (`determineMatchWinner`); the rules file and the client rules page say the same;
- **`StopAtTarget`**: the engine ends the hand at the first trick boundary (or declaration or
  Belote event) at which a running total (hand points + melds + Belote, no bonuses, no contract
  settlement) reaches the target, and the match goes to the higher running total, tie to the
  caller. That value depends on *when* a total crosses, not only on the final remainder, so it
  cannot be applied at the root. When `StopAtTarget` is on and either team is within reach
  (target − total ≤ remaining card points + unscored melds), the solver runs with a race objective:
  the same search with the target check at every trick boundary and a terminal value of ±W for a
  decided match. It is the same solver with one check per boundary, and it cuts off earlier.
  Until that exists, the rule layer plays those situations with "take points now" as its
  preference, and the case is flagged as unmeasured.

For the normal settlement, inside each sampled deal the raw-points minimax stays exact because
`award` is a monotone step function of the remainder; the transform matters only across samples,
which is the point. The race objective is the one case where it is not, and it is solved inside.

**Stopping rule (anytime).** Every card is valued on the same samples, so the paired standard
error of the difference between cards is cheap to track. Sampling runs to a per-trick minimum
(16, 30, 40, 60 samples for tricks 3–6 in phase 2; 3 and 6 on tricks 1–2 when the search is
extended there; the full 3/6/16/30/40/60 schedule measured **+10 ± 3 / +18 ± 4**), then continues while the best card's margin over the runner-up is below one standard error, until
a per-decision deadline; the best card so far is played at the deadline. Details that matter:

- the tie set handed on is every card within one SE of the best, not just the runner-up;
- a margin of exactly zero with zero spread (cards equivalent in every sample) is a tie, so
  "tie" is margin ≤ max(SE, ε), never a wait for the deadline;
- with weighted samples the SE uses the effective sample size (Σw)²/Σw²;
- the minimum is a floor: the think delay stretches to cover it, up to a hard cap set by the
  owner's CPU budget; the deadline only cuts the extra samples.

What the rule does and does not buy. The gain holds at the minimum budget. Consistency does
not, yet: measured at 3, 6 and 12 samples, **about half of the trick-1 and trick-2 decisions and
a quarter to a half of trick-3/4 decisions change card under a different seed** (counting
equivalent cards as changes, so the true figure is somewhat lower; tricks 3–4 at the phase-2
budget of 16/30 samples are unmeasured and stay seed-sensitive, covered by the stability report). Reaching a ≤ 15% change rate on tricks 1–4 needs
the standard error 3–4× smaller, i.e. 10–15× more samples, which is the solver-speed work of
phase 3. With the current solver the stability target is expected to hold from trick 5 on.

**Ties and conventions.** The conventions layer (4.4) is applied as a small fixed bonus (2–3
points, to be measured), not as a tie-break, so that it does not depend on the stability above;
remaining exact ties go to the current rules' preference (cheapest winner, lowest discard) on the
canonically ordered legal set.

**Known weakness: strategy fusion.** PIMC values each sample as if everyone plays with full
knowledge from the next card on, so it over-values plans that are perfect in some deals and
expensive in the real one. The signature is visible in the lab: PIMC's trick-1 regret is 8.4 vs
the current bot's 5.8, and its lead regret when the opponents called is 12.0 vs 8.9; it wins
anyway because the rules lose more elsewhere. The cost is bounded in this game by forced following,
forced cutting and forced over-trumping, which leave the defenders few options. Mitigations, in
order of cost:

1. Measure the gap: the lab's `cheat` policy (double-dummy on the true deal) bounds what any
   information model can recover; the lab results report `cheat − PIMC`.
2. Risk-adjusted aggregation for leads: rank by `mean − λ·SE` across samples instead of the
   mean (one line; the lab prices λ).
3. For the trump-draw decision specifically: compare the conditional means over the samples where
   the master trump sits with the opponents vs with the partner; if they disagree on leading
   trump, the decision is a guess and goes to the rule layer's prior.
4. Carry the per-sample vector through the first plies and fuse only at depth 2 (αμ search,
   Cazenave & Ventos 2020); 2–4× the cost, sits on the existing transposition-table solver.

**Which tricks to search.** Pooled by inverse variance over every lab run (lab results §2.2):
searching every trick at 12 samples is worth **+8.8 ± 1.9** per deal against the current bot;
the search from trick 3 with the current rules on tricks 1–2 is worth **+3.6 ± 1.1**. The value
of searching the opening itself is measured two ways: indirectly, as the difference of those
two, +5.2 ± 2.2; directly, with both sides searching so the deal noise cancels, **+0.5 ± 2.2**;
pooled, **+2.9 ± 1.6**, i.e. somewhere between nothing and about +5, roughly a third of the
search's value with zero inside the interval. The cheap 3/6-sample opening search is worth
nothing over the rules on those tricks (−2.8 ± 2.8, direct). The two direct A/Bs are the primary
evidence on the opening. A dozen samples at trick 1 cost ~3.6 s per decision with the unoptimised
solver; the opening is also where sample noise and strategy fusion cost most and where the fixed
rules (draw trumps, cash a boss, lead safe) are at their best. Phase 2 therefore runs the rule
layer (with the §3 fixes) on tricks 1–2 and the search from trick 3: about a tenth of the CPU,
deterministic openings, and most of the gain that is reliably measured. Extending the search to
tricks 1–2 with ≥ 12 samples is phase-3 work, after the solver optimisations and the
risk-adjusted aggregation, with an expected +0 to +5 that is measured before it ships.

**CPU and latency.** Per sample, unoptimised, on a quiet core: ~0.3 s at trick 1, ~0.1 s at trick
2, 15 ms at trick 3, 6 ms at trick 4, then negligible; tails are 2–3× the mean. The production
think delay is 1.0–2.5 s. The decision is computed when the turn arrives, off the session lock, on
a bounded worker pool shared by all tables, against a deadline, and applied when the think timer
fires. The phase-2 schedule (from trick 3) costs about 0.25 s per decision at trick 3 and less
afterwards, well inside the delay; the anytime rule spends more only when the decision is close
and the deadline allows. Standard double-dummy optimisations (move ordering by trick-winning,
transposition at every ply, merging adjacent ranks held by the same hand) are what would make
early-trick search affordable later; they are not a design dependency.

**Default budget** (the owner's decision: a reasonable default, the host scales vertically). All
four are `BELJOT_*` settings read once in `config`, so raising them is an environment change:

| Setting | Default | Meaning |
|---|---|---|
| search deadline per decision | 0.5 s | the anytime rule stops here; the best card so far is played |
| think-delay cap | 3.0 s | the per-trick minimum samples are a floor; the humanised delay stretches to cover them, never beyond this |
| search workers per host | `max(1, min(4, NumCPU/2))` | a bounded pool shared by every table; a decision that cannot start before its deadline falls back to the rule layer, so bots never starve the API |
| opening search (tricks 1–2) | off | turned on in phase 3 only when 12 samples at trick 1 fit the deadline, i.e. after the solver is roughly 5× faster |

A benchmark test asserts the deadline per trick on the CI runner, and a counter of fallbacks per
hour is the signal that the host needs more cores.

### 4.4 Conventions layer (talking to human partners)

Applied as a fixed bonus on the card the convention asks for, so that the convention fires
consistently rather than only on statistical ties. **Default: 3 points** (the owner's decision),
the same at every table; the convention card is played whenever its search value plus the bonus
is at least the best other card's value, so it yields only when the search says it costs
noticeably more (for instance when the Ten is needed as a second winner in that suit). The lab
prices the bonus inside the search before it ships; in the rule layer the cost measured
≤ 0.02 per hand:

- **Ten under the Ace.** The partner is winning the trick safely (a trump lead, or any boss) and
  the bot holds no trump, so it may discard anything: throw the Ten of a suit in which it also
  holds the Ace. It gives the points and tells the partner to return that suit. Measured cost in
  self-play ≤ 0.02 per hand. The bot never leads the Ten of an A+T suit to "signal" (the owner's
  call; §3.2).
- **Return the signalled suit.** After the partner's Ten discard (4.1 rule 11), prefer leading
  that suit when the bot has no sure trick of its own.
- **Smear high on a partner's safe trick, dump low on the opponents'.** Already the rule; it
  stays as the tie-break.

### 4.5 Declarations, Belote, surrender

Unchanged in the first delivery: always declare, always announce, accept the partner's surrender.
Later (§6): gate Belote as caller on P(make); propose surrender on hopeless hands.

### 4.6 Testing

- Sampler: every hard constraint holds in every sample; the same information state (built from a
  **play history through `Memory`**, never from a hand-written view) yields the same samples; the
  Memory is observed on every state replacement, including the declaration-window timeout.
- Solver: the brute-force and memo-on/off cross-checks from the lab, kept as unit tests.
- Decisions: the existing `TestDecide_*` tables stay valid for the rule layer; PIMC decisions are
  pinned on fixtures where the answer is unambiguous (2-card endgames, forced plays), and on
  fixtures where the stopping rule must reach a margin.
- Strength gate: a **duplicate self-play gate** (v2 vs the frozen v1 copy above a threshold)
  replaces "≥60% vs random".
- Stability report: the share of decisions that change under re-seeding at the production
  budget, per trick, **excluding cards that are equivalent in every sample**, reported next to the
  advantage; expected to meet ≤ 15% from trick 5 with the current solver and on tricks 1–4 only
  after the solver optimisations.
- Information-gap report: `cheat − PIMC` per trick, so the value of future inference work is
  known before it is done.
- Time budget: a benchmark test asserting the per-decision deadline, not a sample count.

### 4.7 Delivery in phases

1. **Memory + bidding table + rule fixes** (no play-engine change): hard constraints 4, 7 and 8
   in `Memory` (5 and 6 are recorded but only pay off once the sampler exists), the memory update
   in `handleDeclarationTimeout`, the bidding table, the led-suit-secure take, Ace before Ten,
   the Ten-under-Ace discard. Measured as one policy (`ledSecure,aceFirst,bidTable,memory4`):
   **+3.9 ± 0.7 (Bitola) / +5.2 ± 0.7 (Croatian)**. Low risk.
2. **Solver + sampler + anytime PIMC from trick 3**, the rule layer (with the phase-1 fixes) on
   tricks 1–2, the exact `award` including the per-sample declaration contest, computed off the
   lock with a deadline and a worker pool; the rule layer stays as the fallback everywhere and
   plays stop-at-target situations within reach until the race objective exists. Measured as it
   ships (phase-1 rules and table on tricks 1–2, 12-sample search from trick 3, 400 deals):
   **+8.0 ± 2.5 (Bitola) / +13.1 ± 2.8 (Croatian)** against the current bot, at ~0.25 s per
   decision. Re-run the bidding calibration under v2 play and re-check the table.
3. **Solver speed, stability and strategy fusion**: solver optimisations, the stability report,
   risk-adjusted leads, the trump-draw conditional, the race objective for stop-at-target; then
   the conventions bonus, measured; then the search on tricks 1–2 with ≥ 12 samples, which the
   lab prices at +0 to +5 (pooled +2.9 ± 1.6), measured before it ships.
4. **Soft inferences and score awareness**, each gated by a lab measurement with ≥ 400 deals.

---

## 5. Open questions for the owner

1. ~~CPU budget per decision and per host~~ — answered: use the default budget in §4.3 and
   scale the host vertically when the fallback counter says so.
2. ~~The size of the conventions bonus~~ — answered: 3 points, as recommended (§4.4).
3. ~~The two owner's-choice bidding rows~~ — answered: both included, as recommended, and
   re-checked after the recalibration under v2 play.
4. ~~The rules file says "both cross ⇒ the taker wins"~~ — answered: the engine is right;
   `.claude/rules/game-rules.md` now states "higher total wins; an exact tie goes to the taker".
5. Is a visible "bot strength" setting wanted later? It falls out of the deadline for free.
6. Should bots propose surrender on hopeless hands once P(make) exists?

---

## 6. Additions raised by the review, not yet measured

In rough order of expected value; each is a lab afternoon before it is a code change.

1. **Match-score awareness** in bidding and play: the engine's both-cross rule (higher total,
   tie to the caller) near the target; `StopAtTarget` as a race; the two obvious bidding cells
   (opponents one hand from the target; own side one hand from it).
2. **Capot pursuit and defence** as explicit objectives (the exact `award` already prices them;
   the rule layer needs "take one trick early when the caller shows capot shape").
3. **Defender leads in the rule layer** (for any trick the search does not run): lead a short
   suit for a ruff; lead the suit where the partner can hold the Ace; lead trump through the
   caller only with J or 9.
4. **Partner-type awareness** (v2 bot, v1 bot, human) for which conventions are reliable.
5. **Surrender proposals and difficulty-proportional think time**: the search yields P(make) and
   the EV gap for free.
6. **Belote gating as caller** on P(make).
7. **A looser, human-like bidder policy in the lab**, so bidding inferences and the table are
   tested against a table that does not bid like the current bot.
