package match

import "time"

// CareerAggregates holds the non-list profile metrics computed across all of a
// user's matches: capot count (won by the viewer's team), average completed-
// match duration, the single best hand the viewer's team ever scored, and the
// current win/loss streak. Zero values are valid for a user with no matches.
type CareerAggregates struct {
	Capots          int
	AvgMatchSeconds int
	BestHandPoints  int
	BestHandNumber  int
	BestHandAt      time.Time
	HasBestHand     bool
	StreakKind      string // "win" | "loss" | "none"
	StreakLength    int
	LastPlayedAt    time.Time
	HasLastPlayed   bool
}

// PartnerAggregate is one most-played teammate row: matches played together and
// wins together (completed matches the viewer's team won). UserID still needs a
// username lookup by the caller.
type PartnerAggregate struct {
	UserID uint
	Played int
	Wins   int
}

// RivalAggregate is one most-faced opponent row: the viewer's wins and losses
// against that opponent across completed matches. UserID still needs a username
// lookup by the caller.
type RivalAggregate struct {
	UserID uint
	Wins   int
	Losses int
}

// MatchRepository defines the persistence interface for match records.
type MatchRepository interface {
	// Create inserts a Match row without any per-hand detail. Retained for
	// callers that either do not have hand data or are intentionally writing
	// only the aggregate record (tests, legacy paths).
	Create(match *Match) error

	// CreateWithHands inserts a Match and its buffered HandResult rows inside
	// a single transaction. If any insert fails, the transaction rolls back
	// so there are never orphaned hand rows or a match without its hands.
	// Pass nil or an empty slice when no hand data is available (e.g. a
	// match abandoned before the first hand was scored).
	CreateWithHands(match *Match, hands []HandResult) error

	// GetMatchesForUser returns a page of completed / abandoned matches in
	// which the given userID appears in any of player1..player4 seats. Hand
	// results are preloaded and ordered by hand_number ASC. total is the count
	// of all matching rows (after the outcome filter), regardless of
	// limit/offset.
	//
	// outcome filters the result set viewer-relative with per-player
	// abandonment semantics: "win"/"loss" match completed rows AND attributable
	// abandoned rows (abandoned_by set to someone other than the viewer) by
	// whether winnerTeam is the viewer's team; "abandoned" matches only the
	// viewer's own abandonments plus NULL-abandoner legacy rows; "" / "all"
	// leaves the completed+abandoned set unfiltered. sort controls ordering:
	// "old" → completed_at ASC, anything else (default "new") → completed_at
	// DESC, both tie-broken by id in the same direction.
	GetMatchesForUser(userID uint, limit, offset int, outcome, sort string) (items []Match, total int64, err error)

	// GetStatsForUser counts matches where userID appears in any of
	// player1..player4 seats, with per-player abandonment semantics. The
	// viewer's team derives from their seat (seats 0/2 → team A index 0; seats
	// 1/3 → team B index 1, mirroring game.TeamForSeat).
	// wins/losses = completed rows PLUS attributable abandoned rows (abandoned
	// by someone other than the viewer) where winnerTeam does / does not match
	// the viewer's team — winnerTeam on abandoned rows is the non-abandoning
	// team, meaningful only when abandoned_by is set (migration 000015).
	// abandoned = the viewer's own abandonments plus NULL-abandoner legacy
	// rows. Executed in a single round-trip via PostgreSQL FILTER aggregation
	// so wins + losses + abandoned is a consistent snapshot of participation
	// count.
	GetStatsForUser(userID uint) (wins, losses, abandoned int, err error)

	// GetCareerAggregatesForUser computes the viewer-relative career metrics
	// (capots won, average completed-match duration, best single hand, current
	// streak) across every match the user participated in.
	GetCareerAggregatesForUser(userID uint) (CareerAggregates, error)

	// GetTopPartnersForUser returns the most-played teammates (same-team seat)
	// ordered by matches played together, capped at limit.
	GetTopPartnersForUser(userID uint, limit int) ([]PartnerAggregate, error)

	// GetTopRivalsForUser returns the most-faced opponents (opposite-team
	// seats) ordered by completed matches played against them, capped at limit.
	GetTopRivalsForUser(userID uint, limit int) ([]RivalAggregate, error)
}
