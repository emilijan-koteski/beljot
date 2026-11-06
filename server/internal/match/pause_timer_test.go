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
	require.NotNil(t, gs.PlayerPauseExpiresAt[1])
	assert.WithinDuration(t, time.Now().Add(120*time.Second), *gs.PlayerPauseExpiresAt[1], 2*time.Second)
	assert.Equal(t, 120, gs.PauseWindowSec)
}

func TestPause_ASecondPauseRunsItsOwnClock(t *testing.T) {
	mgr := startPauseMatch(t, 1102, 120)
	require.NoError(t, mgr.ApplyActionForTest(1102, game.Action{Type: game.ActionPause, PlayerSeat: 1}))
	first := *mgr.GetStateSnapshot(1102).PlayerPauseExpiresAt[1]
	time.Sleep(20 * time.Millisecond)

	require.NoError(t, mgr.ApplyActionForTest(1102, game.Action{Type: game.ActionPause, PlayerSeat: 2}))
	err := mgr.ApplyActionForTest(1102, game.Action{Type: game.ActionPause, PlayerSeat: 1})

	assert.ErrorIs(t, err, apperr.ErrPauseExhausted, "one pause per player per match")
	gs := mgr.GetStateSnapshot(1102)
	assert.Equal(t, first, *gs.PlayerPauseExpiresAt[1], "the first pause keeps its own end")
	require.NotNil(t, gs.PlayerPauseExpiresAt[2])
	assert.True(t, gs.PlayerPauseExpiresAt[2].After(first), "the second pause starts its own two minutes")
}

func TestPause_ResumingYourOwnLeavesTheOtherRunning(t *testing.T) {
	mgr := startPauseMatch(t, 1103, 120)
	require.NoError(t, mgr.ApplyActionForTest(1103, game.Action{Type: game.ActionPause, PlayerSeat: 1}))
	require.NoError(t, mgr.ApplyActionForTest(1103, game.Action{Type: game.ActionPause, PlayerSeat: 2}))

	require.NoError(t, mgr.ApplyActionForTest(1103, game.Action{Type: game.ActionUnpause, PlayerSeat: 1}))
	gs := mgr.GetStateSnapshot(1103)
	assert.Equal(t, game.PhasePaused, gs.Phase, "seat 2's pause is still running")
	assert.Nil(t, gs.PlayerPauseExpiresAt[1])
	assert.NotNil(t, gs.PlayerPauseExpiresAt[2])
	assert.ErrorIs(t, mgr.ApplyActionForTest(1103, game.Action{Type: game.ActionUnpause, PlayerSeat: 1}), apperr.ErrNoActivePause)

	require.NoError(t, mgr.ApplyActionForTest(1103, game.Action{Type: game.ActionUnpause, PlayerSeat: 2}))
	resumed := mgr.GetStateSnapshot(1103)
	assert.Equal(t, game.PhaseBidding, resumed.Phase)
	assert.Equal(t, [4]*time.Time{}, resumed.PlayerPauseExpiresAt)
}

func TestPause_EachPauseRunsOutOnItsOwn(t *testing.T) {
	mgr := startPauseMatch(t, 1104, 1)
	require.NoError(t, mgr.ApplyActionForTest(1104, game.Action{Type: game.ActionPause, PlayerSeat: 1}))
	time.Sleep(600 * time.Millisecond)
	require.NoError(t, mgr.ApplyActionForTest(1104, game.Action{Type: game.ActionPause, PlayerSeat: 3}))

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && mgr.GetStateSnapshot(1104).PausedPlayers[1] {
		time.Sleep(20 * time.Millisecond)
	}
	mid := mgr.GetStateSnapshot(1104)
	require.False(t, mid.PausedPlayers[1], "seat 1's pause ran out")
	if mid.Phase == game.PhasePaused {
		assert.True(t, mid.PausedPlayers[3], "seat 3's later pause keeps the match paused")
	}

	gs := waitForPhaseNot(t, mgr, 1104, game.PhasePaused, 4*time.Second)
	assert.Equal(t, game.PhaseBidding, gs.Phase)
	assert.Equal(t, [4]bool{}, gs.PausedPlayers)
	assert.True(t, gs.PauseUsed[1])
	assert.True(t, gs.PauseUsed[3])
}

func TestPause_KeepsItsClockThroughARejectedAction(t *testing.T) {
	mgr := startPauseMatch(t, 1105, 1)
	require.NoError(t, mgr.ApplyActionForTest(1105, game.Action{Type: game.ActionPause, PlayerSeat: 1}))

	err := mgr.ApplyActionForTest(1105, game.Action{Type: game.ActionPassTrump, PlayerSeat: 0})
	require.ErrorIs(t, err, apperr.ErrGamePaused)

	waitForPhaseNot(t, mgr, 1105, game.PhasePaused, 4*time.Second)
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
	gs.PlayerPauseExpiresAt[0] = &past
	mgr.SetGameStateForTest(1107, gs)
	mgr.HandleReconnect(20)

	resumed := mgr.GetStateSnapshot(1107)
	assert.Equal(t, game.PhaseBidding, resumed.Phase)
	assert.Equal(t, [4]bool{}, resumed.PausedPlayers)
	assert.Nil(t, resumed.PlayerPauseExpiresAt[0])
}

func TestPause_StillRunningAfterAReconnectReturns(t *testing.T) {
	mgr := startPauseMatch(t, 1108, 120)
	require.NoError(t, mgr.ApplyActionForTest(1108, game.Action{Type: game.ActionPause, PlayerSeat: 0}))
	expiry := *mgr.GetStateSnapshot(1108).PlayerPauseExpiresAt[0]

	mgr.HandleDisconnect(20)
	mgr.HandleReconnect(20)

	gs := mgr.GetStateSnapshot(1108)
	assert.Equal(t, game.PhasePaused, gs.Phase)
	assert.Equal(t, expiry, *gs.PlayerPauseExpiresAt[0], "the pause clock keeps its original end")
}

func TestPause_APausersDisconnectEndsOnlyTheirPause(t *testing.T) {
	mgr := startPauseMatch(t, 1109, 120)
	require.NoError(t, mgr.ApplyActionForTest(1109, game.Action{Type: game.ActionPause, PlayerSeat: 0}))
	require.NoError(t, mgr.ApplyActionForTest(1109, game.Action{Type: game.ActionPause, PlayerSeat: 1}))

	mgr.HandleDisconnect(20)
	mgr.HandleReconnect(20)

	gs := mgr.GetStateSnapshot(1109)
	assert.Equal(t, game.PhasePaused, gs.Phase, "seat 0's pause is still running")
	assert.Equal(t, [4]bool{true, false, false, false}, gs.PausedPlayers)
	assert.Nil(t, gs.PlayerPauseExpiresAt[1])
}
