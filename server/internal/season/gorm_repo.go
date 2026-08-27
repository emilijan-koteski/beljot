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

// Compile-time proof that the concrete repo still satisfies the boundary the
// service and handler depend on.
var _ Repository = (*GormRepository)(nil)

// CurrentSeason implements the lazy self-healing resolver (Story 13.1 D1).
//
// Read first: the overwhelmingly common case is a hit, and the read costs one
// indexed lookup. Only on a miss do we compute the calendar quarter and insert
// it, and that insert is idempotent (ON CONFLICT (started_at) DO NOTHING) so two
// match ends racing at a quarter boundary both proceed with the same row rather
// than one failing on a unique violation.
//
// Then re-read rather than trusting the insert: DO NOTHING means "maybe someone
// else won", and the row we want is whichever one is actually in the table.
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
		// Unreachable unless a concurrent writer inserted a window that does not
		// actually cover `now` under the same started_at. Surfaced rather than
		// returning (nil, nil), which every caller would have to special-case.
		return nil, fmt.Errorf("season: no window covers %s after upsert", now.UTC().Format(time.RFC3339))
	}
	return created, nil
}

// findCovering returns the row whose [started_at, ends_at) window contains now,
// or (nil, nil) on a miss. Ordered started_at DESC so that if overlapping rows
// were ever hand-inserted, the most recently started one wins deterministically
// instead of the result depending on physical row order.
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

// ApplySeasonPoints accumulates every seat's award into its (user, season) row
// inside ONE transaction. Mirrors the AddXP / ApplySettlement discipline
// (user/gorm_repo.go:212, wallet/gorm_repo.go): users are processed in ASCENDING
// ID order so a concurrent wallet settlement or XP award -- which lock in the
// same order -- cannot deadlock against this.
//
// The write is a single upsert per user rather than read-then-write: the row may
// not exist yet, and SELECT ... FOR UPDATE cannot lock a row that is absent, so
// two concurrent first-ever writes for the same player would both insert. The
// ON CONFLICT (user_id, season_id) target -- backed by the unique index in
// 000024 -- makes the loser an atomic increment instead of a duplicate.
//
// The increment is expressed IN SQL (sp = player_seasons.sp + EXCLUDED.sp), not
// as "read total, add in Go, write total": the latter loses one of two
// concurrent awards even inside a transaction unless the row was locked first.
//
// rank_tier is refreshed in a second statement from the returned total, because
// the tier is Go arithmetic (TierForSP) and D7 keeps that the single source --
// restating the ladder as a SQL CASE would be a second copy that could drift.
//
// WHY THE SECOND STATEMENT IS NOT REDUNDANT, and why you cannot delete it: the
// upsert's own `rank_tier` value, TierForSP(award.SP), is correct ONLY on the
// INSERT branch, where the row starts from zero so award.SP IS the new total. The
// DO UPDATE branch deliberately does not touch rank_tier at all -- it cannot,
// because the new total (player_seasons.sp + EXCLUDED.sp) is not known to Go
// until RETURNING hands it back. So on every award after a player's first, the
// column is still the PREVIOUS tier until the follow-up UPDATE lands. Removing
// that UPDATE, or "simplifying" the two statements into one, silently freezes
// every returning player's stored tier at whatever it was after their first
// match of the season.
func (r *GormRepository) ApplySeasonPoints(seasonID uint, awards map[uint]SPAward) (map[uint]PlayerSeasonSnapshot, error) {
	snapshots := make(map[uint]PlayerSeasonSnapshot, len(awards))
	if len(awards) == 0 {
		return snapshots, nil
	}

	ids := make([]uint, 0, len(awards))
	for id := range awards {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	err := r.db.Transaction(func(tx *gorm.DB) error {
		for _, id := range ids {
			award := awards[id]
			completed := 0
			if award.Completed {
				completed = 1
			}

			var row struct {
				SP             int
				GamesPlayed    int
				GamesCompleted int
			}
			if err := tx.Raw(`
				INSERT INTO player_seasons
					(user_id, season_id, sp, rank_tier, games_played, games_completed, created_at, updated_at)
				VALUES (?, ?, ?, ?, 1, ?, NOW(), NOW())
				ON CONFLICT (user_id, season_id) DO UPDATE
				SET sp              = player_seasons.sp + EXCLUDED.sp,
					games_played    = player_seasons.games_played + 1,
					games_completed = player_seasons.games_completed + EXCLUDED.games_completed,
					updated_at      = NOW()
				RETURNING sp, games_played, games_completed`,
				id, seasonID, award.SP, TierForSP(award.SP), completed,
			).Scan(&row).Error; err != nil {
				return fmt.Errorf("upserting player_season user=%d season=%d: %w", id, seasonID, err)
			}

			tier := TierForSP(row.SP)
			if err := tx.Exec(
				`UPDATE player_seasons SET rank_tier = ?, updated_at = NOW() WHERE user_id = ? AND season_id = ?`,
				tier, id, seasonID,
			).Error; err != nil {
				return fmt.Errorf("refreshing rank_tier user=%d season=%d: %w", id, seasonID, err)
			}

			snapshots[id] = PlayerSeasonSnapshot{
				SP:             row.SP,
				PreviousSP:     row.SP - award.SP,
				Tier:           tier,
				GamesPlayed:    row.GamesPlayed,
				GamesCompleted: row.GamesCompleted,
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return snapshots, nil
}
