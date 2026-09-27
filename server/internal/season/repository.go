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

	// ApplySeasonPoints scores one finished match for the given season in ONE
	// transaction and returns each player's post-write snapshot.
	//
	// `completed` lists every human seat of the match, keyed by user ID, with its
	// presence at the terminal end. In ascending user-ID order the repository
	// makes sure each player has a row (a missing row is a zero row) and LOCKS it;
	// it then hands every locked total to `changes` at once, floors each new
	// total at 0, and writes sp, the denormalized rank_tier, games_played += 1
	// and games_completed += 1 for a present seat.
	//
	// Reading the totals and writing the new ones under one lock is the point: a
	// match that finishes concurrently with another sharing a player waits for
	// the first to commit and then scores from its result, so neither change is
	// lost. An empty map is a no-op with no DB round-trip.
	//
	// All-or-nothing, like the wallet and XP paths: a failure (including an error
	// from `changes`, or a change missing for a listed player) rolls the whole
	// batch back and returns the error, so four seats never end up half-credited.
	ApplySeasonPoints(seasonID uint, completed map[uint]bool, changes SPChanges) (map[uint]PlayerSeasonSnapshot, error)

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

	// LeaderboardPage returns one offset page of a season's standings, ordered
	// best-first, plus the TOTAL number of rows the same predicate matches (not
	// the page length) so the caller can drive load-more paging.
	//
	// PRECONDITION: limit >= 1 and offset >= 0. Violations return an error rather
	// than a page -- `limit` reaches both a slice pre-allocation (which panics on
	// a negative) and SQL LIMIT (where a negative means "no limit" and returns the
	// whole season). The HTTP handler rejects bad user input with 400 long before
	// this; the check exists for in-process callers that never saw a query string.
	//
	// LeaderboardPage, CountAhead and FindLeaderboardEntry below share ONE
	// VISIBILITY PREDICATE and ONE TOTAL ORDER, and both properties are
	// load-bearing:
	//
	//   predicate  season_id = ? AND users.deleted_at IS NULL
	//              AND games_played >= 1.
	//              The join is written by table name, so GORM's soft-delete scope
	//              does NOT apply and the filter is spelled out (see
	//              internal/room/gorm_repo.go:298 for the live leak this avoids).
	//              `games_played >= 1` is a MEMBERSHIP RULE (Story 13.4): the
	//              ladder is everyone who played the season, INCLUDING players at
	//              0 SP, because SP now falls and a player who loses back down to
	//              0 must keep their place and their viewer block. Excluded rows
	//              are missing from `items`, missing from `total`, and counted in
	//              nobody's position -- all three, or the numbers contradict each
	//              other.
	//   order      sp DESC, user_id ASC. The tiebreak is not cosmetic: without a
	//              second column, two players on equal SP can swap between the
	//              page-1 and page-2 queries and be duplicated or skipped.
	//
	// The TIER IS NOT SELECTED. rank_tier is a denormalized snapshot allowed to
	// lag (Story 13.1 D7); callers derive it with TierForSP(sp). Sorting is by
	// `sp`, which is authoritative.
	//
	// NO METHOD BELOW WRITES ANYTHING -- see FindPlayerSeason's contract above. A
	// leaderboard read that materialised a player_seasons row would list everyone
	// who merely opened the lobby.
	LeaderboardPage(seasonID uint, limit, offset int) ([]LeaderboardEntry, int64, error)

	// FindLeaderboardEntry returns the player's own listable standing, or
	// (nil, nil) when they have none.
	//
	// It answers the viewer block, and it exists SEPARATELY FROM FindPlayerSeason
	// precisely so the viewer is subject to the LIST'S OWN VISIBILITY PREDICATE.
	// FindPlayerSeason sees no `users` join, so it returns rows for soft-deleted
	// accounts and for rows with no games played -- either of which would hand a
	// caller a position counted against a population that does not include them,
	// plus a pinned row they cannot find in the list. A miss therefore covers
	// three cases the caller treats identically: no row, a row with no games, and
	// a soft-deleted account.
	FindLeaderboardEntry(seasonID, userID uint) (*LeaderboardEntry, error)

	// CountAhead returns how many of the season's visible rows sort STRICTLY
	// AHEAD of (sp, userID) under LeaderboardPage's own total order, so the
	// viewer's position is CountAhead + 1.
	//
	// It is a bounded COUNT, not a window function: `sp > ? OR (sp = ? AND
	// user_id < ?)`. A plain COUNT(sp > vSP) would hand every player tied at, say,
	// 900 SP the SAME position, while the list numbers them offset+i+1 -- so a
	// tied viewer could be told "position 4" while standing in slot 6 of the page
	// they are looking at. Counting rows that sort ahead under the FULL order is
	// what makes the two agree.
	//
	// The row need not exist: the caller only calls this once FindPlayerSeason has
	// returned one.
	CountAhead(seasonID uint, sp int, userID uint) (int64, error)

	// --- Story 13.3 reads. NONE of the three methods below writes anything: ---
	// they never create a player_seasons row (FindPlayerSeason's contract above),
	// and unlike CurrentSeason they never create a SEASON row either — a by-id
	// lookup, a listing, and an archive read have no window to lazily heal.

	// FindSeasonByID returns the season row with that id, or (nil, nil) on a
	// miss. The miss is data, not an error: the leaderboard's ?season=<id>
	// selector maps it to apperr.ErrSeasonNotFound at the service layer.
	FindSeasonByID(id uint) (*Season, error)

	// ListSeasons returns every season row, newest-first (started_at DESC) —
	// the order the leaderboard picker renders verbatim. Never nil on success:
	// an empty table is an empty slice.
	ListSeasons() ([]Season, error)

	// PlayerSeasonArchive returns the player's ENDED, PLAYED seasons,
	// newest-first (seasons.started_at DESC). Membership is
	//
	//   row exists AND games_played >= 1 AND seasons.ends_at <= now
	//
	// — the same games_played rule as leaderboardScope, but its own predicate:
	// a played season with 0 SP is included, while the ACTIVE window is always
	// excluded (its record is still moving; the archive is immutable history).
	// An unknown user is an empty slice, not an error — the profile query owns
	// user-existence 404s. Never nil on success.
	PlayerSeasonArchive(userID uint, now time.Time) ([]ArchiveEntry, error)
}
