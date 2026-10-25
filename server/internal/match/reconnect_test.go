package match_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/ws"
)

func newGameReconnectingFixture(seat int) *game.GameState {
	return testfixtures.NewGameReconnecting(seat)
}

// setupDisconnectedGame creates a game session with per-move timer, picks trump
// to reach playing phase, then disconnects the specified seat. Returns the manager
// and the userID of the disconnected player.
func setupDisconnectedGame(t *testing.T, hub *ws.Hub, disconnectSeat int) (*match.Manager, uint) {
	t.Helper()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	err := mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120, 0, true, false, true)
	require.NoError(t, err)

	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)

	activeSeat := state.ActivePlayerSeat
	activeUserID := state.Players[activeSeat].UserID
	client := &ws.Client{UserID: activeUserID}
	msg := ws.WSMessage{
		Type:    "action:pick_trump",
		Payload: []byte(`{}`),
	}
	mgr.HandleAction(client, msg)
	time.Sleep(100 * time.Millisecond)

	state = mgr.GetStateSnapshot(100)
	require.NotNil(t, state)
	require.Equal(t, game.PhasePlaying, state.Phase)

	disconnectedUserID := state.Players[disconnectSeat].UserID
	mgr.HandleDisconnect(disconnectedUserID)
	time.Sleep(100 * time.Millisecond)

	state = mgr.GetStateSnapshot(100)
	require.NotNil(t, state)
	require.Equal(t, game.PhaseDisconnected, state.Phase)
	require.Equal(t, disconnectSeat, state.DisconnectedSeat)
	require.False(t, state.Players[disconnectSeat].Connected)

	return mgr, disconnectedUserID
}

func TestHandleReconnect_RestoresPlayerState(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	mgr, disconnectedUserID := setupDisconnectedGame(t, hub, 0)

	mgr.HandleReconnect(disconnectedUserID)
	time.Sleep(100 * time.Millisecond)

	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)

	assert.Equal(t, game.PhasePlaying, state.Phase)
	assert.Equal(t, "", string(state.PreviousPhase))
	assert.Equal(t, -1, state.DisconnectedSeat)
	assert.Nil(t, state.ReconnectExpiresAt)
	assert.True(t, state.Players[0].Connected)
}

func TestHandleReconnect_RestoresTurnTimer(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	mgr, disconnectedUserID := setupDisconnectedGame(t, hub, 0)

	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)
	assert.Nil(t, state.TurnExpiresAt, "timer should be paused during disconnect")

	mgr.HandleReconnect(disconnectedUserID)
	time.Sleep(100 * time.Millisecond)

	state = mgr.GetStateSnapshot(100)
	require.NotNil(t, state)

	assert.NotNil(t, state.TurnExpiresAt, "per-move timer should be restored on reconnect")
	assert.True(t, state.TurnExpiresAt.After(time.Now()), "timer should be in the future")
	assert.Equal(t, int64(0), state.TurnTimeRemaining, "TurnTimeRemaining should be reset to 0")
}

// TestHandleDisconnect_DuringHandComplete_PausesAndReconnectRestores checks that a
// drop during the score reveal pauses the table and reconnect restores the pause.
func TestHandleDisconnect_DuringHandComplete_PausesAndReconnectRestores(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120, 0, true, false, true))

	gs := mgr.GetStateSnapshot(100)
	require.NotNil(t, gs)
	gs.Phase = game.PhaseHandComplete
	gs.PreviousPhase = ""
	gs.DisconnectedSeat = -1
	gs.TurnExpiresAt = nil
	gs.HandCompleteReady = [4]bool{false, true, true, true}
	mgr.SetGameStateForTest(100, gs)

	mgr.HandleDisconnect(uint(10))
	time.Sleep(100 * time.Millisecond)

	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)
	assert.Equal(t, game.PhaseDisconnected, state.Phase, "a drop during hand-complete must pause the table, not be skipped")
	assert.Equal(t, game.PhaseHandComplete, state.PreviousPhase, "the score-reveal pause is the restore target")
	assert.False(t, state.Players[0].Connected, "dropped seat must be marked disconnected")
	assert.Equal(t, 0, state.DisconnectedSeat)

	mgr.HandleReconnect(uint(10))
	time.Sleep(100 * time.Millisecond)

	state = mgr.GetStateSnapshot(100)
	require.NotNil(t, state)
	assert.Equal(t, game.PhaseHandComplete, state.Phase, "reconnect restores the hand-complete pause")
	assert.Equal(t, "", string(state.PreviousPhase))
	assert.True(t, state.Players[0].Connected, "reconnected seat marked connected")
	assert.Equal(t, -1, state.DisconnectedSeat)
}

// TestHandComplete_AckDoesNotExtendAutoContinueDeadline checks that a continue
// acknowledgement leaves the auto-continue deadline unchanged.
func TestHandComplete_AckDoesNotExtendAutoContinueDeadline(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120, 0, true, false, true))

	gs := mgr.GetStateSnapshot(100)
	require.NotNil(t, gs)
	gs.Phase = game.PhaseHandComplete
	gs.HandCompleteReady = [4]bool{}
	gs.TurnExpiresAt = nil
	mgr.SetGameStateForTest(100, gs)

	deadline := time.Now().Add(time.Hour)
	mgr.SetHandCompleteExpiresAtForTest(100, deadline)

	for _, uid := range []uint{10, 20, 30} {
		mgr.HandleAction(&ws.Client{UserID: uid}, ws.WSMessage{Type: "action:continue", Payload: []byte(`{}`)})
		time.Sleep(20 * time.Millisecond)

		state := mgr.GetStateSnapshot(100)
		require.NotNil(t, state)
		require.Equal(t, game.PhaseHandComplete, state.Phase, "still paused after a partial ack (uid %d)", uid)
		assert.True(t, mgr.HandCompleteExpiresAtForTest(100).Equal(deadline),
			"a continue acknowledgement must not move the auto-continue deadline (uid %d)", uid)
	}
}

func TestHandleReconnect_RejectsExpiredWindow(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	err := mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 1, 0, true, false, true)
	require.NoError(t, err)

	state := mgr.GetStateSnapshot(100)
	activeSeat := state.ActivePlayerSeat
	activeUserID := state.Players[activeSeat].UserID
	client := &ws.Client{UserID: activeUserID}
	msg := ws.WSMessage{
		Type:    "action:pick_trump",
		Payload: []byte(`{}`),
	}
	mgr.HandleAction(client, msg)
	time.Sleep(100 * time.Millisecond)

	mgr.HandleDisconnect(uint(10))
	time.Sleep(100 * time.Millisecond)

	state = mgr.GetStateSnapshot(100)
	require.Equal(t, game.PhaseDisconnected, state.Phase)

	time.Sleep(2200 * time.Millisecond)

	assert.False(t, mgr.HasSession(100), "session should be removed after timeout")

	mgr.HandleReconnect(uint(10))
	time.Sleep(100 * time.Millisecond)

	assert.Nil(t, mgr.GetStateSnapshot(100))

	matches := repo.getMatches()
	require.Len(t, matches, 1)
	assert.Equal(t, "abandoned", matches[0].Status)
}

func TestHandleReconnect_RejectsWrongUser(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	mgr, _ := setupDisconnectedGame(t, hub, 0)

	mgr.HandleReconnect(uint(20))
	time.Sleep(100 * time.Millisecond)

	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)
	assert.Equal(t, game.PhaseDisconnected, state.Phase)
	assert.Equal(t, 0, state.DisconnectedSeat)
	assert.False(t, state.Players[0].Connected)
}

// TestHandleConcurrentDisconnect_AssignsIndependentPerSeatWindows checks that each
// dropped seat gets its own window and the legacy fields track the earliest.
func TestHandleConcurrentDisconnect_AssignsIndependentPerSeatWindows(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	mgr, _ := setupDisconnectedGame(t, hub, 0)

	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state.PlayerReconnectExpiresAt[0], "primary seat has its own expiry")
	primaryExpiry := *state.PlayerReconnectExpiresAt[0]
	for i := 1; i < 4; i++ {
		assert.Nil(t, state.PlayerReconnectExpiresAt[i], "online seats have nil expiry")
	}

	time.Sleep(50 * time.Millisecond)
	secondaryUserID := state.Players[1].UserID
	mgr.HandleDisconnect(secondaryUserID)
	time.Sleep(100 * time.Millisecond)

	state = mgr.GetStateSnapshot(100)
	require.NotNil(t, state.PlayerReconnectExpiresAt[0], "primary expiry survives secondary drop")
	require.NotNil(t, state.PlayerReconnectExpiresAt[1], "secondary gets its own expiry")
	secondaryExpiry := *state.PlayerReconnectExpiresAt[1]

	assert.True(t, secondaryExpiry.After(primaryExpiry),
		"secondary expiry %v should be after primary expiry %v", secondaryExpiry, primaryExpiry)

	assert.Equal(t, 0, state.DisconnectedSeat, "earliest-expiry pointer = primary seat")
	require.NotNil(t, state.ReconnectExpiresAt)
	assert.True(t, state.ReconnectExpiresAt.Equal(primaryExpiry),
		"ReconnectExpiresAt mirrors primary expiry")
}

// TestHandleReconnect_SecondaryReconnectsBeforePrimary checks that the second seat
// to drop can rejoin while the first seat's window keeps running.
func TestHandleReconnect_SecondaryReconnectsBeforePrimary(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	mgr, primaryUserID := setupDisconnectedGame(t, hub, 0)

	state := mgr.GetStateSnapshot(100)
	require.Equal(t, game.PhaseDisconnected, state.Phase)
	require.Equal(t, 0, state.DisconnectedSeat)
	secondaryUserID := state.Players[1].UserID
	mgr.HandleDisconnect(secondaryUserID)
	time.Sleep(100 * time.Millisecond)

	state = mgr.GetStateSnapshot(100)
	require.Equal(t, game.PhaseDisconnected, state.Phase, "phase still disconnected after second drop")
	require.Equal(t, 0, state.DisconnectedSeat, "primary seat unchanged")
	require.False(t, state.Players[0].Connected, "primary still offline")
	require.False(t, state.Players[1].Connected, "secondary marked offline")

	mgr.HandleReconnect(secondaryUserID)
	time.Sleep(100 * time.Millisecond)

	state = mgr.GetStateSnapshot(100)
	assert.Equal(t, game.PhaseDisconnected, state.Phase, "phase remains disconnected")
	assert.Equal(t, 0, state.DisconnectedSeat, "primary seat unchanged")
	assert.NotNil(t, state.ReconnectExpiresAt, "primary reconnect window still running")
	assert.True(t, state.Players[1].Connected, "secondary restored")
	assert.False(t, state.Players[0].Connected, "primary still offline")

	mgr.HandleReconnect(primaryUserID)
	time.Sleep(100 * time.Millisecond)

	state = mgr.GetStateSnapshot(100)
	assert.Equal(t, game.PhasePlaying, state.Phase, "back to playing once both reconnect")
	assert.Equal(t, -1, state.DisconnectedSeat)
	assert.Nil(t, state.ReconnectExpiresAt)
	assert.True(t, state.Players[0].Connected)
	assert.True(t, state.Players[1].Connected)
}

func TestHandleReconnect_NoOpForNonGameUser(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	mgr.HandleReconnect(uint(999))
}

func TestHandleReconnect_NoOpWhenNotDisconnectedPhase(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	err := mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true)
	require.NoError(t, err)

	state := mgr.GetStateSnapshot(100)
	require.Equal(t, game.PhaseBidding, state.Phase)

	mgr.HandleReconnect(uint(10))
	time.Sleep(50 * time.Millisecond)

	state = mgr.GetStateSnapshot(100)
	assert.Equal(t, game.PhaseBidding, state.Phase)
}

func TestHandleReconnect_IdempotentOnSecondCall(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	mgr, disconnectedUserID := setupDisconnectedGame(t, hub, 0)

	mgr.HandleReconnect(disconnectedUserID)
	time.Sleep(100 * time.Millisecond)

	state := mgr.GetStateSnapshot(100)
	require.Equal(t, game.PhasePlaying, state.Phase)

	mgr.HandleReconnect(disconnectedUserID)
	time.Sleep(50 * time.Millisecond)

	state = mgr.GetStateSnapshot(100)
	assert.Equal(t, game.PhasePlaying, state.Phase)
	assert.True(t, state.Players[0].Connected)
}

func TestHandleReconnect_RelaxedTimer_NoTimerRestore(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	err := mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true)
	require.NoError(t, err)

	state := mgr.GetStateSnapshot(100)
	activeSeat := state.ActivePlayerSeat
	activeUserID := state.Players[activeSeat].UserID
	client := &ws.Client{UserID: activeUserID}
	msg := ws.WSMessage{
		Type:    "action:pick_trump",
		Payload: []byte(`{}`),
	}
	mgr.HandleAction(client, msg)
	time.Sleep(100 * time.Millisecond)

	mgr.HandleDisconnect(uint(10))
	time.Sleep(100 * time.Millisecond)

	mgr.HandleReconnect(uint(10))
	time.Sleep(100 * time.Millisecond)

	state = mgr.GetStateSnapshot(100)
	require.NotNil(t, state)
	assert.Equal(t, game.PhasePlaying, state.Phase)
	assert.Nil(t, state.TurnExpiresAt, "relaxed timer should NOT set TurnExpiresAt on reconnect")
}

func TestNewGameReconnectingFixture(t *testing.T) {
	gs := newGameReconnectingFixture(2)

	assert.Equal(t, game.PhaseDisconnected, gs.Phase)
	assert.Equal(t, game.PhasePlaying, gs.PreviousPhase)
	assert.Equal(t, 2, gs.DisconnectedSeat)
	assert.False(t, gs.Players[2].Connected)
	assert.NotNil(t, gs.ReconnectExpiresAt)
	assert.True(t, gs.ReconnectExpiresAt.After(time.Now()))
	assert.Equal(t, int64(15000), gs.TurnTimeRemaining)
	assert.Nil(t, gs.TurnExpiresAt)

	assert.True(t, gs.Players[0].Connected)
	assert.True(t, gs.Players[1].Connected)
	assert.True(t, gs.Players[3].Connected)
}

// setupDisconnectedGameShortWindow creates a game with a 1-second reconnect window
// and disconnects the specified seat. The reconnect timer fires after ~1s.
func setupDisconnectedGameShortWindow(t *testing.T, hub *ws.Hub, disconnectSeat int) (*match.Manager, *mockMatchRepo, uint) {
	t.Helper()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	err := mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 1, 0, true, false, true)
	require.NoError(t, err)

	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)

	activeSeat := state.ActivePlayerSeat
	activeUserID := state.Players[activeSeat].UserID
	client := &ws.Client{UserID: activeUserID}
	msg := ws.WSMessage{
		Type:    "action:pick_trump",
		Payload: []byte(`{}`),
	}
	mgr.HandleAction(client, msg)
	time.Sleep(100 * time.Millisecond)

	state = mgr.GetStateSnapshot(100)
	require.Equal(t, game.PhasePlaying, state.Phase)

	disconnectedUserID := state.Players[disconnectSeat].UserID
	mgr.HandleDisconnect(disconnectedUserID)
	time.Sleep(100 * time.Millisecond)

	state = mgr.GetStateSnapshot(100)
	require.Equal(t, game.PhaseDisconnected, state.Phase)

	return mgr, repo, disconnectedUserID
}

func TestReconnectTimeout_TransitionsToMatchEnd(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	mgr, _, _ := setupDisconnectedGameShortWindow(t, hub, 0)

	time.Sleep(2200 * time.Millisecond)

	assert.False(t, mgr.HasSession(100), "session should be removed after timeout")
	assert.Nil(t, mgr.GetStateSnapshot(100), "state snapshot should be nil after session removed")
}

func TestReconnectTimeout_PersistsAbandonedMatch(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	_, repo, _ := setupDisconnectedGameShortWindow(t, hub, 2)

	time.Sleep(2200 * time.Millisecond)

	matches := repo.getMatches()
	require.Len(t, matches, 1, "one match should be persisted")
	m := matches[0]
	assert.Equal(t, "abandoned", m.Status)
	assert.NotNil(t, m.AbandonedBy)
	assert.Equal(t, uint(30), *m.AbandonedBy)
	assert.Equal(t, 1, m.WinnerTeam, "winner is the non-abandoning team")
	assert.Equal(t, uint(100), m.RoomID)
	assert.Equal(t, "bitola", m.Variant)
	assert.Equal(t, "1001", m.MatchMode)
}

// TestReconnectTimeout_PersistsWinnerTeamOppositeParity covers the other seat
// parity: a team-1 seat abandons, so team 0 must be persisted as the winner.
func TestReconnectTimeout_PersistsWinnerTeamOppositeParity(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	_, repo, _ := setupDisconnectedGameShortWindow(t, hub, 3)

	time.Sleep(2200 * time.Millisecond)

	matches := repo.getMatches()
	require.Len(t, matches, 1, "one match should be persisted")
	m := matches[0]
	assert.Equal(t, "abandoned", m.Status)
	assert.NotNil(t, m.AbandonedBy)
	assert.Equal(t, uint(40), *m.AbandonedBy)
	assert.Equal(t, 0, m.WinnerTeam, "winner is the non-abandoning team")
}

func TestReconnectTimeout_NoOpWhenReconnected(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	mgr, repo, disconnectedUserID := setupDisconnectedGameShortWindow(t, hub, 0)

	mgr.HandleReconnect(disconnectedUserID)
	time.Sleep(100 * time.Millisecond)

	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)
	assert.Equal(t, game.PhasePlaying, state.Phase)

	time.Sleep(2200 * time.Millisecond)

	assert.True(t, mgr.HasSession(100), "session should still exist after reconnection")
	matches := repo.getMatches()
	assert.Len(t, matches, 0, "no match should be persisted when reconnected")
}

func TestReconnectTimeout_RemovesSession(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	mgr, _, _ := setupDisconnectedGameShortWindow(t, hub, 1)

	time.Sleep(2200 * time.Millisecond)

	assert.False(t, mgr.HasSession(100))
	assert.Nil(t, mgr.GetStateSnapshot(100))
}

// TestReconnect_RefreshesDerivedMustPickTrump checks that the reconnect paths'
// direct phase writes refresh the derived MustPickTrump flag.
func TestReconnect_RefreshesDerivedMustPickTrump(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(1288)
	mgr := match.NewManager(hub, newMockMatchRepo())
	require.NoError(t, mgr.StartMatch(roomID, "croatia", "1001", defaultPlayers(), "per-move", 60, 10, 120, 0, true, false, true))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	gs := testfixtures.NewGameCroatianMidBidding(3)
	gs.RoomID = roomID
	seat := gs.ActivePlayerSeat
	gs.DealerSeat = seat
	game.RefreshDerivedFlags(gs)
	require.True(t, gs.MustPickTrump, "the fixture must be the forced-pick state")
	deadline := time.Now().Add(30 * time.Second)
	gs.TurnExpiresAt = &deadline
	gs.TimerDurationSec = 60
	mgr.SetGameStateForTest(roomID, gs)

	userID := gs.Players[seat].UserID
	client := &ws.Client{UserID: userID}

	mgr.HandleAction(client, ws.WSMessage{Type: "action:pause", Payload: []byte(`{}`)})
	paused := mgr.GetStateSnapshot(roomID)
	require.Equal(t, game.PhasePaused, paused.Phase)
	require.False(t, paused.MustPickTrump,
		"nobody is on the bidding clock while paused — this false is correct, and is what "+
			"the restores below must not carry back into PhaseBidding")

	mgr.HandleDisconnect(userID)
	mgr.HandleReconnect(userID)

	back := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, back)
	require.Equal(t, game.PhaseBidding, back.Phase, "the reconnect must restore the bidding turn")
	assert.Equal(t, game.MustPickTrump(back, back.ActivePlayerSeat), back.MustPickTrump,
		"the wire flag must agree with the engine predicate after an out-of-engine phase restore")
	assert.True(t, back.MustPickTrump, "the dealer still has no legal pass")

	_, err := game.ApplyAction(back, game.Action{Type: game.ActionPassTrump, PlayerSeat: back.ActivePlayerSeat})
	assert.Error(t, err, "a pass here is rejected, so the client must never render the control")
}
