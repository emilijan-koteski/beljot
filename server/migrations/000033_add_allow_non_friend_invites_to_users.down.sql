-- Reverse 000033 by dropping the invite privacy switch. The column holds a player
-- setting, not history, but every opt-out is lost. Roll the code back first: with
-- the new code still running, every player would read as opted out (false) and
-- preference writes would fail. Re-running the up migration restores everyone to TRUE.
ALTER TABLE users
  DROP COLUMN allow_non_friend_invites;
