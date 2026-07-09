package match_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/emilijan/beljot/server/internal/match"
)

// getRepoTestDB opens a per-test transaction against the dev DB (BELJOT_DB_URL,
// default port 5433) and rolls it back on cleanup, so tests never leak rows.
func getRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("BELJOT_DB_URL")
	if dsn == "" {
		dsn = "postgres://beljot:beljot_dev_password@localhost:5433/beljot?sslmode=disable"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skip("skipping integration test: database not available")
	}

	tx := db.Begin()
	require.NoError(t, tx.Error, "begin test transaction")
	t.Cleanup(func() {
		tx.Rollback()
	})

	return tx
}

// repoFixtureSuffix returns a per-call unique 8-digit suffix for fixture
// usernames / emails / room codes. Uniqueness matters even though the test
// transaction rolls back: partial unique indexes on users/rooms still check
// the seeded values against COMMITTED dev-DB rows, so fixed names could
// collide with real local data.
func repoFixtureSuffix() string {
	return fmt.Sprintf("%08d", time.Now().UnixNano()%1e8)
}

// seedRepoUser inserts a users row via raw SQL (the match package must not
// import user — user already imports match) and returns its ID. The tag must
// be unique per run (build it from repoFixtureSuffix) and fit the
// VARCHAR(20) username column.
func seedRepoUser(t *testing.T, db *gorm.DB, tag string) uint {
	t.Helper()
	var id uint
	require.NoError(t, db.Raw(`
INSERT INTO users (email, username, password_hash, language_preference)
VALUES (?, ?, 'x', 'en')
RETURNING id`, tag+"@repo.test", tag).Scan(&id).Error)
	return id
}

// seedRepoRoom inserts the rooms row the matches FK requires. Name and the
// VARCHAR(6) code derive from the run-unique suffix so they cannot collide
// with committed dev-DB rows (see repoFixtureSuffix).
func seedRepoRoom(t *testing.T, db *gorm.DB, ownerID uint, suffix string) uint {
	t.Helper()
	var id uint
	require.NoError(t, db.Raw(`
INSERT INTO rooms (name, code, owner_id, status)
VALUES (?, ?, ?, 'completed')
RETURNING id`,
		"repo-test-"+suffix,
		"T"+suffix[len(suffix)-5:],
		ownerID,
	).Scan(&id).Error)
	return id
}

// abandonedFixture seeds the four users + room + the match mix that exercises
// the per-player abandonment matrix. Seats are [a, b, c, d] in every match, so
// a/c are team 0 and b/d team 1. Completed times descend from base so the
// newest-first list order is deterministic (m1 newest .. m4 oldest).
//
//	m1: completed, winner 0                              — a/c win, b/d loss
//	m2: abandoned by a (team 0), winner_team 1           — a abandoned, c loss, b/d win
//	m3: abandoned, NULL abandoner, filler winner_team 0  — abandoned for everyone
//	m4: completed via surrender by b, winner 0           — a/c win, b/d loss
type abandonedFixture struct {
	repo       *match.GormMatchRepository
	a, b, c, d uint
	m1, m2     uint
	m3, m4     uint
}

func seedAbandonedFixture(t *testing.T, db *gorm.DB) abandonedFixture {
	t.Helper()

	f := abandonedFixture{repo: match.NewGormMatchRepository(db)}
	// "rt" + 8-digit suffix + seat letter = 11 chars, inside VARCHAR(20).
	suffix := repoFixtureSuffix()
	f.a = seedRepoUser(t, db, "rt"+suffix+"a")
	f.b = seedRepoUser(t, db, "rt"+suffix+"b")
	f.c = seedRepoUser(t, db, "rt"+suffix+"c")
	f.d = seedRepoUser(t, db, "rt"+suffix+"d")
	roomID := seedRepoRoom(t, db, f.a, suffix)

	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	newMatch := func(offset time.Duration, status string, winnerTeam int, abandonedBy, surrenderedBy *uint) *match.Match {
		aID, bID, cID, dID := f.a, f.b, f.c, f.d
		return &match.Match{
			RoomID:        roomID,
			Player1ID:     &aID,
			Player2ID:     &bID,
			Player3ID:     &cID,
			Player4ID:     &dID,
			TeamAScore:    500,
			TeamBScore:    400,
			WinnerTeam:    winnerTeam,
			Variant:       "bitola",
			MatchMode:     "1001",
			StartedAt:     base.Add(offset - 20*time.Minute),
			CompletedAt:   base.Add(offset),
			Status:        status,
			AbandonedBy:   abandonedBy,
			SurrenderedBy: surrenderedBy,
		}
	}

	m1 := newMatch(0, "completed", 0, nil, nil)
	m2 := newMatch(-1*time.Hour, "abandoned", 1, &f.a, nil)
	m3 := newMatch(-2*time.Hour, "abandoned", 0, nil, nil)
	m4 := newMatch(-3*time.Hour, "completed", 0, nil, &f.b)
	for _, m := range []*match.Match{m1, m2, m3, m4} {
		require.NoError(t, f.repo.Create(m))
	}
	f.m1, f.m2, f.m3, f.m4 = m1.ID, m2.ID, m3.ID, m4.ID
	return f
}

// TestGormMatchRepository_GetStatsForUser_PerPlayerAbandonment pins the
// per-player outcome semantics on abandoned rows: the abandoner (and every
// participant of a NULL-abandoner row) counts "abandoned"; the partner counts
// a loss and the opponents a win via the persisted winner_team. Surrendered
// matches stay plain completed win/loss.
func TestGormMatchRepository_GetStatsForUser_PerPlayerAbandonment(t *testing.T) {
	db := getRepoTestDB(t)
	f := seedAbandonedFixture(t, db)

	cases := []struct {
		name                    string
		viewer                  uint
		wins, losses, abandoned int
	}{
		// a: m1 win, m2 own abandonment, m3 legacy, m4 win.
		{name: "abandoner", viewer: f.a, wins: 2, losses: 0, abandoned: 2},
		// b: m1 loss, m2 opponent of abandoner -> win, m3 legacy, m4 loss.
		{name: "opponent of abandoner", viewer: f.b, wins: 1, losses: 2, abandoned: 1},
		// c: m1 win, m2 partner of abandoner -> loss, m3 legacy, m4 win.
		{name: "partner of abandoner", viewer: f.c, wins: 2, losses: 1, abandoned: 1},
		// d: mirrors b (same team).
		{name: "second opponent", viewer: f.d, wins: 1, losses: 2, abandoned: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wins, losses, abandoned, err := f.repo.GetStatsForUser(tc.viewer)
			require.NoError(t, err)
			assert.Equal(t, tc.wins, wins, "wins")
			assert.Equal(t, tc.losses, losses, "losses")
			assert.Equal(t, tc.abandoned, abandoned, "abandoned")
		})
	}
}

// TestGormMatchRepository_GetMatchesForUser_OutcomeFilterPerPlayer pins the
// outcome filter's mirror of the stats semantics: win/loss include the
// attributable abandoned rows for non-abandoners, "abandoned" keeps only the
// viewer's own abandonments plus NULL-abandoner legacy rows.
func TestGormMatchRepository_GetMatchesForUser_OutcomeFilterPerPlayer(t *testing.T) {
	db := getRepoTestDB(t)
	f := seedAbandonedFixture(t, db)

	cases := []struct {
		name    string
		viewer  uint
		outcome string
		wantIDs []uint // newest-first (completed_at DESC)
	}{
		{name: "abandoner win", viewer: f.a, outcome: "win", wantIDs: []uint{f.m1, f.m4}},
		{name: "abandoner loss", viewer: f.a, outcome: "loss", wantIDs: []uint{}},
		{name: "abandoner abandoned", viewer: f.a, outcome: "abandoned", wantIDs: []uint{f.m2, f.m3}},
		{name: "abandoner all", viewer: f.a, outcome: "", wantIDs: []uint{f.m1, f.m2, f.m3, f.m4}},
		{name: "opponent win includes abandonment", viewer: f.b, outcome: "win", wantIDs: []uint{f.m2}},
		{name: "opponent loss", viewer: f.b, outcome: "loss", wantIDs: []uint{f.m1, f.m4}},
		{name: "opponent abandoned keeps legacy only", viewer: f.b, outcome: "abandoned", wantIDs: []uint{f.m3}},
		{name: "partner loss includes abandonment", viewer: f.c, outcome: "loss", wantIDs: []uint{f.m2}},
		{name: "partner win", viewer: f.c, outcome: "win", wantIDs: []uint{f.m1, f.m4}},
		{name: "partner abandoned keeps legacy only", viewer: f.c, outcome: "abandoned", wantIDs: []uint{f.m3}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items, total, err := f.repo.GetMatchesForUser(tc.viewer, 50, 0, tc.outcome, "new")
			require.NoError(t, err)
			assert.Equal(t, int64(len(tc.wantIDs)), total, "total")
			gotIDs := make([]uint, 0, len(items))
			for _, m := range items {
				gotIDs = append(gotIDs, m.ID)
			}
			assert.Equal(t, tc.wantIDs, gotIDs, "match IDs")
		})
	}
}

// TestGormMatchRepository_Migration15Semantics replays the migration 000015
// backfill expression against seeded pre-backfill rows (filler winner_team 0)
// and verifies attributable abandoned rows flip to the team opposite the
// abandoner while NULL-abandoner rows keep the filler.
func TestGormMatchRepository_Migration15Semantics(t *testing.T) {
	db := getRepoTestDB(t)
	f := seedAbandonedFixture(t, db)

	// Reset the abandoned rows to the historical filler, as pre-000015 data.
	require.NoError(t, db.Exec(
		`UPDATE matches SET winner_team = 0 WHERE id IN (?, ?)`, f.m2, f.m3,
	).Error)

	// The 000015 up expression with the same status + abandoned_by gate the
	// migration uses, additionally scoped to this fixture's match IDs so the
	// replay can never touch (or lock) other abandoned rows that may exist in
	// a shared dev DB.
	require.NoError(t, db.Exec(`
UPDATE matches
SET winner_team = CASE
    WHEN abandoned_by = player1_id OR abandoned_by = player3_id THEN 1
    ELSE 0
END
WHERE status = 'abandoned'
  AND abandoned_by IS NOT NULL
  AND id IN (?, ?)`, f.m2, f.m3).Error)

	var winner int
	require.NoError(t, db.Raw(`SELECT winner_team FROM matches WHERE id = ?`, f.m2).Scan(&winner).Error)
	assert.Equal(t, 1, winner, "abandoner on team 0 -> winner_team backfilled to 1")

	require.NoError(t, db.Raw(`SELECT winner_team FROM matches WHERE id = ?`, f.m3).Scan(&winner).Error)
	assert.Equal(t, 0, winner, "NULL-abandoner row keeps the filler 0")
}
