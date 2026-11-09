package game_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
)

// windowScript is a legal script for NewGameBitolaWindow's first three tricks that spends only side cards, so every
// seat still holds its quarte. Seats play 1-2-3-0 in trick 1 and 3-0-1-2 in tricks 2 and 3.
func windowScript() [3][4]seatPlay {
	return [3][4]seatPlay{
		{{1, "9D"}, {2, "JD"}, {3, "KD"}, {0, "7D"}},
		{{3, "AD"}, {0, "8D"}, {1, "TD"}, {2, "QD"}},
		{{3, "KH"}, {0, "7H"}, {1, "TH"}, {2, "JH"}},
	}
}

// windowTurn is one seat's turn start during card play, as playHandPassingPrompts saw it.
type windowTurn struct {
	trick     int
	seat      int
	prompted  bool
	holdsMeld bool
}

func apply(t *testing.T, gs *game.GameState, action game.Action) *game.GameState {
	t.Helper()
	next, err := game.ApplyAction(gs, action)
	require.NoError(t, err, "seat %d %s", action.PlayerSeat, action.Type)
	return next
}

func playCard(t *testing.T, gs *game.GameState, seat int, id string) *game.GameState {
	t.Helper()
	card, err := game.ParseCard(id)
	require.NoError(t, err)
	return apply(t, gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: seat, Card: &card})
}

func passAndPlay(t *testing.T, gs *game.GameState, seat int, id string) *game.GameState {
	t.Helper()
	require.True(t, gs.AwaitingDeclaration, "seat %d holds a meld and is asked", seat)
	return playCard(t, apply(t, gs, game.Action{Type: game.ActionSkipDeclare, PlayerSeat: seat}), seat, id)
}

// playHandPassingPrompts plays the hand out with each seat's first legal card, passing every declaration prompt and
// skipping Belote, and records every turn start.
func playHandPassingPrompts(t *testing.T, gs *game.GameState) (*game.GameState, []windowTurn) {
	t.Helper()
	var turns []windowTurn
	state := gs
	for step := 0; step < 100 && state.Phase == game.PhasePlaying; step++ {
		if state.PendingBelotSeat != nil {
			state = apply(t, state, game.Action{Type: game.ActionSkipBelot, PlayerSeat: *state.PendingBelotSeat})
			continue
		}
		seat := state.ActivePlayerSeat
		turns = append(turns, windowTurn{
			trick:     state.TrickNumber,
			seat:      seat,
			prompted:  state.AwaitingDeclaration,
			holdsMeld: game.HasDeclarableCombinations(state, seat),
		})
		if state.AwaitingDeclaration {
			state = apply(t, state, game.Action{Type: game.ActionSkipDeclare, PlayerSeat: seat})
		}
		legal := game.LegalCards(state, seat)
		require.NotEmpty(t, legal)
		state = playCard(t, state, seat, legal[0].String())
	}
	return state, turns
}

// TestDeclarationWindowPassesThroughToTrickN checks a hand nobody declares in: only meld holders are asked, only in
// tricks 1..N, nothing is revealed, and a declaration after trick N is refused.
func TestDeclarationWindowPassesThroughToTrickN(t *testing.T) {
	for lastTrick := game.MinDeclarationLastTrick; lastTrick <= game.MaxDeclarationLastTrick; lastTrick++ {
		t.Run(fmt.Sprintf("N=%d", lastTrick), func(t *testing.T) {
			gs := testfixtures.WithDeclarationLastTrick(testfixtures.NewGameFirstTrick(game.SuitHearts), lastTrick)
			gs.AwaitingDeclaration = game.HasDeclarableCombinations(gs, gs.ActivePlayerSeat)

			final, turns := playHandPassingPrompts(t, gs)

			promptedTricks := map[int]bool{}
			meldlessSeatSeen := false
			for _, turn := range turns {
				wantPrompt := turn.trick <= lastTrick && turn.holdsMeld
				assert.Equal(t, wantPrompt, turn.prompted,
					"trick %d seat %d (holds a meld: %v)", turn.trick, turn.seat, turn.holdsMeld)
				if turn.prompted {
					promptedTricks[turn.trick] = true
				}
				if turn.trick <= lastTrick && !turn.holdsMeld {
					meldlessSeatSeen = true
				}
			}
			assert.True(t, promptedTricks[lastTrick], "a meld must survive into trick N, or this proves little")
			assert.True(t, meldlessSeatSeen, "a meldless seat must take a turn inside the window, or this proves little")
			assert.False(t, final.DeclarationsResolved, "nobody declared, so there is no reveal")
			assert.Equal(t, [2]int{0, 0}, final.DeclarationPoints)
			require.NotNil(t, final.LastHandResult)
			assert.Equal(t, 0, final.LastHandResult.TeamADeclPoints+final.LastHandResult.TeamBDeclPoints)
		})
	}

	t.Run("declare after trick N is the wrong phase", func(t *testing.T) {
		gs := testfixtures.WithDeclarationLastTrick(testfixtures.NewGameBitolaWindow(), 1)
		gs.AwaitingDeclaration = true
		for _, p := range windowScript()[0] {
			gs = passAndPlay(t, gs, p.seat, p.card)
		}
		require.Equal(t, 2, gs.TrickNumber)
		require.False(t, gs.AwaitingDeclaration, "the trick-2 leader is not asked after a one-trick window")
		require.True(t, game.HasDeclarableCombinations(gs, gs.ActivePlayerSeat), "though it still holds its quarte")

		_, err := game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: gs.ActivePlayerSeat})
		assert.ErrorIs(t, err, apperr.ErrWrongPhase)
		_, err = game.ApplyAction(gs, game.Action{Type: game.ActionSkipDeclare, PlayerSeat: gs.ActivePlayerSeat})
		assert.ErrorIs(t, err, apperr.ErrWrongPhase)

		gs.AwaitingDeclaration = true
		_, err = game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: gs.ActivePlayerSeat})
		assert.ErrorIs(t, err, apperr.ErrWrongPhase, "even a stray prompt flag cannot reopen the window")
	})
}

// driveToWindowTurn plays windowScript up to the given trick and position, passing every earlier prompt.
func driveToWindowTurn(t *testing.T, gs *game.GameState, trick, position int) *game.GameState {
	t.Helper()
	gs.AwaitingDeclaration = true
	script := windowScript()
	for k := 0; k < trick; k++ {
		for p, play := range script[k] {
			require.Equal(t, play.seat, gs.ActivePlayerSeat, "trick %d position %d", k+1, p+1)
			if k+1 == trick && p == position {
				require.True(t, gs.AwaitingDeclaration, "every seat holds a quarte, so seat %d is asked", play.seat)
				return gs
			}
			gs = passAndPlay(t, gs, play.seat, play.card)
		}
	}
	t.Fatalf("trick %d position %d is outside the script", trick, position)
	return nil
}

// TestDeclarationCycleFromEveryTurn checks a declaration in trick 1, 2 or 3 by the 1st to 4th player opens the cycle
// mid-trick with the declarer answered, then resolves and hands the same trick back to the declarer for good.
func TestDeclarationCycleFromEveryTurn(t *testing.T) {
	for trick := 1; trick <= 3; trick++ {
		for position := 0; position < 4; position++ {
			declarer := windowScript()[trick-1][position].seat
			t.Run(fmt.Sprintf("trick %d, player %d (seat %d)", trick, position+1, declarer), func(t *testing.T) {
				before := driveToWindowTurn(t, testfixtures.NewGameBitolaWindow(), trick, position)

				cycle := apply(t, before, game.Action{Type: game.ActionDeclare, PlayerSeat: declarer})

				assert.Equal(t, game.PhaseDeclaring, cycle.Phase)
				assert.Equal(t, trick, cycle.TrickNumber, "the trick is kept")
				assert.Equal(t, before.CurrentTrick, cycle.CurrentTrick, "with its cards on the table")
				assert.Equal(t, declarer, cycle.ActivePlayerSeat, "the declarer is the resume seat")
				assert.False(t, cycle.AwaitingDeclaration)
				for seat := range cycle.Players {
					assert.Equal(t, seat == declarer, cycle.Players[seat].DeclarationAnswered, "seat %d answered", seat)
				}
				require.Len(t, cycle.Players[declarer].Declarations, 1)
				assert.Equal(t, 50, cycle.Players[declarer].Declarations[0].Value)

				for _, action := range []string{game.ActionDeclare, game.ActionSkipDeclare} {
					_, err := game.ApplyAction(cycle, game.Action{Type: action, PlayerSeat: declarer})
					assert.ErrorIs(t, err, apperr.ErrWrongPhase, "the declarer is never asked twice (%s)", action)
				}

				resumed := skipRestOfCycle(t, cycle)

				assert.Equal(t, game.PhasePlaying, resumed.Phase)
				assert.True(t, resumed.DeclarationsResolved)
				assert.Equal(t, trick, resumed.TrickNumber)
				assert.Equal(t, before.CurrentTrick, resumed.CurrentTrick)
				assert.Equal(t, declarer, resumed.ActivePlayerSeat, "play resumes at the declarer's card")
				assert.False(t, resumed.AwaitingDeclaration, "with no new prompt")
				assert.Equal(t, 50, resumed.DeclarationPoints[game.TeamForSeat(declarer)])
				for seat := range resumed.Players {
					assert.False(t, resumed.Players[seat].DeclarationAnswered, "answers are cleared for seat %d", seat)
				}

				_, err := game.ApplyAction(resumed, game.Action{Type: game.ActionDeclare, PlayerSeat: declarer})
				assert.ErrorIs(t, err, apperr.ErrWrongPhase, "declarations are over for the hand")

				final, turns := playHandPassingPrompts(t, resumed)
				for _, turn := range turns {
					assert.False(t, turn.prompted, "nobody is prompted after the resolution (trick %d seat %d)",
						turn.trick, turn.seat)
				}
				require.NotNil(t, final.LastHandResult)
				assert.Equal(t, 50, final.LastHandResult.TeamADeclPoints+final.LastHandResult.TeamBDeclPoints)
			})
		}
	}
}

// TestDeclarationCycleWithSeveralDeclarers checks every declaration made inside the cycle competes in its single
// resolution, with the trick-1 leader winning a full tie, and that a meldless declare is refused.
func TestDeclarationCycleWithSeveralDeclarers(t *testing.T) {
	gs := driveToWindowTurn(t, testfixtures.NewGameBitolaWindow(), 1, 0)
	cycle := apply(t, gs, game.Action{Type: game.ActionDeclare, PlayerSeat: 1})

	cycle = apply(t, cycle, game.Action{Type: game.ActionDeclare, PlayerSeat: 3})
	cycle = apply(t, cycle, game.Action{Type: game.ActionDeclare, PlayerSeat: 0})
	require.Equal(t, game.PhaseDeclaring, cycle.Phase, "seat 2 still owes an answer")

	resolved := apply(t, cycle, game.Action{Type: game.ActionDeclare, PlayerSeat: 2})

	require.Equal(t, game.PhasePlaying, resolved.Phase)
	assert.True(t, resolved.DeclarationsContested)
	assert.Equal(t, 100, resolved.DeclarationPoints[game.TeamB],
		"four 50s tie down to the Ace-topped runs and seat 1 leads trick 1, so team B scores both quartes")
	assert.Equal(t, 0, resolved.DeclarationPoints[game.TeamA])
	assert.Empty(t, resolved.Players[0].Declarations, "the losing team's melds are dropped")
	assert.Empty(t, resolved.Players[2].Declarations)
	assert.Len(t, resolved.Players[1].Declarations, 1)
	assert.Len(t, resolved.Players[3].Declarations, 1)
	assert.Equal(t, 1, resolved.ActivePlayerSeat)

	t.Run("a meldless declare inside the cycle is refused", func(t *testing.T) {
		meldless := testfixtures.NewGameBitolaWindow()
		meldless.Players[2].Hand = makeCards("7C", "9C", "JC", "KC", "JH", "QH", "JD", "QD")
		meldless.Players[3].Hand = makeCards("8C", "TC", "QC", "AC", "KH", "AH", "KD", "AD")
		meldless.AwaitingDeclaration = true
		opened := apply(t, meldless, game.Action{Type: game.ActionDeclare, PlayerSeat: 1})

		_, err := game.ApplyAction(opened, game.Action{Type: game.ActionDeclare, PlayerSeat: 2})
		assert.ErrorIs(t, err, apperr.ErrDeclarationNotAvailable)
		apply(t, opened, game.Action{Type: game.ActionSkipDeclare, PlayerSeat: 2})
	})
}

// currentHandFixture is NewGameBitolaWindow with seat 2 down to a clubs tierce and seat 3 meldless.
func currentHandFixture() *game.GameState {
	gs := testfixtures.NewGameBitolaWindow()
	gs.Players[2].Hand = makeCards("7C", "8C", "9C", "QC", "JH", "QH", "JD", "QD")
	gs.Players[3].Hand = makeCards("JC", "TC", "KC", "AC", "KH", "AH", "KD", "AD")
	gs.AwaitingDeclaration = true
	return gs
}

// playCurrentHandTrick1 plays the trick-1 script of TestDeclarationWindowUsesTheCurrentHand, in which seat 0 gives up
// the Ten of its spade quarte and seat 2 wins with the trump Jack.
func playCurrentHandTrick1(t *testing.T, gs *game.GameState) *game.GameState {
	t.Helper()
	gs = passAndPlay(t, gs, 1, "JS")
	gs = passAndPlay(t, gs, 2, "JH")
	require.False(t, gs.AwaitingDeclaration, "seat 3 holds no meld")
	gs = playCard(t, gs, 3, "KH")
	gs = passAndPlay(t, gs, 0, "TS")
	require.Equal(t, 2, gs.TrickNumber)
	require.Equal(t, 2, gs.ActivePlayerSeat, "the trump Jack took trick 1")
	return gs
}

// TestDeclarationWindowUsesTheCurrentHand checks a run card already played shortens the meld the prompt and the
// server offer, or removes it.
func TestDeclarationWindowUsesTheCurrentHand(t *testing.T) {
	t.Run("a quarte that lost its Ten is declared as a tierce", func(t *testing.T) {
		gs := playCurrentHandTrick1(t, currentHandFixture())
		gs = passAndPlay(t, gs, 2, "9C")
		gs = playCard(t, gs, 3, "JC")
		require.True(t, gs.AwaitingDeclaration, "7-8-9 of spades is still a meld")

		cycle := apply(t, gs, game.Action{Type: game.ActionDeclare, PlayerSeat: 0})
		require.Len(t, cycle.Players[0].Declarations, 1)
		assert.Equal(t, 20, cycle.Players[0].Declarations[0].Value)
		assert.Equal(t, makeCards("7S", "8S", "9S"), cycle.Players[0].Declarations[0].Cards)
	})

	t.Run("a tierce that lost a card is gone", func(t *testing.T) {
		gs := playCurrentHandTrick1(t, currentHandFixture())
		gs = passAndPlay(t, gs, 2, "9C")
		gs = playCard(t, gs, 3, "JC")
		gs = passAndPlay(t, gs, 0, "7H")
		gs = passAndPlay(t, gs, 1, "9H")
		require.Equal(t, 3, gs.TrickNumber)
		require.Equal(t, 1, gs.ActivePlayerSeat, "the trump Nine took trick 2")

		gs = passAndPlay(t, gs, 1, "9D")
		require.Equal(t, 2, gs.ActivePlayerSeat)
		assert.False(t, game.HasDeclarableCombinations(gs, 2), "7-8 of clubs is no meld")
		assert.False(t, gs.AwaitingDeclaration, "so seat 2 is not asked")
		_, err := game.ApplyAction(gs, game.Action{Type: game.ActionDeclare, PlayerSeat: 2})
		assert.ErrorIs(t, err, apperr.ErrWrongPhase)
	})
}

// TestDeclarationWindowLeavesCroatiaAlone checks a Croatian hand never prompts during play, since its contest is
// settled in the dedicated phase before trick 1.
func TestDeclarationWindowLeavesCroatiaAlone(t *testing.T) {
	gs := testfixtures.NewGameCroatianDeclaring(game.SuitHearts)
	for _, seat := range []int{1, 2, 3, 0} {
		action := game.ActionSkipDeclare
		if game.HasDeclarableCombinations(gs, seat) {
			action = game.ActionDeclare
		}
		gs = apply(t, gs, game.Action{Type: action, PlayerSeat: seat})
	}
	require.Equal(t, game.PhasePlaying, gs.Phase)
	require.Equal(t, 1, gs.TrickNumber)

	_, turns := playHandPassingPrompts(t, gs)
	for _, turn := range turns {
		assert.False(t, turn.prompted, "trick %d seat %d", turn.trick, turn.seat)
	}
}

// TestNewGameCarriesTheDeclarationWindow checks the room's last declaration trick reaches a Bitola game and its wire
// mirror, while an out-of-range value keeps the preset and a Croatian game has no window at all.
func TestNewGameCarriesTheDeclarationWindow(t *testing.T) {
	newGame := func(variant game.Variant, lastTrick int) *game.GameState {
		return game.NewGame([4]uint{10, 20, 30, 40}, [4]string{"a", "b", "c", "d"}, [4]bool{},
			variant, "1001", 1, true, false, true, false, lastTrick)
	}

	for lastTrick := game.MinDeclarationLastTrick; lastTrick <= game.MaxDeclarationLastTrick; lastTrick++ {
		gs := newGame(game.VariantBitola, lastTrick)
		assert.Equal(t, lastTrick, gs.Rules.DeclarationLastTrick)
		assert.Equal(t, lastTrick, gs.DeclarationLastTrick, "the wire mirror carries it")
	}
	for _, outOfRange := range []int{0, 4, -1} {
		gs := newGame(game.VariantBitola, outOfRange)
		assert.Equal(t, game.DefaultDeclarationLastTrick, gs.Rules.DeclarationLastTrick, "value %d", outOfRange)
	}

	croatian := newGame(game.VariantCroatia, 2)
	assert.Equal(t, 0, croatian.Rules.DeclarationLastTrick, "the dedicated phase has no turn window")
	assert.Equal(t, game.RulesFor(game.VariantCroatia), croatian.Rules)
}

// TestDeclarationWindowPromptDoesNotChangeAnotherSeatsError checks a card from a seat that is not on turn gets the
// same error whether or not the active seat sits at its hidden prompt.
func TestDeclarationWindowPromptDoesNotChangeAnotherSeatsError(t *testing.T) {
	for _, prompted := range []bool{false, true} {
		gs := testfixtures.NewGameBitolaWindow()
		gs.AwaitingDeclaration = prompted
		card := gs.Players[2].Hand[0]

		_, err := game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 2, Card: &card})
		assert.ErrorIs(t, err, apperr.ErrNotYourTurn, "active seat prompted: %v", prompted)
	}
}
