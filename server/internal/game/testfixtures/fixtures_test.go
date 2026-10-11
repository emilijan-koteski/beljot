package testfixtures_test

import (
	"testing"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGameMidBidding(t *testing.T) {
	tests := []struct {
		name              string
		passCount         int
		expectedRound     int
		expectedPassCount int
		expectedActive    int
	}{
		{
			name:              "passCount 0 matches NewGameJustDealt",
			passCount:         0,
			expectedRound:     1,
			expectedPassCount: 0,
			expectedActive:    1,
		},
		{
			name:              "passCount 1 - round 1 with 1 pass",
			passCount:         1,
			expectedRound:     1,
			expectedPassCount: 1,
			expectedActive:    2,
		},
		{
			name:              "passCount 3 - round 1 with 3 passes",
			passCount:         3,
			expectedRound:     1,
			expectedPassCount: 3,
			expectedActive:    0,
		},
		{
			name:              "passCount 4 - round 2 just started",
			passCount:         4,
			expectedRound:     2,
			expectedPassCount: 0,
			expectedActive:    1,
		},
		{
			name:              "passCount 5 - round 2 with 1 pass",
			passCount:         5,
			expectedRound:     2,
			expectedPassCount: 1,
			expectedActive:    2,
		},
		{
			name:              "passCount 7 - round 2 with 3 passes",
			passCount:         7,
			expectedRound:     2,
			expectedPassCount: 3,
			expectedActive:    0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gs := testfixtures.NewGameMidBidding(tc.passCount)

			assert.Equal(t, game.PhaseBidding, gs.Phase)
			assert.Equal(t, tc.expectedRound, gs.BiddingRound, "BiddingRound")
			assert.Equal(t, tc.expectedPassCount, gs.BiddingPassCount, "BiddingPassCount")
			assert.Equal(t, tc.expectedActive, gs.ActivePlayerSeat, "ActivePlayerSeat")
			assert.Equal(t, 0, gs.DealerSeat, "DealerSeat should always be 0")

			for i, p := range gs.Players {
				assert.Len(t, p.Hand, 5, "player at seat %d should have 5 cards", i)
			}
			require.NotNil(t, gs.TrumpCandidate)
			assert.Len(t, gs.Deck, 11, "Deck should hold 11 stage-2 cards")
		})
	}

	t.Run("passCount 0 equals NewGameJustDealt", func(t *testing.T) {
		mid := testfixtures.NewGameMidBidding(0)
		fresh := testfixtures.NewGameJustDealt()
		assert.Equal(t, fresh.BiddingRound, mid.BiddingRound)
		assert.Equal(t, fresh.BiddingPassCount, mid.BiddingPassCount)
		assert.Equal(t, fresh.ActivePlayerSeat, mid.ActivePlayerSeat)
		assert.Equal(t, fresh.DealerSeat, mid.DealerSeat)
		assert.Equal(t, fresh.Phase, mid.Phase)
	})
}

func TestNewGameMidPlay(t *testing.T) {
	tests := []struct {
		name             string
		trickNum         int
		expectedCards    int
		expectedTrickNum int
	}{
		{"trickNum 1 - full hands", 1, 8, 1},
		{"trickNum 2 - 7 cards each", 2, 7, 2},
		{"trickNum 4 - 5 cards each", 4, 5, 4},
		{"trickNum 8 - 1 card each", 8, 1, 8},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gs := testfixtures.NewGameMidPlay(tc.trickNum)

			assert.Equal(t, game.PhasePlaying, gs.Phase)
			assert.Equal(t, tc.expectedTrickNum, gs.TrickNumber)
			require.NotNil(t, gs.TrumpSuit)
			assert.Equal(t, game.SuitHearts, *gs.TrumpSuit)
			require.NotNil(t, gs.TrumpCallerSeat)
			assert.Equal(t, 0, gs.ActivePlayerSeat)
			assert.Empty(t, gs.CurrentTrick)
			assert.Nil(t, gs.LeadSuit)

			for i, p := range gs.Players {
				assert.Len(t, p.Hand, tc.expectedCards, "player at seat %d should have %d cards", i, tc.expectedCards)
			}
		})
	}

	t.Run("all 32 cards accounted for at trick 1", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		seen := make(map[string]bool)
		for _, p := range gs.Players {
			for _, card := range p.Hand {
				id := card.String()
				assert.False(t, seen[id], "duplicate card: %s", id)
				seen[id] = true
			}
		}
		assert.Len(t, seen, 32)
	})

	t.Run("clamping - below 1 becomes 1", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(0)
		assert.Equal(t, 1, gs.TrickNumber)
		for i, p := range gs.Players {
			assert.Len(t, p.Hand, 8, "player at seat %d", i)
		}
	})

	t.Run("clamping - above 8 becomes 8", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(10)
		assert.Equal(t, 8, gs.TrickNumber)
		for i, p := range gs.Players {
			assert.Len(t, p.Hand, 1, "player at seat %d", i)
		}
	})

	t.Run("mixed suits for rule testing", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		hasSuit := func(hand []game.Card, suit game.Suit) bool {
			for _, c := range hand {
				if c.Suit == suit {
					return true
				}
			}
			return false
		}
		assert.True(t, hasSuit(gs.Players[0].Hand, game.SuitSpades), "seat 0 should have spades")
		assert.True(t, hasSuit(gs.Players[0].Hand, game.SuitHearts), "seat 0 should have hearts (trump)")
		assert.False(t, hasSuit(gs.Players[3].Hand, game.SuitHearts), "seat 3 should have no trump")
	})
}

func TestNewGameJustDealt(t *testing.T) {
	gs := testfixtures.NewGameJustDealt()

	t.Run("phase is bidding", func(t *testing.T) {
		assert.Equal(t, game.PhaseBidding, gs.Phase)
	})

	t.Run("each player has 5 cards (stage-1 deal)", func(t *testing.T) {
		for i, p := range gs.Players {
			assert.Len(t, p.Hand, 5, "player at seat %d should have 5 cards", i)
		}
	})

	t.Run("Deck holds 11 cards for stage-2", func(t *testing.T) {
		assert.Len(t, gs.Deck, 11)
	})

	t.Run("all 32 cards accounted for across hands + Deck + candidate", func(t *testing.T) {
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

	t.Run("teams assigned correctly", func(t *testing.T) {
		assert.Equal(t, "teamA", gs.Players[0].Team)
		assert.Equal(t, "teamB", gs.Players[1].Team)
		assert.Equal(t, "teamA", gs.Players[2].Team)
		assert.Equal(t, "teamB", gs.Players[3].Team)
	})

	t.Run("dealer is seat 0", func(t *testing.T) {
		assert.Equal(t, 0, gs.DealerSeat)
	})

	t.Run("active player is seat 1", func(t *testing.T) {
		assert.Equal(t, 1, gs.ActivePlayerSeat)
	})

	t.Run("trump candidate is set", func(t *testing.T) {
		require.NotNil(t, gs.TrumpCandidate)
		assert.Equal(t, game.RankAce, gs.TrumpCandidate.Rank)
		assert.Equal(t, game.SuitHearts, gs.TrumpCandidate.Suit)
	})

	t.Run("variant is Bitola", func(t *testing.T) {
		assert.Equal(t, game.VariantBitola, gs.Variant)
	})

	t.Run("hand number is 1", func(t *testing.T) {
		assert.Equal(t, 1, gs.HandNumber)
	})

	t.Run("all players connected", func(t *testing.T) {
		for i, p := range gs.Players {
			assert.True(t, p.Connected, "player at seat %d should be connected", i)
		}
	})

	t.Run("deterministic hands for reproducibility", func(t *testing.T) {
		gs2 := testfixtures.NewGameJustDealt()
		for i := range gs.Players {
			assert.Equal(t, gs.Players[i].Hand, gs2.Players[i].Hand, "hands should be identical across calls")
		}
	})
}

func TestNewGameJustDealt_NoInstantWinOnAnyPick(t *testing.T) {
	allSuits := []game.Suit{game.SuitSpades, game.SuitHearts, game.SuitDiamonds, game.SuitClubs}

	for passCount := 0; passCount <= 7; passCount++ {
		gs := testfixtures.NewGameMidBidding(passCount)
		picker := gs.ActivePlayerSeat

		if gs.BiddingRound == 1 {
			result, err := game.ApplyAction(gs, game.Action{Type: game.ActionPickTrump, PlayerSeat: picker})
			require.NoError(t, err, "round 1 pick by seat %d (passCount=%d) should succeed", picker, passCount)
			assert.NotEqual(t, game.PhaseMatchEnd, result.Phase,
				"round 1 pick by seat %d (passCount=%d) must not trigger instant-win", picker, passCount)
			assert.Nil(t, result.WinnerTeam, "round 1 pick by seat %d (passCount=%d) must leave WinnerTeam nil", picker, passCount)
			continue
		}

		require.NotNil(t, gs.TrumpCandidate, "round-2 fixture must have a face-up candidate")
		lockedSuit := gs.TrumpCandidate.Suit
		for _, suit := range allSuits {
			if suit == lockedSuit {
				continue
			}
			s := suit
			result, err := game.ApplyAction(gs, game.Action{
				Type:       game.ActionPickTrump,
				PlayerSeat: picker,
				Suit:       &s,
			})
			require.NoError(t, err, "round 2 pick by seat %d suit=%s (passCount=%d)", picker, suit, passCount)
			assert.NotEqual(t, game.PhaseMatchEnd, result.Phase,
				"round 2 pick by seat %d suit=%s (passCount=%d) must not trigger instant-win", picker, suit, passCount)
			assert.Nil(t, result.WinnerTeam,
				"round 2 pick by seat %d suit=%s (passCount=%d) must leave WinnerTeam nil", picker, suit, passCount)
		}
	}
}

func TestNewGameLastTrick(t *testing.T) {
	gs := testfixtures.NewGameLastTrick()

	t.Run("phase is playing at trick 8", func(t *testing.T) {
		assert.Equal(t, game.PhasePlaying, gs.Phase)
		assert.Equal(t, 8, gs.TrickNumber)
	})

	t.Run("each player has 1 card", func(t *testing.T) {
		for i, p := range gs.Players {
			assert.Len(t, p.Hand, 1, "player at seat %d should have 1 card", i)
		}
	})

	t.Run("tricks won sums to 7", func(t *testing.T) {
		total := gs.TricksWon[0] + gs.TricksWon[1]
		assert.Equal(t, 7, total, "7 tricks should have been played before trick 8")
	})

	t.Run("trump is set", func(t *testing.T) {
		require.NotNil(t, gs.TrumpSuit)
		assert.Equal(t, game.SuitHearts, *gs.TrumpSuit)
	})

	t.Run("trump caller is set", func(t *testing.T) {
		require.NotNil(t, gs.TrumpCallerSeat)
		assert.Equal(t, 1, *gs.TrumpCallerSeat, "Team B (seat 1) called trump")
	})

	t.Run("declarations resolved", func(t *testing.T) {
		assert.True(t, gs.DeclarationsResolved)
	})

	t.Run("hand points sum matches 7 tricks", func(t *testing.T) {
		total := gs.HandPoints[0] + gs.HandPoints[1]
		assert.Equal(t, 131, total, "7 tricks with 21 pts remaining = 131")
	})

	t.Run("current trick is empty", func(t *testing.T) {
		assert.Empty(t, gs.CurrentTrick)
	})
}

func TestNewGameCapotInProgress(t *testing.T) {
	gs := testfixtures.NewGameCapotInProgress()

	t.Run("phase is playing at trick 8", func(t *testing.T) {
		assert.Equal(t, game.PhasePlaying, gs.Phase)
		assert.Equal(t, 8, gs.TrickNumber)
	})

	t.Run("each player has 1 card", func(t *testing.T) {
		for i, p := range gs.Players {
			assert.Len(t, p.Hand, 1, "player at seat %d should have 1 card", i)
		}
	})

	t.Run("team A won all 7 tricks", func(t *testing.T) {
		assert.Equal(t, 7, gs.TricksWon[0], "Team A should have 7 tricks")
		assert.Equal(t, 0, gs.TricksWon[1], "Team B should have 0 tricks")
	})

	t.Run("all hand points with team A", func(t *testing.T) {
		assert.Equal(t, 121, gs.HandPoints[0], "Team A should have 121 card points from 7 tricks")
		assert.Equal(t, 0, gs.HandPoints[1], "Team B should have 0 card points")
	})

	t.Run("trump caller is team A", func(t *testing.T) {
		require.NotNil(t, gs.TrumpCallerSeat)
		assert.Equal(t, 0, *gs.TrumpCallerSeat, "Team A (seat 0) called trump")
	})

	t.Run("team A has strongest cards to complete capot", func(t *testing.T) {
		assert.Equal(t, game.RankJack, gs.Players[0].Hand[0].Rank)
		assert.Equal(t, game.SuitHearts, gs.Players[0].Hand[0].Suit)
	})
}

func TestNewGameNearEnd(t *testing.T) {
	gs := testfixtures.NewGameNearEnd(900, 850)

	t.Run("phase is playing at trick 8", func(t *testing.T) {
		assert.Equal(t, game.PhasePlaying, gs.Phase)
		assert.Equal(t, 8, gs.TrickNumber)
	})

	t.Run("team scores are set to provided values", func(t *testing.T) {
		assert.Equal(t, 900, gs.TeamScores[0], "Team A score")
		assert.Equal(t, 850, gs.TeamScores[1], "Team B score")
	})

	t.Run("inherits NewGameLastTrick structure", func(t *testing.T) {
		lastTrick := testfixtures.NewGameLastTrick()
		for i := range gs.Players {
			assert.Equal(t, lastTrick.Players[i].Hand, gs.Players[i].Hand,
				"player %d hand should match NewGameLastTrick", i)
		}
		assert.Equal(t, lastTrick.HandPoints, gs.HandPoints)
		assert.Equal(t, lastTrick.TricksWon, gs.TricksWon)
	})

	t.Run("each player has 1 card", func(t *testing.T) {
		for i, p := range gs.Players {
			assert.Len(t, p.Hand, 1, "player at seat %d should have 1 card", i)
		}
	})

	t.Run("trump caller is team B", func(t *testing.T) {
		require.NotNil(t, gs.TrumpCallerSeat)
		assert.Equal(t, 1, *gs.TrumpCallerSeat, "Team B (seat 1) called trump")
	})

	t.Run("zero scores work", func(t *testing.T) {
		gs0 := testfixtures.NewGameNearEnd(0, 0)
		assert.Equal(t, 0, gs0.TeamScores[0])
		assert.Equal(t, 0, gs0.TeamScores[1])
	})
}
