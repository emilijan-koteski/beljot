package game_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
)

// reachHandComplete plays out trick 8 of NewGameLastTrick so the hand is scored
// and the state holds in PhaseHandComplete (match continues — 70:92).
func reachHandComplete(t *testing.T) *game.GameState {
	t.Helper()
	hc := playTrick8(t, testfixtures.NewGameLastTrick())
	require.Equal(t, game.PhaseHandComplete, hc.Phase, "hand should hold complete, not deal the next hand")
	require.Equal(t, 1, hc.HandNumber, "next hand must not be dealt yet")
	require.NotNil(t, hc.LastHandResult)
	require.NotNil(t, hc.TrickWinnerSeat, "last trick winner preserved (startNewHand not run yet)")
	return hc
}

func TestHandComplete_HoldsBeforeDealingNextHand(t *testing.T) {
	hc := reachHandComplete(t)
	for seat := 0; seat < 4; seat++ {
		assert.Empty(t, hc.Players[seat].Hand, "no next-hand cards dealt during the pause")
	}
	assert.Equal(t, [4]bool{}, hc.HandCompleteReady)
	// The just-resolved last trick must be cleared from the hand-complete state.
	// resolveTrick skips its next-trick reset on the 8th trick, so scoreHand owns
	// this — without it the authoritative match_state re-feeds the client the four
	// last-trick cards, flashing them back at table center after the collect sweep.
	assert.Empty(t, hc.CurrentTrick, "resolved last trick cleared from the hand-complete state")
	assert.Nil(t, hc.LeadSuit, "lead suit cleared with the resolved trick")
}

func TestContinue_AllReadyDealsNextHand(t *testing.T) {
	state := reachHandComplete(t)
	for seat := 0; seat < 4; seat++ {
		ns, err := game.ApplyAction(state, game.Action{Type: game.ActionContinue, PlayerSeat: seat})
		require.NoError(t, err)
		state = ns
		if seat < 3 {
			assert.Equal(t, game.PhaseHandComplete, state.Phase, "still holding until every seat is ready")
			assert.True(t, state.HandCompleteReady[seat], "seat %d marked ready", seat)
		}
	}
	// All four acknowledged → next hand dealt. The engine sets PhaseDealing; the
	// session manager promotes dealing→bidding.
	assert.Equal(t, game.PhaseDealing, state.Phase)
	assert.Equal(t, 2, state.HandNumber)
	assert.Equal(t, 0, state.TrickNumber)
}

func TestContinue_DisconnectedSeatDoesNotBlock(t *testing.T) {
	state := reachHandComplete(t)
	state.Players[2].Connected = false // seat 2 dropped
	for _, seat := range []int{0, 1, 3} {
		ns, err := game.ApplyAction(state, game.Action{Type: game.ActionContinue, PlayerSeat: seat})
		require.NoError(t, err)
		state = ns
	}
	assert.Equal(t, game.PhaseDealing, state.Phase, "disconnected seat is excluded from the ready gate")
	assert.Equal(t, 2, state.HandNumber)
}

func TestContinue_IsIdempotentPerSeat(t *testing.T) {
	state := reachHandComplete(t)
	// Seat 0 continues twice; still waiting on the others.
	for i := 0; i < 2; i++ {
		ns, err := game.ApplyAction(state, game.Action{Type: game.ActionContinue, PlayerSeat: 0})
		require.NoError(t, err)
		state = ns
	}
	assert.Equal(t, game.PhaseHandComplete, state.Phase)
	assert.True(t, state.HandCompleteReady[0])
}

func TestForceAdvanceHandComplete_DealsRegardlessOfReady(t *testing.T) {
	hc := reachHandComplete(t)
	ns, err := game.ForceAdvanceHandComplete(hc)
	require.NoError(t, err)
	assert.Equal(t, game.PhaseDealing, ns.Phase)
	assert.Equal(t, 2, ns.HandNumber)
}

func TestForceAdvanceHandComplete_WrongPhase(t *testing.T) {
	_, err := game.ForceAdvanceHandComplete(testfixtures.NewGameMidPlay(2))
	require.ErrorIs(t, err, apperr.ErrWrongPhase)
}

func TestContinue_WrongPhaseRejected(t *testing.T) {
	_, err := game.ApplyAction(testfixtures.NewGameMidPlay(2), game.Action{
		Type:       game.ActionContinue,
		PlayerSeat: 0,
	})
	require.ErrorIs(t, err, apperr.ErrWrongPhase)
}
