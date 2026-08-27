package season

import "time"

// Season is one competitive window (migration 000024). Windows are calendar
// quarters in UTC with StartedAt inclusive and EndsAt exclusive, so exactly one
// row covers any given instant -- see quarter.go.
//
// GORM auto-pluralizes the struct name to "seasons", which matches the table, so
// no TableName() override is needed.
type Season struct {
	ID uint `gorm:"primaryKey" json:"id"`
	// Machine-stable "YYYY QN" token (e.g. "2026 Q3"), NOT a display string.
	// The client renders it verbatim as an identifier and never translates it.
	Name      string    `gorm:"column:name" json:"name"`
	StartedAt time.Time `gorm:"column:started_at" json:"startedAt"`
	EndsAt    time.Time `gorm:"column:ends_at" json:"endsAt"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// PlayerSeason is one player's record inside one season (migration 000024).
// Rows are IMMUTABLE ACROSS SEASONS: the soft reset at rollover is "a new
// season_id", never an update or a compression of the old row, which is what
// lets Story 13.3 archive a prior season by reading it back unchanged.
//
// GORM auto-pluralizes to "player_seasons", matching the table.
type PlayerSeason struct {
	ID       uint `gorm:"primaryKey" json:"id"`
	UserID   uint `gorm:"column:user_id" json:"userId"`
	SeasonID uint `gorm:"column:season_id" json:"seasonId"`
	// Accumulated Season Points. Monotonic -- there is no decay (PRD: "No
	// decay") and no spend.
	SP int `gorm:"column:sp" json:"sp"`
	// DENORMALIZED SNAPSHOT, never authoritative (Story 13.1 D7). It exists only
	// so operators and Story 13.2's leaderboard can sort/filter in SQL. Always
	// read the tier as TierForSP(SP) instead -- see the column comment in
	// 000024_create_seasons_and_player_seasons.up.sql.
	RankTier string `gorm:"column:rank_tier" json:"rankTier"`
	// +1 for every human seat in a finished match, present or not.
	GamesPlayed int `gorm:"column:games_played" json:"gamesPlayed"`
	// +1 only for seats present at the terminal end -- exactly "matches where
	// this player earned SP" (Story 13.1 D10).
	GamesCompleted int       `gorm:"column:games_completed" json:"gamesCompleted"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// SPAward is one finished match's Season Points contribution for one player, as
// the repository consumes it.
//
// A ZERO SP AWARD IS NOT A NO-OP. Every human seat in the match gets an entry,
// including seats that earned nothing by being absent at the terminal end,
// because GamesPlayed increments for all of them. Completed is the per-seat
// presence gate: true means the seat was at the table when the match ended, and
// it drives GamesCompleted.
type SPAward struct {
	SP        int
	Completed bool
}

// PlayerSeasonSnapshot is one player's season state immediately after the
// match-end write, as returned by ApplySeasonPoints.
//
// PreviousSP is the total BEFORE this award, returned so the caller can decide
// tieredUp without a second read. Tier is the AUTHORITATIVE derived value
// (TierForSP over the new total), never the lagging rank_tier column.
type PlayerSeasonSnapshot struct {
	SP             int
	PreviousSP     int
	Tier           string
	GamesPlayed    int
	GamesCompleted int
}
