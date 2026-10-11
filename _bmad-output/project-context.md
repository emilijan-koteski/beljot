---
project_name: "beljot"
user_name: "Emilijan"
date: "2026-10-11"
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
optimized_for_llm: true
verified_against: "e8d3d0c"
---

# Project Context for AI Agents

_Rules BMAD agents must follow in this repo. The hand-written agent instructions are the source of truth and load automatically in Claude Code: `CLAUDE.md`, `client/CLAUDE.md`, `server/CLAUDE.md`, and `.claude/rules/game-rules.md` (path-scoped to game, bot, match and rules code). Read them first. This file only adds what they do not say; where anything here conflicts with them or with the code, they win._

---

## Stack and versions

- Tool versions are pinned in `mise.toml`: Go 1.26, Node 22, golangci-lint 2.14.0, golang-migrate 4.20.1. Do not assume other versions.
- Stack lists and folder layout: see `CLAUDE.md` (Layout), `client/CLAUDE.md` and `server/CLAUDE.md`. Do not copy them here.
- Do not upgrade Echo to v5. The WebSocket library is `github.com/coder/websocket` (not gorilla, not the old `nhooyr.io` path).
- Persistence is GORM on PostgreSQL 16 with GORM's default naming: snake_case columns, pluralized tables, `CreatedAt`/`UpdatedAt`/`DeletedAt`. Do not override it. IDs are `uint` auto-increment, not UUIDs.
- Avatars live in S3-compatible object storage (Garage in dev and prod), handled by `server/internal/avatar`.

## Running and verifying

- Commands and flags: `CLAUDE.md` (Commands). Always pass `-tags nodynamic` to Go builds and tests.
- `make dev` starts Postgres on host port 5433 (not 5432) and Garage S3 on 3900/3902 via `docker-compose.yml`, then Vite (5173) and the API (8080).
- There are no `make seed`, `make deploy` or `make backup` targets; tests create their own data. `make hooks` opts into `.githooks/pre-commit` (runs `make lint` and `make test`); it is not installed by default, so CI is the gate.
- CI (`.github/workflows/ci.yml`) runs on pull requests to `master` and is called by the Deploy workflow on every push to `master`: `make lint-client`, `make test-client`, golangci-lint, CLI migrations against a Postgres 16 service, `make test-server`.
- Migrations: the binary embeds `server/migrations` and applies pending ones at startup (`server/internal/dbmigrate`). `make migrate` is the manual CLI path against `BELJOT_DB_URL`. Numbering and up/down rules: `server/CLAUDE.md` (Migrations).

## Deployment

- A push to `master` deploys to production unless the commit message contains `[skip ci]` (see `CLAUDE.md`, Git and deployment).
- GitHub Actions builds `beljot-backend` and `beljot-frontend` images to GHCR (only the side that changed), then Dokploy rolls the Docker Swarm services. Full detail: `docs/DEPLOYMENT.md`.
- Traefik terminates TLS and routes `/api`, `/ws` and `/health` to the backend on 8080; everything else goes to the nginx frontend image serving the SPA. The Go server never serves static files.
- Production configuration is `BELJOT_*` environment variables set in Dokploy. Never commit `.env`; `.env.example` holds placeholders only.

## Language rules

**TypeScript**

- Strict mode; no `any`.
- Never use raw `JSON.parse` output for WebSocket frames. All incoming frames go through `shared/hooks/useWsDispatch.ts`, validated by the Zod schemas in `shared/types/wsEvents.schemas.ts`.

**Go**

- JSON fields must be exported with explicit `json:"camelCase"` tags; unexported fields silently vanish from the output.
- `GameState` is a serializable struct grouped in sections (metadata, hand, trick, players, scoring, timer). Put new fields in the right section.
- Log internal errors with `slog`; user-facing messages stay generic.

**Cross-language**

- Timestamps are ISO 8601 strings in JSON and `timestamptz` in Postgres, formatted for display via i18n.
- Card IDs are two characters, rank plus suit (`KS`, `TH`, `7D`). Defined in `server/internal/game/types.go` and `client/src/shared/types/matchTypes.ts` (`CardId`).

## Framework rules

**Client** (details in `client/CLAUDE.md`)

- HTTP goes through `shared/api/` modules on `axiosClient.ts`, consumed via TanStack Query hooks in `shared/hooks/queries/` and `shared/hooks/mutations/`. Do not reference `fetchClient.ts`; it does not exist.
- Stores are `authStore`, `matchStore`, `roomStore`, `chatStore`, `levelUpStore`, `rankChangeStore` in `shared/stores/`.
- `matchStore` is cleared when the player leaves the match page, not on the match-end event; the final state must survive for the score reveal.
- Client-side validation is UX only; the server is the authority.
- Features live in `client/src/features/`: auth, chat, friends, landing, leaderboard, legal, lobby, match, profile, room, rules. New features get their own folder.
- One frontend API module per backend resource (`rooms.ts`, `matches.ts`, `wallet.ts`, ...). A new backend domain with HTTP endpoints gets a new module.

**Server** (details in `server/CLAUDE.md`)

- The session manager is `server/internal/match` (`Manager`): it validates turn order, calls the engine, runs timers, broadcasts, and owns all side effects. The `ws` hub only routes frames by prefix to the registered handlers.
- One multiplexed WebSocket per client carries game state, lobby, chat, timers and presence, distinguished by event `type`.
- Session state is guarded by mutexes; concurrent disconnects create independent pauses with their own reconnect windows. Pauses stack, and the room owner can clear all of them.
- Errors map to HTTP through `appErrorHandler` in `cmd/api/main.go` using `internal/apperr` values. REST routes live under `/api/v1/`, plural and kebab-case.

**Authentication**

- Access token in memory (`authStore`), refresh token in an httpOnly cookie. The WebSocket authenticates with the access token in its first message; on expiry the client refreshes through `axiosClient.ts` and re-authenticates.

## Testing rules

Baseline rules are in `client/CLAUDE.md` and `server/CLAUDE.md` (Testing). In addition:

- Engine tests go through `ApplyAction` only. Internal files (`bidding.go`, `declarations.go`, `scoring.go`, `validation.go`) are covered by their effect on its output. Factories that do not fit get a new factory in `internal/game/testfixtures`.
- Session tests cover reconnection at each phase (bidding, playing, between tricks, between hands) and assert the projected view for the seat, never raw state.
- Database tests wrap each test in a transaction rolled back in `t.Cleanup`; they never share data across tests.
- Client game components are tested as renderers of server state, not as game logic.

## Code quality

| Context          | Convention                        | Example                                     |
| ---------------- | --------------------------------- | ------------------------------------------- |
| DB tables        | snake_case, plural                | `users`, `rooms`                            |
| Foreign keys     | `{table_singular}_id`             | `user_id`, `match_id`                       |
| Indexes          | `idx_{table}_{column}`            | `idx_users_email`                           |
| WS events        | prefixed snake_case               | `action:play_card`, `event:trick_resolved`  |
| Client files     | `PascalCase.tsx` / `camelCase.ts` | `PlayingCard.tsx`, `useWebSocket.ts`        |
| Go files         | `snake_case.go`                   | `rules_engine.go`                           |
| i18n keys        | `{feature}.{component}.{element}` | `auth.login.emailLabel`                     |

- Imports: the client uses `simple-import-sort` (`npx eslint --fix`). Go linting is the golangci-lint set in `server/.golangci.yml`; no import-order linter is configured.
- Comments are restricted by a hook; see `CLAUDE.md` (Comments).

## Workflow rules

- Branch and commit formats: `CLAUDE.md` (Git and deployment).
- Keep a story's change focused. File an unrelated bug separately unless it blocks the story.
- A feature is done when it has: handler, repository and tests; any new domain errors in `server/internal/apperr/`; WebSocket events in both `server/internal/ws/events.go` and `client/src/shared/types/wsEvents.ts` (plus its Zod schema); a co-located client test; strings in all four locales (en, sr, hr, mk); `make lint` and `make test` passing.

## Game rules (owner-confirmed)

The full, authoritative list is `.claude/rules/game-rules.md`. Do not "fix" any of these toward generic Belot. Key points for planning:

- Play is counter-clockwise; next seat is `(seat + 1) % 4`. The trick winner leads the next trick.
- Bidding, declarations, deal shape and tie handling branch on `VariantRules` fields (`server/internal/game/types.go`, presets from `RulesFor`), never on `state.Variant`. Keep a separate code path per outcome.
- **Bitola:** two bidding rounds over a flipped candidate card; four passes in round 2 reshuffle and rotate the dealer. Declarations are made during trick 1 and revealed at the start of trick 2. A card can belong to only one declaration.
- **Croatian:** one round, no candidate card. Players bid on six visible cards; the two face-down cards stay hidden from everyone until trump is set. Bidding starts to the dealer's right; if the other three pass, the dealer must pick. A dedicated declaration phase sits between bidding and trick 1, and one card may count in several declarations.
- Only the winning declaration team scores declarations; the losing team's are never revealed. The reveal banner shows for 4 s (1.5 s with reduced motion), from `shared/lib/motion.ts`.
- Belote/Rebelote (+20) counts only when announced: on the Queen it is Belote, on the King Rebelote. Declining on the first card forfeits it.
- Card play (`validation.go` `legalCards`, mirrored in `client/src/features/match/lib/legalCards.ts`): follow suit, overplaying only while the led suit can still win; if void and holding trump, cut and over-trump when able; otherwise any card.
- Scoring (`scoring.go`): the taker's team must score strictly more; a tie fails and all points go to the opponents (`contractingTotal <= opposingTotal` in `scoreHand`). Bitola's preset already declares `TieRuleHangingPoints`, but `scoreHand` does not read it yet; the current tie behavior is a deliberate stand-in, not a bug.
- Capot (+100) replaces the last-trick bonus (+10). If both teams cross the target in the same hand, the taker's team wins. Timer auto-play picks the first legal card sorted by suit, then rank.
- Reconnecting or observing seats receive a projected state: own hand real, other hands `[]` with `handCount`, no deck. Deal and splash grace in `match/deal_grace.go` must match `motion.ts`.

---

Last Updated: 2026-10-11
