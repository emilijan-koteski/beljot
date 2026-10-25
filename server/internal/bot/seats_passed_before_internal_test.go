package bot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeatsPassedBeforeFollowsDirection(t *testing.T) {
	tests := []struct {
		name      string
		clockwise bool
		dealer    int
		seat      int
		want      int
	}{
		{name: "counter-clockwise first bidder", clockwise: false, dealer: 0, seat: 1, want: 0},
		{name: "counter-clockwise third bidder", clockwise: false, dealer: 0, seat: 3, want: 2},
		{name: "counter-clockwise dealer speaks last", clockwise: false, dealer: 0, seat: 0, want: 3},
		{name: "clockwise first bidder", clockwise: true, dealer: 0, seat: 3, want: 0},
		{name: "clockwise third bidder", clockwise: true, dealer: 0, seat: 1, want: 2},
		{name: "clockwise dealer's partner", clockwise: true, dealer: 2, seat: 0, want: 1},
		{name: "clockwise dealer speaks last", clockwise: true, dealer: 2, seat: 2, want: 3},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := View{Seat: tc.seat, DealerSeat: tc.dealer, Clockwise: tc.clockwise}
			assert.Equal(t, tc.want, seatsPassedBefore(v))
		})
	}
}
