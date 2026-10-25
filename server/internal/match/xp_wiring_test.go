package match_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/user"
	"github.com/emilijan/beljot/server/internal/ws"
)

// stubXPAwarder records the awards it receives and returns canned new totals,
// using the real level curve for NewLevel and LeveledUp.
type stubXPAwarder struct {
	applyCalls int
	lastAwards map[uint]int
	newTotals  map[uint]int
	totalXP    map[uint]int
}

func (s *stubXPAwarder) ApplyXPAwards(awards map[uint]int) (map[uint]int, error) {
	s.applyCalls++
	s.lastAwards = awards
	out := make(map[uint]int, len(awards))
	for id, delta := range awards {
		if v, ok := s.newTotals[id]; ok {
			out[id] = v
		} else {
			out[id] = delta
		}
	}
	return out, nil
}

func (s *stubXPAwarder) LevelForXP(totalXP int) int { return user.LevelForXP(totalXP) }

func (s *stubXPAwarder) LevelsForUsers(ids []uint) (map[uint]int, error) {
	out := make(map[uint]int, len(ids))
	for _, id := range ids {
		out[id] = user.LevelForXP(s.totalXP[id])
	}
	return out, nil
}

// decodeXPAwarded extracts the typed payload from an event:xp_awarded envelope.
func decodeXPAwarded(t *testing.T, msg []byte) ws.XPAwardedPayload {
	t.Helper()
	var env struct {
		Type    string              `json:"type"`
		Payload ws.XPAwardedPayload `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(msg, &env))
	return env.Payload
}

// countTypeBetween counts messages of eventType in calls[(after, before)].
func countTypeBetween(calls []hubCall, eventType string, after, before int) int {
	n := 0
	for i := after + 1; i < before; i++ {
		if containsType(calls[i].msg, eventType) {
			n++
		}
	}
	return n
}

func firstIndexOfType(calls []hubCall, eventType string) int {
	for i, c := range calls {
		if containsType(c.msg, eventType) {
			return i
		}
	}
	return -1
}

// A free match still awards XP to both teams, with one event:xp_awarded per human
// ordered before the trailing event:match_state.
func TestHandleMatchEnd_AwardsXP(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	awarder := &stubXPAwarder{newTotals: map[uint]int{10: 101, 20: 70, 30: 101, 40: 70}}
	mgr := match.NewManager(hub, repo)
	mgr.SetXPAwarder(awarder)

	roomID := uint(200)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	finalState := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, finalState)
	winner := game.TeamA
	finalState.WinnerTeam = &winner
	finalState.Phase = game.PhaseMatchEnd
	finalState.TeamScores[game.TeamA] = 1010
	finalState.TeamScores[game.TeamB] = 700

	mgr.HandleMatchEndForTest(roomID, finalState, nil, ws.MatchEndPayload{WinnerTeam: game.TeamA})

	require.Equal(t, 1, awarder.applyCalls)
	assert.Equal(t, map[uint]int{10: 101, 20: 70, 30: 101, 40: 70}, awarder.lastAwards)

	calls := hub.snapshot()
	matchEndIdx := firstIndexOfType(calls, "event:match_end")
	require.GreaterOrEqual(t, matchEndIdx, 0, "event:match_end must fire")
	trailingStateIdx := indexOfTypeAfter(calls, "event:match_state", matchEndIdx)
	require.GreaterOrEqual(t, trailingStateIdx, 0, "trailing event:match_state must follow match_end")

	assert.Equal(t, 4, countTypeBetween(calls, "event:xp_awarded", matchEndIdx, trailingStateIdx),
		"all four humans receive an xp_awarded between match_end and match_state")

	for i := matchEndIdx + 1; i < trailingStateIdx; i++ {
		if containsType(calls[i].msg, "event:xp_awarded") {
			p := decodeXPAwarded(t, calls[i].msg)
			assert.Positive(t, p.XPEarned)
			assert.Equal(t, user.LevelForXP(p.NewTotalXP), p.NewLevel)
		}
	}
}

// TestHandleMatchEnd_NoAwarderNoXP verifies that with no XPAwarder wired, no
// xp_awarded events are emitted (nil-tolerant, mirroring walletSettler).
func TestHandleMatchEnd_NoAwarderNoXP(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	mgr := match.NewManager(hub, repo)

	roomID := uint(202)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	finalState := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, finalState)
	winner := game.TeamA
	finalState.WinnerTeam = &winner
	finalState.Phase = game.PhaseMatchEnd
	finalState.TeamScores[game.TeamA] = 1010
	finalState.TeamScores[game.TeamB] = 700

	mgr.HandleMatchEndForTest(roomID, finalState, nil, ws.MatchEndPayload{WinnerTeam: game.TeamA})

	for _, c := range hub.snapshot() {
		assert.False(t, containsType(c.msg, "event:xp_awarded"), "no xp_awarded without an awarder")
	}
}

// TestHandleMatchEnd_BotSeatEarnsNoXP verifies a bot seat (UserID 0) is never in
// the awards map and never receives an xp_awarded event — only the three humans.
func TestHandleMatchEnd_BotSeatEarnsNoXP(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	awarder := &stubXPAwarder{}
	mgr := match.NewManager(hub, repo)
	mgr.SetXPAwarder(awarder)
	mgr.SetBotDelayForTest(time.Hour, time.Hour)

	roomID := uint(203)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", mixedPlayers(3), "relaxed", 0, 10, 120, 0, true, false, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	finalState := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, finalState)
	winner := game.TeamA
	finalState.WinnerTeam = &winner
	finalState.Phase = game.PhaseMatchEnd
	finalState.TeamScores[game.TeamA] = 1000
	finalState.TeamScores[game.TeamB] = 300

	mgr.HandleMatchEndForTest(roomID, finalState, nil, ws.MatchEndPayload{WinnerTeam: game.TeamA})

	require.Equal(t, 1, awarder.applyCalls)
	assert.Equal(t, map[uint]int{10: 100, 20: 30, 30: 100}, awarder.lastAwards)
	_, hasBot := awarder.lastAwards[0]
	assert.False(t, hasBot, "bot seat (userID 0) must never appear in the awards map")

	xpCount := 0
	for _, c := range hub.snapshot() {
		if containsType(c.msg, "event:xp_awarded") {
			xpCount++
		}
	}
	assert.Equal(t, 3, xpCount, "only the three humans receive xp_awarded")
}

// On abandonment the whole abandoning team forfeits XP while the other team earns,
// with each event ordered after coin_settlement and before match_state.
func TestAbandonment_AwardsXP(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	awarder := &stubXPAwarder{newTotals: map[uint]int{20: 30, 40: 30}}
	mgr := match.NewManager(hub, repo)
	mgr.SetXPAwarder(awarder)

	roomID := uint(201)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120, 0, true, false, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	gs := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, gs)
	gs.TeamScores[game.TeamA] = 900
	gs.TeamScores[game.TeamB] = 300
	mgr.SetGameStateForTest(roomID, gs)

	mgr.AbandonSeatForTest(roomID, 2)

	require.Equal(t, 1, awarder.applyCalls)
	assert.Equal(t, map[uint]int{20: 30, 40: 30}, awarder.lastAwards)
	_, aForfeited := awarder.lastAwards[10]
	assert.False(t, aForfeited, "abandoning team member 10 must forfeit (absent from awards)")
	_, aForfeited2 := awarder.lastAwards[30]
	assert.False(t, aForfeited2, "abandoning team member 30 must forfeit (absent from awards)")

	calls := hub.snapshot()
	abandonedIdx := firstIndexOfType(calls, "event:match_abandoned")
	require.GreaterOrEqual(t, abandonedIdx, 0, "event:match_abandoned must fire")
	trailingStateIdx := indexOfTypeAfter(calls, "event:match_state", abandonedIdx)
	require.GreaterOrEqual(t, trailingStateIdx, 0, "trailing event:match_state must follow match_abandoned")

	assert.Equal(t, 2, countTypeBetween(calls, "event:xp_awarded", abandonedIdx, trailingStateIdx),
		"the two non-abandoning humans receive an xp_awarded between match_abandoned and match_state")
}
