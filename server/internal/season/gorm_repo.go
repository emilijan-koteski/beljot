package season

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"gorm.io/gorm"
)

type GormRepository struct {
	db *gorm.DB
}

func NewGormRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

var _ Repository = (*GormRepository)(nil)

// CurrentSeason returns the season covering now, inserting the calendar quarter on a miss.
// The insert is ON CONFLICT DO NOTHING and followed by a re-read, so racing callers get the same row.
func (r *GormRepository) CurrentSeason(now time.Time) (*Season, error) {
	existing, err := r.findCovering(now)
	if err != nil {
		return nil, fmt.Errorf("reading current season: %w", err)
	}
	if existing != nil {
		return existing, nil
	}

	start, end := QuarterBounds(now)
	if err := r.db.Exec(`
		INSERT INTO seasons (name, started_at, ends_at, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
		ON CONFLICT (started_at) DO NOTHING`,
		QuarterName(start), start, end,
	).Error; err != nil {
		return nil, fmt.Errorf("creating season %s: %w", QuarterName(start), err)
	}

	created, err := r.findCovering(now)
	if err != nil {
		return nil, fmt.Errorf("re-reading current season: %w", err)
	}
	if created == nil {
		return nil, fmt.Errorf("season: no window covers %s after upsert", now.UTC().Format(time.RFC3339))
	}
	return created, nil
}

// findCovering returns the row whose [started_at, ends_at) window contains now, or (nil, nil).
// Overlapping rows resolve to the most recently started one.
func (r *GormRepository) findCovering(now time.Time) (*Season, error) {
	var s Season
	err := r.db.
		Where("started_at <= ? AND ends_at > ?", now.UTC(), now.UTC()).
		Order("started_at DESC").
		First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func (r *GormRepository) FindPlayerSeason(userID, seasonID uint) (*PlayerSeason, error) {
	var ps PlayerSeason
	err := r.db.
		Where("user_id = ? AND season_id = ?", userID, seasonID).
		First(&ps).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &ps, nil
}

// ApplySeasonPoints scores one match in a single transaction: it upserts and locks each player's row
// in ascending user-ID order, computes all changes at once, then writes the floored totals and ranks.
func (r *GormRepository) ApplySeasonPoints(seasonID uint, seats map[uint]SeatResult, changes SPChanges) (map[uint]PlayerSeasonSnapshot, error) {
	snapshots := make(map[uint]PlayerSeasonSnapshot, len(seats))
	if len(seats) == 0 {
		return snapshots, nil
	}

	ids := make([]uint, 0, len(seats))
	for id := range seats {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	err := r.db.Transaction(func(tx *gorm.DB) error {
		current := make(map[uint]int, len(ids))
		for _, id := range ids {
			zeroTier, zeroDivision := RankForSP(0)
			if err := tx.Exec(`
				INSERT INTO player_seasons
					(user_id, season_id, sp, rank_tier, rank_division, games_played, games_completed, created_at, updated_at)
				VALUES (?, ?, 0, ?, ?, 0, 0, NOW(), NOW())
				ON CONFLICT (user_id, season_id) DO NOTHING`,
				id, seasonID, zeroTier, divisionOrNil(zeroDivision),
			).Error; err != nil {
				return fmt.Errorf("ensuring player_season user=%d season=%d: %w", id, seasonID, err)
			}

			var locked []int
			if err := tx.Raw(
				`SELECT sp FROM player_seasons WHERE user_id = ? AND season_id = ? FOR UPDATE`,
				id, seasonID,
			).Scan(&locked).Error; err != nil {
				return fmt.Errorf("locking player_season user=%d season=%d: %w", id, seasonID, err)
			}
			if len(locked) != 1 {
				return fmt.Errorf("locking player_season user=%d season=%d: found %d rows", id, seasonID, len(locked))
			}
			current[id] = locked[0]
		}

		deltas, err := changes(current)
		if err != nil {
			return fmt.Errorf("computing season points: %w", err)
		}

		for _, id := range ids {
			delta, ok := deltas[id]
			if !ok {
				return fmt.Errorf("computing season points: no change for user %d", id)
			}
			next := ApplySPChange(current[id], delta)
			tier, division := RankForSP(next)
			present, won := 0, 0
			if seats[id].Completed {
				present = 1
			}
			if seats[id].Won {
				won = 1
			}

			var row struct {
				GamesPlayed    int
				GamesCompleted int
				GamesWon       int
			}
			if err := tx.Raw(`
				UPDATE player_seasons
				SET sp              = ?,
					rank_tier       = ?,
					rank_division   = ?,
					games_played    = games_played + 1,
					games_completed = games_completed + ?,
					games_won       = games_won + ?,
					updated_at      = NOW()
				WHERE user_id = ? AND season_id = ?
				RETURNING games_played, games_completed, games_won`,
				next, tier, divisionOrNil(division), present, won, id, seasonID,
			).Scan(&row).Error; err != nil {
				return fmt.Errorf("writing player_season user=%d season=%d: %w", id, seasonID, err)
			}

			snapshots[id] = PlayerSeasonSnapshot{
				SP:             next,
				PreviousSP:     current[id],
				Tier:           tier,
				Division:       division,
				GamesPlayed:    row.GamesPlayed,
				GamesCompleted: row.GamesCompleted,
				GamesWon:       row.GamesWon,
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return snapshots, nil
}

// leaderboardScope is the single source of the leaderboard's visibility predicate, shared by
// the page, its total and CountAhead. It hand-writes `users.deleted_at IS NULL` because
// Table()/Joins() skips the soft-delete scope.
func (r *GormRepository) leaderboardScope(seasonID uint) *gorm.DB {
	return r.db.
		Table("player_seasons").
		Joins("JOIN users ON users.id = player_seasons.user_id").
		Where("player_seasons.season_id = ?", seasonID).
		Where("users.deleted_at IS NULL").
		Where("player_seasons.games_played >= 1")
}

const allocHintCap = 1000

// LeaderboardPage returns one page ordered by sp DESC, user_id ASC plus the total row count.
// It rejects out-of-range arguments, since a negative limit would panic make or disable SQL LIMIT.
func (r *GormRepository) LeaderboardPage(seasonID uint, limit, offset int) ([]LeaderboardEntry, int64, error) {
	if limit < 1 {
		return nil, 0, fmt.Errorf("season: leaderboard limit must be >= 1, got %d", limit)
	}
	if offset < 0 {
		return nil, 0, fmt.Errorf("season: leaderboard offset must be >= 0, got %d", offset)
	}

	total, err := r.CountLeaderboard(seasonID)
	if err != nil {
		return nil, 0, err
	}

	entries := make([]LeaderboardEntry, 0, min(limit, allocHintCap))
	err = r.leaderboardScope(seasonID).
		Select(`player_seasons.user_id       AS user_id,
		        users.username               AS username,
		        users.avatar_key             AS avatar_key,
		        player_seasons.sp            AS sp,
		        player_seasons.rank_tier     AS rank_tier,
		        player_seasons.rank_division AS rank_division,
		        player_seasons.games_played  AS games_played,
		        player_seasons.games_won     AS games_won`).
		Order("player_seasons.sp DESC").
		Order("player_seasons.user_id ASC").
		Limit(limit).
		Offset(offset).
		Scan(&entries).Error
	if err != nil {
		return nil, 0, fmt.Errorf("reading leaderboard page season=%d: %w", seasonID, err)
	}
	return entries, total, nil
}

// FindLeaderboardEntry returns the viewer's row through leaderboardScope, so soft-deleted or unplayed
// rows have no standing; it returns (nil, nil) on a miss.
func (r *GormRepository) FindLeaderboardEntry(seasonID, userID uint) (*LeaderboardEntry, error) {
	var entries []LeaderboardEntry
	err := r.leaderboardScope(seasonID).
		Select(`player_seasons.user_id       AS user_id,
		        users.username               AS username,
		        users.avatar_key             AS avatar_key,
		        player_seasons.sp            AS sp,
		        player_seasons.rank_tier     AS rank_tier,
		        player_seasons.rank_division AS rank_division,
		        player_seasons.games_played  AS games_played,
		        player_seasons.games_won     AS games_won`).
		Where("player_seasons.user_id = ?", userID).
		Limit(1).
		Scan(&entries).Error
	if err != nil {
		return nil, fmt.Errorf("reading leaderboard entry season=%d user=%d: %w", seasonID, userID, err)
	}
	if len(entries) == 0 {
		return nil, nil
	}
	return &entries[0], nil
}

// CountLeaderboard returns how many players the season's leaderboard lists.
func (r *GormRepository) CountLeaderboard(seasonID uint) (int64, error) {
	var total int64
	if err := r.leaderboardScope(seasonID).Count(&total).Error; err != nil {
		return 0, fmt.Errorf("counting leaderboard rows season=%d: %w", seasonID, err)
	}
	return total, nil
}

// FindSeasonByID returns the season with the given id, or (nil, nil) on a miss.
func (r *GormRepository) FindSeasonByID(id uint) (*Season, error) {
	var s Season
	if err := r.db.First(&s, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading season %d: %w", id, err)
	}
	return &s, nil
}

// ListSeasons returns every season, newest first.
func (r *GormRepository) ListSeasons() ([]Season, error) {
	seasons := make([]Season, 0, 8)
	if err := r.db.Order("started_at DESC").Find(&seasons).Error; err != nil {
		return nil, fmt.Errorf("listing seasons: %w", err)
	}
	return seasons, nil
}

// PlayerSeasonArchive returns a user's ended seasons with games_played >= 1, newest first.
// A soft-deleted user gets an empty archive rather than an error.
func (r *GormRepository) PlayerSeasonArchive(userID uint, now time.Time) ([]ArchiveEntry, error) {
	entries := make([]ArchiveEntry, 0, 4)
	err := r.db.
		Table("player_seasons").
		Joins("JOIN seasons ON seasons.id = player_seasons.season_id").
		Joins("JOIN users ON users.id = player_seasons.user_id").
		Where("player_seasons.user_id = ?", userID).
		Where("users.deleted_at IS NULL").
		Where("player_seasons.games_played >= 1").
		Where("seasons.ends_at <= ?", now.UTC()).
		Select(`player_seasons.season_id     AS season_id,
		        seasons.name                 AS season_name,
		        seasons.started_at           AS started_at,
		        seasons.ends_at              AS ends_at,
		        player_seasons.sp            AS sp,
		        player_seasons.rank_tier     AS rank_tier,
		        player_seasons.rank_division AS rank_division,
		        player_seasons.games_played  AS games_played,
		        player_seasons.games_won     AS games_won`).
		Order("seasons.started_at DESC").
		Scan(&entries).Error
	if err != nil {
		return nil, fmt.Errorf("reading season archive user=%d: %w", userID, err)
	}
	return entries, nil
}

// CountAhead counts rows strictly ahead of (sp, userID) under LeaderboardPage's ordering.
func (r *GormRepository) CountAhead(seasonID uint, sp int, userID uint) (int64, error) {
	var ahead int64
	err := r.leaderboardScope(seasonID).
		Where("(player_seasons.sp > ? OR (player_seasons.sp = ? AND player_seasons.user_id < ?))",
			sp, sp, userID).
		Count(&ahead).Error
	if err != nil {
		return 0, fmt.Errorf("counting rows ahead season=%d user=%d: %w", seasonID, userID, err)
	}
	return ahead, nil
}
