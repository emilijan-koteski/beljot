-- Reverse 000034 by dropping the per-season win count.
ALTER TABLE player_seasons DROP COLUMN IF EXISTS games_won;
