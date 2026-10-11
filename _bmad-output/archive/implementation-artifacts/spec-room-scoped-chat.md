---
title: "Room-Scoped Chat (third chat channel)"
type: feature
created: 2026-04-22
status: done
baseline_commit: 6ecd73c2cc8bb445325b85301a8550a819205f3d
context:
  - _bmad-output/project-context.md
  - _bmad-output/archive/implementation-artifacts/6-1-global-lobby-chat.md
  - _bmad-output/archive/implementation-artifacts/6-2-match-scoped-chat.md
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The room-waiting screen (`/rooms/:id`) has no chat. Today players can banter in the global lobby (Story 6.1) and inside a match (Story 6.2), but between joining a room and the game starting there is a silent gap — both seated and still-unseated players in that room have nowhere to coordinate.

**Approach:** Add a third chat channel `"room"` that broadcasts to every user in `room_players` for the given room (seated + unseated). Mirror the existing 6.1 / 6.2 pattern: same `action:chat_message` envelope with a new `channel` value + `roomId`, same `ChatPanel` component rendered inside `RoomLobby`, same ephemeral ring-buffer semantics, cleared on room unmount.

## Boundaries & Constraints

**Always:**

- WS contract stays in sync: any field added to [client/src/shared/types/wsEvents.ts](client/src/shared/types/wsEvents.ts) is added to [server/internal/ws/events.go](server/internal/ws/events.go) in the same commit.
- Recipient list is server-authoritative: look up members via the room repository; never trust a client-supplied member list.
- Server stamps `timestamp` as `RFC3339Nano` UTC. Silent drop on every failure path — no `error:` events. (Mirrors 6.1 AC #7 and 6.2 AC #3.)
- Reuse the existing `ChatPanel` component with an extended `channel` union — do NOT fork into a third panel file.
- i18n keys added to BOTH `en.json` and `sr.json` in the same commit.
- Immutable Zustand updates and 200-message ring buffer — identical to `globalMessages` / `matchMessages`.
- `data-testid` selectors in tests; no Tailwind class assertions.
- Run `npx prettier --write .` in `client/` before committing (standing feedback in auto-memory).

**Ask First:**

- If adding the room chat panel forces a fundamental layout rewrite of `RoomLobby.tsx` beyond a wrapper 2-col grid (e.g. rewriting the seat grid itself), HALT and confirm.
- If server membership gating turns out to require new room-repository methods beyond what `FindPlayersByRoomID` + `FindByID` already provide, HALT and confirm before adding repo surface.

**Never:**

- No DB persistence, no REST endpoint, no history on join — chat is ephemeral by design (consistent with 6.1 / 6.2).
- No unread-badge / collapsible-sidebar mechanism (the panel is always visible on the room page).
- No rate-limiting, moderation, or sanitisation (deferred with 6.1 D72 / 6.2 D78 — Phase 1 is intentionally unmoderated).
- No replay of room chat into the match — `MatchChatSidebar` stays on its own `matchMessages` partition.
- No chat visibility to players who are NOT in `room_players` for that room (a user who left the room must not receive its events).

## I/O & Edge-Case Matrix

| Scenario                                   | Input / State                                                                                        | Expected Output / Behavior                                                                                                     | Error Handling |
| ------------------------------------------ | ---------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------ | -------------- |
| Seated player sends message                | `{ channel: "room", roomId: 42, text: "hi" }`, sender in `room_players(42)`, room status `"waiting"` | Server broadcasts `system:chat_message` with `scope: "room"` to all userIDs in `room_players(42)`, including sender (own-echo) | N/A            |
| Unseated (joined-but-no-seat) player sends | Same as above, sender has `seat == nil`                                                              | Same broadcast — seat membership does not gate chat                                                                            | N/A            |
| Non-member sends to room chat              | Sender userID is NOT in `room_players(roomId)`                                                       | Silent drop, INFO log, no broadcast                                                                                            | Silent         |
| Room status is not `"waiting"`             | Room exists but `status != "waiting"` (game in progress / finished)                                  | Silent drop, INFO log                                                                                                          | Silent         |
| Unknown roomId                             | `roomId` does not resolve to any row in `rooms`                                                      | Silent drop, INFO log                                                                                                          | Silent         |
| Missing roomId                             | `channel: "room"` but `roomId == nil`                                                                | Silent drop, INFO log                                                                                                          | Silent         |
| Empty / whitespace / >500 runes text       | Any channel, invalid text                                                                            | No broadcast (reuses existing prologue)                                                                                        | Silent         |
| Recipient in match                         | Recipient userID appears in both `room_players(X)` AND is currently in-game                          | Recipient still receives room chat — the in-game exclusion applies to `global` only, not `room`                                | N/A            |
| Player leaves room                         | Route unmounts (leave / game-start navigation / close tab)                                           | `chatStore.roomMessages` reset to `[]` via `clearRoom()`                                                                       | N/A            |
| Game-start transition                      | `gameStarted` flag flips, client navigates to `/game/:id`                                            | RoomLobby unmounts → `clearRoom()` fires; GamePage mounts with empty `matchMessages`                                           | N/A            |

</frozen-after-approval>

## Code Map

- [server/internal/chat/handler.go](server/internal/chat/handler.go) — add `ChannelRoom` constant + `handleRoom`; extend dependency interface.
- [server/internal/chat/handler_test.go](server/internal/chat/handler_test.go) — add room-scope suite mirroring match tests.
- [server/internal/ws/events.go](server/internal/ws/events.go) — add `RoomID *uint` to `ChatMessageRequest`; widen `ChatMessagePayload.Scope` doc.
- [server/cmd/api/main.go](server/cmd/api/main.go) — construct a `RoomMembership` adapter wired to the room repo; pass into `chat.NewHandler`.
- [server/internal/room/repository.go](server/internal/room/repository.go) — reuse `FindByID` + `FindPlayersByRoomID` (no new methods expected).
- [client/src/shared/types/wsEvents.ts](client/src/shared/types/wsEvents.ts) — widen `ChatMessageRequest.channel` + add `roomId?: number`; widen `ChatMessagePayload.scope`.
- [client/src/shared/stores/chatStore.ts](client/src/shared/stores/chatStore.ts) — add `roomMessages` partition, `appendRoom`, `clearRoom`. No monotonic counter needed (panel is always visible).
- [client/src/shared/stores/chatStore.test.ts](client/src/shared/stores/chatStore.test.ts) — add room-partition tests + isolation from global/match.
- [client/src/shared/hooks/useWsDispatch.ts](client/src/shared/hooks/useWsDispatch.ts) — route `scope === "room"` → `appendRoom`. Reuse existing defensive payload validation.
- [client/src/shared/hooks/useWsDispatch.test.ts](client/src/shared/hooks/useWsDispatch.test.ts) — add room-dispatch + isolation tests.
- [client/src/features/chat/ChatPanel.tsx](client/src/features/chat/ChatPanel.tsx) — widen `channel` union to `"global" | "match" | "room"`, add `roomId?: number`; branch i18n keys + outgoing payload.
- [client/src/features/chat/ChatPanel.test.tsx](client/src/features/chat/ChatPanel.test.tsx) — add room-mode test cases.
- [client/src/features/lobby/RoomLobby.tsx](client/src/features/lobby/RoomLobby.tsx) — mount `<ChatPanel channel="room" roomId={id} />` inside a 2-col wrapper; call `clearRoom()` from the existing unmount `useEffect`.
- [client/src/features/lobby/RoomLobby.test.tsx](client/src/features/lobby/RoomLobby.test.tsx) — assert panel presence; assert `clearRoom` fires on unmount.
- [client/src/shared/i18n/en.json](client/src/shared/i18n/en.json) + [client/src/shared/i18n/sr.json](client/src/shared/i18n/sr.json) — add `room.chat.*` namespace (`title`, `placeholder`).

## Tasks & Acceptance

**Execution:**

- [x] `server/internal/ws/events.go` -- add `RoomID *uint` to `ChatMessageRequest`; update `Scope` doc to `"global" | "match" | "room"`.
- [x] `client/src/shared/types/wsEvents.ts` -- same-commit mirror: widen `channel` union, add `roomId?: number`, widen `scope`.
- [x] `server/internal/chat/handler.go` -- add `ChannelRoom = "room"`, new `RoomMembership` dependency (method `RoomMembers(roomID uint) ([]uint, waiting bool)` — returns non-nil userIDs iff room exists AND status is `"waiting"`), new `handleRoom` + dispatch branch.
- [x] `server/internal/chat/handler_test.go` -- extend fakes; cover every I/O matrix row (broadcast to all members, sender in members / not, unseated member, missing roomId, unknown room, non-waiting status).
- [x] `server/cmd/api/main.go` -- implement `RoomMembership` adapter backed by the existing `room.RoomRepository` (combines `FindByID` for status + `FindPlayersByRoomID` for userIDs); pass to `chat.NewHandler`.
- [x] `client/src/shared/stores/chatStore.ts` -- add `roomMessages`, `appendRoom`, `clearRoom` via the existing `appendWithCap` helper.
- [x] `client/src/shared/stores/chatStore.test.ts` -- room-partition append/cap/clear + cross-partition isolation.
- [x] `client/src/shared/hooks/useWsDispatch.ts` -- add `scope === "room"` branch → `appendRoom`. Keep the defensive payload validation unchanged.
- [x] `client/src/shared/hooks/useWsDispatch.test.ts` -- dispatch + isolation tests.
- [x] `client/src/features/chat/ChatPanel.tsx` -- widen prop union; route to correct store partition; branch title/placeholder i18n keys; validate `roomId` analogous to existing `matchId` guard.
- [x] `client/src/features/chat/ChatPanel.test.tsx` -- room-mode cases (seed `roomMessages`, submit payload shape, reject invalid `roomId`).
- [x] `client/src/features/lobby/RoomLobby.tsx` -- wrap existing content in a responsive 2-col grid at `md+`; mount `<ChatPanel channel="room" roomId={Number(id)} className="..." />` in the right column; stacked below on `sm`. Call `useChatStore.getState().clearRoom()` inside the existing unmount `useEffect` (co-located with `useRoomLobbyStore.getState().reset()`).
- [x] `client/src/features/lobby/RoomLobby.test.tsx` -- panel-presence assertion; `clearRoom` fires on unmount.
- [x] `client/src/shared/i18n/en.json` + `client/src/shared/i18n/sr.json` -- add `room.chat.{title,placeholder}` in parity.

**Acceptance Criteria:**

- Given two players are in the same room on `/rooms/:id` (one seated, one unseated) and the room status is `"waiting"`, when either sends a chat message, then both see it in their `ChatPanel` with server-stamped timestamp — including the sender's own echo.
- Given a third player is in a DIFFERENT room's `/rooms/:id`, when any player in the first room sends chat, then the third player's `chatStore.roomMessages` does not receive the event.
- Given a player leaves the room (button click, browser back, or navigating to `/game/:id` on match start), when `RoomLobby` unmounts, then `chatStore.roomMessages` is `[]`.
- Given `roomMessages` exists alongside `matchMessages` and `globalMessages`, when any message of any scope is appended, then only the matching partition grows (no cross-leak).
- Given a user not in `room_players(42)` sends `{ channel: "room", roomId: 42, ... }`, when the server processes the action, then no broadcast fires and nothing is sent to the sender.
- Given the room's `status != "waiting"`, when any user sends a room message, then no broadcast fires.
- Given `make lint` and `make test` run from repo root, when the build completes, then all Go packages and all vitest files are green with the new tests included.

## Spec Change Log

### 2026-04-22 — Review patch: id-keyed cleanup effect in RoomLobby

- **Finding:** EdgeCase-hunter flagged cross-room chat leak on `/rooms/A` → `/rooms/B` navigation — React Router v6 keeps `RoomLobby` mounted across `:id` param changes, so the `[]`-dep unmount cleanup calling `clearRoom()` + `useRoomLobbyStore.reset()` never fired. Room-A chat messages would have rendered under room-B's header.
- **Patch:** Changed the reset-on-unmount effect's dep array from `[]` to `[id]` in [client/src/features/lobby/RoomLobby.tsx:103-109](../../client/src/features/lobby/RoomLobby.tsx#L103-L109) so cleanup fires both on unmount AND on route-param change. Existing unmount tests still pass via the same cleanup.
- **KEEP (reviewer-validated decisions):** narrow new `RoomMembership` interface (one interface per dependency, chat package stays decoupled from room package); server-side `status == "waiting"` gate folded inside `RoomMembers` so `handleRoom` stays flat; unmount cleanup co-located with `useRoomLobbyStore.reset()` in a single `useEffect`; additive 2-col layout preserving `data-testid="room-lobby"` on the inner div so all prior `RoomLobby` tests survived untouched; boundary test at exactly 500 runes; `appendRoom` deliberately does NOT increment `matchMessagesReceivedTotal` (honours the "Never: no unread counter" constraint).

## Design Notes

**Why reuse `ChatPanel` again rather than a new component.** Stories 6.1 and 6.2 already proved the channel-branch pattern (prop union + switch on `channel` for title/placeholder/payload). A third branch is additive and keeps rendering/validation single-sourced. Forking would triple the surface area for future tweaks (timestamp hover, markdown, mentions).

**Why a fresh `RoomMembership` interface rather than folding into `GameMembership`.** The chat handler currently depends on `GameMembership` for in-game lookups (session manager). Room membership lives in the room repository, a different subsystem. Keeping a separate narrow interface avoids the chat package pulling in the room package directly and preserves the pattern: one interface per dependency.

**Status gate in the server, not just the client.** Mirrors Story 6.1's "in-game player can't send global" discipline — server authority, silent drop, INFO log. Clients don't need to know the rule; the server enforces it.

**No unread counter / collapse.** The room-waiting page is low-attention (static list, waiting for seats). A visible panel is simpler than the match-chat collapsible sidebar and matches the lobby-page placement.

## Verification

**Commands:**

- `make lint` from repo root -- expected: clean (Go + TypeScript + Prettier all pass; run `npx prettier --write .` in `client/` first).
- `make test` from repo root -- expected: all Go packages pass and all vitest files pass with the new chat + room-lobby tests added.
- `go vet ./...` in `server/` -- expected: clean.

**Manual checks:**

- Open three browser tabs with three different users: A and B join Room X (`/rooms/X`), C joins Room Y. A sends "hi" → B sees it in the room chat panel, C does NOT. B takes a seat, A stays unseated → B can still receive and send.
- Click "Start game" as the owner when all 4 seated → all seated players navigate to `/game/X`; room chat panel disappears; `chatStore.roomMessages` is `[]` on every client; match chat continues on `MatchChatSidebar`.
- Send a room message from A while C tries the same `roomId: X` from outside → C's send must be silently dropped server-side (grep `slog` output for "room send dropped").

## Suggested Review Order

**WS contract (start here — defines the shape of the change)**

- New `"room"` channel + `RoomID *uint` field on the client→server action payload; scope union widened.
  [`events.go:122`](../../../server/internal/ws/events.go#L122)

- TS mirror landed in the same commit — keeps contract parity.
  [`wsEvents.ts:260`](../../../client/src/shared/types/wsEvents.ts#L260)

**Server authorisation & dispatch**

- New `RoomMembership` interface is the narrow dependency seam between `chat` and `room` packages.
  [`handler.go:29`](../../../server/internal/chat/handler.go#L29)

- Dispatch switch — the entire change is additive, matching the `case ChannelMatch` shape 1:1.
  [`handler.go:99`](../../../server/internal/chat/handler.go#L99)

- `handleRoom` — silent-drop on every failure path, server-stamped RFC3339Nano, broadcast to the fixed member list.
  [`handler.go:200`](../../../server/internal/chat/handler.go#L200)

- Adapter folds the `status == "waiting"` gate into the membership lookup so `handleRoom` stays flat.
  [`main.go:180`](../../../server/cmd/api/main.go#L180)

- Wired into the composite action handler; `NewHandler` now takes four deps.
  [`main.go:119`](../../../server/cmd/api/main.go#L119)

**Client state + dispatch**

- Third `roomMessages` partition uses the shared `appendWithCap` helper — same 200-msg ring buffer as the other two.
  [`chatStore.ts:35`](../../../client/src/shared/stores/chatStore.ts#L35)

- `scope === "room"` branch with a defence-in-depth `currentRoomId` guard mirroring the match-scope pattern.
  [`useWsDispatch.ts:328`](../../../client/src/shared/hooks/useWsDispatch.ts#L328)

**UI — reuse `ChatPanel`, no fork**

- `ChatChannel` union widened to three values; `roomId` prop validated the same way `matchId` is.
  [`ChatPanel.tsx:14`](../../client/src/features/chat/ChatPanel.tsx#L14)

- Channel-aware title/placeholder keys + request-payload branch.
  [`ChatPanel.tsx:32`](../../client/src/features/chat/ChatPanel.tsx#L32)

**Mount site + lifecycle (includes review patch)**

- Wraps the original seat grid in a 2-col grid at `md+`, stacks below on `sm`. `data-testid="room-lobby"` preserved.
  [`RoomLobby.tsx:219`](../../client/src/features/lobby/RoomLobby.tsx#L219)

- Panel mount — always-visible by design (no collapse/unread badge).
  [`RoomLobby.tsx:355`](../../client/src/features/lobby/RoomLobby.tsx#L355)

- **Review patch:** dep array `[]` → `[id]` so cleanup fires on `/rooms/A` → `/rooms/B` navigation, not just unmount.
  [`RoomLobby.tsx:103`](../../client/src/features/lobby/RoomLobby.tsx#L103)

**Peripherals (tests + i18n)**

- New handler suite covers broadcast/own-echo/non-member/unknown-room/missing-id/boundary/recipient-in-match.
  [`handler_test.go:302`](../../../server/internal/chat/handler_test.go#L302)

- Store tests for the third partition including cross-partition isolation.
  [`chatStore.test.ts:134`](../../../client/src/shared/stores/chatStore.test.ts#L134)

- Dispatcher tests for scope=room happy-path and null-guard drop.
  [`useWsDispatch.test.ts:562`](../../../client/src/shared/hooks/useWsDispatch.test.ts#L562)

- ChatPanel room-mode cases — send payload shape, rejected `roomId` values, partition isolation.
  [`ChatPanel.test.tsx:220`](../../client/src/features/chat/ChatPanel.test.tsx#L220)

- RoomLobby panel presence + `clearRoom()`-on-unmount assertion.
  [`RoomLobby.test.tsx:612`](../../client/src/features/lobby/RoomLobby.test.tsx#L612)

- i18n keys added in parity; no other locale work needed for Phase 1.
  [`en.json:80`](../../../client/src/shared/i18n/en.json#L80), [`sr.json:80`](../../../client/src/shared/i18n/sr.json#L80)
