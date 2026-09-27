-- Reverse 000029 by dropping the job queue.
--
-- The rows a completed recalculation wrote stay as they are: they are ordinary
-- player_seasons totals, and there is nothing to restore the old-formula
-- totals from.
DROP TABLE IF EXISTS season_recalculations;
