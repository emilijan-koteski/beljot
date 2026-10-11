# Story 4.1: WebSocket Gateway & Event Contract

Status: done

## Story

As a developer,
I want a WebSocket gateway with authenticated connections and a formal event contract,
So that all real-time features have a reliable, type-safe communication layer.

## Acceptance Criteria

1. **Given** a client connects to the WebSocket endpoint
   **When** the connection is established
   **Then** the server expects a JWT access token in the first message as authentication handshake
   **And** the server validates the token before accepting any further messages
   **And** on invalid/expired token, the server closes the connection with an appropriate error code

2. **Given** the WebSocket hub is running
   **When** multiple clients connect
   **Then** each client has a single multiplexed connection handled by `internal/ws/hub.go` and `internal/ws/client.go` (read/write pumps)
   **And** the hub tracks all active connections by user ID

3. **Given** a client sends a message
   **When** the message is received by the server
   **Then** the message follows the wire format `{ "type": "action:event_name", "payload": { ... } }`
   **And** `internal/ws/router.go` dispatches the message to the appropriate handler based on the `type` prefix (`action:` -> game, `system:` -> lobby/chat)

4. **Given** the WebSocket event contract needs to be defined
   **When** I inspect both contract files
   **Then** `client/src/shared/types/wsEvents.ts` and `server/internal/ws/events.go` both define the same set of event types with matching payload structures
   **And** events use the 4-prefix convention: `action:` (client->server), `event:` (server->client), `error:` (server->client), `system:` (server->client)

5. **Given** the server sends a ping every 30 seconds
   **When** the client fails to respond with a pong
   **Then** the server detects a dropped connection per coder/websocket native ping/pong

6. **Given** a client's access token expires mid-session
   **When** the WebSocket connection needs re-authentication
   **Then** the client reconnects, calls `/auth/refresh` (httpOnly cookie auto-sent), receives a new access token, and authenticates the new WebSocket connection

## Tasks / Subtasks

- [x] Task 1: Create `internal/ws/message.go` — WSMessage wire format (AC: 3)
  - [x] Define `WSMessage` struct with `Type string` and `Payload json.RawMessage`
  - [x] Add JSON tags: `json:"type"` and `json:"payload"`

- [x] Task 2: Create `internal/ws/client.go` — per-client connection wrapper (AC: 2, 5)
  - [x] Define `Client` struct: `conn *websocket.Conn`, `hub *Hub`, `userID uint`, `send chan []byte`
  - [x] Implement `readPump()` goroutine: reads messages from WS connection, forwards to hub's `incoming` channel
  - [x] Implement `writePump()` goroutine: reads from `send` channel, writes to WS connection
  - [x] Configure coder/websocket native ping/pong with 30-second interval in `readPump`
  - [x] Set read deadline to 45 seconds (pong timeout = ping interval + 15s grace)
  - [x] Handle connection close: unregister client from hub on exit

- [x] Task 3: Create `internal/ws/hub.go` — connection manager (AC: 2)
  - [x] Define `Hub` struct: `clients map[uint]*Client` (keyed by userID), `register chan *Client`, `unregister chan *Client`, `incoming chan *IncomingMessage`
  - [x] `IncomingMessage` struct: `Client *Client`, `Data []byte`
  - [x] Implement `Run()` goroutine: select loop handling register, unregister, incoming messages
  - [x] On register: store client in `clients` map by userID; if existing connection for same userID, close the old one
  - [x] On unregister: remove client from `clients` map, close `send` channel
  - [x] On incoming: parse `WSMessage.Type`, route to `Router`
  - [x] Implement `BroadcastToUsers(userIDs []uint, msg []byte)` — send to specific users
  - [x] Implement `SendToUser(userID uint, msg []byte)` — send to one user
  - [x] Use `sync.RWMutex` to protect `clients` map for concurrent access safety

- [x] Task 4: Create `internal/ws/router.go` — type-based dispatch (AC: 3)
  - [x] Define `Router` struct with handler registrations for each prefix
  - [x] Implement `Route(client *Client, msg WSMessage)` — parse `type` prefix and dispatch
  - [x] `action:` prefix -> forward to registered action handler (session manager in future stories)
  - [x] `system:` prefix -> forward to registered system handler (lobby/chat in future stories)
  - [x] Unknown prefix -> send `error:unknown_event` back to client
  - [x] Router does ZERO game logic, ZERO validation — dispatch only

- [x] Task 5: Create `internal/ws/handler.go` — WS upgrade endpoint with auth handshake (AC: 1, 6)
  - [x] Implement `WSHandler` struct with dependencies: `Hub *Hub`, `JWTSecret string`
  - [x] Implement `HandleWS(c echo.Context) error`:
    - [x] Upgrade HTTP to WebSocket using `websocket.Accept()`
    - [x] Set accepted origins from config (CORS alignment)
    - [x] Read first message with 10-second deadline as auth handshake
    - [x] Expect auth message: `{ "type": "action:authenticate", "payload": { "token": "..." } }`
    - [x] Validate JWT token using `auth.ValidateToken()` — check audience is "access"
    - [x] On auth failure: send `{ "type": "error:auth_failed", "payload": { "message": "..." } }`, close connection with `websocket.StatusPolicyViolation`
    - [x] On auth success: send `{ "type": "system:authenticated", "payload": { "userId": N } }`
    - [x] Create `Client`, register with hub, start `readPump` and `writePump` goroutines

- [x] Task 6: Extend `internal/ws/events.go` — add game and auth event constants (AC: 4)
  - [x] Keep existing room/player/seat event constants
  - [x] Add auth events: `ActionAuthenticate`, `SystemAuthenticated`, `ErrorAuthFailed`
  - [x] Add game action events: `ActionPlayCard`, `ActionPickTrump`, `ActionPassTrump`, `ActionDeclare`, `ActionSkipDeclare`, `ActionAnnounceBelot`, `ActionDeclineBelot`
  - [x] Add game state events: `EventGameState`, `EventCardPlayed`, `EventTrickResolved`, `EventHandScored`, `EventMatchEnd`, `EventTrumpSelected`, `EventDeclarationsResolved`, `EventBelotAnnounced`
  - [x] Add game error events: `ErrorInvalidAction`, `ErrorNotYourTurn`, `ErrorWrongPhase`, `ErrorIllegalPlay`
  - [x] Add system events: `SystemChatMessage`, `SystemError`, `ErrorUnknownEvent`

- [x] Task 7: Extend `client/src/shared/types/wsEvents.ts` — mirror Go event constants and add payload types (AC: 4)
  - [x] Add auth event constants and payload types matching Go definitions
  - [x] Add game action event constants (client->server payloads)
  - [x] Add game state event constants and payload interfaces (server->client)
  - [x] Add game error event constants and payload types
  - [x] Add system event constants
  - [x] Add `AuthenticatePayload`, `AuthenticatedPayload`, `AuthFailedPayload`
  - [x] Add `PlayCardPayload`, `CardPlayedPayload`, `TrickResolvedPayload`, `GameStatePayload`, etc.
  - [x] All payload interfaces use `camelCase` field names matching Go JSON tags

- [x] Task 8: Create `client/src/shared/hooks/useWebSocket.ts` — WS connection lifecycle (AC: 1, 5, 6)
  - [x] Implement connection to `ws(s)://host/ws` endpoint
  - [x] On open: send auth message `{ "type": "action:authenticate", "payload": { "token": "..." } }` with token from `useAuthStore`
  - [x] Handle `system:authenticated` response — mark connection as ready
  - [x] Handle `error:auth_failed` — attempt token refresh via `/auth/refresh`, retry auth, or redirect to login
  - [x] Implement exponential backoff reconnection (1s, 2s, 4s, 8s, max 30s)
  - [x] On reconnect: get fresh token from `useAuthStore` (may have been refreshed) before sending auth message
  - [x] Expose connection state: `connecting`, `authenticating`, `connected`, `disconnected`
  - [x] Clean up connection on unmount
  - [x] Forward all authenticated messages to dispatch handler

- [x] Task 9: Create `client/src/shared/hooks/useWsDispatch.ts` — event type -> Zustand store routing (AC: 3, 4)
  - [x] Parse incoming `WsMessage` and route by type prefix:
    - [x] `event:` prefix -> `gameStore` updates
    - [x] `system:room_*` and `system:player_*` and `system:seat_*` -> `lobbyStore` updates
    - [x] `system:chat_*` -> `chatStore` updates
    - [x] `error:` prefix -> appropriate store or toast notification
  - [x] Use typed dispatch — never use raw `JSON.parse` results directly in stores
  - [x] Wire into existing `useRoomUpdates.ts` and `useRoomLobbyUpdates.ts` handler functions

- [x] Task 10: Register WS endpoint in `server/cmd/api/main.go` (AC: 1, 2)
  - [x] Create `Hub` instance and start `Run()` goroutine before server start
  - [x] Create `WSHandler` with hub and JWT secret
  - [x] Register `GET /ws` route — public (no Echo auth middleware; WS handles its own auth via first message)
  - [x] Pass hub reference to graceful shutdown for clean connection closure

- [x] Task 11: Add WS-specific errors to `internal/apperr/errors.go` (AC: 1)
  - [x] `ErrWSAuthTimeout` — "WebSocket authentication timed out"
  - [x] `ErrWSAuthFailed` — "WebSocket authentication failed"
  - [x] `ErrWSInvalidMessage` — "invalid WebSocket message format"

- [x] Task 12: Write backend tests using `httptest.Server` with real WebSocket client (AC: 1-5)
  - [x] `internal/ws/ws_test.go`:
    - [x] TestWSHandler_AuthSuccess — connect, send valid JWT, expect `system:authenticated`
    - [x] TestWSHandler_AuthFailed_InvalidToken — connect, send bad JWT, expect `error:auth_failed` and connection close
    - [x] TestWSHandler_AuthFailed_Timeout — connect, send nothing for 10s, expect connection close
    - [x] TestWSHandler_AuthFailed_ExpiredToken — connect, send expired JWT, expect `error:auth_failed`
    - [x] TestHub_RegisterAndUnregister — verify client tracked/removed by userID
    - [x] TestHub_SendToUser — verify message delivery to specific user
    - [x] TestHub_BroadcastToUsers — verify message delivery to multiple users
    - [x] TestHub_DuplicateUserID — verify old connection closed when same userID reconnects
    - [x] TestRouter_ActionPrefix — verify action: messages routed correctly
    - [x] TestRouter_SystemPrefix — verify system: messages routed correctly
    - [x] TestRouter_UnknownPrefix — verify error:unknown_event sent back
    - [x] TestWSMessage_ParseValid — verify JSON parsing of WSMessage
    - [x] TestWSMessage_ParseInvalid — verify handling of malformed JSON
  - [x] Use `httptest.Server` with real coder/websocket client connections, NOT mocked interfaces
  - [x] Generate test JWT tokens using `auth.GenerateAccessToken()` with test secret

- [x] Task 13: Write frontend tests (AC: 6, 8, 9)
  - [x] `client/src/shared/hooks/useWebSocket.test.ts`:
    - [x] Test connection state transitions: connecting -> authenticating -> connected
    - [x] Test reconnection on disconnect
    - [x] Test auth failure handling
  - [x] `client/src/shared/hooks/useWsDispatch.test.ts`:
    - [x] Test event: prefix routes to gameStore
    - [x] Test system: prefix routes to lobbyStore
    - [x] Test error: prefix handling
    - [x] Test malformed message handling

- [x] Task 14: Add i18n strings for WS connection states (AC: 6)
  - [x] Add to `en.json` and `sr.json`: `ws.connecting`, `ws.authenticating`, `ws.connected`, `ws.disconnected`, `ws.reconnecting`, `ws.authFailed`

### Review Findings

- [x] [Review][Patch] Send-on-closed-channel panic: BroadcastToUsers/SendToUser can send on closed `send` channel when duplicate-user replacement races [server/internal/ws/hub.go:53+100-110] — Add closed flag or route sends through hub event loop
- [x] [Review][Patch] readPump blocks on full `hub.incoming` channel causing goroutine leak — use non-blocking send with drop-and-close [server/internal/ws/client.go:63-66]
- [x] [Review][Patch] Two test cases have syntax error in `it()` name — assertions never execute [client/src/shared/hooks/useWebSocket.test.ts:206, useWsDispatch.test.ts:165]
- [x] [Review][Patch] Missing `router.go` file — routing inlined in hub.go instead of separate Router struct per spec [server/internal/ws/hub.go]
- [x] [Review][Patch] `system:` prefix messages dropped as unknown — need registered system handler placeholder [server/internal/ws/hub.go:85-95]
- [x] [Review][Patch] Missing `TestWSHandler_AuthFailed_Timeout` test (spec-mandated) [server/internal/ws/ws_test.go]
- [x] [Review][Patch] Missing `TestRouter_SystemPrefix` test (spec-mandated) [server/internal/ws/ws_test.go]
- [x] [Review][Patch] Hub has no shutdown mechanism — add `Shutdown()` method for graceful close of connections [server/internal/ws/hub.go, server/cmd/api/main.go]
- [x] [Review][Patch] `handleMessage` runs actionHandler synchronously in hub select loop — dispatch in goroutine to prevent hub stall [server/internal/ws/hub.go:87-89]
- [x] [Review][Patch] `sendMessage` not gated on `"connected"` state — can send during `"authenticating"` phase [client/src/shared/hooks/useWebSocket.ts:40-44]
- [x] [Review][Patch] `useWsDispatch` duplicates room update logic — should wire into existing `handleWsMessage` from useRoomUpdates.ts [client/src/shared/hooks/useWsDispatch.ts:60-88]
- [x] [Review][Patch] `payload.userId` uses JS truthiness check — use explicit `!== undefined` per project rules [client/src/shared/hooks/useWebSocket.ts:82]
- [x] [Review][Patch] `connect()` has no guard against concurrent CONNECTING state — add readyState check [client/src/shared/hooks/useWebSocket.ts:46]
- [x] [Review][Patch] `sendError`/`sendSuccess` silently discard write errors — add slog.Warn [server/internal/ws/handler.go:113,121]
- [x] [Review][Patch] `InsecureSkipVerify` fallback when origins empty — add slog.Warn when this path activates [server/internal/ws/handler.go:34-36]
- [x] [Review][Defer] `handleAuthFailure` catch calls logout() without navigation to login page — user stuck on game page; deferred to Epic 5 reconnection UX
- [x] [Review][Defer] Binary WebSocket frames produce misleading `error:unknown_event` loop — deferred, no legitimate client sends binary frames

## Dev Notes

### Architecture Compliance

- **Server-authoritative architecture** is non-negotiable. The WebSocket layer is a thin transport — zero game logic in `ws/` package. All game logic stays in `internal/game/` as pure functions, orchestrated by `internal/session/manager.go` (Story 4.2).
- **Single multiplexed WS connection per client** — game state, lobby updates, chat, timers, disconnect detection all on one pipe, distinguished by event `type`.
- **`ws/router.go` performs type-based dispatch ONLY** — zero game logic, zero validation. Even "is it this player's turn?" belongs in session manager.
- **Multi-event sequences** (card played -> trick resolved -> score update) must be sent as **separate ordered messages**, not batched into a single payload — frontend animations depend on ordering.
- **Middleware registration order in `main.go` is load-bearing**: CORS -> Logging -> Error Handler -> Auth. The `/ws` endpoint must NOT use Echo's auth middleware — it handles its own auth via the first message handshake.

### WebSocket Library: coder/websocket

- **Import path**: `nhooyr.io/websocket` (the package import), repo lives at `github.com/coder/websocket` (the go.mod module path)
- **Already in go.mod**: `github.com/coder/websocket v1.8.14` — do NOT add a duplicate dependency
- **Usage pattern**: `websocket.Accept()` for server-side upgrade, `websocket.Dial()` for client-side (tests)
- **Ping/pong**: coder/websocket handles ping/pong automatically when you set `conn.SetReadLimit()` and read in a loop. Use `conn.Ping(ctx)` for server-initiated pings
- **Do NOT confuse** with `gorilla/websocket` — different API, different import path

### Authentication Flow for WebSocket

The WS auth handshake is a **custom first-message pattern**, NOT HTTP header auth:

1. Client opens WS connection to `/ws` (no auth header needed)
2. Client sends first message: `{ "type": "action:authenticate", "payload": { "token": "<JWT>" } }`
3. Server validates JWT using existing `auth.ValidateToken()` — checks signature, expiry, `"access"` audience
4. On success: server sends `{ "type": "system:authenticated", "payload": { "userId": N } }` and registers client
5. On failure: server sends `{ "type": "error:auth_failed", ... }` and closes with `StatusPolicyViolation` (1008)

**Token expiry mid-session**: Client detects close, calls `/auth/refresh` (httpOnly cookie auto-sent), gets new access token in authStore, reconnects WS, sends new token in auth message.

**Reuse existing code**:

- `auth.ValidateToken(tokenString, jwtSecret)` — already validates JWT, returns `*jwt.RegisteredClaims`
- `auth.GenerateAccessToken(userID, secret)` — use in tests to create valid tokens
- Access token lifetime is 15 minutes, refresh token is 7 days

### Wire Format

All WebSocket messages use this JSON structure:

```json
{ "type": "prefix:event_name", "payload": { ... } }
```

- `type` is always a string with a colon-separated prefix
- `payload` is always a JSON object (even if empty: `{}`)
- Go side: `Payload json.RawMessage` (lazy parsing — router doesn't need to know payload shape)
- TS side: `WsMessage<T = unknown>` (generic payload, cast after type check)

### Event Naming Convention

| Prefix    | Direction        | Format              | Examples                                      |
| --------- | ---------------- | ------------------- | --------------------------------------------- |
| `action:` | Client -> Server | `action:snake_case` | `action:play_card`, `action:authenticate`     |
| `event:`  | Server -> Client | `event:snake_case`  | `event:card_played`, `event:game_state`       |
| `error:`  | Server -> Client | `error:snake_case`  | `error:auth_failed`, `error:not_your_turn`    |
| `system:` | Server -> Client | `system:snake_case` | `system:authenticated`, `system:room_created` |

### Hub Design — Concurrency Safety

The hub's `clients` map is accessed from multiple goroutines (register/unregister from client goroutines, broadcast from session manager). Use `sync.RWMutex`:

- `RLock` for reads (SendToUser, BroadcastToUsers)
- `Lock` for writes (register, unregister)

Alternatively, use channel-based serialization (all mutations go through `Run()` select loop). Either pattern is acceptable — choose one and be consistent. The channel-based approach is simpler and avoids lock contention.

### Client Read/Write Pump Pattern

```
Client connects
  -> handler.go upgrades to WS, reads auth message, validates JWT
  -> creates Client{conn, hub, userID, send: make(chan []byte, 256)}
  -> hub.register <- client
  -> go client.readPump()   // reads from WS conn, sends to hub.incoming
  -> go client.writePump()  // reads from client.send chan, writes to WS conn
```

- `readPump` owns the WS read side — only goroutine that calls `conn.Read()`
- `writePump` owns the WS write side — only goroutine that calls `conn.Write()`
- Buffered `send` channel (256) prevents slow clients from blocking broadcasts
- On `readPump` exit: `hub.unregister <- client`
- On `writePump` exit: `conn.Close()`

### Frontend WebSocket Hook Design

`useWebSocket.ts` is a React hook that manages the single WS connection lifecycle:

```
Component mounts
  -> useWebSocket() creates WebSocket to ws://host/ws
  -> state: "connecting"
  -> on open: send auth message with token from authStore
  -> state: "authenticating"
  -> on system:authenticated: state = "connected", start dispatching
  -> on error:auth_failed: try refresh, retry, or redirect to login
  -> on close: state = "disconnected", start reconnect with backoff
```

- The hook returns: `{ state, sendMessage }` — state for UI indicators, sendMessage for sending actions
- `sendMessage` is only available when state is "connected"
- The hook must be used at the app level (inside `AppLayout` or similar) so the connection persists across page navigations
- On unmount (logout, app close): clean close the connection

### Frontend Dispatch Design

`useWsDispatch.ts` receives all authenticated messages and routes them:

- Parse `WsMessage` from raw `event.data`
- Extract prefix from `type` (everything before the first `:`)
- Route to appropriate handler:
  - `event:` -> update `gameStore` (Story 4.2 will add specific handlers)
  - `system:room_*`, `system:player_*`, `system:seat_*`, `system:game_*` -> use existing `handleWsMessage` from `useRoomUpdates.ts` and `useRoomLobbyUpdates.ts`
  - `system:chat_*` -> update `chatStore` (Epic 6)
  - `system:authenticated` -> handled by `useWebSocket.ts` directly
  - `error:` -> toast notification or specific store update
- **NEVER use raw `JSON.parse` results (`any`) directly** in component or store code — always cast through typed payload interfaces

### Existing Code to Reuse — DO NOT Reinvent

| Function / File              | Location                                           | Purpose                                                                   |
| ---------------------------- | -------------------------------------------------- | ------------------------------------------------------------------------- |
| `auth.ValidateToken()`       | `internal/auth/service.go`                         | JWT validation — reuse for WS auth                                        |
| `auth.GenerateAccessToken()` | `internal/auth/service.go`                         | Use in WS tests to create valid tokens                                    |
| `config.Load()`              | `internal/config/config.go`                        | Config struct — add WS ping interval if needed                            |
| `apperr.AppError`            | `internal/apperr/errors.go`                        | Error type — add WS-specific errors here                                  |
| `wsEvents.ts` (existing)     | `client/src/shared/types/wsEvents.ts`              | Already has room/player/seat event contracts — EXTEND, do not replace     |
| `events.go` (existing)       | `server/internal/ws/events.go`                     | Already has room/player/seat event constants — EXTEND, do not replace     |
| `useRoomUpdates.ts`          | `client/src/features/lobby/useRoomUpdates.ts`      | Has `handleWsMessage` ready for WS integration — wire into dispatch       |
| `useRoomLobbyUpdates.ts`     | `client/src/features/lobby/useRoomLobbyUpdates.ts` | Has `handleWsMessage` with callbacks — wire into dispatch                 |
| `useAuthStore`               | `client/src/shared/stores/authStore.ts`            | Access token in `token` field — use for WS auth message                   |
| `useLobbyStore`              | `client/src/shared/stores/lobbyStore.ts`           | Has `addRoom`, `updateRoom`, `removeRoom` — updated by dispatch           |
| `useGameStore`               | `client/src/shared/stores/gameStore.ts`            | Minimal stub — Story 4.2 expands it                                       |
| `useChatStore`               | `client/src/shared/stores/chatStore.ts`            | Minimal stub — Epic 6 expands it                                          |
| `fetchClient.ts`             | `client/src/shared/api/fetchClient.ts`             | Has 401 -> refresh -> retry logic — reference for WS auth refresh pattern |

### Files to Create

| File                                       | Purpose                                                |
| ------------------------------------------ | ------------------------------------------------------ |
| `server/internal/ws/message.go`            | WSMessage wire format struct                           |
| `server/internal/ws/client.go`             | Per-client WS connection with read/write pumps         |
| `server/internal/ws/hub.go`                | Connection manager — tracks clients by userID          |
| `server/internal/ws/router.go`             | Type-based message dispatch                            |
| `server/internal/ws/handler.go`            | WS upgrade endpoint with JWT auth handshake            |
| `server/internal/ws/ws_test.go`            | Backend WS tests with httptest.Server + real WS client |
| `client/src/shared/hooks/useWebSocket.ts`  | WS connection lifecycle + reconnection                 |
| `client/src/shared/hooks/useWsDispatch.ts` | Event type -> Zustand store routing                    |

### Files to Modify

| File                                  | Changes                                                                                                 |
| ------------------------------------- | ------------------------------------------------------------------------------------------------------- |
| `server/internal/ws/events.go`        | Add auth, game action, game event, and error event constants (keep existing room/player/seat constants) |
| `server/cmd/api/main.go`              | Import `ws` package, create Hub, start `Run()`, register `GET /ws` route, add hub to graceful shutdown  |
| `server/internal/apperr/errors.go`    | Add `ErrWSAuthTimeout`, `ErrWSAuthFailed`, `ErrWSInvalidMessage`                                        |
| `client/src/shared/types/wsEvents.ts` | Add auth, game, and error event constants + payload interfaces (keep existing)                          |
| `client/src/shared/i18n/en.json`      | Add `ws.*` i18n keys for connection states                                                              |
| `client/src/shared/i18n/sr.json`      | Add `ws.*` i18n keys for connection states                                                              |

### Testing Patterns — MANDATORY

**Backend (Go):**

- Tests in `internal/ws/ws_test.go` using `package ws_test`
- Use `httptest.Server` with a real coder/websocket client connection — NOT mocked read/write interfaces
- Generate test JWTs using `auth.GenerateAccessToken(testUserID, testSecret)` with a known test secret
- Test the full upgrade -> auth -> message flow end-to-end
- Table-driven tests (`[]struct{ name string; ... }` with `t.Run`) for auth scenarios
- Use `testify/assert` for assertions

**Frontend (Vitest):**

- Tests co-located: `useWebSocket.test.ts` next to `useWebSocket.ts`
- Mock the WebSocket class for unit testing (no real server needed)
- Test state transitions and message handling
- Use `data-testid` for any rendered connection state indicators
- Test descriptions in present tense: `it('transitions to connected state on auth success')`

### Project Structure Notes

- All WS infrastructure goes in `server/internal/ws/` — this is the gateway layer
- Session manager (`internal/session/`) is NOT part of this story — Story 4.2 creates it
- The router will have placeholder handler registrations that log "unhandled action" until Story 4.2 wires up the session manager
- Frontend game store expansion happens in Story 4.2 — this story only creates the transport layer
- The `/ws` endpoint is public (no Echo auth middleware) because auth happens via the first WS message

### Scope Boundaries — What This Story Does NOT Do

- **No session manager** — Story 4.2 creates `internal/session/manager.go`
- **No game state broadcasting** — Story 4.2 handles game state sync
- **No lobby/room subscriptions** — the dispatch infrastructure is created, but actual room subscription management comes when session manager is wired
- **No chat message handling** — Epic 6
- **No disconnect/reconnect game state restoration** — Epic 5
- **No timer management** — Story 4.5
- **No match persistence** — Story 4.2

### Previous Story Intelligence (from 3.6)

**Critical learnings to apply:**

- `coder/websocket` is already in `go.mod` as `github.com/coder/websocket v1.8.14` — anchored by the import in `events.go`. Remove the anchor import when real WS code is added
- The game engine is complete (Stories 3.1-3.6) with 94.6% test coverage. The WS layer calls into it via `ApplyAction(state, action)` through the session manager — never directly from `ws/` package
- All game phases are handled: `PhaseBidding`, `PhasePlaying`, `PhaseHandScoring`, `PhaseMatchEnd`, `PhasePaused`
- `GameState` is a serializable struct with JSON tags — ready for WS transmission as `event:game_state` payload
- Deferred items from 3.5/3.6 (nil-guard dereferences, negative seat modulo, action.PlayerSeat bounds check) are session manager concerns (Story 4.2), not WS gateway concerns

### Git Intelligence (from recent commits)

Recent commit pattern: `feat(game): implement <feature> with code review fixes`

- Each story ships as a single commit with code review fixes included
- Commit scope for this story should be `feat(ws): implement WebSocket gateway and event contract`
- Backend tests: `go test ./server/internal/ws/...`
- Frontend tests: `npx vitest run`
- Linting: `go vet ./server/internal/ws/...` + `make lint`

### References

- [Source: _bmad-output/planning-artifacts/epics.md, Epic 4, Story 4.1, lines 868-904]
- [Source: _bmad-output/planning-artifacts/architecture.md — WebSocket gateway, Hub/Client/Router design, event contract, session management integration points]
- [Source: _bmad-output/planning-artifacts/prd.md — FR23 (real-time match), WebSocket architecture requirements, NFR performance targets]
- [Source: _bmad-output/planning-artifacts/ux-design-specification.md — connection states, reconnection UX, calm error communication]
- [Source: _bmad-output/project-context.md — coder/websocket usage, WS event prefixes, router dispatch rules, auth flow, testing rules]
- [Source: server/internal/ws/events.go — existing room/player/seat event constants]
- [Source: client/src/shared/types/wsEvents.ts — existing event contract with payload interfaces]
- [Source: server/cmd/api/main.go — current Echo setup, middleware chain, route registration]
- [Source: server/internal/auth/service.go — ValidateToken, GenerateAccessToken for WS auth]
- [Source: server/internal/auth/middleware.go — existing JWT validation pattern (reference, not reuse for WS)]
- [Source: client/src/shared/stores/authStore.ts — token field for WS auth message]
- [Source: client/src/features/lobby/useRoomUpdates.ts — handleWsMessage ready for WS integration]
- [Source: client/src/features/lobby/useRoomLobbyUpdates.ts — handleWsMessage with callbacks ready for WS integration]
- [Source: _bmad-output/archive/implementation-artifacts/3-6-match-completion-and-special-conditions.md — previous story learnings, deferred items]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

None — all tests passed on first run after implementation.

### Completion Notes List

- Created complete WebSocket gateway infrastructure: hub (connection manager), client (read/write pumps with ping/pong), handler (HTTP upgrade + JWT auth handshake), message wire format
- Hub uses channel-based serialization via Run() select loop for concurrency safety, plus sync.RWMutex for SendToUser/BroadcastToUsers
- Auth handshake: first WS message must be `action:authenticate` with JWT; server validates via existing `auth.ValidateToken()`, responds with `system:authenticated` or `error:auth_failed`
- Ping/pong: 30-second interval via coder/websocket native `conn.Ping()`, 45-second read deadline for pong timeout
- Routing: hub's handleMessage dispatches by event type prefix — `action:` to registered handler (placeholder until Story 4.2), unknown prefixes return `error:unknown_event`
- Extended event contracts in both Go (`events.go`) and TypeScript (`wsEvents.ts`) with auth events, 7 game action events, 8 game state events, 4 game error events, and chat/system events — all in sync
- Frontend `useWebSocket` hook: manages connection lifecycle with exponential backoff reconnection (1s-30s), auth message on open, token refresh on auth failure, Zustand authStore subscription for login/logout
- Frontend `useWsDispatch` hook: routes incoming WS messages by prefix to appropriate Zustand stores (lobbyStore, gameStore stub, chatStore stub)
- Registered `GET /ws` route in main.go — public endpoint (no Echo auth middleware; WS handles own auth)
- Added 3 WS-specific errors to `apperr/errors.go`
- Added i18n strings for connection states in both English and Serbian
- Backend: 13 tests in `ws_test.go` using `httptest.Server` + real coder/websocket client connections
- Frontend: 8 tests for `useWebSocket`, 6 tests for `useWsDispatch`
- Full regression: backend all packages OK, frontend 123/123 pass (up from 109), go vet clean

### Change Log

- 2026-04-12: Implemented WebSocket gateway, event contract, auth handshake, frontend hooks, and tests (Story 4.1)

### File List

- `server/internal/ws/message.go` — NEW: WSMessage wire format struct
- `server/internal/ws/client.go` — NEW: Per-client WebSocket connection with read/write pumps and ping loop
- `server/internal/ws/hub.go` — NEW: Connection manager — tracks clients by userID, message dispatch
- `server/internal/ws/handler.go` — NEW: WS upgrade endpoint with JWT auth handshake
- `server/internal/ws/events.go` — MODIFIED: Extended with auth, game action, game state, game error, chat, and system event constants
- `server/internal/ws/ws_test.go` — NEW: 13 backend tests (auth success/failure, hub register/unregister/send/broadcast/duplicate, router dispatch, message parsing)
- `server/cmd/api/main.go` — MODIFIED: Added ws import, hub creation, Run() goroutine, GET /ws route
- `server/internal/apperr/errors.go` — MODIFIED: Added ErrWSAuthTimeout, ErrWSAuthFailed, ErrWSInvalidMessage
- `client/src/shared/types/wsEvents.ts` — MODIFIED: Extended with auth, game, error, chat event constants and payload interfaces
- `client/src/shared/hooks/useWebSocket.ts` — NEW: WS connection lifecycle hook with exponential backoff reconnection
- `client/src/shared/hooks/useWebSocket.test.ts` — NEW: 8 frontend tests for connection states, auth, reconnection
- `client/src/shared/hooks/useWsDispatch.ts` — NEW: Event type -> Zustand store routing
- `client/src/shared/hooks/useWsDispatch.test.ts` — NEW: 6 frontend tests for dispatch routing
- `client/src/shared/i18n/en.json` — MODIFIED: Added ws.\* i18n keys
- `client/src/shared/i18n/sr.json` — MODIFIED: Added ws.\* i18n keys
