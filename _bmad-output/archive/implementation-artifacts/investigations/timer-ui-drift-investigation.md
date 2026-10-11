# Investigation: Countdown rings/labels finish 1–3 s after the auto-resolve fires

## Hand-off Brief

1. **What happened.** Every countdown widget (avatar turn ring, prompt button rings, 8 s auto-close dialogs, score-reveal border) visibly shows 1–3 s remaining at the moment the timer's action actually fires — Confirmed: the arc animation is structurally one tick (1 s) behind the wall clock, the seconds label uses `ceil` so it reads "1" through the entire final second, and there is zero client↔server clock-skew correction (variable extra 0–2 s, amplified by WSL2 clock drift in dev).
2. **Where the case stands.** Root causes Confirmed by direct code trace on both stacks; no live measurement was needed — the lag is derivable from the rendering math alone and matches the reported symptom exactly.
3. **What's needed next.** Apply Fix A (one-shot deadline-anchored arc animation + deadline-aligned label ticks), Fix B (server-timestamp-based clock-offset correction), optionally Fix C (small server grace after the advertised deadline).

## Case Info

| Field            | Value                                                                       |
| ---------------- | --------------------------------------------------------------------------- |
| Ticket           | N/A                                                                          |
| Date opened      | 2026-06-10                                                                   |
| Status           | Concluded (diagnosis)                                                        |
| System           | Dev: Go server in WSL2, browser on Windows host. Prod: Contabo VPS + client devices |
| Evidence sources | Source code (client + server), MOTION constants, prior session findings (score-reveal jam memory) |

## Problem Statement

User report: every UI loader — the turn ring + seconds counter around the active player's avatar, the 8 s auto-closing
info dialogs, the score-reveal auto-continue, and the turn-bound prompt dialogs — resolves/auto-acts ~1 s (sometimes
2–3 s) before the visible animation reaches 0. The seconds counter never reaches zero; the server takes the turn while
the UI still claims time remains. This misleads the player into believing they still have time.

## Timer Inventory (Confirmed, full map)

### Server (authoritative, all `time.AfterFunc`, generation-guarded)

| Timer | Duration | Start trigger | Deadline broadcast | Fire handler |
| ----- | -------- | ------------- | ------------------ | ------------ |
| Per-move turn timer | `timerDurationSec` (room config) | `setTurnExpiry` + `startTimerLocked` at action-apply time, same `time.Now()` instant — `server/internal/match/live_match.go:1048-1080` | `TurnExpiresAt` (absolute ISO timestamp) in every state broadcast | `handleTimerExpiry` `live_match.go:1144` → auto-pass / auto-skip-declare / auto-skip-belot / auto-play |
| Prompt-preserving turn timer | remainder of the original deadline | `preserveTimer` branch when seat+phase unchanged (declare, belot, surrender prompts) `live_match.go:293-310` | original `TurnExpiresAt` unchanged | same |
| Error-path re-arm | remainder, clamped ≥ 0 | rejected action restores ORIGINAL deadline `live_match.go:223-238` | clients still hold original timestamp (no broadcast on error) | same |
| Unpause floor | max(remaining, 3 s) | unpause `live_match.go:260-282` | fresh `TurnExpiresAt` | same |
| Hand-complete (score reveal) ceiling | **14 s fixed**, measured from pause start, never extended | `handCompleteExpiresAt` `live_match.go:331-339`, const at `live_match.go:1094` | **not broadcast** (TurnExpiresAt = nil during pause) | `handleHandCompleteTimeout` force-advance |
| Seat reconnect window | `reconnectWindowSec` (default 120 s), per seat | `startSeatReconnectTimerLocked` `server/internal/match/reconnect.go:46-56` | `ReconnectExpiresAt` (absolute timestamp) | abandon match |

Server-side consistency: the `AfterFunc` duration and the broadcast `TurnExpiresAt` are derived from the same
`time.Now()` call — **the server fires essentially exactly at the advertised deadline.** The drift is entirely a
client rendering/clock problem.

### Client (display + auto-ack)

| Widget | Source of truth | Tick mechanism | Used by |
| ------ | --------------- | -------------- | ------- |
| `useTurnCountdown` | server `turnExpiresAt` vs raw `Date.now()` | `Math.ceil((expiry−now)/1000)`, sampled by free-running `setInterval(1000)` — `client/src/features/match/lib/turnCountdown.ts:20-33` | PlayerSeat seconds label + TimerRing |
| `TimerRing` arc | integer `secondsLeft/totalDuration` | dashoffset target = current value, `transition: stroke-dashoffset 1000ms linear` — `client/src/features/match/components/TimerRing.tsx:125` | avatar ring |
| `ButtonTimerRing` (server mode) | `turnExpiresAt` vs raw `Date.now()`, same ceil+interval | same 1 s linear transition — `overlay/ButtonTimerRing.tsx:69-97,184` | TrumpPrompt, BelotPrompt, DeclarationPrompt buttons |
| `ButtonTimerRing` (`clientCountdown`) | local integer decrement from `totalDuration` | `setInterval(1000)`, fires `onExpire` at 0 | client-only reveals |
| `AutoCloseRing` | **local only**: `setTimeout(duration·1000)` fire + per-second continuous pct sampling | pct transition 1 s linear — `overlay/AutoCloseRing.tsx:50-74,140` | TrumpReveal, BelotReveal, DeclarationReveal (8 s / 1.5 s reduced) |
| `ScoreReveal` auto-continue | **local only**: `setTimeout(8000)` (`SCORE_REVEAL_AUTO_CONTINUE`) + per-second pct sampling | `ButtonProgressBorder` pct transition 1 s linear — `ScoreReveal.tsx:173-190,131` | hand-end dialog (server ceiling at 14 s) |

No clock-offset/sync mechanism exists anywhere (grep over both stacks for offset/skew/serverNow: zero hits), and no
server timestamp other than the deadlines themselves is sent.

## Confirmed Findings

### Finding 1: The arc is structurally one tick (~1 s) behind the wall clock

**Evidence:** `TimerRing.tsx:125`, `AutoCloseRing.tsx:140`, `ButtonTimerRing.tsx:184`, `ScoreReveal.tsx:131` — all
four use `transition: stroke-dashoffset 1000ms linear` (or `1s linear`) toward the **current** sampled value.

**Detail:** When a sample at time *t* computes remaining *r*, the arc spends the next 1000 ms animating **toward**
*r* — i.e. at any instant the drawn arc represents the remaining time as of one second ago. The fire (server action
or local `setTimeout`) triggers when the true remaining hits 0; at that instant the arc still displays ~1 s of
progress (1–2 s for `TimerRing`, whose value is additionally quantized to integers). The arc would finish draining
one second *after* the fire, but the dialog unmounts / state changes first. This is the constant ~1 s component on
**every** widget, including the purely client-side 8 s dialogs where no clocks or network are involved.

### Finding 2: The seconds label can never show 0 before the deadline

**Evidence:** `turnCountdown.ts:22` and `ButtonTimerRing.tsx:64,73` — `Math.max(0, Math.ceil((expiry−now)/1000))`,
sampled by a free-running 1 s interval whose phase is unrelated to the deadline.

**Detail:** `ceil` means the label reads "1" for the entire final second (T−1.0 s … T−0.0 s). It can only flip to 0
at the first sample at/after the true deadline — exactly when (or after) the server fires. So the auto-action always
lands while the counter shows 1, by construction. The free-running phase adds up to one more second of staleness.

### Finding 3: The server fires exactly at the advertised deadline — no server-side grace

**Evidence:** `live_match.go:1048-1057` (deadline stamp) and `live_match.go:1067-1080` (AfterFunc arm) use the same
`time.Now()`; `handleTimerExpiry` acts immediately.

**Detail:** There is no buffer between "deadline the client renders" and "moment the server takes the turn." Any
client-side rendering lag or clock skew translates 1:1 into perceived early-firing.

### Finding 4: No clock-skew correction exists

**Evidence:** grep over `client/src` + `server/internal` for offset/skew/serverNow/serverTime: zero hits.
`turnCountdown.ts:21-22` compares the server timestamp directly against raw `Date.now()`.

**Detail:** Any difference between the server clock and the client clock adds (server-ahead) or subtracts
(server-behind) directly from the displayed remaining time at fire.

## Deduced Conclusions

### Deduction 1: The variable 1–3 s spread = constant render lag (1–2 s) + clock skew (0–2 s)

**Based on:** Findings 1, 2, 4 + environment.

**Reasoning:** With perfectly synced clocks the user sees: label "1", arc at 1–2 s of sweep, at the moment the turn is
taken — explaining "sometimes at 1, sometimes at 2." The dev environment runs the Go server inside WSL2 while the
browser uses the Windows host clock; WSL2 clock drift (especially after host sleep/hibernate) of a second or more is a
well-documented phenomenon, and production clients' clocks are arbitrary. A server-ahead skew of +1 s pushes the
observed value to 2–3 s — explaining "sometimes even at 3."

**Conclusion:** The user's exact symptom (1–3 s, variable, on every widget including pure-client dialogs at ~1 s) is
fully reproduced by the three confirmed mechanisms; no additional bug is required.

## Hypothesized Paths

### Hypothesis 1 (user premise): "server duration is shorter than UI duration"

**Status:** Refuted.

**Resolution:** Server duration and broadcast deadline come from the same instant and value
(`live_match.go:1048-1080`). For the score reveal, the server ceiling (14 s) is *longer* than the client 8 s
auto-ack by design. The mismatch is in rendering and clocks, not configured durations.

## Missing Evidence

| Gap | Impact | How to Obtain |
| --- | ------ | ------------- |
| Measured WSL2↔Windows clock delta on the dev box | Would pin the skew share of the 3 s outlier | Compare `date +%s.%N` in WSL2 vs `Date.now()` in the browser console at the same instant |

## Conclusion

**Confidence: High.**

Three compounding mechanisms, all Confirmed in code:

1. **Arc render lag (~1 s, universal):** all rings animate toward the current sample over a 1 s linear transition, so
   the drawn arc always trails the truth by up to one full tick. Affects the avatar TimerRing, ButtonTimerRing,
   AutoCloseRing, and the ScoreReveal border — including the purely client-side 8 s dialogs.
2. **Ceil + free-phase label (~0–1 s):** the seconds counter shows "1" through the whole final second and cannot show
   0 before the deadline.
3. **Uncorrected clock skew (0–2 s, variable):** no time-sync; raw `Date.now()` vs server clock. WSL2 drift in dev,
   arbitrary client clocks in prod.

## Recommended Next Steps

### Fix direction

**Fix A — deadline-anchored animation (client, fixes the universal ~1 s on everything):**
- Replace per-second dashoffset targets with a single one-shot animation: on mount/deadline change, compute
  `remainingMs`, render the arc at the current fraction, then transition `stroke-dashoffset` to fully-drained with
  `transition-duration: remainingMs` linear. The arc reaches 0 exactly at the deadline, continuously smooth (also
  nicer than the current 1 Hz stepping). Apply to TimerRing, ButtonTimerRing, AutoCloseRing, ScoreReveal border.
- Align label ticks to the deadline: chain `setTimeout(remainingMs % 1000)` instead of a free-running
  `setInterval(1000)`, so the integer flips exactly on whole-second boundaries and shows 0 at the deadline.

**Fix B — clock-offset estimation (client+server, fixes the variable skew):**
- Include a `serverNow` timestamp in state broadcasts (or add a WS ping/pong time-sync). Client keeps a smoothed
  offset (min-RTT sample or EWMA) and computes `remaining = expiry − (Date.now() + offset)` in `useTurnCountdown`
  and `ButtonTimerRing`.

**Fix C — small server grace (optional hardening, one line):**
- Advertise `TurnExpiresAt = T` but arm `AfterFunc(T + ~400 ms)`. Guarantees no client with a sane connection ever
  sees the auto-action land while displaying > 0.

Recommended: A + B (C as cheap insurance). The pure-client 8 s dialogs are fully fixed by A alone.

## Side Findings

- The score-reveal auto-ack chain (client 8 s from dialog mount, server 14 s from pause start) was hardened in commit
  `88205e8`; its design is sound — the perceived "fires early" there is Finding 1, not a duration mismatch.
- `ButtonTimerRing.tsx:66`: `return clientCountdown ? totalDuration : totalDuration;` — both branches identical;
  harmless but dead conditional.
- Prompts intentionally inherit the *remaining* turn budget (`live_match.go:293-310`), so a BelotPrompt can mount with
  only seconds left — correct by design but makes the early-fire perception more acute on prompts.
