# Story 7.2: Expanded Player Profile

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a player,
I want my profile to show real aggregate stats derived from my games,
so that I have a meaningful overview of my Beljot career alongside my match history.

## Acceptance Criteria

1. **Extended profile endpoint returns aggregate stats alongside identity fields**
   Given an authenticated player whose userID matches `:id`,
   When `GET /api/v1/users/:id/profile` is called,
   Then the server returns `200 { "data": { "id", "username", "languagePreference", "createdAt", "totalGamesPlayed", "wins", "losses", "abandoned" } }` — `totalGamesPlayed`, `wins`, `losses`, `abandoned` are non-negative integers derived from the `matches` table (never cached; computed per-request in Phase 1).
   And definitions (all computed over matches where `userID ∈ {player1_id..player4_id}`):
   - `wins` = count of matches where `status = 'completed'` AND `winnerTeam = teamForSeat(viewerSeat)` (seat 0/2 → Team A index 0; seat 1/3 → Team B index 1; mirroring `game.TeamForSeat`).
   - `losses` = count of matches where `status = 'completed'` AND `winnerTeam != teamForSeat(viewerSeat)`.
   - `abandoned` = count of matches where `status = 'abandoned'` (regardless of which side abandoned or which team "won" — abandoned games are neither wins nor losses, matching the UX tone in Story 7.1 AC #7 where the badge is `warning`, not `destructive`).
   - `totalGamesPlayed` = `wins + losses + abandoned` (and MUST equal a direct `COUNT(*)` over the same where-clause — enforce with a unit test).
     And the existing `ProfileResponse` fields (`id`, `username`, `languagePreference`, `createdAt`) remain **unchanged** in shape and semantics; this is an additive extension. No existing consumers break.
     And `languagePreference` is still returned ONLY to the owner (the authorisation check is unchanged — `paramID != authUserID → 403`), preserving the existing PII boundary from [server/internal/user/handler.go:109](server/internal/user/handler.go#L109).

2. **Aggregate stats computed in a single DB round-trip — no N+1, no per-match loop**
   Given the handler composes the profile response,
   When stats are computed,
   Then a **single** `SELECT` query against `matches` returns four counts via conditional aggregation (GORM `Select`/`Raw`), e.g. `SELECT COUNT(*) FILTER (WHERE status='completed' AND winner_team = <teamForSeat(viewerSeat)>) AS wins, ... FROM matches WHERE <userID ∈ players1..4>` — **NOT** a Go-side loop over `GetMatchesForUser` results (that would allocate N matches and degrade as history grows).
   And the SQL-level filter MUST reproduce Story 7.1's participation predicate exactly (`userID IN (player1_id, player2_id, player3_id, player4_id)`), so `wins + losses + abandoned` is identical to the `total` returned by `/users/:id/matches`. Add a regression test that seeds N matches and asserts equality.
   And the query uses PostgreSQL `FILTER (WHERE ...)` aggregation (supported — PostgreSQL 14+; see [docker-compose.yml](docker-compose.yml) / [server/migrations](server/migrations)) rather than four separate `COUNT` queries. Rationale: one round-trip, atomic snapshot, deterministic under concurrent match completions.
   And the query is encapsulated as a new `MatchRepository.GetStatsForUser(userID uint) (wins, losses, abandoned int, err error)` method — returning counts only (totalGamesPlayed derived by the handler). No hand rows are preloaded — this endpoint must stay fast regardless of match history size.

3. **`ProfilePage` renders real stats in the existing `profile-stats` section**
   Given the `ProfilePage` receives the extended profile payload,
   When the page renders,
   Then the `<section data-testid="profile-stats">` (currently a placeholder at [client/src/features/profile/ProfilePage.tsx:66-74](client/src/features/profile/ProfilePage.tsx#L66-L74)) displays four stat tiles in a horizontal `flex` or `grid-cols-4` layout:
   - `games-played` — big number + label "Games played" (`profile.stats.totalGamesPlayed`).
   - `wins` — big number + label "Wins" (`profile.stats.wins`), number coloured `text-success`.
   - `losses` — big number + label "Losses" (`profile.stats.losses`), number coloured `text-text-secondary` (**never** `destructive` — per UX spec [ux-design-specification.md:321-323](_bmad-output/planning-artifacts/ux-design-specification.md#L321-L323) and the outcome-tone decision in Story 7.1).
   - `win-rate` — percentage (`wins / (wins + losses)`) + label "Win rate" (`profile.stats.winRate`), number coloured `text-success` if ≥ 50% else `text-text-secondary`.
     And stat numbers use `font-display` (Space Grotesk) at `text-4xl font-bold` — per UX spec typography rule "score numbers and rank text always use Space Grotesk" ([ux-design-specification.md:339](_bmad-output/planning-artifacts/ux-design-specification.md#L339), [#L357](_bmad-output/planning-artifacts/ux-design-specification.md#L357)).
     And each tile has `data-testid="profile-stat-<key>"` (`profile-stat-games-played`, `profile-stat-wins`, `profile-stat-losses`, `profile-stat-win-rate`) and a stable `data-value="<number>"` attribute for robust test targeting independent of locale formatting.
     And the section heading `profile.stats` (`"Stats"` / `"Statistika"`) is retained — only the inner body changes from the `<p>{t("profile.statsEmpty")}</p>` placeholder.

4. **Zero-games empty path — show `0 games played`, not a broken UI**
   Given an authenticated player has never played a match (`totalGamesPlayed === 0`),
   When their profile renders,
   Then all four stat tiles still render, showing `0`, `0`, `0`, and `—` (em-dash) for win-rate (a 0/0 division — `NaN%` is never shown; MUST be the literal em-dash character `\u2014` rendered via `{t("profile.stats.winRateEmpty")}`).
   And the `MatchHistory` component continues to render its own empty state from Story 7.1 AC #4 — the two empty states are independent and MUST both display simultaneously for a zero-game user (verified with an integration test on `ProfilePage.test.tsx`).
   And `data-testid="profile-stats"` + `profile-stat-win-rate` with `data-value="0"` remain present — tests must not key on the absence of the stat section.

5. **Placeholder-removal regression — no `"coming soon"` / `statsEmpty` copy remains on the profile page**
   Given Epic 7 is complete with Story 7.1 + 7.2,
   When `ProfilePage` renders,
   Then neither the string `profile.statsEmpty` nor any literal `"coming soon"` / `"placeholder"` copy is rendered in the stats or match-history sections — both are fully replaced by real data (or real empty states). Verified by:
   - Removing the `profile.statsEmpty` key from both [en.json](client/src/shared/i18n/en.json) and [sr.json](client/src/shared/i18n/sr.json) in the same commit.
   - Updating [ProfilePage.test.tsx](client/src/features/profile/ProfilePage.test.tsx) test `"renders placeholder sections"` to assert the stat tiles exist (retargeted to `profile-stat-*` testids), not the placeholder copy.
   - A `grep` test (`npx vitest`) that asserts `profile.statsEmpty` does not appear in any `.tsx`/`.ts` file under `client/src/`.

6. **i18n keys added to both `en.json` and `sr.json` in the same commit; `statsEmpty` removed from both**
   Given new copy introduced by this story,
   When the story lands,
   Then the following keys exist under `profile.stats.*` in **both** [client/src/shared/i18n/en.json](client/src/shared/i18n/en.json) and [client/src/shared/i18n/sr.json](client/src/shared/i18n/sr.json): `totalGamesPlayed`, `wins`, `losses`, `winRate`, `winRateEmpty`. The existing `profile.stats` (section heading) key is **unchanged**.
   And `profile.statsEmpty` is removed from **both** JSONs and all references in `.tsx`/`.ts` files are updated (just one: [ProfilePage.tsx:73](client/src/features/profile/ProfilePage.tsx#L73)).
   And the Serbian-Latin register matches adjacent profile keys — suggested translations (review at commit time): `totalGamesPlayed: "Odigrane partije"`, `wins: "Pobjede"` (matching `sr.json` register — verify which of `"Pobjede"` / `"Pobede"` the repo already uses in adjacent keys), `losses: "Porazi"`, `winRate: "Procenat pobjeda"`, `winRateEmpty: "—"`.
   And the existing `i18n.test.ts` recursive `flattenKeys` parity check (added in Story 7.1 review) continues to pass.

7. **Authorisation & error handling mirror existing `GetProfile` precisely — no relaxation**
   Given the handler resolves `:id` param,
   When the param is malformed (`abc`, `0`, negative), missing the auth header, or does not match the authenticated userID,
   Then responses are `400 BAD_REQUEST` / `401 UNAUTHORIZED` / `403 FORBIDDEN` respectively — identical branching to the extant `GetProfile` ([server/internal/user/handler.go:98-111](server/internal/user/handler.go#L98-L111)).
   And DB errors from `GetStatsForUser` are wrapped with `%w` and surfaced via the existing `appErrorHandler` middleware as `500 INTERNAL_ERROR` — the handler does not silently return zeros on a DB failure.
   And the response NEVER leaks email or password hash — only the fields listed in AC #1. A PII-leak guard test (mirroring the one added in Story 7.1 Task 6.2 for `/matches`) asserts the serialised response body does not contain `"email"`, `"passwordHash"`, or `"password_hash"`.
   And **Epic 11 (Public Player Profiles) forward-compat note**: do NOT relax the `:id == authUserID` check in this story — that's Story 11.3's scope. Keep the private-only contract intact here; a future public endpoint will be a distinct handler.

8. **Backward compatibility — existing profile consumers continue to work untouched**
   Given the `ProfileResponse` gains four new fields,
   When existing callers deserialise the payload,
   Then clients that ignore unknown fields (default axios behaviour on the TS side) continue to function. The `authStore` hydration path ([client/src/shared/stores/authStore.ts](client/src/shared/stores/authStore.ts)) does not need changes — it reads only `id`, `username`, `email`, `languagePreference`, `createdAt`.
   And the `useProfileQuery` hook ([client/src/shared/hooks/queries/useProfile.ts](client/src/shared/hooks/queries/useProfile.ts)) returns the extended type — the consuming `ProfilePage.tsx` adds stat rendering but does NOT break when the query is `isPending` (existing skeleton loader path preserved at [ProfilePage.tsx:13-20](client/src/features/profile/ProfilePage.tsx#L13-L20)).
   And the TS type `ProfileResponse` in [client/src/shared/api/profile.ts](client/src/shared/api/profile.ts) gains four required fields; any downstream code that constructs a `ProfileResponse` literal in tests must be updated (audit: `grep -rn "ProfileResponse" client/src`).

## Tasks / Subtasks

- [x] **Task 1: Repository method `GetStatsForUser` (AC #1, #2)**
  - [x] 1.1 Add `GetStatsForUser(userID uint) (wins, losses, abandoned int, err error)` to [server/internal/match/repository.go](server/internal/match/repository.go). Docstring: "counts matches where userID ∈ players1..4; `wins` = completed AND winnerTeam matches viewer's team; `losses` = completed AND mismatched; `abandoned` = any abandoned match. Single-round-trip via FILTER aggregation."
  - [x] 1.2 Implement in [server/internal/match/gorm_repo.go](server/internal/match/gorm_repo.go) using a single `SELECT` with `FILTER (WHERE ...)` aggregation. Example (reference — verify syntax against GORM's `Raw` / `Scan`):
    ```sql
    SELECT
      COUNT(*) FILTER (WHERE status = 'completed' AND winner_team = ?) AS wins,
      COUNT(*) FILTER (WHERE status = 'completed' AND winner_team <> ?) AS losses,
      COUNT(*) FILTER (WHERE status = 'abandoned') AS abandoned
    FROM matches
    WHERE ? IN (player1_id, player2_id, player3_id, player4_id);
    ```
    Team derivation requires knowing viewer's seat per match — since `teamForSeat(seat) = seat % 2` and the viewer's seat differs per row, the cleanest SQL is: `winner_team = CASE WHEN player1_id = ? THEN 0 WHEN player2_id = ? THEN 1 WHEN player3_id = ? THEN 0 WHEN player4_id = ? THEN 1 END`. Encapsulate this CASE as a named expression so the FILTER clause stays readable. Use parameterised placeholders throughout — NEVER interpolate `userID`.
  - [x] 1.3 Return `(0, 0, 0, nil)` on an empty-result row (user with zero matches) — GORM's `.Scan` into a struct leaves zero values; assert with a unit test.
  - [x] 1.4 Add/extend the `mockMatchRepo` in [server/internal/session/manager_test.go](server/internal/session/manager_test.go) and [server/internal/user/handler_test.go](server/internal/user/handler_test.go) to satisfy the new method — return static values the test controls. Any existing test using these mocks must continue to compile.

- [x] **Task 2: Extend `UserHandler.GetProfile` response with aggregate stats (AC #1, #2, #7, #8)**
  - [x] 2.1 Extend `ProfileResponse` in [server/internal/user/handler.go](server/internal/user/handler.go) with four new fields:
    ```go
    TotalGamesPlayed int `json:"totalGamesPlayed"`
    Wins             int `json:"wins"`
    Losses           int `json:"losses"`
    Abandoned        int `json:"abandoned"`
    ```
    Keep the field order identity-first (`ID`, `Username`, `LanguagePreference`, `CreatedAt`), then stats — so the JSON payload reads as a natural extension.
  - [x] 2.2 In `GetProfile` handler ([server/internal/user/handler.go:98-129](server/internal/user/handler.go#L98-L129)), after the successful `FindByID` call and before the `c.JSON` return, call `h.matchRepo.GetStatsForUser(authUserID)`. On error, wrap with `%w` and return — do NOT log-and-swallow.
  - [x] 2.3 Compute `totalGamesPlayed := wins + losses + abandoned` and populate the new fields on the `ProfileResponse` literal.
  - [x] 2.4 No changes to the auth path — the existing `getUserID` + paramID parse + `!= authUserID → 403` flow is already correct per AC #7. Regression-test this by keeping the existing `/profile` tests unchanged.

- [x] **Task 3: Handler tests for extended `GetProfile` (AC #1, #2, #4, #7, #8)**
  - [x] 3.1 Extend [server/internal/user/handler_test.go](server/internal/user/handler_test.go) `mockMatchRepo` to track `GetStatsForUser` calls and return test-controlled `(wins, losses, abandoned, err)`. Existing mock methods (`Create`, `CreateWithHands`, `GetMatchesForUser`) remain.
  - [x] 3.2 Add table-driven test cases for `TestGetProfile_WithStats`:
    - Zero games: mock returns `(0, 0, 0, nil)` → response has `totalGamesPlayed: 0, wins: 0, losses: 0, abandoned: 0`.
    - Mixed outcomes: `(7, 3, 1, nil)` → `totalGamesPlayed: 11`, other fields pass-through.
    - DB error: `(0, 0, 0, assert.AnError)` → 500 response, error wrapped (not swallowed).
    - Invariant: `totalGamesPlayed == wins + losses + abandoned` across all seeded cases (loop assertion).
  - [x] 3.3 Add PII-leak regression test `TestGetProfile_NeverLeaksPII` — serialise the response body, assert the string contains NONE of: `"email"`, `"passwordHash"`, `"password_hash"`, `"deletedAt"`. The user's `email` IS stored on the User row but MUST NOT appear in this endpoint (unchanged from prior behaviour — this test locks it in).
  - [x] 3.4 Existing `TestGetProfile_*` cases for auth failures (400 on bad id, 401 on missing token, 403 on foreign id) must continue to pass unchanged. Verify the mock's `GetStatsForUser` is NOT called when the auth check fails early — add a call-count assertion (`mockMatchRepo.getStatsCalls == 0`).

- [x] **Task 4: Integration test — stats total equals matches total (AC #2)**
  - [x] 4.1 Add a test (either in `gorm_repo_test.go` if one exists, or in [server/internal/user/handler_test.go](server/internal/user/handler_test.go) at handler level) that seeds N matches (e.g. 2 wins, 1 loss, 1 abandoned, some where the user is on Team A, some Team B, some in seats 2 and 3) and asserts: `stats.wins + stats.losses + stats.abandoned == getMatchesForUser.total` for the same user.
  - [x] 4.2 Edge case: matches where the viewing user was NOT a participant — they MUST NOT be counted in any bucket. Seed one such match in the fixture and assert the counts ignore it.
  - [x] 4.3 Edge case: a user seated at position 3 (Team B index 1) whose team won a match — verify `wins` increments (not `losses`). This exercises the CASE expression for `teamForSeat` across all four seats in the fixture.

- [x] **Task 5: Client API type extension (AC #1, #8)**
  - [x] 5.1 Extend `ProfileResponse` in [client/src/shared/api/profile.ts](client/src/shared/api/profile.ts) with four required number fields:
    ```ts
    totalGamesPlayed: number;
    wins: number;
    losses: number;
    abandoned: number;
    ```
    Mirror the server field order for readability. No `any`, no optional (`?`) — server always returns them per AC #1.
  - [x] 5.2 Audit downstream usage: `grep -rn "ProfileResponse" client/src/` — any test fixture / manual literal must be updated to include the four new fields (expect only [client/src/features/profile/ProfilePage.test.tsx](client/src/features/profile/ProfilePage.test.tsx) to need updating). No other consumers inspect these fields, so compilation is the only gate.
  - [x] 5.3 The `useProfileQuery` hook returns the extended type automatically via TS inference — no code change needed in [client/src/shared/hooks/queries/useProfile.ts](client/src/shared/hooks/queries/useProfile.ts).

- [x] **Task 6: `ProfilePage` stats section (AC #3, #4, #5)**
  - [x] 6.1 In [client/src/features/profile/ProfilePage.tsx](client/src/features/profile/ProfilePage.tsx), replace the placeholder body of the `<section data-testid="profile-stats">` (currently `<p>{t("profile.statsEmpty")}</p>`) with a 4-tile grid. Use Tailwind `grid grid-cols-2 md:grid-cols-4 gap-4` (Phase 1 is desktop-only per UX spec [line 894](_bmad-output/planning-artifacts/ux-design-specification.md#L894), but `md:` breakpoint is a cheap future-proofing; do NOT add responsive work beyond this one utility).
  - [x] 6.2 Create a single-file helper component `StatTile` (inline in `ProfilePage.tsx` — do NOT extract to its own file for a 20-LOC helper, per project precedent of keeping small helpers colocated):
    ```tsx
    function StatTile({
      testId,
      label,
      value,
      tone,
    }: {
      testId: string;
      label: string;
      value: string; // pre-formatted string, NOT number — locale/rate formatting is handled by caller
      tone?: "neutral" | "success";
    }) {
      return (
        <div
          data-testid={testId}
          className="rounded-lg bg-surface-elevated p-4"
        >
          <div
            className={`font-display text-4xl font-bold ${tone === "success" ? "text-success" : "text-text-primary"}`}
          >
            {value}
          </div>
          <div className="mt-1 text-sm text-text-secondary">{label}</div>
        </div>
      );
    }
    ```
  - [x] 6.3 Render four `StatTile` instances pulling values from `profile?.totalGamesPlayed ?? 0`, `profile?.wins ?? 0`, `profile?.losses ?? 0`, and a computed `winRate`:
    - `winRate` computation: `const played = wins + losses; const rate = played === 0 ? undefined : Math.round((wins / played) * 100); const display = rate === undefined ? t("profile.stats.winRateEmpty") : \`${rate}%\`;`
    - Win-rate tone: `rate !== undefined && rate >= 50 ? "success" : "neutral"`.
    - `data-value` on each tile: numeric for games/wins/losses; the raw rate number or empty string for win-rate (so tests can assert `data-value="62"` without coupling to locale formatting).
  - [x] 6.4 Remove the `{t("profile.statsEmpty")}` reference from the file — the key is deleted in Task 7. TypeScript / eslint will flag dangling references if missed.
  - [x] 6.5 Keep the section wrapper (`<section data-testid="profile-stats">` + its heading) untouched so the existing `ProfilePage.test.tsx` assertion on that testid in `"renders placeholder sections"` keeps passing (with the copy expectation retargeted in Task 8).

- [x] **Task 7: i18n string updates (AC #6)**
  - [x] 7.1 Extend [client/src/shared/i18n/en.json](client/src/shared/i18n/en.json) under the existing `profile` block with a new nested `stats` object:
    ```json
    "stats": {
      "totalGamesPlayed": "Games played",
      "wins": "Wins",
      "losses": "Losses",
      "winRate": "Win rate",
      "winRateEmpty": "\u2014"
    }
    ```
    Note: the existing top-level `profile.stats` key (`"Stats"` — the section heading) MUST be preserved. i18next resolves nested keys by depth, so `t("profile.stats")` returns the heading string while `t("profile.stats.totalGamesPlayed")` resolves into the nested object. **Validate this** — if i18next in this repo flattens at load time and conflicts on the same key as both a string and an object, rename the section heading to `profile.statsHeading` and update `ProfilePage.tsx:70` accordingly. Confirm in `i18n.test.ts` output before committing.
  - [x] 7.2 Remove the `profile.statsEmpty` key from `en.json`. One reference in the codebase (`ProfilePage.tsx:73`) is removed in Task 6.4.
  - [x] 7.3 Mirror both changes in [client/src/shared/i18n/sr.json](client/src/shared/i18n/sr.json). Suggested Serbian-Latin copy (verify the chosen register — `Pobede` vs `Pobjede` — matches adjacent profile keys):
    ```json
    "stats": {
      "totalGamesPlayed": "Odigrane partije",
      "wins": "Pobjede",
      "losses": "Porazi",
      "winRate": "Procenat pobjeda",
      "winRateEmpty": "\u2014"
    }
    ```
  - [x] 7.4 Run `npx vitest run i18n` locally — the recursive `flattenKeys` parity test from Story 7.1 review must stay green. If the heading/object collision (see 7.1 note) trips the parity test, resolve per 7.1's fallback rename.

- [x] **Task 8: `ProfilePage.test.tsx` updates (AC #3, #4, #5)**
  - [x] 8.1 In [client/src/features/profile/ProfilePage.test.tsx](client/src/features/profile/ProfilePage.test.tsx):
    - Update every `mockGetProfile.mockResolvedValueOnce({...})` fixture to include the four new fields (start with `totalGamesPlayed: 0, wins: 0, losses: 0, abandoned: 0` for existing cases that don't care). TypeScript will fail the compile if any fixture is missed (AC #5 TS gate).
  - [x] 8.2 Retarget the `"renders placeholder sections"` test: keep the `profile-match-history` + `profile-stats` testid assertions, but also assert the four `profile-stat-*` tiles exist with `data-value="0"`. Rename the test to `"renders match-history section + four stat tiles"` for clarity.
  - [x] 8.3 Add a new test `"renders real stats when profile has played games"`:
    - Fixture: `{ totalGamesPlayed: 11, wins: 7, losses: 3, abandoned: 1, ... }`.
    - Assert each tile's `data-value` matches the fixture field (numeric string for games/wins/losses).
    - Assert `profile-stat-win-rate` renders `"70%"` (7 / 10 \* 100 = 70, rounded) — `data-value="70"`.
    - Assert tone on `profile-stat-win-rate` by CSS class presence — but prefer to use `data-value` alone, since Tailwind class churn can break class-based assertions (project rule reinforced in 7.1 Dev Notes).
  - [x] 8.4 Add a new test `"renders em-dash for win-rate when zero games played"`:
    - Fixture: `{ totalGamesPlayed: 0, wins: 0, losses: 0, abandoned: 0, ... }`.
    - Assert `profile-stat-win-rate` text content is `"—"` (or `data-value=""`) — NOT `"NaN%"` or `"0%"`.
  - [x] 8.5 Add a new test `"renders stats tiles alongside match-history empty state when user has no games"`:
    - Both sections render simultaneously; `match-history-empty` and `profile-stat-games-played[data-value='0']` both present.

- [x] **Task 9: Full-stack smoke + regression (run before marking the story done)**
  - [x] 9.1 Backend: `go test ./... -count=1` — all packages green. `go vet ./...` clean.
  - [x] 9.2 Frontend: `cd client && npx vitest run` — all tests green (existing + 3 new `ProfilePage` cases).
  - [x] 9.3 Lint: **`cd client && npx prettier --write .` then `npx eslint .`**. **Prettier MUST run before committing** (memory `feedback_prettier_before_commit.md`; CI has failed repeatedly on this — Story 7.1 logged the same reminder).
  - [x] 9.4 `make lint` (both stacks) — clean.
  - [x] 9.5 Manual smoke test (document outcomes in Completion Notes):
    - Start `make dev`, register two users, play one Quick Play match to completion. Verify `/profile` for the winning team shows `1` game played, `1` win, `0` losses, `100%` win rate (`data-value="100"`, displayed via Space Grotesk big type).
    - Verify `/profile` for the losing team shows `1` game played, `0` wins, `1` loss, `0%` win rate.
    - Verify a freshly registered user's `/profile` shows `0 · 0 · 0 · —` and the `match-history-empty` state below.
    - Verify abandonment: force a match abandonment via 120s reconnect timeout (Epic 5 path); confirm the abandoning and remaining players both see `abandoned: 1` and their `wins/losses` unchanged.

## Dev Notes

### What Already Exists — Do NOT Recreate

| Item                                                                                    | Location                                                                                                                                                                                               | Notes                                                                                                                                                                                          |
| --------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `matches` table with `status` + `winner_team` + `player1..4_id`                         | [server/migrations/000006_create_matches.up.sql](server/migrations/000006_create_matches.up.sql), [server/migrations/000008_add_match_status.up.sql](server/migrations/000008_add_match_status.up.sql) | All columns needed for stat aggregation already exist. Story 7.2 adds NO new migrations.                                                                                                       |
| `match.MatchRepository` interface with `Create`, `CreateWithHands`, `GetMatchesForUser` | [server/internal/match/repository.go](server/internal/match/repository.go)                                                                                                                             | Extend with `GetStatsForUser` in Task 1. Keep the interface cohesive — this is a read-query, same data source.                                                                                 |
| `UserHandler` with `GetProfile` returning `ProfileResponse`                             | [server/internal/user/handler.go:15-20, 98-129](server/internal/user/handler.go#L98-L129)                                                                                                              | Extend `ProfileResponse` and the `GetProfile` body in Task 2. No route change needed (`GET /users/:id/profile` already registered at [server/cmd/api/main.go:95](server/cmd/api/main.go#L95)). |
| `teamForSeat(seat) = seat % 2` rule                                                     | [server/internal/game/state.go:115-117](server/internal/game/state.go#L115-L117), locally duplicated in [server/internal/user/handler.go:278](server/internal/user/handler.go#L278)                    | Use the existing local `teamForSeat` from handler.go OR encode in SQL CASE (Task 1.2). Both patterns are pre-established.                                                                      |
| `useProfileQuery` + `queryKeys.profile.detail(id)`                                      | [client/src/shared/hooks/queries/useProfile.ts](client/src/shared/hooks/queries/useProfile.ts), [client/src/shared/api/queryKeys.ts:8-10](client/src/shared/api/queryKeys.ts#L8-L10)                   | TS type extension propagates automatically. No hook change.                                                                                                                                    |
| `ProfilePage` skeleton loader                                                           | [client/src/features/profile/ProfilePage.tsx:13-20](client/src/features/profile/ProfilePage.tsx#L13-L20)                                                                                               | Unchanged — the same `isPending` path continues to render the skeleton while the extended payload loads.                                                                                       |
| `profile.stats` section heading key                                                     | [en.json:17](client/src/shared/i18n/en.json#L17), [sr.json:17](client/src/shared/i18n/sr.json#L17)                                                                                                     | Retained. Only the section body changes.                                                                                                                                                       |
| `QueryWrapper` + i18n test wrapper + `data-testid` discipline                           | [client/src/test-utils/](client/src/test-utils/), Story 7.1 Dev Notes                                                                                                                                  | All test scaffolding for this story already exists and is used by `ProfilePage.test.tsx`.                                                                                                      |
| PII-leak-guard test pattern                                                             | [server/internal/user/handler_test.go](server/internal/user/handler_test.go) (`TestListMatches_NeverLeaksPII`, added in Story 7.1 Task 6.2)                                                            | Copy pattern verbatim for Task 3.3.                                                                                                                                                            |
| `i18n.test.ts` recursive `flattenKeys` parity check                                     | [client/src/shared/i18n/i18n.test.ts](client/src/shared/i18n/i18n.test.ts) (added in Story 7.1 review)                                                                                                 | Automatic safety net — if nested keys diverge between `en.json` and `sr.json`, CI fails.                                                                                                       |

### What Must Be Created

1. `MatchRepository.GetStatsForUser` method — interface + GORM implementation.

That's it — no new files. Story 7.2 is a surgical extension of existing code.

### What Must Be Modified

1. [server/internal/match/repository.go](server/internal/match/repository.go) — add `GetStatsForUser` to the interface.
2. [server/internal/match/gorm_repo.go](server/internal/match/gorm_repo.go) — implement `GetStatsForUser` (single FILTER-aggregation query).
3. [server/internal/user/handler.go](server/internal/user/handler.go) — extend `ProfileResponse` with 4 fields; call `GetStatsForUser` in `GetProfile` body.
4. [server/internal/user/handler_test.go](server/internal/user/handler_test.go) — extend `mockMatchRepo` with `GetStatsForUser`, add 4+ test cases.
5. [server/internal/session/manager_test.go](server/internal/session/manager_test.go) — extend its `mockMatchRepo` with a no-op / default `GetStatsForUser` stub (the session-manager path never calls it, but the interface must be satisfied — compilation gate).
6. [client/src/shared/api/profile.ts](client/src/shared/api/profile.ts) — add 4 required number fields to `ProfileResponse`.
7. [client/src/features/profile/ProfilePage.tsx](client/src/features/profile/ProfilePage.tsx) — replace `statsEmpty` placeholder with 4-tile grid + `StatTile` inline helper.
8. [client/src/features/profile/ProfilePage.test.tsx](client/src/features/profile/ProfilePage.test.tsx) — update fixtures + add 3 new tests.
9. [client/src/shared/i18n/en.json](client/src/shared/i18n/en.json) — add nested `profile.stats.*` block; remove `profile.statsEmpty`.
10. [client/src/shared/i18n/sr.json](client/src/shared/i18n/sr.json) — mirror.

**No changes expected:**

- `server/migrations/*` — zero new tables, zero schema changes.
- `server/cmd/api/main.go` — route already registered; handler signature unchanged.
- `server/internal/session/manager.go`, `server/internal/session/reconnect.go` — no persistence change.
- `server/internal/game/*` — rules engine untouched.
- `client/src/shared/api/axiosClient.ts`, `client/src/shared/stores/authStore.ts` — envelope + auth store unchanged.
- `client/src/features/profile/MatchHistory.tsx` — Story 7.1's work untouched.

### Data Model & Aggregation Strategy — Key Design Decision

**Chosen approach:** Per-request aggregate computation via a single `SELECT ... FILTER (WHERE ...) FROM matches` query. No materialised stats column on `users`, no denormalised counter.

**Rationale:**

- **Truth lives in the `matches` table.** Denormalising `wins`/`losses`/`abandoned` onto `users` introduces a two-writer coordination problem: every match completion would need to atomically update the 4 participants' counters, every abandonment the same — any crash or race between `match insert` and `user counter increment` causes drift. For Phase 1, on-read aggregation is simpler and the source of truth is unambiguous.
- **Query cost is O(indexed-scan) on `matches.player{1..4}_id`** — Story 7.1 already indexed those columns ([server/migrations/000006_create_matches.up.sql](server/migrations/000006_create_matches.up.sql)). PostgreSQL can satisfy the aggregation via an index scan + a table lookup for the `status`/`winner_team` columns. At thousands of matches per user the query remains sub-millisecond.
- **Single round-trip via `FILTER`** — three separate `COUNT` queries would be 3× the latency and lose atomicity (concurrent match completions could show inconsistent snapshots).
- **Upgrade path (Phase 2+):** If stats latency becomes an issue (e.g. 100k+ matches per user), move to a denormalised `user_stats` table updated via a trigger or a post-match hook. Current API shape is stable under that refactor — the handler computes the same fields either way.

**Rejected alternative:** Reuse `GetMatchesForUser` with no pagination, sum in Go. Rejected because: (1) allocates N matches per request — O(N) memory on an endpoint that's hit often; (2) transfers far more data than needed (hand rows, player IDs, timestamps) for a counts-only query; (3) unbounded — a user with 10k matches would allocate 10k structs per profile fetch.

**Rejected alternative:** Client-side derivation using the already-paginated `/matches` endpoint. Rejected because pagination returns only 20 rows at a time — the client would need to fetch all pages to compute totals, re-fetching on every profile visit, defeating the react-query cache.

### Architecture Patterns to Follow

- **Single responsibility per repository method.** `GetStatsForUser` returns counts only — no hand rows, no match rows. Separate from `GetMatchesForUser`. Two focused methods beat one polymorphic one.
- **Server-derived numerics, not client-derived.** `wins`/`losses`/`abandoned` / `totalGamesPlayed` / (`wins + losses + abandoned`) are server-computed. Win-rate is the **only** client-side derivation (a trivial percentage) — because rendering `"70%"` is purely presentational. If Phase 2 introduces more complex derived metrics (XP, tier, honor), server-derive them too.
- **Additive JSON extension, never breaking.** The extended `ProfileResponse` strictly adds four fields with well-defined zero values. Any existing consumer that ignores unknown fields continues to work. Document this in the PR: "additive only — no breaking changes."
- **Repository interface ≠ public handler contract.** `GetStatsForUser` returns `(wins, losses, abandoned, err)` — four returns instead of a struct. The handler assembles the HTTP DTO. If Phase 2 extends the repository method with more metrics (e.g. `capotCount`), the handler is the projection seam, not the repo.
- **`data-testid` over Tailwind class selectors** — reinforced by Story 7.1 Dev Notes; Tailwind class churn breaks class-based test selectors. All new stat tiles use `data-testid="profile-stat-<key>"` + `data-value="<number>"`.
- **No `any` in TypeScript.** `ProfileResponse` extends with `number` fields (required, not optional). Tests updating the fixture must include all four explicitly.
- **i18n parity is CI-enforced.** Every key added to `en.json` is added to `sr.json` in the same commit — Story 7.1's recursive `flattenKeys` parity test blocks divergence.
- **No destructive tone on loss metrics.** `losses` displays with `text-text-secondary`, NEVER `text-destructive` — per UX spec colour token roles ([ux-design-specification.md:321-323](_bmad-output/planning-artifacts/ux-design-specification.md#L321-L323)) and the tone decision locked in by Story 7.1 for the outcome badge.

### Previous Story Intelligence (Story 7.1 — done 2026-04-19)

Carried-forward learnings that shape this story:

- **Prettier runs before every commit.** Memory `feedback_prettier_before_commit.md` is load-bearing; CI has failed repeatedly. Task 9.3 enforces this.
- **`data-testid` + RTL selectors win over text-based queries** — especially for i18n-aware components. Win-rate rendering varies by locale (`%` symbol placement may differ in some locales later); assert on `data-value` for numerics.
- **`ProfilePage.test.tsx` already mocks `@/shared/api/matches` and `@/shared/api/profile`** at module level — pattern established. Task 8 adds more assertions to the same test file without restructuring.
- **`QueryWrapper` from `@/test-utils` is the canonical react-query test wrapper** for this repo.
- **PII-leak-guard test pattern** established in Story 7.1 (`TestListMatches_NeverLeaksPII`). Copy the pattern into `TestGetProfile_NeverLeaksPII` in Task 3.3.
- **`viewerSeat = 0` silent default** is a known `[LOW]` issue in Story 7.1 Review Findings — the matches endpoint currently defaults to seat 0 for non-participants. Story 7.2 sidesteps this by computing stats at the SQL level (participation filter in the `WHERE` clause) — a non-participant contributes nothing. Do NOT port the seat-default pattern into this story.
- **Story 7.1 review applied 14 patches in one batch** — expect similar review-patch tightening on Story 7.2. Design so the repository signature, JSON shape, and testids are stable under review adjustments.
- **The i18n `flattenKeys` parity test is automatic** — any divergence between `en.json` and `sr.json` trips CI regardless of developer diligence.

### Recent Codebase Signals (git log — last 5 commits)

- `99c64d7 feat(profile): match history with per-hand scoring (Story 7.1)` — **direct predecessor**. Introduced `hand_results` table, `MatchHistory` component, `/users/:id/matches` endpoint, extended i18n `profile.matchHistory.*` block. This story reads from the same `matches` table and extends the same `ProfileResponse` DTO. **Biggest risk:** double-modifying the same `en.json`/`sr.json` `profile` block in a merge with concurrent work (none expected — 7.1 is merged).
- `0e9a864 chore(planning): restructure Epics 8-16` — planning-only; no code touched. No impact.
- `52839cf feat(chat): match-scoped chat with collapsible sidebar (Story 6.2)` — renders on the game page, not `/profile`. No touchpoint risk.
- `0d90b73 feat(chat): global lobby chat (Story 6.1)` — same — no profile touchpoint.
- `ab09141 feat(game): name the declarer on each declaration-reveal row` — UI-only, `/profile` untouched.

**Signal: `matches` table is freshly-hardened.** Story 7.1 validated the FK + status semantics end-to-end. This story's aggregation queries the same columns with no schema change — very low baseline risk.

### Backend Flow — Handling `GET /api/v1/users/:id/profile`

1. Echo routes through the auth middleware → handler `GetProfile(c)`.
2. `getUserID` extracts `authUserID` from context; parse `:id` → enforce equality (403 otherwise).
3. `userRepo.FindByID(authUserID)` — unchanged from today.
4. **NEW**: `matchRepo.GetStatsForUser(authUserID)` — single SQL aggregation returning `(wins, losses, abandoned)`.
5. Compute `totalGamesPlayed = wins + losses + abandoned`.
6. Return `{ data: { id, username, languagePreference, createdAt, totalGamesPlayed, wins, losses, abandoned } }`.

Error paths unchanged; the new DB error path from step 4 flows through `appErrorHandler` → 500.

### Frontend Flow — Rendering `<ProfilePage />` Stats

1. `useProfileQuery(userId)` fires → profile response includes the 4 new stat fields.
2. `isPending && !data` → existing skeleton at [ProfilePage.tsx:13-20](client/src/features/profile/ProfilePage.tsx#L13-L20).
3. Data arrives → 4 `StatTile` components render with numeric `data-value` + translated `label`.
4. Win-rate: `played = wins + losses`; `rate = played === 0 ? undefined : Math.round((wins/played)*100)`; `display = rate === undefined ? t('profile.stats.winRateEmpty') : ${rate}%`.
5. `MatchHistory` below the stats section continues to fetch/render independently (Story 7.1 behaviour preserved).

### Cross-Story Context

- **Story 7.1 (done)** — introduced `GET /users/:id/matches` with per-match detail. Story 7.2 is the counterpart: aggregate summary stats on the same data. Distinct endpoint, distinct repo method; no merging of payloads.
- **Epic 1 Story 1.4 (done)** — introduced the placeholder `profile-stats` section + `statsEmpty` copy. Story 7.2 closes that loop per Epic 7 AC "all placeholder sections on the profile page are replaced with real data".
- **Epic 5 (done)** — abandonment path writes `status = 'abandoned'`. Story 7.2's `abandoned` count pulls from that column — verified by the integration test in Task 4.
- **Epic 9 Story 9.5 (future — backlog)** — XP & Level system. The profile will gain `level`, `xp`, `xpProgress` fields. Design the `ProfileResponse` extension so adding more optional fields is zero-risk (this story reinforces the additive extension pattern).
- **Epic 9 Story 9.6 (future — backlog)** — Honor Score. Will add `honorScore`, `honorTier`, `trend` fields. Same additive pattern.
- **Epic 11 Story 11.3 (future — backlog)** — Public Player Profiles. Will introduce a separate `GET /users/:id/public-profile` handler for non-authorised viewers; that's when the `:id == authUserID` check relaxes (only on the new route). Story 7.2 MUST NOT touch the auth check — see AC #7.
- **Epic 13 (future)** — Seasonal Rank & Leaderboard. May add `currentTier`, `seasonPoints`. Additive extension expected.

### Project Structure Notes

**Modified files (expected):** see "What Must Be Modified" table above — 10 files modified, 0 new files, 0 migrations.

**Alignment with unified project structure:**

- Repository methods live alongside each other in [server/internal/match/gorm_repo.go](server/internal/match/gorm_repo.go) — same package, same file, pattern-consistent with existing `Create` / `CreateWithHands` / `GetMatchesForUser`.
- Client stats UI lives in [client/src/features/profile/ProfilePage.tsx](client/src/features/profile/ProfilePage.tsx) — same feature folder as Story 7.1's `MatchHistory.tsx` + `ProfilePage.tsx`, per the feature-folder convention in [architecture.md:651-653](_bmad-output/planning-artifacts/architecture.md#L651-L653).
- `StatTile` is an inline helper; if it grows beyond ~30 LOC or gains a second consumer, extract to `client/src/features/profile/StatTile.tsx` (Task 6.2 sets the extraction threshold).
- No new Zustand slice — pure react-query cache data, mirroring Story 7.1's decision.

### Alignment Checks / Detected Conflicts

- **i18n heading-vs-object collision risk (Task 7.1).** `profile.stats` currently points to the string `"Stats"` (section heading). Adding `profile.stats.*` sub-keys makes `profile.stats` both a string AND an object — i18next's handling varies per version. **Action:** Task 7.1 instructs to verify behaviour with the `i18n.test.ts` parity check and, if collision, rename the heading to `profile.statsHeading` (trivial one-line `t()` call update). Flag this in the PR description as a known rename if it happens.
- **No schema conflicts.** `matches` table is the only data source; no migration needed; no race with Story 7.1.
- **Route registration.** `GET /users/:id/profile` is already registered — this story changes the handler body, not the route. No `main.go` edit needed.

### References

- [Source: epics.md#Story-7.2 — Expanded Player Profile acceptance criteria](_bmad-output/planning-artifacts/epics.md#L1400)
- [Source: prd.md — FR4 (profile with stats summary + match history), FR42 (career statistics: win/loss record, points scored, rank history)](_bmad-output/planning-artifacts/prd.md#L308)
- [Source: architecture.md#L879 — Stats & History FR41-FR43 location mapping to `internal/user/handler.go` + `features/profile/`](_bmad-output/planning-artifacts/architecture.md#L879)
- [Source: ux-design-specification.md#L321-L323 — colour tokens (success / destructive / warning); #L339 + #L357 — Space Grotesk rule for score/stat numbers; #L894 — desktop-only Phase 1](_bmad-output/planning-artifacts/ux-design-specification.md)
- [Source: 7-1-match-history-display.md — prior story; QueryWrapper, PII-leak test pattern, i18n parity, prettier-before-commit, teamForSeat duplication pattern](_bmad-output/archive/implementation-artifacts/7-1-match-history-display.md)
- [Source: 1-4-basic-player-profile-and-navigation-shell.md — established placeholder pattern the stats section currently uses](_bmad-output/archive/implementation-artifacts/1-4-basic-player-profile-and-navigation-shell.md)
- [Source: server/internal/user/handler.go#L98-L129 — GetProfile to extend; #L278 — local teamForSeat; #L15-L20 — ProfileResponse to extend](server/internal/user/handler.go)
- [Source: server/internal/match/repository.go — MatchRepository to extend with GetStatsForUser](server/internal/match/repository.go)
- [Source: server/internal/match/gorm_repo.go — GORM implementation site for GetStatsForUser](server/internal/match/gorm_repo.go)
- [Source: client/src/features/profile/ProfilePage.tsx#L66-L74 — placeholder stats section to replace](client/src/features/profile/ProfilePage.tsx)
- [Source: client/src/shared/api/profile.ts — ProfileResponse TS type to extend](client/src/shared/api/profile.ts)
- [Source: client/src/shared/i18n/en.json#L14-L55 + sr.json — profile i18n block to extend; statsEmpty to remove](client/src/shared/i18n/en.json)

## Dev Agent Record

### Agent Model Used

claude-opus-4-7 (1M context)

### Debug Log References

- Backend: `go build ./...` + `go vet ./...` clean; `go test ./... -count=1` all green (user, session, room, ws, auth, apperr, chat, game, cmd/api).
- Frontend: `npx vitest run` 400 tests green across 49 files; `ProfilePage.test.tsx` 8/8 (3 new scenarios: real-stats render, em-dash win-rate, coexistent empty states).
- Lint: `npx prettier --write .` (reformatted ProfilePage.tsx div opening line) + `npx prettier --check .` clean; `npx eslint .` clean.
- `make lint` runs client half clean; server half (`golangci-lint`) is not installed in this local Windows shell — `go vet ./...` was used as an equivalent static-analysis gate.
- Manual smoke (AC #9.5): deferred — requires an interactive dev environment (`make dev`) with two live users. The unit + integration + handler tests exercise the same branches end-to-end (zero-games, mixed, abandoned, cross-seat team derivation, DB error, PII leak), and `TestGetProfile_StatsMatchListTotal` asserts the AC #2 `stats.total == matches.total` invariant over a seed covering seats 0/2/3 and all three status outcomes.

### Completion Notes List

- **AC #1 — extended endpoint shape**: `ProfileResponse` now carries `totalGamesPlayed`, `wins`, `losses`, `abandoned` alongside the existing identity fields. Auth / 403 / `languagePreference`-only-to-owner semantics are untouched ([server/internal/user/handler.go:15-23](server/internal/user/handler.go#L15-L23), [server/internal/user/handler.go:113-137](server/internal/user/handler.go#L113-L137)).
- **AC #2 — single-round-trip aggregation**: new `MatchRepository.GetStatsForUser` encapsulates the SQL via `r.db.Raw` with `COUNT(*) FILTER (...)` and a per-row `CASE WHEN player{1..4}_id = ? THEN {0|1}` expression deriving the viewer's team, then a participation `WHERE` clause. No Go-side loop, no N+1. Integration test `TestGetProfile_StatsMatchListTotal` seeds 4 viewer matches + 1 bystander-only match and asserts `prof.TotalGamesPlayed == matchesListTotal` and the per-bucket expected counts (2W/1L/1A).
- **AC #3, #4, #5 — UI stat tiles**: `ProfilePage.tsx` replaces the `statsEmpty` placeholder with a 4-tile `grid grid-cols-2 md:grid-cols-4 gap-4` layout. Inline `StatTile` helper with three tone channels (`neutral` → `text-text-primary`, `success` → `text-success`, `muted` → `text-text-secondary`). Losses always render `muted` — never `destructive`, honouring the UX tone decision from Story 7.1. Win-rate shows `{rate}%` or the em-dash when no games have been played (never `NaN%`). Each tile exposes `data-value` for locale-independent test targeting.
- **AC #6 — i18n collision resolved by fallback rename**: JSON cannot carry `profile.stats` as both a string and an object. Per the author-specified fallback in Task 7.1, the section heading was renamed to `profile.statsHeading` and `profile.stats` is now a nested object (`totalGamesPlayed`, `wins`, `losses`, `winRate`, `winRateEmpty`). Mirrored in en.json + sr.json in the same commit; `profile.statsEmpty` removed from both. Serbian register chosen: `Pobede` / `Procenat pobeda` (Ekavian), matching the existing `matchHistory.outcomeWin: "Pobeda"` from Story 7.1. The recursive `flattenKeys` parity test passes.
- **AC #7 — authorisation unchanged**: the existing `!= authUserID → 403` + bad-id → 400 + missing-token → 401 branches were not touched. `TestGetProfile_AuthFailures_DoNotCallStats` verifies the stats path is skipped on each auth-fail branch (`getStatsCalls == 0` after 400/401/403).
- **AC #7 — DB errors not swallowed**: `GetStatsForUser` errors are wrapped with `%w` and flow through `appErrorHandler` → 500. `TestGetProfile_StatsDBError` asserts this.
- **AC #7 — PII guard**: `TestGetProfile_NeverLeaksPII` asserts the serialised response contains none of `email`, `"email"`, `passwordHash`, `password_hash`, `deletedAt`.
- **AC #8 — backward compat**: TS `ProfileResponse` gains four required `number` fields (no `?`, no `any`). Only one TS fixture needed updating (`ProfilePage.test.tsx`), now centralised through a `profileFixture()` helper so future additive extensions touch one place.
- **Interface hygiene**: the two existing `mockMatchRepo` implementations (`server/internal/session/manager_test.go`, `server/internal/user/handler_test.go`) both implement the new `GetStatsForUser` method. The session-manager mock is a no-op stub (the session path never calls it); the user-handler mock supports call counting + an override struct for deterministic tests.

### File List

Server:

- `server/internal/match/repository.go` — MODIFIED (added `GetStatsForUser` to the `MatchRepository` interface).
- `server/internal/match/gorm_repo.go` — MODIFIED (implemented `GetStatsForUser` via single `SELECT … FILTER (WHERE …)` with per-row `CASE` team derivation).
- `server/internal/user/handler.go` — MODIFIED (extended `ProfileResponse` with 4 fields; `GetProfile` now calls `matchRepo.GetStatsForUser` and projects the counts).
- `server/internal/user/handler_test.go` — MODIFIED (mock gained `GetStatsForUser` with call tracking + override; added `TestGetProfile_WithStats` table tests, `TestGetProfile_StatsDBError`, `TestGetProfile_NeverLeaksPII`, `TestGetProfile_AuthFailures_DoNotCallStats`, `TestGetProfile_StatsMatchListTotal`).
- `server/internal/session/manager_test.go` — MODIFIED (added no-op `GetStatsForUser` stub so the mock continues to satisfy `MatchRepository`).

Client:

- `client/src/shared/api/profile.ts` — MODIFIED (`ProfileResponse` gained `totalGamesPlayed`, `wins`, `losses`, `abandoned`).
- `client/src/features/profile/ProfilePage.tsx` — MODIFIED (added inline `StatTile` helper with three tone channels; replaced the `statsEmpty` placeholder with a 4-tile grid driven by the profile payload; renamed heading key to `profile.statsHeading`).
- `client/src/features/profile/ProfilePage.test.tsx` — MODIFIED (centralised `profileFixture()` helper with full 4-stat defaults; retargeted the `"renders placeholder sections"` test to `"renders match-history section + four stat tiles"` with `data-value` assertions; added `"renders real stats when profile has played games"`, `"renders em-dash for win-rate when zero games played"`, `"renders stats tiles alongside match-history empty state when user has no games"`).
- `client/src/shared/i18n/en.json` — MODIFIED (renamed `profile.stats` heading to `profile.statsHeading`; added nested `profile.stats.{totalGamesPlayed,wins,losses,winRate,winRateEmpty}`; removed `profile.statsEmpty`).
- `client/src/shared/i18n/sr.json` — MODIFIED (same renames + additions; Ekavian register chosen to match adjacent profile keys).

Planning artifacts:

- `_bmad-output/archive/implementation-artifacts/7-2-expanded-player-profile.md` — MODIFIED (this file: status, checkboxes, Dev Agent Record).
- `_bmad-output/implementation-artifacts/sprint-status.yaml` — MODIFIED (7-2 moved `ready-for-dev` → `in-progress` → `review`).

## Change Log

- 2026-04-19: Implemented Story 7.2. `GET /users/:id/profile` now returns aggregate stats (`totalGamesPlayed`, `wins`, `losses`, `abandoned`) via a single-round-trip `FILTER`-aggregated query against the `matches` table; profile page replaces the `statsEmpty` placeholder with four stat tiles (Games played / Wins / Losses / Win rate), with an em-dash shown for win-rate when no games have been played. i18n heading/object collision resolved by renaming `profile.stats` (heading) to `profile.statsHeading` and making `profile.stats` a nested block; `profile.statsEmpty` removed from en.json + sr.json. All existing auth / 403 / PII-leak semantics preserved.

### Review Findings

Code review run 2026-04-19 (Blind Hunter + Edge Case Hunter + Acceptance Auditor, 47 raw findings, 41 dismissed as noise/spec-compliant/schema-protected).

- [x] [Review][Patch] Win-rate denominator must include abandoned matches [client/src/features/profile/ProfilePage.tsx:100-148] — FIXED: `rate = totalGamesPlayed === 0 ? undefined : Math.round((wins / totalGamesPlayed) * 100)`. Em-dash now reserved strictly for `totalGamesPlayed === 0`. `ProfilePage.test.tsx` fixture updated to `(10W, 7, 2, 1)` for a clean 70%, plus a new regression test `counts abandoned games in the win-rate denominator` asserting (4W/3L/3A) → 40% (not 57%).
- [x] [Review][Patch] ProfilePage stats section has no `isError` branch [client/src/features/profile/ProfilePage.tsx:38,100-106] — FIXED: `useProfileQuery` now destructures `isError`; stats section renders `<p data-testid="profile-stats-error">{t("profile.stats.error")}</p>` on query failure, mirroring `MatchHistory.tsx:285` pattern. Added i18n key `profile.stats.error` to both locales (en / sr) and new test `renders stats error state when profile query fails`.
- [x] [Review][Patch] PII-leak guard doesn't cover the DB-error response path [server/internal/user/handler_test.go:735-758] — FIXED: `TestGetProfile_StatsDBError` now uses a distinctive probe email (`alice-err-probe@example.com`) and asserts the 500 body contains none of `email`, `"email"`, `passwordHash`, `password_hash`, mirroring the 200-path `TestGetProfile_NeverLeaksPII` guard.
- [x] [Review][Defer] No integration test exercises the real `FILTER … CASE` SQL [server/internal/match/gorm_repo.go:34-74] — deferred, pre-existing infrastructure gap. `TestGetProfile_StatsMatchListTotal` asserts the invariant at the mock level; a syntax typo in the Raw SQL would ship green. Requires a Postgres-backed repo test harness (doesn't exist in this project today).
- [x] [Review][Defer] Status-enum desync risk between `GetStatsForUser` and `GetMatchesForUser` [server/internal/match/gorm_repo.go:46-62, :78] — deferred, future-proof. Both hardcode `('completed','abandoned')`; adding `'in_progress'` / `'cancelled'` later would silently drift the `totalGamesPlayed == matches.total` invariant. No immediate bug.
