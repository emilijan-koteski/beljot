package match_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/ws"
)

type hubCall struct {
	userIDs []uint
	msg     []byte
	at      time.Time
	frames  map[uint][]byte
}

// hubSpy records every broadcast call so tests can assert their order and
// timing. It satisfies session.Broadcaster.
type hubSpy struct {
	mu    sync.Mutex
	calls []hubCall
}

// failIfUnprojectedState panics when an event:match_state payload is sent through
// BroadcastToUsers or SendToUser, which would leak hidden cards to every seat.
func failIfUnprojectedState(spy, primitive string, msg []byte) {
	if containsType(msg, ws.EventMatchState) {
		panic(spy + ": " + ws.EventMatchState + " must ride SendFrames with a per-seat projection (Story 12.10); got it via " + primitive)
	}
}

func (h *hubSpy) BroadcastToUsers(userIDs []uint, msg []byte) {
	failIfUnprojectedState("hubSpy", "BroadcastToUsers", msg)
	h.mu.Lock()
	defer h.mu.Unlock()
	cp := make([]uint, len(userIDs))
	copy(cp, userIDs)
	msgCopy := make([]byte, len(msg))
	copy(msgCopy, msg)
	h.calls = append(h.calls, hubCall{userIDs: cp, msg: msgCopy, at: time.Now()})
}

func (h *hubSpy) SendToUser(userID uint, msg []byte) {
	failIfUnprojectedState("hubSpy", "SendToUser", msg)
	h.mu.Lock()
	defer h.mu.Unlock()
	msgCopy := make([]byte, len(msg))
	copy(msgCopy, msg)
	h.calls = append(h.calls, hubCall{userIDs: []uint{userID}, msg: msgCopy, at: time.Now()})
}

// SendFrames records one call per non-empty frames batch, keeping the first
// frame as the representative message.
func (h *hubSpy) SendFrames(frames []ws.UserFrame) {
	if len(frames) == 0 {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	call := hubCall{at: time.Now(), frames: make(map[uint][]byte, len(frames))}
	for i, f := range frames {
		msgCopy := make([]byte, len(f.Msg))
		copy(msgCopy, f.Msg)
		call.userIDs = append(call.userIDs, f.UserID)
		call.frames[f.UserID] = msgCopy
		if i == 0 {
			call.msg = msgCopy
		}
	}
	h.calls = append(h.calls, call)
}

func (h *hubSpy) snapshot() []hubCall {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]hubCall, len(h.calls))
	copy(out, h.calls)
	return out
}

// timestampedRepo records when CreateWithHands fires and (optionally) returns
// an error to drive the broadcast-even-on-persist-failure variant.
type timestampedRepo struct {
	mu        sync.Mutex
	persistAt time.Time
	called    bool
	err       error
}

func (r *timestampedRepo) Create(m *match.Match) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.called = true
	r.persistAt = time.Now()
	if r.err != nil {
		return r.err
	}
	m.ID = 1
	return nil
}

func (r *timestampedRepo) CreateWithHands(m *match.Match, _ []match.HandResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.called = true
	r.persistAt = time.Now()
	if r.err != nil {
		return r.err
	}
	m.ID = 1
	return nil
}

func (r *timestampedRepo) GetMatchesForUser(_ uint, _, _ int, _, _ string) ([]match.Match, int64, error) {
	return nil, 0, nil
}

func (r *timestampedRepo) GetLastMatchForRoomAndUser(_, _ uint) (*match.Match, error) {
	return nil, nil
}

func (r *timestampedRepo) GetStatsForUser(_ uint) (int, int, int, error) {
	return 0, 0, 0, nil
}

func (r *timestampedRepo) GetCareerAggregatesForUser(_ uint) (match.CareerAggregates, error) {
	return match.CareerAggregates{}, nil
}

func (r *timestampedRepo) GetCareerPointsForUser(_ uint) (int64, error) {
	return 0, nil
}

func (r *timestampedRepo) GetTopPartnersForUser(_ uint, _ int) ([]match.PartnerAggregate, error) {
	return nil, nil
}

func (r *timestampedRepo) GetTopRivalsForUser(_ uint, _ int) ([]match.RivalAggregate, error) {
	return nil, nil
}

func (r *timestampedRepo) GetHonorTrendWindowsForUser(_ uint, _ int) (match.HonorTrendWindows, error) {
	return match.HonorTrendWindows{}, nil
}

func (r *timestampedRepo) wasCalled() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.called
}

func (r *timestampedRepo) at() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.persistAt
}

// TestHandleMatchEnd_PersistsBeforeBroadcast checks that the match row is
// persisted before event:match_end is broadcast.
func TestHandleMatchEnd_PersistsBeforeBroadcast(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	mgr := match.NewManager(hub, repo)

	roomID := uint(100)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	finalState := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, finalState)
	winner := game.TeamA
	finalState.WinnerTeam = &winner
	finalState.Phase = game.PhaseMatchEnd
	finalState.TeamScores = [2]int{1010, 700}

	payload := ws.MatchEndPayload{
		WinnerTeam:       game.TeamA,
		TeamAFinalScore:  1010,
		TeamBFinalScore:  700,
		MatchDurationSec: 600,
	}

	mgr.HandleMatchEndForTest(roomID, finalState, nil, payload)

	require.True(t, repo.wasCalled(), "matchRepo.CreateWithHands must be invoked")

	calls := hub.snapshot()
	require.NotEmpty(t, calls, "handleMatchEnd must broadcast at least one message")

	matchEndIdx := -1
	for i, c := range calls {
		if containsType(c.msg, "event:match_end") {
			matchEndIdx = i
			break
		}
	}
	require.GreaterOrEqual(t, matchEndIdx, 0, "event:match_end broadcast must fire")

	persistAt := repo.at()
	broadcastAt := calls[matchEndIdx].at
	assert.False(
		t,
		broadcastAt.Before(persistAt),
		"event:match_end broadcast (%v) must NOT precede persist (%v) — AC4 invariant",
		broadcastAt, persistAt,
	)

	require.Greater(t, len(calls), matchEndIdx+1, "event:match_state must follow event:match_end")
	assert.True(t,
		containsType(calls[matchEndIdx+1].msg, "event:match_state"),
		"call after event:match_end must be event:match_state",
	)
}

// TestHandleMatchEnd_BroadcastsEvenIfPersistFails locks in Task 4.3's contract:
// if matchRepo.CreateWithHands fails, the broadcast still fires. Stranding the
// four players on the table is worse than a missing match row.
func TestHandleMatchEnd_BroadcastsEvenIfPersistFails(t *testing.T) {
	repo := &timestampedRepo{err: errors.New("simulated DB outage")}
	hub := &hubSpy{}
	mgr := match.NewManager(hub, repo)

	roomID := uint(101)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true, false))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	finalState := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, finalState)
	winner := game.TeamA
	finalState.WinnerTeam = &winner
	finalState.Phase = game.PhaseMatchEnd

	payload := ws.MatchEndPayload{
		WinnerTeam:       game.TeamA,
		TeamAFinalScore:  1010,
		TeamBFinalScore:  700,
		MatchDurationSec: 600,
	}

	mgr.HandleMatchEndForTest(roomID, finalState, nil, payload)

	require.True(t, repo.wasCalled(), "persist must be attempted even when it returns an error")

	calls := hub.snapshot()
	matchEndFired := false
	for _, c := range calls {
		if containsType(c.msg, "event:match_end") {
			matchEndFired = true
			break
		}
	}
	assert.True(t, matchEndFired, "event:match_end must fire even when persist fails (AC4 / Task 4.3 contract)")
}

// containsType returns true if the JSON-encoded WS message has the given
// type. Cheap substring match — sufficient since type strings are unique
// quoted prefixes in the WSMessage JSON envelope.
func containsType(msg []byte, eventType string) bool {
	needle := `"type":"` + eventType + `"`
	return bytesContains(msg, needle)
}

func bytesContains(haystack []byte, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	if len(haystack) < len(needle) {
		return false
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
