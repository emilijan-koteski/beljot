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
				true, false, tt.transfer, false,
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

type roomRuleFlags struct {
	name                 string
	declarationsEnabled  bool
	stopAtTarget         bool
	declarationsTransfer bool
	clockwise            bool
}

var everyPairOfRuleFlagsDiffers = []roomRuleFlags{
	{name: "room defaults", declarationsEnabled: true, stopAtTarget: false, declarationsTransfer: true, clockwise: false},
	{name: "no declarations, stop at target, clockwise", declarationsEnabled: false, stopAtTarget: true, declarationsTransfer: true, clockwise: true},
	{name: "stop at target and declarations voided", declarationsEnabled: true, stopAtTarget: true, declarationsTransfer: false, clockwise: false},
}

// assertRecordedRuleFlags checks a persisted match row carries every room rule flag.
func assertRecordedRuleFlags(t *testing.T, want roomRuleFlags, got *match.Match) {
	t.Helper()
	assert.Equal(t, want.declarationsEnabled, got.DeclarationsEnabled, "declarationsEnabled")
	assert.Equal(t, want.stopAtTarget, got.StopAtTarget, "stopAtTarget")
	assert.Equal(t, want.declarationsTransfer, got.DeclarationsTransfer, "declarationsTransfer")
	assert.Equal(t, want.clockwise, got.Clockwise, "clockwise")
}

// TestAbandonedMatchRecord_CarriesRuleFlags checks a match closed by a reconnect timeout records every room
// rule it was played under, so its history row shows the right chips and no two flags are crossed.
func TestAbandonedMatchRecord_CarriesRuleFlags(t *testing.T) {
	for _, flags := range everyPairOfRuleFlagsDiffers {
		t.Run(flags.name, func(t *testing.T) {
			const roomID = uint(131)
			repo := newMockMatchRepo()
			mgr := match.NewManager(&hubSpy{}, repo)
			mgr.SetBotDelayForTest(time.Minute, time.Minute)
			require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", mixedPlayers(2, 3), "relaxed", 0, 10, 1, 0,
				flags.declarationsEnabled, flags.stopAtTarget, flags.declarationsTransfer, flags.clockwise))

			mgr.HandleDisconnect(10)
			require.True(t, waitFor(5*time.Second, func() bool { return !mgr.HasSession(roomID) }),
				"match must abandon after the reconnect window")

			matches := repo.getMatches()
			require.Len(t, matches, 1)
			assert.Equal(t, "abandoned", matches[0].Status)
			assertRecordedRuleFlags(t, flags, matches[0])
		})
	}
}

// TestReconciledMatchRecord_CarriesRuleFlags checks the abandoned row written for a room left playing by a
// previous process takes every room rule from the stale room.
func TestReconciledMatchRecord_CarriesRuleFlags(t *testing.T) {
	for _, flags := range everyPairOfRuleFlagsDiffers {
		t.Run(flags.name, func(t *testing.T) {
			matchRepo := newMockMatchRepo()
			mgr := match.NewManager(&hubSpy{}, matchRepo)
			stub := newStubRoomRepo()
			stub.rooms = []match.StaleRoom{{
				ID: 132, Variant: "bitola", MatchMode: "1001",
				DeclarationsEnabled:  flags.declarationsEnabled,
				StopAtTarget:         flags.stopAtTarget,
				DeclarationsTransfer: flags.declarationsTransfer,
				Clockwise:            flags.clockwise,
			}}
			stub.botSeats[132] = []int{0, 1, 2, 3}

			require.NoError(t, mgr.ReconcileStaleRooms(stub))

			matches := matchRepo.getMatches()
			require.Len(t, matches, 1)
			assertRecordedRuleFlags(t, flags, matches[0])
		})
	}
}
