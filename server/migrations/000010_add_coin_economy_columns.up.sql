-- Coin economy (Story 9.2): room buy-in + per-match settlement record. Additive
-- ALTER style, mirroring 000009_add_wallet_columns_to_users / 000007_add_bot_players.
--
-- All columns default to 0 so existing rows backfill to 0 — i.e. treated as a
-- 0-stake / no-economy room or match, which is correct for every pre-9.2 row.

-- rooms.coin_buy_in: the per-human stake a room owner sets at create time.
-- min 0, no maximum (owner freedom); quick-play rooms persist 0 in this story
-- (bracketed stakes arrive in Story 9.4). CHECK guards against negatives.
ALTER TABLE rooms ADD COLUMN coin_buy_in INTEGER NOT NULL DEFAULT 0 CHECK (coin_buy_in >= 0);

-- matches.coin_buy_in: the stake captured at StartMatch for THIS match, immune
-- to later edits of rooms.coin_buy_in.
ALTER TABLE matches ADD COLUMN coin_buy_in INTEGER NOT NULL DEFAULT 0;

-- matches.player{1..4}_coin_delta: the net wallet change per seat for this match
-- (winner: share - buy_in; loser: -buy_in; bot seat: 0). Mirrors the existing
-- per-seat player{N}_is_bot denormalization.
ALTER TABLE matches ADD COLUMN player1_coin_delta INTEGER NOT NULL DEFAULT 0;
ALTER TABLE matches ADD COLUMN player2_coin_delta INTEGER NOT NULL DEFAULT 0;
ALTER TABLE matches ADD COLUMN player3_coin_delta INTEGER NOT NULL DEFAULT 0;
ALTER TABLE matches ADD COLUMN player4_coin_delta INTEGER NOT NULL DEFAULT 0;
