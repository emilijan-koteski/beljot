-- Reverse 000021 by dropping the column. Nothing is unrecoverable: it holds
-- owner configuration, not accumulated player history, and there is no backfill
-- to recompute. Dropping it returns every room to declarations-on, which is
-- exactly the state the up migration's default describes.
ALTER TABLE rooms DROP COLUMN declarations_enabled;
