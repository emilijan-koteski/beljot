---
title: 'Per-player results for abandoned matches + early-end disclaimer'
type: 'feature'
created: '2026-07-09'
status: 'done'
review_loop_iteration: 0
baseline_commit: '34b4dbef544e39c00b06b628ee4d2e88eaefd729'
context: ['{project-root}/_bmad-output/project-context.md']
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** When a player abandons a match (disconnect timeout / rage quit — anything but a proper surrender), the match is marked `abandoned` for all four players in stats and history. This stains the three players who stayed and blocks the future honour system (FR56), which must attribute abandonment to the abandoner only.

**Approach:** Make outcomes per-player: abandoner → "abandoned"; partner → "loss"; opponents → "win"; the three non-abandoners get an "ended early" disclaimer in match history. Surrendered matches (already win/loss) get an analogous but **distinct** disclaimer — two separate wordings, one for abandonment ("ended early, a player abandoned"), one for surrender ("ended early by surrender"). Persist the non-abandoning team as `winner_team` on abandonment (already computed for coin settlement) and backfill historical rows.

## Boundaries & Constraints

**Always:**
- Coin settlement and XP behavior stay byte-identical (9.2/9.5 team-forfeit rules are settled, separate decisions).
- Abandoned rows with `abandoned_by IS NULL` (boot-reconcile legacy) stay outcome "abandoned" for all participants; `winner_team` on abandoned rows is meaningful ONLY when `abandoned_by IS NOT NULL` — every query gates on that.
- Migration `000015` has a fully-reversing `.down.sql`.
- JSON `camelCase`; i18n keys in all four files (`en`, `sr`, `mk`, `hr`); no em dash in mk/sr/hr strings.

**Ask First:**
- Any WS contract change (`events.go`/`wsEvents.ts`) — design needs none.
- Any change to career aggregates (streaks/partners/rivals) — they stay completed-only.

**Never:**
- No honour score computation/storage (future epic).
- No new abandonment triggers — sole trigger stays reconnect-window expiry.
- No change to surrender persistence (`status` stays `completed` + `surrendered_by`).
- Don't touch XP/coin formulas (`computeSettlement`/`computeXPAwards`).

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Abandoner views stats/history | `abandoned_by = viewer` | outcome `abandoned`; abandoned stat +1 | N/A |
| Partner views | abandoned, abandoner's team, not abandoner | outcome `loss` + abandonment marker; losses +1 | N/A |
| Opponent views | abandoned, other team | outcome `win` + abandonment marker; wins +1 | N/A |
| Legacy/reconcile row | abandoned, `abandoned_by IS NULL` | outcome `abandoned` for everyone | N/A |
| Surrendered match in history | completed + `surrendered_by` | win/loss chip + distinct surrender marker; stats unchanged | N/A |
| Natural completion | completed, no `surrendered_by` | unchanged, no marker | N/A |
| Live abandonment overlay | `event:match_abandoned` at remaining players | partner sees "ended early, counts as a loss"; opponents "counts as a win" (client-side from `abandonedByPlayer` seat parity) | N/A |
| History outcome filters | filter `win`/`loss`/`abandoned` | filters agree with new per-viewer semantics | N/A |

</frozen-after-approval>

## Code Map

- `server/migrations/` -- highest is `000014`; seats map player1/3 = team 0, player2/4 = team 1
- `server/internal/match/reconnect.go:511-655` -- abandonment finalize; winner computed at :589, persists `WinnerTeam: 0` at :619-647
- `server/internal/match/reconcile.go:63-137` -- boot-reconcile abandons, nil `abandoned_by` (leave as-is)
- `server/internal/match/gorm_repo.go:58-154` -- `GetStatsForUser`, `GetMatchesForUser` (outcome filter SQL, `viewerTeamCase`)
- `server/internal/user/handler.go:559-564` -- outcome DTO mapping
- `client/src/shared/api/matches.ts` -- `MatchListItem`, `MatchOutcome`
- `client/src/features/profile/MatchHistory.tsx` -- `OutcomeChip` :54-88, row :390-520
- `client/src/features/match/components/ReconnectOverlay.tsx:136-205` -- abandonment end panel
- `client/src/shared/i18n/{en,sr,mk,hr}.json`

## Tasks & Acceptance

**Execution:**
- [x] `server/migrations/000015_backfill_abandoned_winner_team.{up,down}.sql` -- up: set `winner_team` = team opposite the abandoner (via seat parity) `WHERE status='abandoned' AND abandoned_by IS NOT NULL`; down: reset those rows to 0
- [x] `server/internal/match/reconnect.go` -- persist the computed non-abandoning team as `WinnerTeam` (replace hardcoded 0; update :632-634 comment)
- [x] `server/internal/match/gorm_repo.go` -- stats: wins/losses also count abandoned rows where `abandoned_by IS NOT NULL AND abandoned_by <> viewer` by `winner_team`; abandoned = `abandoned_by = viewer OR abandoned_by IS NULL`; mirror in `GetMatchesForUser` outcome filter
- [x] `server/internal/user/handler.go` -- outcome mapping per new rules; add `endReason: "natural"|"surrender"|"abandonment"` to match-list DTO
- [x] server tests (existing homes, e.g. `gorm_repo_test.go`, handler tests) -- table-driven cases covering the I/O matrix; update abandonment-persist assertion to real `WinnerTeam`
- [x] `client/src/shared/api/matches.ts` -- add `endReason` to `MatchListItem`
- [x] `client/src/features/profile/MatchHistory.tsx` -- muted "ended early" marker (`data-testid="match-history-ended-early"`, `data-end-reason`) when `endReason !== "natural"` and outcome not `abandoned`; text switches on endReason (abandonment vs surrender wording)
- [x] `client/src/features/match/components/ReconnectOverlay.tsx` -- one result line in abandoned panel: loss text if viewer shares team with `abandonedByPlayer` seat, else win text
- [x] `client/src/shared/i18n/en.json` `sr.json` `mk.json` `hr.json` -- add `profile.matchHistory.endedEarlyAbandonment`, `profile.matchHistory.endedEarlySurrender`, `match.disconnect.abandonCountsWin`, `match.disconnect.abandonCountsLoss`
- [x] co-located FE tests (`MatchHistory.test.tsx`, `ReconnectOverlay.test.tsx`) -- marker presence + wording per endReason; overlay line per viewer seat

**Acceptance Criteria:**
- Given a pre-existing abandoned row with known abandoner, when migration 000015 runs, then `winner_team` is the team opposite the abandoner; NULL-`abandoned_by` rows untouched.
- Given production abandoned rows created before this change (abandoner recorded since the original 5-5 feature), when the migration is applied and the new code deploys, then their stats/history outcomes retroactively become per-player with no manual data fixes; unattributable rows stay `abandoned` for all four players.
- Given an abandonment ends a live match, when settlement and XP events fire, then payloads are identical to pre-change behavior.
- Given the three non-abandoners list history, then rows carry `endReason: "abandonment"`; surrendered matches carry `"surrender"`.

## Design Notes

Persisting `winner_team` (vs deriving abandoner-team in every query) chosen because settlement already computes it, SQL stays uniform, one backfill covers history. The `abandoned_by IS NOT NULL` gate keeps legacy reconcile rows (filler `winner_team=0`) as plain "abandoned". No WS change: `ReconnectOverlay` derives win/loss from existing `abandonedByPlayer` + viewer seat (display derivation, not game logic).

**Production data:** `abandoned_by` has been persisted since abandonment shipped (column exists in `000005`, written by the reconnect finalizer), so nearly all prod abandoned rows are attributable — the backfill retroactively upgrades them; only boot-reconcile rows (nil abandoner, zero scores) remain "abandoned for everyone", which is the correct honest fallback. **Deploy order is load-bearing:** run `make migrate` (000015) BEFORE restarting the server with new code — new queries reading un-backfilled `winner_team=0` would misclassify team-0 partners as winners. Down path: `.down.sql` + old code restores today's behavior exactly.

## Verification

**Commands:**
- `cd server && BELJOT_DB_URL="postgres://beljot:beljot_dev_password@localhost:5433/beljot?sslmode=disable" go test ./...` (mise-shimmed go) -- expected: all pass
- `cd client && npx vitest run` -- expected: all pass incl. i18n parity test
- `make lint` -- expected: clean

**Manual checks (if no CLI):**
- Apply migration 000015 to dev DB (port 5433); spot-check an abandoned row's `winner_team`.
- Production rollout: `make migrate` must complete before the server restarts with the new code (see Design Notes).

## Suggested Review Order

**Winner attribution (entry point)**

- Design core: abandonment now persists the real non-abandoning team instead of filler 0
  [`reconnect.go:637`](../../server/internal/match/reconnect.go#L637)

- Backfill for historical rows; seat-parity CASE, gated on attributable abandoner; deploy-order note
  [`000015_backfill_abandoned_winner_team.up.sql:1`](../../server/migrations/000015_backfill_abandoned_winner_team.up.sql#L1)

- Fully-reversing down path, pairs with reader-code rollback; re-up self-heals
  [`000015_backfill_abandoned_winner_team.down.sql:1`](../../server/migrations/000015_backfill_abandoned_winner_team.down.sql#L1)

**Per-player outcome derivation (server)**

- Stats FILTERs: wins/losses absorb attributed abandoned rows; abandoned = own + unattributable
  [`gorm_repo.go:76`](../../server/internal/match/gorm_repo.go#L76)

- History outcome filter mirrors the same predicates so chips and filters agree
  [`gorm_repo.go:119`](../../server/internal/match/gorm_repo.go#L119)

- Per-viewer outcome switch with explicit status guard; abandoner-only "abandoned"
  [`handler.go:573`](../../server/internal/user/handler.go#L573)

- New `endReason` DTO field feeding the frontend disclaimers
  [`handler.go:583`](../../server/internal/user/handler.go#L583)

- Interface contract docs updated to the new semantics
  [`repository.go:70`](../../server/internal/match/repository.go#L70)

**History disclaimer (client)**

- Optional `endReason` — version-skew tolerant against older server responses
  [`matches.ts:53`](../../client/src/shared/api/matches.ts#L53)

- Marker positive-matches abandonment/surrender only; distinct wording per reason
  [`MatchHistory.tsx:499`](../../client/src/features/profile/MatchHistory.tsx#L499)

**Live overlay result line (client)**

- Guards: valid seats 0-3, viewer is not the abandoner; parity decides win/loss
  [`ReconnectOverlay.tsx:160`](../../client/src/features/match/components/ReconnectOverlay.tsx#L160)

- The rendered line, loss vs win wording
  [`ReconnectOverlay.tsx:223`](../../client/src/features/match/components/ReconnectOverlay.tsx#L223)

- Viewer seat wired from the match store
  [`MatchPage.tsx:1865`](../../client/src/features/match/MatchPage.tsx#L1865)

**Peripherals (tests, i18n)**

- DB-backed table-driven tests: stats matrix, filter matrix, migration replay (fixture-scoped)
  [`gorm_repo_test.go:147`](../../server/internal/match/gorm_repo_test.go#L147)

- Both seat parities of the live persist covered
  [`reconnect_test.go:575`](../../server/internal/match/reconnect_test.go#L575)

- Four new keys x four languages; no em dash outside en
  [`en.json:88`](../../client/src/shared/i18n/en.json#L88)

- Marker + overlay component tests incl. skew and abandoner-viewer cases
  [`MatchHistory.test.tsx:1`](../../client/src/features/profile/MatchHistory.test.tsx#L1)
