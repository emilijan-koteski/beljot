package match

import "time"

// CareerAggregates holds a user's career profile metrics: capots, average match
// duration, best hand and current streak. Zero values are valid for a user with no matches.
type CareerAggregates struct {
	Capots          int
	AvgMatchSeconds int
	BestHandPoints  int
	BestHandNumber  int
	BestHandAt      time.Time
	HasBestHand     bool
	StreakKind      string
	StreakLength    int
	LastPlayedAt    time.Time
	HasLastPlayed   bool
}

// PartnerAggregate is one most-played teammate row with matches and wins together.
// UserID still needs a username lookup by the caller.
type PartnerAggregate struct {
	UserID uint
	Played int
	Wins   int
}

// RivalAggregate is one most-faced opponent row with the viewer's wins and losses
// against them. UserID still needs a username lookup by the caller.
type RivalAggregate struct {
	UserID uint
	Wins   int
	Losses int
}

// MatchRepository defines the persistence interface for match records.
type MatchRepository interface {
	Create(match *Match) error

	CreateWithHands(match *Match, hands []HandResult) error

	GetMatchesForUser(userID uint, limit, offset int, outcome, sort string) (items []Match, total int64, err error)

	GetLastMatchForRoomAndUser(roomID, userID uint) (*Match, error)

	GetStatsForUser(userID uint) (wins, losses, abandoned int, err error)

	GetCareerAggregatesForUser(userID uint) (CareerAggregates, error)

	GetCareerPointsForUser(userID uint) (int64, error)

	GetTopPartnersForUser(userID uint, limit int) ([]PartnerAggregate, error)

	GetTopRivalsForUser(userID uint, limit int) ([]RivalAggregate, error)

	GetHonorTrendWindowsForUser(userID uint, limit int) (HonorTrendWindows, error)
}

// HonorTrendWindows holds the newest `limit` matches and the `limit` before them.
// Compare them only when equal in size; user.HonorTrendWindowed enforces that.
type HonorTrendWindows struct {
	RecentCompleted int
	RecentAbandoned int
	PriorCompleted  int
	PriorAbandoned  int
}
