---
title: 'Return to room after match (v2 — presence layer)'
type: 'feature'
created: '2026-06-16'
status: 'done'
context: ['{project-root}/_bmad-output/project-context.md', '{project-root}/_bmad-output/implementation-artifacts/spec-return-to-room-after-match.md']
baseline_commit: '6b8664153ddbb3daeb95d3c6dea6161634c96851'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** v1 reopens a finished room and re-seats returners, but with no presence concept the lobby cannot tell "returned" from "still on the result dialog": the owner Start button enables as soon as seats are occupied (a not-yet-returned player gets pulled into the next match), reopened rooms never reappear in already-loaded lobby grids (D144), and a player who did not return can be stranded under a stale result overlay or fail to follow a freshly started match (D145).

**Approach:** Add a server-side in-memory presence registry (roomID → set[userID]), seeded as players return/join and cleared on match start or room close, surfaced via a new `system:player_returned` WS event + `returnedUserIds` on the room detail payload. RoomPage shows "waiting to return" seats and gates the owner Start (client-side) on every seated human being present. Fold in D144 (lobby-grid upsert on reopen), D145 (overlay reset + match-started navigation), and D146 (distinct barred / already-started error copy).

## Boundaries & Constraints

**Always:**
- Presence registry is in-memory and best-effort — mirror the existing `LobbyDisconnectHandler` (struct + `sync.Mutex` + map), injected via DI in `main.go` and into `RoomHandler`.
- Mark a user present on `POST /rooms/:id/return` (returner) and on joining/seating into a `waiting` room (fresh joiner). Remove on kick and on leave; clear the whole room entry on match start (manual `StartMatch` + `autoStartIfFull`) and on room close (empty/owner-leave/lobby-timeout).
- Broadcast `system:player_returned {roomId, userId}` to room members when a returner is marked present. Add the event to BOTH contract files (`events.go` + `wsEvents.ts`) in the same commit.
- Add `ReturnedUserIds []uint json:"returnedUserIds"` / `returnedUserIds: number[]` to the room detail payload; populate from the registry in every builder that returns it (`GetRoom`, `GetRoomByCode`, `ReturnToRoom`).
- Owner Start gating is client-side: disabled until every seated human (`seat !== null && isBot !== true`) is in `returnedUserIds`. Reuse RoomPage's existing `allSeated` computation/label pattern; add a "waiting for players" label and a per-seat "waiting to return" badge on `SeatTile` (reuse its badge cluster).
- D144: reopened room must reappear in already-loaded lobby grids — make the lobby `system:room_updated` handler upsert (add-if-missing when `status === "waiting"`), matching the existing `system:room_created` add/de-dupe semantics.
- D145a: when a fresh `match_state` (phase not `match_end`) arrives while `matchEndData !== null`, MatchPage clears `matchEndData` and resets `overlayPhase` to `normal`.
- D145b: a player seated in a room receiving `system:match_started` navigates to `/match/:roomId` even when not on RoomPage (`currentRoomId === null`), using the event's `roomId`.
- D146: branch `handleReturnToRoom`'s catch on `FetchError.code` — distinct copy for `MATCH_ALREADY_STARTED` (409) and `NOT_IN_ROOM` (404), keep the generic message as fallback. Add keys to all four locales (plain punctuation, no em-dashes).

**Ask First:**
- Any DB migration or persistence of presence across server restarts.
- Tying presence to WS connect/disconnect (presence tracks returned/joined, not connection state — the pause/disconnect system is separate).

**Never:**
- No server-side hard-enforcement of presence in `StartMatch` — the registry is best-effort and empty after restart, so server enforcement would deadlock a legitimately-full reopened room. Gating is a client UX gate; the server keeps its existing all-seats-covered check. D145b is the fallback if a not-present seated player is started upon (stale client / race).
- No reservation timeout / auto-kick of not-yet-returned players; no auto-transfer of ownership from an AWOL owner (locked in v1).
- No zod schema for the new system event (room/system events use interfaces + manual dispatch guards, not zod).
- No changes to v1's reopen / re-seat / bot-clear / non-member-rejection behavior beyond adding presence hooks.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|---|---|---|---|
| Returner marked present | Member calls `/return` on reopened room | Registry adds caller; `system:player_returned` to room; response `returnedUserIds` includes caller | N/A |
| Owner Start gating | Reopened room, 4 seats occupied, 1 seated human not yet returned | Start disabled with "waiting" label; that seat shows "waiting to return" | N/A |
| All returned | Last absent seated human returns | `system:player_returned` flips owner Start to enabled (no refetch) | N/A |
| Fresh joiner fills vacated seat | New user joins waiting room | Marked present on join; counts toward Start gate | N/A |
| Kicked player | Owner kicks a member | Removed from registry; their `/return` still rejected (v1) → client routes to lobby with NOT_IN_ROOM copy | 404 NOT_IN_ROOM |
| Reopen visibility (D144) | Room reopens (`completed→waiting`) | Already-loaded lobby grids add the room (upsert) so strangers can discover/join it live | N/A |
| Stale overlay (D145a) | New match's `match_state` arrives while old result overlay up | `matchEndData` cleared, overlay reset to normal | N/A |
| Match started off-RoomPage (D145b) | Seated player not on RoomPage gets `system:match_started` | Navigates to `/match/:roomId` | N/A |
| Already-started return (D146) | Member clicks "Return to room" after next match started | `/return` 409 → distinct "match already started" copy | 409 MATCH_ALREADY_STARTED |

</frozen-after-approval>

## Code Map

- `server/internal/room/presence.go` (new) -- `PresenceRegistry` (map + `sync.Mutex`) with `Add/Remove/Clear/Present`; mirrors `lobby_disconnect.go`
- `server/internal/room/handler.go` -- registry field + ctor param; mark-present in `ReturnToRoom` (+ broadcast `player_returned`) and the join/seat handler; remove in `KickPlayer` / `LeaveRoom`; clear in `StartMatch` / `autoStartIfFull` and the empty-room close path; add `ReturnedUserIds` to `RoomDetailResponse` and populate in `GetRoom` / `GetRoomByCode` / `ReturnToRoom`
- `server/internal/ws/events.go` -- `SystemPlayerReturned` const + `PlayerReturnedPayload{RoomID, UserID}`
- `server/cmd/api/main.go` -- construct `PresenceRegistry`, wire the lobby-timeout close clear point, pass into `NewRoomHandler`
- `client/src/shared/types/wsEvents.ts` -- `SYSTEM_PLAYER_RETURNED` const + `PlayerReturnedPayload` interface
- `client/src/shared/types/apiTypes.ts` -- add `returnedUserIds: number[]` to `RoomDetail`
- `client/src/shared/hooks/useWsDispatch.ts` -- handle `SYSTEM_PLAYER_RETURNED` (currentRoomId gate → `markReturned`); D145b navigation on `SYSTEM_MATCH_STARTED`
- `client/src/shared/stores/roomStore.ts` -- `returnedUserIds` state + `setReturnedUserIds` / `markReturned`; add to `initialState` + `reset`
- `client/src/features/room/RoomPage.tsx` -- seed `returnedUserIds` from query; compute `allReturned`; extend Start `ctaDisabled` + label; pass `waitingToReturn` per seat
- `client/src/features/room/components/SeatTile.tsx` -- "waiting to return" badge prop
- `client/src/features/lobby/useRoomUpdates.ts` -- D144 upsert in the `system:room_updated` handler
- `client/src/features/match/MatchPage.tsx` -- D145a overlay reset; D146 `FetchError`-coded error copy
- `client/src/shared/i18n/{en,sr,hr,mk}.json` -- D146 distinct error keys

## Tasks & Acceptance

**Execution:**
- [x] `server/internal/room/presence.go` -- new `PresenceRegistry` mirroring `lobby_disconnect.go` -- isolated registry primitive
- [x] `server/internal/room/handler.go` -- inject registry; add/remove/clear at the return/join/kick/leave/start/close points; add + populate `ReturnedUserIds`; broadcast `player_returned` on return -- core server behavior
- [x] `server/internal/ws/events.go` -- `SystemPlayerReturned` + `PlayerReturnedPayload` -- contract (server)
- [x] `server/cmd/api/main.go` -- construct/inject registry; wire close clear point -- DI
- [x] `client/src/shared/types/{wsEvents.ts,apiTypes.ts}` -- event const/interface + `returnedUserIds` on `RoomDetail` -- contract (client)
- [x] `client/src/shared/stores/roomStore.ts` -- `returnedUserIds` state + `setReturnedUserIds`/`markReturned` (+ `initialState`/`reset`) -- presence store
- [x] `client/src/shared/hooks/useWsDispatch.ts` -- `player_returned` → `markReturned` (gated); `match_started` → navigate off-RoomPage (D145b) -- dispatch wiring
- [x] `client/src/features/room/{RoomPage.tsx,components/SeatTile.tsx}` -- seed presence, `allReturned` Start gate + label, per-seat "waiting to return" badge -- the UI
- [x] `client/src/features/lobby/useRoomUpdates.ts` -- D144 upsert-on-waiting -- lobby visibility
- [x] `client/src/features/match/MatchPage.tsx` -- D145a overlay reset on fresh `match_state`; D146 coded error copy -- match-side fixes
- [x] `client/src/shared/i18n/{en,sr,hr,mk}.json` -- D146 `MATCH_ALREADY_STARTED` + `NOT_IN_ROOM` keys -- localized copy
- [x] Tests -- `server/internal/room/presence_test.go` (registry unit) + extend `return_to_room_test.go` (present-on-return, removed-on-kick/leave, cleared-on-start, `returnedUserIds` in payload, `player_returned` broadcast); `RoomPage.test.tsx` (Start gated until all returned + waiting badge); `useRoomUpdates` upsert; `MatchPage` D145a/D146 -- regression gate

**Acceptance Criteria:**
- Given a reopened room where only some ex-players have returned, when the owner views it, then Start is disabled and each not-yet-returned seated human shows a "waiting to return" indicator.
- Given the last absent seated human clicks "Return to room", when the server broadcasts `system:player_returned`, then the owner's Start button becomes enabled without a refetch.
- Given a stranger already has the lobby grid loaded, when a finished room reopens, then it reappears in their grid and is joinable.
- Given a player is on the lobby (not RoomPage) and is seated in a room, when the match starts, then they are navigated into `/match/:roomId`.
- Given a new match has started under a stale result overlay, when its `match_state` arrives, then the overlay clears and the live table shows.
- Given a kicked player clicks "Return to room", then they see a "no longer in this room" message; given the next match already started, then they see a distinct "match already started" message.

## Spec Change Log

## Design Notes

- Presence ≠ membership: `room_players` rows survive the match (so returners reclaim their seats), so "returned vs still-on-result-dialog" must be tracked separately in memory. Presence ≠ connection: it is application-level (return/join), not WS connect/disconnect.
- Start gating is client-side by design (see **Never**). D145b is its safety net: if a not-present seated player is started upon (stale client / race), `system:match_started` still pulls them into the match.
- D144: prefer upserting inside the existing `room_updated` handler over emitting `room_created` on reopen — one robust frontend change, semantically correct since the room already existed.

## Verification

**Commands:**
- `make lint` -- expected: passes (Go + TS)
- `make test` -- expected: passes (`go test ./...` + `vitest run`), including new presence tests

**Manual checks:**
- `make dev`, finish a match with 4 clients: each clicks "Return to room"; confirm the owner's Start stays disabled with a per-seat "waiting to return" until all four are back, a separate browser sitting on the lobby sees the reopened room appear, and a client left on the lobby is pulled into the next match when it starts.

## Suggested Review Order

**Presence concept (start here)**

- Entry point — in-memory registry (roomID → set[userID]), best-effort, mirrors lobby-disconnect
  [`presence.go:19`](../../server/internal/room/presence.go#L19)

- Marks the returner present + broadcasts `system:player_returned`; response carries `returnedUserIds`
  [`handler.go:856`](../../server/internal/room/handler.go#L856)

- Presence cleared on match start (paired with add on return/join, remove on kick/leave)
  [`handler.go:2038`](../../server/internal/room/handler.go#L2038)

**WS contract (both files, same commit)**

- New event constant (server)
  [`events.go:223`](../../server/internal/ws/events.go#L223)

- New event constant + payload interface (client)
  [`wsEvents.ts:302`](../../client/src/shared/types/wsEvents.ts#L302)

- `returnedUserIds` on the room-detail payload
  [`apiTypes.ts:78`](../../client/src/shared/types/apiTypes.ts#L78)

**Presence state + dispatch (client)**

- Registry DI wired into the room + lobby-disconnect handlers
  [`main.go:167`](../../server/cmd/api/main.go#L167)

- Store presence state + idempotent `markReturned`
  [`roomStore.ts:65`](../../client/src/shared/stores/roomStore.ts#L65)

- `player_returned` → `markReturned` (room-gated); `match_started` records roomId for D145b
  [`useWsDispatch.ts:513`](../../client/src/shared/hooks/useWsDispatch.ts#L513)

**The UI — owner Start gate + waiting-to-return**

- Start gated on every seated human present (client-side; empty set = untracked fallback)
  [`RoomPage.tsx:679`](../../client/src/features/room/RoomPage.tsx#L679)

- Per-seat "waiting to return" badge
  [`SeatTile.tsx:173`](../../client/src/features/room/components/SeatTile.tsx#L173)

**D144 — reopened room reappears in already-loaded lobby grids**

- `room_updated` handler upserts (add-if-missing on `waiting`)
  [`useRoomUpdates.ts:66`](../../client/src/features/lobby/useRoomUpdates.ts#L66)

**D145 — stranded-player fixes**

- D145a — clear a stale result overlay when a fresh `match_state` arrives
  [`MatchPage.tsx:835`](../../client/src/features/match/MatchPage.tsx#L835)

- D145b — always-mounted navigator routes a seated player into a started match (clears sticky flag)
  [`useMatchStartRedirect.ts:18`](../../client/src/shared/hooks/useMatchStartRedirect.ts#L18)

- Navigator mounted app-wide for authenticated users
  [`WebSocketProvider.tsx:14`](../../client/src/shared/providers/WebSocketProvider.tsx#L14)

**D146 — distinct return-to-room error copy**

- Branch on `FetchError.code` for 409 vs 404
  [`MatchPage.tsx:1155`](../../client/src/features/match/MatchPage.tsx#L1155)

**Tests & locales (peripherals)**

- Registry unit test
  [`presence_test.go:12`](../../server/internal/room/presence_test.go#L12)

- Presence flow: present-on-return, removed-on-kick/leave, cleared-on-start, payload, broadcast
  [`return_to_room_test.go:210`](../../server/internal/room/return_to_room_test.go#L210)

- Start gated until all returned + waiting badge
  [`RoomPage.test.tsx:783`](../../client/src/features/room/RoomPage.test.tsx#L783)

- D146 distinct error copy across all four locales (en shown)
  [`en.json:640`](../../client/src/shared/i18n/en.json#L640)
