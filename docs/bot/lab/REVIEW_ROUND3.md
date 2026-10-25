# Review of BOT_V2_PROPOSAL.md, round 3 (final)

Scope: W1–W7 verification, the corrected early-trick reading (after the three confirmation runs),
consistency of §1/§4.7 with the evidence, sign-off. Only what is wrong or inconsistent is listed.

## 1. W1–W7

| | Present | Correct | Remaining issue |
|---|---|---|---|
| W1 both-cross rule | yes (§4.3, §5 item 4) | yes; the client rules page (`client/src/features/rules/content/en.ts` 297) says "more total points … exactly equal, the team that took trump", same as `determineMatchWinner`; only the rules file differs | **§6 item 1 still says "the 'both cross ⇒ caller wins' rule"**: stale, contradicts §4.3. §3.1 still cites the regression's "+31" for position 1 as evidence after §4.3 says the coefficients are not to be read; quote the +16 ± 3 cell there. |
| W2 stop-at-target race | yes | yes (first boundary, running totals, no settlement, higher wins, tie to caller; solved inside; rule layer meanwhile, flagged unmeasured) | none |
| W3 per-sample declaration contest | yes | correct as far as it goes | **Incomplete.** `event:player_declared` (`ws/events.go` 109–111) tells every seat *which seats declared* during Bitola trick 1, and the engine rejects a declare from a meldless hand (`handleDeclare` → `ErrDeclarationNotAvailable`), so "seat X declared" is an exact constraint: X's sampled hand must contain a declarable combination. Without it the per-sample contest under-prices the opponents' melds exactly when they are known to exist. After the reveal, `DeclarationsResolvedPayload.Contested` bounds the losing team's best meld below the winner's. Both are free; the first is needed for W3 to be right. For bot seats "skipped" is also exact (bots always declare); for humans it is not. |
| W4 Memory on every state change | yes (§3.5, §4.2, §4.6, §4.7) | yes | none |
| W5 stopping rule | yes (four cases, floor with capped delay, fixed bonus, trick-5 expectation, equivalence exclusion) | yes | §4.3 "no early-trick stability problem (the rules are deterministic)" holds for tricks 1–2 only; phase 2 still searches tricks 3–4 at 16/30 samples, where my probes showed 2–5 of 8 decisions changing with the seed at 3–12 samples ("a quarter to a half", not "a third"), unmeasured at the phase-2 budget. Say that tricks 3–4 remain seed-sensitive in phase 2 and are covered by the stability report. |
| W6 table criterion | yes | applied consistently: every lab cell at ≥ 2 SE and ≥ +5 is a row or marked; rows 4, 5, 8, 10, 11 match the cells I computed | Row 10 "9 with two small trumps" vs the cell (9 plus any two non-J/A trumps): say "two other trumps". State whether the measured `bidTable` policy (+4.0 / +3.9) includes the two owner's-choice rows, since the shipped table may differ. The `pimc:12:3` side of the A/B played tricks 1–2 with `lab[]` (rules without the §3 fixes); phase 2 ships the rules with them: conservative, say so. |
| W7 absence of the Ten | yes | yes | none |

## 2. The corrected early-trick reading: is the A/B under-weighted?

Yes. Pooling the Bitola rows by inverse variance:

| quantity | rows | pooled |
|---|---|---|
| search on every trick, 12 samples, vs current | +9.9 ± 3.1, +9.0 ± 3.6, +7.6 ± 3.1 | **+8.8 ± 1.9** |
| search from trick 3, rules on 1–2, vs current | +1.6 ± 1.9 (20 samples), +5.9 ± 3.0, +4.4 ± 1.6 | **+3.6 ± 1.1** |
| search from trick 2, vs current | +6.4 ± 2.5, +3.9 ± 1.9 | +4.8 ± 1.5 |
| value of searching tricks 1–2 at 12 samples, **indirect** (first minus second) | | **+5.2 ± 2.2** |
| the same, **direct A/B** `pimc:12` vs `pimc:12:3` | | **+0.5 ± 2.2** |
| the two pooled | | **+2.9 ± 1.6** |

The direct A/B has the same standard error as the whole set of vs-current rows combined on this
question, so by precision it carries half the weight, not "the noisy end". The two estimates
differ by 4.7 ± 3.1, i.e. 1.5σ, not 2σ: "sits 2σ below the +4–5 those rows imply" treats the
indirect +5.2 ± 2.2 as if it were exact. Three statements in the documents therefore overstate:

- "the search from trick 3 … is worth about +4 to +6" (§1 row 1, §4.3, §4.7, lab §2.2, log):
  the pooled value is +3.6 ± 1.1, so **+3 to +5**; the "+4 to +6" is reached only by discounting
  the +1.6 ± 1.9 row, and "with 20 samples" cannot excuse it (more samples cannot be worth less in
  expectation).
- "the opening carries roughly half of the search's value" (§4.3) and "the other half of the
  gain (+8 to +10 total)" (§4.7 phase 3): pooled, the opening search at 12 samples is worth
  **+3 ± 1.6, between nothing and about +5**, i.e. a third of the gain with "zero" inside the
  interval. The phase-3 expected payoff should be stated as that, to be measured.
- The 3/6-sample opening "worth nothing" (−2.8 ± 2.8 direct) is fair, but it is in the same
  tension with the vs-current rows: `pimcs:3,6,16,…` vs current (+10.1 ± 2.9) minus from-trick-3
  (+3.6 ± 1.1) implies the 3/6 opening is worth +6.5 ± 3.1, against the direct −2.8 ± 2.8
  (2.2σ apart). Both direct A/Bs say the opening search is worth little against an opponent who
  also searches; the small 120–150-deal vs-current rows say it is worth +5–6 against `current`.
  Two readings fit: a couple of high draws among ~8 small rows (expected), or a real effect that
  PIMC's speculative opening lines gain mostly by exploiting `current`'s weak early defence,
  which a searching opponent does not reward. Either way the lab results should present the
  direct A/Bs as the primary evidence on the opening, not as outliers.

The decision is unchanged by any of this: phase 2 = rules on tricks 1–2 + search from trick 3 is
right at the current solver cost, worth +3 to +5 in Bitola (Croatian from trick 3 still
unmeasured, stated correctly in §4.7). What changes is the promise attached to phase 3.

## 3. §1 / §4.7 against the evidence

- §1 row 1, §4.3 and §4.7 phase 2: "+4 to +6" → "+3 to +5" (pooled +3.6 ± 1.1); §4.7 phase 3:
  "roughly the other half (+8 to +10 total)" → "+0 to +5 at 12 samples, measured before it
  ships". Lab §2.2 "sits 2σ below … treated as the noisy end" → report the pooled +2.9 ± 1.6
  and keep both A/Bs as primary.
- §2.2 and the log: "PIMC recovers about a third of the gap" is contradicted by `cheat vs pimc:12`
  +30.9 ± 3.4 ≥ `cheat vs current` +28.5 ± 2.3; duplicate advantages are not transitive across
  opponents. Drop the sentence; the per-trick regret gap in §2.7 is the only meaningful
  information-gap measure.
- The phase-2 package as it ships (table + rule fixes on tricks 1–2 + search from trick 3) is
  still unmeasured as one policy in either variant; one run per variant against `current` is
  minutes (the search from trick 3 is cheap) and would replace the inferred figure.
- §6 item 1 (stale both-cross text) and §3.1 (+31) per §1 above.

## 4. Verdict

Ready to hand to the owner as a decision basis once the edits above are made; none is
structural. The verified core (solver, sampler, harness, hard constraints, the engine-exact award
including the three cases revision 2 got wrong), the bidding table with a stated criterion and a
measured duplicate result, phase 1 with its own measured number (+3.9 ± 0.7 / +5.2 ± 0.7), and
the CPU-driven phase-2 choice are all supported by the evidence. Conditions for sign-off: (1) add
the "seat X declared" constraint to §4.1, without which W3 is mis-specified in Bitola trick 1;
(2) replace "+4 to +6" with "+3 to +5" and "half" with "a third, zero inside the interval"
wherever the opening search is valued, and present the two direct A/Bs as primary evidence; (3)
measure the phase-2 package as it ships, or mark its figure as inferred, before the owner reads
it as that package's value.
