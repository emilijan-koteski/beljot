package game

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClockwiseDealPacketOrder(t *testing.T) {
	tests := []struct {
		name      string
		variant   Variant
		clockwise bool
		rounds    [][4]int
		want      []int
	}{
		{
			name: "bitola counter-clockwise", variant: VariantBitola, clockwise: false,
			rounds: [][4]int{{0, 3, 6, 9}, {12, 14, 16, 18}}, want: []int{1, 2, 3, 0},
		},
		{
			name: "bitola clockwise", variant: VariantBitola, clockwise: true,
			rounds: [][4]int{{0, 3, 6, 9}, {12, 14, 16, 18}}, want: []int{3, 2, 1, 0},
		},
		{
			name: "croatia counter-clockwise", variant: VariantCroatia, clockwise: false,
			rounds: [][4]int{{0, 3, 6, 9}, {12, 15, 18, 21}, {24, 26, 28, 30}}, want: []int{1, 2, 3, 0},
		},
		{
			name: "croatia clockwise", variant: VariantCroatia, clockwise: true,
			rounds: [][4]int{{0, 3, 6, 9}, {12, 15, 18, 21}, {24, 26, 28, 30}}, want: []int{3, 2, 1, 0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rules := RulesFor(tc.variant)
			rules.Clockwise = tc.clockwise
			gs := &GameState{Rules: rules, DealerSeat: 0}
			deck := NewDeck()

			dealCards(gs, deck)

			owner := make(map[Card]int, 32)
			for seat, p := range gs.Players {
				for _, c := range append(append([]Card{}, p.Hand...), p.FaceDownCards...) {
					owner[c] = seat
				}
			}
			for r, packetStarts := range tc.rounds {
				got := make([]int, 0, 4)
				for _, start := range packetStarts {
					got = append(got, owner[deck[start]])
				}
				assert.Equal(t, tc.want, got, "round %d", r+1)
			}
		})
	}
}

func TestPlayDirectionHelpers(t *testing.T) {
	ccw := VariantRules{}
	cw := VariantRules{Clockwise: true}

	for seat := range 4 {
		assert.Equal(t, (seat+1)%4, ccw.NextSeat(seat), "counter-clockwise after seat %d", seat)
		assert.Equal(t, (seat+3)%4, cw.NextSeat(seat), "clockwise after seat %d", seat)
		for n := range 4 {
			assert.Equal(t, n, ccw.PlayDistance(seat, ccw.SeatAfter(seat, n)))
			assert.Equal(t, n, cw.PlayDistance(seat, cw.SeatAfter(seat, n)))
		}
	}
	assert.Equal(t, 1, cw.PlayDistance(3, 2))
	assert.Equal(t, 2, cw.PlayDistance(3, 1))
	assert.Equal(t, 1, ccw.PlayDistance(1, 2))
}
