-- Room-invite privacy switch. When TRUE, any room member may invite this player
-- while they are idle in the lobby; when FALSE, only their friends may. Friend
-- invites are never affected.
--
-- DEFAULT TRUE IS THE BACKFILL. Every existing row takes TRUE, so no companion
-- UPDATE is needed. The Go field deliberately has no GORM default tag, per the
-- server/CLAUDE.md rule for bool columns (GORM would drop an explicit false from
-- INSERTs), so Registration and SSO set it to true explicitly.
ALTER TABLE users
  ADD COLUMN allow_non_friend_invites BOOLEAN NOT NULL DEFAULT TRUE;
