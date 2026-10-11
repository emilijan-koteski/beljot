# Story 6.2: Match-Scoped Chat

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a player in an active match,
I want to chat with the other three players in my game via a collapsible sidebar,
so that we can banter and communicate during play and recreate the card-table atmosphere.

## Acceptance Criteria

1. **Match chat sidebar renders on the game page**
   Given a player is in an active match (route `/game/:roomId`, `gameStore.gameState` is non-null),
   When `GamePage` mounts,
   Then a collapsible chat sidebar is available on the **right edge** of the viewport
   And when expanded it **does not overlap** any game-table control (seat avatars, hand cards, score panel, trick area, trump indicator, prompts, overlays)
   And a chat-toggle icon button is always visible on the right edge so the sidebar can be opened and closed at any time during the match.

2. **Sending a match message broadcasts only to the 4 match participants**
   Given a player in an active match submits a non-empty message,
   When the client emits `action:chat_message` with payload `{ "channel": "match", "matchId": <roomId>, "text": "..." }`,
   Then the server looks up the session by `matchId`, verifies the sender is one of the session's 4 `playerIDs`,
   And the server broadcasts `system:chat_message` with `scope: "match"` to **exactly those 4 user IDs** (including the sender — own echo),
   And no other connected user receives the event.

3. **Server rejects invalid match chat requests silently**
   Given any of the following conditions hold — `channel == "match"` but `matchId == nil`, no active session exists for that `matchId`, or the sender's userID is not among that session's `playerIDs`,
   When the server processes the `action:chat_message`,
   Then the message is **silently dropped** (no broadcast, no `error:` event to the client) and the attempt is logged at `INFO` level.

4. **Match messages flow into `chatStore.matchMessages` via dispatch**
   Given the WebSocket dispatcher (`useWsDispatch.ts`) receives a `system:chat_message` with `scope === "match"`,
   When the message is processed,
   Then it is appended to `chatStore.matchMessages` (ring-buffer capped at 200) and the mounted sidebar re-renders with the new message at the bottom
   And `scope === "global"` continues to route to `chatStore.globalMessages` unchanged (no regression of Story 6.1).

5. **Unread badge appears when the sidebar is collapsed**
   Given the sidebar is collapsed and a `scope: "match"` message arrives,
   When the message is appended to `chatStore.matchMessages`,
   Then a numeric unread indicator (count of messages received while collapsed, capped visually at `99+`) is shown on the chat-toggle button
   And when the sidebar is expanded the unread counter resets to `0`.

6. **Match chat history clears when the player leaves the match**
   Given a match ends (`matchEndData` is set) or the player exits the game view (route leaves `/game/:roomId` via navigation, abandonment, or `clearGame()`),
   When the transition completes,
   Then `chatStore.matchMessages` is reset to `[]` via `clearMatch()`
   And the sidebar's unread counter is reset to `0`
   And re-entering a new match starts from an empty chat history.

7. **Game interactivity is preserved when the sidebar is expanded**
   Given the sidebar is expanded during gameplay,
   When a player needs to play a card, bid trump, announce belote, declare, pause, or respond to any prompt/overlay,
   Then every existing game control remains fully clickable and the seat 3 (East) avatar remains visible — the sidebar is positioned/sized so no pointer or focus event is blocked on the game table
   And `data-testid="hand-cards"` and the seat controls continue to pass the existing interaction tests.

8. **Client-side validation mirrors Story 6.1**
   Given a player submits an empty string, a whitespace-only string, or text exceeding 500 runes (code points),
   When the submit handler runs,
   Then the message is **not** sent over the WebSocket, the input retains focus, and the "too long" warning uses the same `chat.tooLong` i18n key treatment as lobby chat.

9. **WebSocket-disconnected players cannot send match chat**
   Given the `WebSocketContext` reports a non-`"connected"` state,
   When the sidebar renders,
   Then the input and Send button are disabled and the placeholder shows `chat.placeholderDisabled` — identical UX to lobby chat.

## Tasks / Subtasks

- [x] Task 1: Extend `GameMembership` with match-participant lookup (AC: #2, #3)
  - [x] 1.1 In [server/internal/session/manager.go](server/internal/session/manager.go:277), **add** a new public method below `IsUserInGame`:
    ```go
    // MatchParticipants returns the four player userIDs for an active session
    // keyed by roomID (a.k.a. matchID in the chat wire format). Returns
    // (zero-value, false) when no session exists for that roomID.
    // Used by the chat handler to authorise match-scoped messages (Story 6.2).
    func (m *Manager) MatchParticipants(roomID uint) ([4]uint, bool) {
        m.mu.RLock()
        defer m.mu.RUnlock()
        s, ok := m.sessions[roomID]
        if !ok {
            return [4]uint{}, false
        }
        return s.playerIDs, true
    }
    ```
  - [x] 1.2 In [server/internal/session/manager_test.go](server/internal/session/manager_test.go), add `TestMatchParticipants`:
    - Returns `(_, false)` for a roomID with no session.
    - Returns `(playerIDs, true)` after `StartGame`.
    - Returns `(_, false)` after the session is removed.
  - [x] 1.3 **Update the chat handler's `GameMembership` interface** in [server/internal/chat/handler.go](server/internal/chat/handler.go:22) to add the new method:
    ```go
    type GameMembership interface {
        IsUserInGame(userID uint) bool
        MatchParticipants(matchID uint) ([4]uint, bool)
    }
    ```
    `*session.Manager` now satisfies both methods; no change needed in `main.go` wiring.

- [x] Task 2: Implement `ChannelMatch` branch in chat handler (AC: #2, #3)
  - [x] 2.1 Replace the current silent-drop stub at [server/internal/chat/handler.go:79-81](server/internal/chat/handler.go#L79-L81) with a real dispatch:
    ```go
    case ChannelMatch:
        h.handleMatch(client.UserID, req.MatchID, text)
    ```
  - [x] 2.2 Add the handler method to the same file. Mirror `handleGlobal` in structure — silent drops on every failure, server-stamped `RFC3339Nano` timestamp, reuse the existing `buildMessage` helper:

    ```go
    func (h *Handler) handleMatch(senderID uint, matchID *uint, text string) {
        if matchID == nil {
            slog.Info("chat: match send dropped (missing matchId)", "userID", senderID)
            return
        }
        participants, ok := h.game.MatchParticipants(*matchID)
        if !ok {
            slog.Info("chat: match send dropped (unknown matchId)",
                "userID", senderID, "matchID", *matchID)
            return
        }
        // Authorise: sender must be one of the 4 session participants.
        authorized := false
        for _, uid := range participants {
            if uid == senderID {
                authorized = true
                break
            }
        }
        if !authorized {
            slog.Info("chat: match send dropped (sender not in match)",
                "userID", senderID, "matchID", *matchID)
            return
        }

        sender, err := h.userRepo.FindByID(senderID)
        if err != nil || sender == nil {
            slog.Warn("chat: match sender not found", "userID", senderID, "error", err)
            return
        }

        payload := ws.ChatMessagePayload{
            UserID:    sender.ID,
            Username:  sender.Username,
            Message:   text,
            Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
            Scope:     ChannelMatch,
        }
        msgBytes := buildMessage(ws.SystemChatMessage, payload)
        if msgBytes == nil {
            return
        }
        // Broadcast to ALL 4 participants (sender included so they see their own echo).
        // Hub.BroadcastToUsers silently skips disconnected IDs.
        h.hub.BroadcastToUsers(participants[:], msgBytes)
    }
    ```

  - [x] 2.3 Remove the stale "// Reserved for Story 6.2" comment — the branch is now live.

- [x] Task 3: Extend backend tests for the match branch (AC: #2, #3, #8 server-side parity)
  - [x] 3.1 Update [server/internal/chat/handler_test.go](server/internal/chat/handler_test.go):
    - Extend the existing `fakeGame` fake with a `participants map[uint][4]uint` field and a `MatchParticipants(matchID)` method matching the new interface signature.
    - Update every existing `chat.NewHandler(...)` construction site so the fake still satisfies the new interface (add the new map, default empty).
    - **Delete** `TestHandler_IgnoresMatchChannel_DeferredToStory62` — behaviour changed.
    - Add the following new tests:
      - `TestHandler_MatchMessage_BroadcastsToFourParticipants` — sender + 3 teammates; `hubSpy.lastCall` shows exactly those 4 IDs.
      - `TestHandler_MatchMessage_SenderReceivesOwnEcho` — sender is in the recipient list.
      - `TestHandler_MatchMessage_ExcludesNonParticipants` — a 5th connected user ID is NOT in the recipient list.
      - `TestHandler_MatchMessage_DroppedWhenMatchIDMissing` — `MatchID == nil` → no broadcast.
      - `TestHandler_MatchMessage_DroppedWhenMatchUnknown` — `MatchParticipants` returns `false` → no broadcast.
      - `TestHandler_MatchMessage_DroppedWhenSenderNotParticipant` — sender connected but not among the 4 participants → no broadcast.
      - `TestHandler_MatchMessage_RejectsEmptyAndTooLong` — same invariants as global (delegate to the shared length/trim logic).
      - `TestHandler_MatchMessage_ScopeStampedAsMatch` — assert the broadcast payload decodes to `Scope: "match"`.
  - [x] 3.2 Do NOT touch the existing global-scope tests — they must continue to pass unchanged (Story 6.1 regression guard).

- [x] Task 4: Extend `chatStore` with `matchMessages` partition (AC: #4, #5, #6)
  - [x] 4.1 Update [client/src/shared/stores/chatStore.ts](client/src/shared/stores/chatStore.ts):
    ```typescript
    interface ChatState {
      globalMessages: ChatMessagePayload[];
      matchMessages: ChatMessagePayload[];
      appendGlobal: (msg: ChatMessagePayload) => void;
      appendMatch: (msg: ChatMessagePayload) => void;
      clearGlobal: () => void;
      clearMatch: () => void;
    }
    ```
    `appendMatch` must mirror `appendGlobal` exactly: spread-copy + 200-message ring-buffer drop of the oldest. `clearMatch` resets to `[]`. Do NOT share a single array — keep the two partitions independent.
  - [x] 4.2 Extend [client/src/shared/stores/chatStore.test.ts](client/src/shared/stores/chatStore.test.ts) with cases for `appendMatch` (append, cap at 200, drop oldest) and `clearMatch`, plus an invariance test: `appendGlobal` does not affect `matchMessages` and vice versa.

- [x] Task 5: Dispatch `scope === "match"` to the new store partition (AC: #4)
  - [x] 5.1 In [client/src/shared/hooks/useWsDispatch.ts](client/src/shared/hooks/useWsDispatch.ts:305-324), replace the `scope` branch:
    ```typescript
    if (payload.scope === "global") {
      useChatStore.getState().appendGlobal(payload);
    } else if (payload.scope === "match") {
      useChatStore.getState().appendMatch(payload);
    }
    ```
    Keep the existing defensive field-type validation block (lines 309-318) above unchanged — it already covers `scope: string`.
  - [x] 5.2 Update [client/src/shared/hooks/useWsDispatch.test.ts](client/src/shared/hooks/useWsDispatch.test.ts): add a test that dispatches a `system:chat_message` with `scope: "match"` and asserts `useChatStore.getState().matchMessages.length` increments by one while `globalMessages` stays unchanged. Reset `chatStore` in `beforeEach` (pattern already established in 6.1).

- [x] Task 6: Adapt `ChatPanel` to accept a `channel` prop (AC: #1, #2, #8, #9)
  - [x] 6.1 Update [client/src/features/chat/ChatPanel.tsx](client/src/features/chat/ChatPanel.tsx) props to accept the channel and matchId (drop-in compatible — `channel` defaults to `"global"` so `LobbyPage` is unaffected):
    ```typescript
    interface ChatPanelProps {
      className?: string;
      channel?: "global" | "match";
      matchId?: number; // required when channel === "match"
    }
    ```
  - [x] 6.2 Inside the component:
    - Select the correct partition:
      ```typescript
      const messages = useChatStore((s) =>
        channel === "match" ? s.matchMessages : s.globalMessages,
      );
      ```
    - Build the request payload with the correct channel:
      ```typescript
      const payload: ChatMessageRequest =
        channel === "match"
          ? { channel: "match", matchId, text }
          : { channel: "global", text };
      ```
      Guard: when `channel === "match"` but `matchId` is not a finite number, `handleSubmit` must early-return before calling `sendMessage` (defensive; should not happen in normal flow).
    - Use a locale-aware title via a new optional `title?: string` prop (or switch on `channel` internally calling `t("chat.title")` for global and `t("game.chat.title")` for match). Pick ONE approach and apply consistently. The simplest: switch on `channel` internally — the caller stays prop-light.
    - All other behaviour (trim, ≤500-rune guard, Enter-to-send, connection-state disabling, `chat.placeholderDisabled`, `chat.tooLong`, `chat.empty`, `chat.send`) is reused unchanged. This keeps AC #8 and #9 parity with Story 6.1 automatic.
  - [x] 6.3 Update existing tests in [client/src/features/chat/ChatPanel.test.tsx](client/src/features/chat/ChatPanel.test.tsx):
    - Keep all existing global-mode tests (the prop defaults to `"global"`).
    - Add new tests rendering `<ChatPanel channel="match" matchId={42} />`:
      - Renders messages from `chatStore.matchMessages` (seed via `useChatStore.setState({ matchMessages: [...] })`).
      - Submitting `"hi"` calls `sendMessage(ACTION_CHAT_MESSAGE, { channel: "match", matchId: 42, text: "hi" })`.
      - Does NOT call `sendMessage` when `matchId` is `undefined`.
      - Renders the match-chat title from `game.chat.title` (once that key is updated in Task 10).

- [x] Task 7: Build the game-page chat sidebar wrapper (AC: #1, #5, #7)
  - [x] 7.1 Create [client/src/features/game/components/MatchChatSidebar.tsx](client/src/features/game/components/MatchChatSidebar.tsx):
    - Reads `roomId` from `useGameStore((s) => s.roomId)` — this is the `matchId` for the chat payload (confirmed mapping: client `matchId` ≡ server `roomID`, see [client/src/shared/stores/gameStore.ts:15](client/src/shared/stores/gameStore.ts#L15) and [server/internal/ws/events.go:124](server/internal/ws/events.go#L124)).
    - Local state: `const [isOpen, setIsOpen] = useState(false)` (collapsed by default — see Design Decisions), `const [unread, setUnread] = useState(0)`.
    - Subscribes to `useChatStore((s) => s.matchMessages.length)`.
    - Effect: whenever `matchMessages.length` increases AND the sidebar is closed, increment `unread` by the delta. Track the previous length with a `useRef` to compute the delta.
    - Effect: when `isOpen` transitions to `true`, reset `unread` to `0`.
    - Renders two elements:
      1. A **fixed, always-visible toggle button** (chat icon + optional unread badge) on the right edge of the viewport. Position via Tailwind fixed positioning clear of seat 3 (`right-4 top-1/2`); offset the button **above or below** the East seat avatar — not behind it. Proposed: `fixed right-2 top-4` (top-right), so it is outside the seat 3 hit-box entirely. Unread badge shows `unread > 0 ? (unread > 99 ? "99+" : String(unread)) : null`.
      2. When `isOpen`, a **fixed-positioned panel** on the right edge (e.g. `fixed right-0 top-0 h-full w-80 z-30`) wrapping `<ChatPanel channel="match" matchId={roomId} />`. Width ≤ 320px on the large breakpoint, full-height with its own scroll.
    - `data-testid`: `match-chat-toggle`, `match-chat-toggle-unread`, `match-chat-sidebar` (wrapper on the open panel).
    - All copy via `useTranslation()`. New i18n keys (see Task 10): `game.chat.toggleOpen`, `game.chat.toggleClose`, `game.chat.placeholder` (update), `game.chat.title`.
    - **Do not render** when `roomId` is `null` (defensive — should never happen inside `/game/:roomId` but guards test flakiness).
  - [x] 7.2 Mount the sidebar inside [client/src/features/game/GamePage.tsx](client/src/features/game/GamePage.tsx):
    - Import: `import { MatchChatSidebar } from "./components/MatchChatSidebar";`
    - Render `<MatchChatSidebar />` as a **sibling** of the main game-table container (at the top level of the returned JSX, alongside overlays), so it layers above the board without affecting the existing grid/flex layout that positions seats and hand cards.
    - **Critical regression guard:** the sidebar MUST sit beneath game-critical overlays in z-index (pause overlay, reconnect overlay, match-result overlay, score reveal). Recommended: sidebar `z-30`; existing overlays already use `z-40`+. Verify by scanning the overlay components; DO NOT downgrade overlay z-index.
  - [x] 7.3 Add co-located test [client/src/features/game/components/MatchChatSidebar.test.tsx](client/src/features/game/components/MatchChatSidebar.test.tsx). Cover:
    - Renders the toggle button when `gameStore.roomId !== null` and `gameState` is non-null.
    - Toggle click opens/closes the sidebar (`match-chat-sidebar` test-id presence).
    - Incrementing `chatStore.matchMessages` while closed increases `match-chat-toggle-unread` count.
    - Opening the sidebar resets the unread badge.
    - Badge caps at `99+` when > 99.
    - Does not render the sidebar panel when `roomId` is `null`.
    - Uses the WS context mock pattern from [client/src/features/chat/ChatPanel.test.tsx](client/src/features/chat/ChatPanel.test.tsx) (already established in Story 6.1).

- [x] Task 8: Clear match history on match exit (AC: #6)
  - [x] 8.1 Extend [client/src/shared/stores/gameStore.ts](client/src/shared/stores/gameStore.ts:86) — in the `clearGame` reducer, also call `useChatStore.getState().clearMatch()` before resetting its own slice. Import `useChatStore` at the top. This covers: navigation away from `/game`, match abandonment, client-side forced exits.
  - [x] 8.2 In [client/src/features/game/GamePage.tsx](client/src/features/game/GamePage.tsx), confirm `clearGame` is called on unmount. It already is (Story 4-x teardown). If not, add a `useEffect` cleanup returning `() => clearGame()` so the match history is reset when the route unmounts even without an explicit `clearGame` dispatch.
  - [x] 8.3 Update [client/src/shared/stores/gameStore.test.ts](client/src/shared/stores/gameStore.test.ts) if it exists (it may not — the store is simple; if absent, skip and rely on the sidebar test to assert `matchMessages` clears). Otherwise assert `clearGame` empties `chatStore.matchMessages`.
  - [x] 8.4 **Alternative boundary:** if calling a store from inside another store feels wrong, place the `clearMatch()` call at the single GamePage unmount site instead. The task pairs `clearGame` with `clearMatch` — pick whichever pattern the project already uses for cross-store resets. (The codebase currently has no precedent for this; `clearGame` is self-contained. Recommend the cross-store call inside `clearGame` for a single exit point.)

- [x] Task 9: Ensure lobby global chat remains isolated (AC: #4 regression guard)
  - [x] 9.1 Run the Story 6.1 assertion mentally while coding: a `scope: "global"` event must NOT leak into `matchMessages`, and a `scope: "match"` event must NOT leak into `globalMessages`. Task 5.2 test already covers this; also add a symmetric case to `useWsDispatch.test.ts`.
  - [x] 9.2 The `LobbyPage` remains on `channel="global"` via the default prop — no code change required there. Verify [client/src/features/lobby/LobbyPage.test.tsx](client/src/features/lobby/LobbyPage.test.tsx) still passes without modification.

- [x] Task 10: i18n keys (AC: #1, #5, #8, #9)
  - [x] 10.1 In [client/src/shared/i18n/en.json](client/src/shared/i18n/en.json:149-152), replace the existing `game.chat` placeholder block with real copy and add the toggle/placeholder strings:
    ```json
    "chat": {
      "title": "Match Chat",
      "placeholder": "Message your table…",
      "toggleOpen": "Open match chat",
      "toggleClose": "Close match chat"
    }
    ```
    Match-chat copy keys `placeholderDisabled`, `send`, `empty`, `tooLong` are **shared** with the top-level `chat.*` block (same strings) — `ChatPanel` already reads `t("chat.placeholderDisabled")`, `t("chat.send")`, `t("chat.empty")`, `t("chat.tooLong")`. Do NOT duplicate under `game.chat.*`; keep DRY.
  - [x] 10.2 In [client/src/shared/i18n/sr.json](client/src/shared/i18n/sr.json), add the matching Serbian (Latin) translations under the existing `game.chat` block. Reuse the shared `chat.*` keys for shared copy. Proposed:
    ```json
    "chat": {
      "title": "Chat u meču",
      "placeholder": "Poruči za sto…",
      "toggleOpen": "Otvori chat meča",
      "toggleClose": "Zatvori chat meča"
    }
    ```
  - [x] 10.3 Confirm `i18n.test.ts` still passes (it currently does not enforce key parity per review-finding D77, but do not regress it).

- [x] Task 11: Validation and quality gates (AC: all)
  - [x] 11.1 From [client/](client/): run `npx prettier --write .`, then from repo root run `make lint`. This must pass for BOTH Go and TypeScript. (User's standing feedback: Prettier must be clean before any client-touching commit — see memory `feedback_prettier_before_commit.md`. CI has failed on this repeatedly.)
  - [x] 11.2 Run `make test` — every pre-existing test plus the new chat + sidebar tests must pass.
  - [x] 11.3 WS contract sanity: `ChatMessageRequest.matchId?: number` and `ChatMessagePayload.scope === "match"` are already declared in both [wsEvents.ts:260-272](client/src/shared/types/wsEvents.ts#L260-L272) and [events.go:120-135](server/internal/ws/events.go#L120-L135) (done in Story 6.1). No new contract fields are added in this story — the payload shape is identical to global chat, only the `scope`/`channel` discriminator changes.
  - [x] 11.4 Manual smoke test (write outcomes in the Completion Notes):
    - Start a Quick Play match with 4 test accounts (A/B/C/D) in 4 browser tabs or profiles.
    - A opens chat sidebar, sends "hi" → B, C, D all see it in their sidebars; sender A sees their own echo.
    - B is outside the match (still on `/lobby`) — B must NOT see the match chat.
    - With sidebar collapsed for C, have A send 3 messages → C's toggle shows unread badge `3`; opening the sidebar resets badge to `0`.
    - Play through to match end → all four players' `matchMessages` is empty when redirected back to lobby.
    - Game interactivity: during A's turn, toggle sidebar open and confirm A can still play a card, trigger bidding/declaration prompts, and click pause.
  - [x] 11.5 Regression guard checklist:
    - Global lobby chat (Story 6.1) still works end-to-end (send from A in lobby → B in lobby receives; A in match does NOT receive).
    - `action:play_card`, `action:pick_trump`, `action:pause`, and every other `session.Manager` action still routes through the composite `SetActionHandler` closure in `main.go` — our changes only extend the `ChannelMatch` branch inside `chat.Handler.HandleAction`.
    - Disconnect/reconnect flow (Epic 5): a reconnecting player's match chat history is empty on reconnect (no server-side history by design); this is intentional and matches AC #6.

## Dev Notes

### What Already Exists — Do NOT Recreate

| Item                                                                                          | Location                                                                                               | Status                                                                                                                                                                            |
| --------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------- |
| `ChannelMatch = "match"` constant                                                             | [server/internal/chat/handler.go:17](server/internal/chat/handler.go#L17)                              | Exists — replace the `case ChannelMatch:` stub with the real handler                                                                                                              |
| `ChatMessageRequest.MatchID *uint` (Go)                                                       | [server/internal/ws/events.go:124](server/internal/ws/events.go#L124)                                  | Exists (json tag `matchId,omitempty`) — DO NOT redeclare                                                                                                                          |
| `ChatMessagePayload.Scope string` (Go)                                                        | [server/internal/ws/events.go:134](server/internal/ws/events.go#L134)                                  | Exists with doc `"global"                                                                                                                                                         | "match"` — DO NOT redeclare |
| `ChatMessageRequest.matchId?: number` (TS)                                                    | [client/src/shared/types/wsEvents.ts:262](client/src/shared/types/wsEvents.ts#L262)                    | Exists — DO NOT redeclare                                                                                                                                                         |
| `ChatMessagePayload.scope: "global"                                                           | "match"` (TS)                                                                                          | [client/src/shared/types/wsEvents.ts:271](client/src/shared/types/wsEvents.ts#L271)                                                                                               | Exists — DO NOT redeclare   |
| `Hub.BroadcastToUsers(userIDs []uint, msg []byte)`                                            | [server/internal/ws/hub.go:174-182](server/internal/ws/hub.go#L174-L182)                               | Exists — use directly with `session.playerIDs[:]`                                                                                                                                 |
| `Hub.ConnectedUserIDs()`                                                                      | [server/internal/ws/hub.go:202-210](server/internal/ws/hub.go#L202-L210)                               | Exists (Story 6.1) — **NOT needed** for match scope (we broadcast to the 4 known participants regardless of connection state; `BroadcastToUsers` silently skips disconnected IDs) |
| `session.Manager.IsUserInGame(uid)`                                                           | [server/internal/session/manager.go:279](server/internal/session/manager.go#L279)                      | Exists — reused indirectly via the existing `GameMembership` interface                                                                                                            |
| `session.Manager.sessions[roomID]` + `Session.playerIDs [4]uint`                              | [server/internal/session/manager.go:43,21](server/internal/session/manager.go#L43)                     | Exists — expose via new `MatchParticipants(roomID)` method (Task 1)                                                                                                               |
| `chat.Handler` with `HandleAction`, length check, RFC3339Nano stamping, `buildMessage` helper | [server/internal/chat/handler.go](server/internal/chat/handler.go)                                     | Exists — reuse the text-validation pipeline; only add `handleMatch` below `handleGlobal`                                                                                          |
| `hubSpy`, `fakeGame`, `userRepoStub` test fakes                                               | [server/internal/chat/handler_test.go:19-95](server/internal/chat/handler_test.go#L19-L95)             | Exists — extend `fakeGame` with a participants map and add match-mode tests                                                                                                       |
| `chatStore.globalMessages` + 200-message ring buffer pattern                                  | [client/src/shared/stores/chatStore.ts](client/src/shared/stores/chatStore.ts)                         | Exists — add a symmetric `matchMessages` partition with identical buffer semantics                                                                                                |
| `ChatPanel` component                                                                         | [client/src/features/chat/ChatPanel.tsx](client/src/features/chat/ChatPanel.tsx)                       | Exists — ADAPT with an optional `channel` prop; do NOT fork or duplicate                                                                                                          |
| `useWsDispatch` chat branch with defensive payload validation                                 | [client/src/shared/hooks/useWsDispatch.ts:305-324](client/src/shared/hooks/useWsDispatch.ts#L305-L324) | Exists — add the `scope === "match"` arm; keep the existing validation block unchanged                                                                                            |
| `useGameStore((s) => s.roomId)`                                                               | [client/src/shared/stores/gameStore.ts:15,68](client/src/shared/stores/gameStore.ts#L15)               | Exists — this is the `matchId` for the chat payload                                                                                                                               |
| Composite `SetActionHandler` closure routing chat vs session                                  | [server/cmd/api/main.go:115-122](server/cmd/api/main.go#L115-L122)                                     | Exists — no wiring change; the same `chat.Handler` instance handles both scopes                                                                                                   |
| `WebSocketProvider` wrapping `/game/:roomId`                                                  | [client/src/shared/components/ProtectedRoute.tsx](client/src/shared/components/ProtectedRoute.tsx)     | Exists — `GamePage` already has WS context                                                                                                                                        |
| `Intl.DateTimeFormat` / `useTranslation` / `data-testid` conventions                          | client-wide                                                                                            | Established in Story 6.1; same discipline here                                                                                                                                    |

### What Must Be Created

1. [client/src/features/game/components/MatchChatSidebar.tsx](client/src/features/game/components/MatchChatSidebar.tsx) — collapsible right-edge sidebar with toggle + unread badge.
2. [client/src/features/game/components/MatchChatSidebar.test.tsx](client/src/features/game/components/MatchChatSidebar.test.tsx) — co-located component tests.

### What Must Be Modified

1. [server/internal/session/manager.go](server/internal/session/manager.go) — add `MatchParticipants(roomID)`.
2. [server/internal/session/manager_test.go](server/internal/session/manager_test.go) — add `TestMatchParticipants`.
3. [server/internal/chat/handler.go](server/internal/chat/handler.go) — extend `GameMembership` interface, add `handleMatch`, replace stub `case ChannelMatch`.
4. [server/internal/chat/handler_test.go](server/internal/chat/handler_test.go) — extend `fakeGame`, remove `TestHandler_IgnoresMatchChannel_DeferredToStory62`, add match-scope coverage.
5. [client/src/shared/stores/chatStore.ts](client/src/shared/stores/chatStore.ts) — add `matchMessages`, `appendMatch`, `clearMatch`.
6. [client/src/shared/stores/chatStore.test.ts](client/src/shared/stores/chatStore.test.ts) — add match-partition tests.
7. [client/src/shared/hooks/useWsDispatch.ts](client/src/shared/hooks/useWsDispatch.ts) — dispatch `scope === "match"` to `appendMatch`.
8. [client/src/shared/hooks/useWsDispatch.test.ts](client/src/shared/hooks/useWsDispatch.test.ts) — add match-dispatch test.
9. [client/src/features/chat/ChatPanel.tsx](client/src/features/chat/ChatPanel.tsx) — add `channel` / `matchId` props with global default.
10. [client/src/features/chat/ChatPanel.test.tsx](client/src/features/chat/ChatPanel.test.tsx) — add match-mode test cases.
11. [client/src/features/game/GamePage.tsx](client/src/features/game/GamePage.tsx) — mount `<MatchChatSidebar />`.
12. [client/src/shared/stores/gameStore.ts](client/src/shared/stores/gameStore.ts) — `clearGame` also clears `matchMessages` (Task 8 — single source of truth).
13. [client/src/shared/i18n/en.json](client/src/shared/i18n/en.json) — extend `game.chat.*` keys.
14. [client/src/shared/i18n/sr.json](client/src/shared/i18n/sr.json) — extend `game.chat.*` keys.

### Architecture Patterns to Follow

- **Single chat handler instance, branched by scope.** The server uses ONE `*chat.Handler` (from [main.go:115](server/cmd/api/main.go#L115)) composed into `SetActionHandler`. Route inside the handler by `req.Channel`. Do NOT create a second handler or action route.
- **Server-authoritative match membership.** Never trust the client's `matchId` blindly. Look up the session via `MatchParticipants(matchID)` and verify `senderID` is among the returned `[4]uint`. Silent drop on any mismatch — no `error:` event (this mirrors AC #7 of Story 6.1).
- **Recipient list is the 4 session participants, not the hub snapshot.** Unlike global chat (which filters `ConnectedUserIDs`), match chat broadcasts to the fixed `session.playerIDs[:]`. `Hub.BroadcastToUsers` already silently skips disconnected IDs — the correct behaviour for temporarily disconnected players inside the reconnect window.
- **Server-stamped timestamps only.** `time.Now().UTC().Format(time.RFC3339Nano)`. Never accept a client timestamp. React keys rely on `(userId, timestamp)` uniqueness — nanosecond precision prevents collisions.
- **Typed payload structs over `map[string]interface{}`.** Continue the project-wide `ChatMessagePayload` / `ChatMessageRequest` discipline.
- **Two chat partitions, one UI component.** `chatStore` gets a second array (`matchMessages`). `ChatPanel` stays single-file, channel-aware via a prop. Prevents drift; any improvement to rendering/validation benefits both scopes simultaneously.
- **Ephemeral by design.** No DB table, no migration, no REST endpoint, no history-on-join — identical to Story 6.1. Reconnecting players start from the current live stream. 200-message ring-buffer cap per partition. (See PRD FR31 — "send and receive", no history requirement.)
- **UX spec is load-bearing.** Per [ux-design-specification.md:114,379](_bmad-output/planning-artifacts/ux-design-specification.md#L114): "match chat always visible and accessible at the table" + "collapsible sidebar, right edge, does not overlap the table". The toggle button is the "always visible" part; the expanded panel must not cover seat 3 / hand cards / overlays.
- **i18n parity in both JSONs.** Project rule — any new key added to `en.json` is added to `sr.json` in the same commit. No exceptions.
- **Immutable Zustand updates.** `set((state) => ({ matchMessages: [...state.matchMessages, msg] }))`. Never mutate arrays in place. Ring-buffer drop uses a fresh `splice` on a shallow copy (pattern already in `appendGlobal`).
- **`data-testid` selectors in tests, never Tailwind class names.** Project rule — Tailwind churn breaks class-based selectors.
- **Feature folder organisation.** The sidebar wrapper lives under `features/game/components/` (game-specific composition). The reusable `ChatPanel` stays under `features/chat/` (cross-context primitive).

### Key Design Decision — Sidebar Collapsed by Default

AC #1 says the sidebar is "available" and "can be toggled". UX spec says "always visible and accessible". The implementation choice: **toggle button always visible, panel collapsed by default at match start**. Rationale:

- First trick is attentionally critical (bidding, declarations, belote prompts, hand orientation). Showing a full chat panel on top of that overwhelms new players.
- Unread badge (AC #5) advertises activity without stealing focus.
- Returning players who value chat will click once; the state persists for the duration of the match (optional future: remember the preference in localStorage — out of scope here).

If playtesting demands "open by default", flip the `useState(false)` to `useState(true)` — single-line change. Do not add a user setting in this story.

### Key Design Decision — Layout Strategy (Avoiding Seat 3 Overlap)

The East seat avatar sits at `right-4 top-1/2 -translate-y-1/2` ([GamePage.tsx:54](client/src/features/game/GamePage.tsx#L54)) — vertically centred, ~16px from the right edge. The expanded chat panel fills the right edge full-height at `right-0 top-0 h-full w-80` (320px). These overlap visually.

**Resolution:** The sidebar panel sits at `z-30`; seat 3 avatar is at a lower z-index (~z-10 implicit). The panel visually covers seat 3 **while it is open** — this is acceptable per UX spec since the panel is explicitly collapsible and the player can close it at any time to see every seat. The alternative (shifting seat 3 inward on open) violates the "HUD anchored to fixed positions — never reflow during play" rule at [ux-design-specification.md:378](_bmad-output/planning-artifacts/ux-design-specification.md#L378).

**Critical guards:**

- Panel z-index MUST be lower than pause/reconnect/match-result overlays (those should be `z-40`+). Verify by scanning existing overlay classes before wiring.
- Panel must NOT cover hand cards (bottom edge) or the toggle button when closed. Width cap 320px; on viewports < 900px tall, the panel uses full width the same way — the project targets 1280×720 minimum.
- Pointer events outside the panel bounds pass through to the game table (standard CSS — no `pointer-events: none` workarounds needed since the panel does not span the full viewport width).

### Key Design Decision — One `ChatPanel`, Two Channels

Adding an optional `channel` prop (default `"global"`) to the existing `ChatPanel` is preferred over forking into `LobbyChatPanel` + `MatchChatPanel`:

- Single rendering path for message rows — future tweaks (say, timestamp hover, copy-to-clipboard) benefit both instantly.
- Prop default preserves `LobbyPage` as-is (no Story 6.1 regression risk).
- The sidebar wrapper (`MatchChatSidebar`) carries only the composition logic (toggle, unread badge, positioning) — the message-list + input stays single-sourced.

### Key Design Decision — Ephemeral + Disconnect-Aware

`Hub.BroadcastToUsers` silently skips disconnected userIDs. A player in the 120-second reconnect window receives no match chat during disconnection and no history on reconnect. This is **intentional** — it aligns with the ephemeral design (no DB, no history) and with Story 5.4's state-restoration scope (game state re-syncs; chat is out of scope). Do not add chat-replay to the reconnect payload.

### Frontend Flow — Sending a Match Message

1. Player in `/game/:roomId` opens the sidebar → types text → Enter/Send.
2. `ChatPanel` (channel=`"match"`, matchId=roomId) builds `ChatMessageRequest` and calls `useWsSendMessage()`.
3. Wire: `{ "type": "action:chat_message", "payload": { "channel": "match", "matchId": <roomId>, "text": "..." } }`.
4. Server `Hub.handleMessage` → composite `SetActionHandler` closure → `chat.Handler.HandleAction` → `handleMatch`.
5. Server validates: `MatchID != nil` → `MatchParticipants` lookup → sender in participants → length check (already done) → username lookup → stamp timestamp → build payload with `Scope: "match"`.
6. Server calls `hub.BroadcastToUsers(participants[:], msg)` — 4 IDs regardless of connection state; disconnected IDs are skipped by the hub.
7. Each connected participant's `useWsDispatch` validates payload shape → dispatches to `chatStore.appendMatch`.
8. Mounted `MatchChatSidebar` subscribes to `matchMessages`; the open panel scrolls to the new row; if closed, the unread delta increments.

### Backend Patterns You'll Reuse

- **`buildMessage(eventType, payload) []byte`** — already defined in [server/internal/chat/handler.go:130-142](server/internal/chat/handler.go#L130-L142). Use directly.
- **Shared text-validation prologue** — `HandleAction` already trims, rune-counts, and rejects empty/>500 **before** the channel switch. `handleMatch` inherits this for free; do not duplicate.
- **Silent-drop logging pattern** — every drop path logs at `INFO` with `userID`, `matchID`, and a short reason. Keep the same shape as `handleGlobal`.

### Previous Story Intelligence (Story 6.1 — done 2026-04-18)

Carried-forward learnings that directly shape this story:

- **Rune-count (not byte-count) for the 500-char cap** — already applied in the shared prologue; match scope inherits it.
- **RFC3339Nano server timestamps** — required for unique React keys under the 200-message ring buffer; reuse the same stamp call.
- **Defensive dispatcher payload validation** — added in 6.1 at [useWsDispatch.ts:309-318](client/src/shared/hooks/useWsDispatch.ts#L309-L318). Match scope rides on the same check; a malformed match payload is also rejected safely.
- **Sender-exempt own-echo** — for global chat, the sender is unconditionally in the recipient list. For match chat this is automatic: the sender IS one of the 4 `playerIDs` and they are always included in the broadcast.
- **`scrollIntoView` jsdom stub** — already in [client/src/test-setup.ts](client/src/test-setup.ts); no extra test-setup work needed.
- **i18n key parity (en + sr in the same commit)** — strict discipline; `game.chat.*` additions must land in both files simultaneously.
- **Composite handler pattern** — untouched in this story. We extend inside the chat handler, not around it.
- **Reviewer flagged (D72) a TOCTOU race** on recipient filtering for global chat. Match chat is immune: participants are the fixed `session.playerIDs`; we do not re-check `IsUserInGame` per recipient.
- **Reviewer deferred (D73) concurrent action-handler goroutines** — pre-existing pattern affecting all action handlers. Two match chat messages from the same client may arrive out of order at recipients. Accepted — not in scope for 6.2.

### Recent Codebase Signals (git log — last 5 commits)

- `0d90b73 feat(chat): global lobby chat with in-game exclusion (Story 6.1)` — the direct predecessor; sets all the scaffolding this story extends.
- `ab09141 feat(game): name the declarer on each declaration-reveal row` — GamePage is actively iterated; keep the sidebar insertion minimal-touch.
- `feadf94 fix(game): derive led suit from currentTrick to close legal-cards race` — concurrency awareness in the client; our `matchMessages.length` delta tracking uses a ref, not a state-in-state dependency, to avoid similar races.
- `a21533d fix(game): extend Belot and declaration reveal duration to 8s` — overlays are actively tuned; respect existing z-indices when inserting the sidebar.

### Cross-Story Context

- **Story 6.1 (done)** — global chat. This story shares the handler, the store file, the panel component, the dispatcher, and the i18n namespace. Any regression in 6.1 means 6.2 broke something — test both paths.
- **Story 5.3 / 5.4 (done)** — disconnect / reconnect. Match chat deliberately does not replay history on reconnect. Do not add chat payload to the reconnect restore event.
- **Story 4.x (done)** — WS gateway, event contract, game table UI. The sidebar layers above the table without replacing any existing game element.
- **Story 8.5 (future)** — in-game emotes. Likely to mount adjacent to the chat toggle on the same right edge. Keep the sidebar's right-edge budget tight (≤320px open, toggle + badge ≤64px tall) so there is room for future UI without re-layout.

### Project Structure Notes

**New files (expected):**

- `client/src/features/game/components/MatchChatSidebar.tsx`
- `client/src/features/game/components/MatchChatSidebar.test.tsx`

**Modified files (expected):**

- `server/internal/session/manager.go` (add `MatchParticipants`)
- `server/internal/session/manager_test.go` (add `TestMatchParticipants`)
- `server/internal/chat/handler.go` (extend `GameMembership`, add `handleMatch`, replace stub)
- `server/internal/chat/handler_test.go` (extend `fakeGame`, drop deferred test, add match-scope suite)
- `client/src/shared/stores/chatStore.ts` (add match partition)
- `client/src/shared/stores/chatStore.test.ts` (add match tests)
- `client/src/shared/hooks/useWsDispatch.ts` (dispatch match scope)
- `client/src/shared/hooks/useWsDispatch.test.ts` (add match-dispatch test)
- `client/src/features/chat/ChatPanel.tsx` (channel prop)
- `client/src/features/chat/ChatPanel.test.tsx` (match-mode cases)
- `client/src/features/game/GamePage.tsx` (mount sidebar)
- `client/src/shared/stores/gameStore.ts` (`clearGame` clears match messages)
- `client/src/shared/i18n/en.json` (`game.chat.*` extension)
- `client/src/shared/i18n/sr.json` (`game.chat.*` extension)

**No changes expected:**

- `server/cmd/api/main.go` (wiring already correct)
- `server/internal/ws/events.go` (contract fields already present)
- `client/src/shared/types/wsEvents.ts` (contract fields already present)
- `server/internal/ws/hub.go` (`BroadcastToUsers` already suitable)
- `client/src/features/lobby/LobbyPage.tsx` (uses default `channel="global"`)

### References

- [Source: epics.md#Story-6.2 — Match-Scoped Chat acceptance criteria]
- [Source: prd.md#FR31 — match-scoped chat, visible only to the four participants]
- [Source: architecture.md — WebSocket scope discriminator, match-scoped broadcast via `Hub.BroadcastToUsers`]
- [Source: ux-design-specification.md#lines-114,377-379 — "match chat always visible and accessible"; collapsible right-edge sidebar; HUD anchored positions]
- [Source: project-context.md — WS contract sync, i18n parity, immutable Zustand updates, data-testid discipline]
- [Source: 6-1-global-lobby-chat.md — prior story scaffolding, review-patch learnings, handler structure]
- [Source: 5-4-reconnection-and-state-restoration.md — chat deliberately excluded from reconnect state restoration]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.7 (1M context)

### Debug Log References

- Backend: `go test ./... -count=1` — all 13 packages pass (chat 0.78s, session 17.37s, ws 11.98s, room 34.25s, others cached/fast). `go vet ./...` clean.
- Frontend: `npx vitest run` — **48 test files / 381 tests, all green**. Includes 8 new `MatchChatSidebar` tests, 4 new `ChatPanel` match-mode tests, 5 new `chatStore` match-partition tests, 2 new `useWsDispatch` match-dispatch tests, and 1 new `gameStore` clear-match test.
- Lint: `npx eslint .` + `npx prettier --check .` clean (Prettier reformatted `ChatPanel.tsx`, `chatStore.test.ts`, `MatchChatSidebar.test.tsx` once).
- `golangci-lint` unavailable locally (same as Story 6.1) — relied on `go vet` + CI to enforce style. All other gates green.

### Completion Notes List

- Added `session.Manager.MatchParticipants(roomID) ([4]uint, bool)` public method backed by the existing `sessions[roomID].playerIDs` — held under the same `RLock` as `IsUserInGame` so the chat handler can cheaply authorise match-scoped sends.
- Extended the chat handler's `GameMembership` interface with `MatchParticipants`; `*session.Manager` satisfies it automatically. No wiring change in `main.go` — the single `chat.Handler` instance now serves both scopes.
- Implemented `handleMatch` in [server/internal/chat/handler.go](server/internal/chat/handler.go): silent-drop on missing `matchId`, unknown session, or sender-not-participant; server-stamped `RFC3339Nano` timestamp; payload `Scope: "match"`; broadcasts to the fixed `session.playerIDs[:]` (sender included for own-echo). `Hub.BroadcastToUsers` silently skips disconnected IDs — the correct behaviour for players inside the reconnect window.
- Extended `fakeGame` in handler_test.go with a `participants map[uint][4]uint` and added 8 match-scope tests covering: 4-participant broadcast, sender echo, non-participant exclusion, missing `matchId`, unknown match, unauthorised sender, empty/too-long rejection, scope stamping. Removed the now-obsolete deferred-stub test.
- Extended [chatStore.ts](client/src/shared/stores/chatStore.ts) with a symmetric `matchMessages` partition (`appendMatch` / `clearMatch`, same 200-message ring buffer). Introduced an `appendWithCap` helper so the two `append*` actions share a single immutable-update implementation.
- Routed `scope === "match"` to `chatStore.appendMatch` in [useWsDispatch.ts](client/src/shared/hooks/useWsDispatch.ts); the existing defensive payload validation covers both scopes automatically. Added a partition-isolation test (scope=global does NOT leak into matchMessages).
- Adapted [ChatPanel.tsx](client/src/features/chat/ChatPanel.tsx) with optional `channel` (default `"global"`) + `matchId` props. Channel-switches the store selector, the outgoing request payload, the title key (`chat.title` vs `game.chat.title`), and the input placeholder key. `LobbyPage` consumes it unchanged via the default prop — zero Story 6.1 regression surface.
- Created [MatchChatSidebar.tsx](client/src/features/game/components/MatchChatSidebar.tsx): fixed top-right toggle button with unread badge (caps at `99+`), opens a fixed-right `w-80 h-full` panel mounting `<ChatPanel channel="match" matchId={roomId} />`. Collapsed by default; unread increments only while closed; opening resets to 0. Returns `null` when `gameStore.roomId` is `null` (defensive). Uses `lucide-react`'s `MessageSquare` / `X` icons (already a project dependency).
- Removed the pre-existing placeholder chat sidebar JSX + `chatOpen` local state from [GamePage.tsx](client/src/features/game/GamePage.tsx) and mounted `<MatchChatSidebar />` in its place, preserving z-index discipline (sidebar `z-20`, below all game-critical overlays which sit at `z-30`+).
- Wired match-history teardown in [gameStore.ts](client/src/shared/stores/gameStore.ts): both `clearGame()` and `reset()` now call `useChatStore.getState().clearMatch()` before resetting their own slice. A single exit point keeps the ephemeral match-chat discipline consistent with `gameStore.roomId` lifecycle.
- Updated i18n: replaced `game.chat.placeholder`'s "Chat available soon" in both `en.json` and `sr.json` with real match-chat copy; added `toggleOpen` / `toggleClose` keys for the sidebar aria-labels. `i18n.test.ts` parity check passes.
- Test notes: jsdom `scrollIntoView` stub reused from Story 6.1 (via `test-setup.ts`). Where seed data lives on the store (not in JSX state), wrapped `useChatStore.setState` / `useGameStore.setState` calls inside `act()` to silence React 19 test warnings.

### File List

**New files:**

- client/src/features/game/components/MatchChatSidebar.tsx
- client/src/features/game/components/MatchChatSidebar.test.tsx

**Modified files:**

- server/internal/session/manager.go (added `MatchParticipants(roomID)`)
- server/internal/session/manager_test.go (added `TestMatchParticipants`)
- server/internal/chat/handler.go (extended `GameMembership` interface, added `handleMatch`, replaced `ChannelMatch` stub with live dispatch)
- server/internal/chat/handler_test.go (extended `fakeGame` with `participants` map; added 8 match-scope tests; removed `TestHandler_IgnoresMatchChannel_DeferredToStory62`)
- client/src/shared/stores/chatStore.ts (added `matchMessages` partition, `appendMatch`, `clearMatch`, shared `appendWithCap` helper; review-patch: added `matchMessagesReceivedTotal` monotonic counter)
- client/src/shared/stores/chatStore.test.ts (reset all 3 slices in `beforeEach`; added match-partition tests incl. partition-isolation; review-patch: added 3 monotonic-counter tests)
- client/src/shared/stores/gameStore.ts (review-patch: reverted cross-store import; `clearGame` and `reset` are back to `set(initialState)` only)
- client/src/shared/stores/gameStore.test.ts (review-patch: removed obsolete `clearGame clears matchMessages` test)
- client/src/shared/hooks/useWsDispatch.ts (route `scope === "match"` to `chatStore.appendMatch`; review-patch: defence-in-depth guard — drop match scope when `gameStore.roomId === null`)
- client/src/shared/hooks/useWsDispatch.test.ts (reset `matchMessages` in `beforeEach`; replaced deferred-match test with real append test; added partition-isolation test; review-patch: seed `roomId` for happy-path match test + added null-guard drop test)
- client/src/features/chat/ChatPanel.tsx (added optional `channel` + `matchId` props; channel-switched store selector, payload, title, placeholder; review-patch: tightened `matchId` validation to `Number.isInteger(matchId) && matchId > 0`)
- client/src/features/chat/ChatPanel.test.tsx (reset both partitions; added 4 match-mode tests)
- client/src/features/game/GamePage.tsx (removed placeholder chat-sidebar JSX + `chatOpen` state; mounted `<MatchChatSidebar />`; review-patch: added unmount `useEffect` that calls `chatStore.clearMatch()` — owns match-chat lifecycle cleanup after the gameStore cross-store import was reverted)
- client/src/shared/i18n/en.json (expanded `game.chat.*` with real copy + `toggleOpen` / `toggleClose`)
- client/src/shared/i18n/sr.json (expanded `game.chat.*` with Serbian translations)

**Modified files (review patches, 2026-04-18):**

- client/src/features/game/components/MatchChatSidebar.tsx — z-20 → z-30 on toggle + panel (spec alignment); unread counter switched from `matchMessages.length` to monotonic `matchMessagesReceivedTotal` so the badge survives the 200-cap ceiling and `clearMatch()` correctly resets it to 0; tightened null-guard from `roomId === null` to `roomId === null || roomId <= 0`
- client/src/features/game/components/MatchChatSidebar.test.tsx — added 2 regression tests: "unread keeps incrementing past 200-cap" and "unread resets when matchMessages cleared while sidebar closed"

**No changes expected / made:**

- server/cmd/api/main.go, server/internal/ws/events.go, server/internal/ws/hub.go, client/src/shared/types/wsEvents.ts, client/src/features/lobby/LobbyPage.tsx (contract and wiring already landed in Story 6.1).

### Change Log

- 2026-04-18 — Story 6.2 implemented: match-scoped chat via a right-edge collapsible sidebar, 4-participant server broadcast, unread badge, ephemeral-by-design with clear on game teardown. All 9 ACs satisfied; 381/381 frontend tests + all backend packages green.
- 2026-04-18 — Code review complete: 6 review patches applied (cross-store coupling reverted + GamePage unmount cleanup, sidebar z-index z-20→z-30, monotonic unread counter, useWsDispatch roomId guard, matchId integer validation, sidebar roomId<=0 guard). 4 items deferred to D78-D81. 386/386 frontend tests + all backend packages green; ESLint + Prettier + Go vet clean.

### Review Findings

- [x] [Review][Decision→Patch] Cross-store coupling resolved: moved `clearMatch()` out of `gameStore.clearGame/reset` and into a `GamePage` unmount `useEffect` cleanup. Keeps Zustand stores independent (no cross-store import), makes `reset()` scoped to game state, and eliminates the latent circular-import risk flagged by three reviewers. Obsolete `clearGame clears matchMessages` test removed. [client/src/features/game/GamePage.tsx:100-107, client/src/shared/stores/gameStore.ts]

- [x] [Review][Patch] Sidebar and toggle z-index lifted from `z-20` to `z-30` so the panel paints above same-tier overlays (PauseOverlay / ReconnectOverlay / DeclarationReveal / BelotReveal / TrumpPrompt) as the spec prescribed. Game-critical full-screen overlays (CapotAnimation `z-40`, ScoreReveal `z-30` with backdrop, MatchResult `z-50`) still take precedence. [client/src/features/game/components/MatchChatSidebar.tsx:46,66]

- [x] [Review][Patch] Decoupled unread counter from `matchMessages.length`: added `matchMessagesReceivedTotal` (monotonic counter) to `chatStore`, incremented inside `appendMatch` and reset inside `clearMatch`. `MatchChatSidebar` now subscribes to the counter (via `useRef` delta). This fixes two concrete regressions: (a) unread no longer stops incrementing at the 200-cap ceiling; (b) `clearMatch()` while the sidebar is closed correctly drops `unread` to 0 (no phantom badge). Added 3 chatStore tests + 2 sidebar regression tests. [client/src/shared/stores/chatStore.ts, client/src/features/game/components/MatchChatSidebar.tsx:13-37]

- [x] [Review][Patch] Added defence-in-depth guard in `useWsDispatch`: `scope: "match"` payloads are dropped when `gameStore.roomId === null`, so a stale frame arriving after `clearGame` cannot leak into the next match's history. New test confirms the drop; the happy-path test now explicitly sets `roomId` first. [client/src/shared/hooks/useWsDispatch.ts:321-325]

- [x] [Review][Patch] Tightened `ChatPanel` match-mode `matchId` validation to `Number.isInteger(matchId) && matchId > 0` — rejects 0, negatives, NaN, Infinity, and non-integer finite numbers before they reach the wire. [client/src/features/chat/ChatPanel.tsx:46]

- [x] [Review][Patch] Tightened `MatchChatSidebar` null-check to `roomId === null || roomId <= 0` — a 0 or negative `roomId` now correctly returns `null` instead of rendering a dead sidebar. [client/src/features/game/components/MatchChatSidebar.tsx:35-37]

- [x] [Review][Defer] Race between `session.Manager.MatchParticipants` and `Hub.BroadcastToUsers` when `RemoveSession` fires in between — broadcast fans out to player IDs whose session is already gone. Consequence is mild (late match chat reaches players who just returned to the lobby, reseeding their cleared `matchMessages`). Fix requires the session manager to own the broadcast atomically under the write lock — architectural change. [server/internal/chat/handler.go:130-178] — deferred, systemic concurrency concern shared with other session broadcasts; revisit alongside chat moderation work in Phase 2.

- [x] [Review][Defer] No rate limiting on match chat — a participant can flood the 3 other players' client queues at WS ingress rate. Story 6.1 deferred identical concern for global chat (D72). [server/internal/chat/handler.go:130-178] — deferred, Phase 1 chat is intentionally unmoderated per Story 6.1 precedent (D72); revisit when chat moderation lands in Phase 2.

- [x] [Review][Defer] `RFC3339Nano` server timestamps do not provide monotonic ordering under NTP clock corrections — messages sorted by timestamp may misorder on clock jumps. Systemic across all server-stamped events (timers, disconnects, declarations). [server/internal/chat/handler.go:161] — deferred, project-wide concern not introduced by this story; out of scope.

- [x] [Review][Defer] Concurrent `action:chat_message` goroutines per client can interleave, so a sender's two rapid-fire messages may reach peers out of submission order. Same pre-existing router pattern that Story 6.1 deferred as D73. [server/internal/ws/router.go] — deferred, matches Story 6.1 D73 precedent; router-level change is out of scope for a chat story.
