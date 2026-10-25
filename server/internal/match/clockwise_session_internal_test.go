package match

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
)

func TestStartMatch_CarriesClockwiseToTheEngineAndTheBotView(t *testing.T) {
	firstBidder := map[bool]map[int]int{
		false: {0: 1, 1: 2, 2: 3, 3: 0},
		true:  {0: 3, 1: 0, 2: 1, 3: 2},
	}
	players := [4]PlayerSeatInfo{
		{UserID: 10, Username: "a", Seat: 0},
		{UserID: 20, Username: "b", Seat: 1},
		{UserID: 30, Username: "c", Seat: 2},
		{UserID: 40, Username: "d", Seat: 3},
	}

	for _, variant := range []game.Variant{game.VariantBitola, game.VariantCroatia} {
		for _, clockwise := range []bool{false, true} {
			name := string(variant) + "/counter-clockwise"
			if clockwise {
				name = string(variant) + "/clockwise"
			}
			t.Run(name, func(t *testing.T) {
				const roomID = uint(140)
				m := NewManager(&recordingBroadcaster{}, nil)
				require.NoError(t, m.StartMatch(roomID, string(variant), "1001", players, "relaxed", 0, 10, 120, 0,
					true, false, true, clockwise))
				t.Cleanup(func() { m.RemoveSession(roomID) })

				gs := m.GetStateSnapshot(roomID)
				require.NotNil(t, gs)

				assert.Equal(t, clockwise, gs.Rules.Clockwise, "the rule config the engine reads")
				assert.Equal(t, clockwise, gs.Clockwise, "and the wire flag the client renders")
				assert.True(t, gs.Rules.DeclarationsTransfer, "the rule arguments are not crossed")
				assert.False(t, gs.Rules.StopAtTarget, "the rule arguments are not crossed")
				assert.Equal(t, firstBidder[clockwise][gs.DealerSeat], gs.ActivePlayerSeat, "dealer %d", gs.DealerSeat)

				for seat := range 4 {
					assert.Equal(t, clockwise, buildBotView(gs, seat, nil).Clockwise, "seat %d", seat)
				}
			})
		}
	}
}
