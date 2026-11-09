package match_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/ws"
)

const summaryLine = "bot: search summary"

func endMatchForSummary(t *testing.T, mgr *match.Manager, roomID uint) {
	t.Helper()
	finalState := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, finalState)
	winner := game.TeamA
	finalState.WinnerTeam = &winner
	finalState.Phase = game.PhaseMatchEnd
	finalState.TeamScores = [2]int{1010, 700}
	mgr.HandleMatchEndForTest(roomID, finalState, nil, ws.MatchEndPayload{WinnerTeam: game.TeamA, TeamAFinalScore: 1010, TeamBFinalScore: 700})
}

func TestBotSearchSummary_EmittedOnceAtMatchEnd(t *testing.T) {
	logs := captureLogs(t)
	mgr := match.NewManager(&hubSpy{}, newMockMatchRepo())
	mgr.SetDealGraceForTest(false)
	const roomID = uint(310)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", mixedPlayers(1, 3), "relaxed", 0, 10, 120, 0, true, false, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })
	mgr.CancelBotTimersForTest(roomID)

	mgr.RecordBotSearchForTest(roomID, 1, true)
	mgr.RecordBotSearchForTest(roomID, 2, false)
	mgr.RecordBotSearchForTest(roomID, 5, true)
	endMatchForSummary(t, mgr, roomID)

	lines := logs.matched(roomID, summaryLine)
	require.Len(t, lines, 1)
	assert.Contains(t, lines[0], "level=INFO")
	assert.Contains(t, lines[0], "matchID=1 searches=3 missed=2 missedOpening=1")
}

func TestBotSearchSummary_EmittedWhenAHumanAbandonsABotTable(t *testing.T) {
	logs := captureLogs(t)
	mgr := match.NewManager(&hubSpy{}, newMockMatchRepo())
	mgr.SetDealGraceForTest(false)
	const roomID = uint(311)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", mixedPlayers(1, 3), "relaxed", 0, 10, 120, 0, true, false, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })
	mgr.CancelBotTimersForTest(roomID)

	mgr.RecordBotSearchForTest(roomID, 2, true)
	mgr.AbandonSeatForTest(roomID, 0)

	lines := logs.matched(roomID, summaryLine)
	require.Len(t, lines, 1)
	assert.Contains(t, lines[0], "matchID=1 searches=1 missed=1 missedOpening=1")
}

func TestBotSearchSummary_SilentForAHumanOnlyTable(t *testing.T) {
	logs := captureLogs(t)
	mgr := match.NewManager(&hubSpy{}, newMockMatchRepo())
	const roomID = uint(312)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	endMatchForSummary(t, mgr, roomID)

	assert.Empty(t, logs.matched(roomID, summaryLine))
}
