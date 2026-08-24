package game_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGameStateJSONRoundTrip(t *testing.T) {
	trumpSuit := game.SuitHearts
	callerSeat := 2
	leadSuit := game.SuitSpades
	winnerSeat := 1
	expiry := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)

	original := &game.GameState{
		ID:               1,
		RoomID:           42,
		Variant:          game.VariantBitola,
		MatchMode:        "1001",
		Phase:            game.PhasePlaying,
		HandNumber:       3,
		DealerSeat:       0,
		TrumpSuit:        &trumpSuit,
		TrumpCallerSeat:  &callerSeat,
		TrumpCandidate:   &game.Card{Rank: game.RankKing, Suit: game.SuitHearts},
		BiddingRound:     1,
		BiddingPassCount: 2,
		ActivePlayerSeat: 3,
		TrickNumber:      5,
		CurrentTrick: []game.TrickCard{
			{Card: game.Card{Rank: game.RankAce, Suit: game.SuitSpades}, PlayerSeat: 0},
		},
		LeadSuit:        &leadSuit,
		TrickWinnerSeat: &winnerSeat,
		Players: [4]game.PlayerState{
			{Hand: []game.Card{{Rank: game.RankKing, Suit: game.SuitSpades}}, Seat: 0, UserID: 10, Team: "teamA", Declarations: []game.Declaration{}, Connected: true},
			{Hand: []game.Card{{Rank: game.RankAce, Suit: game.SuitHearts}}, Seat: 1, UserID: 20, Team: "teamB", Declarations: []game.Declaration{}, Connected: true},
			{Hand: []game.Card{{Rank: game.Rank9, Suit: game.SuitDiamonds}}, Seat: 2, UserID: 30, Team: "teamA", Declarations: []game.Declaration{}, Connected: false},
			{Hand: []game.Card{{Rank: game.RankJack, Suit: game.SuitClubs}}, Seat: 3, UserID: 40, Team: "teamB", Declarations: []game.Declaration{}, Connected: true},
		},
		TeamScores:        [2]int{450, 380},
		HandPoints:        [2]int{82, 70},
		DeclarationPoints: [2]int{20, 0},
		TricksWon:         [2]int{5, 2},
		TurnExpiresAt:     &expiry,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored game.GameState
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.Equal(t, original.ID, restored.ID)
	assert.Equal(t, original.RoomID, restored.RoomID)
	assert.Equal(t, original.Variant, restored.Variant)
	assert.Equal(t, original.MatchMode, restored.MatchMode)
	assert.Equal(t, original.Phase, restored.Phase)
	assert.Equal(t, original.HandNumber, restored.HandNumber)
	assert.Equal(t, original.DealerSeat, restored.DealerSeat)
	assert.Equal(t, *original.TrumpSuit, *restored.TrumpSuit)
	assert.Equal(t, *original.TrumpCallerSeat, *restored.TrumpCallerSeat)
	assert.Equal(t, *original.TrumpCandidate, *restored.TrumpCandidate)
	assert.Equal(t, original.BiddingRound, restored.BiddingRound)
	assert.Equal(t, original.BiddingPassCount, restored.BiddingPassCount)
	assert.Equal(t, original.ActivePlayerSeat, restored.ActivePlayerSeat)
	assert.Equal(t, original.TrickNumber, restored.TrickNumber)
	assert.Equal(t, original.CurrentTrick, restored.CurrentTrick)
	assert.Equal(t, *original.LeadSuit, *restored.LeadSuit)
	assert.Equal(t, *original.TrickWinnerSeat, *restored.TrickWinnerSeat)
	assert.Equal(t, original.Players, restored.Players)
	assert.Equal(t, original.TeamScores, restored.TeamScores)
	assert.Equal(t, original.HandPoints, restored.HandPoints)
	assert.Equal(t, original.DeclarationPoints, restored.DeclarationPoints)
	assert.Equal(t, original.TricksWon, restored.TricksWon)
	assert.True(t, original.TurnExpiresAt.Equal(*restored.TurnExpiresAt))
}

func TestGameStateJSONNullOptionalFields(t *testing.T) {
	gs := &game.GameState{
		Phase:   game.PhaseBidding,
		Players: [4]game.PlayerState{},
	}

	data, err := json.Marshal(gs)
	require.NoError(t, err)

	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	t.Run("null pointer fields serialize as null", func(t *testing.T) {
		assert.Nil(t, raw["trumpSuit"])
		assert.Nil(t, raw["trumpCallerSeat"])
		assert.Nil(t, raw["trumpCandidate"])
		assert.Nil(t, raw["leadSuit"])
		assert.Nil(t, raw["trickWinnerSeat"])
		assert.Nil(t, raw["turnExpiresAt"])
	})
}

func TestGameStateJSONCamelCaseKeys(t *testing.T) {
	gs := &game.GameState{
		RoomID:           1,
		MatchMode:        "1001",
		HandNumber:       1,
		DealerSeat:       0,
		ActivePlayerSeat: 1,
		BiddingRound:     1,
		BiddingPassCount: 0,
		TrickNumber:      0,
	}

	data, err := json.Marshal(gs)
	require.NoError(t, err)

	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	expectedKeys := []string{
		"id", "roomId", "variant", "matchMode", "phase",
		"handNumber", "dealerSeat", "trumpSuit", "trumpCallerSeat",
		// "deck" left this list with Story 12.10: GameState.Deck is json:"-" —
		// the 11 held-back cards are hidden information and never serialize.
		"trumpCandidate", "biddingRound", "biddingPassCount", "activePlayerSeat",
		"trickNumber", "currentTrick", "leadSuit", "trickWinnerSeat",
		"players", "teamScores", "handPoints", "declarationPoints", "tricksWon",
		"turnExpiresAt",
	}

	for _, key := range expectedKeys {
		_, exists := raw[key]
		assert.True(t, exists, "expected camelCase key %q in JSON output", key)
	}
}

func TestNewGame(t *testing.T) {
	playerIDs := [4]uint{10, 20, 30, 40}
	usernames := [4]string{"alice", "bob", "carol", "dave"}
	gs := game.NewGame(playerIDs, usernames, [4]bool{}, game.VariantBitola, "1001", 42, true)

	t.Run("sets match metadata", func(t *testing.T) {
		assert.Equal(t, uint(42), gs.RoomID)
		assert.Equal(t, game.VariantBitola, gs.Variant)
		assert.Equal(t, "1001", gs.MatchMode)
	})

	t.Run("phase is dealing", func(t *testing.T) {
		assert.Equal(t, game.PhaseDealing, gs.Phase)
	})

	t.Run("active player is the seat after the dealer", func(t *testing.T) {
		assert.Equal(t, (gs.DealerSeat+1)%4, gs.ActivePlayerSeat)
	})

	t.Run("first dealer is drawn roughly uniformly across all four seats", func(t *testing.T) {
		// A "seat appears at least once" check is too weak: a badly skewed draw
		// (say 70/10/10/10) would satisfy it while quietly handing one seat most
		// of the deals, which matters because coin buy-in matches settle real
		// stakes. So assert every seat's SHARE too.
		//
		// 4000 draws, expected 1000 per seat, sd = sqrt(4000*0.25*0.75) ~= 27.
		// The +/-20% band below sits about 7 sd out, so a uniform source
		// effectively never trips it, while any material skew does.
		const draws = 4000
		var counts [4]int
		for range draws {
			g := game.NewGame(playerIDs, usernames, [4]bool{}, game.VariantBitola, "1001", 42, true)
			require.GreaterOrEqual(t, g.DealerSeat, 0)
			require.LessOrEqual(t, g.DealerSeat, 3)
			require.Equal(t, (g.DealerSeat+1)%4, g.ActivePlayerSeat,
				"opening bidder is always derived from the dealer")
			counts[g.DealerSeat]++
		}
		for seat, n := range counts {
			assert.Greater(t, n, draws/4*80/100,
				"seat %d is dealt too rarely to be a uniform draw (%d of %d)", seat, n, draws)
			assert.Less(t, n, draws/4*120/100,
				"seat %d is dealt too often to be a uniform draw (%d of %d)", seat, n, draws)
		}
	})

	t.Run("hand number is 1", func(t *testing.T) {
		assert.Equal(t, 1, gs.HandNumber)
	})

	t.Run("bidding round is 1", func(t *testing.T) {
		assert.Equal(t, 1, gs.BiddingRound)
	})

	t.Run("each player holds exactly 5 cards (stage-1)", func(t *testing.T) {
		for i, p := range gs.Players {
			assert.Len(t, p.Hand, 5, "player at seat %d should have 5 cards after stage-1", i)
		}
	})

	t.Run("Deck holds 11 cards for stage-2", func(t *testing.T) {
		assert.Len(t, gs.Deck, 11, "stage-1 leaves 11 cards undealt for stage-2")
	})

	t.Run("all 32 cards accounted for across hands + deck + candidate", func(t *testing.T) {
		seen := make(map[string]bool)
		for _, p := range gs.Players {
			for _, card := range p.Hand {
				id := card.String()
				assert.False(t, seen[id], "duplicate card: %s", id)
				seen[id] = true
			}
		}
		for _, card := range gs.Deck {
			id := card.String()
			assert.False(t, seen[id], "duplicate card in deck: %s", id)
			seen[id] = true
		}
		require.NotNil(t, gs.TrumpCandidate)
		seen[gs.TrumpCandidate.String()] = true
		assert.Len(t, seen, 32)
	})

	t.Run("trump candidate is set and not in any hand or deck", func(t *testing.T) {
		require.NotNil(t, gs.TrumpCandidate)
		assert.NotEmpty(t, gs.TrumpCandidate.Rank)
		assert.NotEmpty(t, gs.TrumpCandidate.Suit)

		candidateID := gs.TrumpCandidate.String()
		for _, p := range gs.Players {
			for _, card := range p.Hand {
				assert.NotEqual(t, candidateID, card.String(),
					"candidate must NOT be in any hand during stage-1")
			}
		}
		for _, card := range gs.Deck {
			assert.NotEqual(t, candidateID, card.String(),
				"candidate must NOT be in deck during stage-1")
		}
	})

	t.Run("teams are assigned correctly", func(t *testing.T) {
		assert.Equal(t, "teamA", gs.Players[0].Team)
		assert.Equal(t, "teamB", gs.Players[1].Team)
		assert.Equal(t, "teamA", gs.Players[2].Team)
		assert.Equal(t, "teamB", gs.Players[3].Team)
	})

	t.Run("player IDs assigned to correct seats", func(t *testing.T) {
		for i, id := range playerIDs {
			assert.Equal(t, id, gs.Players[i].UserID)
			assert.Equal(t, i, gs.Players[i].Seat)
		}
	})

	t.Run("all players connected", func(t *testing.T) {
		for i, p := range gs.Players {
			assert.True(t, p.Connected, "player at seat %d should be connected", i)
		}
	})

	t.Run("scores initialized to zero", func(t *testing.T) {
		assert.Equal(t, [2]int{0, 0}, gs.TeamScores)
		assert.Equal(t, [2]int{0, 0}, gs.HandPoints)
		assert.Equal(t, [2]int{0, 0}, gs.DeclarationPoints)
		assert.Equal(t, [2]int{0, 0}, gs.TricksWon)
	})
}

func TestTeamForSeat(t *testing.T) {
	tests := []struct {
		seat     int
		expected int
	}{
		{0, game.TeamA},
		{1, game.TeamB},
		{2, game.TeamA},
		{3, game.TeamB},
	}

	for _, tc := range tests {
		t.Run("seat_"+string(rune('0'+tc.seat)), func(t *testing.T) {
			assert.Equal(t, tc.expected, game.TeamForSeat(tc.seat))
		})
	}
}

func TestShuffleDeck(t *testing.T) {
	t.Run("preserves all 32 cards", func(t *testing.T) {
		deck := game.NewDeck()
		game.ShuffleDeck(deck)

		assert.Len(t, deck, 32)
		seen := make(map[string]bool)
		for _, card := range deck {
			id := card.String()
			assert.False(t, seen[id], "duplicate card after shuffle: %s", id)
			seen[id] = true
		}
		assert.Len(t, seen, 32)
	})

	t.Run("produces different orderings", func(t *testing.T) {
		deck1 := game.NewDeck()
		deck2 := game.NewDeck()

		game.ShuffleDeck(deck1)
		game.ShuffleDeck(deck2)

		differences := 0
		for i := range deck1 {
			if deck1[i] != deck2[i] {
				differences++
			}
		}
		assert.Greater(t, differences, 0, "two shuffles should produce different orderings")
	})
}

func TestTeamStringForIndex_SafeOnOutOfRange(t *testing.T) {
	tests := []struct {
		name  string
		index int
	}{
		{"negative", -1},
		{"two", 2},
		{"large", 100},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := game.TeamStringForIndex(tc.index)
			assert.Equal(t, "", result, "out-of-range index should return empty string, not panic")
		})
	}
}

// TestRulesForPresets locks D-VAR-1's foundation: both presets return a fully
// populated config, an unknown variant string falls back to Bitola, and the two
// presets actually differ on the six divergences.
//
// DeclarationsEnabled is the one field exempt from that last assertion, and the
// exemption has its own subtest: it is a per-room setting layered over the
// preset by NewGame, not a variant divergence, so both presets return true.
func TestRulesForPresets(t *testing.T) {
	bitola := game.VariantRules{
		DealShape:           game.DealShapeCandidate,
		HasTrumpCandidate:   true,
		AllPassOutcome:      game.AllPassReshuffleAndRotate,
		DeclarationOverlap:  false,
		DeclarationTiming:   game.DeclarationTimingDuringFirstTrick,
		DeclarationsEnabled: true,
		TieRule:             game.TieRuleHangingPoints,
	}
	croatia := game.VariantRules{
		DealShape:           game.DealShapeAllBeforeBidding,
		HasTrumpCandidate:   false,
		AllPassOutcome:      game.AllPassDealerMustPick,
		DeclarationOverlap:  true,
		DeclarationTiming:   game.DeclarationTimingDedicatedPhase,
		DeclarationsEnabled: true,
		TieRule:             game.TieRuleAllToOpponents,
	}

	tests := []struct {
		name     string
		variant  game.Variant
		expected game.VariantRules
	}{
		{name: "bitola resolves to the bitola preset", variant: game.VariantBitola, expected: bitola},
		{name: "croatia resolves to the croatia preset", variant: game.VariantCroatia, expected: croatia},
		{name: "an unknown variant string falls back to bitola", variant: game.Variant("atlantis"), expected: bitola},
		{name: "an empty variant string falls back to bitola", variant: game.Variant(""), expected: bitola},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rules := game.RulesFor(tc.variant)
			assert.Equal(t, tc.expected, rules)

			// Fully populated: no named-string field may be left at its zero
			// value. The two booleans are legitimately false in one preset each,
			// so they are covered by the exact-equality assertion above.
			assert.NotEmpty(t, rules.DealShape, "DealShape must be populated")
			assert.NotEmpty(t, rules.AllPassOutcome, "AllPassOutcome must be populated")
			assert.NotEmpty(t, rules.DeclarationTiming, "DeclarationTiming must be populated")
			assert.NotEmpty(t, rules.TieRule, "TieRule must be populated")
		})
	}

	t.Run("the two presets differ on every divergence", func(t *testing.T) {
		assert.NotEqual(t, bitola.DealShape, croatia.DealShape)
		assert.NotEqual(t, bitola.HasTrumpCandidate, croatia.HasTrumpCandidate)
		assert.NotEqual(t, bitola.AllPassOutcome, croatia.AllPassOutcome)
		assert.NotEqual(t, bitola.DeclarationOverlap, croatia.DeclarationOverlap)
		assert.NotEqual(t, bitola.DeclarationTiming, croatia.DeclarationTiming)
		assert.NotEqual(t, bitola.TieRule, croatia.TieRule)
	})

	t.Run("declarations-enabled is a room setting, so both presets AGREE on it", func(t *testing.T) {
		// The deliberate exception to the divergence assertion above, asserted
		// rather than left as an absence. It is not a variant property: both
		// variants play with declarations by default, and only NewGame overrides
		// it from the room. A preset that returned false here would give one
		// variant a rule the owner never chose.
		assert.True(t, bitola.DeclarationsEnabled)
		assert.True(t, croatia.DeclarationsEnabled)
	})

	t.Run("the zero-value config is NOT the bitola preset", func(t *testing.T) {
		// This is why every GameState literal must set Rules explicitly — an
		// unset config would deal no candidate and reject every round-1 take.
		assert.NotEqual(t, bitola, game.VariantRules{})
	})
}

// TestNewGameResolvesRulesOnce asserts the config is stamped on the state at
// construction, for every variant string NewGame can be handed.
func TestNewGameResolvesRulesOnce(t *testing.T) {
	tests := []struct {
		name    string
		variant game.Variant
	}{
		{name: "bitola", variant: game.VariantBitola},
		{name: "croatia", variant: game.VariantCroatia},
		{name: "unknown falls back to bitola", variant: game.Variant("nonsense")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gs := game.NewGame([4]uint{10, 20, 30, 40}, [4]string{"a", "b", "c", "d"},
				[4]bool{}, tc.variant, "1001", 1, true)
			assert.Equal(t, game.RulesFor(tc.variant), gs.Rules)
			assert.Equal(t, tc.variant, gs.Variant, "the variant string is stored verbatim")
		})
	}
}

// TestNewGameCroatianDeal covers the Croatian-deal row of the I/O matrix: eight
// cards per seat (six open, two face-down), no candidate, empty deck, and 32-card
// conservation.
func TestNewGameCroatianDeal(t *testing.T) {
	gs := game.NewGame([4]uint{10, 20, 30, 40}, [4]string{"a", "b", "c", "d"},
		[4]bool{}, game.VariantCroatia, "1001", 7, true)

	t.Run("six open cards and two face-down per seat", func(t *testing.T) {
		for i, p := range gs.Players {
			assert.Len(t, p.Hand, 6, "seat %d holds 6 open cards before bidding", i)
			assert.Len(t, p.FaceDownCards, 2, "seat %d holds 2 face-down cards", i)
		}
	})

	t.Run("no trump candidate and no stage-2 reserve", func(t *testing.T) {
		assert.Nil(t, gs.TrumpCandidate, "there is no candidate in this deal shape")
		assert.Empty(t, gs.Deck, "every card is dealt before bidding")
	})

	// FaceDownCount is the only public trace of the hidden pair, and it is what
	// makes an opponent's stack render as 8 instead of 6. Asserted here on a real
	// NewGame rather than on a fixture, because the fixtures author the count and
	// would still pass with the derivation deleted from the deal itself.
	t.Run("the public face-down count is 2 per seat", func(t *testing.T) {
		for i, p := range gs.Players {
			assert.Equal(t, 2, p.FaceDownCount, "seat %d", i)
			assert.Equal(t, len(p.FaceDownCards), p.FaceDownCount,
				"seat %d: the count must equal the cards it counts", i)
		}
	})

	t.Run("the count survives to the wire and drops to 0 when the pair merges", func(t *testing.T) {
		data, err := json.Marshal(gs)
		require.NoError(t, err)
		var raw map[string]any
		require.NoError(t, json.Unmarshal(data, &raw))
		players, ok := raw["players"].([]any)
		require.True(t, ok)
		require.Len(t, players, 4)
		for i, p := range players {
			seat, ok := p.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, float64(2), seat["faceDownCount"], "seat %d on the wire", i)
		}

		// Resolving the bid merges every pair into its owner's hand, so the count
		// must go back to 0 — a stale 2 would render every stack as 10. Built on
		// its own deal: `gs` is a pointer shared with the sibling subtests, and
		// flipping its phase here would leak into them.
		bidding := game.NewGame([4]uint{10, 20, 30, 40}, [4]string{"a", "b", "c", "d"},
			[4]bool{}, game.VariantCroatia, "1001", 7, true)
		bidding.Phase = game.PhaseBidding
		suit := game.SuitSpades
		resolved, err := game.ApplyAction(bidding, game.Action{
			Type:       game.ActionPickTrump,
			PlayerSeat: bidding.ActivePlayerSeat,
			Suit:       &suit,
		})
		require.NoError(t, err)
		for i, p := range resolved.Players {
			assert.Zero(t, p.FaceDownCount, "seat %d after the merge", i)
			assert.Len(t, p.Hand, 8, "seat %d holds all eight once bidding resolves", i)
		}
	})

	t.Run("all 32 cards accounted for exactly once", func(t *testing.T) {
		seen := make(map[string]bool, 32)
		for _, p := range gs.Players {
			for _, c := range append(append([]game.Card{}, p.Hand...), p.FaceDownCards...) {
				id := c.String()
				assert.False(t, seen[id], "duplicate card: %s", id)
				seen[id] = true
			}
		}
		assert.Len(t, seen, 32)
	})

	t.Run("a face-down card is never also in an open hand", func(t *testing.T) {
		open := make(map[string]bool, 24)
		for _, p := range gs.Players {
			for _, c := range p.Hand {
				open[c.String()] = true
			}
		}
		for i, p := range gs.Players {
			for _, c := range p.FaceDownCards {
				assert.False(t, open[c.String()],
					"seat %d's face-down %s must not also sit in an open hand", i, c)
			}
		}
	})

	t.Run("phase and opening bidder match the Bitola deal", func(t *testing.T) {
		assert.Equal(t, game.PhaseDealing, gs.Phase)
		assert.Equal(t, (gs.DealerSeat+1)%4, gs.ActivePlayerSeat)
		assert.Equal(t, 1, gs.BiddingRound)
		assert.Equal(t, 1, gs.HandNumber)
	})
}

// TestGameStateJSONOmitsServerOnlyRuleFields is the wire-shape guard for the
// fields this story added: none of them may appear on the wire, for either
// variant.
func TestGameStateJSONOmitsServerOnlyRuleFields(t *testing.T) {
	tests := []struct {
		name    string
		variant game.Variant
	}{
		{name: "bitola", variant: game.VariantBitola},
		{name: "croatia", variant: game.VariantCroatia},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gs := game.NewGame([4]uint{10, 20, 30, 40}, [4]string{"a", "b", "c", "d"},
				[4]bool{}, tc.variant, "1001", 1, true)

			data, err := json.Marshal(gs)
			require.NoError(t, err)

			var raw map[string]any
			require.NoError(t, json.Unmarshal(data, &raw))
			for _, key := range []string{"rules", "Rules"} {
				_, exists := raw[key]
				assert.False(t, exists, "server-only field %q must not reach the wire", key)
			}

			players, ok := raw["players"].([]any)
			require.True(t, ok)
			require.Len(t, players, 4)
			for i, p := range players {
				seat, ok := p.(map[string]any)
				require.True(t, ok)
				for _, key := range []string{"faceDownCards", "FaceDownCards"} {
					_, exists := seat[key]
					assert.False(t, exists, "seat %d's %q must not reach the wire", i, key)
				}
				// The COUNT does ride the wire — it is the public half. Zero for
				// a deal shape that holds nothing back, so a Bitola client can
				// add it unconditionally.
				want := float64(0)
				if tc.variant == game.VariantCroatia {
					want = 2
				}
				assert.Equal(t, want, seat["faceDownCount"], "seat %d's faceDownCount", i)
			}
		})
	}
}
