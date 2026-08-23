package game

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Internal tests for unexported checkInstantWin function.
// These tests use deterministic card distributions to verify instant-win detection.

func TestCheckInstantWin_AllTrumpCards(t *testing.T) {
	trumpCandidate := Card{Rank: Rank7, Suit: SuitHearts}
	gs := &GameState{
		TrumpCandidate: &trumpCandidate,
		Players: [4]PlayerState{
			{Seat: 0, Hand: []Card{
				{Rank: Rank7, Suit: SuitSpades}, {Rank: Rank8, Suit: SuitSpades},
				{Rank: Rank9, Suit: SuitSpades}, {Rank: RankTen, Suit: SuitSpades},
				{Rank: RankJack, Suit: SuitSpades}, {Rank: RankQueen, Suit: SuitSpades},
				{Rank: RankKing, Suit: SuitSpades}, {Rank: RankAce, Suit: SuitSpades},
			}},
			{Seat: 1, Hand: []Card{
				{Rank: Rank7, Suit: SuitHearts}, {Rank: Rank8, Suit: SuitHearts},
				{Rank: Rank9, Suit: SuitHearts}, {Rank: RankTen, Suit: SuitHearts},
				{Rank: RankJack, Suit: SuitHearts}, {Rank: RankQueen, Suit: SuitHearts},
				{Rank: RankKing, Suit: SuitHearts}, {Rank: RankAce, Suit: SuitHearts},
			}},
			{Seat: 2, Hand: []Card{
				{Rank: Rank7, Suit: SuitDiamonds}, {Rank: Rank8, Suit: SuitDiamonds},
				{Rank: Rank9, Suit: SuitDiamonds}, {Rank: RankTen, Suit: SuitDiamonds},
				{Rank: RankJack, Suit: SuitDiamonds}, {Rank: RankQueen, Suit: SuitDiamonds},
				{Rank: RankKing, Suit: SuitDiamonds}, {Rank: RankAce, Suit: SuitDiamonds},
			}},
			{Seat: 3, Hand: []Card{
				{Rank: Rank7, Suit: SuitClubs}, {Rank: Rank8, Suit: SuitClubs},
				{Rank: Rank9, Suit: SuitClubs}, {Rank: RankTen, Suit: SuitClubs},
				{Rank: RankJack, Suit: SuitClubs}, {Rank: RankQueen, Suit: SuitClubs},
				{Rank: RankKing, Suit: SuitClubs}, {Rank: RankAce, Suit: SuitClubs},
			}},
		},
	}

	result := checkInstantWin(gs)

	require.NotNil(t, result, "should detect instant-win")
	assert.Equal(t, TeamB, *result, "seat 1 (team B) has all 8 Hearts (trump)")
}

func TestCheckInstantWin_NoInstantWin(t *testing.T) {
	trumpCandidate := Card{Rank: Rank7, Suit: SuitHearts}
	gs := &GameState{
		TrumpCandidate: &trumpCandidate,
		Players: [4]PlayerState{
			{Seat: 0, Hand: []Card{
				{Rank: Rank7, Suit: SuitHearts}, // 1 heart on seat 0
				{Rank: Rank8, Suit: SuitSpades}, {Rank: Rank9, Suit: SuitSpades},
				{Rank: RankTen, Suit: SuitSpades}, {Rank: RankJack, Suit: SuitSpades},
				{Rank: RankQueen, Suit: SuitSpades}, {Rank: RankKing, Suit: SuitSpades},
				{Rank: RankAce, Suit: SuitSpades},
			}},
			{Seat: 1, Hand: []Card{
				{Rank: Rank7, Suit: SuitSpades}, // 1 spade mixed in
				{Rank: Rank8, Suit: SuitHearts}, {Rank: Rank9, Suit: SuitHearts},
				{Rank: RankTen, Suit: SuitHearts}, {Rank: RankJack, Suit: SuitHearts},
				{Rank: RankQueen, Suit: SuitHearts}, {Rank: RankKing, Suit: SuitHearts},
				{Rank: RankAce, Suit: SuitHearts},
			}},
			{Seat: 2, Hand: []Card{
				{Rank: Rank7, Suit: SuitDiamonds}, {Rank: Rank8, Suit: SuitDiamonds},
				{Rank: Rank9, Suit: SuitDiamonds}, {Rank: RankTen, Suit: SuitDiamonds},
				{Rank: RankJack, Suit: SuitDiamonds}, {Rank: RankQueen, Suit: SuitDiamonds},
				{Rank: RankKing, Suit: SuitDiamonds}, {Rank: RankAce, Suit: SuitDiamonds},
			}},
			{Seat: 3, Hand: []Card{
				{Rank: Rank7, Suit: SuitClubs}, {Rank: Rank8, Suit: SuitClubs},
				{Rank: Rank9, Suit: SuitClubs}, {Rank: RankTen, Suit: SuitClubs},
				{Rank: RankJack, Suit: SuitClubs}, {Rank: RankQueen, Suit: SuitClubs},
				{Rank: RankKing, Suit: SuitClubs}, {Rank: RankAce, Suit: SuitClubs},
			}},
		},
	}

	result := checkInstantWin(gs)
	assert.Nil(t, result, "no player has all 8 trump cards")
}

func TestCheckInstantWin_NilTrumpCandidate(t *testing.T) {
	gs := &GameState{
		TrumpCandidate: nil,
	}

	result := checkInstantWin(gs)
	assert.Nil(t, result, "should return nil when TrumpCandidate is nil")
}

func TestCheckInstantWin_Seat0TeamA(t *testing.T) {
	// Verify correct team is returned when seat 0 (team A) has all trump
	trumpCandidate := Card{Rank: Rank7, Suit: SuitSpades}
	gs := &GameState{
		TrumpCandidate: &trumpCandidate,
		Players: [4]PlayerState{
			{Seat: 0, Hand: []Card{
				{Rank: Rank7, Suit: SuitSpades}, {Rank: Rank8, Suit: SuitSpades},
				{Rank: Rank9, Suit: SuitSpades}, {Rank: RankTen, Suit: SuitSpades},
				{Rank: RankJack, Suit: SuitSpades}, {Rank: RankQueen, Suit: SuitSpades},
				{Rank: RankKing, Suit: SuitSpades}, {Rank: RankAce, Suit: SuitSpades},
			}},
			{Seat: 1, Hand: []Card{{Rank: Rank7, Suit: SuitHearts}}},
			{Seat: 2, Hand: []Card{{Rank: Rank7, Suit: SuitDiamonds}}},
			{Seat: 3, Hand: []Card{{Rank: Rank7, Suit: SuitClubs}}},
		},
	}

	result := checkInstantWin(gs)

	require.NotNil(t, result)
	assert.Equal(t, TeamA, *result, "seat 0 (team A) has all 8 Spades (trump)")
}

// TestStartNewHandDealsPerConfig locks that hand 2 onwards is dealt the same way
// hand 1 was. dealCards has three callers and only one is NewGame, so a
// config-driven deal that is only wired into NewGame would silently deal hand 2
// of a Croatian match Bitola-style.
func TestStartNewHandDealsPerConfig(t *testing.T) {
	tests := []struct {
		name          string
		variant       Variant
		wantOpen      int
		wantFaceDown  int
		wantDeck      int
		wantCandidate bool
	}{
		{
			name:    "bitola re-deals stage-1 with a fresh candidate",
			variant: VariantBitola, wantOpen: 5, wantFaceDown: 0, wantDeck: 11, wantCandidate: true,
		},
		{
			name: "croatia re-deals everything before bidding", variant: VariantCroatia,
			wantOpen: 6, wantFaceDown: 2, wantDeck: 0, wantCandidate: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gs := NewGame([4]uint{10, 20, 30, 40}, [4]string{"a", "b", "c", "d"},
				[4]bool{}, tc.variant, "1001", 1)
			dealer := gs.DealerSeat

			startNewHand(gs)

			assert.Equal(t, 2, gs.HandNumber)
			assert.Equal(t, (dealer+1)%4, gs.DealerSeat, "dealer rotates")
			assert.Equal(t, tc.wantCandidate, gs.TrumpCandidate != nil)
			assert.Len(t, gs.Deck, tc.wantDeck)

			seen := make(map[string]bool, 32)
			for i, p := range gs.Players {
				assert.Len(t, p.Hand, tc.wantOpen, "seat %d open cards", i)
				assert.Len(t, p.FaceDownCards, tc.wantFaceDown, "seat %d face-down cards", i)
				for _, c := range p.Hand {
					seen[c.String()] = true
				}
				for _, c := range p.FaceDownCards {
					seen[c.String()] = true
				}
			}
			for _, c := range gs.Deck {
				seen[c.String()] = true
			}
			if gs.TrumpCandidate != nil {
				seen[gs.TrumpCandidate.String()] = true
			}
			assert.Len(t, seen, 32, "all 32 cards present exactly once after the re-deal")
		})
	}
}

// allCardsInPlay returns every card the state holds anywhere: open hands,
// face-down slots, the undealt reserve, and the public candidate.
func allCardsInPlay(state *GameState) []Card {
	out := make([]Card, 0, 32)
	for i := range state.Players {
		out = append(out, state.Players[i].Hand...)
		out = append(out, state.Players[i].FaceDownCards...)
	}
	out = append(out, state.Deck...)
	if state.TrumpCandidate != nil {
		out = append(out, *state.TrumpCandidate)
	}
	return out
}

// TestReshuffleAndRedealRecoversFaceDownCards covers the face-down recovery in
// reshuffleAndRedeal's pooling loop. The guard exists because a short pool makes
// dealCards fall back to a fresh deck, and its own comment names the failure it
// prevents — so it needs a test rather than a reachability argument.
//
// A state carrying face-down cards while resolving to a reshuffle outcome is
// malformed (the config that deals them refuses the dealer's fourth pass, so
// its bidding can never pass out), which is exactly the situation the guard is
// defending.
func TestReshuffleAndRedealRecoversFaceDownCards(t *testing.T) {
	// buildPooled lays a given 32-card slice out as 6 open + 2 face-down per
	// seat under a reshuffle-and-rotate config.
	buildPooled := func(cards []Card) *GameState {
		require.Len(t, cards, 32)
		gs := &GameState{
			Variant:    VariantBitola,
			Rules:      RulesFor(VariantBitola),
			Phase:      PhaseBidding,
			DealerSeat: 0,
		}
		idx := 0
		for seat := range gs.Players {
			gs.Players[seat].Seat = seat
			gs.Players[seat].Hand = append([]Card{}, cards[idx:idx+6]...)
			idx += 6
		}
		for seat := range gs.Players {
			gs.Players[seat].FaceDownCards = append([]Card{}, cards[idx:idx+2]...)
			idx += 2
		}
		return gs
	}

	t.Run("the re-dealt state holds 32 cards with no duplicates", func(t *testing.T) {
		// Without the recovery, the 8 face-down cards are neither pooled nor
		// cleared: the short pool triggers a fresh deck AND the stale hidden
		// cards survive on the seats, so the state ends up holding 40 entries
		// with duplicates.
		gs := buildPooled(NewDeck())

		result := reshuffleAndRedeal(gs)

		cards := allCardsInPlay(result)
		require.Len(t, cards, 32, "the re-dealt state must hold exactly 32 cards")
		seen := make(map[string]bool, 32)
		for _, c := range cards {
			assert.False(t, seen[c.String()], "duplicate card after reshuffle: %s", c)
			seen[c.String()] = true
		}
		assert.Len(t, seen, 32)
	})

	t.Run("the hidden slots are cleared before the re-deal", func(t *testing.T) {
		gs := buildPooled(NewDeck())

		result := reshuffleAndRedeal(gs)

		for i, p := range result.Players {
			assert.Empty(t, p.FaceDownCards,
				"seat %d's hidden slot must be cleared — the candidate deal shape never refills it", i)
			assert.Len(t, p.Hand, 5, "seat %d gets a normal stage-1 hand", i)
		}
		assert.Len(t, result.Deck, 11)
		require.NotNil(t, result.TrumpCandidate)
	})

	t.Run("the pool is the state's own cards, not a fresh deck", func(t *testing.T) {
		// Mark the pool so the two sources are distinguishable: drop the Ace of
		// Spades and duplicate a face-down card in its place. The pool still has
		// 32 cards, so the recovery path deals THESE cards — and the Ace of
		// Spades stays absent. If the face-down cards were not pooled, the pool
		// would be 24, dealCards would fall back to NewDeck, and the Ace of
		// Spades would reappear.
		cards := NewDeck()
		var marker Card
		for i, c := range cards {
			if c.Rank == RankAce && c.Suit == SuitSpades {
				// cards[24:32] become the face-down slots in buildPooled.
				marker = cards[24]
				cards[i] = marker
				break
			}
		}
		require.NotEqual(t, Card{}, marker, "the marker card must have been chosen")

		gs := buildPooled(cards)

		result := reshuffleAndRedeal(gs)

		inPlay := allCardsInPlay(result)
		require.Len(t, inPlay, 32)
		for _, c := range inPlay {
			assert.False(t, c.Rank == RankAce && c.Suit == SuitSpades,
				"the Ace of Spades was not in the pool, so a fresh deck must not have been substituted")
		}
		markerCount := 0
		for _, c := range inPlay {
			if c == marker {
				markerCount++
			}
		}
		assert.Equal(t, 2, markerCount,
			"the marked duplicate proves the pool came from the state's own cards")
	})
}

// TestFaceDownCountTracksCardsAcrossHands pins the invariant syncFaceDownCounts
// documents but nothing enforced: the public FaceDownCount never diverges from
// the server-only FaceDownCards it counts.
//
// It matters at the HAND BOUNDARY specifically. startNewHand and
// reshuffleAndRedeal both clear FaceDownCards themselves and are covered only
// transitively, by falling through to dealCards — so an early return added to
// either one before its dealCards call would ship the previous hand's count with
// no cards behind it, rendering a wrong stack size on every opponent's seat with
// no test to catch it. This is an internal test because startNewHand is the
// unexported entry point that boundary runs through.
func TestFaceDownCountTracksCardsAcrossHands(t *testing.T) {
	assertInSync := func(t *testing.T, state *GameState, want int, when string) {
		t.Helper()
		for seat := range state.Players {
			p := state.Players[seat]
			assert.Equal(t, len(p.FaceDownCards), p.FaceDownCount,
				"seat %d %s: the wire count must equal the cards behind it", seat, when)
			assert.Equal(t, want, p.FaceDownCount, "seat %d %s", seat, when)
		}
	}

	t.Run("croatian deal shape", func(t *testing.T) {
		state := NewGame([4]uint{1, 2, 3, 4}, [4]string{"a", "b", "c", "d"},
			[4]bool{}, VariantCroatia, "1001", 1)
		assertInSync(t, state, 2, "on the opening deal")

		// Bidding resolves: the pair merges into Hand and the count must follow.
		state.Phase = PhaseBidding
		suit := SuitHearts
		picked, err := ApplyAction(state, Action{
			Type: ActionPickTrump, PlayerSeat: state.ActivePlayerSeat, Suit: &suit,
		})
		require.NoError(t, err)
		assertInSync(t, picked, 0, "once bidding resolved")
		for seat := range picked.Players {
			require.Len(t, picked.Players[seat].Hand, 8, "seat %d holds all eight after the merge", seat)
		}

		// Hand 2 is dealt by the same path a scored hand takes.
		startNewHand(picked)
		assertInSync(t, picked, 2, "after startNewHand dealt hand 2")
	})

	t.Run("candidate deal shape never carries a count", func(t *testing.T) {
		state := NewGame([4]uint{1, 2, 3, 4}, [4]string{"a", "b", "c", "d"},
			[4]bool{}, VariantBitola, "1001", 1)
		assertInSync(t, state, 0, "on the opening deal")

		// Four round-2 passes reshuffle and re-deal — the other transitively
		// covered path.
		state.Phase = PhaseBidding
		state.BiddingRound = 2
		state.BiddingPassCount = 3
		reshuffled, err := ApplyAction(state, Action{
			Type: ActionPassTrump, PlayerSeat: state.ActivePlayerSeat,
		})
		require.NoError(t, err)
		require.Equal(t, PhaseDealing, reshuffled.Phase, "the fourth round-2 pass re-deals")
		assertInSync(t, reshuffled, 0, "after the reshuffle")

		startNewHand(reshuffled)
		assertInSync(t, reshuffled, 0, "after startNewHand dealt hand 2")
	})
}
