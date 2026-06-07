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
			// Card removed from hand
			assert.Len(t, result.Players[0].Hand, 7)
			// Card added to trick
			assert.Len(t, result.CurrentTrick, 1)
			assert.Equal(t, tc.card.Rank, result.CurrentTrick[0].Card.Rank)
			assert.Equal(t, tc.card.Suit, result.CurrentTrick[0].Card.Suit)
			assert.Equal(t, 0, result.CurrentTrick[0].PlayerSeat)
			// Lead suit set
			require.NotNil(t, result.LeadSuit)
			assert.Equal(t, tc.card.Suit, *result.LeadSuit)
			// Active player advances
			assert.Equal(t, 1, result.ActivePlayerSeat)
		})
	}
}

// TestPlayCardFollowSuit tests that players must follow the led suit.
func TestPlayCardFollowSuit(t *testing.T) {
	t.Run("player with led suit cards must play one", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		// Seat 0 leads a spade
		leadAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankAce, Suit: game.SuitSpades}}
		gs, err := game.ApplyAction(gs, leadAction)
		require.NoError(t, err)

		// Seat 1 has spades (JS, 9S, 8S, 7S) — must play one
		followAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.RankJack, Suit: game.SuitSpades}}
		result, err := game.ApplyAction(gs, followAction)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result.CurrentTrick, 2)
	})

	t.Run("playing non-led suit when holding led suit returns ErrIllegalPlay", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		// Seat 0 leads a spade
		leadAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankAce, Suit: game.SuitSpades}}
		gs, err := game.ApplyAction(gs, leadAction)
		require.NoError(t, err)

		// Seat 1 has spades but tries to play a heart (trump)
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
		// Set seat 2 as active (has KH, QH, 8H, 7H)
		gs.ActivePlayerSeat = 2
		// Seat 2 leads with QH (trump)
		leadAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 2,
			Card: &game.Card{Rank: game.RankQueen, Suit: game.SuitHearts}}
		gs, err := game.ApplyAction(gs, leadAction)
		require.NoError(t, err)

		// Seat 3 has no trump — must play any card (void in trump, no obligation to follow)
		// But first test seat 0 (active=3 now, but seat 3 has no trump)
		// Advance: after seat 2 lead, active is seat 3
		// Seat 3 has no hearts — any card is legal
		anyCard := game.Action{Type: game.ActionPlayCard, PlayerSeat: 3,
			Card: &game.Card{Rank: game.RankJack, Suit: game.SuitDiamonds}}
		result, err := game.ApplyAction(gs, anyCard)
		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("trump led - lower trump rejected when higher available", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		// Seat 0 leads with TH (trump, rank order 4)
		gs.ActivePlayerSeat = 0
		leadAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankTen, Suit: game.SuitHearts}}
		gs, err := game.ApplyAction(gs, leadAction)
		require.NoError(t, err)

		// Seat 1 has JH (order 7) and 9H (order 6) — both beat TH (order 4)
		// Playing 9H is valid (over-trumps)
		validAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.Rank9, Suit: game.SuitHearts}}
		result, err := game.ApplyAction(gs, validAction)
		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("trump led - playing lower trump when higher available is illegal", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		// Give seat 1 a mix of higher and lower trumps
		gs.Players[1].Hand = []game.Card{
			{Rank: game.RankJack, Suit: game.SuitHearts}, // trump order 7
			{Rank: game.Rank9, Suit: game.SuitHearts},    // trump order 6
			{Rank: game.Rank7, Suit: game.SuitHearts},    // trump order 0 (LOWER)
			{Rank: game.RankJack, Suit: game.SuitSpades},
			{Rank: game.Rank9, Suit: game.SuitSpades},
			{Rank: game.Rank8, Suit: game.SuitSpades},
			{Rank: game.RankQueen, Suit: game.SuitDiamonds},
			{Rank: game.Rank8, Suit: game.SuitClubs},
		}
		// Seat 0 leads with AH (trump, rank order 5)
		gs.ActivePlayerSeat = 0
		leadAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankAce, Suit: game.SuitHearts}}
		gs, err := game.ApplyAction(gs, leadAction)
		require.NoError(t, err)

		// Seat 1 has JH(7), 9H(6) which beat AH(5), plus 7H(0) which can't.
		// Playing 7H (lower trump) when higher trumps are available is illegal.
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
		// Seat 1 (team B) leads JS — opponent to seat 2 (team A)
		gs.ActivePlayerSeat = 1
		leadAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.RankJack, Suit: game.SuitSpades}}
		gs, err := game.ApplyAction(gs, leadAction)
		require.NoError(t, err)

		// Seat 2 (team A) is void in spades, opponent team B (seat 1) is winning.
		// Seat 2 has trump hearts (KH, QH, 8H, 7H) — must play trump.
		// Playing a non-trump card should be illegal.
		illegalAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 2,
			Card: &game.Card{Rank: game.RankAce, Suit: game.SuitDiamonds}}
		result, err := game.ApplyAction(gs, illegalAction)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, apperr.ErrIllegalPlay)
	})

	t.Run("void in led suit opponent winning - trump accepted", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		// Seat 1 (team B) leads JS
		gs.ActivePlayerSeat = 1
		leadAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.RankJack, Suit: game.SuitSpades}}
		gs, err := game.ApplyAction(gs, leadAction)
		require.NoError(t, err)

		// Seat 2 (team A) is void in spades, opponent winning — plays trump
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
		// Seat 0 leads spade
		leadAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankQueen, Suit: game.SuitSpades}}
		gs, err := game.ApplyAction(gs, leadAction)
		require.NoError(t, err)

		// Seat 1 plays trump 9H (rank order 6) — opponent trumping
		gs.Players[1].Hand = []game.Card{
			{Rank: game.Rank9, Suit: game.SuitHearts},
			{Rank: game.Rank8, Suit: game.SuitClubs},
		}
		followAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.Rank9, Suit: game.SuitHearts}}
		gs, err = game.ApplyAction(gs, followAction)
		require.NoError(t, err)

		// Seat 2 (team A) is void in spades, opponent (seat 1 team B) winning with 9H.
		// Seat 2 has KH (order 3), QH (order 2), 8H (order 1), 7H (order 0)
		// None can over-trump 9H (order 6), so any trump is legal.
		lowerTrumpAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 2,
			Card: &game.Card{Rank: game.RankKing, Suit: game.SuitHearts}}
		result, err := game.ApplyAction(gs, lowerTrumpAction)

		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("lower trump rejected when higher available - obligation path with trump in trick", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		// Set up: seat 0 leads spade, seat 1 (void in spades) trumps with 8H,
		// seat 2 (void in spades) must over-trump 8H.
		// Give seat 1 no spades, with 8H as its only trump (taken from seat 2).
		gs.Players[1].Hand = []game.Card{
			{Rank: game.Rank8, Suit: game.SuitHearts}, // trump order 1
			{Rank: game.Rank8, Suit: game.SuitClubs},
			{Rank: game.Rank9, Suit: game.SuitClubs},
			{Rank: game.RankQueen, Suit: game.SuitDiamonds},
			{Rank: game.RankJack, Suit: game.SuitDiamonds},
			{Rank: game.Rank9, Suit: game.SuitDiamonds},
			{Rank: game.Rank8, Suit: game.SuitDiamonds},
			{Rank: game.Rank7, Suit: game.SuitDiamonds},
		}
		// Give seat 2 trumps KH(3), QH(2), 7H(0) — no 8H (seat 1 has it)
		gs.Players[2].Hand = []game.Card{
			{Rank: game.RankKing, Suit: game.SuitHearts},  // trump order 3 — over-trumps
			{Rank: game.RankQueen, Suit: game.SuitHearts}, // trump order 2 — over-trumps
			{Rank: game.Rank7, Suit: game.SuitHearts},     // trump order 0 — CANNOT over-trump
			{Rank: game.RankAce, Suit: game.SuitDiamonds},
			{Rank: game.RankTen, Suit: game.SuitDiamonds},
			{Rank: game.RankAce, Suit: game.SuitClubs},
			{Rank: game.RankTen, Suit: game.SuitClubs},
			{Rank: game.Rank7, Suit: game.SuitClubs},
		}

		// Seat 0 leads QS
		leadAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankQueen, Suit: game.SuitSpades}}
		gs, err := game.ApplyAction(gs, leadAction)
		require.NoError(t, err)

		// Seat 1 (team B) void in spades, opponent (seat 0 team A) winning → must trump
		trumpAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.Rank8, Suit: game.SuitHearts}}
		gs, err = game.ApplyAction(gs, trumpAction)
		require.NoError(t, err)

		// Seat 2 (team A) void in spades, opponent team B (seat 1) winning with 8H (order 1).
		// Seat 2 has KH(3) and QH(2) which beat 8H, plus 7H(0) which can't.
		// Playing 7H should be illegal since higher over-trumps are available.
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
		// Seat 0 (team A) leads with AS (highest non-trump spade)
		leadAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankAce, Suit: game.SuitSpades}}
		gs, err := game.ApplyAction(gs, leadAction)
		require.NoError(t, err)

		// Seat 1 (team B) follows
		followAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.Rank7, Suit: game.SuitSpades}}
		gs, err = game.ApplyAction(gs, followAction)
		require.NoError(t, err)

		// Seat 2 (team A) — partner seat 0 is winning with AS.
		// Seat 2 is void in spades and holds trump hearts (KH, QH, 8H, 7H).
		// Non-trump AD is illegal — must cut.
		nonTrumpAttempt := game.Action{Type: game.ActionPlayCard, PlayerSeat: 2,
			Card: &game.Card{Rank: game.RankAce, Suit: game.SuitDiamonds}}
		_, err = game.ApplyAction(gs, nonTrumpAttempt)
		assert.ErrorIs(t, err, apperr.ErrIllegalPlay,
			"non-trump play must be rejected: void in led suit with trump in hand")

		// Trump 7H is legal (any-trump fallback when no trump on table).
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
		// Seat 0 leads spade
		leadAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankAce, Suit: game.SuitSpades}}
		gs, err := game.ApplyAction(gs, leadAction)
		require.NoError(t, err)

		// Seat 1 follows spade
		followAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.RankJack, Suit: game.SuitSpades}}
		gs, err = game.ApplyAction(gs, followAction)
		require.NoError(t, err)

		// Seat 2 plays trump (team A partner winning, but that's fine)
		trumpAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 2,
			Card: &game.Card{Rank: game.RankKing, Suit: game.SuitHearts}}
		gs, err = game.ApplyAction(gs, trumpAction)
		require.NoError(t, err)

		// Seat 3 has no spades AND no trump hearts — any card is legal
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
		// Override seat 2's hand to have no trumps so the void-no-trump branch
		// applies: AD remains a legal play and seat 0 (AS) still wins the trick.
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
		// Play 4 cards: spades led, seat 2 void in spades and trumps → AD legal.
		cards := []struct {
			seat int
			card game.Card
		}{
			{0, game.Card{Rank: game.RankAce, Suit: game.SuitSpades}},   // 11 pts
			{1, game.Card{Rank: game.RankJack, Suit: game.SuitSpades}},  // 2 pts
			{2, game.Card{Rank: game.RankAce, Suit: game.SuitDiamonds}}, // void, no trump, any card
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

		// AS wins (highest non-trump of led suit) — seat 0 (team A)
		// Points: AS(11) + JS(2) + AD(11) + JD(2) = 26
		assert.Equal(t, 26, result.HandPoints[0]) // Team A
		assert.Equal(t, 0, result.HandPoints[1])  // Team B
		assert.Equal(t, 1, result.TricksWon[0])
		assert.Equal(t, 0, result.TricksWon[1])
		// Winner leads next trick
		assert.Equal(t, 0, result.ActivePlayerSeat)
		assert.Equal(t, 2, result.TrickNumber)
		// Trick cleared
		assert.Empty(t, result.CurrentTrick)
		assert.Nil(t, result.LeadSuit)
		assert.Nil(t, result.TrickWinnerSeat)
		// Phase stays playing
		assert.Equal(t, game.PhasePlaying, result.Phase)
	})

	t.Run("trump beats non-trump", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		// Seat 0 leads QS (non-trump)
		cards := []struct {
			seat int
			card game.Card
		}{
			{0, game.Card{Rank: game.RankQueen, Suit: game.SuitSpades}},
			{1, game.Card{Rank: game.Rank7, Suit: game.SuitSpades}},
			{2, game.Card{Rank: game.Rank7, Suit: game.SuitHearts}},   // trump — void in spades
			{3, game.Card{Rank: game.Rank7, Suit: game.SuitDiamonds}}, // any card
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

		// 7H (trump) wins — seat 2 (team A)
		assert.Equal(t, 2, result.ActivePlayerSeat) // winner leads next
		assert.Equal(t, 1, result.TricksWon[0])     // Team A
	})

	t.Run("higher trump beats lower trump", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		// Seat 0 leads QS
		cards := []struct {
			seat int
			card game.Card
		}{
			{0, game.Card{Rank: game.RankQueen, Suit: game.SuitSpades}},
			{1, game.Card{Rank: game.Rank9, Suit: game.SuitSpades}},
			{2, game.Card{Rank: game.Rank8, Suit: game.SuitHearts}}, // trump 8H
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

		// 8H (trump, order 1) wins — seat 2
		assert.Equal(t, 2, result.ActivePlayerSeat)
	})
}

// TestTrickPointCalculation tests card point accumulation.
func TestTrickPointCalculation(t *testing.T) {
	t.Run("correct points for mixed trick", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		// Trump is Hearts. Play: AS(11), 9S(0), KH(trump 4), 7D(0)
		cards := []struct {
			seat int
			card game.Card
		}{
			{0, game.Card{Rank: game.RankAce, Suit: game.SuitSpades}},
			{1, game.Card{Rank: game.Rank9, Suit: game.SuitSpades}},
			{2, game.Card{Rank: game.RankKing, Suit: game.SuitHearts}}, // trump K=4pts
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

		// KH (trump) wins — seat 2 (team A)
		// Points: AS=11(non-trump), 9S=0(non-trump), KH=4(trump), 7D=0(non-trump) = 15
		assert.Equal(t, 15, result.HandPoints[0]) // Team A gets all trick points
	})
}

// TestEighthTrickTransition tests phase transition after the 8th trick.
func TestEighthTrickTransition(t *testing.T) {
	t.Run("8th trick transitions to PhaseHandScoring", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(8)
		// Each player has 1 card. Play all 4.
		// Seat 0: AS, Seat 1: JS, Seat 2: AD, Seat 3: JD
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

		// After trick 8, scoring runs atomically — state holds in PhaseHandComplete
		// (next hand deals on action:continue) or, on a crossed target, PhaseMatchEnd.
		assert.Contains(t, []game.Phase{game.PhaseHandComplete, game.PhaseMatchEnd}, result.Phase,
			"should advance past PhaseHandScoring to PhaseHandComplete or PhaseMatchEnd")
	})
}

// TestPlayCardErrors tests error conditions.
func TestPlayCardErrors(t *testing.T) {
	t.Run("card not in hand returns ErrInvalidCard", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		// Seat 0 doesn't have 7D
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
		gs := testfixtures.NewGameMidPlay(1) // Active player is 0
		action := game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.RankJack, Suit: game.SuitSpades}}

		result, err := game.ApplyAction(gs, action)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, apperr.ErrNotYourTurn)
	})

	t.Run("play_card in non-playing phase returns ErrWrongPhase", func(t *testing.T) {
		gs := testfixtures.NewGameJustDealt() // bidding phase
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

		// Original state unchanged
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

		// Modifying result's pointer field should not affect original
		newSuit := game.SuitClubs
		result.LeadSuit = &newSuit
		// Original TrumpSuit should be unchanged
		assert.Equal(t, game.SuitHearts, *gs.TrumpSuit)
	})
}

// TestFull8TrickHand plays a complete 8-trick hand and verifies that the
// game progresses through scoring under the no-partner-exemption rule.
func TestFull8TrickHand(t *testing.T) {
	t.Run("all 32 cards played, hand ends with scoring", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)

		// Trick 1: seat 0 leads AS, seat 2 (void in spades, holds trumps) must
		// cut with lowest trump 7H — seat 2 wins (team A).
		trick1 := []struct {
			seat int
			card game.Card
		}{
			{0, game.Card{Rank: game.RankAce, Suit: game.SuitSpades}},
			{1, game.Card{Rank: game.Rank9, Suit: game.SuitSpades}},
			{2, game.Card{Rank: game.Rank7, Suit: game.SuitHearts}}, // void in S, must trump
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
		assert.Equal(t, 2, state.ActivePlayerSeat) // 7H trump wins → seat 2

		// Trick 2: seat 2 leads AC; everyone follows.
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
		assert.Equal(t, 2, state.ActivePlayerSeat) // AC wins → seat 2

		// Trick 3: seat 2 leads TC. Seat 0 void in clubs, holds trumps → must
		// cut with TH. Seat 1 void in clubs, must over-trump TH → 9H.
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
		assert.Equal(t, 1, state.ActivePlayerSeat) // 9H over-trumps TH → seat 1

		// Trick 4: seat 1 leads JH (top trump). Seat 2 has only lower trumps,
		// any-trump fallback → 8H. Seat 3 void trump → any (7D). Seat 0 must
		// follow trump, only AH, doesn't beat JH, any-trump fallback → AH.
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
		assert.Equal(t, 1, state.ActivePlayerSeat) // JH wins → seat 1

		// Trick 5: seat 1 leads QD. Seat 2 holds AD + TD — must follow and
		// overplay QD with TD (lowest overplay). Seat 3 follows D (low),
		// seat 0 must follow D and overplay → KD.
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
		assert.Equal(t, 2, state.ActivePlayerSeat) // QH trump wins → seat 2

		// Trick 6: seat 2 leads AD. Seat 3 follows D. Seats 0 and 1 are void
		// in diamonds and hold no trumps → any card.
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
		assert.Equal(t, 2, state.ActivePlayerSeat) // AD wins → seat 2

		// Trick 7: seat 2 leads KH (trump). All others void in trump and any-card.
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
		assert.Equal(t, 2, state.ActivePlayerSeat) // KH wins → seat 2

		// Trick 8: each player has one card.
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

		// After trick 8, scoring runs atomically — state holds in PhaseHandComplete
		// (the next hand deals on action:continue) or, if a team crossed the target,
		// PhaseMatchEnd. Scoring adds the last-trick bonus (+10) to HandPoints and
		// records TeamScores before the hold.
		assert.Contains(t, []game.Phase{game.PhaseHandComplete, game.PhaseMatchEnd}, state.Phase,
			"should advance past PhaseHandScoring to PhaseHandComplete or PhaseMatchEnd")
		totalTeamScores := state.TeamScores[0] + state.TeamScores[1]
		assert.Greater(t, totalTeamScores, 0, "TeamScores should reflect scored hand points (152 card pts + 10 last-trick bonus = 162)")

		if state.Phase == game.PhaseBidding {
			// New hand: each player gets 8 cards, HandPoints reset
			for i, p := range state.Players {
				assert.Len(t, p.Hand, 8, "player %d should have 8 cards after new deal", i)
			}
		}
	})
}
