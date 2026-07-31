package match

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestComputeHonorEvents pins the per-seat honor bucketing (Story 9.7). There
// are exactly two buckets — completed and abandoned (D1) — and the whole matrix
// of AC3 reduces to this one function plus the fact that reconcile.go never
// calls it.
func TestComputeHonorEvents(t *testing.T) {
	ids := [4]uint{10, 20, 30, 40} // seats 0,2 → team A; seats 1,3 → team B
	noBots := [4]bool{false, false, false, false}

	tests := []struct {
		name      string
		playerIDs [4]uint
		botSeats  [4]bool
		// disconnected expresses ABSENCE rather than presence, so the zero value
		// reads as "everybody was still at the table" and only the rows that
		// actually exercise the presence gate have to spell it out.
		disconnected  [4]bool
		abandonedSeat int
		want          map[uint]HonorEvent
	}{
		{
			// Natural end (win/loss). EVERY human seat completed — the losing
			// team finished the match too, and honor measures finishing, not
			// winning.
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
			// An accepted surrender routes through handleMatchEnd with Status
			// still "completed", so it is byte-identical to the row above. The
			// surrendering team agreed to end the match; they did not walk out.
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
			// Abandonment: ONLY the seat whose reconnect window expired is
			// charged. The other three stayed — including the abandoner's own
			// TEAMMATE, who is not punished for their partner's disconnect
			// (unlike coins and XP, which forfeit team-wide).
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
			// Bot seats never accrue honor (sprint-change-proposal-2026-06-11
			// Change 2). Seat 3 is a bot; only the three humans get events.
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
			// Defensive: a bot seat can't disconnect, but if abandonedSeat ever
			// pointed at one, the bot must still be skipped entirely rather
			// than writing honor for userID 0.
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
			// Review pass 2: seat 1's timer fired, but seat 2 is ALSO absent inside
			// their own still-open window. BOTH are charged — the rule is presence,
			// not whose timer happened to expire, so there is no advantage to
			// quitting second.
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
			// Worst case: everybody drops. Nobody saw the match through, so nobody
			// is credited a completion.
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
			// The presence gate is abandonment-only. A natural end reached a real
			// terminal state, so a seat that reads as disconnected in the final
			// snapshot is still credited. This path must not change.
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

// TestComputeHonorEvents_ConcurrentDoubleDisconnect pins AC3's concurrent-drop
// requirement as amended by PO decision 2026-07-29 (review pass 2): when two
// seats are both inside reconnect windows, BOTH are charged an abandonment. The
// rule is PRESENCE at match end, not whose timer happened to expire first.
//
// This is the third rule tried on this code path, and the history is the reason
// the test is explicit:
//
//  1. As originally shipped, the other absent seat was CREDITED a completion, on
//     the reasoning that "the match ended out from under them". That rewards a
//     player who walked out and never returned, and makes quitting SECOND
//     strictly better than quitting first — a gameable bypass of the signal
//     Story 9.8 gates room access on.
//  2. Review pass 1 made it neutral (no event). That removed the reward but
//     replaced it with a worse hole: no row is written, so the raw totals never
//     increment and a repeat second-quitter stayed pinned at the 80 prior with
//     isNewPlayer=true forever.
//  3. Charging on absence is monotonic and leaves no ordering incentive.
//
// It does NOT create a new abandonment trigger — what ends a match and what lands
// in matches.abandoned_by are unchanged; only the honor bucket differs.
func TestComputeHonorEvents_ConcurrentDoubleDisconnect(t *testing.T) {
	ids := [4]uint{10, 20, 30, 40}
	noBots := [4]bool{false, false, false, false}

	// Seats 1 and 2 are both disconnected; seat 1's timer fires first and ends
	// the match, so handleSeatReconnectTimeout passes abandonedSeat = 1.
	connected := [4]bool{true, false, false, true}
	got := computeHonorEvents(ids, noBots, connected, 1)

	assert.Equal(t, HonorEvent{Abandoned: true}, got[20], "the seat whose timer fired is charged")
	assert.Equal(t, HonorEvent{Abandoned: true}, got[30],
		"the other absent seat is charged too — quitting second must carry the same penalty as quitting first")

	assert.Equal(t, HonorEvent{Abandoned: false}, got[10], "a seat that stayed is credited")
	assert.Equal(t, HonorEvent{Abandoned: false}, got[40], "a seat that stayed is credited")
	assert.Len(t, got, 4, "every human seat is an honor subject — absence is charged, never skipped")
}
