-- Count each player's wins per season, so the leaderboard can show a season
-- record (wins and losses) and a win rate next to the Season Points.
--
-- From here on the match-end award (season.ApplySeasonPoints) and a queued
-- recalculation (season/recalc.go) keep games_won current. This migration
-- backfills every existing season once from its stored matches, with the same
-- rules the replay uses (season/replay.go, OutcomeFor):
--   * matches completed or abandoned with completed_at inside the season;
--   * a known match mode, and no boot-reconcile placeholder (an abandoned
--     match with no abandoner);
--   * a human seat wins when its team (seat % 2) is the winner team, and an
--     abandoned match is won by the team that did not abandon.
-- Season Points, ranks and games played are not touched. The count is capped
-- at games_played, so a match the live award never scored cannot push a
-- record past the games it shows.
ALTER TABLE player_seasons ADD COLUMN games_won INTEGER NOT NULL DEFAULT 0;

WITH seats AS (
    SELECT s.id AS season_id,
           seat.user_id,
           seat.team,
           CASE
               WHEN m.status = 'abandoned' THEN 1 - (
                   CASE m.abandoned_by
                       WHEN m.player1_id THEN 0
                       WHEN m.player2_id THEN 1
                       WHEN m.player3_id THEN 0
                       WHEN m.player4_id THEN 1
                   END)
               ELSE m.winner_team
           END AS winner_team
    FROM seasons s
    JOIN matches m
      ON m.completed_at >= s.started_at
     AND m.completed_at < s.ends_at
     AND m.status IN ('completed', 'abandoned')
     AND m.match_mode IN ('501', '701', '1001')
     AND NOT (m.status = 'abandoned' AND m.abandoned_by IS NULL)
    CROSS JOIN LATERAL (VALUES
        (m.player1_id, m.player1_is_bot, 0),
        (m.player2_id, m.player2_is_bot, 1),
        (m.player3_id, m.player3_is_bot, 0),
        (m.player4_id, m.player4_is_bot, 1)
    ) AS seat (user_id, is_bot, team)
    WHERE seat.user_id IS NOT NULL
      AND NOT seat.is_bot
),
wins AS (
    SELECT season_id, user_id, COUNT(*) AS won
    FROM seats
    WHERE team = winner_team
    GROUP BY season_id, user_id
)
UPDATE player_seasons ps
SET games_won = LEAST(wins.won, ps.games_played)
FROM wins
WHERE ps.season_id = wins.season_id
  AND ps.user_id = wins.user_id;
