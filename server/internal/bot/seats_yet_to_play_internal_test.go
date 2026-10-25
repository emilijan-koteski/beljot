package bot

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/emilijan/beljot/server/internal/game"
)

func TestSeatsYetToPlayFollowsDirection(t *testing.T) {
	tests := []struct {
		name      string
		clockwise bool
		leader    int
		want      []int
	}{
		{name: "counter-clockwise", clockwise: false, leader: 1, want: []int{3, 0}},
		{name: "clockwise", clockwise: true, leader: 3, want: []int{1, 0}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := View{
				Seat:      2,
				Clockwise: tc.clockwise,
				CurrentTrick: []game.TrickCard{
					{Card: game.Card{Rank: game.Rank7, Suit: game.SuitSpades}, PlayerSeat: tc.leader},
				},
			}

			assert.Equal(t, tc.want, seatsYetToPlay(v))
		})
	}
}
