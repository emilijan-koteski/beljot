package match_test

// Regression tests for the score-reveal (hand-complete) pause jam class:
//
//  1. An errored action processed during the pause must NOT disarm the
//     auto-continue fallback (the error path used to cancel the timer and
//     never re-arm it — TurnExpiresAt is nil in hand_complete, so the
//     per-move restore branch could not fire — stranding the table when a
//     Connected-but-unresponsive seat never acknowledged).
//  2. A continue that arrives AFTER the pause already advanced (the client's
//     8s auto-ack racing the server's force-advance) is a benign race: it
//     must be ignored silently — no error toast, and the freshly-armed turn
//     timer must not be disturbed.
//  3. A user re-registering a WebSocket while still part of an active session
//     must receive the authoritative state directly. When the hub REPLACES a
//     zombie socket no disconnect handler ever fires, the seat stays
//     Connected, HandleReconnect no-ops — and there is no client→server
//     "request state" message, so without this push the refreshed client
//     would never receive state at all.

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/ws"
)

// injectHandCompletePause puts the session in the score-reveal pause with no
// acknowledgements and seeds the fixed auto-continue deadline.
func injectHandCompletePause(t *testing.T, mgr *match.Manager, deadline time.Time) {
	t.Helper()
	gs := mgr.GetStateSnapshot(100)
	require.NotNil(t, gs)
	gs.Phase = game.PhaseHandComplete
	gs.HandCompleteReady = [4]bool{}
	gs.TurnExpiresAt = nil
	mgr.SetGameStateForTest(100, gs)
	mgr.SetHandCompleteExpiresAtForTest(100, deadline)
}

func TestHandComplete_ErroredActionDoesNotKillAutoContinue(t *testing.T) {
	hub := &hubSpy{}
	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120, 0))

	injectHandCompletePause(t, mgr, time.Now().Add(600*time.Millisecond))

	// One acknowledgement arms the fallback timer against the seeded deadline.
	mgr.HandleAction(&ws.Client{UserID: 10}, ws.WSMessage{Type: "action:continue", Payload: []byte(`{}`)})
	time.Sleep(20 * time.Millisecond)

	// A stale play_card lands during the pause (e.g. a tap while the collect
	// animation still covers the table) → ErrWrongPhase. The error path must
	// re-arm the fallback against the SAME deadline, not leave it dead.
	mgr.HandleAction(&ws.Client{UserID: 20}, ws.WSMessage{Type: "action:play_card", Payload: []byte(`{"cardId":"AS"}`)})
	time.Sleep(20 * time.Millisecond)

	// Past the deadline the fallback must have force-advanced the pause.
	time.Sleep(1 * time.Second)
	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)
	assert.NotEqual(t, game.PhaseHandComplete, state.Phase,
		"an errored action during the pause must not disarm the auto-continue fallback")
}

func TestHandComplete_LateContinueIsSilentlyIgnored(t *testing.T) {
	hub := &hubSpy{}
	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	// 1s per-move window so the auto-pass assertion below stays fast.
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 1, 10, 120, 0))

	// Fallback far away: the advance must come from the four acknowledgements.
	injectHandCompletePause(t, mgr, time.Now().Add(time.Hour))

	for _, uid := range []uint{10, 20, 30, 40} {
		mgr.HandleAction(&ws.Client{UserID: uid}, ws.WSMessage{Type: "action:continue", Payload: []byte(`{}`)})
		time.Sleep(10 * time.Millisecond)
	}
	mid := mgr.GetStateSnapshot(100)
	require.NotNil(t, mid)
	require.Equal(t, game.PhaseBidding, mid.Phase, "all-acks should deal the next hand")
	firstBidder := mid.ActivePlayerSeat

	// A late auto-ack lands while bidding is underway (the 8s client countdown
	// started after the trick-collect sweep, so it can trail the advance).
	mgr.HandleAction(&ws.Client{UserID: 20}, ws.WSMessage{Type: "action:continue", Payload: []byte(`{}`)})
	time.Sleep(20 * time.Millisecond)

	// No error event may be sent to the late acker. Error sends are targeted
	// (SendToUser → a single-recipient call); broadcasts go to all four.
	for _, call := range hub.snapshot() {
		if len(call.userIDs) == 1 && call.userIDs[0] == 20 {
			var msg ws.WSMessage
			require.NoError(t, json.Unmarshal(call.msg, &msg))
			assert.NotContains(t, msg.Type, "error",
				"late continue must not produce an error event (got %s)", msg.Type)
		}
	}

	// The late continue must not have disturbed the turn timer: the
	// unresponsive first bidder still gets auto-passed on schedule (1s window
	// + 400ms expiryGrace, plus settle margin).
	time.Sleep(2200 * time.Millisecond)
	after := mgr.GetStateSnapshot(100)
	require.NotNil(t, after)
	assert.NotEqual(t, firstBidder, after.ActivePlayerSeat,
		"auto-pass should still fire after a late continue")
	assert.Equal(t, 1, after.BiddingPassCount)
}

func TestSyncStateOnConnect_PushesSnapshotToSessionMember(t *testing.T) {
	hub := &hubSpy{}
	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120, 0))

	before := len(hub.snapshot())
	mgr.SyncStateOnConnect(20)

	calls := hub.snapshot()
	require.Len(t, calls, before+1, "exactly one targeted send expected")
	last := calls[len(calls)-1]
	assert.Equal(t, []uint{20}, last.userIDs, "snapshot must go only to the (re)connecting user")

	var msg ws.WSMessage
	require.NoError(t, json.Unmarshal(last.msg, &msg))
	assert.Equal(t, ws.EventMatchState, msg.Type)

	var state game.GameState
	require.NoError(t, json.Unmarshal(msg.Payload, &state))
	assert.Equal(t, uint(100), state.RoomID, "payload must carry the live session state")
}

func TestSyncStateOnConnect_NoOpForNonSessionUser(t *testing.T) {
	hub := &hubSpy{}
	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120, 0))

	before := len(hub.snapshot())
	mgr.SyncStateOnConnect(999) // lobby user, not in any session
	assert.Len(t, hub.snapshot(), before, "no send for users outside a session")
}
