-- Reverse 000026 by dropping the two audio volume columns (their CHECK
-- constraints go with them).
--
-- Nothing is unrecoverable: the columns hold player settings, not accumulated
-- history, and dropping them returns every player to the default level of 70 —
-- exactly the state the up migration's default describes, and the level the
-- client plays at when the fields are missing.
ALTER TABLE users
  DROP COLUMN music_volume,
  DROP COLUMN sound_volume;
