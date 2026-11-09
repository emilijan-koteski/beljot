-- Reverse 000037 by dropping both columns. They hold rule configuration, not player
-- history, and dropping them returns every room and match to the standard window.
ALTER TABLE matches DROP COLUMN declaration_last_trick;
ALTER TABLE rooms DROP COLUMN declaration_last_trick;
