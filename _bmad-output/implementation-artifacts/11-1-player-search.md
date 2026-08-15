---
baseline_commit: e830dc360310541f8c6ef482e38222f50ae27511
---

# Story 11.1: Player Search

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a player,
I want to search for other players by username,
so that I can find friends and view their profiles.

## Acceptance Criteria

**Epic source:** `_bmad-output/planning-artifacts/epics.md#Story-11.1-Player-Search` (FR5). Covers the three epic ACs, expanded into testable form. The scope decision on AC5 (navigation target) is recorded in Dev Notes → **Scope & Sequencing Decision**.

1. **AC1 — Search endpoint returns matches.** An authenticated `GET /api/v1/users?search=<query>` returns users whose username matches `<query>` **case-insensitively** as a **substring** (`ILIKE '%query%'`). Response envelope: `{ "data": [ { "id": <uint>, "username": "<string>" }, ... ] }`. Results exclude the requesting user themselves and exclude soft-deleted users. Results are ordered by `username` ascending and capped at a fixed server-side limit (10).
2. **AC2 — Input validation.** A missing, empty, or whitespace-only `search` param returns `400` with the standard error envelope (`{ "error": { "code": "BAD_REQUEST", "message": ... } }`). LIKE metacharacters (`%`, `_`, `\`) in the query are treated as literal characters, never as wildcards (e.g. searching `%` returns only users whose username literally contains `%` — in practice none, since the username charset is `[a-zA-Z0-9_]`; `_` is a valid username char and must match literally).
3. **AC3 — Live-filtering search UI.** A player in the lobby can type into a player-search input; results update live (debounced) as they type, driven by the AC1 endpoint via TanStack Query. No network request fires until the trimmed query is at least 2 characters. Each result row shows the matched username and is keyboard-accessible.
4. **AC4 — Empty state.** When the endpoint returns zero matches for a non-empty query, the UI shows an empty state with the exact message pattern **"No players found matching '<query>'"** (localized, query interpolated), rendered with `data-testid="player-search-empty"`.
5. **AC5 — Navigate to public profile.** Clicking (or activating via keyboard) a result navigates to that player's public-profile route `/players/:id` (i.e. `navigate('/players/' + id)`). **The `/players/:id` route and public-profile page were delivered by Story 11.3 (done)** — this story owns only the navigation wiring, verified in tests by asserting `useNavigate` is called with `/players/<id>`. See Dev Notes → **Scope & Sequencing Decision**.
6. **AC6 — i18n & quality gates.** All new user-facing strings are added to **all four** locale files (`en`, `sr`, `mk`, `hr`) with the `mk` values all-Cyrillic and no em dash (`—`) in `mk`/`sr`/`hr`. `make lint` and `make test` pass (Go: `go vet` + `golangci-lint` + `gofmt`; client: `tsc` build + `vitest` + `eslint` + `prettier`; i18n parity test green).

## Tasks / Subtasks

- [x] **Task 0: Branch setup — whole-epic-on-one-branch**
  - [x] **Continue on the current branch `feat/11-3-public-player-profiles`** (do NOT cut a new per-story branch). Per user direction, the remaining Epic 11 stories (11-1, 11-2, 11-4, 11-5) ship as ONE feature/PR on this branch — the same whole-scope-on-one-branch pattern used for 9.7+9.8. The branch already contains Epic 9 (honor, merged `dc14308`) + Story 11.3, so the `/players/:id` public-profile route this story navigates to already exists. This overrides the "one story = one branch = one PR" default for Epic 11 only, by explicit instruction.

- [x] **Task 1: Backend — player search endpoint** (AC: #1, #2)
  - [x] Add repository method `SearchByUsername(query string, excludeUserID uint, limit int) ([]User, error)` to the `UserRepository` interface in [server/internal/user/repository.go](server/internal/user/repository.go). Documented (case-insensitive substring, excludes self + soft-deleted, capped, metacharacters escaped).
  - [x] Implemented in `GormUserRepository` in [server/internal/user/gorm_repo.go](server/internal/user/gorm_repo.go) with `ESCAPE '\'` + an unexported `escapeLike`/`likeEscaper` (single-pass `strings.NewReplacer` escaping `\`, `%`, `_`). Model query keeps the `deleted_at IS NULL` default scope. Verified escape behavior in the DB-backed repo test.
  - [x] Updated every `user.UserRepository` mock (compiler-driven via `go vet ./...`): `mockUserRepo` (user/handler_test), `fakeHonorRepo` (honor_service_test), `fakeLevelRepo` (xp_service_test), `auth.mockUserRepo`, `chat.userRepoStub`, `lobby.fakeUserRepo`. The `match.stubHonorRecorder` satisfies the narrow `match.HonorRecorder`, so it was correctly NOT touched.
  - [x] Added `SearchUsers(c echo.Context) error` handler in [server/internal/user/handler.go](server/internal/user/handler.go) + `PlayerSearchResult` DTO + `const searchResultLimit = 10`. `strings.TrimSpace` → empty is `apperr.ErrBadRequest`; caller id via `getUserID`; non-nil `items` slice so no-match serializes as `[]`.
  - [x] Registered `api.GET("/users", userHandler.SearchUsers)` in [server/cmd/api/main.go](server/cmd/api/main.go) in the authenticated `api` group. No path collision with `/users/:id/profile` (verified in tests via `e.ServeHTTP`).
  - [x] **Tests:**
    - [x] Repo DB-backed test `TestGormUserRepository_SearchByUsername` in [server/internal/user/user_test.go](server/internal/user/user_test.go): case-insensitive, substring, self-exclusion, soft-deleted exclusion, literal `_`, literal `%` (does not match everyone), ordering, and limit cap. Ran against dev DB `:5433` (all 6 subtests RAN, not skipped).
    - [x] Handler tests in [server/internal/user/handler_test.go](server/internal/user/handler_test.go): happy path returns ordered `{data:[{id,username}]}` with self excluded; no-match returns `"data":[]`; missing/empty/whitespace `search` → `400 BAD_REQUEST`; missing auth → `401`.
  - [x] **(Optional / deferred perf)** No `pg_trgm`/functional index added — out of scope at Phase-1/2 scale, as specified. Decision recorded in Completion Notes.

- [x] **Task 2: Frontend — API client, query, debounce** (AC: #1, #3)
  - [x] Added `PlayerSearchResult` type in [client/src/shared/types/apiTypes.ts](client/src/shared/types/apiTypes.ts) — a distinct narrow shape (`id` + `username`), NOT the self `User` type.
  - [x] New API module [client/src/shared/api/users.ts](client/src/shared/api/users.ts): `searchUsers(search)` returns the unwrapped payload (no `.data.data` — the axios interceptor unwraps).
  - [x] Extended the key factory in [client/src/shared/api/queryKeys.ts](client/src/shared/api/queryKeys.ts): `users.search(query)`.
  - [x] Created [client/src/shared/hooks/useDebounce.ts](client/src/shared/hooks/useDebounce.ts) (`useDebounce<T>(value, delayMs=250)`) + co-located `useDebounce.test.ts` (fake-timers: initial value, delayed update, reset-on-rapid-change).
  - [x] Created [client/src/shared/hooks/queries/useUserSearch.ts](client/src/shared/hooks/queries/useUserSearch.ts): `useQuery` keyed on the trimmed debounced term, `enabled: trimmed.length >= 2`, `placeholderData: keepPreviousData` (v5 idiom imported from `@tanstack/react-query`).

- [x] **Task 3: Frontend — search UI + empty state + navigation** (AC: #3, #4, #5)
  - [x] Created [client/src/features/lobby/components/PlayerSearch.tsx](client/src/features/lobby/components/PlayerSearch.tsx): controlled `<input data-testid="player-search">`, conditional clear button (`data-testid="player-search-clear"`, i18n `aria-label`), owns raw `value` state → `useDebounce` → `useUserSearch`.
  - [x] Results list (each row a keyboard-accessible `<button data-testid="player-search-result" data-user-id>`); loading affordance (`player-search-loading`); empty state (`player-search-empty`) shown only when active (≥2 trimmed chars), settled, and `results.length === 0`, text `t("lobby.playerSearch.empty", { query })`.
  - [x] Result activation → `useNavigate()` → `navigate('/players/' + id)`.
  - [x] Mounted `<PlayerSearch />` in [client/src/features/lobby/LobbyPage.tsx](client/src/features/lobby/LobbyPage.tsx) as its own "Find players" card after `<FilterRail />`, with its own input state (not entangled with the room `search`).
  - [x] **Tests:** [client/src/features/lobby/components/PlayerSearch.test.tsx](client/src/features/lobby/components/PlayerSearch.test.tsx) — mocks `@/shared/api/users` + `useNavigate`; renders results after ≥2 chars; no `searchUsers` call for a 1-char query; interpolated empty state; result-click navigates to `/players/7`; clear resets the input.

- [x] **Task 4: i18n** (AC: #6)
  - [x] Added the `lobby.playerSearch` block (`label`, `placeholder`, `loading`, `empty`, `clear`, `resultAria`) to all four locales [en.json](client/src/shared/i18n/en.json)/[sr.json](client/src/shared/i18n/sr.json)/[mk.json](client/src/shared/i18n/mk.json)/[hr.json](client/src/shared/i18n/hr.json). `{{query}}`/`{{username}}` interpolation verified in all four; `mk` all-Cyrillic; no em dash in `mk`/`sr`/`hr`. `i18n.parity.test.ts` green.

- [x] **Task 5: Full validation gates** (AC: #6)
  - [x] `make lint` + `make test` equivalents green — server: `gofmt` clean (only the pre-existing `profile_identity_handler_test.go`), `go vet` clean, `golangci-lint v1.64.8` clean, `go test ./...` all packages ok with the DB-backed search test PASSING (not skipped) against dev DB `:5433`; client: `tsc -p tsconfig.build.json` clean, `vitest run` 104 files / 1128 tests, `eslint` clean, `prettier --check` clean. See Completion Notes re: the golangci-lint v1/v2 toolchain nuance.
  - [x] File List and Completion Notes updated.

## Dev Notes

### Scope & Sequencing Decision (READ FIRST)

- **This story delivers player search end-to-end** (backend endpoint + live-search lobby UI + empty state + result-click navigation), matching FR5 and the epic's three ACs.
- **AC5's navigation target `/players/:id` now EXISTS.** Story 11.3 (**done**, commit `0c4500e`) registered the `/players/:id` route + `PublicPlayerProfilePage` and made `GET /users/:id/profile` public (self → full `ProfileResponse`, any other viewer → the narrower `PublicProfileResponse`). 11.1 simply navigates there; it does NOT build the page or touch the endpoint.
- **Decision:** 11.1 wires the result click to `navigate('/players/' + id)` and verifies that wiring by asserting the navigation call in tests (mocked `useNavigate`). 11.1 does **not** register the `/players/:id` route or build the destination page, and does **not** widen the profile endpoint — that would be doing Story 11.3's work and would pull the honor/xp/stats public-profile surface into a search story.
- **Consequence:** the reorder was taken — **Story 11.3 shipped before 11.1**, so the `/players/:id` route and public-profile page already exist on this branch. 11.1's result-click navigates to a real, registered route; the earlier "dead-seam until 11.3 ships" caveat no longer applies. (Do NOT widen the profile endpoint or rebuild the page — 11.3 owns them.)
- **The endpoint DTO is intentionally minimal (`id` + `username`).** The epic AC says "returns matching usernames." Enriching results with honor/level chips (via `HonorService.HonorForUsers`, mirroring `RoomCard`) is a reasonable future enhancement but is **out of scope** here to keep the search story tight; note it in Completion Notes if you build the hook for it.

### Backend implementation notes

- **Where new code goes** (all in the existing `user` package + `main.go` — no new package, no new DI wiring; `UserHandler` already holds `userRepo`):
  | Concern | File | Anchor |
  |---|---|---|
  | Interface method | [server/internal/user/repository.go](server/internal/user/repository.go) | add to `UserRepository` |
  | Repo impl + `escapeLike` | [server/internal/user/gorm_repo.go](server/internal/user/gorm_repo.go) | after `FindByUsername` (~`:76`) |
  | Handler + DTO + limit const | [server/internal/user/handler.go](server/internal/user/handler.go) | mirror `ListMatches` (`:552`) / `parseMatchesQuery` (`:600`) |
  | Route | [server/cmd/api/main.go](server/cmd/api/main.go) | `api` group (~`:135`) |
  | Optional index migration | `server/migrations/000019_*` | pattern of `000002` (partial index) — **deferred** |
- **The closest precedent is `ListMatches` (same package)**, NOT room browse. Room "search" (Story 2-2) is client-side filtering over an already-fetched array — `RoomHandler.ListRooms` filters by `status` only, with **no `LIKE`/`?search=`** anywhere. This player-search introduces the **first** `ILIKE` query in the Go backend. Copy the query-param + validation + bounded-limit discipline from `parseMatchesQuery`.
- **Username storage is case-sensitive** `VARCHAR(20)` with a **partial** unique index (`CREATE UNIQUE INDEX idx_users_username ON users (username) WHERE deleted_at IS NULL`, migration `000002`). No `citext`, no collation. Case-insensitive search therefore needs `ILIKE` (or `LOWER(username) LIKE LOWER(?)`) and will not use the unique index — fine at current scale.
- **Wildcard-escape is mandatory** (AC2). `_` is a legal username character, so an un-escaped `_` in the query would act as a single-char wildcard and over-match. Escape `\`, `%`, `_` before interpolating into the `%...%` pattern. Verify the escape behavior in the repo test (Postgres `ILIKE` default escape char is `\`).
- **Response/error envelopes are inlined by handlers** (there are no envelope helper functions). Success: `c.JSON(http.StatusOK, map[string]interface{}{"data": items})`. Errors: just `return apperr.ErrBadRequest` — the central `appErrorHandler` (registered as `e.HTTPErrorHandler` in `main.go`) renders `{ "error": { "code", "message" } }` with the right status. Reuse the generic `apperr.ErrBadRequest` (400); do **not** invent a new error code for empty-query (no AC needs a distinct code). Wrap unexpected repo errors with `fmt.Errorf("searching users: %w", err)` (→ 500).
- **Auth context:** the route is on the authenticated `api` group, so `getUserID(c)` (reads `c.Get("userID")`, `handler.go:201`) yields the caller's id for self-exclusion. No token → the middleware already 401s before the handler runs; the handler can still defensively treat a missing id as `ErrUnauthorized`.
- **`json:"-"` honor raw columns** on the `User` model are never serialized — irrelevant to the search DTO, which is its own minimal struct. Do not return `User` directly.

### Frontend implementation notes

- **Client stack facts** (current, verified): API wrapper is [client/src/shared/api/axiosClient.ts](client/src/shared/api/axiosClient.ts) (NOT `fetchClient`), base URL `/api/v1`, response interceptor unwraps `{data}` (so api fns return the inner payload), request interceptor injects the `Authorization: Bearer` header, and 401→refresh→retry-once is handled centrally. Data fetching is **TanStack Query v5** everywhere (no manual `fetch`-in-`useEffect` for lists). Query keys are centralized in `queryKeys.ts`. Named exports only (`grep "export default"` → zero). Path alias `@/` → `client/src/`.
- **Live-search pattern** = the **UI shell** of the room search (`FilterRail` input + clear button; `RoomGrid` list + empty-state-with-testids) combined with the **data path** of `useProfile`/`useMatches` (query term in the key, `enabled`-gating). There is **no existing `useDebounce`** — create it (Task 2). Use `placeholderData: keepPreviousData` (v5) so results don't flash to empty between keystrokes.
- **`data-testid` naming** mirrors the room-search family (`room-list-search`, `room-list-empty`, `room-list-clear-search`): use `player-search`, `player-search-clear`, `player-search-result`, `player-search-empty`. Tests select by testid, never CSS classes.
- **Reusable profile UI (for context, not this story):** the redesigned honor band is `features/profile/components/HonorHeroBand.tsx` (props-driven, replaced the old `HonorPanel`) and the XP/level bar lives in `features/profile/components/IdentityHero.tsx` via `@/shared/components/XpBar`. Story 11.3 will reuse these for the public profile; 11.1 does not touch them.
- **`getProfile(userId)` in [client/src/shared/api/profile.ts](client/src/shared/api/profile.ts) already takes an id** and `ProfileResponse` already documents its honor fields as public-safe — but the endpoint is self-only server-side, so 11.3 must add a public endpoint before that client fn can be pointed at other users. Not this story.

### i18n notes

- Locale files: [client/src/shared/i18n/en.json](client/src/shared/i18n/en.json) `sr.json` `mk.json` `hr.json`; wired in `i18n.ts` (`SUPPORTED_LANGUAGES = ["en","hr","sr","mk"]`, fallback `en`). Add keys to **all four**. Follow the existing `lobby.search`/`lobby.empty` block conventions (`{{var}}` interpolation).
- Enforced rules: `i18n.parity.test.ts` fails on any missing/extra/empty leaf across locales. `mk` must be all-Cyrillic (proper nouns like "Beljot" stay Latin in every locale). No em dash (`—`) in `mk`/`sr`/`hr` (ad-hoc convention, no single global lint — the author must comply; use `…` for the placeholder ellipsis, which is fine in all locales).

### Testing standards summary

- **Go:** `testing` + `testify`. Two coexisting handler-test styles — (a) mock-repo + `httptest` request via `e.ServeHTTP` (see `mockUserRepo` + `setupUserHandler` in `handler_test.go`; real JWT via `auth.GenerateAccessToken`, `testErrorHandler` mirrors prod), and (b) DB-backed repo tests via `getTestDB` (per-test `tx.Begin()` + `t.Cleanup(tx.Rollback)`, DSN default dev DB `:5433`, **skips** if no DB). Use (b) for the `ILIKE` repo query (needs real Postgres), (a) for handler param-validation.
- **Client:** Vitest + Testing Library. Mock the api module with `vi.mock("@/shared/api/users")`; render with `QueryWrapper`/`TestProviders` + `BrowserRouter` from [client/src/test-utils.tsx](client/src/test-utils.tsx); fixtures via `makeUser`; assert by `data-testid` inside `waitFor`; present-tense `it(...)`. For the debounce hook use `vi.useFakeTimers()`.

### Known Traps (from project-context + recent Epic 9 stories)

- **Mock blast radius:** adding a method to the `UserRepository` interface forces every mock implementer to be updated or the build breaks. Grep the whole tree, not just the user package. (This exact cost was called out in `honor_service.go`; Epic 9 avoided it for read-only helpers by putting them on a concrete service — but a username query belongs on the repo, and the handler depends on the interface, so update the mocks.)
- **Never JS-truthiness numeric/bool from Go** (project rule): not relevant to `{id,username}` here, but keep in mind if you enrich results with `honorScore`/`level` later (`=== 0`, never `|| 80`).
- **Return `[]` not `null`:** initialize the Go result slice non-nil so no-match serializes as `[]` and the client's `.length === 0` empty-state check works without a null guard.
- **GORM default scope:** model-based queries auto-append `deleted_at IS NULL`; a raw `.Table("users")` query would bypass it and leak soft-deleted users. Use the model query.
- **One story = one branch = one PR.** If you spot an unrelated bug, file it in `deferred-work.md`, don't fix it here.

### Project Structure Notes

- New files: `server/internal/user/` (no new files — additions to existing `repository.go`/`gorm_repo.go`/`handler.go`/tests); `client/src/shared/api/users.ts`, `client/src/shared/hooks/useDebounce.ts` (+ test), `client/src/shared/hooks/queries/useUserSearch.ts`, `client/src/features/lobby/components/PlayerSearch.tsx` (+ test). Additions to `main.go`, `apiTypes.ts`, `queryKeys.ts`, `LobbyPage.tsx`, four i18n JSONs.
- All placements conform to the documented feature-folder + `shared/` conventions in `project-context.md` and `architecture.md` (§ frontend structure). No structural variance. Frontend API client files map 1:1 to backend domains — `users.ts` ↔ the `user` domain search route (note `profile.ts` already covers the self `/users/:id/profile`; the collection/search route is the natural home for a new `users.ts`).

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-11.1-Player-Search] — user story + 3 ACs (FR5).
- [Source: _bmad-output/planning-artifacts/epics.md#Epic-11] — Epic 11 objectives, FRs (FR5, FR6, FR47, FR61, FR62), Phase 3.
- [Source: _bmad-output/planning-artifacts/prd.md] — FR5 "Players can search for other players by username".
- [Source: _bmad-output/project-context.md] — TS/Go language rules, feature-folder structure, i18n rules, testing rules, naming conventions, API response formats.
- [Source: server/internal/user/handler.go] — `ListMatches`/`parseMatchesQuery` (query-param + bounded-limit precedent); `ProfileResponse` (self-only, public-safe honor fields); `GetProfile` self-only gate.
- [Source: server/internal/user/repository.go + gorm_repo.go] — `UserRepository` interface + `FindByUsername` exact-match precedent.
- [Source: server/cmd/api/main.go] — `api` authenticated group + `/users/:id/*` route registration site.
- [Source: server/internal/apperr/errors.go + main.go appErrorHandler] — error codes + central error envelope.
- [Source: server/migrations/000002_create_users.up.sql] — username `VARCHAR(20)` + partial unique index (case-sensitive).
- [Source: client/src/App.tsx] — router table (no `/players/:id` yet; `/profile` self-only).
- [Source: client/src/shared/api/axiosClient.ts] — wrapper, `{data}` unwrap, auth header, 401 refresh.
- [Source: client/src/shared/api/profile.ts + matches.ts + rooms.ts] — API-fn idiom, GET-with-params.
- [Source: client/src/shared/hooks/queries/useProfile.ts + useMatches.ts] — `useQuery` key + `enabled`-gating pattern.
- [Source: client/src/features/lobby/components/FilterRail.tsx + RoomGrid.tsx + LobbyPage.tsx] — search-UI shell, empty-state, testid conventions, mount point.
- [Source: client/src/shared/types/apiTypes.ts] — `User`/`RoomPlayer` types (why the search DTO must be a new narrow type).
- [Source: client/src/shared/i18n/*.json + i18n.parity.test.ts] — locale files + parity/quality enforcement.
- [Source: client/src/test-utils.tsx] — `makeUser`, `QueryWrapper`, `TestProviders`.

## Dev Agent Record

### Agent Model Used

Opus 4.8 (claude-opus-4-8) — BMad dev-story workflow.

### Debug Log References

- `go vet ./...` was used as the compiler-driven way to find the full `user.UserRepository` mock blast radius after adding `SearchByUsername` (it typechecks test files, unlike `go build`). It named exactly five broken mocks: `fakeHonorRepo`, `fakeLevelRepo`, `auth.mockUserRepo`, `chat.userRepoStub`, `lobby.fakeUserRepo`. `match.stubHonorRecorder` was correctly untouched (it satisfies the narrow `match.HonorRecorder`).
- The DB-backed repo test was confirmed to RUN (not skip) with `go test -run TestGormUserRepository_SearchByUsername -v` against dev DB `:5433` — all 6 subtests executed, including the literal-`_` and literal-`%` wildcard-escape probes.

### Completion Notes List

**What shipped:** player search end-to-end — `GET /api/v1/users?search=` (case-insensitive `ILIKE '%q%'`, self + soft-deleted excluded, ordered, capped at 10) → a live-search "Find players" card in the lobby (debounced, ≥2-char gate, empty state, result-click navigation to the existing `/players/:id` public profile from Story 11.3).

**Key decisions / notes:**

- **Wildcard escaping (AC2)** uses a single-pass `strings.NewReplacer` (`\`→`\\`, `%`→`\%`, `_`→`\_`) plus an explicit `ILIKE ? ESCAPE '\'`. Postgres' default LIKE escape is already `\`, but stating it keeps the escaper/clause pairing self-documenting. Verified in the DB test: a `car_ol` query matches only the literal `car_ol` (not `carXol`), and a `%` query returns only usernames literally containing `%` (not everyone).
- **No `pg_trgm`/functional index** was added — intentionally out of scope at Phase-1/2 scale (≤50 concurrent, small users table), exactly as the story's optional/deferred item states. If ever justified by a benchmark it is migration `000019_*` (last shipped is `000018`).
- **DTO is intentionally minimal (`id` + `username`).** No honor/level enrichment — kept the search story tight, per the Scope note. The endpoint does NOT widen the `/users/:id/profile` surface (Story 11.3 owns that).
- **`data-testid="player-search-loading"`** was added beyond the story's named testids for the loading affordance the task calls for — harmless additive test hook.
- **Toolchain note (golangci-lint):** the only `golangci-lint` on this machine is v2.12.2 (mise-managed), which cannot parse the repo's v1-format `server/.golangci.yml` ("unsupported version of the configuration"). This is a pre-existing repo-wide environment mismatch, NOT caused by this change. To actually satisfy AC6's lint gate I installed the project-pinned **v1.64.8** into a scratch `GOBIN` (`go install …@v1.64.8`) and ran it — it passes clean on the whole server module. `go vet` + `gofmt` are also clean. No repo config was modified (migrating `.golangci.yml` to v2 is out of scope).

**Gates:** server — gofmt clean (except the pre-existing `profile_identity_handler_test.go`), `go vet` clean, `golangci-lint v1.64.8` clean, `go test ./...` all packages ok (DB-backed search test passing, dev DB `:5433`). client — `tsc` clean, `vitest run` 104 files / 1128 tests, `eslint` clean, `prettier --check` clean, i18n 4-locale parity green (mk all-Cyrillic, no em dash in mk/sr/hr).

**Not done (stated, not implied):** no manual E2E was run (all gates above are automated). The `GET /users` search endpoint is now a newly-public authenticated read with no rate limiting — the same platform-wide enumeration/rate-limit concern deferred by Story 11.3's review applies here and remains a deferred, infra-level item, not this story's scope.

### File List

**Backend (server/):**

- `internal/user/repository.go` — added `SearchByUsername` to the `UserRepository` interface (documented).
- `internal/user/gorm_repo.go` — implemented `SearchByUsername` + `escapeLike`/`likeEscaper` helpers.
- `internal/user/handler.go` — added `PlayerSearchResult` DTO, `const searchResultLimit = 10`, `SearchUsers` handler; added `strings` import.
- `cmd/api/main.go` — registered `api.GET("/users", userHandler.SearchUsers)`.
- `internal/user/user_test.go` — added DB-backed `TestGormUserRepository_SearchByUsername` (6 subtests).
- `internal/user/handler_test.go` — added `mockUserRepo.SearchByUsername`, registered the `/users` route in the test harness, added `doSearchUsers` + `TestSearchUsers_*` (success/no-match/bad-request/missing-auth).
- `internal/user/honor_service_test.go` — added `fakeHonorRepo.SearchByUsername` stub.
- `internal/user/xp_service_test.go` — added `fakeLevelRepo.SearchByUsername` stub.
- `internal/auth/handler_test.go` — added `mockUserRepo.SearchByUsername` stub.
- `internal/chat/handler_test.go` — added `userRepoStub.SearchByUsername` stub.
- `internal/lobby/lobby_test.go` — added `fakeUserRepo.SearchByUsername` stub.

**Frontend (client/src/):**

- `shared/types/apiTypes.ts` — added `PlayerSearchResult` interface.
- `shared/api/users.ts` — NEW: `searchUsers(search)` API module.
- `shared/api/queryKeys.ts` — added `users.search(query)` key.
- `shared/hooks/useDebounce.ts` — NEW: generic `useDebounce` hook.
- `shared/hooks/useDebounce.test.ts` — NEW: fake-timer tests.
- `shared/hooks/queries/useUserSearch.ts` — NEW: live-search query hook.
- `features/lobby/components/PlayerSearch.tsx` — NEW: search UI + empty/loading states + navigation.
- `features/lobby/components/PlayerSearch.test.tsx` — NEW: component tests.
- `features/lobby/LobbyPage.tsx` — mounted `<PlayerSearch />`.
- `shared/i18n/en.json`, `sr.json`, `mk.json`, `hr.json` — added the `lobby.playerSearch` block.

**Story spec (this file):** added `baseline_commit` frontmatter; checked off Tasks 0–5; Status → review.

## Change Log

- **2026-08-14 — Implemented Story 11.1 Player Search (dev-story).** Backend `GET /users?search=` (ILIKE + wildcard-escape, self/soft-delete excluded, capped at 10), live-search lobby UI (`useDebounce` + TanStack v5 `keepPreviousData`, ≥2-char gate, empty state, `/players/:id` navigation), i18n ×4. All gates green (server: `go vet` / `gofmt` / `golangci-lint v1.64.8` / `go test ./...` incl. the DB-backed repo test; client: `tsc` / `vitest` 1128 / `eslint` / `prettier` / i18n parity). Status → review.

## Senior Developer Review — bmad-code-review (2026-08-14)

3-layer adversarial pass (Blind Hunter + Edge Case Hunter + Acceptance Auditor — all three green, all at Opus 4.8). All six ACs verified independently against the real code (not the story checkboxes): AC1/AC2/AC3/AC5/AC6 SATISFIED; AC4 satisfied in substance (correct `data-testid`, localization, `{{query}}` interpolation). Wildcard-escape (`\ % _` + `ESCAPE '\'`), GORM soft-delete default scope, `[]`-not-`null` serialization, the full `UserRepository` mock blast-radius (6 mocks), and i18n parity (mk all-Cyrillic, no em dash in mk/sr/hr) were all confirmed correct. 3 findings kept, 3 dismissed as noise (query-length cap — negligible, auth-gated + result-capped; case-sensitive `ORDER BY username` — collation-dependent, AC1 literally satisfied; en empty-state double-quote glyph vs AC4's single-quote notation — locale-appropriate quoting is deliberate).

### Review Findings

- [x] [Review][Decision] No error affordance for a failed search — On a search request error (500 / network drop) the component renders no error state; with `placeholderData: keepPreviousData` a prior term's results can even stay visible as if they matched the new query. Consistent with the codebase's read-query convention (no inline error UI; app-level Error Boundary + `toast.error` for mutations). [client/src/features/lobby/components/PlayerSearch.tsx:32] — **RESOLVED 2026-08-14: accepted as-is (matches the established read-query convention; low-frequency), dismissed.**
- [x] [Review][Patch] Stale results linger after Clear / narrowing below 2 chars [client/src/features/lobby/components/PlayerSearch.tsx:67] — **FIXED 2026-08-14: gated the results list on `isActive && results.length > 0` (matches the loading/empty affordances). Added 2 regression tests (clear + narrow-below-2), each proven to fail without the fix. Gates green: vitest 7/7, eslint, prettier, tsc.**
- [x] [Review][Defer] Public authenticated search endpoint has no rate-limiting / enumeration guard [server/internal/user/handler.go] — deferred, pre-existing (platform-wide infra item, already tracked from the Story 11.3 review)
