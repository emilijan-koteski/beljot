-- Reverse 000025 by dropping the two audio preference columns.
--
-- Nothing is unrecoverable: the columns hold player settings, not accumulated
-- history, and dropping them returns every player to sound and music on —
-- exactly the state the up migration's default describes.
ALTER TABLE users
  DROP COLUMN music_enabled,
  DROP COLUMN sound_enabled;
