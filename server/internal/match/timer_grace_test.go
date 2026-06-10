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

// TestMatchMessages_CarryServerNow locks the clock-sync contract: every match
// message stamps the server wall clock on the envelope so clients can estimate
// their clock offset and render TurnExpiresAt / ReconnectExpiresAt countdowns
// against corrected time instead of a possibly-skewed Date.now().
func TestMatchMessages_CarryServerNow(t *testing.T) {
	hub := &hubSpy{}
	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	before := time.Now()
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120))
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

// TestWSMessage_ServerNowIsOptionalOnTheWire guards the inbound direction:
// client messages carry no serverNow, and the field must serialize as absent
// (omitempty), never as null — the client types the envelope field as
// optional (`serverNow?: string` on WsMessage in wsEvents.ts).
func TestWSMessage_ServerNowIsOptionalOnTheWire(t *testing.T) {
	b, err := json.Marshal(ws.WSMessage{Type: "action:continue", Payload: []byte(`{}`)})
	require.NoError(t, err)
	assert.NotContains(t, string(b), "serverNow")
}

// TestPerMoveTimer_FiresAfterAdvertisedDeadlineNotBefore locks the expiryGrace
// contract: TurnExpiresAt advertises start+duration exactly (clients count down
// to it and reach 0 there), while the server's auto-action fires expiryGrace
// later — never before the advertised deadline. Players must always see "0"
// before the server acts in their name.
func TestPerMoveTimer_FiresAfterAdvertisedDeadlineNotBefore(t *testing.T) {
	hub := &hubSpy{}
	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	start := time.Now()
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 1, 10, 120))

	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)
	firstBidder := state.ActivePlayerSeat

	// The advertised deadline reflects the configured duration only — the
	// grace is the server's private cushion, never added to what clients see.
	require.NotNil(t, state.TurnExpiresAt)
	assert.WithinDuration(t, start.Add(1*time.Second), *state.TurnExpiresAt, 200*time.Millisecond,
		"advertised TurnExpiresAt must be start+duration, without the grace")

	// Before the advertised deadline the seat must be untouched. Checking at
	// 700ms leaves a wide cushion to the 1.4s fire — load-tolerant because
	// AfterFunc can fire late under load, but never early.
	time.Sleep(700 * time.Millisecond)
	mid := mgr.GetStateSnapshot(100)
	require.NotNil(t, mid)
	assert.Equal(t, firstBidder, mid.ActivePlayerSeat,
		"auto-action must not fire before the advertised deadline")

	// By deadline+grace+settle the auto-pass must have landed.
	time.Sleep(1500 * time.Millisecond)
	after := mgr.GetStateSnapshot(100)
	require.NotNil(t, after)
	assert.NotEqual(t, firstBidder, after.ActivePlayerSeat,
		"auto-action must fire once the advertised deadline plus grace elapse")
}
