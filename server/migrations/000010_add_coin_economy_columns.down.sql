-- Reverse 000010 by dropping the six economy columns in reverse order.
ALTER TABLE matches DROP COLUMN player4_coin_delta;
ALTER TABLE matches DROP COLUMN player3_coin_delta;
ALTER TABLE matches DROP COLUMN player2_coin_delta;
ALTER TABLE matches DROP COLUMN player1_coin_delta;
ALTER TABLE matches DROP COLUMN coin_buy_in;
ALTER TABLE rooms DROP COLUMN coin_buy_in;
