CREATE TABLE room_bots (
    id SERIAL PRIMARY KEY,
    room_id INTEGER NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    seat INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_room_bots_room_seat ON room_bots(room_id, seat);
CREATE INDEX idx_room_bots_room_id ON room_bots(room_id);

-- Bot seats have no user account, so the per-seat player FKs become nullable.
-- The FK constraints remain and still validate every non-NULL value.
ALTER TABLE matches ALTER COLUMN player1_id DROP NOT NULL;
ALTER TABLE matches ALTER COLUMN player2_id DROP NOT NULL;
ALTER TABLE matches ALTER COLUMN player3_id DROP NOT NULL;
ALTER TABLE matches ALTER COLUMN player4_id DROP NOT NULL;

ALTER TABLE matches ADD COLUMN player1_is_bot BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE matches ADD COLUMN player2_is_bot BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE matches ADD COLUMN player3_is_bot BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE matches ADD COLUMN player4_is_bot BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE matches ADD COLUMN has_bots BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX idx_matches_has_bots ON matches(has_bots);
