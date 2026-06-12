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
		"trumpCandidate", "biddingRound", "biddingPassCount", "deck", "activePlayerSeat",
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
	gs := game.NewGame(playerIDs, usernames, [4]bool{}, game.VariantBitola, "1001", 42)

	t.Run("sets match metadata", func(t *testing.T) {
		assert.Equal(t, uint(42), gs.RoomID)
		assert.Equal(t, game.VariantBitola, gs.Variant)
		assert.Equal(t, "1001", gs.MatchMode)
	})

	t.Run("phase is dealing", func(t *testing.T) {
		assert.Equal(t, game.PhaseDealing, gs.Phase)
	})

	t.Run("dealer is seat 0 for first hand", func(t *testing.T) {
		assert.Equal(t, 0, gs.DealerSeat)
	})

	t.Run("active player is seat 1 (after dealer)", func(t *testing.T) {
		assert.Equal(t, 1, gs.ActivePlayerSeat)
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
