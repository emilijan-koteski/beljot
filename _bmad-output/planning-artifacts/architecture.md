---
stepsCompleted: [1, 2, 3, 4, 5, 6, 7, 8]
lastStep: 8
status: "complete"
completedAt: "2026-04-05"
inputDocuments:
  - product-brief-beljot-2026-02-21.md
  - prd.md
  - prd-validation-report.md
  - ux-design-specification.md
workflowType: "architecture"
project_name: "beljot"
user_name: "Emilijan"
date: "2026-04-05"
---

# Architecture Decision Document

_This document builds collaboratively through step-by-step discovery. Sections are appended as we work through each architectural decision together._

## Project Context Analysis

### Requirements Overview

**Functional Requirements:**

52 FRs across 8 domain groups, with clear Phase 1/Phase 2+ separation (though not labeled in the FR section):

| Domain                  | FRs       | Phase 1       | Phase 2+        | Architectural Weight                                           |
| ----------------------- | --------- | ------------- | --------------- | -------------------------------------------------------------- |
| User Account Management | FR1–FR6   | FR1–FR2, FR4  | FR3, FR5, FR6   | Auth system, session management, profile storage               |
| Game Rules Engine       | FR7–FR15  | FR7, FR9–FR14 | FR8, FR15       | Core server-side state machine, variant-branching rules engine |
| Lobby & Room Management | FR16–FR22 | FR16–FR22     | —               | Room lifecycle, lobby state broadcasting, matchmaking queue    |
| Real-Time Game Session  | FR23–FR29 | FR23–FR28     | FR28a, FR29     | WebSocket game sync, pause/reconnect system, auto-play         |
| Communication           | FR30–FR32 | FR30–FR31     | FR32            | Global + match-scoped chat channels                            |
| Player Progression      | FR33–FR40 | —             | FR33–FR40       | XP/level system, ELO engine, rank tiers, seasons               |
| Stats & Match History   | FR41–FR43 | FR41 (basic)  | FR42–FR43       | Match recording, career statistics, partial XP                 |
| Platform & Localization | FR44–FR52 | FR44, FR46    | FR45, FR47–FR52 | i18n framework, future mobile/social extensibility             |

**Phase 1 MVP core:** Auth + Bitola rules engine + lobby/rooms + real-time game session + chat + disconnect handling + basic match history. Approximately 25 FRs in active scope.

**Non-Functional Requirements:**

13 NFRs across 4 categories — all with specific numeric targets:

| Category    | Key Constraints                                                                               | Architectural Impact                                                             |
| ----------- | --------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| Performance | Timer sync ±1s, state render <200ms, card sync <500ms, page load <3s, WS reconnect <1s        | Server clock authority, optimized WebSocket payloads, lightweight SPA bundle     |
| Security    | HTTPS/WSS, hashed passwords, server-only game logic, time-limited tokens, data access control | Server-authoritative architecture is non-negotiable; zero client-side game logic |
| Scalability | Phase 1: 10 concurrent games, Phase 2: 50 concurrent, horizontal scaling ready                | Stateless-friendly server design, externalized game state storage                |
| Reliability | 99.5% uptime, <5% disconnection rate, full state preservation, single-player isolation        | Durable game state, graceful degradation, connection health monitoring           |

**Scale & Complexity:**

- Primary domain: Full-stack real-time web application (multiplayer card game)
- Complexity level: Medium-High
- Estimated architectural components: ~12–15 major components (auth service, rules engine, game session manager, lobby/room manager, matchmaking, chat, WebSocket gateway, timer service, progression/ELO engine, stats recorder, i18n layer, client SPA, data persistence layer)

### Technical Constraints & Dependencies

**Hard constraints from PRD + UX:**

1. **Server-authoritative architecture** — all game logic server-side; client renders received state only (NFR Security + PRD Web App Requirements)
2. **React SPA** — UX spec is built around React + Tailwind + shadcn/ui component system
3. **WebSocket for all real-time features** — game state, lobby, chat, disconnect detection, timer sync
4. **Desktop web only** — 1280x720 minimum viewport, evergreen browsers, no mobile
5. **i18n at launch** — English + Serbian (Latin), extensible architecture for future languages
6. **No AI/bot opponents** — disconnection = pause or abandon, never AI fill-in
7. **Solo developer** — architecture must be pragmatically scoped; avoid over-engineering

**Implicit constraints:**

8. **Greenfield project** — no legacy systems, no migration concerns, full technology freedom (within React SPA constraint)
9. **Passion project budget** — hosting/infrastructure costs should be minimized; no enterprise-tier services assumed
10. **Phase 1 scale is small** — 10 concurrent games, ~40 players. Architecture should be right-sized for this, not over-provisioned

### Cross-Cutting Concerns Identified

1. **Authentication & Authorization** — Session tokens needed across HTTP (lobby, profile, stats) and WebSocket (game, chat) channels. Reconnection must restore authenticated game state.

2. **WebSocket Connection Lifecycle** — Single persistent connection per client handling: lobby presence, room state, game state sync, chat messages, timer events, disconnect detection. Connection loss triggers reconnect attempts within 1s (NFR).

3. **Game State Consistency** — The server-side game state is the single source of truth. Every client action (card play, declaration, trump pick) is a request to the server; the server validates, updates state, and broadcasts. No optimistic client updates for game logic.

4. **Timer Synchronization** — Per-move timers must stay within ±1s of server time across all clients. Reconnect countdown timers broadcast to remaining players. Season countdown (Phase 2) visible in lobby.

5. **Internationalization (i18n)** — All user-facing strings (lobby, game prompts, chat system messages, rules reference, error messages) must go through an i18n layer from day one.

6. **Error & Disconnect Resilience** — Every real-time feature must handle: connection loss, reconnection, mid-action disconnection (e.g., disconnect after card play sent but before broadcast received), and graceful degradation.

7. **Phased Feature Delivery** — Architecture must cleanly support Phase 1 (Bitola only, casual only) while making Phase 2 additions (Croatian variant, ranked/ELO, progression) a natural extension, not a rewrite.

## Starter Template Evaluation

### Primary Technology Domain

Full-stack real-time web application — TypeScript React SPA frontend + Go backend, PostgreSQL database, WebSocket real-time communication. Split-language monorepo.

### Starter Options Considered

| Option                         | Description                                                                        | Verdict                                                                               |
| ------------------------------ | ---------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------- |
| Composed (Vite + Go module)    | Official Vite react-swc-ts template + Go module with Echo, assembled in a monorepo | **Selected** — clean foundations, official tooling, matches developer experience      |
| Community Go+React boilerplate | Third-party monorepo templates                                                     | Rejected — CRUD-focused, poorly maintained, not suited to real-time multiplayer       |
| Turborepo/Nx monorepo          | JS-focused monorepo orchestration                                                  | Rejected — designed for JS/TS monorepos, adds complexity with no value for Go backend |

### Selected Starter: Composed (Vite + Go Module Monorepo)

**Rationale for Selection:**

- Developer already has intermediate experience with React+TypeScript, Echo, GORM, and PostgreSQL — zero learning curve on core tools
- Official starters (Vite, Go modules) are always current and well-documented
- Split-language monorepo is best served by simple directory structure + Makefile, not JS-focused tooling
- Composing from clean starters avoids inheriting opinionated boilerplate decisions that don't fit a real-time multiplayer game

**Initialization Commands:**

```bash
# Frontend
npm create vite@latest client -- --template react-swc-ts
cd client && npx shadcn@latest init

# Backend
mkdir -p server/cmd/api server/internal
cd server && go mod init github.com/yourusername/beljot/server
go get github.com/labstack/echo/v4
go get gorm.io/gorm gorm.io/driver/postgres
go get nhooyr.io/websocket
```

### Architectural Decisions Provided by Starter

**Language & Runtime:**

- Frontend: TypeScript (strict mode), React 19, compiled via SWC
- Backend: Go (latest stable), compiled to single binary
- No shared type generation between frontend/backend (manual sync or future codegen)

**Styling Solution:**

- Tailwind CSS v4 via `@tailwindcss/vite` plugin — utility-first, custom design token system for Balatro register
- shadcn/ui components copied into codebase — fully owned, themed to project tokens

**Build Tooling:**

- Frontend: Vite 8 (dev server + production bundler), SWC for fast TypeScript compilation
- Backend: `go build` producing a single static binary
- Orchestration: Makefile for unified dev/build/test commands

**Testing Framework:**

- Frontend: Vitest (Vite-native, Jest-compatible API)
- Backend: Go standard `testing` package + testify (assertions)

**Code Organization:**

- Frontend: `src/` with component-based structure, feature folders for game/lobby/profile
- Backend: `cmd/` for entrypoints, `internal/` for private packages (following Go project layout conventions)

**Development Experience:**

- Vite HMR for instant frontend updates
- Go's built-in tooling (`go vet`, `go fmt`, `go test`)
- Docker Compose for local PostgreSQL
- Makefile: `make dev` (concurrent client + server), `make build`, `make test`, `make migrate`

**Note:** Project initialization using these commands should be the first implementation story.

## Core Architectural Decisions

### Decision Priority Analysis

**Critical Decisions (Block Implementation):**

- In-memory game state as serializable structs + PostgreSQL persistence
- JWT authentication (access token in memory, refresh token in httpOnly cookie)
- JSON WebSocket message protocol with typed events and formal contract
- Server-authoritative game logic — rules engine as pure function (state + action → new state)
- Absolute server timestamps for all timer synchronization

**Important Decisions (Shape Architecture):**

- Partitioned Zustand stores (auth, lobby, game, chat)
- React Router for client routing
- react-i18next for internationalization
- golang-migrate for database migrations + seed script
- Caddy on host, app + PostgreSQL in Docker containers
- Single multiplexed WebSocket connection per client
- Echo v4 (stable, supported through Dec 2026)

**Deferred Decisions (Post-MVP):**

- Redis caching layer (Phase 2, when scale justifies it)
- Redis for game state (Phase 2, if horizontal scaling needed)
- Prometheus + Grafana monitoring (Phase 2)
- Echo v5 migration (after v5 has 6+ months of community hardening)
- API versioning (not needed until public API exists)

### Data Architecture

| Decision            | Choice                                              | Rationale                                                                                                                                                                                                 |
| ------------------- | --------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Active game state   | In-memory as serializable Go structs (not raw maps) | Fastest read/write; serializable struct enables reconnection state snapshots and future Redis migration as a mechanical change. Phase 1 scale (10 concurrent games) doesn't justify external state store. |
| Persistent storage  | PostgreSQL (via GORM)                               | Accounts, match history, stats, leaderboard, room config. Developer's known stack.                                                                                                                        |
| Migration strategy  | golang-migrate + seed script                        | Explicit versioned SQL migrations with rollback support. `make seed` target populates dev database with test accounts, rooms, and game states in various phases for faster development.                   |
| Caching             | None (Phase 1)                                      | 40 concurrent players don't justify a caching layer. Design DB queries with caching in mind (clean single queries, no N+1) so Redis layers on trivially in Phase 2.                                       |
| Game state on crash | Lost (acceptable Phase 1)                           | In-memory state lost on restart. Serializable struct design means Redis-backed persistence is a Phase 2 drop-in upgrade.                                                                                  |

### Authentication & Security

| Decision               | Choice                                | Rationale                                                                                                                                                                                    |
| ---------------------- | ------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Auth strategy          | JWT (access + refresh tokens)         | Stateless, works across HTTP and WebSocket. No session store required.                                                                                                                       |
| Access token storage   | In-memory only (Zustand auth store)   | Never in localStorage — prevents XSS token theft. On page refresh, client calls `/auth/refresh` to get a new access token.                                                                   |
| Access token lifetime  | ~15 minutes                           | Short-lived, limits exposure window.                                                                                                                                                         |
| Refresh token storage  | httpOnly cookie                       | Secure, auto-sent on `/auth/refresh` calls. Not accessible to JavaScript.                                                                                                                    |
| Refresh token lifetime | ~7 days                               | Used to rotate access tokens silently.                                                                                                                                                       |
| Password hashing       | bcrypt                                | Battle-tested, secure, well-supported in Go ecosystem.                                                                                                                                       |
| WebSocket auth         | JWT in first message after connection | Server validates before accepting game commands. On token expiry mid-game: reconnect → call /auth/refresh (cookie auto-sent) → get new access token → authenticate WebSocket with new token. |
| CORS                   | Configured for specific origin(s)     | Only the frontend domain allowed; no wildcard.                                                                                                                                               |
| Transport security     | HTTPS + WSS only                      | Enforced by Caddy with automatic Let's Encrypt certificates.                                                                                                                                 |

### API & Communication Patterns

| Decision              | Choice                                         | Rationale                                                                                                                                                                                           |
| --------------------- | ---------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| HTTP API style        | RESTful JSON                                   | Standard REST for: auth, profiles, rooms, match history, stats, leaderboard.                                                                                                                        |
| Real-time protocol    | WebSocket (coder/websocket)                    | Single multiplexed connection per client. All game state, lobby updates, chat, timer sync, disconnect detection over one pipe — distinguished by event `type`. No separate connections per concern. |
| WS message format     | JSON with typed events + formal contract       | Typed event system with documented TypeScript type file and Go equivalent defined before implementation. Prevents frontend/backend drift.                                                           |
| WS message structure  | `{ "type": "event_name", "payload": { ... } }` | Incremental events during play, full state snapshots on reconnection (enabled by serializable game state struct).                                                                                   |
| Timer synchronization | Absolute server timestamps                     | Server sends "turn expires at Unix timestamp X" — clients render their own countdown. Eliminates drift entirely. Never send relative "start a 30s countdown."                                       |
| Error handling        | Structured error responses                     | HTTP: `{ "error": { "code": "...", "message": "..." } }`. WS: `{ "type": "error", "payload": { "code": "...", "message": "..." } }`                                                                 |
| API documentation     | Swagger/OpenAPI for REST                       | WebSocket event contract documented as TypeScript types + Go types (the formal contract).                                                                                                           |

### Frontend Architecture

| Decision               | Choice                                             | Rationale                                                                                                                                                             |
| ---------------------- | -------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| State management       | Zustand — partitioned stores                       | Separate stores: auth/session, lobby state, game state, chat. Game store wiped on match end; auth store persists. Clean boundaries prevent cross-concern state leaks. |
| Routing                | React Router v7                                    | Standard, well-documented, matches developer experience.                                                                                                              |
| i18n                   | react-i18next                                      | JSON-based translation files, large ecosystem, straightforward setup.                                                                                                 |
| WebSocket client       | Native WebSocket API + custom hook                 | Thin wrapper managing connection lifecycle, reconnection, and event dispatch to appropriate Zustand stores. Single multiplexed connection.                            |
| Component architecture | Feature folders (game/, lobby/, profile/, shared/) | Game components are purely presentational — all state from server via WebSocket, no local game logic.                                                                 |

### Backend Architecture

| Decision            | Choice                                    | Rationale                                                                                                                                                                                                                 |
| ------------------- | ----------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Framework           | Echo v4                                   | Battle-tested, developer's known stack, supported through Dec 2026. v5 migration deferred until v5 has 6+ months of community hardening.                                                                                  |
| Rules engine design | Pure function: state + action → new state | No side effects inside engine. Session manager handles broadcasting, persistence, timers. Makes rules engine trivially testable — feed state, feed action, assert output. Critical for "rule correctness is priority #1." |
| Session manager     | Orchestrator calling rules engine         | Receives player actions via WebSocket, calls rules engine, broadcasts results, manages timers, handles disconnect/reconnect. Side effects live here, not in the rules engine.                                             |
| Game state shape    | Serializable Go struct                    | Clean struct with JSON tags. Enables: reconnection state snapshots, future Redis serialization, test fixture creation. Not raw maps.                                                                                      |

### Infrastructure & Deployment

| Decision             | Choice                                                                    | Rationale                                                                                                           |
| -------------------- | ------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------- |
| Production host      | Contabo VPS                                                               | Self-hosted, cost-effective for passion project. Single server for Phase 1–2.                                       |
| Reverse proxy        | Caddy — on host (not in Docker)                                           | Automatic HTTPS via Let's Encrypt with direct port 80/443 access. No Docker networking complexity for TLS renewal.  |
| Containerization     | Docker (dev + prod)                                                       | Docker Compose for local dev. Production: Go app + PostgreSQL as Docker containers behind host-level Caddy.         |
| Production topology  | Host Caddy → Docker Go container (API + WS) → Docker PostgreSQL container | Caddy handles TLS termination and proxies to Go container. Vite build output served as static files by Go or Caddy. |
| CI/CD                | GitHub Actions                                                            | Automated build + test on push. Deployment starts manual (`make deploy`), automated later.                          |
| Logging              | Go slog (stdlib)                                                          | Structured JSON logging to stdout, captured by Docker logs.                                                         |
| Monitoring (Phase 1) | Health endpoint + UptimeRobot                                             | Basic uptime monitoring. Prometheus + Grafana for Phase 2.                                                          |
| Environment config   | Environment variables                                                     | `.env` in dev (Docker Compose), Docker env vars in production. No secrets in repo.                                  |

### Decision Impact Analysis

**Implementation Sequence:**

1. Project scaffold (monorepo structure, Makefile, Docker Compose, seed script)
2. PostgreSQL schema + golang-migrate setup
3. WebSocket event contract (TypeScript types + Go types — define before building)
4. Auth system (JWT, bcrypt, access/refresh token flow, httpOnly cookies)
5. WebSocket gateway (coder/websocket, single multiplexed connection, auth handshake)
6. Rules engine (Bitola variant, pure function, comprehensive test suite)
7. Game session manager (orchestrates rules engine + WebSocket + absolute timers)
8. Lobby & room management (REST + WebSocket room state broadcasts)
9. Chat (global + match-scoped, over existing multiplexed WebSocket)
10. Frontend SPA (React + partitioned Zustand stores + React Router + game components)
11. Disconnect/reconnect handling (state snapshot from serializable struct)
12. Dockerized production deployment + host-level Caddy

**Cross-Component Dependencies:**

- WebSocket gateway depends on auth (JWT validation in first message)
- Game session manager depends on rules engine (pure function) + WebSocket gateway + timer (absolute timestamps)
- Lobby depends on WebSocket gateway (room state broadcasts) + auth
- Chat rides on the existing multiplexed WebSocket connection
- Frontend game components depend on formal WebSocket event contract
- Disconnect handling depends on serializable game state (snapshot → send to reconnecting client)
- Reconnection auth flow: reconnect → /auth/refresh (httpOnly cookie) → new access token → authenticate WebSocket

## Implementation Patterns & Consistency Rules

### Pattern Categories Defined

**Critical Conflict Points Identified:** 6 categories, 30+ specific areas where AI agents could make different choices — all resolved below.

### Naming Patterns

**Database Naming Conventions (PostgreSQL):**

| Element      | Convention             | Example                      |
| ------------ | ---------------------- | ---------------------------- |
| Tables       | `snake_case`, plural   | `users`, `matches`, `rooms`  |
| Columns      | `snake_case`           | `player_id`, `created_at`    |
| Foreign keys | `{table_singular}_id`  | `user_id`, `match_id`        |
| Indexes      | `idx_{table}_{column}` | `idx_users_email`            |
| Enums        | `snake_case`           | `game_variant`, `match_mode` |

**JSON Wire Format (API + WebSocket):**

`camelCase` for all fields — enforced via Go struct tags:

```go
type User struct {
    ID        uint      `json:"id"`
    Username  string    `json:"username"`
    CreatedAt time.Time `json:"createdAt"`
}
```

**REST Endpoint Naming:**

Plural, kebab-case: `/api/v1/match-history`, `/api/v1/rooms`, `/api/v1/auth/refresh`

**WebSocket Event Naming:**

Four prefixes with `snake_case` names:

| Prefix    | Direction       | Domain                   | Examples                                                                   |
| --------- | --------------- | ------------------------ | -------------------------------------------------------------------------- |
| `action:` | Client → Server | Player game actions      | `action:play_card`, `action:pick_trump`, `action:pause`                    |
| `event:`  | Server → Client | Game state changes       | `event:card_played`, `event:trick_resolved`, `event:game_state`            |
| `error:`  | Server → Client | Error responses          | `error:invalid_action`, `error:not_your_turn`                              |
| `system:` | Server → Client | Non-game platform events | `system:room_updated`, `system:chat_message`, `system:player_joined_lobby` |

**Frontend File Naming:**

| Type        | Convention       | Example                              |
| ----------- | ---------------- | ------------------------------------ |
| Components  | `PascalCase.tsx` | `PlayingCard.tsx`, `ScorePanel.tsx`  |
| Hooks       | `camelCase.ts`   | `useWebSocket.ts`, `useGameStore.ts` |
| Stores      | `camelCase.ts`   | `gameStore.ts`, `authStore.ts`       |
| Utils       | `camelCase.ts`   | `formatScore.ts`                     |
| Types       | `camelCase.ts`   | `gameTypes.ts`, `wsEvents.ts`        |
| API clients | `camelCase.ts`   | `auth.ts`, `rooms.ts`, `matches.ts`  |

**Backend File Naming:**

Go convention: `snake_case.go` — `game_state.go`, `rules_engine.go`, `ws_handler.go`
Test files: `_test.go` suffix co-located — `rules_engine_test.go`

### Structure Patterns

**Backend Domain Package Shape:**

Every domain package follows the same internal structure:

```
internal/{domain}/
├── model.go          # GORM model + domain struct
├── repository.go     # Repository interface
├── gorm_repo.go      # GORM implementation of interface
├── handler.go        # Echo HTTP handlers
├── service.go        # Business logic (if needed beyond handler)
└── {domain}_test.go  # Tests
```

**Centralized Error Types:**

All domain errors in a single package:

```
internal/apperr/
└── errors.go         # All typed application errors
```

```go
var (
    ErrRoomFull    = NewAppError("ROOM_FULL", "room is full", 409)
    ErrNotYourTurn = NewAppError("NOT_YOUR_TURN", "not your turn", 403)
    ErrInvalidCard = NewAppError("INVALID_CARD", "card not playable", 400)
)
```

Every handler checks `errors.Is()`. Every agent adds new errors to this one file.

**Frontend Feature Organization:**

```
client/src/
├── features/
│   ├── auth/           # Login, register, token management
│   ├── game/           # Game table, cards, prompts, score
│   ├── lobby/          # Room list, create room, matchmaking
│   ├── profile/        # Player profile, match history, stats
│   └── chat/           # Global + match chat
├── shared/
│   ├── components/     # Reusable UI (Button, Modal, Toast)
│   ├── hooks/          # Shared hooks (useWebSocket, useAuth)
│   ├── stores/         # Zustand stores (authStore, gameStore, lobbyStore, chatStore)
│   ├── types/          # TypeScript types + WS event contract
│   ├── api/            # HTTP client layer (one file per domain)
│   │   ├── fetchClient.ts  # Shared wrapper (auth headers, error parsing, token refresh)
│   │   ├── auth.ts         # login(), register(), refresh()
│   │   ├── rooms.ts        # getRooms(), createRoom(), joinRoom()
│   │   └── matches.ts      # getMatchHistory(), getMatchDetail()
│   └── i18n/           # Translation JSON files
├── App.tsx
└── main.tsx
```

Components never call `fetch()` directly — all HTTP requests go through `shared/api/`.

**WebSocket Event Contract — Single Source of Truth:**

```
client/src/shared/types/wsEvents.ts    # TypeScript definitions
server/internal/ws/events.go           # Go definitions
```

**Process rule:** When an agent adds a new event, both files are updated in the same commit. No exceptions.

**Game State Struct Field Ordering:**

```go
type GameState struct {
    // Match metadata (ID, variant, mode, room, players)
    // Current hand state (hand number, dealer, trump suit)
    // Current trick state (trick number, cards played, leading suit)
    // Player states (hands held, declarations, connection status)
    // Scoring (team scores, hand points, declarations points)
    // Timer state (active player, turn expiry timestamp)
}
```

New fields added in the correct section. No random ordering.

### Format Patterns

**HTTP API Response Format:**

Success:

```json
{ "data": { ... } }
```

Error:

```json
{ "error": { "code": "ROOM_FULL", "message": "This room is already full" } }
```

HTTP status codes used correctly: 200, 201, 400, 401, 403, 404, 409, 500.

**WebSocket Message Format:**

```json
{ "type": "event:card_played", "payload": { "cardId": "KS", "playerId": 3 } }
```

**Date/Time:** ISO 8601 strings in JSON (`"2026-04-05T14:30:00Z"`). Stored as `timestamptz` in PostgreSQL. Displayed via i18n locale formatting.

**IDs:** Integer auto-increment (GORM default). UUIDs deferred — overkill at Phase 1 scale.

### Communication Patterns

**Zustand Store Pattern:**

Each store follows the same shape:

```typescript
interface GameStore {
  // State
  gameState: GameState | null;
  isLoading: boolean;
  // Actions
  setGameState: (state: GameState) => void;
  clearGameState: () => void;
}
```

Immutable updates only — replace whole objects, no direct mutation.

**State Store Partitioning:**

| Store        | Lifecycle                               | Contents                                |
| ------------ | --------------------------------------- | --------------------------------------- |
| `authStore`  | Persists across app                     | Access token, user profile, login state |
| `lobbyStore` | Active in lobby                         | Room list, filters, matchmaking status  |
| `gameStore`  | Active during match, wiped on match end | Full game state from server             |
| `chatStore`  | Active when connected                   | Global + match chat messages            |

### Process Patterns

**Error Handling — Backend:**

- All domain errors defined in `internal/apperr/errors.go`
- Echo error handler middleware maps domain errors → HTTP status codes / WS error events
- Internal errors logged via slog; user-facing message is generic
- Handlers use `errors.Is()` for control flow

**Error Handling — Frontend:**

- HTTP errors caught in `fetchClient.ts` wrapper → toast for user-facing errors
- WebSocket errors dispatched to relevant Zustand store → UI reacts to store state
- React Error Boundary at app level as safety net

**Loading States:**

- Each Zustand store manages its own `isLoading` boolean
- Skeleton loaders for lists (room list, match history)
- No full-page spinners — local loading indicators only

**Validation:**

- Server is the authority — all input validated server-side
- Frontend validation is cosmetic/UX only (blur validation on forms)
- Never trust client-submitted game actions

**Rules Engine Test Fixtures:**

Test fixture factory functions in `internal/game/testfixtures/`:

```go
func GameStateWithLedSuit(suit Suit) *GameState { ... }
func GameStateAtTrumpBidding(variant Variant) *GameState { ... }
func GameStateWithDeclarations(decls []Declaration) *GameState { ... }
```

All rules engine tests use factory functions — no raw struct literals.

### Enforcement Guidelines

**Automated Enforcement:**

- **Go:** `gofmt` + `golangci-lint` (`.golangci.yml`) — runs in CI, blocks merge on failure
- **TypeScript:** ESLint + Prettier — runs in CI, blocks merge on failure
- **Pre-commit hook:** `make lint` runs both Go and TS linters
- **CI pipeline:** `make test` + `make lint` on every push

**Feature-Complete Checklist (Mandatory Quality Gate):**

Every new feature, every agent, every time:

- [ ] Server handler + repository layer + tests
- [ ] Domain errors added to `internal/apperr/errors.go` (if new error cases)
- [ ] WebSocket events added to _both_ contract files (if real-time feature)
- [ ] Frontend component + co-located test
- [ ] API client function in `shared/api/` (if HTTP endpoint)
- [ ] i18n strings added to all translation files
- [ ] Linter passes (`make lint`)
- [ ] All existing tests pass (`make test`)

**All AI Agents MUST:**

1. Follow naming conventions — no exceptions
2. Use the established domain package shape for backend features
3. Use feature folders for frontend — new features get a new folder under `features/`
4. Use the `action:` / `event:` / `error:` / `system:` prefix convention for all WebSocket messages
5. Place game logic exclusively in `server/internal/game/` — rules engine is pure, no side effects
6. Never store access tokens in localStorage — memory only
7. Use `camelCase` for all JSON wire format fields
8. Co-locate tests with source files
9. Use structured error response format for all HTTP and WebSocket errors
10. Update both WS event contract files in the same commit
11. Use test fixture factories for rules engine tests — no raw struct literals
12. Run `make lint` before committing
13. Complete the feature-complete checklist before marking work done

### Anti-Patterns

| Do NOT                                             | Do Instead                                         |
| -------------------------------------------------- | -------------------------------------------------- |
| Call `fetch()` directly in components              | Use `shared/api/` client functions                 |
| Define errors inline in handlers                   | Add to `internal/apperr/errors.go`                 |
| Put game logic in handlers or WebSocket layer      | Keep in `internal/game/` as pure functions         |
| Store JWT in localStorage                          | Keep access token in memory (Zustand)              |
| Build game state with raw struct literals in tests | Use `testfixtures/` factory functions              |
| Add WS events to only one language's contract file | Update both `wsEvents.ts` and `events.go` together |
| Use `PascalCase` or `snake_case` in JSON           | Always `camelCase` in wire format                  |
| Send relative timer durations ("30 seconds")       | Send absolute timestamps ("expires at X")          |

## Project Structure & Boundaries

### Complete Project Directory Structure

```
beljot/
├── Makefile                          # Build orchestration: dev, build, test, lint, migrate, seed, deploy
├── Caddyfile                         # Caddy reverse proxy config (production TLS + WS proxy)
├── docker-compose.yml                # Local dev: PostgreSQL only (Go runs natively for hot reload)
├── docker-compose.prod.yml           # Production: Go app + PostgreSQL containers
├── .env.example                      # All env vars with placeholder values (checked into git)
├── .gitignore
├── .github/
│   └── workflows/
│       └── ci.yml                    # GitHub Actions: make test + make lint on push
├── scripts/
│   ├── deploy.sh                     # SSH + docker compose pull + restart on VPS
│   └── backup-db.sh                  # PostgreSQL dump from production (safety net)
│
├── client/                           # ── FRONTEND (React SPA) ──
│   ├── package.json
│   ├── vite.config.ts
│   ├── tsconfig.json
│   ├── tsconfig.app.json
│   ├── tsconfig.node.json
│   ├── eslint.config.js
│   ├── .prettierrc
│   ├── index.html
│   ├── public/
│   │   └── favicon.ico
│   ├── e2e/                          # E2E tests (.e2e.ts)
│   │   └── game-flow.e2e.ts
│   └── src/
│       ├── main.tsx                  # App entrypoint, React root
│       ├── App.tsx                   # Router + providers + layout
│       ├── index.css                 # Tailwind CSS v4 imports + design tokens
│       │
│       ├── features/
│       │   ├── auth/
│       │   │   ├── LoginPage.tsx
│       │   │   ├── LoginPage.test.tsx
│       │   │   ├── RegisterPage.tsx
│       │   │   └── RegisterPage.test.tsx
│       │   │
│       │   ├── lobby/
│       │   │   ├── LobbyPage.tsx
│       │   │   ├── LobbyPage.test.tsx
│       │   │   ├── RoomList.tsx
│       │   │   ├── RoomCard.tsx
│       │   │   ├── CreateRoomModal.tsx
│       │   │   ├── CreateRoomModal.test.tsx
│       │   │   ├── RoomLobby.tsx
│       │   │   ├── QuickPlay.tsx
│       │   │   └── RankBanner.tsx
│       │   │
│       │   ├── game/
│       │   │   ├── GamePage.tsx           # Page-level layout, WS lifecycle for game
│       │   │   ├── GamePage.test.tsx
│       │   │   └── components/            # Subfolder: 10+ game-specific components
│       │   │       ├── PlayingCard.tsx
│       │   │       ├── PlayingCard.test.tsx
│       │   │       ├── HandCards.tsx
│       │   │       ├── HandCards.test.tsx
│       │   │       ├── PlayerSeat.tsx
│       │   │       ├── TrickArea.tsx
│       │   │       ├── TrumpPrompt.tsx
│       │   │       ├── DeclarationPrompt.tsx
│       │   │       ├── ScorePanel.tsx
│       │   │       ├── ScorePanel.test.tsx
│       │   │       ├── TimerRing.tsx
│       │   │       ├── ReconnectOverlay.tsx
│       │   │       └── ScoreReveal.tsx
│       │   │
│       │   ├── profile/
│       │   │   ├── ProfilePage.tsx
│       │   │   ├── ProfilePage.test.tsx
│       │   │   └── MatchHistory.tsx
│       │   │
│       │   └── chat/
│       │       ├── ChatPanel.tsx
│       │       ├── ChatPanel.test.tsx
│       │       └── ChatMessage.tsx
│       │
│       └── shared/
│           ├── components/
│           │   ├── ui/                    # shadcn/ui primitives — added via `npx shadcn@latest add`
│           │   │   ├── button.tsx         #   NEVER create manually; always use shadcn CLI
│           │   │   ├── dialog.tsx
│           │   │   ├── input.tsx
│           │   │   ├── tabs.tsx
│           │   │   ├── toast.tsx
│           │   │   ├── tooltip.tsx
│           │   │   ├── dropdown-menu.tsx
│           │   │   ├── progress.tsx
│           │   │   └── badge.tsx
│           │   ├── Toast.tsx              # App-level toast container
│           │   └── ErrorBoundary.tsx
│           │
│           ├── hooks/
│           │   ├── useWebSocket.ts        # WS connection lifecycle + reconnection logic
│           │   ├── useWsAuth.ts           # WS auth handshake + token refresh on reconnect
│           │   ├── useWsDispatch.ts       # Event type → Zustand store routing
│           │   └── useAuth.ts             # HTTP auth: login, logout, token refresh
│           │
│           ├── stores/
│           │   ├── authStore.ts           # Access token, user profile, login state
│           │   ├── lobbyStore.ts          # Room list, filters, matchmaking status
│           │   ├── gameStore.ts           # Game state from server (wiped on match end)
│           │   └── chatStore.ts           # Global + match chat messages
│           │
│           ├── types/
│           │   ├── wsEvents.ts            # ★ WebSocket event contract (TypeScript side)
│           │   ├── gameTypes.ts           # GameState, Card, Player, Score types
│           │   └── apiTypes.ts            # HTTP API request/response types
│           │
│           ├── api/
│           │   ├── fetchClient.ts         # Shared wrapper: auth headers, error parsing, token refresh
│           │   ├── auth.ts                # login(), register(), refresh()
│           │   ├── rooms.ts              # getRooms(), createRoom(), joinRoom()
│           │   ├── matches.ts             # getMatchHistory(), getMatchDetail()
│           │   └── profile.ts             # getProfile(), updateProfile()
│           │
│           ├── utils/
│           │   ├── cn.ts                  # shadcn/ui class merge utility (clsx + tailwind-merge)
│           │   └── formatDate.ts          # i18n-aware date formatting helper
│           │
│           └── i18n/
│               ├── i18n.ts                # react-i18next config
│               ├── en.json                # English translations
│               └── sr.json                # Serbian (Latin) translations
│
└── server/                               # ── BACKEND (Go + Echo) ──
    ├── go.mod
    ├── go.sum
    ├── Dockerfile                         # Multi-stage: build Go binary → minimal production image
    ├── .golangci.yml                      # golangci-lint configuration
    │
    ├── cmd/
    │   └── api/
    │       └── main.go                    # Entrypoint: config load, DB init, Echo server, routes
    │
    ├── internal/
    │   ├── config/
    │   │   └── config.go                  # ★ Centralized env config struct, loaded once at startup
    │   │
    │   ├── apperr/
    │   │   └── errors.go                  # ★ All domain errors (ErrRoomFull, ErrNotYourTurn, etc.)
    │   │
    │   ├── auth/
    │   │   ├── handler.go                 # POST /auth/register, /auth/login, /auth/refresh
    │   │   ├── service.go                 # JWT generation, validation, bcrypt hashing
    │   │   ├── middleware.go              # Echo JWT auth middleware
    │   │   └── auth_test.go
    │   │
    │   ├── user/
    │   │   ├── model.go                   # User GORM model
    │   │   ├── repository.go              # UserRepository interface
    │   │   ├── gorm_repo.go              # GORM implementation
    │   │   ├── handler.go                 # GET /users/:id/profile, /users/:id/matches
    │   │   └── user_test.go
    │   │
    │   ├── game/
    │   │   ├── types.go                   # ★ Card, Suit, Rank, Declaration, Action, Variant types
    │   │   ├── state.go                   # ★ GameState serializable struct (field-ordered)
    │   │   ├── rules_engine.go            # ★ Pure function: ApplyAction(state, action) → (state, error)
    │   │   ├── bidding.go                 # Trump bidding logic per variant
    │   │   ├── declarations.go            # Declaration detection + resolution
    │   │   ├── scoring.go                 # Card points, last trick, Capot, failed hands
    │   │   ├── validation.go              # Legal move checking (suit-following, trump obligations)
    │   │   ├── auto_play.go              # Timer expiry: select first legal card by suit/rank
    │   │   ├── testfixtures/
    │   │   │   └── fixtures.go            # ★ Factory functions for test game states
    │   │   ├── rules_engine_test.go
    │   │   ├── bidding_test.go
    │   │   ├── declarations_test.go
    │   │   ├── scoring_test.go
    │   │   └── validation_test.go
    │   │
    │   ├── session/
    │   │   ├── manager.go                 # ★ Orchestrator: WS actions → rules engine → broadcast
    │   │   ├── timer.go                   # Per-move timer (absolute timestamps), auto-play trigger
    │   │   ├── reconnect.go              # Disconnect detection, reconnect window, state snapshot
    │   │   └── session_test.go
    │   │
    │   ├── lobby/
    │   │   ├── model.go                   # Room GORM model
    │   │   ├── repository.go              # RoomRepository interface
    │   │   ├── gorm_repo.go              # GORM implementation
    │   │   ├── handler.go                 # CRUD /rooms, /rooms/:id/join, /rooms/:id/start
    │   │   ├── matchmaking.go            # Quick Play queue logic
    │   │   └── lobby_test.go
    │   │
    │   ├── chat/
    │   │   ├── handler.go                 # Chat message handling (global + match-scoped)
    │   │   └── chat_test.go
    │   │
    │   ├── ws/
    │   │   ├── hub.go                     # ★ Connection manager: tracks all active WS connections
    │   │   ├── client.go                  # Per-client WS connection wrapper, read/write pumps
    │   │   ├── handler.go                 # WS upgrade endpoint, auth handshake
    │   │   ├── message.go                 # ★ WSMessage wire format struct (Type + RawMessage payload)
    │   │   ├── events.go                  # ★ WebSocket event contract (Go side)
    │   │   ├── router.go                  # Event type → handler dispatch (action:, system:)
    │   │   └── ws_test.go
    │   │
    │   ├── middleware/
    │   │   ├── cors.go                    # CORS config (specific origins from Config)
    │   │   ├── logging.go                 # slog request logging
    │   │   └── error_handler.go          # Echo error handler: AppError → HTTP/WS error response
    │   │
    │   └── db/
    │       ├── database.go                # GORM PostgreSQL connection setup
    │       └── seed.go                    # make seed: test accounts, rooms, game states
    │
    └── migrations/                        # golang-migrate SQL files
        ├── 000001_create_users.up.sql
        ├── 000001_create_users.down.sql
        ├── 000002_create_rooms.up.sql
        ├── 000002_create_rooms.down.sql
        ├── 000003_create_matches.up.sql
        └── 000003_create_matches.down.sql
```

### Architectural Boundaries

**API Boundaries:**

| Boundary                       | Protocol              | Authentication            | Description                                                |
| ------------------------------ | --------------------- | ------------------------- | ---------------------------------------------------------- |
| Client ↔ REST API              | HTTPS                 | JWT Bearer (access token) | Auth, profile, rooms CRUD, match history, stats            |
| Client ↔ WebSocket             | WSS                   | JWT in first message      | Game state, lobby presence, chat, timers, disconnect       |
| Echo Handlers ↔ Repositories   | Go interface          | N/A (internal)            | Handlers call repository interfaces, never GORM directly   |
| Session Manager ↔ Rules Engine | Go function call      | N/A (internal)            | Session manager calls pure functions, handles side effects |
| Middleware ↔ Handlers          | Echo middleware chain | N/A (internal)            | Auth, CORS, logging, error handling wrap all routes        |

**Component Boundary Diagram:**

```
┌─────────────────────────────────────────────────────┐
│                    CLIENT (React SPA)                │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐            │
│  │ features/ │ │ shared/  │ │ shared/  │            │
│  │ game/     │ │ stores/  │ │ api/     │            │
│  │ lobby/    │ │ hooks/   │ │          │            │
│  │ auth/     │ │          │ │          │            │
│  └────┬──┬──┘ └────┬─────┘ └────┬─────┘            │
│       │  │         │            │                    │
│       │  └─────────┘            │                    │
│       │   (Zustand stores)      │                    │
└───────┼─────────────────────────┼────────────────────┘
        │ WebSocket               │ HTTPS
        │ (single multiplexed)    │ (REST JSON)
┌───────┼─────────────────────────┼────────────────────┐
│       │         SERVER (Go)     │                     │
│  ┌────▼─────┐             ┌─────▼──────┐             │
│  │ ws/hub   │             │ auth/      │             │
│  │ ws/router│             │ user/      │             │
│  └────┬─────┘             │ lobby/     │             │
│       │                   └─────┬──────┘             │
│  ┌────▼──────────┐              │                    │
│  │ session/      │              │                    │
│  │ manager       │◄─────────────┘                    │
│  └────┬──────────┘                                   │
│       │ (pure function call)                         │
│  ┌────▼──────────┐                                   │
│  │ game/         │                                   │
│  │ rules_engine  │  ← NO side effects               │
│  └───────────────┘                                   │
│                                                      │
│  ┌───────────────┐                                   │
│  │ db/ + GORM    │◄── All repositories               │
│  └───────┬───────┘                                   │
└──────────┼───────────────────────────────────────────┘
           │
    ┌──────▼──────┐
    │ PostgreSQL  │
    └─────────────┘
```

**Data Boundaries:**

| Layer                  | Access Pattern                  | Who Calls It                                                         |
| ---------------------- | ------------------------------- | -------------------------------------------------------------------- |
| GORM models            | Repository implementations only | Never called from handlers directly                                  |
| Repository interfaces  | Handlers and services           | Defined per domain package                                           |
| Game state (in-memory) | Session manager only            | WS hub routes actions to session manager                             |
| PostgreSQL             | GORM via repository layer       | Accounts, rooms, completed matches, stats                            |
| Config                 | `config.Config` struct          | Loaded once in `main.go`, passed via DI — no `os.Getenv()` elsewhere |

### Requirements to Structure Mapping

**FR Category → Directory Mapping:**

| FR Category                   | Backend Location                    | Frontend Location                                |
| ----------------------------- | ----------------------------------- | ------------------------------------------------ |
| User Account (FR1–FR6)        | `internal/auth/`, `internal/user/`  | `features/auth/`, `shared/api/auth.ts`           |
| Game Rules Engine (FR7–FR15)  | `internal/game/`                    | N/A (server-only)                                |
| Lobby & Rooms (FR16–FR22)     | `internal/lobby/`                   | `features/lobby/`, `shared/api/rooms.ts`         |
| Real-Time Session (FR23–FR29) | `internal/session/`, `internal/ws/` | `features/game/`, `shared/hooks/useWebSocket.ts` |
| Communication (FR30–FR32)     | `internal/chat/`                    | `features/chat/`                                 |
| Progression (FR33–FR40)       | `internal/user/` (extended Phase 2) | `features/lobby/RankBanner.tsx` (Phase 2)        |
| Stats & History (FR41–FR43)   | `internal/user/handler.go`          | `features/profile/`, `shared/api/matches.ts`     |
| i18n (FR44–FR46)              | N/A (frontend-only)                 | `shared/i18n/`                                   |

**Cross-Cutting Concerns → Location:**

| Concern             | Backend                                                    | Frontend                                                           |
| ------------------- | ---------------------------------------------------------- | ------------------------------------------------------------------ |
| Configuration       | `internal/config/config.go`                                | `.env.example` (reference)                                         |
| Authentication      | `internal/auth/`, `internal/middleware/`                   | `shared/hooks/useAuth.ts`, `shared/stores/authStore.ts`            |
| WebSocket lifecycle | `internal/ws/`                                             | `shared/hooks/useWebSocket.ts`, `useWsAuth.ts`, `useWsDispatch.ts` |
| Error handling      | `internal/apperr/`, `internal/middleware/error_handler.go` | `shared/api/fetchClient.ts`, `shared/components/ErrorBoundary.tsx` |
| i18n                | N/A                                                        | `shared/i18n/`                                                     |
| Logging             | `internal/middleware/logging.go` (slog)                    | Browser console (dev only)                                         |

### Integration Points

**Internal Communication:**

| From            | To                    | Mechanism                                         |
| --------------- | --------------------- | ------------------------------------------------- |
| WS Hub          | Session Manager       | Go channel or direct method call per game room    |
| Session Manager | Rules Engine          | Pure function call: `ApplyAction(state, action)`  |
| Session Manager | WS Hub                | Broadcast game state to room participants         |
| Session Manager | Timer                 | Start/cancel per-move timer via goroutine         |
| Lobby Handler   | WS Hub                | Broadcast room state changes to lobby subscribers |
| Chat Handler    | WS Hub                | Broadcast chat messages to room or global channel |
| All Handlers    | Repository interfaces | Data persistence via GORM implementations         |
| All packages    | Config                | `config.Config` struct injected at startup        |

**Data Flow — Card Play Action:**

```
Client clicks card
  → WS message: { "type": "action:play_card", "payload": { "cardId": "KS" } }
  → ws/hub.go receives → ws/router.go dispatches to session/manager.go
  → manager validates player turn + calls game/rules_engine.go ApplyAction()
  → rules engine returns new GameState (or error)
  → manager broadcasts: { "type": "event:card_played", "payload": { ... } } to all 4 clients
  → manager checks trick completion → if 4th card: resolve trick, update scores
  → manager resets per-move timer for next player (absolute timestamp)
  → clients receive event → useWsDispatch.ts → gameStore updates → React re-renders
```

### Development Workflow Integration

**`make dev`** — runs concurrently:

- `docker compose up -d postgres` (PostgreSQL only, port 5432)
- `cd client && npm run dev` (Vite dev server, port 5173)
- `cd server && go run cmd/api/main.go` (Echo server, port 8080)

**`make build`** — production build:

- `cd client && npm run build` → `client/dist/` (static files)
- `cd server && docker build -t beljot-server .` (multi-stage Dockerfile → minimal image)

**`make test`** — both stacks:

- `cd client && npx vitest run`
- `cd server && go test ./...`

**`make lint`** — both stacks:

- `cd client && npx eslint . && npx prettier --check .`
- `cd server && golangci-lint run`

**`make migrate`** — `golang-migrate -path server/migrations -database $DB_URL up`

**`make seed`** — `cd server && go run cmd/api/main.go -seed`

**`make deploy`** — `./scripts/deploy.sh` (build, push images, SSH restart on VPS)

**`make backup`** — `./scripts/backup-db.sh` (PostgreSQL dump from production)

**Environment Variable Convention:**

| Context    | Source                                       | DB_URL Example                                                    |
| ---------- | -------------------------------------------- | ----------------------------------------------------------------- |
| Dev        | `.env` loaded by Docker Compose + `go run`   | `postgres://beljot:dev@localhost:5432/beljot?sslmode=disable`     |
| Production | Docker env vars in `docker-compose.prod.yml` | `postgres://beljot:$PROD_PW@postgres:5432/beljot?sslmode=disable` |
| CI         | GitHub Actions secrets                       | `postgres://test:test@localhost:5432/beljot_test?sslmode=disable` |
| Migrate    | Same `.env` or `$DB_URL` env var             | (matches current context)                                         |

## Architecture Validation Results

### Coherence Validation

**Decision Compatibility:** All technology choices verified compatible. No version conflicts. Go + Echo v4 + GORM + coder/websocket form a stable backend stack. React 19 + Vite 8 + Tailwind v4 + shadcn/ui form a stable frontend stack. JWT auth works across both HTTP and WebSocket channels.

**Pattern Consistency:** Naming conventions (snake_case DB → camelCase JSON → PascalCase Go → camelCase TS) form a clear, non-contradictory conversion chain. WebSocket event prefixes (action/event/error/system) are consistent across both contract files. Repository pattern applied uniformly across all backend domain packages.

**Structure Alignment:** Project structure directly maps to all architectural decisions. Config package supports DI. apperr package supports centralized error handling. game/ package enforces pure function engine. ws/ package supports multiplexed connection. All patterns have explicit directory homes.

### Requirements Coverage

**Functional Requirements:** 25 Phase 1 FRs fully covered with specific file-to-FR mapping. 27 deferred FRs (Phase 2–4) have clear architectural extension paths. 0 coverage gaps.

**Non-Functional Requirements:** 17/17 NFRs addressed with specific architectural mechanisms. Performance targets supported by technology choices. Security enforced by server-authoritative architecture + JWT + Caddy TLS. Scalability via serializable state with Redis upgrade path.

**Phase 1 Validated User Journeys:**

| Journey                         | Phase 1 Status | Notes                                                                                        |
| ------------------------------- | -------------- | -------------------------------------------------------------------------------------------- |
| Journey 1 — Ana (Casual)        | Phase 1        | Valid if using Bitola variant + 1001 mode (PRD bug: journey says 501, but 501 is Phase 2)    |
| Journey 2 — Marko (Competitive) | Phase 2        | Requires ranked system, XP/levels, Croatian variant                                          |
| Journey 3 — Ivan (Diaspora)     | Phase 1        | Bitola variant, room code join, all Phase 1 features                                         |
| Journey 4 — Darko (Room Owner)  | Phase 1        | Valid if using Bitola variant only (PRD bug: journey says Croatian, but Croatian is Phase 2) |
| Journey 5 — Edge Cases          | Phase 1        | Disconnect/reconnect, timer expiry — all Phase 1                                             |

Agents must not implement Croatian variant or 501 mode in Phase 1 — even if referenced in user journeys. The PRD validation report flagged these contradictions.

**Phase 2 Requirements Pending Specification:**

These Phase 2 FRs have undefined formulas in the PRD. Do not invent values — they require product decisions:

- **FR28:** "Appropriate XP/ELO outcomes" on match abandonment — outcomes undefined
- **FR38:** "Scaled ELO penalties by game progress" — scaling formula not specified (brief suggests x0.5 early to x2.0 late but PRD doesn't include it)
- **FR43:** "Partial XP based on game progress" — no formula or tier table defined

### Game Domain Specifications

#### Game State Machine Phases

The rules engine must enforce phase-based action validation. This is the contract between the session manager and the rules engine:

| Phase             | Valid Player Actions                     | Transitions To                                        |
| ----------------- | ---------------------------------------- | ----------------------------------------------------- |
| `dealing`         | (automatic — no player actions)          | `bidding`                                             |
| `bidding`         | `pick_trump`, `pass_trump`               | `playing` (if picked) or `dealing` (Bitola reshuffle) |
| `playing`         | `play_card`, `declare`, `skip_declare`   | `trick_resolving` (4th card played)                   |
| `trick_resolving` | (automatic — score, sweep)               | `playing` (next trick) or `hand_scoring` (8th trick)  |
| `hand_scoring`    | (automatic — calculate, check match end) | `dealing` (next hand) or `match_end`                  |
| `match_end`       | (none — persist results to DB)           | —                                                     |
| `paused`          | `unpause`, `owner_unpause`               | (return to previous phase)                            |
| `disconnected`    | `reconnect`                              | (return to previous phase)                            |

The `GameState.Phase` field must always reflect the current phase. `ApplyAction()` rejects any action not valid for the current phase.

#### Card Encoding Convention

Two-character format: `{Rank}{Suit}` — used in WebSocket payloads, GameState struct, test fixtures, and frontend rendering.

**Ranks:**

| Card  | Code |
| ----- | ---- |
| 7     | `7`  |
| 8     | `8`  |
| 9     | `9`  |
| 10    | `T`  |
| Jack  | `J`  |
| Queen | `Q`  |
| King  | `K`  |
| Ace   | `A`  |

**Suits:**

| Suit     | Code |
| -------- | ---- |
| Spades   | `S`  |
| Hearts   | `H`  |
| Diamonds | `D`  |
| Clubs    | `C`  |

Examples: `KS` = King of Spades, `TH` = 10 of Hearts, `7D` = 7 of Diamonds, `AC` = Ace of Clubs.

Full deck (32 cards): `7S 8S 9S TS JS QS KS AS 7H 8H 9H TH JH QH KH AH 7D 8D 9D TD JD QD KD AD 7C 8C 9C TC JC QC KC AC`

#### Player Seat Mapping

**Server canonical:** `seat 0, 1, 2, 3` — counter-clockwise from dealer.

| Seat | Team | Play Order |
| ---- | ---- | ---------- |
| 0    | A    | 1st        |
| 1    | B    | 2nd        |
| 2    | A    | 3rd        |
| 3    | B    | 4th        |

- Partners: 0+2 (Team A) vs 1+3 (Team B) — partners face each other
- Play order: `0 → 1 → 2 → 3 → 0` (counter-clockwise)
- Dealer rotates: `0 → 1 → 2 → 3 → 0` each hand

**Client mapping:** The client receiving the game state maps `myIndex → South (bottom)`, then assigns West, North, East counter-clockwise from there. The server never sends screen positions — only seat indices.

#### Game Phase Error Specification

Negative test cases — every entry becomes a rules engine test:

| Phase          | Invalid Action                           | Expected Error          | Test Priority |
| -------------- | ---------------------------------------- | ----------------------- | ------------- |
| `bidding`      | `play_card`                              | `ErrWrongPhase`         | High          |
| `playing`      | `pick_trump`                             | `ErrWrongPhase`         | High          |
| `playing`      | Card not in hand                         | `ErrInvalidCard`        | Critical      |
| `playing`      | Card violating suit-following            | `ErrIllegalPlay`        | Critical      |
| `playing`      | Action from non-active player            | `ErrNotYourTurn`        | Critical      |
| `playing`      | Declare after first trick                | `ErrWrongPhase`         | High          |
| `bidding`      | Pick trump when not active bidder        | `ErrNotYourTurn`        | High          |
| `paused`       | Any game action                          | `ErrGamePaused`         | Medium        |
| `disconnected` | Any game action from disconnected player | `ErrPlayerDisconnected` | Medium        |
| `match_end`    | Any game action                          | `ErrWrongPhase`         | Low           |

### Testing & Quality Gates

#### Test Coverage Targets

| Component           | Coverage Target | Rationale                             | CI Enforcement             |
| ------------------- | --------------- | ------------------------------------- | -------------------------- |
| `internal/game/`    | >90%            | Rule correctness is priority #1 (PRD) | Fail build below threshold |
| `internal/auth/`    | >80%            | Security-critical path                | Fail build below threshold |
| `internal/session/` | >70%            | Complex orchestration                 | Warn below threshold       |
| `internal/lobby/`   | >60%            | Mostly CRUD, lower risk               | Warn below threshold       |
| `internal/ws/`      | >60%            | Connection lifecycle                  | Warn below threshold       |
| Frontend components | >50%            | UI testing diminishing returns        | Informational              |

#### Minimum Test Fixture Set

The `internal/game/testfixtures/fixtures.go` must include factory functions for at minimum these 7 critical game states:

| Fixture                     | Factory Function                             | Covers                             |
| --------------------------- | -------------------------------------------- | ---------------------------------- |
| Fresh deal                  | `NewGameJustDealt()`                         | Bidding tests                      |
| Mid-bidding                 | `NewGameMidBidding(passCount int)`           | Bidding logic, Bitola reshuffle    |
| Trump selected, first trick | `NewGameFirstTrick(trump Suit)`              | Declaration tests                  |
| Mid-game                    | `NewGameMidPlay(trickNum int)`               | Card play, suit-following, scoring |
| Last trick                  | `NewGameLastTrick()`                         | Last trick bonus, hand scoring     |
| Near match end              | `NewGameNearEnd(teamAScore, teamBScore int)` | Match completion, 1001 threshold   |
| Capot in progress           | `NewGameCapotInProgress()`                   | Capot detection and scoring        |

#### Deployment Smoke Test

Before any release to the VPS, verify:

- [ ] Can register a new account
- [ ] Can log in and receive JWT tokens
- [ ] Can create a room
- [ ] Can join a room (4 players via WebSocket)
- [ ] Can complete one full hand (deal → bid → play 8 tricks → score)
- [ ] WebSocket reconnection works (disconnect + reconnect within window)

If any check fails, do not deploy. Manual initially, automated via E2E tests in Phase 2.

### Additional Enforcement Rules

- **Never call GORM `AutoMigrate()`** — all schema changes via golang-migrate SQL files only
- **WebSocket health check:** Server sends ping every 30 seconds (coder/websocket native). Missed pong triggers disconnect detection in `session/reconnect.go`
- **Rules reference:** Located in `features/rules/` with static markdown content rendered as React component. Accessible from lobby nav and in-game overlay

### Architecture Completeness Checklist

**Requirements Analysis**

- [x] Project context thoroughly analyzed (52 FRs, 13 NFRs, 5 user journeys)
- [x] Scale and complexity assessed (medium-high, 10→50 concurrent games)
- [x] Technical constraints identified (server-authoritative, React SPA, WebSocket, desktop-only)
- [x] Cross-cutting concerns mapped (auth, WS lifecycle, game state, timers, i18n, errors, phased delivery)

**Architectural Decisions**

- [x] Critical decisions documented with versions (Echo v4, Vite 8, React 19, PostgreSQL, coder/websocket)
- [x] Technology stack fully specified
- [x] Integration patterns defined (REST + multiplexed WS, repository pattern, pure function engine)
- [x] Performance considerations addressed (absolute timestamps, serializable state, minimal render chain)
- [x] Security architecture defined (JWT, bcrypt, server-authoritative, HTTPS/WSS)

**Implementation Patterns**

- [x] Naming conventions established (DB, JSON, REST, WS events, files, card encoding)
- [x] Structure patterns defined (domain packages, feature folders, co-located tests)
- [x] Communication patterns specified (WS event contract, Zustand stores, API client layer)
- [x] Process patterns documented (error handling, loading states, validation, test fixtures)
- [x] Enforcement guidelines defined (linting, CI, coverage targets, feature-complete checklist, smoke test)

**Project Structure**

- [x] Complete directory structure defined (~100 files)
- [x] Component boundaries established (API, WS, data, service boundaries)
- [x] Integration points mapped (data flow diagram)
- [x] Requirements to structure mapping complete
- [x] Development workflow integration defined (Makefile targets)

**Game Domain**

- [x] State machine phases and valid actions defined
- [x] Card encoding convention standardized
- [x] Player seat mapping canonicalized
- [x] Phase error specification documented (negative test cases)
- [x] Test fixture minimum set defined (7 critical states)

### Architecture Readiness Assessment

**Overall Status:** READY FOR IMPLEMENTATION

**Confidence Level:** High — all Phase 1 FRs covered, all NFRs addressed, no critical gaps, comprehensive patterns, game domain specifications, and quality gates defined.

**Key Strengths:**

- Pure function rules engine with explicit state machine phases — trivially testable
- Serializable game state with canonical card encoding and seat mapping — no ambiguity
- Formal WebSocket event contract with 4-prefix convention — prevents drift
- Comprehensive enforcement: linting, CI, coverage targets, feature-complete checklist, smoke test
- Right-sized for solo developer with clear Phase 2 upgrade paths
- Game domain fully specified: phases, card encoding, seat mapping, error cases, test fixtures

**Areas for Future Enhancement:**

- Redis caching/state layer (Phase 2 scale)
- Echo v5 migration (after 6+ months community hardening)
- OpenAPI spec generation from Echo handlers
- Automated E2E smoke tests (replacing manual checklist)
- Prometheus + Grafana monitoring (Phase 2)
- ELO/XP formulas (Phase 2, pending product specification)

### Implementation Handoff

**AI Agent Guidelines:**

- Follow all architectural decisions exactly as documented
- Use implementation patterns consistently across all components
- Respect project structure and boundaries
- Enforce game state machine phases — reject actions invalid for current phase
- Use the 2-character card encoding convention everywhere
- Use seat indices 0-3 server-side, map to compass positions client-side only
- Complete the feature-complete checklist for every feature
- Never deviate from the WebSocket event contract without updating both files
- Never call GORM AutoMigrate
- Rules engine tests must use fixture factory functions

**First Implementation Priority:**

1. Project scaffold: Vite + Go module + Makefile + Docker Compose + .env.example
2. PostgreSQL schema: users table via golang-migrate
3. WebSocket event contract: define types in both `wsEvents.ts` and `events.go`
4. Auth system: JWT, bcrypt, register/login/refresh endpoints
5. WebSocket gateway: coder/websocket, multiplexed connection, auth handshake
6. Rules engine: Bitola variant, pure function, state machine phases, test suite (>90% coverage)
