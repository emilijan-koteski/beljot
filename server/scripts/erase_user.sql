-- Erase one user by anonymising the users row in place (GDPR Art. 17).
-- Usage: psql "$BELJOT_DB_URL" -X -v uid=<user id> -f server/scripts/erase_user.sql
-- Mirrors internal/account/gorm_repo.go. Matches are never touched: abandoned_by and
-- surrendered_by must keep the tombstone id, because NULL there counts against every seat.
-- Afterwards delete the two avatar objects it prints from S3 and purge them from Cloudflare.

\set ON_ERROR_STOP on

\if :{?uid}
\else
  \echo 'usage: psql -X -v uid=<user id> -f erase_user.sql'
  \quit
\endif

BEGIN;

SELECT id AS erase_id, coalesce(avatar_key, '') AS erase_avatar_key
  FROM users WHERE id = :uid AND deleted_at IS NULL FOR UPDATE \gset

SELECT EXISTS (
  SELECT 1 FROM room_players rp
  JOIN rooms r ON r.id = rp.room_id
  WHERE rp.user_id = :uid AND r.status IN ('waiting', 'playing') AND r.deleted_at IS NULL
) AS erase_seated \gset

\if :erase_seated
  \echo 'ABORTED: the user is seated in a waiting or playing room. Retry once the match is over.'
  ROLLBACK;
  \quit
\endif

DELETE FROM refresh_tokens        WHERE user_id = :uid;
DELETE FROM password_reset_tokens WHERE user_id = :uid;
DELETE FROM user_identities       WHERE user_id = :uid;
DELETE FROM friendships           WHERE user_id = :uid OR friend_id = :uid;
DELETE FROM player_seasons        WHERE user_id = :uid;
DELETE FROM room_players          WHERE user_id = :uid;

UPDATE rooms SET name = 'room-' || id, password_hash = NULL WHERE owner_id = :uid;

UPDATE users SET
  email = 'erased+' || id || '@invalid', username = 'erased_' || id, password_hash = '',
  avatar_key = NULL, language_preference = 'en', card_deck_preference = 'french',
  theme_preference = 'brass', sound_enabled = true, music_enabled = true,
  sound_volume = 70, music_volume = 70, allow_non_friend_invites = true,
  wallet_balance = 0, last_login_at = NULL, login_streak_days = 0, total_xp = 0,
  username_changed_at = NULL, honor_completed_weight = 0, honor_abandoned_weight = 0,
  honor_decayed_at = NULL, honor_completed_total = 0, honor_abandoned_total = 0,
  honor_score = 80, created_at = now(), updated_at = now(), deleted_at = now()
WHERE id = :uid;

SELECT
  (SELECT count(*) FROM refresh_tokens        WHERE user_id = :uid)
+ (SELECT count(*) FROM password_reset_tokens WHERE user_id = :uid)
+ (SELECT count(*) FROM user_identities       WHERE user_id = :uid)
+ (SELECT count(*) FROM friendships           WHERE user_id = :uid OR friend_id = :uid)
+ (SELECT count(*) FROM player_seasons        WHERE user_id = :uid)
+ (SELECT count(*) FROM room_players          WHERE user_id = :uid) = 0 AS erase_clean \gset

\if :erase_clean
  COMMIT;
  \echo 'Erased user' :erase_id
  SELECT CASE WHEN :'erase_avatar_key' = '' THEN 'no avatar to delete'
              ELSE :'erase_avatar_key' || '/256.webp and ' || :'erase_avatar_key' || '/128.webp' END AS avatar_objects;
\else
  \echo 'ABORTED: satellite rows remain, nothing was changed.'
  ROLLBACK;
\endif
