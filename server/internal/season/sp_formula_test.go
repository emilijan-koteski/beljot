package season_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/season"
)

// placeholderFormula is the constant set the spec's I/O matrix is written in
// (W=30, L=20, with the approved S and bot value), restated here so the matrix
// rows keep their meaning if the live constants are retuned. S and the bot value
// only matter where E != 0.5.
var placeholderFormula = season.SPFormula{
	BaseWin:    30,
	BaseLoss:   20,
	Scale:      385,
	BotSeatSP:  600,
	CapotBonus: 5,
}

// Seat user IDs for the all-human table. Team A is seats 0 and 2, team B 1 and 3.
const (
	uSeat0 uint = 10
	uSeat1 uint = 11
	uSeat2 uint = 12
	uSeat3 uint = 13
)

// humanTable is a four-human outcome with every seat present and no abandonment.
func humanTable(winner int, scores [2]int, target int) match.MatchOutcome {
	return match.MatchOutcome{
		Seats: [4]match.OutcomeSeat{
			{UserID: uSeat0, Team: 0, Completed: true},
			{UserID: uSeat1, Team: 1, Completed: true},
			{UserID: uSeat2, Team: 0, Completed: true},
			{UserID: uSeat3, Team: 1, Completed: true},
		},
		WinnerTeam:    winner,
		TeamScores:    scores,
		Target:        target,
		AbandonedSeat: -1,
	}
}

// evenSP puts every human on the same SP, so both team averages are equal and
// E = 0.5 (the matrix's default).
func evenSP(sp int) map[uint]int {
	return map[uint]int{uSeat0: sp, uSeat1: sp, uSeat2: sp, uSeat3: sp}
}

// teamChanges asserts both seats of each team got the expected change.
func assertTeamChanges(t *testing.T, got map[uint]int, teamA, teamB int) {
	t.Helper()
	assert.Equal(t, teamA, got[uSeat0], "seat 0 (team A)")
	assert.Equal(t, teamA, got[uSeat2], "seat 2 (team A)")
	assert.Equal(t, teamB, got[uSeat1], "seat 1 (team B)")
	assert.Equal(t, teamB, got[uSeat3], "seat 3 (team B)")
	assert.Len(t, got, 4)
}

// The spec's I/O & edge-case matrix, one row per case (placeholders W=30, L=20,
// equal team averages unless stated). The leaderboard and Q4-reset rows are
// persistence rules and are tested against the database, not here.
func TestSPFormula_Matrix(t *testing.T) {
	t.Run("natural win, 1001, 1100:700 -> m=0.90, +27 / -18", func(t *testing.T) {
		got, err := placeholderFormula.Changes(humanTable(0, [2]int{1100, 700}, 1001), evenSP(500))
		require.NoError(t, err)
		assertTeamChanges(t, got, 27, -18)
	})

	t.Run("surrender by A at 300:500 -> m=0.5+701/1001=1.20, B +36 / A -24", func(t *testing.T) {
		o := humanTable(1, [2]int{300, 500}, 1001)
		o.Surrender = true
		got, err := placeholderFormula.Changes(o, evenSP(500))
		require.NoError(t, err)
		assertTeamChanges(t, got, -24, 36)
	})

	t.Run("instant win, 0:0 with no hands -> m=1.5, +45 / -30", func(t *testing.T) {
		o := humanTable(0, [2]int{0, 0}, 1001)
		o.InstantWin = true
		got, err := placeholderFormula.Changes(o, evenSP(500))
		require.NoError(t, err)
		assertTeamChanges(t, got, 45, -30)
	})

	t.Run("Capot by the losers -> losers -18+5 = -13", func(t *testing.T) {
		o := humanTable(0, [2]int{1100, 700}, 1001)
		o.CapotTeams = [2]bool{false, true}
		got, err := placeholderFormula.Changes(o, evenSP(500))
		require.NoError(t, err)
		assertTeamChanges(t, got, 27, -13)
	})

	t.Run("floor: loser on 10 SP computing -18 ends on 0, an applied -10", func(t *testing.T) {
		got, err := placeholderFormula.Changes(humanTable(0, [2]int{1100, 700}, 1001), evenSP(10))
		require.NoError(t, err)
		require.Equal(t, -18, got[uSeat1], "the formula's own change is not floored")

		total := season.ApplySPChange(10, got[uSeat1])
		assert.Equal(t, 0, total)
		assert.Equal(t, -10, total-10, "the applied change is new minus previous")

		// A first-ever row (missing = 0 SP) after a loss writes 0, never a negative.
		assert.Equal(t, 0, season.ApplySPChange(0, got[uSeat1]))
	})

	t.Run("bot table: own avg (x+B)/2 against B, bots get no entry", func(t *testing.T) {
		o := match.MatchOutcome{
			Seats: [4]match.OutcomeSeat{
				{UserID: uSeat0, Team: 0, Completed: true},
				{IsBot: true, Team: 1},
				{IsBot: true, Team: 0},
				{IsBot: true, Team: 1},
			},
			WinnerTeam:    0,
			TeamScores:    [2]int{1100, 700},
			Target:        1001,
			AbandonedSeat: -1,
		}
		// x = 0: own avg 300 vs 600, E = 1/(1+10^(300/385)) = 0.143.
		got, err := placeholderFormula.Changes(o, map[uint]int{})
		require.NoError(t, err)
		assert.Equal(t, map[uint]int{uSeat0: 46}, got, "30·0.8996·2·0.857 = 46.28")

		o.WinnerTeam = 1
		o.TeamScores = [2]int{700, 1100}
		got, err = placeholderFormula.Changes(o, map[uint]int{})
		require.NoError(t, err)
		assert.Equal(t, map[uint]int{uSeat0: -5}, got, "-20·0.8996·2·0.143 = -5.13")

		// x = 1000: own avg 800 vs 600, E = 0.768; a win pays far less.
		o.WinnerTeam = 0
		o.TeamScores = [2]int{1100, 700}
		got, err = placeholderFormula.Changes(o, map[uint]int{uSeat0: 1000})
		require.NoError(t, err)
		assert.Equal(t, map[uint]int{uSeat0: 13}, got, "30·0.8996·2·0.232 = 12.53")
	})

	t.Run("abandonment: seat 0 expires, seat 3 disconnected", func(t *testing.T) {
		o := humanTable(1, [2]int{300, 500}, 1001)
		o.AbandonedSeat = 0
		o.Seats[0].Completed = false
		o.Seats[3].Completed = false
		got, err := placeholderFormula.Changes(o, evenSP(500))
		require.NoError(t, err)

		assert.Equal(t, -120, got[uSeat0], "abandoner: -2 × (20 × 1.5 × 2)")
		assert.Equal(t, -12, got[uSeat2], "teammate: ½ × the -24 a surrender at 300:500 costs")
		assert.Equal(t, 36, got[uSeat1], "opponent: a surrender-scored win")
		assert.Equal(t, 36, got[uSeat3], "a disconnected opponent is still scored by the team result")
	})
}

func TestSPFormula_AbandonmentCapotBonus(t *testing.T) {
	o := humanTable(1, [2]int{300, 500}, 1001)
	o.AbandonedSeat = 0
	o.CapotTeams = [2]bool{true, true}
	got, err := placeholderFormula.Changes(o, evenSP(500))
	require.NoError(t, err)

	assert.Equal(t, -120, got[uSeat0], "the abandoner never gets the Capot bonus")
	assert.Equal(t, -12+5, got[uSeat2], "the teammate keeps their team's Capot bonus")
	assert.Equal(t, 36+5, got[uSeat1])
	assert.Equal(t, 36+5, got[uSeat3])
}

func TestSPFormula_AbandonmentBeforeAnyHandIsTheMaximumMargin(t *testing.T) {
	o := humanTable(0, [2]int{0, 0}, 501)
	o.AbandonedSeat = 1
	got, err := placeholderFormula.Changes(o, evenSP(0))
	require.NoError(t, err)

	assert.Equal(t, 45, got[uSeat0], "0:0 scored as a surrender is m = 1.5")
	assert.Equal(t, -15, got[uSeat3], "½ × -30")
	assert.Equal(t, -120, got[uSeat1])
}

func TestSPFormula_Margin(t *testing.T) {
	cases := []struct {
		name     string
		scores   [2]int
		target   int
		wantWin  int
		wantLoss int
	}{
		// 0.5 + 0/1001 = 0.5 is the floor of the clamp: +15 / -10.
		{"a dead-level finish is the minimum margin", [2]int{1001, 1001}, 1001, 15, -10},
		// The taker can win a both-cross hand with FEWER points; the clamp holds at 0.5.
		{"a winner below the loser clamps to 0.5", [2]int{1010, 1050}, 1001, 15, -10},
		// 0.5 + 1001/1001 = 1.5.
		{"a whitewash is the maximum margin", [2]int{1001, 0}, 1001, 45, -30},
		{"above the maximum clamps to 1.5", [2]int{1200, 0}, 1001, 45, -30},
		// 0.5 + 300/501 = 1.099: +33 / -22.
		{"the 501 target scales the margin", [2]int{520, 220}, 501, 33, -22},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := placeholderFormula.Changes(humanTable(0, tc.scores, tc.target), evenSP(300))
			require.NoError(t, err)
			assertTeamChanges(t, got, tc.wantWin, tc.wantLoss)
		})
	}
}

func TestSPFormula_TeammatesShareTheChangeAtUnevenSP(t *testing.T) {
	sp := map[uint]int{uSeat0: 900, uSeat2: 100, uSeat1: 400, uSeat3: 600}
	got, err := placeholderFormula.Changes(humanTable(0, [2]int{1100, 700}, 1001), sp)
	require.NoError(t, err)

	// Both averages are 500, so E = 0.5 even though no two players match.
	assertTeamChanges(t, got, 27, -18)
}

func TestSPFormula_ExpectedResultScalesTheChange(t *testing.T) {
	o := humanTable(0, [2]int{1100, 700}, 1001)

	favourite, err := placeholderFormula.Changes(o, map[uint]int{uSeat0: 1400, uSeat2: 1400, uSeat1: 0, uSeat3: 0})
	require.NoError(t, err)
	underdog, err := placeholderFormula.Changes(o, map[uint]int{uSeat0: 0, uSeat2: 0, uSeat1: 1400, uSeat3: 1400})
	require.NoError(t, err)

	// Gap 1400 at S=385: E = 0.9998. The favourite's win is worth almost nothing
	// and the beaten underdog barely loses.
	assert.Equal(t, 0, favourite[uSeat0])
	assert.Equal(t, 0, favourite[uSeat1])
	// The underdog's win is worth nearly the full 2·W·m, the beaten favourite
	// loses nearly the full 2·L·m.
	assert.Equal(t, 54, underdog[uSeat0])
	assert.Equal(t, -36, underdog[uSeat1])
}

// The forge constraint: the reward for beating weaker teams must fall well below
// 0.5×, or a player who beats bots more than two thirds of the time climbs
// forever. On the Grandmaster floor (1400) at a bot table the own average is 1000
// against 600, and at S = 385 the multiplier 2·(1 − E) is about 0.17×.
func TestSPFormula_WinsOverWeakerTablesShrinkWellBelowHalf(t *testing.T) {
	o := match.MatchOutcome{
		Seats: [4]match.OutcomeSeat{
			{UserID: uSeat0, Team: 0}, {IsBot: true, Team: 1},
			{IsBot: true, Team: 0}, {IsBot: true, Team: 1},
		},
		WinnerTeam: 0, TeamScores: [2]int{1001, 0}, Target: 1001, AbandonedSeat: -1,
	}
	got, err := placeholderFormula.Changes(o, map[uint]int{uSeat0: 1400})
	require.NoError(t, err)
	fullWin := placeholderFormula.BaseWin * 1.5 * 2
	assert.Equal(t, 8, got[uSeat0], "own avg 1000 vs 600: 2·(1 − 0.916) = 0.17 of the 90 a whitewash can pay")
	assert.Less(t, float64(got[uSeat0]), 0.25*fullWin)
}

func TestSPFormula_PresenceDoesNotGateTheChange(t *testing.T) {
	o := humanTable(0, [2]int{1100, 700}, 1001)
	for i := range o.Seats {
		o.Seats[i].Completed = false
	}
	got, err := placeholderFormula.Changes(o, evenSP(500))
	require.NoError(t, err)
	assertTeamChanges(t, got, 27, -18)
}

func TestSPFormula_RejectsAMalformedOutcome(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(o *match.MatchOutcome)
	}{
		{"winner out of range", func(o *match.MatchOutcome) { o.WinnerTeam = 2 }},
		{"zero target", func(o *match.MatchOutcome) { o.Target = 0 }},
		{"abandoned seat out of range", func(o *match.MatchOutcome) { o.AbandonedSeat = 4 }},
		{"seat team out of range", func(o *match.MatchOutcome) { o.Seats[1].Team = 3 }},
		{"three seats on one team", func(o *match.MatchOutcome) { o.Seats[1].Team = 0 }},
		{"abandoner on the winning team", func(o *match.MatchOutcome) { o.AbandonedSeat = 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := humanTable(0, [2]int{1100, 700}, 1001)
			tc.mutate(&o)
			_, err := placeholderFormula.Changes(o, evenSP(500))
			assert.Error(t, err)
		})
	}

	zeroScale := placeholderFormula
	zeroScale.Scale = 0
	_, err := zeroScale.Changes(humanTable(0, [2]int{1100, 700}, 1001), evenSP(500))
	assert.Error(t, err, "S = 0 would divide by zero")
}

func TestSPFormula_AbandonPenalty(t *testing.T) {
	assert.Equal(t, -120, placeholderFormula.AbandonPenalty(), "-2 × (20 × 1.5 × 2)")
}

func TestApplySPChange(t *testing.T) {
	assert.Equal(t, 127, season.ApplySPChange(100, 27))
	assert.Equal(t, 82, season.ApplySPChange(100, -18))
	assert.Equal(t, 0, season.ApplySPChange(18, -18))
	assert.Equal(t, 0, season.ApplySPChange(5, -120))
	assert.Equal(t, 5, season.ApplySPChange(0, 5), "a positive change from 0 applies in full")
	// The Capot bonus is part of the change BEFORE the floor: a loser on 0 SP
	// whose -18 becomes -13 with the bonus still ends on 0.
	assert.Equal(t, 0, season.ApplySPChange(0, -18+5))
}

// The live constants, restated as literals so a retune is a deliberate edit here
// as well as in sp_formula.go.
func TestDefaultSPFormula(t *testing.T) {
	f := season.DefaultSPFormula()
	assert.Equal(t, 30.0, f.BaseWin)
	assert.Equal(t, 20.0, f.BaseLoss)
	assert.Equal(t, 385.0, f.Scale)
	assert.Equal(t, 5, f.CapotBonus)
	assert.Equal(t, 600, f.BotSeatSP)
	assert.Equal(t, -120, f.AbandonPenalty())
	assert.Greater(t, f.BaseWin, f.BaseLoss, "W > L, so an even player creeps up")

	gold, ok := season.TierFloor(season.TierGold)
	require.True(t, ok)
	assert.Equal(t, gold, f.BotSeatSP, "a bot seat is the Gold 1 floor, read off the ladder")
}

func TestComputeSPChanges_UsesTheLiveConstants(t *testing.T) {
	o := humanTable(0, [2]int{1100, 700}, 1001)
	want, err := season.DefaultSPFormula().Changes(o, evenSP(500))
	require.NoError(t, err)
	got, err := season.ComputeSPChanges(o, evenSP(500))
	require.NoError(t, err)
	assert.Equal(t, want, got)
}
