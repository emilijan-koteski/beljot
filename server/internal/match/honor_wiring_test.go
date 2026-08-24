package match_test

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/user"
	"github.com/emilijan/beljot/server/internal/ws"
)

// stubHonorRecorder records the events ApplyHonorEvents received and derives its
// snapshots from the REAL honor math, so the emitted event reflects production
// scores and tiers rather than canned values. Satisfies match.HonorRecorder.
type stubHonorRecorder struct {
	mu         sync.Mutex
	applyCalls int
	lastEvents map[uint]match.HonorEvent
	lastNow    time.Time
	err        error
	// priorCompleted seeds each user's pre-existing raw completed total so a
	// test can push a player over the New Player floor.
	priorCompleted map[uint]int64
}

func (s *stubHonorRecorder) ApplyHonorEvents(events map[uint]match.HonorEvent, now time.Time) (map[uint]match.HonorSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.applyCalls++
	s.lastEvents = events
	s.lastNow = now
	if s.err != nil {
		return nil, s.err
	}

	out := make(map[uint]match.HonorSnapshot, len(events))
	for id, ev := range events {
		completed := s.priorCompleted[id]
		var abandoned int64
		if ev.Abandoned {
			abandoned = 1
		} else {
			completed++
		}
		score := user.HonorScore(float64(completed), float64(abandoned), nil, now)
		out[id] = match.HonorSnapshot{
			Score:          score,
			Tier:           user.HonorTier(score),
			CompletedTotal: completed,
			AbandonedTotal: abandoned,
			IsNewPlayer:    user.IsNewPlayer(completed, abandoned),
		}
	}
	return out, nil
}

func (s *stubHonorRecorder) snapshotCalls() (int, map[uint]match.HonorEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.applyCalls, s.lastEvents
}

// lastIndexOfType is the mirror of firstIndexOfType (xp_wiring_test.go).
// Ordering assertions need both ends: "every xp_awarded precedes every
// honor_updated" is LAST(xp) < FIRST(honor), and "every honor_updated precedes
// match_state" is LAST(honor) < match_state.
func lastIndexOfType(calls []hubCall, eventType string) int {
	for i := len(calls) - 1; i >= 0; i-- {
		if containsType(calls[i].msg, eventType) {
			return i
		}
	}
	return -1
}

// decodeHonorUpdated extracts the typed payload from an event:honor_updated
// envelope.
func decodeHonorUpdated(t *testing.T, msg []byte) ws.HonorUpdatedPayload {
	t.Helper()
	var env struct {
		Type    string                 `json:"type"`
		Payload ws.HonorUpdatedPayload `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(msg, &env))
	return env.Payload
}

// AC3 scenario 1 — NATURAL END (win/loss). Every human seat is credited a
// completion, and the per-user event lands after xp_awarded and before the
// trailing match_state.
func TestHandleMatchEnd_RecordsHonorForEverySeat(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	recorder := &stubHonorRecorder{priorCompleted: map[uint]int64{10: 40, 20: 40, 30: 40, 40: 40}}
	mgr := match.NewManager(hub, repo)
	mgr.SetXPAwarder(&stubXPAwarder{})
	mgr.SetHonorRecorder(recorder)

	roomID := uint(300)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	finalState := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, finalState)
	winner := game.TeamA
	finalState.WinnerTeam = &winner
	finalState.Phase = game.PhaseMatchEnd
	finalState.TeamScores[game.TeamA] = 1010
	finalState.TeamScores[game.TeamB] = 700

	mgr.HandleMatchEndForTest(roomID, finalState, nil, ws.MatchEndPayload{WinnerTeam: game.TeamA})

	calls, events := recorder.snapshotCalls()
	require.Equal(t, 1, calls)
	assert.Equal(t, map[uint]match.HonorEvent{
		10: {Abandoned: false}, 20: {Abandoned: false},
		30: {Abandoned: false}, 40: {Abandoned: false},
	}, events, "the losing team completed the match too")

	hubCalls := hub.snapshot()
	matchEndIdx := firstIndexOfType(hubCalls, "event:match_end")
	require.GreaterOrEqual(t, matchEndIdx, 0)
	trailingStateIdx := indexOfTypeAfter(hubCalls, "event:match_state", matchEndIdx)
	require.GreaterOrEqual(t, trailingStateIdx, 0)

	assert.Equal(t, 4, countTypeBetween(hubCalls, "event:honor_updated", matchEndIdx, trailingStateIdx),
		"all four humans receive an honor_updated between match_end and match_state")

	for i := matchEndIdx + 1; i < trailingStateIdx; i++ {
		if containsType(hubCalls[i].msg, "event:honor_updated") {
			p := decodeHonorUpdated(t, hubCalls[i].msg)
			assert.Equal(t, int64(41), p.HonorCompletedTotal)
			assert.Equal(t, int64(0), p.HonorAbandonedTotal)
			// LITERALS, not user.HonorTier(p.HonorScore) — re-deriving the expected
			// tier from the same function the stub used to build it made this pass
			// for any implementation, including an inverted one (review pass 2).
			// 41 completed / 0 abandoned -> 100*45/46 = 97.83 -> 98 -> "exemplary".
			assert.Equal(t, 98, p.HonorScore)
			assert.Equal(t, "exemplary", p.HonorTier)
			assert.False(t, p.IsNewPlayer)
		}
	}
}

// AC5 ordering — honor_updated must land strictly AFTER xp_awarded and strictly
// BEFORE the trailing match_state, in the natural-end finalizer.
func TestHandleMatchEnd_HonorFollowsXPAndPrecedesMatchState(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	mgr := match.NewManager(hub, repo)
	mgr.SetXPAwarder(&stubXPAwarder{})
	mgr.SetHonorRecorder(&stubHonorRecorder{})

	roomID := uint(301)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	finalState := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, finalState)
	winner := game.TeamA
	finalState.WinnerTeam = &winner
	finalState.Phase = game.PhaseMatchEnd
	finalState.TeamScores[game.TeamA] = 1010
	finalState.TeamScores[game.TeamB] = 700

	mgr.HandleMatchEndForTest(roomID, finalState, nil, ws.MatchEndPayload{WinnerTeam: game.TeamA})

	calls := hub.snapshot()
	matchEndIdx := firstIndexOfType(calls, "event:match_end")
	require.GreaterOrEqual(t, matchEndIdx, 0)
	lastXPIdx := lastIndexOfType(calls, "event:xp_awarded")
	firstHonorIdx := firstIndexOfType(calls, "event:honor_updated")
	trailingStateIdx := indexOfTypeAfter(calls, "event:match_state", matchEndIdx)

	require.GreaterOrEqual(t, lastXPIdx, 0, "event:xp_awarded must fire")
	require.GreaterOrEqual(t, firstHonorIdx, 0, "event:honor_updated must fire")
	require.GreaterOrEqual(t, trailingStateIdx, 0)

	assert.Greater(t, firstHonorIdx, lastXPIdx, "honor_updated must follow every xp_awarded")
	assert.Less(t, lastIndexOfType(calls, "event:honor_updated"), trailingStateIdx,
		"every honor_updated must precede the trailing match_state")
}

// AC3 scenario 2 — SURRENDER. An accepted surrender routes through
// handleMatchEnd with the match Status still "completed", so every seat —
// including the surrendering team — is credited a completion.
func TestHandleMatchEnd_SurrenderStillCountsCompleted(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	recorder := &stubHonorRecorder{}
	mgr := match.NewManager(hub, repo)
	mgr.SetHonorRecorder(recorder)

	roomID := uint(302)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	finalState := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, finalState)
	winner := game.TeamB
	finalState.WinnerTeam = &winner
	finalState.Phase = game.PhaseMatchEnd

	surrenderedBy := uint(10)
	mgr.HandleMatchEndForTest(roomID, finalState, &surrenderedBy, ws.MatchEndPayload{WinnerTeam: game.TeamB})

	_, events := recorder.snapshotCalls()
	for _, uid := range []uint{10, 20, 30, 40} {
		assert.Equal(t, match.HonorEvent{Abandoned: false}, events[uid],
			"surrender is an agreed end, not a walk-out — seat %d keeps its completion", uid)
	}
}

// AC3 scenario 3 — ABANDONMENT. The expired seat is charged; the other three,
// INCLUDING THE ABANDONER'S TEAMMATE, are credited completions. This is the
// one place honor deliberately diverges from coins and XP, which forfeit
// team-wide.
func TestAbandonment_ChargesOnlyTheExpiredSeat(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	recorder := &stubHonorRecorder{}
	mgr := match.NewManager(hub, repo)
	mgr.SetXPAwarder(&stubXPAwarder{})
	mgr.SetHonorRecorder(recorder)

	roomID := uint(303)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120, 0, true))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	// Non-zero team scores so the non-abandoning team actually earns XP — the
	// ordering assertion below needs an xp_awarded to sit between.
	gs := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, gs)
	gs.TeamScores[game.TeamA] = 900
	gs.TeamScores[game.TeamB] = 300
	mgr.SetGameStateForTest(roomID, gs)

	// Seat 2 (userID 30, team A) abandons. Seat 0 (userID 10) is its teammate.
	mgr.AbandonSeatForTest(roomID, 2)

	calls, events := recorder.snapshotCalls()
	require.Equal(t, 1, calls)
	assert.Equal(t, map[uint]match.HonorEvent{
		10: {Abandoned: false}, // teammate stayed — NOT punished
		20: {Abandoned: false},
		30: {Abandoned: true}, // the seat whose reconnect window expired
		40: {Abandoned: false},
	}, events)

	hubCalls := hub.snapshot()
	abandonedIdx := firstIndexOfType(hubCalls, "event:match_abandoned")
	require.GreaterOrEqual(t, abandonedIdx, 0)
	trailingStateIdx := indexOfTypeAfter(hubCalls, "event:match_state", abandonedIdx)
	require.GreaterOrEqual(t, trailingStateIdx, 0)

	assert.Equal(t, 4, countTypeBetween(hubCalls, "event:honor_updated", abandonedIdx, trailingStateIdx),
		"all four humans receive an honor_updated — including the abandoner")

	// Ordering holds on this finalizer too (it broadcasts BEFORE persisting).
	lastXPIdx := lastIndexOfType(hubCalls, "event:xp_awarded")
	firstHonorIdx := firstIndexOfType(hubCalls, "event:honor_updated")
	require.GreaterOrEqual(t, lastXPIdx, 0)
	assert.Greater(t, firstHonorIdx, lastXPIdx, "honor_updated must follow every xp_awarded")
	assert.Less(t, lastIndexOfType(hubCalls, "event:honor_updated"), trailingStateIdx)

	// The abandoner's own payload carries the abandonment, and the others' do not.
	for i := abandonedIdx + 1; i < trailingStateIdx; i++ {
		if !containsType(hubCalls[i].msg, "event:honor_updated") {
			continue
		}
		p := decodeHonorUpdated(t, hubCalls[i].msg)
		if hubCalls[i].userIDs[0] == 30 {
			assert.Equal(t, int64(1), p.HonorAbandonedTotal)
			assert.Equal(t, int64(0), p.HonorCompletedTotal)
		} else {
			assert.Equal(t, int64(0), p.HonorAbandonedTotal)
			assert.Equal(t, int64(1), p.HonorCompletedTotal)
		}
	}
}

// AC3 scenario 4 — BOOT RECONCILE of a stale room. A crashed server is a server
// fault, not a player signal, so nobody's honor moves. reconcile.go has no live
// session and never touches the recorder; this test asserts that explicitly so
// a future refactor cannot quietly wire it in.
func TestReconcileStaleRooms_ChangesNobodysHonor(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	recorder := &stubHonorRecorder{}
	mgr := match.NewManager(hub, repo)
	mgr.SetHonorRecorder(recorder)

	roomRepo := newStubRoomRepo()
	roomRepo.rooms = []match.StaleRoom{{ID: 900, Variant: "bitola", MatchMode: "1001", UpdatedAt: time.Now()}}
	roomRepo.players[900] = []match.StaleRoomPlayer{
		{Seat: intp(0), UserID: 10},
		{Seat: intp(1), UserID: 20},
		{Seat: intp(2), UserID: 30},
		{Seat: intp(3), UserID: 40},
	}

	require.NoError(t, mgr.ReconcileStaleRooms(roomRepo))

	// The room WAS closed out and a match row WAS persisted...
	assert.Equal(t, "completed", roomRepo.statusUpdates[900])
	assert.True(t, repo.wasCalled(), "a boot-reconcile match row is still persisted")

	// ...but no honor was recorded and no honor event was emitted.
	calls, _ := recorder.snapshotCalls()
	assert.Zero(t, calls, "a boot reconcile is a server fault — nobody's honor moves")
	for _, c := range hub.snapshot() {
		assert.False(t, containsType(c.msg, "event:honor_updated"),
			"a boot reconcile must emit no honor_updated")
	}
}

// AC3 scenario 5 — DISCONNECT RESOLVED INSIDE THE WINDOW. No terminal state is
// reached, so there is no honor event at all; the match carries on and scores a
// completion later, at its natural end.
func TestDisconnectThenReconnect_EmitsNoHonorEvent(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	recorder := &stubHonorRecorder{}
	mgr := match.NewManager(hub, repo)
	mgr.SetHonorRecorder(recorder)

	roomID := uint(304)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120, 0, true))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	// Seat 2 drops and comes back well inside the reconnect window.
	mgr.HandleDisconnect(30)
	calls, _ := recorder.snapshotCalls()
	require.Zero(t, calls, "a disconnect alone must not touch honor")

	mgr.HandleReconnect(30)
	calls, _ = recorder.snapshotCalls()
	assert.Zero(t, calls, "a resolved disconnect must not touch honor")

	for _, c := range hub.snapshot() {
		assert.False(t, containsType(c.msg, "event:honor_updated"),
			"no honor_updated before a terminal state")
	}

	// The match then reaches its natural end and everyone scores completed.
	finalState := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, finalState)
	winner := game.TeamA
	finalState.WinnerTeam = &winner
	finalState.Phase = game.PhaseMatchEnd
	mgr.HandleMatchEndForTest(roomID, finalState, nil, ws.MatchEndPayload{WinnerTeam: game.TeamA})

	calls, events := recorder.snapshotCalls()
	require.Equal(t, 1, calls)
	assert.Equal(t, match.HonorEvent{Abandoned: false}, events[30],
		"the reconnected player finished the match and is credited for it")
}

// Nil-tolerance: with no HonorRecorder wired, nothing is emitted (mirrors
// walletSettler and xpAwarder).
func TestHandleMatchEnd_NoRecorderNoHonor(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	mgr := match.NewManager(hub, repo)
	// No SetHonorRecorder call.

	roomID := uint(305)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	finalState := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, finalState)
	winner := game.TeamA
	finalState.WinnerTeam = &winner
	finalState.Phase = game.PhaseMatchEnd

	mgr.HandleMatchEndForTest(roomID, finalState, nil, ws.MatchEndPayload{WinnerTeam: game.TeamA})

	for _, c := range hub.snapshot() {
		assert.False(t, containsType(c.msg, "event:honor_updated"), "no honor_updated without a recorder")
	}
}

// Best-effort degradation: a recorder failure logs and skips the honor events
// but must NEVER block match_end or the trailing match_state.
func TestHandleMatchEnd_HonorFailureDoesNotBlockBroadcasts(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	mgr := match.NewManager(hub, repo)
	mgr.SetHonorRecorder(&stubHonorRecorder{err: errors.New("db down")})

	roomID := uint(306)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	finalState := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, finalState)
	winner := game.TeamA
	finalState.WinnerTeam = &winner
	finalState.Phase = game.PhaseMatchEnd

	mgr.HandleMatchEndForTest(roomID, finalState, nil, ws.MatchEndPayload{WinnerTeam: game.TeamA})

	calls := hub.snapshot()
	matchEndIdx := firstIndexOfType(calls, "event:match_end")
	require.GreaterOrEqual(t, matchEndIdx, 0, "match_end must fire even when honor fails")
	require.GreaterOrEqual(t, indexOfTypeAfter(calls, "event:match_state", matchEndIdx), 0,
		"the trailing match_state must fire even when honor fails")
	for _, c := range calls {
		assert.False(t, containsType(c.msg, "event:honor_updated"), "a failed write emits no event")
	}
}

// A bot seat is never an honor subject and never receives the event.
func TestHandleMatchEnd_BotSeatAccruesNoHonor(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	recorder := &stubHonorRecorder{}
	mgr := match.NewManager(hub, repo)
	mgr.SetHonorRecorder(recorder)
	mgr.SetBotDelayForTest(time.Hour, time.Hour)

	roomID := uint(307)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", mixedPlayers(3), "relaxed", 0, 10, 120, 0, true))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	finalState := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, finalState)
	winner := game.TeamA
	finalState.WinnerTeam = &winner
	finalState.Phase = game.PhaseMatchEnd

	mgr.HandleMatchEndForTest(roomID, finalState, nil, ws.MatchEndPayload{WinnerTeam: game.TeamA})

	_, events := recorder.snapshotCalls()
	assert.Equal(t, map[uint]match.HonorEvent{
		10: {Abandoned: false}, 20: {Abandoned: false}, 30: {Abandoned: false},
	}, events)
	_, hasBot := events[0]
	assert.False(t, hasBot, "bot seat (userID 0) must never appear in the honor events")

	honorCount := 0
	for _, c := range hub.snapshot() {
		if containsType(c.msg, "event:honor_updated") {
			honorCount++
		}
	}
	assert.Equal(t, 3, honorCount, "only the three humans receive honor_updated")
}

// A New Player still gets the real score and tier on the wire — suppression is
// presentation-only, because Story 9.8's join gate reads these fields.
func TestHandleMatchEnd_NewPlayerStillCarriesScoreAndTier(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	mgr := match.NewManager(hub, repo)
	mgr.SetHonorRecorder(&stubHonorRecorder{}) // no prior completions → New Player

	roomID := uint(308)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	finalState := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, finalState)
	winner := game.TeamA
	finalState.WinnerTeam = &winner
	finalState.Phase = game.PhaseMatchEnd

	mgr.HandleMatchEndForTest(roomID, finalState, nil, ws.MatchEndPayload{WinnerTeam: game.TeamA})

	seen := 0
	for _, c := range hub.snapshot() {
		if !containsType(c.msg, "event:honor_updated") {
			continue
		}
		seen++
		p := decodeHonorUpdated(t, c.msg)
		assert.True(t, p.IsNewPlayer, "one completed match is under the floor of 5")
		assert.Positive(t, p.HonorScore, "the real score is still sent — 9.8's gate needs it")
		assert.NotEmpty(t, p.HonorTier, "the real tier is still sent")
	}
	assert.Equal(t, 4, seen)
}
