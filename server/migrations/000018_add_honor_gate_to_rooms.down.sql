-- Reverse 000018 by dropping the two honor-gate columns in the reverse order
-- they were added. Nothing is unrecoverable in the sense 000017's down file
-- warns about: these columns hold owner configuration, not accumulated player
-- history, and there is no backfill to recompute. Dropping them returns every
-- room to ungated, which is exactly the state the up migration's defaults
-- describe.
ALTER TABLE rooms DROP COLUMN allow_new_players;
ALTER TABLE rooms DROP COLUMN min_honor;
