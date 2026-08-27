---
title: 'Story 13.2: Seasonal Leaderboard'
type: 'feature'
created: '2026-08-27'
status: 'done'
review_loop_iteration: 0
baseline_commit: 30a943dcc188a8e733e70196ba426682af47f033
context: ['{project-root}/_bmad-output/project-context.md', '{project-root}/_bmad-output/implementation-artifacts/epic-13-context.md']
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Story 13.1 shipped SP accrual, the 8-tier ladder and the lobby RankBanner, so a player can see *their own* standing — but there is no way to see anyone else's. `player_seasons` already carries every player's SP and `idx_player_seasons_season_sp` was shipped in 000024 specifically for this read, yet no endpoint, no lobby panel and no page consume it. Without a leaderboard the quarterly re-climb loop has no social target.

**Approach:** One new authenticated read endpoint `GET /api/v1/leaderboard?season=current` over the existing `season` package and `seasonService`, returning an offset-paginated, SP-ordered page plus the viewer's own position. Three consumers: a top-10 lobby right-panel widget, a full paginated `/leaderboard` page reached from a new top-nav tab, and a shared row/badge pair extracted from `RankBanner` so the tier treatment is defined once.

## Boundaries & Constraints

**Always:**
- **Tier on the wire is derived, never the stored column.** Sort and filter in SQL by `sp`; compute every response `tier` with `season.TierForSP(sp)`. 13.1 D7: `rank_tier` is a denormalized snapshot and is never authoritative.
- **One total order, used by both halves of the response:** `ORDER BY sp DESC, user_id ASC`. The viewer's `position` MUST be counted under that exact same order, or a tied viewer gets a position that contradicts the list they are standing in.
- **Soft-deleted users are excluded, in the list AND in the viewer count, by the same predicate.** A `Joins`/`Table` query gets no GORM soft-delete scope, so `users.deleted_at IS NULL` must be written explicitly (see `internal/room/gorm_repo.go:298` for the live leak this avoids).
- Reuse `seasonRepo`/`seasonService` from `main.go:226-227`; register the route beside `main.go:386`. Reuse the season package's existing `getUserID` (`handler.go:47`).
- Client tier colour comes only from `SEASON_TIER_COLOR`/`_SOFT`/`_LINE` in `shared/lib/seasonTier.ts`, applied by inline `style` — the ramp is a runtime value and cannot be a Tailwind class (`index.css:245-249`). Run every server-supplied tier token through `normalizeSeasonTier` before rendering.
- Envelope follows the one existing paginated precedent verbatim: `{ items, total, limit, offset }` inside `{ "data": ... }`, item slices built with `make(..., 0, n)` so an empty result serializes `[]`.
- i18n: new keys land in **all four** locales; no em dash in `mk`/`sr`/`hr`. Both parity tests must stay green.

**Ask First:**
- Adding an avatar, level, or honor field to a leaderboard row (no avatar column exists; level would mean loading `total_xp` and deriving).
- Any new migration or index — investigation confirms 000024's two indexes are sufficient.
- Making the endpoint reachable unauthenticated (there is no optional-auth middleware; it would need a second public route).

**Never:**
- **No WebSocket push.** Pull-only: page load plus poll. Do not add an event to `events.go`/`wsEvents.ts`, and do not invalidate the leaderboard from `useWsDispatch`.
- No `season` to `user` Go import (13.3 will likely make `user` import `season`; join by table name and scan into a local struct).
- No `RANK() OVER` or window function over the season — the position is a bounded `COUNT` of rows ordering strictly ahead.
- No `season=<id>` prior-season selector, no rollover, no profile archive — Story 13.3.
- No new shadcn primitive (`table`, `pagination`, `skeleton`, `card` are all absent by choice); no numbered pager — the codebase's pagination idiom is load-more.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Lobby widget | `?season=current` (no limit) | `limit` defaults to 10; 10 rows, `position` 1..10 | N/A |
| Explicit paging | `?season=current&limit=20&offset=40` | rows 41..60; `position` = `offset + index + 1` | N/A |
| Bad `limit` | `limit=0`, `limit=51`, `limit=abc` | no body | `apperr.ErrBadRequest` |
| Bad `offset` | `offset=-1`, `offset=x` | no body | `apperr.ErrBadRequest` |
| Bad season selector | `season=2026Q1`, `season=7` | no body | `apperr.ErrBadRequest` (only `current` or absent accepted) |
| Empty season | no `player_seasons` rows yet | `{items: [], total: 0, ...}`, `viewer: null` | N/A |
| Viewer never played | authed, no `player_seasons` row | `viewer: null` | N/A |
| Viewer played, 0 SP | row exists, `sp = 0` | `viewer: null` (AC: highlight only with *any* SP) | N/A |
| Viewer on-page | viewer's row is inside the returned page | `viewer.position` equals that row's `position` | N/A |
| Tied SP | three players at 900 SP | positions 1,2,3 by ascending `user_id`; a tied viewer's `position` matches its own list slot exactly | N/A |
| Soft-deleted player | a top-SP user with `deleted_at` set | omitted from `items`, excluded from `total`, and not counted in any `position` | N/A |
| Grandmaster row | `sp = 20000` | `tier: "grandmaster"` | N/A |
| Unknown tier token (skew) | server sends a tier the client build lacks | `normalizeSeasonTier` falls back from `sp`; row still renders | N/A |
| Offset past end | `offset` greater than `total` | `items: []`, `total` unchanged | N/A |

</frozen-after-approval>

## Code Map

**Server — extend, do not create a package**
- `server/internal/season/repository.go:8-47` -- `Repository` has exactly 3 methods; add 2. Its read/write contract doc (`:33-45`) is why a read must never create a `player_seasons` row.
- `server/internal/season/gorm_repo.go:85-97` -- `FindPlayerSeason`, reused for the viewer block. `findCovering` (`:70-83`) shows the query idiom.
- `server/internal/season/service.go:120-154` -- `CurrentSeasonView` is the exact shape to mirror: `resolveSeason` (`:51-61`, the nil guard) then a repo read, returning the handler's DTO directly.
- `server/internal/season/handler.go:24-33,69-83` -- `CurrentSeasonView` DTO plus `GetCurrentSeason`. Auth via unexported `getUserID` (`:47-57`); service failure becomes a wrapped `fmt.Errorf` and then a 500 via `appErrorHandler`.
- `server/internal/season/tier.go:90-99` -- `TierForSP(sp) string`. The only authoritative tier source.
- `server/internal/user/handler.go:819-860` -- `parseMatchesQuery`: the ONLY pagination precedent. `limit`/`offset`, `defaultLimit`/`maxLimit` consts, enum params via a `switch` allowlist, every violation to `apperr.ErrBadRequest`. Envelope at `:227-233`.
- `server/internal/match/gorm_repo.go:194-216` -- `Count` then `Limit`/`Offset` off one reused builder, ordering always tie-broken by a second column for stable paging.
- `server/migrations/000024_create_seasons_and_player_seasons.up.sql` -- `idx_player_seasons_season_sp (season_id, sp DESC)` and `idx_player_seasons_user_season`. Read-only evidence: **no new index is needed**.
- `server/cmd/api/main.go:223-228,380-386` -- `seasonRepo`/`seasonService`/`SetSPAwarder`; the comments at `:225` and `:383` already reserve the slot for this route. The `api` group (`:154`) is the authenticated one.
- `server/internal/room/gorm_repo.go:298` -- read-only: a `Table("users")` query missing `deleted_at IS NULL`. The hole this story must not copy.
- `server/internal/season/handler_test.go:24-39,93-104,126-174` -- `mockRepo` (adding interface methods breaks it), the `call()` harness setting `c.Set("userID", ...)`, and `TestGetCurrentSeason_WirePayloadKeysAreExact`, the map-decode gate to replicate.
- `server/internal/season/gorm_repo_test.go:18-59` -- `getTestDB` (per-test `Begin` plus `Rollback`, skips when no DB), `makeUser`, `makeSeason` (far-future 2095+ windows).

**Client — extract first, then consume twice**
- `client/src/features/lobby/components/RankBanner.tsx:57,66-80` -- the tier badge is inline JSX: `size-11` with inner `size-5`, `SEASON_TIER_SOFT` ground, `1px solid color` border, `0 0 14px -2px color` glow, `data-tier` for testability. Extract verbatim.
- `client/src/shared/lib/seasonTier.ts:55,83,116-149` -- `seasonSpOrZero`, `normalizeSeasonTier`, and the three token maps. The header (`:104-115`) names this story as the duplication it exists to prevent.
- `client/src/shared/api/season.ts:13-15` -- `getCurrentSeason`; add beside it. The client is `axiosClient` (there is **no** `fetchClient`); its interceptor unwraps `{data}` (`axiosClient.ts:210-217`).
- `client/src/shared/api/queryKeys.ts:59-65` -- `season.current()`; add `season.leaderboard(...)` in the same style.
- `client/src/shared/hooks/queries/useCurrentSeason.ts:11-27` -- the banner hook, deliberately **not** polled. Contrast `useLobbyStats.ts` (`REFETCH_INTERVAL_MS` as a named const with rationale plus `refetchOnWindowFocus: true`) — that is the pattern the widget copies, at a much longer interval.
- `client/src/shared/hooks/queries/useMatches.ts:14-32` -- `useInfiniteQuery` with an offset `pageParam` and `getNextPageParam` off `total`. The page's hook.
- `client/src/features/profile/MatchHistory.tsx:96-153` -- the full four-branch body: `animate-pulse` skeleton, error `<p>`, dashed empty box, `<ul>` plus load-more button plus a "showing N of M" caption. Copy this shape.
- `client/src/features/profile/ProfilePage.tsx:162-193` -- the only main/aside split precedent: `grid grid-cols-1 gap-5 lg:grid-cols-[minmax(0,1fr)_320px] lg:items-start` with `<aside className="... lg:sticky lg:top-20">`.
- `client/src/features/lobby/LobbyPage.tsx:220,229-239,241` -- root `mx-auto max-w-330 px-7 py-8 pb-32`; a single-column stack, **no split exists yet**. `RankBanner`/`FriendList` stay full-width; the split wraps `FilterRail`, `RoomGrid` and the footnote from `:241`.
- `client/src/features/friends/FriendList.tsx:44-47,82-89` -- panel wrapper `bg-surface border-border mb-3.5 rounded-lg border p-3.5`, `<ul>`/`<li>` rows with `data-user-id`. No shared skeleton or table primitive exists.
- `client/src/shared/components/TopBar.tsx:22-26,134-156,290-314` -- `navItems` is data-driven; one entry feeds desktop **and** the mobile dropdown. `data-testid` derives from `labelKey.split(".")[1]`. Read-only warning at `:106-121`: the md..lg band is width-constrained and a fourth tab lands exactly there.
- `client/src/App.tsx:87-99` -- `/leaderboard` must sit inside `ProtectedRoute` then `AppLayout` to get the TopBar. The path is confirmed free; there is no lazy loading anywhere.
- `client/src/shared/stores/authStore.ts:19` -- `useAuthStore((s) => s.user)` for the viewer's username on the pinned row.
- `client/src/shared/i18n/en.json:14-22,1373-1395` -- the `nav` block; `season` is the last namespace and has no `leaderboard` keys. Parity gates: `i18n.parity.test.ts:63-102` and `i18n.test.ts:124-139`.
- **Operational trap for the i18n edits:** this project's Bash console is cp1251, so printing Cyrillic or diacritics crashes the command and can half-apply a multi-file edit loop. Read and write the four locale JSON files with the file-editing tools only, never via `cat`, `grep`, `sed` or a heredoc in a shell.
- Read-only: `client/src/shared/components/TopBar.test.tsx`, `AppLayout.test.tsx`, `App.test.tsx` assert the existing nav test-ids, so adding a tab may require updating them.

## Tasks & Acceptance

**Execution:**
- [x] `server/internal/season/model.go` -- add `LeaderboardEntry{UserID uint; Username string; SP int; GamesPlayed int}` as the repo scan target -- keeps the joined shape off the wire DTO and off `PlayerSeason`.
- [x] `server/internal/season/repository.go` -- add `LeaderboardPage(seasonID uint, limit, offset int) (entries []LeaderboardEntry, total int64, err error)` and `CountAhead(seasonID uint, sp int, userID uint) (int64, error)`; document that both apply the identical visibility predicate and the identical total order, and that neither writes.
- [x] `server/internal/season/gorm_repo.go` -- implement both against `player_seasons` joined to `users` with an explicit `users.deleted_at IS NULL`; one reused builder feeding `Count` then an ordered `Limit`/`Offset` `Scan`; `ORDER BY sp DESC, user_id ASC`. `CountAhead` counts `sp > ? OR (sp = ? AND user_id < ?)`.
- [x] `server/internal/season/service.go` -- add `LeaderboardView(userID uint, limit, offset int, now time.Time)`: `resolveSeason`, then `LeaderboardPage`, deriving each row's `tier` via `TierForSP` and `position` via `offset+i+1`, then the viewer block from `FindPlayerSeason` plus `CountAhead`, `nil` when the row is absent or `sp == 0`.
- [x] `server/internal/season/handler.go` -- add `LeaderboardRowView`/`LeaderboardViewerView`/`LeaderboardView` DTOs (camelCase tags), `parseLeaderboardQuery` (`defaultLimit = 10`, `maxLimit = 50`, `season` allowlist), and `GetLeaderboard` mirroring `GetCurrentSeason`'s auth, error and envelope handling.
- [x] `server/cmd/api/main.go` -- register `api.GET("/leaderboard", seasonHandler.GetLeaderboard)` immediately after `:386` -- no new repo or service construction.
- [x] `server/internal/season/handler_test.go` -- extend `mockRepo` with the two new methods; add wire-key exactness (map decode, both nested shapes), every I/O-matrix param-validation case, the viewer-null cases, tie-position agreement, and a `repo.rows` emptiness assertion proving the read wrote nothing.
- [x] `server/internal/season/gorm_repo_test.go` -- integration tests for SP ordering, the `user_id` tiebreak, soft-deleted-user exclusion from items *and* `total`, offset paging with no duplicates or gaps, and `CountAhead` agreeing with the row's list position for tied and untied players.
- [x] `client/src/shared/components/season/TierBadge.tsx` (plus `.test.tsx`) -- extract `RankBanner.tsx:66-80` verbatim behind `tier` and `size` props; keep `data-tier` and `aria-hidden`.
- [x] `client/src/features/lobby/components/RankBanner.tsx` -- replace the inline badge with `<TierBadge>` -- no visual change; the existing `rank-badge` test-id and assertions must still pass.
- [x] `client/src/shared/types/apiTypes.ts` -- add `LeaderboardRow`, `LeaderboardViewer`, `LeaderboardResponse`; type `tier` as `string` (not the union), matching `CurrentSeasonResponse.rankTier`'s skew guard.
- [x] `client/src/shared/api/season.ts` -- add `getSeasonLeaderboard(limit: number, offset: number)` sending `season=current` -- beside `getCurrentSeason`.
- [x] `client/src/shared/api/queryKeys.ts` -- add `season.leaderboard: (limit, offset) => ["season","leaderboard",limit,offset]`.
- [x] `client/src/shared/hooks/queries/useSeasonLeaderboard.ts` -- export a polled `useSeasonLeaderboardQuery(limit)` for the widget (a named `REFETCH_INTERVAL_MS` const carrying its rationale) and `useSeasonLeaderboardInfiniteQuery(pageSize)` for the page, modelled on `useMatches.ts`.
- [x] `client/src/shared/components/season/LeaderboardRow.tsx` -- one row renderer shared by widget and page: position, `TierBadge`, username, SP (`toLocaleString` plus `tabular-nums`), games played; `data-user-id`, plus `data-self` and a tinted ground when it is the viewer.
- [x] `client/src/features/lobby/components/LeaderboardPanel.tsx` (plus `.test.tsx`) -- the top-10 widget in the `FriendList` card idiom with loading, error and empty branches and a `Link` to `/leaderboard`.
- [x] `client/src/features/lobby/LobbyPage.tsx` -- wrap `FilterRail`, `RoomGrid` and the footnote in the `ProfilePage` grid split and put `LeaderboardPanel` in the `<aside>`; `HeroBlock`, `RankBanner` and `FriendList` stay full-width.
- [x] `client/src/features/leaderboard/LeaderboardPage.tsx` (plus `.test.tsx`) -- the full page: `SectionHeader`, the four `MatchHistory` body branches, load-more with "showing N of M", and the viewer's row pinned below the list when `viewer` is set but off-page.
- [x] `client/src/App.tsx` -- add `<Route path="/leaderboard" element={<LeaderboardPage />} />` inside `ProtectedRoute` then `AppLayout`.
- [x] `client/src/shared/components/TopBar.tsx` -- add `{ path: "/leaderboard", labelKey: "nav.leaderboard" }` to `navItems`; verify the md..lg band still does not scroll horizontally.
- [x] `client/src/shared/i18n/en.json`, `sr.json`, `hr.json`, `mk.json` -- add `nav.leaderboard` and a `season.leaderboard` block (`eyebrow`, `title`, `sub`, `loading`, `error`, `empty`, `loadMore`, `showing`, `viewAll`, `you`, column labels and aria strings) in all four; keep parity, and no em dash in mk/sr/hr.
- [x] `client/src/shared/components/TopBar.test.tsx` and `client/src/App.test.tsx` -- update the nav and route assertions the new tab touches.

**Acceptance Criteria:**
- Given a signed-in player in the lobby, when the page renders, then the right panel shows the top 10 by SP with position, username, tier badge and SP, and the panel links to `/leaderboard`.
- Given the "Leaderboard" tab in the top nav (desktop row and mobile menu), when it is activated, then `/leaderboard` renders the full list with load-more paging and the active-tab styling matches the other tabs.
- Given the viewer has SP in the active season, when the leaderboard renders, then their own row is visually marked, and it is pinned into view if it falls outside the loaded pages.
- Given the viewer has no SP or no season row, when the leaderboard renders, then no own-row marker and no pinned row appear, and nothing errors.
- Given standings change because matches completed, when the lobby is revisited or the widget's poll fires, then the new order is shown, with no WebSocket event added in either contract file.
- Given `make lint` and `make test`, when they run, then both exit 0 with no new i18n-parity or WS-golden failures.

## Spec Change Log

### 2026-08-27 — ladder membership resolved (review iteration 1, no loopback)

**Triggering finding:** all three review layers independently found the same contradiction. `leaderboardScope` filtered only on `season_id` and `users.deleted_at IS NULL`, so a player holding a `player_seasons` row at 0 SP was listed with a real `position` and counted in `total` — while `viewerPosition` returned `nil` for `record.SP <= 0`, telling that same player they had no standing. Two further defects were direct consequences: `LeaderboardPanel` marked the viewer's own row from `authStore` while `LeaderboardPage` marked it from the server `viewer` block (so a 0-SP player saw their row marked in the lobby and unmarked on the full page), and the empty-state copy in all four locales ("Nobody has earned Season Points yet this season") became false the moment one 0-SP row existed.

**Root cause:** this spec. The frozen I/O matrix pins `viewer: null` at 0 SP but never states whether 0-SP rows appear in the list, and the two are coupled. Classified `intent_gap` (root cause inside `<frozen-after-approval>`), which normally forces a revert and replan.

**What was amended:** nothing inside the frozen block. The owner resolved the ambiguity directly on 2026-08-27: **the ladder is SP earners only** — `leaderboardScope` gains `player_seasons.sp > 0`, so the list, `total` and `CountAhead` all exclude 0-SP rows. This keeps the frozen matrix's `viewer: null` row true rather than contradicting it, makes the empty-state copy accurate, and collapses the two self-marking rules into one. Recorded here rather than in the frozen block because the frozen block is human-owned.

**Deliberate deviation from the workflow:** the code was NOT reverted before resolving this. The ambiguity was contained to one SQL predicate and its tests, so re-deriving ~30 verified files was disproportionate. The fix shipped as patch P1 alongside the other review patches.

**Known-bad state avoided:** a leaderboard whose own two surfaces disagree about who the viewer is, whose empty state lies, and which advertises "top SP earners" while listing players who have earned none.

**KEEP — what worked and must survive any future re-derivation:**
- The single `leaderboardScope` helper as the ONLY place the visibility predicate is written. The list, `total` and `CountAhead` all routing through it is what made this defect a one-line fix instead of three divergent fixes. Never inline the predicate at a call site.
- Deriving `tier` from `TierForSP(sp)` on every row rather than reading the stored `rank_tier` column, and `TestGetLeaderboard_TierIsDerivedNotTheStoredColumn` which pins it.
- Counting the viewer's position under the list's full total order (`sp > ? OR (sp = ? AND user_id < ?)`) rather than `COUNT(sp > ?)`, and `TestLeaderboardCountAhead_AgreesWithEveryRowsListPosition` which proves the count and the list cannot drift.
- The explicit `users.deleted_at IS NULL` on a `Table`/`Joins` query, which gets no GORM soft-delete scope.
- `TestGetLeaderboard_WirePayloadKeysAreExact` (map-decode, not struct-decode) as the wire-contract gate.
- Assertions must check `len(Items)` and `total`, not only the field under test. This defect survived the first pass precisely because `TestGetLeaderboard_ViewerWithZeroSPIsNull` asserted `Viewer` alone.

## Design Notes

**Why the viewer's position is a `COUNT` under the list's own order, not `RANK()` or `COUNT(sp > x)`:**
`COUNT(*) WHERE season_id = ? AND sp > vSP` gives every tied player the *same* number, but the list numbers them `offset+i+1` — so a viewer tied at 900 SP could be told "position 4" while standing in slot 6 of the page they are looking at. Counting rows that sort strictly ahead under the full order closes that:

```go
// One order, two consumers: the list's ORDER BY and this COUNT must agree.
err := q.Where("(player_seasons.sp > ? OR (player_seasons.sp = ? AND player_seasons.user_id < ?))",
    sp, sp, userID).Count(&ahead).Error
// position = ahead + 1
```

`q` is the same builder as the list query: same join, same `users.deleted_at IS NULL`. If the two predicates ever diverge, the viewer's position silently drifts from the list.

**Why join by table name instead of importing `user`:** `season` importing `user` would block 13.3 from putting seasonal rank on the public profile (`user` would then import `season`, a cycle). Joining `users` by name and scanning into `LeaderboardEntry` keeps both directions open, at the cost of losing GORM's soft-delete scope — hence the explicit predicate, which is the one thing a reviewer should check first.

**Why the viewer block carries no username:** the viewer is the authenticated caller, so the client already holds their name in `authStore`; shipping it would mean a third query for the one row whose name is never in doubt. `LeaderboardRow` takes `username` as a prop, so the pinned row renders through the same component.

## Verification

**Commands:**
- `make lint` -- expected: exit 0 (client `tsc --noEmit` plus ESLint plus Prettier; server `golangci-lint run ./...` with zero findings)
- `make test` -- expected: exit 0; `go test ./...` green in every package and `npx vitest run` green with both i18n parity tests passing
- `cd server && go test ./internal/season/... -run Leaderboard -v` -- expected: the integration tests actually run (not `SKIP`) against the dev DB on 5433, proving the join and the soft-delete predicate were exercised
- `cd server && go vet ./... && gofmt -l ./internal ./cmd` -- expected: no output

**Manual checks (if no CLI):**
- Lobby at 768-1023px: four nav tabs with no horizontal scroll (the `TopBar.tsx:106-121` regression band); the leaderboard aside collapses under the room grid below `lg`.
- The tier badge in a leaderboard row and in the RankBanner show the same colour and glow for the same tier, and both re-root correctly on the `.game-table` felt scope.

## Suggested Review Order

**The membership rule and the one predicate behind it**

- Start here: one scope is the ONLY place visibility is written, so `sp > 0` was a one-line fix
  [`gorm_repo.go:223`](../../server/internal/season/gorm_repo.go#L223)

- The contract that binds all three readers to that scope and to one total order
  [`repository.go:58`](../../server/internal/season/repository.go#L58)

- Why the viewer's rank is `CountAhead + 1` under the list's order, not `COUNT(sp > mine)`
  [`gorm_repo.go:332`](../../server/internal/season/gorm_repo.go#L332)

- Exists only because `FindPlayerSeason` leaks soft-deleted and 0-SP rows into the viewer block
  [`gorm_repo.go:313`](../../server/internal/season/gorm_repo.go#L313)

- Position and derived tier assembled here; stored `rank_tier` is never trusted
  [`service.go:172`](../../server/internal/season/service.go#L172)

- Three misses collapse to one nil: no row, zero SP, soft-deleted account with a live JWT
  [`service.go:244`](../../server/internal/season/service.go#L244)

**Request boundary**

- Bounds live here; `offset` gained the ceiling it never had, documented as a bound not a target
  [`handler.go:159`](../../server/internal/season/handler.go#L159)

- Empty `limit=` reads as absent, matching `parseMatchesQuery`; the doc now says so
  [`handler.go:173`](../../server/internal/season/handler.go#L173)

- Defence in depth: a negative limit would otherwise panic `make`, and `Limit(-1)` returns the season
  [`gorm_repo.go:266`](../../server/internal/season/gorm_repo.go#L266)

- Reuses the season service already built for 13.1; pull-only rationale sits at the route
  [`main.go:387`](../../server/cmd/api/main.go#L387)

**Wire shape**

- `viewer` is deliberately name-free: the caller already knows their own username
  [`handler.go:114`](../../server/internal/season/handler.go#L114)

- Envelope mirrors the one paginated precedent in the codebase
  [`handler.go:134`](../../server/internal/season/handler.go#L134)

**Client: the two surfaces must agree**

- Both surfaces now mark self from the server `viewer` block; this one used `authStore` and disagreed
  [`LeaderboardPanel.tsx:39`](../../client/src/features/lobby/components/LeaderboardPanel.tsx#L39)

- A single transient poll failure used to collapse a populated widget to one error line
  [`LeaderboardPanel.tsx:46`](../../client/src/features/lobby/components/LeaderboardPanel.tsx#L46)

- Standing and total read the LAST page; page 0 went stale as the reader paged deeper
  [`LeaderboardPage.tsx:62`](../../client/src/features/leaderboard/LeaderboardPage.tsx#L62)

- Driven by `hasNextPage`, not `items.length < total` — the two can disagree and stall the button
  [`LeaderboardPage.tsx:115`](../../client/src/features/leaderboard/LeaderboardPage.tsx#L115)

- Dedupes by `userId`: offset paging can repeat a row when SP shifts between fetches
  [`LeaderboardPage.tsx:34`](../../client/src/features/leaderboard/LeaderboardPage.tsx#L34)

**Shared rendering, extracted so the tier is defined once**

- One `sr-only` sentence is the row's whole accessible name; every visible cell is `aria-hidden`
  [`LeaderboardRow.tsx:101`](../../client/src/shared/components/season/LeaderboardRow.tsx#L101)

- An SP helper must not sanitize a match count
  [`LeaderboardRow.tsx:20`](../../client/src/shared/components/season/LeaderboardRow.tsx#L20)

- A lookup, not interpolation: Tailwind v4 scans source text, so `size-${n}` never compiles
  [`TierBadge.tsx:16`](../../client/src/shared/components/season/TierBadge.tsx#L16)

**Fetching and placement**

- The widget polls because standings have no push path; the banner beside it deliberately does not
  [`useSeasonLeaderboard.ts:21`](../../client/src/shared/hooks/queries/useSeasonLeaderboard.ts#L21)

- Offset advances by rows actually loaded, so a short page cannot skip or repeat a row
  [`useSeasonLeaderboard.ts:62`](../../client/src/shared/hooks/queries/useSeasonLeaderboard.ts#L62)

- Keyed by page size only; the widget and the page are separate cache entries
  [`queryKeys.ts:79`](../../client/src/shared/api/queryKeys.ts#L79)

- The lobby's first main/aside split; everything above it stays full-width
  [`LobbyPage.tsx:278`](../../client/src/features/lobby/LobbyPage.tsx#L278)

- Inside `ProtectedRoute` and `AppLayout`, so the page is auth-gated and keeps the TopBar
  [`App.tsx:95`](../../client/src/App.tsx#L95)

- One entry feeds the desktop row and the mobile menu; the md..lg width band was re-measured
  [`TopBar.tsx:33`](../../client/src/shared/components/TopBar.tsx#L33)

**Tests that pin what nearly shipped broken**

- The assertion whose absence let the 0-SP contradiction through: now checks `Items` and `Total`
  [`handler_test.go`](../../server/internal/season/handler_test.go)

- Proves the count and the list can never drift, for tied and untied players
  [`gorm_repo_test.go`](../../server/internal/season/gorm_repo_test.go)

- Renders the real `<App />` at `/leaderboard`; every prior test declared its own route
  [`App.test.tsx`](../../client/src/App.test.tsx)

- The only place the client's URL and params meet the server's parser
  [`season.test.ts`](../../client/src/shared/api/season.test.ts)

- Asserts the panel is actually mounted in the lobby, and mocks the season API it now calls
  [`LobbyPage.test.tsx`](../../client/src/features/lobby/LobbyPage.test.tsx)
