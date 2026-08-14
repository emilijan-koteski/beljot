# Story 11.4: Friend Whisper Chat

Status: ready-for-dev

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a player,
I want to send private "whisper" messages to my friends from the lobby, a room, or a match,
so that I can chat one-on-one with people I know — but never with someone I'm currently playing with.

## Acceptance Criteria

**Epic source:** `_bmad-output/planning-artifacts/epics.md#Story-11.4-Friend-Whisper-Chat` (FR61) + `sprint-change-proposal-2026-08-14.md §4d`. The six epic ACs are expanded below. The one substantive deviation from the epic's literal wording — the WS event **prefix** (`system:whisper`, not `event:whisper`) — is recorded and justified in Dev Notes → **Design Decision D1** (**PO-confirmed 2026-08-14**).

1. **AC1 — `/w` command sends a private whisper.** In any chat input (lobby / room / match dock), typing `/w <friendUsername> <message>` sends a private whisper: the client parses the `/w ` prefix, extracts the target username and message, and emits `action:whisper` `{ toUsername, text }` (NOT `action:chat_message`). The server validates and delivers a `system:whisper` message privately to **both** participants (own-echo to the sender + push to the recipient), appearing only in their shared whisper thread. The `/w` command, target username, and message text are **never** broadcast to any other player. Text is trimmed and rune-count-capped at 500 (mirrors chat, `chat/handler.go:83-92`); empty text is ignored client-side; a self-whisper (`toUsername` == caller) is ignored/rejected.
2. **AC2 — Friends-only (server-authoritative).** The server resolves `toUsername` → target user via `user.UserRepository.FindByUsername`, then checks `friend.Repository.AreFriends(sender, target)` (Story 11.2). A non-friend (or unknown username) → `error:not_friends` sent to the sender only; the client shows an inline hint ("You can only whisper friends"). The friend check is authoritative on the server — never a client-only gate.
3. **AC3 — Anti-collusion: cannot whisper someone in your current room/match.** If the target is a friend **currently in the SAME active room or match** as the sender (teammate or opponent), the server rejects with `error:whisper_blocked_in_game` ("You can't whisper someone you're currently playing with"). Enforced server-side (authoritative) using the presence registry — via `room.FindPlayerRoom(sender)` and `room.FindPlayerRoom(target)` sharing the same non-nil `RoomID` (covers both `waiting` rooms and `playing` matches in one query); friends in the lobby or in a **different** room/match remain valid targets. This is NOT merely hidden in the client.
4. **AC4 — Offline recipient rejected (real-time only).** If the target friend is not currently connected, the server rejects with `error:whisper_recipient_offline`. This MUST be an explicit `hub.IsConnected(targetID)` pre-check — `SendToUser` is a silent no-op for offline users, so there is no delivery-failure feedback to rely on. Whispers are real-time only; there is no offline inbox (consistent with the Epic 6 chat model).
5. **AC5 — Distinct rendering + channel switching.** Open whisper threads render in the chat panel as **visually distinct (pink-tinted) bubbles**, labelled with the friend's username. The player can switch between the primary channel (lobby/room/match) and each open whisper thread via a **tab control and the `Tab` key** (Valorant-style channel cycling). Each thread is keyed by the friend and shows an unread indicator when not active.
6. **AC6 — Ephemeral (no persistence).** Whisper threads are **not persisted server-side** (no DB table, no history) — the server only routes live messages, exactly like Epic 6 chat. When a participant leaves the lobby/room/match or disconnects, threads are ephemeral (client-side state is cleared on the same lifecycle as the other chat channels). No message is ever written to Postgres.
7. **AC7 — WS contract (D1) + drift gate.** The whisper events are `action:whisper` (client→server) + `system:whisper` (server→client), plus `error:not_friends`, `error:whisper_blocked_in_game`, `error:whisper_recipient_offline`. All are added to **both** contract files (`server/internal/ws/events.go` + `client/src/shared/types/wsEvents.ts`) in the **same commit**. Because these use `action:`/`system:`/`error:` prefixes (not `event:`), they incur **zero** drift-gate touchpoints (no golden JSON, no Zod schema, no contract-test rows) — verified against the chat/emote precedent. See D1.
8. **AC8 — i18n & quality gates.** All new user-facing strings (whisper hints/errors, tab labels, placeholder) are in **all four** locales (`en`/`sr`/`mk`/`hr`), `mk` all-Cyrillic, no em dash in `mk`/`sr`/`hr`, parity test green. `make lint` + `make test` pass.

## Tasks / Subtasks

- [ ] **Task 0: Branch setup — whole-epic-on-one-branch**
  - [ ] **Continue on the current branch `feat/11-3-public-player-profiles`** (do NOT cut a new branch). Per user direction, Epic 11 (11-1/11-2/11-4/11-5) ships as ONE feature/PR on this branch (same pattern as 9.7+9.8). **Hard dependency: Story 11.2** must be implemented first (or in the same branch) — this story consumes `friend.Repository.AreFriends`, `user.UserRepository.FindByUsername`, and (client) the friend-list query. Do 11.2 before 11.4 in the on-branch order.

- [ ] **Task 1: Backend — whisper WS contract** (AC: #1, #2, #3, #4, #7)
  - [ ] Add to `server/internal/ws/events.go`: `const ActionWhisper = "action:whisper"`, `const SystemWhisper = "system:whisper"`, and error consts `ErrorNotFriends = "error:not_friends"`, `ErrorWhisperBlockedInGame = "error:whisper_blocked_in_game"`, `ErrorWhisperRecipientOffline = "error:whisper_recipient_offline"`. Payload structs (mirror `ChatMessageRequest`/`ChatMessagePayload` at `events.go:434-448`):
    ```go
    type WhisperRequest struct {  // client → server
        ToUsername string `json:"toUsername"`
        Text       string `json:"text"`
    }
    type WhisperPayload struct {  // server → client (sent to BOTH participants)
        FromUserID   uint   `json:"fromUserId"`
        FromUsername string `json:"fromUsername"`
        ToUserID     uint   `json:"toUserId"`
        ToUsername   string `json:"toUsername"`
        Message      string `json:"message"`
        Timestamp    string `json:"timestamp"` // RFC3339Nano UTC, like chat
    }
    ```
  - [ ] Mirror the consts + a `WhisperPayload` TS interface in `client/src/shared/types/wsEvents.ts` (SAME commit). **`action:`/`system:`/`error:` are all OUTSIDE the drift gate** — do NOT add a golden JSON, a Zod schema, or a contract-test row (verified: chat/emote `system:*`/`action:*` events have none; see `events.go:336-349`, `wsEvents.ts:468-472`).

- [ ] **Task 2: Backend — whisper handler + action-router wiring** (AC: #1, #2, #3, #4, #6)
  - [ ] Add whisper handling in the `chat` package (it already owns the `action:*` chat pipeline, `userRepo`, and the hub) — a new `chat/whisper_handler.go` (or a method on the existing handler) that plugs into the composite action router alongside `action:chat_message` / `action:emote` (see `chat/handler.go:72-104` `HandleAction`; wired in `cmd/api/main.go` ~`:319-333`). It must **return silently** when `msg.Type != ws.ActionWhisper` (composite-router safety).
  - [ ] Inject narrow interfaces (keep it unit-testable + avoid import cycles — `room`/`match` must not be imported concretely if it risks a cycle; use small local interfaces):
    ```go
    type FriendChecker interface { AreFriends(a, b uint) (bool, error) }
    type PresenceLocator interface { FindPlayerRoom(userID uint) (*room.RoomPlayer, error) } // or a bool SameRoomOrMatch(a,b)
    type Notifier interface { IsConnected(userID uint) bool; SendToUser(userID uint, msg []byte) }
    ```
  - [ ] Handler flow for `action:whisper`:
    1. Unmarshal `WhisperRequest`; trim; validate `text != "" && utf8.RuneCountInString(text) <= 500`; ignore self-target (`toUsername` == sender's username).
    2. Resolve target: `userRepo.FindByUsername(toUsername)`; nil → `error:not_friends` (do not disclose "no such user" — a non-existent user isn't a friend).
    3. `friend.AreFriends(senderID, targetID)` → false → `error:not_friends`.
    4. **Anti-collusion:** `FindPlayerRoom(senderID)` and `FindPlayerRoom(targetID)`; if both non-nil and same `RoomID` → `error:whisper_blocked_in_game`. (Covers waiting rooms AND active matches — `FindPlayerRoom` matches `waiting` OR `playing`, `room/gorm_repo.go:164-177`. Optionally also short-circuit via `match.Manager.MatchParticipantsByUser` for the in-match case.)
    5. **Offline:** `notifier.IsConnected(targetID)` false → `error:whisper_recipient_offline`.
    6. Build `WhisperPayload` (`Timestamp: time.Now().UTC().Format(time.RFC3339Nano)`) and `SendToUser` to **both** the target and the sender (own-echo, so the sender's thread updates) via `buildMessage(ws.SystemWhisper, payload)`.
  - [ ] Errors go to the SENDER only: `hub.SendToUser(senderID, buildMessage(ws.ErrorXxx, map[string]string{"message": ...}))` (the `sendError` pattern at `match/live_match.go:1247-1250`). Nothing is persisted (ephemeral — no model/repo/migration, mirroring chat).

- [ ] **Task 3: Backend tests** (AC: #1, #2, #3, #4)
  - [ ] Handler unit tests (mirror `chat/handler_test.go`: a `hubSpy` capturing `SendToUser`/`IsConnected`, plus fakes for `FriendChecker`, `PresenceLocator`, `userRepo`): friend whisper → `system:whisper` delivered to BOTH sender and target; non-friend → `error:not_friends` (to sender only); unknown username → `error:not_friends`; same-room/same-match target → `error:whisper_blocked_in_game`; offline target → `error:whisper_recipient_offline` (and NOT delivered); text > 500 runes rejected; empty/self ignored; a wrong `msg.Type` is a silent no-op.
  - [ ] One real-websocket integration test in the `ws_test.go` style (`httptest.Server` + two real `coder/websocket` clients per project rule): sender whispers an online friend in a different context → the second client receives exactly one `system:whisper` with the right payload, and a third uninvolved client receives nothing.

- [ ] **Task 4: Frontend — whisper store surface** (AC: #1, #5, #6)
  - [ ] Extend `client/src/shared/stores/chatStore.ts` (or a small dedicated `whisperStore.ts`) with a per-friend thread structure — `whisperThreads: Record<string, WhisperMessage[]>` keyed by the other participant's username (or userId), an `activeChannel` selector (primary | `whisper:<username>`), and per-thread unread counts. Add `appendWhisper(payload)`, `clearWhispers()`, `markThreadRead(key)`. Cap each thread with the existing `appendWithCap`/`MAX_MESSAGES` (200) pattern (`chatStore.ts:37-46`). This is net-new surface — today `chatStore` has only three flat channel arrays and no active-channel/unread concept. Clear whisper state on the same lifecycle the other channels reset (navigation away / disconnect).

- [ ] **Task 5: Frontend — `/w` parsing, tabbed switcher, pink bubbles** (AC: #1, #5)
  - [ ] In `client/src/features/chat/ChatDock.tsx` `send()` (~`:157-170`): detect a `/w ` prefix, parse `<username>` + `<message>`, and emit `sendWs(ACTION_WHISPER, { toUsername, text })` instead of `ACTION_CHAT_MESSAGE`. Reject an empty message client-side; keep the 500-char slice. (No `/` command parsing exists today — this is net-new.)
  - [ ] Add a **tab/channel switcher** above the message list: the primary channel (the dock's `variant`) plus one tab per open whisper thread, cycled with the **`Tab` key** and clickable. The active tab drives which message slice renders. Extend the existing testid convention (`${testIdRoot}-...`) with `whisper-tab-<username>`, `whisper-tab-active`, etc.
  - [ ] Render whisper bubbles in a **pink** variant of `ChatLine` (`ChatDock.tsx:371-380`) — add a pink token set analogous to the `.chat-dock-match` felt re-skin (`ChatDock.tsx:177-180`); label each thread with the friend's username. Never render whisper content in the primary channel.

- [ ] **Task 6: Frontend — WS dispatch (whisper + errors)** (AC: #1, #2, #3, #4)
  - [ ] Add a `SYSTEM_WHISPER` case to `dispatchSystemEvent` in `client/src/shared/hooks/useWsDispatch.ts` (~`:558`, next to `SYSTEM_CHAT_MESSAGE` at `:800`): validate the payload defensively (`typeof fromUserId === "number"`, etc. — never JS-truthiness on Go numerics), determine the thread key (the OTHER participant relative to the current user), and `appendWhisper`. Open/focus the thread tab if new.
  - [ ] Add `error:not_friends` / `error:whisper_blocked_in_game` / `error:whisper_recipient_offline` handling in `dispatchErrorEvent` (~`:868-899`): surface a localized inline hint or `toast.error` (mirror the `ERROR_SURRENDER_EXHAUSTED` toast branch at `:875-879`). Import the new consts at the top (`:84-101`).

- [ ] **Task 7: i18n** (AC: #8)
  - [ ] Add a `whisper.*` block to all four locales `client/src/shared/i18n/{en,sr,mk,hr}.json`: the `/w` input placeholder/help, tab label ("Whisper to {{username}}"), and the three error hints (`notFriends`, `blockedInGame`, `recipientOffline`). `mk` all-Cyrillic; NO em dash in `mk`/`sr`/`hr`. `i18n.parity.test.ts` green.

- [ ] **Task 8: Full validation gates** (AC: #8)
  - [ ] `make lint` + `make test` green (Go real-websocket test RUNS; client vitest for the store, `/w` parse, tab-cycle, dispatch). Update File List + Completion Notes. If the WS integration test needs the dev DB, run against `:5433`; the whisper path itself is DB-free (ephemeral) but `FindPlayerRoom`/`FindByUsername`/`AreFriends` touch Postgres.

## Dev Notes

### Design Decisions (READ FIRST)

- **D1 — WS prefix: `system:whisper`, NOT `event:whisper` (deviation from the epic's literal wording, justified).** The epic AC says "new `event:whisper` WS event, **analogous to the Epic 6 chat events**." But the Epic 6 chat events are actually `action:chat_message` (client→server) + **`system:chat_message`** (server→client) — there is no `event:chat_message`. `event:*` is reserved for **in-match game-state** payloads with an ordering contract, and it is the ONLY prefix inside the WS drift gate (goldens + Zod schemas + contract tests). A whisper is an ephemeral platform message, structurally identical to chat/emote (both `action:`+`system:`). **Decision:** follow the chat/emote precedent — `action:whisper` + `system:whisper` — which (a) honors the AC's stated intent ("analogous to Epic 6 chat"), (b) keeps whisper out of the drift gate (2-file diff, not 6), and (c) avoids giving an ephemeral message the ordering semantics of `event:`. **PO-confirmed 2026-08-14: `system:whisper` accepted** — proceed with `action:whisper` + `system:whisper` (zero drift-gate touchpoints). The literal `event:whisper` is NOT used.
- **D2 — Friends dependency is hard (Story 11.2).** The friend model does not exist until 11.2. The whisper friend-check consumes `friend.Repository.AreFriends(a, b)` (defined in 11.2 for exactly this reason) and, client-side, the friend-list query for the whisper target UX. 11.2 must land first on this shared branch.
- **D3 — Anti-collusion is a server-side presence check, one query.** "Same active room OR match" collapses to "same `RoomID`" because a live match is always tied to a `playing` room. `room.FindPlayerRoom(userID)` returns the user's row for a room in `waiting` OR `playing` status (`room/gorm_repo.go:164-177`) — so comparing `FindPlayerRoom(sender).RoomID == FindPlayerRoom(target).RoomID` (both non-nil) authoritatively covers both cases. The rule MUST be enforced on the server (the point is to prevent card-info collusion — a client-hidden check is defeatable).
- **D4 — Offline must be an explicit pre-check.** `hub.SendToUser` silently drops for offline users (`hub.go:178-185`) — there is no send-failure signal. `error:whisper_recipient_offline` requires calling `hub.IsConnected(targetID)` before building the payload.
- **D5 — Ephemeral, like chat.** No model, repository, migration, or DB write. The server routes live `system:whisper` messages and forgets them. Client threads live only in memory and clear on the normal chat lifecycle.

### Backend implementation notes

- **Chat precedent (mirror it):** `server/internal/chat/handler.go` — `HandleAction(client, msg)` returns silently on a non-matching type, validates rune count (not bytes), resolves username via `userRepo.FindByID`, builds a payload with `time.Now().UTC().Format(time.RFC3339Nano)`, and broadcasts. `buildMessage(type, payload)` (`chat/handler.go:251-263`) marshals `ws.WSMessage{Type, Payload}`. Whisper reuses all of this shape.
- **Action-router composition:** inbound `action:*` is dispatched (on a goroutine — `ws/router.go:19`, `hub.go:161`) to a composite handler chaining chat + emote + the session manager. Add whisper to that chain (extend the chat handler or add a sibling in the `chat` package and register it). Because dispatch is per-goroutine, cross-action ordering isn't guaranteed globally (pre-existing, documented) — irrelevant to a single whisper.
- **Presence primitives:** `room.FindPlayerRoom(userID) (*RoomPlayer, error)` (authoritative, both statuses); alternatives `match.Manager.IsUserInMatch`/`MatchParticipantsByUser` (in-memory, match-only), `hub.IsConnected`/`ConnectedUserIDs` (online). Inject via narrow interfaces to avoid import cycles and keep tests hub-free.
- **Error emission:** to the sender only, `hub.SendToUser(senderID, buildMessage(ws.ErrorXxx, map[string]string{"message": msg}))`. `error:*` is outside the drift gate. Chat/emote today silently drop invalid sends — whisper deliberately emits errors, a small extension of that path.
- **Username resolution:** `user.UserRepository.FindByUsername(username)` (`user/gorm_repo.go:67`) turns the typed `/w <username>` into a userID. Username is case-sensitive `VARCHAR(20)` — the whisper target must match exactly (no ILIKE here; that's search's job).

### Frontend implementation notes

- **Stack facts:** one multiplexed WS per client, distinguished by `type` (`useWebSocket.ts:61-64`, `useWsSendMessage()`); each `send()` is a discrete message (not batched). Dispatch splits on prefix in `useWsDispatch.ts` (`:112-126`). `chatStore` is three flat capped arrays with NO active-channel/unread concept (unread is tracked locally per `ChatDock`). The tabbed switcher, Tab-cycling, `/` command parser, and pink-bubble variant are **all net-new** — none exist today.
- **ChatDock:** single floating dock instantiated per `variant` (`"lobby"|"room"|"match"`) via `LobbyChatDock`/`RoomChatDock`/`MatchChatDock`. `ChatLine` styles mine vs others (`:371-380`); the `.chat-dock-match` re-skin (`:177-180`) is the template for a pink whisper skin. testid root convention: `${testIdRoot}-{dock,fab,list,input,send,...}`.
- **Never JS-truthiness on Go numerics/bools** — validate `system:whisper` payload fields with `typeof === "number"`/`Number.isInteger`; a real userId of any value is legitimate.

### i18n notes

- Four locales in `client/src/shared/i18n/` (`["en","hr","sr","mk"]`, fallback `en`). Existing chat keys live under `chat.*`, `room.chat.*`, `lobby.chat.*`, `match.chat.*`; add a new `whisper.*` namespace across all four. `i18n.parity.test.ts` enforces 1:1 leaf parity + non-empty. `mk` all-Cyrillic; no em dash (`—`) in `mk`/`sr`/`hr` (convention, not an automated lint — comply manually). `{{username}}` interpolation.

### Testing standards summary

- **Go:** WebSocket **handler unit tests** use a `hubSpy`/`Broadcaster` fake (see `chat/handler_test.go:95-136`) + fakes for the friend/presence/user deps — no real hub. **End-to-end WS tests** MUST use `httptest.Server` + a real `coder/websocket` client (project rule; template `ws/ws_test.go` — `setupTestServer`, `dialWS`, `sendAuthMessage`, `readMessage`). testify `assert`/`require`.
- **Client:** Vitest + Testing Library; reset stores in `beforeEach`; feed synthetic `system:whisper`/`error:*` messages to `useWsDispatch` and assert store/thread updates; component tests for the `/w` parse (assert `ACTION_WHISPER` payload), tab-cycle (Tab key), and pink-bubble rendering via `data-testid`. `test-utils.tsx` (`makeUser`, `QueryWrapper`, `TestProviders`).

### Known Traps

- **`system:` not `event:`** (D1) — do NOT add drift-gate goldens/Zod/contract rows; but DO put both contract files in the same commit.
- **Anti-collusion is server-authoritative** (D3) — the `FindPlayerRoom` same-`RoomID` check must be on the server; a client-only block is defeatable and defeats the purpose.
- **Offline pre-check** (D4) — `IsConnected` before send; `SendToUser` gives no failure signal.
- **Ephemeral** (D5) — no DB anywhere; do not add a model/repo/migration.
- **Own-echo** — send `system:whisper` to BOTH participants so the sender's thread renders (chat does the same; forgetting it leaves the sender's own message invisible).
- **Friend dependency** — `AreFriends` comes from 11.2; land 11.2 first on the branch.
- **Net-new UI** — tabs, Tab-key cycling, `/w` parser, pink bubbles, per-thread unread all have zero precedent; budget UI + test time.
- **Never JS-truthiness on Go numerics/bools**; **one story = one branch is overridden** for this epic (whole epic on one branch) — but still file unrelated bugs to `deferred-work.md`.

### Project Structure Notes

- **New backend:** `server/internal/chat/whisper_handler.go` (+ tests) — or whisper methods on the existing chat handler; additions to `server/internal/ws/events.go` (whisper action/system + 3 error consts + payloads), `server/cmd/api/main.go` (wire the whisper handler into the action-router chain with the friend repo + presence locator). NO migration, NO model, NO repository (ephemeral).
- **New frontend:** whisper thread state in `client/src/shared/stores/chatStore.ts` (or a new `whisperStore.ts`); `/w` parse + tab switcher + pink bubbles in `client/src/features/chat/ChatDock.tsx`; a `SYSTEM_WHISPER` + three `error:*` cases in `client/src/shared/hooks/useWsDispatch.ts`; the whisper consts + `WhisperPayload` type in `client/src/shared/types/wsEvents.ts`; `whisper.*` in four i18n JSONs. Conforms to feature-folder + `shared/` conventions.

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-11.4-Friend-Whisper-Chat] — user story + 6 ACs (FR61).
- [Source: _bmad-output/planning-artifacts/sprint-change-proposal-2026-08-14.md §4c/§4d] — FR61; `/w` command; anti-collusion server-side; pink bubbles + Tab switching; ephemeral, online-only.
- [Source: _bmad-output/implementation-artifacts/11-2-friend-requests-and-friend-list.md] — friend model + `friend.Repository.AreFriends` (the friend-check dependency); friend-list query.
- [Source: _bmad-output/implementation-artifacts/6-1-global-lobby-chat.md + 6-2-match-scoped-chat.md] — the ephemeral chat precedent (note: their File Lists cite pre-refactor paths; current paths are `chat/handler.go`, `ChatDock.tsx`, `MatchChatDock.tsx`, `matchStore`).
- [Source: server/internal/chat/handler.go:72-104,251-263] — `HandleAction`, rune-count validation, `buildMessage`, broadcast pattern.
- [Source: server/internal/chat/handler_test.go:20-136] — `hubSpy` + fakes handler-test template.
- [Source: server/internal/ws/events.go:5-9,336-349,383-448] — prefix conventions; `system:*`/`error:*` outside the drift gate; chat/emote payload precedent.
- [Source: server/internal/ws/hub.go:178-232] — `SendToUser` (silent no-op offline), `IsConnected`, `ConnectedUserIDs`.
- [Source: server/internal/room/gorm_repo.go:164-177] — `FindPlayerRoom` (waiting OR playing → same-room/match presence check).
- [Source: server/internal/match/live_match.go:615-672,1247-1250] — `IsUserInMatch`/`MatchParticipantsByUser`; `sendError` per-user error pattern.
- [Source: server/internal/user/gorm_repo.go:67] — `FindByUsername` (resolve `/w <username>`).
- [Source: server/internal/ws/ws_test.go] — real-websocket integration test template.
- [Source: client/src/features/chat/ChatDock.tsx:157-180,371-380] — `send()` (add `/w` parse), `.chat-dock-match` re-skin (pink template), `ChatLine` bubbles.
- [Source: client/src/shared/stores/chatStore.ts:5,37-46] — flat channels + `appendWithCap`/`MAX_MESSAGES` (net-new whisper thread structure).
- [Source: client/src/shared/hooks/useWsDispatch.ts:558,800-831,868-899] — `dispatchSystemEvent`/`dispatchErrorEvent` (add `system:whisper` + 3 `error:*` cases).
- [Source: client/src/shared/hooks/useWebSocket.ts:61-64] — single multiplexed WS send.
- [Source: client/src/shared/types/wsEvents.ts:468-472,517-561] — WS const/type mirror; `system:*` outside gate; chat payload shape.
- [Source: client/src/shared/i18n/*.json + i18n.parity.test.ts] — locales + parity/quality enforcement.
- [Source: _bmad-output/project-context.md] — WS contract rule (both files same commit), real-websocket test rule, "never JS-truthiness on Go zero values", i18n rules, prefix conventions.

## Dev Agent Record

### Agent Model Used

_TBD by dev-story_

### Debug Log References

### Completion Notes List

- Ultimate context engine analysis completed — comprehensive developer guide created.

### File List
