package match_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/match"
)

// TestStartMatch_CarriesDeclarationsTransferIntoTheSession checks the room's choice survives StartMatch
// and lands on the resolved config the engine scores with, not just on the wire mirror.
func TestStartMatch_CarriesDeclarationsTransferIntoTheSession(t *testing.T) {
	tests := []struct {
		name     string
		variant  string
		transfer bool
	}{
		{"bitola transferring declarations", "bitola", true},
		{"bitola voiding declarations", "bitola", false},
		{"croatia transferring declarations", "croatia", true},
		{"croatia voiding declarations", "croatia", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const roomID = uint(130)
			hub := &hubSpy{}
			mgr := match.NewManager(hub, newMockMatchRepo())
			require.NoError(t, mgr.StartMatch(
				roomID, tt.variant, "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0,
				true, false, tt.transfer,
			))
			t.Cleanup(func() { mgr.RemoveSession(roomID) })

			gs := mgr.GetStateSnapshot(roomID)
			require.NotNil(t, gs)

			assert.Equal(t, tt.transfer, gs.Rules.DeclarationsTransfer, "the rule config the engine reads")
			assert.Equal(t, tt.transfer, gs.DeclarationsTransfer, "and the wire flag the client renders")
			assert.True(t, gs.Rules.DeclarationsEnabled, "the declarations toggle is independent")
			assert.False(t, gs.Rules.StopAtTarget, "the stop-at-target toggle is independent")

			preset := game.RulesFor(game.Variant(tt.variant))
			assert.Equal(t, preset.DealShape, gs.Rules.DealShape)
			assert.Equal(t, preset.DeclarationTiming, gs.Rules.DeclarationTiming)
			assert.Equal(t, preset.TieRule, gs.Rules.TieRule)
		})
	}
}

// TestAbandonedMatchRecord_CarriesDeclarationsTransfer checks a match closed by a reconnect timeout records
// the rule it was played under, so its history row is not mislabelled.
func TestAbandonedMatchRecord_CarriesDeclarationsTransfer(t *testing.T) {
	for _, want := range []bool{true, false} {
		const roomID = uint(131)
		repo := newMockMatchRepo()
		mgr := match.NewManager(&hubSpy{}, repo)
		mgr.SetBotDelayForTest(time.Minute, time.Minute)
		require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", mixedPlayers(2, 3), "relaxed", 0, 10, 1, 0, true, false, want))

		mgr.HandleDisconnect(10)
		require.True(t, waitFor(5*time.Second, func() bool { return !mgr.HasSession(roomID) }),
			"match must abandon after the reconnect window")

		matches := repo.getMatches()
		require.Len(t, matches, 1)
		assert.Equal(t, "abandoned", matches[0].Status)
		assert.Equal(t, want, matches[0].DeclarationsTransfer)
	}
}

// TestReconciledMatchRecord_CarriesDeclarationsTransfer checks the abandoned row written for a room left
// playing by a previous process takes the room's rule.
func TestReconciledMatchRecord_CarriesDeclarationsTransfer(t *testing.T) {
	for _, want := range []bool{true, false} {
		matchRepo := newMockMatchRepo()
		mgr := match.NewManager(&hubSpy{}, matchRepo)
		stub := newStubRoomRepo()
		stub.rooms = []match.StaleRoom{{ID: 132, Variant: "bitola", MatchMode: "1001", DeclarationsTransfer: want}}
		stub.botSeats[132] = []int{0, 1, 2, 3}

		require.NoError(t, mgr.ReconcileStaleRooms(stub))

		matches := matchRepo.getMatches()
		require.Len(t, matches, 1)
		assert.Equal(t, want, matches[0].DeclarationsTransfer)
	}
}
