---
title: "Quick Play: Croatian/501 defaults + auto-fill empty seats with bots"
type: feature
created: "2026-09-05"
status: done
baseline_commit: 1dc8ca6e8f3657d3d49449c6f3c60425d98005eb
review_loop_iteration: 0
context:
  - "{project-root}/_bmad-output/project-context.md"
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Quick Play only ever starts once **four humans** seat themselves, and its synthesized rooms default to Bitola / 1001. A lone (or partial) queue stalls indefinitely, and the default game is the long variant. We want Quick Play to (a) default to the **Croatian** variant at **501** points, and (b) start on its own by backfilling empty seats with bots on a paced schedule so a player is never stuck waiting.

**Approach:** Flip the two Quick Play synthesis defaults. Add a server-side, room-keyed **auto-fill scheduler** that arms when a Quick Play room is created and, on a timed cadence, seats a bot into the lowest empty seat until the room is full — at which point the existing auto-start path runs the match. The cadence adapts to how many idle players are online:

- **Few players online (0 idle in the lobby):** waiting for humans is pointless — add a bot every **3 seconds** to the lowest empty seat until the room fills, then start.
- **Players are online but not joining (≥1 idle in the lobby):** be patient — a **20-second inactivity timer** adds one bot each time 20s pass with no new human. Any human joining **resets** the 20s timer. Fully quiet ⇒ 3 bots added over ~60 seconds, then the match starts (1 human + 3 bots).

## Boundaries & Constraints

**Always:**

- Newly synthesized Quick Play rooms use `Variant: "croatia"` and `MatchMode: "501"`. The Quick Play matchmaking query matches Croatian rooms.
- The scheduler is server-authoritative and room-keyed; it fills seats only in **Quick Play**, `waiting` rooms, and only the lowest-indexed empty seat, one bot per tick.
- Bots are seated by reusing the existing bot infrastructure (`room_bots` table via `tx.AddBot`, `system:bot_added` broadcast, seat snapshot). A bot-filled seat and the eventual match are marked bot-inclusive exactly as owner-added bots are today.
- **Threshold = ≥1 idle lobby player** selects the patient (20s) cadence; **0 idle** selects the fast (3s) cadence. "Idle" = a connected user who is neither in a waiting room nor in a live match (the lobby-stats `inLobby` bucket), excluding the initiator. The cadence is decided once when the scheduler is armed.
- **Human join in the patient path resets the 20s inactivity timer** (a fresh human never has a bot dropped on them within 20s of joining).
- **Bots keep their seats.** A later human fills only a still-empty seat; an already-seated bot is never removed to make room (3 humans + 1 bot is a valid outcome).
- When a bot fills the 4th seat, the match auto-starts through the existing `autoStartIfFull` → `startAutoStartedMatch` path, which must now include bot seats in the rules-engine seat info and charge **only human** seats.
- The scheduler tick runs under the room row-lock (`FindByIDForUpdate` inside `RunInTransaction`) and is generation-guarded so a stale/cancelled timer never seats a bot.
- The scheduler self-terminates (and seats no bot) when, at tick time, the room is missing, not `waiting`, not Quick Play, or already full; it is cancelled when the match starts.

**Ask First:**

- If any consumer other than the frontend depends on Quick Play always being Bitola/1001 (e.g. a hardcoded assertion elsewhere), surface it before changing the defaults.
- If the bracket buy-in interacts with bot fill in a way that would charge a bot or mischarge the lone human, halt and confirm before proceeding.

**Never:**

- Do not relax the `AddBot` REST handler's `ErrBotsNotAllowed` guard — owners still cannot manually add bots to a Quick Play room. Bot fill is purely the automated scheduler calling the repository directly.
- Do not change the manual (custom-room) start flow, `SelectSeat`, `JoinRoom`, or owner bot add/remove behavior.
- Do not remove or "bump" a seated bot to prefer a human; no disconnect/mid-match bot replacement.
- Do not make bots pay the coin buy-in or accrue XP/coins/honor/stats.
- Do not block real-time (sleep) in request handlers — all delays are `time.AfterFunc` timers.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Fast fill (nobody idle) | New QP room, 1 human, idle-lobby count == 0 | Every 3s a bot fills the lowest empty seat; at 4 occupants the match auto-starts (1 human + 3 bots) | Tick on a non-waiting/full/missing room seats nothing and stops |
| Patient fill (someone idle) | New QP room, 1 human, idle-lobby count ≥ 1 | After 20s with no new human, add 1 bot; repeat each quiet 20s; fully quiet ⇒ 3 bots at ~60s ⇒ auto-start | Same self-terminate guard |
| Human joins during patient wait | Another human quick-plays/quick-joins the waiting room | The 20s inactivity timer resets; the human takes an empty seat; already-seated bots stay | N/A |
| Room fills with humans first | 4 humans seat before the timer fires | Existing human auto-start fires unchanged; scheduler is cancelled | N/A |
| Bot fills the 4th seat | Scheduler tick seats a bot in the last empty seat | `autoStartIfFull` counts humans+bots == 4, flips to `playing`, starts the session with bot seat-info, charges humans only, broadcasts `system:match_started` | On `StartMatch` failure, existing revert path runs |
| Sole human leaves waiting QP room with bots seated | LeaveRoom by the last human | Room closes (`completed`); scheduler self-terminates on next tick / cancellation; no bot-only room persists | N/A |
| QP defaults | New QP room synthesized | Room is `croatia` / `501`; matchmaking finds Croatian QP rooms | N/A |

</frozen-after-approval>

## Code Map

- `server/internal/room/handler.go:3817-3860` — QuickPlay room synthesis. Change `Variant: "bitola"` → `"croatia"` and `MatchMode: "1001"` → `"501"`; fix the now-stale "It is Bitola, 1001…" comments (3852-3857).
- `server/internal/room/handler.go:3908-3961` — QuickPlay tail. When `createdNew`, arm the scheduler (`startQuickFill`); when the joiner joined an existing room, `resetQuickFill`; when `autoStartIfFull` returns matchStarted, `cancelQuickFill`.
- `server/internal/room/handler.go:4029-4062` — QuickJoin tail. A human joined a specific QP room → `resetQuickFill`; cancel on matchStarted.
- `server/internal/room/handler.go:2663-2734` `autoStartIfFull` — count seats as **humans + bots**; load bots (`tx.FindBotsByRoomID`) and pass to `startAutoStartedMatch`; on successful start, `cancelQuickFill(roomID)`.
- `server/internal/room/handler.go:2448-2551` `startAutoStartedMatch` — accept bots and build bot seat-info entries `{Seat, IsBot:true}` (mirror the manual-start block at `3566-3583`); charging stays human-only (iterates `players`).
- `server/internal/room/handler.go:3566-3583` — reference: manual-start seatInfo build with bots (the pattern to mirror).
- `server/internal/room/handler.go:3235-3237` — `AddBot` `ErrBotsNotAllowed` guard for QP rooms — **leave intact**; the scheduler bypasses the handler and calls the repo directly.
- `server/internal/room/handler.go:243-268` — `RoomHandler` struct + `NewRoomHandler`. Add a `*quickFillScheduler` field, constructed with default intervals (test-overridable). Idle-lobby count via capability type-assertions on the already-held `hub`/`matchStarter` (see Design Notes) — no new constructor params.
- `server/internal/room/gorm_repo.go:226-240` `FindQuickPlayRoomExcluding` — change the hardcoded `variant = "bitola"` predicate to `"croatia"`.
- `server/internal/room/quick_play_variant_test.go` — update the bitola-predicate expectations to croatia.
- **NEW** `server/internal/room/quick_fill.go` — the scheduler: roomID-keyed `time.AfterFunc` registry (model on `lobby_disconnect.go`), fast/patient mode chosen at arm time, generation-guarded ticks, injectable `fastInterval`/`patientInterval`. Each tick: `RunInTransaction`+`FindByIDForUpdate`, re-check QP/waiting/empty-seat, pick lowest seat free of humans (`FindPlayerBySeat`) and bots (`FindBotsByRoomID`), `tx.AddBot`, then post-commit broadcast `ws.SystemBotAdded` + `broadcastRoomSeatSnapshot`, then `h.autoStartIfFull`.
- `server/internal/room/lobby_disconnect.go` — reference pattern for a mutex-guarded, roomID-keyed timer registry with cancel.
- `server/internal/lobby/lobby.go:73-125` `GetStats` — reference for the idle-lobby bucketing (connected − in-match − in-waiting-room).
- Reuse: `pickFirstEmptySeat`, `teamForSeat`, `broadcastRoomSeatSnapshot`, `voidInvitesIfFull`, `ws.SystemBotAdded`, repo `AddBot`/`FindBotsByRoomID`/`FindPlayerBySeat`/`FindUserIDsByRoomStatus`.
- `client/src/features/lobby/components/MatchmakingDiagram.tsx:193-217` — render a bot occupant (`occupant.isBot`) with `botDisplayName(t, occupant.seat)` and the bot Avatar glyph instead of the blank `occupant.username`.
- `client/src/shared/lib/botName.ts` — reuse `botDisplayName`.
- `client/src/features/lobby/MatchmakingPage.tsx` — verify the `found` (seated) count includes merged bot entries; bots already flow in via `system:bot_added` (useWsDispatch/roomStore).

## Tasks & Acceptance

**Execution:**

- [x] `server/internal/room/handler.go` — QuickPlay synthesis: `Variant` → `"croatia"`, `MatchMode` → `"501"`; refresh stale comments.
- [x] `server/internal/room/gorm_repo.go` — `FindQuickPlayRoomExcluding` variant predicate → `"croatia"`.
- [x] `server/internal/room/quick_fill.go` (NEW) — implement the scheduler (`startQuickFill(roomID, initiatorID)`, `resetQuickFill(roomID)`, `cancelQuickFill(roomID)`, generation-guarded tick that seats one bot in the lowest empty seat and calls `autoStartIfFull`). Intervals injectable; default fast=3s, patient=20s. Mode chosen at arm time from the idle-lobby count (≥1 ⇒ patient).
- [x] `server/internal/room/handler.go` — `RoomHandler` holds the scheduler; add an `idleLobbyCount(exclude uint) int` helper using capability type-assertions on `hub`/`matchStarter` + `repo.FindUserIDsByRoomStatus("waiting")` (returns 0 when the capabilities are absent, i.e. tests).
- [x] `server/internal/room/handler.go` — wire `startQuickFill`/`resetQuickFill`/`cancelQuickFill` into QuickPlay, QuickJoin, and the `autoStartIfFull` success path.
- [x] `server/internal/room/handler.go` — `autoStartIfFull` counts humans+bots; `startAutoStartedMatch` includes bot seat-info and charges humans only.
- [x] `server/internal/room/quick_fill_test.go` (NEW) — cover the I/O matrix with ~1ms intervals: fast fill reaches 4 and starts; patient adds one bot per quiet interval; a human join resets the patient timer; a bot filling the 4th seat auto-starts with correct human+bot seat-info and human-only charge; stale/cancelled ticks are no-ops; non-waiting room self-terminates.
- [x] `server/internal/room/handler_test.go` — QuickPlay creates a `croatia`/`501` room; `TestQuickPlay_*` mocks updated.
- [x] `server/internal/room/quick_play_variant_test.go` — bitola → croatia expectations.
- [x] `client/src/features/lobby/components/MatchmakingDiagram.tsx` — bot occupants render the localized bot name + bot glyph.
- [x] `client/src/features/lobby/components/MatchmakingDiagram.test.tsx` — a bot orbit seat shows the bot name/glyph, not a blank; a `croatia`/`501` room shows the right chips.
- [ ] Verify no new i18n keys are required (`bots.seatName` exists in all four locales); add none unless a new string surfaces.

**Acceptance Criteria:**

- Given a fresh Quick Play with no other idle player online, when 3 seconds pass, then a bot occupies the next empty seat, and after the room fills the match auto-starts with 1 human + 3 bots.
- Given a fresh Quick Play with at least one idle player online, when 20 seconds pass with no new human, then exactly one bot is added; fully quiet, the match starts at ~60s with 3 bots.
- Given the patient path, when a human joins mid-wait, then the 20-second timer restarts and no bot is added within 20s of that join, and any already-seated bot remains.
- Given a bot fills the fourth seat, when the match auto-starts, then the room is `playing`, the session runs with the bot seats flagged `IsBot`, only human seats are charged, and `system:match_started` is broadcast.
- Given a new Quick Play room, when it is synthesized, then its variant is `croatia` and its match mode is `501`, and the matchmaking query pairs Croatian Quick Play rooms.
- Given the sole human leaves a waiting Quick Play room that already has bots seated, then the room closes and no bot-only room lingers.

## Design Notes

**Idle-lobby count without new wiring.** The room handler already holds the concrete `*ws.Hub` (as `Broadcaster`) and `*match.Manager` (as `MatchStarter`). Both already implement `ConnectedUserIDs()` / `InMatchUserIDs()` for `lobby.GetStats`. Type-assert them to small optional capability interfaces; when absent (test stubs), return 0 → fast path (a safe, sleep-free default). Idle count mirrors `lobby.GetStats`: connected − in-match − in-waiting-room (`repo.FindUserIDsByRoomStatus("waiting")`), excluding the initiator.

**Why an inactivity timer, not a fixed 60s clock.** With "reset on human join", the patient path is naturally a single 20s inactivity timer re-armed on each join; three quiet ticks land the third bot at ~60s — the user's "start after a minute with 3 bots" emerges, and mid-queue humans always get a fresh 20s window.

**Tick safety.** The tick copies the `AddBot` handler's envelope (`RunInTransaction` + `FindByIDForUpdate`, re-validate QP/waiting/seat-free, humans+bots < 4) so it serializes against concurrent human joins and auto-start, and is generation-guarded like the existing bot/turn timers so a cancelled or superseded timer is inert.

## Verification

**Commands:**
- `cd server && go test ./internal/room/... ./internal/match/... ./internal/game/...` — expected: all pass, incl. new scheduler tests.
- `make lint` — expected: golangci-lint + ESLint + Prettier clean.
- `make test` — expected: full Go + Vitest suite green (incl. i18n parity).

**Manual checks:**
- `make dev`, then Quick Play from one browser with no one else online → bots appear every ~3s in the matchmaking diagram (named + bot glyph) and the Croatia/501 match starts with 3 bots.
- With a second idle session connected (not joining) → first bot appears at ~20s, third at ~60s, then the match starts; if the second session joins mid-wait, the countdown restarts.
- `psql` the latest match row → `has_bots = true`, bot seats NULL + flagged, the human charged once.

## Suggested Review Order

**Quick Play defaults (Croatian / 501)**

- The two synthesized defaults flip — the whole reason for the variant/mode change.
  [`handler.go:3850`](../../server/internal/room/handler.go#L3850)

- Matchmaking now pairs Croatian Quick Play rooms (predicate flip, defence-in-depth).
  [`gorm_repo.go:240`](../../server/internal/room/gorm_repo.go#L240)

**Auto-fill scheduler (core — start here)**

- Entry point: arms the timer and picks the fast (3s) vs patient (20s) cadence ONCE from the idle-lobby count.
  [`quick_fill.go:123`](../../server/internal/room/quick_fill.go#L123)

- The paced tick: row-locked + generation-guarded, seats one bot in the lowest empty seat, then hands off to auto-start.
  [`quick_fill.go:210`](../../server/internal/room/quick_fill.go#L210)

- Idle-lobby count via capability type-assertions on the held hub/manager — no new constructor wiring.
  [`quick_fill.go:78`](../../server/internal/room/quick_fill.go#L78)

- Reset restarts the patient inactivity window on a human join (cadence kept); cancel stops it on start.
  [`quick_fill.go:160`](../../server/internal/room/quick_fill.go#L160)

**Handler ↔ scheduler wiring**

- QuickPlay arms on a new room, resets on join-existing (guarded by `!matchStarted`).
  [`handler.go:3990`](../../server/internal/room/handler.go#L3990)

- QuickJoin resets the inactivity timer for a specific room.
  [`handler.go:4110`](../../server/internal/room/handler.go#L4110)

- Auto-start now counts humans + bots and cancels the scheduler on a successful start.
  [`handler.go:2713`](../../server/internal/room/handler.go#L2713)

**Auto-start with bots**

- Bot seats travel into the rules engine flagged `IsBot`; the charge loop bills humans only.
  [`handler.go:2470`](../../server/internal/room/handler.go#L2470)

**UI**

- Matchmaking diagram renders a bot orbit seat with the localized name + bot glyph, not a blank.
  [`MatchmakingDiagram.tsx:190`](../../client/src/features/lobby/components/MatchmakingDiagram.tsx#L190)

**Tests (peripherals)**

- Fast fill → four seats → auto-start with 1 human + 3 bots, bot seats flagged, human-only charge.
  [`quick_fill_test.go:74`](../../server/internal/room/quick_fill_test.go#L74)

- Mixed 2 humans + 2 bots auto-start; both humans charged.
  [`quick_fill_test.go:129`](../../server/internal/room/quick_fill_test.go#L129)

- The handler→scheduler wiring is actually triggered (regression guard for the whole feature).
  [`handler_test.go:2576`](../../server/internal/room/handler_test.go#L2576)
