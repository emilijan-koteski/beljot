CREATE TABLE user_identities (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider VARCHAR(20) NOT NULL,
    provider_user_id VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One local account per external identity, and one identity per provider per
-- account. Constraint names carry "provider_subject" / "user_provider" so the
-- repo's 23505 mapping can tell them apart if it ever needs to.
CREATE UNIQUE INDEX idx_user_identities_provider_subject ON user_identities (provider, provider_user_id);
CREATE UNIQUE INDEX idx_user_identities_user_provider ON user_identities (user_id, provider);
