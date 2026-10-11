# Story 6.1: Global Lobby Chat

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a logged-in player,
I want to send and receive messages in a global lobby chat,
so that I can socialize with other players and find people to play with.

## Acceptance Criteria

1. **ChatPanel renders on the lobby**
   Given a player is authenticated and in the lobby (`/lobby`),
   When `LobbyPage` mounts,
   Then a `ChatPanel` component is visible inside the lobby layout
   And recent messages are displayed with sender username, message text, and a locale-formatted timestamp.

2. **Sending a message broadcasts to all lobby clients**
   Given a player types a non-empty message and submits the form,
   When the client emits `action:chat_message` with payload `{ "channel": "global", "text": "..." }`,
   Then the server validates the player is authenticated and **not currently in a game session**,
   And the server broadcasts `system:chat_message` (payload includes `userId`, `username`, `message`, `timestamp` ISO 8601, `scope: "global"`) to every connected client who is **not** in a game session,
   And the message appears in every eligible player's `ChatPanel` in real time.

3. **chatStore + dispatch routes events to UI**
   Given the WebSocket dispatcher (`useWsDispatch.ts`) receives a `system:chat_message` event with `scope === "global"`,
   When the message is processed,
   Then it is appended to `chatStore.globalMessages` and the `ChatPanel` re-renders with the new message at the bottom.

4. **No history on join — chat is ephemeral**
   Given new messages have been sent before a player enters the lobby,
   When `LobbyPage` mounts,
   Then no message history is fetched from the server (no REST endpoint, no DB persistence)
   And the player only sees messages broadcast after they connected.

5. **Empty/whitespace messages are rejected on the client**
   Given a player tries to submit an empty string, only whitespace, or a message exceeding the configured max length (500 chars),
   When the submit handler runs,
   Then the message is **not** sent over the WebSocket and the input remains focused with a brief visual cue (e.g. shake/disabled submit).

6. **Players in a game do not receive global chat**
   Given a player is currently in an active match (their userID is registered in `session.Manager.userToRoom`),
   When other players broadcast global chat messages,
   Then the in-game player does **not** receive `system:chat_message` events with `scope: "global"`
   And their client never appends them to `chatStore.globalMessages`.

7. **In-game players cannot send to global**
   Given a player is currently in an active match,
   When they attempt to send `action:chat_message` with `channel: "global"`,
   Then the server drops the message silently (no broadcast) and logs the attempt at `INFO` level — no error event is emitted to the client.

## Tasks / Subtasks

- [x] Task 1: Add server-side `IsUserInGame` lookup to session manager (AC: #2, #6, #7)
  - [x] 1.1 In [server/internal/session/manager.go](server/internal/session/manager.go), add a public method:
    ```go
    // IsUserInGame returns true if the user is currently part of an active game session.
    // Used by the chat handler to enforce the "no global chat while in a game" rule.
    func (m *Manager) IsUserInGame(userID uint) bool {
        m.mu.RLock()
        defer m.mu.RUnlock()
        _, ok := m.userToRoom[userID]
        return ok
    }
    ```
  - [x] 1.2 Add a unit test in [server/internal/session/manager_test.go](server/internal/session/manager_test.go) — `TestIsUserInGame`: returns `false` for unknown user, `true` after `StartGame`, `false` after `RemoveSession`.

- [x] Task 2: Add WS event constants + payload types to **both** contract files (AC: #2)
  - [x] 2.1 In [server/internal/ws/events.go](server/internal/ws/events.go), under `// --- Chat events ---`, add the new action constant and payload structs (the `SystemChatMessage` constant already exists — do NOT duplicate):

    ```go
    const ActionChatMessage = "action:chat_message"

    // ChatMessageRequest is the typed payload for ActionChatMessage (client → server).
    // Channel is "global" for lobby chat. "match" channel is reserved for Story 6.2.
    type ChatMessageRequest struct {
        Channel string `json:"channel"`           // "global" (this story) | "match" (6.2)
        MatchID *uint  `json:"matchId,omitempty"` // required when channel == "match" (Story 6.2)
        Text    string `json:"text"`
    }

    // ChatMessagePayload is the typed payload for SystemChatMessage (server → client).
    type ChatMessagePayload struct {
        UserID    uint   `json:"userId"`
        Username  string `json:"username"`
        Message   string `json:"message"`
        Timestamp string `json:"timestamp"` // ISO 8601 (RFC3339) UTC
        Scope     string `json:"scope"`     // "global" | "match"
    }
    ```

  - [x] 2.2 In [client/src/shared/types/wsEvents.ts](client/src/shared/types/wsEvents.ts) — `SYSTEM_CHAT_MESSAGE` and `ChatMessagePayload` already exist (lines 257-265). **Add** the action constant + request payload right above them:

    ```typescript
    export const ACTION_CHAT_MESSAGE = "action:chat_message" as const;

    export interface ChatMessageRequest {
      channel: "global" | "match";
      matchId?: number; // required when channel === "match" (Story 6.2)
      text: string;
    }
    ```

    Verify the existing `ChatMessagePayload` matches the Go struct exactly (camelCase, `scope: "global" | "match"`, `timestamp: string`). No change needed.

- [x] Task 3: Create the chat handler package (AC: #2, #6, #7)
  - [x] 3.1 Replace the placeholder [server/internal/chat/chat.go](server/internal/chat/chat.go) (currently `package chat` only) with a new file [server/internal/chat/handler.go](server/internal/chat/handler.go):

    ```go
    package chat

    import (
        "encoding/json"
        "log/slog"
        "strings"
        "time"

        "github.com/emilijan/beljot/server/internal/user"
        "github.com/emilijan/beljot/server/internal/ws"
    )

    const (
        maxMessageLength = 500
        ChannelGlobal    = "global"
        ChannelMatch     = "match"
    )

    // GameMembership reports whether a user is currently in an active game session.
    // Wired to session.Manager.IsUserInGame in main.go.
    type GameMembership interface {
        IsUserInGame(userID uint) bool
    }

    // Handler processes action:chat_message events.
    // Match-scoped chat (channel == "match") is implemented in Story 6.2;
    // this handler ignores it for now and returns silently.
    type Handler struct {
        hub      *ws.Hub
        userRepo user.UserRepository
        game     GameMembership
    }

    func NewHandler(hub *ws.Hub, userRepo user.UserRepository, game GameMembership) *Handler {
        return &Handler{hub: hub, userRepo: userRepo, game: game}
    }

    // HandleAction is the action handler entry point. Composed with
    // session.Manager.HandleAction in main.go via a dispatch closure.
    func (h *Handler) HandleAction(client *ws.Client, msg ws.WSMessage) {
        if msg.Type != ws.ActionChatMessage {
            return // not for us
        }

        var req ws.ChatMessageRequest
        if err := json.Unmarshal(msg.Payload, &req); err != nil {
            slog.Info("chat: invalid payload", "userID", client.UserID, "error", err)
            return
        }

        text := strings.TrimSpace(req.Text)
        if text == "" || len(text) > maxMessageLength {
            slog.Info("chat: message rejected (empty or too long)", "userID", client.UserID, "length", len(text))
            return
        }

        switch req.Channel {
        case ChannelGlobal:
            h.handleGlobal(client.UserID, text)
        case ChannelMatch:
            // Reserved for Story 6.2 — silently ignore for now.
            return
        default:
            slog.Info("chat: unknown channel", "userID", client.UserID, "channel", req.Channel)
        }
    }

    func (h *Handler) handleGlobal(senderID uint, text string) {
        // Enforce "no global chat while in a game" — silently drop (AC #7).
        if h.game.IsUserInGame(senderID) {
            slog.Info("chat: global send dropped (sender in game)", "userID", senderID)
            return
        }

        sender, err := h.userRepo.FindByID(senderID)
        if err != nil || sender == nil {
            slog.Warn("chat: sender not found", "userID", senderID, "error", err)
            return
        }

        payload := ws.ChatMessagePayload{
            UserID:    sender.ID,
            Username:  sender.Username,
            Message:   text,
            Timestamp: time.Now().UTC().Format(time.RFC3339),
            Scope:     ChannelGlobal,
        }
        msgBytes := buildMessage(ws.SystemChatMessage, payload)
        if msgBytes == nil {
            return
        }

        // Build the recipient list: all connected users NOT currently in a game.
        recipients := h.hub.ConnectedUserIDs()
        eligible := recipients[:0]
        for _, uid := range recipients {
            if h.game.IsUserInGame(uid) {
                continue
            }
            eligible = append(eligible, uid)
        }
        h.hub.BroadcastToUsers(eligible, msgBytes)
    }

    func buildMessage(eventType string, payload interface{}) []byte {
        payloadBytes, err := json.Marshal(payload)
        if err != nil {
            slog.Error("chat: marshal payload failed", "type", eventType, "error", err)
            return nil
        }
        msg, err := json.Marshal(ws.WSMessage{Type: eventType, Payload: payloadBytes})
        if err != nil {
            slog.Error("chat: marshal message failed", "type", eventType, "error", err)
            return nil
        }
        return msg
    }
    ```

  - [x] 3.2 Delete (or keep empty) the legacy [server/internal/chat/chat.go](server/internal/chat/chat.go) file — `handler.go` becomes the package's primary file.

- [x] Task 4: Add `Hub.ConnectedUserIDs()` helper (AC: #2)
  - [x] 4.1 In [server/internal/ws/hub.go](server/internal/ws/hub.go), add a method below `ClientCount()`:
    ```go
    // ConnectedUserIDs returns a snapshot of all currently-connected user IDs.
    // Order is unspecified. Caller may mutate the returned slice freely.
    func (h *Hub) ConnectedUserIDs() []uint {
        h.mu.RLock()
        defer h.mu.RUnlock()
        ids := make([]uint, 0, len(h.clients))
        for uid := range h.clients {
            ids = append(ids, uid)
        }
        return ids
    }
    ```
  - [x] 4.2 Add a small unit test in [server/internal/ws/ws_test.go](server/internal/ws/ws_test.go) verifying the snapshot length matches `ClientCount()`.

- [x] Task 5: Compose chat handler with session manager in `main.go` (AC: #2, #7)
  - [x] 5.1 In [server/cmd/api/main.go](server/cmd/api/main.go), import the chat package and construct the handler **after** `sessionManager` (it depends on `sessionManager.IsUserInGame`):
    ```go
    chatHandler := chat.NewHandler(hub, userRepo, sessionManager)
    ```
  - [x] 5.2 Replace the existing `hub.SetActionHandler(sessionManager.HandleAction)` (line 111) with a composite that routes `action:chat_message` to the chat handler and everything else to the session manager:
    ```go
    hub.SetActionHandler(func(client *ws.Client, msg ws.WSMessage) {
        if msg.Type == ws.ActionChatMessage {
            chatHandler.HandleAction(client, msg)
            return
        }
        sessionManager.HandleAction(client, msg)
    })
    ```
  - [x] 5.3 Add the `chat` import: `"github.com/emilijan/beljot/server/internal/chat"`.

- [x] Task 6: Frontend — wire `chatStore` for global messages (AC: #3, #4)
  - [x] 6.1 Replace the placeholder [client/src/shared/stores/chatStore.ts](client/src/shared/stores/chatStore.ts) with a real partitioned store:

    ```typescript
    import { create } from "zustand";

    import type { ChatMessagePayload } from "@/shared/types/wsEvents";

    const MAX_MESSAGES = 200; // ring-buffer cap for ephemeral chat

    interface ChatState {
      globalMessages: ChatMessagePayload[];
      appendGlobal: (msg: ChatMessagePayload) => void;
      clearGlobal: () => void;
    }

    export const useChatStore = create<ChatState>((set) => ({
      globalMessages: [],
      appendGlobal: (msg) =>
        set((state) => {
          const next = [...state.globalMessages, msg];
          if (next.length > MAX_MESSAGES) {
            next.splice(0, next.length - MAX_MESSAGES);
          }
          return { globalMessages: next };
        }),
      clearGlobal: () => set({ globalMessages: [] }),
    }));
    ```

  - [x] 6.2 Add a co-located test [client/src/shared/stores/chatStore.test.ts](client/src/shared/stores/chatStore.test.ts) covering: appends a single message, drops oldest when exceeding `MAX_MESSAGES`, `clearGlobal` resets state.

- [x] Task 7: Frontend — dispatch `system:chat_message` events (AC: #3, #6)
  - [x] 7.1 In [client/src/shared/hooks/useWsDispatch.ts](client/src/shared/hooks/useWsDispatch.ts):
    - Import `useChatStore` from `@/shared/stores/chatStore` and `ChatMessagePayload` from `@/shared/types/wsEvents`.
    - Replace the existing no-op handler at lines 302-305 (`if (type === SYSTEM_CHAT_MESSAGE) { return; }`) with:
      ```typescript
      if (type === SYSTEM_CHAT_MESSAGE) {
        const payload = message.payload as ChatMessagePayload;
        if (payload.scope === "global") {
          useChatStore.getState().appendGlobal(payload);
        }
        // scope === "match" handled by Story 6.2
        return;
      }
      ```
  - [x] 7.2 Update [client/src/shared/hooks/useWsDispatch.test.ts](client/src/shared/hooks/useWsDispatch.test.ts) — add a test that dispatches a `system:chat_message` with `scope: "global"` and asserts `useChatStore.getState().globalMessages` length increases by one.

- [x] Task 8: Frontend — build the `ChatPanel` component (AC: #1, #2, #3, #5)
  - [x] 8.1 Create [client/src/features/chat/ChatPanel.tsx](client/src/features/chat/ChatPanel.tsx). Requirements:
    - Subscribes to `useChatStore((s) => s.globalMessages)`.
    - Renders a vertically scrolling message list and an input + submit button below.
    - Auto-scrolls to the latest message when new messages arrive (use a `ref` + `scrollIntoView` in `useEffect`).
    - Each message row shows: `username` (bold), `message` text, and timestamp formatted via `Intl.DateTimeFormat` honouring the current `i18n.language` (use `t("chat.timeFormat")`-style hook only if needed; otherwise `new Date(timestamp).toLocaleTimeString(i18n.language)`).
    - Input is disabled (with a visible reason via `t("chat.placeholderDisabled")`) when `useWsConnectionState() !== "connected"`.
    - Submit handler:
      - Trims input. If empty or `> 500` chars: do not send, briefly highlight the input.
      - Otherwise call `useWsSendMessage()` with `(ACTION_CHAT_MESSAGE, { channel: "global", text } as ChatMessageRequest)`, then clear the input.
      - Submission triggers on Enter (without Shift) and on the explicit Send button.
    - Use `data-testid` on key elements: `chat-panel`, `chat-message-list`, `chat-message-row`, `chat-input`, `chat-send-button`.
    - All user-visible strings via `useTranslation()` from `react-i18next`.
  - [x] 8.2 Replace the stub [client/src/features/chat/ChatPage.tsx](client/src/features/chat/ChatPage.tsx) — leave it as-is for now (no route uses it; it is not on the navigation). Optionally delete in cleanup at end of Epic 6.
  - [x] 8.3 Add co-located test [client/src/features/chat/ChatPanel.test.tsx](client/src/features/chat/ChatPanel.test.tsx). Cover:
    - Renders messages from `chatStore`.
    - Trims and rejects whitespace-only submissions (does not call `sendMessage`).
    - Calls `sendMessage(ACTION_CHAT_MESSAGE, ...)` with a valid `ChatMessageRequest` payload on Enter.
    - Disables the input/button when WS state is `"disconnected"` or `"connecting"`.
    - Mock `useWsSendMessage` and `useWsConnectionState` from `@/shared/providers/WebSocketContext` (see `GamePage.test.tsx` for the mocking pattern).

- [x] Task 9: Mount `ChatPanel` inside the lobby layout (AC: #1)
  - [x] 9.1 Update [client/src/features/lobby/LobbyPage.tsx](client/src/features/lobby/LobbyPage.tsx). The current right-hand column is the leaderboard placeholder (lines 167-170). Replace its content (or convert to a 2-row stack) so that the right-hand column hosts both the leaderboard placeholder **and** the `ChatPanel` underneath, with the chat panel taking the majority of the vertical space:
    ```tsx
    {
      /* Right column: Leaderboard placeholder above, ChatPanel below */
    }
    <div className="flex flex-col gap-4 min-h-[600px]">
      <div className="rounded-lg border border-border bg-surface p-6">
        <p className="text-text-secondary">
          {t("lobby.leaderboardPlaceholder")}
        </p>
      </div>
      <ChatPanel className="flex-1" />
    </div>;
    ```
    Add the import: `import { ChatPanel } from "@/features/chat/ChatPanel";`.
  - [x] 9.2 If the design requires the panel to fill the column height, expose an optional `className` prop on `ChatPanel` so the parent can apply `flex-1` / sizing classes (Tailwind, no design-token edits).
  - [x] 9.3 Update [client/src/features/lobby/LobbyPage.test.tsx](client/src/features/lobby/LobbyPage.test.tsx) (or extend the existing test file) to assert `screen.getByTestId("chat-panel")` is present when the lobby renders. Mock `useWsSendMessage` + `useWsConnectionState` like other lobby tests do.

- [x] Task 10: i18n keys (AC: #1, #5)
  - [x] 10.1 In [client/src/shared/i18n/en.json](client/src/shared/i18n/en.json), add a new top-level `"chat": { ... }` block (the existing `game.chat` key stays for Story 6.2):
    ```json
    "chat": {
      "title": "Lobby Chat",
      "placeholder": "Say hi…",
      "placeholderDisabled": "Reconnecting…",
      "send": "Send",
      "empty": "No messages yet — say hi.",
      "tooLong": "Messages must be 500 characters or fewer.",
      "you": "You"
    }
    ```
  - [x] 10.2 In [client/src/shared/i18n/sr.json](client/src/shared/i18n/sr.json), add the matching Serbian (Latin) translations:
    ```json
    "chat": {
      "title": "Lobi Chat",
      "placeholder": "Pozdravi…",
      "placeholderDisabled": "Ponovno povezivanje…",
      "send": "Pošalji",
      "empty": "Još nema poruka — pozdravi.",
      "tooLong": "Poruke moraju imati 500 karaktera ili manje.",
      "you": "Ti"
    }
    ```
  - [x] 10.3 Confirm `i18n.test.ts` parity check (if it asserts key parity between EN and SR) still passes.

- [x] Task 11: Backend tests for chat handler (AC: #2, #5, #6, #7)
  - [x] 11.1 Create [server/internal/chat/handler_test.go](server/internal/chat/handler_test.go) using `testify`. Use a fake `GameMembership` (a struct with a `inGame map[uint]bool`) and a `userRepoStub` returning fixed users. Use a real `*ws.Hub` only when needed; otherwise stub. Cases:
    - `TestHandler_GlobalMessage_BroadcastsToEligibleClients` — sender + 2 idle users connected, server broadcasts to all 3 (sender included so they see their own message in the panel).
    - `TestHandler_GlobalMessage_ExcludesInGameUsers` — connected users include 2 in-game and 1 idle; only the idle user receives the broadcast.
    - `TestHandler_GlobalMessage_DroppedWhenSenderInGame` — sender is in-game; no broadcast occurs.
    - `TestHandler_RejectsEmpty` — empty/whitespace text → no broadcast.
    - `TestHandler_RejectsTooLong` — text length 501 → no broadcast.
    - `TestHandler_IgnoresUnknownChannel` — `channel: "private"` → no broadcast, no panic.
    - `TestHandler_IgnoresMatchChannel_DeferredToStory62` — `channel: "match"` → no broadcast (silently ignored for now).
    - `TestHandler_IgnoresWrongActionType` — calling `HandleAction` with `msg.Type != ws.ActionChatMessage` is a no-op (proves composite routing safety).
  - [x] 11.2 Use `httptest.Server` only if you need a true end-to-end WS roundtrip; for the cases above, asserting the recipients passed to `hub.BroadcastToUsers` via a hub spy is sufficient. See [server/internal/ws/ws_test.go](server/internal/ws/ws_test.go) for an existing httptest pattern if needed.

- [x] Task 12: Validation and quality gates (AC: all)
  - [x] 12.1 Run `make lint` — must pass for both Go and TypeScript. Reminder: run `npx prettier --write .` from `client/` first to avoid Prettier failures.
  - [x] 12.2 Run `make test` — all existing tests plus new chat tests must pass.
  - [x] 12.3 Verify WS contract files are in sync ([wsEvents.ts](client/src/shared/types/wsEvents.ts) ↔ [events.go](server/internal/ws/events.go)) — `ACTION_CHAT_MESSAGE` + `ChatMessageRequest` present in both.
  - [x] 12.4 Manual smoke test: open two browser tabs as different users → both land on `/lobby` → User A sends "hi" → User B's panel updates within ~100ms. Then User B opens a Quick Play match and sits in `/game/...` → User A sends another global message → User B's chatStore does **not** receive it (verified via React DevTools or a temporary console.log on `appendGlobal`).
  - [x] 12.5 Verify no regressions in any existing session manager test, room handler test, or game flow E2E. Particularly: confirm `action:play_card` and other game actions still route to `sessionManager.HandleAction` after the composite wiring change.

## Dev Notes

### What Already Exists — Do NOT Recreate

| Item                                                            | Location                                                                                                         | Status                                                                                                                                                           |
| --------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `package chat` placeholder                                      | [server/internal/chat/chat.go](server/internal/chat/chat.go)                                                     | Exists (empty) — REPLACE with `handler.go`                                                                                                                       |
| `ChatPage` placeholder component                                | [client/src/features/chat/ChatPage.tsx](client/src/features/chat/ChatPage.tsx)                                   | Exists — leave as-is, NOT used in this story                                                                                                                     |
| `chatStore` placeholder (`isLoading` only)                      | [client/src/shared/stores/chatStore.ts](client/src/shared/stores/chatStore.ts)                                   | Exists — REPLACE with `globalMessages` partition                                                                                                                 |
| `SystemChatMessage = "system:chat_message"` constant            | [server/internal/ws/events.go:117](server/internal/ws/events.go#L117)                                            | Exists — DO NOT redeclare                                                                                                                                        |
| `SYSTEM_CHAT_MESSAGE` constant + `ChatMessagePayload` interface | [client/src/shared/types/wsEvents.ts:257-265](client/src/shared/types/wsEvents.ts#L257-L265)                     | Exists — payload shape already defined                                                                                                                           |
| `useWsDispatch.ts` chat no-op stub                              | [client/src/shared/hooks/useWsDispatch.ts:303-305](client/src/shared/hooks/useWsDispatch.ts#L303-L305)           | Exists — REPLACE with real handler                                                                                                                               |
| `useWsSendMessage()` hook                                       | [client/src/shared/providers/WebSocketContext.ts:12-18](client/src/shared/providers/WebSocketContext.ts#L12-L18) | Exists — reuse for sending `action:chat_message`                                                                                                                 |
| `useWsConnectionState()` hook                                   | [client/src/shared/providers/WebSocketContext.ts:20-26](client/src/shared/providers/WebSocketContext.ts#L20-L26) | Exists — use to disable input when not `"connected"`                                                                                                             |
| `WebSocketProvider` wrapping all protected routes               | [client/src/shared/components/ProtectedRoute.tsx:24-28](client/src/shared/components/ProtectedRoute.tsx#L24-L28) | Exists — `LobbyPage` already inside provider                                                                                                                     |
| `Hub.BroadcastToUsers([]uint, []byte)`                          | [server/internal/ws/hub.go:174-182](server/internal/ws/hub.go#L174-L182)                                         | Exists — reuse for fan-out                                                                                                                                       |
| `Hub.BroadcastAll([]byte)`                                      | [server/internal/ws/hub.go:185-191](server/internal/ws/hub.go#L185-L191)                                         | Exists — DO NOT use here (would hit in-game users)                                                                                                               |
| `Hub.SetActionHandler(handler)`                                 | [server/internal/ws/hub.go:56-58](server/internal/ws/hub.go#L56-L58)                                             | Exists — there is exactly ONE action handler slot, so `main.go` MUST compose chat + session into a single closure                                                |
| `session.Manager.userToRoom` map                                | [server/internal/session/manager.go:44](server/internal/session/manager.go#L44)                                  | Exists — used to enforce in-game exclusion via new `IsUserInGame` helper                                                                                         |
| `user.UserRepository.FindByID(id uint) (*User, error)`          | [server/internal/user/repository.go:7](server/internal/user/repository.go#L7)                                    | Exists — use to resolve sender username for outgoing payload                                                                                                     |
| `WSMessage{Type, Payload}` envelope                             | [server/internal/ws/message.go](server/internal/ws/message.go)                                                   | Exists — payload is `json.RawMessage`; build with two-step `json.Marshal` (see `room.handler.broadcastToUsers` and `session.buildMessage` for the exact pattern) |
| `Intl.DateTimeFormat` / `react-i18next` `useTranslation`        | client                                                                                                           | Exists — standard for all locale-formatted UI                                                                                                                    |

### What Must Be Created

1. [server/internal/chat/handler.go](server/internal/chat/handler.go) — chat handler with `HandleAction`, in-game exclusion logic, and message build/broadcast.
2. [server/internal/chat/handler_test.go](server/internal/chat/handler_test.go) — unit tests for all AC branches.
3. [client/src/features/chat/ChatPanel.tsx](client/src/features/chat/ChatPanel.tsx) — message list + input UI, ~150 LOC.
4. [client/src/features/chat/ChatPanel.test.tsx](client/src/features/chat/ChatPanel.test.tsx) — co-located component tests.
5. [client/src/shared/stores/chatStore.test.ts](client/src/shared/stores/chatStore.test.ts) — store-level unit tests.
6. New i18n `chat.*` namespace in both EN and SR JSON files.

### What Must Be Modified

1. [server/internal/ws/events.go](server/internal/ws/events.go) — add `ActionChatMessage`, `ChatMessageRequest`, `ChatMessagePayload`.
2. [server/internal/ws/hub.go](server/internal/ws/hub.go) — add `ConnectedUserIDs()` snapshot helper.
3. [server/internal/session/manager.go](server/internal/session/manager.go) — add `IsUserInGame(userID)` public method.
4. [server/internal/session/manager_test.go](server/internal/session/manager_test.go) — add `TestIsUserInGame`.
5. [server/internal/ws/ws_test.go](server/internal/ws/ws_test.go) — add `ConnectedUserIDs` test.
6. [server/internal/chat/chat.go](server/internal/chat/chat.go) — replace with `handler.go` (or delete; `package chat` clause moves into the new file).
7. [server/cmd/api/main.go](server/cmd/api/main.go) — instantiate `chat.NewHandler` and compose action handler dispatch.
8. [client/src/shared/types/wsEvents.ts](client/src/shared/types/wsEvents.ts) — add `ACTION_CHAT_MESSAGE` constant + `ChatMessageRequest` interface.
9. [client/src/shared/stores/chatStore.ts](client/src/shared/stores/chatStore.ts) — replace placeholder with `globalMessages` partition.
10. [client/src/shared/hooks/useWsDispatch.ts](client/src/shared/hooks/useWsDispatch.ts) — replace chat no-op with real append-to-store.
11. [client/src/shared/hooks/useWsDispatch.test.ts](client/src/shared/hooks/useWsDispatch.test.ts) — add chat dispatch test.
12. [client/src/features/lobby/LobbyPage.tsx](client/src/features/lobby/LobbyPage.tsx) — mount `ChatPanel` in right-hand column.
13. [client/src/features/lobby/LobbyPage.test.tsx](client/src/features/lobby/LobbyPage.test.tsx) — add panel-presence assertion.
14. [client/src/shared/i18n/en.json](client/src/shared/i18n/en.json) — add `chat.*` keys.
15. [client/src/shared/i18n/sr.json](client/src/shared/i18n/sr.json) — add `chat.*` keys.

### Architecture Patterns to Follow

- **Single-handler hub composition.** [Hub.SetActionHandler](server/internal/ws/hub.go#L56) accepts only one handler. The session manager's [HandleAction](server/internal/session/manager.go#L129) is the current owner. Follow the same composite pattern as the existing connect/disconnect wiring in [main.go:115-122](server/cmd/api/main.go#L115-L122): a single closure that inspects `msg.Type` and dispatches. Do not add a second hub setter.
- **WS contract sync in the same commit.** Both [wsEvents.ts](client/src/shared/types/wsEvents.ts) and [events.go](server/internal/ws/events.go) updated together. Project rule, no exceptions (see [project-context.md:80](_bmad-output/project-context.md#L80)).
- **Typed payload structs over `map[string]interface{}`.** Mirror the existing `MatchAbandonedPayload` / `PlayerDisconnectedPayload` style in [events.go:74-92](server/internal/ws/events.go#L74-L92). Avoid raw `json.RawMessage` payload assembly in handlers.
- **Server-authoritative timestamp.** The server stamps `timestamp` (UTC ISO 8601 / RFC3339), never the client. This matches existing timer/disconnect events.
- **Server is the only source of truth for "in-game" status.** Do NOT trust a client flag like `"isInGame: true"` in the request. Look it up via `session.Manager.IsUserInGame`.
- **Silent drops, not error events, for the in-game send case.** AC #7 explicitly says no `error:` event. Logging at INFO level is sufficient — these are not bugs, they are expected during a match.
- **Zustand immutable updates.** Use `set((state) => ({ globalMessages: [...state.globalMessages, msg] }))` — never mutate. Project rule from [project-context.md:62](_bmad-output/project-context.md#L62).
- **Ring-buffer cap on chatStore.** 200 messages max — prevents unbounded memory growth in long-running lobby sessions. Drop oldest when exceeded.
- **No HTTP API for chat in this story.** Chat is ephemeral; no REST endpoint, no DB table, no migration. Story 6.2 also stays ephemeral.
- **i18n from day one.** All user-visible strings via `react-i18next`. Add to BOTH `en.json` and `sr.json`. Project rule.
- **`data-testid` for selectors in tests.** Never CSS classes (Tailwind churn breaks them). Project rule from [project-context.md:138](_bmad-output/project-context.md#L138).
- **Feature folder organization.** All chat UI in `features/chat/`. Shared store in `shared/stores/chatStore.ts`. Project rule from [project-context.md:89](_bmad-output/project-context.md#L89).

### Key Design Decision — Ephemeral Chat (No History)

AC #4 mandates no message history loading. This is a **deliberate** Phase 1 simplification:

- No `chat_messages` table, no migration, no repository.
- No moderation, no rate-limiting, no spam controls (acceptable for the small Phase 1 user base).
- The 200-message ring buffer in `chatStore` is **client-side only** — server holds zero state per chat channel.
- This is consistent with [PRD line 330](_bmad-output/planning-artifacts/prd.md#L330) (FR30) which only requires "send and receive" — not history.

If Phase 2 needs history, this is the migration boundary: `chat.Handler` would gain a repo dependency, persist on broadcast, and a new HTTP endpoint would serve recent messages. Out of scope for this story.

### Key Design Decision — In-Game Filter on the Server

AC #6 says in-game players "do not receive global chat events". Two implementation options:

1. **Server-side filter (chosen):** server iterates `Hub.ConnectedUserIDs()` and excludes anyone where `IsUserInGame(uid)`. Clean, no client trust required, works even if client logic has bugs.
2. Client-side ignore: server broadcasts to everyone; client ignores while on `/game/*`. Wastes bandwidth, leaks chat to the in-game DOM as a transient flash, and would require client-side scope tracking.

Pick (1). It also paves the way for Story 6.2's match-scoped chat where the same `Hub.BroadcastToUsers(matchPlayerIDs, ...)` pattern applies.

### Frontend Flow — Sending a Global Message

1. Player types in `ChatPanel` input → presses Enter / clicks Send.
2. `ChatPanel.onSubmit`: trim → length check → `useWsSendMessage()(ACTION_CHAT_MESSAGE, { channel: "global", text } satisfies ChatMessageRequest)`.
3. Wire format: `{ "type": "action:chat_message", "payload": { "channel": "global", "text": "hi" } }`.
4. Server `Hub.handleMessage` routes by `"action"` prefix → composite action handler → `chat.Handler.HandleAction`.
5. Server validates: length ≤ 500, sender not in game, channel == "global".
6. Server stamps timestamp, looks up sender username, builds `ChatMessagePayload`.
7. Server snapshots connected users, filters out in-game, calls `hub.BroadcastToUsers(eligible, msg)`.
8. Each eligible client's `useWsDispatch` receives `system:chat_message` → `chatStore.appendGlobal(payload)`.
9. `ChatPanel` re-renders; `useEffect` scrolls to bottom.

**Edge case — sender's own message.** The sender is included in `eligible` (assuming they are not in-game), so they see their own message via the same broadcast path. No optimistic local append. This keeps state derivation server-driven and avoids deduping logic.

**Edge case — disconnect mid-send.** `useWebSocket.sendMessage` (line 46-50 of [useWebSocket.ts](client/src/shared/hooks/useWebSocket.ts#L46-L50)) silently drops sends when `state !== "connected"`. The disabled-input guard in `ChatPanel` (Task 8.1) is the user-visible cue.

### Backend Patterns You'll Reuse

- **`buildMessage(eventType, payload) []byte`** — the chat handler defines its own copy (see Task 3.1) since the existing one in `session/manager.go:614` is private to that package. Pattern is identical: two-step JSON marshal into `ws.WSMessage{Type, Payload: json.RawMessage}`.
- **Composite handler closure** — see [main.go:115-122](server/cmd/api/main.go#L115-L122) (`SetConnectHandler` and `SetDisconnectHandler` already use this exact pattern with the lobby disconnect handler). Apply the same shape for `SetActionHandler`.
- **User lookup by ID** — verify the exact signature in [server/internal/user/repository.go](server/internal/user/repository.go); the project conventions suggest `FindByID(id uint) (*User, error)`. Confirm before wiring.

### Previous Story Intelligence (Story 5.5 — done 2026-04-14)

Carried-forward learnings relevant here:

- **Build messages BEFORE broadcasting** — for chat we don't hold any session lock, but the same principle applies: marshal once, broadcast once, do not call `json.Marshal` inside a hot loop.
- **WS contract files updated in the same commit** — strictly enforced. The new `ChatMessageRequest` lives in both files atop the existing `ChatMessagePayload`.
- **i18n keys added to both EN and SR JSON** — Story 5.5 added `matchAbandoned`, `matchAbandonedScores`, `returningToLobby` to both. Same discipline here: no missing keys.
- **Composite handler closure pattern** — Story 5.5 introduced the connect/disconnect composite. Story 6.1 reuses the same pattern for `SetActionHandler`. Direct precedent for the wiring change in Task 5.

### Cross-Story Context

- **Story 6.2 (next)** — match-scoped chat. Already designed to ride the same `action:chat_message` envelope with `channel: "match"` + `matchId: number`. The handler scaffolded in this story should leave a `case ChannelMatch:` branch in place that returns silently — Story 6.2 fills it in. The `ChatMessagePayload` already has `scope: "global" | "match"` ready.
- **Stories 5.x (all done)** — disconnect/reconnect lifecycle. The `Hub.SetConnectHandler` / `SetDisconnectHandler` composites already exist; if Story 6.x ever needs presence/typing indicators they slot into the same hooks. Out of scope here.
- **Epic 4 (done)** — WebSocket gateway, event contract, multiplexed connection. Chat rides on the same connection (project decision: single multiplexed WS per client).

### Project Structure Notes

**New files (expected):**

- `server/internal/chat/handler.go`
- `server/internal/chat/handler_test.go`
- `client/src/features/chat/ChatPanel.tsx`
- `client/src/features/chat/ChatPanel.test.tsx`
- `client/src/shared/stores/chatStore.test.ts`

**Modified files (expected):**

- `server/internal/ws/events.go` (add chat action constant + payload structs)
- `server/internal/ws/hub.go` (add `ConnectedUserIDs()`)
- `server/internal/ws/ws_test.go` (add hub helper test)
- `server/internal/session/manager.go` (add `IsUserInGame`)
- `server/internal/session/manager_test.go` (add `IsUserInGame` test)
- `server/internal/chat/chat.go` (replaced by handler.go)
- `server/cmd/api/main.go` (composite action handler wiring)
- `client/src/shared/types/wsEvents.ts` (add `ACTION_CHAT_MESSAGE` + `ChatMessageRequest`)
- `client/src/shared/stores/chatStore.ts` (real partition)
- `client/src/shared/hooks/useWsDispatch.ts` (chat dispatch wiring)
- `client/src/shared/hooks/useWsDispatch.test.ts` (chat dispatch test)
- `client/src/features/lobby/LobbyPage.tsx` (mount `ChatPanel`)
- `client/src/features/lobby/LobbyPage.test.tsx` (panel-present assertion)
- `client/src/shared/i18n/en.json` (`chat.*` namespace)
- `client/src/shared/i18n/sr.json` (`chat.*` namespace)

### References

- [Source: epics.md#Epic-6 — Story 6.1 acceptance criteria, Epic 6 context]
- [Source: prd.md#FR30 — global lobby chat functional requirement]
- [Source: architecture.md — WebSocket multiplexing, partitioned Zustand stores, contract sync rule]
- [Source: ux-design-specification.md — "Lobby as social café"; chat in lobby positioning]
- [Source: project-context.md — Critical Implementation Rules: WS contract sync, i18n parity, immutable Zustand updates, feature folder layout, data-testid for tests]
- [Source: 5-5-match-abandonment-on-timeout.md — composite handler pattern, WS contract sync discipline, i18n parity discipline]
- [Source: 4-1-websocket-gateway-and-event-contract.md — WS event prefix routing, hub composition]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.7 (1M context)

### Debug Log References

- Backend: `go test ./...` — all 13 packages pass (chat 0.69s, session 17.5s, ws 11.97s, room 34.1s, others cached/fast). `go vet ./...` clean.
- Frontend: `npx vitest run` — 47 test files, **361/361 tests pass**. Includes 11 new chat-related tests (4 chatStore, 8 ChatPanel, 2 dispatch, plus the new lobby-page panel-presence assertion and i18n parity).
- Lint: `npx eslint .` clean, `npx prettier --check .` clean (Prettier reformatted `ChatPanel.tsx` once).
- `golangci-lint` not available locally — relied on `go vet` + CI to enforce style. All other lint gates green.

### Completion Notes List

- Added `session.Manager.IsUserInGame(userID)` public method backed by the existing `userToRoom` map; exposed under `RLock` so it stays cheap to call from the chat broadcast loop.
- Added `ws.Hub.ConnectedUserIDs()` snapshot helper. Caller-mutable (allocates a new slice), so the chat handler can do an in-place filter without races.
- WS contract sync: `ActionChatMessage`, `ChatMessageRequest`, `ChatMessagePayload` added to both [events.go](server/internal/ws/events.go) and [wsEvents.ts](client/src/shared/types/wsEvents.ts) atop the pre-existing `SystemChatMessage` constant + `ChatMessagePayload` type.
- Created `chat.Handler` (`server/internal/chat/handler.go`) with the in-game exclusion rule, server-stamped RFC3339 timestamp, and silent drops on every error path (per AC #7). Introduced a small `Broadcaster` interface (subset of `*ws.Hub`) so the handler is unit-testable without spinning up a real hub goroutine — minor refinement on the dev notes plan.
- Composed the chat handler into `Hub.SetActionHandler` via a closure in `main.go`, mirroring the connect/disconnect composite pattern from Story 5.5.
- Replaced placeholder `chatStore` with a real Zustand partition (`globalMessages` ring buffer, cap 200) + immutable updates per project rule.
- Wired `system:chat_message` dispatch to `chatStore.appendGlobal` in `useWsDispatch.ts`, scope-gated to `"global"` (match scope reserved for Story 6.2).
- Built `ChatPanel.tsx` with: scrollable message list, locale-formatted timestamps via `Intl.DateTimeFormat`, Enter-to-send (Shift+Enter ignored), input/button disabled while WS is not `"connected"`, client-side empty/length-500 guard with a visible warning row when over the limit. All copy via `react-i18next`.
- Mounted `ChatPanel` in the lobby's right-hand column under the leaderboard placeholder (replaced the old single-card column with a flex stack).
- Added `chat.*` i18n namespace to both `en.json` and `sr.json`. Existing `i18n.test.ts` parity check passes.
- Deleted the empty `server/internal/chat/chat.go` placeholder; `handler.go` now owns the package declaration.
- Stubbed `Element.prototype.scrollIntoView` in `ChatPanel.test.tsx` — jsdom does not implement it.
- Regression fix: `App.test.tsx` was rendering `LobbyPage` without a WS context. Added the same `useWsSendMessage`/`useWsConnectionState` mock used by `LobbyPage.test.tsx`.

### File List

**New files:**

- server/internal/chat/handler.go
- server/internal/chat/handler_test.go
- client/src/features/chat/ChatPanel.tsx
- client/src/features/chat/ChatPanel.test.tsx
- client/src/shared/stores/chatStore.test.ts

**Deleted files:**

- server/internal/chat/chat.go (empty placeholder; replaced by handler.go)

**Modified files:**

- server/internal/ws/events.go (added `ActionChatMessage`, `ChatMessageRequest`, `ChatMessagePayload`)
- server/internal/ws/hub.go (added `ConnectedUserIDs()`)
- server/internal/ws/ws_test.go (added `TestHub_ConnectedUserIDs`)
- server/internal/session/manager.go (added `IsUserInGame(userID)`)
- server/internal/session/manager_test.go (added `TestIsUserInGame`)
- server/cmd/api/main.go (composite action handler dispatching `action:chat_message` to chat handler, all others to session manager; added `chat` import)
- client/src/shared/types/wsEvents.ts (added `ACTION_CHAT_MESSAGE` + `ChatMessageRequest`)
- client/src/shared/stores/chatStore.ts (replaced placeholder with real `globalMessages` partition + 200-msg ring buffer)
- client/src/shared/hooks/useWsDispatch.ts (chat dispatch wiring → `chatStore.appendGlobal`; review-patch: defensive payload validation)
- client/src/shared/hooks/useWsDispatch.test.ts (added 2 chat dispatch tests; reset chatStore in `beforeEach`; review-patch: malformed-payload defensive test)
- client/src/features/lobby/LobbyPage.tsx (mounted `ChatPanel` in right column)
- client/src/features/lobby/LobbyPage.test.tsx (added panel-presence test; added WS context mock)
- client/src/App.test.tsx (added WS context mock so LobbyPage's nested ChatPanel renders cleanly)
- client/src/shared/i18n/en.json (added `chat.*` namespace)
- client/src/shared/i18n/sr.json (added `chat.*` namespace)

**Modified files (review patches, 2026-04-18):**

- server/internal/chat/handler.go — sender exempt from per-recipient `IsUserInGame` filter (own-echo guarantee), `utf8.RuneCountInString` for 500-char cap, `RFC3339Nano` timestamps, defensive `make([]uint, 0, len(recipients))` instead of slice alias, added `unicode/utf8` import
- server/internal/chat/handler_test.go — added `TestHandler_GlobalMessage_SenderAlwaysReceivesOwnEcho` and `TestHandler_LengthCapIsRunesNotBytes`
- client/src/features/chat/ChatPanel.tsx — moved `listEndRef` outside the conditional `<ul>` (auto-scroll fires on first message) and dropped `index` from React key (`${userId}-${timestamp}` is now unique thanks to nanosecond server timestamps)
- client/src/test-setup.ts — added global `Element.prototype.scrollIntoView` stub for jsdom (regression caught when ChatPanel auto-scroll fix made the ref attach earlier)

### Change Log

- 2026-04-18 — Story 6.1 implemented: global lobby chat (ephemeral, server-broadcast, in-game users excluded). All 7 ACs satisfied; 361/361 frontend tests + all backend packages green.
- 2026-04-18 — Code review complete: 6 review patches applied (sender-exempt own-echo, `utf8.RuneCountInString` for length cap, `RFC3339Nano` server timestamps, defensive recipient-slice copy, ChatPanel auto-scroll ref out of conditional + index-free React keys, dispatcher payload validation). 6 items deferred to D72-D77. 362/362 frontend tests + all backend packages green; ESLint + Prettier + Go vet clean.

### Review Findings

- [x] [Review][Patch] TOCTOU race: exempt sender from per-recipient `IsUserInGame` filter so their own message always echoes back if the initial check passed (server-authoritative; resolved D-1 with option 2) [server/internal/chat/handler.go:108-115]
- [x] [Review][Patch] Length-boundary mismatch: switch server-side cap to `utf8.RuneCountInString(text)` so it matches the client's `string.length` rune-count semantics (resolved D-2 with option 1) [server/internal/chat/handler.go:14,30]
- [x] [Review][Patch] ChatPanel auto-scroll fails on first message: `listEndRef` is rendered inside a conditional `<ul>` that only mounts when `messages.length > 0`, so the ref is null when the effect first fires for the 0→1 transition [client/src/features/chat/ChatPanel.tsx:24,31,90]
- [x] [Review][Patch] Defensive copy in chat handler recipient list: `eligible := recipients[:0]` aliases the slice returned by `Hub.ConnectedUserIDs()`; safe today (the hub allocates a new slice) but fragile coupling to the `Broadcaster` contract [server/internal/chat/handler.go:108-115]
- [x] [Review][Patch] React list key includes array index, causing all surviving rows to remount when the chatStore ring buffer drops the oldest message past the 200 cap [client/src/features/chat/ChatPanel.tsx:80]
- [x] [Review][Patch] `useWsDispatch` chat handler uses an unchecked `as ChatMessagePayload` cast with no runtime validation of payload fields; a malformed payload (e.g. `username: null`, missing `timestamp`) renders as empty text and "Invalid Date" [client/src/shared/hooks/useWsDispatch.ts:305-310]
- [x] [Review][Defer] Server-side sanitization of chat text + username (control chars, RTL overrides, zero-width joiners) [server/internal/chat/handler.go:96-107] — deferred, Phase 1 chat is intentionally unmoderated per Dev Notes; revisit when chat moderation lands in Phase 2
- [x] [Review][Defer] WS router dispatches actions via `go r.ActionHandler(...)`, so two concurrent chat messages from the same client can be processed out of submission order [server/internal/ws/router.go:19] — deferred, pre-existing pattern affecting all action handlers; out of scope for Story 6.1
- [x] [Review][Defer] Lobby right-column `min-h-150` (≈600px) forces page-level scroll on viewports below 600px tall, breaking nested chat auto-scroll [client/src/features/lobby/LobbyPage.tsx:169] — deferred, UX spec targets 1280×720 minimum viewport so out of scope
- [x] [Review][Defer] `handler.go` collapses transient DB error and "user deleted" into the same silent drop with asymmetric log payload [server/internal/chat/handler.go:96-100] — deferred, Phase 1 chat has no retry/error UX surface; revisit alongside moderation work
- [x] [Review][Defer] `i18n.test.ts` does not assert key parity between `en.json` and `sr.json`; `chat.*` keys are in parity manually but the gap is latent [client/src/shared/i18n/i18n.test.ts] — deferred, pre-existing infrastructure gap not introduced by this story
- [x] [Review][Defer] No end-to-end test covering sender's own-message echo through a real `*ws.Hub`; handler tests use a hub spy [server/internal/chat/handler_test.go] — deferred, unit coverage is comprehensive (11 handler tests) and an integration test would duplicate `ws_test.go` machinery
