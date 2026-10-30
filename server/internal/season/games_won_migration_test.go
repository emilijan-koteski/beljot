package season_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/season"
)

// Migration 000034's backfill counts each player's wins from the season's stored matches with the replay's rules,
// capped at the games the row has played; it touches no other column.
func TestGamesWonMigration_BackfillsFromStoredMatches(t *testing.T) {
	db := getTestDB(t)
	repo := season.NewGormRepository(db)

	up, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000034_add_games_won_to_player_seasons.up.sql"))
	require.NoError(t, err)
	backfill := string(up)[strings.Index(string(up), "WITH seats AS"):]

	start := time.Date(2093, time.January, 1, 0, 0, 0, 0, time.UTC)
	s := makeSeason(t, db, start)
	a := makeUser(t, db, "gw-a@s.test")
	b := makeUser(t, db, "gw-b@s.test")
	c := makeUser(t, db, "gw-c@s.test")
	d := makeUser(t, db, "gw-d@s.test")
	roomID := replayRoom(t, db, a.ID)

	at := func(m match.Match, offset time.Duration) match.Match {
		m.CompletedAt = start.Add(offset)
		return m
	}
	seats := [4]uint{a.ID, b.ID, c.ID, d.ID}
	storeMatch(t, db, roomID, at(replayFourHumans(0, seats, 0, 1100, 700), time.Hour))
	abandoned := replayFourHumans(0, seats, 1, 200, 400)
	abandoned.Status, abandoned.AbandonedBy = "abandoned", uintPtr(d.ID)
	storeMatch(t, db, roomID, at(abandoned, 2*time.Hour))
	placeholder := replayFourHumans(0, seats, 1, 200, 400)
	placeholder.Status = "abandoned"
	storeMatch(t, db, roomID, at(placeholder, 3*time.Hour))
	unknownMode := replayFourHumans(0, seats, 1, 1100, 700)
	unknownMode.MatchMode = "301"
	storeMatch(t, db, roomID, at(unknownMode, 4*time.Hour))
	storeMatch(t, db, roomID, at(replayFourHumans(0, seats, 1, 700, 1100), -time.Hour))

	for _, u := range []uint{a.ID, b.ID, d.ID} {
		for range 2 {
			_, err := applyAwards(repo, s.ID, map[uint]fixedAward{u: {change: 30, completed: true}})
			require.NoError(t, err)
		}
	}
	_, err = applyAwards(repo, s.ID, map[uint]fixedAward{c.ID: {change: 30, completed: true}})
	require.NoError(t, err)

	require.NoError(t, db.Exec(backfill).Error)

	for name, tc := range map[string]struct {
		id      uint
		won, sp int
	}{
		"a wins the natural finish and the abandonment": {a.ID, 2, 60},
		"b never sits on the winning team":              {b.ID, 0, 60},
		"c's two wins are capped at its one game":       {c.ID, 1, 30},
		"d abandoned, so its team lost":                 {d.ID, 0, 60},
	} {
		row, err := repo.FindPlayerSeason(tc.id, s.ID)
		require.NoError(t, err)
		require.NotNil(t, row, name)
		assert.Equal(t, tc.won, row.GamesWon, name)
		assert.Equal(t, tc.sp, row.SP, "%s: Season Points are untouched", name)
	}
}
