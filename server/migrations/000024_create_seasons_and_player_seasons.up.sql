-- Seasonal rank (Story 13.1): Season Points (SP) accrue per match into a
-- per-player, per-season row, and an 8-tier ladder is DERIVED from the total.
--
-- TWO TABLES, ONE JOB.
--   seasons        - the competitive window. Calendar quarters in UTC:
--                    Q1 Jan 1-Apr 1, Q2 Apr 1-Jul 1, Q3 Jul 1-Oct 1,
--                    Q4 Oct 1-Jan 1. started_at INCLUSIVE, ends_at EXCLUSIVE,
--                    so consecutive windows neither gap nor overlap.
--   player_seasons - one row per (user, season). SP is an accumulator; the two
--                    counters are the season's participation record.
--
-- NO BACKFILL, ON PURPOSE. Unlike 000017 (honor), nothing is reconstructed from
-- `matches` here and nothing should ever be. A season is a FRESH competitive
-- window: SP starts at zero for everyone, by design, and there is no historical
-- SP to recover -- the SP formula did not exist when those matches were played,
-- so any "backfill" would be an invention. Do not later "fix" this omission.
--
-- WHY seasons IS SEEDED BELOW. Story 13.1 ships no scheduler (that is Story
-- 13.3), so the season resolver in server/internal/season/gorm_repo.go is LAZILY
-- SELF-HEALING: it reads the row covering `now` and, on a miss, computes the
-- calendar quarter and inserts it idempotently. The seed here means the table is
-- never empty on a fresh deploy; the resolver means the system cannot silently
-- stop accruing SP the day the seeded window ends. Story 13.3 then owns
-- SCHEDULED rollover (creating the next window ahead of time plus the prior-
-- season archive), not correctness of the current window.

CREATE TABLE seasons (
    id         SERIAL PRIMARY KEY,
    -- Machine-stable token, "YYYY QN" (e.g. "2026 Q3"). NOT a display string:
    -- the client renders it verbatim as an identifier and never translates it.
    -- A themed marketing name ("Season 1: Ember") would be a SEPARATE
    -- display-name column, added when it is actually asked for.
    name       VARCHAR(32) NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    ends_at    TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_seasons_window CHECK (ends_at > started_at),
    -- The conflict target the lazy resolver's INSERT ... ON CONFLICT needs, and
    -- the atomic backstop against two concurrent match-end resolutions both
    -- inserting the same quarter. A window is identified by where it STARTS.
    CONSTRAINT uq_seasons_started_at UNIQUE (started_at)
);

CREATE TABLE player_seasons (
    id         SERIAL PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    season_id  INTEGER NOT NULL REFERENCES seasons (id) ON DELETE CASCADE,
    -- BIGINT, not INTEGER: this is an accumulator summed by a 64-bit Go int.
    -- Exactly the width trap the Story 9.5 review caught when total_xp shipped
    -- as INTEGER and had to be widened. SP never decreases (PRD: "No decay"),
    -- so the CHECK is a guard against a bad write, not a business rule.
    sp         BIGINT NOT NULL DEFAULT 0 CHECK (sp >= 0),
    -- DENORMALIZED SNAPSHOT - READ THIS BEFORE USING IT.
    -- rank_tier exists ONLY so operators and Story 13.2's leaderboard query can
    -- sort and filter in SQL. It is refreshed on every SP write and is
    -- therefore ALLOWED TO LAG. The authoritative tier is always
    -- season.TierForSP(sp) -- pure arithmetic over a value already loaded.
    --
    -- Unlike users.honor_score (000017) there is no decay here and SP is
    -- monotonic, so stored and derived can never actually disagree. The derived
    -- call stays the single source anyway, so no future reader learns the wrong
    -- habit from this column.
    rank_tier  VARCHAR(16) NOT NULL DEFAULT 'iron',
    -- +1 for EVERY human seat in a finished match, present at the terminal end
    -- or not. Bot and empty seats increment neither counter.
    games_played    INTEGER NOT NULL DEFAULT 0 CHECK (games_played >= 0),
    -- +1 only for seats PRESENT at the terminal end -- the same gate SP
    -- eligibility uses (Story 13.1 D5, which reuses honor's per-seat presence
    -- rule). So games_completed is exactly "matches where this player earned
    -- SP", and games_played - games_completed is their in-season absence count.
    games_completed INTEGER NOT NULL DEFAULT 0 CHECK (games_completed >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The upsert conflict target for the match-end write, AND the atomic backstop
-- against two concurrent match-end writes both inserting a first row for the
-- same (user, season). Mirrors idx_friendships_pair's role in 000019.
CREATE UNIQUE INDEX idx_player_seasons_user_season ON player_seasons (user_id, season_id);

-- Story 13.2's leaderboard read: the top N of one season by SP. Added here
-- because the schema AC (Story 13.1 AC4) lives here, even though its only
-- consumer ships in the next story.
CREATE INDEX idx_player_seasons_season_sp ON player_seasons (season_id, sp DESC);

-- Seed the calendar quarter containing deploy time, so the table is never empty
-- and the first match end after deploy has a window to write into. Idempotent
-- via the UNIQUE (started_at) conflict target, so a down/up cycle (or a second
-- run) is safe.
-- EVERY EXPRESSION BELOW RUNS ON THE NAIVE (timestamp WITHOUT time zone) QUARTER
-- START, and only the two output columns are cast back to timestamptz. That is
-- load-bearing, not stylistic: `+ INTERVAL '3 months'` and `to_char(...)` applied
-- to a TIMESTAMPTZ are evaluated in the SESSION TimeZone, so under, say,
-- TimeZone='America/New_York' the Q4 window came out named "2026 Q3" and ended at
-- 2026-12-31T01:00Z instead of 2027-01-01T00:00Z (the EDT->EST shift), which both
-- misnames the row and leaves a one-hour hole that the Go resolver's half-open
-- [started_at, ends_at) lookup falls straight through. Dormant while the server
-- runs UTC; a latent bug the first time it does not. Keep the arithmetic naive.
INSERT INTO seasons (name, started_at, ends_at)
SELECT to_char(q.qstart, 'YYYY "Q"Q'),
       q.qstart AT TIME ZONE 'UTC',
       (q.qstart + INTERVAL '3 months') AT TIME ZONE 'UTC'
  FROM (
        SELECT date_trunc('quarter', NOW() AT TIME ZONE 'UTC') AS qstart
       ) AS q
    ON CONFLICT (started_at) DO NOTHING;
