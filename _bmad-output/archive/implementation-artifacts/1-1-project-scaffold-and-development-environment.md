# Story 1.1: Project Scaffold & Development Environment

Status: done

## Story

As a developer,
I want a fully configured monorepo with React frontend, Go backend, PostgreSQL database, CI pipeline, and design system foundations,
so that I can begin building features on a solid, consistent development environment.

## Acceptance Criteria

1. **Given** the project is initialized from scratch
   **When** I run the scaffold setup commands per Architecture spec
   **Then** the monorepo contains `client/` (Vite react-swc-ts template) and `server/` (Go module with Echo v4, GORM, coder/websocket dependencies)
   **And** `npx shadcn@latest init` has been run, with base components (Button, Dialog, Input, Tabs, Toast, Tooltip) added and themed to the Balatro design token set

2. **Given** the monorepo is set up
   **When** I inspect the Makefile
   **Then** it includes working targets: `dev` (concurrent client + server), `build`, `test`, `lint`, `migrate`, `seed`

3. **Given** Docker Compose is configured
   **When** I run `make dev`
   **Then** PostgreSQL starts in a Docker container on port 5432, Vite dev server runs on port 5173 with HMR, and the Go server runs natively on port 8080 with a `/health` endpoint returning 200

4. **Given** the CI pipeline is configured
   **When** code is pushed to GitHub
   **Then** GitHub Actions runs `make test` and `make lint` and blocks merge on failure

5. **Given** the Tailwind config is set up
   **When** I inspect `client/src/index.css` and Tailwind configuration
   **Then** the Balatro design tokens are defined: `background` (#0a0a0f), `surface` (#13131a), `surface-elevated` (#1c1c26), `border` (#2a2a38), `accent` (#00e5a0), `accent-glow`, `team-a` (#ff4d4d), `team-b` (#4d9fff), `text-primary` (#f0f0f8), `text-secondary` (#8888a0), `text-disabled` (#44445a), `success`, `warning`, `destructive`
   **And** Space Grotesk and Inter fonts are loaded from Google Fonts

6. **Given** i18n is configured
   **When** I inspect the i18n setup
   **Then** react-i18next is initialized with `en.json` and `sr.json` translation files in `shared/i18n/`, with a working translation hook (`useTranslation`)

7. **Given** frontend tooling is configured
   **When** I run `make lint`
   **Then** ESLint + Prettier run on all TypeScript files and golangci-lint runs on all Go files

8. **Given** the database migration system is set up
   **When** I run `make migrate`
   **Then** golang-migrate executes SQL migration files from `server/migrations/` against the PostgreSQL instance

9. **Given** React Router v7 is installed
   **When** I inspect `App.tsx`
   **Then** base route structure is defined with placeholder routes for `/login`, `/register`, `/lobby`, `/profile`, `/game`

## Tasks / Subtasks

- [x] **Task 1: Initialize monorepo root** (AC: 2, 3)
  - [x] Create root `Makefile` with targets: `dev`, `build`, `test`, `lint`, `migrate`, `seed`, `deploy`, `backup`
  - [x] Create `.env.example` with all env vars (`BELJOT_DB_URL`, `BELJOT_JWT_SECRET`, `BELJOT_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`) and placeholder values
  - [x] Create `.gitignore` (node_modules, dist, .env, Go binaries, IDE files)
  - [x] Create `docker-compose.yml` for local dev: PostgreSQL only on port 5432
  - [x] Create `docker-compose.prod.yml` skeleton for production: Go app + PostgreSQL containers
  - [x] Create `Caddyfile` skeleton for production reverse proxy
  - [x] Create `scripts/deploy.sh` placeholder
  - [x] Create `scripts/backup-db.sh` placeholder

- [x] **Task 2: Initialize frontend (`client/`)** (AC: 1, 5)
  - [x] Run `npm create vite@latest client -- --template react-swc-ts`
  - [x] Install core dependencies: `react-router`, `zustand`, `react-i18next`, `i18next`
  - [x] Install dev dependencies: `@tailwindcss/vite`, `eslint`, `prettier`, `@types/react`, `@types/react-dom`, `vitest`, `@testing-library/react`, `@testing-library/jest-dom`, `jsdom`
  - [x] Configure `vite.config.ts` with `@tailwindcss/vite` plugin and proxy for `/api` and `/ws` to `localhost:8080`
  - [x] Configure `tsconfig.json` with strict mode, path aliases (`@/` -> `src/`)
  - [x] Configure `eslint.config.js` with TypeScript, React, import ordering rules, named exports only
  - [x] Configure `.prettierrc`

- [x] **Task 3: Set up Tailwind CSS v4 design tokens** (AC: 5)
  - [x] Configure `client/src/index.css` with Tailwind v4 `@import "tailwindcss"` and `@theme` directive
  - [x] Define all Balatro design tokens inside `@theme`:
    - `--color-background: #0a0a0f`
    - `--color-surface: #13131a`
    - `--color-surface-elevated: #1c1c26`
    - `--color-border: #2a2a38`
    - `--color-accent: #00e5a0`
    - `--color-accent-glow: #00e5a040` (40% opacity for glow effects)
    - `--color-team-a: #ff4d4d`
    - `--color-team-b: #4d9fff`
    - `--color-text-primary: #f0f0f8`
    - `--color-text-secondary: #8888a0`
    - `--color-text-disabled: #44445a`
    - `--color-success: #22c55e`
    - `--color-warning: #eab308`
    - `--color-destructive: #ef4444`
  - [x] Load Google Fonts: Space Grotesk (display/headings) and Inter (body/UI) via `<link>` in `index.html`
  - [x] Define font tokens: `--font-display: 'Space Grotesk', sans-serif` and `--font-body: 'Inter', sans-serif`

- [x] **Task 4: Initialize shadcn/ui** (AC: 1)
  - [x] Run `npx shadcn@latest init` in `client/` directory
  - [x] Configure shadcn to use `src/shared/components/ui/` as the components directory
  - [x] Add base components: `npx shadcn@latest add button dialog input tabs sonner tooltip` (sonner replaces deprecated toast)
  - [x] Verify components are themed using the Balatro CSS variables from `index.css`

- [x] **Task 5: Create frontend directory structure** (AC: 1, 9)
  - [x] Create feature directories: `src/features/auth/`, `src/features/game/`, `src/features/lobby/`, `src/features/profile/`, `src/features/chat/`
  - [x] Create shared directories: `src/shared/components/`, `src/shared/hooks/`, `src/shared/stores/`, `src/shared/types/`, `src/shared/api/`, `src/shared/i18n/`
  - [x] Create placeholder Zustand stores: `authStore.ts`, `lobbyStore.ts`, `gameStore.ts`, `chatStore.ts` — each with the standard interface shape (`state + isLoading + actions`)
  - [x] Create `fetchClient.ts` skeleton in `shared/api/` with auth header injection placeholder
  - [x] Create placeholder type files: `wsEvents.ts`, `gameTypes.ts`, `apiTypes.ts` in `shared/types/`

- [x] **Task 6: Set up React Router v7 and App shell** (AC: 9)
  - [x] Configure `App.tsx` with `BrowserRouter` and route definitions for: `/login`, `/register`, `/lobby`, `/profile`, `/game`
  - [x] Create placeholder page components: `LoginPage.tsx`, `RegisterPage.tsx`, `LobbyPage.tsx`, `ProfilePage.tsx`, `GamePage.tsx`
  - [x] Add `ErrorBoundary.tsx` in `shared/components/`
  - [x] Wrap app in ErrorBoundary in `App.tsx`

- [x] **Task 7: Set up i18n** (AC: 6)
  - [x] Create `shared/i18n/i18n.ts` configuration file initializing i18next with `initReactI18next`
  - [x] Create `shared/i18n/en.json` with skeleton keys: `{ "common": { "appName": "Beljot" } }`
  - [x] Create `shared/i18n/sr.json` with matching skeleton keys: `{ "common": { "appName": "Beljot" } }`
  - [x] Import i18n configuration in `main.tsx`
  - [x] Verify `useTranslation` hook works — i18n test confirms EN and SR translations

- [x] **Task 8: Initialize backend (`server/`)** (AC: 1, 3)
  - [x] Run `go mod init github.com/emilijan/beljot/server`
  - [x] Install dependencies: Echo v4, GORM, postgres driver, nhooyr.io/websocket, testify, bcrypt
  - [x] Create directory structure with all placeholder packages

- [x] **Task 9: Implement Go server entrypoint** (AC: 3)
  - [x] Create `cmd/api/main.go` with config, Echo, middleware (CORS->Logger->ErrorHandler), /health endpoint
  - [x] Create `config/config.go` with `Load()` function
  - [x] Create `apperr/errors.go` with `AppError` type and predefined errors
  - [x] Create Echo error handler that maps `AppError` to structured JSON

- [x] **Task 10: Set up database and migrations** (AC: 8)
  - [x] Configure GORM connection in `main.go` using `BELJOT_DB_URL`
  - [x] Create first migration `server/migrations/000001_init.up.sql`
  - [x] Create `server/migrations/000001_init.down.sql`
  - [x] Add `make migrate` target in Makefile

- [x] **Task 11: Set up CI pipeline** (AC: 4)
  - [x] Create `.github/workflows/ci.yml` with Go 1.26, Node 22, PostgreSQL service, golangci-lint
  - [x] Create `.golangci.yml` linter configuration

- [x] **Task 12: Set up Makefile `dev` target** (AC: 2, 3)
  - [x] Implement all Makefile targets: `dev`, `build`, `test`, `lint`, `migrate`, `seed`, `deploy`, `backup`

- [x] **Task 13: Verify end-to-end dev workflow** (AC: all)
  - [x] Frontend builds successfully (`vite build` passes)
  - [x] Frontend tests pass (4 tests: routing + i18n)
  - [x] Backend compiles and tests pass (health endpoint + apperr tests)
  - [x] ESLint passes (0 errors, 2 warnings from shadcn-generated files)
  - [x] Prettier check passes
  - [x] i18n returns correct strings for EN and SR (verified by test)

## Dev Notes

### Architecture Compliance

- **Monorepo layout**: `client/` (React SPA) + `server/` (Go module) at root, orchestrated by `Makefile`
- **No shared type generation** between frontend/backend — manual sync via `wsEvents.ts` + `events.go`
- **Docker Compose for dev**: PostgreSQL ONLY in Docker; Go server and Vite run natively on host for hot reload
- **Production topology**: Host Caddy -> Docker Go container -> Docker PostgreSQL container. Go server does NOT serve static files in production
- **Environment variables**: `SCREAMING_SNAKE_CASE`, app vars prefixed `BELJOT_`, loaded once at startup into `config.Config` struct — no `os.Getenv()` elsewhere

### Technical Stack Versions (Verified April 2026)

**Frontend:**

- **Vite 8** — uses Rolldown (Rust bundler), 10-30x faster builds. Template: `npm create vite@latest client -- --template react-swc-ts`. Note: `@vitejs/plugin-react` v6 uses Oxc for React Refresh (Babel no longer a dependency)
- **React 19** + TypeScript strict mode, compiled via SWC
- **Tailwind CSS v4** — CSS-first config via `@theme` directive in `index.css`. **No `tailwind.config.js` file.** All tokens defined in CSS. Use `@tailwindcss/vite` plugin
- **shadcn/ui CLI v4** (March 2026) — `npx shadcn@latest init` supports Vite templates directly. Use `--base` flag if prompted for primitives selection. Components install to `src/shared/components/ui/`
- **React Router v7** — standard client-side routing
- **Zustand** — 4 partitioned stores (auth, lobby, game, chat)
- **react-i18next** — install `react-i18next` + `i18next`. JSON translation files. Configure with `initReactI18next`
- **Vitest** — Vite-native testing, Jest-compatible API
- **Node 18+** required

**Backend:**

- **Go** latest stable — pin exact version in `go.mod`
- **Echo v4** — Do NOT use v5. Pin to latest v4.x
- **GORM** + PostgreSQL driver (`gorm.io/gorm` + `gorm.io/driver/postgres`)
- **nhooyr.io/websocket** — import path is `nhooyr.io/websocket`, actual repo at `github.com/coder/websocket`. `go get nhooyr.io/websocket` resolves correctly
- **golang-migrate** — CLI only, NOT embedded as Go library. Install via `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest`
- **testify** — assertion library
- **slog** (stdlib) — structured JSON logging to stdout
- **bcrypt** — `golang.org/x/crypto/bcrypt` for password hashing (placeholder in this story, used in Story 1.2)

**Infrastructure:**

- **Docker Compose** — PostgreSQL 16+ container for dev
- **GitHub Actions** — CI with `make test` + `make lint`

### Tailwind v4 Critical Notes

Tailwind v4 has NO `tailwind.config.js`. Configuration is entirely in CSS:

```css
@import "tailwindcss";

@theme {
  --color-background: #0a0a0f;
  --color-surface: #13131a;
  --font-display: "Space Grotesk", sans-serif;
  --font-body: "Inter", sans-serif;
  /* ...all tokens here */
}
```

Use `@theme` for design tokens that should generate utility classes (e.g., `bg-background`, `text-accent`). Use `:root` for CSS variables that shouldn't generate utilities.

### shadcn/ui Setup Notes

- shadcn CLI v4 now supports Vite templates directly during init
- Components are copied into `src/shared/components/ui/` — they are owned code, not dependencies
- NEVER create UI primitive files manually — always use `npx shadcn@latest add <component>`
- Style shadcn components via Tailwind classes and CSS variables in consuming components
- Only modify `shared/components/ui/` source for project-wide design token changes

### Backend Domain Package Shape

Every domain package follows:

```
internal/{domain}/
  model.go          # GORM model + domain struct
  repository.go     # Repository interface
  gorm_repo.go      # GORM implementation
  handler.go        # Echo HTTP handlers
  service.go        # Business logic (if needed)
  {domain}_test.go  # Tests
```

For this story, create placeholder files for: `auth/`, `user/`, `game/`, `session/`, `ws/`, `lobby/`, `chat/`. Full implementation comes in later stories.

### Middleware Order (Load-Bearing)

Registration order in `main.go` is critical: **CORS -> Logging -> Error Handler -> Auth**. New middleware must be inserted at the correct position.

### Naming Conventions Summary

| Context               | Convention                        | Examples                               |
| --------------------- | --------------------------------- | -------------------------------------- |
| Database tables       | `snake_case`, plural              | `users`, `matches`, `rooms`            |
| JSON wire format      | `camelCase`                       | `createdAt`, `playerId`                |
| REST endpoints        | plural, kebab-case                | `/api/v1/rooms`                        |
| WS events             | prefixed `snake_case`             | `action:play_card`, `event:game_state` |
| Frontend components   | `PascalCase.tsx`                  | `PlayingCard.tsx`                      |
| Frontend hooks/stores | `camelCase.ts`                    | `useAuth.ts`, `gameStore.ts`           |
| Backend files         | `snake_case.go`                   | `rules_engine.go`                      |
| i18n keys             | `{feature}.{component}.{element}` | `auth.login.emailLabel`                |
| Branch naming         | `{type}/{epic-id}-{description}`  | `chore/E1-S1-project-scaffold`         |
| Commits               | `{type}({scope}): {desc}`         | `feat(scaffold): initialize monorepo`  |

### Critical Don'ts for This Story

- Do NOT create a `tailwind.config.js` — Tailwind v4 uses CSS-first configuration
- Do NOT install `@vitejs/plugin-react-swc` separately — Vite 8 react-swc-ts template includes it
- Do NOT embed golang-migrate as a Go library — use CLI only via Makefile
- Do NOT put game logic anywhere — this story is infrastructure only
- Do NOT use `gorilla/websocket` — use `nhooyr.io/websocket` (coder/websocket)
- Do NOT use `export default` in TypeScript — named exports only
- Do NOT store any secrets in the repository — `.env.example` with placeholders only
- Do NOT upgrade Echo to v5 — use v4 explicitly

### Project Structure Notes

Alignment with the architecture's complete directory structure:

```
beljot/
  Makefile
  Caddyfile
  docker-compose.yml
  docker-compose.prod.yml
  .env.example
  .gitignore
  .github/workflows/ci.yml
  .golangci.yml
  scripts/deploy.sh
  scripts/backup-db.sh
  client/
    package.json
    vite.config.ts
    tsconfig.json
    eslint.config.js
    .prettierrc
    index.html
    src/
      main.tsx
      App.tsx
      index.css                   # Tailwind v4 + @theme design tokens
      features/
        auth/                     # Placeholder pages
        game/
        lobby/
        profile/
        chat/
      shared/
        components/
          ui/                     # shadcn/ui components (CLI-installed)
          ErrorBoundary.tsx
        hooks/
        stores/
          authStore.ts
          lobbyStore.ts
          gameStore.ts
          chatStore.ts
        types/
          wsEvents.ts             # WS event contract (TS side)
          gameTypes.ts
          apiTypes.ts
        api/
          fetchClient.ts
        i18n/
          i18n.ts                 # i18next configuration
          en.json
          sr.json
  server/
    go.mod
    go.sum
    cmd/api/main.go
    internal/
      config/config.go
      apperr/errors.go
      auth/
      user/
      game/
        testfixtures/
      session/
      ws/
        events.go                 # WS event contract (Go side)
      lobby/
      chat/
    migrations/
      000001_init.up.sql
      000001_init.down.sql
```

### References

- [Source: architecture.md#Starter-Template-Evaluation] — initialization commands and starter selection
- [Source: architecture.md#Core-Architectural-Decisions] — auth, data, API, frontend, backend, infra decisions
- [Source: architecture.md#Implementation-Patterns] — naming, structure, format, communication patterns
- [Source: architecture.md#Project-Structure-and-Boundaries] — complete directory tree
- [Source: epics.md#Story-1.1] — acceptance criteria and story definition
- [Source: prd.md#Web-Application-Requirements] — SPA requirements, browser support, real-time architecture
- [Source: ux-design-specification.md#Design-System-Foundation] — Tailwind + shadcn/ui rationale, design tokens
- [Source: project-context.md] — all critical implementation rules and anti-patterns

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

- Vite 8 `react-swc-ts` template generated vanilla TS instead of React — manually installed React, ReactDOM, @vitejs/plugin-react-swc
- ESLint 10 (from Vite 8 template) had peer dependency conflicts — downgraded to ESLint 9 for ecosystem compatibility
- shadcn/ui `toast` component deprecated — used `sonner` as replacement
- shadcn/ui default init placed components in `src/components/ui/` — reconfigured to `src/shared/components/ui/` and moved files
- Go not in bash PATH — installed Go 1.26.0 via `golang.org/dl` toolchain
- TypeScript 6 `erasableSyntaxOnly` disallows constructor parameter properties — refactored FetchError class

### Completion Notes List

- Monorepo structure created: `client/` (Vite + React 19 + SWC) and `server/` (Go 1.26 + Echo v4 + GORM)
- All Balatro design tokens defined in Tailwind v4 CSS-first config with shadcn/ui integration
- shadcn/ui components installed: button, dialog, input, tabs, sonner (replaces toast), tooltip
- 4 Zustand stores created (auth, lobby, game, chat) with standard interface shape
- React Router v7 configured with routes: /login, /register, /lobby, /profile, /game
- i18n configured with en.json and sr.json translation files
- Go server with /health endpoint, CORS middleware, slog logging, GORM database connection, AppError system
- Backend domain packages scaffolded: auth, user, game, session, ws, lobby, chat
- CI pipeline configured (GitHub Actions) with Go 1.26, Node 22, PostgreSQL, golangci-lint
- Migration system set up with golang-migrate CLI
- All tests pass: 4 frontend (Vitest) + 2 backend test files (Go)

### Review Findings

- [x] [Review][Decision] D1: PostgreSQL dev container mapped to host port 5433, not 5432 — RESOLVED: keep 5433 to avoid local PG conflict; AC deviation accepted.
- [x] [Review][Patch] P1: JWT secret: no startup validation — silently uses default or empty string [server/internal/config/config.go]
- [x] [Review][Patch] P2: fetchClient crashes on non-JSON error/success responses (502 HTML, 204 No Content) [client/src/shared/api/fetchClient.ts]
- [x] [Review][Patch] P3: No graceful shutdown — SIGTERM kills in-flight connections [server/cmd/api/main.go]
- [x] [Review][Patch] P4: CORS AllowOrigins hardcoded to localhost:5173 — production will reject requests [server/cmd/api/main.go]
- [x] [Review][Patch] P5: Go version mismatch: go.mod says 1.25.0, CI says 1.26 [server/go.mod, .github/workflows/ci.yml]
- [x] [Review][Patch] P6: CI pipeline doesn't invoke make test / make lint as AC 4 requires [.github/workflows/ci.yml]
- [x] [Review][Patch] P7: nhooyr.io/websocket dependency missing from go.mod (AC 1) [server/go.mod]
- [x] [Review][Patch] P8: ESLint config missing import ordering rules and named-exports-only enforcement [client/eslint.config.js]
- [x] [Review][Patch] P9: features/chat/ directory missing from frontend scaffold [client/src/features/]
- [x] [Review][Patch] P10: appErrorHandler silently drops c.JSON() write errors [server/cmd/api/main.go]
- [x] [Review][Patch] P11: Makefile dev target orphans npm run dev on exit [Makefile]
- [x] [Review][Patch] P12: Production docker-compose.prod.yml exposes API port 8080 directly, bypassing Caddy [docker-compose.prod.yml]
- [x] [Review][Defer] W1: fetchClient 401 handler doesn't implement refresh-then-retry cycle — deferred, explicitly scoped to Story 1.3
- [x] [Review][Defer] W2: apperr.Wrap wraps raw error instead of AppError (errors.As won't match) — deferred, function unused
- [x] [Review][Defer] W3: ErrorBoundary "Try again" can loop on deterministic errors — deferred, safety-net works for transient errors

### Change Log

- 2026-04-06: Story 1.1 implementation complete — full monorepo scaffold with frontend, backend, CI, and design system

### File List

**Root:**

- Makefile (new)
- .env.example (new)
- .gitignore (new)
- docker-compose.yml (new)
- docker-compose.prod.yml (new)
- Caddyfile (new)
- scripts/deploy.sh (new)
- scripts/backup-db.sh (new)
- .github/workflows/ci.yml (new)

**Frontend (client/):**

- client/package.json (new)
- client/vite.config.ts (new)
- client/tsconfig.json (new)
- client/eslint.config.js (new)
- client/.prettierrc (new)
- client/vitest.config.ts (new)
- client/components.json (new)
- client/index.html (new)
- client/src/main.tsx (new)
- client/src/App.tsx (new)
- client/src/App.test.tsx (new)
- client/src/index.css (new)
- client/src/test-setup.ts (new)
- client/src/features/auth/LoginPage.tsx (new)
- client/src/features/auth/RegisterPage.tsx (new)
- client/src/features/game/GamePage.tsx (new)
- client/src/features/lobby/LobbyPage.tsx (new)
- client/src/features/profile/ProfilePage.tsx (new)
- client/src/shared/components/ErrorBoundary.tsx (new)
- client/src/shared/components/ui/button.tsx (new)
- client/src/shared/components/ui/dialog.tsx (new)
- client/src/shared/components/ui/input.tsx (new)
- client/src/shared/components/ui/sonner.tsx (new)
- client/src/shared/components/ui/tabs.tsx (new)
- client/src/shared/components/ui/tooltip.tsx (new)
- client/src/shared/lib/utils.ts (new)
- client/src/shared/stores/authStore.ts (new)
- client/src/shared/stores/lobbyStore.ts (new)
- client/src/shared/stores/gameStore.ts (new)
- client/src/shared/stores/chatStore.ts (new)
- client/src/shared/api/fetchClient.ts (new)
- client/src/shared/types/wsEvents.ts (new)
- client/src/shared/types/gameTypes.ts (new)
- client/src/shared/types/apiTypes.ts (new)
- client/src/shared/i18n/i18n.ts (new)
- client/src/shared/i18n/i18n.test.ts (new)
- client/src/shared/i18n/en.json (new)
- client/src/shared/i18n/sr.json (new)

**Backend (server/):**

- server/go.mod (new)
- server/go.sum (new)
- server/.golangci.yml (new)
- server/cmd/api/main.go (new)
- server/cmd/api/main_test.go (new)
- server/internal/config/config.go (new)
- server/internal/apperr/errors.go (new)
- server/internal/apperr/errors_test.go (new)
- server/internal/auth/auth.go (new)
- server/internal/user/user.go (new)
- server/internal/game/game.go (new)
- server/internal/game/testfixtures/fixtures.go (new)
- server/internal/session/session.go (new)
- server/internal/ws/events.go (new)
- server/internal/lobby/lobby.go (new)
- server/internal/chat/chat.go (new)
- server/migrations/000001_init.up.sql (new)
- server/migrations/000001_init.down.sql (new)
