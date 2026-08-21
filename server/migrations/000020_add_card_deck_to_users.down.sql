-- Reverse 000020 by dropping the card-deck preference column.
--
-- Nothing is unrecoverable: the column holds a purely visual player setting, not
-- accumulated history, and dropping it returns every player to the French deck —
-- exactly the state the up migration's default describes.
ALTER TABLE users DROP COLUMN card_deck_preference;
