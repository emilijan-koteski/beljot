package game_test

import (
	"errors"
	"testing"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNoDeclarationsSkipsMeldPrompt covers the Bitola half of the skip. A seat
// holding a perfectly good meld plays its first card and is never asked.
func TestNoDeclarationsSkipsMeldPrompt(t *testing.T) {
	tests := []struct {
		name    string
		newGame func() *game.GameState
	}{
		{
			name:    "bitola",
			newGame: func() *game.GameState { return testfixtures.NewGameFirstTrick(game.SuitHearts) },
		},
		{
			name:    "croatia",
			newGame: func() *game.GameState { return testfixtures.NewGameCroatianFirstTrick(game.SuitHearts) },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gs := testfixtures.WithoutDeclarations(tc.newGame())
			require.False(t, gs.AwaitingDeclaration, "fixture must not start mid-prompt")

			after, err := game.ApplyAction(gs, game.Action{
				Type:       game.ActionPlayCard,
				PlayerSeat: 1,
				Card:       &game.Card{Rank: game.RankQueen, Suit: game.SuitSpades},
			})
			require.NoError(t, err)

			assert.False(t, after.AwaitingDeclaration,
				"no seat may be asked to declare in a room without declarations")
			assert.Equal(t, 2, after.ActivePlayerSeat,
				"turn advances normally instead of stalling on a prompt")
			assert.Empty(t, after.Players[1].Declarations, "nothing is stored")
			assert.Equal(t, [2]int{0, 0}, after.DeclarationPoints)
		})
	}
}

// TestNoDeclarationsSkipsBelotPrompt checks the Belot prompt is skipped when declarations are
// disabled.
func TestNoDeclarationsSkipsBelotPrompt(t *testing.T) {
	tests := []struct {
		name    string
		newGame func() *game.GameState
	}{
		{"bitola", func() *game.GameState { return testfixtures.NewGameFirstTrick(game.SuitHearts) }},
		{"croatia", func() *game.GameState { return testfixtures.NewGameCroatianFirstTrick(game.SuitHearts) }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gs := testfixtures.WithoutDeclarations(tc.newGame())
			gs.ActivePlayerSeat = 0

			after, err := game.ApplyAction(gs, game.Action{
				Type:       game.ActionPlayCard,
				PlayerSeat: 0,
				Card:       &game.Card{Rank: game.RankKing, Suit: game.SuitHearts},
			})
			require.NoError(t, err)

			assert.Nil(t, after.PendingBelotSeat, "no seat may be asked to announce Belote")
			assert.False(t, after.BelotAnnounced)
			assert.Equal(t, [2]int{0, 0}, after.BelotPoints, "no +20 may be awarded")
			assert.Equal(t, 1, after.ActivePlayerSeat,
				"turn advances instead of holding for a Belote answer")
		})
	}
}

// TestNoDeclarationsSkipsBelotOnQueenToo guards the other half of the K/Q pair.
// The prompt is rank-bound, not play-order-bound, so leading the Queen while
// holding the King is an independent trigger.
func TestNoDeclarationsSkipsBelotOnQueenToo(t *testing.T) {
	gs := testfixtures.WithoutDeclarations(testfixtures.NewGameFirstTrick(game.SuitHearts))
	gs.ActivePlayerSeat = 0

	after, err := game.ApplyAction(gs, game.Action{
		Type:       game.ActionPlayCard,
		PlayerSeat: 0,
		Card:       &game.Card{Rank: game.RankQueen, Suit: game.SuitHearts},
	})
	require.NoError(t, err)

	assert.Nil(t, after.PendingBelotSeat)
	assert.Equal(t, [2]int{0, 0}, after.BelotPoints)
}

// TestNoDeclarationsCroatianSkipsDeclaringPhase checks a resolved Croatian bid opens trick 1
// directly instead of PhaseDeclaring.
func TestNoDeclarationsCroatianSkipsDeclaringPhase(t *testing.T) {
	gs := testfixtures.WithoutDeclarations(testfixtures.NewGameCroatianJustDealt())
	require.Equal(t, game.DeclarationTimingDedicatedPhase, gs.Rules.DeclarationTiming,
		"the Croatian timing preset must be intact — the skip is the config, not the timing")

	suit := game.SuitHearts
	after, err := game.ApplyAction(gs, game.Action{
		Type:       game.ActionPickTrump,
		PlayerSeat: gs.ActivePlayerSeat,
		Suit:       &suit,
	})
	require.NoError(t, err)

	assert.Equal(t, game.PhasePlaying, after.Phase, "must go straight to trick 1")
	assert.NotEqual(t, game.PhaseDeclaring, after.Phase)
	assert.Equal(t, 1, after.TrickNumber)
	assert.Equal(t, (after.DealerSeat+1)%4, after.ActivePlayerSeat,
		"trick 1 opens on the seat to the dealer's right")
	assert.False(t, after.AwaitingDeclaration)
	for seat := range after.Players {
		assert.Len(t, after.Players[seat].Hand, 8,
			"seat %d still receives its merged face-down pair", seat)
	}
}

// TestNoDeclarationsCroatianStillEntersPhaseWhenEnabled checks the same fixture does open
// PhaseDeclaring when declarations are enabled.
func TestNoDeclarationsCroatianStillEntersPhaseWhenEnabled(t *testing.T) {
	gs := testfixtures.NewGameCroatianJustDealt()

	suit := game.SuitHearts
	after, err := game.ApplyAction(gs, game.Action{
		Type:       game.ActionPickTrump,
		PlayerSeat: gs.ActivePlayerSeat,
		Suit:       &suit,
	})
	require.NoError(t, err)

	assert.Equal(t, game.PhaseDeclaring, after.Phase)
}

// TestNoDeclarationsRejectsStaleDeclareActions covers a client that shipped
// before the toggle, or a hand-crafted socket frame. Nothing may mutate state.
func TestNoDeclarationsRejectsStaleDeclareActions(t *testing.T) {
	tests := []struct {
		name   string
		action game.Action
	}{
		{"declare", game.Action{Type: game.ActionDeclare, PlayerSeat: 1}},
		{"skip_declare", game.Action{Type: game.ActionSkipDeclare, PlayerSeat: 1}},
		{"announce_belot", game.Action{Type: game.ActionAnnounceBelot, PlayerSeat: 0}},
		{"skip_belot", game.Action{Type: game.ActionSkipBelot, PlayerSeat: 0}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gs := testfixtures.WithoutDeclarations(testfixtures.NewGameFirstTrick(game.SuitHearts))
			before := *gs

			after, err := game.ApplyAction(gs, tc.action)

			require.Error(t, err, "the action has no legal effect in this room")
			assert.Nil(t, after, "a rejected action returns no state")
			assert.Equal(t, before.Phase, gs.Phase, "the input state is not mutated")
			assert.Equal(t, [2]int{0, 0}, gs.DeclarationPoints)
			assert.Equal(t, [2]int{0, 0}, gs.BelotPoints)
		})
	}
}

// TestNoDeclarationsCroatianRejectsDeclareOutsideItsPhase checks Croatian declare actions fail
// with a phase error when declarations are disabled.
func TestNoDeclarationsCroatianRejectsDeclareOutsideItsPhase(t *testing.T) {
	gs := testfixtures.WithoutDeclarations(testfixtures.NewGameCroatianFirstTrick(game.SuitHearts))

	_, err := game.ApplyAction(gs, game.Action{Type: game.ActionSkipDeclare, PlayerSeat: 1})

	require.Error(t, err)
	assert.True(t, errorIsOneOf(err, apperr.ErrWrongPhase, apperr.ErrActionRequired),
		"expected a phase or action-required rejection, got %v", err)
}

// TestNoDeclarationsSurvivesTheHandBoundary checks declarations stay disabled from hand 2 onwards,
// driven entirely through ApplyAction.
func TestNoDeclarationsSurvivesTheHandBoundary(t *testing.T) {
	gs := testfixtures.WithoutDeclarations(testfixtures.NewGameLastTrick())

	complete := playFinalTrick(t, gs)
	require.Equal(t, game.PhaseHandComplete, complete.Phase,
		"the fixture's scores must not end the match, or there is no next hand to check")

	next := complete
	for seat := range 4 {
		var err error
		next, err = game.ApplyAction(next, game.Action{Type: game.ActionContinue, PlayerSeat: seat})
		require.NoError(t, err)
	}

	require.Equal(t, 2, next.HandNumber, "hand 2 must have been dealt")
	assert.False(t, next.Rules.DeclarationsEnabled, "the config is immutable across hands")
	assert.True(t, next.DeclarationsResolved,
		"hand 2 must start with the contest already settled, exactly as hand 1 did")
	assert.False(t, next.AwaitingDeclaration)
	assert.Equal(t, [2]int{0, 0}, next.DeclarationPoints)
	assert.Equal(t, [2]int{0, 0}, next.BelotPoints)
	assert.False(t, next.DeclarationsEnabled, "the wire flag survives the boundary too")
}

// TestDeclarationsOnStillReopensEachHand is the negative control for the test
// above: with declarations ON the same boundary must leave the contest OPEN, or
// the seed has been applied unconditionally and every room lost its melds.
func TestDeclarationsOnStillReopensEachHand(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()

	complete := playFinalTrick(t, gs)
	require.Equal(t, game.PhaseHandComplete, complete.Phase)

	next := complete
	for seat := range 4 {
		var err error
		next, err = game.ApplyAction(next, game.Action{Type: game.ActionContinue, PlayerSeat: seat})
		require.NoError(t, err)
	}

	require.Equal(t, 2, next.HandNumber)
	assert.False(t, next.DeclarationsResolved, "hand 2 must reopen the declaration contest")
}

// playFinalTrick plays out trick 8 one card per seat, driving the hand into scoring.
func playFinalTrick(t *testing.T, gs *game.GameState) *game.GameState {
	t.Helper()
	state := gs
	for range 4 {
		seat := state.ActivePlayerSeat
		require.Len(t, state.Players[seat].Hand, 1, "seat %d holds one card at trick 8", seat)
		card := state.Players[seat].Hand[0]
		next, err := game.ApplyAction(state, game.Action{
			Type: game.ActionPlayCard, PlayerSeat: seat, Card: &card,
		})
		require.NoError(t, err, "play_card for seat %d", seat)
		state = next
	}
	return state
}

// TestNoDeclarationsCarriesOntoTheWire pins the one rule-config field the client
// is told about. It is derived in RefreshDerivedFlags, so it must survive every
// action rather than only the initial snapshot.
func TestNoDeclarationsCarriesOntoTheWire(t *testing.T) {
	t.Run("off", func(t *testing.T) {
		gs := game.NewGame([4]uint{10, 20, 30, 40}, [4]string{"a", "b", "c", "d"}, [4]bool{},
			game.VariantBitola, "1001", 1, false, false, true, false)
		assert.False(t, gs.DeclarationsEnabled)
		assert.Equal(t, gs.Rules.DeclarationsEnabled, gs.DeclarationsEnabled)

		for seat := range 4 {
			assert.False(t, game.ProjectForSeat(gs, seat).DeclarationsEnabled,
				"seat %d must be told the room's setting", seat)
		}
	})

	t.Run("on", func(t *testing.T) {
		gs := game.NewGame([4]uint{10, 20, 30, 40}, [4]string{"a", "b", "c", "d"}, [4]bool{},
			game.VariantBitola, "1001", 1, true, false, true, false)
		assert.True(t, gs.DeclarationsEnabled)
		assert.False(t, gs.DeclarationsResolved, "an ordinary hand starts with the contest open")
	})

	t.Run("refreshed after an action, never left stale", func(t *testing.T) {
		gs := testfixtures.WithoutDeclarations(testfixtures.NewGameFirstTrick(game.SuitHearts))
		gs.DeclarationsEnabled = true

		after, err := game.ApplyAction(gs, game.Action{
			Type:       game.ActionPlayCard,
			PlayerSeat: 1,
			Card:       &game.Card{Rank: game.RankQueen, Suit: game.SuitSpades},
		})
		require.NoError(t, err)

		assert.False(t, after.DeclarationsEnabled,
			"the wire flag is recomputed from Rules, so it cannot drift")
	})
}

// TestNoDeclarationsKeepsHandsPrivate checks Declarations stay empty in a declarations-off room,
// since projection never masks them there.
func TestNoDeclarationsKeepsHandsPrivate(t *testing.T) {
	gs := testfixtures.WithoutDeclarations(testfixtures.NewGameFirstTrick(game.SuitHearts))
	require.True(t, gs.DeclarationsResolved, "the seed is the precondition this test exists for")

	for viewer := range 4 {
		projected := game.ProjectForSeat(gs, viewer)

		require.Len(t, projected.Players[viewer].Hand, 8, "seat %d sees its own hand", viewer)
		for seat := range 4 {
			assert.Empty(t, projected.Players[seat].Declarations,
				"seat %d exposes no declarations to viewer %d — there are none to expose", seat, viewer)
			if seat == viewer {
				continue
			}
			assert.Empty(t, projected.Players[seat].Hand,
				"viewer %d must not receive seat %d's cards", viewer, seat)
			assert.Equal(t, 8, projected.Players[seat].HandCount,
				"but must still see how many seat %d holds", seat)
		}
		assert.Nil(t, projected.Deck, "the undealt deck never rides the wire (Story 12.10)")
	}
}

// TestNoDeclarationsScoresOnCardPointsAlone is the outcome the whole feature is
// for: the hand still scores and still pays the last-trick bonus, with nothing
// added from melds or Belote.
func TestNoDeclarationsScoresOnCardPointsAlone(t *testing.T) {
	off := playFinalTrick(t, testfixtures.WithoutDeclarations(testfixtures.NewGameLastTrick()))
	on := playFinalTrick(t, testfixtures.NewGameLastTrick())

	res := off.LastHandResult
	require.NotNil(t, res, "a completed hand must publish its result")

	assert.Equal(t, 0, res.TeamADeclPoints)
	assert.Equal(t, 0, res.TeamBDeclPoints)
	assert.Equal(t, 10, res.LastTrickBonus, "the last-trick bonus is untouched by the toggle")
	assert.Equal(t, [2]int{0, 0}, off.BelotPoints)

	assert.Equal(t, on.TeamScores, off.TeamScores,
		"card points, last trick and Capot are unaffected on a hand nobody declared in")
}

// TestNoDeclarationsWithRealMeldsInHandScoresZeroForThem checks real melds score zero when
// declarations are disabled, while the same hand with them enabled scores the melds.
func TestNoDeclarationsWithRealMeldsInHandScoresZeroForThem(t *testing.T) {
	off := testfixtures.WithoutDeclarations(testfixtures.NewGameFirstTrick(game.SuitHearts))
	require.True(t, meldsExistSomewhere(off), "the fixture must actually hold melds, or this proves nothing")

	offFinal := playWholeHand(t, off)
	require.NotNil(t, offFinal.LastHandResult)

	assert.Equal(t, 0, offFinal.LastHandResult.TeamADeclPoints,
		"a quarte and two tierces on the table must still score nothing")
	assert.Equal(t, 0, offFinal.LastHandResult.TeamBDeclPoints)
	assert.Equal(t, [2]int{0, 0}, offFinal.DeclarationPoints)
	assert.Equal(t, [2]int{0, 0}, offFinal.BelotPoints,
		"seat 0 held K+Q of trump all hand and was never asked")
	assert.Equal(t, 152, offFinal.LastHandResult.TeamACardPoints+offFinal.LastHandResult.TeamBCardPoints,
		"the raw card-point total is untouched by the toggle")
	assert.Equal(t, 162,
		offFinal.LastHandResult.TeamACardPoints+offFinal.LastHandResult.TeamBCardPoints+
			offFinal.LastHandResult.LastTrickBonus+offFinal.LastHandResult.CapotBonus,
		"and card points plus the last-trick or Capot bonus still make the full 162")

	onFinal := playWholeHand(t, testfixtures.NewGameFirstTrick(game.SuitHearts))
	require.NotNil(t, onFinal.LastHandResult)
	onDecl := onFinal.LastHandResult.TeamADeclPoints + onFinal.LastHandResult.TeamBDeclPoints
	assert.Positive(t, onDecl,
		"with declarations ON the same hand must award meld points, or the OFF assertions prove nothing")
}

// meldsExistSomewhere reports whether any seat holds a declarable combination,
// under the state's own overlap rule.
func meldsExistSomewhere(state *game.GameState) bool {
	for seat := range 4 {
		if game.HasDeclarableCombinations(state, seat) {
			return true
		}
	}
	return false
}

// playWholeHand plays each seat's first legal card and answers any prompt until the hand leaves
// the playing phase.
func playWholeHand(t *testing.T, gs *game.GameState) *game.GameState {
	t.Helper()
	state := gs
	for step := 0; step < 200; step++ {
		if state.Phase != game.PhasePlaying && state.Phase != game.PhaseDeclaring {
			return state
		}

		seat := state.ActivePlayerSeat

		switch {
		case state.PendingBelotSeat != nil:
			next, err := game.ApplyAction(state, game.Action{
				Type: game.ActionAnnounceBelot, PlayerSeat: *state.PendingBelotSeat,
			})
			require.NoError(t, err)
			state = next
			continue
		case state.AwaitingDeclaration:
			next, err := game.ApplyAction(state, game.Action{
				Type: game.ActionDeclare, PlayerSeat: seat,
			})
			require.NoError(t, err)
			state = next
			continue
		case state.Phase == game.PhaseDeclaring:
			next, err := game.ApplyAction(state, game.Action{
				Type: game.ActionSkipDeclare, PlayerSeat: seat,
			})
			require.NoError(t, err)
			state = next
			continue
		}

		played := false
		for _, card := range state.Players[seat].Hand {
			next, err := game.ApplyAction(state, game.Action{
				Type: game.ActionPlayCard, PlayerSeat: seat, Card: &card,
			})
			if err != nil {
				continue
			}
			state = next
			played = true
			break
		}
		require.True(t, played, "seat %d had no legal card at trick %d", seat, state.TrickNumber)
	}
	t.Fatal("the hand did not finish within 200 steps")
	return nil
}

func errorIsOneOf(err error, targets ...error) bool {
	for _, target := range targets {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}
