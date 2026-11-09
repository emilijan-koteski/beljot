package search

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
)

func TestCallerMadeShare(t *testing.T) {
	card := CardIdx(game.Card{Suit: game.SuitClubs, Rank: game.Rank7})
	tests := []struct {
		name       string
		race       *Race
		remainders []int
		want       float64
	}{
		{"made, tied, made, fell", nil, []int{-26, -28, 40, -40}, 0.5},
		{"the match won by the caller counts as made", &Race{Target: 1001, Caller: game.TeamA}, []int{RaceWin, -RaceWin, -26}, 2.0 / 3},
	}
	for _, rule := range []game.TieRule{game.TieRuleAllToOpponents, game.TieRuleHangingPoints} {
		settle := Settlement{
			HandPoints: [2]int{60, 52}, DeclPoints: [2]int{20, 0}, TricksWon: [2]int{3, 4},
			CallerTeam: game.TeamA, TieRule: rule, RemainingCardPoints: 40,
		}
		for _, tt := range tests {
			t.Run(string(rule)+" "+tt.name, func(t *testing.T) {
				votes := newTally([]int{card}, tt.race, true)
				for _, remainder := range tt.remainders {
					require.True(t, votes.add(map[int]int{card: remainder}, settle, -1, -1))
				}
				assert.InDelta(t, tt.want, votes.result().CallerMade, 1e-9)
			})
		}
	}
}

func TestCallerMadeIsLeftOutUnlessTracked(t *testing.T) {
	settle := Settlement{HandPoints: [2]int{60, 52}, TricksWon: [2]int{3, 4}, CallerTeam: game.TeamA, RemainingCardPoints: 40}
	card := CardIdx(game.Card{Suit: game.SuitClubs, Rank: game.Rank7})
	votes := newTally([]int{card}, nil, false)
	require.True(t, votes.add(map[int]int{card: 40}, settle, -1, -1))
	assert.Zero(t, votes.result().CallerMade)
	assert.InDelta(t, -float64(Settle(settle, 40)), votes.result().Mean, 1e-9)
}
