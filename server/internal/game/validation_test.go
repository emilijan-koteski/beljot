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
		// Set seat 2's hand to only trump cards
		gs.Players[2].Hand = []game.Card{
			{Rank: game.RankKing, Suit: game.SuitHearts},
			{Rank: game.RankQueen, Suit: game.SuitHearts},
			{Rank: game.Rank8, Suit: game.SuitHearts},
			{Rank: game.Rank7, Suit: game.SuitHearts},
		}

		// Seat 1 (team B) leads with JS — opponent to seat 2
		gs.ActivePlayerSeat = 1
		leadAction := game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.RankJack, Suit: game.SuitSpades}}
		gs, err := game.ApplyAction(gs, leadAction)
		require.NoError(t, err)

		// Seat 2 is void in spades, opponent winning, has only trump.
		// No trump in trick yet, so no over-trump obligation — any trump is legal.
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
		gs := testfixtures.NewGameMidPlay(8) // 1 card per player
		// Seat 0 has only AS
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
		// Play 3 cards: seat 0 leads spade, seat 1 follows, seat 2 trumps
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

		// Seat 3 has 4 diamonds and 4 clubs, no spades.
		// Seat 2 (team A, partner of seat 0) played trump KH and is winning.
		// Seat 3 is team B — partner exemption does NOT apply (opponent winning).
		// Actually seat 2 played trump KH — who is winning?
		// Cards: AS(seat 0, rank 7 non-trump), JS(seat 1, rank 3 non-trump), KH(seat 2, trump rank 3)
		// Trump beats non-trump — seat 2 (team A) is winning.
		// Seat 3 is team B — opponent (team A) is winning.
		// Seat 3 has no spades AND no trump — any card is legal.
		anyCard := game.Action{Type: game.ActionPlayCard, PlayerSeat: 3,
			Card: &game.Card{Rank: game.RankKing, Suit: game.SuitClubs}}
		result, err := game.ApplyAction(state, anyCard)

		require.NoError(t, err)
		require.NotNil(t, result)
	})
}

// TestValidationPartnerTrumped tests that the must-overplay rule still applies
// when partner played the highest card so far — Bitola has no partner-winning
// exemption for void-in-led-suit either (covered separately).
func TestValidationPartnerTrumped(t *testing.T) {
	t.Run("partner trumped and is winning - has led suit, must follow", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		// Seat 1 (team B) leads spade, Seat 2 (team A) trumps (partner to seat 0)
		gs.ActivePlayerSeat = 1
		plays := []struct {
			seat int
			card game.Card
		}{
			{1, game.Card{Rank: game.RankJack, Suit: game.SuitSpades}},
			{2, game.Card{Rank: game.RankKing, Suit: game.SuitHearts}}, // trump, wins
			{3, game.Card{Rank: game.Rank7, Suit: game.SuitDiamonds}},  // any card (void, no trump)
		}

		state := gs
		for _, p := range plays {
			action := game.Action{Type: game.ActionPlayCard, PlayerSeat: p.seat, Card: &p.card}
			var err error
			state, err = game.ApplyAction(state, action)
			require.NoError(t, err)
		}

		// Seat 0 (team A) — partner seat 2 trumped and is winning.
		// Spade was led. Seat 0 has spades; must follow suit regardless of partner state.
		validSpade := game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankAce, Suit: game.SuitSpades}}
		result, err := game.ApplyAction(state, validSpade)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Playing a non-spade should be illegal (has spades)
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

// TestValidationMustOverplayLedSuit tests the Bitola variant rule that a player
// holding led-suit cards must play one strictly higher than the highest
// led-suit card already on the table — applies whether the led suit is trump
// or not.
func TestValidationMustOverplayLedSuit(t *testing.T) {
	t.Run("must overplay led non-trump suit when higher card available", func(t *testing.T) {
		// Trump = Hearts. Seat 0 leads KS. Seat 1's hand: 8S, AS, plus padding.
		// Highest spade on table = KS (NonTrumpRankOrder K=5).
		// Seat 1's spades: 8S(1), AS(7). Only AS overplays → 8S must be illegal.
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

		// 8S is below KS — must be rejected.
		illegalLow := game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.Rank8, Suit: game.SuitSpades}}
		_, err = game.ApplyAction(gs, illegalLow)
		assert.ErrorIs(t, err, apperr.ErrIllegalPlay)

		// AS overplays KS — legal.
		legalHigh := game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.RankAce, Suit: game.SuitSpades}}
		result, err := game.ApplyAction(gs, legalHigh)
		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("falls back to any same-suit card when no overplay exists", func(t *testing.T) {
		// Trump = Hearts. Seat 0 leads AS (top non-trump). Seat 1 has only 7S, 8S.
		// No spade overplays AS → any spade is legal (rule 2).
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
		// Trump = Hearts. Seat 0 leads KS. Seat 1 follows 7S. Seat 2 (void in
		// spades) cuts with 7H. Seat 3's turn — holds 8S, AS, a trump (9H) and
		// clubs. Once the trick is cut by a trump the led suit can no longer
		// win, so seat 3 may follow with ANY spade (8S, the low card, is now
		// legal) — but must still follow suit: the off-suit clubs and even the
		// trump 9H stay illegal because seat 3 holds the led suit.
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
				{Rank: game.Rank9, Suit: game.SuitHearts}, // trump — still illegal: must follow spades
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

		// Both the low spade and the high spade are now legal follows.
		for _, legalRank := range []game.Rank{game.Rank8, game.RankAce} {
			result, err := game.ApplyAction(setup(), game.Action{Type: game.ActionPlayCard, PlayerSeat: 3,
				Card: &game.Card{Rank: legalRank, Suit: game.SuitSpades}})
			require.NoError(t, err, "%s of S must be legal once the trick is cut by a trump", legalRank)
			require.NotNil(t, result)
		}

		// Off-suit clubs remain illegal — following suit is still mandatory.
		for _, illegalRank := range []game.Rank{game.RankKing, game.RankQueen} {
			_, err := game.ApplyAction(setup(), game.Action{Type: game.ActionPlayCard, PlayerSeat: 3,
				Card: &game.Card{Rank: illegalRank, Suit: game.SuitClubs}})
			assert.ErrorIs(t, err, apperr.ErrIllegalPlay, "%s of C must be illegal: seat 3 still holds spades", illegalRank)
		}

		// A trump in hand is NOT a legal alternative — holding the led suit
		// forces a follow even after the trick has been cut.
		_, err := game.ApplyAction(setup(), game.Action{Type: game.ActionPlayCard, PlayerSeat: 3,
			Card: &game.Card{Rank: game.Rank9, Suit: game.SuitHearts}})
		assert.ErrorIs(t, err, apperr.ErrIllegalPlay, "9H (trump) must be illegal: seat 3 still holds spades")
	})

	t.Run("multiple higher led-suit cards — all strictly higher are legal", func(t *testing.T) {
		// Trump = Hearts. Seat 0 leads QS (non-trump rank 4). Seat 1 has TS,
		// KS, AS (ranks 6, 5, 7), and 7C. All three spades overplay QS — each
		// must be a legal play (rule "all strictly higher" — not just the max).
		// Run each candidate through a fresh fixture to avoid state aliasing.
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
		// Trump = Hearts. Seat 2 (partner of seat 0) leads KS (rank 5).
		// Seat 3 plays 7S (rank 0). Seat 0's turn — has 8S, AS. Partner
		// exemption applies only to void-in-led-suit (rule 3). Since seat 0
		// has spades, the overplay rule still fires → 8S must be rejected,
		// AS must be legal.
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

		// Seat 2 leads KS.
		gs, err := game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 2,
			Card: &game.Card{Rank: game.RankKing, Suit: game.SuitSpades}})
		require.NoError(t, err)
		// Seat 3 follows with 7S (no overplay possible from seat 3's hand).
		gs, err = game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 3,
			Card: &game.Card{Rank: game.Rank7, Suit: game.SuitSpades}})
		require.NoError(t, err)

		// Seat 0: 8S below KS — illegal even though partner played the high card.
		_, err = game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.Rank8, Suit: game.SuitSpades}})
		assert.ErrorIs(t, err, apperr.ErrIllegalPlay)

		// Seat 0: AS overplays — legal.
		result, err := game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankAce, Suit: game.SuitSpades}})
		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("img_two: void with trump, partner already trumped — must over-trump", func(t *testing.T) {
		// Trump = Hearts. Trick: cvetanka leads 9C, emilijan trumps QH, irena
		// follows 7C, kiro to play. Kiro is void in clubs and holds 9H + TH
		// (both higher than QH(2)). Diamonds in hand must be illegal; only the
		// over-trumps are legal. Seats CCW: 1 → 2 → 3 → 0 — kiro is seat 0
		// playing fourth.
		setup := func() *game.GameState {
			gs := testfixtures.NewGameMidPlay(1)
			gs.ActivePlayerSeat = 1
			gs.Players[0].Hand = []game.Card{
				{Rank: game.Rank9, Suit: game.SuitHearts},   // trump order 6
				{Rank: game.RankTen, Suit: game.SuitHearts}, // trump order 4
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
				{Rank: game.RankQueen, Suit: game.SuitHearts}, // trump order 2
				{Rank: game.RankAce, Suit: game.SuitDiamonds},
			}
			gs.Players[3].Hand = []game.Card{
				{Rank: game.Rank7, Suit: game.SuitClubs}, // only club
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

		// Each diamond must be rejected.
		for _, illegalRank := range []game.Rank{game.Rank8, game.RankTen, game.RankJack, game.RankQueen, game.RankAce} {
			_, err := game.ApplyAction(setup(), game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
				Card: &game.Card{Rank: illegalRank, Suit: game.SuitDiamonds}})
			assert.ErrorIs(t, err, apperr.ErrIllegalPlay, "%s of D must be illegal: void in clubs with over-trump available", illegalRank)
		}

		// Each over-trump (9H, TH) must be accepted.
		for _, legalRank := range []game.Rank{game.Rank9, game.RankTen} {
			result, err := game.ApplyAction(setup(), game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
				Card: &game.Card{Rank: legalRank, Suit: game.SuitHearts}})
			require.NoError(t, err, "%s of H must over-trump QH", legalRank)
			require.NotNil(t, result)
		}
	})

	t.Run("img_three: void in non-trump, partner winning, has one trump — only that trump", func(t *testing.T) {
		// Trump = Spades. Trick: 9C, TC, AC by three players. Emilijan to play
		// (seat 0), holds 8S (only trump) plus J♦ K♦. No trump on table; partner
		// is currently winning AC. Under new rule, the single trump 8S must be
		// the only legal play; diamonds are illegal.
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

		// JD/KD illegal — must trump.
		_, err = game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankJack, Suit: game.SuitDiamonds}})
		assert.ErrorIs(t, err, apperr.ErrIllegalPlay, "JD must be illegal: void in clubs, has 8S trump")
		_, err = game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankKing, Suit: game.SuitDiamonds}})
		assert.ErrorIs(t, err, apperr.ErrIllegalPlay, "KD must be illegal: void in clubs, has 8S trump")

		// 8S legal — only trump in hand.
		result, err := game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.Rank8, Suit: game.SuitSpades}})
		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("img_four: void in non-trump, no trump on table yet — only trumps legal", func(t *testing.T) {
		// Trump = Hearts. Trick: seat 1 leads 7S, seat 2 follows 9S (overplay),
		// seat 3 has no spades and no trumps → any card. Then seat 0 (kiro)
		// plays — void in spades, holds multiple hearts plus diamonds. Diamonds
		// must be illegal; every trump must be legal (no over-trump constraint
		// because no trump is on the table yet).
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
		// Sanity branch 2b: when player is void in led suit AND holds no trump,
		// the original "any card" rule still applies.
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

		// Seat 0 leads diamonds wait — seat 0 has no diamonds. Reset: seat 1 leads.
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

		// Seat 0 void in diamonds, no hearts (no trump). Spade or Club legal.
		result, err := game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankKing, Suit: game.SuitClubs}})
		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("trump-led over-trump regression", func(t *testing.T) {
		// Trump = Hearts. Seat 0 leads with QH (trump rank 2). Seat 1 has trump
		// JH (rank 7) and 9H (rank 6), both overplays. Seat 1's hand also has
		// non-trumps and a low trump 7H (rank 0) — but only the overplays
		// are legal under the unified rule.
		gs := testfixtures.NewGameMidPlay(1)
		gs.Players[0].Hand = []game.Card{
			{Rank: game.RankQueen, Suit: game.SuitHearts}, // trump lead
			{Rank: game.RankAce, Suit: game.SuitSpades},
			{Rank: game.RankTen, Suit: game.SuitSpades},
			{Rank: game.RankKing, Suit: game.SuitSpades},
			{Rank: game.RankAce, Suit: game.SuitClubs},
		}
		gs.Players[1].Hand = []game.Card{
			{Rank: game.RankJack, Suit: game.SuitHearts}, // trump 7
			{Rank: game.Rank9, Suit: game.SuitHearts},    // trump 6
			{Rank: game.Rank7, Suit: game.SuitHearts},    // trump 0
			{Rank: game.Rank8, Suit: game.SuitClubs},
		}

		gs, err := game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 0,
			Card: &game.Card{Rank: game.RankQueen, Suit: game.SuitHearts}})
		require.NoError(t, err)

		// 7H is trump but does not over-trump QH — illegal.
		_, err = game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.Rank7, Suit: game.SuitHearts}})
		assert.ErrorIs(t, err, apperr.ErrIllegalPlay)

		// JH over-trumps QH — legal.
		result, err := game.ApplyAction(gs, game.Action{Type: game.ActionPlayCard, PlayerSeat: 1,
			Card: &game.Card{Rank: game.RankJack, Suit: game.SuitHearts}})
		require.NoError(t, err)
		require.NotNil(t, result)
	})
}
