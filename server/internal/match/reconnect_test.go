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

	// Start game with per-move timer (1s short for tests) and 120s reconnect window
	err := mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120)
	require.NoError(t, err)

	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)

	// Pick trump to reach playing phase
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

	// Disconnect the specified player
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

	// Reconnect the player
	mgr.HandleReconnect(disconnectedUserID)
	time.Sleep(100 * time.Millisecond)

	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)

	// Phase restored from disconnected to playing
	assert.Equal(t, game.PhasePlaying, state.Phase)
	assert.Equal(t, "", string(state.PreviousPhase))
	// Disconnect fields cleared
	assert.Equal(t, -1, state.DisconnectedSeat)
	assert.Nil(t, state.ReconnectExpiresAt)
	// Player marked connected
	assert.True(t, state.Players[0].Connected)
}

func TestHandleReconnect_RestoresTurnTimer(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	mgr, disconnectedUserID := setupDisconnectedGame(t, hub, 0)

	// Check TurnTimeRemaining was captured during disconnect
	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)
	assert.Nil(t, state.TurnExpiresAt, "timer should be paused during disconnect")

	// Reconnect
	mgr.HandleReconnect(disconnectedUserID)
	time.Sleep(100 * time.Millisecond)

	state = mgr.GetStateSnapshot(100)
	require.NotNil(t, state)

	// Turn timer should be restored
	assert.NotNil(t, state.TurnExpiresAt, "per-move timer should be restored on reconnect")
	assert.True(t, state.TurnExpiresAt.After(time.Now()), "timer should be in the future")
	assert.Equal(t, int64(0), state.TurnTimeRemaining, "TurnTimeRemaining should be reset to 0")
}

// TestHandleDisconnect_DuringHandComplete_PausesAndReconnectRestores guards the
// production jam fix: a socket drop during the score-reveal (hand-complete)
// pause must pause the table (PhaseDisconnected), not be silently skipped.
// Otherwise the dropped seat stays Connected=true and allConnectedReady keeps
// waiting for an acknowledgement that can never arrive — jamming the table for
// the full auto-continue window (the reported mobile-client symptom). Reconnect
// restores the pause so the score dialog reappears and play resumes.
func TestHandleDisconnect_DuringHandComplete_PausesAndReconnectRestores(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120))

	// Inject the score-reveal pause: hand scored, holding for continue, with the
	// other three players already acknowledged.
	gs := mgr.GetStateSnapshot(100)
	require.NotNil(t, gs)
	gs.Phase = game.PhaseHandComplete
	gs.PreviousPhase = ""
	gs.DisconnectedSeat = -1
	gs.TurnExpiresAt = nil
	gs.HandCompleteReady = [4]bool{false, true, true, true}
	mgr.SetGameStateForTest(100, gs)

	// Seat 0 (the only un-acked player) drops mid-pause.
	mgr.HandleDisconnect(uint(10))
	time.Sleep(100 * time.Millisecond)

	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)
	assert.Equal(t, game.PhaseDisconnected, state.Phase, "a drop during hand-complete must pause the table, not be skipped")
	assert.Equal(t, game.PhaseHandComplete, state.PreviousPhase, "the score-reveal pause is the restore target")
	assert.False(t, state.Players[0].Connected, "dropped seat must be marked disconnected")
	assert.Equal(t, 0, state.DisconnectedSeat)

	// Reconnect restores the pause so the dialog reappears and play can resume.
	mgr.HandleReconnect(uint(10))
	time.Sleep(100 * time.Millisecond)

	state = mgr.GetStateSnapshot(100)
	require.NotNil(t, state)
	assert.Equal(t, game.PhaseHandComplete, state.Phase, "reconnect restores the hand-complete pause")
	assert.Equal(t, "", string(state.PreviousPhase))
	assert.True(t, state.Players[0].Connected, "reconnected seat marked connected")
	assert.Equal(t, -1, state.DisconnectedSeat)
}

// TestHandComplete_AckDoesNotExtendAutoContinueDeadline locks in the fix that a
// player's continue acknowledgement must NOT push back the auto-continue
// deadline. The deadline is fixed when the score-reveal pause starts; if each
// ack reset it, the present players' clicks would keep extending the wait for a
// missing/idle seat — the production jam. Deterministic: asserts the stored
// deadline directly, with no timer wait.
func TestHandComplete_AckDoesNotExtendAutoContinueDeadline(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120))

	// Put the table in the score-reveal pause with nobody acknowledged yet.
	gs := mgr.GetStateSnapshot(100)
	require.NotNil(t, gs)
	gs.Phase = game.PhaseHandComplete
	gs.HandCompleteReady = [4]bool{}
	gs.TurnExpiresAt = nil
	mgr.SetGameStateForTest(100, gs)

	// Seed the fixed deadline far in the future (as entry into the pause does),
	// so the auto-continue timer can't fire during the test.
	deadline := time.Now().Add(time.Hour)
	mgr.SetHandCompleteExpiresAtForTest(100, deadline)

	// Three of four players acknowledge in turn. None completes readiness (seat 3
	// never acks), so the phase stays hand_complete and each ack re-enters the
	// auto-continue arming branch — which must leave the deadline untouched.
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

	// Start game with very short reconnect window (1 second)
	err := mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 1)
	require.NoError(t, err)

	// Pick trump to reach playing phase
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

	// Disconnect seat 0
	mgr.HandleDisconnect(uint(10))
	time.Sleep(100 * time.Millisecond)

	state = mgr.GetStateSnapshot(100)
	require.Equal(t, game.PhaseDisconnected, state.Phase)

	// Wait for reconnect window to expire — handleReconnectTimeout fires
	// expiryGrace (400ms) past the 1s window and removes the session
	// (Story 5.5 match abandonment)
	time.Sleep(2200 * time.Millisecond)

	// Session is removed after timeout-triggered abandonment
	assert.False(t, mgr.HasSession(100), "session should be removed after timeout")

	// Attempt reconnect — HandleReconnect should be a no-op (user no longer in userToRoom)
	mgr.HandleReconnect(uint(10))
	time.Sleep(100 * time.Millisecond)

	// Session still doesn't exist
	assert.Nil(t, mgr.GetStateSnapshot(100))

	// Match was persisted as abandoned
	matches := repo.getMatches()
	require.Len(t, matches, 1)
	assert.Equal(t, "abandoned", matches[0].Status)
}

func TestHandleReconnect_RejectsWrongUser(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	mgr, _ := setupDisconnectedGame(t, hub, 0)

	// Try to reconnect as a different user (seat 1, userID 20)
	mgr.HandleReconnect(uint(20))
	time.Sleep(100 * time.Millisecond)

	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)
	// Still disconnected — wrong user tried to reconnect
	assert.Equal(t, game.PhaseDisconnected, state.Phase)
	assert.Equal(t, 0, state.DisconnectedSeat)
	assert.False(t, state.Players[0].Connected)
}

// Per-seat windows: each disconnect gets its own `reconnectWindowSec` from
// its drop time. PlayerReconnectExpiresAt[seat] holds that seat's expiry; the
// legacy single-pointer ReconnectExpiresAt + DisconnectedSeat are recomputed
// to point at whichever seat closes soonest.
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

	// Delay so the second drop's window is meaningfully later than the first.
	time.Sleep(50 * time.Millisecond)
	secondaryUserID := state.Players[1].UserID
	mgr.HandleDisconnect(secondaryUserID)
	time.Sleep(100 * time.Millisecond)

	state = mgr.GetStateSnapshot(100)
	require.NotNil(t, state.PlayerReconnectExpiresAt[0], "primary expiry survives secondary drop")
	require.NotNil(t, state.PlayerReconnectExpiresAt[1], "secondary gets its own expiry")
	secondaryExpiry := *state.PlayerReconnectExpiresAt[1]

	// Secondary's window must be later than primary's — they got their own
	// `reconnectWindowSec` from their own drop, not the leftover of primary's.
	assert.True(t, secondaryExpiry.After(primaryExpiry),
		"secondary expiry %v should be after primary expiry %v", secondaryExpiry, primaryExpiry)

	// The earliest-expiry derivation (ReconnectExpiresAt + DisconnectedSeat)
	// points at the *primary* seat — the one whose clock fires first.
	assert.Equal(t, 0, state.DisconnectedSeat, "earliest-expiry pointer = primary seat")
	require.NotNil(t, state.ReconnectExpiresAt)
	assert.True(t, state.ReconnectExpiresAt.Equal(primaryExpiry),
		"ReconnectExpiresAt mirrors primary expiry")
}

// Concurrent-disconnect path: when a second player drops while the first
// player's reconnect window is already running, both seats are flagged
// Connected=false. Each must be able to rejoin in any order — earlier the
// `playerIDs[seat] != userID` guard silently swallowed the secondary's
// reconnect, leaving their client stuck on the splash until the no-active-
// session fallback bounced them to /lobby. With per-seat timers, secondary
// rejoin flips their Connected back to true while primary's window keeps
// running.
func TestHandleReconnect_SecondaryReconnectsBeforePrimary(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	mgr, primaryUserID := setupDisconnectedGame(t, hub, 0)

	// Drop a second player (seat 1). Concurrent disconnect path: marks them
	// Connected=false and reuses the primary's reconnect expiry.
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

	// Reconnect the SECONDARY first — earlier this was a silent no-op.
	mgr.HandleReconnect(secondaryUserID)
	time.Sleep(100 * time.Millisecond)

	state = mgr.GetStateSnapshot(100)
	// Phase stays disconnected — primary hasn't returned yet.
	assert.Equal(t, game.PhaseDisconnected, state.Phase, "phase remains disconnected")
	assert.Equal(t, 0, state.DisconnectedSeat, "primary seat unchanged")
	assert.NotNil(t, state.ReconnectExpiresAt, "primary reconnect window still running")
	// Secondary is now back online; primary still offline.
	assert.True(t, state.Players[1].Connected, "secondary restored")
	assert.False(t, state.Players[0].Connected, "primary still offline")

	// Now reconnect the primary — chained logic should NOT promote anyone
	// (no remaining offline seats), so the game returns to playing.
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

	// No session exists — calling HandleReconnect should not panic
	mgr.HandleReconnect(uint(999))
	// No assertion needed — just verify no panic
}

func TestHandleReconnect_NoOpWhenNotDisconnectedPhase(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	err := mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120)
	require.NoError(t, err)

	// Game is in bidding phase, not disconnected
	state := mgr.GetStateSnapshot(100)
	require.Equal(t, game.PhaseBidding, state.Phase)

	// HandleReconnect should be a no-op
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

	// First reconnect
	mgr.HandleReconnect(disconnectedUserID)
	time.Sleep(100 * time.Millisecond)

	state := mgr.GetStateSnapshot(100)
	require.Equal(t, game.PhasePlaying, state.Phase)

	// Second reconnect (idempotent — phase is no longer disconnected)
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

	// Start with relaxed timer (no per-move timer)
	err := mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120)
	require.NoError(t, err)

	// Pick trump to reach playing phase
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

	// Disconnect seat 0
	mgr.HandleDisconnect(uint(10))
	time.Sleep(100 * time.Millisecond)

	// Reconnect
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

	// Other players remain connected
	assert.True(t, gs.Players[0].Connected)
	assert.True(t, gs.Players[1].Connected)
	assert.True(t, gs.Players[3].Connected)
}

// --- handleReconnectTimeout tests (Story 5.5) ---

// setupDisconnectedGameShortWindow creates a game with a 1-second reconnect window
// and disconnects the specified seat. The reconnect timer fires after ~1s.
func setupDisconnectedGameShortWindow(t *testing.T, hub *ws.Hub, disconnectSeat int) (*match.Manager, *mockMatchRepo, uint) {
	t.Helper()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	// Start game with 1-second reconnect window for fast timeout
	err := mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 1)
	require.NoError(t, err)

	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)

	// Pick trump to reach playing phase
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

	// Disconnect the specified player
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

	// Wait for the 1-second reconnect window to expire
	time.Sleep(2200 * time.Millisecond)

	// Session should be removed after abandonment
	assert.False(t, mgr.HasSession(100), "session should be removed after timeout")
	assert.Nil(t, mgr.GetStateSnapshot(100), "state snapshot should be nil after session removed")
}

func TestReconnectTimeout_PersistsAbandonedMatch(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	_, repo, _ := setupDisconnectedGameShortWindow(t, hub, 2)

	// Wait for the 1-second reconnect window to expire
	time.Sleep(2200 * time.Millisecond)

	// Match should be persisted with abandoned status
	matches := repo.getMatches()
	require.Len(t, matches, 1, "one match should be persisted")
	m := matches[0]
	assert.Equal(t, "abandoned", m.Status)
	assert.NotNil(t, m.AbandonedBy)
	assert.Equal(t, uint(30), *m.AbandonedBy) // seat 2 = userID 30
	assert.Equal(t, uint(100), m.RoomID)
	assert.Equal(t, "bitola", m.Variant)
	assert.Equal(t, "1001", m.MatchMode)
}

func TestReconnectTimeout_NoOpWhenReconnected(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	mgr, repo, disconnectedUserID := setupDisconnectedGameShortWindow(t, hub, 0)

	// Reconnect before timeout
	mgr.HandleReconnect(disconnectedUserID)
	time.Sleep(100 * time.Millisecond)

	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)
	assert.Equal(t, game.PhasePlaying, state.Phase)

	// Wait past the original timeout window
	time.Sleep(2200 * time.Millisecond)

	// Session should still exist (timeout was cancelled by reconnection)
	assert.True(t, mgr.HasSession(100), "session should still exist after reconnection")
	matches := repo.getMatches()
	assert.Len(t, matches, 0, "no match should be persisted when reconnected")
}

func TestReconnectTimeout_RemovesSession(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	mgr, _, _ := setupDisconnectedGameShortWindow(t, hub, 1)

	// Wait for timeout
	time.Sleep(2200 * time.Millisecond)

	assert.False(t, mgr.HasSession(100))
	assert.Nil(t, mgr.GetStateSnapshot(100))
}
