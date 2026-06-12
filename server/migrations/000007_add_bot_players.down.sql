DROP TABLE room_bots;

-- Bot-inclusive matches hold NULL player IDs that would block restoring the
-- NOT NULL constraints below; their hand_results rows cascade with them.
DELETE FROM matches WHERE has_bots = TRUE;

DROP INDEX idx_matches_has_bots;

ALTER TABLE matches DROP COLUMN has_bots;
ALTER TABLE matches DROP COLUMN player4_is_bot;
ALTER TABLE matches DROP COLUMN player3_is_bot;
ALTER TABLE matches DROP COLUMN player2_is_bot;
ALTER TABLE matches DROP COLUMN player1_is_bot;

ALTER TABLE matches ALTER COLUMN player1_id SET NOT NULL;
ALTER TABLE matches ALTER COLUMN player2_id SET NOT NULL;
ALTER TABLE matches ALTER COLUMN player3_id SET NOT NULL;
ALTER TABLE matches ALTER COLUMN player4_id SET NOT NULL;
