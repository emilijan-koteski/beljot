-- Reverse 000030 by dropping the avatar prefix column.
--
-- The objects themselves stay in the bucket; only the pointer to them is lost,
-- and every player falls back to the initial disc.
ALTER TABLE users DROP COLUMN avatar_key;
