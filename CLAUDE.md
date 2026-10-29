# Beljot

Online multiplayer Belot card game (Bitola and Croatian variants). Go backend in `server/`, React + TypeScript frontend in `client/`. The server is authoritative: all game logic runs server-side and the client only renders state it receives.

Folder-specific guidance loads on demand from `client/CLAUDE.md` and `server/CLAUDE.md`. Game-rule correctness notes live in `.claude/rules/game-rules.md`.

## Commands

Tool versions are pinned in `mise.toml` (Go 1.26, Node 22, golangci-lint 2.14.0). Run from the repo root.

```bash
make dev           # docker compose (Postgres :5433, Garage S3 :3900) + Vite + Go API
make lint          # client: tsc, eslint, prettier --check; server: golangci-lint
make test          # vitest run + go test -tags nodynamic ./...
make lint-client   # or lint-server / test-client / test-server for one side
make build         # client dist + server binary
make hooks         # once per clone: pre-commit runs make lint and make test
```

- Always pass `-tags nodynamic` to `go test`, `go run` and `go build`. It selects the pure-Go WebP encoder the production image ships.
- Server tests that need Postgres skip unless `BELJOT_TEST_DB_URL` is set. CI sets it; locally run `make dev` first and export it.
- `BELJOT_BOT_SIM=1 go test -tags nodynamic ./internal/bot/` runs the bot simulation gates (≈10 s); CI sets it.
- Run `make lint` and `make test` for the side you changed before calling work done.

## Layout

```
client/src/features/<feature>/   pages and components per feature (match, room, lobby, profile, ...)
client/src/shared/               api/, stores/, hooks/, types/, i18n/, components/ui/ (shadcn)
server/cmd/api/                  entry point: config, migrations, DI wiring, middleware, routes
server/internal/<domain>/        one package per domain (auth, room, match, game, season, wallet, ...)
server/internal/game/            pure rules engine
server/internal/match/           live match sessions, timers, settlement
server/internal/ws/              WebSocket hub and event contract
server/migrations/               SQL migrations, embedded and applied by the binary at startup
docs/                            deployment, migration, audio, card deck notes
_bmad-output/                    BMAD planning artifacts (specs, epics, stories)
```

## Cross-cutting rules

- **WebSocket contract lives in two files.** `server/internal/ws/events.go` and `client/src/shared/types/wsEvents.ts` change together in the same commit. Event names are prefixed `action:`, `event:`, `error:` or `system:`.
- **JSON is camelCase on the wire.** Go structs carry explicit `json:"camelCase"` tags.
- **Go zero values are real values.** `0`, `""` and `false` arrive as values, never `null`. The client compares explicitly (`=== 0`, `=== false`) and never uses truthiness on server fields.
- **Timers use absolute server timestamps** ("expires at"), never relative durations.
- **Multi-step game events are separate ordered messages.** Client animations depend on the order.
- **Domain errors live in `server/internal/apperr/`.** HTTP errors are `{ "error": { "code", "message" } }`, success is `{ "data": ... }`.
- **Client and server mirror some constants by hand:** honor bands, the season tier ladder, deal and splash timings, card IDs, the match-mode target (`game.MatchTarget` and `matchTargetFor` in `client/src/shared/types/matchTypes.ts`), the seat step (`game.StepSeat` and `seatAfter` in `client/src/features/match/lib/dealSchedule.ts`), and the new-room defaults (`defaultMoveSeconds` and `roomWord` in `server/internal/room/` against `DEFAULT_MOVE_SECONDS` in `CreateRoomModal.tsx` and `lobby.createRoomModal.defaultName` in the four locales). When you change one side, search the other for the same value.
- **i18n:** every UI string goes into all four locales in `client/src/shared/i18n/` (en, sr, hr, mk). A parity test fails otherwise.

## Comments

A PreToolUse hook (`.claude/hooks/comment_policy.py`) rejects new comments that break this rule:

- Allowed: a doc comment of one or two short sentences, at most three lines, directly above a function, method or top-level type. Tool directives (`//go:`, `//nolint`, `eslint-disable`, `@ts-expect-error`) are fine.
- Not allowed: comments inside function bodies, between struct, interface, object or JSX members, at the end of a line, above test blocks, section banners, and story or ticket references.
- Express intent through names, small functions and named constants instead.
- Audit any path with `python3 .claude/hooks/comment_policy.py audit --lines <path>`. Use `/clean-comments <path>` to clean an area.

## Git and deployment

- **A push to `master` deploys to production.** The Deploy workflow runs CI, builds images to GHCR and rolls the Dokploy services.
- **`[skip ci]` in a commit message skips both CI and the deploy.** Use it only for changes that must not ship yet, or that cannot affect the build.
- CI also runs on pull requests to `master`.
- Branches: `{type}/{short-description}`, types `feat`, `fix`, `chore`, `refactor`, `test`, `docs`, `ci`.
- Commits: `{type}({scope}): {description}`, scope being the domain package or client feature, description under 72 characters.
- Never commit `.env`. Configuration comes from `BELJOT_*` environment variables read once in `server/internal/config`.

## Planning artifacts

Specs, epics and stories live in `_bmad-output/`. BMAD skills (`bmad-*`) read `_bmad-output/project-context.md`. Where it conflicts with this file or the code, the code and this file win.
