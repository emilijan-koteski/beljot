# Story 11.1: Player Search

Status: ready-for-dev

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

- [ ] **Task 0: Branch setup — whole-epic-on-one-branch**
  - [ ] **Continue on the current branch `feat/11-3-public-player-profiles`** (do NOT cut a new per-story branch). Per user direction, the remaining Epic 11 stories (11-1, 11-2, 11-4, 11-5) ship as ONE feature/PR on this branch — the same whole-scope-on-one-branch pattern used for 9.7+9.8. The branch already contains Epic 9 (honor, merged `dc14308`) + Story 11.3, so the `/players/:id` public-profile route this story navigates to already exists. This overrides the "one story = one branch = one PR" default for Epic 11 only, by explicit instruction.

- [ ] **Task 1: Backend — player search endpoint** (AC: #1, #2)
  - [ ] Add repository method `SearchByUsername(query string, excludeUserID uint, limit int) ([]User, error)` to the `UserRepository` interface in [server/internal/user/repository.go](server/internal/user/repository.go). Document it (case-insensitive substring, excludes self + soft-deleted, capped).
  - [ ] Implement it in `GormUserRepository` in [server/internal/user/gorm_repo.go](server/internal/user/gorm_repo.go): `r.db.Where("username ILIKE ?", "%"+escapeLike(query)+"%").Where("id <> ?", excludeUserID).Order("username ASC").Limit(limit).Find(&users)`. GORM's default scope already appends `deleted_at IS NULL` for model queries — do NOT use a raw `.Table(...)` query that would bypass it. Add an unexported `escapeLike(s string) string` helper that escapes `\`, `%`, and `_` (the query then uses the default `\` escape char, or an explicit `ESCAPE '\'` clause — verify the GORM/Postgres behavior in the repo test).
  - [ ] Update **every** `UserRepository` mock to implement the new method so the build compiles. Start with `mockUserRepo` in [server/internal/user/handler_test.go](server/internal/user/handler_test.go); then grep the tree for other `UserRepository` mock implementers (`grep -rl "UserRepository" server/internal --include=*_test.go` and any non-test mock) and add the stub to each. This mock blast radius is a known cost of touching the shared interface — see Dev Notes → **Known Traps**.
  - [ ] Add `SearchUsers(c echo.Context) error` handler in [server/internal/user/handler.go](server/internal/user/handler.go), modeled on `ListMatches`/`parseMatchesQuery` conventions (`handler.go:552`, `:600`): read `c.QueryParam("search")`, `strings.TrimSpace`, if empty → `return apperr.ErrBadRequest`; resolve caller id via `getUserID(c)`; call the repo with a package-level `const searchResultLimit = 10`; map results to a minimal DTO `PlayerSearchResult{ ID uint \`json:"id"\`; Username string \`json:"username"\` }`; return `c.JSON(http.StatusOK, map[string]interface{}{"data": items})`. Return a non-nil empty slice (`[]PlayerSearchResult{}`) not `nil`, so the JSON is `[]` not `null`.
  - [ ] Register the route in [server/cmd/api/main.go](server/cmd/api/main.go) inside the authenticated `api` group (around `main.go:135`, alongside the `/users/:id/*` routes): `api.GET("/users", userHandler.SearchUsers)`. Confirmed no path collision with `GET /users/:id/profile` in Echo. It inherits `auth.AuthMiddleware` automatically.
  - [ ] **Tests (RED first):**
    - [ ] Repo DB-backed test in [server/internal/user/user_test.go](server/internal/user/user_test.go) using the `getTestDB` tx-rollback helper (DSN defaults to dev DB `:5433`, test skips if DB unavailable): assert case-insensitive match (`"ali"` finds `"Alice"`), substring match, self-exclusion, soft-deleted exclusion, wildcard-escape (a seeded username containing `_` is matched literally and a `%` query does not match everyone), ordering, and the limit cap.
    - [ ] Handler test in [server/internal/user/handler_test.go](server/internal/user/handler_test.go) using `mockUserRepo` + `setupUserHandler` + a real JWT: empty/whitespace `search` → `400`; missing param → `400`; happy path returns `{data:[{id,username}]}` shape; no-match returns `{data:[]}`; unauthenticated (no token) → `401`.
  - [ ] **(Optional / deferred perf)** A `pg_trgm` GIN or `LOWER(username)` functional index for substring search is **not required** at Phase-1/2 scale (≤50 concurrent, small users table) and is intentionally out of scope. If added later it is migration `000019_*` (last shipped migration is `000018`) with matching `.up.sql`/`.down.sql`. Do not add it in this story unless a benchmark justifies it — note the decision in Completion Notes.

- [ ] **Task 2: Frontend — API client, query, debounce** (AC: #1, #3)
  - [ ] Add `PlayerSearchResult` type: `export interface PlayerSearchResult { id: number; username: string }` in [client/src/shared/types/apiTypes.ts](client/src/shared/types/apiTypes.ts) (named export, alongside `User`/`RoomPlayer`). Do NOT reuse the self `User` type — it carries `email` and is not a public/search shape.
  - [ ] New API module [client/src/shared/api/users.ts](client/src/shared/api/users.ts): `export function searchUsers(search: string): Promise<PlayerSearchResult[]> { return axiosClient.get("/users", { params: { search } }); }`. Return the unwrapped payload directly — the axios response interceptor already unwraps `{data}`; never reference `.data.data`.
  - [ ] Extend the key factory in [client/src/shared/api/queryKeys.ts](client/src/shared/api/queryKeys.ts): `users: { search: (q: string) => ["users", "search", q] as const }`.
  - [ ] Create a reusable debounce hook [client/src/shared/hooks/useDebounce.ts](client/src/shared/hooks/useDebounce.ts) — `export function useDebounce<T>(value: T, delayMs = 250): T` (standard `useState` + `useEffect` + `setTimeout`/cleanup). None exists today; this is the first one. Co-locate a small `useDebounce.test.ts` using `vi.useFakeTimers()`.
  - [ ] Create query hook [client/src/shared/hooks/queries/useUserSearch.ts](client/src/shared/hooks/queries/useUserSearch.ts): `useQuery({ queryKey: queryKeys.users.search(debounced), queryFn: () => searchUsers(debounced), enabled: debounced.trim().length >= 2, placeholderData: keepPreviousData })`. Import `keepPreviousData` from `@tanstack/react-query` (v5 idiom — NOT the v4 `keepPreviousData: true` option). Gate `enabled` so no request fires below 2 trimmed chars.

- [ ] **Task 3: Frontend — search UI + empty state + navigation** (AC: #3, #4, #5)
  - [ ] Create `features/lobby/components/PlayerSearch.tsx` mirroring the controlled-input + clear-button pattern of [client/src/features/lobby/components/FilterRail.tsx](client/src/features/lobby/components/FilterRail.tsx). Controlled `<input>` with `data-testid="player-search"`, localized placeholder, a conditional clear button `data-testid="player-search-clear"` (`aria-label` from i18n). Owns the raw input `value` state; pass it through `useDebounce` before feeding `useUserSearch`.
  - [ ] Render results below the input (mirror the list/empty-state split of [client/src/features/lobby/components/RoomGrid.tsx](client/src/features/lobby/components/RoomGrid.tsx)): a loading affordance while the query is fetching, a results list where each row is a `<button>` (keyboard-accessible) showing the username with `data-testid="player-search-result"` (and e.g. `data-user-id` for tests), and the empty state `data-testid="player-search-empty"` shown only when the (debounced) query is non-empty, the query has settled, and `results.length === 0`. Empty text: `t("lobby.playerSearch.empty", { query })`.
  - [ ] Result activation → `const navigate = useNavigate();` then `navigate('/players/' + id)`. (Route/page owned by Story 11.3 — see Scope & Sequencing Decision.)
  - [ ] Mount `<PlayerSearch />` in [client/src/features/lobby/LobbyPage.tsx](client/src/features/lobby/LobbyPage.tsx) as its own section (natural placement: a sibling block after `<FilterRail />`, or a clearly-labeled "Find players" area). Do not entangle it with the room `search` state — player search has its own input state and its own network-backed query.
  - [ ] **Tests:** component tests co-located (`PlayerSearch.test.tsx`) using `vi.mock("@/shared/api/users")`, rendered with `QueryWrapper` + `BrowserRouter` (or `TestProviders`), `makeUser`-style fixtures for results: renders results after typing ≥2 chars; asserts no `searchUsers` call for a 1-char query; renders the interpolated empty state; clicking a result calls a mocked `useNavigate` with `/players/<id>`; clear button resets the input. Use present-tense `it(...)` descriptions and `data-testid` selectors.

- [ ] **Task 4: i18n** (AC: #6)
  - [ ] Add a `lobby.playerSearch` block to all four locale files [client/src/shared/i18n/en.json](client/src/shared/i18n/en.json), [sr.json](client/src/shared/i18n/sr.json), [mk.json](client/src/shared/i18n/mk.json), [hr.json](client/src/shared/i18n/hr.json). Suggested keys: `label` (section heading, e.g. "Find players"), `placeholder` ("Search players by username…"), `empty` ("No players found matching \"{{query}}\""), `clear` (clear-button aria-label, "Clear search"), `resultAria` (e.g. "View {{username}}'s profile"). Use `{{query}}`/`{{username}}` interpolation exactly. `mk` values ALL-Cyrillic; NO em dash in `mk`/`sr`/`hr` (use `…` for ellipsis is fine; avoid `—`). The `i18n.parity.test.ts` enforces 1:1 leaf parity + non-empty values across all four locales.

- [ ] **Task 5: Full validation gates** (AC: #6)
  - [ ] `make lint` (Go + client) and `make test` (`go test ./...` + `npx vitest run`) green. Confirm the DB-backed repo test passes (not skips) against dev DB `:5433`, or explicitly note in Completion Notes if the DB was unavailable in the dev environment and the test skipped.
  - [ ] Update the File List and Completion Notes.

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

_TBD by dev-story_

### Debug Log References

### Completion Notes List

- Ultimate context engine analysis completed — comprehensive developer guide created.

### File List
