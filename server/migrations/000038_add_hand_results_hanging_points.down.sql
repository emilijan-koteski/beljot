-- Reverse 000038 by dropping the four hanging-points columns. Hand totals already
-- include any payout, so match scores and history totals stay correct without them.
ALTER TABLE hand_results DROP COLUMN hanging_paid_team;
ALTER TABLE hand_results DROP COLUMN hanging_paid_amount;
ALTER TABLE hand_results DROP COLUMN hanging_created_team;
ALTER TABLE hand_results DROP COLUMN hanging_created_amount;
