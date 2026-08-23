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

// --- Test doubles ---
//
// Story 8.5-1 AC7: new tests added by Tasks 1, 4 use synchronous assertions
// (handleMatchEnd is synchronous, so no polling is needed) instead of
// time.Sleep. If a future async test path is added, prefer channels or a
// waitFor(cond, timeout) polling helper — never a fixed sleep.

type hubCall struct {
	userIDs []uint
	msg     []byte
	at      time.Time
	// frames holds the per-recipient bytes when the call came through
	// SendFrames (per-seat projected match_state, Story 12.10); nil for
	// BroadcastToUsers / SendToUser calls. msg is then the FIRST frame, kept
	// as the representative for type/ordering assertions — every frame in one
	// call carries the same event type.
	frames map[uint][]byte
}

// hubSpy records every BroadcastToUsers / SendToUser call so tests can assert
// the order and timing of broadcasts. Mirrors the chat / emote handler test
// pattern (chat/handler_test.go, emote/handler_test.go) — same shape, no new
// mocking style introduced. Satisfies session.Broadcaster.
type hubSpy struct {
	mu    sync.Mutex
	calls []hubCall
}

// failIfUnprojectedState hard-fails the run when an event:match_state payload
// arrives through an identical-bytes primitive. Story 12.10 routes EVERY state
// frame through SendFrames carrying a per-seat game.ProjectForSeat mask; a
// state payload on BroadcastToUsers or SendToUser is an unprojected send that
// would ship hidden cards to all four seats. It panics (rather than t.Errorf)
// so the failure fires AT the offending send with a stack trace naming the
// call site, without threading *testing.T through every spy constructor —
// this turns the whole wiring suite into an adoption sweep.
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

// SendFrames records ONE call per frames batch — the wire log keeps one entry
// per logical state event, so every count/order assertion over calls still
// holds, and a single-recipient entry still means a targeted SendToUser.
// An empty batch (every recipient seat is a bot) records nothing, mirroring
// recordingBroadcaster — a phantom nil-msg call would poison type decoding.
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

// --- Tests (Story 8.5-1 AC4 — resolves D101) ---

// TestHandleMatchEnd_PersistsBeforeBroadcast locks in the AC4 invariant:
// matchRepo.CreateWithHands MUST complete BEFORE event:match_end is broadcast,
// so a client that receives match_end and immediately reads the match row
// (Story 9.1+) finds it.
func TestHandleMatchEnd_PersistsBeforeBroadcast(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	mgr := match.NewManager(hub, repo)

	roomID := uint(100)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	// Build a final state and a payload that look like a real match end.
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

	// The first hubSpy call after handleMatchEnd must be event:match_end and
	// must occur AFTER the persist timestamp.
	calls := hub.snapshot()
	require.NotEmpty(t, calls, "handleMatchEnd must broadcast at least one message")

	// Find the event:match_end broadcast.
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

	// match_state must follow match_end (preserves client-facing order so
	// MatchPage's stale-state redirect does not race matchEndData arrival).
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
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0))
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
