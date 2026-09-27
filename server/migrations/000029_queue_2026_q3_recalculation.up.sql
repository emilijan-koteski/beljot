-- Re-score 2026 Q3 with the win/loss formula (spec-q3-season-recalculation).
--
-- 2026 Q3's standings were scored by the retired climb-only formula, so its
-- totals run into the thousands and would read as Grandmaster on the Story 13.4
-- floors. The release ships before 2026-10-01, so rather than let the live
-- formula overwrite Q3's ranks one match at a time over old totals, the whole
-- quarter is replayed once from its stored matches through the live formula and
-- ladder, and each player's row is rewritten with the result: SP, tier,
-- division, games played and games completed. The rest of Q3 then scores live
-- from those totals.
--
-- THIS MIGRATION ONLY QUEUES THE JOB. The replay is Go arithmetic
-- (season/replay.go, the same core cmd/sptune tuned the constants with), which
-- SQL cannot restate without a second copy of the formula. The server runs
-- every pending job once at startup, after the migrations and before it
-- accepts connections (season.RunPendingRecalculations), and stamps
-- completed_at in the same transaction that writes the rows. A failed run
-- rolls back and leaves the job pending for the next start; a done job never
-- runs again.
--
-- KEYED BY THE SEASON'S START, not its id, and with no foreign key: a database
-- that never had a 2026 Q3 season row (a fresh one migrated later) still gets
-- the job, and the server marks it done without writing anything.
--
-- 2026 Q3 is the only season queued. Every other season, and 000027's Q4
-- reset, are untouched.
CREATE TABLE season_recalculations (
    season_started_at TIMESTAMPTZ PRIMARY KEY,
    requested_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at      TIMESTAMPTZ NULL
);

INSERT INTO season_recalculations (season_started_at)
VALUES (TIMESTAMPTZ '2026-07-01 00:00:00+00');
