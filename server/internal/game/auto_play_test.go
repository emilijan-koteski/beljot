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
				// Seat 0 leads: AS TS KS QS AH TH KD 7C
				// Sorted by suit (S<H<D<C) then rank (7<8<9<T<J<Q<K<A)
				// Spades first, lowest rank: T(3) < Q(5) < K(6) < A(7) → TS
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
				// Seat 0 has 5 spades from stage-1: 7S 8S 9S TS JS
				// Sorted: same suit S, rank order 7<8<9<T<J → pick 7S
				gs.ActivePlayerSeat = 0
				return gs
			},
			expected: "7S",
		},
		{
			name: "must follow suit returns lowest legal card from led suit",
			setup: func() *game.GameState {
				gs := testfixtures.NewGameMidPlay(1)
				// Seat 1 hand: JS 9S 8S 7S JH 9H QD 8C
				leadSuit := game.SuitSpades
				gs.LeadSuit = &leadSuit
				gs.CurrentTrick = []game.TrickCard{
					{Card: game.Card{Rank: game.RankAce, Suit: game.SuitSpades}, PlayerSeat: 0},
				}
				gs.ActivePlayerSeat = 1
				// Legal: JS 9S 8S 7S (must follow spades)
				// Sorted by rank: 7S first
				return gs
			},
			expected: "7S",
		},
		{
			name: "void in led suit and no trump plays lowest by suit then rank",
			setup: func() *game.GameState {
				gs := testfixtures.NewGameMidPlay(1)
				// Seat 3 has no Spades and no Hearts(trump): JD 9D 8D 7D KC QC JC 9C
				leadSuit := game.SuitSpades
				gs.LeadSuit = &leadSuit
				gs.CurrentTrick = []game.TrickCard{
					{Card: game.Card{Rank: game.RankAce, Suit: game.SuitSpades}, PlayerSeat: 0},
				}
				gs.ActivePlayerSeat = 3
				// No spades, no trump → any card legal
				// Sorted: D(7D 8D 9D JD) then C(9C JC QC KC) → 7D first
				return gs
			},
			expected: "7D",
		},
		{
			name: "single card in hand returns that card",
			setup: func() *game.GameState {
				gs := testfixtures.NewGameMidPlay(8)
				// Seat 0 at trick 8: only 1 card left (AS)
				gs.ActivePlayerSeat = 0
				return gs
			},
			expected: "AS",
		},
		{
			name: "seat 2 leading with mixed suits picks lowest of first suit",
			setup: func() *game.GameState {
				gs := testfixtures.NewGameMidPlay(1)
				// Seat 2: AD TD KH QH 8H 7H AC TC
				// Leading: all cards legal
				// Sorted: H(7H 8H QH KH) then D(TD AD) then C(TC AC)
				// First: 7H
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

// TestAutoPickTrumpSuit covers the picker the session manager calls for an
// absent player who is forbidden to pass. The cases that matter are the two the
// helper exists for: the face-down pair changing the answer at round 2, and a
// tie resolving deterministically.
func TestAutoPickTrumpSuit(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *game.GameState
		expected game.Suit
	}{
		{
			// Croatian round 2, the forced-pick state. Seat 0's OPEN six are
			// 7S 8S 9S TS 7H 8H — four spades against two hearts — and its
			// face-down pair is JS QS, so spades wins either way. The baseline
			// for the case below.
			name: "croatian round 2 picks the longest suit",
			setup: func() *game.GameState {
				return testfixtures.NewGameCroatianMidBidding(7)
			},
			expected: game.SuitSpades,
		},
		{
			// The whole reason this helper is not a one-liner: the seat's open
			// hand says hearts 3, spades 2, but the face-down pair is two more
			// spades. Reading Hand alone would name hearts — a pick from six of
			// the eight cards the player actually holds.
			name: "face-down pair changes the answer",
			setup: func() *game.GameState {
				gs := testfixtures.NewGameCroatianMidBidding(7)
				seat := gs.ActivePlayerSeat
				gs.Players[seat].Hand = cardsFrom("7S", "8S", "7H", "8H", "9H", "7D")
				gs.Players[seat].FaceDownCards = cardsFrom("9S", "TS")
				return gs
			},
			expected: game.SuitSpades,
		},
		{
			// Equal length: spades and hearts both 4. suitOrder puts S before H,
			// so the answer never depends on hand order or map iteration.
			name: "ties break on suit order",
			setup: func() *game.GameState {
				gs := testfixtures.NewGameCroatianMidBidding(7)
				seat := gs.ActivePlayerSeat
				gs.Players[seat].Hand = cardsFrom("7H", "8H", "9H", "TH", "7S", "8S")
				gs.Players[seat].FaceDownCards = cardsFrom("9S", "TS")
				return gs
			},
			expected: game.SuitSpades,
		},
		{
			// Same holding with hearts one longer — proving the tie case above
			// is decided by suit order and not by hearts being unreachable.
			name: "longer suit beats suit order",
			setup: func() *game.GameState {
				gs := testfixtures.NewGameCroatianMidBidding(7)
				seat := gs.ActivePlayerSeat
				gs.Players[seat].Hand = cardsFrom("7H", "8H", "9H", "TH", "JH", "7S")
				gs.Players[seat].FaceDownCards = cardsFrom("8S", "9S")
				return gs
			},
			expected: game.SuitHearts,
		},
		{
			// Defence in depth, not a reachable Croatian state: the only config
			// that forces a pick deals no candidate. handlePickTrump rejects the
			// candidate's own suit as already spent, so naming spades here would
			// swap one rejected auto-action for another — clubs is the longest
			// suit that is actually pickable.
			name: "a candidate's own suit is never named",
			setup: func() *game.GameState {
				gs := testfixtures.NewGameCroatianMidBidding(7)
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
	gs := testfixtures.NewGameCroatianMidBidding(7)
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
	gs := testfixtures.NewGameCroatianMidBidding(7)
	seat := gs.ActivePlayerSeat
	gs.Players[seat].Hand = []game.Card{}
	gs.Players[seat].FaceDownCards = nil

	_, err := game.AutoPickTrumpSuit(gs, gs.ActivePlayerSeat)
	assert.Error(t, err)
}

// card / cardsFrom keep the table above readable. game_test has no shared card
// helper, so these stay local to this file.
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
