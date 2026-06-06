---
project_name: "beljot"
user_name: "Emilijan"
date: "2026-04-06"
sections_completed:
  [
    "technology_stack",
    "language_rules",
    "framework_rules",
    "testing_rules",
    "code_quality",
    "workflow_rules",
    "critical_rules",
  ]
status: "complete"
rule_count: 95
optimized_for_llm: true
---

# Project Context for AI Agents

_This file contains critical rules and patterns that AI agents must follow when implementing code in this project. Focus on unobvious details that agents might otherwise miss._

---

## Technology Stack & Versions

### Frontend

- **TypeScript** (strict mode), **React 19**, compiled via **SWC**
- **Vite 6** — dev server + production bundler
- **Tailwind CSS v4** via `@tailwindcss/vite` plugin — CSS-first configuration only. **No `tailwind.config.js`** — all theming via `@theme` directives in `index.css`
- **shadcn/ui** — components are owned copies in `src/shared/components/ui/`. Install via `npx shadcn@latest add <component>`, never `npm install`. Never create UI primitive files manually
- **Zustand** — 5 partitioned stores: `authStore`, `lobbyStore`, `gameStore`, `chatStore`, `roomLobbyStore`
- **React Router v7**
- **react-i18next** — JSON translation files, English + Serbian (Latin)
- **Vitest** — Vite-native testing (Jest-compatible API)
- **Node 18+** required (React 19 + Vite 6 dependency)

### Backend

- **Go** (latest stable) — pin exact version in `go.mod`
- **Echo v4** — Do NOT upgrade to v5. v5 migration deferred until community hardening (Dec 2026+)
- **GORM** + **PostgreSQL** — uses GORM default naming strategy: `snake_case` columns, auto-pluralized table names, magic `CreatedAt`/`UpdatedAt`/`DeletedAt` fields. Do not override these conventions
- **nhooyr.io/websocket** — import path is `nhooyr.io/websocket`, repo lives at `github.com/coder/websocket` (rebrand). Do not confuse with gorilla/websocket
- **golang-migrate** — CLI-only, invoked via Makefile (`make migrate`). Not embedded as a Go library in application code
- **testify** — assertion library for Go tests
- **slog** (stdlib) — structured JSON logging to stdout
- **bcrypt** — password hashing

### Infrastructure

- **Docker Compose** (dev) — PostgreSQL only. Go server and Vite dev server run natively on host for hot reload
- **Docker** (prod) — Go app + PostgreSQL as containers behind host-level Caddy
- **Caddy** — on host (not in Docker), automatic HTTPS via Let's Encrypt
- **GitHub Actions CI** — `make test` + `make lint` on every push. CI needs: Go (pinned), Node (pinned), PostgreSQL, `golangci-lint`
- **Contabo VPS** — single server for Phase 1-2

### Build Orchestration

- **Makefile** — unified commands: `dev`, `build`, `test`, `lint`, `migrate`, `seed`, `deploy`, `backup`

## Critical Implementation Rules

### Language-Specific Rules

**TypeScript:**

- Strict mode enforced — no `any` types, no implicit `any`
- JSON wire format is always `camelCase` (matches Go struct JSON tags)
- No shared type generation between frontend/backend — manual sync via the WebSocket event contract files (`wsEvents.ts` + `events.go`)
- Immutable state updates only in Zustand stores — replace whole objects, never mutate directly
- Use union literal types (`type Suit = 'hearts' | 'diamonds' | ...`), not TypeScript `enum` — mirrors Go string constants without drift
- Never use JS truthiness checks on numeric or boolean fields from the Go backend — Go zero values (`0`, `""`, `false`) serialize as real values, not `null`. Always use explicit comparisons (`=== 0`, `=== false`)
- All incoming WebSocket messages must be parsed and validated through a typed dispatch function — never use raw `JSON.parse` results (`any`) directly in component or store code

**Go:**

- All JSON struct tags use `camelCase` (e.g., `json:"createdAt"`)
- All struct fields intended for JSON must be exported (PascalCase) with explicit `json:"camelCase"` tags — unexported fields silently vanish from JSON output
- Game rules engine is a **pure function**: `ApplyAction(state, action) -> (state, error)` — zero side effects inside the engine
- Rules engine must **clone slices before mutation** — use `slices.Clone()` or `copy()` when manipulating cards, players, or declarations within `ApplyAction`. Go slices share underlying arrays; modifying without copying violates the pure function contract
- GameState is a **serializable Go struct** with ordered field sections (metadata, hand state, trick state, player states, scoring, timer) — new fields go in the correct section
- Config loaded once at startup into `config.Config` struct, injected via DI — no `os.Getenv()` calls anywhere else
- Structured logging via `slog` — internal errors logged, user-facing messages are generic
- Error control flow uses `errors.Is()` against centralized `apperr` errors — always wrap errors with `%w` verb (`fmt.Errorf("...: %w", err)`), never `%v`, to preserve the error chain
- Use `*time.Time` (pointer) for optional timestamp fields — Go's `time.Time` zero value serializes as `"0001-01-01T00:00:00Z"`, not `null`
- GORM soft deletes (`DeletedAt`) are automatic — all GORM queries exclude soft-deleted rows. If raw SQL is unavoidable, always filter on `deleted_at IS NULL`

**Cross-Language:**

- WebSocket event contract defined in both `wsEvents.ts` and `events.go` — both files updated in the same commit, no exceptions
- Date/time: ISO 8601 strings in JSON, `timestamptz` in PostgreSQL, locale-formatted for display via i18n
- IDs: integer auto-increment (GORM default) — no UUIDs in Phase 1

### Framework-Specific Rules

**React / Frontend:**

- Components are **purely presentational** for game features — all game state from server via WebSocket, no local game logic
- Feature folder organization: `features/auth/`, `features/game/`, `features/lobby/`, `features/profile/`, `features/chat/` — new features get their own folder
- Components never call `fetch()` directly — all HTTP requests through `shared/api/` client functions via `fetchClient.ts` (handles auth headers, error parsing, token refresh)
- `fetchClient.ts` owns the entire 401 → refresh → retry cycle — individual API functions never handle 401 themselves. Only if refresh fails does it redirect to login
- Zustand store partitioning by lifecycle: `authStore` persists across app, `gameStore` wiped on match end, `lobbyStore` active in lobby, `chatStore` active when connected, `roomLobbyStore` active in room lobby (real-time seat/player updates via WS)
- `gameStore` is wiped on **navigation away from the game page**, not on receiving `game_over` event — final game state must remain available for score reveal and post-game display
- Each store manages its own `isLoading` boolean — skeleton loaders for lists, no full-page spinners
- React Error Boundary at app level as safety net
- Frontend validation is cosmetic/UX only (blur validation on forms) — server is the authority
- Style shadcn/ui components via Tailwind classes and CSS variables in consuming components — modify `shared/components/ui/` source files only for project-wide design token changes, never for feature-specific styling

**Echo / Backend:**

- Every domain package follows the same shape: `model.go`, `repository.go`, `gorm_repo.go`, `handler.go`, `service.go` (if needed), `{domain}_test.go`
- Handlers call repository **interfaces**, never GORM directly
- All domain errors defined in `internal/apperr/errors.go` — Echo error handler middleware maps them to HTTP status codes / WS error events
- Middleware registration order in `main.go` is load-bearing: **CORS → Logging → Error Handler → Auth**. New middleware must be inserted at the correct position
- Single multiplexed WebSocket connection per client — game state, lobby updates, chat, timers, disconnect detection all on one pipe, distinguished by event `type`
- WebSocket event prefixes: `action:` (client→server), `event:` (server→client game state), `error:` (server→client errors), `system:` (server→client platform events)
- `ws/router.go` performs **type-based dispatch only** — zero game logic, zero validation. Even "is it this player's turn?" belongs in session manager
- Session manager is the orchestrator — receives player actions, calls rules engine, broadcasts results, manages timers. Side effects live here, not in the rules engine
- Multi-event sequences (card played → trick resolved → score update) must be sent as **separate ordered messages**, not batched into a single payload — frontend animations depend on ordering
- On WebSocket reconnection, after auth handshake succeeds, the server must **restore the client's room/lobby subscriptions** from session state before sending the game state snapshot
- Timer synchronization via absolute server timestamps — server sends "turn expires at Unix timestamp X", clients render countdown. Never send relative durations
- WebSocket handler tests must use `httptest.Server` with a real WebSocket client connection, not mocked read/write interfaces

**Authentication Flow:**

- JWT access token stored in memory only (Zustand `authStore`) — never in `localStorage`
- Refresh token in `httpOnly` cookie — auto-sent on `/auth/refresh` calls
- WebSocket auth: JWT sent in first message after connection. On token expiry mid-game: reconnect → `/auth/refresh` (cookie auto-sent) → new access token → authenticate WebSocket

### Testing Rules

**Backend Testing (Go):**

- Go standard `testing` package + `testify` for assertions
- Tests co-located with source files: `rules_engine_test.go` next to `rules_engine.go`
- Rules engine tests use **factory functions** from `internal/game/testfixtures/` exclusively — no raw `GameState{}` struct literals, even if they compile. When `GameState` gains new fields, factories are the single update point. If no existing factory fits, create a new one
- Rules engine tests must use **Go table-driven test pattern** (`[]struct{ name string; ... }` with `t.Run`) — each test function covers one rule category with multiple cases per variant
- Test the rules engine through **`ApplyAction` only** — internal functions (`bidding.go`, `declarations.go`, `scoring.go`, `validation.go`) are tested indirectly through their effect on `ApplyAction` output. This preserves refactoring freedom
- Session manager tests must include **reconnection scenarios**: disconnect at each game phase (bidding, playing, between tricks, between hands). Verify state snapshot sent to reconnecting client matches current server state exactly
- Integration tests that touch PostgreSQL must use a **per-test transaction with rollback** — never rely on test execution order or shared test data between test functions
- Tests create all their own data — never reference or depend on seed data from `make seed`. Seed data is for manual development only

**Frontend Testing (Vitest):**

- Vitest — Vite-native, Jest-compatible API
- Tests co-located with components: `LoginPage.test.tsx` next to `LoginPage.tsx`
- Game components are presentational — test that they render correctly given server state, not game logic
- Use `data-testid` attributes for element selection, never CSS class names or DOM structure (classes change with Tailwind tweaks)
- Test descriptions use present tense: `it('renders error when login fails')`, not `it('should render error...')`

**Quality Gates:**

- `make lint` + `make test` in CI on every push — blocks merge on failure
- Pre-commit hook runs `make lint`
- `make test` runs both stacks: `go test ./...` + `npx vitest run`

### Code Quality & Style Rules

**Naming Conventions:**

| Context             | Convention                        | Examples                                               |
| ------------------- | --------------------------------- | ------------------------------------------------------ |
| Database tables     | `snake_case`, plural              | `users`, `matches`, `rooms`                            |
| Database columns    | `snake_case`                      | `player_id`, `created_at`                              |
| Foreign keys        | `{table_singular}_id`             | `user_id`, `match_id`                                  |
| Indexes             | `idx_{table}_{column}`            | `idx_users_email`                                      |
| JSON wire format    | `camelCase`                       | `createdAt`, `playerId`                                |
| REST endpoints      | plural, kebab-case                | `/api/v1/match-history`, `/api/v1/rooms`               |
| WS events           | prefixed `snake_case`             | `action:play_card`, `event:trick_resolved`             |
| Frontend components | `PascalCase.tsx`                  | `PlayingCard.tsx`, `ScorePanel.tsx`                    |
| Frontend hooks      | `camelCase.ts`                    | `useWebSocket.ts`, `useAuth.ts`                        |
| Frontend stores     | `camelCase.ts`                    | `gameStore.ts`, `authStore.ts`                         |
| Frontend types      | `camelCase.ts`                    | `gameTypes.ts`, `wsEvents.ts`                          |
| Backend files       | `snake_case.go`                   | `rules_engine.go`, `ws_handler.go`                     |
| Backend tests       | `_test.go` suffix                 | `rules_engine_test.go`                                 |
| i18n keys           | `{feature}.{component}.{element}` | `auth.login.emailLabel`, `game.trumpPrompt.pickButton` |

**GORM Struct Tag Bridge (three conventions in one field):**

```go
type Match struct {
    ID        uint      `gorm:"primaryKey" json:"id"`
    RoomID    uint      `gorm:"column:room_id" json:"roomId"`    // DB: room_id, JSON: roomId
    CreatedAt time.Time `json:"createdAt"`                        // DB: created_at (GORM auto)
}
```

**Import Ordering:**

- TypeScript: (1) React/framework → (2) third-party libraries → (3) `shared/` imports → (4) relative imports. Blank line between each group. Enforced by ESLint import plugin
- Go: (1) standard library → (2) third-party packages → (3) internal project packages. Blank line between groups. Enforced by `goimports`

**Export Convention:**

- Named exports only in TypeScript — no `export default`. Component filename must match the exported component name exactly (`PlayingCard.tsx` exports `PlayingCard`)

**API Client Mapping:**

- Frontend API client files in `shared/api/` map 1:1 to backend domain packages. New backend domain = new API client file. Use the REST resource name for the file

**Linting & Formatting:**

- Go: `gofmt` + `golangci-lint` (`.golangci.yml`) — runs in CI, blocks merge on failure
- TypeScript: ESLint + Prettier — runs in CI, blocks merge on failure
- Pre-commit hook: `make lint` runs both Go and TS linters

**Feature-Complete Checklist (Definition of Done):**

Every feature must pass all items before it is considered complete — this is a hard gate, not a suggestion:

- [ ] Server handler + repository layer + tests
- [ ] Domain errors added to `internal/apperr/errors.go` (if new error cases)
- [ ] WebSocket events added to **both** contract files (if real-time feature)
- [ ] Frontend component + co-located test
- [ ] API client function in `shared/api/` (if HTTP endpoint)
- [ ] i18n strings added to **all** translation files
- [ ] Linter passes (`make lint`)
- [ ] All existing tests pass (`make test`)

### Development Workflow Rules

**Build Commands:**

- `make dev` — runs concurrently: Docker Compose PostgreSQL (port 5432) + Vite dev server (port 5173) + Go Echo server (port 8080). If startup fails, check for orphaned processes on ports 5173/8080 first
- `make build` — production: `npm run build` → `client/dist/` + `docker build` Go binary
- `make test` — both stacks: `npx vitest run` + `go test ./...`
- `make lint` — both stacks: ESLint + Prettier + `golangci-lint`
- `make migrate` — `golang-migrate` CLI against `$DB_URL`
- `make seed` — populates dev database with test accounts, rooms, and game states (development only, not for tests)
- `make deploy` — `scripts/deploy.sh` (build, push images, SSH restart on VPS)

**Branch Naming:**

- Format: `{type}/{epic-or-story-id}-{short-description}`
- Types: `feat`, `fix`, `chore`, `refactor`, `test`
- Examples: `feat/E2-S3-auth-login`, `fix/E4-S1-trick-resolution`, `chore/E1-S1-project-scaffold`
- Always reference the story or epic ID so branches trace back to the sprint plan

**Commit Messages:**

- Format: `{type}({scope}): {description}`
- Scope matches backend domain package or frontend feature folder
- Keep description under 72 characters
- Examples: `feat(game): implement trump bidding for Bitola variant`, `fix(ws): restore room subscriptions on reconnect`, `test(rules): add table-driven tests for declaration scoring`

**One Story = One Branch = One PR:**

- Each story gets its own branch and PR — no mixing stories in a single branch
- If you discover a bug while working on a story, file it separately — don't fix it in the current branch unless it directly blocks the story

**Database Migrations:**

- Migration files use sequential numbering: `{NNNNNN}_{description}.up.sql` / `.down.sql`
- Before creating a new migration, check the highest existing number in `server/migrations/` and increment by 1. Never skip numbers
- Every `.up.sql` migration must have a corresponding `.down.sql` that fully reverses the change — no exceptions

**Environment Variables:**

- `SCREAMING_SNAKE_CASE` naming convention
- App-specific vars prefixed with `BELJOT_`: `BELJOT_DB_URL`, `BELJOT_JWT_SECRET`, `BELJOT_PORT`
- PostgreSQL vars use standard Docker names: `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`
- Dev: `.env` loaded by Docker Compose + `go run`
- Production: Docker env vars in `docker-compose.prod.yml`
- CI: GitHub Actions secrets
- `.env.example` with all vars and placeholder values checked into git — no secrets in repo

**Production Topology:**

- Caddy (on host) serves static frontend files from `client/dist/` directly and proxies `/api/` and `/ws` to the Go Docker container
- The Go server does NOT serve static files in production
- Go app + PostgreSQL run as Docker containers behind host-level Caddy

**API Response Formats:**

- HTTP success: `{ "data": { ... } }`
- HTTP error: `{ "error": { "code": "ROOM_FULL", "message": "..." } }`
- HTTP status codes: 200, 201, 400, 401, 403, 404, 409, 500
- WebSocket message: `{ "type": "event:card_played", "payload": { ... } }`

**CI/CD:**

- GitHub Actions: `make test` + `make lint` on every push — blocks merge on failure
- Deployment starts manual (`make deploy`), automated later

### Critical Don't-Miss Rules

**Anti-Patterns:**

| Do NOT                                             | Do Instead                                                            |
| -------------------------------------------------- | --------------------------------------------------------------------- |
| Call `fetch()` directly in components              | Use `shared/api/` client functions                                    |
| Define errors inline in handlers                   | Add to `internal/apperr/errors.go`                                    |
| Put game logic in handlers or WebSocket layer      | Keep in `internal/game/` as pure functions                            |
| Store JWT in localStorage                          | Keep access token in memory (Zustand)                                 |
| Build game state with raw struct literals in tests | Use `testfixtures/` factory functions                                 |
| Add WS events to only one contract file            | Update both `wsEvents.ts` and `events.go` together                    |
| Use `PascalCase` or `snake_case` in JSON           | Always `camelCase` in wire format                                     |
| Send relative timer durations ("30 seconds")       | Send absolute timestamps ("expires at X")                             |
| Assume clockwise turn order                        | Counter-clockwise: `(currentPlayer + 1) % 4` — seats are numbered CCW |
| Continue rotation after trick resolves             | Trick winner leads the next trick — rotation resets                   |
| Implement generic trump bidding                    | Branch by variant from the start — separate code paths                |
| Auto-award Belote bonus (K+Q of trump)             | Require player announcement — prompt on trump K/Q play                |

**Security Rules:**

- Server-authoritative architecture is non-negotiable — zero client-side game logic
- All game logic executes server-side; client renders received state only
- Never trust client-submitted game actions — validate everything server-side
- HTTPS + WSS only, enforced by Caddy
- CORS configured for specific origin(s) — no wildcard
- No secrets in repo — `.env.example` has placeholders only

**Game Rules — Critical Correctness:**

- **Turn order is counter-clockwise** — dealing, bidding, and trick play. Seats are numbered counter-clockwise (0→1→2→3→0). Turn advancement: `(currentPlayer + 1) % 4`. Never clockwise
- **Trick winner leads next trick** — rotation resets to the winner, then continues counter-clockwise
- **Trump bidding branches by variant** — Bitola: reshuffle-and-rotate-dealer when no one picks in round 2. Croatian: last player forced to pick. Separate code paths in `bidding.go`, never a generic flow
- **Declarations resolved at first trick only** — after all four cards played on trick 1. Highest-value set wins ties. Only the winning team's declarations count. After resolution, declarations are locked for that hand
- **Declaration single-use (Bitola)** — a single card may belong to only one declaration group. When two detected groups share a card (e.g. sequence 9-T-J-Q of spades AND four Jacks sharing J♠), keep the higher-value group and drop the other. Enforced in `detectDeclarations` via a dedup pass. Croatian variant allows overlap (deferred)
- **Declaration phase timing (Bitola)** — declarations happen during trick 1 as each player plays their first card; reveal fires at the start of trick 2 before the first card. Croatian variant has a separate declaration phase between bidding and trick 1 (all players click yes/no), reveal at the start of trick 1 (deferred)
- **Declaration reveal** — small panel anchored to the winning player's seat showing their actual cards for ~4 seconds (1.5s for reduced-motion). Auto-dismisses. Losing team's declarations are never revealed. Points remain pending (not locked in) until hand-end scoring
- **Belote / Rebelote bonus requires player announcement** — when a player plays trump K or Q and holds the other, UI prompts for announcement. Terminology is rank-bound, not play-order-bound: announcing on the Queen is "Belote", on the King is "Rebelote". If declined on the first K/Q played, bonus is forfeit — no second chance on the other card. +20 pts to the announcing team, pending until hand end. Reveal shows the announced card only (not the partner). Same in both variants
- **Three-layer card validation** in `validation.go`: (1) follow led suit if possible, (2) if void in led suit, must play trump if held, (3) if trumping, must play higher trump than current highest trump in trick if possible
- **Two-phase scoring** in `scoring.go`: hand scoring (card points + declarations + last trick/Capot bonuses → failed hand check) feeds into match scoring (cumulative team totals → win condition check). Keep as separate functions
- **Failed hand**: the taker's team must score **strictly more** than the opponents to succeed. An equal total (a tie, e.g. 81:81 of the 162 base points) is a failed hand for the taker. On a failed hand the taker's team scores 0 and all points (both teams' card points + declarations + bonuses) transfer to the opponents. Decided in exactly one place — `scoring.go` `scoreHand` (`contractingTotal <= opposingTotal`)
- **Tied-hand rule is variant-specific (interim simplification).** The "tie → all points to the opponents" behavior above is the **Croatian-variant** rule, currently applied to **all** variants. The **Bitola variant** must eventually use **hanging points (carry-over)** on a tie (points held over, nobody scores, carried to the next decisive hand) — deferred to Epic 12 (Variant Expansion, Phase 3). Do NOT "fix" the current Bitola tie behavior as a bug; it is a deliberate stand-in. See `deferred-work.md` and `spec-fix-tie-fails-trump-caller.md`
- **Capot** (+100) replaces last-trick bonus (+10), not added to it
- **Match target tiebreaker**: if both teams cross 1001/501 in the same hand, the taker's team (team that called trump) wins — taking trump is the tiebreaker
- **Auto-play on timer expiry**: first legal card sorted by suit then rank

**Card ID Format (Standardized Everywhere):**

- Two-character strings: rank + suit
- Rank: `7`, `8`, `9`, `T` (ten), `J`, `Q`, `K`, `A`
- Suit: `S` (spades), `H` (hearts), `D` (diamonds), `C` (clubs)
- Examples: `KS` = King of Spades, `TH` = Ten of Hearts, `7D` = Seven of Diamonds
- Used in game state, WebSocket payloads, and frontend rendering
- Defined once in `game/types.go` and `gameTypes.ts`

**Disconnection & Reconnection Edge Cases:**

- Disconnection mid-trick, during trump bidding, after card played but before broadcast — all must be handled
- Reconnecting player gets full state snapshot from serializable GameState struct
- Session manager must handle **concurrent disconnection** safely — use mutex or channel-based synchronization. Two simultaneous disconnects = two pause events with independent reconnect windows
- Pause stacking: multiple players can have active pauses simultaneously
- Room owner can override and clear all active pauses

---

## Usage Guidelines

**For AI Agents:**

- Read this file before implementing any code
- Follow ALL rules exactly as documented
- When in doubt, prefer the more restrictive option
- Update this file if new patterns emerge

**For Humans:**

- Keep this file lean and focused on agent needs
- Update when technology stack changes
- Review quarterly for outdated rules
- Remove rules that become obvious over time

Last Updated: 2026-04-06
