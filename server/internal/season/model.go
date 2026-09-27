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
// lets Story 13.3 archive a prior season by reading it back unchanged. The one
// deliberate rewrite is a queued recalculation (recalc.go, migration 000029),
// which re-scores a whole season from its stored matches.
//
// GORM auto-pluralizes to "player_seasons", matching the table.
type PlayerSeason struct {
	ID       uint `gorm:"primaryKey" json:"id"`
	UserID   uint `gorm:"column:user_id" json:"userId"`
	SeasonID uint `gorm:"column:season_id" json:"seasonId"`
	// The player's Season Points this season. Rises with wins and falls with
	// losses (sp_formula.go), never below 0 (the engine floors every total and
	// the column keeps CHECK (sp >= 0)). No decay (PRD: "No decay") and no spend.
	SP int `gorm:"column:sp" json:"sp"`
	// THE RANK SNAPSHOT: the tier and division SP stood at after the row's last
	// award, written together by every award (Story 13.5). RankDivision is 1-3,
	// or nil for Master, Grandmaster and any row written before migration
	// 000028 that no recalculation has rewritten since (000029 re-scores 2026
	// Q3, divisions included).
	//
	// WHO READS IT depends on the season (see 000028): a RUNNING season derives
	// its rank from SP with the live ladder and ignores these two fields; an
	// ENDED season reads them as stored, because its SP was scored under
	// floors and a formula that are no longer live.
	RankTier     string `gorm:"column:rank_tier" json:"rankTier"`
	RankDivision *int   `gorm:"column:rank_division" json:"rankDivision"`
	// +1 for every human seat in a finished match, present or not.
	GamesPlayed int `gorm:"column:games_played" json:"gamesPlayed"`
	// +1 only for seats present at the terminal end (Story 13.1 D10). A
	// PRESENCE counter: since Story 13.4 every seat is scored by its team's
	// result whether present or not, so this no longer means "matches that
	// earned SP".
	GamesCompleted int       `gorm:"column:games_completed" json:"gamesCompleted"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// SPChanges computes one finished match's Season Points changes from every
// seated human's CURRENT season total, as read (and locked) inside the award
// transaction. A player with no row yet is passed as 0. It returns the formula's
// change per player, before the 0 floor, which the repository applies.
//
// It is how the formula (sp_formula.go) runs inside the repository's
// transaction while the repository stays persistence-only: the service supplies
// the closure, the repository supplies the locked totals.
type SPChanges func(current map[uint]int) (map[uint]int, error)

// PlayerSeasonSnapshot is one player's season state immediately after the
// match-end write, as returned by ApplySeasonPoints.
//
// PreviousSP is the total BEFORE this match, as read under the row lock, so the
// applied change is SP - PreviousSP and the caller can compare the rank before
// and after without a second read. Tier and Division are the rank of the new
// total (RankForSP), exactly as written to rank_tier / rank_division; Division
// is 0 for Master and Grandmaster, which the column stores as NULL.
type PlayerSeasonSnapshot struct {
	SP             int
	PreviousSP     int
	Tier           string
	Division       int
	GamesPlayed    int
	GamesCompleted int
}

// LeaderboardEntry is ONE ROW OF THE JOINED READ behind Story 13.2's
// leaderboard: a player_seasons row plus the username it belongs to.
//
// It is a SCAN TARGET, not a wire type and not a model. Three separate reasons
// keep it its own struct:
//
//  1. It is not PlayerSeason. PlayerSeason mirrors the table; this carries a
//     column from `users` that no table holds together with SP, so scanning the
//     join into PlayerSeason would mean adding a phantom Username field to the
//     model and hoping nobody ever writes it back.
//  2. It is not the DTO either. The handler's LeaderboardRowView adds `position`
//     and the resolved rank (derived from SP for a running season, the stored
//     snapshot below for an ended one); keeping them apart is what stops the
//     join's shape from leaking onto the wire (or the wire's `position` from
//     looking like something the database supplied).
//  3. THERE IS NO `season` -> `user` GO IMPORT. Story 13.3 will likely have
//     `user` import `season` (seasonal rank on the public profile), so the
//     reverse edge must stay closed. The join is written by TABLE NAME and lands
//     here instead of in a user.User (Story 13.2 D2).
//
// The explicit `column:sp` tag matches PlayerSeason's: the default naming
// strategy is not relied on for an all-caps field name.
//
// RankTier and RankDivision are the row's stored rank snapshot. The service
// reads them only for an ENDED season (see PlayerSeason.RankTier).
type LeaderboardEntry struct {
	UserID       uint   `gorm:"column:user_id"`
	Username     string `gorm:"column:username"`
	SP           int    `gorm:"column:sp"`
	RankTier     string `gorm:"column:rank_tier"`
	RankDivision *int   `gorm:"column:rank_division"`
	GamesPlayed  int    `gorm:"column:games_played"`
}

// ArchiveEntry is ONE ROW OF THE JOINED READ behind Story 13.3's prior-season
// archive: a player_seasons row plus the window it was earned in, read back
// UNCHANGED (the rows are immutable once their season ends — see PlayerSeason).
//
// A SCAN TARGET, like LeaderboardEntry above, and its own struct for the same
// reasons: it mixes columns from two tables, and the handler's ArchiveRowView
// is built from it field by field, so the join's shape never reaches the wire
// directly.
//
// RankTier and RankDivision are the stored snapshot, and for the archive they
// ARE the rank: every archived season has ended, so its rank is the one it
// finished on, never re-derived from SP on today's floors (Story 13.5).
//
// MEMBERSHIP IS NOT leaderboardScope's. The archive lists "seasons you actually
// played" (games_played >= 1) that have ENDED, where the ladder lists everyone who
// played the season in view, running or not. The two agree on games_played but
// not on the window, so they stay two predicates in two documented homes; never
// share the scope helper.
type ArchiveEntry struct {
	SeasonID     uint      `gorm:"column:season_id"`
	SeasonName   string    `gorm:"column:season_name"`
	StartedAt    time.Time `gorm:"column:started_at"`
	EndsAt       time.Time `gorm:"column:ends_at"`
	SP           int       `gorm:"column:sp"`
	RankTier     string    `gorm:"column:rank_tier"`
	RankDivision *int      `gorm:"column:rank_division"`
	GamesPlayed  int       `gorm:"column:games_played"`
}
