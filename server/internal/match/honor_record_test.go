package match

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestComputeHonorEvents checks per-seat completed/abandoned bucketing across end reasons, bots and disconnects.
func TestComputeHonorEvents(t *testing.T) {
	ids := [4]uint{10, 20, 30, 40}
	noBots := [4]bool{false, false, false, false}

	tests := []struct {
		name          string
		playerIDs     [4]uint
		botSeats      [4]bool
		disconnected  [4]bool
		abandonedSeat int
		want          map[uint]HonorEvent
	}{
		{
			name:          "natural end credits every human seat a completion",
			playerIDs:     ids,
			botSeats:      noBots,
			abandonedSeat: -1,
			want: map[uint]HonorEvent{
				10: {Abandoned: false}, 20: {Abandoned: false},
				30: {Abandoned: false}, 40: {Abandoned: false},
			},
		},
		{
			name:          "accepted surrender is identical to a natural end",
			playerIDs:     ids,
			botSeats:      noBots,
			abandonedSeat: -1,
			want: map[uint]HonorEvent{
				10: {Abandoned: false}, 20: {Abandoned: false},
				30: {Abandoned: false}, 40: {Abandoned: false},
			},
		},
		{
			name:          "abandonment charges only the expired seat",
			playerIDs:     ids,
			botSeats:      noBots,
			abandonedSeat: 2,
			want: map[uint]HonorEvent{
				10: {Abandoned: false}, 20: {Abandoned: false},
				30: {Abandoned: true}, 40: {Abandoned: false},
			},
		},
		{
			name:          "abandonment by seat 0",
			playerIDs:     ids,
			botSeats:      noBots,
			abandonedSeat: 0,
			want: map[uint]HonorEvent{
				10: {Abandoned: true}, 20: {Abandoned: false},
				30: {Abandoned: false}, 40: {Abandoned: false},
			},
		},
		{
			name:          "bot seats accrue nothing",
			playerIDs:     [4]uint{10, 20, 30, 0},
			botSeats:      [4]bool{false, false, false, true},
			abandonedSeat: -1,
			want: map[uint]HonorEvent{
				10: {Abandoned: false}, 20: {Abandoned: false}, 30: {Abandoned: false},
			},
		},
		{
			name:          "an empty seat (userID 0, not flagged bot) accrues nothing",
			playerIDs:     [4]uint{10, 0, 30, 40},
			botSeats:      noBots,
			abandonedSeat: -1,
			want: map[uint]HonorEvent{
				10: {Abandoned: false}, 30: {Abandoned: false}, 40: {Abandoned: false},
			},
		},
		{
			name:          "an abandoned bot seat still accrues nothing",
			playerIDs:     [4]uint{10, 20, 30, 0},
			botSeats:      [4]bool{false, false, false, true},
			abandonedSeat: 3,
			want: map[uint]HonorEvent{
				10: {Abandoned: false}, 20: {Abandoned: false}, 30: {Abandoned: false},
			},
		},
		{
			name:          "all-bot table produces no events at all",
			playerIDs:     [4]uint{0, 0, 0, 0},
			botSeats:      [4]bool{true, true, true, true},
			abandonedSeat: -1,
			want:          map[uint]HonorEvent{},
		},
		{
			name:          "every absent seat is charged, not just the expired one",
			playerIDs:     ids,
			botSeats:      noBots,
			disconnected:  [4]bool{false, true, true, false},
			abandonedSeat: 1,
			want: map[uint]HonorEvent{
				10: {Abandoned: false}, 20: {Abandoned: true},
				30: {Abandoned: true}, 40: {Abandoned: false},
			},
		},
		{
			name:          "all four absent are all charged",
			playerIDs:     ids,
			botSeats:      noBots,
			disconnected:  [4]bool{true, true, true, true},
			abandonedSeat: 0,
			want: map[uint]HonorEvent{
				10: {Abandoned: true}, 20: {Abandoned: true},
				30: {Abandoned: true}, 40: {Abandoned: true},
			},
		},
		{
			name:          "a disconnected seat at a natural end is still credited",
			playerIDs:     ids,
			botSeats:      noBots,
			disconnected:  [4]bool{false, false, true, false},
			abandonedSeat: -1,
			want: map[uint]HonorEvent{
				10: {Abandoned: false}, 20: {Abandoned: false},
				30: {Abandoned: false}, 40: {Abandoned: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var connected [4]bool
			for i := range connected {
				connected[i] = !tt.disconnected[i]
			}
			got := computeHonorEvents(tt.playerIDs, tt.botSeats, connected, tt.abandonedSeat)
			assert.Equal(t, tt.want, got)
			_, hasBotPlaceholder := got[0]
			assert.False(t, hasBotPlaceholder, "userID 0 must never be an honor subject")
		})
	}
}

// TestComputeHonorEvents_ConcurrentDoubleDisconnect checks every seat absent at match end is charged,
// not only the seat whose timer fired, so quitting second gains nothing.
func TestComputeHonorEvents_ConcurrentDoubleDisconnect(t *testing.T) {
	ids := [4]uint{10, 20, 30, 40}
	noBots := [4]bool{false, false, false, false}

	connected := [4]bool{true, false, false, true}
	got := computeHonorEvents(ids, noBots, connected, 1)

	assert.Equal(t, HonorEvent{Abandoned: true}, got[20], "the seat whose timer fired is charged")
	assert.Equal(t, HonorEvent{Abandoned: true}, got[30],
		"the other absent seat is charged too — quitting second must carry the same penalty as quitting first")

	assert.Equal(t, HonorEvent{Abandoned: false}, got[10], "a seat that stayed is credited")
	assert.Equal(t, HonorEvent{Abandoned: false}, got[40], "a seat that stayed is credited")
	assert.Len(t, got, 4, "every human seat is an honor subject — absence is charged, never skipped")
}
