package match

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/emilijan/beljot/server/internal/game"
)

// Table-driven coverage of the match-side SP outcome builder (Story 13.4).
// buildSPOutcome and capotTeams are unexported, so this test lives in package
// `match` alongside them; the formula itself is tested in season, and the
// wiring and event ordering in sp_wiring_test.go (package match_test).

func capot(team int) *int { return &team }

func TestBuildSPOutcome(t *testing.T) {
	allHuman := [4]uint{10, 20, 30, 40}
	allPresent := [4]bool{true, true, true, true}

	cases := []struct {
		name  string
		facts spMatchFacts
		want  MatchOutcome
	}{
		{
			name: "natural 1001 finish, every seat present",
			facts: spMatchFacts{
				playerIDs: allHuman, connected: allPresent,
				winnerTeam: game.TeamA, teamScores: [2]int{1010, 700}, matchMode: "1001",
				abandonedSeat: -1,
			},
			want: MatchOutcome{
				Seats: [4]OutcomeSeat{
					{UserID: 10, Team: 0, Completed: true},
					{UserID: 20, Team: 1, Completed: true},
					{UserID: 30, Team: 0, Completed: true},
					{UserID: 40, Team: 1, Completed: true},
				},
				WinnerTeam: 0, TeamScores: [2]int{1010, 700}, Target: 1001, AbandonedSeat: -1,
			},
		},
		{
			// The target is the engine's own for the mode.
			name: "501 surrender with stale presence flags",
			facts: spMatchFacts{
				playerIDs: allHuman, connected: [4]bool{false, false, true, true},
				winnerTeam: game.TeamB, teamScores: [2]int{200, 300}, matchMode: "501",
				surrender: true, abandonedSeat: -1,
			},
			want: MatchOutcome{
				Seats: [4]OutcomeSeat{
					{UserID: 10, Team: 0, Completed: true},
					{UserID: 20, Team: 1, Completed: true},
					{UserID: 30, Team: 0, Completed: true},
					{UserID: 40, Team: 1, Completed: true},
				},
				WinnerTeam: 1, TeamScores: [2]int{200, 300}, Target: 501, Surrender: true, AbandonedSeat: -1,
			},
		},
		{
			// Bot seats carry no user; any id sitting in a bot seat's slot is
			// ignored. Capot teams come from the hand rows, once per team.
			name: "bot table with an instant win and Capots",
			facts: spMatchFacts{
				playerIDs: [4]uint{10, 0, 99, 0}, botSeats: [4]bool{false, true, true, true},
				connected: allPresent, winnerTeam: game.TeamA, matchMode: "1001",
				instantWin: true,
				hands: []HandResult{
					{HandNumber: 1, Capot: true, CapotTeam: capot(game.TeamB)},
					{HandNumber: 2},
					{HandNumber: 3, Capot: true, CapotTeam: capot(game.TeamB)},
				},
				abandonedSeat: -1,
			},
			want: MatchOutcome{
				Seats: [4]OutcomeSeat{
					{UserID: 10, Team: 0, Completed: true},
					{IsBot: true, Team: 1},
					{IsBot: true, Team: 0},
					{IsBot: true, Team: 1},
				},
				WinnerTeam: 0, Target: 1001, InstantWin: true, CapotTeams: [2]bool{false, true}, AbandonedSeat: -1,
			},
		},
		{
			// Seat 0's window expired and seat 3 is inside its own window: both
			// are absent (Completed false) but both stay in the outcome, and the
			// formula scores seat 3 by its team's result.
			name: "abandonment with a second seat disconnected",
			facts: spMatchFacts{
				playerIDs: allHuman, connected: [4]bool{false, true, true, false},
				winnerTeam: game.TeamB, teamScores: [2]int{300, 500}, matchMode: "1001",
				hands:         []HandResult{{HandNumber: 1, Capot: true, CapotTeam: capot(game.TeamA)}},
				abandonedSeat: 0,
			},
			want: MatchOutcome{
				Seats: [4]OutcomeSeat{
					{UserID: 10, Team: 0, Completed: false},
					{UserID: 20, Team: 1, Completed: true},
					{UserID: 30, Team: 0, Completed: true},
					{UserID: 40, Team: 1, Completed: false},
				},
				WinnerTeam: 1, TeamScores: [2]int{300, 500}, Target: 1001,
				CapotTeams: [2]bool{true, false}, AbandonedSeat: 0,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, buildSPOutcome(tc.facts))
		})
	}
}

// The presence gate is the SAME rule computeHonorEvents applies. Asserted
// explicitly, over the same inputs, so the two can never drift apart silently
// (Story 13.1 D5 reuses honor's gate deliberately).
func TestSPSeatPresent_MatchesTheHonorPresenceGate(t *testing.T) {
	playerIDs := [4]uint{10, 20, 30, 40}
	noBots := [4]bool{}

	for _, tc := range []struct {
		name          string
		connected     [4]bool
		abandonedSeat int
	}{
		{"natural end with stale flags", [4]bool{false, false, true, true}, -1},
		{"single abandonment", [4]bool{false, true, true, true}, 0},
		{"double disconnect", [4]bool{false, true, false, true}, 0},
		{"expired seat still reads connected", [4]bool{true, true, true, true}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events := computeHonorEvents(playerIDs, noBots, tc.connected, tc.abandonedSeat)
			for seat := 0; seat < 4; seat++ {
				honorSaysPresent := !events[playerIDs[seat]].Abandoned
				assert.Equal(t, honorSaysPresent, spSeatPresent(tc.connected, seat, tc.abandonedSeat),
					"seat %d: SP and honor must agree on presence", seat)
			}
		})
	}
}

func TestCapotTeams(t *testing.T) {
	assert.Equal(t, [2]bool{}, capotTeams(nil), "no hands (an instant win) has no Capot")
	assert.Equal(t, [2]bool{}, capotTeams([]HandResult{{}, {}}))
	assert.Equal(t, [2]bool{true, false}, capotTeams([]HandResult{{}, {Capot: true, CapotTeam: capot(0)}}))
	assert.Equal(t, [2]bool{true, true},
		capotTeams([]HandResult{{Capot: true, CapotTeam: capot(1)}, {Capot: true, CapotTeam: capot(0)}}),
		"both teams can earn the bonus in one match")
	assert.Equal(t, [2]bool{false, true},
		capotTeams([]HandResult{{Capot: true, CapotTeam: capot(1)}, {Capot: true, CapotTeam: capot(1)}}),
		"two Capots are still one bonus — the answer is a flag per team")
	assert.Equal(t, [2]bool{}, capotTeams([]HandResult{{Capot: true}, {Capot: true, CapotTeam: capot(5)}}),
		"a Capot without a valid team cannot be attributed")
}
