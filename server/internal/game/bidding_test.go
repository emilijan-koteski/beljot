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
			expectedTrump: game.SuitHearts,
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
			assert.Equal(t, 1, result.ActivePlayerSeat)
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
	gs := testfixtures.NewGameMidBidding(3)
	action := game.Action{
		Type:       game.ActionPassTrump,
		PlayerSeat: 0,
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
	gs := testfixtures.NewGameMidBidding(4)
	action := game.Action{
		Type:       game.ActionPickTrump,
		PlayerSeat: 1,
	}

	result, err := game.ApplyAction(gs, action)

	assert.Nil(t, result)
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrInvalidBid)
}

// TestRound2PickCandidateSuitRejected checks that picking the candidate's suit in round 2 fails with ErrInvalidBid.
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
	gs := testfixtures.NewGameMidBidding(7)
	action := game.Action{
		Type:       game.ActionPassTrump,
		PlayerSeat: 0,
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

	for i, p := range result.Players {
		assert.Len(t, p.Hand, 5, "seat %d should have 5 cards after stage-1 re-deal", i)
	}
	assert.Len(t, result.Deck, 11, "Deck should hold 11 cards after re-deal")

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
			wrongSeat:  0,
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

			origPhase := gs.Phase
			origPassCount := gs.BiddingPassCount
			origRound := gs.BiddingRound
			origActive := gs.ActivePlayerSeat
			origDealer := gs.DealerSeat
			origHand0 := make([]game.Card, len(gs.Players[0].Hand))
			copy(origHand0, gs.Players[0].Hand)

			_, _ = game.ApplyAction(gs, tc.action)

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
	gs := testfixtures.NewGameJustDealt()

	for i := 0; i < 8; i++ {
		action := game.Action{
			Type:       game.ActionPassTrump,
			PlayerSeat: gs.ActivePlayerSeat,
		}
		var err error
		gs, err = game.ApplyAction(gs, action)
		require.NoError(t, err, "pass %d should succeed", i+1)
	}

	assert.Equal(t, game.PhaseDealing, gs.Phase, "reshuffle sets dealing phase")
	assert.Equal(t, 1, gs.DealerSeat, "dealer should rotate to seat 1 after first reshuffle")
	assert.Equal(t, 2, gs.ActivePlayerSeat, "active should be (1+1)%4=2")
	assert.Equal(t, 1, gs.BiddingRound, "round should reset to 1")
	assert.Equal(t, 0, gs.BiddingPassCount, "pass count should reset to 0")

	gs.Phase = game.PhaseBidding

	for i := 0; i < 8; i++ {
		action := game.Action{
			Type:       game.ActionPassTrump,
			PlayerSeat: gs.ActivePlayerSeat,
		}
		var err error
		gs, err = game.ApplyAction(gs, action)
		require.NoError(t, err, "pass %d (second round) should succeed", i+1)
	}

	assert.Equal(t, 2, gs.DealerSeat, "dealer should rotate to seat 2 after second reshuffle")
	assert.Equal(t, 3, gs.ActivePlayerSeat, "active should be (2+1)%4=3")
	assert.Equal(t, 1, gs.BiddingRound)
	assert.Equal(t, 0, gs.BiddingPassCount)

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

// TestPickTrumpStage2Rotation checks the stage-2 table rotation: 3 cards per non-picker, 2 plus the candidate for the picker.
// Every starting card must be accounted for exactly once.
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

			for i, p := range result.Players {
				assert.Len(t, p.Hand, 8, "seat %d should have 8 cards after stage-2", i)
			}
			assert.Empty(t, result.Deck, "Deck should be cleared after stage-2")
			assert.Nil(t, result.TrumpCandidate, "TrumpCandidate should be cleared after stage-2")

			candidateCount := 0
			for _, c := range result.Players[tc.picker].Hand {
				if c == originalCandidate {
					candidateCount++
				}
			}
			assert.Equal(t, 1, candidateCount,
				"picker should receive the originally face-up candidate exactly once")

			finalCards := collectCards(result)
			assert.ElementsMatch(t, startingCards, finalCards, "all 32 cards preserved through stage-2")

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
	candidate := *gs.TrumpCandidate
	expectedSeat1Adds := []game.Card{gs.Deck[0], gs.Deck[1], candidate}
	expectedSeat2Adds := []game.Card{gs.Deck[2], gs.Deck[3], gs.Deck[4]}
	expectedSeat3Adds := []game.Card{gs.Deck[5], gs.Deck[6], gs.Deck[7]}
	expectedSeat0Adds := []game.Card{gs.Deck[8], gs.Deck[9], gs.Deck[10]}

	result, err := game.ApplyAction(gs, game.Action{Type: game.ActionPickTrump, PlayerSeat: 1})
	require.NoError(t, err)

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
	gs := testfixtures.NewGameJustDealt()
	spades := game.SuitSpades
	action := game.Action{
		Type:       game.ActionPickTrump,
		PlayerSeat: 1,
		Suit:       &spades,
	}

	result, err := game.ApplyAction(gs, action)

	require.NoError(t, err)
	require.NotNil(t, result.TrumpSuit)
	assert.Equal(t, game.SuitHearts, *result.TrumpSuit, "round 1 should use candidate suit, not action.Suit")
}

// TestCroatianPickRound1 checks a free-suit round-1 pick folds in the face-down cards and opens the declaration phase.
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
				"pinned to (DealerSeat+1)%4 — the seat that will lead trick 1")
			assert.False(t, result.AwaitingDeclaration,
				"the dedicated phase asks all four seats at once; nobody is on the clock")
			assert.Equal(t, 0, result.TrickNumber, "no trick is open during the declaration phase")
			assert.Empty(t, result.CurrentTrick)

			assert.Equal(t, handBefore+2, len(result.Players[tc.activeSeat].Hand),
				"picker takes no extra card — only their own two face-down cards")
			for i, p := range result.Players {
				assert.Len(t, p.Hand, 8, "seat %d holds 8 after the reveal-and-merge", i)
				assert.Empty(t, p.FaceDownCards, "seat %d's hidden slot is cleared", i)
			}
			assert.Empty(t, result.Deck, "no stage-2 reserve exists")
			assert.Nil(t, result.TrumpCandidate)

			assertCardsAreFullDeck(t, collectCards(result))
			assert.ElementsMatch(t, startingCards, collectCards(result), "all 32 cards preserved")
		})
	}
}

// TestCroatianPickRejectsBadSuit covers both invalid-suit rows of the matrix,
// for a voluntary early pick and for the dealer's forced pick: a missing suit
// and an unknown suit are ErrInvalidBid, and the original state is untouched.
func TestCroatianPickRejectsBadSuit(t *testing.T) {
	badSuit := game.Suit("X")
	tests := []struct {
		name       string
		passCount  int
		activeSeat int
		suit       *game.Suit
	}{
		{name: "early pick with no suit", passCount: 0, activeSeat: 1, suit: nil},
		{name: "early pick with an unknown suit", passCount: 0, activeSeat: 1, suit: &badSuit},
		{name: "forced dealer pick with no suit", passCount: 3, activeSeat: 0, suit: nil},
		{name: "forced dealer pick with an unknown suit", passCount: 3, activeSeat: 0, suit: &badSuit},
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

// TestCroatianThirdPassForcesDealer checks the third pass forces the dealer while face-down cards stay hidden.
func TestCroatianThirdPassForcesDealer(t *testing.T) {
	gs := testfixtures.NewGameCroatianMidBidding(2)

	result, err := game.ApplyAction(gs, game.Action{Type: game.ActionPassTrump, PlayerSeat: 3})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, game.PhaseBidding, result.Phase)
	assert.Equal(t, 1, result.BiddingRound, "there is no second round — bidding stays in round 1")
	assert.Equal(t, 3, result.BiddingPassCount)
	assert.Equal(t, result.DealerSeat, result.ActivePlayerSeat, "the dealer speaks last")
	assert.True(t, result.MustPickTrump, "the dealer has no legal pass — pod mus")

	for i, p := range result.Players {
		assert.Len(t, p.Hand, 6, "seat %d's open hand is unchanged by the third pass", i)
		assert.Len(t, p.FaceDownCards, 2, "seat %d's two cards stay OUT of Hand", i)
	}

	for _, suit := range game.AllSuits {
		s := suit
		picked, err := game.ApplyAction(result, game.Action{
			Type:       game.ActionPickTrump,
			PlayerSeat: result.ActivePlayerSeat,
			Suit:       &s,
		})
		require.NoError(t, err, "suit %s must be available to the forced dealer", suit)
		require.NotNil(t, picked.TrumpSuit)
		assert.Equal(t, suit, *picked.TrumpSuit)
	}
}

// TestCroatianDealerCannotPass checks the dealer's pass is rejected after three passes.
func TestCroatianDealerCannotPass(t *testing.T) {
	gs := testfixtures.NewGameCroatianMidBidding(3)
	require.Equal(t, 1, gs.BiddingRound)
	require.Equal(t, 3, gs.BiddingPassCount)
	require.Equal(t, gs.DealerSeat, gs.ActivePlayerSeat, "the dealer bids last")

	result, err := game.ApplyAction(gs, game.Action{
		Type:       game.ActionPassTrump,
		PlayerSeat: gs.DealerSeat,
	})

	assert.Nil(t, result)
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrMustPickTrump)
	assert.NotErrorIs(t, err, apperr.ErrInvalidBid, "must be distinguishable from a bad-suit refusal")
	assert.Equal(t, game.PhaseBidding, gs.Phase, "no round 2, no reshuffle — state is untouched")
	assert.Equal(t, 1, gs.BiddingRound)
	assert.Equal(t, 1, gs.HandNumber)
	assert.Equal(t, 0, gs.DealerSeat, "the dealer does not rotate")

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

// TestDealerMustPickWithCandidateForcesOnlyInRound2 checks that with a candidate the dealer is forced only at the end of round 2.
func TestDealerMustPickWithCandidateForcesOnlyInRound2(t *testing.T) {
	gs := testfixtures.NewGameJustDealt()
	gs.Rules.AllPassOutcome = game.AllPassDealerMustPick

	for i := 0; i < 4; i++ {
		require.False(t, game.MustPickTrump(gs, gs.ActivePlayerSeat),
			"no seat is forced during the candidate round (pass %d)", i+1)
		next, err := game.ApplyAction(gs, game.Action{
			Type:       game.ActionPassTrump,
			PlayerSeat: gs.ActivePlayerSeat,
		})
		require.NoError(t, err, "round-1 pass %d must be legal", i+1)
		gs = next
	}
	require.Equal(t, 2, gs.BiddingRound, "a passed-out candidate round opens round 2")
	require.Equal(t, game.PhaseBidding, gs.Phase)

	for i := 0; i < 3; i++ {
		next, err := game.ApplyAction(gs, game.Action{
			Type:       game.ActionPassTrump,
			PlayerSeat: gs.ActivePlayerSeat,
		})
		require.NoError(t, err, "round-2 pass %d must be legal", i+1)
		gs = next
	}
	require.Equal(t, gs.DealerSeat, gs.ActivePlayerSeat, "the dealer bids last in round 2")
	require.True(t, game.MustPickTrump(gs, gs.ActivePlayerSeat))

	rejected, err := game.ApplyAction(gs, game.Action{
		Type:       game.ActionPassTrump,
		PlayerSeat: gs.DealerSeat,
	})
	assert.Nil(t, rejected)
	require.ErrorIs(t, err, apperr.ErrMustPickTrump)
	assert.Equal(t, game.PhaseBidding, gs.Phase, "no reshuffle — the dealer must pick instead")
}

// TestCroatianEarlierPassesStillLegal guards against the forced-dealer rule
// over-reaching: only the dealer's own (fourth) pass is refused.
func TestCroatianEarlierPassesStillLegal(t *testing.T) {
	tests := []struct {
		name               string
		passCount          int
		activeSeat         int
		expectedRound      int
		expectedPassCount  int
		expectedActiveSeat int
	}{
		{name: "first pass", passCount: 0, activeSeat: 1, expectedRound: 1, expectedPassCount: 1, expectedActiveSeat: 2},
		{name: "second pass", passCount: 1, activeSeat: 2, expectedRound: 1, expectedPassCount: 2, expectedActiveSeat: 3},
		{name: "third pass", passCount: 2, activeSeat: 3, expectedRound: 1, expectedPassCount: 3, expectedActiveSeat: 0},
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

// TestFaceDownCardsNeverSerialized checks no snapshot before bidding resolves contains any seat's face-down cards.
func TestFaceDownCardsNeverSerialized(t *testing.T) {
	tests := []struct {
		name      string
		passCount int
	}{
		{name: "just dealt", passCount: 0},
		{name: "mid bidding", passCount: 2},
		{name: "dealer on the clock after three passes", passCount: 3},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gs := testfixtures.NewGameCroatianMidBidding(tc.passCount)

			data, err := json.Marshal(gs)
			require.NoError(t, err)

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

			assert.NotContains(t, raw, "faceDownCards")
			assert.NotContains(t, raw, "rules")
		})
	}
}

// cardIDsInPayload walks an unmarshalled match_state payload and returns the ID of every card it contains.
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
// hand from NewGame through three passes to the dealer's forced pick, so the
// fixtures' hand-built layout is not the only thing under test.
func TestCroatianFullBiddingFromNewGame(t *testing.T) {
	gs := game.NewGame([4]uint{10, 20, 30, 40}, [4]string{"a", "b", "c", "d"},
		[4]bool{}, game.VariantCroatia, "1001", 1, true, false, true)
	startingCards := collectCards(gs)
	require.Len(t, startingCards, 32)

	gs.Phase = game.PhaseBidding

	for i := 0; i < 3; i++ {
		var err error
		gs, err = game.ApplyAction(gs, game.Action{
			Type:       game.ActionPassTrump,
			PlayerSeat: gs.ActivePlayerSeat,
		})
		require.NoError(t, err, "pass %d", i+1)
		assert.Equal(t, 1, gs.BiddingRound, "Croatian bidding never leaves round 1")
	}
	require.Equal(t, gs.DealerSeat, gs.ActivePlayerSeat, "the dealer speaks last")
	for i, p := range gs.Players {
		assert.Len(t, p.Hand, 6, "seat %d still holds six visible cards", i)
		assert.Len(t, p.FaceDownCards, 2, "seat %d's pair stays hidden through bidding", i)
	}

	rejected, err := game.ApplyAction(gs, game.Action{
		Type:       game.ActionPassTrump,
		PlayerSeat: gs.DealerSeat,
	})
	assert.Nil(t, rejected)
	require.ErrorIs(t, err, apperr.ErrMustPickTrump)
	assert.Equal(t, 1, gs.BiddingRound)

	suit := game.SuitHearts
	final, err := game.ApplyAction(gs, game.Action{
		Type:       game.ActionPickTrump,
		PlayerSeat: gs.DealerSeat,
		Suit:       &suit,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, final.BiddingRound, "round 1 throughout — a second round is unreachable")
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

	require.Equal(t, game.PhaseDeclaring, final.Phase)
	for seat := 0; seat < 4; seat++ {
		require.Equal(t, game.PhaseDeclaring, final.Phase,
			"the phase stays open until every seat has answered (seat %d)", seat)
		require.False(t, final.AwaitingDeclaration, "no seat is ever on the clock here")
		final, err = game.ApplyAction(final, game.Action{
			Type:       game.ActionSkipDeclare,
			PlayerSeat: seat,
		})
		require.NoError(t, err, "skip_declare from seat %d", seat)
	}
	require.NotEqual(t, game.PhaseDeclaring, final.Phase, "the fourth answer closes the phase")
	assert.Equal(t, game.PhasePlaying, final.Phase, "the phase hands off to trick 1")
	assert.Equal(t, 1, final.TrickNumber)
	assert.True(t, final.DeclarationsResolved)
	assert.False(t, final.AwaitingDeclaration, "nothing is owed at trick 1 any more")
	assert.Equal(t, (final.DealerSeat+1)%4, final.ActivePlayerSeat, "the seat after the dealer leads")

	lead := final.Players[final.ActivePlayerSeat].Hand[0]
	played, err := game.ApplyAction(final, game.Action{
		Type:       game.ActionPlayCard,
		PlayerSeat: final.ActivePlayerSeat,
		Card:       &lead,
	})
	require.NoError(t, err)
	assert.Len(t, played.CurrentTrick, 1)
}

// TestMustPickTrumpWireFlag checks the snapshot flag that tells the client whether to offer a Pass control.
func TestMustPickTrumpWireFlag(t *testing.T) {
	t.Run("false through a Croatian hand until the dealer is on the clock", func(t *testing.T) {
		gs := testfixtures.NewGameCroatianJustDealt()
		assert.False(t, gs.MustPickTrump, "nobody is forced on the opening bid")

		for i := 0; i < 3; i++ {
			next, err := game.ApplyAction(gs, game.Action{
				Type:       game.ActionPassTrump,
				PlayerSeat: gs.ActivePlayerSeat,
			})
			require.NoError(t, err, "pass %d must be legal", i+1)
			gs = next
			assert.Equal(t, i == 2, gs.MustPickTrump,
				"after %d passes the flag must be %v", i+1, i == 2)
			assert.Equal(t, 1, gs.BiddingRound, "the flag rises inside round 1")
		}
		assert.Equal(t, gs.DealerSeat, gs.ActivePlayerSeat,
			"the forced seat is the dealer, bidding last")

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
