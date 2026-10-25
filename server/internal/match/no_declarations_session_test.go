package match_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/ws"
)

func TestStartMatch_CarriesDeclarationsSettingIntoTheSession(t *testing.T) {
	tests := []struct {
		name                string
		variant             string
		declarationsEnabled bool
	}{
		{"bitola with declarations", "bitola", true},
		{"bitola without declarations", "bitola", false},
		{"croatia with declarations", "croatia", true},
		{"croatia without declarations", "croatia", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const roomID = uint(100)
			hub := ws.NewHub()
			go hub.Run()
			t.Cleanup(hub.Shutdown)
			mgr := match.NewManager(hub, newMockMatchRepo())
			require.NoError(t, mgr.StartMatch(
				roomID, tt.variant, "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0,
				tt.declarationsEnabled, false, true,
			))
			t.Cleanup(func() { mgr.RemoveSession(roomID) })

			gs := mgr.GetStateSnapshot(roomID)
			require.NotNil(t, gs)

			assert.Equal(t, tt.declarationsEnabled, gs.Rules.DeclarationsEnabled,
				"the rule config the engine reads")
			assert.Equal(t, tt.declarationsEnabled, gs.DeclarationsEnabled,
				"and the wire flag the client renders")
			assert.Equal(t, !tt.declarationsEnabled, gs.DeclarationsResolved,
				"a declarations-off hand starts with the contest already settled")

			if tt.variant == "croatia" {
				assert.Equal(t, game.DeclarationTimingDedicatedPhase, gs.Rules.DeclarationTiming)
				assert.True(t, gs.Rules.DeclarationOverlap)
			} else {
				assert.Equal(t, game.DeclarationTimingDuringFirstTrick, gs.Rules.DeclarationTiming)
				assert.True(t, gs.Rules.HasTrumpCandidate)
			}
		})
	}
}

// TestCroatianSessionWithoutDeclarationsNeverOpensThePhase checks that a
// declarations-off Croatian session goes straight from the bid to trick 1.
func TestCroatianSessionWithoutDeclarationsNeverOpensThePhase(t *testing.T) {
	const roomID = uint(101)
	hub := &hubSpy{}
	mgr := match.NewManager(hub, newMockMatchRepo())
	require.NoError(t, mgr.StartMatch(
		roomID, "croatia", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, false, false, true,
	))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	gs := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, gs)
	require.Equal(t, game.PhaseBidding, gs.Phase, "the session auto-transitions dealing to bidding")

	before := len(hub.snapshot())

	suit := game.SuitHearts
	require.NoError(t, mgr.ApplyActionForTest(roomID, game.Action{
		Type:       game.ActionPickTrump,
		PlayerSeat: gs.ActivePlayerSeat,
		Suit:       &suit,
	}))

	after := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, after)
	assert.Equal(t, game.PhasePlaying, after.Phase)
	assert.Equal(t, 1, after.TrickNumber)
	assert.False(t, after.DeclarationsEnabled, "the flag survives the phase transition")

	events := wireEvents(t, hub.snapshot()[before:])
	assert.Equal(t, -1, indexOfKind(events, ws.EventDeclarationsResolved),
		"a declarations-off table must emit no declarations_resolved reveal")
	assert.Equal(t, -1, indexOfMatchStateWithPhase(events, string(game.PhaseDeclaring)),
		"and no match_state may ever carry the declaring phase")
}
