-- Reverse 000032 by dropping both columns. They hold rule configuration, not
-- player history, and dropping them returns every room and match to the
-- counter-clockwise play the up migration's default describes.
ALTER TABLE matches DROP COLUMN clockwise;
ALTER TABLE rooms DROP COLUMN clockwise;
