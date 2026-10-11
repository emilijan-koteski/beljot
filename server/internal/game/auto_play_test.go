package game_test

import (
	"testing"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAutoPlay(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *game.GameState
		expected string
	}{
		{
			name: "leading with mixed hand selects first by suit then rank",
			setup: func() *game.GameState {
				gs := testfixtures.NewGameMidPlay(1)
				gs.ActivePlayerSeat = 0
				return gs
			},
			expected: "TS",
		},
		{
			name: "leading with single suit selects lowest rank",
			setup: func() *game.GameState {
				gs := testfixtures.NewGameJustDealt()
				gs.Phase = game.PhasePlaying
				trump := game.SuitHearts
				gs.TrumpSuit = &trump
				caller := 1
				gs.TrumpCallerSeat = &caller
				gs.DeclarationsResolved = true
				gs.BelotAnnounced = true
				gs.TrickNumber = 1
				gs.ActivePlayerSeat = 0
				return gs
			},
			expected: "7S",
		},
		{
			name: "must follow suit returns lowest legal card from led suit",
			setup: func() *game.GameState {
				gs := testfixtures.NewGameMidPlay(1)
				leadSuit := game.SuitSpades
				gs.LeadSuit = &leadSuit
				gs.CurrentTrick = []game.TrickCard{
					{Card: game.Card{Rank: game.RankAce, Suit: game.SuitSpades}, PlayerSeat: 0},
				}
				gs.ActivePlayerSeat = 1
				return gs
			},
			expected: "7S",
		},
		{
			name: "void in led suit and no trump plays lowest by suit then rank",
			setup: func() *game.GameState {
				gs := testfixtures.NewGameMidPlay(1)
				leadSuit := game.SuitSpades
				gs.LeadSuit = &leadSuit
				gs.CurrentTrick = []game.TrickCard{
					{Card: game.Card{Rank: game.RankAce, Suit: game.SuitSpades}, PlayerSeat: 0},
				}
				gs.ActivePlayerSeat = 3
				return gs
			},
			expected: "7D",
		},
		{
			name: "single card in hand returns that card",
			setup: func() *game.GameState {
				gs := testfixtures.NewGameMidPlay(8)
				gs.ActivePlayerSeat = 0
				return gs
			},
			expected: "AS",
		},
		{
			name: "seat 2 leading with mixed suits picks lowest of first suit",
			setup: func() *game.GameState {
				gs := testfixtures.NewGameMidPlay(1)
				gs.ActivePlayerSeat = 2
				return gs
			},
			expected: "7H",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gs := tt.setup()
			cardID, err := game.AutoPlay(gs)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, cardID)
		})
	}
}

func TestAutoPlay_ErrorOnEmptyHand(t *testing.T) {
	gs := testfixtures.NewGameMidPlay(1)
	gs.Players[0].Hand = []game.Card{}
	gs.ActivePlayerSeat = 0

	_, err := game.AutoPlay(gs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no legal cards")
}

// TestAutoPickTrumpSuit covers the picker used for an absent player who may not pass.
// It checks that only the six visible cards are scanned and that ties resolve deterministically.
func TestAutoPickTrumpSuit(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *game.GameState
		expected game.Suit
	}{
		{
			name: "forced dealer picks the longest visible suit",
			setup: func() *game.GameState {
				return testfixtures.NewGameCroatianMidBidding(3)
			},
			expected: game.SuitSpades,
		},
		{
			name: "face-down pair does not change the answer",
			setup: func() *game.GameState {
				gs := testfixtures.NewGameCroatianMidBidding(3)
				seat := gs.ActivePlayerSeat
				gs.Players[seat].Hand = cardsFrom("7S", "8S", "7H", "8H", "9H", "7D")
				gs.Players[seat].FaceDownCards = cardsFrom("9S", "TS")
				return gs
			},
			expected: game.SuitHearts,
		},
		{
			name: "ties break on suit order",
			setup: func() *game.GameState {
				gs := testfixtures.NewGameCroatianMidBidding(3)
				seat := gs.ActivePlayerSeat
				gs.Players[seat].Hand = cardsFrom("7H", "8H", "9H", "7S", "8S", "9S")
				gs.Players[seat].FaceDownCards = cardsFrom("TS", "TH")
				return gs
			},
			expected: game.SuitSpades,
		},
		{
			name: "longer suit beats suit order",
			setup: func() *game.GameState {
				gs := testfixtures.NewGameCroatianMidBidding(3)
				seat := gs.ActivePlayerSeat
				gs.Players[seat].Hand = cardsFrom("7H", "8H", "9H", "TH", "7S", "8S")
				gs.Players[seat].FaceDownCards = cardsFrom("9S", "TS")
				return gs
			},
			expected: game.SuitHearts,
		},
		{
			name: "a candidate's own suit is never named",
			setup: func() *game.GameState {
				gs := testfixtures.NewGameCroatianMidBidding(3)
				seat := gs.ActivePlayerSeat
				gs.Players[seat].Hand = cardsFrom("7S", "8S", "9S", "TS", "7C", "8C")
				gs.Players[seat].FaceDownCards = cardsFrom("9C", "TC")
				candidate := card("JS")
				gs.TrumpCandidate = &candidate
				return gs
			},
			expected: game.SuitClubs,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gs := tt.setup()
			suit, err := game.AutoPickTrumpSuit(gs, gs.ActivePlayerSeat)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, suit)
		})
	}
}

// TestAutoPickTrumpSuit_AlwaysAccepted closes the loop the helper exists for:
// whatever it names must survive the very action the engine was rejecting.
func TestAutoPickTrumpSuit_AlwaysAccepted(t *testing.T) {
	gs := testfixtures.NewGameCroatianMidBidding(3)
	require.True(t, game.MustPickTrump(gs, gs.ActivePlayerSeat), "fixture must be the forced-pick state")
	require.Equal(t, gs.DealerSeat, gs.ActivePlayerSeat)

	suit, err := game.AutoPickTrumpSuit(gs, gs.ActivePlayerSeat)
	require.NoError(t, err)

	next, err := game.ApplyAction(gs, game.Action{
		Type:       game.ActionPickTrump,
		PlayerSeat: gs.ActivePlayerSeat,
		Suit:       &suit,
	})
	require.NoError(t, err, "the auto-picked suit must be accepted, not rejected")
	require.NotNil(t, next.TrumpSuit)
	assert.Equal(t, suit, *next.TrumpSuit)
}

func TestAutoPickTrumpSuit_ErrorOnEmptyHolding(t *testing.T) {
	gs := testfixtures.NewGameCroatianMidBidding(3)
	seat := gs.ActivePlayerSeat
	gs.Players[seat].Hand = []game.Card{}
	gs.Players[seat].FaceDownCards = nil

	_, err := game.AutoPickTrumpSuit(gs, gs.ActivePlayerSeat)
	assert.Error(t, err)
}

// card parses a card ID, panicking on invalid input.
func card(id string) game.Card {
	c, err := game.ParseCard(id)
	if err != nil {
		panic(err)
	}
	return c
}

func cardsFrom(ids ...string) []game.Card {
	out := make([]game.Card, len(ids))
	for i, id := range ids {
		out[i] = card(id)
	}
	return out
}
