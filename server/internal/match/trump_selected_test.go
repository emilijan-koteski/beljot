package match_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/ws"
)

// TestTrumpSelected_EmittedWithEmptyCardIDWhenNoCandidate locks the emit change:
// a take under a candidate-less variant must still fire event:trump_selected,
// with an empty cardId rather than being suppressed.
func TestTrumpSelected_EmittedWithEmptyCardIDWhenNoCandidate(t *testing.T) {
	tests := []struct {
		name       string
		state      func() *game.GameState
		suit       *game.Suit
		wantCardID string
	}{
		{
			name:       "no candidate — empty cardId",
			state:      func() *game.GameState { return testfixtures.NewGameCroatianJustDealt() },
			suit:       func() *game.Suit { s := game.SuitSpades; return &s }(),
			wantCardID: "",
		},
		{
			// The FORCED path: three passes recorded, the dealer on the clock
			// with no legal pass. The forced pick must announce itself on the
			// wire exactly like a voluntary one — empty cardId, no candidate.
			name: "forced dealer pick — still emits with empty cardId",
			state: func() *game.GameState {
				gs := testfixtures.NewGameCroatianMidBidding(3)
				if !game.MustPickTrump(gs, gs.ActivePlayerSeat) {
					panic("fixture must be the forced-pick state")
				}
				return gs
			},
			suit:       func() *game.Suit { s := game.SuitSpades; return &s }(),
			wantCardID: "",
		},
		{
			name:       "candidate present — the absorbed card rides along",
			state:      func() *game.GameState { return testfixtures.NewGameJustDealt() },
			suit:       nil,
			wantCardID: "AH", // the fixture's face-up candidate
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hub := &hubSpy{}
			mgr := match.NewManager(hub, newMockMatchRepo())

			const roomID = uint(100)
			require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false))
			t.Cleanup(func() { mgr.RemoveSession(roomID) })

			gs := tc.state()
			gs.RoomID = roomID
			mgr.SetGameStateForTest(roomID, gs)

			require.NoError(t, mgr.ApplyActionForTest(roomID, game.Action{
				Type:       game.ActionPickTrump,
				PlayerSeat: gs.ActivePlayerSeat,
				Suit:       tc.suit,
			}))

			var found bool
			for _, c := range hub.snapshot() {
				var msg struct {
					Type    string          `json:"type"`
					Payload json.RawMessage `json:"payload"`
				}
				require.NoError(t, json.Unmarshal(c.msg, &msg))
				if msg.Type != ws.EventTrumpSelected {
					continue
				}
				var payload ws.TrumpSelectedPayload
				require.NoError(t, json.Unmarshal(msg.Payload, &payload))
				found = true
				assert.Equal(t, tc.wantCardID, payload.CardID)
				assert.Equal(t, gs.ActivePlayerSeat, payload.PlayerSeat)
				assert.NotEmpty(t, payload.TrumpSuit)
			}
			assert.True(t, found, "event:trump_selected must be emitted for every successful take")
		})
	}
}
