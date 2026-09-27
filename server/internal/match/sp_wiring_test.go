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

// stubSPAwarder records the outcome ApplySeasonPoints received and scores it with
// the REAL formula (season.ComputeSPChanges, the 0 floor, the real ladder), so the
// emitted events reflect production arithmetic rather than canned values.
// Satisfies match.SPAwarder.
type stubSPAwarder struct {
	mu          sync.Mutex
	applyCalls  int
	lastOutcome match.MatchOutcome
	lastNow     time.Time
	err         error
	// priorSP seeds each user's pre-existing season total (missing = 0).
	priorSP map[uint]int
}

func (s *stubSPAwarder) ApplySeasonPoints(outcome match.MatchOutcome, now time.Time) (map[uint]match.PlayerSeasonSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.applyCalls++
	s.lastOutcome = outcome
	s.lastNow = now
	if s.err != nil {
		return nil, s.err
	}

	changes, err := season.ComputeSPChanges(outcome, s.priorSP)
	if err != nil {
		return nil, err
	}
	out := make(map[uint]match.PlayerSeasonSnapshot, len(changes))
	for id, change := range changes {
		prior := s.priorSP[id]
		next := season.ApplySPChange(prior, change)
		out[id] = match.PlayerSeasonSnapshot{
			SeasonName: "2026 Q3",
			SP:         next,
			SPChange:   next - prior,
			RankTier:   season.TierForSP(next),
			TieredUp:   season.TierClimbed(prior, next),
		}
	}
	return out, nil
}

func (s *stubSPAwarder) snapshotCalls() (int, match.MatchOutcome) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.applyCalls, s.lastOutcome
}

func (s *stubSPAwarder) snapshotNow() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastNow
}

// evenPrior puts every default player on the same total, so E = 0.5 and no
// change is floored.
func evenPrior(sp int) map[uint]int {
	return map[uint]int{10: sp, 20: sp, 30: sp, 40: sp}
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

// seasonPointsByUser collects every event:season_points_awarded payload, keyed
// by recipient.
func seasonPointsByUser(t *testing.T, hub *hubSpy) map[uint]ws.SeasonPointsAwardedPayload {
	t.Helper()
	out := map[uint]ws.SeasonPointsAwardedPayload{}
	for _, c := range hub.snapshot() {
		if containsType(c.msg, "event:season_points_awarded") {
			require.Len(t, c.userIDs, 1, "season_points_awarded is sent per user")
			out[c.userIDs[0]] = decodeSeasonPoints(t, c.msg)
		}
	}
	return out
}

// bufferCapot drives a real scored Capot hand for `team` into the session's
// buffer the way play does: an old->new state pair in which the hand advanced
// and the new state carries the scored Capot result.
func bufferCapot(t *testing.T, mgr *match.Manager, roomID uint, team int) {
	t.Helper()
	before := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, before)
	after := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, after)
	after.HandNumber = before.HandNumber + 1
	after.LastHandResult = &game.HandScore{
		HandNumber: 1, Capot: true, CapotTeam: &team, CapotBonus: 90,
		TeamACardPoints: 162, TeamAHandTotal: 252, ContractingTeam: team,
	}
	mgr.BufferHandResultIfScored(roomID, before, after)
	require.Len(t, mgr.HandResults(roomID), 1, "the capot hand must be buffered")
}

// endMatch finalizes a started session through handleMatchEnd with the given
// winner and scores.
func endMatch(t *testing.T, mgr *match.Manager, roomID uint, winner int, a, b int, surrenderedBy *uint, payload ws.MatchEndPayload, mutate func(*game.GameState)) {
	t.Helper()
	finalState := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, finalState)
	finalState.WinnerTeam = &winner
	finalState.Phase = game.PhaseMatchEnd
	finalState.TeamScores[game.TeamA] = a
	finalState.TeamScores[game.TeamB] = b
	if mutate != nil {
		mutate(finalState)
	}
	payload.WinnerTeam = winner
	mgr.HandleMatchEndForTest(roomID, finalState, surrenderedBy, payload)
}

func allPresentSeats() [4]match.OutcomeSeat {
	return [4]match.OutcomeSeat{
		{UserID: 10, Team: 0, Completed: true},
		{UserID: 20, Team: 1, Completed: true},
		{UserID: 30, Team: 0, Completed: true},
		{UserID: 40, Team: 1, Completed: true},
	}
}

// NATURAL END. The finalizer hands over the whole outcome, every human gets
// their own event carrying the APPLIED change, and the events land after
// honor_updated and before the trailing match_state. From 0 SP the losers'
// -16 floors to an applied 0.
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

	endMatch(t, mgr, roomID, game.TeamA, 1010, 700, nil, ws.MatchEndPayload{OutcomeReason: ws.OutcomeReasonNatural}, nil)

	calls, outcome := awarder.snapshotCalls()
	require.Equal(t, 1, calls)
	assert.Equal(t, match.MatchOutcome{
		Seats:      allPresentSeats(),
		WinnerTeam: game.TeamA, TeamScores: [2]int{1010, 700}, Target: 1001, AbandonedSeat: -1,
	}, outcome)
	assertFinalizerStamp(t, awarder)

	hubCalls := hub.snapshot()
	matchEndIdx := firstIndexOfType(hubCalls, "event:match_end")
	require.GreaterOrEqual(t, matchEndIdx, 0)
	trailingStateIdx := indexOfTypeAfter(hubCalls, "event:match_state", matchEndIdx)
	require.GreaterOrEqual(t, trailingStateIdx, 0)
	assert.Equal(t, 4, countTypeBetween(hubCalls, "event:season_points_awarded", matchEndIdx, trailingStateIdx),
		"all four humans receive a season_points_awarded between match_end and match_state")

	// m = 0.5 + 310/1001 = 0.81, E = 0.5: winners +24, losers -16 floored to 0.
	got := seasonPointsByUser(t, hub)
	for _, id := range []uint{10, 30} {
		assert.Equal(t, 24, got[id].SPEarned, "user %d", id)
		assert.Equal(t, 24, got[id].NewSeasonSP)
	}
	for _, id := range []uint{20, 40} {
		assert.Equal(t, 0, got[id].SPEarned, "user %d: the event carries the applied change, not -16", id)
		assert.Equal(t, 0, got[id].NewSeasonSP)
	}
	for id, p := range got {
		assert.Equal(t, "2026 Q3", p.SeasonName, "user %d: the machine-stable window token rides along", id)
		assert.Equal(t, "iron", p.RankTier, "a stable token, never a display string")
		assert.False(t, p.TieredUp)
	}
}

// ORDERING — season_points_awarded lands strictly AFTER honor_updated and
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

	endMatch(t, mgr, roomID, game.TeamA, 1010, 700, nil, ws.MatchEndPayload{}, nil)

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

// TIER CHANGES ARE PER PLAYER, AND ONLY A CLIMB IS A TIER-UP. User 10 climbs
// from Iron into Bronze; user 20 drops from Bronze into Iron, which must not
// read as a tier-up.
func TestHandleMatchEnd_TierUpIsPerPlayerAndOnlyForAClimb(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	// Team A averages 70, team B 80, so E_A = 0.485: A wins +25, B loses -17.
	awarder := &stubSPAwarder{priorSP: map[uint]int{10: 140, 20: 160}}
	mgr := match.NewManager(hub, repo)
	mgr.SetSPAwarder(awarder)

	roomID := uint(402)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	endMatch(t, mgr, roomID, game.TeamA, 1010, 700, nil, ws.MatchEndPayload{}, nil)

	got := seasonPointsByUser(t, hub)
	require.Len(t, got, 4)

	assert.True(t, got[10].TieredUp, "140 + 25 crosses the 150 Bronze floor")
	assert.Equal(t, 165, got[10].NewSeasonSP)
	assert.Equal(t, "bronze", got[10].RankTier)

	assert.False(t, got[20].TieredUp, "a drop from Bronze to Iron is not a tier-up")
	assert.Equal(t, -17, got[20].SPEarned, "a loss is carried as a negative change")
	assert.Equal(t, 143, got[20].NewSeasonSP)
	assert.Equal(t, "iron", got[20].RankTier)

	assert.False(t, got[30].TieredUp)
	assert.Equal(t, 25, got[30].SPEarned)
	assert.False(t, got[40].TieredUp)
	assert.Equal(t, 0, got[40].SPEarned, "a loss from 0 SP applies nothing")
}

// A Capot reaches the outcome from the BUFFERED hand results, for the team that
// made it. This also exercises the hoist: the hand-results copy happens before
// the SP award, so a Capot recorded during play actually reaches the formula.
func TestHandleMatchEnd_CapotBonusGoesToTheCapotTeam(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	awarder := &stubSPAwarder{priorSP: evenPrior(500)}
	mgr := match.NewManager(hub, repo)
	mgr.SetSPAwarder(awarder)

	roomID := uint(403)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	bufferCapot(t, mgr, roomID, game.TeamB)
	endMatch(t, mgr, roomID, game.TeamA, 1010, 700, nil, ws.MatchEndPayload{}, nil)

	_, outcome := awarder.snapshotCalls()
	assert.Equal(t, [2]bool{false, true}, outcome.CapotTeams)

	got := seasonPointsByUser(t, hub)
	assert.Equal(t, 24, got[10].SPEarned, "the winners made no Capot")
	assert.Equal(t, -16+5, got[20].SPEarned, "the losing team's Capot softens its loss")
	assert.Equal(t, -11, got[40].SPEarned)
}

// The 501 target reaches the outcome from the match mode, so the margin scales by
// it: 520:220 is m = 0.5 + 300/501 = 1.10 (it would be 0.80 against 1001).
func TestHandleMatchEnd_The501TargetScalesTheMargin(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	awarder := &stubSPAwarder{priorSP: evenPrior(500)}
	mgr := match.NewManager(hub, repo)
	mgr.SetSPAwarder(awarder)

	roomID := uint(414)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "501", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	endMatch(t, mgr, roomID, game.TeamA, 520, 220, nil, ws.MatchEndPayload{}, nil)

	_, outcome := awarder.snapshotCalls()
	assert.Equal(t, 501, outcome.Target)
	got := seasonPointsByUser(t, hub)
	assert.Equal(t, 33, got[10].SPEarned)
	assert.Equal(t, -22, got[20].SPEarned)
}

// An instant win on a fresh deal: TeamScores [0,0] and no hand results, yet the
// maximum margin still applies, because the engine records the outcome on the
// state instead of leaving the match layer to infer it.
func TestHandleMatchEnd_InstantWinIsTheMaximumMargin(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	awarder := &stubSPAwarder{priorSP: evenPrior(500)}
	mgr := match.NewManager(hub, repo)
	mgr.SetSPAwarder(awarder)

	roomID := uint(404)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	endMatch(t, mgr, roomID, game.TeamA, 0, 0, nil, ws.MatchEndPayload{}, func(gs *game.GameState) {
		gs.WonByInstantWin = true
	})

	_, outcome := awarder.snapshotCalls()
	assert.True(t, outcome.InstantWin)
	got := seasonPointsByUser(t, hub)
	assert.Equal(t, 45, got[10].SPEarned, "m = 1.5: 30 × 1.5 × 2 × 0.5")
	assert.Equal(t, -30, got[20].SPEarned)
}

// An accepted surrender routes through handleMatchEnd with the engine's winner
// already resolved: the outcome is a surrender, the winners' points count as the
// target, and the win follows WinnerTeam rather than the higher score.
func TestHandleMatchEnd_SurrenderScoresAsIfTheWinnersReachedTheTarget(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	awarder := &stubSPAwarder{priorSP: evenPrior(500)}
	mgr := match.NewManager(hub, repo)
	mgr.SetSPAwarder(awarder)

	roomID := uint(410)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	// Team A is AHEAD on points but surrendered, so team B is the winner.
	surrenderedBy := uint(10)
	proposer := 0
	endMatch(t, mgr, roomID, game.TeamB, 900, 200, &surrenderedBy,
		ws.MatchEndPayload{OutcomeReason: ws.OutcomeReasonSurrender, SurrenderedBySeat: &proposer}, nil)

	_, outcome := awarder.snapshotCalls()
	assert.True(t, outcome.Surrender)
	assert.Equal(t, game.TeamB, outcome.WinnerTeam)

	// m = 0.5 + (1001 - 900)/1001 = 0.60, E = 0.5: B +18, A -12.
	got := seasonPointsByUser(t, hub)
	assert.Equal(t, 18, got[20].SPEarned, "the win follows the resolved winner, not the score")
	assert.Equal(t, 18, got[40].SPEarned)
	assert.Equal(t, -12, got[10].SPEarned)
	assert.Equal(t, -12, got[30].SPEarned)
}

// The same, end to end: a real surrender request and accept through the action
// path, so the outcome's Surrender flag comes from the match-end payload the
// engine's result actually produced rather than from a hand-built one.
func TestSurrender_ThroughTheActionPathIsScoredAsASurrender(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	awarder := &stubSPAwarder{priorSP: evenPrior(500)}
	mgr := match.NewManager(hub, repo)
	mgr.SetSPAwarder(awarder)

	roomID := uint(413)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	// Seat 0 (team A) proposes, seat 2 (its partner) accepts.
	mgr.HandleAction(&ws.Client{UserID: 10}, ws.WSMessage{Type: "action:surrender_request", Payload: json.RawMessage(`{}`)})
	mgr.HandleAction(&ws.Client{UserID: 30}, ws.WSMessage{Type: "action:surrender_accept", Payload: json.RawMessage(`{}`)})
	require.True(t, waitFor(2*time.Second, func() bool {
		calls, _ := awarder.snapshotCalls()
		return calls == 1
	}), "the accepted surrender must reach the awarder")

	_, outcome := awarder.snapshotCalls()
	assert.True(t, outcome.Surrender)
	assert.Equal(t, game.TeamB, outcome.WinnerTeam)
	assert.Equal(t, -1, outcome.AbandonedSeat)
	// 0:0 before any hand, scored as a surrender: m = 0.5 + 1001/1001 = 1.5.
	got := seasonPointsByUser(t, hub)
	assert.Equal(t, 45, got[20].SPEarned)
	assert.Equal(t, -30, got[10].SPEarned)
}

// A surrender accepted with a team already over the target in a "dosta" room
// runs the stop and finalizes as target_reached, surrenderedBy still set. The
// winners really reached the target, so it is scored on the real margin, not as
// a surrender.
func TestHandleMatchEnd_SurrenderThatFinalizesAtTargetIsANaturalMargin(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	awarder := &stubSPAwarder{priorSP: evenPrior(500)}
	mgr := match.NewManager(hub, repo)
	mgr.SetSPAwarder(awarder)

	roomID := uint(412)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	surrenderedBy := uint(20)
	proposer := 1
	endMatch(t, mgr, roomID, game.TeamA, 1100, 600, &surrenderedBy,
		ws.MatchEndPayload{OutcomeReason: ws.OutcomeReasonTargetReached, SurrenderedBySeat: &proposer}, nil)

	_, outcome := awarder.snapshotCalls()
	assert.False(t, outcome.Surrender, "a target_reached finish is not scored as a surrender")

	// Natural m = 0.5 + 500/1001 = 1.00: +30 / -20. Scored as a surrender the
	// winners' 1100 would have been cut to the 1001 target, m = 0.90: +27 / -18.
	got := seasonPointsByUser(t, hub)
	assert.Equal(t, 30, got[10].SPEarned)
	assert.Equal(t, -20, got[20].SPEarned)
}

// ABANDONMENT. The abandoner takes the fixed penalty, the teammate half of the
// surrender loss, the opponents a surrender-scored win. Every human seat still
// counts a games_played, so every one of them is in the outcome, the abandoner
// with Completed false.
func TestAbandonment_ScoresEverySeatByItsRole(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	awarder := &stubSPAwarder{priorSP: evenPrior(500)}
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

	calls, outcome := awarder.snapshotCalls()
	require.Equal(t, 1, calls)
	seats := allPresentSeats()
	seats[2].Completed = false
	assert.Equal(t, match.MatchOutcome{
		Seats:      seats,
		WinnerTeam: game.TeamB, TeamScores: [2]int{900, 300}, Target: 1001, AbandonedSeat: 2,
	}, outcome, "the winner is the non-abandoning team whatever the score")
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

	// Surrender margin m = 0.5 + (1001 - 900)/1001 = 0.60, E = 0.5.
	got := seasonPointsByUser(t, hub)
	assert.Equal(t, -120, got[30].SPEarned, "the abandoner: -2 × (20 × 1.5 × 2)")
	assert.Equal(t, 380, got[30].NewSeasonSP)
	assert.False(t, got[30].TieredUp)
	assert.Equal(t, -6, got[10].SPEarned, "the teammate: half of the -12 a surrender costs")
	assert.Equal(t, 18, got[20].SPEarned, "the opponents: a surrender-scored win")
	assert.Equal(t, 18, got[40].SPEarned)
}

// The abandonment finalizer reads the 501 target too: its surrender margin is
// m = 0.5 + (501 - 200)/501 = 1.10, so the opponents get +33 and the teammate
// half of -22.
func TestAbandonment_The501TargetScalesTheMargin(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	awarder := &stubSPAwarder{priorSP: evenPrior(500)}
	mgr := match.NewManager(hub, repo)
	mgr.SetSPAwarder(awarder)

	roomID := uint(415)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "501", defaultPlayers(), "per-move", 30, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	gs := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, gs)
	gs.TeamScores[game.TeamA] = 200
	gs.TeamScores[game.TeamB] = 300
	mgr.SetGameStateForTest(roomID, gs)

	mgr.AbandonSeatForTest(roomID, 2)

	_, outcome := awarder.snapshotCalls()
	assert.Equal(t, 501, outcome.Target)
	got := seasonPointsByUser(t, hub)
	assert.Equal(t, 33, got[20].SPEarned)
	assert.Equal(t, -11, got[10].SPEarned)
	assert.Equal(t, -120, got[30].SPEarned)
}

// The Capot teams are captured on the ABANDONMENT finalizer too, from its own
// under-lock copy of the hand results — covering them only on the natural end
// would leave this path free to drop them without any test noticing. The
// teammate keeps the Capot bonus; the abandoner never gets it.
func TestAbandonment_CapotBonusSkipsOnlyTheAbandoner(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	awarder := &stubSPAwarder{priorSP: evenPrior(500)}
	mgr := match.NewManager(hub, repo)
	mgr.SetSPAwarder(awarder)

	roomID := uint(411)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	bufferCapot(t, mgr, roomID, game.TeamA)

	gs := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, gs)
	gs.TeamScores[game.TeamA] = 900
	gs.TeamScores[game.TeamB] = 300
	mgr.SetGameStateForTest(roomID, gs)

	mgr.AbandonSeatForTest(roomID, 2)

	_, outcome := awarder.snapshotCalls()
	assert.Equal(t, [2]bool{true, false}, outcome.CapotTeams)
	got := seasonPointsByUser(t, hub)
	assert.Equal(t, -120, got[30].SPEarned, "no Capot bonus for the abandoner")
	assert.Equal(t, -6+5, got[10].SPEarned, "the teammate keeps the team's Capot bonus")
	assert.Equal(t, 18, got[20].SPEarned)
}

// PRESENCE DOES NOT GATE SP. Seat 3 drops first and is still inside its own
// window when seat 2's expires: it is absent at the end (Completed false, so no
// games_completed) but is still scored by its team's result — a win.
func TestAbandonment_AnAbsentOpponentIsStillScoredByTheResult(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	awarder := &stubSPAwarder{priorSP: evenPrior(500)}
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

	mgr.HandleDisconnect(40)
	mgr.AbandonSeatForTest(roomID, 2)

	_, outcome := awarder.snapshotCalls()
	assert.False(t, outcome.Seats[2].Completed, "the expired seat is absent")
	assert.False(t, outcome.Seats[3].Completed, "a seat inside its own window is absent too")
	assert.True(t, outcome.Seats[0].Completed)
	assert.True(t, outcome.Seats[1].Completed)

	got := seasonPointsByUser(t, hub)
	assert.Equal(t, 18, got[40].SPEarned, "the absent opponent still gets the team's win")
	assert.Equal(t, -120, got[30].SPEarned, "only the expired seat takes the penalty")
	assert.Equal(t, -6, got[10].SPEarned)
}

// BOT AVERAGING. A bot seat is never an SP subject and never receives the event,
// but it counts as the Gold 1 floor (600) in its team's average. Team B here is
// user 20 (100 SP) and a bot, an average of 350 against team A's 0, so team A's
// win is worth far more than against an even table.
func TestHandleMatchEnd_BotSeatCountsAsTheGoldFloor(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	awarder := &stubSPAwarder{priorSP: map[uint]int{20: 100}}
	mgr := match.NewManager(hub, repo)
	mgr.SetSPAwarder(awarder)
	mgr.SetBotDelayForTest(time.Hour, time.Hour)

	roomID := uint(407)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", mixedPlayers(3), "relaxed", 0, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	endMatch(t, mgr, roomID, game.TeamA, 500, 300, nil, ws.MatchEndPayload{}, nil)

	_, outcome := awarder.snapshotCalls()
	assert.Equal(t, match.OutcomeSeat{IsBot: true, Team: 1}, outcome.Seats[3], "a bot seat carries no user")

	// m = 0.5 + 200/1001 = 0.70; E_A = 1/(1+10^(350/385)) = 0.11: A +37, B -25.
	// (With the bot counted as 0 SP, team B would average 50 and A get +24.)
	got := seasonPointsByUser(t, hub)
	require.Len(t, got, 3, "only the three humans receive season_points_awarded")
	assert.Equal(t, 37, got[10].SPEarned)
	assert.Equal(t, 37, got[30].SPEarned)
	assert.Equal(t, -25, got[20].SPEarned)
	assert.Equal(t, 75, got[20].NewSeasonSP)
	_, hasBot := got[0]
	assert.False(t, hasBot, "bot seat (userID 0) must never receive an event")
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

	endMatch(t, mgr, roomID, game.TeamA, 0, 0, nil, ws.MatchEndPayload{}, nil)

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

	endMatch(t, mgr, roomID, game.TeamA, 0, 0, nil, ws.MatchEndPayload{}, nil)

	calls := hub.snapshot()
	matchEndIdx := firstIndexOfType(calls, "event:match_end")
	require.GreaterOrEqual(t, matchEndIdx, 0, "match_end must fire even when SP fails")
	require.GreaterOrEqual(t, indexOfTypeAfter(calls, "event:match_state", matchEndIdx), 0,
		"the trailing match_state must fire even when SP fails")
	for _, c := range calls {
		assert.False(t, containsType(c.msg, "event:season_points_awarded"), "a failed write emits no event")
	}
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
