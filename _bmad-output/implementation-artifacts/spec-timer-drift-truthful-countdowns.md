---
title: 'Truthful countdowns: deadline-anchored rings, clock-offset sync, server expiry grace'
type: 'bugfix'
created: '2026-06-10'
status: 'done'
baseline_commit: '6303c2028870fc2166e2e05b2c61afb92f6d7c0c'
context:
  - '{project-root}/_bmad-output/implementation-artifacts/investigations/timer-ui-drift-investigation.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Every countdown widget (avatar turn ring + seconds label, prompt button rings, 8 s auto-close dialogs, score-reveal border, reconnect ring) visibly shows 1–3 s remaining when the auto-resolve actually fires: ring arcs animate toward per-second samples with a 1 s transition (always one tick behind), the `ceil` label can't show 0 before the deadline, and no client↔server clock-offset correction exists.

**Approach:** (A) Drive ring arcs with a single deadline-anchored CSS animation (shared keyframe + negative `animation-delay`) so the arc empties exactly at the deadline; align label ticks to whole-second boundaries of the deadline. (B) Stamp `serverNow` on the WS envelope from the match broadcaster; client keeps a smoothed clock offset and renders all server deadlines against corrected time. (C) Server fires auto-actions 400 ms after the advertised deadline (grace), so a synced client always sees 0 before the action lands.

## Boundaries & Constraints

**Always:** Update `wsEvents.ts` and the Go contract in the same commit. Preserve reduced-motion behavior (static, non-animated arc that still updates per tick). Keep the authoritative fire mechanisms unchanged (server `AfterFunc` for turns; client `setTimeout` for the 8 s local dialogs). Keep urgency thresholds (`URGENT_FRACTION`) and color semantics intact. Immutable Zustand updates; camelCase wire format.

**Ask First:** Changing any user-facing duration (turn seconds, 8 s dialogs, 14 s ceiling, 120 s reconnect window). Adding a new WS event type (the envelope field should suffice).

**Never:** Don't move auto-resolve authority to the client for server timers. Don't introduce a ping/pong sync protocol — envelope sampling is enough. Don't switch rings to `pathLength` normalization (use a CSS-var keyframe so geometry math stays untouched). Don't batch multi-event sequences.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Turn expiry, synced clocks | per-move timer reaches deadline T | label shows 0 at ~T; arc empty at T; server auto-acts at T+400 ms | N/A |
| Client clock 2 s behind server | state with `turnExpiresAt`, `serverNow` | offset correction ⇒ countdown matches server's true remaining | N/A |
| `serverNow` absent (old/other senders) | non-match WS message | offset unchanged; raw `Date.now()` fallback when no samples yet | N/A |
| Remount mid-window (reconnect, tab return) | `turnExpiresAt` in 12 s, total 30 s | arc resumes at 12/30 and empties at T (negative delay) | N/A |
| Skewed sample makes remaining > total | corrected remaining > `timerDurationSec` | clamp: delay ≤ 0, progress ≤ 1 | N/A |
| 8 s local dialog (AutoCloseRing / ScoreReveal) | mount at t0 | arc empties at t0+8 s, same instant the `setTimeout` fires | N/A |
| Reduced motion | `prefers-reduced-motion` | no CSS animation; arc updates statically per tick; existing short durations kept | N/A |
| Prompt inherits partial turn budget | BelotPrompt mounts with 5 s left of 30 | ring starts at 5/30 and empties at the true deadline | N/A |

</frozen-after-approval>

## Code Map

- `server/internal/ws/message.go` — WS envelope (`WSMessage`); gains optional `ServerNow`
- `server/internal/match/live_match.go` — `buildMessage` (stamp point, :1028); `startTimerLocked` (:1067); re-arm sites needing grace: error path (:235), unpause (:280), preserveTimer (:307)
- `server/internal/match/reconnect.go` — `startSeatReconnectTimerLocked` (:46) grace
- `server/internal/match/auto_action_test.go` — 1 s timer + 1.5 s sleep gets tight with grace; bump settle sleep
- `client/src/shared/types/wsEvents.ts` — envelope type `WsMessage` (:11)
- `client/src/shared/hooks/useWebSocket.ts` — single parse point (:98) → record offset sample
- `client/src/shared/lib/clockSync.ts` — NEW: offset estimation, `serverNow()`, `remainingMs(iso)`
- `client/src/features/match/lib/turnCountdown.ts` — `useTurnCountdown` (boundary-aligned ticks, corrected now); NEW `useRingDrain` style hook
- `client/src/index.css` — shared `@keyframes ring-drain` ending at `var(--ring-empty)`
- `client/src/features/match/components/TimerRing.tsx`, `overlay/ButtonTimerRing.tsx`, `overlay/AutoCloseRing.tsx`, `ScoreReveal.tsx` (`ButtonProgressBorder`), `ReconnectOverlay.tsx` (`useCountdownSeconds` :78, ring :336) — animation-driven arcs

## Tasks & Acceptance

**Execution:**
- [x] `server/internal/ws/message.go` — add `ServerNow *time.Time \`json:"serverNow,omitempty"\`` to `WSMessage` — envelope-level stamp avoids touching `GameState`
- [x] `server/internal/match/live_match.go` — stamp `ServerNow` in `buildMessage`; add `expiryGrace = 400ms` const; arm turn timer at deadline+grace at all four sites (extract small helper to centralize grace + generation capture) — client renders deadline, server enforces deadline+grace
- [x] `server/internal/match/reconnect.go` — apply same grace to seat-reconnect `AfterFunc`
- [x] server tests — assert `serverNow` present on match messages; assert auto-action does NOT fire before advertised deadline but does by deadline+grace+ε; adjust sleeps in `auto_action_test.go` (new file `timer_grace_test.go`; sleeps bumped in `auto_action_test.go`, `score_reveal_test.go`, `reconnect_test.go`)
- [x] `client/src/shared/lib/clockSync.ts` (+ test) — rolling window (≈10) of `serverNow − Date.now()` samples, offset = max(window) (min-latency bias); `serverNow()`, `remainingMs(iso)`; test-only reset
- [x] `client/src/shared/types/wsEvents.ts` — optional `serverNow?: string` on `WsMessage`
- [x] `client/src/shared/hooks/useWebSocket.ts` — record sample for every message carrying `serverNow`
- [x] `client/src/index.css` — `@keyframes ring-drain { to { stroke-dashoffset: var(--ring-empty, 1) } }`
- [x] `client/src/features/match/lib/turnCountdown.ts` (+ test) — chained `setTimeout(remaining % 1000 + ε)` aligned to deadline boundaries, corrected via `clockSync`; shared helper producing the drain-animation style (duration = total, delay = −elapsed, memoized per deadline so re-renders never restart the animation); lazy state init so a transient 0 can't fire consumers' `onExpire` on mount
- [x] `client/.../TimerRing.tsx` — arc via drain animation (`--ring-empty` = circumference); remove 1 s dashoffset transition; keep color-flip transition; static per-tick dashoffset retained as reduced-motion fallback (via `useReducedMotion`)
- [x] `client/.../overlay/ButtonTimerRing.tsx` — same animation treatment for both modes; aligned ticks for label/urgency/onExpire (server mode now reuses `useTurnCountdown`); fix dead conditional (:66)
- [x] `client/.../overlay/AutoCloseRing.tsx` — arc = drain animation over `duration` from mount; keep `setTimeout` fire + `paused` behavior (no animation while paused)
- [x] `client/.../ScoreReveal.tsx` — `ButtonProgressBorder` driven by drain animation; keep 8 s `setTimeout` fire + `acknowledged` cancel behavior
- [x] `client/.../ReconnectOverlay.tsx` — countdown via corrected clock + aligned ticks (shared hook replaces local `useCountdownSeconds`); ring via drain animation (ring geometry hoisted to module consts so the hook runs above the abandoned-state early return)
- [x] client component tests — update timing assertions (fake timers): boundary-aligned tick in `ReconnectOverlay.test.tsx`; new drain-animation regression test in `TimerRing.test.tsx`; new `AutoCloseRing.test.tsx` (8 s fire, click-once, paused, drain style, reduced-motion stepping); `onContinue` 8 s fire covered by `ScoreReveal.test.tsx`
- [x] adversarial-review patches — reconnect-resume turn timer now uses `armTurnTimerLocked` (+grace) and in-grace reconnects are accepted (`reconnect.go`); animated SVG elements are keyed by deadline (running CSS animations don't re-anchor on delay changes); `ringDrainStyle` end-anchors when skew makes remaining exceed the window; reduced-motion stepped fallbacks restored in `AutoCloseRing`/`ScoreReveal`; spurious-`onExpire` guard + pct clamp in `ButtonTimerRing`; defensive `Stop()` in `armTurnTimerLocked`; age-based sample eviction in `clockSync`; four more `reconnect_test.go` sleeps bumped; D136 (fixed grace vs high-latency links) deferred

**Acceptance Criteria:**
- Given a per-move turn with synced clocks, when the server auto-plays, then the client has already displayed 0 and an empty arc before the action lands (grace ≥ render margin).
- Given a client whose clock differs from the server by ±2 s, when any server-deadline countdown renders after the first match broadcast, then displayed remaining time matches the server's true remaining within ~1 network latency.
- Given any 8 s local dialog, when the auto-fire triggers, then the visible arc is empty at that instant (not at ~12.5%).
- Given `prefers-reduced-motion`, when any countdown runs, then no CSS drain animation is applied and the arc still updates per second statically.
- Given `make test` and `make lint`, both pass on both stacks.

## Design Notes

Negative-delay drain: one keyframe animates `stroke-dashoffset` from its inline value (full) to `--ring-empty`; per ring set `animationDuration = total`, `animationDelay = −(total − remaining)` computed once per deadline (memo) so the browser positions the sweep exactly and re-renders don't restart it. Offset estimation: each sample = trueOffset − latency, so max(window) approximates the min-latency sample; window expiry handles drift (WSL2 dev, NTP steps).

## Verification

**Commands:**
- `cd server && go test ./internal/match/... ./internal/ws/...` — expected: pass, incl. new grace + serverNow tests
- `cd client && npx vitest run` — expected: pass, incl. clockSync + turnCountdown + updated component tests
- `make lint` — expected: clean
- `make test` — expected: both stacks green

**Manual checks (if no CLI):**
- In a per-move match, watch a full turn expire: seconds hit 0 and the ring empties visibly before the auto-played card moves; info reveals close exactly as the X-ring empties.

## Suggested Review Order

**The truth contract (server): advertise T, fire at T+grace**

- The whole design in one constant — why the server waits 400 ms past the advertised deadline.
  [`live_match.go:1055`](../../server/internal/match/live_match.go#L1055)

- Single arming helper: grace + generation capture + defensive Stop, used by every per-move site.
  [`live_match.go:1068`](../../server/internal/match/live_match.go#L1068)

- Error-path and prompt-preserving re-arms route through the helper (original deadline kept).
  [`live_match.go:230`](../../server/internal/match/live_match.go#L230)

- Reconnect-resume turn timer gets the same grace (review patch — was a raw AfterFunc).
  [`reconnect.go:176`](../../server/internal/match/reconnect.go#L176)

- Seat-abandon timer fires grace-late too, and in-grace reconnects are accepted, not rejected.
  [`reconnect.go:379`](../../server/internal/match/reconnect.go#L379)

**Clock-offset sync (wire + client)**

- `serverNow` rides the WS envelope — one stamp point covers all ~20 match broadcasts.
  [`message.go:22`](../../server/internal/ws/message.go#L22)
  [`live_match.go:1025`](../../server/internal/match/live_match.go#L1025)

- Offset estimator: max of a rolling, age-capped sample window (min-latency bias, step recovery).
  [`clockSync.ts:1`](../../client/src/shared/lib/clockSync.ts#L1)

- One sampling point in the WS receive path; contract type alongside.
  [`useWebSocket.ts:107`](../../client/src/shared/hooks/useWebSocket.ts#L107)
  [`wsEvents.ts:20`](../../client/src/shared/types/wsEvents.ts#L20)

**Deadline-anchored rendering (the visible fix)**

- The shared keyframe: one animation per ring, anchored by negative delay, never per-tick transitions.
  [`index.css:504`](../../client/src/index.css#L504)

- `ringDrainStyle` / `useRingDrain`: end-anchored math, memo semantics, and the key-by-deadline CSS caveat.
  [`turnCountdown.ts:97`](../../client/src/features/match/lib/turnCountdown.ts#L97)

- Label ticks self-schedule on the deadline's whole-second boundaries; lazy init guards consumers.
  [`turnCountdown.ts:26`](../../client/src/features/match/lib/turnCountdown.ts#L26)

- Avatar ring: animation owns the arc; quantized dashoffset stays as reduced-motion fallback.
  [`TimerRing.tsx:79`](../../client/src/features/match/components/TimerRing.tsx#L79)

- Prompt button ring: shared hook for both modes, stale-0 `onExpire` guard, keyed rect.
  [`ButtonTimerRing.tsx:100`](../../client/src/features/match/components/overlay/ButtonTimerRing.tsx#L100)

- 8 s dialogs: mount-anchored sweep matches the `setTimeout` fire; stepped reduced-motion fallback.
  [`AutoCloseRing.tsx:65`](../../client/src/features/match/components/overlay/AutoCloseRing.tsx#L65)
  [`ScoreReveal.tsx:190`](../../client/src/features/match/components/ScoreReveal.tsx#L190)

- Reconnect overlay: shared hook replaces the local duplicate; center ring keyed by earliest expiry.
  [`ReconnectOverlay.tsx:106`](../../client/src/features/match/components/ReconnectOverlay.tsx#L106)

**Peripherals: tests and timing margins**

- Grace contract locked: advertised deadline excludes grace; no fire before it; serverNow on every message.
  [`timer_grace_test.go:19`](../../server/internal/match/timer_grace_test.go#L19)

- Estimator + hook behavior: offset math, boundary ticks, end-anchored drain styles.
  [`clockSync.test.ts:1`](../../client/src/shared/lib/clockSync.test.ts#L1)
  [`turnCountdown.test.ts:1`](../../client/src/features/match/lib/turnCountdown.test.ts#L1)

- New AutoCloseRing suite + drain regression in TimerRing; sleep margins widened for the grace.
  [`AutoCloseRing.test.tsx:1`](../../client/src/features/match/components/overlay/AutoCloseRing.test.tsx#L1)
  [`reconnect_test.go:240`](../../server/internal/match/reconnect_test.go#L240)
