# Beljot bot v2: lab method and results

Companion to `BOT_V2_PROPOSAL.md`. Everything here was produced by the throwaway lab in
`docs/bot/lab/` (a copy of the sources; they were run from a git worktree as
`server/internal/botlab` + `server/cmd/botlab`, against the real engine and the real bot).
Date: 2026-10-25, commit 2d70456. §3.4 was run on 2026-10-25/26 on branch `feat/bot-v2-phase2`
at `dd4ec76`, after the lab snapshot was updated to the phase-1 `View` (§1.5, §4).

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
  stay in the table, as the owner decided, and whether to keep them remains the owner's call.
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
search plays only the cards from trick 3.

## 4. Reproducing

Build from a throwaway worktree, never inside the repository: the lab files are not part of the
server module.

```
git worktree add /tmp/lab HEAD --detach
mkdir -p /tmp/lab/server/internal/botlab /tmp/lab/server/cmd/botlab
cp docs/bot/lab/{dd,sim,pimc,flags,bidev,labbot,view,memory}.go /tmp/lab/server/internal/botlab/
cp docs/bot/lab/{main,bidtable,bidpos}.go   /tmp/lab/server/cmd/botlab/
cd /tmp/lab/server && go build -tags nodynamic -o botlab ./cmd/botlab
./botlab -exp=ddcheck -n=300
./botlab -exp=duplicate -a=current -b=lab -n=4000                     # phase 1 against the frozen v1, §3.4
./botlab -exp=duplicate -a=pimc:12 -b=lab -n=120                      # §2.2: `lab` is the old `current`
./botlab -exp=duplicate -a=lab -flagsA=bidEV -b=lab -n=4000 -variant=croatia
./botlab -exp=regret -a=lab -n=600
./botlab -exp=bidpos -a=lab -n=60000 -csv=bidpos_bitola.csv && python3 -I fit_bid.py bidpos_bitola.csv
./botlab -exp=bidpos -a=current -n=20000 -seed=41 -csv=recal_bitola.csv -workers=6 && python3 -I class_table.py recal_bitola.csv
```

The Python scripts are in `docs/bot/lab/` and need numpy. `labbot.go` is the pre-phase-1
`internal/bot/bot.go` with `package botlab` and the flag hooks described in §1.5 (`aceFirst`,
`ruffNN`, `noPartnerDraw`, `drawLow`, `bidEV`), frozen; the hooks are a few lines each and are
listed in the review log. `view.go` and `memory.go` follow the production `bot.View` and
`bot.Memory` as of phase 2, and `sim.go` builds the `View` the way `match.buildBotView` does, so a
later change to those production types needs the same change in the snapshot before it builds.
Because the snapshot records the announced Belote pair as located cards for every policy, a re-run
of a §2 row can differ slightly from its published number.
