package season

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// One-time SEASON RECALCULATIONS (migration 000029).
//
// A recalculation re-scores a whole season from its stored matches with the
// LIVE formula and ladder, and writes each player's result over their
// player_seasons row. It exists for 2026 Q3, whose standings were scored by the
// retired climb-only formula: 000029 queues one job for the season starting
// 2026-07-01, and the server runs pending jobs once at startup, after the
// migrations and before it accepts connections (cmd/api/main.go). Later Q3
// matches then score live from the recalculated totals.
//
// A job is a season_recalculations row keyed by the season's started_at. It is
// PENDING while completed_at is NULL and DONE once it is set; a done job never
// runs again. A failed job rolls back whole and stays pending, so the next
// start retries it.

// RecalculationResult reports one job that ran to completion.
type RecalculationResult struct {
	SeasonStartedAt time.Time
	// SeasonFound is false when no season row starts at SeasonStartedAt (a
	// fresh database that never had that quarter): the job is marked done and
	// nothing is written. SeasonID and SeasonName are then zero.
	SeasonFound bool
	SeasonID    uint
	SeasonName  string
	// Rewritten is how many player_seasons rows were written (inserted or
	// overwritten): one per player of the replay.
	Rewritten int
	Summary   ReplaySummary
	// Untouched lists, ascending, the users whose row in the season the replay
	// did not cover (they sat in no scored match of its window). Their rows are
	// left as they are; the caller logs them.
	Untouched []uint
}

// RunPendingRecalculations runs every pending job, oldest season first, each in
// its own transaction, and stamps completed_at = now on the ones it finishes.
//
// It returns the jobs that ran. A failing job does not stop the others: its
// error is joined into the returned error and it stays pending. A job another
// process finished while this one waited on its lock is skipped, not re-run.
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

// The recalculation transaction's bounds on waiting for a row lock and on any
// one statement (Postgres interval syntax, SET LOCAL so they end with it).
const (
	recalcLockTimeout      = "10s"
	recalcStatementTimeout = "120s"
)

// recalculationJob is the scan target for a season_recalculations key.
type recalculationJob struct {
	SeasonStartedAt time.Time `gorm:"column:season_started_at"`
}

// runRecalculation runs one job in ONE transaction:
//
//  1. LOCK the job row, re-checking that it is still pending (a second process
//     that waited on the lock finds it done and returns ran = false).
//  2. FIND the season by started_at. None: mark the job done, write nothing.
//  3. REPLAY the season's window [started_at, ends_at) with the live formula
//     and ladder (ReplayWindow). It reads the matches, never the rows it is
//     about to overwrite, so an old-formula award that landed before this run
//     is replaced like every other total.
//  4. WRITE each replayed player's row in ASCENDING user-ID order, the
//     ApplySeasonPoints discipline: INSERT a zero row ON CONFLICT DO NOTHING
//     (a player who never had a row gets one), lock it with SELECT ... FOR
//     UPDATE, then set sp, the rank snapshot and both counters outright.
//  5. MARK the job done in the same transaction, so the rows and the stamp
//     commit together or not at all.
//
// It runs at startup before the server accepts connections, so it must never
// wait forever: the transaction first sets a lock timeout and a statement
// timeout, and a lock another session holds past them fails the job (logged,
// still pending, retried on the next start) instead of hanging boot.
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
	// replay.Players is in ascending user-ID order: the lock order.
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
