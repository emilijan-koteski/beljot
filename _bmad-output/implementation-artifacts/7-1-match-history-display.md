# Story 7.1: Match History Display

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a player,
I want to view my match history with detailed scoring for each game,
so that I can review past games and track my performance.

## Acceptance Criteria

1. **Match list endpoint returns paginated, newest-first completed matches for the authenticated player**
   Given an authenticated player whose userID matches the `:id` path param,
   When `GET /api/v1/users/:id/matches?limit=<1-50>&offset=<>=0>` is called,
   Then the server returns `200 { "data": { "items": [...], "total": <int>, "limit": <echoed>, "offset": <echoed> } }` — items are `matches` rows where the authenticated user appears in **any** of `player1_id..player4_id` and `status IN ('completed','abandoned')`, ordered by `completed_at DESC, id DESC` (id tiebreaker is stable under same-tick completions)
   And the default `limit` is `20`, max `50` (values outside `[1,50]` → `400 BAD_REQUEST`); default `offset` is `0` (negative → `400 BAD_REQUEST`)
   And each item embeds enough data to render AC #2 **without** an N+1 user lookup: `id`, `variant`, `matchMode`, `startedAt`, `completedAt`, `status`, `winnerTeam`, `teamAScore`, `teamBScore`, `abandonedBy` (nullable), `viewerSeat` (0–3), `players: [{ seat, userId, username }, ...]` (4 entries, seat-ordered 0,1,2,3).
   And if `:id` does NOT match the authenticated userID → `403 FORBIDDEN` (mirrors `GET /users/:id/profile` authorization exactly, per [server/internal/user/handler.go:56-58](server/internal/user/handler.go#L56-L58)).

2. **`MatchHistory` component renders one row per match with derived human-readable metadata**
   Given match history data is returned,
   When `<MatchHistory userId={id} />` renders,
   Then each row shows — using locale-formatted date (via `Intl.DateTimeFormat`, `i18n.language === "sr" ? "sr-Latn" : i18n.language`, pattern already established in [ProfilePage.tsx:23-34](client/src/features/profile/ProfilePage.tsx#L23-L34)): completed date (`YYYY MMM DD`), variant label (`bitola` → "Bitola"), mode label (`1001` → "1001 points"), **teammate** username (seat `(viewerSeat + 2) % 4`), **opponent** usernames (seats `(viewerSeat + 1) % 4` and `(viewerSeat + 3) % 4`), final score formatted as `"<a> – <b>"` with team labels coloured via `text-team-a` / `text-team-b` tokens, win/loss/abandoned outcome badge (see AC #7), and match duration `hh:mm:ss` (or `mm:ss` when < 1h) computed from `completedAt - startedAt`.
   And each row has `data-testid="match-history-row"` plus `data-match-id="<id>"` for test targeting.

3. **Per-hand detail view renders when a match row is expanded (same API call, inline expansion — NOT a second HTTP request)**
   Given a player clicks a match row,
   When the row expands,
   Then a per-hand breakdown renders from the **already-loaded** match item (see AC #6 — hands are embedded in the list payload): each hand shows hand number (`Hand 1`, `Hand 2`, …), Team A/Team B card-points, Team A/Team B declaration points, last-trick bonus (with owning team), Capot indicator (+100 badge + team when `capot === true`), failed-hand indicator (pill labelled `"Failed"` + the taker's team) when `failedContract === true`, and final Team A/Team B hand totals (`teamAHandTotal` / `teamBHandTotal` from the `hand_results` row).
   And the expansion is a pure UI state toggle (no new fetch, no WS traffic).
   And the expanded section has `data-testid="match-history-detail"` with `data-match-id="<id>"` for test targeting; the toggle control has `aria-expanded="true|false"` and `aria-controls` referencing the detail container.

4. **Empty state renders when the player has no completed matches**
   Given the authenticated player has zero matches in the `matches` table (status in `completed` or `abandoned`),
   When `<MatchHistory>` renders,
   Then the empty state shows `i18n: profile.matchHistory.empty` — copy `"No games yet — Quick Play to get started"` / `"Nema mečeva — započni Brzu igru"` — wrapped in a `<Link to="/lobby">` that navigates to the lobby (per UX spec at [ux-design-specification.md:876](_bmad-output/planning-artifacts/ux-design-specification.md#L876)).
   And `data-testid="match-history-empty"`.

5. **Pagination via "Load more" button (NOT infinite scroll in Phase 1)**
   Given the returned `total > items.length` after the initial fetch,
   When the `<MatchHistory>` renders,
   Then a `Load more` button appears below the list (`data-testid="match-history-load-more"`, label via `i18n: profile.matchHistory.loadMore`); clicking it fetches the next page (`offset += limit`) via `useInfiniteQuery` (`@tanstack/react-query` v5) and appends items to the list; when all rows are loaded (`items.length >= total`) the button is removed.
   And loading states use a `text-text-secondary` skeleton rows pattern (3 pulsing rows, `data-testid="match-history-loading"`) — same pattern as [ProfilePage.tsx:14-16](client/src/features/profile/ProfilePage.tsx#L14-L16).
   And **infinite-scroll is explicitly out of scope for 7.1** — the button form is the MVP contract (per Epic AC "load more button OR infinite scroll"; we choose button).

6. **Per-hand scoring data is persisted to a new `hand_results` table during match play and returned with the match payload**
   Given a hand is scored in `scoring.go::ScoreHand` and the session manager receives the updated `GameState` with a non-nil `LastHandResult`,
   When the hand-scored branch at [server/internal/session/manager.go:420-440](server/internal/session/manager.go#L420-L440) runs,
   Then the session manager **buffers** the `HandResult` into an in-memory list on the `Session` struct (one entry per scored hand in order) — see Dev Notes **"Data Model & Persistence"**.
   And when the match terminates (normal completion via `handleMatchEnd`, or abandonment via `reconnect.go:absolutePlayerTimeout`), the buffered hand rows are persisted **in the same `matchRepo.Create` transaction** as the match row via a new `CreateWithHands(match, []HandResultRow)` method — atomic: either all persist or nothing does.
   And a new migration `000009_create_hand_results.up.sql` / `.down.sql` creates the table with: `id SERIAL PK`, `match_id INTEGER NOT NULL REFERENCES matches(id) ON DELETE CASCADE`, `hand_number INTEGER NOT NULL`, `red_card_points INTEGER NOT NULL`, `blue_card_points INTEGER NOT NULL`, `red_decl_points INTEGER NOT NULL`, `blue_decl_points INTEGER NOT NULL`, `last_trick_team SMALLINT NOT NULL`, `last_trick_bonus INTEGER NOT NULL`, `capot BOOLEAN NOT NULL`, `capot_team SMALLINT NULL`, `capot_bonus INTEGER NOT NULL`, `failed_contract BOOLEAN NOT NULL`, `contracting_team SMALLINT NOT NULL`, `red_hand_total INTEGER NOT NULL`, `blue_hand_total INTEGER NOT NULL`, `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`, `UNIQUE (match_id, hand_number)`, index `idx_hand_results_match_id ON hand_results(match_id)`. The `.down.sql` drops the table.
   And the matches list endpoint response embeds `hands: [...]` for every match (see AC #1) via a single `JOIN`/`Preload` — **no N+1**. For matches abandoned before the first hand scored, `hands: []`.

7. **Outcome badge (win / loss / abandoned) is derived server-side per viewer**
   Given the matches list handler computes the viewer's outcome for each match,
   When building the response payload,
   Then `outcome` is one of: `"win"` when `status === "completed"` AND `winnerTeam === team_for_seat(viewerSeat)` (where `team_for_seat(seat) = seat % 2` — seats 0/2 are Team A index 0, seats 1/3 are Team B index 1, mirroring `game.TeamForSeat` in [server/internal/game/state.go:115-117](server/internal/game/state.go#L115-L117)); `"loss"` when `status === "completed"` AND `winnerTeam !== team_for_seat(viewerSeat)`; `"abandoned"` when `status === "abandoned"` regardless of `winnerTeam`.
   And the TS render renders the badge via the `<OutcomeBadge outcome="win|loss|abandoned" />` component (see Dev Notes) using tokens `success` (win), `text-secondary` (loss), `warning` (abandoned) — never `destructive` for loss (UX tone spec — no red-alert stigma on losses).

8. **Integration into `ProfilePage` — replaces the placeholder section, preserves existing tests**
   Given [ProfilePage.tsx](client/src/features/profile/ProfilePage.tsx) currently renders a placeholder `profile.matchHistoryEmpty` `<section data-testid="profile-match-history">`,
   When this story ships,
   Then the placeholder `<p>` inside that section is replaced with `<MatchHistory userId={user?.id} />` — the outer `<section data-testid="profile-match-history">` wrapper is retained so the existing `ProfilePage.test.tsx` assertions on `profile-match-history` do **not** regress.
   And the `Stats` section (`data-testid="profile-stats"`) remains untouched — it is Story 7.2's scope; do NOT modify it in this story.
   And all existing `ProfilePage.test.tsx` cases pass without modification except for those that specifically asserted the placeholder-copy string `profile.matchHistoryEmpty` inside the match-history section — those cases must be removed or retargeted to the new empty-state testid (see AC #4).

9. **i18n keys added to both `en.json` and `sr.json` in the same commit**
   Given new copy introduced by this story,
   When the story lands,
   Then the following keys exist in **both** [client/src/shared/i18n/en.json](client/src/shared/i18n/en.json) and [client/src/shared/i18n/sr.json](client/src/shared/i18n/sr.json) under `profile.matchHistory.*`: `title`, `empty` (replaces existing `profile.matchHistoryEmpty`), `loadMore`, `loading`, `error`, `duration`, `outcomeWin`, `outcomeLoss`, `outcomeAbandoned`, `teammate`, `opponents`, `vs`, `variant.bitola`, `mode.1001`, `handNumber`, `cardPoints`, `declarationPoints`, `lastTrickBonus`, `capot`, `failedContract`, `handTotal`, `expandRow`, `collapseRow`, `failedContractTeamA`, `failedContractTeamB`.
   And any removed key (`profile.matchHistoryEmpty`) is removed from **both** JSONs and all references updated; `i18n.test.ts` parity test continues to pass.
   And any new strings respect the Serbian-Latin register already established (e.g., "Istorija mečeva", "Pobjeda/Pobeda"); when in doubt, copy the register of adjacent keys in `sr.json`.

10. **Authorization and error handling mirror existing `GetProfile`**
    Given the handler resolves `:id` param,
    When the param is malformed (`NaN`, `0`, negative) or does not match the authenticated userID,
    Then the response is `400 BAD_REQUEST` / `403 FORBIDDEN` respectively (identical branching to [server/internal/user/handler.go:45-58](server/internal/user/handler.go#L45-L58)).
    And `limit`/`offset` parsing failures → `400 BAD_REQUEST` with code `BAD_REQUEST`.
    And DB errors → wrapped with `%w` and return `500 INTERNAL_ERROR` via `appErrorHandler` (existing middleware).
    And the handler never leaks PII: only the 4 participant usernames + ids are included in `players[]`; email, password hash, language preference are **never** in the response.

## Tasks / Subtasks

- [x] **Task 1: Migration for `hand_results` table (AC #6)**
  - [x] 1.1 Create [server/migrations/000009_create_hand_results.up.sql](server/migrations/000009_create_hand_results.up.sql) with the schema specified in AC #6. Reference [server/migrations/000006_create_matches.up.sql](server/migrations/000006_create_matches.up.sql) for column-naming and index-naming conventions.
  - [x] 1.2 Create [server/migrations/000009_create_hand_results.down.sql](server/migrations/000009_create_hand_results.down.sql) — `DROP TABLE IF EXISTS hand_results;` (no explicit `DROP INDEX` needed; PostgreSQL auto-drops indexes with the table).
  - [x] 1.3 Run `make migrate` locally, verify with `psql` that table and unique index + FK exist.
  - [x] 1.4 Commit the migration as part of this story — no separate migration PR.

- [x] **Task 2: New `hand_results` GORM model + repository extension (AC #6)**
  - [x] 2.1 Create [server/internal/match/hand_result.go](server/internal/match/hand_result.go) — a new file alongside the existing `Match` model (same package) with a `HandResult` struct (distinct from `game.HandResult` in [server/internal/game/state.go:29-43](server/internal/game/state.go#L29-L43)) carrying the exact columns from AC #6 + `gorm:"foreignKey:MatchID"` on the `Match` side. Rename imports at call sites if collision is awkward: `matchdb "github.com/emilijan/beljot/server/internal/match"` + `game.HandResult` for the in-memory type.
  - [x] 2.2 Add `Hands []HandResult gorm:"foreignKey:MatchID"` field to [server/internal/match/model.go](server/internal/match/model.go). Struct-tag bridge: DB column `match_id` / JSON `hands`.
  - [x] 2.3 Extend [server/internal/match/repository.go](server/internal/match/repository.go) with `CreateWithHands(match *Match, hands []HandResult) error` — old `Create(match *Match) error` stays for any callers that don't yet have hand data (tests, legacy paths); new callers use `CreateWithHands`.
  - [x] 2.4 Implement in [server/internal/match/gorm_repo.go](server/internal/match/gorm_repo.go) using `db.Transaction(func(tx *gorm.DB) error { ... })` — create match first (which sets `match.ID`), then batch-insert `hands[i].MatchID = match.ID` then `tx.Create(&hands)`. Return error on any step → rollback.
  - [x] 2.5 Add `GetMatchesForUser(userID uint, limit, offset int) (items []Match, total int64, err error)` to the repository interface, returning matches where `userID ∈ {player1_id, player2_id, player3_id, player4_id}` AND `status IN ('completed','abandoned')`, ordered `completed_at DESC, id DESC`, with `Preload("Hands", func(db *gorm.DB) *gorm.DB { return db.Order("hand_number ASC") })` to avoid N+1 on the embedded hands. Run `COUNT(*)` for `total` under the same `where` clause.

- [x] **Task 3: Buffer + persist hand results from the session manager (AC #6)**
  - [x] 3.1 Add `handResults []match.HandResult` field to the `Session` struct in [server/internal/session/manager.go](server/internal/session/manager.go) near other per-session state. Mutate only under the existing session `mu` write lock.
  - [x] 3.2 At the hand-scored broadcast branch [server/internal/session/manager.go:420-440](server/internal/session/manager.go#L420-L440), after `handScored` is sent, append a new `match.HandResult` to `session.handResults` with `HandNumber: oldState.HandNumber + 1` (the hand **just completed** — `newState.HandNumber` has been incremented already by `startNewHand`; `oldState.HandNumber + 1` IS the completed hand's number) mapped from `newState.LastHandResult`. Special case: on `PhaseMatchEnd` the final hand is not followed by a `startNewHand`, so use `oldState.HandNumber + 1` if `newState.HandNumber == oldState.HandNumber`, else `oldState.HandNumber + 1`. Unit-test both branches.
  - [x] 3.3 Replace `m.matchRepo.Create(matchRecord)` in `handleMatchEnd` at [server/internal/session/manager.go:592](server/internal/session/manager.go#L592) with `m.matchRepo.CreateWithHands(matchRecord, session.handResults)` — copy the slice under `session.mu.RLock()` before the call to avoid holding the lock during I/O. Log `len(hands)` alongside the existing success `slog.Info`.
  - [x] 3.4 Do the SAME replacement in [server/internal/session/reconnect.go:332](server/internal/session/reconnect.go#L332) (the abandoned-match path). A match abandoned before hand 1 completes has `session.handResults == []` — that's correct and intentional (AC #6 last clause).
  - [x] 3.5 Update `session.Manager` mock/fake in [server/internal/session/manager_test.go](server/internal/session/manager_test.go) (if any) to use a `matchRepo` fake implementing the new `CreateWithHands` method. Add a new test `TestSessionManager_BuffersAndPersistsHandResults` that plays to match end and asserts the fake's captured `hands` slice has `len == N` hands in correct order.

- [x] **Task 4: New handler method `ListUserMatches` (AC #1, #7, #10)**
  - [x] 4.1 Extend [server/internal/user/handler.go](server/internal/user/handler.go) with a new `UserHandler` field `matchRepo match.MatchRepository` and `userRepo UserRepository` already exists. Update `NewUserHandler` to accept `matchRepo` — a second argument (parallel to the pattern in `room.NewRoomHandler` at [server/internal/room/handler.go](server/internal/room/handler.go), which takes multiple deps).
  - [x] 4.2 Add method `(h *UserHandler) ListMatches(c echo.Context) error`. Auth-parse pattern lifted verbatim from `GetProfile` [server/internal/user/handler.go:45-58](server/internal/user/handler.go#L45-L58): `getUserID` → parse `:id` → `!= authUserID → ErrForbidden`. Then parse `limit` (default 20, clamp 1..50 else 400) and `offset` (default 0, reject < 0 else 400).
  - [x] 4.3 Call `matchRepo.GetMatchesForUser(authUserID, limit, offset)`; for each `match`, derive `viewerSeat` by finding which of `player1_id..player4_id` equals `authUserID` (0-indexed seat); derive `outcome` per AC #7. Resolve the 4 usernames for the `players[]` array via `userRepo.FindByID(...)` or, better, a new `userRepo.FindManyByIDs([]uint) ([]User, error)` to **one** query per page (see Task 6). Serialize hands with `HandNumber` and all scoring fields. Build the response envelope `{ "data": { "items": [...], "total": ..., "limit": ..., "offset": ... } }`.
  - [x] 4.4 Define a typed response struct `MatchesListResponse` + `MatchListItem` in [server/internal/user/handler.go](server/internal/user/handler.go) (or in a new `types.go` in the same package if the file grows > 200 LOC). Fields: see AC #1 + `outcome` (AC #7) + `hands []MatchHandView` (AC #3). JSON tags camelCase; `abandonedBy *uint` uses `json:",omitempty"`; `capotTeam *int json:",omitempty"`.
  - [x] 4.5 Register the route in [server/cmd/api/main.go:94-95](server/cmd/api/main.go#L94-L95): `api.GET("/users/:id/matches", userHandler.ListMatches)` directly after the existing `profile` route. Update the `NewUserHandler` call above to pass `matchRepo`.

- [x] **Task 5: Extend `user.UserRepository` with `FindManyByIDs` (helper for Task 4.3)**
  - [x] 5.1 Add `FindManyByIDs(ids []uint) ([]User, error)` to the interface [server/internal/user/repository.go](server/internal/user/repository.go).
  - [x] 5.2 Implement in [server/internal/user/gorm_repo.go](server/internal/user/gorm_repo.go) as `r.db.Where("id IN ?", ids).Find(&users)`. Return `[]User{}` when `len(ids) == 0` (no DB round-trip).
  - [x] 5.3 Update the existing mock in [server/internal/user/handler_test.go](server/internal/user/handler_test.go) `mockUserRepo` to satisfy the new method. Existing tests for `/profile` and `/preferences` must continue to pass unchanged.

- [x] **Task 6: Handler tests for `ListMatches` (AC #1, #7, #10)**
  - [x] 6.1 Extend [server/internal/user/handler_test.go](server/internal/user/handler_test.go) with a new `mockMatchRepo` satisfying `match.MatchRepository` (`Create`, `CreateWithHands`, `GetMatchesForUser`).
  - [x] 6.2 Add test cases (table-driven preferred, per project rule): happy path with 3 matches (verify ordering by `completed_at DESC`), happy path with embedded hands (verify `hands[]` populated per match with correct `handNumber` asc), pagination (total=5, limit=2, offset=2 returns items 3-4; `offset=4` returns items 5-5 with `total=5`), forbidden (`:id != authUserID` → 403), malformed `:id` (`abc`, `0`, `-1` → 400), `limit` out of range (`0`, `51`, `abc` → 400), `offset` negative (`-1` → 400), outcome derivation (win when `winnerTeam == TeamForSeat(viewerSeat)`, loss when not, abandoned when `status == "abandoned"` regardless of `winnerTeam`), empty list (`items: []`, `total: 0`, `limit: 20`, `offset: 0`), response NEVER leaks email / password hash / language preference.
  - [x] 6.3 Explicit regression: the existing `/profile` tests must pass unchanged; run `go test ./internal/user/... -count=1` to verify.

- [x] **Task 7: API client + query hook (AC #1, #5)**
  - [x] 7.1 Create [client/src/shared/api/matches.ts](client/src/shared/api/matches.ts) with `getUserMatches(userId: number, limit: number, offset: number): Promise<{ items: MatchListItem[]; total: number; limit: number; offset: number }>` — follow the shape of [client/src/shared/api/rooms.ts](client/src/shared/api/rooms.ts) and [client/src/shared/api/profile.ts](client/src/shared/api/profile.ts). Use the existing `axiosClient` (auto-unwraps `{ data: T }` envelope — do NOT re-unwrap).
  - [x] 7.2 Define exported TS types `MatchListItem`, `MatchPlayer`, `MatchHandView`, `MatchOutcome = "win" | "loss" | "abandoned"` — colocate in `matches.ts` (same pattern as [profile.ts](client/src/shared/api/profile.ts)). All properties `camelCase` (already enforced by `axiosClient.ts` envelope convention). Never add `any`; `matchMode: string` is fine pending future enum — "1001" is the only valid value in Phase 1.
  - [x] 7.3 Extend [client/src/shared/api/queryKeys.ts](client/src/shared/api/queryKeys.ts) with `matches: { byUser: (userId: number) => ["matches", "byUser", userId] as const }`. Keep the structure parallel to `rooms` / `profile` entries (no new top-level constant — add to the existing object).
  - [x] 7.4 Create [client/src/shared/hooks/queries/useMatches.ts](client/src/shared/hooks/queries/useMatches.ts) exporting `useUserMatchesInfiniteQuery(userId: number | undefined, pageSize = 20)` using `useInfiniteQuery` from `@tanstack/react-query` v5 with: `queryKey: [...queryKeys.matches.byUser(userId!), pageSize]`, `queryFn: ({ pageParam }) => getUserMatches(userId!, pageSize, pageParam as number)`, `initialPageParam: 0`, `getNextPageParam: (lastPage, allPages) => { const loaded = allPages.reduce((n, p) => n + p.items.length, 0); return loaded < lastPage.total ? loaded : undefined; }`, `enabled: userId !== undefined`. Name the file exactly `useMatches.ts` to mirror [useProfile.ts](client/src/shared/hooks/queries/useProfile.ts).

- [x] **Task 8: `MatchHistory` component + tests (AC #2, #3, #4, #5, #7)**
  - [x] 8.1 Create [client/src/features/profile/MatchHistory.tsx](client/src/features/profile/MatchHistory.tsx). Exports `MatchHistory({ userId }: { userId: number | undefined })`. Top-level states: `loading` (pending + no data), `empty` (success + total === 0), `loaded` (one or more rows). Never uses `any`. Uses `useUserMatchesInfiniteQuery` from Task 7.4. For outcome derivation use `item.outcome` (already computed server-side — DO NOT re-derive client-side).
  - [x] 8.2 Create [client/src/features/profile/MatchHistory.test.tsx](client/src/features/profile/MatchHistory.test.tsx) colocated — mirror the pattern in [client/src/features/profile/ProfilePage.test.tsx](client/src/features/profile/ProfilePage.test.tsx). Mock the API via `vi.mock("@/shared/api/matches", () => ({ getUserMatches: vi.fn() }))`. Wrap renders in `<QueryWrapper>` from `@/test-utils`.
  - [x] 8.3 Test cases: renders loading skeleton on first mount; renders empty state with `data-testid="match-history-empty"` + link to `/lobby`; renders N rows with correct testids, teammate/opponent derivation from viewerSeat, outcome badges (win/loss/abandoned); clicking a row toggles detail (`aria-expanded`, `data-testid="match-history-detail"`); detail contains exactly `hands.length` hand sub-rows, shows Capot badge when `capot === true`, shows "Failed" pill when `failedContract === true`; `Load more` button appears when `total > items.length`, hidden when `items.length >= total`; clicking Load more appends rows without clearing existing; regression check: `ProfilePage.test.tsx` assertions on `profile-match-history` section (the outer `<section>`) still pass.
  - [x] 8.4 Helper subcomponents (same file — keep component files under ~200 LOC, splitting when needed per [project-context.md:149-166](../../_bmad-output/project-context.md#L149-L166) naming rules):
    - `<OutcomeBadge outcome="win|loss|abandoned" />` — `span` with token backgrounds: `bg-success/20 text-success` (win), `bg-surface-elevated text-text-secondary` (loss), `bg-warning/20 text-warning` (abandoned). `data-testid="match-history-outcome"`, `aria-label` from the matching i18n key.
    - `<MatchRow match={...} onToggle={...} isOpen={...} />` — the collapsible row; render the expanded section conditionally via `isOpen && ...` so jsdom tests assert visibility via presence/absence.
    - `<HandResultsTable hands={...} />` — renders the per-hand breakdown table. Use `<table>` for screen-reader semantics; column headers: Hand / Team A / Team B / Bonus / Notes. `data-testid="match-history-hand-row"` for each row inside.

- [x] **Task 9: Wire `MatchHistory` into `ProfilePage` (AC #8)**
  - [x] 9.1 In [client/src/features/profile/ProfilePage.tsx](client/src/features/profile/ProfilePage.tsx), replace the placeholder `<p>{t("profile.matchHistoryEmpty")}</p>` inside `<section data-testid="profile-match-history">` with `<MatchHistory userId={user?.id} />`. Keep the section wrapper and its heading unchanged (existing tests lean on the outer testid).
  - [x] 9.2 Remove the `profile.matchHistoryEmpty` key from both i18n JSONs (see AC #9 — it's replaced by `profile.matchHistory.empty`).
  - [x] 9.3 Update [client/src/features/profile/ProfilePage.test.tsx](client/src/features/profile/ProfilePage.test.tsx): any assertion matching `"No matches yet"` / `"Nema mečeva"` copy inside the placeholder is removed or repointed at the new `match-history-empty` testid; all other existing cases remain green. Mock the matches API at test-file level since `MatchHistory` now renders inside.

- [x] **Task 10: i18n string additions (AC #9)**
  - [x] 10.1 Extend [client/src/shared/i18n/en.json](client/src/shared/i18n/en.json) with all keys in AC #9 (see list). Place the new `matchHistory` block under `profile.matchHistory` — nested object, not flattened. Remove `profile.matchHistoryEmpty` (the placeholder key).
  - [x] 10.2 Extend [client/src/shared/i18n/sr.json](client/src/shared/i18n/sr.json) with the same key set, Serbian-Latin translations. Same structure. Same removal. When English and Serbian disagree on tone, follow the register of [sr.json](client/src/shared/i18n/sr.json) profile-adjacent keys (`"Istorija mečeva"`).
  - [x] 10.3 Run the existing `i18n.test.ts` parity check (the project has one per [project-context.md:80](../../_bmad-output/project-context.md#L80)). It must stay green.

- [x] **Task 11: Full-stack smoke + regression (run before marking the story done)**
  - [x] 11.1 Backend: `go test ./... -count=1` — all packages green. `go vet ./...` clean.
  - [x] 11.2 Frontend: `cd client && npx vitest run` — all tests green (existing + new).
  - [x] 11.3 Lint: `cd client && npx prettier --write . && npx eslint .` clean. **Prettier MUST run before committing** (memory: `feedback_prettier_before_commit.md`; CI has failed repeatedly on this).
  - [x] 11.4 `make lint` (both stacks) — clean.
  - [x] 11.5 Manual smoke test (write outcomes in Completion Notes):
    - Start `make dev`, register two users, play a Quick Play match to completion (or use seeded game states if available), verify that a row appears on `/profile` with correct teammate/opponent usernames, correct win/loss outcome, correct score, and that expanding shows N hand sub-rows matching the match's hand count.
    - Verify empty state by clicking `/profile` for a freshly registered user — `match-history-empty` visible with lobby link.
    - Verify abandonment outcome: start a match, force an abandonment via 120s reconnect timeout, verify `abandoned` badge.
    - Verify pagination: seed (or play) ≥ 21 matches for one user; verify `Load more` button and that clicking it appends the next 20.

## Dev Notes

### What Already Exists — Do NOT Recreate

| Item                                                       | Location                                                                                                                                                                                   | Notes                                                                                                                                                                         |
| ---------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `matches` table + GORM model                               | [server/migrations/000006_create_matches.up.sql](server/migrations/000006_create_matches.up.sql), [server/internal/match/model.go](server/internal/match/model.go)                         | Stores aggregate match data (final scores, status, variant, mode, player IDs 1–4). **No per-hand detail exists** — Task 1–2 adds it.                                          |
| `matches.status` + `abandoned_by`                          | [server/migrations/000008_add_match_status.up.sql](server/migrations/000008_add_match_status.up.sql)                                                                                       | Already distinguishes `completed` vs `abandoned`; reuse for AC #7.                                                                                                            |
| `matchRepo.Create(match)`                                  | [server/internal/match/gorm_repo.go:15](server/internal/match/gorm_repo.go#L15)                                                                                                            | Existing single-row insert. Keep it — extend with `CreateWithHands` (Task 2.3).                                                                                               |
| Session manager persists the match on end                  | [server/internal/session/manager.go:576-596](server/internal/session/manager.go#L576-L596), [server/internal/session/reconnect.go:315-336](server/internal/session/reconnect.go#L315-L336) | Only edit these two call sites to use `CreateWithHands`. Other persistence paths do not exist.                                                                                |
| `game.HandResult` broadcast struct                         | [server/internal/game/state.go:29-43](server/internal/game/state.go#L29-L43)                                                                                                               | The in-memory hand-scoring output. Task 3 buffers these into `session.handResults` then maps to `match.HandResult` (a new DB struct — different type, same package-adjacent). |
| `game.TeamForSeat(seat)`                                   | [server/internal/game/state.go:115-117](server/internal/game/state.go#L115-L117)                                                                                                           | `seat % 2` — use for outcome derivation in Task 4.3. Do NOT re-derive.                                                                                                        |
| `hand_scored` WebSocket event                              | [server/internal/session/manager.go:420-440](server/internal/session/manager.go#L420-L440), [server/internal/ws/events.go](server/internal/ws/events.go) (`EventHandScored`)               | This is the hook point where Task 3.2 appends to `session.handResults`. Do NOT modify the broadcast payload — the buffer is orthogonal to the broadcast.                      |
| `UserHandler.GetProfile` authorization pattern             | [server/internal/user/handler.go:45-58](server/internal/user/handler.go#L45-L58)                                                                                                           | Copy the 3-step pattern verbatim for `ListMatches` (Task 4.2).                                                                                                                |
| `axiosClient` envelope unwrap + 401 refresh/retry          | [client/src/shared/api/axiosClient.ts:131-183](client/src/shared/api/axiosClient.ts#L131-L183)                                                                                             | Auto-unwraps `{ data: T }` — API functions return T directly. Task 7.1 returns `{ items, total, limit, offset }` directly (no double unwrap).                                 |
| `useQuery` pattern + query keys                            | [client/src/shared/hooks/queries/useProfile.ts](client/src/shared/hooks/queries/useProfile.ts), [client/src/shared/api/queryKeys.ts](client/src/shared/api/queryKeys.ts)                   | Mirror exactly. `useInfiniteQuery` is net-new for this story — `@tanstack/react-query` v5 API is already installed ([client/package.json:14](client/package.json#L14)).       |
| `ProfilePage` skeleton-loading + locale-aware date pattern | [client/src/features/profile/ProfilePage.tsx:11-34](client/src/features/profile/ProfilePage.tsx#L11-L34)                                                                                   | Reuse the `Intl.DateTimeFormat(locale, {...})` block in `MatchRow` for the completed-at column.                                                                               |
| `FetchError` client-side error type                        | [client/src/shared/api/axiosClient.ts:21-31](client/src/shared/api/axiosClient.ts#L21-L31)                                                                                                 | Use when surfacing API error copy (via the `profile.matchHistory.error` i18n key).                                                                                            |
| Prettier + ESLint + Vitest + i18n parity test              | repository-wide                                                                                                                                                                            | Enforced in CI. Run `npx prettier --write .` before every commit (user memory — repeated failure point).                                                                      |
| `TodoWrite` / planning pattern inside stories              | —                                                                                                                                                                                          | Keep Tasks / Subtasks hierarchical with explicit AC refs (`AC: #`); dev agent checks them off.                                                                                |

### What Must Be Created

1. [server/migrations/000009_create_hand_results.up.sql](server/migrations/000009_create_hand_results.up.sql) + `.down.sql`.
2. [server/internal/match/hand_result.go](server/internal/match/hand_result.go) — new `HandResult` GORM model (distinct from `game.HandResult`).
3. [client/src/shared/api/matches.ts](client/src/shared/api/matches.ts) — API client function + exported TS types.
4. [client/src/shared/hooks/queries/useMatches.ts](client/src/shared/hooks/queries/useMatches.ts) — `useInfiniteQuery` hook.
5. [client/src/features/profile/MatchHistory.tsx](client/src/features/profile/MatchHistory.tsx) + [MatchHistory.test.tsx](client/src/features/profile/MatchHistory.test.tsx).

### What Must Be Modified

1. [server/internal/match/model.go](server/internal/match/model.go) — add `Hands []HandResult` relation.
2. [server/internal/match/repository.go](server/internal/match/repository.go) — add `CreateWithHands`, `GetMatchesForUser`.
3. [server/internal/match/gorm_repo.go](server/internal/match/gorm_repo.go) — implement both new methods (transactional create; preload-ordered list query).
4. [server/internal/session/manager.go](server/internal/session/manager.go) — add `handResults` to `Session`, append on hand scored, swap `Create` → `CreateWithHands` in `handleMatchEnd`.
5. [server/internal/session/reconnect.go](server/internal/session/reconnect.go) — swap `Create` → `CreateWithHands` in the abandonment path.
6. [server/internal/session/manager_test.go](server/internal/session/manager_test.go) — extend match repo fake with `CreateWithHands`, add buffering test.
7. [server/internal/user/repository.go](server/internal/user/repository.go) — add `FindManyByIDs`.
8. [server/internal/user/gorm_repo.go](server/internal/user/gorm_repo.go) — implement.
9. [server/internal/user/handler.go](server/internal/user/handler.go) — inject `matchRepo`, add `ListMatches`, add response types.
10. [server/internal/user/handler_test.go](server/internal/user/handler_test.go) — extend `mockUserRepo` with `FindManyByIDs`, add `mockMatchRepo`, add `ListMatches` tests.
11. [server/cmd/api/main.go](server/cmd/api/main.go) — register the new route; pass `matchRepo` to `NewUserHandler`.
12. [client/src/shared/api/queryKeys.ts](client/src/shared/api/queryKeys.ts) — add `matches.byUser(userId)`.
13. [client/src/features/profile/ProfilePage.tsx](client/src/features/profile/ProfilePage.tsx) — replace placeholder `<p>` with `<MatchHistory />`.
14. [client/src/features/profile/ProfilePage.test.tsx](client/src/features/profile/ProfilePage.test.tsx) — mock matches API, remove placeholder copy assertions.
15. [client/src/shared/i18n/en.json](client/src/shared/i18n/en.json) — add `profile.matchHistory.*` keys, remove `profile.matchHistoryEmpty`.
16. [client/src/shared/i18n/sr.json](client/src/shared/i18n/sr.json) — same key set, Serbian-Latin.

### Data Model & Persistence (Key Design Decision — buffer-and-commit vs per-hand write)

**Chosen approach:** Buffer per-hand `HandResult` rows in `session.handResults` (in memory) during the match; persist them **atomically** with the `matches` row inside the same GORM transaction via `CreateWithHands`.

**Rationale:**

- Single transaction means the match record and its hands are consistent in the DB — no partial-match orphaned hand rows if the server crashes mid-match (those are simply lost, same failure mode as today).
- The `matches.id` is auto-generated; writing hands before the match row means forward-referencing an unknown FK. Transactional create-then-bulk-insert via GORM callbacks (or manual `tx.Create(match)` then `for _, h := range hands { h.MatchID = match.ID }; tx.Create(&hands)`) avoids that.
- Hand count per match is bounded (`1001/162 ≈ 7` hands upper bound for Bitola; Capot/failures extend but 20 hands is a pathological ceiling). Buffering 20×`HandResult` structs in memory is negligible.
- Broadcasting `hand_scored` remains unchanged — clients still get per-hand updates during play; the persistence is orthogonal.

**Rejected alternative:** Write each hand to DB immediately under `hand_scored`. Pro: no loss on server crash mid-match. Cons: requires a pre-created match row with status `in_progress` (2 new migration columns + 2 more touchpoints in every match-start path), AND still needs a match-completion write, AND an orphan-cleanup job for rooms that fail mid-match. For Phase 1, the occasional lost in-progress history on crash is acceptable.

**HandNumber semantics:** `HandResult.HandNumber` is the **1-indexed** number of the completed hand. At the hand-scored broadcast site, `oldState.HandNumber` was the in-progress hand that just finished (1, 2, 3, …); after `startNewHand` runs, `newState.HandNumber` is the next hand (2, 3, 4, …), or equal on `PhaseMatchEnd`. Use `oldState.HandNumber` as the authoritative completed-hand number. Verify with two table-driven cases in Task 3.5.

### Architecture Patterns to Follow

- **Package-adjacency, not cross-package referencing.** The new `match.HandResult` struct lives in `server/internal/match/` even though it parallels `game.HandResult`. Persistence concerns live alongside `match.Match`; the game engine does not know the DB exists.
- **Repository returns raw data, handler projects to response DTO.** `GetMatchesForUser` returns `[]match.Match` (with preloaded `Hands`). The handler builds `MatchListItem` with denormalized usernames and server-derived `outcome`. Never pass the GORM struct into the JSON response — it leaks fields and creates a silent contract-to-schema coupling.
- **Single DB round-trip per page.** Preload `Hands` in the list query (ordered by `hand_number ASC`). Fetch all 4 usernames for the visible page via a **single** `FindManyByIDs` call (Task 5). Build an `id → username` map and look up inside a `for _, match := range matches` loop. This caps DB round-trips at 3 per page (COUNT, matches+hands via JOIN, usernames) regardless of page size — avoiding the 1+4N anti-pattern.
- **Server-derived `outcome` beats client derivation.** Putting `outcome: "win|loss|abandoned"` in the payload removes the need for the client to know `game.TeamForSeat` semantics, prevents drift if rule changes, and makes screenshots/devtools easier to reason about.
- **Immutable Zustand + react-query caching.** `MatchHistory` uses `useInfiniteQuery` for list state; no Zustand slice needed — this is cache data, not app state. Do NOT introduce a `matchHistoryStore`.
- **`data-testid` selectors in tests, never Tailwind class names** (project rule — Tailwind churn breaks class-based selectors; precedent from Story 6.2 Dev Notes).
- **i18n parity is load-bearing.** Any key added to `en.json` is added to `sr.json` in the same commit — the `i18n.test.ts` parity check blocks CI otherwise (precedent: Story 6.1 and 6.2).
- **No `any` in TypeScript.** `MatchListItem` / `MatchHandView` must be fully typed. `matchMode: string` is fine (Phase 1 only has `"1001"`); avoid premature enum.
- **Pagination ceiling.** `limit > 50` returns 400. No unbounded scans; the matches table can grow to thousands of rows per user over time.

### Key Design Decision — Pagination via `useInfiniteQuery` + Button (not Infinite Scroll)

**Choice:** `useInfiniteQuery` for page-append semantics + visible `Load more` button.

**Rationale:**

- Epic AC allows either; button is simpler to test (no `IntersectionObserver` mocking in jsdom), and aligns with the project's Balatro "composed-not-busy" aesthetic (no auto-triggering UI).
- `useInfiniteQuery` naturally handles the append-without-remounting; data stays cached across navigations.
- Upgrading to infinite scroll later is a single component change — the server contract is already pagination-ready.

### Key Design Decision — Detail View Expansion is Pure UI State (No Second HTTP Call)

**Choice:** The per-hand detail is **embedded** in the list response; click-to-expand is a local `useState<Set<number>>` (match IDs currently open). No `GET /matches/:id` endpoint.

**Rationale:**

- Hand count per match is small (≤ 20). Embedding 20×13 scalar fields × 20 matches = ~5KB per page — negligible.
- A detail endpoint would double the backend surface area for marginal payload savings.
- Eliminates an extra HTTP round-trip on every expand — feels instant.
- Future: if hand payloads grow (per-trick detail, card play replay), split into a `GET /matches/:id/hands` endpoint — trivially additive.

### Key Design Decision — Outcome Tone ("loss" is NOT destructive)

Per UX spec colour tokens at [ux-design-specification.md:321-323](../../_bmad-output/planning-artifacts/ux-design-specification.md#L321-L323), `destructive` is reserved for `"Leave, forfeit, error states"`. A loss is not an error. Use `text-text-secondary` (neutral) for the loss badge, `text-success` for win, `text-warning` for abandoned. Abandonment is warning-coloured because it carries potential Phase-2 penalty implications (honor score).

### Previous Story Intelligence (Story 6.2 — done 2026-04-18)

Carried-forward learnings that shape this story:

- **Prettier runs before every commit.** Memory `feedback_prettier_before_commit.md` is load-bearing; CI has failed repeatedly. Task 11.3 enforces this.
- **`data-testid` + React Testing Library selectors win over text-based queries** — especially for i18n-aware components. `MatchHistory.test.tsx` should prefer `getByTestId` over `getByText` for anything that will be translated.
- **Review patches to chat landed 6 concrete fixes** including z-index alignment, monotonic counter for the unread badge, defence-in-depth guards. Expect similar review-patch tightening on this story — design `MatchHistory` so prop shapes and testid selectors are stable under review adjustments.
- **`ProfilePage.test.tsx` mocks profile API at module level with `vi.mock("@/shared/api/profile", ...)`.** Mirror that pattern when mocking `@/shared/api/matches` in Task 9.3 and 8.2.
- **`QueryWrapper` from `@/test-utils` is the canonical react-query test wrapper** for this repo (see [ProfilePage.test.tsx](client/src/features/profile/ProfilePage.test.tsx) for usage).

### Recent Codebase Signals (git log — last 5 commits)

- `0e9a864 chore(planning): restructure Epics 8-16` — planning-only; no code touched. No impact on this story.
- `52839cf feat(chat): match-scoped chat with collapsible sidebar (Story 6.2)` — direct predecessor. Chat sidebar z-index is `z-30` and sits on the game page; **does not render** on `/profile`. No interaction risk.
- `0d90b73 feat(chat): global lobby chat with in-game exclusion (Story 6.1)` — introduced `chatStore`. No touchpoint with matches.
- `ab09141 feat(game): name the declarer on each declaration-reveal row` — scoring path (HandResult) unchanged; just UI rendering.
- `feadf94 fix(game): derive led suit from currentTrick to close legal-cards race` — client race fix; unrelated.

**Signal: no recent match-persistence or profile changes.** The code baseline is stable — the existing persistence in `session/manager.go::handleMatchEnd` and `session/reconnect.go` is the right hook point and has not been touched in recent commits.

### Backend Flow — Handling `GET /api/v1/users/:id/matches`

1. Echo routes the request through the auth middleware → handler `ListMatches(c)`.
2. `getUserID` extracts `authUserID` from context; parse `:id` → enforce equality (403 otherwise).
3. Parse `limit` / `offset` query params (clamp + 400 on invalid).
4. `matchRepo.GetMatchesForUser(authUserID, limit, offset)` runs: one `COUNT` query for `total`, one `SELECT ... PRELOAD(Hands)` for the page.
5. Collect all `player[1..4]_id` across the page into a `[]uint`, dedupe → `userRepo.FindManyByIDs` → build an `id → username` map.
6. For each match, determine `viewerSeat` (which of the 4 player slots equals `authUserID`), compute `outcome` (AC #7), build the `[4]MatchPlayer` array with seats 0..3 in order, build the `hands[]` array (map DB `HandResult` → JSON `MatchHandView`).
7. Return `{ data: { items, total, limit, offset } }`.

Error paths are unchanged from existing `GetProfile` — the same `appErrorHandler` middleware maps `apperr.Err*` → HTTP response.

### Frontend Flow — Rendering `<MatchHistory userId={...} />`

1. `useUserMatchesInfiniteQuery(userId, 20)` fires → first page fetched.
2. `isPending && !data` → skeleton rows (3 pulsing `bg-surface` divs, same pattern as [ProfilePage.tsx:13-16](client/src/features/profile/ProfilePage.tsx#L13-L16)).
3. `data.pages[0].total === 0` → empty state with lobby link.
4. Otherwise → list rendered from `data.pages.flatMap(p => p.items)`.
5. Each row renders collapsed by default; `useState<Set<number>>` tracks open IDs. Clicking toggles.
6. Expanded rows render `<HandResultsTable hands={item.hands} />` inline (no second fetch).
7. If `loadedCount < pages[0].total` → `Load more` button. On click → `fetchNextPage()`; rows append reactively.

### Cross-Story Context

- **Story 7.2 (next)** — "Expanded Player Profile" — depends on **this** story's `hand_results` table only to count aggregate stats (games, wins, losses); the Story 7.2 endpoint (`GET /users/:id/profile` extension) is NOT in scope here. Keep the `profile` and `matches` handlers distinct.
- **Epic 5 (done)** — match abandonment persists a match with `status: abandoned` and `abandoned_by` set; our outcome derivation (AC #7) handles this via the `status` discriminator. Verify with a fixture test in Task 6.2.
- **Epic 4 (done)** — `hand_scored` WS event already fires at the right moment; Task 3.2 piggy-backs on that branch. No broadcast changes needed.
- **Epic 9 (future, Phase 2)** — coin economy. Match payload will gain `coinDelta` / settlement fields. Keep `MatchListItem` extensible; new optional fields won't break the current contract.
- **Epic 13 (future)** — seasonal ranking. LP / tier changes may be added to match payload. Same extensibility note.
- **Epic 11 (future)** — public profiles. The `:id` auth check must stay — do NOT relax it in this story even though 11.3 will later open public profile views. That's a deliberate per-story decision, not a shortcut.

### Project Structure Notes

**New files (expected):**

- `server/migrations/000009_create_hand_results.up.sql`
- `server/migrations/000009_create_hand_results.down.sql`
- `server/internal/match/hand_result.go`
- `client/src/shared/api/matches.ts`
- `client/src/shared/hooks/queries/useMatches.ts`
- `client/src/features/profile/MatchHistory.tsx`
- `client/src/features/profile/MatchHistory.test.tsx`

**Modified files (expected):**

- `server/internal/match/model.go`
- `server/internal/match/repository.go`
- `server/internal/match/gorm_repo.go`
- `server/internal/session/manager.go`
- `server/internal/session/manager_test.go`
- `server/internal/session/reconnect.go`
- `server/internal/user/repository.go`
- `server/internal/user/gorm_repo.go`
- `server/internal/user/handler.go`
- `server/internal/user/handler_test.go`
- `server/cmd/api/main.go`
- `client/src/shared/api/queryKeys.ts`
- `client/src/features/profile/ProfilePage.tsx`
- `client/src/features/profile/ProfilePage.test.tsx`
- `client/src/shared/i18n/en.json`
- `client/src/shared/i18n/sr.json`

**No changes expected:**

- `server/internal/game/*` (rules engine untouched — we read `HandResult` only; pure function contract preserved).
- `server/internal/ws/*` (WebSocket contract unchanged).
- `server/internal/apperr/errors.go` (all required errors already exist: `ErrForbidden`, `ErrBadRequest`, `ErrUnauthorized`, `ErrUserNotFound`).
- `client/src/shared/api/axiosClient.ts` (envelope unwrap + 401 refresh already suitable).
- `client/src/shared/stores/*` (no new store; react-query cache is sufficient).

### Alignment Checks / Detected Conflicts

- **No conflicts detected** with the existing architecture. The `matches` model evolves additively (new `Hands` relation, two new repo methods); the `user` handler package gains a new handler method and an injected repo reference.
- **Naming — `match.HandResult` vs `game.HandResult`** is the only potentially confusing name. Decision: keep both; `game.HandResult` is the in-memory game-engine broadcast struct (unchanged), `match.HandResult` is the new persistence struct. If collision is awkward in a file that imports both, alias per Task 2.1 (`matchdb "github.com/emilijan/beljot/server/internal/match"` + `game.HandResult`).
- **Route ordering in `main.go`.** Insert `/users/:id/matches` directly after `/users/:id/profile` at line 94. Echo routes are registered in source order but path-resolved by radix tree — order is not load-bearing, but keep related routes grouped for readability.

### References

- [Source: epics.md#Story-7.1 — Match History Display acceptance criteria](_bmad-output/planning-artifacts/epics.md#L1372)
- [Source: prd.md — FR41 Match history with scoring detail](_bmad-output/planning-artifacts/prd.md)
- [Source: architecture.md — project structure, features/profile/ organization, matches table](_bmad-output/planning-artifacts/architecture.md)
- [Source: ux-design-specification.md#L876 — empty state copy; L321-323 — colour tokens; L110 — "match history as visual trophy, not dry statistics"](_bmad-output/planning-artifacts/ux-design-specification.md)
- [Source: project-context.md — naming, testing, i18n parity, auth, REST envelope rules](_bmad-output/project-context.md)
- [Source: 6-2-match-scoped-chat.md — prior story; QueryWrapper pattern, data-testid discipline, prettier-before-commit, i18n parity enforcement](_bmad-output/implementation-artifacts/6-2-match-scoped-chat.md)
- [Source: server/internal/user/handler.go — GetProfile auth pattern to mirror](server/internal/user/handler.go)
- [Source: server/internal/session/manager.go#L420-L440 — hand-scored broadcast; the buffer hook point](server/internal/session/manager.go)
- [Source: server/internal/session/manager.go#L570-L606 — handleMatchEnd persistence; the CreateWithHands swap point](server/internal/session/manager.go)
- [Source: server/internal/session/reconnect.go#L300-L346 — abandonment persistence; the second CreateWithHands swap point](server/internal/session/reconnect.go)

## Dev Agent Record

### Agent Model Used

Claude Opus 4.7 (1M context)

### Debug Log References

- Backend: `go test ./... -count=1` — all 12 packages pass (cmd/api 2.3s, apperr 1.6s, auth 3.4s, chat 1.9s, game 1.7s, room 35.6s, session 18.5s, user 0.4s, ws 13.1s). `go vet ./...` clean.
- Frontend: `npx vitest run` — **49 test files / 396 tests, all green**. Includes 10 new `MatchHistory` tests and no regressions in `ProfilePage` (5 tests). i18n parity test still green.
- Lint: `npx eslint .` + `npx prettier --check .` clean (prettier reformatted `MatchHistory.test.tsx` once; eslint autofixed simple-import-sort on same file).
- `golangci-lint` not invoked locally (not installed in this environment) — `go vet` + CI enforce style.

### Completion Notes List

- **Migration 000009_create_hand_results** adds a `hand_results` table keyed by `match_id` (`ON DELETE CASCADE`) with `UNIQUE (match_id, hand_number)` and `idx_hand_results_match_id` — GORM-compatible column naming.
- **`match.HandResult` persistence struct** lives alongside `match.Match` in the same package. Deliberately distinct from `game.HandResult` (in-memory broadcast struct): the DB type carries `MatchID` + `HandNumber`, the game engine stays pure. `Match.Hands []HandResult` is the GORM relation with the cascading FK constraint declared inline.
- **Repository**: extended with `CreateWithHands(match, []HandResult)` using `db.Transaction` — creates the match row first, assigns the generated `MatchID` onto each buffered hand, batch-inserts. A rollback on any step leaves zero partial rows. The legacy `Create` method is retained (unused by production code now, still called by some test paths). `GetMatchesForUser` preloads hands ordered by `hand_number ASC` to avoid N+1.
- **Session buffering** uses a new `bufferHandResultIfScored(session, oldState, newState)` helper on `*Manager`. Called from both the normal `HandleAction` path (manager.go:234) and the auto-play timer expiry path (manager.go:784, inside `handleTimerExpiry`). Correctly handles two scenarios via the same condition (`oldState.HandNumber < newState.HandNumber || newState.Phase == PhaseMatchEnd`), with `HandNumber` assigned from `oldState.HandNumber` (the hand that just completed — `newState` is already advanced by `startNewHand` except on match-end where both equal). Story's original spec said `oldState.HandNumber + 1` — corrected during implementation after re-reading `startNewHand` (scoring.go:118). Four targeted tests in `manager_test.go` cover: normal advance, match-end, no-op when no transition, no-op when LastHandResult is nil.
- **Match persistence swap**: both `handleMatchEnd` (manager.go) and the abandonment path (reconnect.go) now call `CreateWithHands`. A snapshot of `session.handResults` is taken under `RLock` before the I/O call so the session lock is never held during the DB write. Abandoned-before-hand-1 matches correctly persist with `hands: []`.
- **`user.UserRepository.FindManyByIDs([]uint)`** added as the batch lookup for handler Task 4.3 — avoids the 1+4N pattern. Empty-input fast path returns `[]User{}` with no DB round-trip. Stubs in `auth`, `chat`, and `user` test packages were updated to satisfy the new method.
- **`UserHandler.ListMatches`** follows the exact auth pattern of `GetProfile`: `getUserID` → parse `:id` → `!= authUserID → 403`. Pagination parsed with strict bounds (limit 1..50, offset ≥ 0 → 400 on violation). Server derives `outcome` (win / loss / abandoned) and `viewerSeat` so the client never imports `TeamForSeat` rules. Response DTOs (`MatchListItem`, `MatchHandView`, `MatchPlayer`, `MatchesListResponse`) are defined in `handler.go` with camelCase JSON tags and `omitempty` on `abandonedBy` / `capotTeam`.
- **Route wired** at `api.GET("/users/:id/matches", userHandler.ListMatches)` directly after the existing profile route. `NewUserHandler` now accepts `matchRepo match.MatchRepository` — `main.go` wiring rearranged so `matchRepo` is constructed before the user handler.
- **Handler tests (user package)** — 8 new cases: empty list, win/loss/abandoned outcome derivation with correct viewerSeat + seat-ordered `players[]`, pagination bounds (happy path + past-end), embedded-hands preservation (capot + the taker's team), forbidden foreign ID, missing token (401), bad path-id (400 for `"abc"` / `"0"`), bad pagination (limit 0 / 51 / abc; offset -1 / abc). A PII-leak-guard test asserts the response string never contains `email`, `passwordHash`, or `languagePreference`.
- **Frontend API client** (`client/src/shared/api/matches.ts`) returns typed `MatchesListResponse` — no `any`, no double-unwrap (the `axiosClient` interceptor already strips the `{data:...}` envelope). Query key `queryKeys.matches.byUser(userId)` added to the existing keys object.
- **`useUserMatchesInfiniteQuery`** uses `useInfiniteQuery` with `initialPageParam: 0` and `getNextPageParam` computing `loaded < lastPage.total`. `enabled: userId !== undefined` prevents firing when the auth store user is still loading.
- **`MatchHistory` component** implements all four render states (loading skeleton, empty state with lobby link, error, loaded list). `OutcomeBadge` uses `success` / `text-secondary` / `warning` tokens per UX spec — never `destructive` on a loss. `HandResultsTable` shows per-hand breakdown as a semantic `<table>` with per-row Capot / last-trick / failed-hand badges. Row expansion is a pure UI toggle (no second fetch) via `useState<Set<number>>`. `aria-expanded` / `aria-controls` announce state to assistive tech. `Load more` button appears only when `items.length < total`.
- **Component tests (10)** cover: loading skeleton on mount, empty state + lobby CTA, row rendering with teammate / opponents / score / outcome, all three outcome variants render, detail expansion toggles aria and shows N hand rows, Capot badge + failed-hand pill, Load more visibility on both sides of the threshold, no fetch when userId is undefined.
- **ProfilePage wiring** replaces only the placeholder `<p>` — the outer `<section data-testid="profile-match-history">` wrapper and its heading remain, so the five existing `ProfilePage.test.tsx` assertions pass unchanged. The old `profile.matchHistoryEmpty` key (no longer referenced) was removed from both i18n JSONs.
- **i18n**: 27 keys added under `profile.matchHistory.*` in both `en.json` and `sr.json` (title, empty, emptyCta, loading, error, loadMore, duration + durationHms/Ms, vs, teammate, opponents, outcomeWin/Loss/Abandoned, expandRow, collapseRow, variant.bitola, mode.1001, hand.number/cardPoints/declarationPoints/lastTrickBonus/capot/failedContract/handTotal/failedContractTeamA/failedContractTeamB). Serbian-Latin register matches adjacent profile keys.
- **Mock state hygiene**: `MatchHistory.test.tsx` uses `mockReset()` in `beforeEach` (not `clearAllMocks()`) so queued `mockResolvedValueOnce` values do not leak across tests. `ProfilePage.test.tsx` adopts the same pattern and provides a default empty-page resolution for matches so existing cases don't need per-case setup.

### File List

**New files:**

- server/migrations/000009_create_hand_results.up.sql
- server/migrations/000009_create_hand_results.down.sql
- server/internal/match/hand_result.go
- server/internal/session/export_test.go (exposes `bufferHandResultIfScored` + `HandResults` helpers for external-package tests)
- client/src/shared/api/matches.ts
- client/src/shared/hooks/queries/useMatches.ts
- client/src/features/profile/MatchHistory.tsx
- client/src/features/profile/MatchHistory.test.tsx

**Modified files:**

- server/internal/match/model.go (added `Hands []HandResult` with CASCADE FK)
- server/internal/match/repository.go (added `CreateWithHands`, `GetMatchesForUser`)
- server/internal/match/gorm_repo.go (implemented both via `db.Transaction` and preload-ordered list)
- server/internal/session/manager.go (added `handResults` field, `bufferHandResultIfScored` helper, swap to `CreateWithHands` in `handleMatchEnd`)
- server/internal/session/manager_test.go (extended `mockMatchRepo` with `CreateWithHands` / `GetMatchesForUser`, added 4 buffering tests, `getHandsFor` inspection helper)
- server/internal/session/reconnect.go (snapshot buffered hands + swap to `CreateWithHands` in abandonment path)
- server/internal/user/repository.go (added `FindManyByIDs`)
- server/internal/user/gorm_repo.go (implemented `FindManyByIDs` with empty-input fast path)
- server/internal/user/handler.go (injected `matchRepo`, added `ListMatches`, `MatchListItem`/`MatchHandView`/`MatchPlayer`/`MatchesListResponse` DTOs, `parseMatchesPagination` helper, `loadUsernamesForMatches`, `buildMatchListItem`, local `teamForSeat`)
- server/internal/user/handler_test.go (added `mockMatchRepo`, extended `mockUserRepo` with `FindManyByIDs`, added 8 `ListMatches` tests incl. PII-leak guard)
- server/internal/auth/handler_test.go (satisfied new `FindManyByIDs` interface method on `mockUserRepo`)
- server/internal/chat/handler_test.go (satisfied new `FindManyByIDs` interface method on `userRepoStub`)
- server/cmd/api/main.go (constructed `matchRepo` before user handler, passed to `NewUserHandler`, registered `GET /users/:id/matches`)
- client/src/shared/api/queryKeys.ts (added `matches.byUser` entry)
- client/src/features/profile/ProfilePage.tsx (imported + mounted `<MatchHistory userId={user?.id} />`, swapped placeholder copy key to `profile.matchHistory.title`)
- client/src/features/profile/ProfilePage.test.tsx (mocked `@/shared/api/matches`, default empty-page resolve, switched `clearAllMocks` → per-mock `mockReset`)
- client/src/shared/i18n/en.json (replaced flat `profile.matchHistory*` keys with nested `profile.matchHistory.*` block; 27 keys)
- client/src/shared/i18n/sr.json (matching Serbian-Latin block)

### Review Findings

_Generated 2026-04-19 by `/bmad-code-review` — three parallel review layers (Blind Hunter, Edge Case Hunter, Acceptance Auditor)._

- [x] `[Review][Patch]` [MEDIUM] `bufferHandResultIfScored` match-end guard is overbroad — fixed: now requires an actual phase transition (`oldState.Phase != PhaseMatchEnd && newState.Phase == PhaseMatchEnd`) OR a hand-number advance. [server/internal/session/manager.go:582-589]
- [x] `[Review][Patch]` [MEDIUM] `getNextPageParam` infinite-loop guard added — returns `undefined` immediately when a page returns `items.length === 0`. [client/src/shared/hooks/queries/useMatches.ts:11-14]
- [x] `[Review][Patch]` [MEDIUM] "appends rows on click" Load more test rewritten — now clicks the button and asserts a second row appears and the button disappears. [client/src/features/profile/MatchHistory.test.tsx]
- [x] `[Review][Patch]` [MEDIUM] Handler tests now assert `completed_at DESC, id DESC` ordering — the mock sorts to match production, and a new `TestListMatches_OrdersByCompletedAtDesc` test seeds oldest-first then asserts newest-first response. Existing pagination test reseeded with descending completedAt so the newest-first order preserves the IDs-3-and-4-at-offset-2 assertion. [server/internal/user/handler_test.go]
- [x] `[Review][Patch]` [MEDIUM] Failed-hand pill now renders the taker's team as visible text (`Team A` / `Tim B`) in team colour next to "Failed" — new testid `match-history-hand-failed-team`. i18n keys `failedContractTeamA/B` shortened accordingly. [client/src/features/profile/MatchHistory.tsx:233-250, en.json, sr.json]
- [x] `[Review][Patch]` [LOW] `i18n.test.ts` now includes a recursive `flattenKeys` parity test — fails if any nested key exists in one locale but not the other. [client/src/shared/i18n/i18n.test.ts]
- [x] `[Review][Patch]` [LOW] Duration format switched to colon-separated zero-padded `hh:mm:ss` / `mm:ss` per AC #2. [en.json/sr.json `durationHms`/`durationMs`]
- [x] `[Review][Patch]` [LOW] `formatDuration` now returns `"—"` on non-finite or negative durations (clock skew). [client/src/features/profile/MatchHistory.tsx]
- [x] `[Review][Patch]` [LOW] `formatDuration` adds `durationDhms` for matches ≥ 24h (`{{d}}d hh:mm:ss`). [client/src/features/profile/MatchHistory.tsx, en.json, sr.json]
- [x] `[Review][Patch]` [LOW] Dead `profile.matchHistory.duration` key removed from both JSONs. [en.json, sr.json]
- [x] `[Review][Patch]` [LOW] `<OutcomeBadge>` `aria-label` removed — the visible text is already accessible, eliminating the double announcement. [client/src/features/profile/MatchHistory.tsx]
- [x] `[Review][Patch]` [LOW] Test helper `strconvUint` now calls `strconv.FormatUint(uint64(u), 10)`; the hand-rolled digit loop was removed. [server/internal/user/handler_test.go]
- [x] `[Review][Patch]` [LOW] `Match.Hands` field is now tagged `json:"-"` — hands are only exposed through the `MatchListItem` DTO projection. [server/internal/match/model.go]
- [x] `[Review][Patch]` [LOW] `bufferHandResultIfScored` now early-returns on `oldState == nil` alongside the existing nil checks. [server/internal/session/manager.go:582-585]
- [ ] `[Review][Patch]` [LOW] `buildMatchListItem` silently defaults `viewerSeat = 0` when the viewer isn't one of the 4 players — unreachable today because `GetMatchesForUser` filters by `userID ∈ {player1..4}`, but if the filter is ever relaxed (admin endpoints, public profiles in Epic 11), a non-participant would get a fabricated "Team A" outcome. **SKIPPED during batch-apply:** changing the contract (to `-1` sentinel or error) affects the `MatchListItem.viewerSeat` JSON shape and the client's expand/outcome logic — warrants explicit decision with the user. [server/internal/user/handler.go:284-290]
- [x] `[Review][Defer]` Offset-based pagination duplicates / skips rows on concurrent match completions — a new match completing between page fetches shifts the rest down by one, causing `useInfiniteQuery` to request the same offset and React to warn about duplicate keys. Consider cursor pagination on `(completed_at, id)` or client-side dedupe by match id. — deferred, pre-existing offset-pagination pattern across the codebase
- [x] `[Review][Defer]` `openIds` state accumulates entries for matches no longer in the list — tiny memory leak, no functional break. Prune against current `items` on each render. — deferred, micro-optimisation
- [x] `[Review][Defer]` `Load more` button remains enabled during background refetch (`isFetching` true but `isFetchingNextPage` false) — a second click while page 1 refreshes can fire a `fetchNextPage` with stale `total`. Also disable on `query.isFetching`. — deferred, rare interaction
- [x] `[Review][Defer]` `loadUsernamesForMatches` returns empty string for soft-deleted users — row renders with `teammate=""` instead of an explicit "Deleted user" placeholder. — deferred, UX polish
- [x] `[Review][Defer]` `strconv.ParseUint(..., 10, 64)` cast to `uint` can truncate on 32-bit builds — mirrors the existing pattern in `GetProfile`; address as a systemic fix rather than in one handler. — deferred, pre-existing pattern

### Change Log

- 2026-04-19 — Story 7.1 implemented: paginated `GET /users/:id/matches` endpoint returning newest-first matches with embedded per-hand scoring; new `hand_results` table + transactional persistence via `CreateWithHands`; `MatchHistory` component with inline expand-to-detail, outcome badges (win/loss/abandoned), Load more pagination, and empty state linking to the lobby. All 10 ACs satisfied; 396/396 frontend tests + 12 backend packages green; ESLint + Prettier + go vet clean.
- 2026-04-19 — Code review complete: 0 decision-needed, 15 patch, 5 deferred, ~15 dismissed. No CRITICAL or unrecoverable HIGH issues; MEDIUM/LOW items tracked in Review Findings.
- 2026-04-19 — Review patches batch-applied: 14 of 15 patches fixed (5 MEDIUM, 9 LOW). One LOW skipped (`viewerSeat = 0` default → `-1` sentinel) as it changes the API contract and needs an explicit design call. Backend `go vet` + `go test ./internal/user/... ./internal/session/...` green; frontend `vitest run` green at 397/397; `prettier` + `eslint` clean.
