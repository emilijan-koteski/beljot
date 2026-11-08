# Beljot bot v2: lab method and results

Companion to `BOT_V2_PROPOSAL.md`. Everything here was produced by the throwaway lab in
`docs/bot/lab/` (a copy of the sources; they were run from a git worktree as
`server/internal/botlab` + `server/cmd/botlab`, against the real engine and the real bot).
Date: 2026-10-25, commit 2d70456. §3.4 was run on 2026-10-25/26 on branch `feat/bot-v2-phase2`
at `dd4ec76`, after the lab snapshot was updated to the phase-1 `View` (§1.5, §4). §3.5 was run on 2026-11-07 on
branch `feat/bot-v2-phase3` at `49f242f3`, with the shipped search on every seat.

## 1. Instruments

### 1.1 Double-dummy solver (`dd.go`)

Exact minimax with alpha-beta and a transposition table at trick boundaries over the four hands
(32-bit masks), fixed trump. Legality mirrors `game/validation.go` (follow suit; must overplay
the led suit while it can still win; must cut when void; must over-trump). Value = team A minus
team B points for the remainder, including +10 for the last trick or ±100 for a capot.

Verification: `botlab -exp=ddcheck -n=300` compares against a plain recursive minimax with no
pruning or memo on random 2–4-card endgames: **300 positions, 0 mismatches**. `-exp=ddcheck2
-n=150` compares memo-on against memo-off alpha-beta from real deals after 2–3 random tricks
(5–6 cards per hand): **150 positions, 0 mismatches**.

Speed (`-exp=bench`, 20 random deals per row, one core, Go 1.26, no solver optimisations; the
second column was measured while other jobs loaded the machine, so treat both as upper bounds):

| Solve from | quiet machine | loaded machine | nodes |
|---|---|---|---|
| trick 1 | 364 ms | 239 ms | 0.6–1.2 M |
| trick 2 | — | 105 ms | 254 k |
| trick 3 | 3.4 ms | 15 ms | 29–41 k |
| trick 4 | — | 6.1 ms | 15 k |
| trick 5 | 0.1 ms | 1.1 ms | 1.2 k |
| trick 6 | — | 0.1 ms | 211 |
| trick 7 | — | <0.1 ms | 30 |

### 1.2 Duplicate self-play (`sim.go`, `-exp=duplicate`)

Each deal (shuffled deck + dealer) is played twice: policy A on seats 0/2 and B on 1/3, then
swapped. The score is A's banked award minus B's, averaged over the two games, so the deal's luck
cancels. Reported: mean advantage per hand ± standard error over deals. A Bitola deal that passes
out in round 2 scores 0/0 (that is the game's own value of it), so "per hand" means per dealt
hand including passed-out ones; divide by the played fraction for "per played hand" (e.g. the
PIMC rows below had 21% pass-outs, so +9.9 per deal is +12.5 per played hand). Both policies receive exactly the
`View` that `match.buildBotView` constructs, from one shared public `Memory`.

### 1.3 Regret (`-exp=regret`)

One policy on all four seats; at every card decision with more than one legal card, the solver
values every legal card in the **actual** deal; regret = best − chosen, from the mover's team's
perspective. Grouped by context (lead / follow, who is winning, position in trick, void or not)
and by trick.

### 1.4 Bidding rollouts (`-exp=bidpos`, `-exp=bidtable`)

For every deal, bidding is walked naturally with the current bot; at each decision point the
lab forces a call of each allowed suit and, separately, a pass, then plays the hand out with the
current bot on all seats. Each row: hand features, round, passes before, EV(call), EV(pass),
whether the bot would naturally call. Bitola: 60 000 deals → 502 021 rows. Croatian: 30 000 deals
→ 328 472 rows.

Phase 4 adds `-shape=<list>` to `bidpos` and `duplicate` (`docs/bot/lab/shape.go`): deals are drawn
until some seat's bidding position fits the shape, in the vocabulary of `class_table.py` (`c4`,
`c4+`, `J`/`noJ`, `9`/`no9`, `A`/`noA`, `sA0`/`sA1`/`sA1+`/`sA2+`, `pos1` = passes before the
seat), so a rare bidding row gets thousands of occurrences in minutes. `bidpos` also requires the
natural auction of `-a` to reach the matching position, so every targeted deal yields at least one
matching row; `duplicate` only requires the position to exist. Targeted deals are biased: the
CSV is valid for the targeted cell only, never for a call rate, a table-wide value or a pooled
total, and `bidpos` prints a warning line saying so. `-exp=shapecheck` prints 20 matching deals
for eyeballing the predicate.

`-exp=bidscore` (`docs/bot/lab/bidscore.go`) measures a bidding decision by the match result instead
of the hand points. Each pair is one deal with match scores drawn from `-scoreState` against the
target 1001, for the tested seat's team: `oppClose` (opponents 10-100 short, own side 200-600),
`ownClose` (the mirror), `bothClose` (both 10-100) and `neither` (both 300-600). The tested seat is
the first seat whose natural position fits `-shape`, or seat 0 without a shape; deals where the
natural auction of `-a` does not reach it with a free choice are redrawn. The deal is played twice,
the seat forcing a call (the candidate's suit in Bitola round 1, else the suit with the most
trumps, ties by trump-order points, because `trumpSuitScore` is not exported) and forcing a pass,
with policy `-a` on all seats and the natural bids of the other seats. An undecided match continues
with fresh random deals (the same stream for both branches of a pair) up to 12 hands, then scores 0.
The forced call starts the play with a memory that knows the candidate card went to the caller, as a
natural call does (`PlayHandFrom`, also used by `bidpos`). A forced pass at the reshuffle seat
(Bitola round 2, three passes) is scored as a passed-out hand. The walk and the recorded bids are
made with neutral scores and replayed under the real scores (identical today, because bidding
ignores the score).
The line is `bidscore <state>: n pairs, call-minus-pass match wins +x.xxx ± y.yyy, call wins a/n,
pass wins b/n`, where "wins" counts the pairs in which the tested team won the match. `-stopAtTarget`
(default true; also on `race`) set to false makes a match end only at a hand end.

### 1.5 Policies

- `current`: production `bot.Decide`. At commit 2d70456 this was the v1 bot, and every table in
  §2 was measured against it. Now the snapshot's `current` is the phase-1 rule layer (§3.4), and
  `lab` with no flags, the frozen pre-phase-1 copy, plays the old `current` (the copy measured
  identical to it in §2.1): to reproduce a §2 row, replace `current` by `lab` in its command. The
  lab view told v1 which seat received the candidate card (via `ObserveCandidate`), which
  production v1 did not; production now records the same fact as a located card, and the announced
  Belote pair too, which the snapshot also records, for every policy.
- `cheat`: double-dummy optimum on the actual deal (perfect information; an upper bound, never
  fair). Bidding by `lab[]`.
- `lab[flags]`: a copy of `bot.go` with experiment switches (`aceFirst`, `ruffNN`, `noPartnerDraw`,
  `drawLow`, `tenSignal`, `ledSecure`, `bidEV` (regression), `bidTable` (hand-shape table),
  `memory4` (trump void on discard)).
- `pimc:S[:T]`: PIMC with `S` samples per decision from trick `T` (default 1); `pimcs:a,b,c,...`
  samples per trick; bidding and declarations by `lab[flags]`; sampler flags `declInfer`,
  `bidInfer`, `contract` (capot-correct since revision 2), `riskNN` (rank by mean − NN/10 · SE).
- `shipped[:T]`: the production search path from trick `T` (default 3): `bot.SearchAppliesWith`,
  `bot.SearchCardWith`, `bot.DecideWith`, with no deadline, otherwise `bot.Decide`. The tuning is
  `bot.DefaultSearchTuning()` plus the flags `phase2` (zero tuning and the phase-2 sample caps
  16/30/40 on tricks 3/4/5, so it plays the bot of fc49e0d), `defTrump3`, `defTrump10`,
  `defTrumpVeto`, `noDefTrump`, `tenUnder3`, `noTenUnder`, `riskNN` (λ = NN/10), `noRisk`,
  `drawCond`, `noDrawCond`, `race`, `noRace`, `opening12`, `noOpening`. `duplicate` prints the
  applied, decided, deferred and defender-trump-lead counts of a side that reports them.
- `random`: uniform over legal options.

## 2. Results

### 2.1 Sanity

| Match | Deals | Advantage / hand | Share | Notes |
|---|---|---|---|---|
| current vs random, Bitola | 2000 | +73.6 ± 1.7 | 69.1% | matches the repo's ≥60% test |
| current vs random, Croatian | 1000 | +86.0 ± 2.3 | 72.2% | |
| current vs lab[] (identical copy), Bitola | 2000 | 0.00 ± 0.00 | 50.0% | copy is faithful; **1240 / 4000 hands passed out (31%)** |

### 2.2 Play: PIMC vs current (same bidding on both sides)

Every row has its own `-seed` (different deal sets), so rows are not like-for-like comparisons
of sample counts; the reviewer's independent reproductions of `pimc:12 vs current` (120 deals)
gave +9.0 ± 3.6 (seed 1) and +7.6 ± 3.1 (seed 2).

| A | B | Variant | Deals | Seed | Advantage / hand | Share |
|---|---|---|---|---|---|---|
| pimc:12 | current | Bitola | 120 | 21 | **+9.9 ± 3.1** | 53.1% |
| pimc:30 | current | Bitola | 120 | 22 | **+15.0 ± 4.3** | 55.3% |
| pimc:12 | current | Croatian | 120 | 23 | **+18.2 ± 4.6** | 54.7% |
| pimc:20:2 (current rules on trick 1, PIMC after) | current | Bitola | 150 | 61 | **+6.4 ± 2.5** | 52.5% |
| pimc:20:3 (current rules on tricks 1–2, PIMC after) | current | Bitola | 250 | 62 | +1.6 ± 1.9 | 50.6% |
| pimc:12:3 (reviewer's run) | current | Bitola | 150 | 1 | +5.9 ± 3.0 | 52.3% |
| pimc:12 | pimc:12:3 (direct A/B) | Bitola | 300 | 113 | **+0.5 ± 2.2** | 50.2% |
| pimc:12:3 (confirmation) | current | Bitola | 400 | 131 | **+4.4 ± 1.6** | 51.6% |
| pimc:12:2 | current | Bitola | 300 | 133 | +3.9 ± 1.9 | 51.4% |
| pimcs:3,6,16,30,40,60,60 | pimcs:0,0,16,30,40,60,60 (rules on tricks 1–2) | Bitola | 300 | 132 | **−2.8 ± 2.8** | 49.1% |

Reading (final, pooled by inverse variance):

| quantity | rows | pooled |
|---|---|---|
| search on every trick, 12 samples, vs current | +9.9 ± 3.1, +9.0 ± 3.6, +7.6 ± 3.1 | **+8.8 ± 1.9** |
| search from trick 3, rules on 1–2, vs current | +1.6 ± 1.9 (20 samples), +5.9 ± 3.0, +4.4 ± 1.6 | **+3.6 ± 1.1** |
| search from trick 2, vs current | +6.4 ± 2.5, +3.9 ± 1.9 | +4.8 ± 1.5 |
| value of searching tricks 1–2 at 12 samples, indirect (first minus second) | | +5.2 ± 2.2 |
| the same, direct A/B `pimc:12` vs `pimc:12:3` | | **+0.5 ± 2.2** |
| the two pooled | | **+2.9 ± 1.6** |
| 3/6-sample opening vs rules on tricks 1–2, direct A/B | | **−2.8 ± 2.8** |

The two direct A/Bs are the primary evidence on the opening: at 12 samples it is worth between
nothing and about +5, and at 3/6 samples nothing. The indirect estimate from the small vs-current
rows is 1.5σ higher; either a couple of high draws among ~8 small rows, or a real effect in
which PIMC's speculative opening lines gain mostly by exploiting the current bot's weak early
defence, which a searching opponent does not reward. With the unoptimised solver, 12 samples at
trick 1 cost ~3.6 s per decision, which is why the affordable step is the search from trick 3
and the opening search waits for the faster solver. The `pimc:12:3` side of the A/Bs played
tricks 1–2 with `lab[]`, the rules without the §3 fixes; the shipped phase 2 uses the fixes, so
the comparison is conservative for it.
(The first from-trick-3 row was re-run after a binary that ignored the from-trick option was
replaced; the 0-second run it replaced is discarded.)

| pimcs:6,10,20,30,40,60,60 (samples per trick 1..7) | current | Bitola | 150 | 66 | **+9.3 ± 3.2** | 53.7% |
| pimcs:3,6,16,30,40,60,60 | current | Bitola | 150 | 67 | **+10.1 ± 2.9** | 53.5% |
| pimcs:2,4,12,20,30,40,40 | current | Bitola | 150 | 68 | +4.9 ± 2.9 | 51.8% |
| pimcs:3,6,16,30,40,60,60 | current | Croatian | 150 | 69 | **+18.5 ± 4.4** | 54.5% |
| cheat (perfect information) | current | Bitola | 300 | 111 | **+28.5 ± 2.3** | 60.3% |
| cheat | pimc:12 | Bitola | 150 | 112 | +30.9 ± 3.4 | 60.4% |

Information gap: a perfect-information player beats the current bot by 28.5 per deal and PIMC
by 30.9. Duplicate advantages are not transitive across opponents (a cheating opponent also
punishes PIMC's speculative lines), so no "share of the gap recovered" is read from these two
rows; the per-trick regret comparison in §2.7 is the only meaningful information-gap measure.

Reading: three samples on trick 1 and six on trick 2 keep the gain within noise of 12–20
samples; two and four do not. These schedule rows are each ±3, so the knee is indicative, not
established. The reviewer measured decision stability at a fixed 12 samples: 5/10 trick-1 and 4/9
trick-2 decisions changed card under a different seed (same seed: identical, so determinism
holds); hence the anytime stopping rule in the proposal. The value of the search
early in the hand comes from the exact evaluation of each sampled deal, not from averaging many
deals. CPU for that schedule with the unoptimised solver: ~1 s on trick 1, ~0.6 s on trick 2,
~0.25 s on trick 3, then negligible.

### 2.3 Play: single-rule variants vs current (Bitola, 3000 deals each)

| Variant | Advantage / hand | Reading |
|---|---|---|
| `aceFirst` (cash the Ace before the Ten) | **+0.48 ± 0.12** | small, significant, in the expected direction |
| `ruff40` / `ruff50` / `ruff60` (skip a boss whose sampled ruff risk ≥ threshold) | −0.05 / +0.07 / +0.14 (± 0.3) | zero: the fallback lead is no better than the risky cash |
| `aceFirst,ruff50` | +0.15 ± 0.34 | |
| `noPartnerDraw` | **−0.65 ± 0.31** | the partner trump draw helps |
| `drawLow` (draw with the lowest trump instead of Q/K/T/A) | +0.06 ± 0.10 | no difference |
| `tenSignal` (discard the Ten, not the Ace, from A+T on the partner's safe trick) | −0.02 ± 0.01 (Bitola), +0.02 ± 0.05 (Croatian, 4000 deals) | cost ≤ 0.02 per hand (the Bitola figure is a 2σ negative of that size) |
| `aceFirst`, Croatian, 4000 deals | +0.53 ± 0.13 | same as Bitola |
| `ledSecure` (over an opponent, take with the cheapest card no unseen led-suit card beats, instead of the cheapest card that wins now), 4000 deals | **+0.52 ± 0.27** (Bitola), **+1.31 ± 0.35** (Croatian) | the "3rd hand plays the Queen under the Ace" leak |
| `ledSecure,aceFirst,bidEV` (rule-only package with the regression bidding), 4000 deals | +3.75 ± 0.76 (Bitola), +5.98 ± 0.85 (Croatian) | |
| `bidTable` (the proposal's hand-shape table, every row included), 4000 deals | **+4.03 ± 0.65** (Bitola), **+3.87 ± 0.68** (Croatian; +3.90 ± 0.71 before the Croatian four-with-Ace row was added) | pass-outs 1538 / 8000 (Bitola) |
| `memory4` (void in trump after a side-suit discard, fed to the rule layer), 4000 deals | −0.07 ± 0.07 (Bitola), +0.12 ± 0.08 (Croatian) | inert for the rule layer |
| **`ledSecure,aceFirst,bidTable,memory4` (phase-1 package)**, 4000 deals | **+3.91 ± 0.71** (Bitola), **+5.23 ± 0.72** (Croatian; +5.16 ± 0.74 before the four-with-Ace row) | the phase-1 number |

### 2.4 Bidding: calibrated formula vs current

| A | B | Variant | Deals | Advantage / hand | A called / made | B called / made | Passed out |
|---|---|---|---|---|---|---|---|
| lab[bidEV] | current | Bitola | 4000 | **+2.29 ± 0.72** | 4004 / 83% | 2407 / 87% | 1589 / 8000 |
| lab[bidEV] | current | Croatian | 4000 | **+4.12 ± 0.80** | 4291 / 80% | 3709 / 78% | 0 |
| lab[bidEV,aceFirst] | current | Bitola | 4000 | **+3.69 ± 0.72** | | | |
| lab[bidEV] | random | Bitola | 2000 | +70.9 ± 1.6 | | | |
| lab[bidEV] | lab[bidEV] | Bitola | 3000 | 0 | 82% made | 82% made | **802 / 6000 (13%)** |
| lab[bidEV] | lab[bidEV] | Croatian | 3000 | 0 | 79% made | | 0 |

### 2.5 Combined upgrade estimate

| A | B | Variant | Deals | Advantage / hand |
|---|---|---|---|---|
| pimc:20 + bidEV | current | Bitola | 120 | +17.8 ± 5.4 (share 55.5%; A made 83% of 121 calls, B 77% of 75; 44/240 passed out) |
| pimc:20 + bidEV | current | Croatian | 120 | +19.4 ± 6.2 (share 55.1%; A made 82% of 126 calls, B 74% of 114) |
| **phase 2 as it ships**: `pimc:12:3[ledSecure,aceFirst,bidTable,memory4]` (phase-1 rules and table on tricks 1–2, 12-sample search from trick 3) | current | Bitola | 400 (seed 141) | **+8.0 ± 2.5** (A made 86% of 411 calls, B 90% of 234; 155/800 passed out) |
| phase 2 as it ships | current | Croatian | 400 (seed 141) | **+13.1 ± 2.8** with every table row (+11.2 ± 2.9 before the four-with-Ace row was added) |

### 2.6 Information-model ablations (pimc:12 with flag vs plain pimc:12, Bitola, 400 deals)

| Flag | Advantage / hand |
|---|---|
| `bidInfer` (down-weight deals inconsistent with the bidding, 0.2) | **−1.9 ± 1.9** (not significant; the "reasonable bidder" model does not match how the lab bot actually bids, so the inference mis-weights deals) |
| `contract` (contract-aware award at the root), **with the capot bug** the review found (caller capot scored as totA − totB) | +1.4 ± 1.8 |
| `contract`, capot-correct, 300 deals | −1.1 ± 1.7 (not resolved; PIMC vs PIMC with identical bidding makes contract situations symmetric, so this design under-measures it) |
| `declInfer` (run neighbours absent, every seat) | +1.9 ± 1.8 (positive, not resolved at this n) |
| `risk10` (rank cards by mean − 1·SE across samples, a strategy-fusion mitigation), 300 deals | +2.3 ± 2.0 (positive direction, not resolved) |

Both effects are small by construction (the contract rule only bites in close hands; run
inference only when a sequence was revealed) and both are exact or safe, so they are kept in the
design but ranked below the play engine and the bidding. Resolving them to ±1 would need about
4 000 deals per ablation (an hour each with 12 samples); the implementation phase can run that.

### 2.7 Regret by context

Current bot, Bitola, 600 deals, 8 681 decisions: mean **4.30** points per decision, 84 per hand
over all four seats, 81.7% of decisions double-dummy optimal.

| Context | mean regret | n | share of total |
|---|---|---|---|
| lead / our side called | 7.91 | 2044 | **43.3%** |
| lead / their side called | 8.94 | 1050 | **25.2%** |
| follow, opponent winning, 2nd, following suit | 2.58 | 968 | 6.7% |
| follow, opponent winning, 2nd, void | 2.09 | 902 | 5.1% |
| follow, opponent winning, 3rd, following | 4.10 | 393 | 4.3% |
| follow, partner winning, 3rd, following | 2.41 | 659 | 4.3% |
| follow, partner winning, 3rd, void | 4.13 | 559 | 6.2% |
| follow, 4th seat (any) | 0.2–2.4 | 1840 | 3.8% |

By trick: 5.8, 5.9, 4.6, 5.2, 4.3, 2.9, 1.2 (tricks 1–7).

PIMC (12 samples), 60 deals: mean **4.26** per decision, 83.8% optimal; leads 8.2 / 12.0; by
trick 8.4, 4.8, 6.3, 4.3, 3.7, 1.5, 0.6. Same regret as the current bot, yet +10 per hand in
duplicate play: full-information regret does not rank imperfect-information policies.

### 2.8 Suit-distribution facts behind the Ace/Ten rule

Bot holds `k` cards of a side suit; the other 24 cards are random (opponents = LHO/RHO).

| k | P(both opponents can follow round 1) | round 2 | round 3 |
|---|---|---|---|
| 1 | 0.934 | 0.575 | 0.095 |
| 2 | 0.881 | 0.402 | 0.023 |
| 3 | 0.796 | 0.221 | 0.000 |
| 4 | 0.664 | 0.074 | — |
| 5 | 0.474 | 0.000 | — |

P(at least one opponent is void in the suit **and** holds a trump), bot holds `k` of the suit and
`t` trumps, Monte Carlo 200 000 deals:

| k | t | round 1 | round 2 |
|---|---|---|---|
| 2 | 1 | 0.118 | **0.592** |
| 2 | 3 | 0.115 | 0.570 |
| 3 | 1 | 0.203 | **0.770** |
| 3 | 3 | 0.195 | 0.737 |

### 2.9 Phase 3 measurements

These rows use the `shipped` policy (§1.5): the production search path, so a number here is what a
player meets. Every tuning is measured against the cumulative baseline, the bot as shipped at the
previous step, never against the phase-2 snapshot alone. Duplicate, 400 deals per row unless noted;
advantage is per hand, ± is one standard error, commit is the server code the lab was built from.

Summary. Pooled is inverse-variance over the two variants. The commit is the code the measured
rows reproduce at (the flags that select the item belong to the command in its row below).

| Item | Pooled effect ± SE | Decision | Commit |
|---|---|---|---|
| A. Solver speed (five optimisations) | no strength change (identical `shipped:3` vs `current` result before and after); trick-1 12-sample mean 889 to 341 ms, trick-3 41 to 20.5 ms | ships | abbce774, b9159346 |
| B1. `DefenderTrumpLeadBonus` | -0.19 ± 0.21 (bonus 10); bonus 3 -0.09 ± 0.14; veto -1.16 ± 0.37 | ships at 10 under the convention rule; veto worse at 2 SE | c5567547 |
| B2. `TenUnderAceBonus` | -0.00 ± 0.04 | ships at 3 under the convention rule | c5567547 |
| B3. `LeadRiskLambda` | risk5 +0.06 ± 0.18; risk10 -0.17 ± 0.30 | off (0), not better at 2 SE | c5567547 |
| B4. `TrumpDrawConditional` | -0.05 ± 0.32 | off, not better at 2 SE | c5567547 |
| Sample caps 32/60/80 on tricks 3/4/5 | +0.79 ± 0.21 (z 3.8, four runs) | ships | 3cec38ef |
| C1. Own Belote in the settlement | no measurement row: a correctness fix pinned by tests | ships | 9d0b5a2a |
| C2. `RaceObjective` | +0.0233 ± 0.0043 match edge per game (z 5.4) | ships on | 1c535a08 |
| D. Opening search (`OpeningSamples` 12) | +0.83 ± 0.99 (z 0.8) | not shipped, `OpeningSamples = 0` | 2b241506 |
| E. Bidding table under the search | see §3.5 (row 4 retired, rows 7 and 8 kept) | ships | 3960d458, 0cbbd083 |
| Phase regression, `shipped:3` vs the phase-2 bot | +1.24 ± 0.54 (z 2.3) | gate met | 1ed9c60d |

| Policy A | Policy B | Variant | Deals | Seed | Commit | Flags / command | Result |
|---|---|---|---|---|---|---|---|
| `shipped:3` | `shipped:3` | Bitola | 40 | 1 | d3bdf40 | `-exp=duplicate -a=shipped:3 -b=shipped:3 -n=40 -seed=1` | +0.00 ± 0.00; counts of A and B equal (applied 492, decided 492, deferred 0) |
| `shipped:3` | `current` | Bitola | 400 | 301 | d3bdf40 | `-exp=duplicate -a=shipped:3 -b=current -n=400 -seed=301 -workers=6` | +12.02 ± 1.90 (A share 53.5%); decided 4838, deferred 0, defender trump leads 55 |
| `shipped:3` | `current` | Croatian | 400 | 302 | d3bdf40 | `-exp=duplicate -a=shipped:3 -b=current -n=400 -seed=302 -workers=6 -variant=croatia` | +10.96 ± 2.24 (A share 52.8%); decided 5513, deferred 0, defender trump leads 62 |
| `shipped:3` | `current` | Bitola | 400 | 301 | abbce77 | `-exp=duplicate -a=shipped:3 -b=current -n=400 -seed=301 -workers=6` | +12.02 ± 1.90 (A share 53.5%); decided 4838, deferred 0, defender trump leads 55; identical to d3bdf40 after the solver speed-up, wall 10 s before and 4 s after (`-workers=6`) |

#### Stability and the cheat gap, before B (16/30/40 schedule)

Stability: `shipped:3` plays all four seats; at every position where the search ran, the search is
re-run with sampler seed salts 0..4 and the position counts as changed if any salted card differs
from the salt-0 card. Cheat gap: double-dummy on the true deal against the shipped bot.

| Policy A | Policy B | Variant | Deals | Seed | Commit | Flags / command | Result |
|---|---|---|---|---|---|---|---|
| `shipped:3` | (self-play) | Bitola | 300 | 501 | 7734271 | `-exp=stability -a=shipped:3 -n=300 -seeds=5 -seed=501 -workers=6` | positions per trick 3..7: 711 / 731 / 769 / 770 / 813; changed 51.9% / 45.0% / 33.2% / 19.7% / 6.3%; pooled tricks 3-4 **48.4%** (1442), tricks 5-8 **19.5%** (2352) |
| `shipped:3` | (self-play) | Croatian | 300 | 502 | 7734271 | `-exp=stability -a=shipped:3 -n=300 -seeds=5 -seed=502 -workers=6 -variant=croatia` | positions per trick 3..7: 824 / 807 / 810 / 829 / 858; changed 56.4% / 49.6% / 39.4% / 20.7% / 8.2%; pooled tricks 3-4 **53.0%** (1631), tricks 5-8 **22.5%** (2497) |
| `cheat` | `shipped:3` | Bitola | 400 | 503 | 7734271 | `-exp=duplicate -a=cheat -b=shipped:3 -n=400 -seed=503 -workers=6` | **+24.15 ± 2.91** (A share 57.3%); A made 95% of 255 calls, B 78% of 398; 147/800 passed out; same result at b915934, since production salt is 0 |

Reading: the chosen card is not stable under a re-seed in the early searched tricks, well above
30 % on tricks 3-5 in both variants (trick 5 in Bitola is 33.2 %); the later tricks settle. The
cap on samples was not changed here.

#### B. Human play where it pays

Duplicate, 1000 deals per row, `-workers=6`, `±` is one standard error of the pair difference.
Every A/B is cumulative: the baseline is the bot with every earlier shipped item on, and only the
item under test varies through flags. Croatian rows use the Bitola seed plus one, except B3 (the
two λ rows already use 631 and 632, so Croatian uses 731 and 732). Pooled is inverse-variance
(weight 1/SE² per variant). Rows built before `4198b4fe` carry the defaults of the step in the
commit column (the defaults were edited and the lab rebuilt between steps): the shipped
defaults at c5567547 are the zero tuning, so the command names the earlier items as flags
(`defTrump10`, `tenUnder3`) on both sides. Croatian rows add `-variant=croatia`.

Frequency (B1), both sides `shipped:3`, all phase-3 flags off, seed 601:

| Variant | Commit | Command | Result |
|---|---|---|---|
| Bitola | c5567547 | `-exp=duplicate -a=shipped:3 -b=shipped:3 -n=1000 -seed=601 -workers=6` | +0.00 ± 0.00; decided 12460, defender trump leads 152 (1.2 % of decisions) |
| Croatian | c5567547 | `-exp=duplicate -a=shipped:3 -b=shipped:3 -n=1000 -seed=601 -workers=6 -variant=croatia` | +0.00 ± 0.00; decided 13724, defender trump leads 168 (1.2 %) |

| Item (A vs B) | Variant | Seed | Commit | Advantage | Pooled (z) | Decision | Command |
|---|---|---|---|---|---|---|---|
| B1 `defTrump3` vs none | Bitola | 611 | c5567547 | -0.06 ± 0.23 (trump leads 103 vs 143) |  |  | `-exp=duplicate -a=shipped:3 -flagsA=defTrump3 -b=shipped:3 -n=1000 -seed=611 -workers=6` |
| B1 `defTrump3` vs none | Croatian | 612 | c5567547 | -0.10 ± 0.17 (120 vs 186) | -0.09 ± 0.14 (-0.6) | not worse | `-exp=duplicate -a=shipped:3 -flagsA=defTrump3 -b=shipped:3 -n=1000 -seed=612 -workers=6 -variant=croatia` |
| B1 `defTrump10` vs none | Bitola | 611 | c5567547 | -0.21 ± 0.33 (75 vs 143) |  |  | `-exp=duplicate -a=shipped:3 -flagsA=defTrump10 -b=shipped:3 -n=1000 -seed=611 -workers=6` |
| B1 `defTrump10` vs none | Croatian | 612 | c5567547 | -0.17 ± 0.28 (82 vs 186) | -0.19 ± 0.21 (-0.9) | not worse, **largest such, ships (10)** | `-exp=duplicate -a=shipped:3 -flagsA=defTrump10 -b=shipped:3 -n=1000 -seed=612 -workers=6 -variant=croatia` |
| B1 `defTrumpVeto` vs none | Bitola | 611 | c5567547 | -0.81 ± 0.50 (0 vs 143) |  |  | `-exp=duplicate -a=shipped:3 -flagsA=defTrumpVeto -b=shipped:3 -n=1000 -seed=611 -workers=6` |
| B1 `defTrumpVeto` vs none | Croatian | 612 | c5567547 | -1.60 ± 0.56 (0 vs 186) | -1.16 ± 0.37 (-3.1) | worse at 2 SE (Croatian alone -2.9), not shipped | `-exp=duplicate -a=shipped:3 -flagsA=defTrumpVeto -b=shipped:3 -n=1000 -seed=612 -workers=6 -variant=croatia` |
| B2 `tenUnder3` vs `noTenUnder` | Bitola | 621 | c5567547 | +0.02 ± 0.05 |  |  | `-exp=duplicate -a=shipped:3 -flagsA=tenUnder3,defTrump10 -b=shipped:3 -flagsB=noTenUnder,defTrump10 -n=1000 -seed=621 -workers=6` |
| B2 `tenUnder3` vs `noTenUnder` | Croatian | 622 | c5567547 | -0.07 ± 0.09 | -0.00 ± 0.04 (0.0) | not worse, **ships (3)** | `-exp=duplicate -a=shipped:3 -flagsA=tenUnder3,defTrump10 -b=shipped:3 -flagsB=noTenUnder,defTrump10 -n=1000 -seed=622 -workers=6 -variant=croatia` |
| B3 `risk5` vs `noRisk` | Bitola | 631 | c5567547 | +0.06 ± 0.24 |  |  | `-exp=duplicate -a=shipped:3 -flagsA=risk5,defTrump10,tenUnder3 -b=shipped:3 -flagsB=noRisk,defTrump10,tenUnder3 -n=1000 -seed=631 -workers=6` |
| B3 `risk5` vs `noRisk` | Croatian | 731 | c5567547 | +0.05 ± 0.28 | +0.06 ± 0.18 (+0.3) | not better at 2 SE, not shipped | `-exp=duplicate -a=shipped:3 -flagsA=risk5,defTrump10,tenUnder3 -b=shipped:3 -flagsB=noRisk,defTrump10,tenUnder3 -n=1000 -seed=731 -workers=6 -variant=croatia` |
| B3 `risk10` vs `noRisk` | Bitola | 632 | c5567547 | -0.36 ± 0.34 |  |  | `-exp=duplicate -a=shipped:3 -flagsA=risk10,defTrump10,tenUnder3 -b=shipped:3 -flagsB=noRisk,defTrump10,tenUnder3 -n=1000 -seed=632 -workers=6` |
| B3 `risk10` vs `noRisk` | Croatian | 732 | c5567547 | +0.44 ± 0.60 | -0.17 ± 0.30 (-0.6) | not better at 2 SE, not shipped | `-exp=duplicate -a=shipped:3 -flagsA=risk10,defTrump10,tenUnder3 -b=shipped:3 -flagsB=noRisk,defTrump10,tenUnder3 -n=1000 -seed=732 -workers=6 -variant=croatia` |
| B4 `drawCond` vs `noDrawCond` | Bitola | 641 | c5567547 | +0.14 ± 0.45 (deferred 267 of 12271 = 2.2 %) |  |  | `-exp=duplicate -a=shipped:3 -flagsA=drawCond,defTrump10,tenUnder3 -b=shipped:3 -flagsB=noDrawCond,defTrump10,tenUnder3 -n=1000 -seed=641 -workers=6` |
| B4 `drawCond` vs `noDrawCond` | Croatian | 642 | c5567547 | -0.24 ± 0.46 (deferred 344 of 13634 = 2.5 %) | -0.05 ± 0.32 (-0.1) | not better at 2 SE, not shipped | `-exp=duplicate -a=shipped:3 -flagsA=drawCond,defTrump10,tenUnder3 -b=shipped:3 -flagsB=noDrawCond,defTrump10,tenUnder3 -n=1000 -seed=642 -workers=6 -variant=croatia` |
| Caps `caps2x` (32/60/80 on tricks 3/4/5) vs `noCaps` | Bitola | 661 | c5567547 | +0.68 ± 0.59 |  |  | `-exp=duplicate -a=shipped:3 -flagsA=caps2x,defTrump10,tenUnder3 -b=shipped:3 -flagsB=noCaps,defTrump10,tenUnder3 -n=1000 -seed=661 -workers=6` |
| Caps `caps2x` vs `noCaps` | Croatian | 662 | c5567547 | +0.78 ± 0.56 | +0.73 ± 0.41 (+1.8) | below 2 SE, sent to a confirmation run | `-exp=duplicate -a=shipped:3 -flagsA=caps2x,defTrump10,tenUnder3 -b=shipped:3 -flagsB=noCaps,defTrump10,tenUnder3 -n=1000 -seed=662 -workers=6 -variant=croatia` |
| Caps `caps2x` vs `noCaps`, confirmation (3000 deals) | Bitola | 663 | 3cec38ef | +0.91 ± 0.31 |  |  | `-exp=duplicate -a=shipped:3 -flagsA=caps2x -b=shipped:3 -flagsB=noCaps -n=3000 -seed=663 -workers=6` |
| Caps `caps2x` vs `noCaps`, confirmation (3000 deals) | Croatian | 664 | 3cec38ef | +0.65 ± 0.39 | four runs pooled **+0.79 ± 0.21 (+3.8)** | better at 2 SE, no opposite sign, **ships (32/60/80 on tricks 3/4/5 in `search.SamplesForTrick`)** | `-exp=duplicate -a=shipped:3 -flagsA=caps2x -b=shipped:3 -flagsB=noCaps -n=3000 -seed=664 -workers=6 -variant=croatia` |

Four-run pool by weights 1/SE²: (0.68·2.87 + 0.78·3.19 + 0.91·10.41 + 0.65·6.58) / 23.04
= +0.79, SE 1/√23.04 = 0.21.

| Item | Result |
|---|---|
| `DefenderTrumpLeadBonus` (B1) | ships at 10 |
| `TenUnderAceBonus` (B2) | ships at 3 |
| `LeadRiskLambda` (B3) | off |
| `TrumpDrawConditional` (B4) | off |
| Sample caps | ships: `search.SamplesForTrick` is 32/60/80 on tricks 3/4/5 (60 from trick 6), `SampleCaps` stays 0 |
| `RaceObjective` (C2) | ships on, see C below |

B1 ships by the convention rule although its pooled point estimate is -0.19 ± 0.21: it is the
largest bonus that is not worse at 2 SE, and the veto (the only stronger form) is worse at 3.1 SE
pooled.

Gates after the schedule change (f32985b0): the trick-3 benchmark mean is 29.8 ms (max 89.9 ms,
limit 250 ms); search share 82.0 % over 60 hands (9250/11280, 412 plays); search minus rules
+7.10 ± 5.35 per deal over 40 deals (434 plays). The benchmark mean is machine dependent: at the
final HEAD it reads 52 to 54 ms, and the same test at f32985b0 reads 54 ms on the same machine
(three runs each, nothing else loading it).

Gates at `4198b4fe`: `go test -tags nodynamic ./internal/bot/...` passes;
`BELJOT_BOT_SIM=1 ... -run TestSimulation_Search` gives search share 74.6 % over 60 hands
(8587/11510, search ran at 406 card plays) against the random baseline, and search minus rules
+3.45 ± 4.62 per deal over 40 deals (search ran at 436 card plays).

Stability and cheat gap after B, before the 32/60/80 schedule (lab at `4198b4fe` defaults, same
sizes as the run before B):

| Policy A | Policy B | Variant | Deals | Seed | Commit | Flags / command | Result |
|---|---|---|---|---|---|---|---|
| `shipped:3` | (self-play) | Bitola | 300 | 651 | 4198b4fe | `-exp=stability -a=shipped:3 -n=300 -seeds=5 -seed=651 -workers=6` | positions per trick 3..7: 749 / 764 / 750 / 773 / 808; changed 52.7% / 46.3% / 33.6% / 16.8% / 6.3%; pooled tricks 3-4 **49.5%** (1513), tricks 5-8 **18.6%** (2331) |
| `shipped:3` | (self-play) | Croatian | 300 | 652 | 4198b4fe | `-exp=stability -a=shipped:3 -n=300 -seeds=5 -seed=652 -workers=6 -variant=croatia` | positions per trick 3..7: 791 / 847 / 820 / 816 / 876; changed 52.5% / 47.3% / 35.6% / 19.1% / 6.3%; pooled tricks 3-4 **49.8%** (1638), tricks 5-8 **20.0%** (2512) |
| `cheat` | `shipped:3` | Bitola | 400 | 653 | 4198b4fe | `-exp=duplicate -a=cheat -b=shipped:3 -n=400 -seed=653 -workers=6` | **+24.22 ± 2.94** (A share 57.6%); A made 95% of 224 calls, B 76% of 401; 175/800 passed out |

The cheat gap is unchanged within noise (+24.15 before, +24.22 after).

#### C. Endgame objectives

C2, the race objective (`SearchTuning.RaceObjective`). On a stop-at-target table the engine ends
the match the moment a team's running total (match score, hand points, declarations, Belote, no
last-trick or Capot bonus) reaches the target after a trick, a declaration reveal or a Belote. With
the flag on, the search no longer hands such positions to the rules: the solver tracks the card
points each team gains and values a trick that decides the match at ±1000, and the chooser keeps
those values as they are. The last trick goes through the hand score in the engine (no mid-hand
stop there), so the solver keeps the plain remainder for it.

`-exp=race` (`race.go`): per deal, one random team starts 10 to 160 short of 1001 and the other
10 to 400 short, stop at target on; duplicate pairing with the scores fixed to the teams; each game
scores +1 when A's team wins the match, -1 when it loses, 0 when the hand leaves it open; the
result is the mean of the per-deal pair average. Lab built from 1c535a08 (B1, B2 and the sample caps on).

| Item (A vs B) | Variant | Seed | Commit | Command | Match edge per game | Decided | Pooled (z) | Decision |
|---|---|---|---|---|---|---|---|---|
| C2 `race` vs `noRace` | Bitola | 701 | 1c535a08 | `-exp=race -a=shipped:3 -flagsA=race -b=shipped:3 -flagsB=noRace -n=1000 -seed=701 -workers=6` | +0.0150 ± 0.0061 (A won 635, B won 605) | 1240 / 2000 (62.0 %), 1032 mid-hand stops, 248 passed out | | |
| C2 `race` vs `noRace` | Croatian | 702 | 1c535a08 | `-exp=race -a=shipped:3 -flagsA=race -b=shipped:3 -flagsB=noRace -n=1000 -seed=702 -workers=6 -variant=croatia` | +0.0315 ± 0.0061 (A won 767, B won 704) | 1471 / 2000 (73.5 %), 1230 mid-hand stops | **+0.0233 ± 0.0043 (+5.4)** | better at 2 SE in both variants, **ships (`RaceObjective: true`)** |

Pooled by weights 1/SE²: the two SEs are equal, so the mean is (0.0150 + 0.0315) / 2 = +0.0233 and
the SE 0.0061 / √2 = 0.0043. A match edge of +0.023 per game is about 1.2 points of match-win
probability on these tables. Searched decisions for A rose from 1514 to 7510 (Bitola) and from
1729 to 8202 (Croatian) against the `noRace` side, which still hands the in-reach positions to the
rules. Self-play sanity (`-flagsA=race -flagsB=race`, 20 deals): +0.0000 ± 0.0000.

Gates after shipping: `go test -tags nodynamic ./internal/bot/... ./internal/match/` passes;
`BELJOT_BOT_SIM=1 ... -run TestSimulation_Search` gives search share 72.1 % over 60 hands
(8430/11690, 403 plays) and search minus rules +7.10 ± 5.35 per deal over 40 deals (434 plays).
Those simulations play without stop at target, so the flag does not reach them; the search share
moves with the engine's random dealer (78.5 %, 79.6 % and 83.7 % on three runs with the flag off).

After the Belote-at-announcement fix (24b575bb). Until that commit the race banked the seat's own
certain Belote at the root, so a seat holding the trump King and Queen could see its team at the
target before a card was played and value almost every line as won. The solver now credits the
+20 when the seat plays the first card of the pair and checks the target at that moment, as the
engine does at the announcement. The lab needed no change: `NewDealStateWith` plays with
declarations on, the default of both variants, and the `shipped` policy answers the Belote prompt
through `bot.Decide`, which always announces, as production does. The rows are re-run with the
same commands and seeds on a lab built from 24b575bb and paired with the same lab built from
ef2e3c42, the last commit before the fix. The ef2e3c42 rows differ from the 1c535a08 rows above
because the bidding table was recalibrated in between (§3.5), which changes the deals that pass out.

| Item (A vs B) | Variant | Seed | Commit | Command | Match edge per game | Decided |
|---|---|---|---|---|---|---|
| C2 `race` vs `noRace`, before the fix | Bitola | 701 | ef2e3c42 | `-exp=race -a=shipped:3 -flagsA=race -b=shipped:3 -flagsB=noRace -n=1000 -seed=701 -workers=6` | +0.0150 ± 0.0062 (A won 630, B won 600) | 1230 / 2000 (61.5 %), 1023 mid-hand stops, 258 passed out; A searched 7457 |
| C2 `race` vs `noRace`, after the fix | Bitola | 701 | 24b575bb | same | +0.0160 ± 0.0063 (A won 631, B won 599) | 1230 / 2000 (61.5 %), 1023 mid-hand stops, 258 passed out; A searched 7457 |
| C2 `race` vs `noRace`, before the fix | Croatian | 702 | ef2e3c42 | `-exp=race -a=shipped:3 -flagsA=race -b=shipped:3 -flagsB=noRace -n=1000 -seed=702 -workers=6 -variant=croatia` | +0.0335 ± 0.0062 (A won 769, B won 702) | 1471 / 2000 (73.5 %), 1230 mid-hand stops; A searched 8177 |
| C2 `race` vs `noRace`, after the fix | Croatian | 702 | 24b575bb | same | +0.0335 ± 0.0062 (A won 769, B won 702) | 1471 / 2000 (73.5 %), 1230 mid-hand stops; A searched 8175 |

Paired on the same deals, the fix moved the Bitola edge by +0.0010 (one match changed hands, A won
630 then 631) and left the Croatian edge unchanged (the same wins; A's searched decisions went from
8177 to 8175, so a few lines differ without changing a match). Against the shipped rows (+0.0150 ±
0.0061 and +0.0315 ± 0.0061) the fixed bot is not worse at 2 SE in either variant, so the gate is
met. The fix changes valuations only where the seat holds a certain Belote within reach of the
target, which is rare in these rows.

#### D. The opening search

D, searching tricks 1 and 2 as well (`SearchTuning.OpeningSamples = 12`, flag `opening12`). A is
`shipped:1` with the flag, B is `shipped:3` (production: search from trick 3); both carry the
shipped defaults (`DefenderTrumpLeadBonus 10`, `TenUnderAceBonus 3`, `RaceObjective true`).
Lab built from 2b241506.

| Item (A vs B) | Variant | Seed | Commit | Command | Advantage per hand | Applied / Decided (A) | Pooled (z) | Decision |
|---|---|---|---|---|---|---|---|---|
| D `shipped:1[opening12]` vs `shipped:3` | Bitola | 801 | 2b241506 | `-exp=duplicate -a=shipped:1 -flagsA=opening12 -b=shipped:3 -n=1000 -seed=801 -workers=6` | -0.54 ± 1.33 (A share 49.8%) | 17571 / 17571 (B: 12369) | | |
| D `shipped:1[opening12]` vs `shipped:3` | Croatian | 802 | 2b241506 | `-exp=duplicate -a=shipped:1 -flagsA=opening12 -b=shipped:3 -n=1000 -seed=802 -workers=6 -variant=croatia` | +2.56 ± 1.49 (A share 50.6%) | 19416 / 19416 (B: 13564) | **+0.83 ± 0.99 (+0.8)** | |

Pooled by weights 1/SE²: (-0.54 / 1.33² + 2.56 / 1.49²) / (1/1.33² + 1/1.49²) = +0.83, SE
1 / √(1/1.33² + 1/1.49²) = 0.99, z = 0.84.

Trick-1 benchmark (`TestSearchTrick1StaysUnderTheDeadline`, three runs, nothing else loading the
machine): means 318.4, 319.3 and 318.5 ms (median 318.5 ms); maxima 574.4, 578.3 and 573.4 ms
(median 574.4 ms).

Gate conditions: (1) benchmark mean at most 400 ms: met (318.5 ms). (2) pooled gain positive at
2 SE with no variant opposite at 2 SE: not met (+0.83 ± 0.99, z = 0.8; Bitola -0.54 is the
opposite sign but well inside 2 SE). The decision is the controller's and the owner's.

#### Final defaults: regression, stability and the cheat gap

Run at the end of phase 3 on the lab built from 1ed9c60d (server code identical to 0cbbd083, the
shipped defaults `DefenderTrumpLeadBonus 10`, `TenUnderAceBonus 3`, `RaceObjective true`, caps
32/60/80 on tricks 3/4/5, `OpeningSamples 0`, bidding recalibrated under the search).

Regression against the phase-2 bot. A is `shipped:3`. B is `shipped:3` with the flag `phase2`:
zero tuning and the phase-2 caps 16/30/40 on tricks 3/4/5 (§1.5), so B is the phase-2 bot of
fc49e0d except for two things that stay on both sides: the own-Belote settlement fix (9d0b5a2a) and
the equivalence-tested solver speed-ups. Duplicate, 1000 deals (2000 hands), `-workers=6`.

| Policy A | Policy B | Variant | Deals | Seed | Commit | Command | Result |
|---|---|---|---|---|---|---|---|
| `shipped:3` | `shipped:3[phase2]` | Bitola | 1000 | 1001 | 1ed9c60d | `-exp=duplicate -a=shipped:3 -b=shipped:3 -flagsB=phase2 -n=1000 -seed=1001 -workers=6` | **+1.84 ± 0.82** (A share 50.5 %); A called 873 (made 81 %), B called 873 (made 79 %), passed out 254 / 2000; A defender trump leads 69, B 137 |
| `shipped:3` | `shipped:3[phase2]` | Croatian | 1000 | 1002 | 1ed9c60d | `-exp=duplicate -a=shipped:3 -b=shipped:3 -flagsB=phase2 -n=1000 -seed=1002 -workers=6 -variant=croatia` | **+0.79 ± 0.71** (A share 50.2 %); A called 1000 (made 78 %), B called 1000 (made 77 %); A defender trump leads 88, B 184 |

Pooled by weights 1/SE²: w = 1/0.82² = 1.487 and 1/0.71² = 1.984; mean = (1.84 · 1.487 + 0.79 ·
1.984) / 3.471 = **+1.24**, SE = 1/√3.471 = **0.54**, z = 2.3. Both variants are positive and A is
not worse than B at 2 SE, so the gate is met. Both sides bid with the same table, since the
tuning does not touch bidding, so the row measures the card play. Duplicate rooms never stop at
the target (`NewDealState` passes `stopAtTarget=false`), so `RaceObjective` has no effect here and
the row measures the schedule and the two conventions; the race is measured by `-exp=race` (C).

Stability and cheat gap at the final defaults, same sizes as before B; the rows above ("before
the 32/60/80 schedule") are the older measurements.

| Policy A | Policy B | Variant | Deals | Seed | Commit | Command | Result |
|---|---|---|---|---|---|---|---|
| `shipped:3` | (self-play) | Bitola | 300 | 1003 | 1ed9c60d | `-exp=stability -a=shipped:3 -n=300 -seeds=5 -seed=1003 -workers=6` | positions per trick 3..7: 739 / 699 / 690 / 727 / 747; changed 48.0% / 42.2% / 32.0% / 17.5% / 5.6%; pooled tricks 3-4 **45.2%** (1438), tricks 5-8 **18.0%** (2164) |
| `shipped:3` | (self-play) | Croatian | 300 | 1004 | 1ed9c60d | `-exp=stability -a=shipped:3 -n=300 -seeds=5 -seed=1004 -workers=6 -variant=croatia` | positions per trick 3..7: 803 / 767 / 803 / 795 / 862; changed 52.3% / 46.3% / 36.9% / 21.5% / 5.2%; pooled tricks 3-4 **49.4%** (1570), tricks 5-8 **20.8%** (2460) |
| `cheat` | `shipped:3` | Bitola | 400 | 1005 | 1ed9c60d | `-exp=duplicate -a=cheat -b=shipped:3 -n=400 -seed=1005 -workers=6` | **+27.18 ± 2.72** (A share 58.7%); A made 95% of 243 calls, B 77% of 380; 177/800 passed out |

Reading: doubling the samples on tricks 3 to 5 lowered the instability only a little (tricks 3-4:
49.5 % to 45.2 % in Bitola, 49.8 % to 49.4 % in Croatian, on different seeds), so about half of
the cards chosen on tricks 3 and 4 still change under a reseed, and the later tricks settle as
before. The cheat gap is +27.18 ± 2.72 against +24.22 ± 2.94 before the schedule change; the
difference (+2.96) is 0.7 SE of the difference between two runs on different seeds, and the
bidding table changed in between, so it is not read as a change in play.

### 2.10 Phase 4 measurements

Rows use the `shipped` policy (§1.5), `-workers=6`, and the commit of the server code the lab was
built from. The summary is kept at the top; the rows below it carry command, seed and commit.

| item | pooled effect ± SE | metric | decision | commit |
|---|---|---|---|---|

#### Sanity

| policy | shape | variant | n | seed | commit | command | result |
|---|---|---|---|---|---|---|---|
| `shipped:3` | `A,c4,noJ,no9,sA0` | Bitola | 300 deals | 1101 | 37ccec7d | `-exp=bidpos -a=shipped:3 -shape=A,c4,noJ,no9,sA0 -n=300 -seed=1101 -csv=shape_probe.csv -workers=6` | 133 s; `class_table.py` cell `-A 4 0` n = 300 (gain +7.5 ± 9.3); 3 304 non-forced rows; 20 of 503 random deals carry a matching position |

| policy | experiment | variant | n | seed | commit | command | result |
|---|---|---|---|---|---|---|---|
| `shipped:3` | `race`, match decided at hand ends | Bitola | 100 deals x2 | 1105 | e6442166 | `-exp=race -a=shipped:3 -b=shipped:3 -stopAtTarget=false -n=100 -seed=1105 -workers=6` | `decided 118 / 200 games (59.0%): A won 59, B won 59; mid-hand stops 0, passed out 38`; 3 s |
| `shipped:3` | `bidscore oppClose` | Bitola | 100 pairs | 1106 | e6442166 | `-exp=bidscore -a=shipped:3 -scoreState=oppClose -n=100 -seed=1106 -workers=6` | `bidscore oppClose: 100 pairs, call-minus-pass match wins +0.020 ± 0.020, call wins 3/100, pass wins 2/100` (before the candidate-memory fix); 2.9 s, 0.03 s per pair (a 200-pair `neither` probe took 0.15 s per pair; sizes Task 13) |
| `shipped:3` | `bidscore oppClose`, candidate observed after the forced call | Bitola | 100 pairs | 1106 | FIXSHA | `-exp=bidscore -a=shipped:3 -scoreState=oppClose -n=100 -seed=1106 -workers=6` | `bidscore oppClose: 100 pairs, call-minus-pass match wins +0.000 ± 0.028, call wins 2/100, pass wins 2/100`; 3.0 s, 0.03 s per pair |
| `shipped:3` | `race`, match decided at hand ends, same code | Bitola | 100 deals x2 | 1105 | FIXSHA | `-exp=race -a=shipped:3 -b=shipped:3 -stopAtTarget=false -n=100 -seed=1105 -workers=6` | `decided 118 / 200 games (59.0%): A won 59, B won 59; mid-hand stops 0, passed out 38`; 3 s |

#### Counters at the phase-3 bot

Both sides are `shipped:3`, so the A and B lines are identical by construction (each side plays one
seat parity in each game of a pair). Server code at aaec08b0 plus `events.go` as committed with these rows, `-workers=6`, 400 deals per variant. Events:
blind Ten (a non-trump Ten led, or played onto a trick an opponent is winning, while that suit's
Ace is unplayed and neither in the hand nor known in the partner's); high kept then ruffed (last
seat wins cheaply in the led suit while holding a higher Ace or Ten of it, which later lands on a
non-trump-led trick won by an opponent's trump); capots made/conceded by the policy's team; caller belote then fell (caller team announced
Belote, declarations transfer on, the hand fell). Belote is seen at the next card play, so a
Belote announced with the very last card is not counted.

| variant | seed | commit | command | result |
|---|---|---|---|---|
| Bitola | 1103 | aaec08b0 + events.go | `-exp=duplicate -a=shipped:3 -b=shipped:3 -events -n=400 -seed=1103 -workers=6` | `events A: decisions 11200, blind tens 164 (14.6 per 1000), high kept then ruffed 3 (0.3 per 1000), capots made/conceded 19/19, caller belote then fell 6` |
| Croatian | 1104 | aaec08b0 + events.go | `-exp=duplicate -a=shipped:3 -b=shipped:3 -events -n=400 -seed=1104 -variant=croatia -workers=6` | `events A: decisions 12800, blind tens 168 (13.1 per 1000), high kept then ruffed 2 (0.2 per 1000), capots made/conceded 15/15, caller belote then fell 4` |

## 3. Bidding calibration tables

### 3.1 By round and position (Bitola; rows are decision points × allowed suits)

| round | passes before | n | EV(call) | EV(pass) | bot calls | P(gain>0) | missed gain per row when passing | loss per row when calling |
|---|---|---|---|---|---|---|---|---|
| 1 | 0 | 60 000 | −44.6 | −6.7 | 13.7% | 43.6% | 29.2 | 4.8 |
| 1 | 1 | 51 792 | −32.4 | −0.3 | 14.7% | 46.7% | 31.3 | 4.6 |
| 1 | 2 | 44 173 | −45.0 | −9.6 | 15.9% | 48.2% | 31.6 | 5.0 |
| 1 | 3 | 37 164 | −31.6 | −2.0 | 17.3% | 52.4% | 33.6 | 4.8 |
| 2 | 0 | 92 166 | −75.4 | −5.3 | 3.9% | 40.6% | 29.1 | 1.3 |
| 2 | 1–3 | 216 726 | −72 | −2 | 3.9% | 37–44% | 29–38 | 1.1–1.7 |

Croatian, round 1: positions 0/1/2 call 6.2% naturally with 30–32 missed per row; position 3 is
the forced dealer (25% calls, 15.8 lost per row by construction).

### 3.2 By holding class (non-forced rows, pooled over positions; `J9` = both held, `A` = trump Ace held; side Aces = exactly that many; gain = EV(call) − EV(pass) ± SE)

| class | trumps | side Aces | Bitola n | Bitola gain | Croatian n | Croatian gain | current bot calls |
|---|---|---|---|---|---|---|---|
| J | 2 | 0 | 18 168 | −24.2 ± 1.1 | 9 426 | −20.6 ± 1.7 | 0% |
| J | 2 | 1 | 11 987 | −0.1 ± 1.3 | 6 251 | −9.1 ± 2.0 | 0% |
| J | 2 | 2 | 1 961 | **+22.3 ± 3.3** | 1 073 | −1.6 ± 5.0 | 0% |
| J9 | 2 | 0 | 3 680 | **+16.9 ± 2.3** | 1 886 | **+22.6 ± 3.7** | 0% |
| J9 | 2 | 1 | 2 302 | +41.5 ± 2.7 | 1 300 | +35.4 ± 4.4 | 99% / 90% |
| JA | 2 | 1 | 2 318 | +8.5 ± 3.0 | 1 267 | −15.7 ± 4.6 | 0% |
| 9A | 2 | 2 | 406 | −2.4 ± 8.0 | 188 | −65 ± 11 | 0% |
| – | 3 | 1 | 4 397 | −26.6 ± 2.3 | 1 970 | −25.3 ± 3.7 | 0% |
| –A | 3 | 1 | 4 313 | −20.8 ± 2.3 | 1 972 | −38.0 ± 3.7 | 0% |
| 9 | 3 | 1 | 4 244 | +5.4 ± 2.3 | 1 866 | −6.9 ± 3.8 | 0% |
| 9A | 3 | 0 | 4 496 | −13.3 ± 2.1 | 2 034 | −19.0 ± 3.6 | 0% |
| 9A | 3 | 1 | 2 188 | **+5.2 ± 3.1** | 1 043 | **−18.6 ± 5.0** | 100% / 96% |
| J | 3 | 0 | 9 248 | **+29.8 ± 1.5** | 4 061 | **+22.7 ± 2.5** | 30% / 38% |
| J | 3 | 1 | 4 386 | +49.4 ± 2.0 | 1 992 | +46.6 ± 3.5 | 100% |
| J9 | 3 | 0 | 4 533 | +61.6 ± 2.0 | 2 016 | +64.3 ± 3.4 | 61% / 66% |
| JA | 3 | 0 | 4 691 | +29.5 ± 2.0 | 2 176 | +27.5 ± 3.3 | 60% / 65% |
| – (no J/9/A) | 4 | 0 | 828 | −4.2 ± 5.4 | 338 | +5.3 ± 9.3 | 100% |
| – | 4 | 1 | 211 | +28.2 ± 10.6 | 94 | +33 ± 15 | 100% |
| –A | 4 | 0 | 1 609 | +8.7 ± 3.7 | 556 | +10.0 ± 7.4 | 100% |
| 9 | 4 | 0 | 1 559 | +38.1 ± 3.9 | 628 | +33.3 ± 6.5 | 100% |
| J | 4 | 0 | 1 643 | +93.4 ± 3.2 | 612 | +80.1 ± 6.2 | 100% |
| J9 | 4 | 0 | 1 760 | +118.9 ± 2.9 | 653 | +100.9 ± 5.6 | 100% |
| J9 / JA / J9A / 9A | 5 | 0 | ~200 each | +107 to +185 | — | — | 100% |

Named shapes: "J+8+7, no side Ace" +5.6 ± 4.7 / +14.2 ± 8.1; "J + one other doubleton, no side
honours" −28.1 ± 2.1 / −28.3 ± 3.1; "A + two small, side Ace" −17.2 ± 2.2 / −38.0 ± 3.5.

Value of three policies on the same non-forced rows (per decision row; the duplicate match is
the out-of-sample check):

| policy | Bitola value / row (call rate) | Croatian value / row (call rate) |
|---|---|---|
| current `wantsTrump` | +4.96 (8.2%) | +3.06 (6.2%) |
| regression, threshold 0 | +5.77 (13.2%) | +3.93 (9.9%) |
| reviewer's six-line hand rule | +5.69 (11.2%) | +3.66 (9.0%) |
| **proposed table (proposal §4.3)** | **+5.71 (11.3%)** | **+3.90 (9.5%)** |

Small-cell caution: the earlier "specific round-1 first-bidder hands" list (n ≈ 50–130, SE ≈
13) is kept only as colour: `J98` +63, `J97` +39, `JK8` +32, `JQ8` +22, `J87` +3, `9A8`+side
Ace −64, `9AQ`+side Ace −33, `TK87` −27, `TQ87` −36. With SE 13 a cell needs |gain| > 26 to
mean anything; the pooled cells above are the evidence.

Croatian position effect (gain ± SE by passes before the seat; position 1 is the dealer's
partner, whose pass can leave the dealer forced):

| shape | position 0 | position 1 | position 2 |
|---|---|---|---|
| J + other doubleton, side Ace | −26.6 ± 2.7 | **+16.4 ± 2.9** | −13.4 ± 3.5 |
| J9 doubleton | +14.3 ± 4.1 | +59.7 ± 4.5 | +14.5 ± 5.7 |
| J + two others (no 9/A) | +25.2 ± 3.2 | +48.2 ± 3.2 | +19.3 ± 4.2 |
| three small | −37.4 ± 3.1 | −14.6 ± 3.6 | −44.1 ± 4.6 |

Bid-time visibility factors (for the deferred bidding-consistency inference): a pattern present
in the final 8-card hand was visible when the seat bid with probability J+9 0.357 / 0.536, all
four of a suit 0.071 / 0.214, J with its two suit-mates 0.179 / 0.357 (Bitola 5 of 8 / Croatian
6 of 8).

### 3.3 Linear fit and threshold policy

Fit of gain on the features of §4.3 of the proposal (R² ≈ 0.11; the outcome of one deal is
noisy, the class means are not). Evaluating "call iff fitted gain > threshold" on the same rows:

| threshold | Bitola call rate | value per row | Croatian call rate | value per row |
|---|---|---|---|---|
| −20 | 21.0% | +5.00 | 15.1% | +3.38 |
| −10 | 16.9% | +5.56 | 11.7% | +3.98 |
| **0** | 13.2% | **+5.77** | 9.6% | +3.91 |
| +10 | 10.5% | +5.56 | 7.1% | +3.55 |
| +20 | 8.2% | +5.16 | 5.0% | +3.14 |
| natural (`wantsTrump`) | 8.2% | +4.96 | 9.1% | +2.15 |

The duplicate result in 2.4 is the out-of-sample confirmation, including the opponents' reaction.

### 3.4 Recalibration under the phase-1 rule layer (2026-10-25/26)

Phase 1 changed the play (the led-suit-secure take, the Ace before the Ten, the Ten under the Ace,
located cards in memory) and replaced the bidding formula with the hand-shape table, so the §3.2
gains, measured against the pre-phase-1 bot, can have moved. The proposal (§4.3, §4.7) asks for
them to be re-checked under the new play. This section is that re-check. It changes nothing in
the table.

**Instrument.** The lab snapshot in `docs/bot/lab/` was brought up to the branch. `BuildView`
fills the `View` fields phases 1 and 2 added (`HasTrumpCandidate`, `DeclaredCards`, `Clockwise`,
`TrickNumber`, `DeclPoints`, `MatchTarget`, `StopAtTarget`, `DeclarationsTransfer`) and
`CurrentBot` passes them on. `Memory` keeps declared and located cards apart (`KnownCards` is
their union, `DeclaredCards` the declarations alone), `PlayHand` records the announced Belote pair
as located cards as `observeBotMemoryLocked` does, and `CurrentBot` hands the rule layer the void
matrix that includes trump voids, as production's `Memory` does. Every `NewGame` call takes the
two new trailing flags (`true, false`). `labbot.go` stays the frozen pre-phase-1 copy. The lab's
`current` is now `bot.Decide` of branch `feat/bot-v2-phase2` at `dd4ec76`: the phase-1 rule layer,
with no search. Every seat of every rollout plays by the rules.

Check of the snapshot: `-exp=duplicate -a=current -b=lab -n=4000` (4 000 deals × 2) gives
**+4.50 ± 0.69** (Bitola, seed 51) and **+5.02 ± 0.72** (Croatian, seed 52), against +4.05 ± 0.69
and +5.20 ± 0.71 measured for the implemented package against the same frozen copy.

**Runs.** The first two are the re-check. The last two use the same instrument with five times
the deals, because the first two have about twice the standard error of §3.2.

| Run | Variant | Deals | Seed | Non-forced rows | Wall time |
|---|---|---|---|---|---|
| re-check | Bitola | 20 000 | 41 | 116 748 | 16 s |
| re-check | Croatian | 10 000 | 42 | 83 164 (93 588 with the forced dealer rows) | 11 s |
| confirmation | Bitola | 100 000 | 43 | 582 979 | 85 s |
| confirmation | Croatian | 50 000 | 44 | 416 144 | 57 s |

```
./botlab -exp=bidpos -a=current -n=20000 -seed=41 -csv=recal_bitola.csv -workers=6
./botlab -exp=bidpos -a=current -n=10000 -seed=42 -variant=croatia -csv=recal_croatia.csv -workers=6
python3 -I class_table.py recal_bitola.csv recal_croatia.csv
```

The runs are fast because no search is involved. They have fewer rows per deal than the original
run (Bitola 5.8 against 8.4, Croatian 9.4 against 10.9 with its forced rows): the walk stops at
the first natural call, and the phase-1 table calls more often. A class is the same hand
shape in both measurements, so the class-by-class comparison stands. What differs is the mix of
positions and earlier passes behind each class.

**Class tables, re-check runs** (same layout as §3.2, non-forced rows pooled over positions, rows
with n ≥ 150 in either variant, gain = EV(call) − EV(pass) ± SE). A bold gain moved by more than 2
SE of this measurement against the §3.2 cell of the same class.

| class | trumps | side Aces | Bitola n | Bitola gain | Croatian n | Croatian gain |
|---|---|---|---|---|---|---|
| - | 0 | 0 | 4 427 | −110.2 ± 2.1 | 5 088 | −103.6 ± 2.0 |
| - | 0 | 1 | 4 522 | −101.2 ± 2.2 | 5 548 | −102.7 ± 1.9 |
| - | 0 | 2 | 1 382 | −90.9 ± 3.9 | 1 710 | −108.7 ± 3.4 |
| - | 1 | 0 | 11 626 | −97.1 ± 1.3 | 9 175 | −86.1 ± 1.5 |
| - | 1 | 1 | 10 042 | −88.6 ± 1.4 | 8 053 | −86.8 ± 1.6 |
| - | 1 | 2 | 2 306 | −79.3 ± 3.1 | 1 891 | −86.7 ± 3.4 |
| -A | 1 | 0 | 2 027 | −100.4 ± 3.1 | 1 930 | −102.3 ± 3.2 |
| -A | 1 | 1 | 1 810 | −86.2 ± 3.5 | 1 620 | −103.8 ± 3.6 |
| -A | 1 | 2 | 484 | −42.7 ± 7.2 | 356 | −121.0 ± 8.1 |
| 9 | 1 | 0 | 2 199 | −83.4 ± 3.1 | 1 884 | −75.0 ± 3.5 |
| 9 | 1 | 1 | 1 932 | −73.1 ± 3.2 | 1 677 | −80.5 ± 3.7 |
| 9 | 1 | 2 | 457 | −61.2 ± 6.9 | 359 | −72.8 ± 8.1 |
| J | 1 | 0 | 2 185 | −46.1 ± 3.1 | 2 047 | −51.2 ± 3.4 |
| J | 1 | 1 | 1 863 | −26.4 ± 3.3 | 1 775 | −52.0 ± 3.7 |
| J | 1 | 2 | 386 | −2.5 ± 7.3 | 419 | −61.8 ± 7.7 |
| - | 2 | 0 | 9 011 | −77.8 ± 1.5 | 5 320 | −67.8 ± 2.0 |
| - | 2 | 1 | 5 994 | −65.4 ± 1.9 | 3 637 | −65.3 ± 2.6 |
| - | 2 | 2 | 954 | −49.3 ± 5.0 | 568 | −54.6 ± 6.4 |
| -A | 2 | 0 | 4 388 | −72.5 ± 2.2 | 2 764 | −77.4 ± 2.9 |
| -A | 2 | 1 | 2 925 | −70.4 ± 2.7 | 1 788 | −74.9 ± 3.6 |
| -A | 2 | 2 | 529 | −45.1 ± 7.0 | 278 | −93.1 ± 8.2 |
| 9 | 2 | 0 | 4 254 | −60.4 ± 2.2 | 2 763 | −50.3 ± 3.0 |
| 9 | 2 | 1 | 2 874 | −43.4 ± 2.8 | 1 797 | −46.6 ± 3.7 |
| 9 | 2 | 2 | 465 | −13.7 ± 7.3 | 270 | −33.4 ± 9.8 |
| 9A | 2 | 0 | 793 | −56.9 ± 5.4 | 509 | −68.4 ± 6.7 |
| 9A | 2 | 1 | 579 | −48.3 ± 5.9 | 332 | −49.9 ± 8.7 |
| J | 2 | 0 | 4 361 | **−18.5 ± 2.2** | 2 987 | −18.8 ± 2.9 |
| J | 2 | 1 | 2 824 | −0.5 ± 2.7 | 1 912 | −5.3 ± 3.6 |
| J | 2 | 2 | 470 | +13.8 ± 6.9 | 326 | −5.5 ± 8.5 |
| J9 | 2 | 0 | 862 | +21.1 ± 4.7 | 607 | +30.4 ± 6.8 |
| J9 | 2 | 1 | 538 | +36.6 ± 6.0 | 402 | +37.9 ± 7.6 |
| JA | 2 | 0 | 850 | −11.5 ± 4.9 | 583 | −34.6 ± 6.6 |
| JA | 2 | 1 | 561 | +6.3 ± 6.4 | 363 | −4.9 ± 8.0 |
| - | 3 | 0 | 2 458 | −45.2 ± 3.1 | 1 177 | −28.0 ± 4.7 |
| - | 3 | 1 | 1 157 | −27.5 ± 4.5 | 582 | −27.1 ± 6.8 |
| -A | 3 | 0 | 2 356 | −45.2 ± 3.1 | 1 178 | −43.7 ± 4.6 |
| -A | 3 | 1 | 1 095 | −24.6 ± 4.7 | 577 | −29.8 ± 6.6 |
| 9 | 3 | 0 | 2 386 | −20.7 ± 3.1 | 1 173 | −12.9 ± 4.7 |
| 9 | 3 | 1 | 1 106 | +3.8 ± 4.7 | 575 | −2.9 ± 6.7 |
| 9A | 3 | 0 | 1 161 | **−24.8 ± 4.3** | 605 | −20.3 ± 6.4 |
| 9A | 3 | 1 | 561 | +11.8 ± 6.2 | 289 | −8.6 ± 9.4 |
| J | 3 | 0 | 2 451 | +31.0 ± 2.9 | 1 245 | +23.1 ± 4.5 |
| J | 3 | 1 | 1 227 | +45.4 ± 4.0 | 597 | +42.0 ± 6.2 |
| J9 | 3 | 0 | 1 239 | +63.3 ± 3.8 | 652 | +64.1 ± 5.5 |
| J9 | 3 | 1 | 612 | +81.6 ± 5.5 | 313 | +68.1 ± 8.7 |
| J9A | 3 | 0 | 244 | +74.5 ± 8.4 | — | — |
| JA | 3 | 0 | 1 304 | +25.9 ± 4.0 | 621 | +33.0 ± 6.3 |
| JA | 3 | 1 | 582 | +51.3 ± 5.6 | 279 | +37.9 ± 9.5 |
| - | 4 | 0 | 234 | −1.7 ± 11.3 | — | — |
| -A | 4 | 0 | 442 | −2.5 ± 7.3 | 151 | +17.5 ± 13.1 |
| 9 | 4 | 0 | 450 | +41.6 ± 6.8 | 198 | +26.8 ± 11.1 |
| 9A | 4 | 0 | 494 | +16.8 ± 6.9 | 202 | +20.0 ± 11.1 |
| J | 4 | 0 | 480 | +81.2 ± 6.5 | 199 | +94.3 ± 11.0 |
| J9 | 4 | 0 | 525 | +118.9 ± 5.7 | 241 | +100.3 ± 9.9 |
| J9 | 4 | 1 | 150 | +145.3 ± 10.2 | — | — |
| J9A | 4 | 0 | 295 | +87.0 ± 7.6 | — | — |
| JA | 4 | 0 | 490 | +70.7 ± 6.4 | 198 | +80.4 ± 9.8 |
| JA | 4 | 1 | 180 | +92.1 ± 11.2 | — | — |

**What moved in the re-check runs.** Of the §3.2 classes, 20 (Bitola) and 19 (Croatian) have
n ≥ 150 here and were compared. The test is "more than 2 SE of the new measurement", which is
lenient at this sample size (the SE is about twice the old one, so a move shows only when it is at
least about 4 points for the largest cells and 10 to 25 points for the small ones).

| Variant | Class | Trumps | Side Aces | §3.2 | Re-check | Change | In SE of the re-check |
|---|---|---|---|---|---|---|---|
| Bitola | J | 2 | 0 | −24.2 ± 1.1 | −18.5 ± 2.2 | +5.7 | 2.6 |
| Bitola | 9A | 3 | 0 | −13.3 ± 2.1 | −24.8 ± 4.3 | −11.5 | 2.7 |
| Croatian | none | | | | | | |

Against the proposal's §4.3 evidence cells the same test moves one cell: Croatian row 12 with the
seat first to speak, −27 ± 3 → −12.7 ± 4.5 (+14.3, 3.2 SE). It is still negative, so the position
rule (do not call there) stands. One cell changes sign without passing the test: **Bitola row 4**
(four or more trumps including the Ace, no Jack or 9, no side Ace) is −2.5 ± 7.3 here against +9 ±
4 (+8.7 ± 3.7 in §3.2). The change is 1.6 SE of the new run, and the confirmation run below puts
the cell back at +6.6 ± 3.2, so the flip is sampling noise.

**Confirmation runs.** With 100 000 Bitola and 50 000 Croatian deals the SE is back at the level
of §3.2. Classes whose gain moved by more than 2 SE of the confirmation run:

| Variant | Class | Trumps | Side Aces | §3.2 | Confirmation | Change |
|---|---|---|---|---|---|---|
| Bitola | J | 2 | 0 | −24.2 ± 1.1 | −20.2 ± 1.0 | +4.0 |
| Bitola | J | 2 | 1 | −0.1 ± 1.3 | −4.2 ± 1.2 | −4.1 |
| Bitola | J | 2 | 2 | +22.3 ± 3.3 | +15.3 ± 3.0 | −7.0 |
| Bitola | JA | 2 | 1 | +8.5 ± 3.0 | +3.0 ± 2.7 | −5.5 |
| Bitola | A (no J/9) | 3 | 1 | −20.8 ± 2.3 | −25.0 ± 2.0 | −4.2 |
| Bitola | 9 | 3 | 1 | +5.4 ± 2.3 | +1.2 ± 2.0 | −4.2 |
| Bitola | 9A | 3 | 0 | −13.3 ± 2.1 | −18.1 ± 1.9 | −4.8 |
| Bitola | J | 3 | 0 | +29.8 ± 1.5 | +26.5 ± 1.3 | −3.3 |
| Bitola | J9 | 3 | 0 | +61.6 ± 2.0 | +65.5 ± 1.7 | +3.9 |
| Bitola | 9 | 4 | 0 | +38.1 ± 3.9 | +28.4 ± 3.2 | −9.7 |
| Bitola | J | 4 | 0 | +93.4 ± 3.2 | +82.3 ± 2.8 | −11.1 |
| Croatian | J | 2 | 2 | −1.6 ± 5.0 | +10.0 ± 4.1 | +11.6 |
| Croatian | A (no J/9) | 3 | 1 | −38.0 ± 3.7 | −31.9 ± 3.0 | +6.1 |
| Croatian | 9 | 3 | 1 | −6.9 ± 3.8 | +0.7 ± 3.0 | +7.6 |
| Croatian | 9A | 3 | 1 | −18.6 ± 5.0 | −9.9 ± 4.1 | +8.7 |
| Croatian | J | 3 | 0 | +22.7 ± 2.5 | +31.4 ± 2.0 | +8.7 |
| Croatian | J | 3 | 1 | +46.6 ± 3.5 | +41.0 ± 2.8 | −5.6 |
| Croatian | A (no J/9) | 4 | 0 | +10.0 ± 7.4 | −5.3 ± 5.2 | −15.3 |
| Croatian | 9 | 4 | 0 | +33.3 ± 6.5 | +21.5 ± 5.4 | −11.8 |

In Bitola most called classes drifted down by 3 to 11 points (J with three trumps −3.3, the 9 with
four −9.7, the Jack with four −11.1) while J9 with three trumps moved up 3.9. These moves do not
change what the table calls, except as discussed below. Three Croatian classes change sign (J, two
trumps, two side Aces; the 9 with three trumps and one side Ace, which is +0.7 ± 3.0, so zero;
the Ace with four trumps). No class changes sign in Bitola among those the table relies on.

**The table's evidence cells, per row** (gain ± SE, n in brackets; "Proposal" is the §4.3 figure;
the two columns are the re-check and the confirmation runs of the same variant). Bitola:

| Row | Cell | Proposal | Re-check (20 000 deals) | Confirmation (100 000 deals) |
|---|---|---|---|---|
| 1 | J9, 2 trumps, no side Ace | +17 ± 2 | +21.1 ± 4.7 (n 862) | +13.5 ± 2.1 (n 4 261) |
| 1 | J9, 3 trumps, no side Ace | +62 ± 2 | +63.3 ± 3.8 (n 1 239) | +65.5 ± 1.7 (n 6 159) |
| 1 | J9, 4 trumps, no side Ace | +119 ± 3 | +118.9 ± 5.7 (n 525) | +114.2 ± 2.7 (n 2 448) |
| 2 | J, 3 trumps, no side Ace | +30 ± 2 | +31.0 ± 2.9 (n 2 451) | +26.5 ± 1.3 (n 12 048) |
| 2 | J, 3 trumps, one side Ace | +49 ± 2 | +45.4 ± 4.0 (n 1 227) | +49.3 ± 1.8 (n 5 907) |
| 3 | 9, 4 trumps, no J/A, no side Ace | +38 ± 4 | +41.6 ± 6.8 (n 450) | +28.4 ± 3.2 (n 2 364) |
| 4 | A, 4 trumps, no J/9, no side Ace | +9 ± 4 | −2.5 ± 7.3 (n 442) | +6.6 ± 3.2 (n 2 294) |
| 5 | 4 trumps, no J/9/A, exactly one side Ace | +28 ± 11 | +31.4 ± 18.3 (n 66) | +14.1 ± 8.3 (n 353) |
| 6 | 5 or more trumps | +107 to +185 | +112.3 ± 7.3 (n 418) | +120.0 ± 3.4 (n 2 008) |
| 7 | 9 and A, 3 trumps, one or more side Aces | +8 ± 3 | +12.0 ± 6.0 (n 634) | +2.4 ± 2.7 (n 3 120) |
| 8 | J and A, 2 trumps, one or more side Aces | +13 ± 3 | +6.4 ± 6.1 (n 645) | +7.4 ± 2.5 (n 3 375) |
| 9 | J, 2 trumps (no 9/A), two side Aces | +22 ± 3 | +13.8 ± 6.9 (n 470) | +15.3 ± 3.0 (n 2 404) |
| 10 | 9, 3 trumps (no J/A), one or more side Aces | +7 ± 2 | +6.2 ± 4.5 (n 1 223) | +3.9 ± 1.9 (n 6 374) |
| 11 | J, 2 trumps (no 9), a side Ace, third to speak | +13 ± 2 | +8.9 ± 4.9 (n 833) | +7.8 ± 2.1 (n 4 327) |

Croatian (rows marked "out" are shapes the table does not call in Croatia):

| Row | Cell | Proposal | Re-check (10 000 deals) | Confirmation (50 000 deals) |
|---|---|---|---|---|
| 1 | J9, 2 trumps, no side Ace | +23 ± 4 | +30.4 ± 6.8 (n 607) | +17.3 ± 2.8 (n 2 933) |
| 1 | J9, 3 trumps, no side Ace | +64 ± 3 | +64.1 ± 5.5 (n 652) | +67.6 ± 2.6 (n 3 223) |
| 1 | J9, 4 trumps, no side Ace | +101 ± 6 | +100.3 ± 9.9 (n 241) | +104.2 ± 4.8 (n 895) |
| 2 | J, 3 trumps, no side Ace | +23 ± 3 | +23.1 ± 4.5 (n 1 245) | +31.4 ± 2.0 (n 6 303) |
| 2 | J, 3 trumps, one side Ace | +47 ± 4 | +42.0 ± 6.2 (n 597) | +41.0 ± 2.8 (n 3 024) |
| 3 | 9, 4 trumps, no J/A, no side Ace | +33 ± 7 | +26.8 ± 11.1 (n 198) | +21.5 ± 5.4 (n 931) |
| 4 | A, 4 trumps, no J/9, no side Ace | +10 ± 7 | +17.5 ± 13.1 (n 151) | −5.3 ± 5.2 (n 957) |
| 5 | 4 trumps, no J/9/A, exactly one side Ace | +33 ± 15 | +20.7 ± 24.9 (n 29) | +9.6 ± 14.6 (n 153) |
| 6 | 5 or more trumps | no cell ≥ 150 | +103.2 ± 14.3 (n 111) | +107.1 ± 6.5 (n 563) |
| 12 | J, 2 trumps (no 9), a side Ace, second to speak | +16 ± 3 | +13.8 ± 5.0 (n 901) | +12.9 ± 2.2 (n 4 518) |
| 12 (pass) | same, first to speak | −27 ± 3 | −12.7 ± 4.5 (n 1 233) | −17.0 ± 2.1 (n 5 959) |
| 12 (pass) | same, third to speak | −13 ± 4 | −19.6 ± 6.9 (n 524) | −17.8 ± 3.0 (n 2 857) |
| 7 (out) | 9 and A, 3 trumps, one side Ace | −19 ± 5 | −8.6 ± 9.4 (n 289) | −9.9 ± 4.1 (n 1 426) |
| 8 (out) | J and A, 2 trumps, a side Ace | −16 ± 5 | −3.6 ± 7.3 (n 420) | −13.3 ± 3.2 (n 2 335) |
| 9 (out) | J, 2 trumps (no 9/A), two side Aces | −2 ± 5 | −5.5 ± 8.5 (n 326) | +10.0 ± 4.1 (n 1 500) |
| 10 (out) | 9, 3 trumps (no J/A), a side Ace | −7 ± 4 | −0.0 ± 6.4 (n 630) | −0.2 ± 2.9 (n 3 155) |

**Policy value of the table under the new rollouts** (value per non-forced row ± standard error
over rows, call rate in brackets). `shipped table` is the bot's own calls (the `natural` column:
the production table, one suit per decision). `hand rule A` and `class_table.py table` are the two
reference rules of `class_table.py`. The second is the proposal table as coded there: rows 1 to 7,
9 and 12 (row 7 Bitola only, row 12 Croatian only, row 9 in both variants), without Bitola rows 8,
10 and 11. The script scores every matching suit of a decision as its own row, while the bot calls
one suit.

| Policy | Bitola 20 000 deals | Bitola 100 000 | Croatian 10 000 | Croatian 50 000 |
|---|---|---|---|---|
| shipped table | +6.42 ± 0.17 (15.1%) | +6.25 ± 0.08 (15.1%) | +3.92 ± 0.17 (8.9%) | +3.83 ± 0.07 (8.8%) |
| hand rule A | +6.29 ± 0.16 (13.1%) | +6.18 ± 0.07 (13.0%) | +3.80 ± 0.17 (9.1%) | +3.80 ± 0.07 (9.0%) |
| `class_table.py` table | +6.31 ± 0.16 (13.1%) | +6.19 ± 0.07 (13.1%) | +4.00 ± 0.17 (9.6%) | +3.93 ± 0.08 (9.6%) |

The same three rules in §3.2, under the pre-phase-1 rollouts, were +4.96 / +5.69 / +5.71 (Bitola)
and +3.06 / +3.66 / +3.90 (Croatian), for the then-current `wantsTrump`, the reviewer's hand rule
and the proposal table. Absolute values depend on the row mix and are not comparable across the
two measurements. Within each run the shipped table is level with or ahead of the references in
Bitola (paired difference against the script's table +0.10 ± 0.06 and +0.07 ± 0.03 per row). In
Croatia the script's table is ahead (+0.08 ± 0.05 and +0.11 ± 0.02) through its scoring, not a
missing row: with its row 9 removed the gap stays (+0.12 ± 0.02 in the 50 000-deal run), and each
of the 1 921 rows it then calls and the bot does not (of 416 144, mean gain +27) is a decision at
which the bot called a different matching suit.

**Reading.**

- The strong rows hold under the new play: rows 1, 2, 3 and 6 in both variants, Bitola rows 8, 9
  and 11, and Croatian row 12 at its position (+12.9 ± 2.2 second to speak, against −17.0 ± 2.1
  first and −17.8 ± 3.0 third). Cells on those rows are at most 12 points from their §4.3 figure
  (some by more than 2 SE of the confirmation run, see the tables above), and none is below the
  +5 inclusion line.
- **The two rows the proposal flagged for re-check no longer meet the inclusion criterion** (gain
  at least +5 and at least 2 SE). Bitola row 7 (9 and Ace with three trumps, a side Ace) is
  **+2.4 ± 2.7** (n 3 120) in the confirmation run, against +8 ± 3. Croatian row 4 (the Ace with
  four or more trumps, no Jack or 9, no side Ace) is **−5.3 ± 5.2** (n 957), against +10 ± 7;
  the 10 000-deal run alone had +17.5 ± 13.1 (n 151), which shows how loose that cell is. Three
  more cells are near the line: Bitola row 4 passes narrowly (+6.6 ± 3.2, 2.1 SE), Bitola row 10
  is just under +5 (+3.9 ± 1.9), and row 5 is positive but small (Bitola +14.1 ± 8.3, n 353;
  Croatian +9.6 ± 14.6, n 153, under 2 SE).
- Keeping or dropping those rows barely moves the table's value: the cell's n × gain over the
  rows of the run is +0.013 per row for Bitola row 7 and −0.012 for the Croatian row 4 cell. They
  stay in the table, as the owner decided, and the owner confirmed on 2026-10-26 that they stay.
- **Croatian row 9 is a candidate, not a change.** The J doubleton with two or more side Aces,
  which the table calls in Bitola only, moved from −2 ± 5 to **+10.0 ± 4.1** (n 1 500, 2.4 SE) in
  the 50 000-deal run when the pair has no Ace, and is +6.4 ± 3.7 (n 1 831, 1.7 SE) in the shape
  row 9 calls (the pair may hold the Ace). The 10 000-deal run has −5.5 ± 8.5 and −3.9 ± 7.8.
  Nothing is applied.
- Position cells depend on how the later seats bid under the same table, so they are the cells
  most likely to move with it. The Croatian first-to-speak cell of row 12 did (−27 ± 3 in §3.2,
  −17.0 ± 2.1 here) and stays negative.

**Recalibration under the search.** This check does not cover the search. No lab policy
runs the shipped schedule (`search.SamplesForTrick`, the early stop and the deadline) over the real
phase-1 `bot.Decide` and `bot.SearchCard`: the lab's `pimc` policies (§2.2) use the frozen `labbot`
rules on the early tricks and their own sample counts. Its rollouts are also expensive at the
current solver speed. The two simulation gates measure about 11 ms per searched card play (420
searches in 4.7 s over 60 hands, with two searching seats), so about 0.16 s per hand with all
four seats searching. The 60 000 Bitola and 30 000 Croatian deals of the original calibration come
to about 930 000 rollouts at the phase-1 call rate (9.7 and 11.7 per deal, counted from the CSVs
above), which is roughly 40 CPU-hours, several hours on six workers. That is an estimate from the
gate timings, not a run. The phase-3 solver speed-ups (`BOT_V2_PROPOSAL.md` §4.7) are what make
repeating it routine, and until then the table is calibrated under the rule layer while the
search plays only the cards from trick 3. §3.5 is that repeat, at the sizes of the original
calibration.

### 3.5 Recalibration under the search (2026-11-07)

§3.4 re-checked the table under the phase-1 rule layer and noted that nothing yet measured the
search. This section does that: the same `bidpos` rollouts, with every seat playing the
`shipped` policy, which is the real bot (`bot.Decide` with the search at its shipped defaults:
from trick 3, 32 / 60 / 80 / 60 samples, defender trump-lead bonus 10, Ten-under-Ace 3, race
objective on). It applies the numeric half of the proposal's criterion only (gain at least 2 SE
and at least +5 per hand, n at least 150). Whether a human partner would accept a shape is the
owner's call at PR review, so every cell that passes the numbers is in the table and every flip
is listed with its hand shape. Rows 4 (Croatian) and 7 (Bitola), which the owner kept by decision
on 2026-10-26, are governed by the same numeric criterion this time; the owner's decision of
2026-10-26 keeps Croatian row 4 (now retired, see the verdicts) and Bitola row 7 (now confirmed)
unless this recalibration says otherwise.

**Instrument.** The lab snapshot of `docs/bot/lab/` (including `shipped.go`) built over the
server at commit 49f242f3 (branch `feat/bot-v2-phase3`, the bidding table of twelve rows as
shipped there), in a scratch tree. `bidpos` walks the bidding with the shipped bot and, at each
decision point, rolls the call and the pass out with `-a=shipped:3` on all four seats. The decision
basis is the pooled 60 000 Bitola / 30 000 Croatian deals of the table below.

**First step: 20 000 / 10 000 deals.** The sizes of §3.4 were run first (seeds 901 and 902, 3 086 s
and 1 826 s sequentially on six workers, 115 903 and 84 112 non-forced rows). Their standard
errors are about twice those of the pooled run, and four cells failed the 2 SE test there:

| Row | Variant | Shape | Gain ± SE (n) at 20 000 / 10 000 |
|---|---|---|---|
| 4 | Bitola | the Ace with four or more trumps, no Jack or 9, no side Ace | −2.3 ± 6.9 (492) |
| 4 | Croatian | the same shape | +15.0 ± 11.4 (179) |
| 7 | Bitola | 9 and Ace, exactly three trumps, one or more side Aces | +7.9 ± 6.0 (622) |
| 8 | Bitola | J and Ace, exactly two trumps, one or more side Aces | +9.5 ± 6.2 (650) |

A first commit (3960d458) retired all four. **Rows 7 and 8 had looked out at that size and the
extension reversed it:** pooled they are +10.9 ± 3.4 and +11.9 ± 3.4 and pass, and the two clauses
were restored.

**Extension to the original sizes.** Seeds 903 and 904, run in parallel on six workers each,
pooled with the first step (header kept once) into 60 000 Bitola and 30 000 Croatian deals:

```
./botlab -exp=bidpos -a=shipped:3 -n=40000 -seed=903 -csv=recal3_bitola_903.csv -workers=6
./botlab -exp=bidpos -a=shipped:3 -n=20000 -seed=904 -variant=croatia -csv=recal3_croatia_904.csv -workers=6
```

| Run | Variant | Deals | Seed | Non-forced rows | Wall time |
|---|---|---|---|---|---|
| first step | Bitola | 20 000 | 901 | 115 903 | 3 086 s |
| first step | Croatian | 10 000 | 902 | 84 112 | 1 826 s |
| extension | Bitola | 40 000 | 903 | | 8 770 s |
| extension | Croatian | 20 000 | 904 | | 6 478 s |
| pooled | Bitola | 60 000 | 901 + 903 | 348 794 | |
| pooled | Croatian | 30 000 | 902 + 904 | 249 804 | |

`class_table.py` prints one block per CSV and does not pool, so its class tables were produced
on the concatenated CSV (`python3 -I class_table.py recal3_bitola_pooled.csv`, and the same for
Croatian). The evidence cells, the position cells and the reference scores below come from two
small scratch scripts over the same CSVs (cell filters equal to the shapes in the tables; the
table functions mirror `matchesBidTable` before and after the change); they are not from
`class_table.py`.

**Class tables, pooled** (same layout as §3.4: non-forced rows pooled over positions, n at least
150 in either variant, gain = EV(call) − EV(pass) ± SE). A bold gain moved by more than 2 combined
SE (this run's SE and the §3.4 re-check cell's, added in quadrature) against the §3.4 re-check
cell of the same class. 13 Bitola and 16 Croatian cells moved by that test.

| class | trumps | side Aces | Bitola n | Bitola gain | Croatian n | Croatian gain |
|---|---|---|---|---|---|---|
| - | 0 | 0 | 12 996 | −107.0 ± 1.3 | 14 866 | **−98.7 ± 1.2** |
| - | 0 | 1 | 13 495 | −96.3 ± 1.3 | 16 848 | −100.6 ± 1.1 |
| - | 0 | 2 | 4 344 | **−80.5 ± 2.3** | 5 127 | −104.1 ± 2.1 |
| - | 1 | 0 | 34 489 | −94.6 ± 0.8 | 27 700 | **−79.6 ± 0.9** |
| - | 1 | 1 | 29 966 | **−82.8 ± 0.8** | 24 151 | **−81.9 ± 1.0** |
| - | 1 | 2 | 6 909 | **−71.3 ± 1.9** | 5 730 | **−78.4 ± 2.0** |
| -A | 1 | 0 | 6 461 | −98.1 ± 1.8 | 5 621 | −102.4 ± 2.0 |
| -A | 1 | 1 | 5 694 | −84.7 ± 2.0 | 4 936 | −103.6 ± 2.1 |
| -A | 1 | 2 | 1 510 | **−60.3 ± 4.3** | 1 166 | **−100.0 ± 4.4** |
| 9 | 1 | 0 | 6 471 | −83.3 ± 1.8 | 5 546 | −74.1 ± 2.0 |
| 9 | 1 | 1 | 5 783 | −67.4 ± 2.0 | 5 009 | **−71.7 ± 2.2** |
| 9 | 1 | 2 | 1 333 | −50.4 ± 4.3 | 1 194 | −63.0 ± 4.4 |
| J | 1 | 0 | 6 346 | −44.9 ± 1.9 | 6 081 | −47.1 ± 2.0 |
| J | 1 | 1 | 5 553 | −25.9 ± 2.1 | 5 365 | **−39.9 ± 2.2** |
| J | 1 | 2 | 1 321 | −2.4 ± 4.2 | 1 301 | **−33.6 ± 4.4** |
| - | 2 | 0 | 25 914 | **−72.9 ± 0.9** | 16 161 | **−57.2 ± 1.2** |
| - | 2 | 1 | 17 688 | **−58.2 ± 1.1** | 10 698 | **−58.5 ± 1.5** |
| - | 2 | 2 | 2 949 | −39.6 ± 2.9 | 1 845 | −50.6 ± 3.7 |
| -A | 2 | 0 | 12 796 | −72.4 ± 1.3 | 8 200 | −77.1 ± 1.7 |
| -A | 2 | 1 | 8 453 | **−62.7 ± 1.7** | 5 537 | −74.9 ± 2.1 |
| -A | 2 | 2 | 1 541 | −39.7 ± 4.2 | 904 | **−61.8 ± 5.0** |
| 9 | 2 | 0 | 12 743 | **−49.0 ± 1.3** | 8 118 | **−40.4 ± 1.7** |
| 9 | 2 | 1 | 8 880 | **−33.9 ± 1.6** | 5 521 | **−32.2 ± 2.2** |
| 9 | 2 | 2 | 1 491 | −16.2 ± 4.2 | 888 | −30.8 ± 5.3 |
| 9A | 2 | 0 | 2 561 | −51.3 ± 3.0 | 1 673 | −56.9 ± 3.7 |
| 9A | 2 | 1 | 1 673 | −35.6 ± 3.7 | 1 138 | −55.0 ± 4.7 |
| 9A | 2 | 2 | 328 | −19.2 ± 9.7 | 159 | −51.6 ± 12.7 |
| J | 2 | 0 | 12 767 | −17.8 ± 1.3 | 8 728 | −13.3 ± 1.7 |
| J | 2 | 1 | 8 766 | +2.6 ± 1.6 | 5 778 | −3.4 ± 2.1 |
| J | 2 | 2 | 1 421 | +25.1 ± 4.1 | 939 | +8.4 ± 5.4 |
| J9 | 2 | 0 | 2 508 | +24.1 ± 2.8 | 1 667 | +24.7 ± 3.8 |
| J9 | 2 | 1 | 1 741 | +44.0 ± 3.5 | 1 173 | +45.6 ± 4.7 |
| J9 | 2 | 2 | 310 | +83.1 ± 8.4 | 226 | +74.9 ± 11.1 |
| JA | 2 | 0 | 2 567 | −14.8 ± 3.0 | 1 731 | −27.1 ± 3.8 |
| JA | 2 | 1 | 1 694 | +7.8 ± 3.7 | 1 206 | −16.7 ± 4.7 |
| JA | 2 | 2 | 300 | +35.3 ± 8.9 | 185 | −13.9 ± 12.4 |
| - | 3 | 0 | 7 213 | −39.9 ± 1.8 | 3 618 | −29.4 ± 2.7 |
| - | 3 | 1 | 3 567 | −25.6 ± 2.6 | 1 667 | −17.2 ± 4.1 |
| - | 3 | 2 | 342 | −15.9 ± 9.1 | 159 | −14.8 ± 13.2 |
| -A | 3 | 0 | 7 235 | **−37.2 ± 1.8** | 3 552 | −37.3 ± 2.7 |
| -A | 3 | 1 | 3 484 | −27.6 ± 2.7 | 1 565 | −34.8 ± 4.1 |
| -A | 3 | 2 | 376 | +0.6 ± 8.5 | — | — |
| 9 | 3 | 0 | 7 117 | **−9.3 ± 1.8** | 3 532 | **−0.3 ± 2.7** |
| 9 | 3 | 1 | 3 592 | +5.9 ± 2.6 | 1 712 | +11.7 ± 4.1 |
| 9 | 3 | 2 | 335 | +14.9 ± 9.2 | 161 | +21.6 ± 12.4 |
| 9A | 3 | 0 | 3 550 | **−8.3 ± 2.5** | 1 815 | −20.4 ± 3.7 |
| 9A | 3 | 1 | 1 688 | +9.6 ± 3.6 | 905 | −0.0 ± 5.3 |
| 9A | 3 | 2 | 196 | +21.7 ± 11.8 | — | — |
| J | 3 | 0 | 7 260 | +34.4 ± 1.7 | 3 785 | **+38.3 ± 2.6** |
| J | 3 | 1 | 3 478 | **+57.0 ± 2.4** | 1 868 | +49.6 ± 3.6 |
| J | 3 | 2 | 373 | +75.8 ± 7.7 | 170 | +49.1 ± 12.0 |
| J9 | 3 | 0 | 3 783 | +64.7 ± 2.3 | 1 951 | +65.4 ± 3.5 |
| J9 | 3 | 1 | 1 744 | +90.1 ± 3.4 | 902 | **+91.1 ± 5.5** |
| J9 | 3 | 2 | 178 | +122.9 ± 11.1 | — | — |
| J9A | 3 | 0 | 749 | +61.8 ± 5.2 | 365 | +55.3 ± 7.9 |
| J9A | 3 | 1 | 402 | +82.0 ± 6.9 | 152 | +65.7 ± 12.0 |
| JA | 3 | 0 | 3 820 | +29.3 ± 2.4 | 1 943 | +24.7 ± 3.6 |
| JA | 3 | 1 | 1 811 | +52.1 ± 3.5 | 900 | +25.8 ± 5.1 |
| JA | 3 | 2 | 179 | +99.1 ± 12.1 | — | — |
| - | 4 | 0 | 679 | −9.8 ± 6.0 | 280 | +6.9 ± 9.8 |
| - | 4 | 1 | 203 | +4.4 ± 11.7 | — | — |
| -A | 4 | 0 | 1 424 | +7.0 ± 4.2 | 544 | +5.7 ± 7.1 |
| -A | 4 | 1 | 425 | +23.1 ± 7.7 | 151 | +13.0 ± 12.9 |
| 9 | 4 | 0 | 1 440 | +31.9 ± 4.0 | 546 | +38.8 ± 7.6 |
| 9 | 4 | 1 | 396 | +54.1 ± 8.0 | — | — |
| 9A | 4 | 0 | 1 367 | +19.9 ± 4.1 | 598 | +39.6 ± 6.5 |
| 9A | 4 | 1 | 408 | +38.6 ± 7.6 | 174 | +35.5 ± 11.3 |
| J | 4 | 0 | 1 453 | +82.9 ± 3.7 | 585 | +87.4 ± 6.2 |
| J | 4 | 1 | 431 | +100.1 ± 7.1 | 171 | +89.7 ± 11.0 |
| J9 | 4 | 0 | 1 514 | +115.4 ± 3.7 | 570 | +105.0 ± 6.3 |
| J9 | 4 | 1 | 469 | +144.6 ± 6.4 | 188 | +130.8 ± 11.7 |
| J9A | 4 | 0 | 756 | +89.4 ± 5.2 | 301 | +93.1 ± 8.5 |
| J9A | 4 | 1 | 241 | +119.8 ± 9.9 | — | — |
| JA | 4 | 0 | 1 552 | +72.3 ± 3.7 | 545 | +67.0 ± 6.2 |
| JA | 4 | 1 | 404 | +106.8 ± 7.6 | 154 | +86.4 ± 11.9 |
| 9A | 5 | 0 | 166 | +81.1 ± 13.0 | — | — |
| J9 | 5 | 0 | 196 | +164.4 ± 10.4 | — | — |
| J9A | 5 | 0 | 191 | +141.4 ± 11.5 | — | — |
| JA | 5 | 0 | 187 | +132.4 ± 11.2 | — | — |

**What moved.** Nearly every class table-called or not is less bad under the search than under the
rules, by 5 to 14 points in the large cells (the pass side of each comparison is worth less when
every seat plays the search), so the table as a whole shifted toward calling.

**The table's evidence cells, pooled (60 000 / 30 000 deals)** (gain ± SE; the shape of each cell
is the one of the §3.4 evidence tables; verdict by the numeric criterion). Bitola:

| Row | Cell | §3.4 confirmation (100 000) | Search, first step | Search, pooled | Verdict |
|---|---|---|---|---|---|
| 1 | J9, 2 trumps, no side Ace | +13.5 ± 2.1 | +20.5 ± 4.6 | +24.1 ± 2.8 (n 2 508) | in |
| 1 | J9, 3 trumps, no side Ace | +65.5 ± 1.7 | +66.1 ± 3.7 | +64.2 ± 2.1 (n 4 532) | in |
| 1 | J9, 4 trumps, no side Ace | +114.2 ± 2.7 | +100.4 ± 5.1 | +106.7 ± 3.0 (n 2 270) | in |
| 2 | J, 3 trumps, no side Ace | +26.5 ± 1.3 | +36.3 ± 3.0 | +34.4 ± 1.7 (n 7 260) | in |
| 2 | J, 3 trumps, one side Ace | +49.3 ± 1.8 | +59.9 ± 4.2 | +57.0 ± 2.4 (n 3 478) | in |
| 3 | 9, 4 trumps, no J/A, no side Ace | +28.4 ± 3.2 | +28.0 ± 7.3 | +31.9 ± 4.0 (n 1 440) | in |
| 4 | A, 4 trumps, no J/9, no side Ace | +6.6 ± 3.2 | −2.3 ± 6.9 | +7.0 ± 4.2 (n 1 424, 1.7 SE) | **out** (fails 2 SE) |
| 5 | 4 trumps, no J/9/A, exactly one side Ace | +14.1 ± 8.3 | +5.6 ± 20.9 (n 67) | +4.4 ± 11.7 (n 203) | see the note on row 5 |
| 6 | 5 or more trumps | +120.0 ± 3.4 | +127.0 ± 8.6 | +121.1 ± 4.7 (n 1 196) | in |
| 7 | 9 and A, 3 trumps, one or more side Aces | +2.4 ± 2.7 | +7.9 ± 6.0 | +10.9 ± 3.4 (n 1 884, 3.2 SE) | in (restored) |
| 8 | J and A, 2 trumps, one or more side Aces | +7.4 ± 2.5 | +9.5 ± 6.2 | +11.9 ± 3.4 (n 1 994, 3.5 SE) | in (restored) |
| 9 | J, 2 trumps (no 9/A), two side Aces | +15.3 ± 3.0 | +39.0 ± 6.6 | +25.1 ± 4.1 (n 1 421) | in |
| 10 | 9, 3 trumps (no J/A), one or more side Aces | +3.9 ± 1.9 | +8.7 ± 4.2 | +6.7 ± 2.5 (n 3 927, 2.7 SE) | in |
| 11 | J, 2 trumps (no 9), a side Ace, third to speak | +7.8 ± 2.1 | +11.5 ± 4.9 | +14.1 ± 2.9 (n 2 540) | in |

Croatian (rows marked "out" are shapes the table does not call in Croatia):

| Row | Cell | §3.4 confirmation (50 000) | Search, first step | Search, pooled | Verdict |
|---|---|---|---|---|---|
| 1 | J9, 2 trumps, no side Ace | +17.3 ± 2.8 | +29.7 ± 6.4 | +24.7 ± 3.8 (n 1 667) | in |
| 1 | J9, 3 trumps, no side Ace | +67.6 ± 2.6 | +64.6 ± 5.6 | +63.8 ± 3.2 (n 2 316) | in |
| 1 | J9, 4 trumps, no side Ace | +104.2 ± 4.8 | +104.5 ± 8.9 | +100.9 ± 5.1 (n 871) | in |
| 2 | J, 3 trumps, no side Ace | +31.4 ± 2.0 | +39.8 ± 4.5 | +38.3 ± 2.6 (n 3 785) | in |
| 2 | J, 3 trumps, one side Ace | +41.0 ± 2.8 | +50.8 ± 6.3 | +49.6 ± 3.6 (n 1 868) | in |
| 3 | 9, 4 trumps, no J/A, no side Ace | +21.5 ± 5.4 | +31.8 ± 13.9 | +38.8 ± 7.6 (n 546) | in |
| 4 | A, 4 trumps, no J/9, no side Ace | −5.3 ± 5.2 | +15.0 ± 11.4 | +5.7 ± 7.1 (n 544, 0.8 SE) | **out** |
| 5 | 4 trumps, no J/9/A, exactly one side Ace | +9.6 ± 14.6 | +37.7 ± 40.0 (n 18) | +55.3 ± 16.5 (n 85) | no verdict (n below 150), stays |
| 6 | 5 or more trumps | +107.1 ± 6.5 | +126.8 ± 15.4 | +113.1 ± 8.6 (n 353) | in |
| 12 | J, 2 trumps (no 9), a side Ace, second to speak | +12.9 ± 2.2 | +18.3 ± 4.9 | +15.9 ± 2.9 (n 2 724) | in |
| 12 (pass) | same, first to speak | −17.0 ± 2.1 | −20.9 ± 5.0 | −17.0 ± 2.8 (n 3 603) | stays out |
| 12 (pass) | same, third to speak | −17.8 ± 3.0 | −17.1 ± 7.0 | −9.3 ± 3.9 (n 1 781) | stays out |
| 7 (out) | 9 and A, 3 trumps, one side Ace | −9.9 ± 4.1 | −0.6 ± 8.8 | −0.0 ± 5.3 (n 905) | stays out |
| 8 (out) | J and A, 2 trumps, a side Ace | −13.3 ± 3.2 | −16.2 ± 7.8 | −16.3 ± 4.4 (n 1 391) | stays out |
| 9 (out) | J, 2 trumps (no 9/A), two side Aces | +10.0 ± 4.1 | +16.8 ± 9.5 | +8.4 ± 5.4 (n 939, 1.5 SE) | stays out |
| 9 (out) | same, the pair may hold the Ace (the shape row 9 calls) | +6.4 ± 3.7 | +17.0 ± 8.7 | +4.7 ± 5.0 (n 1 124) | stays out |
| 10 (out) | 9, 3 trumps (no J/A), a side Ace | −0.2 ± 2.9 | +11.9 ± 6.5 | +12.6 ± 3.9 (n 1 873, 3.2 SE) | passes the numbers, see the findings below |

**Verdicts.**

- **Row 4, Bitola and Croatian: out.** Bitola +7.0 ± 4.2 (n 1 424, 1.7 SE), Croatian +5.7 ± 7.1
  (n 544, 0.8 SE). The code row is one rule for both variants, so it leaves both. The same hands
  with a side Ace still call through row 5.
- **Row 7, Bitola: in.** +10.9 ± 3.4 (n 1 884, 3.2 SE); with exactly one side Ace +9.6 ± 3.6
  (n 1 688). Croatian row 7 stays out (−0.0 ± 5.3).
- **Row 8, Bitola: in.** +11.9 ± 3.4 (n 1 994, 3.5 SE). Croatian row 8 stays out (−16.3 ± 4.4).
- **Croatian row-9 shape (J doubleton, two side Aces): stays out.** +8.4 ± 5.4 (n 939, 1.5 SE) with
  no Ace in the pair, +4.7 ± 5.0 (n 1 124) in the shape row 9 calls. The 20 000 / 10 000 step
  (+17.0 ± 8.7) was noise.
- Rows 1, 2, 3, 6, 9, 10 and 11 (Bitola) and rows 1, 2, 3, 6 and 12 (Croatian) pass. Bitola row
  10 is the narrowest (+6.7 ± 2.5). Croatian row 6, which had n 111 before, now has n 353 and
  passes. The position cells of Croatian row 12 hold (+15.9 ± 2.9 second to speak, −17.0 ± 2.8
  first, −9.3 ± 3.9 third).
- **Row 5, Bitola: kept, its evidence cell fails.** The row's own evidence cell (exactly one side
  Ace, no J/9/A) is now measurable (n 203) and does not pass (+4.4 ± 11.7). The clause calls every
  four-trump hand with a side Ace, and the hands only it calls (four trumps, no Jack or 9, Ace
  allowed, side Ace) measure +16.4 ± 6.4 (n 656, 2.6 SE), so the row stays; the no-Ace part of the
  hands the row calls is +6.3 ± 11.5 (n 218); the pure cell is too small to separate from zero.
  Croatian row 5 has n 85 in its cell (+55.3 ± 16.5) and +27.9 ± 10.0 (n 248, 2.8 SE) with the
  Ace allowed.

**Flips against the table shipped at 49f242f:**

| Row | Variant | Shape | Old verdict | New verdict | Gain ± SE (n) |
|---|---|---|---|---|---|
| 4 | Bitola | the Ace with four or more trumps, no Jack or 9, no side Ace | in (owner's decision) | out | +7.0 ± 4.2 (1 424), 1.7 SE |
| 4 | Croatian | the same shape | in (owner's decision) | out | +5.7 ± 7.1 (544), 0.8 SE |
| 7 | Bitola | 9 and Ace, exactly three trumps, one or more side Aces | in | in (unchanged) | +10.9 ± 3.4 (1 884) |
| 8 | Bitola | J and Ace, exactly two trumps, one or more side Aces | in | in (unchanged) | +11.9 ± 3.4 (1 994) |

**Bitola row 4.** The pooled cell is +7.0 ± 4.2 under the search, against +6.6 ± 3.2 at 100 000
deals in §3.4: the same gain, positive both times, and above the +5 line. The criterion fails on
power, not on sign: the shape is 0.4 % of the decision rows (1 424 of 348 794) and a 2 SE test at
that frequency needs a standard error under 3.5, which this run (SE 4.2) does not reach.
Retiring the row costs the lab's own scoring −0.028 ± 0.017 per non-forced row (1.6 SE). The
revert is one clause (`p.count >= 4 && p.ace ||` in `matchesBidTable`, with the row 4 test cases
flipped back), and the owner judges the shape at PR review. Croatian row 4 is +5.7 ± 7.1 and has
no case in either direction.

**Reference scores** (value per non-forced row ± SE over rows, call rate in brackets; script
`pol16.py` in the scratch tree, not `class_table.py`, which scores another table; the script scores
every matching suit of a decision as its own row, while the bot calls one suit):

| Policy | Bitola 60 000 deals | Croatian 30 000 deals |
|---|---|---|
| the bot's own calls (`natural`) | +6.99 ± 0.10 (15.2 %) | +4.19 ± 0.10 (8.9 %) |
| table at 49f242f | +7.01 ± 0.10 (15.3 %) | +4.31 ± 0.10 (9.3 %) |
| after the flips (row 4 out) | +6.98 ± 0.10 (14.9 %) | +4.30 ± 0.10 (9.1 %) |
| first commit (rows 4, 7, 8 out) | +6.90 ± 0.10 (13.9 %) | +4.30 ± 0.10 (9.1 %) |

Paired differences against the table at 49f242f: row 4 out −0.028 ± 0.017 (Bitola, 1 424 rows) and
−0.012 ± 0.015 (Croatian, 544 rows); rows 4, 7 and 8 out −0.109 ± 0.030 (Bitola, 4 628 rows), which
is why the first commit was reversed. These are in-sample.

**Findings that pass the numbers and are not applied.** The pooled data shows cells outside the
table that clear the criterion. They are new calls rather than checks of the twelve rows, so they
are reported here for the owner and not coded:

- Croatian row 10 shape (9 with three trumps, no Jack or Ace, a side Ace): +12.6 ± 3.9 (n 1 873,
  3.2 SE), a row the table calls in Bitola only. It sat at +11.9 ± 6.5 in the first step.
- Bitola row 11 shape (J doubleton, no 9, a side Ace) at the other seats: second to speak
  +12.3 ± 2.5 (n 3 383, 5.0 SE), first to speak −6.6 ± 2.3, third to speak +14.1 ± 2.9 (the
  current row), dealer (J without the Ace, exactly one side Ace) +12.8 ± 4.4 (n 1 444; one or more
  side Aces +16.0 ± 4.1, n 1 662).
- Croatian 9 with three trumps and exactly one side Ace, second to speak: +29.3 ± 7.0 (n 532; one
  or more side Aces +30.8 ± 6.6, n 575); without the side Ace +19.4 ± 4.5 (n 1 174).

## 4. Reproducing

Build from a throwaway worktree, never inside the repository: the lab files are not part of the
server module.

```
git worktree add /tmp/lab HEAD --detach
mkdir -p /tmp/lab/server/internal/botlab /tmp/lab/server/cmd/botlab
cp docs/bot/lab/{dd,sim,pimc,flags,bidev,labbot,view,memory,shipped,events}.go /tmp/lab/server/internal/botlab/
cp docs/bot/lab/{main,bidtable,bidpos,stability,race,shape,shape_check,bidscore}.go /tmp/lab/server/cmd/botlab/
cd /tmp/lab/server && go build -tags nodynamic -o botlab ./cmd/botlab
./botlab -exp=ddcheck -n=300
./botlab -exp=duplicate -a=current -b=lab -n=4000                     # phase 1 against the frozen v1, §3.4
./botlab -exp=duplicate -a=pimc:12 -b=lab -n=120                      # §2.2: `lab` is the old `current`
./botlab -exp=duplicate -a=lab -flagsA=bidEV -b=lab -n=4000 -variant=croatia
./botlab -exp=regret -a=lab -n=600
./botlab -exp=duplicate -a=shipped:3 -b=current -n=400 -seed=301 -workers=6                  # §2.9, the `shipped` policy
./botlab -exp=duplicate -a=shipped:3 -b=shipped:3 -flagsB=phase2 -n=1000 -seed=1001 -workers=6   # §2.9 regression, phase 3 against the phase-2 bot
./botlab -exp=stability -a=shipped:3 -n=300 -seeds=5 -seed=1003 -workers=6                       # §2.9 stability
./botlab -exp=duplicate -a=cheat -b=shipped:3 -n=400 -seed=1005 -workers=6                       # §2.9 cheat gap
./botlab -exp=race -a=shipped:3 -flagsA=race -b=shipped:3 -flagsB=noRace -n=1000 -seed=701 -workers=6   # §2.9 C
./botlab -exp=race -a=shipped:3 -flagsA=race -b=shipped:3 -flagsB=noRace -n=1000 -seed=702 -workers=6 -variant=croatia
./botlab -exp=bidpos -a=lab -n=60000 -csv=bidpos_bitola.csv && python3 -I fit_bid.py bidpos_bitola.csv
./botlab -exp=bidpos -a=current -n=20000 -seed=41 -csv=recal_bitola.csv -workers=6 && python3 -I class_table.py recal_bitola.csv
./botlab -exp=bidpos -a=shipped:3 -shape=A,c4,noJ,no9,sA0 -n=300 -seed=1101 -csv=shape_probe.csv -workers=6 && python3 -I class_table.py shape_probe.csv   # §2.10, targeted deals
./botlab -exp=duplicate -a=shipped:3 -b=shipped:3 -events -n=400 -seed=1103 -workers=6   # §2.10, table event counts
./botlab -exp=race -a=shipped:3 -b=shipped:3 -stopAtTarget=false -n=100 -seed=1105 -workers=6   # §2.10, match decided at hand ends
./botlab -exp=bidscore -a=shipped:3 -scoreState=oppClose -n=100 -seed=1106 -workers=6   # §2.10, match-score bidding probe
```

The Python scripts are in `docs/bot/lab/` and need numpy. `labbot.go` is the pre-phase-1
`internal/bot/bot.go` with `package botlab` and the flag hooks described in §1.5 (`aceFirst`,
`ruffNN`, `noPartnerDraw`, `drawLow`, `bidEV`), frozen; the hooks are a few lines each and are
listed in the review log. `view.go` and `memory.go` follow the production `bot.View` and
`bot.Memory` as of phase 2, and `sim.go` builds the `View` the way `match.buildBotView` does, so a
later change to those production types needs the same change in the snapshot before it builds.
Because the snapshot records the announced Belote pair as located cards for every policy, a re-run
of a §2 row can differ slightly from its published number.
