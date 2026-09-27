package season_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/season"
)

func uintPtr(v uint) *uint { return &v }

var replayT0 = time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)

// replayBotTable is user `human` at seat 0 with a bot partner, against two bots.
func replayBotTable(id uint, human uint, winner int, a, b int) match.Match {
	return match.Match{
		ID: id, Status: "completed", MatchMode: "1001",
		Player1ID: uintPtr(human), Player2IsBot: true, Player3IsBot: true, Player4IsBot: true,
		TeamAScore: a, TeamBScore: b, WinnerTeam: winner,
		CompletedAt: replayT0.Add(time.Duration(id) * time.Minute),
	}
}

// replayFourHumans seats the users at seats 0-3 (team A is seats 0 and 2).
func replayFourHumans(id uint, seats [4]uint, winner int, a, b int) match.Match {
	return match.Match{
		ID: id, Status: "completed", MatchMode: "1001",
		Player1ID: uintPtr(seats[0]), Player2ID: uintPtr(seats[1]), Player3ID: uintPtr(seats[2]), Player4ID: uintPtr(seats[3]),
		TeamAScore: a, TeamBScore: b, WinnerTeam: winner,
		CompletedAt: replayT0.Add(time.Duration(id) * time.Minute),
	}
}

func TestOutcomeFor(t *testing.T) {
	t.Run("natural finish", func(t *testing.T) {
		m := replayFourHumans(1, [4]uint{1, 2, 3, 4}, 0, 1100, 700)
		o, err := season.OutcomeFor(m)
		require.NoError(t, err)
		assert.Equal(t, match.MatchOutcome{
			Seats: [4]match.OutcomeSeat{
				{UserID: 1, Team: 0, Completed: true},
				{UserID: 2, Team: 1, Completed: true},
				{UserID: 3, Team: 0, Completed: true},
				{UserID: 4, Team: 1, Completed: true},
			},
			WinnerTeam: 0, TeamScores: [2]int{1100, 700}, Target: 1001, AbandonedSeat: -1,
		}, o)
	})

	t.Run("bot seats carry no user", func(t *testing.T) {
		o, err := season.OutcomeFor(replayBotTable(1, 7, 0, 1100, 700))
		require.NoError(t, err)
		assert.Equal(t, match.OutcomeSeat{UserID: 7, Team: 0, Completed: true}, o.Seats[0])
		for seat := 1; seat < 4; seat++ {
			assert.True(t, o.Seats[seat].IsBot)
			assert.Zero(t, o.Seats[seat].UserID)
			assert.Equal(t, seat%2, o.Seats[seat].Team)
		}
	})

	t.Run("surrender: surrendered_by set and the winners below the target", func(t *testing.T) {
		m := replayFourHumans(1, [4]uint{1, 2, 3, 4}, 1, 300, 500)
		m.SurrenderedBy = uintPtr(1)
		o, err := season.OutcomeFor(m)
		require.NoError(t, err)
		assert.True(t, o.Surrender)
		assert.False(t, o.InstantWin)
	})

	t.Run("a surrender that finalized at the target is a natural finish", func(t *testing.T) {
		m := replayFourHumans(1, [4]uint{1, 2, 3, 4}, 1, 640, 1012)
		m.SurrenderedBy = uintPtr(1)
		o, err := season.OutcomeFor(m)
		require.NoError(t, err)
		assert.False(t, o.Surrender)
		assert.False(t, o.InstantWin)
	})

	t.Run("instant win: completed, no surrender, winners below the target", func(t *testing.T) {
		o, err := season.OutcomeFor(replayFourHumans(1, [4]uint{1, 2, 3, 4}, 1, 0, 0))
		require.NoError(t, err)
		assert.True(t, o.InstantWin)
		assert.False(t, o.Surrender)
	})

	t.Run("the 501 target", func(t *testing.T) {
		m := replayFourHumans(1, [4]uint{1, 2, 3, 4}, 0, 520, 200)
		m.MatchMode = "501"
		o, err := season.OutcomeFor(m)
		require.NoError(t, err)
		assert.Equal(t, 501, o.Target)
		assert.False(t, o.InstantWin, "520 reached the 501 target")
	})

	t.Run("abandonment: the abandoner's seat, and the other team wins", func(t *testing.T) {
		m := replayFourHumans(1, [4]uint{1, 2, 3, 4}, 0, 200, 400)
		m.Status = "abandoned"
		m.AbandonedBy = uintPtr(3)
		o, err := season.OutcomeFor(m)
		require.NoError(t, err)
		assert.Equal(t, 2, o.AbandonedSeat)
		assert.False(t, o.Seats[2].Completed, "the abandoner is the one seat a stored match can mark absent")
		for _, seat := range []int{0, 1, 3} {
			assert.True(t, o.Seats[seat].Completed, "seat %d", seat)
		}
		assert.Equal(t, 1, o.WinnerTeam, "the winner is the non-abandoning team whatever winner_team says")
		assert.False(t, o.Surrender, "the formula scores an abandonment as a surrender itself")
		assert.False(t, o.InstantWin, "an abandonment below the target is not an instant win")
	})

	t.Run("Capot teams come from the hand rows", func(t *testing.T) {
		m := replayFourHumans(1, [4]uint{1, 2, 3, 4}, 0, 1100, 700)
		m.Hands = []match.HandResult{
			{HandNumber: 1, Capot: true, CapotTeam: intPtr(1)},
			{HandNumber: 2},
			{HandNumber: 3, Capot: true, CapotTeam: intPtr(1)},
			{HandNumber: 4, Capot: true},
		}
		o, err := season.OutcomeFor(m)
		require.NoError(t, err)
		assert.Equal(t, [2]bool{false, true}, o.CapotTeams)
	})

	t.Run("skips", func(t *testing.T) {
		placeholder := replayFourHumans(1, [4]uint{1, 2, 3, 4}, 0, 200, 400)
		placeholder.Status = "abandoned"
		_, err := season.OutcomeFor(placeholder)
		assert.ErrorIs(t, err, season.ErrReconcilePlaceholder)

		unknown := replayFourHumans(1, [4]uint{1, 2, 3, 4}, 0, 1100, 700)
		unknown.MatchMode = "classic"
		_, err = season.OutcomeFor(unknown)
		assert.ErrorIs(t, err, season.ErrUnknownMode)
		assert.Equal(t, season.ErrUnknownMode.Error(), season.SkipReason(err), "every unknown mode counts under one key")

		stranger := replayFourHumans(1, [4]uint{1, 2, 3, 4}, 0, 200, 400)
		stranger.Status = "abandoned"
		stranger.AbandonedBy = uintPtr(99)
		_, err = season.OutcomeFor(stranger)
		assert.ErrorIs(t, err, season.ErrAbandonerNotSeated)
	})
}

// replayWindowByHand is a short window worked by hand with W=30, L=20, S=430 and
// a bot seat on 600, on a test ladder whose Silver floor is 40. It is the
// window cmd/sptune's own TestReplay reports on.
func replayWindowByHand() (season.SPFormula, []match.Match) {
	f := season.SPFormula{BaseWin: 30, BaseLoss: 20, Scale: 430, BotSeatSP: 600, CapotBonus: 5}

	// 1. Bot table, user 1 wins 1100:700 from 0 SP. Own avg 300 vs 600, E = 0.167:
	//    +45. The bots' team made a Capot, which pays nobody.
	m1 := replayBotTable(1, 1, 0, 1100, 700)
	m1.Hands = []match.HandResult{{HandNumber: 1, Capot: true, CapotTeam: intPtr(1)}}
	// 2. Four humans, users 1 and 3 (avg 22.5) surrender to 2 and 4 (avg 0) at
	//    300:500. m = 1.2003, E_winner = 0.470: winners +38, losers -25, floored.
	m2 := replayFourHumans(2, [4]uint{1, 2, 3, 4}, 1, 300, 500)
	m2.SurrenderedBy = uintPtr(1)
	// 3. A boot-reconcile placeholder: skipped, and counted in nobody's games.
	m3 := replayFourHumans(3, [4]uint{1, 2, 3, 4}, 0, 100, 100)
	m3.Status = "abandoned"
	// 4. 501, users 2 and 4 (avg 38) against user 1 and a bot (avg 310); user 4
	//    abandons at 200:400. m = 1.1008, E_winner = 0.811: user 1 +12, user 2
	//    half of -8.32 = -4, user 4 -120 (floored to 0).
	m4 := match.Match{
		ID: 4, Status: "abandoned", MatchMode: "501", AbandonedBy: uintPtr(4),
		Player1ID: uintPtr(2), Player2ID: uintPtr(1), Player3ID: uintPtr(4), Player4IsBot: true,
		TeamAScore: 200, TeamBScore: 400, WinnerTeam: 1, CompletedAt: replayT0.Add(4 * time.Minute),
	}
	// 5. An unknown mode: skipped.
	m5 := replayFourHumans(5, [4]uint{1, 2, 3, 4}, 0, 1100, 700)
	m5.MatchMode = "301"
	return f, []match.Match{m1, m2, m3, m4, m5}
}

func TestReplayMatches(t *testing.T) {
	f, matches := replayWindowByHand()
	ladder, err := season.NewLadder([]int{0, 20, 40, 600, 800, 1000, 1200, 1400})
	require.NoError(t, err)

	var seen []season.ReplayedMatch
	got := season.ReplayMatches(matches, f, ladder, func(rm season.ReplayedMatch) { seen = append(seen, rm) })

	assert.Equal(t, []season.ReplayedPlayer{
		{UserID: 1, SP: 32, Tier: "bronze", Division: 2, GamesPlayed: 3, GamesCompleted: 3},
		{UserID: 2, SP: 34, Tier: "bronze", Division: 3, GamesPlayed: 2, GamesCompleted: 2},
		{UserID: 3, SP: 0, Tier: "iron", Division: 1, GamesPlayed: 1, GamesCompleted: 1},
		{UserID: 4, SP: 0, Tier: "iron", Division: 1, GamesPlayed: 2, GamesCompleted: 1},
	}, got.Players, "ascending user ID; the abandoner's abandoned match is played, not completed")

	assert.Equal(t, season.ReplaySummary{
		Loaded: 5, Scored: 3,
		Skipped: map[string]int{
			season.ErrReconcilePlaceholder.Error(): 1,
			season.ErrUnknownMode.Error():          1,
		},
	}, got.Summary)

	// The observer sees each scored match once, in order, with every human
	// seat's total before and after it (after the 0 floor).
	require.Len(t, seen, 3)
	assert.Equal(t, []uint{1, 2, 4}, []uint{seen[0].Match.ID, seen[1].Match.ID, seen[2].Match.ID})
	assert.Equal(t, [4]int{45, 0, 0, 0}, seen[1].Before)
	assert.Equal(t, [4]int{20, 38, 0, 38}, seen[1].After, "user 3's -25 from 0 is floored at 0")
	assert.Equal(t, [4]int{38, 20, 38, 0}, seen[2].Before)
	assert.Equal(t, [4]int{34, 32, 0, 0}, seen[2].After, "user 4's -120 abandonment is floored at 0")
	assert.True(t, seen[1].Outcome.Surrender)
	assert.Equal(t, 2, seen[2].Outcome.AbandonedSeat)
}

// A replay reads only its input, so replaying the same matches twice gives the
// same result: what makes a forced re-run of a recalculation a no-op.
func TestReplayMatches_IsRepeatable(t *testing.T) {
	f, matches := replayWindowByHand()
	first := season.ReplayMatches(matches, f, season.DefaultLadder(), nil)
	second := season.ReplayMatches(matches, f, season.DefaultLadder(), nil)
	assert.Equal(t, first, second)
}

func TestReplayMatches_AFormulaRejectionIsSkipped(t *testing.T) {
	bad := replayFourHumans(1, [4]uint{1, 2, 3, 4}, 7, 1100, 700)
	good := replayBotTable(2, 1, 0, 1100, 700)

	got := season.ReplayMatches([]match.Match{bad, good}, season.DefaultSPFormula(), season.DefaultLadder(), nil)

	assert.Equal(t, 1, got.Summary.Scored)
	require.Len(t, got.Summary.Skipped, 1)
	for reason := range got.Summary.Skipped {
		assert.Contains(t, reason, "formula rejected the outcome")
	}
	require.Len(t, got.Players, 1, "a rejected match counts in nobody's games")
	assert.Equal(t, uint(1), got.Players[0].UserID)
	assert.Equal(t, 1, got.Players[0].GamesPlayed)
}

// Replaying one match is exactly ComputeSPChanges-then-floor for every seat: the
// replay is only as good as its agreement with the award path.
func TestReplayMatches_IsTheAwardPathArithmetic(t *testing.T) {
	m := replayFourHumans(1, [4]uint{1, 2, 3, 4}, 0, 1100, 700)
	o, err := season.OutcomeFor(m)
	require.NoError(t, err)
	want, err := season.ComputeSPChanges(o, map[uint]int{})
	require.NoError(t, err)

	got := season.ReplayMatches([]match.Match{m}, season.DefaultSPFormula(), season.DefaultLadder(), nil)
	require.Len(t, got.Players, 4)
	for _, p := range got.Players {
		sp := season.ApplySPChange(0, want[p.UserID])
		tier, division := season.RankForSP(sp)
		assert.Equal(t, season.ReplayedPlayer{
			UserID: p.UserID, SP: sp, Tier: tier, Division: division, GamesPlayed: 1, GamesCompleted: 1,
		}, p)
	}
}

// replayRoom inserts a room the test's matches can reference.
func replayRoom(t *testing.T, db *gorm.DB, ownerID uint) uint {
	t.Helper()
	suffix := fmt.Sprintf("%08d", time.Now().UnixNano()%1e8)
	var roomID uint
	require.NoError(t, db.Raw(`
		INSERT INTO rooms (name, code, owner_id, status) VALUES (?, ?, ?, 'completed') RETURNING id`,
		"replay-"+suffix, "R"+suffix[len(suffix)-5:], ownerID).Scan(&roomID).Error)
	return roomID
}

// storeMatch writes m (its ID left to the database) and its hand rows.
func storeMatch(t *testing.T, db *gorm.DB, roomID uint, m match.Match) uint {
	t.Helper()
	hands := m.Hands
	m.ID, m.Hands, m.RoomID = 0, nil, roomID
	m.Variant = "bitola"
	if m.StartedAt.IsZero() {
		m.StartedAt = m.CompletedAt.Add(-time.Hour)
	}
	m.HasBots = m.Player1IsBot || m.Player2IsBot || m.Player3IsBot || m.Player4IsBot
	require.NoError(t, db.Create(&m).Error)
	for _, h := range hands {
		h.MatchID = m.ID
		require.NoError(t, db.Create(&h).Error)
	}
	return m.ID
}

func TestLoadWindow(t *testing.T) {
	db := getTestDB(t)
	alice := makeUser(t, db, "lw-a@s.test")
	roomID := replayRoom(t, db, alice.ID)

	from := time.Date(2091, 7, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 3, 0)
	at := func(completedAt time.Time, status string) match.Match {
		m := replayBotTable(0, alice.ID, 0, 1100, 700)
		m.CompletedAt, m.Status = completedAt, status
		return m
	}
	withHands := at(from.Add(48*time.Hour), "completed")
	withHands.Hands = []match.HandResult{
		{HandNumber: 2, Capot: true, CapotTeam: intPtr(1)},
		{HandNumber: 1},
	}
	second := storeMatch(t, db, roomID, withHands)
	first := storeMatch(t, db, roomID, at(from, "completed"))
	tieA := storeMatch(t, db, roomID, at(from.Add(72*time.Hour), "abandoned"))
	tieB := storeMatch(t, db, roomID, at(from.Add(72*time.Hour), "completed"))
	storeMatch(t, db, roomID, at(from.Add(-time.Second), "completed")) // before the window
	storeMatch(t, db, roomID, at(to, "completed"))                     // the end is exclusive
	storeMatch(t, db, roomID, at(from.Add(time.Hour), "in_progress"))  // not finished

	matches, err := season.LoadWindow(db, from, to)
	require.NoError(t, err)

	ids := make([]uint, 0, len(matches))
	for _, m := range matches {
		ids = append(ids, m.ID)
	}
	assert.Equal(t, []uint{first, second, min(tieA, tieB), max(tieA, tieB)}, ids,
		"completed_at order, id breaking ties, finished rows inside [from, to) only")

	require.Len(t, matches[1].Hands, 2)
	assert.Equal(t, 1, matches[1].Hands[0].HandNumber, "hands in hand order")
	o, err := season.OutcomeFor(matches[1])
	require.NoError(t, err)
	assert.Equal(t, [2]bool{false, true}, o.CapotTeams)
}
