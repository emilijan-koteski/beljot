package emote

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/ws"
)

type fakeGame struct {
	byUser map[uint]userMembership
}

type userMembership struct {
	participants [4]uint
	seat         int
}

func newFakeGame() *fakeGame {
	return &fakeGame{byUser: make(map[uint]userMembership)}
}

// addMatch wires a four-seat match into the fake. Each of the four userIDs
// resolves to (participants, seatIndex) when MatchParticipantsByUser is called.
func (g *fakeGame) addMatch(participants [4]uint) {
	for seat, uid := range participants {
		g.byUser[uid] = userMembership{participants: participants, seat: seat}
	}
}

func (g *fakeGame) MatchParticipantsByUser(userID uint) ([4]uint, int, bool) {
	m, ok := g.byUser[userID]
	if !ok {
		return [4]uint{}, -1, false
	}
	return m.participants, m.seat, true
}

type hubSpy struct {
	mu    sync.Mutex
	calls []hubCall
}

type hubCall struct {
	userIDs []uint
	msg     []byte
}

func (h *hubSpy) BroadcastToUsers(userIDs []uint, msg []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	dup := make([]uint, len(userIDs))
	copy(dup, userIDs)
	dupMsg := make([]byte, len(msg))
	copy(dupMsg, msg)
	h.calls = append(h.calls, hubCall{userIDs: dup, msg: dupMsg})
}

func (h *hubSpy) callCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.calls)
}

func (h *hubSpy) lastCall(t *testing.T) hubCall {
	t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	require.NotEmpty(t, h.calls, "expected at least one broadcast")
	return h.calls[len(h.calls)-1]
}

func emoteAction(t *testing.T, emoteID string) ws.WSMessage {
	t.Helper()
	payload, err := json.Marshal(ws.EmoteRequest{Emote: emoteID})
	require.NoError(t, err)
	return ws.WSMessage{Type: ws.ActionEmote, Payload: payload}
}

func newHandlerWithMatch(participants [4]uint) (*Handler, *hubSpy, *fakeGame) {
	hub := &hubSpy{}
	game := newFakeGame()
	game.addMatch(participants)
	return NewHandler(hub, game), hub, game
}

func TestHandler_IgnoresWrongActionType(t *testing.T) {
	h, hub, _ := newHandlerWithMatch([4]uint{10, 20, 30, 40})

	wrongAction := ws.WSMessage{Type: ws.ActionPlayCard, Payload: json.RawMessage(`{}`)}
	h.HandleAction(&ws.Client{UserID: 10}, wrongAction)

	assert.Equal(t, 0, hub.callCount(),
		"non-emote action types are no-ops (composite routing safety)")
}

func TestHandler_InvalidPayload_NoBroadcast(t *testing.T) {
	h, hub, _ := newHandlerWithMatch([4]uint{10, 20, 30, 40})

	garbage := ws.WSMessage{Type: ws.ActionEmote, Payload: json.RawMessage(`not json`)}
	h.HandleAction(&ws.Client{UserID: 10}, garbage)

	assert.Equal(t, 0, hub.callCount())
}

func TestHandler_UnknownEmote_SilentlyDropped(t *testing.T) {
	h, hub, _ := newHandlerWithMatch([4]uint{10, 20, 30, 40})

	cases := []string{"", "shrug", "THUMBS_UP", "rocket"}
	for _, emote := range cases {
		h.HandleAction(&ws.Client{UserID: 10}, emoteAction(t, emote))
	}

	assert.Equal(t, 0, hub.callCount(),
		"unknown emote IDs do not produce broadcasts")
}

func TestHandler_SenderNotInGame_SilentlyDropped(t *testing.T) {
	hub := &hubSpy{}
	h := NewHandler(hub, newFakeGame())

	h.HandleAction(&ws.Client{UserID: 10}, emoteAction(t, "thumbs_up"))

	assert.Equal(t, 0, hub.callCount(),
		"sender outside any active match → silent drop")
}

func TestHandler_ValidEmote_BroadcastsToFourParticipants(t *testing.T) {
	h, hub, _ := newHandlerWithMatch([4]uint{10, 20, 30, 40})

	h.HandleAction(&ws.Client{UserID: 30}, emoteAction(t, "clap"))

	require.Equal(t, 1, hub.callCount())
	call := hub.lastCall(t)
	assert.ElementsMatch(t, []uint{10, 20, 30, 40}, call.userIDs,
		"broadcast targets the four match participants (sender included)")

	var env ws.WSMessage
	require.NoError(t, json.Unmarshal(call.msg, &env))
	assert.Equal(t, ws.SystemEmote, env.Type)

	var payload ws.EmotePayload
	require.NoError(t, json.Unmarshal(env.Payload, &payload))
	assert.Equal(t, 2, payload.PlayerSeat, "user 30 is at seat 2")
	assert.Equal(t, ws.EmoteClap, payload.Emote)
}

func TestHandler_RateLimit_DropsSecondEmoteWithinWindow(t *testing.T) {
	h, hub, _ := newHandlerWithMatch([4]uint{10, 20, 30, 40})

	h.HandleAction(&ws.Client{UserID: 10}, emoteAction(t, "thumbs_up"))
	h.HandleAction(&ws.Client{UserID: 10}, emoteAction(t, "laugh"))

	assert.Equal(t, 1, hub.callCount(),
		"second emote within 3 s window dropped — only the first broadcasts")
}

func TestHandler_RateLimit_AllowsAfterWindow(t *testing.T) {
	h, hub, _ := newHandlerWithMatch([4]uint{10, 20, 30, 40})

	h.HandleAction(&ws.Client{UserID: 10}, emoteAction(t, "thumbs_up"))
	require.Equal(t, 1, hub.callCount())

	h.mu.Lock()
	h.lastEmoteAt[10] = time.Now().Add(-3500 * time.Millisecond)
	h.mu.Unlock()

	h.HandleAction(&ws.Client{UserID: 10}, emoteAction(t, "laugh"))
	assert.Equal(t, 2, hub.callCount(),
		"second emote after rate-limit window broadcasts")
}

func TestHandler_RateLimit_PerUserNotGlobal(t *testing.T) {
	h, hub, _ := newHandlerWithMatch([4]uint{10, 20, 30, 40})

	h.HandleAction(&ws.Client{UserID: 10}, emoteAction(t, "thumbs_up"))
	h.HandleAction(&ws.Client{UserID: 20}, emoteAction(t, "laugh"))

	assert.Equal(t, 2, hub.callCount(),
		"per-user rate limit allows simultaneous emotes from different participants")
}

func TestHandler_PayloadSeatIndexMatchesSender(t *testing.T) {
	h, hub, _ := newHandlerWithMatch([4]uint{10, 20, 30, 40})

	cases := []struct {
		userID       uint
		expectedSeat int
	}{
		{10, 0},
		{20, 1},
		{30, 2},
		{40, 3},
	}

	for _, tc := range cases {
		h.mu.Lock()
		delete(h.lastEmoteAt, tc.userID)
		h.mu.Unlock()

		h.HandleAction(&ws.Client{UserID: tc.userID}, emoteAction(t, "heart"))

		call := hub.lastCall(t)
		var env ws.WSMessage
		require.NoError(t, json.Unmarshal(call.msg, &env))
		var payload ws.EmotePayload
		require.NoError(t, json.Unmarshal(env.Payload, &payload))
		assert.Equal(t, tc.expectedSeat, payload.PlayerSeat,
			"user %d expected at seat %d", tc.userID, tc.expectedSeat)
		assert.Equal(t, ws.EmoteHeart, payload.Emote)
	}
}

func TestHandler_RemoveUser_ClearsRateLimit(t *testing.T) {
	participants := [4]uint{1, 2, 3, 4}
	h, hub, _ := newHandlerWithMatch(participants)

	userID := uint(1)
	client := &ws.Client{UserID: userID}

	h.HandleAction(client, emoteAction(t, "heart"))
	assert.Equal(t, 1, hub.callCount(), "first emote should broadcast")

	h.HandleAction(client, emoteAction(t, "heart"))
	assert.Equal(t, 1, hub.callCount(), "second immediate emote should be rate-limited")

	h.RemoveUser(userID)

	h.HandleAction(client, emoteAction(t, "heart"))
	assert.Equal(t, 2, hub.callCount(), "emote after RemoveUser should broadcast (rate limit cleared)")
}
