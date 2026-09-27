-- Reverse 000028 by dropping the division snapshot (its CHECK goes with it).
--
-- Nothing authoritative is lost: a running season's division is derived from
-- sp, and an ended season falls back to its stored rank_tier alone, which is
-- how rows written before this migration render.
ALTER TABLE player_seasons
  DROP COLUMN rank_division;
