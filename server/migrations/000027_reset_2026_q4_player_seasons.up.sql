-- Clear 2026 Q4's standings at the release of the win/loss ladder (Story 13.4,
-- sprint-change-proposal-2026-09-26).
--
-- The release that ships the competitive SP formula deploys ON OR AFTER
-- 2026-10-01 00:00 UTC, the start of Q4. Any Q4 player_seasons rows already
-- present when it lands were scored by the OLD, climb-only formula (+50 for
-- finishing, +100 for a win, game points / 10) during the hours or days between
-- the quarter boundary and the deploy. Deleting them means Q4 standings come
-- only from the new formula: everyone starts Q4 at Iron with 0 SP, which is the
-- soft reset every season gets anyway.
--
-- SCOPED TO ONE WINDOW, by its start instant, not by "the current season":
-- 2026 Q3 and every earlier season are archive history and are never touched,
-- and a later run (a fresh database, a re-applied migration in another quarter)
-- can only ever clear this one quarter. The seasons row itself is kept; the lazy
-- resolver and the rollover job would recreate it anyway, and the leaderboard's
-- season picker should still list Q4.
--
-- If no 2026 Q4 season row exists yet (a deploy that runs before any match or
-- job has created it), this deletes nothing.
DELETE FROM player_seasons
WHERE season_id IN (
    SELECT id FROM seasons WHERE started_at = TIMESTAMPTZ '2026-10-01 00:00:00+00'
);
