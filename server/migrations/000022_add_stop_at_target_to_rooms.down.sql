-- Reverse 000022 by dropping the column. Nothing is unrecoverable: it holds
-- owner configuration, not accumulated player history, and there is no backfill
-- to recompute. Dropping it returns every room to finishing its hand before the
-- target is checked, which is exactly the state the up migration's default
-- describes.
ALTER TABLE rooms DROP COLUMN stop_at_target;
