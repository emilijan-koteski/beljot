CREATE TABLE friendships (
    id         SERIAL PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,  -- requester
    friend_id  INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,  -- recipient
    status     VARCHAR(20) NOT NULL DEFAULT 'pending',                    -- 'pending' | 'accepted'
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_friendships_not_self CHECK (user_id <> friend_id)
);

-- The unique index is on the NORMALIZED (unordered) pair, so at most one row can
-- exist per pair regardless of direction. This closes the reverse-duplicate race
-- (B->A created concurrently with A->B, both slipping past the handler's
-- non-atomic FindByPair pre-check): the losing INSERT hits 23505, which the repo
-- maps to ErrFriendRequestExists (a clean 409). FindByPair stays the fast
-- pre-check; this index is the atomic backstop. No soft-delete column:
-- decline/unfriend hard-delete, like user_identities / refresh_tokens.
-- ON DELETE CASCADE drops a user's rows with the user.
CREATE UNIQUE INDEX idx_friendships_pair ON friendships (LEAST(user_id, friend_id), GREATEST(user_id, friend_id));
CREATE INDEX idx_friendships_friend_id ON friendships (friend_id);
CREATE INDEX idx_friendships_user_id_status ON friendships (user_id, status);
