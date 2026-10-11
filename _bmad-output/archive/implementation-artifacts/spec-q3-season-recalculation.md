---
title: 'Recalculate 2026 Q3 standings with the win/loss formula'
type: 'feature'
created: '2026-09-26'
status: 'done'
baseline_commit: 'a290a7c8294ce081443d77a490c4bde50902e766'
route: 'dispatch'
review_loop_iteration: 0
context: ['{project-root}/_bmad-output/implementation-artifacts/spec-13-4-competitive-sp-formula.md', '{project-root}/_bmad-output/implementation-artifacts/spec-13-5-divisions-demotion-and-sp-feedback.md']
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** 2026 Q3's standings were scored by the old climb-only formula: 16 production rows with 101–30,841 SP, 9 of them at or above the new Grandmaster floor. A deploy before 2026-10-01 00:00 UTC would show them on the new floors and overwrite their stored ranks one match at a time, leaving Q3 mixed. The owner wants Q3 recalculated with the new formula, so those ranks come down (request 2026-09-26).

**Approach:** Replay the stored 2026 Q3 matches in completion order through the live 13.4 formula and ladder. This is the same replay `cmd/sptune` ran for the approved constants, with the replay core moved into `internal/season` so the server and `sptune` share it. Write each player's recalculated SP, tier, division and match counters into their Q3 `player_seasons` row in one transaction. Any later Q3 match then continues from those totals. As a consequence, Q3 gets divisions ("Silver 3"), superseding 13.5's "pre-Q4 seasons have no division" for Q3 only.

## Boundaries & Constraints

**Always:** the replay reads `matches` (and `hand_results`), never the old `player_seasons` totals, so any old-formula write that lands before the recalculation is overwritten. It uses `DefaultSPFormula()` and the live ladder, applies the 0 floor exactly like `ApplySeasonPoints`, and runs in ONE transaction that locks the rows it writes in ascending user-ID order. Idempotent: running it twice yields the same rows. The Q4 reset (000027) is unchanged.

- **Decision (2026-09-26): replay window = all of 2026 Q3** (completed_at in [2026-07-01, 2026-10-01)), reproducing the approved replay. Players who only played before SP tracking began get a Q3 row; `games_played` / `games_completed` are recomputed from the replayed matches.
- **Decision (2026-09-26): runs automatically, once, on the deploy that ships it.** Migration 000029 queues a one-time job for the season starting 2026-07-01; the server runs pending jobs at startup, after migrations and before it accepts connections, writes the rows and marks the job done in the same transaction. A failure is logged and the server still starts; the job stays pending and retries on the next start.
- **Decision (2026-09-26): the owner deploys before 2026-10-01.** The rest of Q3 then scores live with the new formula from the recalculated totals; 000027 finds no Q4 rows and Q4 starts clean.

**Never:** no change to seasons other than 2026 Q3; no change to the formula, the constants or the floors; no deletion of the `seasons` row.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected |
|---|---|---|
| Regular | kiro's Q3 matches (dump of 2026-09-26) | row sp 534, silver, division 3, as the approved replay |
| Pre-tracking player | only played before 2026-08-27, no Q3 row | row created with the replay's result |
| Old write before recalc | an old-formula award lands between migrate and start | overwritten by the replay |
| Abandoner | a Q3 abandonment | −120 applied in order; `games_completed` excludes it |
| Reconcile placeholder | `abandoned`, `abandoned_by` NULL | skipped, as live |
| Run twice | job already done | nothing runs; a forced re-run yields identical rows |
| No season row | fresh database without a 2026 Q3 season | job marked done, nothing written |

</frozen-after-approval>

## Code Map

- `server/cmd/sptune/replay.go` -- `loadWindow`, `outcomeFor` (surrender / instant-win inference, placeholder skip, abandoned seat), `replay`: move the reusable core into `internal/season`; `sptune` keeps its report.
- `server/internal/season/gorm_repo.go` -- `ApplySeasonPoints` locking/writing pattern (ascending IDs, zero-row insert, `RankForSP`, `divisionOrNil`).
- `server/cmd/api/main.go:159-160,240-248,432` -- season service built, rollover started, reconcile, then `e.Start`: the startup job slots in before reconcile and HTTP.
- `server/migrations/` -- next is 000029.

## Tasks & Acceptance

**Execution:**
- [x] `server/internal/season/replay.go` (+test) -- move `loadWindow`, `outcomeFor` and the replay loop out of `cmd/sptune`: `ReplayWindow(db, from, to, formula, ladder)` returns per-player final SP, tier, division, games played / completed (abandoner = not completed), plus the skip summary.
- [x] `server/cmd/sptune/*.go` -- use the shared core; its report output must stay byte-identical on the Q3 copy.
- [x] `server/migrations/000029_queue_2026_q3_recalculation.{up,down}.sql` -- `season_recalculations (season_started_at TIMESTAMPTZ PRIMARY KEY, requested_at, completed_at NULL)` and the pending row for 2026-07-01T00:00Z; down drops the table.
- [x] `server/internal/season/recalc.go` (+integration test on `beljot_e13`) -- `RunPendingRecalculations(db, now)`: per pending job, in one transaction: lock the job row, find the season by `started_at` (none: mark done), replay its window, and per player in ascending ID insert-zero-row, lock, then `UPDATE sp, rank_tier, rank_division, games_played, games_completed`; mark `completed_at`. Rows of the season the replay does not cover are left as they are and logged.
- [x] `server/cmd/api/main.go` -- run it after the season service is built and before reconcile / `e.Start`; log the outcome (players rewritten, skipped matches).

**Acceptance Criteria:**
- Given the 2026-09-26 production copy with 000029 applied, when the server starts, then every Q3 row matches the `sptune` replay table and the job reads done; a restart changes nothing.
- Given `make lint` and `make test` (integration on `beljot_e13`), when they run, then both pass.

## Implementation Notes

- Verify against a COPY of `beljot_q3` (e.g. `beljot_q3_recalc` created from it), never `beljot_q3` itself (it stays the pristine replay input) and never the shared `beljot` DB. URLs in `/tmp/claude-1000/-home-emilijan-projects-beljot/b97fb269-6cd9-4087-9c90-7b36e2b67ee1/scratchpad/dburls.env`; migrate with `mise which migrate`.
- **Review baseline.** 13.4, 13.5 and the rank-change dialog are uncommitted in the same tree, so `baseline_commit` (HEAD) includes them. Diff this change alone against git tree `260e0d9bbb691c091815801bb104674e2042c6b4` (server/ + client/, untracked included, written via a temporary index; no commit).
- **Rehearsal DB.** `beljot_q3_recalc` is a fresh copy of `beljot_q3` (`BELJOT_Q3_RECALC_DB_URL` in the env file) for applying 000029 and running the startup job; the orchestrating session runs the production-copy rehearsal.
- `games_completed` from stored data can only mark the abandoner absent (the live presence snapshot is not stored); accepted.
- **After review patches (2026-09-27).** Lock/statement timeouts on the job transaction, comment corrections, sptune nil guard, Warn on a missing season. Re-verified: server all packages on `beljot_e13`, client 151 files / 2219 tests, `make lint` 0 issues; the production-copy rehearsal repeated on a FRESH copy: 25 rows equal the approved replay, job done in ~0.1 s before `starting server`, second start skipped with rows unchanged.

## Spec Change Log

## Review Triage Log

Pass 1 (2026-09-26): blind-hunter (B), edge-case-hunter (E), verification-gap (V).

| # | Finding | Verdict | Evidence | Route |
|---|---|---|---|---|
| B6 / E3 | Startup transaction waits on `FOR UPDATE` with no timeout, so a held lock hangs boot before `e.Start` | medium | Confirmed: no `lock_timeout`/`statement_timeout`; the code itself anticipates another holder of the job lock | patch |
| B4 / V-other | Comments still say Q3 is untouched / pre-division / "totals in the thousands" (000027, 000024, service.go, handler.go, seasonTier.ts, apiTypes.ts, SeasonArchiveRow.tsx, LeaderboardRow.tsx) | low | Confirmed | patch |
| B5 | LeaderboardPage test labels Q3 as the pre-division season with 3500 SP | low | Confirmed (`LeaderboardPage.test.tsx` ~505) | patch |
| B8 | `sptune` dereferences a missing `stats` entry if the core ever returns a player the observer skipped | low | Confirmed: duplicated human-seat check | patch |
| B10 | "Season not found" logged at Info though it closes the job for good | low | Confirmed | patch |
| B1 / E1 | A Q3 row the replay does not cover keeps its old total | low | Rehearsal on the 2026-09-26 production copy: 0 uncovered rows (no WARN line); only a live award whose match row failed to save could produce one, and the job logs a WARN with the user ids | reject (release note: check the log for that WARN after deploy) |
| B2 | The rewrite keeps no copy of the old Q3 values | medium | True; `deploy.yml` takes no backup first; the daily VPS backup has them for 7 days | defer (release checklist: run `scripts/backup-db.sh` right before merging) |
| B3 / E6 | 000027 can miss old-formula Q4 awards written between `migrate` and `up -d` if the deploy slips past Oct 1 | low | Owner deploys before 2026-10-01, when 000027 deletes nothing | reject |
| B11 | Players get no explanation for the rank drop | medium | True; product communication, not code | defer (owner announcement) |
| F | Planning docs still describe Q3 as untouched / early deploy as a risk | medium | Confirmed (epic-13-context, epics, sprint-change-proposal, specs 13.4/13.5, deferred-work) | defer (owner's planning update) |
| V1 | No test pins the boot call or its placement before `e.Start` | medium | Pre-verified; one-time job; the rehearsal ran the real binary and logged the job before `starting server` | defer |
| B7 | Two-process "already done" branch untested | low | True; single-instance server; the lock timeout covers a stuck holder | reject |
| B9 | No test that the ended recalculated Q3 renders with divisions | false | `TestService_EndedSeasonsReadTheStoredRank` covers ended seasons reading stored tier and division; the recalculation writes exactly those columns | reject |
| E2 | More than 65535 bind parameters | false | Q3 has ~900 matches and 25 players | reject |
| E4 | `completed_at` vs the finalizer's stamp at a quarter boundary | false | Live SP began 2026-08-27, so no Q2/Q3 boundary match was ever awarded | reject |
| E5 | One user id in two seats | false | Unreachable: a user holds one seat | reject |
- **Production-copy rehearsal (orchestrator, 2026-09-26).** `beljot_q3_recalc` (copy of the 2026-09-26 dump) migrated 26 → 29; the real api binary started against it with the job pending: `season recalculation: done`, 25 players rewritten, 909 loaded / 908 scored / 1 placeholder skipped, logged before `starting server`. All 25 Q3 rows equal the approved replay table on SP, tier and division (e.g. kiro 30841 grandmaster → 534 silver 3; emilijan 13433 master → 407 silver 2; Cece 8634 diamond → 686 gold 2); the 9 pre-tracking players got rows; the job is stamped done. A second start skipped the job and left every row unchanged.
