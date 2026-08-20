package game_test

import (
	"encoding/json"
	"testing"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPickTrumpRound1(t *testing.T) {
	tests := []struct {
		name          string
		passCount     int
		activeSeat    int
		expectedTrump game.Suit
	}{
		{
			name:          "first bidder picks immediately",
			passCount:     0,
			activeSeat:    1,
			expectedTrump: game.SuitHearts, // trump candidate is 7H
		},
		{
			name:          "second bidder picks after 1 pass",
			passCount:     1,
			activeSeat:    2,
			expectedTrump: game.SuitHearts,
		},
		{
			name:          "third bidder picks after 2 passes",
			passCount:     2,
			activeSeat:    3,
			expectedTrump: game.SuitHearts,
		},
		{
			name:          "fourth bidder picks after 3 passes",
			passCount:     3,
			activeSeat:    0,
			expectedTrump: game.SuitHearts,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gs := testfixtures.NewGameMidBidding(tc.passCount)
			action := game.Action{
				Type:       game.ActionPickTrump,
				PlayerSeat: tc.activeSeat,
			}

			result, err := game.ApplyAction(gs, action)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, game.PhasePlaying, result.Phase)
			require.NotNil(t, result.TrumpSuit)
			assert.Equal(t, tc.expectedTrump, *result.TrumpSuit)
			require.NotNil(t, result.TrumpCallerSeat)
			assert.Equal(t, tc.activeSeat, *result.TrumpCallerSeat)
			assert.Equal(t, 1, result.ActivePlayerSeat) // (DealerSeat+1)%4 = 1
			assert.Equal(t, 1, result.TrickNumber)
			assert.Empty(t, result.CurrentTrick)
		})
	}
}

func TestPassTrumpSequence(t *testing.T) {
	tests := []struct {
		name               string
		passCount          int
		activeSeat         int
		expectedPassCount  int
		expectedActiveSeat int
		expectedRound      int
	}{
		{
			name:               "first pass in round 1",
			passCount:          0,
			activeSeat:         1,
			expectedPassCount:  1,
			expectedActiveSeat: 2,
			expectedRound:      1,
		},
		{
			name:               "second pass in round 1",
			passCount:          1,
			activeSeat:         2,
			expectedPassCount:  2,
			expectedActiveSeat: 3,
			expectedRound:      1,
		},
		{
			name:               "third pass in round 1",
			passCount:          2,
			activeSeat:         3,
			expectedPassCount:  3,
			expectedActiveSeat: 0,
			expectedRound:      1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gs := testfixtures.NewGameMidBidding(tc.passCount)
			action := game.Action{
				Type:       game.ActionPassTrump,
				PlayerSeat: tc.activeSeat,
			}

			result, err := game.ApplyAction(gs, action)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, game.PhaseBidding, result.Phase)
			assert.Equal(t, tc.expectedPassCount, result.BiddingPassCount)
			assert.Equal(t, tc.expectedActiveSeat, result.ActivePlayerSeat)
			assert.Equal(t, tc.expectedRound, result.BiddingRound)
			assert.Nil(t, result.TrumpSuit, "trump should not be set during passing")
		})
	}
}

func TestRound1ToRound2Transition(t *testing.T) {
	// 3 passes already applied, 4th pass triggers round 2
	gs := testfixtures.NewGameMidBidding(3)
	action := game.Action{
		Type:       game.ActionPassTrump,
		PlayerSeat: 0, // seat 0 is the active bidder after 3 passes
	}

	result, err := game.ApplyAction(gs, action)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, game.PhaseBidding, result.Phase)
	assert.Equal(t, 2, result.BiddingRound, "should transition to round 2")
	assert.Equal(t, 0, result.BiddingPassCount, "pass count resets in round 2")
	assert.Equal(t, 1, result.ActivePlayerSeat, "bidding restarts from (DealerSeat+1)%4")
	assert.Nil(t, result.TrumpSuit, "trump should not be set")
}

func TestPickTrumpRound2(t *testing.T) {
	tests := []struct {
		name          string
		passCount     int
		activeSeat    int
		chosenSuit    game.Suit
		expectedTrump game.Suit
	}{
		{
			name:          "pick spades in round 2",
			passCount:     4,
			activeSeat:    1,
			chosenSuit:    game.SuitSpades,
			expectedTrump: game.SuitSpades,
		},
		{
			name:          "pick diamonds in round 2 after 1 pass",
			passCount:     5,
			activeSeat:    2,
			chosenSuit:    game.SuitDiamonds,
			expectedTrump: game.SuitDiamonds,
		},
		{
			name:          "pick clubs in round 2 after 2 passes",
			passCount:     6,
			activeSeat:    3,
			chosenSuit:    game.SuitClubs,
			expectedTrump: game.SuitClubs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gs := testfixtures.NewGameMidBidding(tc.passCount)
			suit := tc.chosenSuit
			action := game.Action{
				Type:       game.ActionPickTrump,
				PlayerSeat: tc.activeSeat,
				Suit:       &suit,
			}

			result, err := game.ApplyAction(gs, action)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, game.PhasePlaying, result.Phase)
			require.NotNil(t, result.TrumpSuit)
			assert.Equal(t, tc.expectedTrump, *result.TrumpSuit)
			require.NotNil(t, result.TrumpCallerSeat)
			assert.Equal(t, tc.activeSeat, *result.TrumpCallerSeat)
			assert.Equal(t, 1, result.ActivePlayerSeat)
			assert.Equal(t, 1, result.TrickNumber)
			assert.Empty(t, result.CurrentTrick)
		})
	}
}

func TestRound2PickWithoutSuit(t *testing.T) {
	gs := testfixtures.NewGameMidBidding(4) // round 2
	action := game.Action{
		Type:       game.ActionPickTrump,
		PlayerSeat: 1, // active bidder
		// Suit is nil — should fail
	}

	result, err := game.ApplyAction(gs, action)

	assert.Nil(t, result)
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrInvalidBid)
}

// TestRound2PickCandidateSuitRejected locks the Bitola round-2 rule that the
// originally face-up candidate's suit is "spent" — picking it in round 2 must
// be rejected with ErrInvalidBid, regardless of which seat is the active
// bidder.
func TestRound2PickCandidateSuitRejected(t *testing.T) {
	tests := []struct {
		name       string
		passCount  int
		activeSeat int
	}{
		{name: "round 2 just started, seat 1 active", passCount: 4, activeSeat: 1},
		{name: "round 2 with 1 pass, seat 2 active", passCount: 5, activeSeat: 2},
		{name: "round 2 with 2 passes, seat 3 active", passCount: 6, activeSeat: 3},
		{name: "round 2 with 3 passes, seat 0 active", passCount: 7, activeSeat: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gs := testfixtures.NewGameMidBidding(tc.passCount)
			require.NotNil(t, gs.TrumpCandidate, "fixture must have a face-up candidate")
			lockedSuit := gs.TrumpCandidate.Suit

			// Snapshot key state before the rejected action to assert immutability.
			origPhase := gs.Phase
			origRound := gs.BiddingRound
			origPassCount := gs.BiddingPassCount
			origActiveSeat := gs.ActivePlayerSeat
			origCandidate := *gs.TrumpCandidate

			action := game.Action{
				Type:       game.ActionPickTrump,
				PlayerSeat: tc.activeSeat,
				Suit:       &lockedSuit,
			}

			result, err := game.ApplyAction(gs, action)

			assert.Nil(t, result)
			require.Error(t, err)
			assert.ErrorIs(t, err, apperr.ErrInvalidBid)

			// Original state must not be mutated by a rejected pick.
			assert.Equal(t, origPhase, gs.Phase)
			assert.Equal(t, origRound, gs.BiddingRound)
			assert.Equal(t, origPassCount, gs.BiddingPassCount)
			assert.Equal(t, origActiveSeat, gs.ActivePlayerSeat)
			assert.Nil(t, gs.TrumpSuit, "trump must remain unset after rejected pick")
			require.NotNil(t, gs.TrumpCandidate, "candidate must still be present after rejected pick")
			assert.Equal(t, origCandidate, *gs.TrumpCandidate)
		})
	}
}

func TestRound2FullPassReshuffle(t *testing.T) {
	// 7 passes applied (round 2 with 3 passes), 8th pass triggers reshuffle
	gs := testfixtures.NewGameMidBidding(7)
	action := game.Action{
		Type:       game.ActionPassTrump,
		PlayerSeat: 0, // seat 0 is active after 7 passes
	}

	result, err := game.ApplyAction(gs, action)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, game.PhaseDealing, result.Phase, "reshuffle transitions to dealing phase")
	assert.Equal(t, 1, result.BiddingRound, "round resets to 1 after reshuffle")
	assert.Equal(t, 0, result.BiddingPassCount, "pass count resets after reshuffle")
	assert.Equal(t, 1, result.DealerSeat, "dealer rotates from 0 to 1")
	assert.Equal(t, 2, result.ActivePlayerSeat, "(new dealer + 1) % 4 = 2")
	assert.Nil(t, result.TrumpSuit, "trump should be nil after reshuffle")
	assert.Nil(t, result.TrumpCallerSeat, "caller should be nil after reshuffle")
	require.NotNil(t, result.TrumpCandidate, "new trump candidate should be revealed")
	assert.Equal(t, 1, result.HandNumber, "hand number unchanged during reshuffle")

	// Stage-1 sizes after re-deal.
	for i, p := range result.Players {
		assert.Len(t, p.Hand, 5, "seat %d should have 5 cards after stage-1 re-deal", i)
	}
	assert.Len(t, result.Deck, 11, "Deck should hold 11 cards after re-deal")

	// Card conservation: same 32 cards across hands + Deck + candidate, with
	// no duplicates anywhere. Use the shared collectCards helper.
	assertCardsAreFullDeck(t, collectCards(result))
}

// assertCardsAreFullDeck validates that the given slice contains every Bitola
// card exactly once. Used to enforce card-conservation across stage-1, stage-2,
// and reshuffle.
func assertCardsAreFullDeck(t *testing.T, cards []game.Card) {
	t.Helper()
	require.Len(t, cards, 32, "expected exactly 32 cards across all locations")
	seen := make(map[string]bool, 32)
	for _, c := range cards {
		id := c.String()
		assert.False(t, seen[id], "duplicate card: %s", id)
		seen[id] = true
	}
	assert.Len(t, seen, 32, "all 32 cards must be unique")
}

func TestErrNotYourTurn(t *testing.T) {
	tests := []struct {
		name       string
		actionType string
		wrongSeat  int
	}{
		{
			name:       "pick_trump from wrong player",
			actionType: game.ActionPickTrump,
			wrongSeat:  0, // active bidder is seat 1
		},
		{
			name:       "pass_trump from wrong player",
			actionType: game.ActionPassTrump,
			wrongSeat:  2,
		},
		{
			name:       "pick_trump from seat 3",
			actionType: game.ActionPickTrump,
			wrongSeat:  3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gs := testfixtures.NewGameJustDealt()
			action := game.Action{
				Type:       tc.actionType,
				PlayerSeat: tc.wrongSeat,
			}

			result, err := game.ApplyAction(gs, action)

			assert.Nil(t, result)
			require.Error(t, err)
			assert.ErrorIs(t, err, apperr.ErrNotYourTurn)
		})
	}
}

func TestErrWrongPhase(t *testing.T) {
	tests := []struct {
		name       string
		phase      game.Phase
		actionType string
	}{
		{
			name:       "pick_trump in playing phase",
			phase:      game.PhasePlaying,
			actionType: game.ActionPickTrump,
		},
		{
			name:       "pass_trump in playing phase",
			phase:      game.PhasePlaying,
			actionType: game.ActionPassTrump,
		},
		{
			name:       "pick_trump in match_end phase",
			phase:      game.PhaseMatchEnd,
			actionType: game.ActionPickTrump,
		},
		{
			name:       "play_card in bidding phase",
			phase:      game.PhaseBidding,
			actionType: game.ActionPlayCard,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gs := testfixtures.NewGameJustDealt()
			gs.Phase = tc.phase
			action := game.Action{
				Type:       tc.actionType,
				PlayerSeat: gs.ActivePlayerSeat,
			}

			result, err := game.ApplyAction(gs, action)

			assert.Nil(t, result)
			require.Error(t, err)
			assert.ErrorIs(t, err, apperr.ErrWrongPhase)
		})
	}
}

func TestStateImmutability(t *testing.T) {
	tests := []struct {
		name   string
		action game.Action
	}{
		{
			name: "pass_trump does not mutate original",
			action: game.Action{
				Type:       game.ActionPassTrump,
				PlayerSeat: 1,
			},
		},
		{
			name: "pick_trump does not mutate original",
			action: game.Action{
				Type:       game.ActionPickTrump,
				PlayerSeat: 1,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gs := testfixtures.NewGameJustDealt()

			// Snapshot original values
			origPhase := gs.Phase
			origPassCount := gs.BiddingPassCount
			origRound := gs.BiddingRound
			origActive := gs.ActivePlayerSeat
			origDealer := gs.DealerSeat
			origHand0 := make([]game.Card, len(gs.Players[0].Hand))
			copy(origHand0, gs.Players[0].Hand)

			_, _ = game.ApplyAction(gs, tc.action)

			// Verify original state is unchanged
			assert.Equal(t, origPhase, gs.Phase, "Phase should not be mutated")
			assert.Equal(t, origPassCount, gs.BiddingPassCount, "BiddingPassCount should not be mutated")
			assert.Equal(t, origRound, gs.BiddingRound, "BiddingRound should not be mutated")
			assert.Equal(t, origActive, gs.ActivePlayerSeat, "ActivePlayerSeat should not be mutated")
			assert.Equal(t, origDealer, gs.DealerSeat, "DealerSeat should not be mutated")
			assert.Equal(t, origHand0, gs.Players[0].Hand, "Player hands should not be mutated")
		})
	}
}

func TestMultipleReshuffles(t *testing.T) {
	// Start from a fresh game and apply 8 passes to trigger first reshuffle
	gs := testfixtures.NewGameJustDealt()

	// Apply 8 passes (full round 1 + full round 2) to trigger reshuffle
	for i := 0; i < 8; i++ {
		action := game.Action{
			Type:       game.ActionPassTrump,
			PlayerSeat: gs.ActivePlayerSeat,
		}
		var err error
		gs, err = game.ApplyAction(gs, action)
		require.NoError(t, err, "pass %d should succeed", i+1)
	}

	// After first reshuffle: dealer should be 1, phase is dealing
	assert.Equal(t, game.PhaseDealing, gs.Phase, "reshuffle sets dealing phase")
	assert.Equal(t, 1, gs.DealerSeat, "dealer should rotate to seat 1 after first reshuffle")
	assert.Equal(t, 2, gs.ActivePlayerSeat, "active should be (1+1)%4=2")
	assert.Equal(t, 1, gs.BiddingRound, "round should reset to 1")
	assert.Equal(t, 0, gs.BiddingPassCount, "pass count should reset to 0")

	// Simulate session manager auto-transition to bidding
	gs.Phase = game.PhaseBidding

	// Apply 8 more passes to trigger second reshuffle
	for i := 0; i < 8; i++ {
		action := game.Action{
			Type:       game.ActionPassTrump,
			PlayerSeat: gs.ActivePlayerSeat,
		}
		var err error
		gs, err = game.ApplyAction(gs, action)
		require.NoError(t, err, "pass %d (second round) should succeed", i+1)
	}

	// After second reshuffle: dealer should be 2
	assert.Equal(t, 2, gs.DealerSeat, "dealer should rotate to seat 2 after second reshuffle")
	assert.Equal(t, 3, gs.ActivePlayerSeat, "active should be (2+1)%4=3")
	assert.Equal(t, 1, gs.BiddingRound)
	assert.Equal(t, 0, gs.BiddingPassCount)

	// Verify card integrity after multiple reshuffles: 5 per hand + 11 in Deck + 1 candidate.
	seen := make(map[string]bool)
	for _, p := range gs.Players {
		assert.Len(t, p.Hand, 5)
		for _, card := range p.Hand {
			id := card.String()
			assert.False(t, seen[id], "duplicate card: %s", id)
			seen[id] = true
		}
	}
	assert.Len(t, gs.Deck, 11)
	for _, card := range gs.Deck {
		id := card.String()
		assert.False(t, seen[id], "duplicate card in deck: %s", id)
		seen[id] = true
	}
	require.NotNil(t, gs.TrumpCandidate)
	seen[gs.TrumpCandidate.String()] = true
	assert.Len(t, seen, 32)
}

// TestPickTrumpStage2Rotation verifies the real-table card distribution rule:
// the dealer rotates from (Dealer+1)%4 around the table dealing 3 to each
// non-picker seat, 2 to the picker in their natural slot; the public
// candidate is then appended to the picker's hand. Card-conservation:
// every starting card is accounted for exactly once after the deal.
func TestPickTrumpStage2Rotation(t *testing.T) {
	tests := []struct {
		name       string
		passCount  int
		picker     int
		round2Suit *game.Suit
	}{
		{name: "round 1, picker = seat 1 (first bidder)", passCount: 0, picker: 1},
		{name: "round 1, picker = seat 2", passCount: 1, picker: 2},
		{name: "round 1, picker = seat 3", passCount: 2, picker: 3},
		{name: "round 1, picker = seat 0 (last bidder)", passCount: 3, picker: 0},
		{name: "round 2, picker = seat 1, suit = spades", passCount: 4, picker: 1, round2Suit: suitPtr(game.SuitSpades)},
		{name: "round 2, picker = seat 0, suit = clubs", passCount: 7, picker: 0, round2Suit: suitPtr(game.SuitClubs)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gs := testfixtures.NewGameMidBidding(tc.passCount)

			// Snapshot the starting card layout so we can verify conservation.
			startingCards := collectCards(gs)
			require.NotNil(t, gs.TrumpCandidate, "fixture must have a face-up candidate")
			originalCandidate := *gs.TrumpCandidate

			action := game.Action{
				Type:       game.ActionPickTrump,
				PlayerSeat: tc.picker,
				Suit:       tc.round2Suit,
			}

			result, err := game.ApplyAction(gs, action)
			require.NoError(t, err)
			require.NotNil(t, result)

			// After stage-2 distribution: each hand has 8 cards, Deck empty, candidate cleared.
			for i, p := range result.Players {
				assert.Len(t, p.Hand, 8, "seat %d should have 8 cards after stage-2", i)
			}
			assert.Empty(t, result.Deck, "Deck should be cleared after stage-2")
			assert.Nil(t, result.TrumpCandidate, "TrumpCandidate should be cleared after stage-2")

			// The originally face-up candidate transfers to the picker's hand
			// in BOTH rounds — round 2 still inherits the candidate even
			// though the suit is freely chosen. Assert exactly-once so a
			// future fixture regression that puts a duplicate of the
			// candidate into the picker's stage-1 hand can't silently
			// satisfy the membership check.
			candidateCount := 0
			for _, c := range result.Players[tc.picker].Hand {
				if c == originalCandidate {
					candidateCount++
				}
			}
			assert.Equal(t, 1, candidateCount,
				"picker should receive the originally face-up candidate exactly once")

			// Card conservation: same 32 cards across all hands now.
			finalCards := collectCards(result)
			assert.ElementsMatch(t, startingCards, finalCards, "all 32 cards preserved through stage-2")

			// Trump locked correctly.
			require.NotNil(t, result.TrumpSuit)
			if tc.round2Suit != nil {
				assert.Equal(t, *tc.round2Suit, *result.TrumpSuit, "round 2 trump matches action.Suit")
			} else {
				assert.Equal(t, game.SuitHearts, *result.TrumpSuit, "round 1 trump = candidate suit (hearts)")
			}
		})
	}
}

// TestPickTrumpRound1_AppendsCandidateAndDealsCorrectCards spot-checks the
// canonical round-1 first-bidder rotation against the deterministic fixture
// layout, so a regression in the slice math is caught at the card level.
func TestPickTrumpRound1_AppendsCandidateAndDealsCorrectCards(t *testing.T) {
	gs := testfixtures.NewGameJustDealt()
	candidate := *gs.TrumpCandidate // AH
	expectedSeat1Adds := []game.Card{gs.Deck[0], gs.Deck[1], candidate}
	expectedSeat2Adds := []game.Card{gs.Deck[2], gs.Deck[3], gs.Deck[4]}
	expectedSeat3Adds := []game.Card{gs.Deck[5], gs.Deck[6], gs.Deck[7]}
	expectedSeat0Adds := []game.Card{gs.Deck[8], gs.Deck[9], gs.Deck[10]}

	result, err := game.ApplyAction(gs, game.Action{Type: game.ActionPickTrump, PlayerSeat: 1})
	require.NoError(t, err)

	// Each seat's final hand should be (their initial 5) ++ (the cards above).
	assert.Equal(t, append(append([]game.Card{}, gs.Players[1].Hand...), expectedSeat1Adds...), result.Players[1].Hand,
		"seat 1 (picker) gets Deck[0:2] + candidate after their initial 5")
	assert.Equal(t, append(append([]game.Card{}, gs.Players[2].Hand...), expectedSeat2Adds...), result.Players[2].Hand,
		"seat 2 gets Deck[2:5] after their initial 5")
	assert.Equal(t, append(append([]game.Card{}, gs.Players[3].Hand...), expectedSeat3Adds...), result.Players[3].Hand,
		"seat 3 gets Deck[5:8] after their initial 5")
	assert.Equal(t, append(append([]game.Card{}, gs.Players[0].Hand...), expectedSeat0Adds...), result.Players[0].Hand,
		"seat 0 gets Deck[8:11] after their initial 5")
}

// suitPtr is a one-line helper to take the address of a Suit literal.
func suitPtr(s game.Suit) *game.Suit { return &s }

// collectCards returns every card present anywhere in the game state:
// hands, face-down slots, deck, and the visible trump candidate. Bitola states
// have no face-down cards, so that term contributes nothing there.
func collectCards(gs *game.GameState) []game.Card {
	out := make([]game.Card, 0, 32)
	for i := range gs.Players {
		out = append(out, gs.Players[i].Hand...)
		out = append(out, gs.Players[i].FaceDownCards...)
	}
	out = append(out, gs.Deck...)
	if gs.TrumpCandidate != nil {
		out = append(out, *gs.TrumpCandidate)
	}
	return out
}

func TestRound1IgnoresActionSuit(t *testing.T) {
	// In round 1, Action.Suit should be ignored — trump is always the candidate's suit
	gs := testfixtures.NewGameJustDealt()
	spades := game.SuitSpades
	action := game.Action{
		Type:       game.ActionPickTrump,
		PlayerSeat: 1,
		Suit:       &spades, // attempt to pick spades, but candidate is hearts
	}

	result, err := game.ApplyAction(gs, action)

	require.NoError(t, err)
	require.NotNil(t, result.TrumpSuit)
	assert.Equal(t, game.SuitHearts, *result.TrumpSuit, "round 1 should use candidate suit, not action.Suit")
}

// --- Variant rule config: the free-suit / all-before-bidding divergences ---

// TestCroatianPickRound1 locks the round-1 divergence: with no trump candidate,
// the picker names any suit freely, takes no card, and the face-down cards fold
// into every hand as bidding resolves. A resolved Croatian bid opens the
// dedicated declaration phase rather than trick 1 — every seat in this fixture
// holds a meld, so the cursor stops on the first of them.
func TestCroatianPickRound1(t *testing.T) {
	tests := []struct {
		name       string
		passCount  int
		activeSeat int
		chosenSuit game.Suit
	}{
		{name: "first bidder names spades", passCount: 0, activeSeat: 1, chosenSuit: game.SuitSpades},
		{name: "second bidder names hearts after 1 pass", passCount: 1, activeSeat: 2, chosenSuit: game.SuitHearts},
		{name: "third bidder names diamonds after 2 passes", passCount: 2, activeSeat: 3, chosenSuit: game.SuitDiamonds},
		{name: "dealer names clubs after 3 passes", passCount: 3, activeSeat: 0, chosenSuit: game.SuitClubs},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gs := testfixtures.NewGameCroatianMidBidding(tc.passCount)
			startingCards := collectCards(gs)
			handBefore := len(gs.Players[tc.activeSeat].Hand)
			suit := tc.chosenSuit
			action := game.Action{
				Type:       game.ActionPickTrump,
				PlayerSeat: tc.activeSeat,
				Suit:       &suit,
			}

			result, err := game.ApplyAction(gs, action)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, game.PhaseDeclaring, result.Phase)
			require.NotNil(t, result.TrumpSuit)
			assert.Equal(t, tc.chosenSuit, *result.TrumpSuit, "trump is the freely named suit")
			require.NotNil(t, result.TrumpCallerSeat)
			assert.Equal(t, tc.activeSeat, *result.TrumpCallerSeat)
			assert.Equal(t, 1, result.ActivePlayerSeat,
				"declarations open at (DealerSeat+1)%4 — the seat that will lead trick 1")
			assert.True(t, result.AwaitingDeclaration, "that seat holds a meld, so it is prompted")
			assert.Equal(t, 0, result.TrickNumber, "no trick is open during the declaration phase")
			assert.Empty(t, result.CurrentTrick)

			// The picker draws no card: their hand grows only by their own two
			// face-down cards, exactly like every other seat.
			assert.Equal(t, handBefore+2, len(result.Players[tc.activeSeat].Hand),
				"picker takes no extra card — only their own two face-down cards")
			for i, p := range result.Players {
				assert.Len(t, p.Hand, 8, "seat %d holds 8 after the reveal-and-merge", i)
				assert.Empty(t, p.FaceDownCards, "seat %d's hidden slot is cleared", i)
			}
			assert.Empty(t, result.Deck, "no stage-2 reserve exists")
			assert.Nil(t, result.TrumpCandidate)
			assert.False(t, result.FaceDownRevealed, "the reveal flag is spent once bidding resolves")

			assertCardsAreFullDeck(t, collectCards(result))
			assert.ElementsMatch(t, startingCards, collectCards(result), "all 32 cards preserved")
		})
	}
}

// TestCroatianPickRejectsBadSuit covers both invalid-suit rows of the matrix in
// both rounds: a missing suit and an unknown suit are ErrInvalidBid, and the
// original state is untouched.
func TestCroatianPickRejectsBadSuit(t *testing.T) {
	badSuit := game.Suit("X")
	tests := []struct {
		name       string
		passCount  int
		activeSeat int
		suit       *game.Suit
	}{
		{name: "round 1 pick with no suit", passCount: 0, activeSeat: 1, suit: nil},
		{name: "round 1 pick with an unknown suit", passCount: 0, activeSeat: 1, suit: &badSuit},
		{name: "round 2 pick with no suit", passCount: 4, activeSeat: 1, suit: nil},
		{name: "round 2 pick with an unknown suit", passCount: 4, activeSeat: 1, suit: &badSuit},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gs := testfixtures.NewGameCroatianMidBidding(tc.passCount)
			origPhase := gs.Phase
			origRound := gs.BiddingRound
			origPassCount := gs.BiddingPassCount
			origActiveSeat := gs.ActivePlayerSeat
			origCards := collectCards(gs)

			result, err := game.ApplyAction(gs, game.Action{
				Type:       game.ActionPickTrump,
				PlayerSeat: tc.activeSeat,
				Suit:       tc.suit,
			})

			assert.Nil(t, result)
			require.Error(t, err)
			assert.ErrorIs(t, err, apperr.ErrInvalidBid)

			assert.Equal(t, origPhase, gs.Phase)
			assert.Equal(t, origRound, gs.BiddingRound)
			assert.Equal(t, origPassCount, gs.BiddingPassCount)
			assert.Equal(t, origActiveSeat, gs.ActivePlayerSeat)
			assert.Nil(t, gs.TrumpSuit, "trump must remain unset after a rejected pick")
			assert.ElementsMatch(t, origCards, collectCards(gs), "cards must not move")
		})
	}
}

// TestCroatianRound1PassedOutRevealsFaceDown locks the round-2 reveal: the
// fourth round-1 pass opens round 2 at dealer+1 with all four suits available
// and marks every seat's two face-down cards as revealed — while keeping them
// out of Hand, so they still cannot ride any snapshot.
func TestCroatianRound1PassedOutRevealsFaceDown(t *testing.T) {
	gs := testfixtures.NewGameCroatianMidBidding(3)
	require.False(t, gs.FaceDownRevealed, "not revealed before the fourth pass")

	result, err := game.ApplyAction(gs, game.Action{Type: game.ActionPassTrump, PlayerSeat: 0})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, game.PhaseBidding, result.Phase)
	assert.Equal(t, 2, result.BiddingRound)
	assert.Equal(t, 0, result.BiddingPassCount)
	assert.Equal(t, (result.DealerSeat+1)%4, result.ActivePlayerSeat)
	assert.True(t, result.FaceDownRevealed, "the fourth round-1 pass marks the reveal")

	for i, p := range result.Players {
		assert.Len(t, p.Hand, 6, "seat %d's open hand is unchanged by the reveal", i)
		assert.Len(t, p.FaceDownCards, 2, "seat %d's two cards stay OUT of Hand", i)
	}

	// All four suits are open in round 2 — nothing was spent, because no
	// candidate ever existed.
	for _, suit := range game.AllSuits {
		s := suit
		picked, err := game.ApplyAction(result, game.Action{
			Type:       game.ActionPickTrump,
			PlayerSeat: result.ActivePlayerSeat,
			Suit:       &s,
		})
		require.NoError(t, err, "suit %s must be available in round 2", suit)
		require.NotNil(t, picked.TrumpSuit)
		assert.Equal(t, suit, *picked.TrumpSuit)
	}
}

// TestCroatianDealerCannotPassInRound2 locks the forced dealer pick: the fourth
// round-2 pass is rejected, so reshuffleAndRedeal is unreachable under this
// config, and pick_trump is the dealer's only legal action.
func TestCroatianDealerCannotPassInRound2(t *testing.T) {
	gs := testfixtures.NewGameCroatianMidBidding(7)
	require.Equal(t, 2, gs.BiddingRound)
	require.Equal(t, 3, gs.BiddingPassCount)
	require.Equal(t, gs.DealerSeat, gs.ActivePlayerSeat, "the dealer bids last in round 2")

	result, err := game.ApplyAction(gs, game.Action{
		Type:       game.ActionPassTrump,
		PlayerSeat: gs.DealerSeat,
	})

	assert.Nil(t, result)
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrInvalidBid)
	assert.Equal(t, game.PhaseBidding, gs.Phase, "no reshuffle — state is untouched")
	assert.Equal(t, 1, gs.HandNumber)
	assert.Equal(t, 0, gs.DealerSeat, "the dealer does not rotate")

	// The dealer CAN pick, and that resolves the hand.
	suit := game.SuitClubs
	picked, err := game.ApplyAction(gs, game.Action{
		Type:       game.ActionPickTrump,
		PlayerSeat: gs.DealerSeat,
		Suit:       &suit,
	})
	require.NoError(t, err)
	assert.Equal(t, game.PhaseDeclaring, picked.Phase, "a resolved Croatian bid opens the declaration phase")
	require.NotNil(t, picked.TrumpSuit)
	assert.Equal(t, game.SuitClubs, *picked.TrumpSuit)
}

// TestCroatianEarlierPassesStillLegal guards against the forced-dealer rule
// over-reaching: only the FOURTH round-2 pass is refused.
func TestCroatianEarlierPassesStillLegal(t *testing.T) {
	tests := []struct {
		name               string
		passCount          int
		activeSeat         int
		expectedRound      int
		expectedPassCount  int
		expectedActiveSeat int
	}{
		{name: "first pass in round 1", passCount: 0, activeSeat: 1, expectedRound: 1, expectedPassCount: 1, expectedActiveSeat: 2},
		{name: "third pass in round 1", passCount: 2, activeSeat: 3, expectedRound: 1, expectedPassCount: 3, expectedActiveSeat: 0},
		{name: "first pass in round 2", passCount: 4, activeSeat: 1, expectedRound: 2, expectedPassCount: 1, expectedActiveSeat: 2},
		{name: "third pass in round 2", passCount: 6, activeSeat: 3, expectedRound: 2, expectedPassCount: 3, expectedActiveSeat: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gs := testfixtures.NewGameCroatianMidBidding(tc.passCount)

			result, err := game.ApplyAction(gs, game.Action{
				Type:       game.ActionPassTrump,
				PlayerSeat: tc.activeSeat,
			})

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, game.PhaseBidding, result.Phase)
			assert.Equal(t, tc.expectedRound, result.BiddingRound)
			assert.Equal(t, tc.expectedPassCount, result.BiddingPassCount)
			assert.Equal(t, tc.expectedActiveSeat, result.ActivePlayerSeat)
			assert.Nil(t, result.TrumpSuit)
		})
	}
}

// TestFaceDownCardsNeverSerialized is the security assertion: at every point
// before bidding resolves, a marshalled snapshot contains none of any seat's
// two face-down cards — including in that seat's own payload, because the same
// bytes go to all four seats.
func TestFaceDownCardsNeverSerialized(t *testing.T) {
	tests := []struct {
		name      string
		passCount int
	}{
		{name: "just dealt, round 1", passCount: 0},
		{name: "mid round 1", passCount: 2},
		{name: "round 2 just opened, cards revealed to their owners", passCount: 4},
		{name: "round 2 with the dealer on the clock", passCount: 7},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gs := testfixtures.NewGameCroatianMidBidding(tc.passCount)

			data, err := json.Marshal(gs)
			require.NoError(t, err)

			// Collect the hidden cards, then collect every card the payload
			// actually carries by WALKING the unmarshalled structure. A substring
			// search for `"rank":"X","suit":"Y"` would only hold while Card's
			// field order and json tags are exactly today's — reorder the struct
			// and the assertion would pass vacuously on any payload.
			hidden := map[string]int{}
			for i := range gs.Players {
				for _, c := range gs.Players[i].FaceDownCards {
					hidden[c.String()] = i
				}
			}
			require.Len(t, hidden, 8, "the fixture must actually be hiding 8 distinct cards")

			var raw map[string]any
			require.NoError(t, json.Unmarshal(data, &raw))

			for _, id := range cardIDsInPayload(t, raw) {
				seat, isHidden := hidden[id]
				assert.False(t, isHidden,
					"seat %d's face-down card %s must not appear anywhere in a snapshot", seat, id)
			}

			// The flag itself is server-only too — nothing about the hidden
			// cards reaches the wire.
			assert.NotContains(t, raw, "faceDownRevealed")
			assert.NotContains(t, raw, "faceDownCards")
			assert.NotContains(t, raw, "rules")
		})
	}
}

// cardIDsInPayload walks an unmarshalled match_state payload and returns the ID
// of every card it carries, wherever it sits — hands, the deck, the trump
// candidate, trick cards, declaration cards, or anywhere a future field puts
// one. Structure-driven rather than string-driven, so it keeps finding cards
// through a Card field reorder or a rename of the field that holds them.
func cardIDsInPayload(t *testing.T, node any) []string {
	t.Helper()
	var out []string
	switch v := node.(type) {
	case map[string]any:
		rank, rankOK := v["rank"].(string)
		suit, suitOK := v["suit"].(string)
		if rankOK && suitOK {
			out = append(out, rank+suit)
		}
		for _, child := range v {
			out = append(out, cardIDsInPayload(t, child)...)
		}
	case []any:
		for _, child := range v {
			out = append(out, cardIDsInPayload(t, child)...)
		}
	}
	return out
}

// TestBitolaBiddingUnaffectedByConfig re-asserts the Bitola paths that the
// config gates now guard, from the Croatian-capable code: the round-1 candidate
// binding, the spent-suit lock, and the reshuffle outcome.
func TestBitolaBiddingUnaffectedByConfig(t *testing.T) {
	t.Run("round 1 still binds to the candidate and ignores action.Suit", func(t *testing.T) {
		gs := testfixtures.NewGameJustDealt()
		spades := game.SuitSpades
		result, err := game.ApplyAction(gs, game.Action{
			Type:       game.ActionPickTrump,
			PlayerSeat: 1,
			Suit:       &spades,
		})
		require.NoError(t, err)
		require.NotNil(t, result.TrumpSuit)
		assert.Equal(t, game.SuitHearts, *result.TrumpSuit)
	})

	t.Run("the fourth round-2 pass still reshuffles and rotates the dealer", func(t *testing.T) {
		gs := testfixtures.NewGameMidBidding(7)
		result, err := game.ApplyAction(gs, game.Action{Type: game.ActionPassTrump, PlayerSeat: 0})
		require.NoError(t, err)
		assert.Equal(t, game.PhaseDealing, result.Phase)
		assert.Equal(t, 1, result.DealerSeat)
		require.NotNil(t, result.TrumpCandidate)
	})

	t.Run("a stage-1 state with a short deck is still rejected as wrong phase", func(t *testing.T) {
		gs := testfixtures.NewGameJustDealt()
		gs.Deck = gs.Deck[:5]
		result, err := game.ApplyAction(gs, game.Action{Type: game.ActionPickTrump, PlayerSeat: 1})
		assert.Nil(t, result)
		require.Error(t, err)
		assert.ErrorIs(t, err, apperr.ErrWrongPhase)
	})
}

// TestPickTrumpRejectsStateContradictingItsDealShape locks the other half of the
// deal-shape guard: a state whose cards contradict its own config is rejected
// rather than reaching the rotation with an unvalidated deck.
func TestPickTrumpRejectsStateContradictingItsDealShape(t *testing.T) {
	spades := game.SuitSpades
	tests := []struct {
		name   string
		mutate func(gs *game.GameState)
	}{
		{
			name: "no-candidate config but a candidate is present",
			mutate: func(gs *game.GameState) {
				gs.TrumpCandidate = &game.Card{Rank: game.Rank7, Suit: game.SuitHearts}
			},
		},
		{
			name: "no-candidate config but the reserve is not empty",
			mutate: func(gs *game.GameState) {
				gs.Deck = []game.Card{{Rank: game.Rank7, Suit: game.SuitHearts}}
			},
		},
		{
			name: "candidate config but the candidate is missing",
			mutate: func(gs *game.GameState) {
				gs.Rules = game.RulesFor(game.VariantBitola)
				gs.TrumpCandidate = nil
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gs := testfixtures.NewGameCroatianJustDealt()
			tc.mutate(gs)

			result, err := game.ApplyAction(gs, game.Action{
				Type:       game.ActionPickTrump,
				PlayerSeat: gs.ActivePlayerSeat,
				Suit:       &spades,
			})

			assert.Nil(t, result)
			require.Error(t, err)
			assert.ErrorIs(t, err, apperr.ErrWrongPhase)
		})
	}
}

// TestCroatianFullBiddingFromNewGame walks a real (randomly dealt) Croatian
// hand from NewGame through a passed-out round 1 to the dealer's forced round-2
// pick, so the fixtures' hand-built layout is not the only thing under test.
func TestCroatianFullBiddingFromNewGame(t *testing.T) {
	gs := game.NewGame([4]uint{10, 20, 30, 40}, [4]string{"a", "b", "c", "d"},
		[4]bool{}, game.VariantCroatia, "1001", 1)
	startingCards := collectCards(gs)
	require.Len(t, startingCards, 32)

	// The session manager performs the dealing→bidding transition.
	gs.Phase = game.PhaseBidding

	// Round 1: everybody passes.
	for i := 0; i < 4; i++ {
		var err error
		gs, err = game.ApplyAction(gs, game.Action{
			Type:       game.ActionPassTrump,
			PlayerSeat: gs.ActivePlayerSeat,
		})
		require.NoError(t, err, "round-1 pass %d", i+1)
	}
	assert.Equal(t, game.PhaseBidding, gs.Phase, "no reshuffle after round 1")
	assert.Equal(t, 2, gs.BiddingRound)
	assert.True(t, gs.FaceDownRevealed)
	assert.Equal(t, (gs.DealerSeat+1)%4, gs.ActivePlayerSeat)

	// Round 2: three pass, then the dealer is refused a pass.
	for i := 0; i < 3; i++ {
		var err error
		gs, err = game.ApplyAction(gs, game.Action{
			Type:       game.ActionPassTrump,
			PlayerSeat: gs.ActivePlayerSeat,
		})
		require.NoError(t, err, "round-2 pass %d", i+1)
	}
	require.Equal(t, gs.DealerSeat, gs.ActivePlayerSeat)

	rejected, err := game.ApplyAction(gs, game.Action{
		Type:       game.ActionPassTrump,
		PlayerSeat: gs.DealerSeat,
	})
	assert.Nil(t, rejected)
	require.ErrorIs(t, err, apperr.ErrInvalidBid)

	// The dealer names a suit and the hand starts.
	suit := game.SuitHearts
	final, err := game.ApplyAction(gs, game.Action{
		Type:       game.ActionPickTrump,
		PlayerSeat: gs.DealerSeat,
		Suit:       &suit,
	})
	require.NoError(t, err)
	// A resolved Croatian bid opens the dedicated declaration phase, never trick 1
	// directly — unless the random deal left nobody holding a meld, in which case
	// the phase opens and resolves inside this same transition.
	require.Contains(t, []game.Phase{game.PhaseDeclaring, game.PhasePlaying}, final.Phase)
	require.NotNil(t, final.TrumpSuit)
	assert.Equal(t, game.SuitHearts, *final.TrumpSuit)
	require.NotNil(t, final.TrumpCallerSeat)
	assert.Equal(t, gs.DealerSeat, *final.TrumpCallerSeat)
	for i, p := range final.Players {
		assert.Len(t, p.Hand, 8, "seat %d holds 8 once bidding resolves", i)
		assert.Empty(t, p.FaceDownCards)
	}
	assert.ElementsMatch(t, startingCards, collectCards(final), "all 32 cards preserved end to end")

	// Walk the declaration phase to its end. Every prompted seat skips; the
	// phase must terminate in at most four answers whatever the random deal
	// produced.
	for i := 0; final.Phase == game.PhaseDeclaring; i++ {
		require.Less(t, i, 4, "the declaration phase must resolve within four answers")
		require.True(t, final.AwaitingDeclaration, "the phase only persists with a prompt outstanding")
		final, err = game.ApplyAction(final, game.Action{
			Type:       game.ActionSkipDeclare,
			PlayerSeat: final.ActivePlayerSeat,
		})
		require.NoError(t, err, "skip_declare %d", i+1)
	}
	assert.Equal(t, game.PhasePlaying, final.Phase, "the phase hands off to trick 1")
	assert.Equal(t, 1, final.TrickNumber)
	assert.True(t, final.DeclarationsResolved)
	assert.False(t, final.AwaitingDeclaration, "nothing is owed at trick 1 any more")
	assert.Equal(t, (final.DealerSeat+1)%4, final.ActivePlayerSeat, "the seat after the dealer leads")

	// The hand is genuinely playable: the leader can play a legal card with no
	// declaration owed.
	lead := final.Players[final.ActivePlayerSeat].Hand[0]
	played, err := game.ApplyAction(final, game.Action{
		Type:       game.ActionPlayCard,
		PlayerSeat: final.ActivePlayerSeat,
		Card:       &lead,
	})
	require.NoError(t, err)
	assert.Len(t, played.CurrentTrick, 1)
}

// TestMustPickTrumpWireFlag pins the derived snapshot flag the client reads to
// decide whether to offer a Pass control. It is refreshed at ApplyAction's
// single exit, so the assertions below drive real actions rather than setting
// counters — the point is that no handler can leave it stale.
func TestMustPickTrumpWireFlag(t *testing.T) {
	t.Run("false through a Croatian hand until the dealer is on the clock", func(t *testing.T) {
		gs := testfixtures.NewGameCroatianJustDealt()
		assert.False(t, gs.MustPickTrump, "nobody is forced on the opening bid")

		// Seven passes: round 1 passed out, then three in round 2. Only the
		// last one leaves the dealer with no legal pass.
		for i := 0; i < 7; i++ {
			next, err := game.ApplyAction(gs, game.Action{
				Type:       game.ActionPassTrump,
				PlayerSeat: gs.ActivePlayerSeat,
			})
			require.NoError(t, err, "pass %d must be legal", i+1)
			gs = next
			assert.Equal(t, i == 6, gs.MustPickTrump,
				"after %d passes the flag must be %v", i+1, i == 6)
		}
		assert.Equal(t, gs.DealerSeat, gs.ActivePlayerSeat,
			"the forced seat is the dealer, bidding last in round 2")

		// The forced pick clears it again.
		suit := game.SuitSpades
		resolved, err := game.ApplyAction(gs, game.Action{
			Type:       game.ActionPickTrump,
			PlayerSeat: gs.ActivePlayerSeat,
			Suit:       &suit,
		})
		require.NoError(t, err)
		assert.False(t, resolved.MustPickTrump, "bidding is over — nobody is on the clock")
	})

	t.Run("never true in Bitola", func(t *testing.T) {
		// Bitola's config reshuffles a passed-out round 2 instead of forcing the
		// dealer, so no pass count can ever raise this flag.
		gs := testfixtures.NewGameJustDealt()
		for i := 0; i < 8; i++ {
			next, err := game.ApplyAction(gs, game.Action{
				Type:       game.ActionPassTrump,
				PlayerSeat: gs.ActivePlayerSeat,
			})
			require.NoError(t, err, "pass %d must be legal", i+1)
			gs = next
			assert.False(t, gs.MustPickTrump, "after %d passes", i+1)
		}
	})
}
