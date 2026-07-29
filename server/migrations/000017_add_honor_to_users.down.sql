-- Reverse 000017 by dropping the six honor columns in the reverse order they
-- were added. The backfilled values are recoverable: re-running the up
-- migration recomputes every column from `matches` (that backfill is written to
-- be re-runnable as a reconciliation recipe). Any operator forgiveness applied
-- via ResetHonor is NOT recoverable — it exists only in these columns.
ALTER TABLE users DROP COLUMN honor_score;
ALTER TABLE users DROP COLUMN honor_abandoned_total;
ALTER TABLE users DROP COLUMN honor_completed_total;
ALTER TABLE users DROP COLUMN honor_decayed_at;
ALTER TABLE users DROP COLUMN honor_abandoned_weight;
ALTER TABLE users DROP COLUMN honor_completed_weight;
