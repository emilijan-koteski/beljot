---
baseline_commit: 9064985d28977af5315f79230656f5e1404028d1
---

# Story 9.4: Quick Play Coin Bracketing

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a player,
I want Quick Play to match me against players with similar coin balances when I'm low on coins,
so that I can keep playing without being priced out.

> **PO decision (2026-06-19):** the epic's illustrative granular bands (`0 / 1–50 / 51–150 / 151–499 / 500+`) are **superseded by a binary model** — the buy-in is **either 500 or 0**, decided solely by whether a player can afford the standard 500 stake. Players who can afford 500 play for 500; everyone else plays for free. See [Design Decision D1](#design-decisions). This keeps the implementation simple and **requires no migration / no new column** — the existing `rooms.coin_buy_in` (0 or 500) is itself the bracket key.

## Acceptance Criteria

**AC1 — Affordability bracketing at queue time**
**Given** a player queues for Quick Play
**When** the matchmaking service evaluates their queue entry
**Then** if their wallet balance ≥ 500, they are placed in the **default bracket** (buy-in 500)
**And** if their wallet balance < 500, they are placed in the **free bracket** (buy-in 0)
**And** the two brackets are kept strictly separate — a player only ever joins or creates a synthesized room whose buy-in matches their own bracket, so all four humans in any started match carry the same affordable stake

**AC2 — Synthesized room carries the bracket stake; all four charged at start**
**Given** a Quick Play match is formed (4 players seated in a synthesized room of one bracket)
**When** the server-side room auto-starts the match
**Then** the match's per-human stake (`coinBuyIn`) is the bracket value — **500** (default) or **0** (free)
**And** all four players are charged that stake atomically on match start, reusing the Story 9.2/9.3 charge path (buy-in 0 → no charge)
**And** settlement at match end proceeds per Story 9.2 rules (pot = humans × stake, winners split equally, etc.)

**AC3 — Zero/low-coin players play for free**
**Given** a player has fewer than 500 coins (including exactly 0)
**When** they Quick Play
**Then** they are placed in the free bracket (buy-in 0) → no charge, pot = 0, no settlement
**And** the match still completes normally and counts for XP/honor per the other Epic 9 stories (XP = Story 9.5, honor = Story 9.7 — **out of scope here**; this story adds no XP/honor code)

**AC4 — Bracket integrity on the lobby "tap a Quick Play room" path (`QuickJoin`)**
**Given** a player taps a specific Quick Play room in the lobby grid (`QuickJoin`)
**When** the join is evaluated
**Then** the join is **rejected with a clear error** if the room's bracket (its `coinBuyIn`) does not match the player's own bracket — a cross-bracket tap never seats the player into a stake they don't belong to
**And** the auto-matchmaking `QuickPlay` path always seats the player into a room of their own bracket

**AC5 — Quick Play default timer is per-move 30s (not relaxed)**
**Given** a Quick Play room is synthesized
**When** its config is set
**Then** its timer style is **`per-move`** (was `relaxed`) with a **30-second** per-move duration (was no timer)
**And** that timer flows through unchanged to the auto-started match session (the auto-start path already forwards the room's `TimerStyle` + `TimerDurationSeconds` to `StartMatch`)

**AC6 — No regressions to the existing Quick Play flow**
**Given** the existing matchmaking behaviors (FIFO oldest-room-first, `SKIP LOCKED` + retry loop, drift exclusion, auto-start on the 4th joiner, revert-on-start-failure, `ALREADY_IN_ROOM` guard, `LeaveSeat` blocked / `LeaveRoom` allowed)
**When** bracketing + the timer default are added
**Then** every one of those behaviors is preserved — bracketing only narrows *which* waiting room a player may join; the timer change only affects newly synthesized rooms' config

---

## Tasks / Subtasks

> **Read [Dev Notes](#dev-notes) in full before starting.** The single most important fact: **the Quick Play auto-start path does NOT charge stakes today** — charging lives only in the manual `POST /rooms/:id/start` handler. This story's core work is (a) affordability-bracketed matchmaking and (b) adding the charge/eject logic to the auto-start path by **reusing the Story 9.3 helpers** (`ejectInsolventAtStart`, `transferOwnershipOrClose`, `WalletService`) — do not re-implement charging. **No migration and no new DB column are needed** (per [Decision D1](#design-decisions)) — `coin_buy_in` (0 or 500) is the bracket key.

- [x] **Task 1 — Bracket helper (AC1)**
  - [x] Add a pure helper to the room package, e.g. `func quickPlayBuyIn(balance int) int { if balance >= quickPlayStandardBuyIn { return quickPlayStandardBuyIn }; return 0 }`, with a named const `quickPlayStandardBuyIn = 500`. Unit-test it table-driven across boundaries: 0, 499, 500, 501, large.
  - [x] **Do NOT add a migration or a `quick_play_bracket` column.** The bracket is fully expressed by `rooms.coin_buy_in ∈ {0, 500}`.

- [x] **Task 2 — Bracket-aware matchmaking query (AC1, AC5)**
  - [x] Add a `buyIn int` parameter to `FindQuickPlayRoomExcluding` in [server/internal/room/gorm_repo.go:226](../../server/internal/room/gorm_repo.go#L226) and add `AND coin_buy_in = ?` to its `WHERE`. Preserve `is_quick_play=true AND status='waiting' AND player_count<4`, `SKIP LOCKED`, `ORDER BY created_at ASC`, and the `excludedRoomIDs` drift-exclusion exactly.
  - [x] Update the `RoomRepository` interface in [server/internal/room/repository.go:24-29](../../server/internal/room/repository.go#L24-L29) (both `FindQuickPlayRoom` and `FindQuickPlayRoomExcluding`) and **every mock**: `mockRoomRepo` ([handler_test.go:195-203](../../server/internal/room/handler_test.go#L195)), `mockRepoForLobby` ([lobby_disconnect_test.go:89-92](../../server/internal/room/lobby_disconnect_test.go#L89)), and any others surfaced by `grep -rn "FindQuickPlayRoom"`.
  - [x] *(Index note: the existing partial index `idx_rooms_quick_play` on `(is_quick_play, status, player_count)` still serves the query as a prefix; `coin_buy_in` is a low-cardinality residual filter. Adding it to the index is optional and NOT required for this story — flag only if a perf concern arises.)*

- [x] **Task 3 — `QuickPlay` handler: bracket the caller, stamp synthesized rooms (AC1)**
  - [x] In `QuickPlay` ([handler.go:2694](../../server/internal/room/handler.go#L2694)), read the caller's balance via `h.walletService.GetBalance(userID)` and compute `buyIn := quickPlayBuyIn(balance)` **before** the retry loop. Guard `h.walletService == nil` (tests without a wired wallet): fall back to `buyIn = 0` so the matchmaking path still runs.
  - [x] Pass `buyIn` into `tx.FindQuickPlayRoomExcluding(triedRoomIDs, buyIn)`.
  - [x] On the create-new-room branch ([handler.go:2748-2762](../../server/internal/room/handler.go#L2748)), set `CoinBuyIn: buyIn` (was hard-coded `0`). **Update the stale comment** at [handler.go:2756-2759](../../server/internal/room/handler.go#L2756) to describe the 9.4 binary bracket.
  - [x] **Change the synthesized timer (AC5):** in the same `newRoom` literal, set `TimerStyle: "per-move"` (was `"relaxed"`) and `TimerDurationSeconds:` a `*int` pointing to **30** (was unset/nil). Valid styles are `relaxed` / `per-move`; `per-move` requires a duration in [10,120] ([handler.go:336-344](../../server/internal/room/handler.go#L336)) — 30 is in range. Use the package idiom for the `*int` (e.g. `d := 30; … TimerDurationSeconds: &d`).
  - [x] The `system:room_created` broadcast already includes `coinBuyIn`, `timerStyle`, and `timerDurationSeconds` ([handler.go:2822-2826](../../server/internal/room/handler.go#L2822)) — confirm it now carries the bracket value (500/0) and the per-move/30s timer, no extra field needed.

- [x] **Task 4 — `QuickJoin` bracket guard (AC4)**
  - [x] In `QuickJoin` ([handler.go:2854+](../../server/internal/room/handler.go#L2854)), after loading the target quick-play room, compute the caller's bracket (`quickPlayBuyIn(callerBalance)`) and **reject** if `room.CoinBuyIn != callerBuyIn` — return a clean `apperr` (define `ErrQuickPlayBracketMismatch`).
  - [x] Add `ErrQuickPlayBracketMismatch` to [server/internal/apperr/errors.go](../../server/internal/apperr/errors.go) with HTTP + WS mappings consistent with sibling quick-play errors (`ErrQuickPlayLeaveSeatBlocked`, `ErrRoomFull`). The client surfaces it as a short message (i18n in all four locales if a new string is needed).

- [x] **Task 5 — Charge at auto-start (AC2, AC3) — the core change**
  - [x] Add the charge/eject sequence to `startAutoStartedMatch` ([handler.go:1537](../../server/internal/room/handler.go#L1537)) **mirroring the manual-start path** at [handler.go:2571-2643](../../server/internal/room/handler.go#L2571): when `autoStartRoom.CoinBuyIn > 0 && h.walletService != nil` → prefilter `GetBalances` → eject every insolvent seat via `ejectInsolventAtStart` → atomic `ChargeStakes` with the TOCTOU re-eject → on `StartMatch` failure, **refund** charged humans via `ApplySettlement`, revert the room to `waiting`, broadcast `error:match_start_failed`, return the error. Skip the whole block when `CoinBuyIn == 0` (free bracket).
  - [x] Charge `autoStartRoom.CoinBuyIn` directly (500). **No `min`-of-balances computation** — the bracket value is the stake (per Decision D1). The room was synthesized with `coin_buy_in = 500` only for ≥500 players, so all four can afford it.
  - [x] Keep the existing row-locked structure of `autoStartIfFull` ([handler.go:1671](../../server/internal/room/handler.go#L1671)); the charge happens in `startAutoStartedMatch` (called after the lock tx commits the `playing` flip), exactly where the manual path charges relative to its status flip — be careful to mirror the revert/eject ordering so a failed charge reverts the room to `waiting` (use the existing `revertAutoStart` / status-revert helpers).
  - [x] Remove/replace the obsolete "no charge happens on this auto-start path" comment at [handler.go:1556-1558](../../server/internal/room/handler.go#L1556).
  - [x] Insolvency at quick-play auto-start is a **near-impossible edge** (a ≥500 player held by `ALREADY_IN_ROOM` cannot spend elsewhere; balances only rise via daily reward), but wire the eject path anyway as the safety net and confirm an ejected seat cleanly reverts the room to `waiting` with 3 seated.

- [x] **Task 6 — Backend tests (AC1–AC5)**
  - [x] Update the now-false `TestQuickPlay_RoomIsFree` ([coin_handler_test.go:190](../../server/internal/room/coin_handler_test.go#L190)) — a ≥500 player's synthesized room now carries buy-in 500; a <500 player's room stays 0. Rename/rewrite accordingly.
  - [x] New tests: bracket assignment (≥500 → 500, <500 → 0, exactly 0 → 0); matchmaking keeps the two pools separate (a free-bracket player does NOT join a 500 room and vice-versa); auto-start charges 500 for the default bracket and settles per 9.2; free-bracket auto-start charges nothing / settles nothing; `QuickJoin` cross-bracket rejection; existing auto-start/revert/drift behaviors still pass.
  - [x] **Timer (AC5):** assert a synthesized Quick Play room carries `TimerStyle == "per-move"` and `TimerDurationSeconds == 30`, and that the value reaches `StartMatch` on auto-start. Update any existing `TestQuickPlay_*` / `TestQuickJoin_*` assertions that expect `"relaxed"` / nil duration (e.g. in [handler_test.go](../../server/internal/room/handler_test.go) and [coin_handler_test.go](../../server/internal/room/coin_handler_test.go)).
  - [x] Reuse the Story 9.3 coin-test harness: `stubWallet` (seed balances), `setupCoinTestBC`, `broadcastsOfType` in [coin_handler_test.go](../../server/internal/room/coin_handler_test.go). Table-driven, co-located. `go test ./... && go vet ./...` clean.

- [x] **Task 7 — Frontend: verify settlement/balance UX fires for Quick Play; optional bracket display (AC2, AC3)**
  - [x] **No new event wiring should be required**: `event:coin_settlement` (toast + `authStore.walletBalance` update via `useWsDispatch.ts`) and `system:insolvent_ejected` (always-mounted lobby modal/redirect) already exist from Stories 9.2/9.3 and fire on the same events a Quick Play match will now emit. Add/extend a client test proving the settlement toast + balance update render after a default-bracket Quick Play match (they never did before, since buy-in was 0).
  - [x] Remove any client assumption that "Quick Play is free" (check [MatchmakingPage.tsx](../../client/src/features/lobby/MatchmakingPage.tsx), [QuickPlayTile.tsx](../../client/src/features/lobby/components/QuickPlayTile.tsx), and their tests).
  - [x] If `QuickJoin` returns the new bracket-mismatch error, ensure the client renders a sensible message (i18n all four locales if a new key is added).
  - [x] **Optional (not an AC):** surface the bracket on the matchmaking/lobby UI ("Playing for 500 coins" vs "Free"). If implemented, add i18n keys to **all four** locales (`en/sr/mk/hr`) per the [localization rules](#references). See [Decision D4](#design-decisions).

- [x] **Task 8 — Definition of Done sweep**
  - [x] WS contract: only touch `wsEvents.ts` + `events.go` together if a new event is added (none expected — reuse existing).
  - [x] `make lint` + `make test` green (both stacks); `gofmt`/`goimports`, ESLint/Prettier clean on changed files; i18n parity test passes if any strings were added.

### Review Findings

_3-layer adversarial code review (Blind Hunter, Edge Case Hunter, Acceptance Auditor), 2026-06-19. AC1–AC6 all ✅ satisfied; D1/D2/D3/D5 honored. 14 raw findings → 2 patch (Low), 1 defer, 10 dismissed (verified false-positives / handled-elsewhere)._

- [x] [Review][Patch] mk i18n: button name unquoted in bracket-mismatch string — quoted as `„Брза игра“` to match mk prose convention (mk.json:43/65/455) [client/src/shared/i18n/mk.json:486] — FIXED 2026-06-19
- [x] [Review][Patch] No test exercises the atomic-`ChargeStakes`-returns-insolvent (TOCTOU) re-eject sub-branch on the auto-start path; the prefilter-eject branch is covered, this one is not (near-impossible prod edge, money-handling code) [server/internal/room/handler.go:1592-1660] — FIXED 2026-06-19: added `TestQuickJoin_AutoStartChargeTOCTOUEjectsInsolventSeat`
- [x] [Review][Defer] Refund-on-`StartMatch`-failure is best-effort-with-log only — if `ApplySettlement` also fails the four players stay debited with no durable reconciliation [server/internal/room/handler.go:1639-1654] — deferred, pre-existing (inherits the Story 9.3 accepted interim; refund-failure coin-durability was already deferred in 9.3). See deferred-work.md D154.

---

## Dev Notes

### What this story actually is

Quick Play today synthesizes **free** rooms (`CoinBuyIn: 0`) and auto-starts when the 4th human is seated — **with no coin charge anywhere on that path**. This story makes Quick Play economically live with a **binary affordability bracket**: players who can afford the standard 500 stake are pooled together and play for 500; everyone else is pooled together and plays for free (0). The 500 stake is then charged at auto-start (the manual-start path already charges; the auto-start path does not). Settlement at match end is already fully reusable.

The bulk of the work is **backend, in `server/internal/room/handler.go`**. The frontend should need little-to-no new code because the settlement toast, wallet-balance update, and insolvency modal already exist (Stories 9.2/9.3) and fire on the events a Quick Play match will now emit. **No migration, no schema change** — the bracket is `rooms.coin_buy_in ∈ {0, 500}`.

### Bracketing model (the concrete algorithm)

```
buyIn(balance) = (balance >= 500) ? 500 : 0      // two brackets only

queue (QuickPlay):
  b := quickPlayBuyIn(callerBalance)              // 500 or 0
  room := FindQuickPlayRoomExcluding(tried, b)    // only rooms in the SAME pool
  if room == nil: create room with CoinBuyIn = b, TimerStyle = "per-move", TimerDurationSeconds = 30   // AC5
  seat caller; if 4 seated -> autoStartIfFull

tap a specific room (QuickJoin):
  if room.CoinBuyIn != quickPlayBuyIn(callerBalance): reject (ErrQuickPlayBracketMismatch)

auto-start (startAutoStartedMatch):
  if room.CoinBuyIn > 0: charge all 4 = room.CoinBuyIn (reuse 9.3 prefilter→eject→ChargeStakes→TOCTOU→refund)
  StartMatch(..., room.CoinBuyIn)                 // settlement uses this stake; 0 => no-op
```

- A ≥500 player is gated into the 500 pool **at join time**; held by `ALREADY_IN_ROOM` they can't spend elsewhere and balances only rise, so all four still afford 500 at auto-start. The eject path is a defensive safety net, not an expected branch.
- The two pools never mix: matchmaking filters on `coin_buy_in`, and `QuickJoin` rejects cross-pool taps. So "the players that joined [a 500 room] can afford the 500" is invariant.

### Current state of the code being modified (READ THESE)

| File | Today | This story changes |
|---|---|---|
| [room/handler.go `QuickPlay`](../../server/internal/room/handler.go#L2694) | Finds oldest waiting quick-play room (any) via `FindQuickPlayRoomExcluding`; else synthesizes a new room hard-coded `CoinBuyIn: 0`, `TimerStyle: "relaxed"`, no timer duration. Retry loop w/ drift exclusion; auto-seats creator; calls `autoStartIfFull`. | Compute caller's bracket buy-in (500/0) from balance; pass it to the finder; set `CoinBuyIn = buyIn`; set `TimerStyle: "per-move"` + `TimerDurationSeconds: 30` (AC5) on synthesized rooms. |
| [room/handler.go `QuickJoin`](../../server/internal/room/handler.go#L2854) | Seats caller into a specific tapped quick-play room; rejects non-quick / full / not-waiting; auto-starts. | Reject when the room's `CoinBuyIn` ≠ the caller's bracket. |
| [room/handler.go `startAutoStartedMatch`](../../server/internal/room/handler.go#L1537) | Builds `seatInfo`, calls `StartMatch(..., autoStartRoom.CoinBuyIn)`. **Comment L1556-1558 says "no charge happens on this auto-start path."** | Add prefilter→eject→charge→TOCTOU→refund when `CoinBuyIn > 0`, mirroring the manual-start path. |
| [room/handler.go `autoStartIfFull`](../../server/internal/room/handler.go#L1671) | Row-locks room, re-fetches players inside tx, flips to `playing` when 4 seated, calls `startAutoStartedMatch`; `revertAutoStart` on failure. | No structural change; the new charge lives in `startAutoStartedMatch`. Ensure a failed charge reverts to `waiting`. |
| [room/gorm_repo.go `FindQuickPlayRoomExcluding`](../../server/internal/room/gorm_repo.go#L226) | `WHERE is_quick_play=true AND status='waiting' AND player_count<4`, `SKIP LOCKED`, `ORDER BY created_at ASC`. | Add `AND coin_buy_in = ?` + a `buyIn` signature param. |
| [room/repository.go interface](../../server/internal/room/repository.go#L24) | `FindQuickPlayRoom()` / `FindQuickPlayRoomExcluding(excluded)`. | Add the `buyIn` param; update all mocks. |
| [room/model.go `Room`](../../server/internal/room/model.go#L9) | Has `CoinBuyIn int` (default 0, CHECK ≥0), `IsQuickPlay bool`. | **No change** — `CoinBuyIn` is the bracket key. |
| [room/handler.go manual `/start`](../../server/internal/room/handler.go#L2571-L2643) | **The reference implementation of charging** (prefilter `GetBalances` → `ejectInsolventAtStart` for all insolvent → atomic `ChargeStakes` w/ TOCTOU re-eject → `ApplySettlement` refund on `StartMatch` failure). | Don't modify — **copy its structure** into `startAutoStartedMatch`. |
| [match/settlement.go](../../server/internal/match/settlement.go) | `settleMatch`/`computeSettlement`. **No-op when `coinBuyIn <= 0`.** Pot = humans × buyIn; winners split; remainder to lowest seat; no-human-winner = sink. | **No change** — fully reused; free bracket (0) settles to nothing automatically. |

### Reuse map — DO NOT reinvent

- **Charging:** `WalletService` interface ([handler.go:65-70](../../server/internal/room/handler.go#L65)) — `GetBalance`, `GetBalances`, `ChargeStakes(userIDs, amount) (insolventUserID, err)`, `ApplySettlement(credits)`. Already injected into `RoomHandler`. Wallet layer does atomic `db.Transaction()` with `FOR UPDATE` in ascending userID order.
- **Insolvency ejection:** `ejectInsolventAtStart(roomID, room, insolventIDs, balances, members)` ([handler.go:906](../../server/internal/room/handler.go#L906)) and `transferOwnershipOrClose(...)` ([handler.go:708](../../server/internal/room/handler.go#L708)). Pass the **full prefilter `balances` map** to ejection (Story 9.3 patch: a one-entry map makes solvent heirs look broke and wrongly closes the room).
- **Settlement:** `match/settlement.go` — untouched.
- **Errors:** `apperr.ErrInsufficientCoins` (409) exists; add only `ErrQuickPlayBracketMismatch` for Task 4.
- **WS events:** `event:coin_settlement`, `system:insolvent_ejected`, `system:room_closed_insolvent`, `error:match_start_failed` exist on both contract files — no new event expected.
- **Frontend coin UX:** settlement toast + `authStore.walletBalance` update (`useWsDispatch.ts`); `InsolventEjectionModal` + always-mounted `useInsolventEjectRedirect` (works from `/matchmaking/:id`). All exist.

### Critical correctness rules (project-context.md + Story 9.3 learnings)

- **Bots never stake** — Quick Play never seats bots (always 4 humans), so this is moot, but keep the human-only assumption explicit.
- **Charge BEFORE the session goes live, atomically** — a failed charge must never leave a live match with an unpaid stake. `ChargeStakes` (FOR UPDATE) is the authoritative money guard; never trust a pre-check.
- **Refund on `StartMatch` failure** via `ApplySettlement` (best-effort-with-log is the accepted Story 9.3 interim — match it; do not build an outbox).
- **Eject path must `system:player_left` per freed seat** so remaining clients don't show a stale seat/owner (Story 9.3 patch — already handled inside `ejectInsolventAtStart`; just call it).
- **Money mutations are transactional/atomic**; the rules engine, scoring, declarations, and turn order are untouched by this story.
- **Wire format `camelCase`**, GORM `snake_case` columns — no new columns here, so nothing to add.

### Previous Story Intelligence (Story 9.3 — immediate predecessor)

From `9-3-insolvency-ejection-and-room-persistence.md` Dev Agent Record:
- The charge/eject/refund machinery was built for the **manual** start path and the return-to-room path. This story **extends it to the auto-start path** — same helpers, new caller.
- Heed these 9.3 patches (inherited automatically if you copy the manual-start charge block faithfully): (1) pass the **full** balances map to `ejectInsolventAtStart`/`transferOwnershipOrClose`; (2) broadcast `system:player_left` per freed seat; (3) add a best-effort out-of-tx `UpdateStatus(roomID,"waiting")` revert in any eject error fall-through.
- Client insolvency modal/redirect is driven by `system:insolvent_ejected` and is always-mounted → ejection from the matchmaking screen already works.
- No new external dependency, reused `ErrInsufficientCoins`; bots (UserID 0) never balance-checked/charged/ejected.

### Git Intelligence

The economy is freshly landed and hot: recent commits `9-2` (`feat(wallet): room buy-in & match coin settlement`), `9-3` (`feat(wallet): insolvency ejection & room persistence`), and the match-stake HUD touched exactly the files this story extends — `room/handler.go` (+674 lines across 9.2/9.3), `match/settlement.go`, `wallet/*`, `ws/events.go`. The 9.2/9.3 work shipped on `feat/9-2-…` / `feat/9-3-…` branches merged via PR. Follow the convention: branch `feat/9-4-quick-play-coin-bracketing`, one story = one branch = one PR.

### Project Structure Notes

- Backend domain shape is fixed (`model.go`, `repository.go`, `gorm_repo.go`, `handler.go`, `_test.go`); all changes stay in the existing `room` package — no new package, **no migration**.
- GORM conventions unchanged (no new columns). Tests co-located; Go table-driven; `data-testid` for any new client elements (none expected).

### Design Decisions

- **D1 — Buy-in is binary 500/0 (RESOLVED by PO 2026-06-19).** The epic's granular proximity bands (`0 / 1–50 / 51–150 / 151–499`) are dropped. A player who can afford 500 plays for 500; everyone else plays for free. Rationale: simple, no migration, fully serves the "never priced out" goal. `rooms.coin_buy_in ∈ {0, 500}` is the bracket key and the charge amount — no `min`-of-balances, no `quick_play_bracket` column.
- **D2 — Bracket boundary = 500 (the standard create-room default buy-in).** Single threshold; `balance >= 500`.
- **D3 — `QuickJoin` cross-bracket tap → reject (RESOLVED by PO 2026-06-19).** Return `ErrQuickPlayBracketMismatch`; the client shows a short message. (Not a silent reroute.)
- **D4 — Show the bracket on the matchmaking screen?** Not required by any AC; marked optional. If added, needs i18n in all four locales.
- **D5 — Quick Play default timer = `per-move` 30s (requested by user 2026-06-19).** Quick Play rooms were synthesized as `relaxed` (no timer); they now default to `per-move` / 30s so casual matchmade games keep moving. Server-driven: the per-move timer UI already exists (Story 4.5) and the value flows through `StartMatch`, so no game-page change is needed — only the synthesized-room literal and any test fixtures that asserted `relaxed`.

### Testing Standards

- **Backend:** Go `testing` + `testify`; table-driven; co-located; tests create their own data (no seed dependency). Matchmaking/charge tests run against the room handler with the Story 9.3 `stubWallet` + broadcast-capture harness (`setupCoinTestBC`, `broadcastsOfType`). Rules-engine factories are irrelevant (no engine change). `go test ./... && go vet ./...` clean.
- **Frontend:** Vitest; presentational; assert the existing settlement toast + balance update render for a default-bracket Quick Play match; `data-testid` selection.
- **Definition of Done:** server handler+repo+tests; WS contract both-files (only if touched); i18n all locales (only if touched); `make lint`; `make test`.

### References

- Epic + ACs: [epics.md → Story 9.4](../planning-artifacts/epics.md#L1858-L1880); Epic 9 overview [epics.md:1720](../planning-artifacts/epics.md#L1720); Story 9.2 settlement [epics.md:1763](../planning-artifacts/epics.md#L1763); Story 9.3 charge/eject [epics.md:1823](../planning-artifacts/epics.md#L1823). *(Note: the epic's example bands are superseded by Decision D1.)*
- Quick Play: [`QuickPlay`:2694](../../server/internal/room/handler.go#L2694), [`QuickJoin`:2854](../../server/internal/room/handler.go#L2854), [`autoStartIfFull`:1671](../../server/internal/room/handler.go#L1671), [`startAutoStartedMatch`:1537](../../server/internal/room/handler.go#L1537), [`seatPlayerIntoQuickRoom`:1640](../../server/internal/room/handler.go#L1640).
- Charge reference: [handler.go:2571-2643](../../server/internal/room/handler.go#L2571); eject helpers [handler.go:906](../../server/internal/room/handler.go#L906) & [708](../../server/internal/room/handler.go#L708).
- Repo query: [gorm_repo.go:226](../../server/internal/room/gorm_repo.go#L226); interface [repository.go:24](../../server/internal/room/repository.go#L24); mocks [handler_test.go:195](../../server/internal/room/handler_test.go#L195), [lobby_disconnect_test.go:89](../../server/internal/room/lobby_disconnect_test.go#L89).
- Model: [model.go:9-38](../../server/internal/room/model.go#L9). Settlement: [match/settlement.go](../../server/internal/match/settlement.go). Wallet: [wallet/service.go](../../server/internal/wallet/service.go). Errors: [apperr/errors.go](../../server/internal/apperr/errors.go).
- Predecessor: [9-3-insolvency-ejection-and-room-persistence.md](9-3-insolvency-ejection-and-room-persistence.md).
- Project rules: [project-context.md](../project-context.md) (server-authoritative, atomic wallet mutations, `camelCase` wire, GORM conventions). Localization terminology (banned word "contract"; mk all-Cyrillic; idiomatic mk/hr/sr) applies to any new i18n strings.

## Dev Agent Record

### Agent Model Used

claude-opus-4-8 (Opus 4.8, 1M context) — BMad create-story workflow

### Debug Log References

- Initial backend test run surfaced `TestQuickJoin_AutoStartInsolventSeatEjected` returning HTTP 409 (`QUICK_PLAY_BRACKET_MISMATCH`) instead of 200. Root cause was the **test stub**, not handler code: `stubWallet.GetBalance` returned the scalar `balance` (0) and ignored the per-user `balances` map, so the solvent 4th joiner's join-time bracket read resolved to 0 and the QuickJoin bracket guard correctly rejected it. Fixed the stub to consult `balances` first (mirrors the real wallet so `GetBalance`/`GetBalances` agree). Full room suite re-verified green afterward — no production behavior change.

### Completion Notes List

**Implemented (Story 9.4 — Quick Play Coin Bracketing):**

- **Binary affordability bracket (AC1, Decision D1).** Added `quickPlayBuyIn(balance) int` + const `quickPlayStandardBuyIn = 500` in `room/handler.go`: `balance >= 500 → 500, else 0`. This value is BOTH the matchmaking pool key (`rooms.coin_buy_in`) and the per-human stake. **No migration / no new column** — `coin_buy_in ∈ {0, 500}` is the bracket key.
- **Bracket-aware matchmaking (AC1).** `FindQuickPlayRoom`/`FindQuickPlayRoomExcluding` gained a `buyIn int` param + `AND coin_buy_in = ?` (interface + GORM impl + all 3 mocks updated). `QuickPlay` reads the caller's balance once before the retry loop, brackets it, passes it to the finder, and stamps the synthesized room's `CoinBuyIn = buyIn`. A `nil` walletService degrades to the free bracket so legacy/test paths still run.
- **Per-move 30s timer (AC5, Decision D5).** Added const `quickPlayTimerDurationSeconds = 30`; synthesized rooms now carry `TimerStyle: "per-move"` + `TimerDurationSeconds: &30` (was `relaxed`/nil). Verified the value reaches `StartMatch` on the auto-start path.
- **QuickJoin bracket guard (AC4).** Added `apperr.ErrQuickPlayBracketMismatch` (409, `QUICK_PLAY_BRACKET_MISMATCH`); `QuickJoin` rejects a cross-bracket tap (checked before the capacity check). Client surfaces it via a new `lobby.errors.quickPlayBracketMismatch` string in all four locales + a `LobbyPage` handler.
- **Charge at auto-start (AC2/AC3) — the core change.** `startAutoStartedMatch` now mirrors the manual `/start` charge block: `GetBalances` prefilter → `ejectInsolventAtStart` for any insolvent seat → atomic `ChargeStakes` with the TOCTOU re-eject → refund via `ApplySettlement` on `StartMatch` failure. Skipped entirely when `CoinBuyIn == 0` (free bracket). The full prefilter `balances` map is passed to the eject helper (Story 9.3 patch — avoids wrongly closing a room with solvent heirs).
- **Caller contract fix.** `autoStartIfFull` now skips the generic `revertAutoStart` when `startAutoStartedMatch` returns `ErrInsufficientCoins` — the eject path already reverts the room and fans out the per-player ejection, so running the generic revert would wrongly broadcast `error:match_start_failed` to the remaining players. Generic start/charge failures still revert + broadcast as before.
- **No regressions (AC6).** FIFO/`SKIP LOCKED`/drift-exclusion/auto-start/revert/`ALREADY_IN_ROOM`/`LeaveSeat`-blocked behaviors all preserved — full existing room suite passes. No new WS events (reused `event:coin_settlement`, `system:insolvent_ejected`, `error:match_start_failed`), so no contract-file changes.
- **Frontend.** The settlement balance-update + result-dialog handler in `useWsDispatch.ts` is match-source-agnostic, so a default-bracket Quick Play match now drives it unchanged (it never fired before, since QP was free) — added a Story 9.4 test documenting this. No "Quick Play is free" copy existed to remove; updated the stale Story 9.2 comment in `LobbyPage.tsx`. Bracket display on the matchmaking UI (Decision D4) was optional and not implemented.

**Verification:** backend `go build ./...` ✓, `go vet ./...` ✓, `golangci-lint run ./...` ✓, `go test ./...` ✓; frontend ESLint ✓, Prettier ✓, `npx vitest run` (866 tests) ✓, i18n parity ✓.

**Out of scope (per ACs):** XP (Story 9.5), honor (Story 9.7), optional matchmaking bracket display (Decision D4).

### File List

**Backend (`server/`):**
- `internal/room/handler.go` — `quickPlayBuyIn` helper + consts; `QuickPlay` bracketing + per-move/30s synthesized-room timer; `QuickJoin` bracket guard; `startAutoStartedMatch` charge/eject/refund; `autoStartIfFull` skip-revert on insolvency.
- `internal/room/gorm_repo.go` — `FindQuickPlayRoom`/`FindQuickPlayRoomExcluding` `buyIn` param + `coin_buy_in` filter.
- `internal/room/repository.go` — `RoomRepository` interface signatures for the two finders.
- `internal/apperr/errors.go` — added `ErrQuickPlayBracketMismatch`.
- `internal/room/bracket_test.go` — NEW: `quickPlayBuyIn` table-driven unit test.
- `internal/room/coin_handler_test.go` — `stubWallet.GetBalance` consistency fix; rewrote `TestQuickPlay_RoomIsFree` → `TestQuickPlay_SynthesizedRoomCarriesBracketAndTimer`; added pool-separation, cross-bracket-reject, auto-start charge/free/insolvency/refund tests + `seedQuickPlayRoomWithStake` helper.
- `internal/room/handler_test.go` — `mockRoomRepo` finder signatures; `fakeMatchStarter` timer-capture fields; `TestQuickPlay_CreatesNewRoom` timer assertions.
- `internal/room/lobby_disconnect_test.go` — `mockRepoForLobby` finder signatures.
- `internal/lobby/lobby_test.go` — `fakeRoomRepo` finder signatures.

**Frontend (`client/`):**
- `src/features/lobby/LobbyPage.tsx` — `QUICK_PLAY_BRACKET_MISMATCH` handler + updated comment.
- `src/shared/i18n/{en,sr,mk,hr}.json` — new `lobby.errors.quickPlayBracketMismatch` string.
- `src/shared/hooks/useWsDispatch.test.ts` — Story 9.4 Quick Play settlement test.
- `src/features/lobby/LobbyPage.test.tsx` — cross-bracket quick-join toast test.

### Change Log

| Date | Change |
|---|---|
| 2026-06-19 | Implemented Story 9.4 (dev-story): binary affordability bracketing for Quick Play (500/0), bracket-aware matchmaking + synthesized-room stake, `QuickJoin` cross-bracket guard (`ErrQuickPlayBracketMismatch` + i18n ×4), charge/eject/refund at auto-start (reusing Story 9.2/9.3 helpers), per-move/30s default timer for synthesized rooms. Backend + frontend tests added; full `make lint`/`make test` green. Status → review. |
