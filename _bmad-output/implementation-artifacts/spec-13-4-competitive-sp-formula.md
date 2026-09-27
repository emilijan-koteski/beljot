---
title: 'Story 13.4: Competitive SP Formula (Win/Loss Ladder)'
type: 'feature'
created: '2026-09-26'
status: 'done'
baseline_commit: 'a290a7c8294ce081443d77a490c4bde50902e766'
route: 'dispatch'
review_loop_iteration: 0
context: ['{project-root}/_bmad-output/project-context.md', '{project-root}/_bmad-output/implementation-artifacts/epic-13-context.md', '{project-root}/_bmad-output/planning-artifacts/sprint-change-proposal-2026-09-26.md']
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Every finished match adds SP (a loss is worth about +80), nobody can drop, and the SP term duplicates XP, so the seasonal ladder measures volume, not skill.

**Approach:** Replace 13.1's formula with the canonical win/loss formula (sprint-change-proposal-2026-09-26 §4) computed inside the season service's award transaction from every seated human's current SP. Tune its constants first with an offline replay of Q3 2026, make leaderboard membership "played ≥ 1 match", and clear Q4 rows at release.

## Boundaries & Constraints

**Always:**
- Formula exactly as §4: winners `+W·m·2(1−E_w)`, losers `−L·m·2·E_l`, rounded half away from zero; teammates get the same change; `m = 0.5 + (winPts − losePts)/target` clamped [0.5, 1.5], surrender → winPts := target, instant win → 1.5; `E = 1/(1+10^((oppAvg−ownAvg)/S))`, team avg = mean of both seats, bot seat = Gold 1 floor; +5 once per match to each human of a team that made ≥ 1 Capot, after scaling.
- Abandonment: abandoner a fixed `−2 × (L × 1.5 × 2)`, no Capot bonus; teammate `½ ×` the loss a surrender at that moment would cost (surrender margin, current E), plus their team's Capot bonus; opponents a surrender-scored win. Every human seat is scored by team result whatever its presence; `games_completed` keeps its presence meaning.
- Totals clamp at 0 before writing; `CHECK (sp >= 0)` stays. The snapshot's change is the APPLIED change (new − previous), so a 10-SP player losing 18 reports −10.
- Rows are read and written under the existing ascending-user-ID order inside one transaction; missing row = 0 SP. `match` never imports `season`.
- Constants are named consts in `season`; `tier.go` stays the single tier source, and `seasonTier.ts` mirrors the new floors in the same change.
- **Checkpoint:** after the replay (Tasks 1-4), HALT and show Emilijan the constants and per-player results. Nothing after Task 4 starts until they are approved and written into this block.
- Integration tests run against a dedicated local DB (`beljot_e13`), never by migrating the shared `beljot` dev DB from this branch.
- **Decision (2026-09-26): replay data = a production `pg_dump`** (the VPS's daily `/var/backups/beljot/beljot-<stamp>.sql.gz` from `scripts/backup-db.sh`), supplied by Emilijan and restored into a scratch local DB `beljot_q3`, which the command reads only.
- **Decision (2026-09-26): one spec for all of 13.4**, kept above the token target; the checkpoint separates tuning from the build.
- **Decision (2026-09-26, checkpoint approved by Emilijan): the constants.** W = 30, L = 20, S = 385, Capot bonus +5, bot seat = Gold 1 floor = 600, abandon penalty −120. Tier floors: Iron 0 · Bronze 150 · Silver 300 · Gold 600 · Platinum 800 · Diamond 1000 · Master 1200 · Grandmaster 1400; divisions split each band into thirds. Evidence: Q3 replay with real margins settles 50/60/70/80/85 % at Gold 3 / Platinum 2 / Diamond 1 / Master / 1409 (Grandmaster floor); a 50 % player reaches Silver at a median of 17 matches (S was lowered from 430 because real win margins exceed loss margins).

**Never:** divisions on the wire, in storage or in the UI; payload shape, demotion notice or SP line (all 13.5). `TieredUp` must still be true only for a climb. No change to coins, XP, honor, boot-reconcile (still awards nothing) or matchmaking. The replay never writes to any DB, and is never built into the image.

## I/O & Edge-Case Matrix

Placeholders W=30, L=20; equal team averages (E=0.5) unless stated.

| Scenario | Input / State | Expected |
|---|---|---|
| Natural win, 1001 | 1100:700 | m=0.90 → winners +27, losers −18 |
| Surrender | team A surrenders at A 300 : B 500 | m=0.5+701/1001=1.20 → B +36, A −24 |
| Instant win | 0:0, no hands | m=1.5 → +45 / −30 |
| Capot by losers | loser team made a Capot | losers −18+5 = −13 |
| Floor | loser at 10 SP, formula −18 | total 0, change −10; first-ever row after a loss writes 0 |
| Bot table | human (x) + bot vs 2 bots | own avg (x+B)/2, opp avg B; bots get no row |
| Abandonment | seat 0 expires, seat 3 disconnected | seat 0 −120; seat 2 ½ surrender-loss; seats 1, 3 surrender-win; seat 3 `Completed=false` |
| Leaderboard | row sp=0, games_played=1 | listed, counted, viewer block present |
| Q4 reset | rows in 2026 Q4 and Q3 | Q4 rows deleted, Q3 untouched |

</frozen-after-approval>

## Code Map

- `server/internal/match/sp_award.go` -- old formula (`computeSPAwards`, consts, `capotOccurred`), `spSeatPresent` (keep: it is the `Completed` gate), `awardSeasonPoints` builds the award map and the ws payload.
- `server/internal/match/live_match.go:142-180` -- `SPAward`, `PlayerSeasonSnapshot`, `SPAwarder`; `handleMatchEnd:1383` (award at :1458; `finalState` has `MatchMode`, `TeamScores`, `WonByInstantWin`; `handsCopy` :1403 has `CapotTeam *int`). Surrender flag is `matchEndPayload.OutcomeReason == ws.OutcomeReasonSurrender` (a surrender accept can finalize as `target_reached`, `surrenderedBy` still set); surrendering team = `TeamForSeat(*SurrenderedBySeat)`. The comment at :2213 claiming "a surrender never runs the stop" is wrong: fix it.
- `server/internal/match/reconnect.go:573-722` -- abandonment finalizer: scores, `connected`, `botSeats`, `handsCopy`, `MatchMode` captured under lock; `winningTeam = 1 − TeamForSeat(abandonedSeat)`; one abandoned seat per match.
- `server/internal/game/scoring.go:468` -- unexported `matchTarget(mode)`: export `MatchTarget`, keep callers :140, :271.
- `server/internal/season/service.go:72-106` -- `ApplySeasonPoints` adapter; `TieredUp` at :102 is also true on a tier-down.
- `server/internal/season/gorm_repo.go:129-191` -- upsert-increment write (first-row INSERT writes `award.SP` straight in); `leaderboardScope:223-229` has the `sp > 0` membership shared by page, total, `CountAhead`, `FindLeaderboardEntry`.
- `server/internal/season/tier.go` -- floors + the "FLAT LADDER" header, which must go.
- `server/internal/season/model.go:57-74`, `repository.go:20-30,58-80` -- duplicate types and the contract docs to rewrite.
- `server/internal/ws/events.go:147-170` -- payload doc restates the old formula (update the comment only; shape is 13.5).
- `server/cmd/api/main.go:43-58` -- config + `gorm.Open` to copy; `server/Dockerfile:14` builds only `./cmd/api`.
- Stored data: `matches` (`player{1..4}_id` NULL + `player{N}_is_bot` for bots; team = seat % 2; `winner_team`, `team_a/b_score`, `match_mode` '501'/'1001', `status` completed|abandoned, `abandoned_by`, `surrendered_by`, `completed_at`); `hand_results.capot_team`. `abandoned_by IS NULL` rows are reconcile placeholders: skip. Loader pattern: `match/gorm_repo.go:164`.
- Tests pinning old behaviour: `match/sp_award_test.go`, `sp_wiring_test.go` (14), `season/tier_test.go`, `gorm_repo_test.go` (ApplySeasonPoints ×5, zero-SP membership ×4), `handler_test.go` (`mockRepo:24`, zero-SP ladder ×4, `PrecomputesTheSnapshot`), `client/src/shared/lib/seasonTier.test.ts`.

## Tasks & Acceptance

**Execution:**
- [x] `server/internal/match/live_match.go` -- add `MatchOutcome` (per seat: user id, bot, team, completed; winner, scores, target, surrender, instant win, Capot teams, abandoned seat); `SPAwarder.ApplySeasonPoints(outcome, now)`; snapshot gains `SPChange`.
- [x] `server/internal/season/sp_formula.go` (+test) -- named consts and pure `ComputeSPChanges(outcome, currentSP)`; table-test every I/O row.
- [x] `server/internal/season/tier.go` (+test) -- floors as named data; pure `RankForSP(sp) (tier, division)` (thirds for Iron–Diamond, 0 above) for the replay report and for 13.5.
- [x] `server/cmd/sptune/main.go` (+test) -- replay the window in `completed_at, id` order through `ComputeSPChanges`, in memory; flags `--db-url`, `--from`, `--to` and constant overrides; infer surrender (`surrendered_by` set and winner below target) and instant win (completed, no surrender, winner below target); per player: matches, win %, bot-only matches and win %, final SP, tier, division, match index at Silver; plus a bot-only synthetic sweep (p = 0.5…0.9) giving the settle point and SP after 20 matches. **→ CHECKPOINT.**
- [x] `server/internal/game/scoring.go` -- export `MatchTarget`.
- [x] `server/internal/match/sp_award.go`, `live_match.go`, `reconnect.go` -- build the outcome at both finalizers (Capot teams from `handsCopy`), drop the old formula, send `SPEarned = snap.SPChange`.
- [x] `server/internal/season/{model,repository,gorm_repo,service}.go` -- one transaction: per user ascending, `INSERT … ON CONFLICT DO NOTHING` a zero row, `SELECT … FOR UPDATE`, compute via callback, clamp, `UPDATE` sp/rank_tier/counters; `TieredUp` on a climb only; membership `games_played >= 1` in `leaderboardScope`; rewrite the contract comments.
- [x] `server/migrations/000027_reset_2026_q4_player_seasons.{up,down}.sql` -- delete `player_seasons` of the season starting 2026-10-01T00:00Z; the down is a documented no-op.
- [x] `client/src/shared/lib/seasonTier.ts` (+test), `client/src/shared/types/apiTypes.ts:312` -- mirror floors; drop "Monotonic".
- [x] Update every pinned test above; add wiring tests for a surrender that finalizes at target, abandonment presence, and bot averaging.

**Acceptance Criteria:**
- Given a finished match, when SP is applied, then each human's change matches `ComputeSPChanges` over the pre-match rows read in the same transaction, and no total is below 0.
- Given two matches finishing concurrently with a shared player, when both award, then neither change is lost.
- Given `make lint` and `make test`, when they run, then both pass with the season integration tests running against `beljot_e13`.

## Implementation Notes

- **Environment (prepared 2026-09-26, before implementation).** Local Postgres `beljot-postgres-1` on 5433 now also holds two scratch DBs. Their URLs are in `/tmp/claude-1000/-home-emilijan-projects-beljot/b97fb269-6cd9-4087-9c90-7b36e2b67ee1/scratchpad/dburls.env` (`BELJOT_E13_DB_URL`, `BELJOT_Q3_DB_URL`; source the file, never print it).
  - `beljot_q3`: the production dump `beljot-20260926T115731Z.sql` restored as-is (schema 26, 1187 matches, 84 users). Read-only input for `sptune`.
  - `beljot_e13`: migrated to 26 from `server/migrations`; run this branch's integration tests with `BELJOT_DB_URL=$BELJOT_E13_DB_URL`, and apply new migrations here with `mise which migrate`. Never touch the shared `beljot` DB.
  - Baseline on `beljot_e13` is green (season/match/user; season 0 SKIP). Client baseline: 147 files / 2139 tests pass; `client/node_modules` installed.
- **Checkpoint (2026-09-26).** Tasks 1–4 done. The replay against `beljot_q3` ran in the orchestrating session after Emilijan allowed the read; outputs are in the scratchpad (`replay-final.txt`, `trace-4.txt`). The orchestrator added `--trace-user <id>` to `sptune` (`replay()` gained a `traceUser` parameter, `replaySummary.Trace`, `printTrace` in `main.go`); keep it. Constants approved and recorded in the frozen block.

## Spec Change Log

## Review Triage Log

Pass 1 (2026-09-26): blind-hunter (B), edge-case-hunter (E), verification-gap (V).

| # | Finding | Verdict | Evidence | Route |
|---|---|---|---|---|
| E1 / V-other / B3a | Ended seasons (Q3) re-derive tier via `TierForSP` on the new floors in leaderboard rows, viewer row and archive (`service.go` 233/297/358): old-formula totals read as Grandmaster | high | Confirmed: floors moved to 0–1400 while Q3 totals are in the thousands; archive and `?season=<id>` both call `TierForSP` | defer → 13.5 (intent assigns stored tier/snapshot reads to 13.5; 13.5 must cover prior-season LEADERBOARD rows too, not only the archive; 13.4 must not ship without 13.5) |
| E2 / B2 | "Deploy on or after 2026-10-01" is only a comment; `deploy.yml` auto-deploys on push to master | medium | Confirmed trigger (`on: push: branches: [master]`). An early merge scores Q3's last days with the new formula over old totals. A migration guard would break CI (`ci.yml:61` migrates a fresh DB) and `reset_migration_test` until Oct 1 | defer (owner-held release precondition in the AC's Given; surfaced to owner as a release-checklist decision; no code revert, the root cause is deploy timing, not code) |
| E3 / B1 | A match finishing between `run --rm migrate` and `up -d` is scored by the old binary into Q4 after the reset | low | Confirmed ordering (`deploy.yml:124-127`); window is seconds, affects at most one table's four players once | reject (low, fix is a startup reset or workflow change); surfaced in release notes: deploy with no live matches |
| V1 | 501 target never verified through either finalizer; dropping `matchMode:` from `spMatchFacts` still passes | medium | Pre-verified gap; all SP wiring tests start "1001" | patch |
| V2 | Concurrency test passes without `FOR UPDATE` (shared player has no prior row, INSERT wait serialises) | medium | Pre-verified gap | patch |
| V3 / B6 | No test sends a negative `spEarned` (client dispatch or golden) | medium | Pre-verified gap; golden 200, client 251/0 | patch |
| B4 | `apiTypes.ts` docs still say viewer null at 0 SP (:392), gamesCompleted = SP earners (:335), spIntoTier "earned" (:325) | low | Confirmed stale against new server contract | patch |
| B5 | Old-ladder fixture values remain (4000 SP gold, 20000 GM, iron band 500, golden 1700 "silver") across client/server tests | low | Confirmed values impossible on the new ladder; misleads 13.5 work in the same files | patch |
| E6 / B8a | `sptune` header: best win omits Capot bonus, worst loss is `AbandonPenalty()/2` | low | Confirmed `main.go:234-235` | patch |
| B8b | Abandon multiple not overridable in `SPFormula` | low | True; not needed for tuning | reject |
| E7 / B9a | `sptune` trace row stores raw change, not applied | low | Confirmed `replay.go` trace `Change: changes[...]` | patch |
| B9b | Skip lines print in map order | low | Confirmed range over map | patch (sort keys) |
| B9c | Partner-abandoned trace row labelled "loss" | low | True; adds a branch/field | reject |
| B10 | Stale "spectacular bonus" comment `instant_win_flag_test.go:67-69` | low | Confirmed | patch |
| B11 | Misleading message at `sp_formula_test.go:302` (Capot "still counts" on a floored loss) | low | Confirmed wording; formula adds the bonus before the floor, so a 0-SP loser stays 0 | patch (wording) |
| B7a | Lock order untested (one shared player cannot deadlock) | low | True; removing `slices.Sort` is unlikely and the fix is a new deadlock test | reject |
| B7b | Concurrency test's own `gorm.Open` pool never closed | low | Confirmed | patch |
| B3b | `games_played >= 1` now lists Q3's 0-SP rows on the frozen Q3 ladder | false | Intended: the AC's membership rule applies to every season view, and the viewer rule is shared | reject |
| B12 | Leaderboard empty copy/subtitle predates the membership change | false | "Nobody has earned Season Points yet" is still true when nobody has played; subtitle is tagline copy | reject |
| E4 | `sptune` accepts NaN/Inf floats | low | Dev-only flags; fix adds guards | reject |
| E5 | `--sweep-margin` out of [0.5, 1.5] | low | Dev-only flag; fix adds a guard | reject |
| E8 | Nil `Ladder` literal panics in `Progress` | false | Only built via `DefaultLadder`/`NewLadder`; no caller builds a literal | reject |
| E9 | Duplicate user ID across two seats not rejected | false | Unreachable: a user holds one seat (room/session invariants) | reject |
| E10 | `mockRepo` writes rows before a missing-change error | low | Test-only divergence, no test exercises it | reject |
| E11 | `WinnerTeam` nil defaults to team A | maybe-false | Pre-existing default shared with coins/XP; engine sets the winner before `PhaseMatchEnd`; no reachable nil shown | reject (would need a reachable nil-winner match end) |

## Design Notes

Reading "current SP" and writing the new total must be one locked step, so the old in-SQL increment becomes insert-zero-then-lock: a missing row cannot be locked, but a just-inserted zero row can, and this also fixes the negative-first-row INSERT for free. The formula stays in `season` and runs as a callback inside the repository transaction, so the repository stays persistence-only.

Why the replay reports the synthetic sweep too: with `W/L = 1.5`, the bot-only settle point is `B + 2S·log10(1.5p/(1−p))`. So equal band widths above Gold put 50/60/70/80 % in Gold/Platinum/Diamond/Master, and 85 % lands on the Grandmaster floor exactly. The "50 % reaches Silver in ~20" target depends on real margins and table mix, which only the data shows.

## Verification

**Commands:**
- `cd server && BELJOT_DB_URL=<beljot_e13 url> go test -count=1 ./...` -- expected: pass, season integration tests RUN, not SKIP.
- `cd client && npx vitest run` -- expected: pass.
- `make lint` -- expected: exit 0.
- `cd server && go run ./cmd/sptune --db-url <q3 source>` -- expected: per-player table and synthetic sweep printed; no writes.
