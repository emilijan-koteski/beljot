# Review of BOT_V2_PROPOSAL.md, round 2

Scope: only what is still wrong or missing in revision 2. Everything that survived round 1 and
did not change is not restated. New measurements are in
`/tmp/claude-1000/-mnt-data-Projects-personal-beljot/3858171b-642d-4aff-af15-4a9551b19784/scratchpad/review/`
(`tablecheck.py`, `runs/pimctime_8deals_3s.txt`, `runs/pimctime_8deals_6s.txt`). I did not touch
the five PENDING rows.

## Verdict

Revision 2 resolves O2, O3 (as far as the data allows), O4, O5, O8, O9, O10, O11, O12 and O13
faithfully, and the lab fixes I checked (`contractAward` capot branches, `cheat`, `riskLambda`,
`NoMemo`) are correct. Three things are still wrong rather than merely unmeasured: the `award`
specification follows `game-rules.md` for the both-cross case while the engine does the opposite,
the stop-at-target mode is not root-transformable so the "monotone, exact inside a sample"
argument no longer covers it, and one production path replaces the game state without updating
the bot Memory, which breaks the "same information state ⇒ same card" claim exactly where v2
would notice it. The stopping rule is a correct specification but, at the budget the proposal
sets, it cannot deliver the stability gate it was introduced for on tricks 1–4; the proposal
should say so instead of gating the conventions layer on it. Phase 1 is safe to ship after one
cheap measurement it currently borrows from a different policy.

## Still wrong

### W1. `award`: the both-cross rule is specified the opposite way from the engine (Q3)

§4.3: "near the target without [StopAtTarget], the 'both cross ⇒ the caller wins' rule applies."
That is `.claude/rules/game-rules.md` line 45. The engine does something else:
`determineMatchWinner` (`scoring.go` 248–262) gives the match to the **higher total** when both
cross, and only an exact tie of the two totals goes to the caller. Example: caller's team at 950,
opponents at 960; the hand scores 82 / 80 (contract made); totals 1032 / 1040; the engine awards
the match to the opponents, the proposal's `award` would value that line as a win. The owner's
constraint is "rules are the engine's", so the bot must implement `scoring.go`; the rules file
should be reconciled by the owner (it is an owner-confirmed document that disagrees with the
code), but that is outside this proposal. Resolution for O7 is therefore "accepted" in the log but
the accepted text is wrong.

### W2. Stop-at-target is path-dependent; the root transform is not exact for it (Q3)

`stopAtTargetIfReached` (`scoring.go` 126–159) runs after every trick resolution
(`resolveTrickWithDeclarations`, *before* `scoreHand`, including after trick 8), after the
declaration contest resolves, and after a Belote announcement (deferred while the trick-1 contest
is open). When it fires, both teams bank their **running totals** (hand points + melds + Belote,
no last-trick or capot bonus, no contract settlement) and the match ends by
`determineMatchWinner` on those totals: higher wins, tie to the caller. Consequences the
proposal's one sentence does not capture:

- "we reach the target first" is not the rule: both can cross at the same check point; then the
  higher running total wins, tie to the caller.
- A caller who is failing the contract but whose running total touches the target at a trick
  boundary **wins the match**; the contract rule never runs. Conversely, after trick 8, if a
  running total reaches the target the contract rule is skipped there too.
- The objective is therefore a function of the *path* (which trick boundary a total crosses at),
  not of the final remainder differential. §4.3's sentence "inside each sampled deal the
  raw-points minimax stays exact because `award` is a monotone step function of the remainder"
  is true for the normal settlement and false for this mode: grabbing 11 points now can be worth
  more than 30 points over the remaining tricks.

Fix: when `StopAtTarget` is on and either team's running total is within reach
(target − total ≤ remaining card points + unscored melds), solve the sample with a race objective:
the same search with an early exit at each trick boundary and a terminal value ±W (match won /
lost) plus the usual differential otherwise. It is the same solver with one extra check per
boundary and is cheaper, not dearer, because it cuts off. Until it exists, the proposal should say
that in that situation the rule layer plays (with "take points now" as the tie preference) and
that this is unmeasured, rather than listing stop-at-target under "specified exactly".

### W3. Own pending melds in Bitola trick 1 are not banked points (Q3)

§4.3 carries "the bot's own declared melds" into `award` during trick 1. They score only if the
bot's team wins the declaration contest, which depends on the opponents' and the partner's hidden
melds. The clean fix is exact and free: each sample is a full deal, so at trick 1 run
`detectDeclarations` and `resolveDeclarations` on the sampled hands and let each sample settle its
own contest (the trick-1 leader for the tiebreak is known). That also prices the opponents'
possible melds, which the current text ignores entirely.

### W4. A production path updates the game state without updating the bot Memory (Q4)

`handleDeclarationTimeout` (`live_match.go` 1164–1206) calls `ForceCloseDeclarationPhase`, sets
`session.gameState = newState` (1187) and `maybeScheduleBotAction` (1205) with **no
`observeBotMemory` call**. The other three state replacements (454→463, 1234→1240, 1423→1439)
observe. After a Croatian declaration window expires, the first bot decision of trick 1 runs with
a Memory that lacks the revealed declarations; the Memory catches up on the next applied action
(463 observes whenever `DeclarationsResolved`). Today the cost is one rule-layer decision without
`KnownCards`; under v2 the sampler would place a publicly revealed card in a random hand for that
decision, and the canonical-hash claim "same information state ⇒ same card" fails because the
Memory is not the information state. One-line fix, but it must be in the phase-1 list.

Two smaller determinism points: the Bitola round-2 reshuffle keeps `HandNumber`, so
`SyncHand` does not reset anything recorded during bidding (rule 9's who-passed-what, when it
comes); and the tie-break among *exactly* equal cards must be canonical (sort the legal set
before the argmax), because `Players[seat].Hand` order is deal- and `removeCard`-dependent and two
equal information states can present it differently.

### W5. The stopping rule cannot reach the stability gate on tricks 1–4 at the proposed budget (Q2)

The rule is specified correctly (paired SE on shared samples, per-trick minimum, deadline, best
so far at the deadline). What it buys is overstated. I measured decision stability at the
proposed per-trick minimum (3 samples at trick 1, 6 at trick 2; three independent seeds, 8 Bitola
deals, every decision with more than one legal card):

| trick | 3 samples: changed / n | 6 samples | 12 samples (round 1) | mean ms at 3 / 6 |
|---|---|---|---|---|
| 1 | **5 / 10** | **5 / 10** | 5 / 10 | 299 / 1059 (max 752 / 4897) |
| 2 | **5 / 9** | **5 / 9** | 4 / 9 | 71 / 143 |
| 3 | 5 / 8 | 2 / 8 | 2 / 8 | 16 / 27 |
| 4 | 4 / 8 | 4 / 8 | 3 / 8 | 4 / 7 |
| 5–7 | 3/8, 1/6, 3/6 | 2/8, 1/6, 3/6 | 2/8, 0/6, 2/6 | ≤ 2 |

At n = 3 the paired SE is an estimate from three differences (two degrees of freedom), so
"margin > 1 SE" is satisfied spuriously most of the time and the rule stops at the minimum; the
stability at tricks 1–2 stays at roughly one decision in two whether the budget is 3, 6 or 12
samples. Getting from ~50% to the ≤ 15% gate needs the SE roughly 3–4× smaller, i.e. 10–15×
more samples at trick 1, which is the solver optimisation work of phase 3, not a stopping-rule
property. Three consequences for the text:

1. §4.3/§4.6 should state that the gate is expected to pass from trick 5 on with the current
   solver and on tricks 1–4 only after the 5–10× solver work; otherwise the conventions layer,
   which is gated on it, silently never ships for the tricks where humans notice.
2. The metric must exclude cards that are **equivalent** (identical value in every sample, e.g.
   touching ranks in one hand, or two cards that both lose the same trick); my probe counts those
   as flips, so part of the numbers above is benign. The author should measure with the
   equivalence exclusion before fixing the threshold; the canonical tie-break makes equivalent
   cards deterministic anyway.
3. Specify the failure modes the rule as written has: (a) two cards with identical values give
   SE = 0 and the margin test "margin > SE" is never true, so the decision runs to the deadline;
   define "tie" as margin ≤ max(SE, ε) and stop. (b) Three-way near-ties: the tie set handed to
   the conventions layer must be every card within one SE of the best, not just the runner-up.
   (c) Weighted samples (rule 10 at 0.5, phase 4): SE must use the effective sample size
   (Σw)²/Σw²; the lab's `riskLambda` uses n = Σw, which under-estimates it once weights vary.
   (d) Deadline below the minimum: the trick-1 maximum was 4.9 s for 6 samples on a quiet host
   against a 1.0–2.5 s think delay; say whether the minimum is a floor (the delay stretches) or
   the deadline wins (then the decision can rest on 1–2 samples; the stability figure for that case
   is worse than the table above).

### W6. The hand-shape table has no stated inclusion criterion, and it is applied unevenly (Q1)

I re-derived the table on the lab rows (`tablecheck.py`): rows 1–8 score **+5.71** (Bitola, 11.3%)
and **+3.94** (Croatian, 9.2%; the document says +3.90, same within the cell definition). Row 8 is
supported by the data, not by the regression: J doubleton (no 9/A) with a side Ace in Croatian is
**+17.7 ± 3.1** at position 1 (n = 2404) against **−25.2 ± 3.0** at position 0 and **−12.8 ± 3.8**
at position 2; without row 8 the Croatian table drops to +3.78. Keep it, with one caveat in the
text: its value comes from position 2 passing ~90% of the time (a current-bot table), so it
shrinks against humans who call more there.

The problem is which marginal cells are in and which are out. Row 6 is in on a **+5.2 ± 3.1**
(exactly one side Ace, 1.7σ) or **+7.9 ± 3.0** (one or more, 2.6σ) cell, while these Bitola cells
are out:

| cell (Bitola, non-forced rows) | gain | σ | human-plausible? |
|---|---|---|---|
| J + A doubleton with a side Ace | **+13.1 ± 2.8** (n = 2717) | 4.7 | yes: two of the three top trumps plus an outside winner is a routine call |
| 9 + two others (no J/A) with a side Ace | **+7.3 ± 2.2** (n = 4705) | 3.3 | marginal |
| J doubleton (no 9/A) with a side Ace, position 2 | **+12.6 ± 2.4** (n = 3134) | 5.2 | the same shape row 8 accepts in Croatian |

Adding the first two lifts the table to **+5.81**, above the regression's +5.77. Either state the
criterion (for example: gain ≥ 2 SE and ≥ +5 per row, and a human-plausibility veto) and apply it
to every cell, or list these as owner's-choice rows. In Croatian the weakest included half-row is
row 3's "four with the Ace" at +10.0 ± 7.4 (1.4σ); under the same criterion it would be out.

Two smaller table points: the proposal quotes the one-or-more-side-Ace cells for rows 4 and 6
(+34 ± 10, +8 ± 3) while the lab table lists the exactly-one cells (+28.2 ± 10.6, +5.2 ± 3.1)
for the same rows without saying so; quote one cell. Bitola round 2 is unspecified when two or
three suits match a row (use the same ranking as the forced pick) and the −4 round-2 cost is not
in the table (acceptable, say so).

### W7. Rule 10's "exact in both directions between two v2 bots" is not true under the tie rule

§4.1 rule 10: between two v2 bots "the Ten not thrown when it could have been is also
information". The v2 sender throws the Ten only on a statistical tie of its own samples (§4.4),
which the receiver cannot reconstruct, so the absence of the Ten is weak evidence, not an exact
inference. Keep the positive direction at 0.5; drop or down-weight the negative one.

## Review-log faithfulness (Q5)

- O7: marked accepted; the accepted text is wrong on the both-cross rule (W1) and the
  stop-at-target sentence does not cover what the engine does (W2).
- O1: marked accepted; the specification is present, but the property it was accepted for
  (stability, as a gate for conventions) is not demonstrated and is not reachable on tricks 1–4
  at the stated budget (W5). The log's "strength is unaffected (the +10 holds at 3 samples)" is
  correct and is a different claim.
- O4: the proposal's §3.1 and row 4 say "+34 ± 10" (one-or-more side Aces); the lab class table
  says "+28.2 ± 10.6" (exactly one); same for row 6. Not a misstatement of the objection, but the
  two documents quote different cells for one row.
- O13: the lab results §2.3 still reads "the convention is free" next to −0.02 ± 0.01; the
  proposal was fixed, the lab file was not.
- O2, O3, O5, O6, O8, O9, O10, O11, O12: resolutions match what I argued and are present in the
  text. On O5 the "different seeds, not binary" explanation for the 50/80 pass-out rows is
  consistent with the seeds now listed (21, 22, 23); the from-trick-3 re-run after a binary change
  is acknowledged.

## §1 ordering and phase 1 (Q6)

The ordering is right: #1 and #2 carry significant, reproduced effects; #3 is small and
significant; #4 (+1.9 ± 1.8), #5 (+1.4 ± 1.8) and #6 are ≤ 1σ and should be labelled "not
resolved" in the "measured" column rather than listed as values.

Phase 1 is safe to ship on its own **after one measurement it does not yet have**. The
+3.8 / +6.0 quoted for it is the `ledSecure,aceFirst,bidEV` package: regression bidding, not the
table, and without Memory rules 4–8. The table is only evaluated in-sample per row; rule 4
(void in trump after a side-suit discard) changes the rule layer's behaviour through
`opponentMayHoldTrump`, `partnerVoidInTrump`, `leadIntoPartnerVoid`, `bossWorthGuarding` and
`retainLastTrickWinner`, almost certainly for the better but unmeasured; rules 5–6 are dead
weight in phase 1 unless `threats()` becomes per-seat, which is new code. A `bidTable` flag and a
`memory48` flag in the lab and one 4000-deal duplicate run per variant (rules only: minutes) turn
the borrowed number into the phase-1 number. Add W4's one-line Memory fix to the phase-1 list.
`ledSecure` itself reads correctly (`cheapestLedSecureWinner`: cheapest led-suit winner that no
unseen or opponent-held led-suit card outranks; conservative about the partner's unseen cards;
ruffs rightly ignored) and its +0.5 / +1.3 is consistent with what the rule does.

## Still missing (new in round 2)

1. A per-sample declaration contest at Bitola trick 1 (W3): exact, free, and it prices the
   opponents' hidden melds, which nothing in the proposal does today.
2. The race solver for stop-at-target (W2), or an explicit "rule layer plays here" fallback.
3. The stability metric with equivalence exclusion, and the statement of which tricks the gate
   is expected to pass with the current solver (W5).
4. The Memory observation in `handleDeclarationTimeout` and a reshuffle hook for bidding
   bookkeeping (W4).
