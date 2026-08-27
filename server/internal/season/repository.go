package season

import "time"

// Repository is the persistence boundary for seasons and per-player season
// records. The service and the handler depend on this interface, never on GORM
// directly.
type Repository interface {
	// CurrentSeason returns the season window covering now, creating it if it
	// does not exist yet (Story 13.1 D1 -- the LAZILY SELF-HEALING resolver).
	//
	// On a miss it computes the calendar quarter containing now, inserts it with
	// INSERT ... ON CONFLICT (started_at) DO NOTHING, and re-reads. That is what
	// makes Story 13.3's scheduler an OPTIMISATION rather than a prerequisite:
	// without it, SP accrual would silently stop the day the seeded window ended.
	//
	// Never returns (nil, nil): either a window or an error.
	CurrentSeason(now time.Time) (*Season, error)

	// ApplySeasonPoints applies every listed player's award to the given season
	// in ONE transaction and returns each player's post-write snapshot.
	//
	// Per user it upserts on (user_id, season_id): sp += award.SP,
	// games_played += 1, games_completed += 1 when award.Completed, and refreshes
	// the denormalized rank_tier from the new total. An empty map is a no-op with
	// no DB round-trip.
	//
	// All-or-nothing, like the wallet and XP paths: a failure rolls the whole
	// batch back and returns the error, so four seats never end up half-credited.
	ApplySeasonPoints(seasonID uint, awards map[uint]SPAward) (map[uint]PlayerSeasonSnapshot, error)

	// FindPlayerSeason returns the player's row for a season, or (nil, nil) when
	// they have not played in it yet.
	//
	// A READ NEVER WRITES player_seasons. A player with no row is the zero state
	// (0 SP, Iron), which the caller renders directly. It is never a 404 and never
	// a lazily created row -- a GET that inserted one would put a row in Story
	// 13.2's leaderboard for anyone who merely opened the lobby.
	//
	// Note the boundary precisely: the read path CAN write `seasons`, because
	// CurrentSeason above is a lazy resolver and GET /api/v1/seasons/current goes
	// through it. Creating the season window on demand is idempotent, bounded to
	// one row per quarter, and identical to what the write path would create; a
	// per-player row is neither. So the invariant is "reads do not create PLAYER
	// records", not "reads do not write".
	FindPlayerSeason(userID, seasonID uint) (*PlayerSeason, error)
}
