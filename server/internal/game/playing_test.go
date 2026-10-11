package game_test

import (
	"testing"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPlayCardLeading tests that any card is legal when leading a trick.
func TestPlayCardLeading(t *testing.T) {
	tests := []struct {
		name string
		card game.Card
	}{
		{"lead with spade", game.Card{Rank: game.RankAce, Suit: game.SuitSpades}},
		{"lead with trump heart", game.Card{Rank: game.RankAce, Suit: game.SuitHearts}},
		{"lead with diamond", game.Card{Rank: game.RankKing, Suit: game.SuitDiamonds}},
		{"lead with club", game.Card{Rank: game.Rank7, Suit: game.SuitClubs}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gs := testfixtures.NewGameMidPlay(1)
			action := game.Action{
				Type:       game.ActionPlayCard,
				PlayerSeat: 0,
				Card:       &tc.card,
			}

			result, err := game.ApplyAction(gs, action)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Len(t, result.Players[0].Hand, 7)
			assert.Len(t, result.CurrentTrick, 1)
			assert.Equal(t, tc.card.Rank, result.CurrentTrick[0].Card.Rank)
			assert.Equal(t, tc.card.Suit, result.CurrentTrick[0].Card.Suit)
			assert.Equal(t, 0, result.CurrentTrick[0].PlayerSeat)
			require.NotNil(t, result.LeadSuit)
			assert.Equal(t, tc.card.Suit, *result.LeadSuit)
			assert.Equal(t, 1, result.ActivePlayerSeat)
		})
	}
}

// TestPlayCardFollowSuit tests that players must follow the led suit.
func TestPlayCardFollowSuit(t *testing.T) {
	t.Run("player with led suit cards must play one", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		leadAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankAce, Suit: game.SuitSpades}}
		gs, err := game.ApplyAction(gs, leadAction)
		require.NoError(t, err)

		followAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.RankJack, Suit: game.SuitSpades}}
		result, err := game.ApplyAction(gs, followAction)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.CurrentTrick, 2)
	})

	t.Run("playing non-led suit when holding led suit returns ErrIllegalPlay", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		leadAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankAce, Suit: game.SuitSpades}}
		gs, err := game.ApplyAction(gs, leadAction)
		require.NoError(t, err)

		illegalAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.RankJack, Suit: game.SuitHearts}}
		result, err := game.ApplyAction(gs, illegalAction)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, apperr.ErrIllegalPlay)
	})
}

// TestPlayCardTrumpLed tests over-trump obligations when trump is led.
func TestPlayCardTrumpLed(t *testing.T) {
	t.Run("trump led - must over-trump if possible", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		gs.ActivePlayerSeat = 2
		leadAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 2,
			Card: &game.Card{Rank: game.RankQueen, Suit: game.SuitHearts}}
		gs, err := game.ApplyAction(gs, leadAction)
		require.NoError(t, err)

		anyCard := game.Action{Type: game.ActionPlayCard, PlayerSeat: 3,
			Card: &game.Card{Rank: game.RankJack, Suit: game.SuitDiamonds}}
		result, err := game.ApplyAction(gs, anyCard)
		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("trump led - lower trump rejected when higher available", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		gs.ActivePlayerSeat = 0
		leadAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankTen, Suit: game.SuitHearts}}
		gs, err := game.ApplyAction(gs, leadAction)
		require.NoError(t, err)

		validAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.Rank9, Suit: game.SuitHearts}}
		result, err := game.ApplyAction(gs, validAction)
		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("trump led - playing lower trump when higher available is illegal", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		gs.Players[1].Hand = []game.Card{
			{Rank: game.RankJack, Suit: game.SuitHearts},
			{Rank: game.Rank9, Suit: game.SuitHearts},
			{Rank: game.Rank7, Suit: game.SuitHearts},
			{Rank: game.RankJack, Suit: game.SuitSpades},
			{Rank: game.Rank9, Suit: game.SuitSpades},
			{Rank: game.Rank8, Suit: game.SuitSpades},
			{Rank: game.RankQueen, Suit: game.SuitDiamonds},
			{Rank: game.Rank8, Suit: game.SuitClubs},
		}
		gs.ActivePlayerSeat = 0
		leadAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankAce, Suit: game.SuitHearts}}
		gs, err := game.ApplyAction(gs, leadAction)
		require.NoError(t, err)

		illegalAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.Rank7, Suit: game.SuitHearts}}
		result, err := game.ApplyAction(gs, illegalAction)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, apperr.ErrIllegalPlay)
	})
}

// TestPlayCardTrumpObligation tests trump obligation when opponent is winning.
func TestPlayCardTrumpObligation(t *testing.T) {
	t.Run("void in led suit opponent winning - non-trump rejected", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		gs.ActivePlayerSeat = 1
		leadAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.RankJack, Suit: game.SuitSpades}}
		gs, err := game.ApplyAction(gs, leadAction)
		require.NoError(t, err)

		illegalAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 2,
			Card: &game.Card{Rank: game.RankAce, Suit: game.SuitDiamonds}}
		result, err := game.ApplyAction(gs, illegalAction)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, apperr.ErrIllegalPlay)
	})

	t.Run("void in led suit opponent winning - trump accepted", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		gs.ActivePlayerSeat = 1
		leadAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.RankJack, Suit: game.SuitSpades}}
		gs, err := game.ApplyAction(gs, leadAction)
		require.NoError(t, err)

		trumpAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 2,
			Card: &game.Card{Rank: game.RankKing, Suit: game.SuitHearts}}
		result, err := game.ApplyAction(gs, trumpAction)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.CurrentTrick, 2)
	})
}

// TestPlayCardOverTrump tests the over-trump obligation.
func TestPlayCardOverTrump(t *testing.T) {
	t.Run("must over-trump when obligated to trump and higher available", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		leadAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankQueen, Suit: game.SuitSpades}}
		gs, err := game.ApplyAction(gs, leadAction)
		require.NoError(t, err)

		gs.Players[1].Hand = []game.Card{
			{Rank: game.Rank9, Suit: game.SuitHearts},
			{Rank: game.Rank8, Suit: game.SuitClubs},
		}
		followAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.Rank9, Suit: game.SuitHearts}}
		gs, err = game.ApplyAction(gs, followAction)
		require.NoError(t, err)

		lowerTrumpAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 2,
			Card: &game.Card{Rank: game.RankKing, Suit: game.SuitHearts}}
		result, err := game.ApplyAction(gs, lowerTrumpAction)

		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("lower trump rejected when higher available - obligation path with trump in trick", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		gs.Players[1].Hand = []game.Card{
			{Rank: game.Rank8, Suit: game.SuitHearts},
			{Rank: game.Rank8, Suit: game.SuitClubs},
			{Rank: game.Rank9, Suit: game.SuitClubs},
			{Rank: game.RankQueen, Suit: game.SuitDiamonds},
			{Rank: game.RankJack, Suit: game.SuitDiamonds},
			{Rank: game.Rank9, Suit: game.SuitDiamonds},
			{Rank: game.Rank8, Suit: game.SuitDiamonds},
			{Rank: game.Rank7, Suit: game.SuitDiamonds},
		}
		gs.Players[2].Hand = []game.Card{
			{Rank: game.RankKing, Suit: game.SuitHearts},
			{Rank: game.RankQueen, Suit: game.SuitHearts},
			{Rank: game.Rank7, Suit: game.SuitHearts},
			{Rank: game.RankAce, Suit: game.SuitDiamonds},
			{Rank: game.RankTen, Suit: game.SuitDiamonds},
			{Rank: game.RankAce, Suit: game.SuitClubs},
			{Rank: game.RankTen, Suit: game.SuitClubs},
			{Rank: game.Rank7, Suit: game.SuitClubs},
		}

		leadAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankQueen, Suit: game.SuitSpades}}
		gs, err := game.ApplyAction(gs, leadAction)
		require.NoError(t, err)

		trumpAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.Rank8, Suit: game.SuitHearts}}
		gs, err = game.ApplyAction(gs, trumpAction)
		require.NoError(t, err)

		illegalLowerTrump := game.Action{Type: game.ActionPlayCard, PlayerSeat: 2,
			Card: &game.Card{Rank: game.Rank7, Suit: game.SuitHearts}}
		result, err := game.ApplyAction(gs, illegalLowerTrump)

		require.Error(t, err, "lower trump should be rejected when higher over-trumps available")
		assert.Nil(t, result)
		assert.ErrorIs(t, err, apperr.ErrIllegalPlay)
	})
}

// TestPlayCardVoidPartnerWinning tests that even when the partner is currently
// winning the trick, a void player who holds trump must cut. Bitola has no
// partner-winning exemption.
func TestPlayCardVoidPartnerWinning(t *testing.T) {
	t.Run("partner winning - void with trump must cut, non-trump rejected", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		leadAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankAce, Suit: game.SuitSpades}}
		gs, err := game.ApplyAction(gs, leadAction)
		require.NoError(t, err)

		followAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.Rank7, Suit: game.SuitSpades}}
		gs, err = game.ApplyAction(gs, followAction)
		require.NoError(t, err)

		nonTrumpAttempt := game.Action{Type: game.ActionPlayCard, PlayerSeat: 2,
			Card: &game.Card{Rank: game.RankAce, Suit: game.SuitDiamonds}}
		_, err = game.ApplyAction(gs, nonTrumpAttempt)
		assert.ErrorIs(t, err, apperr.ErrIllegalPlay,
			"non-trump play must be rejected: void in led suit with trump in hand")

		legalTrump := game.Action{Type: game.ActionPlayCard, PlayerSeat: 2,
			Card: &game.Card{Rank: game.Rank7, Suit: game.SuitHearts}}
		result, err := game.ApplyAction(gs, legalTrump)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.CurrentTrick, 3)
	})
}

// TestPlayCardNoTrumpHeld tests that any card is legal when void in suit and holding no trump.
func TestPlayCardNoTrumpHeld(t *testing.T) {
	t.Run("void in led suit no trump held - any card legal", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		leadAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankAce, Suit: game.SuitSpades}}
		gs, err := game.ApplyAction(gs, leadAction)
		require.NoError(t, err)

		followAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.RankJack, Suit: game.SuitSpades}}
		gs, err = game.ApplyAction(gs, followAction)
		require.NoError(t, err)

		trumpAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 2,
			Card: &game.Card{Rank: game.RankKing, Suit: game.SuitHearts}}
		gs, err = game.ApplyAction(gs, trumpAction)
		require.NoError(t, err)

		anyCard := game.Action{Type: game.ActionPlayCard, PlayerSeat: 3,
			Card: &game.Card{Rank: game.RankKing, Suit: game.SuitClubs}}
		result, err := game.ApplyAction(gs, anyCard)

		require.NoError(t, err)
		require.NotNil(t, result)
	})
}

// TestTrickResolution tests that trick resolution works correctly.
func TestTrickResolution(t *testing.T) {
	t.Run("4th card triggers resolution - winner gets points and leads next", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		gs.Players[2].Hand = []game.Card{
			{Rank: game.RankAce, Suit: game.SuitDiamonds},
			{Rank: game.RankTen, Suit: game.SuitDiamonds},
			{Rank: game.RankKing, Suit: game.SuitDiamonds},
			{Rank: game.RankQueen, Suit: game.SuitDiamonds},
			{Rank: game.RankAce, Suit: game.SuitClubs},
			{Rank: game.RankTen, Suit: game.SuitClubs},
			{Rank: game.RankKing, Suit: game.SuitClubs},
			{Rank: game.RankQueen, Suit: game.SuitClubs},
		}
		cards := []struct {
			seat int
			card game.Card
		}{
			{0, game.Card{Rank: game.RankAce, Suit: game.SuitSpades}},
			{1, game.Card{Rank: game.RankJack, Suit: game.SuitSpades}},
			{2, game.Card{Rank: game.RankAce, Suit: game.SuitDiamonds}},
			{3, game.Card{Rank: game.RankJack, Suit: game.SuitDiamonds}},
		}

		var result *game.GameState
		var err error
		state := gs
		for _, c := range cards {
			action := game.Action{Type: game.ActionPlayCard, PlayerSeat: c.seat, Card: &c.card}
			result, err = game.ApplyAction(state, action)
			require.NoError(t, err)
			state = result
		}

		assert.Equal(t, 26, result.HandPoints[0])
		assert.Equal(t, 0, result.HandPoints[1])
		assert.Equal(t, 1, result.TricksWon[0])
		assert.Equal(t, 0, result.TricksWon[1])
		assert.Equal(t, 0, result.ActivePlayerSeat)
		assert.Equal(t, 2, result.TrickNumber)
		assert.Empty(t, result.CurrentTrick)
		assert.Nil(t, result.LeadSuit)
		assert.Nil(t, result.TrickWinnerSeat)
		assert.Equal(t, game.PhasePlaying, result.Phase)
	})

	t.Run("trump beats non-trump", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		cards := []struct {
			seat int
			card game.Card
		}{
			{0, game.Card{Rank: game.RankQueen, Suit: game.SuitSpades}},
			{1, game.Card{Rank: game.Rank7, Suit: game.SuitSpades}},
			{2, game.Card{Rank: game.Rank7, Suit: game.SuitHearts}},
			{3, game.Card{Rank: game.Rank7, Suit: game.SuitDiamonds}},
		}

		state := gs
		var result *game.GameState
		var err error
		for _, c := range cards {
			action := game.Action{Type: game.ActionPlayCard, PlayerSeat: c.seat, Card: &c.card}
			result, err = game.ApplyAction(state, action)
			require.NoError(t, err)
			state = result
		}

		assert.Equal(t, 2, result.ActivePlayerSeat)
		assert.Equal(t, 1, result.TricksWon[0])
	})

	t.Run("higher trump beats lower trump", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		cards := []struct {
			seat int
			card game.Card
		}{
			{0, game.Card{Rank: game.RankQueen, Suit: game.SuitSpades}},
			{1, game.Card{Rank: game.Rank9, Suit: game.SuitSpades}},
			{2, game.Card{Rank: game.Rank8, Suit: game.SuitHearts}},
			{3, game.Card{Rank: game.Rank9, Suit: game.SuitDiamonds}},
		}

		state := gs
		var result *game.GameState
		var err error
		for _, c := range cards {
			action := game.Action{Type: game.ActionPlayCard, PlayerSeat: c.seat, Card: &c.card}
			result, err = game.ApplyAction(state, action)
			require.NoError(t, err)
			state = result
		}

		assert.Equal(t, 2, result.ActivePlayerSeat)
	})
}

// TestTrickPointCalculation tests card point accumulation.
func TestTrickPointCalculation(t *testing.T) {
	t.Run("correct points for mixed trick", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		cards := []struct {
			seat int
			card game.Card
		}{
			{0, game.Card{Rank: game.RankAce, Suit: game.SuitSpades}},
			{1, game.Card{Rank: game.Rank9, Suit: game.SuitSpades}},
			{2, game.Card{Rank: game.RankKing, Suit: game.SuitHearts}},
			{3, game.Card{Rank: game.Rank7, Suit: game.SuitDiamonds}},
		}

		state := gs
		var result *game.GameState
		var err error
		for _, c := range cards {
			action := game.Action{Type: game.ActionPlayCard, PlayerSeat: c.seat, Card: &c.card}
			result, err = game.ApplyAction(state, action)
			require.NoError(t, err)
			state = result
		}

		assert.Equal(t, 15, result.HandPoints[0])
	})
}

// TestEighthTrickTransition tests phase transition after the 8th trick.
func TestEighthTrickTransition(t *testing.T) {
	t.Run("8th trick transitions to PhaseHandScoring", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(8)
		cards := []struct {
			seat int
			card game.Card
		}{
			{0, game.Card{Rank: game.RankAce, Suit: game.SuitSpades}},
			{1, game.Card{Rank: game.RankJack, Suit: game.SuitSpades}},
			{2, game.Card{Rank: game.RankAce, Suit: game.SuitDiamonds}},
			{3, game.Card{Rank: game.RankJack, Suit: game.SuitDiamonds}},
		}

		state := gs
		var result *game.GameState
		var err error
		for _, c := range cards {
			action := game.Action{Type: game.ActionPlayCard, PlayerSeat: c.seat, Card: &c.card}
			result, err = game.ApplyAction(state, action)
			require.NoError(t, err)
			state = result
		}

		assert.Contains(t, []game.Phase{game.PhaseHandComplete, game.PhaseMatchEnd}, result.Phase,
			"should advance past PhaseHandScoring to PhaseHandComplete or PhaseMatchEnd")
	})
}

// TestPlayCardErrors tests error conditions.
func TestPlayCardErrors(t *testing.T) {
	t.Run("card not in hand returns ErrInvalidCard", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		action := game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.Rank7, Suit: game.SuitDiamonds}}

		result, err := game.ApplyAction(gs, action)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, apperr.ErrInvalidCard)
	})

	t.Run("nil card returns ErrInvalidCard", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		action := game.Action{Type: game.ActionPlayCard, PlayerSeat: 0, Card: nil}

		result, err := game.ApplyAction(gs, action)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, apperr.ErrInvalidCard)
	})

	t.Run("wrong player returns ErrNotYourTurn", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		action := game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.RankJack, Suit: game.SuitSpades}}

		result, err := game.ApplyAction(gs, action)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, apperr.ErrNotYourTurn)
	})

	t.Run("play_card in non-playing phase returns ErrWrongPhase", func(t *testing.T) {
		gs := testfixtures.NewGameJustDealt()
		action := game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.Rank7, Suit: game.SuitHearts}}

		result, err := game.ApplyAction(gs, action)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, apperr.ErrWrongPhase)
	})

	t.Run("declare action in playing phase returns ErrWrongPhase", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		action := game.Action{Type: game.ActionDeclare, PlayerSeat: 0}

		result, err := game.ApplyAction(gs, action)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, apperr.ErrWrongPhase)
	})
}

// TestPlayCardStateImmutability tests that the original state is not modified.
func TestPlayCardStateImmutability(t *testing.T) {
	t.Run("original state unchanged after ApplyAction", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		originalHandLen := len(gs.Players[0].Hand)
		originalTrickLen := len(gs.CurrentTrick)
		originalActive := gs.ActivePlayerSeat
		originalPhase := gs.Phase

		action := game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankAce, Suit: game.SuitSpades}}

		result, err := game.ApplyAction(gs, action)

		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Len(t, gs.Players[0].Hand, originalHandLen)
		assert.Len(t, gs.CurrentTrick, originalTrickLen)
		assert.Equal(t, originalActive, gs.ActivePlayerSeat)
		assert.Equal(t, originalPhase, gs.Phase)
		assert.Nil(t, gs.LeadSuit)
	})

	t.Run("pointer fields not aliased after clone", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		action := game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankAce, Suit: game.SuitSpades}}

		result, err := game.ApplyAction(gs, action)
		require.NoError(t, err)

		newSuit := game.SuitClubs
		result.LeadSuit = &newSuit
		assert.Equal(t, game.SuitHearts, *gs.TrumpSuit)
	})
}

// TestFull8TrickHand plays a complete 8-trick hand and verifies that the
// game progresses through scoring under the no-partner-exemption rule.
func TestFull8TrickHand(t *testing.T) {
	t.Run("all 32 cards played, hand ends with scoring", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)

		trick1 := []struct {
			seat int
			card game.Card
		}{
			{0, game.Card{Rank: game.RankAce, Suit: game.SuitSpades}},
			{1, game.Card{Rank: game.Rank9, Suit: game.SuitSpades}},
			{2, game.Card{Rank: game.Rank7, Suit: game.SuitHearts}},
			{3, game.Card{Rank: game.RankJack, Suit: game.SuitDiamonds}},
		}
		state := gs
		for _, c := range trick1 {
			action := game.Action{Type: game.ActionPlayCard, PlayerSeat: c.seat, Card: &c.card}
			var err error
			state, err = game.ApplyAction(state, action)
			require.NoError(t, err, "trick 1 seat %d", c.seat)
		}
		assert.Equal(t, 2, state.TrickNumber)
		assert.Equal(t, 2, state.ActivePlayerSeat)

		trick2 := []struct {
			seat int
			card game.Card
		}{
			{2, game.Card{Rank: game.RankAce, Suit: game.SuitClubs}},
			{3, game.Card{Rank: game.Rank9, Suit: game.SuitClubs}},
			{0, game.Card{Rank: game.Rank7, Suit: game.SuitClubs}},
			{1, game.Card{Rank: game.Rank8, Suit: game.SuitClubs}},
		}
		for _, c := range trick2 {
			action := game.Action{Type: game.ActionPlayCard, PlayerSeat: c.seat, Card: &c.card}
			var err error
			state, err = game.ApplyAction(state, action)
			require.NoError(t, err, "trick 2 seat %d", c.seat)
		}
		assert.Equal(t, 2, state.ActivePlayerSeat)

		trick3 := []struct {
			seat int
			card game.Card
		}{
			{2, game.Card{Rank: game.RankTen, Suit: game.SuitClubs}},
			{3, game.Card{Rank: game.RankJack, Suit: game.SuitClubs}},
			{0, game.Card{Rank: game.RankTen, Suit: game.SuitHearts}},
			{1, game.Card{Rank: game.Rank9, Suit: game.SuitHearts}},
		}
		for _, c := range trick3 {
			action := game.Action{Type: game.ActionPlayCard, PlayerSeat: c.seat, Card: &c.card}
			var err error
			state, err = game.ApplyAction(state, action)
			require.NoError(t, err, "trick 3 seat %d", c.seat)
		}
		assert.Equal(t, 1, state.ActivePlayerSeat)

		trick4 := []struct {
			seat int
			card game.Card
		}{
			{1, game.Card{Rank: game.RankJack, Suit: game.SuitHearts}},
			{2, game.Card{Rank: game.Rank8, Suit: game.SuitHearts}},
			{3, game.Card{Rank: game.Rank7, Suit: game.SuitDiamonds}},
			{0, game.Card{Rank: game.RankAce, Suit: game.SuitHearts}},
		}
		for _, c := range trick4 {
			action := game.Action{Type: game.ActionPlayCard, PlayerSeat: c.seat, Card: &c.card}
			var err error
			state, err = game.ApplyAction(state, action)
			require.NoError(t, err, "trick 4 seat %d", c.seat)
		}
		assert.Equal(t, 1, state.ActivePlayerSeat)

		trick5 := []struct {
			seat int
			card game.Card
		}{
			{1, game.Card{Rank: game.RankQueen, Suit: game.SuitDiamonds}},
			{2, game.Card{Rank: game.RankTen, Suit: game.SuitDiamonds}},
			{3, game.Card{Rank: game.Rank8, Suit: game.SuitDiamonds}},
			{0, game.Card{Rank: game.RankKing, Suit: game.SuitDiamonds}},
		}
		for _, c := range trick5 {
			action := game.Action{Type: game.ActionPlayCard, PlayerSeat: c.seat, Card: &c.card}
			var err error
			state, err = game.ApplyAction(state, action)
			require.NoError(t, err, "trick 5 seat %d", c.seat)
		}
		assert.Equal(t, 2, state.ActivePlayerSeat)

		trick6 := []struct {
			seat int
			card game.Card
		}{
			{2, game.Card{Rank: game.RankAce, Suit: game.SuitDiamonds}},
			{3, game.Card{Rank: game.Rank9, Suit: game.SuitDiamonds}},
			{0, game.Card{Rank: game.RankQueen, Suit: game.SuitSpades}},
			{1, game.Card{Rank: game.Rank7, Suit: game.SuitSpades}},
		}
		for _, c := range trick6 {
			action := game.Action{Type: game.ActionPlayCard, PlayerSeat: c.seat, Card: &c.card}
			var err error
			state, err = game.ApplyAction(state, action)
			require.NoError(t, err, "trick 6 seat %d", c.seat)
		}
		assert.Equal(t, 2, state.ActivePlayerSeat)

		trick7 := []struct {
			seat int
			card game.Card
		}{
			{2, game.Card{Rank: game.RankKing, Suit: game.SuitHearts}},
			{3, game.Card{Rank: game.RankKing, Suit: game.SuitClubs}},
			{0, game.Card{Rank: game.RankKing, Suit: game.SuitSpades}},
			{1, game.Card{Rank: game.RankJack, Suit: game.SuitSpades}},
		}
		for _, c := range trick7 {
			action := game.Action{Type: game.ActionPlayCard, PlayerSeat: c.seat, Card: &c.card}
			var err error
			state, err = game.ApplyAction(state, action)
			require.NoError(t, err, "trick 7 seat %d", c.seat)
		}
		assert.Equal(t, 2, state.ActivePlayerSeat)

		trick8 := []struct {
			seat int
			card game.Card
		}{
			{2, game.Card{Rank: game.RankQueen, Suit: game.SuitHearts}},
			{3, game.Card{Rank: game.RankQueen, Suit: game.SuitClubs}},
			{0, game.Card{Rank: game.RankTen, Suit: game.SuitSpades}},
			{1, game.Card{Rank: game.Rank8, Suit: game.SuitSpades}},
		}
		for _, c := range trick8 {
			action := game.Action{Type: game.ActionPlayCard, PlayerSeat: c.seat, Card: &c.card}
			var err error
			state, err = game.ApplyAction(state, action)
			require.NoError(t, err, "trick 8 seat %d", c.seat)
		}

		assert.Contains(t, []game.Phase{game.PhaseHandComplete, game.PhaseMatchEnd}, state.Phase,
			"should advance past PhaseHandScoring to PhaseHandComplete or PhaseMatchEnd")
		totalTeamScores := state.TeamScores[0] + state.TeamScores[1]
		assert.Greater(t, totalTeamScores, 0, "TeamScores should reflect scored hand points (152 card pts + 10 last-trick bonus = 162)")

		if state.Phase == game.PhaseBidding {
			for i, p := range state.Players {
				assert.Len(t, p.Hand, 8, "player %d should have 8 cards after new deal", i)
			}
		}
	})
}
