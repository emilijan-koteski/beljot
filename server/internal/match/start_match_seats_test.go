package match

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
)

func TestStartMatch_RejectsALineUpWithAnEmptySeat(t *testing.T) {
	cases := []struct {
		name    string
		players [4]PlayerSeatInfo
	}{
		{"seat with no player and no bot", [4]PlayerSeatInfo{
			{UserID: 10, Seat: 0}, {Seat: 1, IsBot: true}, {Seat: 2, IsBot: true}, {},
		}},
		{"seat number out of place", [4]PlayerSeatInfo{
			{UserID: 10, Seat: 0}, {UserID: 20, Seat: 1}, {UserID: 30, Seat: 3}, {UserID: 40, Seat: 3},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := NewManager(&recordingBroadcaster{}, nil)

			err := m.StartMatch(100, string(game.VariantCroatia), "501", tc.players, "per-move", 10, 10, 120, 0, true, false, true, false)

			require.Error(t, err)
			assert.Nil(t, m.GetStateSnapshot(100), "no session starts")
			assert.False(t, m.IsUserInMatch(10))
		})
	}
}
