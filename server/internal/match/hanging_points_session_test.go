package match_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/ws"
)

func TestSyncStateOnConnect_CarriesHangingPoints(t *testing.T) {
	hub := &hubSpy{}
	mgr := match.NewManager(hub, newMockMatchRepo())
	const roomID = uint(100)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true, false, 3))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	gs := testfixtures.WithHangingPoints(testfixtures.NewGameMidPlay(3), 81, game.TeamB)
	gs.RoomID = roomID
	mgr.SetGameStateForTest(roomID, gs)

	mgr.SyncStateOnConnect(30)

	frames := lastMatchStateFrames(t, hub.snapshot())
	require.Contains(t, frames, uint(30))
	raw := decodeStateFrame(t, frames[30])
	assert.Equal(t, map[string]any{"amount": float64(81), "team": float64(game.TeamB)}, raw["hangingPoints"],
		"a reconnecting seat sees the points still hanging")
}

func TestHandScored_CarriesTheTieAndThePayout(t *testing.T) {
	hub := &hubSpy{}
	mgr := match.NewManager(hub, newMockMatchRepo())
	const roomID = uint(101)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true, false, 3))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	gs := testfixtures.WithHangingPoints(testfixtures.NewGameLastTrick(), 91, game.TeamA)
	gs.RoomID = roomID
	gs.HandPoints = [2]int{81, 50}
	for seat := range gs.Players {
		gs.Players[seat].UserID = defaultPlayers()[seat].UserID
	}
	mgr.SetGameStateForTest(roomID, gs)

	for seat := 0; seat < 4; seat++ {
		require.NoError(t, mgr.ApplyActionForTest(roomID, game.Action{
			Type: game.ActionPlayCard, PlayerSeat: seat, Card: &gs.Players[seat].Hand[0],
		}))
	}

	var payload ws.HandScoredPayload
	found := false
	for _, call := range hub.snapshot() {
		var msg ws.WSMessage
		if call.msg == nil || json.Unmarshal(call.msg, &msg) != nil || msg.Type != ws.EventHandScored {
			continue
		}
		require.NoError(t, json.Unmarshal(msg.Payload, &payload))
		found = true
	}
	require.True(t, found, "the last trick emits event:hand_scored")
	assert.Equal(t, &ws.HangingPointsPayload{Amount: 81, Team: game.TeamB}, payload.HangingCreated,
		"team B called and tied, so its 81 hangs")
	assert.Equal(t, &ws.HangingPointsPayload{Amount: 91, Team: game.TeamA}, payload.HangingPaid,
		"the earlier 91 goes to the caller's opponents")
	assert.False(t, payload.FailedContract, "a Bitola tie is not a fall")
	assert.Equal(t, 172, payload.TeamAHandTotal, "team A banks its 81 and the 91 paid")
	assert.Equal(t, 0, payload.TeamBHandTotal)
	assert.Equal(t, 172, payload.TeamAMatchScore)
}

func TestBufferHandResultIfScored_HangingPoints(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	mgr := match.NewManager(hub, newMockMatchRepo())
	require.NoError(t, mgr.StartMatch(904, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true, false, 3))

	tied := bufferTestResult(3, 81, 81)
	tied.HangingCreated = &game.HangingPoints{Amount: 81, Team: game.TeamB}
	tied.HangingPaid = &game.HangingPoints{Amount: 91, Team: game.TeamA}
	mgr.BufferHandResultIfScored(904, &game.GameState{HandNumber: 3}, &game.GameState{HandNumber: 4, LastHandResult: tied})
	mgr.BufferHandResultIfScored(904, &game.GameState{HandNumber: 4}, &game.GameState{HandNumber: 5, LastHandResult: bufferTestResult(4, 100, 62)})

	hands := mgr.HandResults(904)
	require.Len(t, hands, 2)
	assert.Equal(t, intPtr(81), hands[0].HangingCreatedAmount)
	assert.Equal(t, intPtr(game.TeamB), hands[0].HangingCreatedTeam)
	assert.Equal(t, intPtr(91), hands[0].HangingPaidAmount)
	assert.Equal(t, intPtr(game.TeamA), hands[0].HangingPaidTeam)
	assert.Nil(t, hands[1].HangingCreatedAmount, "a hand without a tie stores NULL")
	assert.Nil(t, hands[1].HangingCreatedTeam)
	assert.Nil(t, hands[1].HangingPaidAmount)
	assert.Nil(t, hands[1].HangingPaidTeam)
}

func TestBuildBotView_CarriesTheTieRuleAndHangingPoints(t *testing.T) {
	mgr := match.NewManager(&hubSpy{}, newMockMatchRepo())
	const roomID = uint(102)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true, false, 3))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	gs := testfixtures.WithHangingPoints(testfixtures.NewGameMidPlay(3), 81, game.TeamA)
	gs.RoomID = roomID
	mgr.SetGameStateForTest(roomID, gs)

	view := mgr.BuildBotViewForTest(roomID, 1)

	assert.Equal(t, game.TieRuleHangingPoints, view.TieRule)
	assert.Equal(t, &game.HangingPoints{Amount: 81, Team: game.TeamA}, view.HangingPoints)
	snapshot := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, snapshot)
	assert.NotSame(t, snapshot.HangingPoints, view.HangingPoints, "the bot never shares the live entry")
}

func TestAbandonment_DiscardsHangingPoints(t *testing.T) {
	hub := &hubSpy{}
	mgr := match.NewManager(hub, newMockMatchRepo())
	const roomID = uint(103)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true, false, 3))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	gs := testfixtures.WithHangingPoints(testfixtures.NewGameMidPlay(3), 81, game.TeamA)
	gs.RoomID = roomID
	mgr.SetGameStateForTest(roomID, gs)

	mgr.AbandonSeatForTest(roomID, 1)

	frames := lastMatchStateFrames(t, hub.snapshot())
	require.Contains(t, frames, uint(10))
	raw := decodeStateFrame(t, frames[10])
	assert.Equal(t, string(game.PhaseMatchEnd), raw["phase"])
	assert.Contains(t, raw, "hangingPoints")
	assert.Nil(t, raw["hangingPoints"], "an abandoned match discards the points still hanging")
}

func intPtr(v int) *int { return &v }
