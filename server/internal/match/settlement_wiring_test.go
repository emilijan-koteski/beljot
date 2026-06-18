package match_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/ws"
)

// stubSettler records the credits ApplySettlement received and returns canned
// post-settlement balances for GetBalances. Satisfies match.WalletSettler.
type stubSettler struct {
	applyCalls   int
	applyCredits map[uint]int
	balances     map[uint]int
}

func (s *stubSettler) ApplySettlement(credits map[uint]int) error {
	s.applyCalls++
	s.applyCredits = credits
	return nil
}

func (s *stubSettler) GetBalances(_ []uint) (map[uint]int, error) {
	return s.balances, nil
}

func indexOfTypeAfter(calls []hubCall, eventType string, after int) int {
	for i := after + 1; i < len(calls); i++ {
		if containsType(calls[i].msg, eventType) {
			return i
		}
	}
	return -1
}

// TestHandleMatchEnd_SettlesCoins locks in Story 9.2: a staked match credits the
// winning human seats and emits event:coin_settlement per human, ordered AFTER
// event:match_end and BEFORE the trailing event:match_state.
func TestHandleMatchEnd_SettlesCoins(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	settler := &stubSettler{balances: map[uint]int{10: 5500, 20: 4500, 30: 5500, 40: 4500}}
	mgr := match.NewManager(hub, repo)
	mgr.SetWalletSettler(settler)

	roomID := uint(100)
	// coinBuyIn 500 → pot 2000; team A (seats 0,2 → users 10,30) wins.
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 500))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	finalState := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, finalState)
	winner := game.TeamA
	finalState.WinnerTeam = &winner
	finalState.Phase = game.PhaseMatchEnd

	payload := ws.MatchEndPayload{WinnerTeam: game.TeamA, TeamAFinalScore: 1010, TeamBFinalScore: 700, MatchDurationSec: 600}
	mgr.HandleMatchEndForTest(roomID, finalState, nil, payload)

	// Winners (team A: users 10, 30) each credited the full half-pot (1000).
	require.Equal(t, 1, settler.applyCalls)
	assert.Equal(t, map[uint]int{10: 1000, 30: 1000}, settler.applyCredits)

	calls := hub.snapshot()
	matchEndIdx := -1
	for i, c := range calls {
		if containsType(c.msg, "event:match_end") {
			matchEndIdx = i
			break
		}
	}
	require.GreaterOrEqual(t, matchEndIdx, 0, "event:match_end must fire")

	trailingStateIdx := indexOfTypeAfter(calls, "event:match_state", matchEndIdx)
	require.GreaterOrEqual(t, trailingStateIdx, 0, "trailing event:match_state must follow match_end")

	settlementCount := 0
	for i := matchEndIdx + 1; i < trailingStateIdx; i++ {
		if containsType(calls[i].msg, "event:coin_settlement") {
			settlementCount++
		}
	}
	assert.Equal(t, 4, settlementCount, "all four humans receive a coin_settlement between match_end and match_state")
}

// TestHandleMatchEnd_NoEconomyNoSettlement verifies a free match (coinBuyIn 0)
// performs no settlement and emits no coin_settlement events.
func TestHandleMatchEnd_NoEconomyNoSettlement(t *testing.T) {
	repo := &timestampedRepo{}
	hub := &hubSpy{}
	settler := &stubSettler{balances: map[uint]int{}}
	mgr := match.NewManager(hub, repo)
	mgr.SetWalletSettler(settler)

	roomID := uint(101)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	finalState := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, finalState)
	winner := game.TeamA
	finalState.WinnerTeam = &winner
	finalState.Phase = game.PhaseMatchEnd

	mgr.HandleMatchEndForTest(roomID, finalState, nil, ws.MatchEndPayload{WinnerTeam: game.TeamA})

	assert.Equal(t, 0, settler.applyCalls, "no settlement on a free match")
	for _, c := range hub.snapshot() {
		assert.False(t, containsType(c.msg, "event:coin_settlement"), "no coin_settlement on a free match")
	}
}
