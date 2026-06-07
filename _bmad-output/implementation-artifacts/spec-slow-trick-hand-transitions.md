---
title: 'Pace trick/hand transitions so the collect sweep is seen before overlays'
type: 'bugfix'
created: '2026-06-06'
status: 'done'
context: []
baseline_commit: '2f5b93f'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** At the last trick of a hand and at the trick-1→2 boundary, the end-of-hand score reveal and the declaration/belot reveal panels mount the instant their WebSocket payloads arrive — before the trick-collect animation (the ~1s winner glow + the four cards sweeping to the trick winner) has run. The overlay covers or pre-empts the sweep, so players never see the final card played, who won the last trick, or where the cards went. Affects desktop and mobile.

**Approach:** Re-sequence presentation entirely client-side. Hold the score-reveal, declaration-reveal, and belot-reveal overlays until the in-flight trick-collect snapshot (`pendingResolvedTrick`) has cleared, reusing the existing collect lifecycle (glow → sweep → snapshot clear) as the gate. No server, timer, animation-constant, or event-contract changes.

## Boundaries & Constraints

**Always:**
- Keep the fix in `MatchPage.tsx`; use `pendingResolvedTrick` (set on `event:trick_resolved`, cleared by `handleFlightComplete` or the existing ~1.96 s fallback timer) as the single gate.
- Preserve the `pendingResolvedTrick` snapshot pattern and `winnerCollectRect` targeting — both load-bearing (prior spec `spec-fix-in-game-card-flight-animations`).
- Honor reduced-motion: that path clears the snapshot after `TRICK_RESOLVE_PAUSE` (1000 ms), so overlays still appear after a readable beat, never instantly. Behavior identical on desktop and mobile (timing/gating only).

**Ask First:**
- Adding any *extra* readability delay beyond the existing collect window (~1.56 s full-motion / ~1.0 s reduced). Default: introduce no new delay or constant.

**Never:**
- No server-side pause / `time.Sleep` between broadcasts — rejected: the next player's move timer is armed before broadcast, and reconnect snapshots would desync (per server investigation).
- No new motion constants and no edits to `motion.ts` values.
- Do not gate `trumpReveal` (fires during bidding; never races with a trick collect).
- Do not touch the rules engine, the WS event contract, or the dispatcher's state mutations.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Hand-end, full motion | `event:trick_resolved` then `event:hand_scored` back-to-back; snapshot set | 4th card lands → ~1 s winner glow → cards sweep to winner → only then score reveal mounts | Fallback clears snapshot at ~1.96 s → reveal still mounts |
| Trick 1→2 declaration reveal (Bitola) | `trick_resolved` + `declarations_resolved` + `match_state`; snapshot + `declarationReveal` set | trick-1 cards sweep to the winner first → then declaration panel mounts (8 s auto-dismiss starts on mount) | Fallback clear still mounts it |
| Reduced motion | snapshot cleared after 1000 ms, no flights | Overlay mounts after the 1 s glow beat | N/A |
| Capot / belot coincides with resolve | `scoreRevealData.capot` true, or `belotReveal` set, while snapshot set | Sweep shown first → then capot animation / belot reveal | Fallback clear |
| No collect in progress (defensive) | overlay payload set, `pendingResolvedTrick === null` | Overlay mounts immediately (unchanged from today) | N/A |

</frozen-after-approval>

## Code Map

- `client/src/features/match/MatchPage.tsx` -- change sites: score-reveal trigger effect (~L552), declaration-reveal render gate (~L1537), belot-reveal render gate (~L1548), and the sibling `match_result` transition effect (~L666). The collect effect (~L406) and `handleFlightComplete` (~L486) own `pendingResolvedTrick` — unchanged (gate source).
- `client/src/shared/stores/matchStore.ts` -- holds `pendingResolvedTrick`, `scoreRevealData`, `declarationReveal`, `belotReveal` (read-only here).
- `client/src/shared/hooks/useWsDispatch.ts` -- burst order `trick_resolved → declarations_resolved → hand_scored → match_state`; reference only.
- `client/src/features/match/MatchPage.test.tsx` -- store-driven test; extend with gating cases.
- `server/internal/match/live_match.go` -- `broadcastActionResult` builds `event:trick_resolved`; `winnerSeat` derivation fixed via `trickResolvedWinnerSeat` (review iteration 2).
- `server/internal/game/scoring.go` + `server/internal/game/state.go` -- `scoreHand` preserves the last-trick winner seat in `HandScore.LastTrickSeat` (server-only `json:"-"`).

## Tasks & Acceptance

**Execution:**
- [x] `client/src/features/match/MatchPage.tsx` -- In the score-reveal trigger effect (~L552), add `pendingResolvedTrick === null` to the guard and to the dependency array so the `overlayPhase` transition to `capot_animation`/`score_reveal` is deferred until the collect snapshot clears. -- Fixes bug 1 and makes the hand-end collect sweep visible (bug 2).
- [x] `client/src/features/match/MatchPage.tsx` -- Add `&& !pendingResolvedTrick` to the `declarationReveal` render gate (~L1537) and the `belotReveal` render gate (~L1548) so the trick collect is fully seen before either reveal panel mounts. -- Fixes bug 3 and keeps belot consistent.
- [x] `client/src/features/match/MatchPage.test.tsx` -- Add tests for the I/O matrix gating: reveal suppressed while `pendingResolvedTrick` is set and appears once it clears; reduced-motion still gates for the glow beat; defensive null-snapshot shows immediately.
- [x] `client/src/features/match/MatchPage.tsx` -- Gate the sibling `match_result` transition effect (~L666) on `scoreRevealData === null && pendingResolvedTrick === null` so the final-hand score reveal / capot animation is not skipped when `match_end` arrives during the collect. (Review iteration 1.)
- [x] `server/internal/game/{state.go,scoring.go}` + `server/internal/match/live_match.go` -- Fix wrong `event:trick_resolved` `winnerSeat` on a hand's last trick (the collect swept to the next hand's bidder). Preserve the winner seat in `HandScore.LastTrickSeat` and read it via `trickResolvedWinnerSeat`; add engine + broadcast-logic regression tests. (Review iteration 2 — user-reported.)
- [x] `server/internal/match/live_match.go` -- Emit `event:trick_resolved` (+ trick-1 `declarations_resolved`) in the Belot action branch when the Belot completed a deferred 4-card trick, so the collect animation runs (it previously got none). White-box regression tests for the completing-trick and mid-trick cases. (Review iteration 3 — user-requested.)

**Acceptance Criteria:**
- Given the last card of a hand's final trick is played, when `event:hand_scored` arrives, then the four cards glow on the winner and sweep to the winning seat, and the score-reveal dialog appears only after the collect completes (desktop and <768 px).
- Given trick 1 resolves in a Bitola hand with declarations, when the declaration reveal payload arrives, then trick-1's cards visibly sweep to the trick winner before the declaration panel appears.
- Given prefers-reduced-motion is on, when a hand ends, then the score reveal appears after the ~1 s winner-glow hold, never instantly.
- Given the collect `animationend` never fires, when the ~1.96 s fallback clears the snapshot, then the deferred overlay still appears (no permanent suppression / deadlock).
- Given the corrected server `winnerSeat` (review iteration 2), when any trick resolves — including the last trick of a hand — then the cards collect to the actual trick winner's seat (not the next hand's bidder).
- Given the final hand of a match ends, when `match_end` arrives while the collect sweep is still in flight, then the score reveal (or capot animation) is shown first and the match-result overlay appears only after the reveal is dismissed — never skipped; surrender/abandonment ends (no score reveal) still surface the result immediately.

## Spec Change Log

### Review iteration 1 (2026-06-06)
- **Finding (edge-case hunter, high):** Gating only the score-reveal effect on `pendingResolvedTrick` let the sibling `match_result` transition effect (`MatchPage.tsx` ~L666, guarded only by `overlayPhase === "normal"`) win the race on the final hand — `match_end` arrives during the collect while `overlayPhase` is still `normal`, so the final-hand score reveal (and capot animation) was skipped, snapping straight to the result mid-sweep.
- **Amended (patch, code-only — frozen intent unchanged):** Added `scoreRevealData === null && pendingResolvedTrick === null` to the `match_result` effect's guard and deps; the natural end-of-hand transition runs via `handleScoreRevealContinue` after the reveal is dismissed, while surrender/abandonment ends (no score reveal) still fire immediately. Added two regression tests (final-hand ordering — mutation-verified load-bearing — and capot deferral).
- **Known-bad state avoided:** final-hand score reveal / capot celebration silently skipped.
- **KEEP:** the `pendingResolvedTrick`-based client-side gating and the three original gates; do NOT switch to a server-side pause or new motion constants.
- **Rejected findings:** blind-hunter "vacuous tests" concerns (refuted by mutation testing and the defensive control test; the 162-point fixture total is correct for this variant) and a low-confidence pre-existing `myPlayerSeat === null` collect-effect clear gap (self-healing; not reachable for a seated player).

### Review iteration 2 (2026-06-06) — user-reported, supersedes the "targeting is correct" finding
- **Finding (user, high):** On a hand's last trick, points credited the right team but the collect animation swept to an opponent. The iteration-1 conclusion that bug 2's targeting was correct was **wrong**.
- **Root cause:** `event:trick_resolved`'s `winnerSeat` (live_match.go) read `newState.ActivePlayerSeat` as a fallback when `TrickWinnerSeat` was nil. On the last trick, the same `ApplyAction` chains `resolveTrick → scoreHand → startNewHand`, which clears `TrickWinnerSeat` and sets `ActivePlayerSeat = (newDealer+1)%4` (the next hand's first bidder) — so the broadcast sent that unrelated seat. Match-ending tricks were unaffected (no `startNewHand`).
- **Amended (server-side — renegotiates the frozen "Never: no server-side change" boundary at the user's explicit request):** Added `HandScore.LastTrickSeat` (server-only, `json:"-"`, populated in `scoreHand`) and `trickResolvedWinnerSeat(oldState, newState)` in `live_match.go` that reads it on a hand's last trick, removing the fragile `ActivePlayerSeat` fallback. Two regression tests added (engine: `LastTrickSeat` preserved; match: `trickResolvedWinnerSeat` 3-case table — both mutation-verified).
- **Known-bad state avoided:** last-trick collect sweeping to the wrong seat on every non-final hand.
- **KEEP:** `json:"-"` on `LastTrickSeat` — `match_state.lastHandResult` is a client `z.strictObject`; a serialized field would break validation.
- **Related, flagged then fixed (iteration 3):** a trick completed by a trump K/Q that triggers a Belot prompt resolves under the belot action, which emitted no `event:trick_resolved` → that trick got no collect animation. Fixed in iteration 3.

### Review iteration 3 (2026-06-06) — user-requested (the Belot follow-up flagged in iteration 2)
- **Finding:** When a trump K/Q is the 4th card of a trick, it triggers a Belot prompt and `handlePlayCard` defers trick resolution; the trick then resolves under the Belot announce/skip action, whose broadcast emitted only `belot_announced` + `match_state`. The client never received `event:trick_resolved`, so `pendingResolvedTrick` was never set → that trick had **no collect animation** (cards vanished instead of sweeping to the winner).
- **Amended:** The Belot branch now emits `event:trick_resolved` (winner via the same `trickResolvedWinnerSeat` helper) followed by `broadcastDeclarationsResolvedIfTransition` when it completed a 4-card trick (`len(oldState.CurrentTrick) == 4`), before `match_state`. A Belot can only fire on tricks 1-7 (no player holds both trump K and Q at trick 8), so no `hand_scored` follows.
- **Tests:** white-box `broadcastActionResult` tests — completing-trick emits `trick_resolved` (winner correct, ordered before `match_state`); mid-trick Belot emits none. Both mutation-verified.
- **Known-bad state avoided:** a Belot-completed trick silently skipping its collect animation.

### Review iteration 4 (2026-06-07) — user-reported (five follow-ups; all client-side)
- **Collect "blink" (recurring, now root-caused):** the four collect cards re-appeared at center for a beat after sweeping to the winner. `TrickArea`'s suppression set was derived from live flights, so each card un-suppressed the instant its own `animationend` fired — but the resolved-trick snapshot lingered until the *last* flight finished, and the four `animationend` events land in separate React batches, so already-landed cards re-painted statically at center until the snapshot cleared. **Fix:** an `isCollecting` flag (set when the flights are pushed, cleared when the snapshot is torn down) keeps *all* snapshot cards suppressed for the whole collect window — none can flash back. (Iteration-prior `receivedAt` keying addressed a *different* double-batch cause; this stagger gap was untouched.)
- **Score reveal could strand the table:** with only the server's 30 s backstop, a player who never clicked Continue held everyone on the score screen. **Fix:** an 8 s client auto-continue (matching the informational reveals' auto-close, confirmed 8 s) with a countdown ring around the Continue button; each client self-acknowledges, so the next hand deals once everyone is ready. `MOTION.SCORE_REVEAL_AUTO_CONTINUE = 8000`.
- **Belot reveal arrived after the sweep:** the belot reveal shared the declaration reveal's `!pendingResolvedTrick` gate, so when the K/Q was the trick-resolving card the "belote!" toast was deferred behind the collect. **Fix:** drop that gate for the belot reveal only — it now appears the instant the announcement lands, in parallel with the throw/collect (the declaration reveal keeps its deferral by design). 
- **`announce_belot` → "Невалидна акција" + stalled turn (race):** the local belot flow sent `play_card` and `announce_belot` back-to-back; the WS hub handles each inbound action in its own goroutine (`ws/hub.go` `go h.actionHandler`), so `announce_belot` could be applied before `play_card` had set `PendingBelotSeat` → engine rejected it (`ErrBelotNotAvailable`), and the client's old `belotHandledLocally` flag then suppressed the server prompt, stranding the seat. **Fix (client sequencing, no server change):** stash the choice and fire `announce`/`skip` only once the server confirms the deferred play (`pendingBelotSeat === my seat`), reusing the existing server-prompt path. Removes the ordering dependency entirely.
- **Declaration reveal closed on another player's move:** the dispatcher cleared declaration/belot reveals on any standalone `event:match_state`; the `revealJustEmitted` guard only protected the *immediate* trailing snapshot, so the next card-play snapshot wiped the reveal mid-countdown. **Fix:** the dispatcher no longer clears reveals on `match_state` — the reveal owns its 8 s countdown + X — and `MatchPage` clears a reveal only when an overlay covers the table (pause / disconnect / hand-end), which is the sole way a reveal is orphaned past its moment (D69 / D71 reconnect case included, since the disconnect overlay clears it before the resync snapshot).
- **KEEP:** the declaration reveal's collect-deferral gate (wave-1 intent) and the server-gated hand-complete pause (the 8 s client auto-continue is additive, not a replacement for the 30 s backstop).

### Review iteration 5 (2026-06-07) — user-reported follow-ups to iteration 4 (client-side)
- **Score-reveal loader sat inside the button:** the iteration-4 countdown was an inline ring left of the label (`<ring> Continue`). **Fix:** a `ButtonProgressBorder` SVG that traces the button's rounded-rect perimeter (`pathLength={1}` dash normalisation; viewBox measured 1:1 to the button so corners stay true) — the loader is now *around* the button, matching the ring-around-the-control pattern of the informational reveals.
- **Declaration reveal re-appeared every trick (regression surfaced by iteration 4):** removing the dispatcher's clear-on-`match_state` (iteration 4, issue 5) exposed a latent flaw — the declaration reveal is render-gated on the live `!pendingResolvedTrick`, so it unmounted on *every* trick's collect and remounted with a fresh 8 s `AutoCloseRing`. Across fast tricks (4 plays < 8 s) the countdown never reached 0, and since the store value now persisted, the dialog re-appeared every trick (until a trick happened to take > 8 s — the user's "I waited for 2 players and then it stopped"). **Fix:** a `declRevealReady` latch — the reveal is deferred behind the *first* collect only (set ready once the table is first clear of a sweep after it arrives), then stays mounted through later tricks' collects so its own countdown finishes exactly once. Now it closes only on its 8 s timer or the X, never on another player's action and never in a loop — which is precisely the stated requirement. Belot/trump reveals were already collect-independent (belot un-gated in iteration 4; trump never collect-gated), so only the declaration reveal needed it.
- **Test:** mutation-verified regression — the reveal must survive a *later* trick's collect (`MatchPage.test.tsx`, "stays up through a LATER trick's collect"); reverting the gate to `!pendingResolvedTrick` fails it.

### Review iteration 6 (2026-06-07) — user-reported (8th-trick collect blink; server-side)
- **Collect blink survived on the last trick only:** the iteration-4 `isCollecting` suppression fixed the blink for tricks 1-7, but the 8th (hand-ending) trick still flashed the four cards back at center after the sweep. **Root cause (server):** `resolveTrick` returns early on `TrickNumber == 8` (→ `PhaseHandScoring`) and SKIPS its "set up next trick" reset that clears `CurrentTrick`; `scoreHand` didn't clear it either (only `startNewHand` does, which runs later on continue). So the hand-complete / match-end state served to clients still carried the four last-trick cards — the authoritative `match_state` re-populated the client's `currentTrick`, and once the collect snapshot cleared, `TrickArea` fell back to that live trick and repainted the cards (and could even fire a spurious opponent-throw flight for the 4th card).
- **Fix:** `scoreHand` now clears `state.CurrentTrick`/`LeadSuit` after capturing the last-trick winner. `TrickWinnerSeat` is deliberately KEPT — the final-hand `event:trick_resolved` resolves its collect winner from it (clearing it would regress iteration 2). Applies to both exits (hand-complete and match-end), so the final hand is covered too.
- **Test:** mutation-verified — `TestHandComplete_HoldsBeforeDealingNextHand` now asserts `CurrentTrick`/`LeadSuit` are cleared in the hand-complete state; removing the clear fails it.

## Design Notes

`pendingResolvedTrick` is the precise "collect in progress" signal: set the moment a trick resolves, cleared when the last collect flight ends (or via the ~1.96 s fallback). Gating overlays on it re-sequences presentation with no new timers.

Bug 2 turned out to be **two** defects (the initial "targeting is correct" read was wrong — see change log iteration 2): (a) the collect sweep was hidden behind the instantly-mounted overlay (fixed by the gating above); AND (b) a **server** bug — on a hand's last trick the `event:trick_resolved` `winnerSeat` fell back to `ActivePlayerSeat`, which `startNewHand` had already advanced to the *next hand's first bidder*, so the sweep targeted the wrong seat even with the overlay gated. Scoring used the true winner independently, so points stayed correct. Fixed server-side by preserving the winner seat across the hand transition.

Example (score-reveal effect):

```tsx
useEffect(() => {
  if (scoreRevealData !== null && overlayPhase === "normal" && pendingResolvedTrick === null) {
    setOverlayPhase(scoreRevealData.capot ? "capot_animation" : "score_reveal");
  }
}, [scoreRevealData, overlayPhase, pendingResolvedTrick]);
```

## Verification

**Commands:**
- `cd client && npx vitest run src/features/match/MatchPage.test.tsx` -- expected: new gating tests pass.
- `make lint` -- expected: ESLint + Prettier clean (TS strict, no `any`).
- `make test` -- expected: full suite green (no regressions in existing MatchPage / reveal tests).

**Manual checks:**
- Play a full hand: at hand-end confirm the final card + winner glow + sweep are visible before the scoreboard; at trick 2 of a Bitola hand with declarations, confirm the trick-1 sweep plays before the declaration panel. Repeat at a <768 px viewport.

## Suggested Review Order

**Core gating mechanism**

- Entry point — the score reveal / capot animation is held until the collect snapshot clears.
  [`MatchPage.tsx:560`](../../client/src/features/match/MatchPage.tsx#L560)

- Review-caught: the sibling match-result transition gated so the final-hand reveal isn't skipped.
  [`MatchPage.tsx:677`](../../client/src/features/match/MatchPage.tsx#L677)

**Reveal render gates**

- Declaration reveal held until trick 1's cards sweep to the winner.
  [`MatchPage.tsx:1563`](../../client/src/features/match/MatchPage.tsx#L1563)

- Belot reveal on the same gate (a trump K/Q can be the trick-resolving card).
  [`MatchPage.tsx:1576`](../../client/src/features/match/MatchPage.tsx#L1576)

**Tests**

- Gating suite: deferral, reduced-motion glow beat, and defensive immediate-mount.
  [`MatchPage.test.tsx:704`](../../client/src/features/match/MatchPage.test.tsx#L704)

- Final-hand ordering regression (mutation-verified load-bearing).
  [`MatchPage.test.tsx:804`](../../client/src/features/match/MatchPage.test.tsx#L804)
