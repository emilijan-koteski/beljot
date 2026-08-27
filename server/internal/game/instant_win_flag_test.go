package game_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
)

// GameState.WonByInstantWin (Story 13.1 D3) is the engine's record of WHY a match
// ended, so the match layer can state the outcome instead of inferring it — the
// Season-Points bonus has no other signal, because checkInstantWin fires right
// after a deal and leaves TeamScores and the hand results untouched.
//
// Every case drives the engine through ApplyAction only and builds its state from
// a testfixtures factory, never a raw GameState literal.

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
	// The two flags are independent records of two different end reasons.
	assert.False(t, result.StoppedAtTarget, "this match did not end on the stop rule")
}

// The flag must stay FALSE for an ordinary pick. Without this, "instant win"
// would be indistinguishable from "a trump was chosen" and every match would pay
// the spectacular bonus.
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
	// Swap the second stage-2 card for a non-heart: seat 1 ends on 7 hearts.
	gs.Deck[1] = game.Card{Rank: game.Rank7, Suit: game.SuitSpades}

	result, err := game.ApplyAction(gs, game.Action{Type: game.ActionPickTrump, PlayerSeat: 1})
	require.NoError(t, err)
	require.NotNil(t, result)

	require.Equal(t, game.PhasePlaying, result.Phase)
	assert.False(t, result.WonByInstantWin, "7 trump cards is not an instant win")
}

// THE MID-MATCH INSTANT WIN — the case GameState.WonByInstantWin exists for, and
// the one with real money on it: an instant win on hand 5 of a match sitting at
// 500:300 has TeamScores and hand results that look exactly like an ordinary
// finish, so the flag is the ONLY thing that tells the match layer to pay the
// +50 Season Points bonus (Story 13.1 D2/D3).
//
// Every other set-case in this file is hand 1 at 0:0, where an instant win is
// also inferable from "no hand results, zero scores". This one is not inferable
// at all, which is the whole point.
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
	// The scores are untouched, which is precisely why the inference available on
	// a hand-1 instant win ("TeamScores are [0,0]") does not exist here.
	assert.Equal(t, [2]int{500, 300}, result.TeamScores)
	assert.Equal(t, 5, result.HandNumber)
}

// THE OTHER CALL SITE IS STRUCTURALLY UNREACHABLE, and this test is what keeps
// that true — or fails loudly the day it stops being.
//
// scoring.go's startNewHand also calls checkInstantWin, right after dealing. That
// branch cannot fire under either shipped deal shape, for two different reasons:
//
//   - DealShapeCandidate (Bitola): the stage-1 deal leaves each seat holding 5
//     cards, so trumpCount == 8 is arithmetically impossible.
//   - DealShapeAllBeforeBidding (Croatia): the deal sets TrumpCandidate to nil and
//     leaves TrumpSuit nil, so checkInstantWin returns at its `default` arm with no
//     suit to count. (Each seat also holds only 6 cards in Hand, the other 2 being
//     face-down until bidding resolves.)
//
// So the flag can only ever be set from handlePickTrump, which is what every
// other set-case here drives. THIS TEST DOES NOT PROVE THE ASSIGNMENT IN
// startNewHand WORKS — it cannot, because that line cannot execute. It pins the
// two structural invariants that make it dead, so a future variant that deals
// eight open cards alongside a trump reference fails here and tells its author
// the branch just came alive and now needs its own coverage.
//
// startNewHand builds and shuffles its OWN deck (math/rand/v2, no seeding), so
// there is no seam to stack it through and no fixture that can force the case.
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
		// Repeated because the deal is genuinely random; the assertions below are
		// structural, so a single pass would already be conclusive.
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
			// Both nil is what sends checkInstantWin to its `default: return nil`.
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

	// Called directly, the way ApplyAction calls it.
	game.RefreshDerivedFlags(result)
	assert.True(t, result.WonByInstantWin, "...and not recomputed away at the exit")
}

// startNewHand clears it, so the flag can never outlive its hand. Driven through
// the four continue acknowledgements that deal the next hand.
func TestWonByInstantWin_ClearedByStartNewHand(t *testing.T) {
	state := reachHandComplete(t)
	// Simulate a flag left over from an earlier hand's state (unreachable in
	// practice, since an instant win ends the match — reset defensively for
	// exactly the reason StoppedAtTarget is).
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
