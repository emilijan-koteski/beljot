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
	"github.com/emilijan/beljot/server/internal/season"
	"github.com/emilijan/beljot/server/internal/ws"
)

// stubSPAwarder records what ApplySeasonPoints received and derives its snapshots
// from the REAL ladder (season.TierForSP), so the emitted event reflects
// production tier arithmetic rather than canned values. Satisfies match.SPAwarder.
type stubSPAwarder struct {
	mu         sync.Mutex
	applyCalls int
	lastAwards map[uint]match.SPAward
	lastNow    time.Time
	err        error
	// priorSP seeds each user's pre-existing season total so a test can push a
	// player across a tier floor.
	priorSP map[uint]int
}

func (s *stubSPAwarder) ApplySeasonPoints(awards map[uint]match.SPAward, now time.Time) (map[uint]match.PlayerSeasonSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.applyCalls++
	s.lastAwards = awards
	s.lastNow = now
	if s.err != nil {
		return nil, s.err
	}

	out := make(map[uint]match.PlayerSeasonSnapshot, len(awards))
	for id, a := range awards {
		prior := s.priorSP[id]
		next := prior + a.SP
		out[id] = match.PlayerSeasonSnapshot{
			SeasonName: "2026 Q3",
			SP:         next,
			RankTier:   season.TierForSP(next),
			TieredUp:   season.TierForSP(next) != season.TierForSP(prior),
		}
	}
	return out, nil
}

func (s *stubSPAwarder) snapshotCalls() (int, map[uint]match.SPAward) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.applyCalls, s.lastAwards
}

func (s *stubSPAwarder) snapshotNow() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastNow
}

// assertFinalizerStamp checks the `now` the finalizer threaded down. It is the
// value that decides WHICH SEASON WINDOW the match lands in, so an unset or
// non-UTC stamp would silently file a match under the wrong quarter at a
// boundary — and a zero time.Time resolves to year 1, which the lazy resolver
// would happily create a season for.
func assertFinalizerStamp(t *testing.T, awarder *stubSPAwarder) {
	t.Helper()
	now := awarder.snapshotNow()
	require.False(t, now.IsZero(), "the finalizer must stamp `now`, not leave it zero")
	assert.Equal(t, time.UTC, now.Location(), "the stamp must be UTC")
	assert.WithinDuration(t, time.Now().UTC(), now, time.Second,
		"the stamp must be the finalizer's own clock read, not an arbitrary time")
}

// decodeSeasonPoints extracts the typed payload from an
// event:season_points_awarded envelope.
func decodeSeasonPoints(t *testing.T, msg []byte) ws.SeasonPointsAwardedPayload {
	t.Helper()
	var env struct {
		Type    string                        `json:"type"`
		Payload ws.SeasonPointsAwardedPayload `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(msg, &env))
	return env.Payload
}

// AC1 — NATURAL END. Every human seat accrues SP (losers included), each gets its
// own event, and the events land after honor_updated and before the trailing
// match_state.
func TestHandleMatchEnd_AwardsSeasonPoints(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	awarder := &stubSPAwarder{}
	mgr := match.NewManager(hub, repo)
	mgr.SetXPAwarder(&stubXPAwarder{})
	mgr.SetHonorRecorder(&stubHonorRecorder{})
	mgr.SetSPAwarder(awarder)

	roomID := uint(400)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	finalState := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, finalState)
	winner := game.TeamA
	finalState.WinnerTeam = &winner
	finalState.Phase = game.PhaseMatchEnd
	finalState.TeamScores[game.TeamA] = 1010
	finalState.TeamScores[game.TeamB] = 700

	mgr.HandleMatchEndForTest(roomID, finalState, nil, ws.MatchEndPayload{WinnerTeam: game.TeamA})

	calls, awards := awarder.snapshotCalls()
	require.Equal(t, 1, calls)
	// Winners: 50 + 100 + floor(1010/10) = 251. Losers: 50 + floor(700/10) = 120.
	assert.Equal(t, map[uint]match.SPAward{
		10: {SP: 251, Completed: true},
		20: {SP: 120, Completed: true},
		30: {SP: 251, Completed: true},
		40: {SP: 120, Completed: true},
	}, awards, "the losing team accrues SP too, and every seat completed")
	assertFinalizerStamp(t, awarder)

	hubCalls := hub.snapshot()
	matchEndIdx := firstIndexOfType(hubCalls, "event:match_end")
	require.GreaterOrEqual(t, matchEndIdx, 0)
	trailingStateIdx := indexOfTypeAfter(hubCalls, "event:match_state", matchEndIdx)
	require.GreaterOrEqual(t, trailingStateIdx, 0)

	assert.Equal(t, 4, countTypeBetween(hubCalls, "event:season_points_awarded", matchEndIdx, trailingStateIdx),
		"all four humans receive a season_points_awarded between match_end and match_state")

	for i := matchEndIdx + 1; i < trailingStateIdx; i++ {
		if !containsType(hubCalls[i].msg, "event:season_points_awarded") {
			continue
		}
		p := decodeSeasonPoints(t, hubCalls[i].msg)
		assert.Equal(t, "2026 Q3", p.SeasonName, "the machine-stable window token rides along")
		assert.Equal(t, "iron", p.RankTier, "a stable token, never a display string")
		assert.False(t, p.TieredUp)
		if hubCalls[i].userIDs[0] == 10 || hubCalls[i].userIDs[0] == 30 {
			assert.Equal(t, 251, p.SPEarned)
			assert.Equal(t, 251, p.NewSeasonSP)
		} else {
			assert.Equal(t, 120, p.SPEarned)
			assert.Equal(t, 120, p.NewSeasonSP)
		}
	}
}

// AC5 ordering — season_points_awarded lands strictly AFTER honor_updated and
// strictly BEFORE the trailing match_state, on the natural-end finalizer. Adding
// it must also leave honor's own assertion (LAST(honor) < match_state) true.
func TestHandleMatchEnd_SeasonPointsFollowHonorAndPrecedeMatchState(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	mgr := match.NewManager(hub, repo)
	mgr.SetXPAwarder(&stubXPAwarder{})
	mgr.SetHonorRecorder(&stubHonorRecorder{})
	mgr.SetSPAwarder(&stubSPAwarder{})

	roomID := uint(401)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false))
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
	lastHonorIdx := lastIndexOfType(calls, "event:honor_updated")
	firstSPIdx := firstIndexOfType(calls, "event:season_points_awarded")
	lastSPIdx := lastIndexOfType(calls, "event:season_points_awarded")
	trailingStateIdx := indexOfTypeAfter(calls, "event:match_state", matchEndIdx)

	require.GreaterOrEqual(t, lastHonorIdx, 0, "event:honor_updated must fire")
	require.GreaterOrEqual(t, firstSPIdx, 0, "event:season_points_awarded must fire")
	require.GreaterOrEqual(t, trailingStateIdx, 0)

	assert.Greater(t, firstSPIdx, lastHonorIdx, "season_points_awarded must follow every honor_updated")
	assert.Less(t, lastSPIdx, trailingStateIdx, "every season_points_awarded must precede the trailing match_state")
	// The pre-existing honor invariant still holds with SP slotted in between.
	assert.Less(t, lastHonorIdx, trailingStateIdx)
}

// AC2 — crossing a tier floor sets tieredUp on that player's event only.
func TestHandleMatchEnd_TierUpIsPerPlayer(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	// Seat 0 (user 10) sits just below the 500 Bronze floor and crosses it; the
	// others stay put.
	awarder := &stubSPAwarder{priorSP: map[uint]int{10: 400}}
	mgr := match.NewManager(hub, repo)
	mgr.SetSPAwarder(awarder)

	roomID := uint(402)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	finalState := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, finalState)
	winner := game.TeamA
	finalState.WinnerTeam = &winner
	finalState.Phase = game.PhaseMatchEnd
	finalState.TeamScores[game.TeamA] = 1010
	finalState.TeamScores[game.TeamB] = 700

	mgr.HandleMatchEndForTest(roomID, finalState, nil, ws.MatchEndPayload{WinnerTeam: game.TeamA})

	seen := 0
	for _, c := range hub.snapshot() {
		if !containsType(c.msg, "event:season_points_awarded") {
			continue
		}
		seen++
		p := decodeSeasonPoints(t, c.msg)
		if c.userIDs[0] == 10 {
			assert.True(t, p.TieredUp, "400 + 251 crosses the 500 Bronze floor")
			assert.Equal(t, 651, p.NewSeasonSP)
			assert.Equal(t, "bronze", p.RankTier)
		} else {
			assert.False(t, p.TieredUp, "seat %d did not cross a floor", c.userIDs[0])
			assert.Equal(t, "iron", p.RankTier)
		}
	}
	assert.Equal(t, 4, seen)
}

// D2 — a Capot pays every seat +50, read from the BUFFERED hand results. This
// also exercises the D4 hoist: the hand-results copy now happens before the SP
// award, so a Capot recorded during play actually reaches the formula.
func TestHandleMatchEnd_CapotPaysEverySeat(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	awarder := &stubSPAwarder{}
	mgr := match.NewManager(hub, repo)
	mgr.SetSPAwarder(awarder)

	roomID := uint(403)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	// Drive a real Capot hand into the session's buffer the way play does: an
	// old->new state pair in which the hand advanced and the new state carries the
	// scored Capot result.
	capotTeam := game.TeamA
	before := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, before)
	after := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, after)
	after.HandNumber = before.HandNumber + 1
	after.LastHandResult = &game.HandScore{
		HandNumber: 1, Capot: true, CapotTeam: &capotTeam, CapotBonus: 90,
		TeamACardPoints: 162, TeamAHandTotal: 252, ContractingTeam: game.TeamA,
	}
	mgr.BufferHandResultIfScored(roomID, before, after)
	require.Len(t, mgr.HandResults(roomID), 1, "the capot hand must be buffered")

	finalState := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, finalState)
	winner := game.TeamA
	finalState.WinnerTeam = &winner
	finalState.Phase = game.PhaseMatchEnd
	finalState.TeamScores[game.TeamA] = 1010
	finalState.TeamScores[game.TeamB] = 700

	mgr.HandleMatchEndForTest(roomID, finalState, nil, ws.MatchEndPayload{WinnerTeam: game.TeamA})

	_, awards := awarder.snapshotCalls()
	assert.Equal(t, map[uint]match.SPAward{
		10: {SP: 301, Completed: true},
		20: {SP: 170, Completed: true},
		30: {SP: 301, Completed: true},
		40: {SP: 170, Completed: true},
	}, awards, "+50 to all four seats, losers included (D2)")
}

// AC1 / D3 — an instant win on a fresh deal: TeamScores [0,0] and no hand
// results, yet the bonus still lands, because the engine records the outcome on
// the state instead of leaving the match layer to infer it.
func TestHandleMatchEnd_InstantWinPaysTheBonusAtZeroScores(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	awarder := &stubSPAwarder{}
	mgr := match.NewManager(hub, repo)
	mgr.SetSPAwarder(awarder)

	roomID := uint(404)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	finalState := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, finalState)
	winner := game.TeamA
	finalState.WinnerTeam = &winner
	finalState.Phase = game.PhaseMatchEnd
	finalState.WonByInstantWin = true
	// Left at [0,0] deliberately — floor(0/10) == 0 is a legitimate term.

	mgr.HandleMatchEndForTest(roomID, finalState, nil, ws.MatchEndPayload{WinnerTeam: game.TeamA})

	_, awards := awarder.snapshotCalls()
	assert.Equal(t, map[uint]match.SPAward{
		10: {SP: 200, Completed: true}, // 50 + 100 + 0 + 50
		20: {SP: 100, Completed: true}, // 50 +   0 + 0 + 50
		30: {SP: 200, Completed: true},
		40: {SP: 100, Completed: true},
	}, awards)
}

// D5 — ABANDONMENT. The abandoner earns 0 but still counts a games_played; the
// abandoner's TEAMMATE is present and STILL EARNS. This is the divergence from
// coins and XP, which forfeit the whole abandoning team.
func TestAbandonment_ForfeitsSeasonPointsPerSeat(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	awarder := &stubSPAwarder{}
	mgr := match.NewManager(hub, repo)
	mgr.SetXPAwarder(&stubXPAwarder{})
	mgr.SetHonorRecorder(&stubHonorRecorder{})
	mgr.SetSPAwarder(awarder)

	roomID := uint(405)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	gs := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, gs)
	gs.TeamScores[game.TeamA] = 900
	gs.TeamScores[game.TeamB] = 300
	mgr.SetGameStateForTest(roomID, gs)

	// Seat 2 (userID 30, team A) abandons. Seat 0 (userID 10) is its teammate.
	mgr.AbandonSeatForTest(roomID, 2)

	calls, awards := awarder.snapshotCalls()
	require.Equal(t, 1, calls)
	assert.Equal(t, map[uint]match.SPAward{
		// The teammate stayed: team A lost, so 50 + floor(900/10) = 140.
		10: {SP: 140, Completed: true},
		// The winning (non-abandoning) team: 50 + 100 + floor(300/10) = 180.
		20: {SP: 180, Completed: true},
		// The abandoner: zero SP, but Completed false rather than absent from the
		// map — the seat still counts a games_played (D10).
		30: {SP: 0, Completed: false},
		40: {SP: 180, Completed: true},
	}, awards)
	assertFinalizerStamp(t, awarder)

	hubCalls := hub.snapshot()
	abandonedIdx := firstIndexOfType(hubCalls, "event:match_abandoned")
	require.GreaterOrEqual(t, abandonedIdx, 0)
	trailingStateIdx := indexOfTypeAfter(hubCalls, "event:match_state", abandonedIdx)
	require.GreaterOrEqual(t, trailingStateIdx, 0)

	assert.Equal(t, 4, countTypeBetween(hubCalls, "event:season_points_awarded", abandonedIdx, trailingStateIdx),
		"all four humans receive a season_points_awarded — including the abandoner")

	// The ordering slot holds on this finalizer too (it broadcasts BEFORE it
	// persists).
	lastHonorIdx := lastIndexOfType(hubCalls, "event:honor_updated")
	require.GreaterOrEqual(t, lastHonorIdx, 0)
	assert.Greater(t, firstIndexOfType(hubCalls, "event:season_points_awarded"), lastHonorIdx)
	assert.Less(t, lastIndexOfType(hubCalls, "event:season_points_awarded"), trailingStateIdx)

	// The abandoner's own payload carries the zero, and a zero is a REAL value.
	for i := abandonedIdx + 1; i < trailingStateIdx; i++ {
		if !containsType(hubCalls[i].msg, "event:season_points_awarded") {
			continue
		}
		p := decodeSeasonPoints(t, hubCalls[i].msg)
		if hubCalls[i].userIDs[0] == 30 {
			assert.Zero(t, p.SPEarned)
			assert.Zero(t, p.NewSeasonSP)
			assert.False(t, p.TieredUp)
			assert.Equal(t, "iron", p.RankTier, "the tier is still populated for a zero award")
		} else {
			assert.Positive(t, p.SPEarned)
		}
	}
}

// D2 on the ABANDONMENT finalizer. The Capot bonus is derived independently on
// each of the two finalizers — handleMatchEnd reads the hoisted handsCopy,
// handleSeatReconnectTimeout derives `spectacularMatch` under the session lock —
// so covering it only on the natural end left the abandonment path's argument
// free to be a hardcoded false without any test noticing.
//
// A Capot scored before the walk-out still pays the present seats +50; the
// absent seat still earns nothing, because presence gates before the bonus does.
func TestAbandonment_CapotPaysThePresentSeats(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	awarder := &stubSPAwarder{}
	mgr := match.NewManager(hub, repo)
	mgr.SetSPAwarder(awarder)

	roomID := uint(411)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	// Same recipe as TestHandleMatchEnd_CapotPaysEverySeat: a real scored Capot
	// hand buffered the way play buffers one.
	capotTeam := game.TeamA
	before := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, before)
	after := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, after)
	after.HandNumber = before.HandNumber + 1
	after.LastHandResult = &game.HandScore{
		HandNumber: 1, Capot: true, CapotTeam: &capotTeam, CapotBonus: 90,
		TeamACardPoints: 162, TeamAHandTotal: 252, ContractingTeam: game.TeamA,
	}
	mgr.BufferHandResultIfScored(roomID, before, after)
	require.Len(t, mgr.HandResults(roomID), 1, "the capot hand must be buffered before the abandonment")

	gs := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, gs)
	gs.TeamScores[game.TeamA] = 900
	gs.TeamScores[game.TeamB] = 300
	mgr.SetGameStateForTest(roomID, gs)

	// Seat 2 (userID 30, team A) abandons; seat 0 (userID 10) is its teammate.
	mgr.AbandonSeatForTest(roomID, 2)

	_, awards := awarder.snapshotCalls()
	assert.Equal(t, map[uint]match.SPAward{
		// Teammate, present, team A lost: 50 + floor(900/10) + 50 = 190.
		10: {SP: 190, Completed: true},
		// Winning non-abandoning team: 50 + 100 + floor(300/10) + 50 = 230.
		20: {SP: 230, Completed: true},
		// The abandoner earns nothing — presence gates before the bonus applies.
		30: {SP: 0, Completed: false},
		40: {SP: 230, Completed: true},
	}, awards, "a Capot before the walk-out still pays every seat that stayed (D2)")
}

// Concurrent double disconnect on the abandonment path: every ABSENT seat earns
// zero, not just the one whose timer fired.
func TestAbandonment_EverySeatAbsentAtTheEndEarnsZero(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	awarder := &stubSPAwarder{}
	mgr := match.NewManager(hub, repo)
	mgr.SetSPAwarder(awarder)

	roomID := uint(406)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	gs := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, gs)
	gs.TeamScores[game.TeamA] = 900
	gs.TeamScores[game.TeamB] = 300
	mgr.SetGameStateForTest(roomID, gs)

	// Seat 3 drops first and is still inside its own window when seat 2's expires.
	mgr.HandleDisconnect(40)
	mgr.AbandonSeatForTest(roomID, 2)

	_, awards := awarder.snapshotCalls()
	assert.Equal(t, 0, awards[30].SP, "the expired seat earns nothing")
	assert.Equal(t, 0, awards[40].SP, "a seat absent inside its own window earns nothing either")
	assert.False(t, awards[40].Completed)
	assert.Positive(t, awards[10].SP, "the present teammate of the abandoner still earns")
	assert.Positive(t, awards[20].SP)
}

// A bot seat is never an SP subject and never receives the event — the same
// bot/empty guard settlement, XP and honor apply.
func TestHandleMatchEnd_BotSeatAccruesNoSeasonPoints(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	awarder := &stubSPAwarder{}
	mgr := match.NewManager(hub, repo)
	mgr.SetSPAwarder(awarder)
	mgr.SetBotDelayForTest(time.Hour, time.Hour)

	roomID := uint(407)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", mixedPlayers(3), "relaxed", 0, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	finalState := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, finalState)
	winner := game.TeamA
	finalState.WinnerTeam = &winner
	finalState.Phase = game.PhaseMatchEnd
	finalState.TeamScores[game.TeamA] = 500
	finalState.TeamScores[game.TeamB] = 300

	mgr.HandleMatchEndForTest(roomID, finalState, nil, ws.MatchEndPayload{WinnerTeam: game.TeamA})

	_, awards := awarder.snapshotCalls()
	assert.Equal(t, map[uint]match.SPAward{
		10: {SP: 200, Completed: true},
		20: {SP: 80, Completed: true},
		30: {SP: 200, Completed: true},
	}, awards)
	_, hasBot := awards[0]
	assert.False(t, hasBot, "bot seat (userID 0) must never appear in the awards")

	spCount := 0
	for _, c := range hub.snapshot() {
		if containsType(c.msg, "event:season_points_awarded") {
			spCount++
		}
	}
	assert.Equal(t, 3, spCount, "only the three humans receive season_points_awarded")
}

// Nil-tolerance: with no SPAwarder wired, nothing is emitted and nothing breaks
// (mirrors walletSettler, xpAwarder and honorRecorder).
func TestHandleMatchEnd_NoAwarderNoSeasonPoints(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	mgr := match.NewManager(hub, repo)
	// No SetSPAwarder call.

	roomID := uint(408)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	finalState := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, finalState)
	winner := game.TeamA
	finalState.WinnerTeam = &winner
	finalState.Phase = game.PhaseMatchEnd

	mgr.HandleMatchEndForTest(roomID, finalState, nil, ws.MatchEndPayload{WinnerTeam: game.TeamA})

	calls := hub.snapshot()
	require.GreaterOrEqual(t, firstIndexOfType(calls, "event:match_end"), 0)
	for _, c := range calls {
		assert.False(t, containsType(c.msg, "event:season_points_awarded"),
			"no season_points_awarded without an awarder")
	}
}

// Best-effort degradation: an awarder failure logs and skips the SP events but
// must NEVER block match_end or the trailing match_state.
func TestHandleMatchEnd_SeasonPointsFailureDoesNotBlockBroadcasts(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	mgr := match.NewManager(hub, repo)
	mgr.SetSPAwarder(&stubSPAwarder{err: errors.New("db down")})

	roomID := uint(409)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	finalState := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, finalState)
	winner := game.TeamA
	finalState.WinnerTeam = &winner
	finalState.Phase = game.PhaseMatchEnd

	mgr.HandleMatchEndForTest(roomID, finalState, nil, ws.MatchEndPayload{WinnerTeam: game.TeamA})

	calls := hub.snapshot()
	matchEndIdx := firstIndexOfType(calls, "event:match_end")
	require.GreaterOrEqual(t, matchEndIdx, 0, "match_end must fire even when SP fails")
	require.GreaterOrEqual(t, indexOfTypeAfter(calls, "event:match_state", matchEndIdx), 0,
		"the trailing match_state must fire even when SP fails")
	for _, c := range calls {
		assert.False(t, containsType(c.msg, "event:season_points_awarded"), "a failed write emits no event")
	}
}

// An accepted surrender routes through handleMatchEnd with the engine's winner
// already resolved, so it needs no special case: every seat completed, and the
// win bonus follows WinnerTeam rather than the higher score.
func TestHandleMatchEnd_SurrenderAwardsTheResolvedWinner(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	awarder := &stubSPAwarder{}
	mgr := match.NewManager(hub, repo)
	mgr.SetSPAwarder(awarder)

	roomID := uint(410)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	finalState := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, finalState)
	// Team A is AHEAD on points but surrendered, so team B is the winner.
	winner := game.TeamB
	finalState.WinnerTeam = &winner
	finalState.Phase = game.PhaseMatchEnd
	finalState.TeamScores[game.TeamA] = 900
	finalState.TeamScores[game.TeamB] = 200

	surrenderedBy := uint(10)
	mgr.HandleMatchEndForTest(roomID, finalState, &surrenderedBy, ws.MatchEndPayload{WinnerTeam: game.TeamB})

	_, awards := awarder.snapshotCalls()
	assert.Equal(t, map[uint]match.SPAward{
		10: {SP: 140, Completed: true}, // 50 + floor(900/10)
		20: {SP: 170, Completed: true}, // 50 + 100 + floor(200/10)
		30: {SP: 140, Completed: true},
		40: {SP: 170, Completed: true},
	}, awards, "the win bonus follows the resolved winner, not the score")
}

// A boot reconcile of a stale room is a SERVER fault, not a player signal, so
// nobody's SP moves — the same rule honor holds. reconcile.go has no live session
// and never touches the awarder; asserted explicitly so a future refactor cannot
// quietly wire it in.
func TestReconcileStaleRooms_AwardsNoSeasonPoints(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	awarder := &stubSPAwarder{}
	mgr := match.NewManager(hub, repo)
	mgr.SetSPAwarder(awarder)

	roomRepo := newStubRoomRepo()
	roomRepo.rooms = []match.StaleRoom{{ID: 901, Variant: "bitola", MatchMode: "1001", UpdatedAt: time.Now()}}
	roomRepo.players[901] = []match.StaleRoomPlayer{
		{Seat: intp(0), UserID: 10},
		{Seat: intp(1), UserID: 20},
		{Seat: intp(2), UserID: 30},
		{Seat: intp(3), UserID: 40},
	}

	require.NoError(t, mgr.ReconcileStaleRooms(roomRepo))

	calls, _ := awarder.snapshotCalls()
	assert.Zero(t, calls, "a boot reconcile is a server fault — nobody's SP moves")
	for _, c := range hub.snapshot() {
		assert.False(t, containsType(c.msg, "event:season_points_awarded"),
			"a boot reconcile must emit no season_points_awarded")
	}
}
