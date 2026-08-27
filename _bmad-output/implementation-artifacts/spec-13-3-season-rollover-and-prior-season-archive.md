---
title: 'Story 13.3: Season Rollover & Prior-Season Archive'
type: 'feature'
created: '2026-08-27'
status: 'done'
review_loop_iteration: 0
baseline_commit: a7c1679f3a745a2da9b7b0959ac8e7fb08260794
context: ['{project-root}/_bmad-output/project-context.md', '{project-root}/_bmad-output/implementation-artifacts/epic-13-context.md']
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Seasons only roll over when traffic happens to hit the lazy resolver; a lobby tab left open across the quarter boundary renders the dead season forever (deferred-work obligation from 13.1); prior-season records accumulate invisibly — no archive on profiles, no current rank in the public profile response; and the leaderboard cannot show past seasons even though 13.2 reserved `?season=` for exactly this.

**Approach:** A nightly idempotent rollover job that simply drives the existing self-healing resolver; `GET /api/v1/seasons` plus a real `?season=<id>` selector and picker on the leaderboard page; a per-player archive endpoint feeding an archive section on both profile pages; a `seasonRank` block on both profile DTOs; and client-side boundary invalidation with one-time "new season" transition copy.

## Boundaries & Constraints

**Always:**
- Reads never create `player_seasons` rows; only the `seasons` window may be lazily created (contract at `repository.go:39-45`).
- Every `tier` on the wire is derived via `season.TierForSP(sp)` — never the stored `rank_tier` column (13.1 D7).
- Archive membership is **row exists ∧ `games_played >= 1` ∧ season ended (`ends_at <= now`)** — NOT `sp > 0`. Do not reuse `leaderboardScope`: a played season with 0 SP stays in the archive.
- `seasonName` (`"2026 Q3"`) is a machine token rendered verbatim, never translated.
- Quarter math only via `QuarterBounds`/`QuarterName` in Go — no SQL-side quarter arithmetic.
- Import edges: `user → season` is now permitted; `season` must never import `user`; `match` must never import `season`/`user`. Convert `season/gorm_repo_test.go` to `package season_test` in the same change that adds any `user → season` edge (test-package import cycle otherwise).
- The rollover job follows the hub idiom: `done chan struct{}` + `Shutdown()`, started from `main.go`, stopped during graceful shutdown; failures are logged via `slog` and the job keeps ticking.
- Explicit `users.deleted_at IS NULL` on any table-name join (GORM soft-delete scope does not apply there).
- New i18n keys land in all four locales in the same commit; no em dash in mk/sr/hr.

**Ask First:**
- Any new migration or index (investigation says 000024's schema and indexes suffice; a `player_seasons(user_id)` index only if measured necessary).
- Any change to the lobby leaderboard widget beyond staying current-season.
- Mirroring the archive into `CareerResponse` or any surface not named here.

**Never:**
- No ELO, placement matches, or tier sub-divisions — retired model.
- No WebSocket push for rollover, standings, or archive — pull-only; the boundary is handled by client-side invalidation.
- No merging, compression, or mutation of prior-season SP — `player_seasons` rows are immutable once their season ends; soft reset is a new `season_id`.
- No season picker on the lobby widget; no user-existence 404 in the archive endpoint (the profile query owns that).

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Rollover past boundary | job fires, no row covers `now` | exactly one new quarter row (`QuarterName`); rerun is a no-op (`uq_seasons_started_at`) | errors logged, ticker continues |
| Rollover mid-season | covering row exists | no write | N/A |
| Archive happy path | user played 2 ended seasons + current | 2 rows newest-first `{seasonId, seasonName, sp, tier, gamesPlayed, startedAt, endsAt}`; active season excluded | N/A |
| Archive 0-SP season | ended row, `sp=0`, `games_played=1` | row included | N/A |
| Archive empty | no ended played seasons | `{items: []}`; client renders no archive section at all | N/A |
| Archive unknown user | `:id` with no rows | `{items: []}` (200) | N/A |
| seasonRank present | active-season row exists for subject | `seasonRank: {seasonName, tier, sp}` in own and public profile | N/A |
| seasonRank absent | no active-season row | `seasonRank: null`; client hides rank chip | N/A |
| Leaderboard past season | `?season=<id>` of an ended season | that season's standings; viewer block under the same `sp > 0` rule | N/A |
| Leaderboard unknown season | `?season=999` | no body | `apperr.ErrSeasonNotFound` (404) |
| Leaderboard bad selector | `?season=abc`, `?season=-1` | no body | `apperr.ErrBadRequest` |
| Seasons list | `GET /api/v1/seasons` | all seasons newest-first `{id, name, startedAt, endsAt}` | N/A |
| Boundary in open tab | `endsAt` passes on the shared time tick | `season.current` + leaderboard queries invalidated once per boundary; banner flips to the new season; one transition toast | N/A |

</frozen-after-approval>

## Code Map

**Server — season package (extend, no new package)**
- `server/internal/season/quarter.go:22,42` -- `QuarterBounds(now)`, `QuarterName(start)` — pure, tested; next window is `QuarterBounds(current.EndsAt)` (half-open windows abut).
- `server/internal/season/gorm_repo.go:34-64` -- `CurrentSeason(now)`: read-first `findCovering(:70-83)`, then `INSERT ... ON CONFLICT (started_at) DO NOTHING` + re-read. **The rollover job is a thin wrapper over this call** — the idempotency recipe already exists and is tested (`gorm_repo_test.go:102` `TestCurrentSeason_LazilyCreatesAndIsIdempotent`).
- `server/internal/season/repository.go:18-111` -- 6 existing methods; add `FindSeasonByID`, `ListSeasons`, `PlayerSeasonArchive`. Read/write contract doc at `:39-45` — extend it to cover the new reads.
- `server/internal/season/gorm_repo.go:223-229` -- `leaderboardScope`: the table-name-join + explicit `users.deleted_at IS NULL` idiom. The archive query copies the join style but NOT the `sp > 0` membership.
- `server/internal/season/service.go:51,120,172` -- `resolveSeason`, `CurrentSeasonView`, `LeaderboardView` (add a by-id season path), `viewerPosition(:244)`.
- `server/internal/season/handler.go:168-201` -- `parseLeaderboardQuery` already reserves `season`; today only `""`/`"current"` pass. Extend the allowlist to positive integers. DTO/envelope/`getUserID(:48)` patterns; routes at `main.go:386,394`.
- `server/internal/season/tier.go:90` -- `TierForSP`.
- `server/internal/season/model.go:28-47` -- `PlayerSeason` (has `GamesPlayed`/`GamesCompleted`; `:40-44` semantics — `games_played` increments for every finished match). Add `ArchiveEntry` scan target here.
- `server/migrations/000024_create_seasons_and_player_seasons.up.sql:27-85` -- schema; `uq_seasons_started_at UNIQUE` is the idempotency anchor; `idx_player_seasons_user_season` serves the archive read. Highest migration = 000024; **no new migration expected**.
- `server/internal/ws/hub.go:135-137` + `server/cmd/api/main.go:406-427` -- the only graceful-stop idiom (`done` chan + `Shutdown()`; signal handling, 10s timeout). `ws/client.go:109-114` — the only ticker+select shape. Model the rollover job on these.
- `server/internal/apperr/errors.go` -- no season block yet; add `ErrSeasonNotFound` (404) in a new "Season domain errors" group.

**Server — user package (profile rank)**
- `server/internal/user/handler.go:24-108` -- `ProfileResponse` and `PublicProfileResponse`; both gain `seasonRank`. Privacy contract at `:17-23,77-85` — seasonRank is public-safe.
- `server/internal/user/handler.go:292,308,382` -- `NewUserHandler(userRepo, matchRepo)`; `GetProfile` branches self/public at `:382`. Inject a narrow season reader (precedent: `HonorRecorder`); fake it locally in `user/handler_test.go`.
- `server/cmd/api/main.go:153,226-228` -- **wiring order**: `userHandler` is built at `:153`, season repo/service at `:226`. Hoist season construction above the user handler.
- `server/internal/season/gorm_repo_test.go:1,14` -- `package season` (in-package) and imports `user` → converting to `package season_test` is MANDATORY with the new edge. `makeSeason(:54)` uses far-future quarters (2098/2099) — reuse for archive fixtures.
- `server/internal/user/handler_test.go:954-1068` -- public-projection leak tests (`NotContains` pattern); extend for seasonRank. Exact-wire-keys pattern to copy: `season/handler_test.go:251,519`.
- `server/internal/season/handler_test.go:24-53,199` -- `mockRepo` (extend for the 3 new methods or handler tests break).

**Client**
- `client/src/shared/api/season.ts:13,31` -- `getCurrentSeason`, `getSeasonLeaderboard(limit, offset)` (doc at `:22-23` names this story). Add `getSeasons()`, `getSeasonArchive(userId)`, thread a `season` selector param.
- `client/src/shared/api/profile.ts:4-72` -- `ProfileResponse`/`PublicProfileResponse` TS types (NOT in apiTypes.ts); add `seasonRank`.
- `client/src/shared/api/queryKeys.ts:12-24,63-80` -- `season.current()`, `season.leaderboard(limit)` → key must gain the season selector; add `season.list()`, `season.archive(userId)`.
- `client/src/shared/hooks/queries/useSeasonLeaderboard.ts:30,54` -- polled widget hook (stays current-only) + infinite page hook (gains season param).
- `client/src/shared/lib/seasonTier.ts:83,116-162` -- `normalizeSeasonTier`, tier color maps, `seasonDaysRemaining`; header `:113-114` names "13.3 season-archive list" as the duplication these prevent.
- `client/src/shared/components/season/TierBadge.tsx:51` -- reusable at `size="sm"`. `LeaderboardRow.tsx:71,20,101` — NOT reused for archive rows; copy its a11y recipe (one sr-only sentence, cells `aria-hidden`, `finiteOrZero`).
- `client/src/features/profile/PublicPlayerProfilePage.tsx:181-182` -- the reserved slot (comment names this story); testids `profile-season` and `prior-season-archive` are pre-agreed in `PublicPlayerProfilePage.test.tsx:233-242`, which must be **inverted** (keep a hidden-when-empty variant). Mirror into `ProfilePage.tsx:160-193` (above the main/aside grid). `IdentityHero.tsx:225-228` — second reserved comment.
- `client/src/features/profile/components/SectionHeader.tsx` -- section shell for the archive.
- `client/src/features/leaderboard/LeaderboardPage.tsx` -- season picker mounts here, fed by `getSeasons()`, default current, newest-first. `client/src/shared/components/ui/chips.tsx` — recent pill-picker precedent (used by `CreateRoomModal.tsx`); a native select is also acceptable — implementer's choice.
- `client/src/features/lobby/components/RankBanner.tsx:35,83-92` -- consumes `endsAt` + shared 30s `timeTick` (`seasonDaysRemaining` floors at 0 forever — the deferred bug). Boundary effect lives here: invalidate `season.current` + leaderboard keys once per boundary; toast on observed season change (reuse the `season.tierUp.toast` plumbing — grep its usage).
- `client/src/shared/hooks/useWsDispatch.ts:463` -- existing `event:season_points_awarded` invalidation; do not add events.
- `client/src/shared/i18n/en.json:1374+` -- `season` namespace (last block; all four files are 1421 lines). Add `season.archive.*`, `season.picker.*`, `season.banner.newSeason`. Parity gates: `i18n.parity.test.ts:63-102`, `i18n.test.ts:124-139`. **Edit locale files only with file tools — the Bash console is cp1251 and crashes on Cyrillic/diacritics.**
- `client/src/features/profile/ProfilePage.test.tsx:52`, `PublicPlayerProfilePage.test.tsx:34-58` -- fixtures gain `seasonRank`; new season API module needs a `vi.mock` entry.

## Tasks & Acceptance

**Execution:**
- [x] `server/internal/season/model.go` -- add `ArchiveEntry{SeasonID, SeasonName, StartedAt, EndsAt, SP, GamesPlayed}` scan target.
- [x] `server/internal/season/repository.go` -- add `FindSeasonByID(id uint)`, `ListSeasons()`, `PlayerSeasonArchive(userID uint, now time.Time)`; document that none create player rows and archive excludes the active window.
- [x] `server/internal/season/gorm_repo.go` -- implement all three; archive joins `seasons` by table name, filters `games_played >= 1 AND seasons.ends_at <= now`, orders `started_at DESC`.
- [x] `server/internal/season/rollover.go` (new) -- `Rollover` with `Start()`/`Shutdown()`: immediate run then 24h ticker calling `repo.CurrentSeason(time.Now().UTC())`; slog outcome; clock injectable for tests.
- [x] `server/internal/season/service.go` -- `LeaderboardView` gains a season-by-id path (`ErrSeasonNotFound` on miss); add `ArchiveView(userID, now)` (derive tier per row) and `CurrentSeasonRank(userID, now)` returning nil when no row.
- [x] `server/internal/season/handler.go` -- extend `parseLeaderboardQuery` to accept positive-integer ids; add `GetSeasons` (`GET /seasons`) and `GetPlayerSeasonArchive` (`GET /users/:id/seasons`) with camelCase DTOs, empty slices as `[]`.
- [x] `server/internal/apperr/errors.go` -- new Season block with `ErrSeasonNotFound` (404).
- [x] `server/internal/user/handler.go` -- `seasonRank` (nullable) on both profile DTOs via a narrow season-reader interface; populate in both `GetProfile` branches.
- [x] `server/cmd/api/main.go` -- hoist season construction above the user handler; inject the reader; register both routes beside `:386-394`; start the rollover job and stop it in the shutdown path after `hub.Shutdown()`.
- [x] `server/internal/season/gorm_repo_test.go` -- convert to `package season_test`; add integration tests: archive filter (0-SP kept, active excluded, unknown user empty, newest-first), `ListSeasons` order, `FindSeasonByID` miss, rollover idempotency (two runs → one row).
- [x] `server/internal/season/handler_test.go` + `rollover_test.go` -- extend `mockRepo`; wire-keys-exact tests for both new payloads; every selector-validation row of the I/O matrix; job Start/Shutdown race-free under `-race`.
- [x] `server/internal/user/handler_test.go` -- seasonRank present/null on both branches; extend the public-projection leak test with the new key.
- [x] `client/src/shared/api/season.ts` + `profile.ts` + `queryKeys.ts` -- new API fns, `seasonRank` on both TS types, season-aware leaderboard keys, `season.list()`/`season.archive(userId)`.
- [x] `client/src/shared/hooks/queries/useSeasonArchive.ts` (new) + `useSeasonLeaderboard.ts` -- archive/list hooks; infinite hook gains season param (widget hook untouched).
- [x] `client/src/shared/components/season/SeasonArchiveRow.tsx` (+test) -- `TierBadge size="sm"` + verbatim `seasonName` + localized tier label + `sp.toLocaleString()` + games; LeaderboardRow's a11y recipe.
- [x] `client/src/features/profile/components/SeasonSection.tsx` (+test) -- current-rank chip (testid `profile-season`, hidden when `seasonRank` null) + archive list (testid `prior-season-archive`, section absent from DOM when empty); mount in `ProfilePage.tsx` and `PublicPlayerProfilePage.tsx`; invert the absence assertions, keep hidden-when-empty variants.
- [x] `client/src/features/leaderboard/LeaderboardPage.tsx` (+test) -- season picker (newest-first, default current); ended-season view is not polled.
- [x] `client/src/features/lobby/components/RankBanner.tsx` (+test) -- boundary effect: once per boundary invalidate `season.current` + leaderboard queries; one `season.banner.newSeason` toast when the season id changes.
- [x] `client/src/shared/i18n/en.json`, `sr.json`, `hr.json`, `mk.json` -- `season.archive.*`, `season.picker.*`, `season.banner.newSeason`; parity green; no em dash in mk/sr/hr.

**Acceptance Criteria:**
- Given a server running past a quarter boundary with zero traffic, when the nightly job fires, then the new season row exists exactly once and repeated runs change nothing.
- Given a player with ended played seasons, when either profile page renders, then the archive lists them newest-first with derived tiers; given none, the section is absent from the DOM.
- Given any profile response, when serialized, then `seasonRank` appears (object or null) and no private field joins it in the public shape.
- Given `/leaderboard`, when a prior season is picked, then that season's standings and viewer position render; default remains the current season and the lobby widget never changes seasons.
- Given a lobby tab open across the boundary, when the countdown reaches zero, then the banner flips to the new season without a reload and the transition toast fires once.
- Given `make lint` and `make test`, when they run, then both exit 0 with i18n parity and existing season/user suites green.

## Spec Change Log

## Design Notes

**The job is a wrapper, not a scheduler framework:** `CurrentSeason(now)` already computes the covering quarter, inserts with `ON CONFLICT (started_at) DO NOTHING`, and re-reads. The nightly job exists so a zero-traffic deployment still gets its row (and logs prove it ran) — correctness never depends on it because the lazy resolver self-heals. Keep it to ~60 lines: ticker, one repo call, slog, done-chan.

**Why the archive and the leaderboard disagree about membership:** the ladder is "SP earners only" (`sp > 0`, 13.2's resolved rule); the archive is "seasons you actually played" (`games_played >= 1`). A player who finished matches but earned 0 SP must appear in their own history while staying off the ladder. Two predicates, two documented homes — never share the scope helper.

**Why `user` gets a narrow reader instead of calling season handlers:** the profile is one response assembled server-side; a second client round-trip for rank would leak the composition to every consumer. The interface lives in `user` (like `HonorRecorder`), season's `Service` satisfies it, `main.go` wires it — keeping `season` ignorant of `user` and the import DAG acyclic.

## Verification

**Commands:**
- `make lint` -- expected: exit 0 (both stacks).
- `make test` -- expected: exit 0; both i18n parity tests green; `-race` on the rollover tests.
- `cd server && go test ./internal/season/... ./internal/user/... -v` -- expected: integration tests RUN (not SKIP) against the dev DB on 5433, covering the archive predicate and rollover idempotency.

**Manual checks (if no CLI):**
- Insert a season row ending in the past + a played `player_seasons` row for a test user; own and public profile show the archive; leaderboard picker shows both seasons; picking the ended one renders its standings.
- With the lobby open, set the active season's `ends_at` to now+1min; at zero the banner flips and the toast fires once.

## Suggested Review Order

**Rollover: a wrapper over a resolver that already self-heals**

- Start here: the whole job is one idempotent repo call; the ticker is scaffolding
  [`rollover.go:101`](../../server/internal/season/rollover.go#L101)
- Guarded on both sides: a second Start is a no-op, a Start after Shutdown runs nothing
  [`rollover.go:63`](../../server/internal/season/rollover.go#L63)
- Boot-anchored 24h ticker, not wall-clock "3am" — the comments now say what the code does
  [`main.go:238`](../../server/cmd/api/main.go#L238)

**Archive membership: deliberately NOT the ladder's rule**

- `games_played >= 1` and the window ended — a played 0-SP season belongs in history
  [`gorm_repo.go:378`](../../server/internal/season/gorm_repo.go#L378)
- The hand-written `users.deleted_at IS NULL` a Table/Joins query never gets for free
  [`gorm_repo.go:378`](../../server/internal/season/gorm_repo.go#L378)
- Tier derived per row via `TierForSP`; the stored `rank_tier` is never trusted
  [`service.go:337`](../../server/internal/season/service.go#L337)

**Request boundary**

- Ids parse at bit size 32: a 64-bit parse cast to uint would alias a real id
  [`handler.go:179`](../../server/internal/season/handler.go#L179)
- Unknown subject is a 200 empty archive, never a 404 — user existence is the profile's job
  [`handler.go:333`](../../server/internal/season/handler.go#L333)

**The `user → season` edge this story opens**

- The consumer declares the narrow need; `season` still never imports `user`
  [`handler.go:32`](../../server/internal/user/handler.go#L32)
- Season construction hoisted above the user handler so the reader can be injected
  [`main.go:402`](../../server/cmd/api/main.go#L402)

**Boundary observation: two surfaces, one rule**

- Keyed on `endsAt` so the refetched window re-arms it; the `at` stamp escapes clock skew
  [`RankBanner.tsx:73`](../../client/src/features/lobby/components/RankBanner.tsx#L73)
- The page watches the NEWEST known window: at the boundary nothing covers "now"
  [`LeaderboardPage.tsx:84`](../../client/src/features/leaderboard/LeaderboardPage.tsx#L84)
- Past seasons get past-tense copy: "nobody has earned SP yet" is false about a finished quarter
  [`LeaderboardPage.tsx:110`](../../client/src/features/leaderboard/LeaderboardPage.tsx#L110)
- `> 1`, not `> 0`: a single already-selected chip is a control that cannot do anything
  [`LeaderboardPage.tsx:329`](../../client/src/features/leaderboard/LeaderboardPage.tsx#L329)

**Hidden-when-empty, without deleting what is on screen**

- Row count is the only gate: a failed refetch must not drop a rendered archive
  [`SeasonSection.tsx:57`](../../client/src/features/profile/components/SeasonSection.tsx#L57)

**Tests that pin what nearly shipped wrong**

- The second rollover in one session — a fire-once boolean would pass everything else
  [`RankBanner.test.tsx`](../../client/src/features/lobby/components/RankBanner.test.tsx)
- A deleted account's history is unreadable, proven present-then-absent
  [`gorm_repo_test.go`](../../server/internal/season/gorm_repo_test.go)
- Start twice, Start after Shutdown — both silent no-ops
  [`rollover_test.go`](../../server/internal/season/rollover_test.go)
