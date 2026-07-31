-- Honor score (Story 9.7): a per-player reliability signal — "how often does
-- this player finish the matches they start?" — stored on the users table
-- alongside the wallet (000009) and XP (000011) columns.
--
-- WHY STORED AND NOT DERIVED. Honor could be computed from `matches` on every
-- read, but two requirements forbid it (Story 9.7 D3):
--   1. Forgiveness. An operator must be able to pardon or reset a player, and
--      a pure function over immutable match history cannot be overridden.
--   2. Story 9.8's per-join honor gate reads honor on every join attempt; it
--      must be a single column read, not a four-way aggregate over `matches`.
--
-- THE TWO BUCKETS. Only `completed` and `abandoned` exist (Story 9.7 D1). A
-- rage quit reaches this server as a socket close that is byte-identical to a
-- network drop, and per-move timer expiry auto-plays a card rather than
-- abandoning anyone — so there are deliberately NO rage_quits / timeout_abandons
-- columns. Do not add them without a new abandonment trigger to feed them.
--
-- THE DECAY TRICK. Each match contributes a recency weight
-- w = 0.5 ^ (age_days / 90), so old abandonments are forgiven over time. Because
-- every term decays by the same factor,
--
--     Σ 0.5^((now − tᵢ)/H)  =  0.5^((now − last)/H) · Σ 0.5^((last − tᵢ)/H)
--
-- "decay the stored running weight forward, then add the new event" is
-- algebraically IDENTICAL to summing per-match weights from scratch. That is why
-- a running weight plus a reference timestamp (honor_decayed_at) is exact, not
-- an approximation.
--
-- FORGIVENESS RECIPE (Story 9.7 AC9 / D7 — there is no admin UI, by design).
-- The repository ships user.UserRepository.ResetHonor(userID); the equivalent
-- hand-run SQL is:
--
--     UPDATE users
--        SET honor_completed_weight = 0,
--            honor_abandoned_weight = 0,
--            honor_decayed_at       = NULL,
--            honor_abandoned_total  = 0,
--            honor_score            = 80
--      WHERE id = <userID>;
--
-- Note what is NOT in that list: honor_completed_total. A pardon clears the
-- PENALTY, not the EXPERIENCE. That column is the only input to IsNewPlayer, so
-- zeroing it turns a pardoned veteran into a "New Player" — the profile hides
-- their score behind the newcomer chip while StatsGrid still shows their real
-- history, and Story 9.8's join gate reclassifies them as a newcomer instead of
-- as a clean veteran. Keep it.
--
-- THE BACKFILL BELOW IS DEPLOY-ONLY. DO NOT RE-RUN IT.
-- It writes ABSOLUTE values derived purely from `matches`, so re-running it:
--   (1) destroys every forgiveness applied via ResetHonor or the recipe above,
--       silently and unrecoverably — which defeats the whole reason honor is
--       stored rather than derived (see the top of this header); and
--   (2) reverts the concurrent-double-disconnect rule, because `matches` records
--       a single abandoned_by and no per-seat presence, so this query cannot tell
--       that a second absent seat was also charged an abandonment at write time
--       and will re-credit it a completion.
-- If the denormalized counters are ever suspected of drifting, reconcile with a
-- query that EXCLUDES pardoned rows and accepts (2) as a known floor, or add
-- per-seat presence to the match row first. Do not just re-run this.

-- Decayed running weight of this player's COMPLETED matches. NUMERIC(14,6)
-- rather than a float: decay produces fractional values and silent binary
-- float drift in a trust signal that gates room access (Story 9.8) is not
-- acceptable. 14 digits of precision is far beyond the reachable magnitude
-- (the weight converges to at most ~one match per half-life of play).
ALTER TABLE users ADD COLUMN honor_completed_weight NUMERIC(14,6) NOT NULL DEFAULT 0 CHECK (honor_completed_weight >= 0);

-- Decayed running weight of this player's ABANDONED matches (reconnect-window
-- expiry — the sole abandonment trigger). Same type rationale as above.
ALTER TABLE users ADD COLUMN honor_abandoned_weight NUMERIC(14,6) NOT NULL DEFAULT 0 CHECK (honor_abandoned_weight >= 0);

-- Reference timestamp the two weights above were last decayed to. NULLABLE on
-- purpose: NULL means "never decayed", and the pure Go DecayFactor treats it as
-- a factor of exactly 1.0. TIMESTAMPTZ (not DATE) because the half-life math is
-- continuous in seconds, not bucketed by day.
ALTER TABLE users ADD COLUMN honor_decayed_at TIMESTAMPTZ;

-- Raw lifetime count of completed matches — undecayed, monotonic. Together with
-- honor_abandoned_total it drives the "New Player" suppression: fewer than 5
-- matches PLAYED (completed + abandoned) hides the score, so the floor counts
-- EXPERIENCE rather than successes. Also rendered directly as a plain count.
--
-- This column SURVIVES a forgiveness reset — see the recipe in the header.
--
-- BIGINT, not INTEGER: this is a lifetime accumulator summed by a 64-bit Go int,
-- exactly the width trap the Story 9.5 review caught when total_xp shipped as
-- INTEGER and had to be widened.
ALTER TABLE users ADD COLUMN honor_completed_total BIGINT NOT NULL DEFAULT 0 CHECK (honor_completed_total >= 0);

-- Raw lifetime count of abandoned matches. Same type rationale as above.
ALTER TABLE users ADD COLUMN honor_abandoned_total BIGINT NOT NULL DEFAULT 0 CHECK (honor_abandoned_total >= 0);

-- DENORMALIZED SNAPSHOT — READ THIS BEFORE USING IT.
-- honor_score exists ONLY so operators and future SQL-side features can filter
-- and sort (WHERE honor_score < 50). It is refreshed on every honor write and
-- is therefore ALLOWED TO LAG: decay means the true score changes as time
-- passes even when nothing is written, so a stored score is stale by design.
--
-- NEVER render this column and NEVER gate on it. The authoritative value is
-- always user.HonorScore(completedWeight, abandonedWeight, decayedAt, now) —
-- pure arithmetic over three columns of a row you have already loaded.
--
-- SMALLINT because the value is bounded 0-100. DEFAULT 80 is the Beta(4,1)
-- prior — 100 * 4 / 5 — which is what a player with no history scores.
ALTER TABLE users ADD COLUMN honor_score SMALLINT NOT NULL DEFAULT 80 CHECK (honor_score BETWEEN 0 AND 100);

-- Backfill from existing match history so live players do not all reset to the
-- 80 prior on deploy.
--
--   completed = every seat of a status='completed' match (natural end AND
--               accepted surrender — the Status column stays 'completed'),
--               PLUS the three non-abandoning seats of an abandoned match.
--   abandoned = the single seat named by matches.abandoned_by.
--
-- Rows with abandoned_by IS NULL are EXCLUDED ENTIRELY: those are boot-reconcile
-- rows for stale sessions, which are a server fault and not a player signal
-- (Story 9.7 AC3). Bot seats carry NULL player IDs and drop out naturally.
WITH participation AS (
    SELECT
        p.user_id,
        (m.status = 'abandoned' AND m.abandoned_by = p.user_id) AS is_abandoner,
        -- LEAST(1, ...) mirrors DecayFactor's `if elapsed <= 0 { return 1.0 }`
        -- guard. matches.completed_at is written from the APP HOST clock while
        -- this runs on the POSTGRES clock, so a host running ahead makes the
        -- interval negative, the exponent negative, and the weight > 1 — which
        -- would backfill weights ABOVE the real match count and inflate scores.
        -- The CHECK (>= 0) does not catch that. Without this clamp the two paths
        -- would not "agree exactly" as this file claims.
        LEAST(1, power(0.5, EXTRACT(EPOCH FROM (NOW() - m.completed_at)) / (90 * 86400))) AS w
    FROM matches m
    CROSS JOIN LATERAL (
        VALUES (m.player1_id), (m.player2_id), (m.player3_id), (m.player4_id)
    ) AS p(user_id)
    WHERE p.user_id IS NOT NULL
      AND (
            m.status = 'completed'
         OR (m.status = 'abandoned' AND m.abandoned_by IS NOT NULL)
          )
),
agg AS (
    SELECT
        user_id,
        COALESCE(SUM(w) FILTER (WHERE NOT is_abandoner), 0)::numeric AS completed_weight,
        COALESCE(SUM(w) FILTER (WHERE is_abandoner), 0)::numeric     AS abandoned_weight,
        COUNT(*) FILTER (WHERE NOT is_abandoner)                     AS completed_total,
        COUNT(*) FILTER (WHERE is_abandoner)                         AS abandoned_total
    FROM participation
    GROUP BY user_id
)
UPDATE users u
   SET honor_completed_weight = ROUND(agg.completed_weight, 6),
       honor_abandoned_weight = ROUND(agg.abandoned_weight, 6),
       honor_completed_total  = agg.completed_total,
       honor_abandoned_total  = agg.abandoned_total,
       -- Weights are decayed to "right now", so that is the reference stamp.
       honor_decayed_at       = NOW(),
       -- Mirrors the Go formula exactly: 100*(C+4)/(C+4+4A+1), rounded, clamped.
       -- Postgres ROUND(numeric) rounds half away from zero, matching math.Round.
       honor_score            = LEAST(100, GREATEST(0, ROUND(
           100 * (agg.completed_weight + 4)
               / (agg.completed_weight + 4 + 4 * agg.abandoned_weight + 1)
       )))::smallint
  FROM agg
 WHERE u.id = agg.user_id;
