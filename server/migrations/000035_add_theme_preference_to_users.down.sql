-- Reverse 000035 by dropping the theme preference. Every player returns to
-- parchment, which is what the up migration's default describes.
ALTER TABLE users DROP COLUMN IF EXISTS theme_preference;
