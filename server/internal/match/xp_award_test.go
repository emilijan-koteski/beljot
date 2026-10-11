package match

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// XP is floor(teamScore/10) regardless of buy-in, bots earn 0, and a normal loss
// still earns while abandonment forfeits the whole abandoning team.
func TestComputeXPAwards(t *testing.T) {
	ids := [4]uint{10, 20, 30, 40}
	noBots := [4]bool{false, false, false, false}

	tests := []struct {
		name          string
		playerIDs     [4]uint
		botSeats      [4]bool
		teamScores    [2]int
		abandonedSeat int
		want          [4]int
	}{
		{
			name:          "normal 2v2: both teams earn, teammates equal, loser still earns",
			playerIDs:     ids,
			botSeats:      noBots,
			teamScores:    [2]int{1010, 700},
			abandonedSeat: -1,
			want:          [4]int{101, 70, 101, 70},
		},
		{
			name:          "normal: losing team B still earns its points/10",
			playerIDs:     ids,
			botSeats:      noBots,
			teamScores:    [2]int{1000, 50},
			abandonedSeat: -1,
			want:          [4]int{100, 5, 100, 5},
		},
		{
			name:          "bot seat earns 0",
			playerIDs:     [4]uint{10, 0, 30, 40},
			botSeats:      [4]bool{false, true, false, false},
			teamScores:    [2]int{500, 300},
			abandonedSeat: -1,
			want:          [4]int{50, 0, 50, 30},
		},
		{
			name:          "empty seat (no userID) earns 0",
			playerIDs:     [4]uint{10, 20, 30, 0},
			botSeats:      noBots,
			teamScores:    [2]int{400, 200},
			abandonedSeat: -1,
			want:          [4]int{40, 20, 40, 0},
		},
		{
			name:          "abandonment by team A seat: whole team A forfeits, team B earns",
			playerIDs:     ids,
			botSeats:      noBots,
			teamScores:    [2]int{900, 300},
			abandonedSeat: 2,
			want:          [4]int{0, 30, 0, 30},
		},
		{
			name:          "abandonment by team B seat: whole team B forfeits, team A earns",
			playerIDs:     ids,
			botSeats:      noBots,
			teamScores:    [2]int{900, 300},
			abandonedSeat: 1,
			want:          [4]int{90, 0, 90, 0},
		},
		{
			name:          "contrast: abandoning team B earns 0 for the SAME scores a normal loss earns 5",
			playerIDs:     ids,
			botSeats:      noBots,
			teamScores:    [2]int{1000, 50},
			abandonedSeat: 1,
			want:          [4]int{100, 0, 100, 0},
		},
		{
			name:          "abandonment excludes a bot on the non-abandoning team",
			playerIDs:     [4]uint{10, 0, 30, 40},
			botSeats:      [4]bool{false, true, false, false},
			teamScores:    [2]int{900, 300},
			abandonedSeat: 0,
			want:          [4]int{0, 0, 0, 30},
		},
		{
			name:          "zero scores → all zero (no points, no XP)",
			playerIDs:     ids,
			botSeats:      noBots,
			teamScores:    [2]int{0, 0},
			abandonedSeat: -1,
			want:          [4]int{0, 0, 0, 0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := computeXPAwards(tc.playerIDs, tc.botSeats, tc.teamScores, tc.abandonedSeat)
			assert.Equal(t, tc.want, got)

			for seat := 0; seat < 4; seat++ {
				if tc.botSeats[seat] || tc.playerIDs[seat] == 0 {
					assert.Equal(t, 0, got[seat], "bot/empty seat %d must earn 0", seat)
				}
			}
		})
	}
}
