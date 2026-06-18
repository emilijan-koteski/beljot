-- Wallet foundation (Story 9.1): the player's coin wallet lives on the users
-- table, not its own table. Three additive columns, mirroring the additive
-- ALTER style of 000007_add_bot_players.
--
-- wallet_balance defaults to 5000 (the new-player seed). The default both seeds
-- new rows and backfills any existing rows (none in prod). Keep this default in
-- sync with wallet.StartingBalance (server/internal/wallet/service.go).
ALTER TABLE users ADD COLUMN wallet_balance INTEGER NOT NULL DEFAULT 5000 CHECK (wallet_balance >= 0);

-- last_login_at is the UTC calendar date of the player's last counted session.
-- Nullable, no default: an existing user (last_login_at = NULL) is treated as a
-- fresh streak (resets to 1) on their first bootstrap after this migration —
-- acceptable, since there are no production rows yet.
ALTER TABLE users ADD COLUMN last_login_at DATE;

-- login_streak_days defaults to 0. Registration stamps streak 0 + last_login =
-- today, so the day-1 bonus first becomes claimable on the next calendar day.
ALTER TABLE users ADD COLUMN login_streak_days INTEGER NOT NULL DEFAULT 0;
