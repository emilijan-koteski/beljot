// package season_test, NOT season, and the distinction is MANDATORY: this file
// imports `user` for its fixtures, and Story 13.3 opened the `user` -> `season`
// edge (the profile's SeasonRankReader). An in-package test importing `user`
// would close the cycle season(test) -> user -> season and stop compiling.
package season_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

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

// AC1/AC4: the first award INSERTS, and every counter starts from the right base.
func TestApplySeasonPoints_FirstAwardInserts(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2097, time.April, 1, 0, 0, 0, 0, time.UTC))
	winner := makeUser(t, db, "sp-w@s.test")
	absent := makeUser(t, db, "sp-a@s.test")

	snaps, err := repo.ApplySeasonPoints(s.ID, map[uint]season.SPAward{
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
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2097, time.July, 1, 0, 0, 0, 0, time.UTC))
	u := makeUser(t, db, "sp-acc@s.test")

	_, err := repo.ApplySeasonPoints(s.ID, map[uint]season.SPAward{u.ID: {SP: 400, Completed: true}})
	require.NoError(t, err)

	snaps, err := repo.ApplySeasonPoints(s.ID, map[uint]season.SPAward{u.ID: {SP: 150, Completed: true}})
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
	assert.Equal(t, season.TierForSP(row.SP), row.RankTier, "stored and derived must agree")
}

// A player's rows are per-season: a second window starts them from zero (the
// "soft reset" is a new season_id, never an update of the old row).
func TestApplySeasonPoints_SeasonsAreIndependent(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	first := makeSeason(t, db, time.Date(2096, time.January, 1, 0, 0, 0, 0, time.UTC))
	second := makeSeason(t, db, time.Date(2096, time.April, 1, 0, 0, 0, 0, time.UTC))
	u := makeUser(t, db, "sp-two@s.test")

	_, err := repo.ApplySeasonPoints(first.ID, map[uint]season.SPAward{u.ID: {SP: 2000, Completed: true}})
	require.NoError(t, err)
	snaps, err := repo.ApplySeasonPoints(second.ID, map[uint]season.SPAward{u.ID: {SP: 100, Completed: true}})
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
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2096, time.July, 1, 0, 0, 0, 0, time.UTC))

	snaps, err := repo.ApplySeasonPoints(s.ID, nil)
	require.NoError(t, err)
	assert.Empty(t, snaps)
}

// All-or-nothing: an unknown user violates the FK, and the whole batch rolls
// back rather than half-crediting the table.
func TestApplySeasonPoints_UnknownUserRollsBackTheBatch(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2095, time.October, 1, 0, 0, 0, 0, time.UTC))
	good := makeUser(t, db, "sp-good@s.test")

	// A deliberately unassigned id, ordered AFTER the real one so the good write
	// has already happened inside the transaction when the bad one fails.
	_, err := repo.ApplySeasonPoints(s.ID, map[uint]season.SPAward{
		good.ID:              {SP: 200, Completed: true},
		good.ID + 10_000_000: {SP: 200, Completed: true},
	})
	require.Error(t, err)

	row, err := repo.FindPlayerSeason(good.ID, s.ID)
	require.NoError(t, err)
	assert.Nil(t, row, "the successful seat's write must have rolled back with the batch")
}

// --- Story 13.2: leaderboard reads ---

// seedStanding creates a user and drops them into the season with a fixed SP
// total, going through ApplySeasonPoints so the rows are written exactly the way
// the match-end path writes them.
func seedStanding(t *testing.T, db *gorm.DB, repo *season.GormRepository, seasonID uint, email string, sp int) *user.User {
	t.Helper()
	u := makeUser(t, db, email)
	_, err := repo.ApplySeasonPoints(seasonID, map[uint]season.SPAward{u.ID: {SP: sp, Completed: true}})
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

// P1: THE LADDER IS SP EARNERS ONLY (owner decision 2026-08-27). A 0-SP row is
// what ApplySeasonPoints writes for a seat that was absent at the terminal end,
// so these rows are ordinary, not corruption — and this is the real-Postgres
// proof that `player_seasons.sp > 0` keeps them out of the items AND the total.
func TestLeaderboardPage_ExcludesZeroSPRows(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2093, time.July, 1, 0, 0, 0, 0, time.UTC))

	earner := seedStanding(t, db, repo, s.ID, "lb-z1@s.test", 700)
	absentee := seedStanding(t, db, repo, s.ID, "lb-z2@s.test", 0)

	entries, total, err := repo.LeaderboardPage(s.ID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total, "the 0-SP row is excluded from the total too")
	require.Len(t, entries, 1)
	assert.Equal(t, earner.ID, entries[0].UserID)

	// The row exists and still counts a game played — it is simply not ON the
	// ladder. This is a VISIBILITY rule, not a delete.
	row, err := repo.FindPlayerSeason(absentee.ID, s.ID)
	require.NoError(t, err)
	require.NotNil(t, row)
	assert.Equal(t, 0, row.SP)
	assert.Equal(t, 1, row.GamesPlayed)
}

// A season whose only rows are 0-SP is EMPTY, which is what makes the client's
// "Nobody has earned Season Points yet" copy true rather than a lie.
func TestLeaderboardPage_SeasonOfOnlyZeroSPRowsIsEmpty(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2093, time.October, 1, 0, 0, 0, 0, time.UTC))

	seedStanding(t, db, repo, s.ID, "lb-y1@s.test", 0)
	seedStanding(t, db, repo, s.ID, "lb-y2@s.test", 0)

	entries, total, err := repo.LeaderboardPage(s.ID, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, entries)
	assert.Equal(t, int64(0), total)
}

// The exclusion must not break the invariant the whole story turns on: with 0-SP
// rows in the season, CountAhead + 1 still equals each listed row's own slot.
func TestLeaderboardCountAhead_UnaffectedByZeroSPRows(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2092, time.January, 1, 0, 0, 0, 0, time.UTC))

	// Two 0-SP rows deliberately interleaved with the earners by creation order.
	seedStanding(t, db, repo, s.ID, "lb-x0@s.test", 0)
	seedStanding(t, db, repo, s.ID, "lb-x1@s.test", 4000)
	seedStanding(t, db, repo, s.ID, "lb-x2@s.test", 0)
	seedStanding(t, db, repo, s.ID, "lb-x3@s.test", 900)
	seedStanding(t, db, repo, s.ID, "lb-x4@s.test", 900)

	entries := fullLadder(t, repo, s.ID)
	require.Len(t, entries, 3, "only the SP earners are listed")

	for i, e := range entries {
		ahead, err := repo.CountAhead(s.ID, e.SP, e.UserID)
		require.NoError(t, err)
		assert.Equal(t, int64(i), ahead,
			"user %d (sp %d) sits in slot %d — a 0-SP row must not push anyone down",
			e.UserID, e.SP, i+1)
	}
}

// P7: the viewer block now runs through the LIST'S OWN predicate, so the three
// "no standing" cases are decided in one place instead of being re-derived in Go.
func TestLeaderboardFindEntry_MissesTheThreeUnlistableCases(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	s := makeSeason(t, db, time.Date(2092, time.April, 1, 0, 0, 0, 0, time.UTC))

	never := makeUser(t, db, "lb-e1@s.test") // no player_seasons row at all
	zero := seedStanding(t, db, repo, s.ID, "lb-e2@s.test", 0)
	gone := seedStanding(t, db, repo, s.ID, "lb-e3@s.test", 8000)
	live := seedStanding(t, db, repo, s.ID, "lb-e4@s.test", 1200)
	require.NoError(t, db.Delete(&user.User{}, gone.ID).Error)

	for name, id := range map[string]uint{
		"never played":         never.ID,
		"played but 0 SP":      zero.ID,
		"soft-deleted account": gone.ID,
	} {
		got, err := repo.FindLeaderboardEntry(s.ID, id)
		require.NoError(t, err, name)
		assert.Nil(t, got, "%s must have no listable standing", name)
	}

	// The control: a live earner does get one, and it carries the joined username.
	got, err := repo.FindLeaderboardEntry(s.ID, live.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, live.ID, got.UserID)
	assert.Equal(t, live.Username, got.Username)
	assert.Equal(t, 1200, got.SP)
	assert.Equal(t, 1, got.GamesPlayed)

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
// row exists AND games_played >= 1 AND the season ENDED — deliberately NOT
// leaderboardScope's sp > 0. One test seeds all four boundary cases at once so
// the predicate is proven as a whole, plus the newest-first order.
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
	_, err := repo.ApplySeasonPoints(q1.ID, map[uint]season.SPAward{u.ID: {SP: 1800, Completed: true}})
	require.NoError(t, err)
	// Q2: played but 0 SP (an absent seat) — MUST be included; the archive is
	// "seasons you actually played", not "SP earners".
	_, err = repo.ApplySeasonPoints(q2.ID, map[uint]season.SPAward{u.ID: {SP: 0, Completed: false}})
	require.NoError(t, err)
	// Q3: a row with games_played = 0. ApplySeasonPoints can never write one
	// (it always counts the game), so it is inserted raw purely to prove the
	// games_played >= 1 half of the predicate is real and not vacuous.
	require.NoError(t, db.Exec(`
		INSERT INTO player_seasons (user_id, season_id, sp, rank_tier, games_played, games_completed, created_at, updated_at)
		VALUES (?, ?, 0, 'iron', 0, 0, NOW(), NOW())`, u.ID, q3.ID).Error)
	// Q4: played in the ACTIVE window — excluded, its record is still moving.
	_, err = repo.ApplySeasonPoints(q4.ID, map[uint]season.SPAward{u.ID: {SP: 500, Completed: true}})
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
}

// The exact boundary: a season whose ends_at IS now has ended (ends_at is
// exclusive on the window, so the instant it ends it is history).
func TestPlayerSeasonArchive_EndsAtBoundaryIsInclusive(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)
	u := makeUser(t, db, "ar-u2@s.test")
	s := makeSeason(t, db, time.Date(2088, time.January, 1, 0, 0, 0, 0, time.UTC))

	_, err := repo.ApplySeasonPoints(s.ID, map[uint]season.SPAward{u.ID: {SP: 100, Completed: true}})
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

	_, err := repo.ApplySeasonPoints(s.ID, map[uint]season.SPAward{
		gone.ID: {SP: 700, Completed: true},
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

	_, err := repo.ApplySeasonPoints(s.ID, map[uint]season.SPAward{
		mine.ID:   {SP: 100, Completed: true},
		theirs.ID: {SP: 90000, Completed: true},
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
