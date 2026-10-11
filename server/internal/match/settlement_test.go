package match

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// sumDeltas is the coin-conservation check: winners' net gains must exactly
// offset losers' net losses, minus any house sink (no-human-winner case).
func sumDeltas(d [4]int) int {
	return d[0] + d[1] + d[2] + d[3]
}

func TestComputeSettlement(t *testing.T) {
	const S = 500
	ids := [4]uint{10, 20, 30, 40}
	noBots := [4]bool{false, false, false, false}

	tests := []struct {
		name        string
		playerIDs   [4]uint
		botSeats    [4]bool
		winningTeam int
		buyIn       int
		wantDeltas  [4]int
		wantPot     int
		wantCredits map[uint]int
		wantSink    int
	}{
		{
			name:        "all-human 2v2, team 0 wins (classic 4S split)",
			playerIDs:   ids,
			botSeats:    noBots,
			winningTeam: 0,
			buyIn:       S,
			wantDeltas:  [4]int{+S, -S, +S, -S},
			wantPot:     4 * S,
			wantCredits: map[uint]int{10: 2 * S, 30: 2 * S},
		},
		{
			name:        "all-human 2v2, team 1 wins",
			playerIDs:   ids,
			botSeats:    noBots,
			winningTeam: 1,
			buyIn:       S,
			wantDeltas:  [4]int{-S, +S, -S, +S},
			wantPot:     4 * S,
			wantCredits: map[uint]int{20: 2 * S, 40: 2 * S},
		},
		{
			name:        "H+B vs H+B, human winner takes whole pot (bot teammate paid nothing)",
			playerIDs:   [4]uint{10, 20, 0, 40},
			botSeats:    [4]bool{false, false, true, false},
			winningTeam: 0,
			buyIn:       S,
			wantDeltas:  [4]int{+2 * S, -S, 0, -S},
			wantPot:     3 * S,
			wantCredits: map[uint]int{10: 3 * S},
		},
		{
			name:        "H+B vs B+B, lone human on winning team nets zero (gets own stake back)",
			playerIDs:   [4]uint{10, 0, 0, 0},
			botSeats:    [4]bool{false, true, true, true},
			winningTeam: 0,
			buyIn:       S,
			wantDeltas:  [4]int{0, 0, 0, 0},
			wantPot:     1 * S,
			wantCredits: map[uint]int{10: S},
		},
		{
			name:        "H+B vs B+B, lone human loses → coin sink (AC #9)",
			playerIDs:   [4]uint{10, 0, 0, 0},
			botSeats:    [4]bool{false, true, true, true},
			winningTeam: 1,
			buyIn:       S,
			wantDeltas:  [4]int{-S, 0, 0, 0},
			wantPot:     1 * S,
			wantCredits: map[uint]int{},
			wantSink:    S,
		},
		{
			name:        "abandonment: winner = non-abandoning team, whole abandoning team forfeits",
			playerIDs:   ids,
			botSeats:    noBots,
			winningTeam: 1,
			buyIn:       S,
			wantDeltas:  [4]int{-S, +S, -S, +S},
			wantPot:     4 * S,
			wantCredits: map[uint]int{20: 2 * S, 40: 2 * S},
		},
		{
			name:        "odd split: 3 humans, 2-human team wins, remainder to lowest seat (Decision C)",
			playerIDs:   [4]uint{10, 20, 30, 0},
			botSeats:    [4]bool{false, false, false, true},
			winningTeam: 0,
			buyIn:       3,
			wantDeltas:  [4]int{+2, -3, +1, 0},
			wantPot:     9,
			wantCredits: map[uint]int{10: 5, 30: 4},
		},
		{
			name:        "zero buy-in → no economy, all zero",
			playerIDs:   ids,
			botSeats:    noBots,
			winningTeam: 0,
			buyIn:       0,
			wantDeltas:  [4]int{0, 0, 0, 0},
			wantPot:     0,
			wantCredits: map[uint]int{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deltas, credits, pot := computeSettlement(tc.playerIDs, tc.botSeats, tc.winningTeam, tc.buyIn)
			assert.Equal(t, tc.wantDeltas, deltas, "deltas")
			assert.Equal(t, tc.wantPot, pot, "pot")
			assert.Equal(t, tc.wantCredits, credits, "credits")

			assert.Equal(t, -tc.wantSink, sumDeltas(deltas), "coin conservation (sum of deltas)")

			for seat := 0; seat < 4; seat++ {
				if tc.botSeats[seat] {
					assert.Equal(t, 0, deltas[seat], "bot seat %d delta must be 0", seat)
					_, ok := credits[tc.playerIDs[seat]]
					assert.False(t, ok, "bot seat %d must not be credited", seat)
				}
			}
		})
	}
}
