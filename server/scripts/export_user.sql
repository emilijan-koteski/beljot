-- Export one user's personal data as a single JSON document (GDPR Arts. 15 and 20). Read-only.
-- Usage: psql "$BELJOT_DB_URL" -X -q -v uid=<user id> -f server/scripts/export_user.sql > user-<id>.json
-- Other players appear by username only; erased players appear as null.

\set ON_ERROR_STOP on

\if :{?uid}
\else
  \echo 'usage: psql -X -q -v uid=<user id> -f export_user.sql'
  \quit
\endif

\pset tuples_only on
\pset format unaligned

BEGIN TRANSACTION READ ONLY;

WITH me AS (
  SELECT * FROM users WHERE id = :uid
),
player_names AS (
  SELECT id, CASE WHEN deleted_at IS NULL THEN username END AS username FROM users
)
SELECT jsonb_pretty(jsonb_build_object(
  'exportedAt', now(),
  'profile', (
    SELECT to_jsonb(me) - 'password_hash' || jsonb_build_object('has_password', me.password_hash <> '')
    FROM me
  ),
  'identities', coalesce((
    SELECT jsonb_agg(jsonb_build_object(
      'provider', i.provider, 'providerUserId', i.provider_user_id, 'email', i.email, 'linkedAt', i.created_at
    ) ORDER BY i.created_at)
    FROM user_identities i WHERE i.user_id = :uid
  ), '[]'::jsonb),
  'friends', coalesce((
    SELECT jsonb_agg(jsonb_build_object(
      'username', p.username,
      'status', f.status,
      'requestedByMe', f.user_id = :uid,
      'since', f.created_at
    ) ORDER BY f.created_at)
    FROM friendships f
    JOIN player_names p ON p.id = CASE WHEN f.user_id = :uid THEN f.friend_id ELSE f.user_id END
    WHERE f.user_id = :uid OR f.friend_id = :uid
  ), '[]'::jsonb),
  'seasons', coalesce((
    SELECT jsonb_agg(jsonb_build_object(
      'season', s.name, 'startedAt', s.started_at, 'endsAt', s.ends_at,
      'seasonPoints', ps.sp, 'rankTier', ps.rank_tier, 'rankDivision', ps.rank_division,
      'gamesPlayed', ps.games_played, 'gamesCompleted', ps.games_completed, 'gamesWon', ps.games_won
    ) ORDER BY s.started_at)
    FROM player_seasons ps JOIN seasons s ON s.id = ps.season_id
    WHERE ps.user_id = :uid
  ), '[]'::jsonb),
  'matches', coalesce((
    SELECT jsonb_agg(jsonb_build_object(
      'matchId', m.id,
      'startedAt', m.started_at,
      'completedAt', m.completed_at,
      'variant', m.variant,
      'matchMode', m.match_mode,
      'status', m.status,
      'myTeam', CASE WHEN :uid IN (m.player1_id, m.player3_id) THEN 'A' ELSE 'B' END,
      'teamAScore', m.team_a_score,
      'teamBScore', m.team_b_score,
      'won', m.winner_team = CASE WHEN :uid IN (m.player1_id, m.player3_id) THEN 0 ELSE 1 END,
      'abandonedByMe', m.abandoned_by IS NOT DISTINCT FROM :uid,
      'surrenderedByMe', m.surrendered_by IS NOT DISTINCT FROM :uid,
      'myCoinDelta', CASE :uid
        WHEN m.player1_id THEN m.player1_coin_delta WHEN m.player2_id THEN m.player2_coin_delta
        WHEN m.player3_id THEN m.player3_coin_delta ELSE m.player4_coin_delta END,
      'seats', jsonb_build_array(
        jsonb_build_object('seat', 1, 'team', 'A', 'bot', m.player1_is_bot, 'username', p1.username),
        jsonb_build_object('seat', 2, 'team', 'B', 'bot', m.player2_is_bot, 'username', p2.username),
        jsonb_build_object('seat', 3, 'team', 'A', 'bot', m.player3_is_bot, 'username', p3.username),
        jsonb_build_object('seat', 4, 'team', 'B', 'bot', m.player4_is_bot, 'username', p4.username)
      )
    ) ORDER BY m.completed_at)
    FROM matches m
    LEFT JOIN player_names p1 ON p1.id = m.player1_id
    LEFT JOIN player_names p2 ON p2.id = m.player2_id
    LEFT JOIN player_names p3 ON p3.id = m.player3_id
    LEFT JOIN player_names p4 ON p4.id = m.player4_id
    WHERE :uid IN (m.player1_id, m.player2_id, m.player3_id, m.player4_id)
  ), '[]'::jsonb),
  'roomsOwned', coalesce((
    SELECT jsonb_agg(jsonb_build_object(
      'roomId', r.id, 'name', r.name, 'code', r.code, 'status', r.status, 'variant', r.variant,
      'matchMode', r.match_mode, 'private', r.password_hash IS NOT NULL, 'createdAt', r.created_at
    ) ORDER BY r.created_at)
    FROM rooms r WHERE r.owner_id = :uid
  ), '[]'::jsonb),
  'sessions', coalesce((
    SELECT jsonb_agg(jsonb_build_object(
      'startedAt', t.created_at, 'expiresAt', t.expires_at, 'rotatedAt', t.rotated_at, 'revokedAt', t.revoked_at
    ) ORDER BY t.created_at)
    FROM refresh_tokens t WHERE t.user_id = :uid
  ), '[]'::jsonb)
))
FROM me;

COMMIT;
