-- In-match audio volumes. One level per audio channel, 0-100: sound_volume
-- scales the card-play and trick-collect sound effects, music_volume scales the
-- background playlist. They sit beside the two on/off switches from 000025 and
-- do not replace them: a switch still decides whether a channel plays at all,
-- the volume only how loud it is when it does. Muting and unmuting therefore
-- never touch these columns, so a player's chosen levels come back unchanged.
--
-- CLIENT-ONLY BEHAVIOUR. Nothing on the server reads either column: no
-- gameplay, engine, WS-payload or bot behaviour depends on them. They live on
-- the account rather than in browser storage so the levels follow the player
-- across devices, exactly like the switches. Additive ALTER style, mirroring
-- 000025.
--
-- SMALLINT WITH A CHECK, NOT a free integer: a volume is a whole percentage, so
-- the range is enforced in the database as well as in Go. The handler rejects
-- anything outside 0-100 (and any non-integer body value) with 400 before
-- anything is written; the CHECK is the backstop for every other writer.
--
-- DEFAULT 70 IS THE BACKFILL. 70 is the level the client already played at
-- before volumes existed, so every existing row takes 70 and nobody hears a
-- change. Registration and SSO never set either field, so new accounts take 70
-- too — the Go model carries the same default in its gorm tag.
ALTER TABLE users
  ADD COLUMN sound_volume SMALLINT NOT NULL DEFAULT 70 CHECK (sound_volume BETWEEN 0 AND 100),
  ADD COLUMN music_volume SMALLINT NOT NULL DEFAULT 70 CHECK (music_volume BETWEEN 0 AND 100);
