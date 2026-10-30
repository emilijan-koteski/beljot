package season

import "time"

// Repository is the persistence boundary for seasons and per-player season
// records. The service and the handler depend on this interface, never on GORM
// directly.
type Repository interface {
	CurrentSeason(now time.Time) (*Season, error)

	ApplySeasonPoints(seasonID uint, seats map[uint]SeatResult, changes SPChanges) (map[uint]PlayerSeasonSnapshot, error)

	FindPlayerSeason(userID, seasonID uint) (*PlayerSeason, error)

	LeaderboardPage(seasonID uint, limit, offset int) ([]LeaderboardEntry, int64, error)

	FindLeaderboardEntry(seasonID, userID uint) (*LeaderboardEntry, error)

	CountAhead(seasonID uint, sp int, userID uint) (int64, error)

	FindSeasonByID(id uint) (*Season, error)

	ListSeasons() ([]Season, error)

	PlayerSeasonArchive(userID uint, now time.Time) ([]ArchiveEntry, error)
}
