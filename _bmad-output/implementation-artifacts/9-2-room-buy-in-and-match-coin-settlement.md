---
baseline_commit: dc66245
---

# Story 9.2: Room Buy-In & Match Coin Settlement

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a player,
I want to stake coins to play a match and receive winnings per the pot rules,
so that each game carries real economic meaning.

## Acceptance Criteria

> Source: [epics.md#Story 9.2](../planning-artifacts/epics.md) (lines 1763-1821), settlement rewritten by [sprint-change-proposal-2026-06-18.md](../planning-artifacts/sprint-change-proposal-2026-06-18.md) §"9.2 — Settlement (pot rules)". Builds directly on the **done** [Story 9.1](9-1-coin-wallet-foundation.md) wallet foundation. **Two epic ACs are reconciled against shipped code + Story 9.3 — see the ⚠️ Design Decisions section before coding.** The reconciliations are folded into the AC text below.

1. **Room buy-in config field** — Given a room owner is configuring a new room, when they view the create-room modal, then a "Coin buy-in" field is available (integer, **min 0, no maximum** — owner freedom), the default is **500**, and the room record persists `coin_buy_in` on the `rooms` table. Quick-play rooms are created with `coin_buy_in = 0` in this story (the bracketed buy-in is Story 9.4's job — **do not** apply a flat 500 to quick-play, or low-balance players would be matched then ejected at start with no bracketing safety net).

2. **Join affordability check (HTTP, check-only — NOT a deduction)** — Given a player attempts to join a room with a coin buy-in of `S` via `POST /api/v1/rooms/:id/join`, when the request is processed, then the server reads the player's `wallet_balance`, and if `balance < S` the join is rejected with the HTTP domain error `INSUFFICIENT_COINS` (the client shows a modal: "You need [S] coins to join this room — your balance: [balance]"). If `balance ≥ S`, the player is seated. **No coins are deducted at join.** (Reconciliation: the epic's "S is deducted and held as stake" is superseded — money moves only at match start; see Design Decision A.)

3. **Leave before match — no refund needed** — Given a player leaves a room before the match starts (`POST /api/v1/rooms/:id/leave`), when processed, then the existing leave behavior is unchanged and **no coin refund occurs**, because nothing was deducted at join (Design Decision A). The epic's "stake refunded in full" AC is satisfied vacuously.

4. **Stakes charged atomically at match start** — Given the owner starts the match (`POST /api/v1/rooms/:id/start`) for a room with `coin_buy_in = S > 0`, when the start runs, then each **human** seat is debited `S` from `wallet_balance` in a **single transaction** that re-validates `balance ≥ S` atomically per row, and **bots are never charged** (UserID 0 seats are skipped). If any human seat is insolvent at that instant, the entire charge **rolls back**, the match does **not** start, the room status is reverted to `waiting`, and the owner receives the `INSUFFICIENT_COINS` HTTP error. (The polished per-player ejection flow for insolvent-at-start is Story 9.3; 9.2 must at minimum fail safe — never start a match with an unpaid stake.)

5. **Pot formation** — Given stakes are collected for a match, when the pot is formed, then the pot equals the sum of the stakes of the **human** players only (`pot = (number of human seats) × S`); a seated bot contributes 0 and is never charged or paid.

6. **Normal win/loss settlement** — Given a match ends with a normal win/loss outcome, when settlement runs, then each human on the **losing** team forfeits their stake (their `−S` stays in the pot), and the pot is split **equally among the human players on the winning team** (bots on the winning team receive nothing). In an all-human 2v2 this is the classic split: `pot = 4S`, each winner gains `2S` (net `+S`), each loser net `−S`. All human players receive `event:coin_settlement` with their own delta and new balance; the UI shows a settlement toast and updates the header coin balance.

7. **Surrender settlement** — Given a match ends by surrender, when settlement runs, then the surrendering team is treated as the losing team and settlement is **identical to a normal loss** (the surrender path already routes through the same finalizer — see Dev Notes).

8. **Abandonment settlement** — Given a match ends by abandonment (a player fails to reconnect within the window / is timed out to match end), when settlement runs, then the **entire abandoning team is treated as the losing team** — both members forfeit their stake (each net `−S`), with **no teammate refund**. The winning team is the **non-abandoning** team (`1 − TeamForSeat(abandonedSeat)`), **not** the `WinnerTeam` recorded on the abandoned match row (which is hardcoded `0`). Settlement is otherwise identical to a normal loss.

9. **No human winner = coin sink** — Given the winning team contains no human player (e.g. Human+Bot vs Bot+Bot and the bots win), when settlement runs, then there is no human to receive the pot, so the losing humans' forfeited stakes are **removed from circulation** (a house coin sink — nobody is credited). The match record still records each human's `−S` delta.

10. **Post-match cleanup unchanged** — Given a match ends, when post-match cleanup runs, then the room status returns to `waiting` (existing behavior) and all remaining seated players retain their seat for a possible next match. The return-to-room affordability gate is Story 9.3 — **not** in this story's scope.

11. **Settlement atomicity** — Given the coin-settlement code, when it executes, then the wallet mutations (winner credits) run as a **single database transaction**; if any wallet update fails, the entire settlement is rolled back and the error is logged via `slog`. The per-seat coin deltas + the applied `coin_buy_in` are recorded on the `matches` row.

12. **Match record persists economy fields** — Given the schema, when inspected, then `matches` has `coin_buy_in` (integer, default 0) and `player1_coin_delta`…`player4_coin_delta` (integer, default 0; bot seats record 0); `rooms` has `coin_buy_in` (integer, NOT NULL, default 0, `CHECK (coin_buy_in >= 0)`).

13. **Wallet primitives** — Given the `wallet` package, when extended, then it exposes transactional, row-locked **charge** (debit, balance-guarded) and **credit** primitives that bots-skip correctly and respect the `wallet_balance >= 0` constraint, reusing the Story 9.1 `FOR UPDATE` pattern. Feature-complete checklist applies (handler/repo/service/tests, i18n in all 4 locales, both WS contract files for `event:coin_settlement`).

---

## ⚠️ Design Decisions — read before coding

Two epic ACs conflict with **shipped code** and with **Story 9.3** (rewritten in the same 2026-06-18 proposal). Both are resolved here. These are the single most important things to get right; do not silently follow the stale epic wording.

### A. Stake is deducted at **match start**, not at join. Join is a check only.

The epic 9.2 text says coins are "deducted and held as stake" on join and "refunded in full" on leave (an **escrow-at-join** model). But:

- **Story 9.3 + the 2026-06-18 change proposal** state the authoritative money guard is the **atomic balance re-validation inside `StartMatch`'s stake deduction** that "never trusts the prior return-time check." That is incompatible with escrow-at-join (if coins were already escrowed at join, there'd be nothing to "actually deduct at match start").
- **Story 9.1's own Dev Notes** (the same planning pass) describe 9.2 as "deducts a stake **at match start**, splits the pot."
- A player can be in **only one room at a time** (`apperr.ErrAlreadyInRoom`, enforced in `JoinRoom`/`CreateRoom` via `FindPlayerRoom`), so escrow-at-join solves a double-commit problem that **does not exist** here.
- Escrow-at-join would add a large refund surface (leave, owner-eviction, room-close, and the 9.3 return-to-room re-seat), all of which `StartMatch`-deduction avoids.

**Decision:** Join (`JoinRoom`) only **checks** `balance ≥ coin_buy_in` and rejects with `INSUFFICIENT_COINS` if short — it does **not** move coins. `StartMatch` is the single, race-safe, authoritative charge point: it debits every human seat `S` in one transaction (the room is already row-locked to `playing`, so the seat picture is settled). Leave-before-start needs no refund. This is exactly what 9.3 re-validates and extends.

### B. Join/start affordability errors are **HTTP**, not the WS `error:insufficient_coins`.

The epic frames join as `action:join_room` returning `error:insufficient_coins`. **Room join/leave/start are HTTP endpoints in this codebase, not WebSocket actions** (`POST /api/v1/rooms/:id/join`, `/leave`, `/start`, `/quick-play`, `/quick-join` — verified, there is no `action:join_room`). Therefore:

- The join-time and start-time affordability failures surface as the **HTTP domain error** `apperr.ErrInsufficientCoins` (code `INSUFFICIENT_COINS`, 409 Conflict), caught client-side as a `FetchError` and rendered as a modal/toast — exactly like the existing `ROOM_FULL` flow.
- **Do not add a WS `error:insufficient_coins` event in this story.** The only new WS message here is `event:coin_settlement` (server→client, at match end). The WS-based insolvent-at-start *ejection* event belongs to Story 9.3.
- The client composes the "You need [S] coins — your balance: [balance]" message locally (it already knows its own `walletBalance` from `authStore.user` and the room's `coinBuyIn`); the error payload need only carry the code.

### C. Settlement timing & rounding

- **Money moves twice, in two transactions:** debit all human seats at `StartMatch` (tx 1); credit human winners at match end (tx 2). Losers' coins are simply gone (they were debited at start and are not credited back) — this *is* the pot transfer. Between start and end there is no persisted "pot" row; the pot is recomputed deterministically at end from seat composition × the session's captured `coinBuyIn`.
- **Capture `coinBuyIn` into the match session at `StartMatch`** so a later edit to `rooms.coin_buy_in` can't change an in-flight match's stakes.
- **Rounding (the epic omits this):** with an odd human count a split can be fractional — e.g. 3 humans + 1 bot, the 2-human team wins → `pot = 3S`, 2 winners → `1.5S` each. Coins are integers. **Rule:** integer-divide `pot / numHumanWinners`; distribute the remainder one coin at a time to winners in **ascending seat order**. (Decision flagged in Open Questions — default is "remainder to lowest seat.")

---

## Tasks / Subtasks

### Backend — Schema

- [x] **Task 1: Migration `000010_add_coin_economy_columns` (AC: #1, #12)**
  - [x] Highest existing migration is `000009` — use **`000010`**, never skip ([project-context.md] migrations rule). Mirror the additive `ALTER TABLE` style of [000009_add_wallet_columns_to_users.up.sql](../../server/migrations/000009_add_wallet_columns_to_users.up.sql) and [000007_add_bot_players.up.sql](../../server/migrations/000007_add_bot_players.up.sql).
  - [x] `.up.sql`:
    - `ALTER TABLE rooms ADD COLUMN coin_buy_in INTEGER NOT NULL DEFAULT 0 CHECK (coin_buy_in >= 0);`
    - `ALTER TABLE matches ADD COLUMN coin_buy_in INTEGER NOT NULL DEFAULT 0;`
    - `ALTER TABLE matches ADD COLUMN player1_coin_delta INTEGER NOT NULL DEFAULT 0;` … through `player4_coin_delta` (4 columns).
  - [x] `.down.sql`: drop the six columns in reverse, fully reversing the up ([project-context.md] — every up has a complete down).
  - [x] Existing rows backfill to `0` (treated as 0-stake / no economy), which is correct for all pre-9.2 matches and rooms. Comment this.
  - [x] Run `make migrate`; confirm columns exist and the down→up round-trips cleanly.

### Backend — Wallet primitives (extend the Story 9.1 package)

> The `wallet` package today exposes **only** `ProcessDailyLogin` ([wallet/repository.go](../../server/internal/wallet/repository.go), [wallet/service.go](../../server/internal/wallet/service.go)). It has **no** charge/credit primitives — you must add them, reusing the existing `FOR UPDATE` row-lock transaction pattern in [wallet/gorm_repo.go:27-74](../../server/internal/wallet/gorm_repo.go#L27-L74).

- [x] **Task 2: `ChargeStakes` — atomic multi-seat debit (AC: #4, #5, #13)**
  - [x] Add to `wallet.Repository`: `ChargeStakes(userIDs []uint, amount int) (insolventUserID uint, err error)` (or a struct result). Semantics: in **one** `db.Transaction`, for each userID lock the row `FOR UPDATE` (`tx.Clauses(clause.Locking{Strength:"UPDATE"})`), verify `wallet_balance >= amount`, deduct. If **any** row is insolvent → roll back the whole tx and return the offending userID + `apperr.ErrInsufficientCoins`. Lock rows in **ascending userID order** to avoid deadlocks between concurrent charges.
  - [x] `amount == 0` or empty `userIDs` → no-op success (quick-play rooms, all-bot tables).
  - [x] Pass only **human** userIDs (caller filters out UserID 0 bot seats).
  - [x] `wallet.Service.ChargeStakes(userIDs []uint, amount int)` wraps the repo call.

- [x] **Task 3: `ApplySettlement` — atomic multi-seat credit (AC: #6, #9, #11, #13)**
  - [x] Add `wallet.Repository.ApplySettlement(credits map[uint]int) error`: in **one** `db.Transaction`, for each `(userID → amount)` lock `FOR UPDATE` and add `amount` to `wallet_balance`. Roll back + return error if any update fails; caller logs via `slog`. Lock in ascending userID order.
  - [x] Only **winning human** seats appear in `credits` (losers are not credited — their debit at start *is* the forfeit). The no-human-winner case (AC #9) → empty `credits` → no-op (the sink is implicit: nobody is credited, debited coins stay gone).
  - [x] Provide a way to read the resulting balances so `event:coin_settlement` can carry `newBalance` per user (return updated balances from `ApplySettlement`, or expose `GetBalances(userIDs) map[uint]int` reading the post-credit rows).

- [x] **Task 4: `GetBalance` / `CanAfford` for the join check (AC: #2)**
  - [x] Add `wallet.Service.GetBalance(userID uint) (int, error)` (repo reads `user.User.WalletBalance`). Used by `JoinRoom` for the affordability check. Keep it a plain read (no lock) — the authoritative guard is the `ChargeStakes` re-validation at start.
  - [x] Wire any new constructors in [cmd/api/main.go](../../server/cmd/api/main.go) near the existing wallet DI (lines 125-128). The room handler and the match manager will need a handle to the wallet service (see Tasks 6 & 7).

- [x] **Task 5: `apperr.ErrInsufficientCoins` (AC: #2, #4)**
  - [x] Add to [internal/apperr/errors.go](../../server/internal/apperr/errors.go): `ErrInsufficientCoins = NewAppError("INSUFFICIENT_COINS", "insufficient coins", http.StatusConflict)`. Place near `ErrRoomFull`. 409 (not 402 — the project's status set is 200/201/400/401/403/404/409/500 per [project-context.md]).

### Backend — Room create + join + start wiring

- [x] **Task 6: `coin_buy_in` on create-room + join affordability check (AC: #1, #2)**
  - [x] **Model:** add `CoinBuyIn int` to `room.Room` ([room/model.go](../../server/internal/room/model.go)) with `gorm:"not null;default:0"` + `json:"coinBuyIn"`. Place among the config fields (after `ReconnectWindowSec`, before `Status`).
  - [x] **Create:** add `CoinBuyIn *int` (pointer so "omitted" ≠ "0") to `CreateRoomRequest` ([room/handler.go](../../server/internal/room/handler.go) ~line 36); in `CreateRoom`, default to **500** when nil, validate `>= 0` (reject negative with `apperr.ErrBadRequest`), persist on the room. **Quick-play rooms** (`QuickPlay`/`QuickJoin` synthesized rooms) set `CoinBuyIn = 0` explicitly (AC #1).
  - [x] **Join:** in `JoinRoom` ([room/handler.go](../../server/internal/room/handler.go) ~lines 525-624), after the room-found / status / capacity / already-in-room checks and **before** seating, if `room.CoinBuyIn > 0`, read the joiner's balance via the injected wallet service and reject with `apperr.ErrInsufficientCoins` if `balance < room.CoinBuyIn`. Preserve every existing check and broadcast.
  - [x] Inject the wallet service (or a minimal `BalanceReader` interface) into `RoomHandler` — extend its constructor and the `main.go` wiring. Do not import `wallet` in a way that creates a cycle (`wallet` imports `user`; `room` importing `wallet` is fine — verify no cycle).

- [x] **Task 7: Charge stakes at match start (AC: #4, #5)**
  - [x] In `RoomHandler.StartMatch` ([room/handler.go:1996](../../server/internal/room/handler.go#L1996)): after the room→`playing` transaction commits and the `seatInfo [4]match.PlayerSeatInfo` is built (~line 2103), if `room.CoinBuyIn > 0` charge the **human** seats: collect `userIDs` where `!IsBot && UserID != 0`, call `walletService.ChargeStakes(userIDs, room.CoinBuyIn)`.
    - On **success:** proceed to `h.matchStarter.StartMatch(...)`, passing `room.CoinBuyIn` through so the session captures it (Task 8).
    - On **`ErrInsufficientCoins`:** **revert** the room status to `waiting` (Update within a tx), clear nothing else, and return `apperr.ErrInsufficientCoins` to the owner. The match must NOT start. (Story 9.3 will replace this fail-safe with per-player ejection.)
  - [x] **Charge BEFORE `matchStarter.StartMatch`** so a failed charge never leaves a live session. The transaction in `ChargeStakes` guarantees all-or-nothing across the human seats — no partial debits.
  - [x] **Quick-play auto-start** (`autoStartIfFull`) has `CoinBuyIn = 0` in this story → `ChargeStakes` is a no-op; leave that path otherwise unchanged (Story 9.4 owns quick-play stakes).

- [x] **Task 8: Thread `coinBuyIn` into the match session (AC: #5, #11)**
  - [x] Add a `coinBuyIn int` param to `match.Manager.StartMatch` ([live_match.go:123](../../server/internal/match/live_match.go#L123)) and store it on the `LiveMatch` session struct alongside `startedAt`/`timerStyle`. This is the stake authority for settlement, immune to later room edits.
  - [x] Update all `StartMatch` call sites (the owner-start path in room/handler.go and the quick-play `autoStartIfFull` path — grep for `.StartMatch(`). Quick-play passes 0.

### Backend — Settlement at every match-finalize path

> There are **two** finalize paths that persist a match and call `RemoveSession`. Hook settlement into **both**. Grep for every `matchRepo.CreateWithHands(` + `RemoveSession(` to be sure none is missed ([feedback: sweep grep → edit all → re-grep]).

- [x] **Task 9: Settlement helper (pure + atomic) (AC: #5, #6, #8, #9, #11, #C-rounding)**
  - [x] Add a `match`-package settlement function, e.g. `computeSettlement(playerIDs [4]uint, botSeats [4]bool, winningTeam int, buyIn int) (deltas [4]int, credits map[uint]int, pot int)`:
    - `pot = (count of human seats) × buyIn`.
    - human seats on `winningTeam` are winners; `share = pot / numHumanWinners`, distribute `pot % numHumanWinners` one coin each to winners in ascending **seat** order.
    - each human winner delta = `share (+ maybe 1) − buyIn` (they get their share back **plus** winnings; net positive). Wait — winners already paid `buyIn` at start; crediting them `share` makes their net `share − buyIn`. Record `deltas[seat] = share − buyIn` (their wallet net for the match); `credits[userID] = share` (what `ApplySettlement` adds back).
    - each human loser delta = `−buyIn`; not in `credits`.
    - bot seats: delta `0`, never in `credits`.
    - `numHumanWinners == 0` (AC #9) → `credits` empty (sink), loser deltas still `−buyIn`.
  - [x] Keep this **DB-free and table-testable**. Determine `winningTeam` per `game.TeamForSeat` (seats 0/2 → team 0, 1/3 → team 1).
  - [x] Apply: call `walletService.ApplySettlement(credits)` in one tx (Task 3); on error, log via `slog` and still broadcast/persist (mirror the existing "persist failure does not strand clients" philosophy at [live_match.go:1059-1063](../../server/internal/match/live_match.go#L1059-L1063)). Record `deltas` + `buyIn` on the match row (Task 11).

- [x] **Task 10: Wire settlement into `handleMatchEnd` (natural + surrender) (AC: #6, #7)**
  - [x] In `handleMatchEnd` ([live_match.go:1019](../../server/internal/match/live_match.go#L1019)): `winningTeam = *finalState.WinnerTeam` (already computed at line 1020-1023; for surrender the engine sets `WinnerTeam` to the non-surrendering team — surrender therefore needs **no special case** here, satisfying AC #7). Call the settlement helper with `session.coinBuyIn`, apply credits, set the match row's `CoinBuyIn` + `Player{N}CoinDelta` before `CreateWithHands`, then broadcast `event:coin_settlement` per human (Task 12) **after** `event:match_end` and before/around `event:match_state`, preserving the 8.5-1 ordering contract documented at [live_match.go:1007-1018](../../server/internal/match/live_match.go#L1007-L1018).
  - [x] Skip entirely when `session.coinBuyIn == 0` (no economy → no settlement, no event).

- [x] **Task 11: Wire settlement into the abandonment finalizer (AC: #8)**
  - [x] In `handleSeatReconnectTimeout` ([reconnect.go:511](../../server/internal/match/reconnect.go#L511)): the winning team is **`1 - game.TeamForSeat(abandonedSeat)`** — NOT the record's `WinnerTeam` (hardcoded `0` at [reconnect.go:610](../../server/internal/match/reconnect.go#L610)). The whole abandoning team forfeits (both `−S`, no teammate refund). Call the same settlement helper with that winning team, apply credits, record `CoinBuyIn` + deltas on the abandoned match row before `CreateWithHands` ([reconnect.go:619](../../server/internal/match/reconnect.go#L619)), broadcast `event:coin_settlement`.
  - [x] **Check for other abandonment/finalize paths** — match-abandonment-on-timeout (Epic 5-5), all-disconnect, owner-leave-during-play. Grep every place a match is finalized; if another path persists a match and removes the session, it must settle too (or explicitly document why not). Search: `CreateWithHands`, `RemoveSession`, `Status: "abandoned"`.

- [x] **Task 12: Match-record economy fields + `event:coin_settlement` (AC: #6, #11, #12, #13)**
  - [x] **Model:** add `CoinBuyIn int` + `Player1CoinDelta`…`Player4CoinDelta int` to `match.Match` ([match/model.go](../../server/internal/match/model.go)) with `gorm:"not null;default:0"` + camelCase JSON tags. Extend `matchSeatColumns` or the record-building sites to populate deltas (bots → 0). Set them in both finalize paths.
  - [x] **WS contract — `event:coin_settlement` in BOTH files, same commit** ([project-context.md] no exceptions):
    - [events.go](../../server/internal/ws/events.go): `const EventCoinSettlement = "event:coin_settlement"` (near the other `event:` consts ~line 38) + `type CoinSettlementPayload struct { CoinDelta int json:"coinDelta"; NewBalance int json:"newBalance"; Pot int json:"pot" }`.
    - [wsEvents.ts](../../client/src/shared/types/wsEvents.ts): `export const EVENT_COIN_SETTLEMENT = "event:coin_settlement" as const;` + matching `CoinSettlementPayload` interface.
  - [x] **Send per-user** via `m.hub.SendToUser(userID, buildMessage(ws.EventCoinSettlement, payload))` so each human gets their own `coinDelta` + `newBalance`. (Per-user, not broadcast, because `newBalance` differs.) Pot is the same for all.
  - [x] Update [events_contract_test.go](../../server/internal/ws/events_contract_test.go) if it enumerates the contract (it asserts the two files stay in sync).

### Frontend — create-room, join error, settlement

- [x] **Task 13: Buy-in field in create-room + types (AC: #1)**
  - [x] `Room` type ([apiTypes.ts](../../client/src/shared/types/apiTypes.ts)): add `coinBuyIn: number;`. `CreateRoomRequest`: add `coinBuyIn: number;`. [api/rooms.ts](../../client/src/shared/api/rooms.ts) `createRoom` passes it.
  - [x] [CreateRoomModal.tsx](../../client/src/features/room/CreateRoomModal.tsx): add a "Coin buy-in" numeric input (default **500**, min **0**, no max), styled like the existing fields; include it in the submit payload and the `PreviewCard`. Cosmetic validation only (`>= 0`) — server is authority ([project-context.md]).
  - [x] Surface `coinBuyIn` wherever a room is shown so players see the stake before joining: room browse cards and the room lobby header (find the room-card + RoomPage components). Use the same coin icon treatment as Story 9.1's header pill ([coinGold.ts](../../client/src/shared/lib/coinGold.ts), lucide `Coins`). `0` buy-in may render as "Free" — see Open Questions.

- [x] **Task 14: Handle `INSUFFICIENT_COINS` on join (AC: #2)**
  - [x] `ROOM_FULL` is currently handled at **four** join entry points — mirror each so `INSUFFICIENT_COINS` is never silently dropped ([feedback: sweep grep → edit all → re-grep]): [JoinByCodeTile.tsx:35](../../client/src/features/lobby/components/JoinByCodeTile.tsx#L35), [LobbyPage.tsx:121](../../client/src/features/lobby/LobbyPage.tsx#L121) and [:135](../../client/src/features/lobby/LobbyPage.tsx#L135), [RoomPage.tsx:235](../../client/src/features/room/RoomPage.tsx#L235). In each, catch `err instanceof FetchError && err.code === "INSUFFICIENT_COINS"` and show a modal/toast composed locally: "You need {buyIn} coins to join — your balance: {balance}" using `authStore.user.walletBalance` and the room's `coinBuyIn`. Re-grep `INSUFFICIENT_COINS` / `ROOM_FULL` after editing to confirm parity across all sites.
  - [x] The owner `StartMatch` path also returns `INSUFFICIENT_COINS` (if a seated human went insolvent) — handle it on the start button with a clear toast. (Full ejection UX is 9.3.)

- [x] **Task 15: Handle `event:coin_settlement` (AC: #6)**
  - [x] Add `EVENT_COIN_SETTLEMENT` to the imports + `GAME_*` handling in [useWsDispatch.ts](../../client/src/shared/hooks/useWsDispatch.ts) `dispatchGameEvent`. On receipt: **update `authStore.user.walletBalance = payload.newBalance`** (immutable `setUser({...user, walletBalance})` — this is a cross-store update; balance lives on `authStore`, not `gameStore`) and show a settlement toast via `sonner`: "You won {delta} coins" / "You lost {|delta|} coins" (delta sign decides). Defensive payload validation (reject non-integer `coinDelta`/`newBalance`) like the other dispatch cases.
  - [x] `gameStore` is wiped on navigation away (not on `game_over`) — the settlement event arrives while still on the match page, so the toast + balance update fire correctly; the persisted balance on `authStore` survives the later navigation.

- [x] **Task 16: i18n in ALL four locales (AC: #1, #2, #6)**
  - [x] Add keys to **all** of en/sr/mk/hr ([client/src/shared/i18n/](../../client/src/shared/i18n/)) — missing one fails the feature-complete checklist. Keys (use `{feature}.{component}.{element}`): create-room buy-in label/placeholder; `room.errors.insufficientCoins` (with `{{buyIn}}`/`{{balance}}` interpolation); `match.settlement.won`/`.lost` (with `{{amount}}`); buy-in display label ("Buy-in"/"Free"). **mk = all-Cyrillic, sr = Latin**; keep terminology consistent with [reference_localization-terminology] and the existing `wallet.*`/`rewards.*` namespaces from Story 9.1. Idiomatic, not literal English→locale.

### Tests

- [x] **Task 17: Backend tests (AC: all)**
  - [x] `wallet_test.go` (extend): table-driven `ChargeStakes` — happy path debits all humans; one insolvent → whole tx rolls back (all balances unchanged) + `ErrInsufficientCoins`; `amount 0` / empty list no-op; concurrency: two charges don't deadlock (ascending-lock-order). `ApplySettlement` credits correctly; empty credits no-op. **PostgreSQL integration tests use a per-test transaction + rollback and create their own data — never seed data** ([project-context.md]).
  - [x] `match` settlement unit tests (table-driven, DB-free): the `computeSettlement` helper across every case — all-human 2v2 (4S → +S/−S); H+B vs H+B winner/loser; H+B vs B+B humans win (net 0) and humans lose (sink, AC #9); abandonment winner = non-abandoning team (AC #8); **odd split** 3 humans (3S/2 → remainder to lowest seat, AC #C). Assert deltas sum to a coin-conserving total (winners' gains = losers' losses, minus any sink).
  - [x] `room/handler_test.go`: create-room persists `coinBuyIn` (default 500, explicit value, quick-play forces 0); `JoinRoom` rejects with `INSUFFICIENT_COINS` when `balance < buyIn` and seats when `≥`; `StartMatch` charges human seats, reverts room→`waiting` + returns `INSUFFICIENT_COINS` on insolvency, never starts an unpaid match, never charges bots.
  - [x] `ws/events_contract_test.go`: `event:coin_settlement` present in both contract files.
- [x] **Task 18: Frontend tests (AC: #1, #2, #6)**
  - [x] `CreateRoomModal.test.tsx`: buy-in field defaults to 500, rejects negative, submits the value; renders in preview.
  - [x] Join flow test: `INSUFFICIENT_COINS` FetchError shows the modal with composed buy-in/balance message.
  - [x] `useWsDispatch` / settlement test: `event:coin_settlement` updates `authStore.walletBalance` and toasts won/lost by delta sign; ignores malformed payloads. Present-tense `it(...)`, `data-testid` selectors ([project-context.md]).

### Definition of Done (Feature-Complete Checklist — hard gate)

- [x] Server handler + repository layer + tests ✔ (wallet charge/credit/balance, room create/join/start, match settlement)
- [x] Domain error `apperr.ErrInsufficientCoins` added ✔
- [x] WebSocket event `event:coin_settlement` added to **both** `events.go` and `wsEvents.ts` (same commit) ✔
- [x] Frontend component(s) + co-located tests ✔
- [x] API client changes in `shared/api/rooms.ts` ✔
- [x] i18n strings in **all** four translation files ✔
- [x] Linter passes (`make lint`) — note `golangci-lint` may be CI-only locally; fall back to `gofmt -l` + `go vet` (per Story 9.1)
- [x] All existing tests pass (`make test`) — and **update existing fixtures** for the new required `Room.coinBuyIn` / `Match` delta fields (Story 9.1 had to touch ~13 fixtures for a similar required-field addition — expect the same here)

### Review Findings

_Code review 2026-06-19 — 3-layer adversarial (Blind Hunter + Edge Case Hunter + Acceptance Auditor), full diff `dc66245..HEAD`. 24 raw findings → 2 decision-needed (resolved 2026-06-19), 3 patch, 6 deferred, 8 dismissed as noise/by-design. Decisions: D1 (charge-then-start-fails) deferred to Story 9.3; D2 (phantom deltas) patched — record true deltas._

**Patch**

- [x] [Review][Patch] `JoinByCodeTile` insufficient-coins toast renders empty `{{buyIn}}`/`{{balance}}` placeholders — calls `t("room.errors.insufficientCoins")` with no interpolation params; should use the param-less `room.errors.insufficientCoinsGeneric` (already added; used by the quick-join path) [client/src/features/lobby/components/JoinByCodeTile.tsx:36]
- [x] [Review][Patch] `event:coin_settlement` coin fields accept fractional values — Zod schema uses `z.number()` (not `.int()`) for `coinDelta`/`newBalance`/`pot`, and the client dispatch guard validates `coinDelta`/`newBalance` integrality but skips `pot` [client/src/shared/types/wsEvents.schemas.ts:990; client/src/shared/hooks/useWsDispatch.ts]
- [x] [Review][Patch] Wallet-settlement failure records phantom winner deltas on the match row (resolved from decision-needed) — `settleMatch` returns the optimistic computed `deltas` even when `ApplySettlement`/`GetBalances` errors ([settlement.go:41-51](../../server/internal/match/settlement.go#L41-L51)), and the finalize paths persist them to `matches.player{N}_coin_delta` though no wallet credit happened. Fix: on wallet-credit failure, record the **true** wallet outcome (all humans `−buyIn`, winners not credited) instead of the computed positive deltas, so the match ledger matches wallet reality. [server/internal/match/settlement.go:41-51]

**Deferred**

- [x] [Review][Defer] Successful stake charge followed by a failed match-start strands coins and the room (resolved from decision-needed) — charge commits ([handler.go:2180](../../server/internal/room/handler.go#L2180)) before `matchStarter.StartMatch` ([handler.go:2198](../../server/internal/room/handler.go#L2198)); a start error is only `slog`-logged with no revert/refund, so humans are debited, no match runs, and the room is stranded in `playing` [server/internal/room/handler.go:2198] — deferred to Story 9.3: 9.3 owns StartMatch hardening + insolvency/ejection; refund-on-start-failure folds into that work. StartMatch failure is rare (only "session already exists") and the room-stranding half is pre-existing.
- [x] [Review][Defer] One insolvent (or post-join-spent) seat makes `ChargeStakes` roll back the whole table and blocks every start attempt — griefing/denial vector [server/internal/room/handler.go:2180] — deferred, Story 9.3 owns per-player ejection (9.2's "never start an unpaid match" fail-safe is satisfied)
- [x] [Review][Defer] Boot-time reconcile of stale `playing` rooms sinks already-charged stakes with 0 recorded deltas (crash between charge and match-end) [server/internal/match/reconcile.go:91] — deferred, crash-recovery; documented inline as a deliberate limitation
- [x] [Review][Defer] `GetBalances` post-credit read is a separate unlocked transaction → `newBalance` in the settlement event can be stale under a concurrent wallet write [server/internal/match/settlement.go:47] — deferred, spec-sanctioned design (Task 3 offered both options), self-correcting on next read
- [x] [Review][Defer] Natural match-end with nil `WinnerTeam` defaults to TeamA and credits it the pot instead of a sink [server/internal/match/live_match.go handleMatchEnd] — deferred, defensive-only (the engine always sets a winner today)
- [x] [Review][Defer] No maximum on `coinBuyIn`; `pot = numHumans × buyIn` can overflow the `INTEGER` column at extreme stakes [server/internal/match/settlement.go:113] — deferred, gated by the start-time affordability charge; AC1 mandates "no maximum"

---

## Dev Notes

### Why this story matters (Epic context)

Story 9.2 is what makes the wallet (9.1) *mean* something: coins are staked and won. It establishes the **single source of pot math** that downstream stories point to — Epic 8 surrender already references "Story 9.2 for pot math", and Story 9.4 (Quick Play bracketing) says "settlement proceeds per Story 9.2 rules." Get the settlement helper clean and table-tested; later stories reuse it.

Dependencies it sets up:
- **9.3 Insolvency Ejection** — extends the `StartMatch` charge (your fail-safe revert becomes per-player ejection) and adds the return-to-room affordability gate. Build the charge so 9.3 can wrap it.
- **9.4 Quick Play Coin Bracketing** — sets `coin_buy_in` per bracket on the synthesized room; reuses `ChargeStakes` + settlement.

### The two reconciliations (do not skip — see ⚠️ Design Decisions)

1. **Deduct at `StartMatch`, not at join.** Join checks only. (Decision A — backed by 9.3, the change proposal, 9.1's notes, and the one-room-at-a-time constraint.)
2. **HTTP errors, not WS, for affordability.** Room ops are HTTP. Only `event:coin_settlement` is a new WS message. (Decision B.)

### Source tree — files to touch

**Backend (new):** `server/migrations/000010_add_coin_economy_columns.{up,down}.sql`

**Backend (modify):**
- [wallet/repository.go](../../server/internal/wallet/repository.go), [wallet/gorm_repo.go](../../server/internal/wallet/gorm_repo.go), [wallet/service.go](../../server/internal/wallet/service.go), `wallet/wallet_test.go` — add `ChargeStakes`, `ApplySettlement`, `GetBalance` (reuse the `FOR UPDATE` pattern at [gorm_repo.go:27-74](../../server/internal/wallet/gorm_repo.go#L27-L74))
- [apperr/errors.go](../../server/internal/apperr/errors.go) — `ErrInsufficientCoins`
- [room/model.go](../../server/internal/room/model.go) — `CoinBuyIn`; [room/handler.go](../../server/internal/room/handler.go) — `CreateRoomRequest`, `CreateRoom`, `JoinRoom` check (~525-624), `StartMatch` charge (~1996-2120), quick-play `coin_buy_in=0`; `room/handler_test.go`
- [match/model.go](../../server/internal/match/model.go) — `CoinBuyIn` + `Player{N}CoinDelta`; [match/live_match.go](../../server/internal/match/live_match.go) — `StartMatch` param + session field (123-196), `handleMatchEnd` settlement (1019-1079); [match/reconnect.go](../../server/internal/match/reconnect.go) — abandonment settlement (511-633); `match` settlement helper + tests
- [ws/events.go](../../server/internal/ws/events.go) + [ws/events_contract_test.go](../../server/internal/ws/events_contract_test.go)
- [cmd/api/main.go](../../server/cmd/api/main.go) — inject wallet service into room handler + match manager (near wallet DI at 125-128)

**Frontend (modify):** [apiTypes.ts](../../client/src/shared/types/apiTypes.ts), [api/rooms.ts](../../client/src/shared/api/rooms.ts), [CreateRoomModal.tsx](../../client/src/features/room/CreateRoomModal.tsx), room browse card + [RoomPage](../../client/src/features/room/) header (buy-in display), the join handler(s) (`INSUFFICIENT_COINS`), [wsEvents.ts](../../client/src/shared/types/wsEvents.ts), [useWsDispatch.ts](../../client/src/shared/hooks/useWsDispatch.ts) (`event:coin_settlement` → authStore balance + toast), all four `shared/i18n/*.json`

### Reading the code being modified — current behavior to PRESERVE

- **`RoomHandler.StartMatch`** ([room/handler.go:1996](../../server/internal/room/handler.go#L1996)): row-locks the room, validates owner/status/all-seated/seat-coverage, flips `waiting`→`playing` in `RunInTransaction`, clears presence, builds `seatInfo [4]`, calls `matchStarter.StartMatch`. **Preserve all of it.** Insert the charge **after** the room tx commits and the seatInfo is built, **before** `matchStarter.StartMatch`. On insolvency, add a small tx to set status back to `waiting`.
- **`match.Manager.StartMatch`** ([live_match.go:123](../../server/internal/match/live_match.go#L123)): builds `playerIDs`/`botSeats`, creates the `game.NewGame`, registers `userToRoom` (skips UserID 0 bots), broadcasts dealing→bidding. **Only** add the `coinBuyIn` param + session field; don't touch the deal/timer/bot logic. Note `humanUserIDs(playerIDs)` already exists for "humans only."
- **`handleMatchEnd`** ([live_match.go:1019](../../server/internal/match/live_match.go#L1019)): the 8.5-1 **ordering contract** is load-bearing — persist first, then `event:match_end`, then `event:match_state`; both events fire even if persist fails so clients aren't stranded. Slot `event:coin_settlement` after `match_end`. `WinnerTeam` is already resolved here (surrender included). Match record is built and persisted via `CreateWithHands`.
- **`handleSeatReconnectTimeout`** ([reconnect.go:511](../../server/internal/match/reconnect.go#L511)): abandonment finalizer — sets `Phase=MatchEnd`, `WinnerTeam` stays nil but the **record hardcodes `WinnerTeam: 0`** (line 610). For settlement you must compute the real winner = `1 - TeamForSeat(abandonedSeat)`. Persists `Status:"abandoned"`, `AbandonedBy`. Preserve the broadcast/cleanup ordering; add settlement + deltas before `CreateWithHands`.
- **`matchRepo.CreateWithHands`** ([match/gorm_repo.go:43-56](../../server/internal/match/gorm_repo.go#L43-L56)): match + hands in one tx. Your new delta/buy-in columns ride on the same `Match` insert.
- **Bots:** identified by `IsBot` (`game.PlayerState.IsBot`, `match.PlayerSeatInfo.IsBot`); bot seats carry `UserID == 0`. The match model comment already states Epic 9 treats `HasBots`/bot seats as "ignore for coins." Never charge or credit a UserID-0 seat.
- **Wallet atomic pattern** ([wallet/gorm_repo.go:27-74](../../server/internal/wallet/gorm_repo.go#L27-L74)): `db.Transaction` + `tx.Clauses(clause.Locking{Strength:"UPDATE"}).First(&u, id)` → mutate → `Updates`. Reuse verbatim for charge/credit. `wallet_balance` has a DB `CHECK (>= 0)` — your charge must guard in Go *and* will be caught by the constraint as a backstop.

### Architecture & convention guardrails (must follow)

- **Server is the money authority** — frontend buy-in validation is cosmetic; the `StartMatch` charge re-validates ([project-context.md] server-authoritative).
- **GORM three-convention bridge:** `coin_buy_in` (snake) ↔ `CoinBuyIn` (Pascal) ↔ `coinBuyIn` (camel); `player1_coin_delta` ↔ `Player1CoinDelta` ↔ `player1CoinDelta`.
- **JSON wire format always camelCase**; **no JS truthiness on numbers** — `coinBuyIn === 0`, `coinDelta === 0` are real values, compare explicitly ([project-context.md]).
- **Both WS contract files updated together** for `event:coin_settlement` ([project-context.md] no exceptions). Typed dispatch only — never raw `JSON.parse` in components.
- **Multi-event ordering** at match end is animation-load-bearing — keep `event:coin_settlement` after `event:match_end` ([project-context.md] + 8.5-1 contract).
- **`ws/router.go` does dispatch only**; settlement logic lives in the match manager / wallet service, never the WS layer.
- **Frontend:** balance on `authStore.user`, immutable `setUser({...})`; named exports; no `fetch()` in components ([project-context.md]).
- **One story = one branch = one PR.** Branch: `feat/9-2-room-buy-in-settlement`.

### Testing standards summary

- Go: `testing` + `testify`, co-located `_test.go`; **table-driven** for the settlement math + charge/credit; **per-test transaction + rollback** for PostgreSQL; tests create their own data (no `make seed` dependency). The rules-engine "test through `ApplyAction` only / use testfixtures factories" rule is about the **game engine** — settlement lives in the `match`/`wallet` packages and is tested directly.
- Frontend: Vitest, co-located `.test.tsx`, `data-testid` selectors, present-tense `it(...)`; cover `0`-buy-in, insufficient-coins modal, and won/lost/zero settlement deltas.

### Project Structure Notes

- New economy state spans three tables consistently with existing patterns: `rooms.coin_buy_in` (room config, like `match_mode`), `matches.coin_buy_in` + per-seat `player{N}_coin_delta` (mirrors the existing per-seat `player{N}_is_bot` denormalization), and wallet balance stays on `users` (Story 9.1). No new tables — the per-seat-columns style already established for matches is the right fit.
- The `wallet` package keeps its Story 9.1 variance (state on `users`, no own table); the new `ChargeStakes`/`ApplySettlement`/`GetBalance` extend its existing transactional surface.
- **Variance to flag:** settlement reaches across packages (match → wallet). Inject the `wallet.Service` into the match manager (constructor param + `main.go`), and into the room handler for the join check + start charge. Keep import direction acyclic (`wallet`→`user`; `room`/`match`→`wallet` is fine).

### References

- [Source: epics.md#Story 9.2](../planning-artifacts/epics.md) — ACs (1763-1821); Epic 9 overview (1720-1722); Story 9.3 (1823-1856, StartMatch deduction authority); Story 9.4 (1858-1880, "settlement per 9.2").
- [Source: sprint-change-proposal-2026-06-18.md](../planning-artifacts/sprint-change-proposal-2026-06-18.md) — §9.2 settlement rewrite (human-only pot, bots, abandonment full loss, no-human-winner sink); §9.3 StartMatch deduction = authoritative guard; economy constants are placeholders.
- [Source: 9-1-coin-wallet-foundation.md](9-1-coin-wallet-foundation.md) — wallet package, atomic `FOR UPDATE` pattern, "9.2 deducts at match start", coin-pill/`coinGold.ts` display treatment, i18n `wallet.*`/`rewards.*` namespaces.
- [Source: project-context.md](../../_bmad-output/project-context.md) — GORM tag bridge, camelCase wire, server-authoritative game logic, both-WS-files rule, multi-event ordering, atomic wallet mutations, migration numbering, feature-complete checklist, i18n-all-locales, no-JS-truthiness-on-Go-zero-values.
- [Source: server/internal/room/handler.go](../../server/internal/room/handler.go) — `CreateRoom`(255-398), `JoinRoom`(525-624), `LeaveRoom`(626-791), `StartMatch`(1996+), quick-play(2143+).
- [Source: server/internal/match/live_match.go](../../server/internal/match/live_match.go) — `StartMatch`(123-196), `handleMatchEnd`(1019-1079) + 8.5-1 ordering contract (1007-1018), `sendError`(1081).
- [Source: server/internal/match/reconnect.go](../../server/internal/match/reconnect.go) — abandonment finalizer `handleSeatReconnectTimeout`(511-633).
- [Source: server/internal/match/{model,gorm_repo}.go](../../server/internal/match/) — `Match` model + `matchSeatColumns`; `CreateWithHands` transaction.
- [Source: server/internal/wallet/](../../server/internal/wallet/) — current API (`ProcessDailyLogin` only) + `FOR UPDATE` transaction to mirror.
- [Source: server/internal/ws/events.go](../../server/internal/ws/events.go) + [wsEvents.ts](../../client/src/shared/types/wsEvents.ts) — event/error const + payload patterns; [useWsDispatch.ts](../../client/src/shared/hooks/useWsDispatch.ts) typed dispatch.
- [Source: server/migrations/000003_create_rooms.up.sql, 000005_create_matches.up.sql, 000007_add_bot_players.up.sql, 000009_add_wallet_columns_to_users.up.sql](../../server/migrations/) — schema baselines + additive-migration style.

### Previous-story / Git intelligence

- Story 9.1 (done) shipped the `wallet` package + `users.wallet_balance` (CHECK ≥ 0) + the coin-pill UI. 9.2 reuses its transaction pattern and display treatment; it does **not** re-derive any of that.
- Bots (Story 10.3) added `IsBot` flags + nullable player FKs on `matches` and the `room_bots` table — 9.2's "bots never stake" leans entirely on these.
- The match model + `handleMatchEnd` already carry forward-looking comments naming Epic 9 ("ignore bot seats for coins"; the `completed`/`abandoned` status as the load-bearing signal). The hooks were left in deliberately for this story.

---

## Open Questions (for stakeholder confirmation — do NOT block dev on these; defaults chosen)

1. **Deduction timing (Decision A).** This story deducts at **match start** (join is a check only), reconciling the epic's escrow-at-join wording against Story 9.3 + the change proposal + the one-room-at-a-time constraint. **Confirm** this is the intended model (vs. literal escrow-at-join with leave-refunds). *Default applied: deduct-at-start.* This is the single highest-impact decision.
2. **Affordability error transport (Decision B).** Join/start use the **HTTP** `INSUFFICIENT_COINS` error (room ops are HTTP, not WS); only `event:coin_settlement` is a new WS message. **Confirm** — vs. forcing a WS `error:insufficient_coins`. *Default applied: HTTP.*
3. **Odd-split rounding (Decision C).** With an odd human count (e.g. 3 humans, 2-human team wins → 3S/2), the remainder coin goes to the **lowest-seat** winner. **Confirm** the rule (alternatives: highest seat, or pot floored and remainder to sink). *Default applied: remainder to lowest seat.*
4. **Quick-play buy-in = 0 in 9.2.** Quick-play rooms are created free in this story; real bracketed stakes arrive in 9.4. **Confirm** quick-play should be free (un-staked) until 9.4 ships. *Default applied: quick-play free in 9.2.*
5. **Settlement-vs-match-record atomicity.** Wallet credits run as one transaction; the match-row delta columns are written separately (best-effort, consistent with the existing "persist failure must not strand clients" rule). **Confirm** wallets-are-source-of-truth is acceptable (vs. one combined cross-package transaction for wallet credits + match row). *Default applied: separate, wallet tx authoritative.*
6. **`0`-buy-in display label.** Free rooms show "Free" rather than "0 coins" on cards/lobby. **Confirm** copy. *Default applied: "Free".*

## Dev Agent Record

### Agent Model Used

claude-opus-4-8[1m] (Claude Opus 4.8, 1M context) — dev-story workflow.

### Debug Log References

- Dev DB (port 5433) was at migration version 8; applied `000009` + `000010` via the Postgres container (the `migrate` CLI is not installed locally), verified the `000010` down→up round-trip, and stamped `schema_migrations` to 10. This is exactly what `make migrate` performs; no schema drift.
- Local lint: `golangci-lint run ./...` (exit 0) and `gofmt -l` clean on touched files; client `npx eslint .` + `npx prettier --check .` clean; `tsc -p tsconfig.build.json` clean. Pre-existing test-only type errors (`RoomDetail.returnedUserIds` missing in fixtures, surfaced only by the full `tsc --noEmit`) are unrelated to this story and excluded from CI's build typecheck.

### Completion Notes List

**Settlement model (Design Decisions A/B/C applied).** Coins move twice in two transactions: every human seat is debited the buy-in atomically in `StartMatch` (`wallet.ChargeStakes`, `FOR UPDATE`, ascending-userID lock order, all-or-nothing), and the winning human seats are credited at match end (`wallet.ApplySettlement`). Join is a **check only** (`wallet.GetBalance`) — no escrow. Affordability failures surface as the **HTTP** `INSUFFICIENT_COINS` (409), never a WS error; the only new WS message is `event:coin_settlement`. Odd-split remainder goes to the lowest winning seat.

**Pot math is a single pure helper.** `match.computeSettlement(playerIDs, botSeats, winningTeam, buyIn)` is DB-free and table-tested across every case: all-human 2v2, H+B vs H+B, H+B vs B+B (net-zero human winner, and the AC #9 no-human-winner **sink**), abandonment (winner = `1 − TeamForSeat(abandonedSeat)`), odd 3-human split, and zero buy-in. Wired into **both** live finalize paths — `handleMatchEnd` (natural + surrender; surrender needs no special case) and `handleSeatReconnectTimeout` (abandonment). The third finalize path, boot-time `reconcileStaleRoom`, deliberately does **not** settle (no surviving session → no captured buy-in / winner; documented inline).

**Event ordering.** `event:coin_settlement` is sent per-human (each carries its own `coinDelta`/`newBalance`; shared `pot`) **after** `event:match_end` and **before** the trailing `event:match_state`, preserving the Story 8.5-1 ordering contract. Frontend updates `authStore.user.walletBalance` (survives the later gameStore wipe) and toasts won/lost by delta sign (silent on a net-zero delta); malformed payloads are rejected.

**Bots never pay or get paid** (UserID 0 / `IsBot` seats skipped everywhere). **Quick-play rooms are free** (`coin_buy_in = 0`) in this story; bracketed stakes are Story 9.4. The `StartMatch` insolvency fail-safe reverts the room to `waiting` and returns `INSUFFICIENT_COINS` (the polished per-player ejection is Story 9.3).

**WS contract drift gate kept in sync** — added `CoinSettlementPayload` to `events.go` + `wsEvents.ts`, the Go golden + table entry, and the Zod schema + conformance + client contract-test row.

**Verification:** backend `go test ./...` all green (incl. Postgres integration tests for `ChargeStakes`/`ApplySettlement`/`GetBalances` and the room-handler coin tests); frontend `vitest run` 835/835 green. Updated ~12 existing Room/CreateRoomRequest test fixtures + 2 CreateRoomModal payload assertions for the new required `coinBuyIn` field (the same fixture-sweep Story 9.1 flagged). i18n added to all four locales (mk all-Cyrillic, sr Latin), parity test green.

### File List

**Backend — new**

- `server/migrations/000010_add_coin_economy_columns.up.sql`
- `server/migrations/000010_add_coin_economy_columns.down.sql`
- `server/internal/match/settlement.go`
- `server/internal/match/settlement_test.go`
- `server/internal/match/settlement_wiring_test.go`
- `server/internal/room/coin_handler_test.go`

**Backend — modified**

- `server/internal/apperr/errors.go` — `ErrInsufficientCoins`
- `server/internal/wallet/repository.go` — `ChargeStakes`/`ApplySettlement`/`GetBalance`/`GetBalances` on the interface
- `server/internal/wallet/gorm_repo.go` — implementations (reuse the `FOR UPDATE` pattern)
- `server/internal/wallet/service.go` — service wrappers
- `server/internal/wallet/wallet_test.go` — charge/credit/balance + concurrency tests
- `server/internal/room/model.go` — `Room.CoinBuyIn`
- `server/internal/room/handler.go` — create-room buy-in (default 500), join affordability check, `StartMatch` charge + revert, quick-play `coin_buy_in=0`, `WalletService` interface + DI, `coinBuyIn` in lobby broadcasts, threaded into `matchStarter.StartMatch`
- `server/internal/room/handler_test.go` — `fakeMatchStarter` signature + `NewRoomHandler` call sites
- `server/internal/match/model.go` — `Match.CoinBuyIn` + `Player{1..4}CoinDelta`
- `server/internal/match/live_match.go` — session `coinBuyIn`, `WalletSettler` + `SetWalletSettler`, `StartMatch` param, settlement in `handleMatchEnd` + event send
- `server/internal/match/reconnect.go` — abandonment settlement (winner = non-abandoning team)
- `server/internal/match/reconcile.go` — documented no-settlement rationale
- `server/internal/match/*_test.go` — `StartMatch` call sites updated for the new `coinBuyIn` arg (auto_action, bot_driver, manager, matchend, reconnect, score_reveal, timer_grace)
- `server/internal/ws/events.go` — `EventCoinSettlement` + `CoinSettlementPayload`
- `server/internal/ws/events_contract_test.go` — golden table entry
- `server/internal/ws/testdata/events/coin_settlement.json` — new golden
- `server/cmd/api/main.go` — wire wallet service into room handler + `SetWalletSettler` on the session manager

**Frontend — modified**

- `client/src/shared/types/apiTypes.ts` — `Room.coinBuyIn`, `CreateRoomRequest.coinBuyIn`
- `client/src/shared/types/wsEvents.ts` — `EVENT_COIN_SETTLEMENT` + `CoinSettlementPayload`
- `client/src/shared/types/wsEvents.schemas.ts` — `CoinSettlementPayloadSchema` + conformance + witness
- `client/src/shared/types/wsEvents.contract.test.ts` — golden import + table row
- `client/src/shared/hooks/useWsDispatch.ts` — `event:coin_settlement` → authStore balance + won/lost toast
- `client/src/shared/hooks/useWsDispatch.test.ts` — settlement dispatch tests
- `client/src/features/room/CreateRoomModal.tsx` — buy-in field + preview
- `client/src/features/room/CreateRoomModal.test.tsx` — buy-in tests + payload assertions
- `client/src/features/lobby/components/RoomCard.tsx` — buy-in display
- `client/src/features/room/RoomPage.tsx` — buy-in badge, join + start `INSUFFICIENT_COINS` handling
- `client/src/features/lobby/LobbyPage.tsx` — join `INSUFFICIENT_COINS` (composed message) + test
- `client/src/features/lobby/LobbyPage.test.tsx` — insufficient-coins join test
- `client/src/features/lobby/components/JoinByCodeTile.tsx` — join `INSUFFICIENT_COINS`
- `client/src/shared/i18n/{en,sr,mk,hr}.json` — buy-in label/hint, card buy-in/Free, room insufficient-coins errors, match settlement won/lost
- Test fixtures updated for the new required `Room.coinBuyIn`: `MatchmakingPage.test.tsx`, `RoomCard.test.tsx`, `MatchPage.test.tsx`, `RoomPage.{test,bots.test,diamond.test,locale.test}.tsx`, `useRoomUpdates.test.ts`, `roomStore.test.ts`

### Change Log

| Date       | Change                                                                                       |
| ---------- | -------------------------------------------------------------------------------------------- |
| 2026-06-18 | Implemented Story 9.2 — room buy-in, atomic stake charge at match start, and pot settlement (natural/surrender/abandonment/sink) with `event:coin_settlement`; full backend + frontend tests, i18n in all four locales. Status → review. |
