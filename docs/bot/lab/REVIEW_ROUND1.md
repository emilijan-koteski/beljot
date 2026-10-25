# Review of BOT_V2_PROPOSAL.md (round 1)

Reviewer brief: sceptical, verify against code and the lab. Files reviewed at their state of
2026-10-25 13:28 (the proposal and lab results were edited while this review was being written;
where the current text already answers a point I say so). Everything I measured is reproducible
from `/tmp/claude-1000/-mnt-data-Projects-personal-beljot/3858171b-642d-4aff-af15-4a9551b19784/scratchpad/review/`
(`bidcheck.py`, `bidformula.py`, `pimctime/main.go` built through a Go overlay against the lab
worktree, and `runs/`).

## 1. Verdict

The direction is right and the instruments are sound: the double-dummy solver mirrors
`validation.go` exactly, the sampler respects every constraint it is given, the duplicate harness
is fair, and PIMC's +10 to +18 per deal is real and reproduces. The weak parts are the layers
around the search: the bidding regression encodes the current bot's play defects as "Belot
knowledge" (a trump Ace is worth less than a trump 7 in the Croatian formula), the bidding-consistency
weight of 0.2 is wrong by a factor of 3 to 4 because the pass was made on 5 or 6 of the 8 cards,
several §3 verdicts rest on cells with n≈50 and SE≈13, and the contract award mis-scores capot
in exactly the case where capot matters. The one finding that should change the design rather
than the numbers is that at 12 samples roughly half of all trick-1 and trick-2 decisions flip when
only the RNG seed changes, so "deterministic", "table-testable" and "conventions as a 1–2 point
tie-break" are true only in a formal sense until the sample count is set by a noise target rather
than by CPU.

## 2. Objections, by severity

### O1. Early-trick PIMC decisions are a seed lottery; the 1–2 point convention margin is below the noise

**Claim challenged.** §4.2: "Sample counts are small because the double-dummy value of a deal is
exact; the only noise is in the deal itself. 12 samples already beat the current bot." §4.3/§4.4:
ties "within 1 point" / "a 1–2 point margin" are handed to the conventions layer. The deterministic,
table-testable framing of the constraints block.

**Why it is weak.** The noise in the deal is the whole problem. I ran `PIMC.Choose` three times per
decision with three independent RNG seeds, 12 samples each, 6 Bitola deals, every decision with >1
legal card (`runs/pimctime_6deals_12s.txt`):

| trick | decisions | changed card with a different seed | mean ms | max ms |
|---|---|---|---|---|
| 1 | 10 | **5 / 10** | 4710 | 12672 |
| 2 | 9 | **4 / 9** | 817 | 1790 |
| 3 | 8 | 2 / 8 | 156 | 275 |
| 4 | 8 | 3 / 8 | 57 | 152 |
| 5 | 8 | 2 / 8 | 18 | 30 |
| 6 | 6 | 0 / 6 | 14 | 24 |
| 7 | 6 | 2 / 6 | 11 | 19 |

(Same seed twice: identical card every time, so determinism holds; timings are on a host with load
average 24–30 on 12 cores, so treat them as upper bounds, roughly 2× a quiet core.) Half of the
early decisions are therefore chosen by the seed, not by the hand. That is still a strong policy on
average (it wins the duplicate match), but:

- A "tie within 1–2 points" has no meaning when the paired sampling SE of the difference between two
  cards is routinely 5–10 points at trick 1. The convention tie-break will fire or not fire for
  reasons unrelated to the cards, and the "cost bounded by the margin" argument is void: the true
  gap between the chosen card and the best card can be far larger than the sampled gap.
- "Table-testable" only holds for fixtures where the answer is unambiguous (§4.6 says this), which
  excludes exactly the trick-1/2 decisions that carry most of the value (§2.2).
- The view-hash seed means two visually identical situations that differ in an irrelevant field
  (order of `PlayedCards`, the opponents' earlier discards) produce different plays. A human partner
  will read that as inconsistency.

**Recommendation.** (a) Compute the per-card mean and the paired SE across samples (every card is
valued on the same samples, so the variance of the difference is what matters) and stop sampling
when the leader's margin over the runner-up exceeds k·SE or the deadline hits; this makes the sample
count a function of difficulty, which is also the right humanisation signal. (b) Define a tie as
"difference within 1 SE", and only then apply conventions, then the rule-layer preference. (c)
Publish "decision stability under re-seeding" as a lab metric next to the advantage; it should be
below ~15% at every trick before the conventions layer is layered on top. (d) Canonicalise the hash
input (own hand sorted, played set as a mask, trick so far, voids, located cards, scores) so that
equal information states hash equal.

### O2. Soft inference 9 (bidding consistency) uses a weight that is wrong by 3–4× in both variants

**Claim challenged.** §4.1 rule 9: a seat that passed on X "is unlikely to hold J+9 of X, four or
more of X, or the Jack with three of X … Inconsistent samples keep weight 0.2", and the lab result
−1.9 ± 1.9 is explained as the reasonable-bidder model not matching the lab bot.

**Why it is wrong.** The pass was made on the bid-time hand, which is 5 of the final 8 cards in
Bitola (the 3-card remainder is dealt after bidding) and 6 of 8 in Croatian (two face-down cards).
The sampler tests the *final* 8-card hand. The probability that an 8-card hand which now holds the
"inconsistent" pattern already showed it at bid time is a hypergeometric constant:

| pattern in the 8-card hand | P(visible at bid time) Bitola (5 of 8) | Croatian (6 of 8) |
|---|---|---|
| J and 9 of X both held | C(6,3)/C(8,5) = **0.357** | C(6,4)/C(8,6) = **0.536** |
| exactly 4 of X held | C(4,4)·C(4,1)/C(8,5) = **0.071** | C(4,2)/C(8,6) = **0.214** |
| J plus two others of X (3 held) | 35/56 · 10/35 = **0.179** | 21/28 · 10/21 = **0.357** |

So a Bitola seat that passed and now holds four of X was, 93% of the time, looking at three or
fewer of them when it passed. Weighting that deal 0.2 throws away correct deals, which is the
measured −1.9. `bidWeight` in `pimc.go` (lines 104–140) applies the same 0.2 to all three
patterns and to the 8-card hand. The "reasonable bidder" explanation in §2.6 is not the main cause.

**Recommendation.** Derive the weight instead of tuning it: w = P(bid-time subset looked like a
pass) + ε·P(it looked like a call), with the hypergeometric terms above (they depend only on the
counts, so they are a 3×2 table). That also fits the owner's "mathematical, not learned" constraint
better than a magic 0.2. Against humans the bidder model is unknown anyway; keep every soft weight
≥ 0.5 until a mixed-table measurement exists. Rule 9 should not be in phase 1.

### O3. The bidding formula encodes the current bot's play defects, and a six-line table is as good

**Claim challenged.** §4.3: the calibrated formula; "Read as bidding points: every trump is worth
~35, the Jack ~90, the Nine ~47, Ace/Ten/King/Queen ~35"; §1: "can be replaced by hand-written
thresholds" (offered as a fallback, not evaluated).

**Why it is weak.**

(a) *The trump Ace coefficient is nonsense as Belot and the summary hides it.* count + rank per
card (`bidformula.py`): Bitola A **+29.9** vs K +35.9, Q +36.5, T +34.9 (7: +21.5); Croatian A
**+14.4** vs 7 **+25.9**, K +35.0. The Croatian formula values the Ace of trump below the Seven of
trump. This is not a regression artefact: the raw cells say the same. Three trumps without J or 9,
Croatian: with the A −43.1 ± 2.0 (n=6170), with the K instead −30.1 ± 2.8 (n=3639); Bitola −33.5 ±
1.3 vs −32.7 ± 1.7. The rollouts were played by the current bot, whose rules handle a non-master
trump Ace badly, so the regression learned "the Ace is a liability". The proposal says the
calibration must be re-run under the new play engine; it should say that until then the
coefficients are a description of v1's play, not of the game, and must not be read as "bidding
points" by the owner.

(b) *A hand-written table scores the same.* On the lab's own rows (per-row value of "call iff rule",
non-forced rows, `bidcheck.py`):

| policy | Bitola value / row (rate) | Croatian value / row (rate) |
|---|---|---|
| current `wantsTrump` | +4.96 (8.2%) | +3.06 (6.2%) |
| formula, threshold 0 | **+5.77** (13.2%) | **+3.93** (9.9%) |
| hand rule A (J9 with 2+; J with 3+; 4+ with J/9/A; 5+; 9A+side Ace with 3; J doubleton with two side Aces) | **+5.69** (11.2%) | +3.66 (9.0%) |
| hand rule B (J9 with 2+; J with 3+; 4+ with 9 or A; 5+) | +5.53 (10.3%) | +3.76 (8.1%) |

In Bitola the six-line table is within 0.08/row of the formula, i.e. within noise. In Croatian the
0.2–0.3 gap is almost entirely position 1 (formula +6.23 vs rule A +5.37 at pos 1; pos 0 and 2 are
equal), which a table expresses as one extra line ("dealer's partner: one notch looser").

(c) *Its weakest calls would look absurd to a human partner, and the data does not support them.*
Weakest hands that call with no side honours at all, from the published coefficients: Croatian pos 1
calls **J+T, J+Q, J+K doubletons** (+3/+5/+6) and **9-T-Q** (+3). Measured gain for "J+Q doubleton,
no side A/T/K, pos 1": −14.1 ± 12.7 (n=137); "J + one honour doubleton, no side honours, pos 1":
+9.9 ± 7.3 (n=439). Neither is distinguishable from zero. A table can clamp "never call on two
trumps unless J+9".

(d) *Forced-dealer rows are in the Croatian fit.* 50 728 rows (15.4%) have `evPass = 0` by
construction (`bidpos.go` line 73–74: the dealer cannot pass), so their "gain" is EV(call) − 0.
I refit without them: coefficients move (const −132.4, side A +3.6, 8/7 −3.2/−4.2) but the policy
value does not (+3.94 vs +3.93). Harmless for the decision, but the pos 3 coefficient is meaningless
and should not be in the published table; the forced dealer only needs the argmax over suits.

(e) R² ≈ 0.11 is defended correctly (per-row noise), but the published policy table is in-sample;
the duplicate result (+2.3 / +4.1) is the honest number and is small.

**Recommendation.** Ship the bidding as the per-class table (plus a position line for Croatian),
which the owner can read and veto cell by cell, and keep the regression as a lab cross-check. If a
formula is kept, constrain it to be monotone within trump (J ≥ 9 ≥ A ≥ T ≥ K ≥ Q ≥ 8 ≥ 7) and
refit under PIMC play before anything ships. Report every cell with its SE.

### O4. Several §3.1 audit verdicts rest on cells that are noise, or are right in one variant and wrong in the other

**Claims challenged and the data behind them** (`bidcheck.py`, pooled rows unless stated):

| verdict in §3.1 | cited evidence | what the rows say |
|---|---|---|
| four small trumps (no J/9/A): **fix**, "−4" | −4 | Bitola −4.2 ± 5.4 (n=828, no side Aces); +3.8 ± 4.8 with any side cards; Croatian +5.3 ± 9.3. Zero within error; the current rule is not wrong, it is a coin flip. |
| 9+A+side Ace (three trumps): **remove**, "+5, 9A8 −64, 9AQ −33" | 40k-deal bidtable cells, n=48 and n=63 (SE ≈ 13) | Bitola pooled **+7.9 ± 3.0** (n=2442): a profitable call, keep it. Croatian **−19.9 ± 4.7** (n=1156): remove. 9AQ+side Ace in the 60k Bitola data: +14.3 ± 6.5 (all positions), +7.9 ± 13.1 (round 1, first bidder). The −33 is a small-cell draw. |
| J+8+7 with nothing: "marginal (+3)" | — | Bitola +5.6 ± 4.7, Croatian +14.2 ± 8.1. Consistent with the verdict. |
| J + any two: +30 | — | +29.8 ± 1.5 (Bitola), +22.7 ± 2.5 (Croatian). Solid. |

The specific-hand list in lab results §3.2 (n ≈ 50–130, "±10–15") is being used to overturn rules;
with SE 13 a cell needs |gain| > 26 to mean anything, and the two cells that drive the "remove"
verdict are both inside that band in the larger data set.

**Recommendation.** Per-variant verdicts (the formula already is per-variant; the prose is not).
Quote SEs. Drop the specific-hand list or raise n to ≥ 500 per cell.

### O5. The schedule results (§2.2) are too fragile to support "most of the value is in tricks 1–2"

**Claim challenged.** §2.2 reading: "Running the search only from trick 3 keeps almost nothing"
(pimc:20:3 +1.6 ± 1.9, 250 deals; pimc:20:2 +6.4 ± 2.5, 150 deals).

**Why it is weak.** I ran `pimc:12:3 vs current`, 150 deals, seed 1 (`runs/pimc12_from3_vs_current.txt`):
**+5.87 ± 2.95**, share 52.3%, 98/300 passed out. That is 60% of the full-PIMC +9.9, not "almost
nothing", and it is 1.2σ from the proposal's +1.6. Neither number is wrong; both are under-powered
for a schedule comparison (the question is a 2–4 point difference and the SEs are 2–3). Two further
reproducibility problems in the same table: the scratchpad still holds an earlier
`sched_pimc20_from3.txt` for the same row showing a 0-second run with +0.00 (PIMC never fired), so
the row was re-run with a changed binary; and the pimc:12 and pimc:30 rows were run with the
default seed (the reproduction section passes no `-seed`), the same deals and the same bidding, yet
report 50 and 80 pass-outs of 240 — impossible for identical deals and identical bidding, so at
least one row came from a different seed or binary and "+10 with 12 samples, +15 with 30" is not a
like-for-like comparison (per played hand the two rows would read +12.5 and +22.5). My own
reproduction with the current binary (`botlab3`, `-seed=1`, 120 deals) gives **+8.96 ± 3.58**
with **74/240** pass-outs, so the documented pimc:12 row (50/240) is not the current binary at the
default seed either; `-seed=2` gives **+7.61 ± 3.10** (74/240). The PIMC effect is real and
reproduces in magnitude (+9 to +10 per deal, +11 to +13 per played hand); the schedule and
sample-count deltas on top of it do not.

**Recommendation.** Fix `-seed`, record the binary commit per row, and run schedule comparisons at
≥ 400 deals (SE ≈ 1.5) or as a direct A/B (`pimc:S:1` vs `pimc:S:3`), which halves the variance
because both sides are PIMC.

### O6. Strategy fusion is not discussed, and the proposal's own regret table shows it biting

**Claim challenged.** §4.3 PIMC as described, with no mitigation and no measurement of the
information gap.

**Why it matters here.** PIMC assumes every seat plays with perfect information from the next card
on. In this rule set the decisions where that assumption is false and costly are: (i) whether to
draw trumps (the solver "knows" who holds the J and 9 in each sample and leads accordingly), (ii)
which side suit to open (it "knows" who is void and who has trumps left), (iii) the partner's
line (it assumes the partner also sees the deal). Forced following, forced cutting and forced
over-trumping reduce the defenders' options and therefore the damage, which is why PIMC does well
in Belot-like games (Long, Sturtevant, Buro & Furtak 2010: high leaf correlation, fast
disambiguation). But the lab's regret table is a direct signature: PIMC's trick-1 regret is **8.4**
against the current bot's **5.8**, and its lead regret when the opponents called is **12.0** vs
**8.9**. PIMC picks leads that are perfect in some worlds and expensive in the actual one; it wins
anyway because the rules lose more elsewhere.

**Cheap mitigations the proposal should adopt or at least measure.**
1. Measure the gap first: add a `cheat` policy to the lab (double-dummy on the true deal, 20 lines
   using `StateDD`) and report cheat − PIMC per trick. That bounds what any information model can
   recover and tells you whether the trick-1 budget is worth spending on samples at all.
2. αμ search (Cazenave & Ventos, "The αμ search algorithm for the game of bridge", 2020): PIMC
   that carries the vector of per-sample outcomes through the first M plies and only fuses at
   depth M. M = 2 removes fusion on the bot's card and the next reply at 2–4× the cost, and it sits
   naturally on top of the existing transposition-table solver.
3. Cheaper still, for leads only: aggregate with a risk-adjusted statistic (mean − λ·SE, or the
   lower quartile across samples) instead of the mean. Fusion over-rewards high-variance plans;
   this is a one-line change and the lab can price λ.
4. For the specific "draw trumps or not" decision: condition on the sub-samples where the master
   trump is with the opponents vs with the partner and compare the two conditional means; if they
   disagree on the trump lead, the decision is a guess and should go to the rule layer's prior.

### O7. The contract-aware award mis-scores capot and ignores melds during trick 1; StopAtTarget is not modelled

**Claim challenged.** §4.3 `award`: "capot takes everything"; §1 #5.

**Why it is wrong.** `contractAward` (`pimc.go` 356–376): when the caller's team is A and A capots,
it returns `totA − totB`; the engine (`scoring.go` 61–67) awards `allPoints = aTotal + bTotal` to the
capot team, so the award is short by 2·totB = 2·(B's declaration + Belote points) (B's hand points
are 0 by definition of capot). That is precisely the situation where capot is worth pursuing: it
steals the opponents' 20–200 of melds. The mirror case (caller B capots) has the same error; the
"non-caller capots" cases happen to be right because they fall through the "failed contract"
branch. The tie branch (`totA <= totB` ⇒ everything to the opponents) is correct and matches
`failedContract := contractingTotal <= opposingTotal`.

Two further gaps: (a) in Bitola `DeclarationPoints` are 0 until the start of trick 2, so for the
whole first trick the award ignores melds, including the bot's own declared ones, which it knows;
(b) with `StopAtTarget` on, `stopAtTargetIfReached` (`scoring.go` 126–159) banks running totals
the moment either team reaches the target, mid-trick, with **no contract settlement** — so near
the target the "tie loses everything" rule disappears and the objective becomes "reach the
target first", including declarations and Belote the instant they are scored. The proposal never
mentions `StopAtTarget` or match scores in play.

**Recommendation.** Fix the capot branch to `±(totA + totB)`. Carry the bot's own pending meld
value into the view at trick 1 (it is the bot's own information). Add a match-score mode to the
award: with `StopAtTarget`, value = P(reach target before the opponents); without it, the
"both cross ⇒ caller wins" rule (`determineMatchWinner`) must be in the award near the target.
Note the monotonicity argument in §3 below: within a sample the raw-points minimax is still exact
for this objective, so the fix is only at the root.

### O8. The discard signal (rule 10) is unmeasured and its King case will misfire with v1 bots and humans

**Claim challenged.** §4.1 rule 10 (the current text, which already corrected the direction: a
discard proves no trump, so it cannot mean "emptying to ruff"): a Ten or King discard of X on our
trick ⇒ weight 0.3 on samples where the partner lacks the Ace (resp. Ace or Ten) of X.

**Why it is weak.** The v2 bot itself only ever sends the Ten (§4.4). A v1 partner's `bestSmear`
throws the highest-point non-overtaking card that is not an unbacked boss — a lone King of X is
exactly what it discards, meaning nothing about the Ace. A human partner discards a King from
K-x because it is worthless. So in every table that is not two v2 bots, the King clause is a
prior with unknown accuracy, weighted at 0.3 with no measurement (the only measured soft inference,
`bidInfer`, came out negative). In a two-v2-bot partnership the Ten signal is not "soft" at all: v2
always sends it when it legally can inside the tie margin, so its absence is also information.

**Recommendation.** Restrict rule 10 to the Ten; measure it in a mixed table (v2 + v1 partner)
and a v2 + v2 table separately; make the weight depend on whether the partner seat is a v2 bot
(public knowledge at the table). Treat "partner did not throw a Ten when it could have" as the
other half of the same inference between two v2 bots.

### O9. Conventions: the tie-break mechanism cannot carry the convention, and one convention needs the owner's confirmation

**Claim challenged.** §4.4: conventions as a tie-break within 1–2 points; §3.2 "Cash the Ten before
the Ace: remove … as a signal it is sent in the wrong situation".

**Why it is weak.** See O1 for the margin: at 12 samples a 1–2 point margin is noise, so the
convention will be applied inconsistently, which defeats its purpose (a convention is only useful if
the partner can rely on it). The Ten-under-Ace itself is the standard Croatian/Serbian convention
("desetka ispod asa": discard the Ten to show the Ace) and the proposal describes it correctly,
including that it is only available when the discarder is void in trump. Two points for the owner:
(a) many Macedonian/Balkan tables also treat *leading* the Ten from A-T as "I hold the Ace" — the
current bot's T-before-A rule suggests someone at this table thought so; removing it for +0.48
bot-vs-bot points may conflict with what the human partner expects, so the owner should confirm the
table convention before §3.2 is applied; (b) the Ten-under-Ace applies on any trick the partner is
winning safely (a non-trump boss too), not only on trump leads; restricting it to trump leads
wastes the signal.

**Recommendation.** Apply conventions on SE-defined ties (O1), or as a fixed small bonus (2–3
points, measured) rather than a margin; ask the owner about the A-T lead convention; generalise the
Ten signal to any safe partner trick.

### O10. CPU budget: the schedule is about right per sample but the tail is not, and the design must be "anytime"

**Claim challenged.** §4.3 table: trick 1 "4–6 samples / ~1.5 s", trick 2 "10 samples / ~1 s"; the
think delay hides latency.

**What I measured** (`runs/pimctime_6deals_12s.txt`, 12 samples, loaded host): trick 1 **4.7 s
mean, 12.7 s max**; trick 2 0.82 s mean, 1.8 s max; trick 3 0.16 s. Per sample that is ~390 ms /
68 ms / 13 ms, i.e. `RootValues` with the shared memo costs about one `Value` call per sample, so
the per-sample figures in the table hold on a quiet core. But the max/mean ratio of 2.7 at trick 1
means a 6-sample trick-1 decision will sometimes take 5–6 s, far outside the 1.0–2.5 s think delay,
and this is per bot: four bots at a table serialise, several tables multiply. The current text
already moves the computation off the session lock, which was the worst problem.

**Recommendation.** Make PIMC an anytime algorithm by construction: a deadline per decision, samples
run until the deadline or until the margin criterion of O1 is met, and the best-so-far card is
played. Benchmark-test the deadline, not a fixed sample count. Keep trick 1 on the rule layer until
the solver optimisations exist (phase 3 already says this), and run the solver on a bounded worker
pool so N tables cannot starve the API.

### O11. Hard constraints 5 and 6 must be evaluated at the moment the card was played, and the Memory API does not carry that

**Claim challenged.** §4.1 rules 5–6 as worded ("the highest led card on the table", "the highest
trump on the table").

**Why it matters.** `legalCards` evaluates "highest led-suit card" and "led suit can still win" at
the moment the seat plays (`validation.go` 19–25, 43–66). A seat that plays K over the leader's Q is
legal even if a later seat plays the A; evaluated from the finished trick that seat would be
wrongly marked "no card above A". Likewise the "no trump cut yet" condition is about the cards
before that seat's play. The production `Memory.ObservePlay(seat, card, leadSuit)` receives only the
pre-play lead suit, not the trick so far, so the proposal's "free, exact" rules 5–6 need an API
change (pass the trick prefix), and the proposal should say "at the time of the play". Rules 3, 4,
7 and 8 are exactly implied as written (see §3).

### O12. Determinism and reproducibility claims need the Memory in the fixture

The decision is deterministic given the view, and the view is deterministic given the per-match
`Memory`, which lives only in process memory (`live_match.go` 274). That is fine (a restart abandons
the match through `ReconcileStaleRooms`, so there is no replay-from-persistence case), but the
lab-side determinism tests in §4.6 must build the Memory from a play history, not from a hand-written
view, or they will pass on views the production path can never produce. Also note the lab's `current`
policy is better informed than production (the lab Memory has `ObserveCandidate`; production does
not), so lab "current" numbers slightly overstate production v1.

### O13. Smaller points

- §2 "verified against brute force on 300 endgames": those are 2–4 card endgames, which barely
  exercise the transposition-table bound logic (`dd.go` 204–221, 291–309) that a full solve from
  trick 1 relies on. The bound code reads correct to me (fail-soft, bounds stored relative to the
  narrowed window, capot flag in the key), but add a memo-on vs memo-off cross-check from trick 3
  on a few hundred deals; it costs seconds.
- `tenSignal` "−0.02 ± 0.01 (Bitola) … the convention is free": that is a 2σ negative; say "≤ 0.02".
- §1 #3 lists "candidate card sits with the caller" as a free exact inference; it is, in both
  rounds (`handlePickTrump` appends it in round 2 too, as a side card), and the production Memory
  lacks it; the text should say both rounds.
- `bidWeight` for Bitola round 2 marks all four suits as refused, including the candidate suit that
  could not be called in round 2; harmless (it was refused in round 1) but sloppy.
- The lab's `contract` and `declInfer` ablations are still PENDING while the proposal lists #5 as a
  headline item. My own `pimc:12[contract] vs pimc:12[]` (60 deals, seed 3,
  `runs/pimc12_contract_vs_plain.txt`): **+1.80 ± 4.59**, i.e. unmeasured; a 2–3 point effect needs
  ≥ 400 deals even in the PIMC-vs-PIMC design.

## 3. What I tried to break and could not

- **Solver legality and scoring.** `DD.Legal` (`dd.go` 125–177) mirrors `legalCards` line for line:
  follow suit; must overplay while `led == trump || !trumpIn`; must cut when void; must over-trump
  the highest trump; no partner exemption. Trick winner, +10 last trick, ±100 capot replacing the
  bonus, all match `playing.go`/`scoring.go`. `ddcheck -n=200`: 0 mismatches.
- **Sampler.** 2750 samples over 55 decisions: 0 violations of hand sizes, disjointness, located
  cards, voids, or the unseen set; 0 fallbacks to the constructive fill. Same seed ⇒ same card, 55/55.
- **Harness fairness.** One public `Memory` shared by both policies; identical bidding (`pimc:S`
  falls back to `lab[]`, a verified copy of `current`); `CurrentBot` receives every view field;
  `current vs lab[]` = 0.00; pass-outs are symmetric within a pair when bidding is identical, so
  they only dilute, never bias. If anything the lab favours `current` (see O12).
- **Hard constraints 3, 4, 7, 8.** 3 and 4 follow from `legalCards` (a non-led non-trump card is
  legal only when both `suitCards` and `trumpCards` are empty; there is no partner exemption, so a
  discard on the partner's trick still proves trump-void). 7: the candidate joins the caller's hand
  in both rounds; the Belote prompt is masked for other seats (`projection.go` 19–20), so only the
  announcement is public and the inference is exactly as stated. 8: in Bitola `dedupOneCardOneGroup`
  keeps whole declarations, a kept run is maximal in the hand, and a dropped four-of-a-kind cannot
  create a false longer run because an adjacent card would have been inside the run; in Croatian
  overlap changes nothing about maximality; the union-based `runNeighbours` is therefore sound in
  both variants, including runs that contain a card already played in trick 1.
- **Contract award monotonicity.** The award is a non-decreasing step function of the raw remainder
  differential (made: linear; failed: constant), so maximising raw points inside the minimax and
  transforming at the root is exact per sample; the transform only matters across samples, which is
  the intended effect. The only error is the capot branch (O7).
- **Forced-dealer contamination of the Croatian fit.** Real (15.4% of rows) but refitting without
  them leaves the policy value at +3.94 vs +3.93.
- **Position semantics.** `pos = (seat − dealer − 1) mod 4`, so pos 1 is the dealer's partner as
  claimed; the sign pattern (+31 at pos 1, −8 at pos 2 whose pass forces the *opponent* dealer) is
  coherent with the mechanism described.
- **Regret arithmetic.** 43.3% + 25.2% = 68.5% of regret in leads; "81.7% optimal" and the per-trick
  numbers are internally consistent.
- **Discard signal direction.** The version I started from had it backwards (partner "probably holds
  a trump" after a discard that proves trump-void); the current text is consistent with hard rule 4.
- **The session-lock problem** is acknowledged in the current text (compute off the lock, apply on
  the timer).
- **The PIMC advantage reproduces.** `pimc:12 vs current`, Bitola, 120 deals: seed 1 **+8.96 ±
  3.58** (share 53.3%), seed 2 **+7.61 ± 3.10** (share 52.8%), against the documented +9.9 ± 3.1.
  Both PIMC sides made more of their contracts than `current` on the same deals (90% vs 87%, 82% vs
  72%). Only the deal set behind the documented row is in question (O5), not the effect.

## 4. Missing proposals a strong player would expect

1. **Match-score awareness, in bidding and in play.** `determineMatchWinner`: if both teams cross
   in the same hand the caller wins, so a defender near the target must *fail* the contract, not
   merely out-score; a caller near the target only needs to cross. With `StopAtTarget` the hand ends
   the instant a running total reaches the target (melds and Belote count immediately), which turns
   the objective into a race. Bidding: the "optional, not yet measured" threshold shift should be a
   first-class item with the obvious cells (opponents within one hand: call anything playable; we
   are within one hand: call only what makes it alone).
2. **Capot pursuit and defence as explicit objectives.** Fix O7, then add the rule-layer heuristic
   for tricks 1–2: as defender, take one trick early when the caller shows capot shape; as caller,
   value a +100-and-their-melds line correctly.
3. **An information-gap metric and a stability metric in the strength gate** (O1, O6): cheat − PIMC
   per trick and "decisions changed under re-seeding", alongside the duplicate advantage.
4. **Defender-specific leads in the rule layer** (if tricks 1–2 stay rule-based): the current
   `chooseLead` has no defender logic beyond "cash a boss / lead safe low"; a strong defender leads a
   short suit to set up a ruff, or the suit where the partner can hold the Ace, and leads trump
   through the caller only with J or 9. The lab can price each in an afternoon.
5. **Partner-type awareness.** Whether the partner is a v2 bot, a v1 bot or a human is public at the
   table and changes which conventions are reliable (O8) and whether the Ten signal is exact.
6. **Surrender proposals and difficulty-proportional think time.** PIMC yields P(make) and the
   EV gap for free; proposing surrender on hopeless hands and thinking longer on close decisions are
   both cheap and visible to humans.
7. **Belote as caller.** "Always announce" hands 20 points to the opponents when the contract is
   already lost; PIMC can gate it on P(make) (rare, but free once P(make) exists).
8. **A human-like bidder in the lab.** Rule 9's weights and the bidding formula are both calibrated
   against a table of current bots; add a looser bidder policy to test robustness before anything
   that models "how the others bid" ships.
