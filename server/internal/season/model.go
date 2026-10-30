package season

import "time"

// Season is one competitive window: a UTC calendar quarter with StartedAt inclusive and EndsAt exclusive.
type Season struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"column:name" json:"name"`
	StartedAt time.Time `gorm:"column:started_at" json:"startedAt"`
	EndsAt    time.Time `gorm:"column:ends_at" json:"endsAt"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// PlayerSeason is one player's record inside one season, never rewritten once the season ends.
// The one deliberate rewrite is a queued recalculation (recalc.go).
type PlayerSeason struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	UserID         uint      `gorm:"column:user_id" json:"userId"`
	SeasonID       uint      `gorm:"column:season_id" json:"seasonId"`
	SP             int       `gorm:"column:sp" json:"sp"`
	RankTier       string    `gorm:"column:rank_tier" json:"rankTier"`
	RankDivision   *int      `gorm:"column:rank_division" json:"rankDivision"`
	GamesPlayed    int       `gorm:"column:games_played" json:"gamesPlayed"`
	GamesCompleted int       `gorm:"column:games_completed" json:"gamesCompleted"`
	GamesWon       int       `gorm:"column:games_won" json:"gamesWon"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// SeatResult is one human's part in a scored match: whether they were at the table at the end, and whether their team won.
type SeatResult struct {
	Completed bool
	Won       bool
}

// SPChanges computes one match's SP change per seated human from their locked current totals, before the 0 floor.
// It lets the formula run inside the repository's transaction while the repository stays persistence-only.
type SPChanges func(current map[uint]int) (map[uint]int, error)

// PlayerSeasonSnapshot is one player's season state right after the match-end write.
// PreviousSP is the locked pre-match total, and Division is 0 for Master and Grandmaster.
type PlayerSeasonSnapshot struct {
	SP             int
	PreviousSP     int
	Tier           string
	Division       int
	GamesPlayed    int
	GamesCompleted int
	GamesWon       int
}

// LeaderboardEntry is a scan target for one row of the leaderboard join of player_seasons and users.
// It stays separate from PlayerSeason and the wire DTO, and the join uses table names because season must not import user.
type LeaderboardEntry struct {
	UserID       uint    `gorm:"column:user_id"`
	Username     string  `gorm:"column:username"`
	AvatarKey    *string `gorm:"column:avatar_key"`
	SP           int     `gorm:"column:sp"`
	RankTier     string  `gorm:"column:rank_tier"`
	RankDivision *int    `gorm:"column:rank_division"`
	GamesPlayed  int     `gorm:"column:games_played"`
	GamesWon     int     `gorm:"column:games_won"`
}

// ArchiveEntry is a scan target for one ended, played season in a player's archive, with its stored rank snapshot.
// Its membership rule differs from leaderboardScope, so the two predicates must not share a helper.
type ArchiveEntry struct {
	SeasonID     uint      `gorm:"column:season_id"`
	SeasonName   string    `gorm:"column:season_name"`
	StartedAt    time.Time `gorm:"column:started_at"`
	EndsAt       time.Time `gorm:"column:ends_at"`
	SP           int       `gorm:"column:sp"`
	RankTier     string    `gorm:"column:rank_tier"`
	RankDivision *int      `gorm:"column:rank_division"`
	GamesPlayed  int       `gorm:"column:games_played"`
	GamesWon     int       `gorm:"column:games_won"`
}
