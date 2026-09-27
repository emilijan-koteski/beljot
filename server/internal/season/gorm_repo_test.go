// package season_test, NOT season, and the distinction is MANDATORY: this file
// imports `user` for its fixtures, and Story 13.3 opened the `user` -> `season`
// edge (the profile's SeasonRankReader). An in-package test importing `user`
// would close the cycle season(test) -> user -> season and stop compiling.
package season_test

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/season"
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
func makeSeason(t *testing.T, db *gorm.DB, start time.Time) *season.Season {
	t.Helper()
	end := start.AddDate(0, 3, 0)
	s := &season.Season{Name: season.QuarterName(start), StartedAt: start, EndsAt: end}
	require.NoError(t, db.Create(s).Error)
	return s
}

func TestCurrentSeason_ReadsTheCoveringWindow(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)

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
	repo := season.NewGormRepository(db)

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
	repo := season.NewGormRepository(db)

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
	require.NoError(t, db.Model(&season.Season{}).Where("started_at = ?", created.StartedAt).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestFindPlayerSeason_MissIsNilNotAnError(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	u := makeUser(t, db, "sp-miss@s.test")
	s := makeSeason(t, db, time.Date(2097, time.January, 1, 0, 0, 0, 0, time.UTC))

	got, err := repo.FindPlayerSeason(u.ID, s.ID)
	require.NoError(t, err)
	assert.Nil(t, got, "a player who has not played this season has no row — not an error")
}

// fixedAward is one player's entry in a hand-built match: the change to apply
// (bypassing the formula) and their presence at the terminal end.
type fixedAward struct {
	change    int
	completed bool
}

// applyAwards writes one match through ApplySeasonPoints with FIXED changes, so a
// persistence test controls the numbers exactly. The formula's own wiring into
// the transaction is covered by the Service tests below.
func applyAwards(repo *season.GormRepository, seasonID uint, awards map[uint]fixedAward) (map[uint]season.PlayerSeasonSnapshot, error) {
	completed := make(map[uint]bool, len(awards))
	changes := make(map[uint]int, len(awards))
	for id, a := range awards {
		completed[id] = a.completed
		changes[id] = a.change
	}
	return repo.ApplySeasonPoints(seasonID, completed, func(map[uint]int) (map[uint]int, error) {
		return changes, nil
	})
}

// The first match INSERTS a row, and every counter starts from the right base.
// A first-ever LOSS writes 0, never a negative total (the old upsert wrote the
// award straight in as the total).
func TestApplySeasonPoints_FirstMatchInsertsTheRow(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2097, time.April, 1, 0, 0, 0, 0, time.UTC))
	winner := makeUser(t, db, "sp-w@s.test")
	loser := makeUser(t, db, "sp-l@s.test")
	absent := makeUser(t, db, "sp-a@s.test")

	snaps, err := applyAwards(repo, s.ID, map[uint]fixedAward{
		winner.ID: {change: 250, completed: true},
		loser.ID:  {change: -18, completed: true},
		absent.ID: {change: 36, completed: false},
	})
	require.NoError(t, err)

	require.Contains(t, snaps, winner.ID)
	assert.Equal(t, 250, snaps[winner.ID].SP)
	assert.Equal(t, 0, snaps[winner.ID].PreviousSP, "a missing row is 0 SP")
	assert.Equal(t, "bronze", snaps[winner.ID].Tier)
	assert.Equal(t, 1, snaps[winner.ID].GamesPlayed)
	assert.Equal(t, 1, snaps[winner.ID].GamesCompleted)

	assert.Equal(t, 0, snaps[loser.ID].SP, "a first-ever loss writes 0")
	assert.Equal(t, 0, snaps[loser.ID].PreviousSP)
	assert.Equal(t, "iron", snaps[loser.ID].Tier)

	// Presence no longer gates the change; it only drives games_completed.
	assert.Equal(t, 36, snaps[absent.ID].SP)
	assert.Equal(t, 1, snaps[absent.ID].GamesPlayed)
	assert.Equal(t, 0, snaps[absent.ID].GamesCompleted)

	row, err := repo.FindPlayerSeason(loser.ID, s.ID)
	require.NoError(t, err)
	require.NotNil(t, row, "the row is written for a floored loss too")
	assert.Equal(t, 0, row.SP)
	assert.Equal(t, 1, row.GamesPlayed)
	assert.Equal(t, 1, row.GamesCompleted)
}

// SP moves in both directions across matches, the pre-match total comes back so
// the caller can tell a climb from a drop without a second read, and the
// denormalized rank_tier follows the total down as well as up.
func TestApplySeasonPoints_RisesAndFallsAndReportsPreviousSP(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2097, time.July, 1, 0, 0, 0, 0, time.UTC))
	u := makeUser(t, db, "sp-acc@s.test")

	_, err := applyAwards(repo, s.ID, map[uint]fixedAward{u.ID: {change: 200, completed: true}})
	require.NoError(t, err)

	snaps, err := applyAwards(repo, s.ID, map[uint]fixedAward{u.ID: {change: 150, completed: true}})
	require.NoError(t, err)
	assert.Equal(t, 350, snaps[u.ID].SP, "a win adds to the total")
	assert.Equal(t, 200, snaps[u.ID].PreviousSP, "the pre-match total comes back")
	assert.Equal(t, "silver", snaps[u.ID].Tier, "350 SP crosses the 300 Silver floor")

	snaps, err = applyAwards(repo, s.ID, map[uint]fixedAward{u.ID: {change: -100, completed: true}})
	require.NoError(t, err)
	assert.Equal(t, 250, snaps[u.ID].SP, "a loss takes SP away")
	assert.Equal(t, 350, snaps[u.ID].PreviousSP)
	assert.Equal(t, "bronze", snaps[u.ID].Tier, "250 SP is back in Bronze")
	assert.Equal(t, 3, snaps[u.ID].GamesPlayed)
	assert.Equal(t, 3, snaps[u.ID].GamesCompleted)

	row, err := repo.FindPlayerSeason(u.ID, s.ID)
	require.NoError(t, err)
	require.NotNil(t, row)
	assert.Equal(t, 250, row.SP)
	assert.Equal(t, "bronze", row.RankTier, "rank_tier is refreshed on a drop too")
	require.NotNil(t, row.RankDivision)
	assert.Equal(t, 3, *row.RankDivision, "250 SP is Bronze 3, and the division moves with the tier")
	assert.Equal(t, 3, snaps[u.ID].Division)
}

// EVERY AWARD WRITES THE WHOLE RANK (Story 13.5): rank_tier and rank_division
// together, the division NULL for Master and Grandmaster, so a season that
// ends already holds the rank each row finished on.
func TestApplySeasonPoints_WritesTheDivision(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2083, time.January, 1, 0, 0, 0, 0, time.UTC))
	floored := makeUser(t, db, "dv-fl@s.test")
	gold := makeUser(t, db, "dv-gd@s.test")
	master := makeUser(t, db, "dv-ms@s.test")
	gm := makeUser(t, db, "dv-gm@s.test")

	snaps, err := applyAwards(repo, s.ID, map[uint]fixedAward{
		floored.ID: {change: -18, completed: true},
		gold.ID:    {change: 700, completed: true},
		master.ID:  {change: 1250, completed: true},
		gm.ID:      {change: 1500, completed: true},
	})
	require.NoError(t, err)

	cases := []struct {
		name     string
		id       uint
		tier     string
		division *int
	}{
		{"a first-ever loss is Iron 1", floored.ID, "iron", intPtr(1)},
		{"700 SP is Gold 2", gold.ID, "gold", intPtr(2)},
		{"Master has no division", master.ID, "master", nil},
		{"Grandmaster has no division", gm.ID, "grandmaster", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row, err := repo.FindPlayerSeason(tc.id, s.ID)
			require.NoError(t, err)
			require.NotNil(t, row)
			assert.Equal(t, tc.tier, row.RankTier)
			assert.Equal(t, tc.division, row.RankDivision)
			assert.Equal(t, tc.tier, snaps[tc.id].Tier)
			if tc.division == nil {
				assert.Zero(t, snaps[tc.id].Division)
			} else {
				assert.Equal(t, *tc.division, snaps[tc.id].Division)
			}
		})
	}

	// A drop out of Master writes the division back.
	snaps, err = applyAwards(repo, s.ID, map[uint]fixedAward{master.ID: {change: -100, completed: true}})
	require.NoError(t, err)
	row, err := repo.FindPlayerSeason(master.ID, s.ID)
	require.NoError(t, err)
	assert.Equal(t, "diamond", row.RankTier)
	assert.Equal(t, intPtr(3), row.RankDivision, "1150 SP is Diamond 3")
	assert.Equal(t, 3, snaps[master.ID].Division)
}

func intPtr(v int) *int { return &v }

// insertPreDivisionRow writes a row the way a pre-000028 award left it: the
// old formula's total with a bare tier and NULL rank_division.
func insertPreDivisionRow(t *testing.T, db *gorm.DB, userID, seasonID uint, sp int, tier string, gamesPlayed int) {
	t.Helper()
	require.NoError(t, db.Exec(`
		INSERT INTO player_seasons (user_id, season_id, sp, rank_tier, games_played, games_completed, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())`, userID, seasonID, sp, tier, gamesPlayed, gamesPlayed).Error)
}

// THE ACCEPTANCE CRITERION, end to end on real Postgres (Story 13.5): given a
// mix of Q3 and Q4 rows, the archive and both seasons' leaderboards show Q3's
// stored tier with no division, and Q4's tier plus division.
func TestService_EndedSeasonsReadTheStoredRank(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	svc := season.NewService(repo)
	q3 := makeSeason(t, db, time.Date(2082, time.July, 1, 0, 0, 0, 0, time.UTC))
	q4 := makeSeason(t, db, time.Date(2082, time.October, 1, 0, 0, 0, 0, time.UTC))
	now := time.Date(2082, time.November, 15, 0, 0, 0, 0, time.UTC)

	u := makeUser(t, db, "mx-u1@s.test")
	// Q3: the old climb-only formula's 3500 SP, stored as a bare "gold". On the
	// live floors 3500 would derive Grandmaster.
	insertPreDivisionRow(t, db, u.ID, q3.ID, 3500, "gold", 90)
	// Q4: written by the new award path, so it carries its division.
	_, err := applyAwards(repo, q4.ID, map[uint]fixedAward{u.ID: {change: 700, completed: true}})
	require.NoError(t, err)

	archive, err := svc.ArchiveView(u.ID, now)
	require.NoError(t, err)
	require.Len(t, archive.Items, 1, "Q4 is running, so only Q3 is history")
	assert.Equal(t, "gold", archive.Items[0].Tier, "the stored tier, never Grandmaster")
	assert.Nil(t, archive.Items[0].Division)

	past, err := svc.LeaderboardView(u.ID, q3.ID, 10, 0, now)
	require.NoError(t, err)
	require.Len(t, past.Items, 1)
	assert.Equal(t, "gold", past.Items[0].Tier)
	assert.Nil(t, past.Items[0].Division)
	require.NotNil(t, past.Viewer)
	assert.Equal(t, "gold", past.Viewer.Tier)
	assert.Nil(t, past.Viewer.Division)

	current, err := svc.LeaderboardView(u.ID, 0, 10, 0, now)
	require.NoError(t, err)
	require.Len(t, current.Items, 1)
	assert.Equal(t, "gold", current.Items[0].Tier)
	assert.Equal(t, intPtr(2), current.Items[0].Division, "Q4 shows tier plus division")
	require.NotNil(t, current.Viewer)
	assert.Equal(t, intPtr(2), current.Viewer.Division)

	// The same Q4 row once Q4 has ENDED: the archive now lists it, from its
	// stored snapshot, division included.
	later := q4.EndsAt.Add(time.Hour)
	archive, err = svc.ArchiveView(u.ID, later)
	require.NoError(t, err)
	require.Len(t, archive.Items, 2)
	assert.Equal(t, q4.ID, archive.Items[0].SeasonID, "newest-first")
	assert.Equal(t, "gold", archive.Items[0].Tier)
	assert.Equal(t, intPtr(2), archive.Items[0].Division)
	assert.Nil(t, archive.Items[1].Division, "Q3 still has none")
}

// The floor: a player on 10 SP whose loss computes to -18 ends on 0, and the
// snapshot's previous total makes the applied change -10.
func TestApplySeasonPoints_FloorsTheTotalAtZero(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2097, time.October, 1, 0, 0, 0, 0, time.UTC))
	u := makeUser(t, db, "sp-flr@s.test")

	_, err := applyAwards(repo, s.ID, map[uint]fixedAward{u.ID: {change: 10, completed: true}})
	require.NoError(t, err)
	snaps, err := applyAwards(repo, s.ID, map[uint]fixedAward{u.ID: {change: -18, completed: true}})
	require.NoError(t, err)

	assert.Equal(t, 0, snaps[u.ID].SP)
	assert.Equal(t, 10, snaps[u.ID].PreviousSP)
	assert.Equal(t, -10, snaps[u.ID].SP-snaps[u.ID].PreviousSP, "the applied change")
}

// The changes callback receives every listed player's LOCKED current total at
// once, a missing row as 0, and nothing else.
func TestApplySeasonPoints_ChangesSeeTheLockedTotals(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2094, time.January, 1, 0, 0, 0, 0, time.UTC))
	veteran := makeUser(t, db, "sp-vet@s.test")
	rookie := makeUser(t, db, "sp-rk@s.test")
	_, err := applyAwards(repo, s.ID, map[uint]fixedAward{veteran.ID: {change: 500, completed: true}})
	require.NoError(t, err)

	var seen map[uint]int
	snaps, err := repo.ApplySeasonPoints(s.ID, map[uint]bool{veteran.ID: true, rookie.ID: true},
		func(current map[uint]int) (map[uint]int, error) {
			seen = current
			return map[uint]int{veteran.ID: -20, rookie.ID: 30}, nil
		})
	require.NoError(t, err)

	assert.Equal(t, map[uint]int{veteran.ID: 500, rookie.ID: 0}, seen)
	assert.Equal(t, 480, snaps[veteran.ID].SP)
	assert.Equal(t, 30, snaps[rookie.ID].SP)
}

// A failing computation rolls the whole match back, INCLUDING the zero rows the
// lock step inserted: a player whose first match failed to score has no row.
func TestApplySeasonPoints_AFailedComputationWritesNothing(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2094, time.April, 1, 0, 0, 0, 0, time.UTC))
	veteran := makeUser(t, db, "sp-fv@s.test")
	rookie := makeUser(t, db, "sp-fr@s.test")
	_, err := applyAwards(repo, s.ID, map[uint]fixedAward{veteran.ID: {change: 500, completed: true}})
	require.NoError(t, err)

	cases := map[string]season.SPChanges{
		"the formula errors": func(map[uint]int) (map[uint]int, error) {
			return nil, fmt.Errorf("malformed outcome")
		},
		"a listed player has no change": func(map[uint]int) (map[uint]int, error) {
			return map[uint]int{veteran.ID: 30}, nil
		},
	}
	for name, changes := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := repo.ApplySeasonPoints(s.ID, map[uint]bool{veteran.ID: true, rookie.ID: true}, changes)
			require.Error(t, err)

			row, err := repo.FindPlayerSeason(rookie.ID, s.ID)
			require.NoError(t, err)
			assert.Nil(t, row, "the inserted zero row must roll back with the batch")

			row, err = repo.FindPlayerSeason(veteran.ID, s.ID)
			require.NoError(t, err)
			require.NotNil(t, row)
			assert.Equal(t, 500, row.SP)
			assert.Equal(t, 1, row.GamesPlayed, "no counter moved")
		})
	}
}

// A player's rows are per-season: a second window starts them from zero (the
// "soft reset" is a new season_id, never an update of the old row).
func TestApplySeasonPoints_SeasonsAreIndependent(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	first := makeSeason(t, db, time.Date(2096, time.January, 1, 0, 0, 0, 0, time.UTC))
	second := makeSeason(t, db, time.Date(2096, time.April, 1, 0, 0, 0, 0, time.UTC))
	u := makeUser(t, db, "sp-two@s.test")

	_, err := applyAwards(repo, first.ID, map[uint]fixedAward{u.ID: {change: 2000, completed: true}})
	require.NoError(t, err)
	snaps, err := applyAwards(repo, second.ID, map[uint]fixedAward{u.ID: {change: 100, completed: true}})
	require.NoError(t, err)

	assert.Equal(t, 100, snaps[u.ID].SP, "the new season starts from zero")
	assert.Equal(t, 0, snaps[u.ID].PreviousSP)
	assert.Equal(t, "iron", snaps[u.ID].Tier)

	old, err := repo.FindPlayerSeason(u.ID, first.ID)
	require.NoError(t, err)
	require.NotNil(t, old)
	assert.Equal(t, 2000, old.SP, "the prior season's row is left untouched")
	assert.Equal(t, "grandmaster", old.RankTier)
}

func TestApplySeasonPoints_EmptyIsANoOp(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2096, time.July, 1, 0, 0, 0, 0, time.UTC))

	called := false
	snaps, err := repo.ApplySeasonPoints(s.ID, nil, func(map[uint]int) (map[uint]int, error) {
		called = true
		return nil, nil
	})
	require.NoError(t, err)
	assert.Empty(t, snaps)
	assert.False(t, called, "no players, no computation")
}

// All-or-nothing: an unknown user violates the FK, and the whole batch rolls
// back rather than half-crediting the table.
func TestApplySeasonPoints_UnknownUserRollsBackTheBatch(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2095, time.October, 1, 0, 0, 0, 0, time.UTC))
	good := makeUser(t, db, "sp-good@s.test")

	// A deliberately unassigned id, ordered AFTER the real one so the good row
	// has already been inserted inside the transaction when the bad one fails.
	_, err := applyAwards(repo, s.ID, map[uint]fixedAward{
		good.ID:              {change: 200, completed: true},
		good.ID + 10_000_000: {change: 200, completed: true},
	})
	require.Error(t, err)

	row, err := repo.FindPlayerSeason(good.ID, s.ID)
	require.NoError(t, err)
	assert.Nil(t, row, "the successful seat's write must have rolled back with the batch")
}

// THE LOST-UPDATE GUARD, against real Postgres with two real transactions. Two
// matches finish at once and share a player. The first to lock the shared row
// holds it through a slow computation; the second must WAIT, then compute from
// the first's committed total, so both changes land. It needs committed rows two
// separate transactions can contend on, so it commits and hard-deletes its own
// rows on cleanup (the wallet concurrency tests' pattern), and it runs only
// against the database BELJOT_DB_URL names.
func TestApplySeasonPoints_ConcurrentMatchesSharingAPlayerLoseNoChange(t *testing.T) {
	dsn := os.Getenv("BELJOT_DB_URL")
	if dsn == "" {
		t.Skip("skipping concurrency test: BELJOT_DB_URL not set (it commits rows)")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skip("skipping integration test: database not available")
	}
	// Registered first so it runs LAST, after the row cleanup below.
	if sqlDB, err := db.DB(); err == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}

	// A quarter no other test uses; a run that crashed before its cleanup is
	// cleared first so the unique started_at cannot fail this one.
	start := time.Date(2150, time.January, 1, 0, 0, 0, 0, time.UTC)
	db.Exec(`DELETE FROM player_seasons WHERE season_id IN (SELECT id FROM seasons WHERE started_at = ?)`, start)
	db.Exec(`DELETE FROM seasons WHERE started_at = ?`, start)

	stamp := fmt.Sprintf("%08d", time.Now().UnixNano()%1e8)
	shared := makeUser(t, db, "cc"+stamp+"s@s.test")
	a := makeUser(t, db, "cc"+stamp+"a@s.test")
	b := makeUser(t, db, "cc"+stamp+"b@s.test")
	s := makeSeason(t, db, start)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM player_seasons WHERE season_id = ?`, s.ID)
		db.Exec(`DELETE FROM seasons WHERE id = ?`, s.ID)
		for _, u := range []*user.User{shared, a, b} {
			db.Unscoped().Delete(&user.User{}, u.ID)
		}
	})
	repo := season.NewGormRepository(db)

	// A COMMITTED prior total for the shared player, so both matches find an
	// existing row and only SELECT ... FOR UPDATE can serialise them (with no
	// prior row the unique-index wait on the zero-row INSERT would do it alone).
	_, err = applyAwards(repo, s.ID, map[uint]fixedAward{shared.ID: {change: 100, completed: true}})
	require.NoError(t, err)

	var (
		mu    sync.Mutex
		seen  []int
		first sync.Once
		wg    sync.WaitGroup
		errs  [2]error
	)
	match := func(i int, other uint) {
		defer wg.Done()
		_, errs[i] = repo.ApplySeasonPoints(s.ID, map[uint]bool{shared.ID: true, other: true},
			func(current map[uint]int) (map[uint]int, error) {
				mu.Lock()
				seen = append(seen, current[shared.ID])
				mu.Unlock()
				// Whoever locks first holds the row long enough for the other
				// transaction to reach the lock and wait on it.
				first.Do(func() { time.Sleep(300 * time.Millisecond) })
				return map[uint]int{shared.ID: 30, other: 10}, nil
			})
	}
	wg.Add(2)
	go match(0, a.ID)
	go match(1, b.ID)
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])

	assert.ElementsMatch(t, []int{100, 130}, seen,
		"the second match computed from the first's committed total, not from the same stale 100")
	row, err := repo.FindPlayerSeason(shared.ID, s.ID)
	require.NoError(t, err)
	require.NotNil(t, row)
	assert.Equal(t, 160, row.SP, "neither match's change was lost")
	assert.Equal(t, 3, row.GamesPlayed)
}

// THE AWARD PATH END TO END: the service computes each human's change with
// ComputeSPChanges over the totals the repository locked, inside the same
// transaction, and no total lands below 0.
func TestService_ApplySeasonPoints_ScoresFromThePreMatchRows(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	svc := season.NewService(repo)
	s := makeSeason(t, db, time.Date(2086, time.January, 1, 0, 0, 0, 0, time.UTC))
	now := time.Date(2086, time.February, 1, 0, 0, 0, 0, time.UTC)

	strong := makeUser(t, db, "sv-st@s.test")
	weak := makeUser(t, db, "sv-wk@s.test")
	mid := makeUser(t, db, "sv-md@s.test")
	_, err := applyAwards(repo, s.ID, map[uint]fixedAward{
		strong.ID: {change: 900, completed: true},
		weak.ID:   {change: 10, completed: true},
		mid.ID:    {change: 400, completed: true},
	})
	require.NoError(t, err)
	pre := map[uint]int{strong.ID: 900, weak.ID: 10, mid.ID: 400}

	// mid + a bot (avg 500) beat strong + weak (avg 455) 1100:700, so the losers
	// were the underdogs and lose about 16; weak is on 10 SP, so the floor binds.
	// mid dropped before the end and is scored anyway.
	outcome := match.MatchOutcome{
		Seats: [4]match.OutcomeSeat{
			{UserID: strong.ID, Team: 0, Completed: true},
			{UserID: mid.ID, Team: 1, Completed: false},
			{UserID: weak.ID, Team: 0, Completed: true},
			{IsBot: true, Team: 1},
		},
		WinnerTeam: 1, TeamScores: [2]int{700, 1100}, Target: 1001, AbandonedSeat: -1,
	}
	want, err := season.ComputeSPChanges(outcome, pre)
	require.NoError(t, err)

	got, err := svc.ApplySeasonPoints(outcome, now)
	require.NoError(t, err)
	require.Len(t, got, 3, "the bot seat gets no row")

	for id, prev := range pre {
		next := season.ApplySPChange(prev, want[id])
		assert.Equal(t, next, got[id].SP, "user %d", id)
		assert.Equal(t, next-prev, got[id].SPChange, "user %d: the applied change", id)
		assert.GreaterOrEqual(t, got[id].SP, 0)
		row, err := repo.FindPlayerSeason(id, s.ID)
		require.NoError(t, err)
		assert.Equal(t, next, row.SP, "user %d: the row matches the snapshot", id)
	}
	assert.Equal(t, 0, got[weak.ID].SP, "the floored loser")
	assert.Equal(t, -10, got[weak.ID].SPChange)

	row, err := repo.FindPlayerSeason(mid.ID, s.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, row.GamesCompleted, "an absent seat is scored but completes nothing")
	assert.Equal(t, 2, row.GamesPlayed)
}

// --- Story 13.2: leaderboard reads ---

// seedStanding creates a user and drops them into the season with a fixed SP
// total, going through ApplySeasonPoints so the rows are written exactly the way
// the match-end path writes them.
func seedStanding(t *testing.T, db *gorm.DB, repo *season.GormRepository, seasonID uint, email string, sp int) *user.User {
	t.Helper()
	u := makeUser(t, db, email)
	_, err := applyAwards(repo, seasonID, map[uint]fixedAward{u.ID: {change: sp, completed: true}})
	require.NoError(t, err)
	return u
}

// The whole page in one call, for tests that need the full order.
func fullLadder(t *testing.T, repo *season.GormRepository, seasonID uint) []season.LeaderboardEntry {
	t.Helper()
	entries, _, err := repo.LeaderboardPage(seasonID, 100, 0)
	require.NoError(t, err)
	return entries
}

func TestLeaderboardPage_OrdersBySPDescending(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2095, time.January, 1, 0, 0, 0, 0, time.UTC))

	low := seedStanding(t, db, repo, s.ID, "lb-lo@s.test", 100)
	high := seedStanding(t, db, repo, s.ID, "lb-hi@s.test", 9000)
	mid := seedStanding(t, db, repo, s.ID, "lb-md@s.test", 1200)

	entries, total, err := repo.LeaderboardPage(s.ID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	require.Len(t, entries, 3)

	assert.Equal(t, []uint{high.ID, mid.ID, low.ID},
		[]uint{entries[0].UserID, entries[1].UserID, entries[2].UserID},
		"best SP first")
	assert.Equal(t, 9000, entries[0].SP)
	// The username comes from the JOIN, not from a second query or a `season` ->
	// `user` Go import (Story 13.2 D2).
	assert.Equal(t, high.Username, entries[0].Username)
	assert.Equal(t, 1, entries[0].GamesPlayed)
}

// The tiebreak is not cosmetic: without a second ORDER BY column two players on
// equal SP can swap between the page-1 and page-2 queries and be duplicated or
// skipped. Ascending user_id is the tiebreak, and CountAhead counts under it.
// The page and the viewer entry select the stored rank snapshot beside SP, so
// an ended season can show the rank each row finished on (Story 13.5).
func TestLeaderboardReads_SelectTheStoredRank(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2082, time.January, 1, 0, 0, 0, 0, time.UTC))

	divided := seedStanding(t, db, repo, s.ID, "lb-rk1@s.test", 700)
	old := makeUser(t, db, "lb-rk2@s.test")
	insertPreDivisionRow(t, db, old.ID, s.ID, 3500, "gold", 40)

	entries := fullLadder(t, repo, s.ID)
	require.Len(t, entries, 2)
	assert.Equal(t, old.ID, entries[0].UserID)
	assert.Equal(t, "gold", entries[0].RankTier)
	assert.Nil(t, entries[0].RankDivision, "a pre-division row reads back NULL")
	assert.Equal(t, divided.ID, entries[1].UserID)
	assert.Equal(t, "gold", entries[1].RankTier)
	assert.Equal(t, intPtr(2), entries[1].RankDivision)

	viewer, err := repo.FindLeaderboardEntry(s.ID, divided.ID)
	require.NoError(t, err)
	require.NotNil(t, viewer)
	assert.Equal(t, "gold", viewer.RankTier)
	assert.Equal(t, intPtr(2), viewer.RankDivision)
}

func TestLeaderboardPage_BreaksTiesByAscendingUserID(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2095, time.April, 1, 0, 0, 0, 0, time.UTC))

	// Created in ascending id order, all on the same SP.
	a := seedStanding(t, db, repo, s.ID, "lb-t1@s.test", 900)
	b := seedStanding(t, db, repo, s.ID, "lb-t2@s.test", 900)
	c := seedStanding(t, db, repo, s.ID, "lb-t3@s.test", 900)
	require.Less(t, a.ID, b.ID)
	require.Less(t, b.ID, c.ID)

	entries := fullLadder(t, repo, s.ID)
	require.Len(t, entries, 3)
	assert.Equal(t, []uint{a.ID, b.ID, c.ID},
		[]uint{entries[0].UserID, entries[1].UserID, entries[2].UserID})
}

// THE PREDICATE A REVIEWER SHOULD CHECK FIRST. Table()/Joins() takes GORM out of
// model-land, so the soft-delete scope does NOT apply automatically and
// `users.deleted_at IS NULL` is written by hand. A deleted account must vanish
// from the items AND from the total, or the two contradict each other.
func TestLeaderboardPage_ExcludesSoftDeletedUsers(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2095, time.July, 1, 0, 0, 0, 0, time.UTC))

	alive := seedStanding(t, db, repo, s.ID, "lb-al@s.test", 1000)
	gone := seedStanding(t, db, repo, s.ID, "lb-gn@s.test", 50000)

	// Soft delete, the way the app does: GORM stamps deleted_at, the row stays.
	require.NoError(t, db.Delete(&user.User{}, gone.ID).Error)

	entries, total, err := repo.LeaderboardPage(s.ID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total, "the deleted account is not counted in the total either")
	require.Len(t, entries, 1)
	assert.Equal(t, alive.ID, entries[0].UserID,
		"a deleted account must not keep the top slot forever")

	// And the player_seasons row is still there — this is a VISIBILITY filter,
	// not a cascade, so the test proves the query excluded it rather than the
	// data having disappeared.
	row, err := repo.FindPlayerSeason(gone.ID, s.ID)
	require.NoError(t, err)
	require.NotNil(t, row)
	assert.Equal(t, 50000, row.SP)
}

// Paging must partition the ladder exactly: every player once, in order, with no
// gap and no repeat across page boundaries.
func TestLeaderboardPage_OffsetPagingHasNoGapsOrDuplicates(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2095, time.October, 1, 0, 0, 0, 0, time.UTC))

	const n = 7
	want := make([]uint, 0, n)
	for i := 0; i < n; i++ {
		// Descending SP so creation order is also ladder order.
		u := seedStanding(t, db, repo, s.ID, fmt.Sprintf("lb-p%d@s.test", i), (n-i)*100)
		want = append(want, u.ID)
	}

	got := make([]uint, 0, n)
	for offset := 0; ; offset += 3 {
		entries, total, err := repo.LeaderboardPage(s.ID, 3, offset)
		require.NoError(t, err)
		assert.Equal(t, int64(n), total, "the total never changes with the offset")
		if len(entries) == 0 {
			break
		}
		for _, e := range entries {
			got = append(got, e.UserID)
		}
	}
	assert.Equal(t, want, got, "the pages concatenate back into the full ladder exactly once")

	// Offset past the end: an empty page, and the total is unchanged.
	entries, total, err := repo.LeaderboardPage(s.ID, 10, 999)
	require.NoError(t, err)
	assert.Empty(t, entries)
	assert.Equal(t, int64(n), total)
}

func TestLeaderboardPage_EmptySeasonIsEmptyNotAnError(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2094, time.January, 1, 0, 0, 0, 0, time.UTC))

	entries, total, err := repo.LeaderboardPage(s.ID, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, entries)
	assert.Equal(t, int64(0), total)
	assert.NotNil(t, entries, "an empty page is a slice, so it serializes as [] not null")
}

// THE INVARIANT THE WHOLE STORY TURNS ON: CountAhead + 1 must equal the row's own
// slot in the list, for EVERY player — tied and untied alike. If the two
// predicates ever diverge, a viewer is told a position that contradicts the list
// they are standing in, and nothing else would notice.
func TestLeaderboardCountAhead_AgreesWithEveryRowsListPosition(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2094, time.April, 1, 0, 0, 0, 0, time.UTC))

	// Deliberately messy: a clear leader, a three-way tie in the middle, and a
	// two-way tie at the bottom.
	sps := []int{5000, 900, 900, 900, 100, 100}
	for i, sp := range sps {
		seedStanding(t, db, repo, s.ID, fmt.Sprintf("lb-c%d@s.test", i), sp)
	}

	entries := fullLadder(t, repo, s.ID)
	require.Len(t, entries, len(sps))

	for i, e := range entries {
		ahead, err := repo.CountAhead(s.ID, e.SP, e.UserID)
		require.NoError(t, err)
		assert.Equal(t, int64(i), ahead,
			"user %d (sp %d) sits in slot %d, so exactly %d rows sort ahead of it",
			e.UserID, e.SP, i+1, i)
	}

	// Spelled out for the tie specifically: the three players on 900 SP get
	// DISTINCT positions 2, 3, 4 — not the same number, which a plain
	// COUNT(sp > 900) would hand all three.
	tied := make([]int64, 0, 3)
	for _, e := range entries {
		if e.SP != 900 {
			continue
		}
		ahead, err := repo.CountAhead(s.ID, e.SP, e.UserID)
		require.NoError(t, err)
		tied = append(tied, ahead+1)
	}
	assert.Equal(t, []int64{2, 3, 4}, tied)
}

// CountAhead runs through the same scope as the page, so a deleted account must
// not push a live player down a slot.
func TestLeaderboardCountAhead_ExcludesSoftDeletedUsers(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2094, time.July, 1, 0, 0, 0, 0, time.UTC))

	gone := seedStanding(t, db, repo, s.ID, "lb-dg@s.test", 50000)
	me := seedStanding(t, db, repo, s.ID, "lb-dm@s.test", 1000)

	ahead, err := repo.CountAhead(s.ID, 1000, me.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), ahead, "while the other account is live it sits ahead")

	require.NoError(t, db.Delete(&user.User{}, gone.ID).Error)

	ahead, err = repo.CountAhead(s.ID, 1000, me.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), ahead, "once deleted it must stop counting — position 1, matching the list")
}

// Seasons are independent: another window's standings never leak into this one's
// list, total or positions.
func TestLeaderboardPage_IsScopedToOneSeason(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	this := makeSeason(t, db, time.Date(2093, time.January, 1, 0, 0, 0, 0, time.UTC))
	other := makeSeason(t, db, time.Date(2093, time.April, 1, 0, 0, 0, 0, time.UTC))

	mine := seedStanding(t, db, repo, this.ID, "lb-s1@s.test", 300)
	theirs := seedStanding(t, db, repo, other.ID, "lb-s2@s.test", 90000)

	entries, total, err := repo.LeaderboardPage(this.ID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, entries, 1)
	assert.Equal(t, mine.ID, entries[0].UserID)

	ahead, err := repo.CountAhead(this.ID, 300, mine.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), ahead, "the other season's leader does not sit ahead of anyone here")
	assert.NotEqual(t, mine.ID, theirs.ID)
}

// --- Review follow-ups (P1, P7, P8) ---

// insertUnplayedRow writes a player_seasons row with games_played = 0.
// ApplySeasonPoints never commits one (its zero row is updated in the same
// transaction), so it is inserted raw purely to prove the games_played >= 1
// half of the membership predicate is real and not vacuous.
func insertUnplayedRow(t *testing.T, db *gorm.DB, seasonID uint, email string) *user.User {
	t.Helper()
	u := makeUser(t, db, email)
	require.NoError(t, db.Exec(`
		INSERT INTO player_seasons (user_id, season_id, sp, rank_tier, games_played, games_completed, created_at, updated_at)
		VALUES (?, ?, 0, 'iron', 0, 0, NOW(), NOW())`, u.ID, seasonID).Error)
	return u
}

// THE LADDER IS EVERYONE WHO PLAYED (Story 13.4). A player who has lost back
// down to 0 SP is still on it, at a real position, while a row with no games
// played is not: the real-Postgres proof that `games_played >= 1` governs the
// items AND the total.
func TestLeaderboardPage_ListsPlayersAtZeroSP(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2093, time.July, 1, 0, 0, 0, 0, time.UTC))

	earner := seedStanding(t, db, repo, s.ID, "lb-z1@s.test", 700)
	floored := seedStanding(t, db, repo, s.ID, "lb-z2@s.test", -18)
	unplayed := insertUnplayedRow(t, db, s.ID, "lb-z3@s.test")

	entries, total, err := repo.LeaderboardPage(s.ID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total, "the 0-SP player counts; the unplayed row does not")
	require.Len(t, entries, 2)
	assert.Equal(t, earner.ID, entries[0].UserID)
	assert.Equal(t, floored.ID, entries[1].UserID, "0 SP after a loss is still a place on the ladder")
	assert.Equal(t, 0, entries[1].SP)
	assert.Equal(t, 1, entries[1].GamesPlayed)

	// The unplayed row exists; it is simply not ON the ladder. A visibility rule,
	// not a delete.
	row, err := repo.FindPlayerSeason(unplayed.ID, s.ID)
	require.NoError(t, err)
	require.NotNil(t, row)
	assert.Equal(t, 0, row.GamesPlayed)
}

// A season whose only rows have no games played is EMPTY.
func TestLeaderboardPage_SeasonOfOnlyUnplayedRowsIsEmpty(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2093, time.October, 1, 0, 0, 0, 0, time.UTC))

	insertUnplayedRow(t, db, s.ID, "lb-y1@s.test")
	insertUnplayedRow(t, db, s.ID, "lb-y2@s.test")

	entries, total, err := repo.LeaderboardPage(s.ID, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, entries)
	assert.Equal(t, int64(0), total)
}

// The invariant the whole story turns on still holds with 0-SP players listed
// and an unplayed row in the season: CountAhead + 1 equals each listed row's own
// slot, ties at 0 broken by user id like any other tie.
func TestLeaderboardCountAhead_AgreesWithTheListAtZeroSP(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2092, time.January, 1, 0, 0, 0, 0, time.UTC))

	// 0-SP players deliberately interleaved with the others by creation order.
	seedStanding(t, db, repo, s.ID, "lb-x0@s.test", 0)
	seedStanding(t, db, repo, s.ID, "lb-x1@s.test", 4000)
	insertUnplayedRow(t, db, s.ID, "lb-x5@s.test")
	seedStanding(t, db, repo, s.ID, "lb-x2@s.test", 0)
	seedStanding(t, db, repo, s.ID, "lb-x3@s.test", 900)
	seedStanding(t, db, repo, s.ID, "lb-x4@s.test", 900)

	entries := fullLadder(t, repo, s.ID)
	require.Len(t, entries, 5, "every player who played is listed, 0 SP included")
	assert.Zero(t, entries[3].SP)
	assert.Less(t, entries[3].UserID, entries[4].UserID, "a tie at 0 breaks by ascending user id")

	for i, e := range entries {
		ahead, err := repo.CountAhead(s.ID, e.SP, e.UserID)
		require.NoError(t, err)
		assert.Equal(t, int64(i), ahead,
			"user %d (sp %d) sits in slot %d — an unplayed row must not push anyone down",
			e.UserID, e.SP, i+1)
	}
}

// P7: the viewer block runs through the LIST'S OWN predicate, so the three
// "no standing" cases are decided in one place instead of being re-derived in
// Go — and a player at 0 SP who has played is NOT one of them.
func TestLeaderboardFindEntry_MissesTheThreeUnlistableCases(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2092, time.April, 1, 0, 0, 0, 0, time.UTC))

	never := makeUser(t, db, "lb-e1@s.test") // no player_seasons row at all
	unplayed := insertUnplayedRow(t, db, s.ID, "lb-e2@s.test")
	gone := seedStanding(t, db, repo, s.ID, "lb-e3@s.test", 8000)
	live := seedStanding(t, db, repo, s.ID, "lb-e4@s.test", 1200)
	zero := seedStanding(t, db, repo, s.ID, "lb-e5@s.test", 0)
	require.NoError(t, db.Delete(&user.User{}, gone.ID).Error)

	for name, id := range map[string]uint{
		"never played":         never.ID,
		"row with no games":    unplayed.ID,
		"soft-deleted account": gone.ID,
	} {
		got, err := repo.FindLeaderboardEntry(s.ID, id)
		require.NoError(t, err, name)
		assert.Nil(t, got, "%s must have no listable standing", name)
	}

	// The controls: a live player gets one, carrying the joined username, and so
	// does a player who has played down to 0 SP.
	got, err := repo.FindLeaderboardEntry(s.ID, live.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, live.ID, got.UserID)
	assert.Equal(t, live.Username, got.Username)
	assert.Equal(t, 1200, got.SP)
	assert.Equal(t, 1, got.GamesPlayed)

	got, err = repo.FindLeaderboardEntry(s.ID, zero.ID)
	require.NoError(t, err)
	require.NotNil(t, got, "a played 0-SP player has a standing")
	assert.Equal(t, 0, got.SP)

	// AND THE POINT OF THE WHOLE FINDING: FindPlayerSeason — the method the viewer
	// block used to call — still happily returns the soft-deleted account's row.
	// That asymmetry is why FindLeaderboardEntry exists.
	leaky, err := repo.FindPlayerSeason(gone.ID, s.ID)
	require.NoError(t, err)
	require.NotNil(t, leaky, "FindPlayerSeason has no users join, by design")
	assert.Equal(t, 8000, leaky.SP)
}

// FindLeaderboardEntry is scoped to one season like every other leaderboard read.
func TestLeaderboardFindEntry_IsScopedToOneSeason(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	this := makeSeason(t, db, time.Date(2091, time.January, 1, 0, 0, 0, 0, time.UTC))
	other := makeSeason(t, db, time.Date(2091, time.April, 1, 0, 0, 0, 0, time.UTC))

	u := seedStanding(t, db, repo, other.ID, "lb-w1@s.test", 3000)

	got, err := repo.FindLeaderboardEntry(this.ID, u.ID)
	require.NoError(t, err)
	assert.Nil(t, got, "a standing in another window is not a standing in this one")
}

// P8: the bounds check at the Go boundary. `limit` feeds both a slice
// pre-allocation (which PANICS on a negative) and SQL LIMIT (where a negative
// means "no limit" and returns the entire season) — so an unchecked argument is
// either a crash or a silent full-table read, neither attributable to the caller.
func TestLeaderboardPage_RejectsOutOfRangeArguments(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2091, time.July, 1, 0, 0, 0, 0, time.UTC))
	seedStanding(t, db, repo, s.ID, "lb-v1@s.test", 100)
	seedStanding(t, db, repo, s.ID, "lb-v2@s.test", 200)

	cases := []struct {
		name          string
		limit, offset int
	}{
		{"zero limit", 0, 0},
		{"negative limit", -1, 0},
		{"negative offset", 10, -1},
		{"both negative", -5, -5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entries, total, err := repo.LeaderboardPage(s.ID, tc.limit, tc.offset)
			require.Error(t, err, "an out-of-range argument must not reach the database")
			assert.Nil(t, entries)
			assert.Zero(t, total)
		})
	}

	// The smallest legal page still works, so the guard is not off by one.
	entries, total, err := repo.LeaderboardPage(s.ID, 1, 0)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, int64(2), total)
}

// --- Story 13.3: archive, seasons list, by-id lookup, rollover ---

// THE ARCHIVE'S MEMBERSHIP RULE, spelled out against real Postgres:
// row exists AND games_played >= 1 AND the season ENDED. One test seeds all four
// boundary cases at once so the predicate is proven as a whole, plus the
// newest-first order.
func TestPlayerSeasonArchive_MembershipAndOrder(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	u := makeUser(t, db, "ar-u1@s.test")

	// Three ended windows (2089 Q1..Q3) and the "active" one (2089 Q4) — active
	// relative to the `now` this test passes, which sits inside Q4.
	q1 := makeSeason(t, db, time.Date(2089, time.January, 1, 0, 0, 0, 0, time.UTC))
	q2 := makeSeason(t, db, time.Date(2089, time.April, 1, 0, 0, 0, 0, time.UTC))
	q3 := makeSeason(t, db, time.Date(2089, time.July, 1, 0, 0, 0, 0, time.UTC))
	q4 := makeSeason(t, db, time.Date(2089, time.October, 1, 0, 0, 0, 0, time.UTC))
	now := time.Date(2089, time.November, 15, 12, 0, 0, 0, time.UTC)

	// Q1: played, earned SP — the ordinary archive row.
	_, err := applyAwards(repo, q1.ID, map[uint]fixedAward{u.ID: {change: 1800, completed: true}})
	require.NoError(t, err)
	// Q2: played but ended on 0 SP (a floored loss) — MUST be included; the
	// archive is "seasons you actually played".
	_, err = applyAwards(repo, q2.ID, map[uint]fixedAward{u.ID: {change: -30, completed: false}})
	require.NoError(t, err)
	// Q3: a row with games_played = 0. ApplySeasonPoints can never write one
	// (it always counts the game), so it is inserted raw purely to prove the
	// games_played >= 1 half of the predicate is real and not vacuous.
	require.NoError(t, db.Exec(`
		INSERT INTO player_seasons (user_id, season_id, sp, rank_tier, games_played, games_completed, created_at, updated_at)
		VALUES (?, ?, 0, 'iron', 0, 0, NOW(), NOW())`, u.ID, q3.ID).Error)
	// Q4: played in the ACTIVE window — excluded, its record is still moving.
	_, err = applyAwards(repo, q4.ID, map[uint]fixedAward{u.ID: {change: 500, completed: true}})
	require.NoError(t, err)

	entries, err := repo.PlayerSeasonArchive(u.ID, now)
	require.NoError(t, err)
	require.Len(t, entries, 2, "Q1 (earned) and Q2 (played, 0 SP) only")

	// Newest-first: Q2 (Apr) before Q1 (Jan).
	assert.Equal(t, q2.ID, entries[0].SeasonID)
	assert.Equal(t, "2089 Q2", entries[0].SeasonName)
	assert.Equal(t, 0, entries[0].SP, "the 0-SP played season is archive history")
	assert.Equal(t, 1, entries[0].GamesPlayed)
	assert.True(t, q2.StartedAt.UTC().Equal(entries[0].StartedAt.UTC()))
	assert.True(t, q2.EndsAt.UTC().Equal(entries[0].EndsAt.UTC()))

	assert.Equal(t, q1.ID, entries[1].SeasonID)
	assert.Equal(t, 1800, entries[1].SP, "the prior row is read back unchanged")
	// The stored rank snapshot rides along (Story 13.5): what the award wrote.
	assert.Equal(t, "grandmaster", entries[1].RankTier)
	assert.Nil(t, entries[1].RankDivision)
	assert.Equal(t, "iron", entries[0].RankTier)
	assert.Equal(t, intPtr(1), entries[0].RankDivision)
}

// The exact boundary: a season whose ends_at IS now has ended (ends_at is
// exclusive on the window, so the instant it ends it is history).
func TestPlayerSeasonArchive_EndsAtBoundaryIsInclusive(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	u := makeUser(t, db, "ar-u2@s.test")
	s := makeSeason(t, db, time.Date(2088, time.January, 1, 0, 0, 0, 0, time.UTC))

	_, err := applyAwards(repo, s.ID, map[uint]fixedAward{u.ID: {change: 100, completed: true}})
	require.NoError(t, err)

	entries, err := repo.PlayerSeasonArchive(u.ID, s.EndsAt)
	require.NoError(t, err)
	require.Len(t, entries, 1, "at the exact end instant the season is already archived")

	before, err := repo.PlayerSeasonArchive(u.ID, s.EndsAt.Add(-time.Second))
	require.NoError(t, err)
	assert.Empty(t, before, "a second earlier it is still the active window")
}

// An unknown user is an EMPTY archive with a 200-shaped answer — a non-nil
// empty slice, never an error and never a nil that serializes as null. The
// profile query owns user-existence 404s.
func TestPlayerSeasonArchive_UnknownUserIsEmpty(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)

	entries, err := repo.PlayerSeasonArchive(99_999_999, time.Date(2088, time.July, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.NotNil(t, entries)
	assert.Empty(t, entries)
}

// A SOFT-DELETED SUBJECT HAS NO READABLE HISTORY. Same reasoning as
// TestLeaderboardPage_ExcludesSoftDeletedUsers, and the same hand-written
// `users.deleted_at IS NULL`: without the users join this endpoint would serve a
// deleted account's whole season history to any authenticated caller while the
// ladder scrubs the same user. The answer must be an EMPTY archive — a 200 with
// no items, indistinguishable from an unknown id — never a 404, which is the
// profile query's job, not this endpoint's.
func TestPlayerSeasonArchive_ExcludesSoftDeletedUser(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	gone := makeUser(t, db, "ar-del@s.test")
	s := makeSeason(t, db, time.Date(2085, time.January, 1, 0, 0, 0, 0, time.UTC))
	now := time.Date(2085, time.June, 1, 0, 0, 0, 0, time.UTC)

	_, err := applyAwards(repo, s.ID, map[uint]fixedAward{
		gone.ID: {change: 700, completed: true},
	})
	require.NoError(t, err)

	// Present before the delete, so the test proves the FILTER did the work.
	entries, err := repo.PlayerSeasonArchive(gone.ID, now)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	require.NoError(t, db.Delete(&user.User{}, gone.ID).Error)

	entries, err = repo.PlayerSeasonArchive(gone.ID, now)
	require.NoError(t, err)
	assert.NotNil(t, entries)
	assert.Empty(t, entries, "a deleted account's season history is not readable")

	// Visibility filter, not a cascade: the row itself survives.
	row, err := repo.FindPlayerSeason(gone.ID, s.ID)
	require.NoError(t, err)
	require.NotNil(t, row)
}

// The archive is per-player: another player's rows in the same windows never
// leak into this player's history.
func TestPlayerSeasonArchive_IsScopedToOneUser(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	mine := makeUser(t, db, "ar-u3@s.test")
	theirs := makeUser(t, db, "ar-u4@s.test")
	s := makeSeason(t, db, time.Date(2087, time.January, 1, 0, 0, 0, 0, time.UTC))
	now := time.Date(2087, time.June, 1, 0, 0, 0, 0, time.UTC)

	_, err := applyAwards(repo, s.ID, map[uint]fixedAward{
		mine.ID:   {change: 100, completed: true},
		theirs.ID: {change: 90000, completed: true},
	})
	require.NoError(t, err)

	entries, err := repo.PlayerSeasonArchive(mine.ID, now)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, 100, entries[0].SP, "my row, not the other player's")
}

// ListSeasons: newest-first by started_at, every window included (the picker
// renders this order verbatim).
func TestListSeasons_NewestFirst(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)

	older := makeSeason(t, db, time.Date(2086, time.January, 1, 0, 0, 0, 0, time.UTC))
	newer := makeSeason(t, db, time.Date(2086, time.April, 1, 0, 0, 0, 0, time.UTC))
	newest := makeSeason(t, db, time.Date(2086, time.July, 1, 0, 0, 0, 0, time.UTC))

	seasons, err := repo.ListSeasons()
	require.NoError(t, err)
	// The migration seed (and other tests' windows inside this transaction) may
	// add rows; assert the RELATIVE order of the three this test owns.
	pos := map[uint]int{}
	for i, s := range seasons {
		pos[s.ID] = i
	}
	require.Contains(t, pos, older.ID)
	require.Contains(t, pos, newer.ID)
	require.Contains(t, pos, newest.ID)
	assert.Less(t, pos[newest.ID], pos[newer.ID], "started_at DESC")
	assert.Less(t, pos[newer.ID], pos[older.ID], "started_at DESC")
}

func TestFindSeasonByID_HitAndMiss(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2085, time.January, 1, 0, 0, 0, 0, time.UTC))

	got, err := repo.FindSeasonByID(s.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, s.Name, got.Name)

	missing, err := repo.FindSeasonByID(s.ID + 10_000_000)
	require.NoError(t, err)
	assert.Nil(t, missing, "a miss is (nil, nil), mapped to 404 by the service — never an error here")
}

// THE ROLLOVER JOB'S WHOLE CONTRACT against real Postgres: past a boundary one
// pass creates exactly one quarter row, and a second pass changes nothing —
// the uq_seasons_started_at conflict target is the idempotency anchor.
func TestRollover_RunOnceIsIdempotent(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)

	// A fixed clock in a quarter no other test creates.
	now := time.Date(2084, time.August, 10, 3, 0, 0, 0, time.UTC)
	job := season.NewRollover(repo, 0, func() time.Time { return now })

	require.NoError(t, job.RunOnce())
	require.NoError(t, job.RunOnce(), "the rerun is a no-op, not an error")

	var count int64
	require.NoError(t, db.Model(&season.Season{}).
		Where("started_at = ?", time.Date(2084, time.July, 1, 0, 0, 0, 0, time.UTC)).
		Count(&count).Error)
	assert.Equal(t, int64(1), count, "two runs, one row")
}
