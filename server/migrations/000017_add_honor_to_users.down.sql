-- Reverse 000017 by dropping the six honor columns in the reverse order they
-- were added.
--
-- THIS IS A LOSSY REVERSAL. Re-applying the up migration afterwards does NOT
-- restore the prior state and cannot: that migration's backfill is DEPLOY-ONLY
-- (its header says so in capitals), and a down-then-up cycle re-runs it by
-- construction. Two things are destroyed:
--
--   1. Operator forgiveness. Every pardon applied via ResetHonor or the SQL
--      recipe in the up header lives ONLY in these columns. The backfill writes
--      absolute values recomputed from `matches`, so it silently reinstates the
--      penalties the pardon cleared.
--   2. The concurrent-double-disconnect rule. `matches` stores a single
--      abandoned_by and no per-seat presence, so the backfill cannot tell that a
--      second absent seat was ALSO charged an abandonment at write time, and
--      re-credits it a completion — raising a quitter's honor.
--
-- An earlier version of this header claimed the backfill "is written to be
-- re-runnable as a reconciliation recipe". It is not. That sentence was left
-- behind when the up migration's header was corrected during Story 9.7's second
-- review pass, and the two files then contradicted each other about the same
-- statement. If the denormalized counters are ever suspected of drifting,
-- reconcile with a query that EXCLUDES pardoned rows and accepts (2) as a known
-- floor — do not reach for this pair of migrations.
ALTER TABLE users DROP COLUMN honor_score;
ALTER TABLE users DROP COLUMN honor_abandoned_total;
ALTER TABLE users DROP COLUMN honor_completed_total;
ALTER TABLE users DROP COLUMN honor_decayed_at;
ALTER TABLE users DROP COLUMN honor_abandoned_weight;
ALTER TABLE users DROP COLUMN honor_completed_weight;
