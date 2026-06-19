---
baseline_commit: 62723dac598eb716535d01df4274fa05d7103c37
---

# Story 9.3: Insolvency Ejection & Room Persistence Between Matches

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a player,
I want to be cleanly redirected to the lobby when I can no longer afford a room's buy-in,
so that I'm never stuck at a seat I can't use and the room can keep going for those who can.

## Context & Scope Framing (READ FIRST)

This story **does not introduce a new "prepare for next match" phase**. It gates the **already-shipped Return to Room flow** ([spec-return-to-room-after-match.md](spec-return-to-room-after-match.md) v1 + [spec-return-to-room-presence-v2.md](spec-return-to-room-presence-v2.md) v2). The affordability check fires at the moment a player attempts to return, and — for the insolvency / owner-leave case only — it **supersedes v2's "no auto-transfer of ownership from an absent owner" rule**. This re-scoping is the authoritative source: [sprint-change-proposal-2026-06-18.md](../planning-artifacts/sprint-change-proposal-2026-06-18.md#93--insolvency-ejection--room-persistence).

This story also **completes the StartMatch hardening that 9.2 deferred** ([deferred-work.md](deferred-work.md) items 1 & 2, dated 2026-06-19): per-player insolvency ejection at match start (replacing 9.2's whole-table-rollback fail-safe) and refund-on-start-failure.

Three insolvency surfaces are in scope:
1. **Return-time gate** — `POST /rooms/:id/return` checks balance vs buy-in; insolvent → reject + free seat + route to lobby with a modal.
2. **Owner-leave / owner-insolvency ownership transfer** — transfer to first **present-AND-solvent** seated player; if none, **close the room**.
3. **Start-time authoritative guard** — the atomic re-validation inside `StartMatch`'s stake deduction is the single money authority; an insolvent-at-start player is routed through the same ejection flow (per-player, not whole-table rollback).

**Non-goal:** Do **not** add a `match_end` `OutcomeReason` (e.g. `insolvency`). 9.3's ejection happens **between/before** matches, never as a match outcome. The `// Epic 9 will add OutcomeReasonInsolvency and OutcomeReasonHonorEject` note at [server/internal/ws/events.go:155](../../server/internal/ws/events.go#L155) is for future honor work (9.7/9.8), not this story.

## Acceptance Criteria

**AC1 — Return-time affordability gate (`POST /rooms/:id/return`)**
Given a player clicks "Return to room" after a match,
When the request is processed,
Then the server checks their wallet balance against the room `coin_buy_in`;
And if `balance >= coin_buy_in`, the existing return / re-seat / presence behavior proceeds unchanged (idempotent reopen, bot clear, `system:player_returned`, `returnedUserIds`);
And if `balance < coin_buy_in`, the return is **rejected with HTTP 409 `INSUFFICIENT_COINS`**, their `room_players` seat is freed and the change broadcast to the room + lobby, and the client routes them to the lobby with a modal: *"You don't have enough coins to rejoin this room. Balance: [balance]. Buy-in: [buy_in]."*

**AC2 — Zero buy-in never bars**
Given the room `coin_buy_in` is 0,
When the affordability check runs,
Then it always passes (`balance >= 0`) — no one is ever barred for insolvency, and the existing return behavior is byte-for-byte unchanged (no new broadcast, no seat free).

**AC3 — Held-as-seated until the player acts**
Given a player has not yet acted on the result dialog (neither returned nor left),
When the room state is evaluated,
Then they remain **held as seated** (their `room_players` row survives) until they return or pick "Return to lobby" — they are **not** pre-emptively ejected. (No reservation timeout, no background sweep. This preserves the current post-match seat-retention model.)

**AC4 — Ownership transfer on owner barred / owner leaving**
Given the room owner is barred for insolvency (AC1 rejection) **or** the owner picks "Return to lobby" (leave),
When ownership must move,
Then ownership transfers to the **first seated player (seat order ascending) who is both present (has returned) and solvent (`balance >= coin_buy_in`)**;
And if no present-and-solvent player remains, the room is **closed** and everyone still seated is routed to the lobby via `system:room_closed_insolvent`;
And a player who later attempts to return to a now-closed room hits the existing non-member / closed-room path (`MATCH_ALREADY_STARTED` / `NOT_IN_ROOM` / room status `completed`) and is routed to the lobby.

**AC5 — StartMatch is the authoritative money guard (per-player ejection + refund-on-failure)**
Given the stake is actually deducted at match start (`StartMatch`),
When the charge transaction runs,
Then it re-validates `balance >= coin_buy_in` atomically as part of the deduction — the authoritative money guard that **never trusts the prior return-time check**;
And if a seated player is insolvent at that instant (race, or presence cleared by a server restart), they are **routed through the same ejection flow** (seat freed, per-user `system:insolvent_ejected`, transfer ownership if it was the owner) rather than starting the match with an unpaid stake — **replacing 9.2's whole-table rollback** ([deferred-work.md](deferred-work.md) item 2);
And if the atomic charge **succeeds** but `matchStarter.StartMatch` then **fails**, the charged stakes are **refunded** (no coins destroyed, no room stranded in `playing`) ([deferred-work.md](deferred-work.md) item 1).

## Tasks / Subtasks

> Reuse 9.2's economy machinery wholesale. No new migration. No new external dependency. Update both WS contract files in the same commit for every new event. Bots (UserID 0) are never balance-checked, charged, ejected, or counted toward ownership.

- [x] **Task 1 — Backend: return-time affordability gate** (AC1, AC2, AC3)
  - [x] In `RoomHandler.ReturnToRoom` ([server/internal/room/handler.go:863](../../server/internal/room/handler.go#L863)), after the membership check and **before** marking presence (`h.presence.Add` at line 964), add the gate: load the room (`postRoom`/`freshRoom`), and **only when `room.CoinBuyIn > 0 && h.walletService != nil`**, read the caller's balance via `h.walletService.GetBalance(userID)`.
  - [x] If `balance < room.CoinBuyIn`: do **not** add presence; free the caller's seat by removing their `room_players` row (reuse `tx.RemovePlayer` + `tx.DecrementPlayerCount`, mirroring `LeaveRoom`); if the caller is the owner, run the shared ownership-transfer-or-close helper (Task 3); broadcast `system:player_left` to remaining members + `broadcastRoomUpdated` to the lobby; return `apperr.ErrInsufficientCoins` (HTTP 409).
  - [x] If `room.CoinBuyIn == 0` or balance is sufficient: proceed with the existing flow **unchanged** (AC2 — the whole reopen/presence/broadcast path is byte-identical to today).
  - [x] Decide the transaction boundary: the gate read can be a plain read, but the **seat-free + ownership transfer must be inside a `RunInTransaction` + `FindByIDForUpdate`** block to serialize with concurrent returns/leaves (mirror the existing reopen tx at lines 898-938).
  - [x] Keep idempotency intact: an already-freed/non-member caller returns `apperr.ErrNotInRoom` (404) as today.

- [x] **Task 2 — Backend: extend the room handler's `WalletService` interface** (AC1, AC5)
  - [x] `WalletService` ([server/internal/room/handler.go:59-62](../../server/internal/room/handler.go#L59)) currently exposes `GetBalance` + `ChargeStakes`. Add what 9.3 needs: `GetBalances(userIDs []uint) (map[uint]int, error)` (batch solvency check for ownership transfer + start-time prefilter) and a refund capability — reuse the existing `ApplySettlement(credits map[uint]int) error` (already on the wallet `Repository`/`Service`, [server/internal/wallet/repository.go:30](../../server/internal/wallet/repository.go#L30)).
  - [x] `wallet.Service` already implements all of these — just widen the **room package's** interface and update the `coin_handler_test.go` `stubWallet` accordingly. Keep the import direction acyclic (`room → wallet`, never the reverse).

- [x] **Task 3 — Backend: shared "transfer ownership or close" helper, present-AND-solvent** (AC4)
  - [x] Extract a helper (e.g. `func (h *RoomHandler) transferOwnershipOrClose(tx RoomRepository, room *Room, departingOwnerID uint) (newOwnerID *uint, closed bool, err error)`) used by **both** the return-time eject path (Task 1) and `LeaveRoom`'s owner branch ([server/internal/room/handler.go:741-795](../../server/internal/room/handler.go#L741)).
  - [x] Candidate selection: iterate seated `room_players` (`Seat != nil`) in **seat order ascending**; pick the first who is **present** (`h.presence.Present(room.ID)` contains their userID) **AND solvent** (`GetBalances` shows `>= room.CoinBuyIn`; when `CoinBuyIn == 0` every seated human is trivially solvent). Skip bots.
  - [x] If a candidate is found: set `room.OwnerID`, persist, broadcast `system:room_owner_changed` (existing event) + `broadcastRoomUpdated`.
  - [x] If none: close the room (`room.Status = "completed"`), evict remaining members + bots (mirror the existing owner-leave close at lines 770-794), `h.presence.Clear(room.ID)`, and broadcast `system:room_closed_insolvent` (NEW, Task 5) to everyone who was still seated so they route to lobby.
  - [x] **Update `LeaveRoom` to use this helper** — its current rule promotes "first SEATED human" ([handler.go:757-763](../../server/internal/room/handler.go#L757)) with no presence/solvency consideration. 9.3 narrows it to present-AND-solvent for the owner-leave case (this is the documented supersede of v2's "no auto-transfer from an absent owner"). Preserve all existing `system:player_left` broadcasts and the `newOwnerId` payload field.
  - [x] **Edge case — restart caveat:** the presence registry is in-memory and empty after a server restart ([presence.go:16-18](../../server/internal/room/presence.go#L16)). Document inline that an owner-leave immediately after a restart (empty presence) closes the room rather than transferring. Acceptable per the best-effort presence contract; do not add persistence (that is an "Ask First" item in v2).

- [x] **Task 4 — Backend: StartMatch per-player ejection + refund-on-failure** (AC5)
  - [x] Replace the 9.2 whole-table fail-safe at [server/internal/room/handler.go:2189-2205](../../server/internal/room/handler.go#L2189) (the comment "Story 9.3 replaces this fail-safe with per-player ejection" marks the exact spot).
  - [x] **Identify all insolvent seats:** before charging, when `updatedRoom.CoinBuyIn > 0 && h.walletService != nil`, call `GetBalances(humanIDs)` and collect every seated human with `balance < CoinBuyIn`. (`ChargeStakes` only reports the *first* insolvent user and rolls back atomically — [wallet/repository.go:17-24](../../server/internal/wallet/repository.go#L17) — so a prefilter via `GetBalances` is needed to eject *all* of them.)
  - [x] **Eject each insolvent seat:** free the seat (`RemovePlayer` + `DecrementPlayerCount`), send per-user `system:insolvent_ejected` (NEW, Task 5) to that player; if an ejected player is the owner, run the Task 3 helper. Do **not** start the match (a freed seat means not-all-seated); revert room to `waiting`, `broadcastRoomUpdated`, and return `apperr.ErrInsufficientCoins` (409) to the owner so the existing `room.errors.insufficientCoinsStart` toast fires (no client change needed for the owner).
  - [x] **Authoritative charge:** if the prefilter found no insolvent seats, proceed to `ChargeStakes` as the atomic guard. If `ChargeStakes` *still* returns an insolvent user (TOCTOU race between the read and the lock), eject that user via the same path and abort the start — never start with an unpaid stake.
  - [x] **Refund-on-start-failure** ([deferred-work.md](deferred-work.md) item 1): if `ChargeStakes` succeeds but `matchStarter.StartMatch(...)` ([handler.go:2212](../../server/internal/room/handler.go#L2212)) returns an error, **refund** every charged human (`ApplySettlement` with `+CoinBuyIn` each), revert room to `waiting`, and broadcast `error:match_start_failed` (existing event, [events.go:216](../../server/internal/ws/events.go#L216)) so clients stay on the room page instead of navigating to a dead match. Currently the failure is only `slog`-logged and the room is stranded in `playing` with success still broadcast — that bug must be fixed here.
  - [x] Keep `h.presence.Clear(roomID)` ([handler.go:2148](../../server/internal/room/handler.go#L2148)) where it is — but note the ejection flow runs after presence is cleared, so the eject path must not depend on the just-cleared registry.

- [x] **Task 5 — Backend: two new WS events (both contract files, same commit)** (AC1, AC4, AC5)
  - [x] `system:room_closed_insolvent` — broadcast to all still-seated members when the room closes for lack of a present-and-solvent owner. Add const `SystemRoomClosedInsolvent` + payload struct `RoomClosedInsolventPayload { RoomID uint json:"roomId" }` in [server/internal/ws/events.go](../../server/internal/ws/events.go) (place under "Room events", next to `SystemRoomOwnerChanged`). Mirror in [client/src/shared/types/wsEvents.ts](../../client/src/shared/types/wsEvents.ts) as `SYSTEM_ROOM_CLOSED_INSOLVENT` + interface.
  - [x] `system:insolvent_ejected` — per-user push to a player ejected at start (AC5). Payload `InsolventEjectedPayload { RoomID uint json:"roomId"; BuyIn int json:"buyIn"; Balance int json:"balance" }` so the client modal shows exact numbers. Sent via `hub.SendToUser` (per-user, like `event:coin_settlement`), not a room broadcast.
  - [x] **Prefix decision:** use the `system:` prefix for both — every sibling room-lifecycle event is `system:` (`system:room_updated`, `system:room_kicked`, `system:player_returned`, `system:room_owner_changed`) and the architecture taxonomy classes non-game room/platform events as `system:` ([architecture.md#websocket-event-naming](../planning-artifacts/architecture.md)). The change-proposal's literal `event:room_closed_insolvent` was illustrative shorthand; `system:` is the consistent, dispatch-correct choice (frontend routes `system:` → `dispatchSystemEvent` → roomStore).
  - [x] Room/system events are **not** zod-validated (per [wsEvents.schemas.ts:22-26](../../client/src/shared/types/wsEvents.schemas.ts) coverage note) — follow the `PlayerReturnedPayload` / `RoomKickedPayload` precedent: TS interface + manual dispatch-guard, no zod schema, no golden JSON needed. Only add a golden + schema if you choose to (not required for `system:` events).

- [x] **Task 6 — Frontend: return-time rejection → lobby modal** (AC1)
  - [x] In `handleReturnToRoom` ([client/src/features/match/MatchPage.tsx:1140-1172](../../client/src/features/match/MatchPage.tsx#L1140)), add an `INSUFFICIENT_COINS` branch to the existing `err instanceof FetchError ? err.code` switch. On that code: clear match state, set the insolvency-ejection signal (Task 8), and `navigate("/lobby")` — do **not** keep them on the result overlay (unlike the other return errors, insolvency means the seat is gone server-side).
  - [x] The modal shows balance + buy-in. The client knows the room's `coinBuyIn` (it had it on the room/match) and the balance from `authStore.user.walletBalance` — compose the message with `formatCoins()` like the existing join path ([LobbyPage.tsx:144-150](../../client/src/features/lobby/LobbyPage.tsx#L144)).

- [x] **Task 7 — Frontend: WS dispatch for the two new events** (AC4, AC5)
  - [x] In `dispatchSystemEvent` ([client/src/shared/hooks/useWsDispatch.ts:443-646](../../client/src/shared/hooks/useWsDispatch.ts#L443)), add branches mirroring the `SYSTEM_ROOM_KICKED` precedent (lines 580-589):
    - `SYSTEM_INSOLVENT_EJECTED` (per-user): set the insolvency-ejection signal (roomId/buyIn/balance) in the store and let the always-mounted redirect (Task 8) navigate to lobby + show the modal. Gate on nothing — this is a direct per-user push (the player may be on the room page, not the match page).
    - `SYSTEM_ROOM_CLOSED_INSOLVENT`: set a "room closed (insolvent)" signal; recipients route to lobby with a "room closed because no one could afford to own it" modal/notice.
  - [x] Keep the `currentRoomId` gate semantics consistent with sibling room events where applicable.

- [x] **Task 8 — Frontend: ejection signal store field + always-mounted redirect + lobby modal** (AC1, AC4, AC5)
  - [x] Add a roomStore field (mirroring `kickedFromRoomId`, [roomStore.ts:137](../../client/src/shared/stores/roomStore.ts#L137)) to carry the ejection notice, e.g. `insolventEjection: { roomId: number; buyIn: number; balance: number; reason: "ejected" | "roomClosed" } | null`, with a setter + reset.
  - [x] Add an always-mounted redirect hook in the style of `useMatchStartRedirect` ([client/src/shared/hooks/useMatchStartRedirect.ts](../../client/src/shared/hooks/useMatchStartRedirect.ts), mounted in `WebSocketProvider`) that watches the field, navigates the user to `/lobby` from wherever they are, and leaves the signal set for the modal to consume.
  - [x] Show the modal on lobby arrival. There is **no existing arrival-modal pattern** — add a shadcn `Dialog` (max 480px, 32px padding, `surface-elevated`, backdrop-closes, per [ux#modal-and-overlay-patterns](../planning-artifacts/ux-design-specification.md)) hosted in `LobbyPage` that opens when the signal is set and clears it on close. Use calm, non-panic copy with a clear next action (per [ux#feedback-patterns](../planning-artifacts/ux-design-specification.md) — "no dead ends").
  - [x] The return-time HTTP-409 path (Task 6) and the WS-event paths (Task 7) both feed this single signal so the modal is the one consumer.

- [x] **Task 9 — i18n: new strings in all four locales** (AC1, AC4, AC5)
  - [x] Add keys to `en.json`, `sr.json`, `mk.json`, `hr.json` ([client/src/shared/i18n/](../../client/src/shared/i18n/)) — the parity test fails if any locale is missing one:
    - Insolvency-eject modal title + body with `{{balance}}` / `{{buyIn}}` interpolation (en body: *"You don't have enough coins to rejoin this room. Balance: {{balance}}. Buy-in: {{buyIn}}."*).
    - Room-closed-insolvent notice (en: *"This room closed — no remaining player could afford to host it."*).
    - A primary action button (e.g. "Back to lobby" / reuse an existing OK/close key if one fits).
  - [x] **Terminology (locked, [[beljot-i18n-coin-terms]]):** buy-in noun = en `Buy-in`, sr/hr `Ulog`, mk `Влог`. Coin amounts render as icon + grouped number via `formatCoins()` — **no trailing "coins" word** on chips/badges (the modal sentence body may read naturally). **Never use `—` (em dash) in mk/sr/hr** ([[no-emdash-in-mk-sr-hr]]); English only.

- [x] **Task 10 — Tests** (all ACs)
  - [x] Backend (`server/internal/room/`, table-driven + `testify`, per-test tx rollback for any DB-touching test):
    - `return_to_room_test.go` / new test file: return with sufficient balance (unchanged), return insolvent (409 + seat freed + broadcasts), `coin_buy_in == 0` always passes (no seat free), insolvent owner returns → ownership transfers to present-and-solvent / room closes when none.
    - `coin_handler_test.go`: StartMatch with one insolvent seat → that seat ejected + `system:insolvent_ejected` + room reverts to waiting + 409 to owner; StartMatch where `ChargeStakes` succeeds but `matchStarter.StartMatch` fails → stakes refunded + room reverted + `error:match_start_failed`; insolvent owner at start → transfer/close.
    - Ownership helper: present-and-solvent selection order, restart-empty-presence → close.
  - [x] WS contract: golden/contract test only if you added schemas; otherwise ensure `events_contract_test.go` and the TS contract test still pass (new `system:` events without goldens are fine — match the `player_returned` precedent).
  - [x] Frontend (Vitest, `data-testid` selectors, present-tense `it(...)`): `MatchPage` return-insolvent dispatches the signal + navigates; `useWsDispatch` routes both new events; the lobby modal renders with composed balance/buy-in; i18n parity test passes.
  - [x] `make lint` + `make test` green (both stacks). Local fallback if `golangci-lint` is CI-only: `gofmt -l` + `go vet ./...`.

## Dev Notes

### Developer Context (what 9.3 builds on — do not re-derive)

- **Economy is already built (9.1 + 9.2).** Wallet mutations, the `coin_buy_in` column, stake charge, and pot settlement all exist. 9.3 adds **no migration** and **no new external dependency** — it wires existing wallet methods into the room lifecycle. [Source: 9-1, 9-2 File Lists]
- **The Return-to-Room flow is already shipped** (v1 reopen + v2 presence). 9.3 *gates* it; it does not rebuild it. The `room_players` row surviving a match is the existing "room persistence between matches" mechanism — returners reclaim their original seat for free. [Source: [spec-return-to-room-after-match.md](spec-return-to-room-after-match.md), [spec-return-to-room-presence-v2.md](spec-return-to-room-presence-v2.md)]
- **The StartMatch charge is the single money authority** ([handler.go:2189-2205](../../server/internal/room/handler.go#L2189)). 9.2 left a comment there: *"Story 9.3 replaces this fail-safe with per-player ejection."* That is literally Task 4. The return-time check (AC1) is a **gate, not a money movement** — use the plain `GetBalance` read; the authoritative re-validation stays inside `ChargeStakes`' `FOR UPDATE` lock. *"No redundant third UI check."* [Source: [sprint-change-proposal-2026-06-18.md](../planning-artifacts/sprint-change-proposal-2026-06-18.md#93--insolvency-ejection--room-persistence)]

### Technical Requirements & Guardrails (mistakes to NOT make)

- **`ChargeStakes` is all-or-nothing and reports only the FIRST insolvent user**, rolling the whole tx back ([wallet/repository.go:17-24](../../server/internal/wallet/repository.go#L17)). To eject *all* insolvent seats you must prefilter with `GetBalances` (a batch read), then use `ChargeStakes` as the final atomic guard. Do **not** assume one `ChargeStakes` call surfaces every insolvent player.
- **Bots never participate in money or ownership.** Bot seats live in the `bots` slice and carry `UserID == 0`; `room_players` are humans only. Never balance-check, charge, eject, count toward ownership, or push WS to a UserID-0 seat. [Source: [project-context.md] + 9.2 §8]
- **Go zero-value trap:** never use JS-truthiness on numeric/boolean wire fields. Use `room.CoinBuyIn == 0`, `balance < buyIn`, etc. — `0` is a real value, not "absent." [Source: project-context.md Language Rules]
- **Broadcasts are best-effort and post-commit** — never run a `BroadcastTo*` inside a `RunInTransaction` block. Collect what to broadcast inside the tx, fan out after commit (mirror `ReturnToRoom`'s `clearedSeats`/`reopened` pattern, [handler.go:891-959](../../server/internal/room/handler.go#L891)).
- **WS contract is single-source-of-truth:** every new event const + payload goes in **both** `events.go` and `wsEvents.ts` in the **same commit** — no exceptions. Use a typed payload struct, `camelCase` JSON tags, all exported fields. [Source: project-context.md Cross-Language Rules; architecture.md]
- **Errors via `apperr` only**, checked with `errors.Is`, wrapped with `%w`. Reuse `apperr.ErrInsufficientCoins` (409, code `INSUFFICIENT_COINS`, [apperr/errors.go:94](../../server/internal/apperr/errors.go#L94)) for both the return-time rejection and the start-time owner response — **do not invent a new error code** for insolvency. [Source: project-context.md; 9.2 §2]
- **`StartMatch` stranding bug is in-scope:** today, a `matchStarter.StartMatch` error after a successful charge only `slog.Error`s and still broadcasts success, leaving the room in `playing` and coins destroyed ([handler.go:2212-2214](../../server/internal/room/handler.go#L2212)). Fix it (refund + revert + `error:match_start_failed`) — do not leave it. [Source: [deferred-work.md](deferred-work.md) item 1]
- **Owner UX at start is already handled.** The owner's `RoomPage` start handler already shows `room.errors.insufficientCoinsStart` on a 409 ([RoomPage.tsx:636-640](../../client/src/features/room/RoomPage.tsx#L636), with a comment pointing at 9.3). Keep returning 409 to the owner — no new owner-side client code; 9.3 only adds the *ejected player's* notification + the freed seat.

### Architecture Compliance

- **Room domain shape:** `model.go`, `repository.go`, `gorm_repo.go`, `handler.go` (no separate `service.go` in this package today). New seat-mutation methods go on the `RoomRepository` interface ([repository.go](../../server/internal/room/repository.go)) + `gorm_repo.go` impl; handlers call the interface, never GORM directly. [Source: project-context.md Echo/Backend Rules]
- **WS event taxonomy:** `system:` = non-game platform/room events (the correct prefix for both new events). `error:` = server→client errors. The return-time rejection is an **HTTP** 409 (the `/return` endpoint is REST), not a WS error. [Source: architecture.md#websocket-event-naming]
- **Server-authoritative:** all balance/affordability logic is server-side; the client modal is presentation only. [Source: project-context.md Security Rules; architecture.md#validation]
- **No GORM AutoMigrate** — but 9.3 needs no schema change, so no migration file. The next sequential migration number (`000011`) belongs to 9.6 private-rooms (`password_hash`), not 9.3. [Source: architecture.md; sprint-change-proposal-2026-06-18.md]
- **Echo error middleware** maps `apperr` → HTTP `{ "error": { "code", "message" } }`; the frontend `axiosClient` interceptor surfaces it as `FetchError { status, code }`. [Source: architecture.md#error-handling; 9.2 frontend map §3]

### Library / Framework Requirements

- No new libraries. Backend: Go + Echo v4 (do **not** upgrade), GORM, `nhooyr.io/websocket` (import path `nhooyr.io/websocket`), `testify`, `slog`. Frontend: React 19, Zustand, react-i18next, Vitest, shadcn/ui `Dialog` (already vendored in `shared/components/ui/`). [Source: project-context.md Technology Stack]
- Coin formatting: `formatCoins()` ([client/src/shared/lib/formatCoins.ts](../../client/src/shared/lib/formatCoins.ts)) for all displayed amounts. Balance lives on `authStore.user.walletBalance`. [Source: 9.2 §3, §8]

### File Structure Requirements

**Backend (modify):**
- [server/internal/room/handler.go](../../server/internal/room/handler.go) — `ReturnToRoom` gate (Task 1), `WalletService` interface widen (Task 2), `transferOwnershipOrClose` helper + `LeaveRoom` update (Task 3), `StartMatch` ejection + refund (Task 4).
- [server/internal/room/repository.go](../../server/internal/room/repository.go) + [gorm_repo.go](../../server/internal/room/gorm_repo.go) — only if new seat/query methods are needed (existing `RemovePlayer` / `ClearPlayerSeat` / `DecrementPlayerCount` / `FindByIDForUpdate` likely suffice).
- [server/internal/ws/events.go](../../server/internal/ws/events.go) — two new `system:` consts + payload structs (Task 5).
- `server/cmd/api/main.go` — only if the room handler's wallet wiring needs the widened interface (it already receives `wallet.NewService(walletRepo)`).

**Backend (tests):** `server/internal/room/{return_to_room_test.go, coin_handler_test.go}` (+ a new test file if cleaner); `server/internal/ws/events_contract_test.go` (only if goldens added).

**Frontend (modify):**
- [client/src/shared/types/wsEvents.ts](../../client/src/shared/types/wsEvents.ts) — two new consts + interfaces.
- [client/src/shared/hooks/useWsDispatch.ts](../../client/src/shared/hooks/useWsDispatch.ts) — two new dispatch branches.
- [client/src/shared/stores/roomStore.ts](../../client/src/shared/stores/roomStore.ts) — `insolventEjection` field + setter/reset.
- [client/src/features/match/MatchPage.tsx](../../client/src/features/match/MatchPage.tsx) — `handleReturnToRoom` 409 branch.
- New: `client/src/shared/hooks/useInsolventEjectRedirect.ts` (always-mounted, register in [WebSocketProvider.tsx](../../client/src/shared/providers/WebSocketProvider.tsx)).
- [client/src/features/lobby/LobbyPage.tsx](../../client/src/features/lobby/LobbyPage.tsx) — host the arrival modal (new `Dialog`).
- [client/src/shared/i18n/{en,sr,mk,hr}.json](../../client/src/shared/i18n/) — new strings.

**Frontend (tests):** co-located `.test.tsx`/`.test.ts` for each modified unit; i18n parity test.

### Testing Requirements

- **Backend:** Go `testing` + `testify`; table-driven; tests create their own data (no seed); any PostgreSQL-touching test uses a per-test transaction with rollback. WS handler tests (if any) use `httptest.Server` + a real WS client, not mocks. [Source: project-context.md Testing Rules]
- **Money tests must assert no coin is created or destroyed** across eject/refund paths (sum of deltas conserved except the deliberate all-bot-winner sink, which 9.3 does not touch).
- **Frontend:** Vitest; `data-testid` selectors only; present-tense `it(...)`; test components render correctly given store/props, not game logic. [Source: project-context.md Testing Rules]
- **Definition of Done (hard gate):** server handler + repo + tests; new `apperr` (none needed); WS events in **both** contract files; frontend components + co-located tests; API client unchanged (no new endpoint — `/return` and `/start` already exist); i18n in **all** four locales; `make lint` + `make test` pass. [Source: project-context.md Feature-Complete Checklist]

### Previous Story Intelligence (9.1 / 9.2)

- **Settlement is the single source of pot math** — `match.computeSettlement` ([server/internal/match/settlement.go](../../server/internal/match/settlement.go)); 9.3 does not re-derive pot math. The refund (Task 4) is a simple `+CoinBuyIn` per charged human via `ApplySettlement`, not a settlement.
- **9.2 charge wiring** lives entirely in `RoomHandler.StartMatch`; the `match.Manager` only receives `coinBuyIn` as the last `StartMatch` param and settles at end. 9.3's start-time work is **all in the room handler** — the match manager is untouched. [Source: 9-2 §4]
- **Four join entry points needed `INSUFFICIENT_COINS` handling in 9.2** (`JoinByCodeTile`, `LobbyPage` ×2, `RoomPage`). For 9.3, sweep `MatchPage.handleReturnToRoom` is the single new HTTP entry point; do not miss the WS-event paths. [Source: 9-2 §8]
- **Expect to update ~10-13 existing fixtures** if you add a required field anywhere (both 9.1 and 9.2 hit this). 9.3 adds no required model field, so fixture churn should be limited to new store-field defaults in roomStore tests. [Source: 9-2 §1]
- **Locked i18n terminology** carried from 9.2: see [[beljot-i18n-coin-terms]]. No em dash in mk/sr/hr ([[no-emdash-in-mk-sr-hr]]).

### Git Intelligence

Recent commits show the 9.2 cadence: feature commit → code-review-findings commit → QA-fixes commit (`5471188` → `2ef3ad4` → `62723da`). Expect a 3-layer adversarial code-review after dev. **Branch: continue on the existing `feat/9-2-room-buy-in-settlement` — 9.2 and 9.3 ship together as a single feature (one branch, one PR).** This is a deliberate exception to the one-story-one-branch convention: 9.3 closes out 9.2's deferred items (StartMatch hardening) and gates the same buy-in flow, so they integrate as one coherent change. Commit-message scope stays `feat(wallet)`/`feat(room)` per convention. [Source: user decision 2026-06-19; git log; project-context.md Commit Messages]

### Latest Tech Information

No external library research required — every dependency (Go/Echo v4/GORM/websocket, React 19/Zustand/react-i18next/Vitest/shadcn) is already pinned and in active use. No version migration in scope. The only "new" surface is two `system:` WS events and one shadcn `Dialog` usage, both following existing in-repo patterns.

### Project Structure Notes

- **Package naming:** architecture.md predates the rename and calls it `internal/lobby/`; the actual package is `internal/room/`. Use `internal/room/`. [Source: architecture analysis; sprint-change-proposal-2026-06-11.md]
- **`LeaveRoom` vs lobby-disconnect divergence:** `LeaveRoom` promotes "first seated human"; `lobby_disconnect.go` promotes `remainingPlayers[0]` with no seated check ([lobby_disconnect.go:143-145](../../server/internal/room/lobby_disconnect.go#L143)). 9.3 changes `LeaveRoom`'s rule to present-and-solvent. **Do not** silently change the lobby-disconnect path's rule unless an AC requires it — note any divergence you leave behind. (Related open item D147 — incomplete lobby-disconnect broadcast payload — is **not** in 9.3 scope.)
- **Prefix tension is resolved in this story** (Task 5) toward `system:` with rationale; if the dev disagrees, change both contract files and the dispatch routing together and document why.

### References

- [Source: _bmad-output/planning-artifacts/epics.md#story-93-insolvency-ejection--room-persistence-between-matches] — ACs.
- [Source: _bmad-output/planning-artifacts/sprint-change-proposal-2026-06-18.md#93--insolvency-ejection--room-persistence] — authoritative re-scope, ownership rule, supersede statement.
- [Source: _bmad-output/implementation-artifacts/spec-return-to-room-presence-v2.md] — presence model; the "no auto-transfer from an absent owner" rule this story supersedes.
- [Source: _bmad-output/implementation-artifacts/spec-return-to-room-after-match.md] — return-to-room v1 contract; result-dialog UX.
- [Source: _bmad-output/implementation-artifacts/deferred-work.md (items 1 & 2, 2026-06-19)] — StartMatch refund-on-failure + per-player ejection deferred to 9.3.
- [Source: server/internal/room/handler.go:863 (ReturnToRoom), :741-795 (LeaveRoom owner transfer), :2189-2205 (StartMatch charge fail-safe)] — code being modified.
- [Source: server/internal/room/presence.go] — in-memory presence registry (Add/Remove/Clear/Present).
- [Source: server/internal/wallet/repository.go] — `ChargeStakes` / `ApplySettlement` / `GetBalance` / `GetBalances` semantics.
- [Source: server/internal/ws/events.go:155, :218-244] — OutcomeReason non-goal note; room-event consts to extend.
- [Source: client/src/features/match/MatchPage.tsx:1140-1172, client/src/shared/hooks/useWsDispatch.ts:443-646, client/src/shared/stores/roomStore.ts, client/src/shared/hooks/useMatchStartRedirect.ts] — frontend hooks 9.3 extends.
- [Source: _bmad-output/project-context.md] — all critical implementation rules.

## Dev Agent Record

### Agent Model Used

claude-opus-4-8 (Opus 4.8, 1M context) — BMad dev-story workflow

### Debug Log References

- `go test ./...` (server) — all packages pass; `go vet ./...` clean.
- `npx vitest run` (client) — 857 tests across 83 files pass.
- `npx tsc -p tsconfig.build.json --noEmit` — clean (production typecheck).
- `npx eslint` + `npx prettier --check` on changed files — clean.
- `gofmt -l` on changed Go files — clean. (Pre-existing gofmt-version artifact in the untouched `internal/room/lobby_disconnect_test.go` left as-is — out of scope.)

### Completion Notes List

**All five ACs implemented and tested.**

- **AC1 (return-time gate):** `ReturnToRoom` reads the caller's balance vs `coin_buy_in` only for staked rooms with a wired wallet; an insolvent returner is routed through `ejectInsolventReturner` — seat freed in a row-locked tx, `system:player_left` + `broadcastRoomUpdated` fan out post-commit, 409 `INSUFFICIENT_COINS` returned.
- **AC2 (zero buy-in never bars):** the gate is skipped entirely when `CoinBuyIn == 0`; the reopen/presence/broadcast path is byte-identical to before. Covered by `TestReturnToRoom_ZeroBuyIn_NeverBars`.
- **AC3 (held-as-seated):** the gate fires only at the moment of an actual return; no background sweep or reservation timeout was added — a player who hasn't acted keeps their `room_players` row.
- **AC4 (ownership transfer / close):** shared `transferOwnershipOrClose` helper (used by `LeaveRoom`, return-eject, and start-eject) picks the first seated human in seat order satisfying a caller-supplied eligibility predicate, else closes the room and emits `system:room_closed_insolvent`. `LeaveRoom` now uses present-AND-solvent eligibility (the documented supersede of v2's "no auto-transfer from an absent owner"); empty presence (post-restart) closes the room, per the best-effort presence contract.
- **AC5 (StartMatch authoritative guard):** a `GetBalances` prefilter ejects EVERY insolvent seat (since `ChargeStakes` only reports the first), `ChargeStakes` remains the atomic guard with a TOCTOU re-eject, and a post-charge `matchStarter.StartMatch` failure now **refunds** every charged human via `ApplySettlement`, reverts the room to `waiting`, broadcasts `error:match_start_failed`, and returns an error (fixing the prior bug where success was broadcast and coins were destroyed).

**Deliberate, documented refinements of the story design:**

1. **`system:insolvent_ejected` is sent on the return-eject path too** (not only at start). Rationale: the client provably lacks the room's `coinBuyIn` once it leaves RoomPage (`roomStore.reset()` fires on unmount), so the modal's exact balance/buy-in numbers must come from the server. This unifies both ejection paths through one event → one `roomStore.insolventEjection` signal → one lobby modal. `MatchPage`'s 409 handler therefore only clears match state and navigates to `/lobby`; the WS event drives the modal.
2. **`roomStore.reset()` preserves `insolventEjection`** so the signal survives RoomPage's unmount-time reset while navigating the ejected player to the lobby (the modal is the field's sole consumer and clears it on close).
3. **WS prefix `system:` for both new events**, per the architecture taxonomy and sibling room-lifecycle events (resolves the change-proposal's illustrative `event:` shorthand). No zod schema / golden needed (matches the `player_returned` precedent).

No new migration, no new external dependency, no new `apperr` (reused `ErrInsufficientCoins`). Bots (UserID 0) are never balance-checked, charged, ejected, or counted toward ownership.

### File List

**Backend (modified):**

- `server/internal/room/handler.go` — `WalletService` interface widened (`GetBalances` + `ApplySettlement`); `transferOwnershipOrClose` helper; `ejectInsolventReturner` + return-time gate in `ReturnToRoom`; `ejectInsolventAtStart` + StartMatch prefilter/charge/refund rewrite; `LeaveRoom` owner branch routed through the helper (present-AND-solvent).
- `server/internal/ws/events.go` — `SystemRoomClosedInsolvent` + `SystemInsolventEjected` consts and `RoomClosedInsolventPayload` / `InsolventEjectedPayload` structs.

**Backend (tests):**

- `server/internal/room/coin_handler_test.go` — `stubWallet` widened (`GetBalances`/`ApplySettlement` + `balances`/`settle` fields); `setupCoinTestBC` + `broadcastsOfType` helpers; rewrote the insolvent-start test into per-player ejection scenarios + owner-transfer/close + refund-on-failure + TOCTOU.
- `server/internal/room/return_to_room_test.go` — return-time gate tests (sufficient/insolvent/zero-buy-in/owner-transfer/owner-no-heir-close) incl. `system:insolvent_ejected` assertion.
- `server/internal/room/handler_test.go` — `registerRoomRoutes`, `setupTestWithPresence`, `setupTestWithBroadcastAndPresence`; affected owner-transfer tests now populate presence.

**Frontend (modified):**

- `client/src/shared/types/wsEvents.ts` — `SYSTEM_ROOM_CLOSED_INSOLVENT` / `SYSTEM_INSOLVENT_EJECTED` consts + interfaces.
- `client/src/shared/hooks/useWsDispatch.ts` — dispatch branches for both new events.
- `client/src/shared/stores/roomStore.ts` — `InsolventEjection` type, `insolventEjection` field + setter; `reset()` preserves it.
- `client/src/features/match/MatchPage.tsx` — `handleReturnToRoom` `INSUFFICIENT_COINS` branch.
- `client/src/shared/providers/WebSocketProvider.tsx` — mounts `useInsolventEjectRedirect`.
- `client/src/features/lobby/LobbyPage.tsx` — hosts `InsolventEjectionModal`.
- `client/src/shared/i18n/{en,sr,mk,hr}.json` — `lobby.insolventEjection` strings (no em dash in mk/sr/hr).

**Frontend (new):**

- `client/src/shared/hooks/useInsolventEjectRedirect.ts` — always-mounted redirect to `/lobby` on the ejection signal.
- `client/src/features/lobby/components/InsolventEjectionModal.tsx` — lobby arrival modal (single consumer of the signal).

**Frontend (tests):**

- `client/src/shared/hooks/useWsDispatch.test.ts` — both new events.
- `client/src/shared/stores/roomStore.test.ts` — `setInsolventEjection` + reset-preserves behavior.
- `client/src/features/match/MatchPage.test.tsx` — return-insolvent routes to lobby (no overlay toast).
- `client/src/features/lobby/components/InsolventEjectionModal.test.tsx` (new) — modal render + clear-on-action.

### Change Log

- 2026-06-19: Implemented Story 9.3 (insolvency ejection & room persistence between matches) — return-time affordability gate, present-AND-solvent ownership transfer/close, StartMatch per-player ejection + refund-on-failure, two new `system:` WS events, lobby insolvency modal, and i18n in all four locales. All ACs satisfied; full backend + frontend suites green.

## Review Findings

3-layer adversarial code review (Blind Hunter + Edge Case Hunter + Acceptance Auditor), 2026-06-19. Scope: uncommitted 9.3 changes + folded-in 9.2 deferred work. AC coverage confirmed (AC1–AC5 all satisfied); the issues below are robustness/consistency gaps in the new ejection paths. 3 layers ran clean; all findings verified against source before classification.

### Decision Needed (resolved 2026-06-19)

- [x] [Review][Decision] **Refund-on-start-failure can silently destroy coins if `ApplySettlement` errors** ([server/internal/room/handler.go:2612](../../server/internal/room/handler.go#L2612)) — **Resolved: Accept + log follow-up.** Best-effort-with-log is kept as the documented interim, consistent with the rest of the wallet layer (no outbox/saga anywhere); a true fix needs a transactional outbox, which is cross-cutting and beyond 9.3. → moved to **Deferred**.
- [x] [Review][Decision] **Return-time insolvency modal depends entirely on the best-effort `system:insolvent_ejected` WS push** ([client/src/features/match/MatchPage.tsx:1162](../../client/src/features/match/MatchPage.tsx#L1162)) — **Resolved: Thread `coinBuyIn` + local fallback.** → moved to **Patches** (P4).

### Patches

- [x] [Review][Patch] **[HIGH] Charge-time TOCTOU owner-eject wrongly closes a room that has solvent heirs** — ✅ applied 2026-06-19 (full prefilter `balances` map now passed; regression test `TestStartMatch_ChargeTimeInsolventOwnerTransfersOwnership`) [server/internal/room/handler.go:2577] — On the `ChargeStakes` TOCTOU branch, `latest, _ := GetBalances([]uint{insolventID})` passes a single-entry balance map to `ejectInsolventAtStart`. When the insolvent user is the owner, `transferOwnershipOrClose` evaluates each heir via `startEligible(heir) = balances[heir] >= buyIn`, but `latest` has no heir entries → every heir resolves to `0 >= buyIn` (false) → no eligible heir → the room is **closed and solvent players evicted** instead of transferring ownership. Fix: pass the full prefilter `balances` map (in scope at line 2546), merging in the fresh insolvent balance for an accurate modal number. (Also resolves the swallowed-`GetBalances`-error / "Balance: 0" modal imprecision.)
- [x] [Review][Patch] **[MEDIUM] Start-eject omits `system:player_left` → remaining players' RoomPage shows a stale seat + stale owner** — ✅ applied 2026-06-19 (`ejectInsolventAtStart` now captures `newOwnerID` + broadcasts `player_left` per freed seat; regression test `TestStartMatch_InsolventSeatEjected_NotifiesRemainingPlayers`) [server/internal/room/handler.go:955,978] — `ejectInsolventAtStart` discards the helper's `newOwnerID` (`_, didClose, ...`) and broadcasts only `system:room_updated` + per-user `system:insolvent_ejected`. But on the client `system:room_updated` returns early ([useWsDispatch.ts:451](../../client/src/shared/hooks/useWsDispatch.ts#L451)) and does NOT touch `roomStore.players`; only `system:player_left` frees the seat via `removePlayer(userId, count, newOwnerId)` and applies the owner change. So after a start-eject, the owner and remaining seated players see the ejected seat still occupied and a stale host badge until refresh. Fix: capture `newOwnerID` and broadcast `system:player_left` per freed seat (embedding `newOwnerId` when transferred), mirroring `ejectInsolventReturner` ([handler.go:852-872](../../server/internal/room/handler.go#L852)).
- [x] [Review][Patch] **[MEDIUM] `ejectInsolventAtStart` tx failure strands the room in "playing"** — ✅ applied 2026-06-19 (best-effort out-of-tx `UpdateStatus(roomID,"waiting")` added in the error fall-through, mirroring the sibling charge-failure branches) [server/internal/room/handler.go:971] — the revert to "waiting" lives INSIDE the eject tx; if that tx rolls back on a DB error the room stays "playing" (set by the already-committed outer tx). The function only `slog.Error`s and falls through — unlike the charge-read-failure ([:2549](../../server/internal/room/handler.go#L2549)) and charge-failure ([:2581](../../server/internal/room/handler.go#L2581)) branches, which both have an out-of-tx `UpdateStatus(roomID, "waiting")` fallback. Result: a bricked room in "playing" with no live session (next start fails `ErrMatchNotStartable`). Fix: add a best-effort out-of-tx revert in the error fall-through, mirroring the sibling branches.
- [x] [Review][Patch] **[LOW→robustness] (from Decision) Thread `coinBuyIn` into match state + local 409 fallback for the insolvency modal** — ✅ applied 2026-06-19 (`roomBuyInRef` captured from the mount-time `getRoom`; 409 handler seeds `insolventEjection` from `walletBalance`+buyIn only when the WS signal hasn't already landed, so the WS event stays canonical) [client/src/features/match/MatchPage.tsx:1162] [client/src/shared/stores/roomStore.ts] [client/src/shared/stores/authStore.ts] — `MatchPage` currently relies solely on the best-effort `system:insolvent_ejected` WS push to populate the lobby modal; a dropped frame leaves the player on the lobby with no explanation. Make `coinBuyIn` available in match/game state (it is not today) and have the 409 `INSUFFICIENT_COINS` branch set `roomStore.insolventEjection` locally (balance from `authStore.user.walletBalance`, buyIn from match state) as a belt-and-suspenders fallback. The WS event still drives the canonical numbers; the local set guarantees the modal shows even if the frame is lost.

### Deferred

- [x] [Review][Defer] **`transferOwnershipOrClose` close path doesn't `DecrementPlayerCount`** [server/internal/room/handler.go:742-760] — deferred, pre-existing — the closed room's `PlayerCount` is left stale (non-zero) while `room_players` is emptied; the legacy inline close had the same omission. The room is `completed` so count is moot and lobby grids drop completed rooms.
- [x] [Review][Defer] **`ejectInsolventReturner` `player_left` broadcast swallows `FindPlayersByRoomID` error with no log** [server/internal/room/handler.go:852-854] — deferred, pre-existing — broadcasts are best-effort by design; on a read error remaining members silently miss the freed-seat update (no log, unlike the start path). Low-value cleanup.
- [x] [Review][Defer] **(from Decision) Refund-on-start-failure `ApplySettlement` error only logged → coins can be destroyed** [server/internal/room/handler.go:2612] — deferred per decision 2026-06-19 — accept best-effort-with-log as the documented interim; consistent with the rest of the wallet layer (no outbox/saga). A durable fix needs a transactional outbox / reconciliation, which is cross-cutting and beyond 9.3's scope.
