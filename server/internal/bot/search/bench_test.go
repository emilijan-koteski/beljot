package search_test

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/bot/search"
	"github.com/emilijan/beljot/server/internal/game"
)

const (
	trick3Positions = 20
	trick3MeanLimit = 250 * time.Millisecond
	trick3MaxLimit  = 500 * time.Millisecond
)

type trick3Position struct {
	in     search.Information
	pos    search.Position
	legal  []game.Card
	settle search.Settlement
}

// randomTrick3Position deals a full deck, plays two random legal tricks and leaves the winner of the second to lead
// the third from six cards, with every other hand unseen.
func randomTrick3Position(rng *rand.Rand) trick3Position {
	trump := rng.IntN(4)
	clockwise := rng.IntN(2) == 1
	solver := search.NewSolver(trump, clockwise)

	var hands [4]uint32
	for i, card := range rng.Perm(search.NCards) {
		hands[i/8] |= 1 << uint(card)
	}
	leader := rng.IntN(4)
	var won, banked [2]int
	for range 2 {
		var trick []int
		points := 0
		for i := range 4 {
			seat := game.StepSeat(leader, i, clockwise)
			legal := search.CardsOf(solver.Legal(hands[seat], trick))
			card := search.CardIdx(legal[rng.IntN(len(legal))])
			hands[seat] &^= 1 << uint(card)
			trick = append(trick, card)
			points += search.Points(card, trump)
		}
		leader = game.StepSeat(leader, solver.Winner(trick), clockwise)
		team := game.TeamForSeat(leader)
		won[team]++
		banked[team] += points
	}

	pos := search.Position{Trump: trump, Clockwise: clockwise, Leader: leader, Capot: search.CapotFlag(won)}
	settle := settlementOver(trump, nil, hands[:]...)
	settle.HandPoints = banked
	settle.DeclPoints = [2]int{}
	settle.TricksWon = won
	settle.CallerTeam = rng.IntN(2)
	return trick3Position{
		in:     informationFor(leader, hands),
		pos:    pos,
		legal:  legalFor(pos, hands[leader]),
		settle: settle,
	}
}

func TestSearchTrick3StaysUnderTheDeadline(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 3))
	opts := search.Options{Samples: 16, EarlyStopAfter: 8, EarlyStopSE: 1.0}

	var total, worst time.Duration
	for range trick3Positions {
		p := randomTrick3Position(rng)
		start := time.Now()
		_, ok := search.Choose(p.in, p.pos, p.legal, p.settle, opts, farFuture())
		elapsed := time.Since(start)
		require.True(t, ok, "the search must reach a decision on a consistent position")
		total += elapsed
		worst = max(worst, elapsed)
	}

	mean := total / trick3Positions
	t.Logf("trick 3 search over %d positions: mean %v, max %v", trick3Positions, mean, worst)
	assert.Less(t, mean, trick3MeanLimit, "mean search time at trick 3")
	assert.Less(t, worst, trick3MaxLimit, "slowest search time at trick 3")
}
