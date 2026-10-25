package match_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/ws"
)

// lastMatchStateFrames returns the per-recipient frames of the last event:match_state
// sent through the frames primitive, failing the test if none was.
func lastMatchStateFrames(t *testing.T, calls []hubCall) map[uint][]byte {
	t.Helper()
	for i := len(calls) - 1; i >= 0; i-- {
		if calls[i].frames == nil || !containsType(calls[i].msg, ws.EventMatchState) {
			continue
		}
		return calls[i].frames
	}
	t.Fatal("no event:match_state was sent through the per-recipient frames primitive")
	return nil
}

// decodeStateFrame unwraps one recipient's frame into the raw payload map.
func decodeStateFrame(t *testing.T, frame []byte) map[string]any {
	t.Helper()
	var env struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(frame, &env))
	require.Equal(t, ws.EventMatchState, env.Type)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(env.Payload, &raw))
	return raw
}

// One state event fans out as four different frames, each with only the recipient's
// own hand, real handCounts on all seats and no deck key.
func TestMatchState_EachRecipientGetsOwnProjectedFrame(t *testing.T) {
	hub := &hubSpy{}
	mgr := match.NewManager(hub, newMockMatchRepo())

	const roomID = uint(100)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	gs := testfixtures.NewGameJustDealt()
	gs.RoomID = roomID
	realHands := [4][]game.Card{}
	for i := range gs.Players {
		realHands[i] = append([]game.Card{}, gs.Players[i].Hand...)
	}
	mgr.SetGameStateForTest(roomID, gs)

	mgr.HandleAction(&ws.Client{UserID: 20}, ws.WSMessage{Type: "action:pass_trump", Payload: []byte(`{}`)})

	frames := lastMatchStateFrames(t, hub.snapshot())
	require.Len(t, frames, 4, "every human seat gets exactly one frame")

	seatForUser := map[uint]int{10: 0, 20: 1, 30: 2, 40: 3}
	for uid, frame := range frames {
		seat := seatForUser[uid]
		raw := decodeStateFrame(t, frame)

		assert.NotContains(t, raw, "deck", "user %d's frame must carry no deck key", uid)

		players, ok := raw["players"].([]any)
		require.True(t, ok)
		require.Len(t, players, 4, "the players array is never shortened or reordered")

		for i, p := range players {
			seatObj, ok := p.(map[string]any)
			require.True(t, ok)
			hand, ok := seatObj["hand"].([]any)
			require.True(t, ok, "hand must serialize as an array, never null")
			assert.Equal(t, float64(len(realHands[i])), seatObj["handCount"],
				"user %d's frame: seat %d's handCount must be the real hand length", uid, i)
			if i == seat {
				require.Len(t, hand, len(realHands[i]),
					"user %d must receive their own full hand", uid)
				for j, c := range hand {
					card, ok := c.(map[string]any)
					require.True(t, ok)
					assert.Equal(t, string(realHands[i][j].Rank), card["rank"])
					assert.Equal(t, string(realHands[i][j].Suit), card["suit"])
				}
			} else {
				assert.Empty(t, hand,
					"user %d's frame must not carry seat %d's cards", uid, i)
			}
		}
	}

	for _, a := range []uint{10, 20, 30} {
		for _, b := range []uint{20, 30, 40} {
			if a >= b {
				continue
			}
			assert.False(t, bytes.Equal(frames[a], frames[b]),
				"users %d and %d received identical state frames", a, b)
		}
	}
}

// assertProjectedForSeat asserts a decoded match_state payload is projected for seat:
// its own hand real, other hands empty, real handCounts and no deck key.
func assertProjectedForSeat(t *testing.T, raw map[string]any, seat int, realHands [4][]game.Card) {
	t.Helper()

	assert.NotContains(t, raw, "deck", "seat %d's frame must carry no deck key", seat)

	players, ok := raw["players"].([]any)
	require.True(t, ok)
	require.Len(t, players, 4, "the players array is never shortened or reordered")

	for i, p := range players {
		seatObj, ok := p.(map[string]any)
		require.True(t, ok)
		hand, ok := seatObj["hand"].([]any)
		require.True(t, ok, "hand must serialize as an array, never null")
		assert.Equal(t, float64(len(realHands[i])), seatObj["handCount"],
			"seat %d's frame: seat %d's handCount must be the real hand length", seat, i)
		if i == seat {
			require.Len(t, hand, len(realHands[i]),
				"seat %d must receive their own full hand", seat)
			for j, c := range hand {
				card, ok := c.(map[string]any)
				require.True(t, ok)
				assert.Equal(t, string(realHands[i][j].Rank), card["rank"])
				assert.Equal(t, string(realHands[i][j].Suit), card["suit"])
			}
		} else {
			assert.Empty(t, hand,
				"seat %d's frame must not carry seat %d's cards", seat, i)
		}
	}
}

// The snapshot pushed to a reconnecting user is projected for their seat, with all
// four seats holding cards so the masking is non-vacuous.
func TestSyncStateOnConnect_UnicastIsProjectedForReconnectingSeat(t *testing.T) {
	hub := &hubSpy{}
	mgr := match.NewManager(hub, newMockMatchRepo())

	const roomID = uint(100)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	gs := testfixtures.NewGameJustDealt()
	gs.RoomID = roomID
	realHands := [4][]game.Card{}
	for i := range gs.Players {
		realHands[i] = append([]game.Card{}, gs.Players[i].Hand...)
	}
	mgr.SetGameStateForTest(roomID, gs)

	before := len(hub.snapshot())
	mgr.SyncStateOnConnect(30)

	calls := hub.snapshot()
	require.Len(t, calls, before+1, "exactly one targeted send expected")
	last := calls[len(calls)-1]
	require.Equal(t, []uint{30}, last.userIDs, "the snapshot must go only to the (re)connecting user")

	raw := decodeStateFrame(t, last.msg)
	assertProjectedForSeat(t, raw, 2, realHands)
}

// After a disconnect the state batch skips the dropped user and each remaining
// recipient gets their own projection.
func TestHandleDisconnect_RemainingSeatsGetOwnProjections(t *testing.T) {
	hub := &hubSpy{}
	mgr := match.NewManager(hub, newMockMatchRepo())

	const roomID = uint(100)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	gs := testfixtures.NewGameJustDealt()
	gs.RoomID = roomID
	realHands := [4][]game.Card{}
	for i := range gs.Players {
		realHands[i] = append([]game.Card{}, gs.Players[i].Hand...)
	}
	mgr.SetGameStateForTest(roomID, gs)

	mgr.HandleDisconnect(30)

	frames := lastMatchStateFrames(t, hub.snapshot())
	require.Len(t, frames, 3, "three remaining seats, three frames")
	assert.NotContains(t, frames, uint(30), "the disconnected user must get no frame")

	seatForUser := map[uint]int{10: 0, 20: 1, 40: 3}
	for uid, frame := range frames {
		raw := decodeStateFrame(t, frame)
		assertProjectedForSeat(t, raw, seatForUser[uid], realHands)
	}
}

// Bots read the unprojected state in-process, so a bot seat never appears in a frames batch.
func TestMatchState_BotSeatsGetNoFrame(t *testing.T) {
	hub := &hubSpy{}
	mgr := match.NewManager(hub, newMockMatchRepo())

	const roomID = uint(100)
	players := defaultPlayers()
	players[2] = match.PlayerSeatInfo{UserID: 0, Username: "", Seat: 2, IsBot: true}
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", players, "relaxed", 0, 10, 120, 0, true, false, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	gs := testfixtures.NewGameJustDealt()
	gs.RoomID = roomID
	gs.Players[2].IsBot = true
	gs.Players[2].UserID = 0
	mgr.SetGameStateForTest(roomID, gs)

	mgr.HandleAction(&ws.Client{UserID: 20}, ws.WSMessage{Type: "action:pass_trump", Payload: []byte(`{}`)})

	frames := lastMatchStateFrames(t, hub.snapshot())
	require.Len(t, frames, 3, "three humans, three frames")
	assert.NotContains(t, frames, uint(0), "the bot placeholder must never receive a frame")
}
