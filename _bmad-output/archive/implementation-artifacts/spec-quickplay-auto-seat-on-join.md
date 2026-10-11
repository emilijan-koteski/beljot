---
title: "Quick Play: auto-seat on join + auto-start when 4th joiner fills the room"
type: feature
created: "2026-04-29"
status: done
baseline_commit: 036820fab7919acfe0d1033ae062042e35167b8a
context:
  - "{project-root}/_bmad-output/project-context.md"
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Quick Play players land in the room lobby unseated and must manually click a seat. The 4th joiner has the same friction even though every seat is forced — making the matchmaking flow feel slower than the "instant" promise.

**Approach:** When a player calls `POST /rooms/quick-play`, the backend joins/creates the room **and** assigns the first available seat (0→3) atomically. If that assignment fills all four seats, auto-start fires inside the same handler — the 4th joiner is sent straight to the game; the first three remain free to swap to any empty seat in the room lobby.

## Boundaries & Constraints

**Always:**

- Quick Play players are seated by the server in the same transaction as join/create. Seat is the lowest-indexed empty seat (0,1,2,3 in order).
- Team is derived from seat via existing `teamForSeat` (even=teamA, odd=teamB) — no new mapping.
- 4th-joiner auto-start is server-authoritative: room.Status flips to `"playing"`, `gameStarter.StartGame` is invoked, and `system:game_started` + `system:room_updated` broadcast — same orchestration as the existing SelectSeat auto-start branch.
- The QuickPlay response carries `{ room, seat, gameStarted }` so the client can navigate directly to `/game/{id}` when the room auto-started.
- After auto-seat, any player can swap to a different empty seat via the existing SelectSeat path (clicking another empty tile in the room lobby) — no UI change needed for swapping.
- Existing SelectSeat auto-start branch stays in place as a safety net (defensive — unreachable in normal QP flow once auto-seat is in).
- `system:seat_updated` is broadcast for the auto-assigned seat so other room members see the new player land in their seat.

**Ask First:**

- Changing the QuickPlay response shape from `{ data: Room }` to `{ data: { room, seat, gameStarted } }` is a backwards-incompatible wire change. Only the frontend consumes it; ship together. If a non-frontend consumer is identified, halt.

**Never:**

- Do not auto-seat in regular `JoinRoom` or `CreateRoom` — only the QuickPlay path. Manual rooms keep their explicit seat-pick flow.
- Do not broadcast `system:game_started` from QuickPlay if the seated count is < 4.
- Do not change `SelectSeat` semantics, `StartGame` semantics, or the manual-room flow.
- Do not require the frontend to compute the seat — server picks it.

## I/O & Edge-Case Matrix

| Scenario                                     | Input / State                                       | Expected Output / Behavior                                                                                                                                                                | Error Handling                                                                                                       |
| -------------------------------------------- | --------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------- |
| First QP joiner — no rooms exist             | User authed, no QP rooms                            | New QP room created; player at seat 0, team A; `gameStarted=false`; HTTP 200 with `{room, seat:0, gameStarted:false}`; broadcasts `system:room_created` then `system:seat_updated`        | N/A                                                                                                                  |
| 2nd QP joiner — room has 1 seated at 0       | User authed, existing QP room with player at seat 0 | Player added at seat 1, team B; `gameStarted=false`; broadcasts `system:room_updated` + `system:seat_updated`                                                                             | N/A                                                                                                                  |
| 4th QP joiner — fills the last seat          | 3 players at seats 0,1,2                            | Player auto-seated at seat 3; `gameStarted=true`; room.Status=`playing`; `gameStarter.StartGame` called; broadcasts `system:room_updated` + `system:seat_updated` + `system:game_started` | If `StartGame` errors, log via slog; response still returns `gameStarted=true` (mirrors existing SelectSeat pattern) |
| Non-contiguous open seats (0 and 2 occupied) | User joins QP room with seats 0 and 2 filled        | Player lands at seat 1 (first empty slot in 0..3)                                                                                                                                         | N/A                                                                                                                  |
| Player already in any room                   | User authed; FindPlayerRoom returns non-nil         | HTTP 409 ALREADY_IN_ROOM (unchanged)                                                                                                                                                      | apperr.ErrAlreadyInRoom                                                                                              |
| Unauthenticated                              | No JWT                                              | HTTP 401 (unchanged)                                                                                                                                                                      | apperr.ErrUnauthorized                                                                                               |

</frozen-after-approval>

## Code Map

- `server/internal/room/handler.go` — `QuickPlay` (auto-seat + auto-start branch); response shape change
- `server/internal/room/handler_test.go` — update existing QuickPlay/JoinsExistingRoom tests, add fills-first-empty-seat + auto-start-on-fourth tests
- `server/internal/ws/events.go` — reuse `SystemSeatUpdated`, `SystemGameStarted`, `SystemRoomUpdated`, `SystemRoomCreated` (no new event)
- `client/src/shared/api/rooms.ts` — `quickPlay` return type → `QuickPlayResponse`
- `client/src/features/lobby/LobbyPage.tsx` — read `result.gameStarted` and `result.room`, navigate to `/game/{id}` or `/rooms/{id}` accordingly
- `client/src/features/lobby/LobbyPage.test.tsx` — update mocks to new shape; add gameStarted-true → /game navigation test

## Tasks & Acceptance

**Execution:**

- [x] `server/internal/room/handler.go` — In `QuickPlay`, inside the existing transaction, after `AddPlayer` + `IncrementPlayerCount`, find the lowest empty seat 0..3 via `tx.FindPlayerBySeat`, then call `tx.UpdatePlayerSeat(roomID, userID, seat, teamForSeat(seat))`. Capture the assigned seat in an outer var.
- [x] `server/internal/room/handler.go` — After the find/create transaction, run a second transaction (mirroring the SelectSeat auto-start block at handler.go:670–699): if room is QuickPlay + `waiting` + 4 seated, set `Status="playing"`, call `tx.Update(room)`, set `gameStarted=true`, capture the room.
- [x] `server/internal/room/handler.go` — When auto-started: build `[4]PlayerSeatInfo` from players, resolve timer/reconnect window, call `h.gameStarter.StartGame(...)` (slog.Error on failure, do not return an error), broadcast `system:game_started` to room, broadcast `system:room_updated` lobby-wide. Mirror handler.go:701–731.
- [x] `server/internal/room/handler.go` — Always broadcast `system:seat_updated` (room-scoped) for the auto-assigned seat after the find/create tx, with `previousSeat: nil`.
- [x] `server/internal/room/handler.go` — Response shape: `c.JSON(200, {"data": {"room": resultRoom, "seat": assignedSeat, "gameStarted": gameStarted}})`.
- [x] `server/internal/room/handler_test.go` — Update `TestQuickPlay_CreatesNewRoom`: assert response carries `seat=0`, `gameStarted=false`, and that the player record now has Seat=&0, Team=&"teamA".
- [x] `server/internal/room/handler_test.go` — Update `TestQuickPlay_JoinsExistingRoom`: seed the existing player at seat 0, then assert the joining player is seated at seat 1 with team teamB.
- [x] `server/internal/room/handler_test.go` — Update `TestQuickPlay_SkipsNonQuickPlayRooms`: response shape now `{room, seat, gameStarted}`; assert seat=0 on the new room.
- [x] `server/internal/room/handler_test.go` — Add `TestQuickPlay_FillsFirstEmptySeat`: seed QP room with seats 0 and 2 filled (PlayerCount=2, 2 seated players), call QuickPlay, expect seat=1 in response and player record Seat=&1, Team=&"teamB".
- [x] `server/internal/room/handler_test.go` — Add `TestQuickPlay_AutoStartsOnFourthJoiner`: seed QP room with 3 seated players at seats 0,1,2 (PlayerCount=3), call QuickPlay as a 4th user, expect `seat=3`, `gameStarted=true`, room.Status=`"playing"`. Use a setup with a recording broadcaster; assert `system:game_started` was broadcast.
- [x] `client/src/shared/api/rooms.ts` — Add `QuickPlayResponse { room: Room; seat: number; gameStarted: boolean }`. Change `quickPlay()` return type to `Promise<QuickPlayResponse>`.
- [x] `client/src/features/lobby/LobbyPage.tsx` — In `handleQuickPlay`, branch on `result.gameStarted`: true → `navigate('/game/' + result.room.id)`, false → `navigate('/rooms/' + result.room.id)`.
- [x] `client/src/features/lobby/LobbyPage.test.tsx` — Update `mockQuickPlay`/equivalent to resolve with `{ room, seat: 0, gameStarted: false }`. Add a test where it resolves with `gameStarted: true` and asserts `mockNavigate` is called with `/game/{id}`.
- [x] `client/` — Run `npx prettier --write .` before commit (memory: prettier-before-commit).

**Acceptance Criteria:**

- Given a player has no active room, when they POST `/rooms/quick-play`, then they receive `{room, seat, gameStarted}` with seat ∈ {0,1,2,3} and the seat is recorded in `room_players.seat`.
- Given seats 0 and 2 are occupied in the only available QP room, when a player calls QuickPlay, then they are seated at seat 1.
- Given a QP room has 3 seated players, when the 4th player calls QuickPlay, then the response has `gameStarted=true`, the room status is `"playing"` in the DB, `gameStarter.StartGame` is invoked, and `system:game_started` is broadcast to room participants.
- Given a player is auto-seated by QuickPlay, when they click a different empty seat in the room lobby, then `SelectSeat` moves them to that seat (existing behavior, no regression).
- Given the 4th joiner triggers auto-start, when the frontend reads `gameStarted=true` from QuickPlay, then it navigates to `/game/{id}` directly without round-tripping through `/rooms/{id}`.

### Review Findings

- [x] [Review][Patch] Quick Play joiner is invisible to existing room members until refetch — handler emits only `system:seat_updated`, which `roomLobbyStore.updatePlayerSeat` no-ops if the user is not yet in `players`. JoinRoom emits `system:player_joined`; QuickPlay should mirror that. [server/internal/room/handler.go:1213-1232]
- [x] [Review][Patch] `system:seat_updated` username can be empty when `FindPlayersByRoomID` errors — the `_` swallows the error and `username` falls through as `""`. Project-context.md requires wrapping with `%w`; at minimum log the error and continue. [server/internal/room/handler.go:1217-1224]
- [x] [Review][Defer] Auto-start broadcasts `system:game_started` even when `gameStarter.StartGame` returned an error — clients navigate to `/game/{id}` for a session that does not exist. Pre-existing pattern duplicated from the `SelectSeat` auto-start at handler.go:670-731. [server/internal/room/handler.go:1287-1298] — deferred, pre-existing
- [x] [Review][Defer] `pickFirstEmptySeat` returning `apperr.ErrRoomFull` is reachable on `player_count` counter drift; the QuickPlay retry loop only retries on `ErrRoomCodeTaken`/`ErrRoomNameTaken`, so the user gets an opaque 5xx instead of being routed to a different / new room. [server/internal/room/handler.go:1126-1135] — deferred, pre-existing counter-drift gap (see Story 2.5 review findings)
- [x] [Review][Defer] `LeaveRoom` does not gate on `room.Status == "waiting"` under lock, so a leave can race the auto-start tx and `gameStarter.StartGame` runs with a `seatInfo` containing a player who has already left. [server/internal/room/handler.go:443-542 vs handler.go:1234-1300] — deferred, pre-existing concurrency gap that the existing `SelectSeat` auto-start also exposes

## Design Notes

The auto-seat lives inside the **same** transaction as the join/create so a partial state ("player joined but no seat") is impossible. Seat selection is a deterministic linear scan 0→3 via the existing `FindPlayerBySeat` repo method — no new query, no new index. The auto-start branch is intentionally a copy of the SelectSeat auto-start block (handler.go:670–731) rather than a refactored helper: the shapes are similar but the surrounding context (no `previousSeat`, broadcast ordering) differs and a shared helper would obscure that. If a third auto-start trigger appears later, extract then.

The wire change `{data: Room}` → `{data: {room, seat, gameStarted}}` is acceptable because the only consumer is the frontend in this same PR; no other clients exist. The `seat` field is technically derivable from the players list but is included for symmetry with the auto-start case (where the client needs to know without re-fetching).

## Verification

**Commands:**

- `cd server && go test ./internal/room/...` — all room tests pass, including new ones
- `make lint` — golangci-lint + ESLint + Prettier all pass
- `cd client && npx prettier --write .` — formatting locked in
- `make test` — full suite green

**Manual checks:**

- Three browser sessions: each clicks Quick Play → all three pre-seated. The 4th session clicks Quick Play → all four navigate to `/game/{id}` automatically.
