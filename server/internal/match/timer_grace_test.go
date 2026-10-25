package match_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/ws"
)

// Every match message stamps the server wall clock on the envelope so clients can
// correct their countdowns for clock skew.
func TestMatchMessages_CarryServerNow(t *testing.T) {
	hub := &hubSpy{}
	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	before := time.Now()
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120, 0, true, false, true, false))
	after := time.Now()

	calls := hub.snapshot()
	require.NotEmpty(t, calls, "StartMatch must broadcast the initial state")
	for _, call := range calls {
		var msg ws.WSMessage
		require.NoError(t, json.Unmarshal(call.msg, &msg))
		require.NotNil(t, msg.ServerNow, "%s must carry serverNow", msg.Type)
		assert.False(t, msg.ServerNow.Before(before),
			"serverNow must be stamped at send time (got %v before test start)", msg.ServerNow)
		assert.False(t, msg.ServerNow.After(after),
			"serverNow must be stamped at send time (got %v after broadcast returned)", msg.ServerNow)
	}
}

// serverNow must serialize as absent when unset, never as null, because the client
// types it as optional.
func TestWSMessage_ServerNowIsOptionalOnTheWire(t *testing.T) {
	b, err := json.Marshal(ws.WSMessage{Type: "action:continue", Payload: []byte(`{}`)})
	require.NoError(t, err)
	assert.NotContains(t, string(b), "serverNow")
}

// TurnExpiresAt advertises start+duration exactly, while the auto-action fires
// expiryGrace later, so players always see 0 before the server acts.
func TestPerMoveTimer_FiresAfterAdvertisedDeadlineNotBefore(t *testing.T) {
	hub := &hubSpy{}
	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	mgr.SetDealGraceForTest(false)

	start := time.Now()
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 1, 10, 120, 0, true, false, true, false))

	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)
	firstBidder := state.ActivePlayerSeat

	require.NotNil(t, state.TurnExpiresAt)
	assert.WithinDuration(t, start.Add(1*time.Second), *state.TurnExpiresAt, 200*time.Millisecond,
		"advertised TurnExpiresAt must be start+duration, without the grace")

	time.Sleep(700 * time.Millisecond)
	mid := mgr.GetStateSnapshot(100)
	require.NotNil(t, mid)
	assert.Equal(t, firstBidder, mid.ActivePlayerSeat,
		"auto-action must not fire before the advertised deadline")

	time.Sleep(1500 * time.Millisecond)
	after := mgr.GetStateSnapshot(100)
	require.NotNil(t, after)
	assert.NotEqual(t, firstBidder, after.ActivePlayerSeat,
		"auto-action must fire once the advertised deadline plus grace elapse")
}
