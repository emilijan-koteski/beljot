package season_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/season"
	"github.com/emilijan/beljot/server/internal/user"
)

// recalcDB is the per-test transaction with every job the database already
// queued (000029's real 2026 Q3 job) marked done, so a run sees only the jobs
// the test queues. The rollback restores them.
func recalcDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := getTestDB(t)
	require.NoError(t, db.Exec(`UPDATE season_recalculations SET completed_at = NOW() WHERE completed_at IS NULL`).Error)
	return db
}

func queueRecalculation(t *testing.T, db *gorm.DB, start time.Time) {
	t.Helper()
	require.NoError(t, db.Exec(`INSERT INTO season_recalculations (season_started_at) VALUES (?)`, start).Error)
}

// jobCompletedAt is the job's completed_at, nil while it is pending.
func jobCompletedAt(t *testing.T, db *gorm.DB, start time.Time) *time.Time {
	t.Helper()
	var rows []struct{ CompletedAt *time.Time }
	require.NoError(t, db.Raw(`SELECT completed_at FROM season_recalculations WHERE season_started_at = ?`, start).
		Scan(&rows).Error)
	require.Len(t, rows, 1)
	return rows[0].CompletedAt
}

// storedRow is the part of a player_seasons row a recalculation writes.
type storedRow struct {
	SP             int
	RankTier       string
	RankDivision   *int
	GamesPlayed    int
	GamesCompleted int
}

func readRow(t *testing.T, db *gorm.DB, userID, seasonID uint) *storedRow {
	t.Helper()
	var rows []storedRow
	require.NoError(t, db.Raw(`
		SELECT sp, rank_tier, rank_division, games_played, games_completed
		FROM player_seasons WHERE user_id = ? AND season_id = ?`, userID, seasonID).Scan(&rows).Error)
	if len(rows) == 0 {
		return nil
	}
	return &rows[0]
}

func seasonRows(t *testing.T, db *gorm.DB, seasonID uint) map[uint]storedRow {
	t.Helper()
	var rows []struct {
		UserID         uint
		SP             int
		RankTier       string
		RankDivision   *int
		GamesPlayed    int
		GamesCompleted int
	}
	require.NoError(t, db.Raw(`
		SELECT user_id, sp, rank_tier, rank_division, games_played, games_completed
		FROM player_seasons WHERE season_id = ?`, seasonID).Scan(&rows).Error)
	out := make(map[uint]storedRow, len(rows))
	for _, r := range rows {
		out[r.UserID] = storedRow{
			SP: r.SP, RankTier: r.RankTier, RankDivision: r.RankDivision,
			GamesPlayed: r.GamesPlayed, GamesCompleted: r.GamesCompleted,
		}
	}
	return out
}

// oldFormulaRow writes a row the way the retired climb-only formula left it.
func oldFormulaRow(t *testing.T, db *gorm.DB, userID, seasonID uint, sp int, tier string, games int) {
	t.Helper()
	require.NoError(t, db.Exec(`
		INSERT INTO player_seasons (user_id, season_id, sp, rank_tier, games_played, games_completed, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())`, userID, seasonID, sp, tier, games, games).Error)
}

type recalcFixture struct {
	season                  *season.Season
	later                   *season.Season
	alice, bob, carol, dave *user.User
	idle                    *user.User
}

// seedRecalcSeason builds a season window no other test uses, queues its job,
// and stores a small window worked by hand with the live constants (W=30,
// L=20, S=385, bot seat 600, Capot +5):
//
//  1. alice + bot vs two bots, alice wins 1100:700 from 0 SP. Own avg 300 vs
//     600, E = 0.1426, m = 0.8996: +46.
//  2. alice, bob, carol, dave (team A = alice, carol); dave abandons at
//     300:500. A avg 23 vs B avg 0, E_A = 0.5343; scored as B surrendering,
//     m = 0.5 + (1001 - 500)/1001 = 1.0005: alice and carol +28, dave -120
//     and bob half of -18.64 = -9, both floored at 0.
//  3. A boot-reconcile placeholder with all four: skipped.
//  4. 501, alice + bot vs two bots, an instant win (0:0) with a Capot by
//     alice's team. Own avg 337 vs 600, E = 0.1718, m = 1.5: +75 +5 = +80.
//
// Before the run, alice holds an old-formula row (30841 SP, Grandmaster, 99
// games), bob, carol and dave have no row (they played before SP tracking),
// idle has a row but no match in the window, and alice also has a row in the
// following season.
func seedRecalcSeason(t *testing.T, db *gorm.DB) recalcFixture {
	t.Helper()
	start := time.Date(2093, time.July, 1, 0, 0, 0, 0, time.UTC)
	fx := recalcFixture{
		season: makeSeason(t, db, start),
		later:  makeSeason(t, db, start.AddDate(0, 3, 0)),
		alice:  makeUser(t, db, "rc-alice@s.test"),
		bob:    makeUser(t, db, "rc-bob@s.test"),
		carol:  makeUser(t, db, "rc-carol@s.test"),
		dave:   makeUser(t, db, "rc-dave@s.test"),
		idle:   makeUser(t, db, "rc-idle@s.test"),
	}
	queueRecalculation(t, db, start)

	oldFormulaRow(t, db, fx.alice.ID, fx.season.ID, 30841, season.TierGrandmaster, 99)
	oldFormulaRow(t, db, fx.idle.ID, fx.season.ID, 420, season.TierSilver, 3)
	oldFormulaRow(t, db, fx.alice.ID, fx.later.ID, 777, season.TierGold, 5)

	roomID := replayRoom(t, db, fx.alice.ID)
	at := func(d time.Duration) time.Time { return start.Add(d) }
	four := [4]uint{fx.alice.ID, fx.bob.ID, fx.carol.ID, fx.dave.ID}

	m1 := replayBotTable(0, fx.alice.ID, 0, 1100, 700)
	m1.CompletedAt = at(time.Hour)

	m2 := replayFourHumans(0, four, 1, 300, 500)
	m2.Status, m2.AbandonedBy, m2.CompletedAt = "abandoned", uintPtr(fx.dave.ID), at(2*time.Hour)

	m3 := replayFourHumans(0, four, 0, 100, 100)
	m3.Status, m3.CompletedAt = "abandoned", at(3*time.Hour)

	m4 := replayBotTable(0, fx.alice.ID, 0, 0, 0)
	m4.MatchMode, m4.CompletedAt = "501", at(4*time.Hour)
	m4.Hands = []match.HandResult{{HandNumber: 1, Capot: true, CapotTeam: intPtr(0)}}

	// Stored out of completion order, so only completed_at can put them right.
	for _, m := range []match.Match{m4, m2, m1, m3} {
		storeMatch(t, db, roomID, m)
	}
	// Outside the window on both sides: never replayed.
	before := replayBotTable(0, fx.alice.ID, 0, 1100, 700)
	before.CompletedAt = start.Add(-time.Second)
	after := replayBotTable(0, fx.alice.ID, 0, 1100, 700)
	after.CompletedAt = fx.season.EndsAt
	storeMatch(t, db, roomID, before)
	storeMatch(t, db, roomID, after)
	return fx
}

func TestRunPendingRecalculations_RewritesTheSeasonFromItsMatches(t *testing.T) {
	db := recalcDB(t)
	fx := seedRecalcSeason(t, db)
	now := time.Date(2093, time.September, 26, 12, 0, 0, 0, time.UTC)

	results, err := season.RunPendingRecalculations(db, now)
	require.NoError(t, err)
	require.Len(t, results, 1)

	res := results[0]
	assert.True(t, res.SeasonFound)
	assert.Equal(t, fx.season.ID, res.SeasonID)
	assert.Equal(t, "2093 Q3", res.SeasonName)
	assert.True(t, res.SeasonStartedAt.Equal(fx.season.StartedAt))
	assert.Equal(t, 4, res.Rewritten)
	assert.Equal(t, season.ReplaySummary{
		Loaded: 4, Scored: 3,
		Skipped: map[string]int{season.ErrReconcilePlaceholder.Error(): 1},
	}, res.Summary)
	assert.Equal(t, []uint{fx.idle.ID}, res.Untouched)

	assert.Equal(t, map[uint]storedRow{
		// 46 + 28 + 80: the old-formula row is overwritten, games recounted.
		fx.alice.ID: {SP: 154, RankTier: season.TierBronze, RankDivision: intPtr(1), GamesPlayed: 3, GamesCompleted: 3},
		// Pre-tracking players get a row.
		fx.bob.ID:   {SP: 0, RankTier: season.TierIron, RankDivision: intPtr(1), GamesPlayed: 1, GamesCompleted: 1},
		fx.carol.ID: {SP: 28, RankTier: season.TierIron, RankDivision: intPtr(1), GamesPlayed: 1, GamesCompleted: 1},
		// The abandoner: -120 applied (floored at 0), and not completed.
		fx.dave.ID: {SP: 0, RankTier: season.TierIron, RankDivision: intPtr(1), GamesPlayed: 1, GamesCompleted: 0},
		// Not in the replay: left exactly as it was.
		fx.idle.ID: {SP: 420, RankTier: season.TierSilver, GamesPlayed: 3, GamesCompleted: 3},
	}, seasonRows(t, db, fx.season.ID))

	assert.Equal(t, &storedRow{SP: 777, RankTier: season.TierGold, GamesPlayed: 5, GamesCompleted: 5},
		readRow(t, db, fx.alice.ID, fx.later.ID), "another season is never touched")

	done := jobCompletedAt(t, db, fx.season.StartedAt)
	require.NotNil(t, done, "the job reads done")
	assert.True(t, done.Equal(now))

	// The rows are exactly the replay of the window.
	replay, err := season.ReplayWindow(db, fx.season.StartedAt, fx.season.EndsAt, season.DefaultSPFormula(), season.DefaultLadder())
	require.NoError(t, err)
	rows := seasonRows(t, db, fx.season.ID)
	for _, p := range replay.Players {
		tier, division := season.RankForSP(p.SP)
		want := storedRow{SP: p.SP, RankTier: tier, GamesPlayed: p.GamesPlayed, GamesCompleted: p.GamesCompleted}
		if division != 0 {
			want.RankDivision = intPtr(division)
		}
		assert.Equal(t, want, rows[p.UserID], "user %d", p.UserID)
	}
}

// A done job never runs again, and a forced re-run (the job reopened by hand)
// writes the same rows: the replay reads the matches, never the rows.
func TestRunPendingRecalculations_RunsOnceAndIsIdempotent(t *testing.T) {
	db := recalcDB(t)
	fx := seedRecalcSeason(t, db)
	now := time.Date(2093, time.September, 26, 12, 0, 0, 0, time.UTC)

	_, err := season.RunPendingRecalculations(db, now)
	require.NoError(t, err)
	first := seasonRows(t, db, fx.season.ID)

	// Scribble on a row: a second run must not repair it, because it does not run.
	require.NoError(t, db.Exec(`UPDATE player_seasons SET sp = 1 WHERE user_id = ? AND season_id = ?`,
		fx.carol.ID, fx.season.ID).Error)
	again, err := season.RunPendingRecalculations(db, now.Add(time.Hour))
	require.NoError(t, err)
	assert.Empty(t, again, "a done job does not run again")
	assert.Equal(t, 1, readRow(t, db, fx.carol.ID, fx.season.ID).SP)
	assert.True(t, jobCompletedAt(t, db, fx.season.StartedAt).Equal(now), "and keeps its first stamp")

	require.NoError(t, db.Exec(`UPDATE season_recalculations SET completed_at = NULL WHERE season_started_at = ?`,
		fx.season.StartedAt).Error)
	forced, err := season.RunPendingRecalculations(db, now.Add(2*time.Hour))
	require.NoError(t, err)
	require.Len(t, forced, 1)
	assert.Equal(t, first, seasonRows(t, db, fx.season.ID), "a forced re-run yields identical rows")
}

func TestRunPendingRecalculations_NoSeasonRowMarksTheJobDone(t *testing.T) {
	db := recalcDB(t)
	start := time.Date(2094, time.July, 1, 0, 0, 0, 0, time.UTC)
	queueRecalculation(t, db, start)
	var before int64
	require.NoError(t, db.Table("player_seasons").Count(&before).Error)

	now := time.Date(2094, time.August, 1, 0, 0, 0, 0, time.UTC)
	results, err := season.RunPendingRecalculations(db, now)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.False(t, results[0].SeasonFound)
	assert.Zero(t, results[0].Rewritten)

	var after int64
	require.NoError(t, db.Table("player_seasons").Count(&after).Error)
	assert.Equal(t, before, after, "nothing is written")
	done := jobCompletedAt(t, db, start)
	require.NotNil(t, done)
	assert.True(t, done.Equal(now))
}

// The job transaction bounds its lock waits and statements, so a lock another
// session holds fails the job (pending, retried next start) instead of hanging
// boot. SET LOCAL outlives the job's released savepoint until the test
// transaction ends, which is what lets the test read the values back.
func TestRunPendingRecalculations_BoundsItsWaits(t *testing.T) {
	db := recalcDB(t)
	setting := func(name string) string {
		var v string
		require.NoError(t, db.Raw(`SELECT current_setting(?)`, name).Scan(&v).Error)
		return v
	}
	require.NotEqual(t, "10s", setting("lock_timeout"))

	queueRecalculation(t, db, time.Date(2094, time.July, 1, 0, 0, 0, 0, time.UTC))
	_, err := season.RunPendingRecalculations(db, time.Date(2094, time.August, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	assert.Equal(t, "10s", setting("lock_timeout"))
	assert.Equal(t, "2min", setting("statement_timeout"))
}

// A failing job rolls back whole (no row half-written, the job still pending,
// so the next start retries it) and does not stop the jobs after it.
func TestRunPendingRecalculations_AFailedJobStaysPending(t *testing.T) {
	db := recalcDB(t)
	fx := seedRecalcSeason(t, db)
	otherStart := time.Date(2094, time.July, 1, 0, 0, 0, 0, time.UTC)
	queueRecalculation(t, db, otherStart)

	// Make carol's recalculated total (28) unwritable, so the job fails on its
	// third row, after alice's and bob's. NOT VALID so the rows already in the
	// table are not checked; the test drops it again before the retry.
	require.NoError(t, db.Exec(`ALTER TABLE player_seasons ADD CONSTRAINT recalc_test_refuse CHECK (sp <> 28) NOT VALID`).Error)

	results, err := season.RunPendingRecalculations(db, time.Date(2094, time.August, 1, 0, 0, 0, 0, time.UTC))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "2093-07-01T00:00:00Z")
	require.Len(t, results, 1, "the next job still ran")
	assert.True(t, results[0].SeasonStartedAt.Equal(otherStart))

	assert.Nil(t, jobCompletedAt(t, db, fx.season.StartedAt), "the failed job stays pending")
	assert.Nil(t, readRow(t, db, fx.bob.ID, fx.season.ID), "rows written before the failure are rolled back")
	assert.Equal(t, 30841, readRow(t, db, fx.alice.ID, fx.season.ID).SP)

	require.NoError(t, db.Exec(`ALTER TABLE player_seasons DROP CONSTRAINT recalc_test_refuse`).Error)
	retried, err := season.RunPendingRecalculations(db, time.Date(2094, time.August, 2, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Len(t, retried, 1, "the next start retries it")
	assert.Equal(t, 154, readRow(t, db, fx.alice.ID, fx.season.ID).SP)
}

// Migration 000029 queues exactly one job, for the season starting 2026-07-01,
// and its down drops the queue. Run against the real SQL files inside the test
// transaction, so the database's own table is restored on rollback.
func TestRecalculationMigration_QueuesOnly2026Q3(t *testing.T) {
	db := getTestDB(t)
	up, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000029_queue_2026_q3_recalculation.up.sql"))
	require.NoError(t, err)
	down, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000029_queue_2026_q3_recalculation.down.sql"))
	require.NoError(t, err)

	tableExists := func() bool {
		var exists bool
		require.NoError(t, db.Raw(`SELECT to_regclass('season_recalculations') IS NOT NULL`).Scan(&exists).Error)
		return exists
	}

	require.NoError(t, db.Exec(string(down)).Error)
	assert.False(t, tableExists(), "down drops the queue")

	require.NoError(t, db.Exec(string(up)).Error)
	require.True(t, tableExists())
	var jobs []struct {
		SeasonStartedAt time.Time
		RequestedAt     *time.Time
		CompletedAt     *time.Time
	}
	require.NoError(t, db.Raw(`SELECT season_started_at, requested_at, completed_at FROM season_recalculations`).
		Scan(&jobs).Error)
	require.Len(t, jobs, 1)
	assert.True(t, jobs[0].SeasonStartedAt.Equal(time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)))
	assert.NotNil(t, jobs[0].RequestedAt)
	assert.Nil(t, jobs[0].CompletedAt, "queued pending")
}
