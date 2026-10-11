package game_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
)

// instantWinOnPick returns the stage-1 state from TestInstantWin_TriggeredOnPick:
// seat 1 holds 5 hearts, the candidate is 7H, and the deck's stage-2 slice for
// seat 1 is KH AH — so picking trump leaves seat 1 holding all eight hearts.
func instantWinOnPick(t *testing.T) *game.GameState {
	t.Helper()
	gs := testfixtures.NewGameJustDealt()
	candidate := game.Card{Rank: game.Rank7, Suit: game.SuitHearts}
	gs.TrumpCandidate = &candidate
	gs.Players[1].Hand = []game.Card{
		{Rank: game.Rank8, Suit: game.SuitHearts},
		{Rank: game.Rank9, Suit: game.SuitHearts},
		{Rank: game.RankTen, Suit: game.SuitHearts},
		{Rank: game.RankJack, Suit: game.SuitHearts},
		{Rank: game.RankQueen, Suit: game.SuitHearts},
	}
	gs.Deck = []game.Card{
		{Rank: game.RankKing, Suit: game.SuitHearts},
		{Rank: game.RankAce, Suit: game.SuitHearts},
		{Rank: game.RankQueen, Suit: game.SuitSpades},
		{Rank: game.RankKing, Suit: game.SuitSpades},
		{Rank: game.RankAce, Suit: game.SuitSpades},
		{Rank: game.RankQueen, Suit: game.SuitDiamonds},
		{Rank: game.RankKing, Suit: game.SuitDiamonds},
		{Rank: game.RankAce, Suit: game.SuitDiamonds},
		{Rank: game.RankQueen, Suit: game.SuitClubs},
		{Rank: game.RankKing, Suit: game.SuitClubs},
		{Rank: game.RankAce, Suit: game.SuitClubs},
	}
	require.False(t, gs.WonByInstantWin, "the fixture must start with the flag clear")
	return gs
}

func TestWonByInstantWin_SetOnPickTrump(t *testing.T) {
	result, err := game.ApplyAction(instantWinOnPick(t), game.Action{Type: game.ActionPickTrump, PlayerSeat: 1})
	require.NoError(t, err)
	require.NotNil(t, result)

	require.Equal(t, game.PhaseMatchEnd, result.Phase)
	require.NotNil(t, result.WinnerTeam)
	assert.True(t, result.WonByInstantWin,
		"the pick that dealt one seat all 8 trumps must record the instant win")
	assert.False(t, result.StoppedAtTarget, "this match did not end on the stop rule")
}

// The flag must stay FALSE for an ordinary pick. Without this, "instant win"
// would be indistinguishable from "a trump was chosen" and every match would be
// scored at the maximum Season Points margin an instant win gets.
func TestWonByInstantWin_NotSetOnAnOrdinaryPick(t *testing.T) {
	gs := testfixtures.NewGameJustDealt()

	result, err := game.ApplyAction(gs, game.Action{Type: game.ActionPickTrump, PlayerSeat: 1})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.NotEqual(t, game.PhaseMatchEnd, result.Phase)
	assert.False(t, result.WonByInstantWin)
}

// A partial trump holding (7 of 8) is not an instant win, so the flag must not be
// set by the near miss either.
func TestWonByInstantWin_NotSetOnPartialTrump(t *testing.T) {
	gs := instantWinOnPick(t)
	gs.Deck[1] = game.Card{Rank: game.Rank7, Suit: game.SuitSpades}

	result, err := game.ApplyAction(gs, game.Action{Type: game.ActionPickTrump, PlayerSeat: 1})
	require.NoError(t, err)
	require.NotNil(t, result)

	require.Equal(t, game.PhasePlaying, result.Phase)
	assert.False(t, result.WonByInstantWin, "7 trump cards is not an instant win")
}

// TestWonByInstantWin_SetMidMatchWithPointsOnTheBoard checks the flag is set for a mid-match
// instant win, where scores alone cannot reveal it.
func TestWonByInstantWin_SetMidMatchWithPointsOnTheBoard(t *testing.T) {
	gs := instantWinOnPick(t)
	gs.HandNumber = 5
	gs.TeamScores = [2]int{500, 300}

	result, err := game.ApplyAction(gs, game.Action{Type: game.ActionPickTrump, PlayerSeat: 1})
	require.NoError(t, err)
	require.NotNil(t, result)

	require.Equal(t, game.PhaseMatchEnd, result.Phase)
	require.NotNil(t, result.WinnerTeam)
	assert.True(t, result.WonByInstantWin,
		"an instant win on hand 5 must be recorded — nothing else at the match layer can tell")
	assert.Equal(t, [2]int{500, 300}, result.TeamScores)
	assert.Equal(t, 5, result.HandNumber)
}

// TestWonByInstantWin_StartNewHandSiteCannotFireUnderEitherDealShape pins the deal invariants that
// keep startNewHand's instant-win check unreachable under both deal shapes.
func TestWonByInstantWin_StartNewHandSiteCannotFireUnderEitherDealShape(t *testing.T) {
	deal := func(t *testing.T, start *game.GameState) *game.GameState {
		t.Helper()
		state := start
		state.Phase = game.PhaseHandComplete
		state.HandCompleteReady = [4]bool{}
		for seat := 0; seat < 4; seat++ {
			ns, err := game.ApplyAction(state, game.Action{Type: game.ActionContinue, PlayerSeat: seat})
			require.NoError(t, err)
			state = ns
		}
		require.Equal(t, game.PhaseDealing, state.Phase, "the four acknowledgements must deal a fresh hand")
		return state
	}

	t.Run("candidate shape cannot reach eight trumps in a hand", func(t *testing.T) {
		for i := 0; i < 50; i++ {
			dealt := deal(t, testfixtures.NewGameJustDealt())
			require.NotNil(t, dealt.TrumpCandidate, "this shape flips a candidate")
			require.Nil(t, dealt.TrumpSuit, "no trump is locked before bidding")
			for seat := 0; seat < 4; seat++ {
				assert.Len(t, dealt.Players[seat].Hand, 5,
					"five cards cannot contain all eight of a suit")
			}
			assert.NotEqual(t, game.PhaseMatchEnd, dealt.Phase)
			assert.False(t, dealt.WonByInstantWin)
		}
	})

	t.Run("all-before-bidding shape has no trump reference to count", func(t *testing.T) {
		for i := 0; i < 50; i++ {
			dealt := deal(t, testfixtures.NewGameCroatianJustDealt())
			assert.Nil(t, dealt.TrumpCandidate)
			assert.Nil(t, dealt.TrumpSuit)
			assert.NotEqual(t, game.PhaseMatchEnd, dealt.Phase)
			assert.False(t, dealt.WonByInstantWin)
		}
	})
}

// The flag is an EVENT RECORD, not a config mirror, so RefreshDerivedFlags must
// leave it alone. ApplyAction runs RefreshDerivedFlags at its single exit, and a
// recompute there would wipe the record the very action that set it.
func TestWonByInstantWin_SurvivesApplyActionsDerivedFlagRefresh(t *testing.T) {
	result, err := game.ApplyAction(instantWinOnPick(t), game.Action{Type: game.ActionPickTrump, PlayerSeat: 1})
	require.NoError(t, err)
	require.True(t, result.WonByInstantWin, "set by the handler...")

	game.RefreshDerivedFlags(result)
	assert.True(t, result.WonByInstantWin, "...and not recomputed away at the exit")
}

// startNewHand clears it, so the flag can never outlive its hand. Driven through
// the four continue acknowledgements that deal the next hand.
func TestWonByInstantWin_ClearedByStartNewHand(t *testing.T) {
	state := reachHandComplete(t)
	state.WonByInstantWin = true

	for seat := 0; seat < 4; seat++ {
		ns, err := game.ApplyAction(state, game.Action{Type: game.ActionContinue, PlayerSeat: seat})
		require.NoError(t, err)
		state = ns
	}

	require.Equal(t, game.PhaseDealing, state.Phase, "all four acknowledged, so the next hand is dealt")
	require.Equal(t, 2, state.HandNumber)
	assert.False(t, state.WonByInstantWin, "the flag belongs to the hand that just ended")
}

// ForceAdvanceHandComplete goes through the same startNewHand, so the auto-
// continue timeout must clear it too.
func TestWonByInstantWin_ClearedByForceAdvance(t *testing.T) {
	state := reachHandComplete(t)
	state.WonByInstantWin = true

	ns, err := game.ForceAdvanceHandComplete(state)
	require.NoError(t, err)
	assert.False(t, ns.WonByInstantWin)
}

// Server-only: the flag must not appear on the wire, so the match_state contract
// and its golden stay untouched (the same json:"-" rule StoppedAtTarget follows).
func TestWonByInstantWin_IsNotSerialised(t *testing.T) {
	result, err := game.ApplyAction(instantWinOnPick(t), game.Action{Type: game.ActionPickTrump, PlayerSeat: 1})
	require.NoError(t, err)
	require.True(t, result.WonByInstantWin)

	encoded, err := json.Marshal(game.ProjectForSeat(result, 0))
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "wonByInstantWin")
	assert.NotContains(t, string(encoded), "WonByInstantWin")
}
