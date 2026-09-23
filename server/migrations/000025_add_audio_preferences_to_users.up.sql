-- In-match audio preferences. Two independent per-user switches: sound_enabled
-- gates the card-play and trick-collect sound effects, music_enabled gates the
-- background playlist. They are separate on purpose — a player who wants the
-- card feedback without the music (or the reverse) keeps one without the
-- other — and the in-match HUD mute button writes both in one request.
--
-- CLIENT-ONLY BEHAVIOUR. Nothing on the server reads either column: no
-- gameplay, engine, WS-payload or bot behaviour depends on them. They live on
-- the account rather than in browser storage so the choice follows the player
-- across devices. Additive ALTER style, mirroring 000020.
--
-- BOOLEAN, NOT a string enum: each switch is strictly on/off, so there is no
-- allowlist to enforce in Go and no CHECK constraint to widen later. The
-- handler rejects a non-boolean body value with 400 before anything is written.
--
-- DEFAULT TRUE IS THE BACKFILL. Audio is on by default for everyone, so every
-- existing row takes TRUE and no companion UPDATE is needed. Registration and
-- SSO never set either field, so new accounts take TRUE too — the Go model
-- carries the same default in its gorm tag.
ALTER TABLE users
  ADD COLUMN sound_enabled BOOLEAN NOT NULL DEFAULT TRUE,
  ADD COLUMN music_enabled BOOLEAN NOT NULL DEFAULT TRUE;
