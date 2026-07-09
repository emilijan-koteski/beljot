-- Restore the pre-backfill semantics: abandoned rows always persisted the
-- filler winner_team = 0 regardless of the abandoner's team. NULL-abandoner
-- rows were never touched by the up migration, so nothing to undo there.
--
-- Pairs with rolling back the reader code: the old code ignores winner_team
-- on abandoned rows, so filler 0 restores today's behavior exactly. Nothing
-- is permanently lost — re-running the up migration recomputes every
-- attributable row from abandoned_by.
UPDATE matches
SET winner_team = 0
WHERE status = 'abandoned'
  AND abandoned_by IS NOT NULL;
