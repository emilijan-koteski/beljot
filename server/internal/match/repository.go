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
// wins together (matches the viewer's team won — completed plus attributable
// abandoned rows, per-player semantics). UserID still needs a username lookup
// by the caller.
type PartnerAggregate struct {
	UserID uint
	Played int
	Wins   int
}

// RivalAggregate is one most-faced opponent row: the viewer's wins and losses
// against that opponent across completed matches plus attributable abandoned
// rows (per-player semantics). UserID still needs a username lookup by the
// caller.
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

	// GetHonorTrendWindowsForUser splits the user's most recent `2*limit` matches
	// into two adjacent windows — the newest `limit` and the `limit` before those
	// — counting completions versus abandonments in each (Story 9.7 trend).
	//
	// TWO windows rather than one, per the 2026-07-29 code review. The original
	// shape compared one 20-match window against the LIFETIME score, which is
	// not a like-for-like comparison: the Beta(4,1) prior contributes 4
	// pseudo-completions, whose drag is large at n=20 (capping a flawless window
	// at 100*24/25 = 96) and negligible at n=500 (where the lifetime score
	// reaches 98-100). A player who had never abandoned a match therefore read
	// "Slipping -3" forever. Two equal-size windows carry an identical prior
	// drag, so their difference reflects behaviour instead of sample size.
	//
	// This is the ONE honor read that must hit `matches`: the stored honor
	// weights on `users` are running aggregates and cannot be windowed. Call it
	// from the profile read path only — NOT from the auth/TopBar path and NOT
	// from Story 9.8's join gate, both of which are hot.
	//
	// The windows use the canonical viewer gate verbatim: a row counts as
	// abandoned-by-the-viewer when abandoned_by = userID, and as completed when
	// it is a 'completed' row or an abandonment by someone ELSE
	// (abandoned_by IS NOT NULL AND abandoned_by <> userID). Rows with
	// abandoned_by IS NULL are excluded entirely — they are boot-reconcile
	// rows, a server fault rather than a player signal, exactly as the
	// migration 000017 backfill treats them.
	//
	// Ordered by completed_at DESC with LIMIT and no OFFSET (deferred item D82:
	// offset pagination can duplicate rows under concurrent completions); the
	// split is done by ROW_NUMBER inside the single bounded window.
	GetHonorTrendWindowsForUser(userID uint, limit int) (HonorTrendWindows, error)
}

// HonorTrendWindows holds two adjacent slices of a user's recent match history:
// the newest `limit` matches and the `limit` immediately preceding them.
//
// Compare the two ONLY when they hold the same number of matches — see
// user.HonorTrendWindowed, which enforces that. Unequal windows differ in
// Bayesian prior drag, which is the exact defect the two-window shape exists to
// remove.
type HonorTrendWindows struct {
	RecentCompleted int
	RecentAbandoned int
	PriorCompleted  int
	PriorAbandoned  int
}
