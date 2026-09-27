package season_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/emilijan/beljot/server/internal/season"
)

// seasonAt returns the season row starting at `start`, creating it inside the
// test transaction if the database does not have it yet (a migrated database
// already holds the quarter it was migrated in).
func seasonAt(t *testing.T, db *gorm.DB, start time.Time) *season.Season {
	t.Helper()
	require.NoError(t, db.Exec(`
		INSERT INTO seasons (name, started_at, ends_at, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
		ON CONFLICT (started_at) DO NOTHING`,
		season.QuarterName(start), start, start.AddDate(0, 3, 0)).Error)
	var s season.Season
	require.NoError(t, db.Where("started_at = ?", start).First(&s).Error)
	return &s
}

// Migration 000027 clears 2026 Q4's player_seasons rows (the old formula's
// points, written between the quarter boundary and the release) and leaves 2026
// Q3 and every other season untouched. Run against the real SQL file, inside the
// per-test transaction, so the database's own Q4 rows are never really deleted.
func TestResetMigration_ClearsOnly2026Q4(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)

	up, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000027_reset_2026_q4_player_seasons.up.sql"))
	require.NoError(t, err)
	down, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000027_reset_2026_q4_player_seasons.down.sql"))
	require.NoError(t, err)

	q3 := seasonAt(t, db, time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC))
	q4 := seasonAt(t, db, time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC))
	later := makeSeason(t, db, time.Date(2084, time.January, 1, 0, 0, 0, 0, time.UTC))

	u := makeUser(t, db, "q4reset@s.test")
	for _, s := range []*season.Season{q3, q4, later} {
		_, err := applyAwards(repo, s.ID, map[uint]fixedAward{u.ID: {change: 180, completed: true}})
		require.NoError(t, err)
	}

	require.NoError(t, db.Exec(string(up)).Error)

	gone, err := repo.FindPlayerSeason(u.ID, q4.ID)
	require.NoError(t, err)
	assert.Nil(t, gone, "the 2026 Q4 row is cleared")

	for name, s := range map[string]*season.Season{"2026 Q3": q3, "a later season": later} {
		row, err := repo.FindPlayerSeason(u.ID, s.ID)
		require.NoError(t, err)
		require.NotNil(t, row, "%s is untouched", name)
		assert.Equal(t, 180, row.SP, name)
	}

	var q4Seasons int64
	require.NoError(t, db.Model(&season.Season{}).Where("id = ?", q4.ID).Count(&q4Seasons).Error)
	assert.Equal(t, int64(1), q4Seasons, "the season row itself is kept")

	// The down migration is a documented no-op: it runs and changes nothing.
	require.NoError(t, db.Exec(string(down)).Error)
	row, err := repo.FindPlayerSeason(u.ID, q3.ID)
	require.NoError(t, err)
	require.NotNil(t, row)
}
