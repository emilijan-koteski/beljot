-- Reverse 000027: a documented NO-OP.
--
-- The up migration deletes the 2026 Q4 player_seasons rows that the old,
-- climb-only SP formula wrote before the win/loss ladder was released. Those
-- rows are deliberately discarded (Story 13.4): their SP was computed by a
-- formula the release retires, so restoring them would put old-formula points
-- back on the new ladder. There is nothing to recreate them from, and nothing
-- about the schema changed, so rolling back leaves the table exactly as it is.
SELECT 1;
