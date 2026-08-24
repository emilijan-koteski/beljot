-- Reverse 000023 by dropping both columns. Nothing is unrecoverable: they hold
-- the rule configuration a match was played under, not accumulated player
-- history, and dropping them returns every match to being described only by its
-- variant and point target — exactly the state the up migration's defaults
-- describe.
ALTER TABLE matches DROP COLUMN stop_at_target;
ALTER TABLE matches DROP COLUMN declarations_enabled;
