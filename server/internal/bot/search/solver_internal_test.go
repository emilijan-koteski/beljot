package search

import (
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
)

// randomFullDeal deals eight cards to each seat with a random leader, trump suit and direction of play.
func randomFullDeal(rng *rand.Rand) ([4]uint32, int, int, bool) {
	var hands [4]uint32
	for i, card := range rng.Perm(NCards) {
		hands[i/8] |= 1 << uint(card)
	}
	return hands, rng.IntN(4), rng.IntN(4), rng.IntN(2) == 1
}

// randomTrick3Deal plays two random legal tricks from a full deal, then zero to three cards into the third.
func randomTrick3Deal(rng *rand.Rand) ([4]uint32, int, []int, int, bool, int8) {
	hands, leader, trump, clockwise := randomFullDeal(rng)
	solver := NewSolver(trump, clockwise)
	playCard := func(trick []int) []int {
		seat := game.StepSeat(leader, len(trick), clockwise)
		legal := CardsOf(solver.Legal(hands[seat], trick))
		card := CardIdx(legal[rng.IntN(len(legal))])
		hands[seat] &^= 1 << uint(card)
		return append(trick, card)
	}
	var won [2]int
	for range 2 {
		var trick []int
		for range 4 {
			trick = playCard(trick)
		}
		leader = game.StepSeat(leader, solver.Winner(trick), clockwise)
		won[game.TeamForSeat(leader)]++
	}
	var trick []int
	for range rng.IntN(4) {
		trick = playCard(trick)
	}
	return hands, leader, trick, trump, clockwise, CapotFlag(won)
}

func TestOptimisedSolverMatchesPlainOnFullHands(t *testing.T) {
	rng := rand.New(rand.NewPCG(31, 37))
	for i := range 10 {
		hands, leader, trump, clockwise := randomFullDeal(rng)
		fast := NewSolver(trump, clockwise)
		plain := NewSolver(trump, clockwise)
		plain.plain = true
		require.Equal(t, plain.RootValues(hands, leader, nil, capotNone), fast.RootValues(hands, leader, nil, capotNone), "deal %d", i)
	}
	for i := range 40 {
		hands, leader, trick, trump, clockwise, capot := randomTrick3Deal(rng)
		fast := NewSolver(trump, clockwise)
		plain := NewSolver(trump, clockwise)
		plain.plain = true
		require.Equal(t, plain.RootValues(hands, leader, trick, capot), fast.RootValues(hands, leader, trick, capot), "position %d", i)
	}
}

func TestOptimisedSolverMatchesPlainWhenRacing(t *testing.T) {
	rng := rand.New(rand.NewPCG(59, 61))
	for i := range 40 {
		hands, leader, trick, trump, clockwise, capot := randomTrick3Deal(rng)
		race := &Race{Target: 1001, Banked: [2]int{900 + rng.IntN(100), 900 + rng.IntN(100)}, Caller: rng.IntN(2)}
		fast := NewSolver(trump, clockwise)
		fast.SetRace(race)
		plain := NewSolver(trump, clockwise)
		plain.plain = true
		plain.SetRace(race)
		require.Equal(t, plain.RootValues(hands, leader, trick, capot), fast.RootValues(hands, leader, trick, capot), "position %d race %+v", i, *race)
	}
}
