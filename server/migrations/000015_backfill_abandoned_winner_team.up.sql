-- Per-player results for abandoned matches: abandoned rows historically
-- persisted winner_team = 0 as a filler. The real winner is the team opposite
-- the abandoner — the same value coin settlement already computed live.
-- Backfill it from the abandoner's seat (player1/player3 = team 0,
-- player2/player4 = team 1) so stats/history can read win/loss off abandoned
-- rows for the three non-abandoners.
--
-- Rows with abandoned_by IS NULL (boot-reconcile, no attributable abandoner)
-- keep the filler 0 and stay "abandoned" for everyone — every reader gates
-- winner_team on abandoned_by IS NOT NULL. The ELSE arm covers seats 2/4 and
-- also keeps winner_team NOT NULL-safe should abandoned_by ever fail to match
-- a seat (data anomaly — falls back to the historical filler 0).
--
-- DEPLOY ORDER (load-bearing): apply this migration BEFORE deploying the code
-- that reads winner_team off abandoned rows — the new stats/history queries
-- against un-backfilled filler rows would misclassify team-0 partners as
-- winners.
UPDATE matches
SET winner_team = CASE
    WHEN abandoned_by = player1_id OR abandoned_by = player3_id THEN 1
    ELSE 0
END
WHERE status = 'abandoned'
  AND abandoned_by IS NOT NULL;
