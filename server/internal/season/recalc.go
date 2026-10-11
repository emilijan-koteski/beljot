package season

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// RecalculationResult reports one job that ran to completion.
type RecalculationResult struct {
	SeasonStartedAt time.Time
	SeasonFound     bool
	SeasonID        uint
	SeasonName      string
	Rewritten       int
	Summary         ReplaySummary
	Untouched       []uint
}

// RunPendingRecalculations runs every pending job, oldest season first, each in its own transaction.
// A failing job stays pending and its error is joined into the result, without stopping the others.
func RunPendingRecalculations(db *gorm.DB, now time.Time) ([]RecalculationResult, error) {
	var pending []recalculationJob
	if err := db.Raw(`
		SELECT season_started_at FROM season_recalculations
		WHERE completed_at IS NULL
		ORDER BY season_started_at`,
	).Scan(&pending).Error; err != nil {
		return nil, fmt.Errorf("season recalculation: listing pending jobs: %w", err)
	}

	var (
		results []RecalculationResult
		errs    []error
	)
	for _, job := range pending {
		start := job.SeasonStartedAt
		res, ran, err := runRecalculation(db, start, now)
		if err != nil {
			errs = append(errs, fmt.Errorf("season recalculation %s: %w", start.UTC().Format(time.RFC3339), err))
			continue
		}
		if ran {
			results = append(results, res)
		}
	}
	return results, errors.Join(errs...)
}

const (
	recalcLockTimeout      = "10s"
	recalcStatementTimeout = "120s"
)

// recalculationJob is the scan target for a season_recalculations key.
type recalculationJob struct {
	SeasonStartedAt time.Time `gorm:"column:season_started_at"`
}

// runRecalculation replays one season from its stored matches and rewrites its rows in ascending user-ID order,
// marking the job done in the same transaction. Lock and statement waits are bounded so a held lock fails the job instead of hanging boot.
func runRecalculation(db *gorm.DB, start, now time.Time) (RecalculationResult, bool, error) {
	res := RecalculationResult{SeasonStartedAt: start}
	ran := false

	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL lock_timeout = '" + recalcLockTimeout + "'").Error; err != nil {
			return fmt.Errorf("setting the lock timeout: %w", err)
		}
		if err := tx.Exec("SET LOCAL statement_timeout = '" + recalcStatementTimeout + "'").Error; err != nil {
			return fmt.Errorf("setting the statement timeout: %w", err)
		}

		var job []recalculationJob
		if err := tx.Raw(`
			SELECT season_started_at FROM season_recalculations
			WHERE season_started_at = ? AND completed_at IS NULL
			FOR UPDATE`,
			start,
		).Scan(&job).Error; err != nil {
			return fmt.Errorf("locking the job: %w", err)
		}
		if len(job) == 0 {
			return nil
		}
		ran = true

		var seasons []Season
		if err := tx.Where("started_at = ?", start).Limit(1).Find(&seasons).Error; err != nil {
			return fmt.Errorf("finding the season: %w", err)
		}
		if len(seasons) == 1 {
			s := seasons[0]
			res.SeasonFound, res.SeasonID, res.SeasonName = true, s.ID, s.Name
			if err := rewriteSeason(tx, s, &res); err != nil {
				return err
			}
		}

		if err := tx.Exec(
			`UPDATE season_recalculations SET completed_at = ? WHERE season_started_at = ?`,
			now.UTC(), start,
		).Error; err != nil {
			return fmt.Errorf("marking the job done: %w", err)
		}
		return nil
	})
	if err != nil {
		return RecalculationResult{}, false, err
	}
	return res, ran, nil
}

// rewriteSeason replays one season and writes the result (steps 3 and 4 of
// runRecalculation), filling res.
func rewriteSeason(tx *gorm.DB, s Season, res *RecalculationResult) error {
	replay, err := ReplayWindow(tx, s.StartedAt, s.EndsAt, DefaultSPFormula(), DefaultLadder())
	if err != nil {
		return fmt.Errorf("replaying season %s: %w", s.Name, err)
	}
	res.Summary = replay.Summary

	zeroTier, zeroDivision := RankForSP(0)
	written := make([]uint, 0, len(replay.Players))
	for _, p := range replay.Players {
		if err := tx.Exec(`
			INSERT INTO player_seasons
				(user_id, season_id, sp, rank_tier, rank_division, games_played, games_completed, created_at, updated_at)
			VALUES (?, ?, 0, ?, ?, 0, 0, NOW(), NOW())
			ON CONFLICT (user_id, season_id) DO NOTHING`,
			p.UserID, s.ID, zeroTier, divisionOrNil(zeroDivision),
		).Error; err != nil {
			return fmt.Errorf("ensuring player_season user=%d season=%d: %w", p.UserID, s.ID, err)
		}

		var locked []uint
		if err := tx.Raw(
			`SELECT id FROM player_seasons WHERE user_id = ? AND season_id = ? FOR UPDATE`,
			p.UserID, s.ID,
		).Scan(&locked).Error; err != nil {
			return fmt.Errorf("locking player_season user=%d season=%d: %w", p.UserID, s.ID, err)
		}
		if len(locked) != 1 {
			return fmt.Errorf("locking player_season user=%d season=%d: found %d rows", p.UserID, s.ID, len(locked))
		}

		if err := tx.Exec(`
			UPDATE player_seasons
			SET sp              = ?,
				rank_tier       = ?,
				rank_division   = ?,
				games_played    = ?,
				games_completed = ?,
				updated_at      = NOW()
			WHERE user_id = ? AND season_id = ?`,
			p.SP, p.Tier, divisionOrNil(p.Division), p.GamesPlayed, p.GamesCompleted, p.UserID, s.ID,
		).Error; err != nil {
			return fmt.Errorf("writing player_season user=%d season=%d: %w", p.UserID, s.ID, err)
		}
		written = append(written, p.UserID)
	}
	res.Rewritten = len(written)

	untouched := tx.Table("player_seasons").Where("season_id = ?", s.ID)
	if len(written) > 0 {
		untouched = untouched.Where("user_id NOT IN ?", written)
	}
	if err := untouched.Order("user_id ASC").Pluck("user_id", &res.Untouched).Error; err != nil {
		return fmt.Errorf("listing rows the replay did not cover: %w", err)
	}
	return nil
}
