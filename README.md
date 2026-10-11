# Beljot

Beljot is an online multiplayer Belot card game with two rule variants: Bitola and Croatian. It is live at [beljot.online](https://beljot.online). The Go backend in `server/` is authoritative and runs all game logic; the React + TypeScript frontend in `client/` only renders the state it receives over WebSocket.

## Features

- Rooms: create, join and invite players to four-seat tables for either variant
- Quick Play: drop into the next open seat, with bots filling empty places
- Seasons with a tier ladder and a leaderboard
- Coins, XP, levels and honor settled after every match
- Friends, in-room chat and emotes
- Profiles with uploaded avatars stored in S3-compatible object storage
- Email/password and Google sign-in, with password reset by email
- Selectable card decks (French and Croatian suits)
- UI in English, Serbian, Croatian and Macedonian (en, sr, hr, mk)

## Tech stack

| Area | Stack |
|---|---|
| Client | React 19, TypeScript (strict), Vite 6, Tailwind CSS v4, shadcn/ui, Zustand 5, TanStack Query 5, React Router 7, react-i18next, Zod 4 |
| Server | Go 1.26, Echo v4, GORM, coder/websocket, golang-migrate, slog |
| Data | PostgreSQL 16, Garage (S3-compatible) for avatars |
| Tests | Vitest 3 with Testing Library and jsdom; Go `testing` with testify |
| Delivery | GitHub Actions, GHCR images, Dokploy |

## Prerequisites

- [mise](https://mise.jdx.dev): run `mise install` in the repo root to get the pinned Go, Node, golang-migrate and golangci-lint versions from `mise.toml`
- Docker with Docker Compose, for PostgreSQL and Garage

## Getting started

```bash
mise install
cp .env.example .env      # then fill in the values
(cd client && npm install)
make dev
```

`make dev` loads `.env`, starts PostgreSQL and Garage with Docker Compose, then runs the Vite dev server and the Go API side by side.

| Service | Address |
|---|---|
| Client (Vite) | http://localhost:5173 (proxies `/api` and `/ws` to the API) |
| API | http://localhost:8080 (`BELJOT_PORT`) |
| PostgreSQL | localhost:5433 |
| Garage S3 API | localhost:3900 (public bucket web endpoint on :3902) |

Configuration comes from `BELJOT_*` environment variables, read once in `server/internal/config/config.go`. In development every variable has a usable default or is optional: the database URL points at the Compose Postgres, SMTP falls back to logging reset links, and avatar endpoints answer 503 until all `BELJOT_S3_*` and `BELJOT_PUBLIC_ASSETS_URL` values are set. Outside development (`BELJOT_ENV` other than `development`), a default JWT secret or any missing avatar-storage variable stops the server. The binary applies pending migrations at startup.

## Common commands

Run from the repo root.

| Command | What it does |
|---|---|
| `make dev` | Docker Compose services, Vite and the Go API |
| `make build` | Client production build and server binary (`server/bin/api`) |
| `make test` | `test-client` and `test-server` |
| `make test-client` | `npx vitest run` in `client/` |
| `make test-server` | `go test -tags nodynamic ./...` in `server/` |
| `make lint` | `lint-client` and `lint-server` |
| `make lint-client` | `tsc --noEmit`, `eslint .` and `prettier --check .` |
| `make lint-server` | `golangci-lint run ./...` |
| `make migrate` | Apply migrations by hand with the `migrate` CLI against `BELJOT_DB_URL` |
| `make hooks` | Point git at `.githooks/`, enabling the pre-commit hook |

## Running tests

```bash
make test
cd server && go test -tags nodynamic ./internal/game/...        # one package
cd server && go test -tags nodynamic ./internal/game/ -run TestName
cd client && npx vitest run src/features/match                  # one area
```

- Always pass `-tags nodynamic` to `go test`, `go run` and `go build`. It selects the pure-Go WebP encoder that the production image ships.
- Server tests that need PostgreSQL skip unless `BELJOT_TEST_DB_URL` is set. Run `make dev` first, then export it, for example `postgres://beljot:beljot_dev_password@localhost:5433/beljot?sslmode=disable`. CI sets it against a throwaway database.
- The i18n parity test fails if a UI string is missing from any of the four locales.

## Project layout

```
client/src/features/<feature>/   pages and components per feature (match, room, lobby, profile, ...)
client/src/shared/               api/, stores/, hooks/, types/, i18n/, components/ui/ (shadcn)
server/cmd/api/                  entry point: config, migrations, wiring, middleware, routes
server/internal/<domain>/        one package per domain (auth, room, match, season, wallet, ...)
server/internal/game/            pure rules engine
server/internal/match/           live match sessions, timers, settlement
server/internal/ws/              WebSocket hub and event contract
server/internal/bot/             bot decisions
server/migrations/               SQL migrations, embedded in the binary
docs/                            deployment, migration, audio, card deck and bot notes
scripts/                         repository maintenance scripts
.github/workflows/               CI and Deploy workflows
```

`CLAUDE.md`, `client/CLAUDE.md` and `server/CLAUDE.md` hold the detailed conventions for each side. Planning artifacts (specs, epics, stories) live in `_bmad-output/`.

## Deployment

A push to `master` deploys to production. The Deploy workflow runs CI, builds the backend and frontend images that changed, pushes them to GHCR and tells Dokploy to roll the matching services. A commit message containing `[skip ci]` skips both CI and the deploy. CI also runs on pull requests to `master`.

Images, secrets and environment variables are configured in GitHub Actions and the Dokploy project; none of them live in the repo.

## Documentation

| Document | Contents |
|---|---|
| [docs/audio.md](docs/audio.md) | In-match sound effects and music |
| [docs/card-deck.md](docs/card-deck.md) | Card face and suit symbol assets for each deck |
| [docs/bot/BOT_RULES.md](docs/bot/BOT_RULES.md) | How the bot decides its moves |

## Contributing

- Branches: `{type}/{short-description}`, with type one of `feat`, `fix`, `chore`, `refactor`, `test`, `docs`, `ci`.
- Commits: `{type}({scope}): {description}`, where scope is the server domain package or client feature and the description is under 72 characters.
- Run `make hooks` once after cloning. The pre-commit hook runs `make lint` and `make test`.
- The WebSocket contract lives in `server/internal/ws/events.go` and `client/src/shared/types/wsEvents.ts`; change both in the same commit.
- Every UI string goes into all four locale files in `client/src/shared/i18n/`.
- Comment policy: only short doc comments directly above functions, methods and top-level types. A Claude Code PreToolUse hook (`.claude/hooks/comment_policy.py`) rejects other new comments. Audit a path with `python3 .claude/hooks/comment_policy.py audit --lines <path>`.
- Never commit `.env`.
