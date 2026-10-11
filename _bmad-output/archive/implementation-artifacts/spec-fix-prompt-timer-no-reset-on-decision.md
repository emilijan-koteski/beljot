---
title: 'Fix: declaration / belote prompt decisions reset the per-move turn timer'
type: 'bugfix'
created: '2026-05-08'
status: 'done'
baseline_commit: 'ae9fe980a8256e842bfb28ffd749b17fb7bb4063'
context:
  - '{project-root}/_bmad-output/implementation-artifacts/spec-timer-auto-actions-and-dialog-rings.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** In per-move timer mode, when the active player resolves a within-turn prompt — `declare` / `skip_declare`, `announce_belot` / `skip_belot`, or a `play_card` of trump K/Q that triggers a belote prompt — the session manager unconditionally rewrites `TurnExpiresAt` to `now + timerDurationSec`. The same player gets a fresh full-duration window to play their card, which is unfair and breaks the social cue ("their timer didn't reset, so they had nothing to declare").

**Approach:** Treat declaration and belote prompts as part of the player's existing turn window. In `Manager.HandleAction` and `Manager.handleTimerExpiry`, preserve `oldState.TurnExpiresAt` and re-arm `time.AfterFunc` with `time.Until(*oldState.TurnExpiresAt)` whenever the active seat and phase did not change across the action — mirroring the surrender precedent at `manager.go:237-256`. Issue a fresh expiry only when the turn actually transitions (seat advances or phase changes).

## Boundaries & Constraints

**Always:**
- Discriminator is structural, not action-type enumeration: `oldState.TurnExpiresAt != nil && newState.ActivePlayerSeat == oldState.ActivePlayerSeat && newState.Phase == oldState.Phase` → preserve. Otherwise → fresh. This naturally covers `declare`, `skip_declare`, and `play_card → PendingBelotSeat` (seat unchanged) while letting `announce_belot` / `skip_belot` (`finishCardPlay` advances seat) and `pick_trump` (phase changes) get a fresh timer.
- When preserving: copy `oldState.TurnExpiresAt` and `oldState.TimerDurationSec` onto `newState`; compute `remaining := time.Until(*oldState.TurnExpiresAt)`; arm a `time.AfterFunc` only if `remaining > 0`. If non-positive, do not arm — do not silently extend.
- Use the post-`cancelTurnTimer` `session.timerGeneration` value when capturing the closure for `time.AfterFunc`, exactly as the surrender branch does.
- Apply the same fix in **both** `HandleAction` (lines 257-261) and `handleTimerExpiry` (lines 998-1001). Auto-`skip_declare` / auto-`skip_belot` must not extend the doomed player's window.
- Relaxed timer mode (`timerStyle != "per-move"`) keeps current behavior: no expiry, no timer.

**Ask First:**
- If a within-turn action is found that does NOT match the seat/phase predicate but should still preserve (or vice-versa), HALT before adding action-type special cases — the predicate is intentionally minimal.

**Never:**
- Do not enumerate `ActionDeclare`, `ActionSkipDeclare`, `ActionAnnounceBelot`, `ActionSkipBelot`, `ActionPlayCard` by name in the gating condition — the seat/phase predicate covers them all.
- Do not touch the rules engine (`server/internal/game/`). `cloneGameState` already preserves `TurnExpiresAt`; the bug is purely in the session manager's post-mutation rewrite.
- Do not change the WS event contract or any client component — the client passively renders whatever expiry the server sends.
- Do not change `pick_trump`, `pass_trump`, normal `play_card`, pause/unpause, or surrender branches.

## I/O & Edge-Case Matrix

| Scenario | State / Action | Expected `newState.TurnExpiresAt` |
|----------|---------------|-----------------------------------|
| Declare | seat S, expiry T, action `declare` | preserved = T |
| Skip declare | seat S, expiry T, action `skip_declare` | preserved = T |
| Play trump K/Q triggering belote | seat S, expiry T, `play_card` sets `PendingBelotSeat = S` | preserved = T |
| Announce belote | seat S, expiry T, action `announce_belot` advances seat | fresh, > T |
| Skip belote | seat S, expiry T, action `skip_belot` advances seat | fresh, > T |
| Auto skip_declare on expiry | timer fires, `AwaitingDeclaration`, seat unchanged post-action | preserved (already-elapsed); no new `AfterFunc` armed |
| Auto skip_belot on expiry | timer fires, `PendingBelotSeat == expectedSeat`, seat unchanged | preserved; no extension |
| Pick trump | bidding→playing transition, seat changes | fresh |
| Pass trump | seat advances | fresh |
| Normal play_card (no belote) | seat advances | fresh |

</frozen-after-approval>

## Code Map

- `server/internal/session/manager.go` — `HandleAction` lines 236-262 (bug locus #1) and `handleTimerExpiry` lines 992-1001 (bug locus #2). Surrender branch at 237-256 is the template for preserve-and-re-arm. `setTurnExpiry` (889) and `startTimerLocked` (903) stay unchanged.
- `server/internal/game/playing.go:91-96` — confirms `play_card` triggering `PendingBelotSeat` returns without advancing `ActivePlayerSeat`.
- `server/internal/game/declarations.go` — `handleDeclare` / `handleSkipDeclare` do not advance seat; `announce_belot` / `skip_belot` flow through `finishCardPlay` which does.
- `server/internal/session/manager_test.go` — `TestHandleAction_PerMoveTimer_ResetsOnAction` (currently exercises only `pick_trump`) stays green; new declaration / belote tests added here.
- `server/internal/session/auto_action_test.go` — coverage for the auto-action timer path.

## Tasks & Acceptance

**Execution:**
- [x] `server/internal/session/manager.go` — `HandleAction` else-branch now gates `setTurnExpiry`+`startTimerLocked` on the seat/phase predicate. The surrender-specific branch is replaced by a single `preserveTimer` check that subsumes it (surrender, declare/skip_declare, play_card-into-belote all hit it).
- [x] `server/internal/session/manager.go` — `handleTimerExpiry` chains `AutoPlay` after auto-`skip_declare` so the seat advances and the next seat (not the doomed player) gets the fresh timer. Final timer arming is gated on seat-or-phase actually changing.
- [x] `server/internal/session/manager_test.go` — Added all five HandleAction tests + a `firstTrickStateAt` helper. Added `testfixtures` import.
- [x] `server/internal/session/auto_action_test.go` — Added `TestAutoAction_SkipDeclare_ChainsToAutoPlayWithoutExtension` (verifies the chain: seat advances, exactly one card in trick, fresh expiry on next seat) and `TestAutoAction_SkipBelot_AdvancesSeatAndArmsFreshTimer`. Added `testfixtures` import.
- [x] `server/internal/session/export_test.go` — Added `SetGameStateForTest` and `TriggerTimerExpiryForTest` helpers; added `time` import.

**Acceptance Criteria:**
- Given the per-move active player resolves any within-turn prompt (`declare`, `skip_declare`, `play_card`-into-belote, `announce_belot`, `skip_belot`), when seat and phase are unchanged in `newState`, then `newState.TurnExpiresAt == oldState.TurnExpiresAt` and the timer is re-armed with the remaining duration only.
- Given an action that advances the seat or changes phase, when the manager processes it, then `newState.TurnExpiresAt` is `> oldState.TurnExpiresAt` (fresh window for the next seat).
- Given a per-move timer fires on a declaration or belote prompt, when `handleTimerExpiry` auto-issues `skip_declare` / `skip_belot`, then no fresh `time.AfterFunc` is armed for the same seat — the next legitimate turn arms a new one.
- Relaxed timer mode unchanged: `TurnExpiresAt` stays `nil` across all the above.
- `make test` and `make lint` green.

## Verification

**Commands:**
- `cd server && go test ./internal/session/... -run "PerMoveTimer|Declare|Belot|AutoAction" -v` — all listed tests pass, including the seven new ones.
- `cd server && go test ./...` — full Go suite green.
- `cd client && npx vitest run` — full Vitest suite green (no client changes, sanity check).
- `make lint` — clean.

**Manual checks:**
- Start a per-move timer game. On trick 1 with a declaration available, click Declare ~halfway through the timer. Countdown must continue from the same value, not reset. Repeat with Skip and with the belote prompt (play trump K holding Q).

## Suggested Review Order

**Player-initiated path: `HandleAction` preserve-vs-fresh predicate**

- Entry point. Single structural predicate replaces the old surrender-only branch; covers declare/skip_declare/play_card-into-belote/surrender uniformly.
  [`manager.go:236`](../../server/internal/session/manager.go#L236)

- The preserve branch mirrors the prior surrender re-arm verbatim — same `time.Until` + post-cancel `gen` capture pattern, applied to all within-turn prompts.
  [`manager.go:251`](../../server/internal/session/manager.go#L251)

**Auto-action path: structural chain loop in `handleTimerExpiry`**

- Loop replaces the original single-step refresh. Iterates while seat & phase still match `oldState`, picking the next auto-action structurally from `cur`'s open prompts. Bounded depth `maxChainSteps = 3` is the safety net.
  [`manager.go:1001`](../../server/internal/session/manager.go#L1001)

- Selector inside the loop — `PendingBelotSeat → skip_belot`, `AwaitingDeclaration → skip_declare`, otherwise auto-play. No action-type enumeration; the structural shape of `cur` decides.
  [`manager.go:1031`](../../server/internal/session/manager.go#L1031)

- Final timer arming: any non-terminal phase gets a fresh timer for the post-loop `finalState`. Defensive even when seat is unchanged after the loop (rare error path) so the game never stalls.
  [`manager.go:1083`](../../server/internal/session/manager.go#L1083)

- Per-step broadcasts in order. Only step 0 emits `EventAutoAction` (one toast per timer expiry, regardless of chain length); all card-play steps ride `autoPlayed=true`.
  [`manager.go:1101`](../../server/internal/session/manager.go#L1101)

**Test coverage**

- New `firstTrickStateAt` helper drives the five HandleAction tests using `testfixtures.NewGameFirstTrick`.
  [`manager_test.go:980`](../../server/internal/session/manager_test.go#L980)

- HandleAction preserve-expiry tests: declare, skip_declare, play_card-into-belote.
  [`manager_test.go:991`](../../server/internal/session/manager_test.go#L991)

- HandleAction refresh-expiry tests: announce_belot, skip_belot.
  [`manager_test.go:1075`](../../server/internal/session/manager_test.go#L1075)

- Auto-action chain regressions: skip_declare chains to auto-play (no extension), and the chained-belote-prompt edge case (K/Q of trump).
  [`auto_action_test.go:88`](../../server/internal/session/auto_action_test.go#L88)

- Auto-action skip_belot path (seat advances normally via `finishCardPlay`).
  [`auto_action_test.go:201`](../../server/internal/session/auto_action_test.go#L201)

**Test-only helpers**

- `SetGameStateForTest` + `TriggerTimerExpiryForTest` — inject a state and fire `handleTimerExpiry` with a matching `expectedSeat`. Avoids scripting an entire match to reach declaration / belote prompts.
  [`export_test.go:59`](../../server/internal/session/export_test.go#L59)

## Spec Change Log

- **2026-05-08 — Auto-action stall in `handleTimerExpiry` (resolution: chain auto-play).** During implementation, the seat/phase preserve predicate produced a stall in the auto-action path: after auto-`skip_declare`, the seat is unchanged and `time.Until(*oldExpiry) <= 0`, so no `AfterFunc` would be armed and the player would be left owing a card with no scheduler. Resolution: in `handleTimerExpiry` only, when the applied auto-action leaves seat unchanged and `Phase == PhasePlaying` and the player still needs to play (not awaiting declaration anymore, no pending belote), immediately chain `game.AutoPlay` for the same seat in the same call. The chained play advances the seat and the existing fresh-timer branch arms the next seat normally. The `HandleAction` (player-initiated) path is unaffected — real clicks always have `remaining > 0`. Updated the corresponding AC: instead of "no fresh `AfterFunc` armed for the same seat", read "no extension of the doomed player's window — the auto-action chains directly to auto-card-play so the seat advances and the next seat gets a fresh timer." KEEP: the structural seat/phase predicate as the single discriminator; do not re-introduce action-type enumeration.

- **2026-05-08 — Loopback patches from review (chain restructured to a structural loop).** Three reviewer findings filed as patches:
  1. *Stall on chained-belote from auto-play* (Edge Hunter #1): when the chain auto-plays K/Q of trump while the player holds both, `handlePlayCard` opens a belote prompt and returns without advancing the seat — the previous single-step chain had no provision for this and would stall.
  2. *Stall on chain failure* (Edge Hunter #2): if `AutoPlay`/`ParseCard`/`ApplyAction` errored during the chain, the timer was deliberately left unarmed without recovery.
  3. *Action-type enumeration in chain trigger* (Acceptance Auditor #1): the previous `action.Type == ActionSkipDeclare` gate contradicted the KEEP clause from the prior change-log entry.
  Resolution: replaced the single-step explicit chain with a bounded structural loop in `handleTimerExpiry`. While `cur.ActivePlayerSeat == oldState.ActivePlayerSeat && cur.Phase == oldState.Phase`, the loop picks the next auto-action from `cur`'s open prompts (PendingBelotSeat → `skip_belot`, AwaitingDeclaration → `skip_declare`, otherwise auto-play). Loop terminates when the seat or phase advances, or after `maxChainSteps = 3` iterations as a safety net. Each step's `(pre, post, action)` is recorded and broadcast in order. Defensive timer arming on chain exhaustion: when seat is still unchanged after the loop (anomalous error path), arm a fresh timer regardless — preferring the older "free fresh window" bug over a hard stall. Added `TestAutoAction_ChainedBelotPrompt_AdvancesPastDoomedSeat` regression test covering the K/Q-of-trump chain. KEEP: structural seat/phase predicate as the only chain discriminator; bounded depth via constant; one `EventAutoAction` toast per timer expiry (only the first step emits) regardless of chain length.
