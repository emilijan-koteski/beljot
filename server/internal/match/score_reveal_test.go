package match_test

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
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120, 0, true, false, true, false))

	injectHandCompletePause(t, mgr, time.Now().Add(600*time.Millisecond))

	mgr.HandleAction(&ws.Client{UserID: 10}, ws.WSMessage{Type: "action:continue", Payload: []byte(`{}`)})
	time.Sleep(20 * time.Millisecond)

	mgr.HandleAction(&ws.Client{UserID: 20}, ws.WSMessage{Type: "action:play_card", Payload: []byte(`{"cardId":"AS"}`)})
	time.Sleep(20 * time.Millisecond)

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
	mgr.SetDealGraceForTest(false)
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 1, 10, 120, 0, true, false, true, false))

	injectHandCompletePause(t, mgr, time.Now().Add(time.Hour))

	for _, uid := range []uint{10, 20, 30, 40} {
		mgr.HandleAction(&ws.Client{UserID: uid}, ws.WSMessage{Type: "action:continue", Payload: []byte(`{}`)})
		time.Sleep(10 * time.Millisecond)
	}
	mid := mgr.GetStateSnapshot(100)
	require.NotNil(t, mid)
	require.Equal(t, game.PhaseBidding, mid.Phase, "all-acks should deal the next hand")
	firstBidder := mid.ActivePlayerSeat

	mgr.HandleAction(&ws.Client{UserID: 20}, ws.WSMessage{Type: "action:continue", Payload: []byte(`{}`)})
	time.Sleep(20 * time.Millisecond)

	for _, call := range hub.snapshot() {
		if len(call.userIDs) == 1 && call.userIDs[0] == 20 {
			var msg ws.WSMessage
			require.NoError(t, json.Unmarshal(call.msg, &msg))
			assert.NotContains(t, msg.Type, "error",
				"late continue must not produce an error event (got %s)", msg.Type)
		}
	}

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
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120, 0, true, false, true, false))

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
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120, 0, true, false, true, false))

	before := len(hub.snapshot())
	mgr.SyncStateOnConnect(999)
	assert.Len(t, hub.snapshot(), before, "no send for users outside a session")
}
