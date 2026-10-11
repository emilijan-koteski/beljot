---
title: 'Croatian simultaneous declaration phase'
type: 'bugfix'
created: '2026-08-23'
status: 'done'
review_loop_iteration: 0
context: []
baseline_commit: '39346a9d89ac9ce9c1dfdb0048b4a3c969cf9b61'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The Croatian declaration phase (Story 12.6) prompts seats **one at a time**, and `advanceDeclarationPhase` steps past every seat holding no meld. So `activePlayerSeat` during `phase: "declaring"` names exactly the seats that hold melds, and `DeclarationWaiting` renders it verbatim — "Waiting for X to declare or skip". The table learns X has a declaration **before** X decides, and skipping no longer hides anything. `event:player_declared` leaks the rest in real time. A seat is also lime-flagged as "on the clock" in a phase that is not a turn.

**Approach:** Make the phase **simultaneous and fixed-length**. All four seats are asked at once, each answers independently, and the phase closes on the earlier of *all four answered* or an 8-second window — the score-reveal acknowledgement pattern (`HandCompleteReady` / `allConnectedReady` / `handCompleteAutoContinue`) transplanted onto `PhaseDeclaring`. Seats holding no meld get the **same** dialog with an empty state and a disabled Declare, so every screen has an identical footprint. No seat is active, no per-move turn timer runs, and the reveal fires at trick 1 as it does today.

## Boundaries & Constraints

**Always:**
- Nothing rendered before the reveal may correlate with *who holds melds*. `declarationAnswered` is safe precisely because **all four** seats answer regardless of melds; `activePlayerSeat` is safe only because it is pinned to the positional trick-1 leader for the whole phase.
- Bitola's trick-1 path (`DeclarationTimingDuringFirstTrick`, `AwaitingDeclaration` + `ActivePlayerSeat` gating, the live `event:player_declared` banner, the trick-2 reveal) is a **regression surface, not a refactor target** — every pre-existing Bitola test passes unchanged.
- No engine file compares `state.Variant` (D-VAR-1). The phase model is selected by `Rules.DeclarationTiming`, already resolved onto state.
- `resolveDeclarationsForHand` is the single resolution path for both variants and is not touched; answer *order* must not affect the outcome (tie-break rule 5 is positional, keyed on `trickLeaderSeat`).
- A disconnected seat must not hold the table: gate the close on **connected** seats, mirroring `allConnectedReady`.
- The client never re-derives a rule — it drives the dialog off `phase` and server-sent per-seat flags only.

**Ask First:**
- Any change to `resolveDeclarations` ordering/tie-break semantics.
- Widening the wire with anything beyond the single `declarationAnswered` boolean.
- Changing the 8 s player window (`MOTION`) or the server ceiling once chosen.

**Never:**
- Do not keep `DeclarationWaiting` alive in any form — a per-seat "who is deciding" surface is the defect.
- Do not emit `event:player_declared` from inside the dedicated phase.
- Do not run the per-move turn timer, `setTurnExpiry`, or `startTimerLocked` during `PhaseDeclaring`; `turnExpiresAt` stays `null` for its duration.
- Do not touch the declaration reveal (`DeclarationReveal`, 8 s, at trick-1 start) — it already behaves as required.
- Do not change meld detection, `DeclarationOverlap`, scoring, or bidding.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|---|---|---|---|
| Phase opens | Croatian bid resolves | `phase=declaring`, `trickNumber=0`, all `declarationAnswered=false`, `awaitingDeclaration=false`, `turnExpiresAt=null`, `activePlayerSeat=(dealer+1)%4` for the whole phase | N/A |
| Meld holder declares | `declare` from seat 2, unanswered | Melds stored, `declarationAnswered[2]=true`; no `event:player_declared`; other seats' `declarations` stay masked | N/A |
| No-meld seat skips | `skip_declare` from seat 0, no melds | `declarationAnswered[0]=true`, phase continues | N/A |
| No-meld seat declares | `declare` from a seat with no melds | Rejected | `ErrDeclarationNotAvailable` |
| Double answer | Second `declare`/`skip_declare` from an already-answered seat | Rejected, first answer stands | `ErrWrongPhase` |
| All four answer early | 4th answer at t=3 s | Resolve, `event:declarations_resolved`, `phase=playing`, `trickNumber=1`, trick-1 turn timer armed | N/A |
| Window elapses | Ceiling fires with seats unanswered | Unanswered seats treated as skipped, same resolve-and-open-trick-1 path | N/A |
| Seat disconnects mid-phase | Drop during `declaring` | Table pauses (existing disconnect path); close gate ignores disconnected seats | N/A |
| Reconnect mid-phase | Client rejoins during `declaring` | Snapshot carries own `declarationAnswered`; an already-answered seat sees the waiting state, never a second chance | N/A |
| Bitola trick 1 | `DeclarationTimingDuringFirstTrick` | Unchanged one-at-a-time prompt, per-move timer, live declare banner | unchanged |

</frozen-after-approval>

## Code Map

**Engine (`server/internal/game/`)**
- `declarations.go:320-443` -- `openDeclarationPhase` / `advanceDeclarationPhase` (the cursor to delete), `handleDeclare` / `handleSkipDeclare` (`state.Phase == PhaseDeclaring` arms at :412, :438), `handleDeclaring:305`. `resolveDeclarationsForHand:585` is the reuse point — do not modify.
- `state.go:9-52` -- `PlayerState`; new `DeclarationAnswered` bool with json tag `declarationAnswered` goes here, beside `FaceDownCount` / `HandCount`.
- `state.go:188-199` -- `DeclarationSeatsAnswered` (server-only int) — the cursor counter this change removes.
- `state.go:234-240` -- `HandCompleteReady [4]bool` — the precedent to mirror (per-seat answered flags + connected-only gate).
- `continue.go` -- `handleContinue` / `allConnectedReady` / `ForceAdvanceHandComplete`: the exact shape for `handleDeclare`-in-phase, the close gate, and the exported force-close.
- `projection.go:32` -- `ProjectForSeat`. Already masks other seats' `Declarations` while `!DeclarationsResolved`; the new boolean is public and passes through unmasked.
- `pause.go:9-16` -- drop `PhaseDeclaring` from the allowed set (no per-move clock left to preserve; `PhaseHandComplete` is already excluded for the same reason).
- `bidding.go:203-216` -- entry point (`DeclarationTiming == DeclarationTimingDedicatedPhase` → `openDeclarationPhase`); unchanged.
- `surrender.go:16`, `rules_engine.go:84`, `scoring.go:178` -- read-only: confirm they still hold with no cursor.
- `testfixtures/fixtures.go` -- factories; tests must not use raw `GameState{}`.

**Match layer (`server/internal/match/`)**
- `live_match.go:32-36` -- `handCompleteExpiresAt` field; add `declarationExpiresAt` next to it.
- `live_match.go:474-524` -- `HandleAction` timer branches. Remove `PhaseDeclaring` from the per-move branch at :477; add a `PhaseDeclaring` branch modelled on the `PhaseHandComplete` one at :508 (deadline fixed on **entry** only, re-armed against the same deadline).
- `live_match.go:400-430` -- error-path re-arm; needs the same `PhaseDeclaring` fixed-deadline treatment as `PhaseHandComplete`.
- `live_match.go:1444-1505` -- `handCompleteAutoContinue` + `handleHandCompleteTimeout`: the template for `declarationAutoClose` + `handleDeclarationTimeout`.
- `live_match.go:1576-1605` -- `handleTimerExpiry`: delete the `PhaseDeclaring` conjunct at :1576 and the defensive `PhaseDeclaring` case at :1589.
- `live_match.go:1689-1705` -- auto-action chain loop: drop the `PhaseDeclaring` arms.
- `live_match.go:926-945` -- `event:player_declared` emit; suppress when `oldState.Phase == PhaseDeclaring`. `broadcastDeclarationsResolvedIfTransition` stays.
- `live_match.go:1747` -- post-chain timer arm; drop `PhaseDeclaring`.
- `bot_driver.go:96-133` -- `botDecisionSeats` `PhaseDeclaring` arm: schedule **every** bot seat with `!DeclarationAnswered` (currently only the prompted one). `botThinkDelay` must return `botDelayMin` for this phase so bots land well inside the window.
- `reconnect.go:92-97` -- keep `PhaseDeclaring` in the disconnect switch (a drop must pause, or the connected-only gate waits out the ceiling).
- `reconnect.go:425-455` -- restore branch: move `PhaseDeclaring` out of the per-move re-arm into a fresh-ceiling branch beside `PhaseHandComplete`.
- `declaration_phase_test.go`, `auto_action_test.go`, `bot_driver_test.go` -- existing coverage to rewrite.

**Bot (`server/internal/bot/`)**
- `bot.go:29-46` -- the `PhaseDeclaring` crash-guard ladder; decide from the seat's own melds, not `AwaitingDeclaration`.
- `view.go` -- confirm the view carries what the decision needs.

**Client (`client/src/`)**
- `features/match/MatchPage.tsx:1496-1512` -- `showDeclarationPrompt` / `showDeclarationWaiting`. Rewrite: prompt every seat while `phase === "declaring" && !myPlayer.declarationAnswered && trumpReveal === null`; delete `showDeclarationWaiting`.
- `MatchPage.tsx:1622` -- `const isActive = ...`; gate on `phase !== "declaring"` so no seat is lime-flagged.
- `MatchPage.tsx:1330` -- `promptDeclarations` memo (may now be empty). `:1112-1120` -- `handleDeclare` / `handleSkipDeclare`. `:2068-2095` -- render blocks.
- `MatchPage.tsx:625-700` -- `handCompleteAcked` + the phase-exit reset effect: the local-state pattern to mirror for the post-answer waiting state.
- `features/match/components/DeclarationPrompt.tsx` -- empty state, disabled Declare, mount-anchored 8 s auto-skip, post-answer waiting label.
- `features/match/components/ScoreReveal.tsx:159-200` -- the mount-anchored auto-fire + `ringDrainStyle` + `acknowledged` disabled-waiting pattern to copy.
- `features/match/components/DeclarationWaiting.tsx` + `.test.tsx` -- **delete both**.
- `shared/lib/motion.ts:138` -- add `DECLARATION_PHASE_AUTO_SKIP: 8000` beside `SCORE_REVEAL_AUTO_CONTINUE`.
- `shared/types/matchTypes.ts:210` area + `shared/types/wsEvents.schemas.ts:136` area -- add `declarationAnswered` to the `PlayerState` type and its `z.strictObject` schema (strict: an unlisted field fails parsing).
- `shared/i18n/{en,hr,mk,sr}.json` -- `match.declaration.*`: drop `waiting`, add the empty-state and waiting-for-others keys. `i18n.parity.test.ts` gates all four.
- `features/match/lib/declarations.ts`, `engineMirrors.contract.test.ts` -- read-only unless the mirror needs the new flag.

**Contract**
- `server/internal/ws/testdata/events/event_match_state.json` -- add `declarationAnswered` to all four player objects; `ws/events_contract_test.go` gates it. No new event type, so `events.go` / `wsEvents.ts` are unchanged.

## Tasks & Acceptance

**Execution:**
- [x] `server/internal/game/state.go` -- add `PlayerState.DeclarationAnswered` (bool, json tag `declarationAnswered`); delete `DeclarationSeatsAnswered` -- per-seat answered flags replace the cursor.
- [x] `server/internal/game/declarations.go` -- delete `advanceDeclarationPhase`; `openDeclarationPhase` clears all four flags and pins `ActivePlayerSeat=(DealerSeat+1)%4`; `handleDeclare`/`handleSkipDeclare` accept any unanswered seat in `PhaseDeclaring` (reject a repeat with `ErrWrongPhase`) and call a new `maybeCloseDeclarationPhase`; add exported `ForceCloseDeclarationPhase` -- simultaneous answering, with the Bitola `TrickNumber == 1` arms untouched.
- [x] `server/internal/game/pause.go` -- remove `PhaseDeclaring` -- a fixed-window phase has no turn clock to freeze.
- [x] `server/internal/game/projection.go` -- verify the new flag passes through unmasked and add the reasoning to the doc comment -- the mask is by enumeration.
- [x] `server/internal/match/live_match.go` -- add `declarationExpiresAt` + `declarationAutoClose` (8 s player window + the trump reveal's 8 s + grace ≈ 20 s) and `handleDeclarationTimeout`; move `PhaseDeclaring` off the per-move timer in `HandleAction` (success and error paths), out of `handleTimerExpiry` and the chain loop; suppress `event:player_declared` inside the phase -- the phase becomes fixed-length, not turn-taking.
- [x] `server/internal/match/reconnect.go` -- restore `PhaseDeclaring` with a fresh ceiling window instead of a per-move re-arm -- mirrors `PhaseHandComplete`.
- [x] `server/internal/match/bot_driver.go` + `server/internal/bot/bot.go` -- schedule every unanswered bot seat at `botDelayMin`; decide from the seat's own melds -- bots must all answer inside the window.
- [x] `server/internal/ws/testdata/events/event_match_state.json` -- add `declarationAnswered` to all four players -- golden drift gate.
- [x] `client/src/shared/types/matchTypes.ts` + `wsEvents.schemas.ts` -- add `declarationAnswered` -- the schema is strict.
- [x] `client/src/shared/lib/motion.ts` -- add `DECLARATION_PHASE_AUTO_SKIP: 8000`.
- [x] `client/src/features/match/components/DeclarationPrompt.tsx` -- empty state with disabled Declare, mount-anchored 8 s auto-skip drain on Skip, post-answer disabled waiting label -- identical footprint for every seat.
- [x] `client/src/features/match/MatchPage.tsx` -- prompt every unanswered seat; suppress the active-seat highlight during `declaring`; delete the waiting-banner wiring; reset local answered state on phase exit.
- [x] delete `client/src/features/match/components/DeclarationWaiting.tsx` and `DeclarationWaiting.test.tsx`.
- [x] `client/src/shared/i18n/{en,hr,mk,sr}.json` -- drop `match.declaration.waiting`, add empty-state + waiting-for-others keys in all four locales -- idiomatic, never calqued; mk all-Cyrillic; `„…"` quotes in mk/hr/sr.
- [x] `server/internal/game/declarations_test.go` + `server/internal/match/declaration_phase_test.go` -- table-driven tests through `ApplyAction` only, using `testfixtures` factories, covering every I/O Matrix row.
- [x] `client/src/features/match/components/DeclarationPrompt.test.tsx` + `MatchPage.test.tsx` -- meld and no-meld dialogs, auto-skip at 8 s, waiting state, no active highlight during `declaring`.

**Acceptance Criteria:**
- Given a Croatian hand where only seat 2 holds melds, when the declaration phase opens, then every seat sees a dialog, no seat is highlighted as active, and nothing on any of the other three screens identifies seat 2.
- Given the phase is open, when the last connected seat answers before the window elapses, then the reveal and trick 1 start immediately.
- Given the phase is open, when no one answers, then every seat auto-skips at 8 s and trick 1 starts without any player action.
- Given a player has answered, when they look at their screen, then the dialog is still up in a disabled waiting state and closes only when the phase ends.
- Given a Bitola match, when declarations happen at trick 1, then every existing behaviour and test is unchanged.

## Spec Change Log

- **2026-08-23 — review round 1 (patch-only; no loopback).** Three parallel reviewers
  (blind-hunter, edge-case-hunter, verification-gap) ran over the diff. No intent_gap or
  bad_spec finding: the approach held, and every accepted finding was additive.
  Applied as patches:
  - **Rules reference contradicted the new phase.** `features/rules/content/{en,mk,hr,sr}.ts`
    still read "each seat in turn declares or skips". Rewritten in all four locales to describe
    the simultaneous 8s window. The Code Map had not listed this file — the one real spec gap,
    recorded here so a future phase change knows to look there.
  - **No local send latch on the client.** `answered` is server truth arriving a round-trip
    after the click, while the ring's `onExpire` runs on its own schedule — so a Skip at 7.9s
    was followed by an auto-skip at 8.0s that the engine rejected as `ErrWrongPhase`, showing
    the player a toast for something they did right. Added a `sentRef`/`sent` latch mirroring
    ScoreReveal's `firedRef`.
  - **Two verification gaps.** The forced-dealer-pick timeout is the second (player-less) door
    into the phase and nothing asserted it arms the window — without that one line the first
    answer force-closes the contest. Mutation-checked. And deleting the old "not on the clock"
    test left Bitola's trick-1 seat gate untested; re-added, mutation-checked.
  - **Reachability traps.** Restored a defensive `PhaseDeclaring` arm in `handleTimerExpiry`
    (session.turnTimer is one shared field), excluded the phase from the unpause branch that
    precedes its own arm, and seeded the window on the reconnect-with-surviving-pause path.
    All three are unreachable today; branch order is what makes them so.
  - Smaller: `RefreshDerivedFlags` in `ForceCloseDeclarationPhase`, focus-trap `tabIndex` for
    the all-disabled waiting state, phase-gated meld detection in `buildBotView`, corrected two
    comments that overclaimed what the wire hides, replaced a vacuous jsdom width assertion with
    a structural one, added locale-copy and phone-HUD coverage.

  **KEEP if re-derived:** the `HandCompleteReady`/`allConnectedReady`/`ForceAdvance` transplant;
  `ActivePlayerSeat` pinned rather than a `-1` sentinel; the mount-anchored client window with a
  server ceiling; asking meld-less seats so the phase is uniform; and suppressing
  `event:player_declared` inside the phase.

  **Deferred, not this story:** `TestVariantRulesAndDeclarationsContract/declaration_melds.json`
  flakes ~25% per run because `detectDeclarations` iterates Go maps. Reproduced 7/25 at baseline
  39346a9 in a clean worktree with the function byte-identical, and already recorded in
  `deferred-work.md` from an earlier story's review.

## Design Notes

**Why `activePlayerSeat` may stay set.** Pinning it to `(DealerSeat+1)%4` — where it must land for trick 1 anyway — makes it a *positional constant* for the phase, uncorrelated with melds. That avoids introducing a `-1` sentinel into the many paths that assume a 0–3 seat (`reconnect.go:430`, `buildStateFrames`, `PlayerSeat` rendering). The highlight is suppressed client-side on `phase`, which also drops the timer ring since `turnExpiresAt` is `null`.

**Why the window is mount-anchored with a server ceiling.** The trump-take reveal is itself an 8 s dialog and suppresses the declaration prompt (`trumpReveal === null`). A single absolute server deadline set at phase entry would be eaten by it. So: each client auto-skips 8 s after *its* dialog mounts (`ScoreReveal`'s `AUTO_CONTINUE_MS` pattern), and the server holds a fixed force-close ceiling measured from phase entry — exactly the `handCompleteAutoContinue` split, and sized the same way (player window + pre-mount pacing + network grace).

**Close gate:**

```go
func allAnsweredOrDisconnected(state *GameState) bool {
    for i := range state.Players {
        if state.Players[i].Connected && !state.Players[i].DeclarationAnswered {
            return false
        }
    }
    return true
}
```

## Verification

**Commands:**
- `make lint` -- expected: clean, both stacks.
- `make test` -- expected: all green, including every pre-existing Bitola test unchanged.
- `cd server && go test ./internal/game/... ./internal/match/... ./internal/bot/... ./internal/ws/... -count=1` -- expected: pass; the `ws` contract/golden tests confirm the `declarationAnswered` wire addition landed in both the Go golden and the Zod schema.
- `cd client && npx vitest run src/features/match src/shared/i18n` -- expected: pass, including `i18n.parity.test.ts` across all four locales.

**Manual checks:**
- Run a real four-player Croatian match (see the live-match debug harness) with at least one seat holding melds and one holding none: confirm all four dialogs appear together, no seat glows lime, the phase closes early when everyone clicks, closes at ~8 s when nobody does, and the 8 s reveal plays at trick-1 start.
- Inspect the `event:match_state` frames in a websocket inspector during `declaring`: no field distinguishes a meld-holding seat from a meld-less one before `declarationsResolved`.
