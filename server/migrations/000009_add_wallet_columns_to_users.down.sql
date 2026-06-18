-- Reverse 000009 by dropping the three wallet columns in reverse order.
ALTER TABLE users DROP COLUMN login_streak_days;
ALTER TABLE users DROP COLUMN last_login_at;
ALTER TABLE users DROP COLUMN wallet_balance;
