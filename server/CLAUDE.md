# Server

Go 1.26, Echo v4, GORM with PostgreSQL 16, coder/websocket, golang-migrate, testify, slog, S3 (Garage) for avatars.

## Commands

```bash
go test -tags nodynamic ./internal/<pkg>/...          # one package
go test -tags nodynamic ./internal/<pkg>/ -run TestName # one test
go vet -tags nodynamic ./...
golangci-lint run ./...                               # what make lint-server runs
go run -tags nodynamic ./cmd/api                      # API on :8080, needs the BELJOT_* env
```

## Structure

- `cmd/api/main.go` loads config, applies migrations, wires dependencies and registers middleware and routes.
- A domain package has the same shape: `model.go`, `repository.go` (interface), `gorm_repo.go`, `handler.go`, and `service.go` when needed.
- `internal/game/` is the pure rules engine. `ApplyAction(state, action)` returns a new state or an error, with no side effects.
- `internal/match/` is the session manager (`Manager`). It calls the engine, runs timers, broadcasts results and settles coins, XP, honor and season points.
- `internal/ws/` holds the hub and `router.go`, which dispatches by event type only and carries no game logic.
- `internal/bot/` decides bot moves from a projected `View`, never from the raw state.

## Conventions

- **Pin Echo to v4.** Do not upgrade to v5.
- **Middleware order in `main.go` is load-bearing:** CORS, request logging, then the error handler, then auth. The logger's `HandleError: true` makes the logged status match what the client received.
- **Config is read once** into `config.Config` and injected. Never call `os.Getenv` outside `internal/config`.
- **Handlers call repository interfaces,** never GORM directly.
- **Wrap errors with `%w`** and branch with `errors.Is` against `apperr` values.
- **Use `*time.Time` for optional timestamps,** because a zero `time.Time` serializes as year 1.
- **GORM soft deletes are automatic.** Raw SQL must filter `deleted_at IS NULL`.
- **Never give a bool column a GORM `default` tag.** GORM then drops `false` from INSERTs.
- **Import direction is one-way:** `match` never imports `user`, `user` never imports `room`, `season` never imports `user`. Cross-domain calls go through small interfaces declared by the caller.
- **Batch updates lock rows `FOR UPDATE` in ascending user ID order** to avoid deadlocks.
- **Engine code clones slices before mutating** (`slices.Clone`), because shared backing arrays break the pure-function contract.
- **The engine branches on `VariantRules` fields,** never on `state.Variant`.

## Migrations

- Files are `NNNNNN_description.up.sql` and `.down.sql` in `migrations/`. Take the next number after the highest existing one and never skip.
- Every `up` has a `down` that fully reverses it.
- The binary applies pending migrations at startup through `internal/dbmigrate`. `make migrate` runs the CLI by hand against `BELJOT_DB_URL`.

## Testing

- Tests sit next to their source and use `testing` with testify.
- Engine tests build state through `internal/game/testfixtures` factories, never raw `GameState{}` literals, and test through `ApplyAction`.
- Prefer table-driven tests with `t.Run`.
- Database tests skip without `BELJOT_TEST_DB_URL`. They create their own data and never rely on seed data or test order.
- WebSocket tests use `httptest.Server` with a real client connection.
- Tests that depend on the current date must derive their windows from `time.Now()` instead of hard-coding a quarter.
