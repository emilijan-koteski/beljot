-- Bitola hanging points on each hand's history row. A tied Bitola hand leaves the
-- caller's half hanging (hanging_created_*), and the next scored hand pays it out to
-- the side that takes that hand (hanging_paid_*). The team columns are 0 or 1.
--
-- All four columns are nullable: NULL means no tie hung and nothing was paid, which
-- is every existing row and every Croatian hand. NO BACKFILL IS NEEDED.
ALTER TABLE hand_results ADD COLUMN hanging_created_amount INTEGER;
ALTER TABLE hand_results ADD COLUMN hanging_created_team SMALLINT;
ALTER TABLE hand_results ADD COLUMN hanging_paid_amount INTEGER;
ALTER TABLE hand_results ADD COLUMN hanging_paid_team SMALLINT;
