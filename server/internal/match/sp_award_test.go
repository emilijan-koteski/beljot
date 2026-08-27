package match

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Table-driven coverage of the SP formula (Story 13.1 AC1). computeSPAwards is
// unexported, so this test lives in package `match` alongside it — the wiring and
// event-ordering coverage is in sp_wiring_test.go (package match_test).
//
// Expected values are written out as LITERAL ARITHMETIC in each case's comment
// rather than recomputed from the consts, so a retune of a const cannot silently
// make this suite agree with a broken formula.
func TestComputeSPAwards(t *testing.T) {
	allHuman := [4]uint{10, 20, 30, 40}
	noBots := [4]bool{}
	allPresent := [4]bool{true, true, true, true}

	cases := []struct {
		name              string
		playerIDs         [4]uint
		botSeats          [4]bool
		connected         [4]bool
		teamScores        [2]int
		winnerTeam        int
		capotOrInstantWin bool
		abandonedSeat     int
		want              [4]int
	}{
		{
			// Team A (seats 0,2) wins 1010:700.
			// A: 50 + 100 + floor(1010/10)=101       -> 251
			// B: 50 +   0 + floor(700/10)=70         -> 120
			name:          "normal win and loss both earn",
			playerIDs:     allHuman,
			botSeats:      noBots,
			connected:     allPresent,
			teamScores:    [2]int{1010, 700},
			winnerTeam:    0,
			abandonedSeat: -1,
			want:          [4]int{251, 120, 251, 120},
		},
		{
			// Team B wins. Symmetry check — the win bonus follows winnerTeam, not
			// the higher score, so a stop-at-target or surrender result is right.
			name:          "team B winning mirrors exactly",
			playerIDs:     allHuman,
			botSeats:      noBots,
			connected:     allPresent,
			teamScores:    [2]int{700, 1010},
			winnerTeam:    1,
			abandonedSeat: -1,
			want:          [4]int{120, 251, 120, 251},
		},
		{
			// The win bonus is NOT re-derived from scores. A surrender hands the
			// win to the team with FEWER points, and the formula must honour that.
			name:          "the winner with fewer points still takes the win bonus",
			playerIDs:     allHuman,
			botSeats:      noBots,
			connected:     allPresent,
			teamScores:    [2]int{900, 200},
			winnerTeam:    1,
			abandonedSeat: -1,
			want:          [4]int{140, 170, 140, 170},
		},
		{
			// D2: the Capot bonus is MATCH-level. All four seats get +50 — the
			// losing team included.
			// A: 50 + 100 + 101 + 50 -> 301   B: 50 + 0 + 70 + 50 -> 170
			name:              "a Capot pays every seat, winners and losers",
			playerIDs:         allHuman,
			botSeats:          noBots,
			connected:         allPresent,
			teamScores:        [2]int{1010, 700},
			winnerTeam:        0,
			capotOrInstantWin: true,
			abandonedSeat:     -1,
			want:              [4]int{301, 170, 301, 170},
		},
		{
			// An instant win: TeamScores [0,0] and no hand results at all.
			// floor(0/10) == 0 is a LEGITIMATE term, not a bug.
			// A: 50 + 100 + 0 + 50 -> 200     B: 50 + 0 + 0 + 50 -> 100
			name:              "instant win on a fresh deal at 0:0",
			playerIDs:         allHuman,
			botSeats:          noBots,
			connected:         allPresent,
			teamScores:        [2]int{0, 0},
			winnerTeam:        0,
			capotOrInstantWin: true,
			abandonedSeat:     -1,
			want:              [4]int{200, 100, 200, 100},
		},
		{
			// Two Capot hands, or a Capot AND an instant win, are still +50 ONCE:
			// the flag is a boolean, so there is nothing to double.
			// A: 50 + 100 + 20 + 50 -> 220    B: 50 + 0 + 10 + 50 -> 110
			name:              "capot and instant win together still pay +50 once",
			playerIDs:         allHuman,
			botSeats:          noBots,
			connected:         allPresent,
			teamScores:        [2]int{200, 100},
			winnerTeam:        0,
			capotOrInstantWin: true,
			abandonedSeat:     -1,
			want:              [4]int{220, 110, 220, 110},
		},
		{
			// A match with no points on the board yet (a surrender in hand 1).
			// The completion bonus alone keeps a finish worth something.
			name:          "zero scores still pay the completion bonus",
			playerIDs:     allHuman,
			botSeats:      noBots,
			connected:     allPresent,
			teamScores:    [2]int{0, 0},
			winnerTeam:    0,
			abandonedSeat: -1,
			want:          [4]int{150, 50, 150, 50},
		},
		{
			// Bot seats (1 and 3) earn nothing and are not subjects at all.
			name:          "bot seats earn nothing",
			playerIDs:     [4]uint{10, 0, 30, 0},
			botSeats:      [4]bool{false, true, false, true},
			connected:     allPresent,
			teamScores:    [2]int{500, 300},
			winnerTeam:    0,
			abandonedSeat: -1,
			want:          [4]int{200, 0, 200, 0},
		},
		{
			// An empty seat (playerID 0 without the bot flag) is likewise skipped —
			// the exact guard settlement.go and computeXPAwards use.
			name:          "empty seats earn nothing",
			playerIDs:     [4]uint{10, 20, 0, 40},
			botSeats:      noBots,
			connected:     allPresent,
			teamScores:    [2]int{500, 300},
			winnerTeam:    0,
			abandonedSeat: -1,
			want:          [4]int{200, 80, 0, 80},
		},
		{
			// D5, THE HEADLINE CASE. Seat 0 abandons; its TEAMMATE (seat 2) is
			// present and STILL EARNS. This is where SP diverges from coins and XP,
			// which forfeit the whole abandoning team.
			// Non-abandoning team B wins: 50 + 100 + floor(300/10)=30 -> 180
			// Present teammate (team A, lost): 50 + 0 + floor(900/10)=90 -> 140
			name:          "abandonment forfeits per seat, not per team",
			playerIDs:     allHuman,
			botSeats:      noBots,
			connected:     [4]bool{false, true, true, true},
			teamScores:    [2]int{900, 300},
			winnerTeam:    1,
			abandonedSeat: 0,
			want:          [4]int{0, 180, 140, 180},
		},
		{
			// The expired seat is charged even if its Connected flag were somehow
			// stale — seat == abandonedSeat is checked independently of presence.
			name:          "the expired seat is charged even when it reads connected",
			playerIDs:     allHuman,
			botSeats:      noBots,
			connected:     allPresent,
			teamScores:    [2]int{900, 300},
			winnerTeam:    1,
			abandonedSeat: 0,
			want:          [4]int{0, 180, 140, 180},
		},
		{
			// Concurrent double disconnect: seat 0's window expired and seat 3 is
			// also absent inside its own window. Both earn 0; the two present seats
			// earn normally.
			name:          "every absent seat earns zero, not just the expired one",
			playerIDs:     allHuman,
			botSeats:      noBots,
			connected:     [4]bool{false, true, true, false},
			teamScores:    [2]int{900, 300},
			winnerTeam:    1,
			abandonedSeat: 0,
			want:          [4]int{0, 180, 140, 0},
		},
		{
			// A Capot scored before the abandonment still pays the present seats.
			name:              "the capot bonus survives an abandonment for present seats",
			playerIDs:         allHuman,
			botSeats:          noBots,
			connected:         [4]bool{false, true, true, true},
			teamScores:        [2]int{900, 300},
			winnerTeam:        1,
			capotOrInstantWin: true,
			abandonedSeat:     0,
			want:              [4]int{0, 230, 190, 230},
		},
		{
			// The presence array is IGNORED on a natural end: that path reached a
			// real terminal state, so a stale disconnect flag must not zero anyone.
			name:          "presence is not consulted on a natural end",
			playerIDs:     allHuman,
			botSeats:      noBots,
			connected:     [4]bool{false, false, false, false},
			teamScores:    [2]int{1010, 700},
			winnerTeam:    0,
			abandonedSeat: -1,
			want:          [4]int{251, 120, 251, 120},
		},
		{
			// Defensive: a negative team total can never reach the write, because
			// the DB CHECK is sp >= 0 and SP is a monotonic accumulator.
			name:          "a negative team score never produces a negative award",
			playerIDs:     allHuman,
			botSeats:      noBots,
			connected:     allPresent,
			teamScores:    [2]int{-40, 200},
			winnerTeam:    1,
			abandonedSeat: -1,
			want:          [4]int{50, 170, 50, 170},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := computeSPAwards(tc.playerIDs, tc.botSeats, tc.connected,
				tc.teamScores, tc.winnerTeam, tc.capotOrInstantWin, tc.abandonedSeat)
			assert.Equal(t, tc.want, got)
			for seat, sp := range got {
				assert.GreaterOrEqual(t, sp, 0, "seat %d must never earn negative SP", seat)
			}
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

func TestCapotOccurred(t *testing.T) {
	assert.False(t, capotOccurred(nil), "no hands (an instant win) is not a capot")
	assert.False(t, capotOccurred([]HandResult{}), "an empty buffer is not a capot")
	assert.False(t, capotOccurred([]HandResult{{Capot: false}, {Capot: false}}))
	assert.True(t, capotOccurred([]HandResult{{Capot: false}, {Capot: true}}),
		"a capot in any hand counts")
	assert.True(t, capotOccurred([]HandResult{{Capot: true}, {Capot: true}}),
		"two capots are still one bonus — the answer is a bool")
}
