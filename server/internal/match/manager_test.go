package match_test

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/ws"
)

type mockMatchRepo struct {
	mu      sync.Mutex
	matches []*match.Match
	hands   [][]match.HandResult
	err     error
}

func newMockMatchRepo() *mockMatchRepo {
	return &mockMatchRepo{}
}

func (r *mockMatchRepo) Create(m *match.Match) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	m.ID = uint(len(r.matches) + 1)
	r.matches = append(r.matches, m)
	r.hands = append(r.hands, nil)
	return nil
}

func (r *mockMatchRepo) CreateWithHands(m *match.Match, hands []match.HandResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	m.ID = uint(len(r.matches) + 1)
	r.matches = append(r.matches, m)
	handsCopy := make([]match.HandResult, len(hands))
	copy(handsCopy, hands)
	for i := range handsCopy {
		handsCopy[i].MatchID = m.ID
	}
	r.hands = append(r.hands, handsCopy)
	return nil
}

func (r *mockMatchRepo) GetMatchesForUser(userID uint, limit, offset int, outcome, sort string) ([]match.Match, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, 0, r.err
	}
	_, _, _, _, _ = userID, limit, offset, outcome, sort
	return nil, 0, nil
}

// Not exercised by session-manager tests — the room-scoped read belongs to the
// room last-match endpoint, which the manager never calls.
func (r *mockMatchRepo) GetLastMatchForRoomAndUser(roomID, userID uint) (*match.Match, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	_, _ = roomID, userID
	return nil, nil
}

func (r *mockMatchRepo) GetStatsForUser(userID uint) (int, int, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return 0, 0, 0, r.err
	}
	_ = userID
	return 0, 0, 0, nil
}

// Career aggregates are not exercised by session-manager tests.
func (r *mockMatchRepo) GetCareerAggregatesForUser(uint) (match.CareerAggregates, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return match.CareerAggregates{}, r.err
}

func (r *mockMatchRepo) GetCareerPointsForUser(uint) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return 0, r.err
}

func (r *mockMatchRepo) GetTopPartnersForUser(uint, int) ([]match.PartnerAggregate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return nil, r.err
}

func (r *mockMatchRepo) GetTopRivalsForUser(uint, int) ([]match.RivalAggregate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return nil, r.err
}

func (r *mockMatchRepo) GetHonorTrendWindowsForUser(uint, int) (match.HonorTrendWindows, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return match.HonorTrendWindows{}, r.err
}

func (r *mockMatchRepo) getMatches() []*match.Match {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]*match.Match, len(r.matches))
	copy(result, r.matches)
	return result
}

// getHands returns the hand rows persisted with the match at index i. Use it
// after a match ends, since Manager.HandResults reads the already-removed session.
func (r *mockMatchRepo) getHands(i int) []match.HandResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	if i < 0 || i >= len(r.hands) {
		return nil
	}
	out := make([]match.HandResult, len(r.hands[i]))
	copy(out, r.hands[i])
	return out
}

func defaultPlayers() [4]match.PlayerSeatInfo {
	return [4]match.PlayerSeatInfo{
		{UserID: 10, Username: "player1", Seat: 0},
		{UserID: 20, Username: "player2", Seat: 1},
		{UserID: 30, Username: "player3", Seat: 2},
		{UserID: 40, Username: "player4", Seat: 3},
	}
}

func TestStartMatch_CreatesSession(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	err := mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true)
	require.NoError(t, err)

	assert.True(t, mgr.HasSession(100))

	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)
	assert.Equal(t, uint(100), state.RoomID)
	assert.Equal(t, game.Variant("bitola"), state.Variant)
	assert.Equal(t, "1001", state.MatchMode)
	assert.Equal(t, game.PhaseBidding, state.Phase)
	assert.Equal(t, uint(10), state.Players[0].UserID)
	assert.Equal(t, uint(20), state.Players[1].UserID)
	assert.Equal(t, uint(30), state.Players[2].UserID)
	assert.Equal(t, uint(40), state.Players[3].UserID)
}

// Each human seat's avatar URL rides from the room's seat info onto the game
// state, where every event:match_state carries it; bot seats and players
// without one stay nil (null on the wire).
func TestStartMatch_StampsAvatarURLs(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	mgr := match.NewManager(hub, newMockMatchRepo())
	url := "https://assets.test/avatars/55555555-5555-4555-8555-555555555555/128.webp"
	players := [4]match.PlayerSeatInfo{
		{UserID: 10, Username: "player1", Seat: 0, AvatarURL: &url},
		{UserID: 20, Username: "player2", Seat: 1},
		{UserID: 30, Username: "player3", Seat: 2},
		{Seat: 3, IsBot: true},
	}
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", players, "relaxed", 0, 10, 120, 0, true, false, true))
	t.Cleanup(func() { mgr.RemoveSession(100) })

	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)
	require.NotNil(t, state.Players[0].AvatarURL)
	assert.Equal(t, url, *state.Players[0].AvatarURL)
	assert.Nil(t, state.Players[1].AvatarURL)
	assert.Nil(t, state.Players[3].AvatarURL, "a bot seat has no avatar")

	projected := game.ProjectForSeat(state, 1)
	require.NotNil(t, projected.Players[0].AvatarURL, "the per-seat projection keeps every seat's avatar")
	assert.Equal(t, url, *projected.Players[0].AvatarURL)
}

func TestStartMatch_501MatchMode_PropagatesToState(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	err := mgr.StartMatch(100, "bitola", "501", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true)
	require.NoError(t, err)

	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)
	assert.Equal(t, "501", state.MatchMode)
}

func TestStartMatch_BroadcastsInitialState(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	err := mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)
	assert.Equal(t, game.PhaseBidding, state.Phase)
}

func TestHandleAction_InvalidUser(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	client := &ws.Client{UserID: 999}
	payload, _ := json.Marshal(map[string]string{"cardId": "KS"})
	msg := ws.WSMessage{
		Type:    "action:play_card",
		Payload: payload,
	}

	mgr.HandleAction(client, msg)
	time.Sleep(50 * time.Millisecond)
}

func TestHandleAction_PlayCard_ParsesCorrectly(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	err := mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true)
	require.NoError(t, err)

	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)

	activeSeat := state.ActivePlayerSeat
	activeUserID := state.Players[activeSeat].UserID
	require.True(t, len(state.Players[activeSeat].Hand) > 0, "active player should have cards")

	card := state.Players[activeSeat].Hand[0]
	payload, _ := json.Marshal(map[string]string{"cardId": card.String()})
	msg := ws.WSMessage{
		Type:    "action:play_card",
		Payload: payload,
	}

	client := &ws.Client{UserID: activeUserID}
	mgr.HandleAction(client, msg)
	time.Sleep(50 * time.Millisecond)
}

func TestHandleAction_BiddingActions(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	err := mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true)
	require.NoError(t, err)

	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)
	assert.Equal(t, game.PhaseBidding, state.Phase)

	activeSeat := state.ActivePlayerSeat
	activeUserID := state.Players[activeSeat].UserID

	msg := ws.WSMessage{
		Type:    "action:pick_trump",
		Payload: json.RawMessage(`{}`),
	}
	client := &ws.Client{UserID: activeUserID}
	mgr.HandleAction(client, msg)
	time.Sleep(100 * time.Millisecond)

	newState := mgr.GetStateSnapshot(100)
	require.NotNil(t, newState)
	assert.Equal(t, game.PhasePlaying, newState.Phase)
	assert.NotNil(t, newState.TrumpSuit)
}

func TestGetStateSnapshot_NoSession(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	state := mgr.GetStateSnapshot(999)
	assert.Nil(t, state)
}

func TestRemoveSession(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	err := mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true)
	require.NoError(t, err)
	assert.True(t, mgr.HasSession(100))

	mgr.RemoveSession(100)
	assert.False(t, mgr.HasSession(100))
	assert.Nil(t, mgr.GetStateSnapshot(100))
}

func TestHasSession(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	assert.False(t, mgr.HasSession(100))

	err := mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true)
	require.NoError(t, err)

	assert.True(t, mgr.HasSession(100))
}

func TestIsUserInMatch(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	assert.False(t, mgr.IsUserInMatch(10))
	assert.False(t, mgr.IsUserInMatch(999))

	err := mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true)
	require.NoError(t, err)

	assert.True(t, mgr.IsUserInMatch(10))
	assert.True(t, mgr.IsUserInMatch(20))
	assert.True(t, mgr.IsUserInMatch(30))
	assert.True(t, mgr.IsUserInMatch(40))
	assert.False(t, mgr.IsUserInMatch(999), "non-player should not be in game")

	mgr.RemoveSession(100)
	assert.False(t, mgr.IsUserInMatch(10))
	assert.False(t, mgr.IsUserInMatch(20))
	assert.False(t, mgr.IsUserInMatch(30))
	assert.False(t, mgr.IsUserInMatch(40))
}

func TestMatchParticipantsByUser(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	ids, seat, ok := mgr.MatchParticipantsByUser(10)
	assert.False(t, ok)
	assert.Equal(t, -1, seat)
	assert.Equal(t, [4]uint{}, ids)

	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true))

	for expectedSeat, userID := range []uint{10, 20, 30, 40} {
		ids, seat, ok := mgr.MatchParticipantsByUser(userID)
		require.True(t, ok, "user %d should resolve to an active session", userID)
		assert.Equal(t, [4]uint{10, 20, 30, 40}, ids)
		assert.Equal(t, expectedSeat, seat,
			"user %d expected at seat %d", userID, expectedSeat)
	}

	ids, seat, ok = mgr.MatchParticipantsByUser(999)
	assert.False(t, ok)
	assert.Equal(t, -1, seat)
	assert.Equal(t, [4]uint{}, ids)

	mgr.RemoveSession(100)
	for _, userID := range []uint{10, 20, 30, 40} {
		_, _, ok := mgr.MatchParticipantsByUser(userID)
		assert.False(t, ok, "user %d should no longer resolve after session removal", userID)
	}
}

func TestMatchParticipants(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	ids, ok := mgr.MatchParticipants(100)
	assert.False(t, ok)
	assert.Equal(t, [4]uint{}, ids)

	err := mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true)
	require.NoError(t, err)

	ids, ok = mgr.MatchParticipants(100)
	require.True(t, ok)
	assert.Equal(t, [4]uint{10, 20, 30, 40}, ids)

	_, ok = mgr.MatchParticipants(999)
	assert.False(t, ok)

	mgr.RemoveSession(100)
	ids, ok = mgr.MatchParticipants(100)
	assert.False(t, ok)
	assert.Equal(t, [4]uint{}, ids)
}

func TestStartMatch_PerMoveTimer_SetsTurnExpiresAt(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	err := mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120, 0, true, false, true)
	require.NoError(t, err)

	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)
	assert.NotNil(t, state.TurnExpiresAt, "per-move timer should set TurnExpiresAt")
	assert.Equal(t, 30, state.TimerDurationSec)
	assert.True(t, state.TurnExpiresAt.After(time.Now()), "TurnExpiresAt should be in the future")
}

func TestStartMatch_RelaxedTimer_NilTurnExpiresAt(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	err := mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true)
	require.NoError(t, err)

	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)
	assert.Nil(t, state.TurnExpiresAt, "relaxed timer should NOT set TurnExpiresAt")
	assert.Equal(t, 0, state.TimerDurationSec)
}

func TestHandleAction_PerMoveTimer_ResetsOnAction(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	mgr.SetDealGraceForTest(false)

	err := mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120, 0, true, false, true)
	require.NoError(t, err)

	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)
	firstExpiry := state.TurnExpiresAt
	require.NotNil(t, firstExpiry)

	activeSeat := state.ActivePlayerSeat
	activeUserID := state.Players[activeSeat].UserID

	client := &ws.Client{UserID: activeUserID}
	msg := ws.WSMessage{
		Type:    "action:pick_trump",
		Payload: json.RawMessage(`{}`),
	}
	mgr.HandleAction(client, msg)
	time.Sleep(100 * time.Millisecond)

	newState := mgr.GetStateSnapshot(100)
	require.NotNil(t, newState)
	if newState.TurnExpiresAt != nil {
		assert.True(t, !newState.TurnExpiresAt.Before(*firstExpiry),
			"new TurnExpiresAt should be >= first expiry (timer reset)")
	}
}

func TestPerMoveTimer_AutoPlayOnExpiry(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	err := mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 1, 10, 120, 0, true, false, true)
	require.NoError(t, err)

	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)
	assert.Equal(t, game.PhaseBidding, state.Phase)

	activeSeat := state.ActivePlayerSeat
	activeUserID := state.Players[activeSeat].UserID
	client := &ws.Client{UserID: activeUserID}
	msg := ws.WSMessage{
		Type:    "action:pick_trump",
		Payload: json.RawMessage(`{}`),
	}
	mgr.HandleAction(client, msg)
	time.Sleep(100 * time.Millisecond)

	playingState := mgr.GetStateSnapshot(100)
	require.NotNil(t, playingState)
	assert.Equal(t, game.PhasePlaying, playingState.Phase)

	totalBefore := 0
	for _, p := range playingState.Players {
		totalBefore += len(p.Hand)
	}

	time.Sleep(4500 * time.Millisecond)

	afterAutoPlay := mgr.GetStateSnapshot(100)
	require.NotNil(t, afterAutoPlay, "session should still exist after auto-play")

	cardCountAfter := 0
	for _, p := range afterAutoPlay.Players {
		cardCountAfter += len(p.Hand)
	}
	assert.Less(t, cardCountAfter, totalBefore, "auto-play should have played at least one card")
}

func TestPerMoveTimer_ConcurrentActionAndExpiry(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	err := mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 1, 10, 120, 0, true, false, true)
	require.NoError(t, err)

	state := mgr.GetStateSnapshot(100)
	activeSeat := state.ActivePlayerSeat
	activeUserID := state.Players[activeSeat].UserID
	client := &ws.Client{UserID: activeUserID}
	msg := ws.WSMessage{
		Type:    "action:pick_trump",
		Payload: json.RawMessage(`{}`),
	}
	mgr.HandleAction(client, msg)
	time.Sleep(50 * time.Millisecond)

	playingState := mgr.GetStateSnapshot(100)
	require.NotNil(t, playingState)
	require.Equal(t, game.PhasePlaying, playingState.Phase)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := mgr.GetStateSnapshot(100)
			if s == nil {
				return
			}
			seat := s.ActivePlayerSeat
			uid := s.Players[seat].UserID
			c := &ws.Client{UserID: uid}
			if len(s.Players[seat].Hand) > 0 {
				card := s.Players[seat].Hand[0]
				payload, _ := json.Marshal(map[string]string{"cardId": card.String()})
				m := ws.WSMessage{
					Type:    "action:play_card",
					Payload: payload,
				}
				mgr.HandleAction(c, m)
			}
		}()
	}

	time.Sleep(1200 * time.Millisecond)
	wg.Wait()

	finalState := mgr.GetStateSnapshot(100)
	require.NotNil(t, finalState, "session should survive concurrent access")
}

// bufferTestResult builds a game.HandScore with distinctive values for the given
// hand, so mapping correctness can be asserted.
func bufferTestResult(handNumber, a, b int) *game.HandScore {
	capotTeam := game.TeamB
	return &game.HandScore{
		HandNumber:      handNumber,
		TeamACardPoints: a,
		TeamBCardPoints: b,
		TeamADeclPoints: 20,
		TeamBDeclPoints: 50,
		LastTrickTeam:   game.TeamA,
		LastTrickBonus:  10,
		Capot:           true,
		CapotTeam:       &capotTeam,
		CapotBonus:      100,
		FailedContract:  true,
		ContractingTeam: game.TeamA,
		TeamAHandTotal:  a + 20,
		TeamBHandTotal:  b + 150,
	}
}

func TestBufferHandResultIfScored_HandAdvanced(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	require.NoError(t, mgr.StartMatch(900, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true))

	old := &game.GameState{HandNumber: 3}
	next := &game.GameState{HandNumber: 4, LastHandResult: bufferTestResult(3, 81, 81)}

	mgr.BufferHandResultIfScored(900, old, next)

	hands := mgr.HandResults(900)
	require.Len(t, hands, 1)
	assert.Equal(t, 3, hands[0].HandNumber,
		"buffered hand number must be the RESULT's own hand (the hand just completed)")
	assert.Equal(t, 81, hands[0].TeamACardPoints)
	assert.Equal(t, 81, hands[0].TeamBCardPoints)
	assert.True(t, hands[0].Capot)
	require.NotNil(t, hands[0].CapotTeam)
	assert.Equal(t, game.TeamB, *hands[0].CapotTeam)
	assert.True(t, hands[0].FailedContract)
	assert.Equal(t, 101, hands[0].TeamAHandTotal)
	assert.Equal(t, 231, hands[0].TeamBHandTotal)
}

func TestBufferHandResultIfScored_MatchEnd(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	require.NoError(t, mgr.StartMatch(901, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true))

	old := &game.GameState{HandNumber: 7}
	next := &game.GameState{HandNumber: 7, Phase: game.PhaseMatchEnd, LastHandResult: bufferTestResult(7, 100, 62)}

	mgr.BufferHandResultIfScored(901, old, next)

	hands := mgr.HandResults(901)
	require.Len(t, hands, 1)
	assert.Equal(t, 7, hands[0].HandNumber,
		"match-end buffered hand number must be the result's own hand (no startNewHand ran)")
}

func TestBufferHandResultIfScored_Noop_NoTransition(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	require.NoError(t, mgr.StartMatch(902, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true))

	old := &game.GameState{HandNumber: 2}
	next := &game.GameState{HandNumber: 2, Phase: game.PhasePlaying, LastHandResult: bufferTestResult(2, 10, 20)}

	mgr.BufferHandResultIfScored(902, old, next)

	assert.Empty(t, mgr.HandResults(902))
}

func TestBufferHandResultIfScored_Noop_NilHandResult(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	require.NoError(t, mgr.StartMatch(903, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true))

	old := &game.GameState{HandNumber: 2}
	next := &game.GameState{HandNumber: 3, LastHandResult: nil}

	mgr.BufferHandResultIfScored(903, old, next)

	assert.Empty(t, mgr.HandResults(903))
}

func TestSurrender_RequestSucceeds(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	require.NoError(t, mgr.StartMatch(800, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true))

	state := mgr.GetStateSnapshot(800)
	require.NotNil(t, state)
	assert.Equal(t, game.PhaseBidding, state.Phase)

	client := &ws.Client{UserID: 10}
	mgr.HandleAction(client, ws.WSMessage{
		Type:    "action:surrender_request",
		Payload: json.RawMessage(`{}`),
	})
	time.Sleep(50 * time.Millisecond)

	newState := mgr.GetStateSnapshot(800)
	require.NotNil(t, newState)
	require.NotNil(t, newState.SurrenderProposerSeat)
	assert.Equal(t, 0, *newState.SurrenderProposerSeat)
	assert.True(t, newState.SurrenderUsed[0])
	assert.Equal(t, game.PhaseBidding, newState.Phase, "phase unchanged on request")
	assert.True(t, mgr.HasSession(800), "session continues during pending proposal")
}

func TestSurrender_AcceptPersistsMatchAsCompleted(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	require.NoError(t, mgr.StartMatch(801, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true))

	mgr.HandleAction(&ws.Client{UserID: 10}, ws.WSMessage{
		Type:    "action:surrender_request",
		Payload: json.RawMessage(`{}`),
	})
	time.Sleep(50 * time.Millisecond)

	mgr.HandleAction(&ws.Client{UserID: 30}, ws.WSMessage{
		Type:    "action:surrender_accept",
		Payload: json.RawMessage(`{}`),
	})
	time.Sleep(100 * time.Millisecond)

	matches := repo.getMatches()
	require.Len(t, matches, 1)
	m := matches[0]
	assert.Equal(t, "completed", m.Status)
	assert.Equal(t, 1, m.WinnerTeam, "Team B (team 1) wins because team A surrendered")
	require.NotNil(t, m.SurrenderedBy)
	assert.Equal(t, uint(10), *m.SurrenderedBy)
	assert.Nil(t, m.AbandonedBy, "AbandonedBy stays nil for surrender end")

	assert.False(t, mgr.HasSession(801))
}

func TestSurrender_DeclineKeepsSessionActive(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	require.NoError(t, mgr.StartMatch(802, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true))

	mgr.HandleAction(&ws.Client{UserID: 10}, ws.WSMessage{
		Type:    "action:surrender_request",
		Payload: json.RawMessage(`{}`),
	})
	time.Sleep(30 * time.Millisecond)

	mgr.HandleAction(&ws.Client{UserID: 30}, ws.WSMessage{
		Type:    "action:surrender_decline",
		Payload: json.RawMessage(`{}`),
	})
	time.Sleep(30 * time.Millisecond)

	state := mgr.GetStateSnapshot(802)
	require.NotNil(t, state)
	assert.Nil(t, state.SurrenderProposerSeat)
	assert.True(t, state.SurrenderUsed[0], "proposer's attempt remains consumed on decline")
	assert.True(t, mgr.HasSession(802))

	assert.Empty(t, repo.getMatches())
}

func TestSurrender_ExhaustedAfterDecline(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	require.NoError(t, mgr.StartMatch(803, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true))

	mgr.HandleAction(&ws.Client{UserID: 10}, ws.WSMessage{
		Type:    "action:surrender_request",
		Payload: json.RawMessage(`{}`),
	})
	time.Sleep(30 * time.Millisecond)
	mgr.HandleAction(&ws.Client{UserID: 30}, ws.WSMessage{
		Type:    "action:surrender_decline",
		Payload: json.RawMessage(`{}`),
	})
	time.Sleep(30 * time.Millisecond)

	mgr.HandleAction(&ws.Client{UserID: 10}, ws.WSMessage{
		Type:    "action:surrender_request",
		Payload: json.RawMessage(`{}`),
	})
	time.Sleep(30 * time.Millisecond)

	state := mgr.GetStateSnapshot(803)
	require.NotNil(t, state)
	assert.Nil(t, state.SurrenderProposerSeat, "second request rejected; no new proposer")
	assert.True(t, state.SurrenderUsed[0])
}

func TestSurrender_SecondRequestWhilePending_Rejected(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	require.NoError(t, mgr.StartMatch(804, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true))

	mgr.HandleAction(&ws.Client{UserID: 10}, ws.WSMessage{
		Type:    "action:surrender_request",
		Payload: json.RawMessage(`{}`),
	})
	time.Sleep(30 * time.Millisecond)

	mgr.HandleAction(&ws.Client{UserID: 20}, ws.WSMessage{
		Type:    "action:surrender_request",
		Payload: json.RawMessage(`{}`),
	})
	time.Sleep(30 * time.Millisecond)

	state := mgr.GetStateSnapshot(804)
	require.NotNil(t, state)
	require.NotNil(t, state.SurrenderProposerSeat)
	assert.Equal(t, 0, *state.SurrenderProposerSeat, "first proposer unaffected")
	assert.False(t, state.SurrenderUsed[1], "rejected request must not consume seat 1's attempt")
}

func TestSurrender_NonPartnerAccept_Rejected(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	require.NoError(t, mgr.StartMatch(805, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true))

	mgr.HandleAction(&ws.Client{UserID: 10}, ws.WSMessage{
		Type:    "action:surrender_request",
		Payload: json.RawMessage(`{}`),
	})
	time.Sleep(30 * time.Millisecond)

	mgr.HandleAction(&ws.Client{UserID: 20}, ws.WSMessage{
		Type:    "action:surrender_accept",
		Payload: json.RawMessage(`{}`),
	})
	time.Sleep(50 * time.Millisecond)

	state := mgr.GetStateSnapshot(805)
	require.NotNil(t, state)
	assert.NotEqual(t, game.PhaseMatchEnd, state.Phase, "non-partner cannot accept")
	require.NotNil(t, state.SurrenderProposerSeat, "proposal still pending")
	assert.True(t, mgr.HasSession(805))
	assert.Empty(t, repo.getMatches())
}

func TestSurrender_PauseInteraction_PreservesProposal(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	require.NoError(t, mgr.StartMatch(806, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true))

	mgr.HandleAction(&ws.Client{UserID: 10}, ws.WSMessage{
		Type:    "action:surrender_request",
		Payload: json.RawMessage(`{}`),
	})
	time.Sleep(30 * time.Millisecond)

	mgr.HandleAction(&ws.Client{UserID: 20}, ws.WSMessage{
		Type:    "action:pause",
		Payload: json.RawMessage(`{}`),
	})
	time.Sleep(30 * time.Millisecond)

	paused := mgr.GetStateSnapshot(806)
	require.NotNil(t, paused)
	assert.Equal(t, game.PhasePaused, paused.Phase)
	require.NotNil(t, paused.SurrenderProposerSeat, "surrender proposal survives pause")
	assert.Equal(t, 0, *paused.SurrenderProposerSeat)

	mgr.HandleAction(&ws.Client{UserID: 20}, ws.WSMessage{
		Type:    "action:unpause",
		Payload: json.RawMessage(`{}`),
	})
	time.Sleep(30 * time.Millisecond)

	resumed := mgr.GetStateSnapshot(806)
	require.NotNil(t, resumed)
	assert.NotEqual(t, game.PhasePaused, resumed.Phase)
	require.NotNil(t, resumed.SurrenderProposerSeat, "proposal still pending after unpause")
}

func TestRemoveSession_CallsUserRemovedHooks(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	err := mgr.StartMatch(900, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true)
	require.NoError(t, err)

	received := make(chan uint, 4)
	mgr.AddUserRemovedHook(func(userID uint) {
		received <- userID
	})

	mgr.RemoveSession(900)

	var got []uint
	timeout := time.After(100 * time.Millisecond)
	for len(got) < 4 {
		select {
		case uid := <-received:
			got = append(got, uid)
		case <-timeout:
			t.Fatalf("hook received only %d of 4 expected userIDs", len(got))
		}
	}

	assert.ElementsMatch(t, []uint{10, 20, 30, 40}, got, "all 4 playerIDs must be delivered to hook")
	assert.False(t, mgr.HasSession(900), "session should be removed")
}

// firstTrickStateAt builds an injectable state at trick 1 / phase=playing with
// a fixed TurnExpiresAt and the given activeSeat. Trump is hearts so seat 0
// holds Belot (KH+QH) for the play-card-into-belot scenario.
func firstTrickStateAt(activeSeat int, expiry time.Time, awaitingDeclaration bool) *game.GameState {
	gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
	gs.RoomID = 100
	gs.ActivePlayerSeat = activeSeat
	gs.AwaitingDeclaration = awaitingDeclaration
	gs.TurnExpiresAt = &expiry
	gs.TimerDurationSec = 30
	return gs
}

func TestHandleAction_Declare_PreservesTurnExpiry(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120, 0, true, false, true))

	expiry := time.Now().Add(20 * time.Second)
	mgr.SetGameStateForTest(100, firstTrickStateAt(1, expiry, true))

	mgr.HandleAction(&ws.Client{UserID: 20}, ws.WSMessage{
		Type:    ws.ActionDeclare,
		Payload: json.RawMessage(`{}`),
	})
	time.Sleep(50 * time.Millisecond)

	newState := mgr.GetStateSnapshot(100)
	require.NotNil(t, newState)
	require.NotNil(t, newState.TurnExpiresAt, "expiry must persist; declare did not advance the turn")
	assert.True(t, newState.TurnExpiresAt.Equal(expiry),
		"declare must preserve oldState.TurnExpiresAt — got %v, want %v", newState.TurnExpiresAt, expiry)
	assert.Equal(t, 1, newState.ActivePlayerSeat, "declare must not advance the seat")
	assert.False(t, newState.AwaitingDeclaration, "declare clears AwaitingDeclaration")
}

func TestHandleAction_SkipDeclare_PreservesTurnExpiry(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120, 0, true, false, true))

	expiry := time.Now().Add(20 * time.Second)
	mgr.SetGameStateForTest(100, firstTrickStateAt(1, expiry, true))

	mgr.HandleAction(&ws.Client{UserID: 20}, ws.WSMessage{
		Type:    ws.ActionSkipDeclare,
		Payload: json.RawMessage(`{}`),
	})
	time.Sleep(50 * time.Millisecond)

	newState := mgr.GetStateSnapshot(100)
	require.NotNil(t, newState)
	require.NotNil(t, newState.TurnExpiresAt)
	assert.True(t, newState.TurnExpiresAt.Equal(expiry),
		"skip_declare must preserve expiry — got %v, want %v", newState.TurnExpiresAt, expiry)
	assert.Equal(t, 1, newState.ActivePlayerSeat)
	assert.False(t, newState.AwaitingDeclaration)
}

func TestHandleAction_PlayCardTriggeringBelot_PreservesTurnExpiry(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120, 0, true, false, true))

	expiry := time.Now().Add(20 * time.Second)
	mgr.SetGameStateForTest(100, firstTrickStateAt(0, expiry, false))

	mgr.HandleAction(&ws.Client{UserID: 10}, ws.WSMessage{
		Type:    ws.ActionPlayCard,
		Payload: json.RawMessage(`{"cardId":"KH"}`),
	})
	time.Sleep(50 * time.Millisecond)

	newState := mgr.GetStateSnapshot(100)
	require.NotNil(t, newState)
	require.NotNil(t, newState.PendingBelotSeat, "play KH on hearts trump must trigger belot prompt")
	assert.Equal(t, 0, *newState.PendingBelotSeat)
	require.NotNil(t, newState.TurnExpiresAt)
	assert.True(t, newState.TurnExpiresAt.Equal(expiry),
		"play_card-into-belot must preserve expiry — got %v, want %v", newState.TurnExpiresAt, expiry)
	assert.Equal(t, 0, newState.ActivePlayerSeat, "seat must not advance while belot prompt is open")
}

func TestHandleAction_AnnounceBelot_RefreshesTurnExpiryForNextSeat(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120, 0, true, false, true))

	oldExpiry := time.Now().Add(20 * time.Second)
	mgr.SetGameStateForTest(100, firstTrickStateAt(0, oldExpiry, false))
	mgr.HandleAction(&ws.Client{UserID: 10}, ws.WSMessage{
		Type:    ws.ActionPlayCard,
		Payload: json.RawMessage(`{"cardId":"KH"}`),
	})
	time.Sleep(50 * time.Millisecond)

	mid := mgr.GetStateSnapshot(100)
	require.NotNil(t, mid)
	require.NotNil(t, mid.PendingBelotSeat, "setup: belot prompt must be open before announcing")

	mgr.HandleAction(&ws.Client{UserID: 10}, ws.WSMessage{
		Type:    ws.ActionAnnounceBelot,
		Payload: json.RawMessage(`{}`),
	})
	time.Sleep(50 * time.Millisecond)

	newState := mgr.GetStateSnapshot(100)
	require.NotNil(t, newState)
	require.NotNil(t, newState.TurnExpiresAt)
	assert.True(t, newState.TurnExpiresAt.After(oldExpiry),
		"announce_belot advances seat — fresh expiry must be > old expiry")
	assert.NotEqual(t, 0, newState.ActivePlayerSeat, "announce_belot must advance the seat")
}

func TestHandleAction_SkipBelot_RefreshesTurnExpiryForNextSeat(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120, 0, true, false, true))

	oldExpiry := time.Now().Add(20 * time.Second)
	mgr.SetGameStateForTest(100, firstTrickStateAt(0, oldExpiry, false))
	mgr.HandleAction(&ws.Client{UserID: 10}, ws.WSMessage{
		Type:    ws.ActionPlayCard,
		Payload: json.RawMessage(`{"cardId":"KH"}`),
	})
	time.Sleep(50 * time.Millisecond)

	mid := mgr.GetStateSnapshot(100)
	require.NotNil(t, mid)
	require.NotNil(t, mid.PendingBelotSeat, "setup: belot prompt must be open before skipping")

	mgr.HandleAction(&ws.Client{UserID: 10}, ws.WSMessage{
		Type:    "action:skip_belot",
		Payload: json.RawMessage(`{}`),
	})
	time.Sleep(50 * time.Millisecond)

	newState := mgr.GetStateSnapshot(100)
	require.NotNil(t, newState)
	require.NotNil(t, newState.TurnExpiresAt)
	assert.True(t, newState.TurnExpiresAt.After(oldExpiry),
		"skip_belot advances seat — fresh expiry must be > old expiry")
	assert.NotEqual(t, 0, newState.ActivePlayerSeat, "skip_belot must advance the seat")
}

// TestHandleAction_InvalidAction_PreservesOriginalTurnExpiry checks that a rejected
// action keeps the existing TurnExpiresAt instead of minting a fresh turn timer.
func TestHandleAction_InvalidAction_PreservesOriginalTurnExpiry(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 30, 10, 120, 0, true, false, true))

	originalExpiry := time.Now().Add(20 * time.Second)
	mgr.SetGameStateForTest(100, firstTrickStateAt(1, originalExpiry, false))

	mgr.HandleAction(&ws.Client{UserID: 10}, ws.WSMessage{
		Type:    ws.ActionPlayCard,
		Payload: json.RawMessage(`{"cardId":"7H"}`),
	})
	time.Sleep(50 * time.Millisecond)

	after := mgr.GetStateSnapshot(100)
	require.NotNil(t, after)
	require.NotNil(t, after.TurnExpiresAt, "invalid action must not clear the active timer")
	assert.True(t, after.TurnExpiresAt.Equal(originalExpiry),
		"invalid action must preserve oldState.TurnExpiresAt — got %v, want %v",
		after.TurnExpiresAt, originalExpiry)
	assert.Equal(t, 1, after.ActivePlayerSeat, "invalid action must not advance the seat")
}
