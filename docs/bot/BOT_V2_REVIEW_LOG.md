# Beljot bot v2: review log

Round 1: an independent reviewer (a separate agent with the code, the lab binary and the two
documents, instructed to be adversarial) produced thirteen objections and eight missing items.
Each is listed with its resolution. "Accepted" means the proposal changed; "rebutted" means it
did not, with the reason. Round 2 is appended at the end.

## Round 1

| # | Objection (short) | Resolution |
|---|---|---|
| O1 | At a fixed 12 samples, half of the trick-1 and 4/9 trick-2 decisions change with the random seed (reviewer's measurement, 6 deals, 3 seeds). A "tie within 1–2 points" for conventions is therefore noise, "table-testable" holds only for unambiguous fixtures, and a view-hash seed makes equal information states hash differently if an irrelevant field differs. | **Accepted, with the limit stated.** §4.3 specifies anytime sampling with a paired-SE stopping rule and a per-trick minimum; the seed is a canonical hash of the information state; a stability report is in §4.6. Round 2 showed the rule cannot deliver ≤ 15% stability on tricks 1–4 at the affordable budget (W5), so the proposal says so, the conventions layer is a fixed bonus rather than a tie-break, and early-trick consistency is tied to the solver-speed work. Strength is unaffected. |
| O2 | The bidding-consistency weight 0.2 is wrong by 3–4×: a pass was made on 5 of 8 cards (Bitola) or 6 of 8 (Croatian), so a final hand showing the "inconsistent" pattern often did not show it at bid time (J+9 both visible 0.357 / 0.536; all four of a suit 0.071 / 0.214; J with two others 0.179 / 0.357). That, not a bidder-model mismatch, explains the measured −1.9. | **Accepted.** Factors recomputed and confirmed. Rule 9 is deferred to phase 4; when used, `w = 1 − P(visible)·(1 − ε)` with a table-matching bidder model; all soft weights ≥ 0.5 until a mixed-table measurement exists. |
| O3 | The regression encodes the current bot's play defects (Croatian trump Ace +14 vs trump Seven +26); a six-line hand table scores the same (+5.69 vs +5.77 Bitola); its weakest calls (J+Q doubleton at Croatian position 1) are absurd and unsupported; forced-dealer rows (15%) are in the Croatian fit; the threshold table is in-sample. | **Accepted.** Bidding ships as a hand-shape table (§4.3) evaluated on the lab rows at +5.71 / +3.90 vs the formula's +5.77 / +3.93; rows that would look absurd are explicitly excluded; the regression is a lab cross-check and its coefficients are labelled as statistics of v1's play; recalibration under v2 play is a phase-2 step. |
| O4 | Several §3.1 verdicts rest on cells with n≈50 (SE≈13): "four small trumps −4" is −4.2 ± 5.4 (a coin flip, with a side Ace +34 ± 10); "9+A+side Ace: remove" is +7.9 ± 3.0 in Bitola (keep) and −19.9 ± 4.7 in Croatian (remove); the specific-hand list cannot overturn rules. | **Accepted.** §3.1 is per variant with SEs; the specific-hand list is dropped from the proposal and kept in the lab results with its caveat; after round 2 the table quotes the exactly-one-side-Ace cells, matching the lab table. |
| O5 | Schedule comparisons are under-powered: the reviewer's `pimc:12:3` gave +5.9 ± 3.0 vs the proposal's +1.6 ± 1.9; rows were run with different seeds and binaries; "+10 with 12 samples, +15 with 30" is not like-for-like (different deal sets). | **Accepted in part.** The pass-out discrepancy the reviewer flagged is explained by different `-seed` values per row (now recorded in the lab results), not by a binary change; the from-trick-3 row was indeed re-run after a binary fix and is marked. The reading "almost nothing from trick 3" is withdrawn: the lab now says tricks 1–2 carry the larger share and a direct A/B (`pimc:12` vs `pimc:12:3`, 300 deals, both sides PIMC) is reported in the lab results. The PIMC-vs-current effect reproduces across seeds (+7.6, +9.0, +9.9). |
| O6 | Strategy fusion is not discussed; PIMC's trick-1 regret (8.4 vs 5.8) and lead regret when the opponents called (12.0 vs 8.9) are its signature. Mitigations: measure cheat − PIMC; αμ search; risk-adjusted aggregation; conditional split for the trump-draw decision. | **Accepted.** §4.3 has a strategy-fusion subsection with all four; the lab has a `cheat` policy and a `riskNN` aggregation flag; `cheat − PIMC` and `risk10` are reported in the lab results. |
| O7 | `contractAward` under-scores a caller capot (returns totA − totB instead of totA + totB); melds are ignored during Bitola trick 1; `StopAtTarget` and match scores are not modelled. | **Accepted; the first resolution was itself wrong** (round 2, W1–W3). Lab bug fixed and the `contract` ablation re-run (−1.1 ± 1.7, not resolved). §4.3 `award` now follows `scoring.go`: capot = everything; a per-sample declaration contest at Bitola trick 1 instead of counting the bot's own melds; both-cross = higher total, tie to the caller (the rules file says otherwise; flagged to the owner); stop-at-target as a race objective solved inside the search, with the rule layer playing those situations until it exists. The monotonicity argument is kept for the normal settlement only. |
| O8 | The King clause of the discard signal will misfire (v1 and humans discard a lone King for nothing); between two v2 bots the Ten signal is exact, not soft; partner type is public. | **Accepted.** Rule 10 is Ten-only, weight 0.5, partner-type aware; "Ten not thrown when it could have been" is noted as the other half of the inference between v2 bots. |
| O9 | The 1–2 point tie margin cannot carry a convention; the Ten-under-Ace applies on any safe partner trick, not only trump leads; many tables also read a *led* Ten from A+T as "I hold the Ace", so removing that lead needs the owner's confirmation. | **Accepted** for the mechanism (SE ties or a fixed bonus, §4.4) and the generalisation. **Answered** for the lead: the owner stated that leading the Ten from A+T is wrong at this table and that the Ten-under-Ace discard is the convention; §3.2 records this as the owner's call. |
| O10 | Trick-1 tail latency at 12 samples is 2.7× the mean (12.7 s max on a loaded host); four bots serialise; the design must be anytime with a deadline and a bounded worker pool. | **Accepted.** §4.3 CPU paragraph and §4.6 deadline benchmark. |
| O11 | Hard rules 5–6 must be evaluated with the trick as it was when the seat played; `Memory.ObservePlay` only receives the pre-play lead suit. | **Accepted.** Wording and API note in §3.5 and §4.1. |
| O12 | Determinism tests must build the Memory from a play history; the lab's `current` policy is slightly better informed than production (it receives the candidate card). | **Accepted.** §4.6; the lab results note the `current` caveat. |
| O13 | Smaller: add a memo-on/off solver check from mid-hand; "−0.02 ± 0.01" is a 2σ negative, say ≤ 0.02; candidate inference holds in both rounds; `bidWeight` marks the candidate suit as refused in round 2 (harmless); the `contract`/`declInfer` ablations were pending. | **Accepted.** Memo cross-check added (150 positions, 0 mismatches); wording fixed; both rounds stated; ablations reported (+1.4 ± 1.8 before the capot fix, +1.9 ± 1.8). |

Missing proposals raised by the reviewer: match-score awareness (bidding and play, both-cross
rule, stop-at-target race), capot pursuit/defence, information-gap and stability metrics in the
gate, defender-specific leads in the rule layer, partner-type awareness, surrender proposals and
difficulty-proportional think time, Belote gating as caller, a human-like bidder in the lab. All
are in proposal §6 (unmeasured) or §4.6 (gates).

What the reviewer tried to break and could not (kept as the proposal's verified core): solver
legality and scoring line for line against `validation.go`/`scoring.go`; sampler constraints
(2 750 samples, 0 violations) and determinism (55/55); harness fairness (shared public memory,
identical bidding, symmetric pass-outs); hard constraints 3, 4, 7, 8 as implied by the rules in
both variants; the award's monotonicity; position semantics (position 1 = dealer's partner);
regret arithmetic; the corrected direction of the discard inference; the PIMC advantage itself
(+7.6 ± 3.1 and +9.0 ± 3.6 on independent seeds).

## Round 2

The reviewer re-read revision 2, checked every round-1 resolution for faithfulness, re-derived
the bidding table on the lab rows (+5.71 / +3.94, confirming it), measured decision stability at
the proposed minimum budget, and found seven remaining problems. All are accepted; three were
real errors in revision 2.

| # | Objection (short) | Resolution |
|---|---|---|
| W1 | `award` stated "both cross ⇒ the caller wins" (from `game-rules.md`); the engine's `determineMatchWinner` gives the match to the higher total and only a tie to the caller. | **Accepted; error.** §4.3 follows the engine; the discrepancy between the rules file and the engine/client is flagged as an owner question (§5). |
| W2 | Stop-at-target is path-dependent (the hand ends at the first boundary where a running total reaches the target, no contract settlement, higher running total wins), so a root transform is not exact; "we reach the target first" is not the rule either. | **Accepted; error.** §4.3 specifies a race objective solved inside the search (target check at each trick boundary, terminal ±W) when a team is within reach, and the rule layer with "take points now" until that exists; the monotonicity claim is limited to the normal settlement. |
| W3 | The bot's own pending melds in Bitola trick 1 are not banked points; they depend on the contest with hidden melds. | **Accepted; error.** Each sample settles its own declaration contest with the engine's rules, which also prices the opponents' hidden melds. |
| W4 | `handleDeclarationTimeout` replaces the game state without `observeBotMemory`, so after a Croatian declaration window expires the first trick-1 decision runs without the revealed declarations; the canonical-hash claim fails there. Also: the round-2 reshuffle keeps the hand number (bidding bookkeeping must reset), and equal-value ties must be broken on a canonically ordered legal set. | **Accepted.** Verified in `live_match.go`; all three are in §3.5 / §4.2 / §4.6 and the phase-1 list. |
| W5 | At the proposed minimum (3 / 6 samples) about half of the trick-1/2 decisions and a third of trick-3/4 decisions still change with the seed; the stopping rule as a gate for conventions cannot pass on tricks 1–4 until the solver is 10–15× faster; failure modes (SE = 0 ties, three-way ties, effective sample size with weights, deadline below the minimum) were unspecified. | **Accepted.** §4.3 states what the rule does and does not buy, lists the four cases, makes the per-trick minimum a floor with a capped think delay, and moves conventions to a fixed bonus; §4.6 expects the ≤ 15% target from trick 5 with the current solver and excludes equivalent cards from the metric. |
| W6 | The table had no inclusion criterion and was applied unevenly: row 6 was in at 1.7σ while J+A doubleton with a side Ace (+13.1 ± 2.8), 9 with two small and a side Ace (+7.3 ± 2.2) and the Bitola third-seat J doubleton with a side Ace (+12.6 ± 2.4) were out; Croatian "four with the Ace" is 1.4σ; rows 4 and 6 quoted different cells than the lab table; Bitola round 2 ranking unspecified. | **Accepted.** §4.3 states the criterion (≥ 2 SE, ≥ +5, human-plausible), adds the three rows, marks the two borderline cells as the owner's choice, quotes the exactly-one cells, and specifies the ranking among matching suits. The table was then measured as a policy: +4.0 ± 0.7 / +3.9 ± 0.7. |
| W7 | "Between two v2 bots the Ten signal is exact in both directions" is false: the sender only throws it when its search allows, which the receiver cannot reconstruct. | **Accepted.** The absence of the Ten is not used as evidence. |

Round-2 verdicts on the questions asked: the §1 ordering stands, with #4–#6 relabelled "not
resolved"; phase 1 is safe to ship once measured as its own policy, which was then done
(`ledSecure,aceFirst,bidTable,memory4`: +3.9 ± 0.7 / +5.2 ± 0.7) with the memory fix of W4 added
to its list.

Measurements the review asked for, run after round 2: a perfect-information `cheat` policy
beats the current bot by +28.5 ± 2.3 and PIMC by +30.9 ± 3.4 per deal (duplicate advantages
are not transitive across opponents, so no share of the gap is inferred from these); risk-adjusted aggregation +2.3 ± 2.0 (not resolved); the capot-correct contract award
−1.1 ± 1.7 (not resolved; the PIMC-vs-PIMC design under-measures it). The direct A/B `pimc:12`
vs `pimc:12:3` gave +0.5 ± 2.2, and the confirmation runs that followed gave `pimc:12:3` vs
current **+4.4 ± 1.6** (400 deals), `pimc:12:2` vs current +3.9 ± 1.9, and the 3/6-sample
opening schedule against rules-on-tricks-1–2 **−2.8 ± 2.8**. Pooled reading (corrected in round 3): the search from trick 3 is worth +3.6 ± 1.1, the
opening search at 12 samples +2.9 ± 1.6 with the two direct A/Bs as primary evidence; the
affordable phase-2 step is the search from trick 3 with the opening search deferred to the
faster solver (proposal §4.3 and §4.7).

## Round 3 (final)

The reviewer verified W1–W7 against revision 3 and the corrected early-trick text, and signed
off conditionally on three edits, all made:

| # | Remaining issue | Resolution |
|---|---|---|
| R1 | W3 was incomplete: in Bitola the public `event:player_declared` says which seats declared during trick 1, and the engine rejects a declare from a meldless hand, so "seat X declared" is an exact constraint on X's sampled hand (and "did not declare" is exact for bot seats); after the reveal, `contested` bounds the losing team's best meld. Without it the per-sample contest under-prices the opponents' melds exactly when they are known to exist. | **Accepted.** Hard constraint 9 in §4.1 and a row in §3.5. |
| R2 | The early-trick value was overstated: pooled by inverse variance, the search from trick 3 is +3.6 ± 1.1 ("+3 to +5", not "+4 to +6"), the opening search at 12 samples is +2.9 ± 1.6 (a third of the gain with zero inside the interval, not "half"), and the direct A/B carries half the weight, not "the noisy end". "PIMC recovers a third of the gap" is not a valid reading of two non-transitive duplicate rows. | **Accepted.** §1, §4.3, §4.7 and lab §2.2 carry the pooled table and the corrected wording; the gap sentence is dropped. |
| R3 | The phase-2 package as it ships (table + rule fixes on tricks 1–2 + search from trick 3) was unmeasured as one policy. | **Accepted; measured.** `pimc:12:3[ledSecure,aceFirst,bidTable,memory4]` vs current, 400 deals: **+8.0 ± 2.5** (Bitola), **+11.2 ± 2.9** (Croatian). |
| R4 | Smaller: §6 item 1 still quoted the rules-file both-cross wording; §3.1 cited the regression's "+31" after §4.3 said the coefficients are not to be read; row 10 wording; state which owner's-choice rows the measured table includes; tricks 3–4 remain seed-sensitive at the phase-2 budget; the A/B's from-trick-3 side used the rules without the §3 fixes. | **Accepted.** All six edited. |

Verdict (reviewer): ready to hand to the owner as a decision basis; the verified core (solver,
sampler, harness, hard constraints, the engine-exact award), the bidding table with a stated
criterion and a measured duplicate result, phase 1 and phase 2 each with their own measured
number, and the CPU-driven phase-2 choice are supported by the evidence.
