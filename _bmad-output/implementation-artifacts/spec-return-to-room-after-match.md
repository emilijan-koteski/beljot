---
title: 'Return to room after match (v1 — reopen & re-seat)'
type: 'feature'
created: '2026-06-16'
status: 'done'
context: ['{project-root}/_bmad-output/project-context.md']
baseline_commit: 'b6ea0e9ea0611e35d93a63cae75897ad02fe9b86'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** When a match ends, the result dialog's only action is "Return to lobby"; the room is flipped to `completed` and torn down, so a group that wants to keep playing must recreate a room and re-seat every game. This is painful for long sessions.

**Approach:** Add a second result-dialog action, "Return to room", backed by a new `POST /rooms/:id/return` endpoint that reopens the *same* room (`completed → waiting`, same id/code/config/owner) and clears any bots from the previous match. Each returner lands back on their original seat because their `room_players` row survived the match; the owner then starts the next match with the existing manual Start. This is v1 — the presence layer ("waiting to return" display + start-gated-on-all-present) is tracked separately in `deferred-work.md` (v2).

## Boundaries & Constraints

**Always:**
- Reuse the same room record — flip status in place, never copy. Reuse existing room-lobby machinery (seat select/leave, kick, owner-transfer-on-leave, manual `POST /rooms/:id/start`).
- REST-based, server-authoritative (no new client→server WS actions). The endpoint rejects callers who are no longer a `room_players` member — this inherently bars a kicked/left player.
- A returner reclaims their *original* seat: their `room_players` row (with `seat`/`team`) is intact — do not re-seat or clear it.
- Lazy + idempotent reopen: flip to `waiting` on the **first** `/return` only, inside a `FindByIDForUpdate` row-lock tx; a later/concurrent call when already `waiting` is a no-op success.
- Clearing bots deletes the `room_bots` rows and broadcasts `system:bot_removed` per seat. Reuse the existing `system:room_updated` + `system:bot_removed` events — **no new WS event or contract-file change in v1.**
- "Return to lobby" must **leave the room** for a member (best-effort `leaveRoom`) before navigating to `/lobby` — this frees their seat and transfers ownership if they were owner (existing `LeaveRoom`), so a lingering membership can't trap them in `ALREADY_IN_ROOM` once a teammate reopens the room. Navigation to `/lobby` proceeds regardless of the leave result. On match→room navigation ("Return to room"), do NOT leave the room (mirror the existing `fromRoom` guard).
- Add the new button label to **all four** locale files.

**Ask First:**
- Adding any DB migration, or any new WS event / contract-file change (the presence layer that would need these is deferred to v2).
- Changing the existing quick-play auto-start behavior for reopened quick-play rooms.

**Never:**
- No reservation timeout / auto-kick of pending players, and no auto-transfer of ownership away from an AWOL owner — if the owner never returns, remaining players simply leave (accepted behavior).
- No presence tracking, no "waiting to return" UI, and no gating of the owner Start button on whether absent players have actually returned — that is v2 (`deferred-work.md`). v1's known gap: a seat held by a not-yet-returned player counts as "occupied", so the owner could start before that player is back.
- No server-restart durability for the reopened lobby (out of scope).

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| First return | Member calls `/return`, room `completed` | Tx: status→`waiting`, `room_bots` deleted (+`system:bot_removed` per seat); `system:room_updated` to lobby; returns room detail | N/A |
| Repeat / concurrent return | Member calls `/return`, room already `waiting` | No second flip, no error; returns current room detail | N/A |
| Non-member return | Caller not in `room_players` (kicked, or never a member) | Reject; client routes to `/lobby` with a notice | 403/404 `error` |
| Owner starts next match | Reopened room, all 4 seats occupied | Existing `POST /rooms/:id/start` proceeds; bots from prior match are gone | N/A |
| Owner kicks / leaves in reopened room | Owner kicks a member, or owner leaves | Existing kick / leave behavior (status is `waiting`); leave transfers ownership to first remaining member | N/A |
| Picks "Return to lobby" | Member clicks lobby on the result dialog | Best-effort `leaveRoom`: row removed, seat freed, ownership transferred if owner; navigate `/lobby` regardless | Leave failure ignored (still navigate) |

</frozen-after-approval>

## Code Map

- `server/internal/room/handler.go` -- add `ReturnToRoom` handler; reuse `FindByIDForUpdate`, `UpdateStatus`, `FindBotsByRoomID`, `RemoveBot`, `FindPlayersByRoomID`, `RunInTransaction`; reuse the room-detail build used by `GetRoom`
- `server/cmd/api/main.go` -- register `POST /rooms/:id/return` (auth group, alongside the other `/rooms/:id/*` routes)
- `client/src/shared/api/rooms.ts` -- add `returnToRoom(roomId)` → `POST /rooms/{id}/return`
- `client/src/features/match/components/MatchResult.tsx` -- add a secondary "Return to room" `ClassicButton` + `onReturnToRoom` prop + testid `match-result-room-btn`
- `client/src/features/match/MatchPage.tsx` -- `handleReturnToRoom` (call API, `clearGame()`, navigate `/rooms/:roomId`); error → `/lobby` with a notice; `handleReturnToLobby` now best-effort `leaveRoom` for members before navigating; pass the prop into `MatchResult`
- `client/src/shared/i18n/{en,sr,hr,mk}.json` -- add `match.matchResult.returnToRoom`

## Tasks & Acceptance

**Execution:**
- [x] `server/internal/room/handler.go` -- add `ReturnToRoom(c echo.Context) error`: resolve roomID + caller; reject if caller is not a `room_players` member; in a `FindByIDForUpdate` tx, when status is `completed` flip to `waiting` and delete all `room_bots` (collect their seats first); after commit broadcast `system:bot_removed` per cleared seat and `system:room_updated` to the lobby; return the room detail payload (same shape as `GetRoom`). Idempotent no-op when already `waiting`. -- single new endpoint; everything else reuses existing room logic
- [x] `server/cmd/api/main.go` -- register `POST /rooms/:id/return` on the authenticated room route group -- wiring
- [x] `client/src/shared/api/rooms.ts` -- `returnToRoom(roomId: number): Promise<RoomDetail>` -- REST client parity
- [x] `client/src/features/match/components/MatchResult.tsx` -- add a second `ClassicButton` ("Return to room", secondary variant) above/below the existing lobby button, driven by a new `onReturnToRoom` prop; testid `match-result-room-btn` -- the new action
- [x] `client/src/features/match/MatchPage.tsx` -- `handleReturnToRoom`: resolve roomId (store `roomId` / route param), `await returnToRoom(roomId)`, then `clearGame()` + navigate `/rooms/:roomId` (with the no-auto-leave guard); on error `clearGame()` + navigate `/lobby` and surface a notice. `handleReturnToLobby`: best-effort `leaveRoom(roomId)` for members before navigating `/lobby` (frees seat / transfers ownership). Pass `onReturnToRoom` to `<MatchResult>` -- wiring + barred-return handling + lobby-leave
- [x] `client/src/shared/i18n/{en,sr,hr,mk}.json` -- add `match.matchResult.returnToRoom` to all four locales -- localized label
- [x] `server/internal/room/handler_test.go` (or the existing room handler test file) -- cover the I/O matrix: first-return reopen flips status + clears bots + broadcasts, repeat/concurrent return is an idempotent no-op, non-member is rejected. Table-driven; WS broadcasts via `httptest.Server` + real WS client; per-test transaction with rollback -- regression gate
- [x] `client/src/features/match/components/MatchResult.test.tsx` -- renders both buttons and invokes `onReturnToRoom` -- presentational coverage

**Acceptance Criteria:**
- Given a finished match, when each of the four players clicks "Return to room", then all land in the same room (same code) on their original seats, and the room shows `waiting` again.
- Given the previous match used bots, when the room reopens, then no bot from that match remains seated and those seats are open.
- Given the owner clicked "Return to lobby" instead, when they leave, then ownership transfers to a remaining member who can start the next match via the existing Start control.
- Given a player the owner kicked (or who left), when they click "Return to room", then the request is rejected and they are routed to the lobby.
- Given the room is already reopened, when another player clicks "Return to room", then they enter without a second status flip or error.
- Given a player clicks "Return to lobby" after a match, when they leave, then their room membership is removed (seat freed; ownership transferred if they were owner), so a subsequent teammate "Return to room" cannot trap them in `ALREADY_IN_ROOM`.

## Spec Change Log

- **2026-06-16 — review loopback (intent_gap), human-approved.** Edge-case review found that the original frozen boundary "'Return to lobby' is unchanged" left a member's `room_players` row intact; once a teammate reopened the room (`completed → waiting`), the lobby-goer's lingering membership trapped them in `ALREADY_IN_ROOM` on any later join/create, and an AWOL owner never transferred ownership (deadlocking the reopened room). Amended the frozen boundary + added an I/O matrix row + AC: "Return to lobby" now performs a best-effort `leaveRoom` for members before navigating. Avoids the known-bad trap/deadlock state. KEEP: the reopen handler, idempotency, bot-clearing, membership-rejection, and the primary/ghost two-button dialog were all correct and must survive — the only delta is the lobby-leave wiring in `MatchPage.handleReturnToLobby`.

## Design Notes

The reopen endpoint is the only new server surface — `start`, `kick`, `leave`, `seat`, and owner-transfer are reused unchanged. The transaction must be idempotent because all four clients see the dialog at once and may click within the same instant; the `FindByIDForUpdate` row-lock serializes them so only the first flips `completed → waiting`.

Known v1 gap (closed in v2 — `deferred-work.md`): without presence tracking, the owner Start button is enabled even if some ex-players are still on the result dialog. v2 adds the presence registry, a `system:player_returned` event, the "waiting to return" UI, and start-gating.

## Verification

**Commands:**
- `make lint` -- expected: passes (Go + TS)
- `make test` -- expected: passes (`go test ./...` + `vitest run`), including the new `ReturnToRoom` handler test and `MatchResult` test

**Manual checks:**
- Run `make dev`, finish a match with 4 clients; click "Return to room" on each and confirm all land in the same room on the same seats with the same code, any prior-match bots are gone, the owner can start the next match, and a kicked/left player who clicks "Return to room" is bounced to the lobby.

## Suggested Review Order

**Reopen endpoint (start here)**

- Entry point — reopens the room (`completed → waiting`, lazy + idempotent under row-lock), clears bots, rejects non-members
  [`handler.go:729`](../../server/internal/room/handler.go#L729)

- Wires the new REST route beside the other room ops
  [`main.go:188`](../../server/cmd/api/main.go#L188)

**Result-dialog actions & navigation**

- Two-button overlay — primary "Return to room", ghost "Return to lobby"
  [`MatchResult.tsx:151`](../../client/src/features/match/components/MatchResult.tsx#L151)

- Calls the reopen API then routes to the room; on error keeps the overlay with a toast
  [`MatchPage.tsx:1124`](../../client/src/features/match/MatchPage.tsx#L1124)

- Review-driven fix — best-effort `leaveRoom` so lobby-goers aren't trapped when a teammate reopens
  [`MatchPage.tsx:1103`](../../client/src/features/match/MatchPage.tsx#L1103)

- REST client for the reopen endpoint
  [`rooms.ts:39`](../../client/src/shared/api/rooms.ts#L39)

**Localization**

- New button + error labels across all four locales (en shown)
  [`en.json:636`](../../client/src/shared/i18n/en.json#L636)

**Tests**

- Backend — reopen + bot-clear, idempotent silence, non-member reject, playing reject
  [`return_to_room_test.go:83`](../../server/internal/room/return_to_room_test.go#L83)

- Frontend — both actions render and fire their handlers
  [`MatchResult.test.tsx:131`](../../client/src/features/match/components/MatchResult.test.tsx#L131)
