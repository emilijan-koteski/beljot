package game_test

import (
	"testing"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidationAllTrumpCards tests that when all hand cards are trump and
// a non-trump suit is led with opponent winning, all trump cards are legal.
func TestValidationAllTrumpCards(t *testing.T) {
	t.Run("all hand cards are trump - over-trump applies among them", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		gs.Players[2].Hand = []game.Card{
			{Rank: game.RankKing, Suit: game.SuitHearts},
			{Rank: game.RankQueen, Suit: game.SuitHearts},
			{Rank: game.Rank8, Suit: game.SuitHearts},
			{Rank: game.Rank7, Suit: game.SuitHearts},
		}

		gs.ActivePlayerSeat = 1
		leadAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.RankJack, Suit: game.SuitSpades}}
		gs, err := game.ApplyAction(gs, leadAction)
		require.NoError(t, err)

		lowerTrump := game.Action{Type: game.ActionPlayCard, PlayerSeat: 2,
			Card: &game.Card{Rank: game.Rank7, Suit: game.SuitHearts}}
		result, err := game.ApplyAction(gs, lowerTrump)

		require.NoError(t, err)
		require.NotNil(t, result)
	})
}

// TestValidationOneCardLeft tests that the last card is always legal.
func TestValidationOneCardLeft(t *testing.T) {
	t.Run("player with one card left - always legal", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(8)
		action := game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankAce, Suit: game.SuitSpades}}

		result, err := game.ApplyAction(gs, action)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Empty(t, result.Players[0].Hand)
	})
}

// TestValidationFourthPlayer tests that rules still apply for the 4th player.
func TestValidationFourthPlayer(t *testing.T) {
	t.Run("4th player still must follow suit", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		plays := []struct {
			seat int
			card game.Card
		}{
			{0, game.Card{Rank: game.RankAce, Suit: game.SuitSpades}},
			{1, game.Card{Rank: game.RankJack, Suit: game.SuitSpades}},
			{2, game.Card{Rank: game.RankKing, Suit: game.SuitHearts}},
		}

		state := gs
		for _, p := range plays {
			action := game.Action{Type: game.ActionPlayCard, PlayerSeat: p.seat, Card: &p.card}
			var err error
			state, err = game.ApplyAction(state, action)
			require.NoError(t, err)
		}

		anyCard := game.Action{Type: game.ActionPlayCard, PlayerSeat: 3,
			Card: &game.Card{Rank: game.RankKing, Suit: game.SuitClubs}}
		result, err := game.ApplyAction(state, anyCard)

		require.NoError(t, err)
		require.NotNil(t, result)
	})
}

// TestValidationPartnerTrumped tests that the must-overplay rule still applies
// when partner played the highest card so far.
func TestValidationPartnerTrumped(t *testing.T) {
	t.Run("partner trumped and is winning - has led suit, must follow", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		gs.ActivePlayerSeat = 1
		plays := []struct {
			seat int
			card game.Card
		}{
			{1, game.Card{Rank: game.RankJack, Suit: game.SuitSpades}},
			{2, game.Card{Rank: game.RankKing, Suit: game.SuitHearts}},
			{3, game.Card{Rank: game.Rank7, Suit: game.SuitDiamonds}},
		}

		state := gs
		for _, p := range plays {
			action := game.Action{Type: game.ActionPlayCard, PlayerSeat: p.seat, Card: &p.card}
			var err error
			state, err = game.ApplyAction(state, action)
			require.NoError(t, err)
		}

		validSpade := game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankAce, Suit: game.SuitSpades}}
		result, err := game.ApplyAction(state, validSpade)
		require.NoError(t, err)
		require.NotNil(t, result)

		state2 := gs
		for _, p := range plays {
			action := game.Action{Type: game.ActionPlayCard, PlayerSeat: p.seat, Card: &p.card}
			var err error
			state2, err = game.ApplyAction(state2, action)
			require.NoError(t, err)
		}
		illegalNonSpade := game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankAce, Suit: game.SuitHearts}}
		result, err = game.ApplyAction(state2, illegalNonSpade)
		assert.ErrorIs(t, err, apperr.ErrIllegalPlay)
		assert.Nil(t, result)
	})
}

func TestValidationMustOverplayLedSuit(t *testing.T) {
	t.Run("must overplay led non-trump suit when higher card available", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		gs.Players[1].Hand = []game.Card{
			{Rank: game.Rank8, Suit: game.SuitSpades},
			{Rank: game.RankAce, Suit: game.SuitSpades},
			{Rank: game.RankKing, Suit: game.SuitClubs},
			{Rank: game.RankAce, Suit: game.SuitClubs},
		}

		leadAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankKing, Suit: game.SuitSpades}}
		gs, err := game.ApplyAction(gs, leadAction)
		require.NoError(t, err)

		illegalLow := game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.Rank8, Suit: game.SuitSpades}}
		_, err = game.ApplyAction(gs, illegalLow)
		assert.ErrorIs(t, err, apperr.ErrIllegalPlay)

		legalHigh := game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.RankAce, Suit: game.SuitSpades}}
		result, err := game.ApplyAction(gs, legalHigh)
		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("falls back to any same-suit card when no overplay exists", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		gs.Players[1].Hand = []game.Card{
			{Rank: game.Rank7, Suit: game.SuitSpades},
			{Rank: game.Rank8, Suit: game.SuitSpades},
			{Rank: game.RankKing, Suit: game.SuitClubs},
		}

		leadAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankAce, Suit: game.SuitSpades}}
		gs, err := game.ApplyAction(gs, leadAction)
		require.NoError(t, err)

		playLow := game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.Rank7, Suit: game.SuitSpades}}
		result, err := game.ApplyAction(gs, playLow)
		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("led non-trump cut by trump — any led-suit card legal, off-suit illegal", func(t *testing.T) {
		setup := func() *game.GameState {
			gs := testfixtures.NewGameMidPlay(1)
			gs.Players[1].Hand = []game.Card{
				{Rank: game.Rank7, Suit: game.SuitSpades},
				{Rank: game.Rank9, Suit: game.SuitSpades},
				{Rank: game.Rank8, Suit: game.SuitSpades},
				{Rank: game.RankJack, Suit: game.SuitSpades},
				{Rank: game.Rank8, Suit: game.SuitClubs},
			}
			gs.Players[3].Hand = []game.Card{
				{Rank: game.Rank8, Suit: game.SuitSpades},
				{Rank: game.RankAce, Suit: game.SuitSpades},
				{Rank: game.Rank9, Suit: game.SuitHearts},
				{Rank: game.RankKing, Suit: game.SuitClubs},
				{Rank: game.RankQueen, Suit: game.SuitClubs},
			}

			gs, err := game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
				Card: &game.Card{Rank: game.RankKing, Suit: game.SuitSpades}})
			require.NoError(t, err)
			gs, err = game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
				Card: &game.Card{Rank: game.Rank7, Suit: game.SuitSpades}})
			require.NoError(t, err)
			gs, err = game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 2,
				Card: &game.Card{Rank: game.Rank7, Suit: game.SuitHearts}})
			require.NoError(t, err)
			return gs
		}

		for _, legalRank := range []game.Rank{game.Rank8, game.RankAce} {
			result, err := game.ApplyAction(setup(), game.Action{Type: game.ActionPlayCard, PlayerSeat: 3,
				Card: &game.Card{Rank: legalRank, Suit: game.SuitSpades}})
			require.NoError(t, err, "%s of S must be legal once the trick is cut by a trump", legalRank)
			require.NotNil(t, result)
		}

		for _, illegalRank := range []game.Rank{game.RankKing, game.RankQueen} {
			_, err := game.ApplyAction(setup(), game.Action{Type: game.ActionPlayCard, PlayerSeat: 3,
				Card: &game.Card{Rank: illegalRank, Suit: game.SuitClubs}})
			assert.ErrorIs(t, err, apperr.ErrIllegalPlay, "%s of C must be illegal: seat 3 still holds spades", illegalRank)
		}

		_, err := game.ApplyAction(setup(), game.Action{Type: game.ActionPlayCard, PlayerSeat: 3,
			Card: &game.Card{Rank: game.Rank9, Suit: game.SuitHearts}})
		assert.ErrorIs(t, err, apperr.ErrIllegalPlay, "9H (trump) must be illegal: seat 3 still holds spades")
	})

	t.Run("multiple higher led-suit cards — all strictly higher are legal", func(t *testing.T) {
		for _, candidate := range []game.Rank{game.RankTen, game.RankKing, game.RankAce} {
			gs := testfixtures.NewGameMidPlay(1)
			gs.Players[1].Hand = []game.Card{
				{Rank: game.RankTen, Suit: game.SuitSpades},
				{Rank: game.RankKing, Suit: game.SuitSpades},
				{Rank: game.RankAce, Suit: game.SuitSpades},
				{Rank: game.Rank7, Suit: game.SuitClubs},
			}

			led, err := game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard,
				PlayerSeat: 0, Card: &game.Card{Rank: game.RankQueen, Suit: game.SuitSpades}})
			require.NoError(t, err)

			result, err := game.ApplyAction(led, game.Action{Type: game.ActionPlayCard,
				PlayerSeat: 1, Card: &game.Card{Rank: candidate, Suit: game.SuitSpades}})
			require.NoError(t, err, "candidate %s should be a legal overplay of QS", candidate)
			require.NotNil(t, result)
		}
	})

	t.Run("must overplay led-suit when highest card on table came from partner", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		gs.ActivePlayerSeat = 2
		gs.Players[0].Hand = []game.Card{
			{Rank: game.Rank8, Suit: game.SuitSpades},
			{Rank: game.RankAce, Suit: game.SuitSpades},
			{Rank: game.RankKing, Suit: game.SuitClubs},
			{Rank: game.RankQueen, Suit: game.SuitClubs},
		}
		gs.Players[2].Hand = []game.Card{
			{Rank: game.RankKing, Suit: game.SuitSpades},
			{Rank: game.RankTen, Suit: game.SuitDiamonds},
			{Rank: game.RankAce, Suit: game.SuitDiamonds},
			{Rank: game.Rank8, Suit: game.SuitClubs},
		}
		gs.Players[3].Hand = []game.Card{
			{Rank: game.Rank7, Suit: game.SuitSpades},
			{Rank: game.Rank9, Suit: game.SuitDiamonds},
			{Rank: game.RankJack, Suit: game.SuitDiamonds},
			{Rank: game.Rank9, Suit: game.SuitClubs},
		}

		gs, err := game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 2,
			Card: &game.Card{Rank: game.RankKing, Suit: game.SuitSpades}})
		require.NoError(t, err)
		gs, err = game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 3,
			Card: &game.Card{Rank: game.Rank7, Suit: game.SuitSpades}})
		require.NoError(t, err)

		_, err = game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.Rank8, Suit: game.SuitSpades}})
		assert.ErrorIs(t, err, apperr.ErrIllegalPlay)

		result, err := game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankAce, Suit: game.SuitSpades}})
		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("img_two: void with trump, partner already trumped — must over-trump", func(t *testing.T) {
		setup := func() *game.GameState {
			gs := testfixtures.NewGameMidPlay(1)
			gs.ActivePlayerSeat = 1
			gs.Players[0].Hand = []game.Card{
				{Rank: game.Rank9, Suit: game.SuitHearts},
				{Rank: game.RankTen, Suit: game.SuitHearts},
				{Rank: game.Rank8, Suit: game.SuitDiamonds},
				{Rank: game.RankTen, Suit: game.SuitDiamonds},
				{Rank: game.RankJack, Suit: game.SuitDiamonds},
				{Rank: game.RankQueen, Suit: game.SuitDiamonds},
				{Rank: game.RankAce, Suit: game.SuitDiamonds},
			}
			gs.Players[1].Hand = []game.Card{
				{Rank: game.Rank9, Suit: game.SuitClubs},
				{Rank: game.Rank8, Suit: game.SuitClubs},
			}
			gs.Players[2].Hand = []game.Card{
				{Rank: game.RankQueen, Suit: game.SuitHearts},
				{Rank: game.RankAce, Suit: game.SuitDiamonds},
			}
			gs.Players[3].Hand = []game.Card{
				{Rank: game.Rank7, Suit: game.SuitClubs},
				{Rank: game.RankKing, Suit: game.SuitDiamonds},
			}

			gs, err := game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
				Card: &game.Card{Rank: game.Rank9, Suit: game.SuitClubs}})
			require.NoError(t, err)
			gs, err = game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 2,
				Card: &game.Card{Rank: game.RankQueen, Suit: game.SuitHearts}})
			require.NoError(t, err)
			gs, err = game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 3,
				Card: &game.Card{Rank: game.Rank7, Suit: game.SuitClubs}})
			require.NoError(t, err)
			return gs
		}

		for _, illegalRank := range []game.Rank{game.Rank8, game.RankTen, game.RankJack, game.RankQueen, game.RankAce} {
			_, err := game.ApplyAction(setup(), game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
				Card: &game.Card{Rank: illegalRank, Suit: game.SuitDiamonds}})
			assert.ErrorIs(t, err, apperr.ErrIllegalPlay, "%s of D must be illegal: void in clubs with over-trump available", illegalRank)
		}

		for _, legalRank := range []game.Rank{game.Rank9, game.RankTen} {
			result, err := game.ApplyAction(setup(), game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
				Card: &game.Card{Rank: legalRank, Suit: game.SuitHearts}})
			require.NoError(t, err, "%s of H must over-trump QH", legalRank)
			require.NotNil(t, result)
		}
	})

	t.Run("img_three: void in non-trump, partner winning, has one trump — only that trump", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		spades := game.SuitSpades
		gs.TrumpSuit = &spades
		callerSeat := 0
		gs.TrumpCallerSeat = &callerSeat
		gs.ActivePlayerSeat = 1
		gs.Players[0].Hand = []game.Card{
			{Rank: game.Rank8, Suit: game.SuitSpades},
			{Rank: game.RankJack, Suit: game.SuitDiamonds},
			{Rank: game.RankKing, Suit: game.SuitDiamonds},
		}
		gs.Players[1].Hand = []game.Card{
			{Rank: game.Rank9, Suit: game.SuitClubs},
			{Rank: game.Rank7, Suit: game.SuitDiamonds},
		}
		gs.Players[2].Hand = []game.Card{
			{Rank: game.RankAce, Suit: game.SuitClubs},
			{Rank: game.RankQueen, Suit: game.SuitDiamonds},
		}
		gs.Players[3].Hand = []game.Card{
			{Rank: game.RankTen, Suit: game.SuitClubs},
			{Rank: game.Rank8, Suit: game.SuitDiamonds},
		}

		gs, err := game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.Rank9, Suit: game.SuitClubs}})
		require.NoError(t, err)
		gs, err = game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 2,
			Card: &game.Card{Rank: game.RankAce, Suit: game.SuitClubs}})
		require.NoError(t, err)
		gs, err = game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 3,
			Card: &game.Card{Rank: game.RankTen, Suit: game.SuitClubs}})
		require.NoError(t, err)

		_, err = game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankJack, Suit: game.SuitDiamonds}})
		assert.ErrorIs(t, err, apperr.ErrIllegalPlay, "JD must be illegal: void in clubs, has 8S trump")
		_, err = game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankKing, Suit: game.SuitDiamonds}})
		assert.ErrorIs(t, err, apperr.ErrIllegalPlay, "KD must be illegal: void in clubs, has 8S trump")

		result, err := game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.Rank8, Suit: game.SuitSpades}})
		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("img_four: void in non-trump, no trump on table yet — only trumps legal", func(t *testing.T) {
		setup := func() *game.GameState {
			gs := testfixtures.NewGameMidPlay(1)
			gs.ActivePlayerSeat = 1
			gs.Players[0].Hand = []game.Card{
				{Rank: game.Rank7, Suit: game.SuitHearts},
				{Rank: game.Rank8, Suit: game.SuitHearts},
				{Rank: game.RankTen, Suit: game.SuitHearts},
				{Rank: game.RankKing, Suit: game.SuitHearts},
				{Rank: game.Rank7, Suit: game.SuitDiamonds},
				{Rank: game.Rank8, Suit: game.SuitDiamonds},
				{Rank: game.RankTen, Suit: game.SuitDiamonds},
			}
			gs.Players[1].Hand = []game.Card{
				{Rank: game.Rank7, Suit: game.SuitSpades},
			}
			gs.Players[2].Hand = []game.Card{
				{Rank: game.Rank9, Suit: game.SuitSpades},
				{Rank: game.RankAce, Suit: game.SuitDiamonds},
			}
			gs.Players[3].Hand = []game.Card{
				{Rank: game.RankKing, Suit: game.SuitClubs},
				{Rank: game.RankQueen, Suit: game.SuitClubs},
			}

			gs, err := game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
				Card: &game.Card{Rank: game.Rank7, Suit: game.SuitSpades}})
			require.NoError(t, err)
			gs, err = game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 2,
				Card: &game.Card{Rank: game.Rank9, Suit: game.SuitSpades}})
			require.NoError(t, err)
			gs, err = game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 3,
				Card: &game.Card{Rank: game.RankKing, Suit: game.SuitClubs}})
			require.NoError(t, err)
			return gs
		}

		for _, illegalRank := range []game.Rank{game.Rank7, game.Rank8, game.RankTen} {
			_, err := game.ApplyAction(setup(), game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
				Card: &game.Card{Rank: illegalRank, Suit: game.SuitDiamonds}})
			assert.ErrorIs(t, err, apperr.ErrIllegalPlay, "%s of D must be illegal: must trump", illegalRank)
		}

		for _, legalRank := range []game.Rank{game.Rank7, game.Rank8, game.RankTen, game.RankKing} {
			result, err := game.ApplyAction(setup(), game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
				Card: &game.Card{Rank: legalRank, Suit: game.SuitHearts}})
			require.NoError(t, err, "%s of H must be legal", legalRank)
			require.NotNil(t, result)
		}
	})

	t.Run("void in led suit, no trump in hand — any card legal", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		gs.ActivePlayerSeat = 0
		gs.Players[0].Hand = []game.Card{
			{Rank: game.RankAce, Suit: game.SuitSpades},
			{Rank: game.RankKing, Suit: game.SuitClubs},
		}
		gs.Players[1].Hand = []game.Card{
			{Rank: game.Rank8, Suit: game.SuitDiamonds},
		}
		gs.Players[2].Hand = []game.Card{
			{Rank: game.RankAce, Suit: game.SuitDiamonds},
		}
		gs.Players[3].Hand = []game.Card{
			{Rank: game.Rank7, Suit: game.SuitClubs},
		}

		gs.ActivePlayerSeat = 1
		gs, err := game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.Rank8, Suit: game.SuitDiamonds}})
		require.NoError(t, err)
		gs, err = game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 2,
			Card: &game.Card{Rank: game.RankAce, Suit: game.SuitDiamonds}})
		require.NoError(t, err)
		gs, err = game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 3,
			Card: &game.Card{Rank: game.Rank7, Suit: game.SuitClubs}})
		require.NoError(t, err)

		result, err := game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankKing, Suit: game.SuitClubs}})
		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("trump-led over-trump regression", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		gs.Players[0].Hand = []game.Card{
			{Rank: game.RankQueen, Suit: game.SuitHearts},
			{Rank: game.RankAce, Suit: game.SuitSpades},
			{Rank: game.RankTen, Suit: game.SuitSpades},
			{Rank: game.RankKing, Suit: game.SuitSpades},
			{Rank: game.RankAce, Suit: game.SuitClubs},
		}
		gs.Players[1].Hand = []game.Card{
			{Rank: game.RankJack, Suit: game.SuitHearts},
			{Rank: game.Rank9, Suit: game.SuitHearts},
			{Rank: game.Rank7, Suit: game.SuitHearts},
			{Rank: game.Rank8, Suit: game.SuitClubs},
		}

		gs, err := game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankQueen, Suit: game.SuitHearts}})
		require.NoError(t, err)

		_, err = game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.Rank7, Suit: game.SuitHearts}})
		assert.ErrorIs(t, err, apperr.ErrIllegalPlay)

		result, err := game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.RankJack, Suit: game.SuitHearts}})
		require.NoError(t, err)
		require.NotNil(t, result)
	})
}
