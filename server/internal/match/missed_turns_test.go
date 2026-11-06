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

type missedTurnsTable struct {
	mgr    *match.Manager
	hub    *hubSpy
	repo   *mockMatchRepo
	honor  *stubHonorRecorder
	xp     *stubXPAwarder
	roomID uint
}

func startMissedTurnsTable(t *testing.T, roomID uint, players [4]match.PlayerSeatInfo) *missedTurnsTable {
	t.Helper()
	tbl := &missedTurnsTable{
		hub:    &hubSpy{},
		repo:   newMockMatchRepo(),
		honor:  &stubHonorRecorder{},
		xp:     &stubXPAwarder{},
		roomID: roomID,
	}
	tbl.mgr = match.NewManager(tbl.hub, tbl.repo)
	tbl.mgr.SetDealGraceForTest(false)
	tbl.mgr.SetBotDelayForTest(time.Hour, time.Hour)
	tbl.mgr.SetHonorRecorder(tbl.honor)
	tbl.mgr.SetXPAwarder(tbl.xp)
	require.NoError(t, tbl.mgr.StartMatch(roomID, "bitola", "1001", players, "per-move", 60, 10, 120, 0, true, false, true, false))
	t.Cleanup(func() { tbl.mgr.RemoveSession(roomID) })
	return tbl
}

func (tbl *missedTurnsTable) activeSeat(t *testing.T) int {
	t.Helper()
	gs := tbl.mgr.GetStateSnapshot(tbl.roomID)
	require.NotNil(t, gs, "the match ended early")
	require.Equal(t, game.PhaseBidding, gs.Phase)
	return gs.ActivePlayerSeat
}

func (tbl *missedTurnsTable) othersPassUntil(t *testing.T, seat int) {
	t.Helper()
	for active := tbl.activeSeat(t); active != seat; active = tbl.activeSeat(t) {
		require.NoError(t, tbl.mgr.ApplyActionForTest(tbl.roomID, game.Action{Type: game.ActionPassTrump, PlayerSeat: active}))
	}
}

func (tbl *missedTurnsTable) missTurnOf(t *testing.T, seat int) {
	t.Helper()
	tbl.othersPassUntil(t, seat)
	tbl.mgr.ExpireTurnForTest(tbl.roomID)
}

func (tbl *missedTurnsTable) abandoned(t *testing.T) *ws.MatchAbandonedPayload {
	t.Helper()
	for _, call := range tbl.hub.snapshot() {
		var env struct {
			Type    string                   `json:"type"`
			Payload ws.MatchAbandonedPayload `json:"payload"`
		}
		if json.Unmarshal(call.msg, &env) == nil && env.Type == ws.EventMatchAbandoned {
			return &env.Payload
		}
	}
	return nil
}

func (tbl *missedTurnsTable) warnings() []hubCall {
	var out []hubCall
	for _, call := range tbl.hub.snapshot() {
		if containsType(call.msg, ws.EventMissedTurnWarning) {
			out = append(out, call)
		}
	}
	return out
}

func TestMissedTurns_TheSecondInARowWarnsOnlyThatPlayer(t *testing.T) {
	tbl := startMissedTurnsTable(t, 1206, defaultPlayers())
	seat := tbl.activeSeat(t)

	tbl.missTurnOf(t, seat)
	require.Empty(t, tbl.warnings(), "one missed turn gets no warning")
	tbl.missTurnOf(t, seat)

	warnings := tbl.warnings()
	require.Len(t, warnings, 1)
	assert.Equal(t, []uint{defaultPlayers()[seat].UserID}, warnings[0].userIDs, "only the player who missed is warned")
	var env struct {
		Payload ws.MissedTurnWarningPayload `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(warnings[0].msg, &env))
	assert.Equal(t, 2, env.Payload.MissedTurns)
	assert.NotNil(t, tbl.mgr.GetStateSnapshot(tbl.roomID), "a warning is not a forfeit")
}

func TestMissedTurns_TheThirdInARowForfeitsLikeADisconnect(t *testing.T) {
	tbl := startMissedTurnsTable(t, 1201, defaultPlayers())
	seat := tbl.activeSeat(t)

	tbl.missTurnOf(t, seat)
	tbl.missTurnOf(t, seat)
	require.Nil(t, tbl.abandoned(t), "two missed turns are not yet a forfeit")
	tbl.missTurnOf(t, seat)

	payload := tbl.abandoned(t)
	require.NotNil(t, payload, "the third missed turn ends the match")
	assert.Equal(t, seat, payload.AbandonedByPlayer)
	assert.Equal(t, ws.AbandonReasonMissedTurns, payload.Reason)
	assert.Nil(t, tbl.mgr.GetStateSnapshot(tbl.roomID), "the session is closed")

	forfeiter := defaultPlayers()[seat].UserID
	require.Len(t, tbl.repo.matches, 1)
	record := tbl.repo.matches[0]
	assert.Equal(t, "abandoned", record.Status)
	require.NotNil(t, record.AbandonedBy)
	assert.Equal(t, forfeiter, *record.AbandonedBy)
	assert.Equal(t, 1-game.TeamForSeat(seat), record.WinnerTeam)

	_, honor := tbl.honor.snapshotCalls()
	assert.Equal(t, match.HonorEvent{Abandoned: true}, honor[forfeiter], "the forfeiter is charged with an abandonment")
	assert.Zero(t, tbl.xp.lastAwards[forfeiter], "the forfeiting team earns no XP")
}

func TestMissedTurns_AMoveOfYourOwnStartsTheCountAgain(t *testing.T) {
	tbl := startMissedTurnsTable(t, 1202, defaultPlayers())
	seat := tbl.activeSeat(t)

	tbl.missTurnOf(t, seat)
	tbl.missTurnOf(t, seat)
	tbl.othersPassUntil(t, seat)
	require.NoError(t, tbl.mgr.ApplyActionForTest(tbl.roomID, game.Action{Type: game.ActionPassTrump, PlayerSeat: seat}))
	tbl.missTurnOf(t, seat)
	tbl.missTurnOf(t, seat)

	assert.Len(t, tbl.warnings(), 2, "each fresh run of two is warned again")
	assert.Nil(t, tbl.abandoned(t), "four missed turns, but never three in a row")
	assert.NotNil(t, tbl.mgr.GetStateSnapshot(tbl.roomID))
}

func TestMissedTurns_OtherPlayersMissesDoNotBreakTheRun(t *testing.T) {
	tbl := startMissedTurnsTable(t, 1203, defaultPlayers())
	seat := tbl.activeSeat(t)
	next := tbl.mgr.GetStateSnapshot(tbl.roomID).Rules.NextSeat(seat)

	tbl.missTurnOf(t, seat)
	tbl.missTurnOf(t, next)
	tbl.missTurnOf(t, seat)
	tbl.missTurnOf(t, next)
	tbl.missTurnOf(t, seat)

	payload := tbl.abandoned(t)
	require.NotNil(t, payload)
	assert.Equal(t, seat, payload.AbandonedByPlayer)
}

func TestMissedTurns_ABotNeverForfeits(t *testing.T) {
	players := defaultPlayers()
	tbl := startMissedTurnsTable(t, 1204, players)
	botSeat := tbl.activeSeat(t)
	players[botSeat] = match.PlayerSeatInfo{Seat: botSeat, IsBot: true}
	tbl.mgr.RemoveSession(tbl.roomID)
	tbl = startMissedTurnsTable(t, 1204, players)

	for range 4 {
		tbl.missTurnOf(t, botSeat)
	}

	assert.Empty(t, tbl.warnings())
	assert.Nil(t, tbl.abandoned(t))
	assert.NotNil(t, tbl.mgr.GetStateSnapshot(tbl.roomID))
}

func TestMissedTurns_AReconnectTimeoutStillReportsADisconnect(t *testing.T) {
	tbl := startMissedTurnsTable(t, 1205, defaultPlayers())

	tbl.mgr.AbandonSeatForTest(tbl.roomID, 2)

	payload := tbl.abandoned(t)
	require.NotNil(t, payload)
	assert.Equal(t, 2, payload.AbandonedByPlayer)
	assert.Equal(t, ws.AbandonReasonDisconnect, payload.Reason)
}
