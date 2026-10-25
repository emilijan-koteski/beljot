-- Reverse 000031 by dropping both columns. They hold rule configuration, not
-- player history, and dropping them returns every room and match to the
-- transferring rule the up migration's default describes.
ALTER TABLE matches DROP COLUMN declarations_transfer;
ALTER TABLE rooms DROP COLUMN declarations_transfer;
