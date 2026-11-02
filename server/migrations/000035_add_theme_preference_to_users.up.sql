-- Per-user look for the pages outside a match: 'parchment' (the light theme the
-- app has always shipped) or 'brass' (the dark felt-and-brass theme the match
-- table uses). Purely visual; nothing on the server reads it.
--
-- No CHECK constraint, like language_preference and card_deck_preference: the
-- allowlist lives in Go (user.IsSupportedTheme, answering apperr.ErrInvalidTheme).
-- The default is the backfill, since every existing player has only seen parchment.
ALTER TABLE users ADD COLUMN theme_preference VARCHAR(20) NOT NULL DEFAULT 'parchment';
