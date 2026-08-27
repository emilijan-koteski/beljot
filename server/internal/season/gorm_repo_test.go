package season

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/emilijan/beljot/server/internal/user"
)

// --- Integration tests (Postgres; skipped when the DB is unavailable) ---

func getTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("BELJOT_DB_URL")
	if dsn == "" {
		dsn = "postgres://beljot:beljot_dev_password@localhost:5433/beljot?sslmode=disable"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skip("skipping integration test: database not available")
	}

	// Per-test transaction rolled back on cleanup — tests create their own data
	// and never touch seed rows or another test's rows.
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })
	return tx
}

func makeUser(t *testing.T, db *gorm.DB, email string) *user.User {
	t.Helper()
	u := &user.User{
		Email:              email,
		Username:           email[:min(len(email), 12)],
		PasswordHash:       "x",
		LanguagePreference: "en",
	}
	require.NoError(t, db.Create(u).Error)
	return u
}

// makeSeason inserts a window that does NOT collide with the migration's seeded
// quarter, so a test can assert against a season it fully controls. Far-future
// quarters are used for exactly that reason.
func makeSeason(t *testing.T, db *gorm.DB, start time.Time) *Season {
	t.Helper()
	end := start.AddDate(0, 3, 0)
	s := &Season{Name: QuarterName(start), StartedAt: start, EndsAt: end}
	require.NoError(t, db.Create(s).Error)
	return s
}

func TestCurrentSeason_ReadsTheCoveringWindow(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)

	start := time.Date(2099, time.April, 1, 0, 0, 0, 0, time.UTC)
	want := makeSeason(t, db, start)

	got, err := repo.CurrentSeason(start.AddDate(0, 1, 0))
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, want.ID, got.ID)
	assert.Equal(t, "2099 Q2", got.Name)
}

// The boundaries: started_at is INCLUSIVE and ends_at is EXCLUSIVE, so the exact
// end instant belongs to the NEXT quarter — which the lazy resolver then creates.
func TestCurrentSeason_BoundariesAreHalfOpen(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)

	start := time.Date(2099, time.July, 1, 0, 0, 0, 0, time.UTC)
	existing := makeSeason(t, db, start)

	atStart, err := repo.CurrentSeason(start)
	require.NoError(t, err)
	assert.Equal(t, existing.ID, atStart.ID, "started_at is inclusive")

	justBefore, err := repo.CurrentSeason(existing.EndsAt.Add(-time.Second))
	require.NoError(t, err)
	assert.Equal(t, existing.ID, justBefore.ID)

	atEnd, err := repo.CurrentSeason(existing.EndsAt)
	require.NoError(t, err)
	assert.NotEqual(t, existing.ID, atEnd.ID, "ends_at is exclusive — this is the next window")
	assert.Equal(t, "2099 Q4", atEnd.Name)
}

// D1: on a miss the resolver CREATES the calendar quarter rather than returning
// nothing, which is what keeps SP accruing before Story 13.3's scheduler exists.
// It is also idempotent: a second call for the same quarter returns the same row.
func TestCurrentSeason_LazilyCreatesAndIsIdempotent(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)

	now := time.Date(2098, time.November, 20, 13, 45, 0, 0, time.UTC)

	created, err := repo.CurrentSeason(now)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, "2098 Q4", created.Name, "machine-stable YYYY QN token")
	assert.True(t, time.Date(2098, time.October, 1, 0, 0, 0, 0, time.UTC).Equal(created.StartedAt.UTC()))
	assert.True(t, time.Date(2099, time.January, 1, 0, 0, 0, 0, time.UTC).Equal(created.EndsAt.UTC()))

	again, err := repo.CurrentSeason(now.Add(24 * time.Hour))
	require.NoError(t, err)
	assert.Equal(t, created.ID, again.ID, "ON CONFLICT DO NOTHING — no duplicate window")

	var count int64
	require.NoError(t, db.Model(&Season{}).Where("started_at = ?", created.StartedAt).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestFindPlayerSeason_MissIsNilNotAnError(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	u := makeUser(t, db, "sp-miss@s.test")
	s := makeSeason(t, db, time.Date(2097, time.January, 1, 0, 0, 0, 0, time.UTC))

	got, err := repo.FindPlayerSeason(u.ID, s.ID)
	require.NoError(t, err)
	assert.Nil(t, got, "a player who has not played this season has no row — not an error")
}

// AC1/AC4: the first award INSERTS, and every counter starts from the right base.
func TestApplySeasonPoints_FirstAwardInserts(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2097, time.April, 1, 0, 0, 0, 0, time.UTC))
	winner := makeUser(t, db, "sp-w@s.test")
	absent := makeUser(t, db, "sp-a@s.test")

	snaps, err := repo.ApplySeasonPoints(s.ID, map[uint]SPAward{
		winner.ID: {SP: 250, Completed: true},
		absent.ID: {SP: 0, Completed: false},
	})
	require.NoError(t, err)

	require.Contains(t, snaps, winner.ID)
	assert.Equal(t, 250, snaps[winner.ID].SP)
	assert.Equal(t, 0, snaps[winner.ID].PreviousSP)
	assert.Equal(t, "iron", snaps[winner.ID].Tier)
	assert.Equal(t, 1, snaps[winner.ID].GamesPlayed)
	assert.Equal(t, 1, snaps[winner.ID].GamesCompleted)

	// A ZERO AWARD IS NOT A NO-OP: the absent seat still counts a games_played,
	// which is what makes games_played - games_completed the absence count.
	require.Contains(t, snaps, absent.ID)
	assert.Equal(t, 0, snaps[absent.ID].SP)
	assert.Equal(t, 1, snaps[absent.ID].GamesPlayed)
	assert.Equal(t, 0, snaps[absent.ID].GamesCompleted)

	row, err := repo.FindPlayerSeason(absent.ID, s.ID)
	require.NoError(t, err)
	require.NotNil(t, row, "the row is written even for a 0-SP award")
	assert.Equal(t, 0, row.SP)
	assert.Equal(t, 1, row.GamesPlayed)
	assert.Equal(t, 0, row.GamesCompleted)
}

// AC1: SP ACCUMULATES across matches rather than being overwritten, and the
// pre-award total comes back so the caller can decide tieredUp without a second
// read.
func TestApplySeasonPoints_AccumulatesAndReportsPreviousSP(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2097, time.July, 1, 0, 0, 0, 0, time.UTC))
	u := makeUser(t, db, "sp-acc@s.test")

	_, err := repo.ApplySeasonPoints(s.ID, map[uint]SPAward{u.ID: {SP: 400, Completed: true}})
	require.NoError(t, err)

	snaps, err := repo.ApplySeasonPoints(s.ID, map[uint]SPAward{u.ID: {SP: 150, Completed: true}})
	require.NoError(t, err)

	assert.Equal(t, 550, snaps[u.ID].SP, "sp accumulates")
	assert.Equal(t, 400, snaps[u.ID].PreviousSP, "the pre-award total comes back")
	assert.Equal(t, 2, snaps[u.ID].GamesPlayed)
	assert.Equal(t, 2, snaps[u.ID].GamesCompleted)
	// AC2 + D7: the tier is DERIVED from the new total, and the denormalized
	// column is refreshed to match on every write.
	assert.Equal(t, "bronze", snaps[u.ID].Tier, "550 SP crosses the 500 Bronze floor")

	row, err := repo.FindPlayerSeason(u.ID, s.ID)
	require.NoError(t, err)
	require.NotNil(t, row)
	assert.Equal(t, "bronze", row.RankTier, "rank_tier is refreshed on every SP write")
	assert.Equal(t, TierForSP(row.SP), row.RankTier, "stored and derived must agree")
}

// A player's rows are per-season: a second window starts them from zero (the
// "soft reset" is a new season_id, never an update of the old row).
func TestApplySeasonPoints_SeasonsAreIndependent(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	first := makeSeason(t, db, time.Date(2096, time.January, 1, 0, 0, 0, 0, time.UTC))
	second := makeSeason(t, db, time.Date(2096, time.April, 1, 0, 0, 0, 0, time.UTC))
	u := makeUser(t, db, "sp-two@s.test")

	_, err := repo.ApplySeasonPoints(first.ID, map[uint]SPAward{u.ID: {SP: 2000, Completed: true}})
	require.NoError(t, err)
	snaps, err := repo.ApplySeasonPoints(second.ID, map[uint]SPAward{u.ID: {SP: 100, Completed: true}})
	require.NoError(t, err)

	assert.Equal(t, 100, snaps[u.ID].SP, "the new season starts from zero")
	assert.Equal(t, "iron", snaps[u.ID].Tier)

	old, err := repo.FindPlayerSeason(u.ID, first.ID)
	require.NoError(t, err)
	require.NotNil(t, old)
	assert.Equal(t, 2000, old.SP, "the prior season's row is left untouched")
	assert.Equal(t, "silver", old.RankTier)
}

func TestApplySeasonPoints_EmptyIsANoOp(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2096, time.July, 1, 0, 0, 0, 0, time.UTC))

	snaps, err := repo.ApplySeasonPoints(s.ID, nil)
	require.NoError(t, err)
	assert.Empty(t, snaps)
}

// All-or-nothing: an unknown user violates the FK, and the whole batch rolls
// back rather than half-crediting the table.
func TestApplySeasonPoints_UnknownUserRollsBackTheBatch(t *testing.T) {
	db := getTestDB(t)
	repo := NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2095, time.October, 1, 0, 0, 0, 0, time.UTC))
	good := makeUser(t, db, "sp-good@s.test")

	// A deliberately unassigned id, ordered AFTER the real one so the good write
	// has already happened inside the transaction when the bad one fails.
	_, err := repo.ApplySeasonPoints(s.ID, map[uint]SPAward{
		good.ID:              {SP: 200, Completed: true},
		good.ID + 10_000_000: {SP: 200, Completed: true},
	})
	require.Error(t, err)

	row, err := repo.FindPlayerSeason(good.ID, s.ID)
	require.NoError(t, err)
	assert.Nil(t, row, "the successful seat's write must have rolled back with the batch")
}
