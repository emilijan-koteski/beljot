-- Change Username feature: records when the user last changed their username.
-- Drives the server-authoritative 30-day change cooldown and the profile UI
-- hint (next-allowed date). NULLABLE with no default: NULL means the username
-- has never been changed since registration, so the first change is always
-- allowed. TIMESTAMPTZ matches the users table's created_at/updated_at style
-- (unlike the DATE last_login_at, this needs instant precision for the cooldown).
ALTER TABLE users ADD COLUMN username_changed_at TIMESTAMPTZ;
