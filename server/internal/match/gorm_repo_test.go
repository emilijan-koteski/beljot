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

// TestGormMatchRepository_GetCareerAggregatesForUser_StreakPerPlayerAbandonment
// pins the streak's mirror of the stats semantics: attributable abandoned rows
// count win/loss via winner_team, while the viewer's own abandonments and
// NULL-abandoner legacy rows are skipped (the run continues across them).
func TestGormMatchRepository_GetCareerAggregatesForUser_StreakPerPlayerAbandonment(t *testing.T) {
	db := getRepoTestDB(t)
	f := seedAbandonedFixture(t, db)

	cases := []struct {
		name   string
		viewer uint
		kind   string
		length int
	}{
		// a (newest first): m1 win, m2 own abandonment (skipped), m3 legacy
		// (skipped), m4 win — the run continues across the skipped rows.
		{name: "abandoner skips own rows", viewer: f.a, kind: "win", length: 2},
		// b: m1 loss breaks at the m2 abandonment win.
		{name: "opponent of abandoner", viewer: f.b, kind: "loss", length: 1},
		// c: m1 win breaks at m2 — the partner-of-abandoner LOSS now counts
		// (completed-only would have run m1+m4 for a win streak of 2).
		{name: "partner loss breaks streak", viewer: f.c, kind: "win", length: 1},
		// d: mirrors b.
		{name: "second opponent", viewer: f.d, kind: "loss", length: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			agg, err := f.repo.GetCareerAggregatesForUser(tc.viewer)
			require.NoError(t, err)
			assert.Equal(t, tc.kind, agg.StreakKind, "streak kind")
			assert.Equal(t, tc.length, agg.StreakLength, "streak length")
		})
	}
}

// TestGormMatchRepository_GetCareerAggregatesForUser_StreakCountsAbandonmentWin
// reproduces the reported profile bug: a completed loss followed by a newer win
// via opponent abandonment must read as a win streak of 1, not resurface the
// older loss as "loss streak 1" (which is what the completed-only filter did).
func TestGormMatchRepository_GetCareerAggregatesForUser_StreakCountsAbandonmentWin(t *testing.T) {
	db := getRepoTestDB(t)
	repo := match.NewGormMatchRepository(db)

	suffix := repoFixtureSuffix()
	w := seedRepoUser(t, db, "rs"+suffix+"w")
	x := seedRepoUser(t, db, "rs"+suffix+"x")
	y := seedRepoUser(t, db, "rs"+suffix+"y")
	z := seedRepoUser(t, db, "rs"+suffix+"z")
	roomID := seedRepoRoom(t, db, w, suffix)

	base := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	newMatch := func(offset time.Duration, status string, winnerTeam int, abandonedBy *uint) *match.Match {
		wID, xID, yID, zID := w, x, y, z
		return &match.Match{
			RoomID:      roomID,
			Player1ID:   &wID,
			Player2ID:   &xID,
			Player3ID:   &yID,
			Player4ID:   &zID,
			TeamAScore:  500,
			TeamBScore:  400,
			WinnerTeam:  winnerTeam,
			Variant:     "bitola",
			MatchMode:   "1001",
			StartedAt:   base.Add(offset - 20*time.Minute),
			CompletedAt: base.Add(offset),
			Status:      status,
			AbandonedBy: abandonedBy,
		}
	}

	// Older: completed loss for w (team 0). Newest: opponent x abandons,
	// winner_team 0 — an abandonment-derived win for w.
	older := newMatch(-1*time.Hour, "completed", 1, nil)
	newest := newMatch(0, "abandoned", 0, &x)
	require.NoError(t, repo.Create(older))
	require.NoError(t, repo.Create(newest))

	agg, err := repo.GetCareerAggregatesForUser(w)
	require.NoError(t, err)
	assert.Equal(t, "win", agg.StreakKind, "abandonment win must lead the streak")
	assert.Equal(t, 1, agg.StreakLength, "streak length")
}

// TestGormMatchRepository_GetCareerPointsForUser pins the Story 11.3 lifetime
// "career points" aggregate: the subject's OWN team score summed across
// COMPLETED matches only. In the shared fixture the two completed rows (m1
// natural, m4 surrender) each score 500-400 and the two abandoned rows (m2, m3)
// are excluded — so team-A player `a` totals 1000 (team_a_score 500+500) and
// team-B player `b` totals 800 (team_b_score 400+400). A user with no completed
// matches totals 0.
func TestGormMatchRepository_GetCareerPointsForUser(t *testing.T) {
	db := getRepoTestDB(t)
	f := seedAbandonedFixture(t, db)

	t.Run("team A player sums team_a_score over completed matches", func(t *testing.T) {
		pts, err := f.repo.GetCareerPointsForUser(f.a)
		require.NoError(t, err)
		assert.Equal(t, int64(1000), pts, "m1 + m4 team_a_score; abandoned m2/m3 excluded")
	})

	t.Run("team B player sums team_b_score", func(t *testing.T) {
		pts, err := f.repo.GetCareerPointsForUser(f.b)
		require.NoError(t, err)
		assert.Equal(t, int64(800), pts, "m1 + m4 team_b_score")
	})

	t.Run("no completed matches totals zero", func(t *testing.T) {
		lonely := seedRepoUser(t, db, "cp"+repoFixtureSuffix()+"z")
		pts, err := f.repo.GetCareerPointsForUser(lonely)
		require.NoError(t, err)
		assert.Equal(t, int64(0), pts)
	})
}

// TestGormMatchRepository_TopPartnersAndRivals_PerPlayerAbandonment pins the
// partner/rival mirrors of the stats semantics: attributable abandoned rows
// count viewer-relative wins/losses; the viewer's own abandonments and
// NULL-abandoner rows contribute no win/loss (partners still count them as
// played; rivals exclude them entirely).
func TestGormMatchRepository_TopPartnersAndRivals_PerPlayerAbandonment(t *testing.T) {
	db := getRepoTestDB(t)
	f := seedAbandonedFixture(t, db)

	t.Run("partner wins include abandonment win", func(t *testing.T) {
		// b + d: played all 4; the only win is m2 via a's abandonment.
		rows, err := f.repo.GetTopPartnersForUser(f.b, 5)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, f.d, rows[0].UserID, "teammate")
		assert.Equal(t, 4, rows[0].Played, "played")
		assert.Equal(t, 1, rows[0].Wins, "wins")
	})

	t.Run("partner wins exclude viewer's own abandonment", func(t *testing.T) {
		// a + c: m1 and m4 wins; m2 is a's own abandonment (no win), m3 legacy.
		rows, err := f.repo.GetTopPartnersForUser(f.a, 5)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, f.c, rows[0].UserID, "teammate")
		assert.Equal(t, 4, rows[0].Played, "played")
		assert.Equal(t, 2, rows[0].Wins, "wins")
	})

	t.Run("rival record includes abandonment outcomes", func(t *testing.T) {
		// b vs a and c: m1 loss, m2 abandonment win, m4 loss; m3 excluded.
		rows, err := f.repo.GetTopRivalsForUser(f.b, 5)
		require.NoError(t, err)
		require.Len(t, rows, 2)
		byID := map[uint]match.RivalAggregate{}
		for _, r := range rows {
			byID[r.UserID] = r
		}
		for _, opp := range []uint{f.a, f.c} {
			require.Contains(t, byID, opp, "opponent row")
			assert.Equal(t, 1, byID[opp].Wins, "wins vs opponent")
			assert.Equal(t, 2, byID[opp].Losses, "losses vs opponent")
		}
	})

	t.Run("rival record excludes viewer's own abandonment", func(t *testing.T) {
		// a vs b and d: m1 + m4 wins; m2 (own abandonment) and m3 excluded.
		rows, err := f.repo.GetTopRivalsForUser(f.a, 5)
		require.NoError(t, err)
		require.Len(t, rows, 2)
		for _, r := range rows {
			assert.Equal(t, 2, r.Wins, "wins vs opponent")
			assert.Equal(t, 0, r.Losses, "losses vs opponent")
		}
	})
}

// TestGormMatchRepository_GetHonorTrendWindowsForUser pins the Story 9.7 trend
// windows against the shared abandonment fixture. The canonical viewer gate
// applies: your OWN abandonment counts against you, someone ELSE's abandonment
// counts as a completion for you (you stayed), and a NULL-abandoner
// boot-reconcile row is excluded entirely rather than consuming a window slot.
//
// The two-window split (code review 2026-07-29) is done by ROW_NUMBER inside one
// bounded 2*limit fetch, so it is also asserting that rn <= limit and rn > limit
// partition the same ordering the single-window version used.
func TestGormMatchRepository_GetHonorTrendWindowsForUser(t *testing.T) {
	db := getRepoTestDB(t)
	f := seedAbandonedFixture(t, db)

	t.Run("a window wider than the history puts everything in recent", func(t *testing.T) {
		cases := []struct {
			name                 string
			viewer               uint
			completed, abandoned int
		}{
			// a: m1 completed, m2 own abandonment, m3 EXCLUDED, m4 completed.
			{name: "abandoner is charged only their own", viewer: f.a, completed: 2, abandoned: 1},
			// b: m1 + m4 completed, m2 someone else's abandonment -> completed
			// (b stayed), m3 EXCLUDED.
			{name: "opponent of abandoner completed all three", viewer: f.b, completed: 3, abandoned: 0},
			// c: the abandoner's PARTNER. Honor does not punish the teammate.
			{name: "partner of abandoner is not charged", viewer: f.c, completed: 3, abandoned: 0},
			{name: "second opponent mirrors the first", viewer: f.d, completed: 3, abandoned: 0},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				w, err := f.repo.GetHonorTrendWindowsForUser(tc.viewer, 20)
				require.NoError(t, err)
				assert.Equal(t, tc.completed, w.RecentCompleted, "recent completed")
				assert.Equal(t, tc.abandoned, w.RecentAbandoned, "recent abandoned")
				assert.Zero(t, w.PriorCompleted, "only three eligible rows exist, so nothing spills over")
				assert.Zero(t, w.PriorAbandoned)
			})
		}
	})

	t.Run("the limit splits newest from next-newest", func(t *testing.T) {
		// limit 1 fetches 2 rows: m1 (completed) then m2 (abandoned by a).
		w, err := f.repo.GetHonorTrendWindowsForUser(f.a, 1)
		require.NoError(t, err)
		assert.Equal(t, 1, w.RecentCompleted, "m1 is the newest eligible row")
		assert.Equal(t, 0, w.RecentAbandoned)
		assert.Equal(t, 0, w.PriorCompleted)
		assert.Equal(t, 1, w.PriorAbandoned, "m2 is a's own abandonment, one slot back")

		// Same two rows from b's side: b stayed through both.
		w, err = f.repo.GetHonorTrendWindowsForUser(f.b, 1)
		require.NoError(t, err)
		assert.Equal(t, 1, w.RecentCompleted)
		assert.Equal(t, 0, w.RecentAbandoned)
		assert.Equal(t, 1, w.PriorCompleted, "someone else's abandonment is a completion for b")
		assert.Equal(t, 0, w.PriorAbandoned)
	})

	t.Run("a partial prior window reports its real size", func(t *testing.T) {
		// limit 2 fetches up to 4 rows; only 3 are eligible, so recent holds
		// m1+m2 and prior holds just m4. The caller (HonorTrendWindowed) is what
		// refuses to compare unequal windows — the repo reports honestly.
		w, err := f.repo.GetHonorTrendWindowsForUser(f.a, 2)
		require.NoError(t, err)
		assert.Equal(t, 1, w.RecentCompleted)
		assert.Equal(t, 1, w.RecentAbandoned)
		assert.Equal(t, 1, w.PriorCompleted, "m4 is the only row past the recent window")
		assert.Equal(t, 0, w.PriorAbandoned)
	})

	t.Run("a non-positive limit is a no-op", func(t *testing.T) {
		w, err := f.repo.GetHonorTrendWindowsForUser(f.a, 0)
		require.NoError(t, err)
		assert.Equal(t, match.HonorTrendWindows{}, w)
	})

	t.Run("a user with no matches yields zeroes, not an error", func(t *testing.T) {
		stranger := seedRepoUser(t, db, "rt"+repoFixtureSuffix()+"z")
		w, err := f.repo.GetHonorTrendWindowsForUser(stranger, 20)
		require.NoError(t, err)
		assert.Equal(t, match.HonorTrendWindows{}, w)
	})
}

// TestGormMatchRepository_GetLastMatchForRoomAndUser pins the room last-match
// read: it is scoped to one room, gated on the caller occupying a seat (that
// predicate IS the endpoint's authorization), resolves to the newest row, and
// preloads hands in play order.
func TestGormMatchRepository_GetLastMatchForRoomAndUser(t *testing.T) {
	db := getRepoTestDB(t)
	repo := match.NewGormMatchRepository(db)

	suffix := repoFixtureSuffix()
	a := seedRepoUser(t, db, "lm"+suffix+"a")
	b := seedRepoUser(t, db, "lm"+suffix+"b")
	c := seedRepoUser(t, db, "lm"+suffix+"c")
	d := seedRepoUser(t, db, "lm"+suffix+"d")
	stranger := seedRepoUser(t, db, "lm"+suffix+"z")
	roomOne := seedRepoRoom(t, db, a, suffix)
	// A second room needs its own unique name/code — build it from a fresh suffix.
	roomTwo := seedRepoRoom(t, db, a, repoFixtureSuffix())

	base := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	newMatch := func(roomID uint, offset time.Duration, status string) *match.Match {
		aID, bID, cID, dID := a, b, c, d
		return &match.Match{
			RoomID:      roomID,
			Player1ID:   &aID,
			Player2ID:   &bID,
			Player3ID:   &cID,
			Player4ID:   &dID,
			TeamAScore:  1010,
			TeamBScore:  640,
			WinnerTeam:  0,
			Variant:     "bitola",
			MatchMode:   "1001",
			StartedAt:   base.Add(offset - 20*time.Minute),
			CompletedAt: base.Add(offset),
			Status:      status,
		}
	}

	older := newMatch(roomOne, -2*time.Hour, "completed")
	newest := newMatch(roomOne, -1*time.Hour, "abandoned")
	// Newer in wall-clock terms, but a DIFFERENT room — must never be picked.
	otherRoom := newMatch(roomTwo, 0, "completed")
	require.NoError(t, repo.Create(older))
	// Hands are inserted OUT of order so the ASC preload has something to sort.
	require.NoError(t, repo.CreateWithHands(newest, []match.HandResult{
		{HandNumber: 3, TeamAHandTotal: 60, TeamBHandTotal: 102},
		{HandNumber: 1, TeamAHandTotal: 90, TeamBHandTotal: 72},
		{HandNumber: 2, TeamAHandTotal: 81, TeamBHandTotal: 81},
	}))
	require.NoError(t, repo.Create(otherRoom))

	t.Run("participant gets the room's newest match with ordered hands", func(t *testing.T) {
		got, err := repo.GetLastMatchForRoomAndUser(roomOne, b)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, newest.ID, got.ID, "newest row in the room")
		assert.Equal(t, roomOne, got.RoomID)
		require.Len(t, got.Hands, 3)
		assert.Equal(t, []int{1, 2, 3}, []int{
			got.Hands[0].HandNumber, got.Hands[1].HandNumber, got.Hands[2].HandNumber,
		}, "hands preloaded in hand_number ASC order")
	})

	t.Run("every seat is a participant", func(t *testing.T) {
		for _, viewer := range []uint{a, b, c, d} {
			got, err := repo.GetLastMatchForRoomAndUser(roomOne, viewer)
			require.NoError(t, err)
			require.NotNil(t, got, "viewer %d", viewer)
			assert.Equal(t, newest.ID, got.ID)
		}
	})

	t.Run("a non-participant gets nothing, not the row", func(t *testing.T) {
		got, err := repo.GetLastMatchForRoomAndUser(roomOne, stranger)
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("scoping is per room", func(t *testing.T) {
		got, err := repo.GetLastMatchForRoomAndUser(roomTwo, a)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, otherRoom.ID, got.ID, "room two resolves to its own row")
	})

	t.Run("a room with no matches yields nil, not an error", func(t *testing.T) {
		empty := seedRepoRoom(t, db, a, repoFixtureSuffix())
		got, err := repo.GetLastMatchForRoomAndUser(empty, a)
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("an in-progress row is not the last match", func(t *testing.T) {
		room := seedRepoRoom(t, db, a, repoFixtureSuffix())
		live := newMatch(room, time.Hour, "in_progress")
		require.NoError(t, repo.Create(live))
		got, err := repo.GetLastMatchForRoomAndUser(room, a)
		require.NoError(t, err)
		assert.Nil(t, got, "only completed / abandoned rows count")
	})
}
