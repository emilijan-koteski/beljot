package match_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/ws"
)

func startPauseMatch(t *testing.T, roomID uint, windowSec int) *match.Manager {
	t.Helper()
	hub := ws.NewHub()
	go hub.Run()
	t.Cleanup(hub.Shutdown)
	mgr := match.NewManager(hub, newMockMatchRepo())
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, windowSec, 0, true, false, true, false))
	return mgr
}

func waitForPhaseNot(t *testing.T, mgr *match.Manager, roomID uint, phase game.Phase, within time.Duration) *game.GameState {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if gs := mgr.GetStateSnapshot(roomID); gs != nil && gs.Phase != phase {
			return gs
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("room %d still in phase %s after %s", roomID, phase, within)
	return nil
}

func TestPause_RunsForTheReconnectWindow(t *testing.T) {
	mgr := startPauseMatch(t, 1101, 120)

	require.NoError(t, mgr.ApplyActionForTest(1101, game.Action{Type: game.ActionPause, PlayerSeat: 1}))

	gs := mgr.GetStateSnapshot(1101)
	require.Equal(t, game.PhasePaused, gs.Phase)
	require.NotNil(t, gs.PauseExpiresAt)
	assert.WithinDuration(t, time.Now().Add(120*time.Second), *gs.PauseExpiresAt, 2*time.Second)
	assert.Equal(t, 120, gs.PauseTotalSec)
}

func TestExtendPause_AddsOneMoreWindowPerPlayer(t *testing.T) {
	mgr := startPauseMatch(t, 1102, 120)
	require.NoError(t, mgr.ApplyActionForTest(1102, game.Action{Type: game.ActionPause, PlayerSeat: 1}))
	before := *mgr.GetStateSnapshot(1102).PauseExpiresAt

	require.NoError(t, mgr.ApplyActionForTest(1102, game.Action{Type: game.ActionExtendPause, PlayerSeat: 2}))
	require.NoError(t, mgr.ApplyActionForTest(1102, game.Action{Type: game.ActionExtendPause, PlayerSeat: 1}))
	err := mgr.ApplyActionForTest(1102, game.Action{Type: game.ActionExtendPause, PlayerSeat: 2})

	assert.ErrorIs(t, err, apperr.ErrPauseExtendUsed)
	gs := mgr.GetStateSnapshot(1102)
	assert.Equal(t, before.Add(240*time.Second), *gs.PauseExpiresAt)
	assert.Equal(t, 360, gs.PauseTotalSec)
	assert.Equal(t, [4]bool{false, true, true, false}, gs.PauseExtended)
}

func TestPause_ResumesWhenTheTimeRunsOut(t *testing.T) {
	mgr := startPauseMatch(t, 1103, 1)
	require.NoError(t, mgr.ApplyActionForTest(1103, game.Action{Type: game.ActionPause, PlayerSeat: 1}))
	require.NoError(t, mgr.ApplyActionForTest(1103, game.Action{Type: game.ActionExtendPause, PlayerSeat: 3}))

	gs := waitForPhaseNot(t, mgr, 1103, game.PhasePaused, 4*time.Second)

	assert.Equal(t, game.PhaseBidding, gs.Phase)
	assert.Equal(t, [4]bool{}, gs.PausedPlayers, "every hold ends with the time")
	assert.Nil(t, gs.PauseExpiresAt)
	assert.True(t, gs.PauseUsed[1], "the pause stays used for the rest of the match")
}

func TestPause_KeepsItsClockThroughARejectedAction(t *testing.T) {
	mgr := startPauseMatch(t, 1104, 1)
	require.NoError(t, mgr.ApplyActionForTest(1104, game.Action{Type: game.ActionPause, PlayerSeat: 1}))

	err := mgr.ApplyActionForTest(1104, game.Action{Type: game.ActionPassTrump, PlayerSeat: 0})
	require.ErrorIs(t, err, apperr.ErrGamePaused)

	waitForPhaseNot(t, mgr, 1104, game.PhasePaused, 4*time.Second)
}

func TestPause_StaysUntilEveryHolderResumes(t *testing.T) {
	mgr := startPauseMatch(t, 1105, 120)
	require.NoError(t, mgr.ApplyActionForTest(1105, game.Action{Type: game.ActionPause, PlayerSeat: 1}))
	require.NoError(t, mgr.ApplyActionForTest(1105, game.Action{Type: game.ActionExtendPause, PlayerSeat: 2}))

	require.NoError(t, mgr.ApplyActionForTest(1105, game.Action{Type: game.ActionUnpause, PlayerSeat: 1}))
	assert.Equal(t, game.PhasePaused, mgr.GetStateSnapshot(1105).Phase, "seat 2 still holds the pause it extended")

	require.NoError(t, mgr.ApplyActionForTest(1105, game.Action{Type: game.ActionUnpause, PlayerSeat: 2}))
	gs := mgr.GetStateSnapshot(1105)
	assert.Equal(t, game.PhaseBidding, gs.Phase)
	assert.Nil(t, gs.PauseExpiresAt)
}

func TestPause_TheHostCannotResumeForEveryone(t *testing.T) {
	mgr := startPauseMatch(t, 1106, 120)
	require.NoError(t, mgr.ApplyActionForTest(1106, game.Action{Type: game.ActionPause, PlayerSeat: 1}))

	assert.Error(t, mgr.ApplyActionForTest(1106, game.Action{Type: "owner_unpause", PlayerSeat: 0}))
	assert.ErrorIs(t, mgr.ApplyActionForTest(1106, game.Action{Type: game.ActionUnpause, PlayerSeat: 0}), apperr.ErrNoActivePause)
	assert.Equal(t, game.PhasePaused, mgr.GetStateSnapshot(1106).Phase)
}

func TestPause_ThatRanOutDuringADisconnectDoesNotReturnOnReconnect(t *testing.T) {
	mgr := startPauseMatch(t, 1107, 120)
	require.NoError(t, mgr.ApplyActionForTest(1107, game.Action{Type: game.ActionPause, PlayerSeat: 0}))
	mgr.HandleDisconnect(20)
	require.Equal(t, game.PhaseDisconnected, mgr.GetStateSnapshot(1107).Phase)

	gs := mgr.GetStateSnapshot(1107)
	past := time.Now().Add(-time.Second)
	gs.PauseExpiresAt = &past
	mgr.SetGameStateForTest(1107, gs)
	mgr.HandleReconnect(20)

	resumed := mgr.GetStateSnapshot(1107)
	assert.Equal(t, game.PhaseBidding, resumed.Phase)
	assert.Equal(t, [4]bool{}, resumed.PausedPlayers)
	assert.Nil(t, resumed.PauseExpiresAt)
}

func TestPause_StillRunningAfterAReconnectReturns(t *testing.T) {
	mgr := startPauseMatch(t, 1108, 120)
	require.NoError(t, mgr.ApplyActionForTest(1108, game.Action{Type: game.ActionPause, PlayerSeat: 0}))
	expiry := *mgr.GetStateSnapshot(1108).PauseExpiresAt

	mgr.HandleDisconnect(20)
	mgr.HandleReconnect(20)

	gs := mgr.GetStateSnapshot(1108)
	assert.Equal(t, game.PhasePaused, gs.Phase)
	assert.Equal(t, expiry, *gs.PauseExpiresAt, "the pause clock keeps its original end")
}
