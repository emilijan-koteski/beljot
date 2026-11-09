package search_test

import (
	"math/rand/v2"
	"os"
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
	trick1Positions = 10
	trick1MeanLimit = 1000 * time.Millisecond
	trick1Samples   = 12
	raceBenchScore  = 940
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
	opts := search.Options{Samples: search.SamplesForTrick(3), EarlyStopAfter: 8, EarlyStopSE: 1.0}

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
}

func TestSearchTrick3RaceTiming(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 3))
	opts := search.Options{Samples: search.SamplesForTrick(3), EarlyStopAfter: 8, EarlyStopSE: 1.0}

	var total, worst time.Duration
	for range trick3Positions {
		p := randomTrick3Position(rng)
		race := raceWithinReach(p)
		p.pos.Race = &race
		start := time.Now()
		_, ok := search.Choose(p.in, p.pos, p.legal, p.settle, opts, farFuture())
		elapsed := time.Since(start)
		require.True(t, ok, "the search must reach a decision on a consistent position")
		total += elapsed
		worst = max(worst, elapsed)
	}

	t.Logf("trick 3 race search over %d positions: mean %v, max %v (logged, not asserted)",
		trick3Positions, total/trick3Positions, worst)
}

// raceWithinReach puts both teams of a trick-3 position at raceBenchScore before the hand on a stop-at-target table,
// each within reach of 1001, with the leader's Belote when it holds the trump King and Queen.
func raceWithinReach(p trick3Position) search.Race {
	race := search.Race{Target: 1001, Caller: p.settle.CallerTeam}
	for team := range race.Banked {
		race.Banked[team] = raceBenchScore + p.settle.HandPoints[team]
	}
	if pair := belotePair(p.pos.Trump); p.in.Own&pair == pair {
		race.BeloteSeat, race.BeloteCards = p.in.Seat, pair
	}
	return race
}

// randomTrick1Position deals eight cards to each seat and leaves a random leader to open trick 1 under a random trump.
func randomTrick1Position(rng *rand.Rand) trick3Position {
	trump := rng.IntN(4)
	clockwise := rng.IntN(2) == 1
	var hands [4]uint32
	for i, card := range rng.Perm(search.NCards) {
		hands[i/8] |= 1 << uint(card)
	}
	leader := rng.IntN(4)
	pos := search.Position{Trump: trump, Clockwise: clockwise, Leader: leader}
	settle := settlementOver(trump, nil, hands[:]...)
	settle.HandPoints = [2]int{}
	settle.DeclPoints = [2]int{}
	settle.TricksWon = [2]int{}
	settle.CallerTeam = rng.IntN(2)
	return trick3Position{
		in:     informationFor(leader, hands),
		pos:    pos,
		legal:  legalFor(pos, hands[leader]),
		settle: settle,
	}
}

// TestSearchTrick1StaysUnderTwiceTheDeadline guards the opening search against a gross slowdown.
// The bound is twice the 500 ms deadline: a GitHub runner measured a 555 ms mean where a desk
// machine measured 332 ms, so the deadline itself is not a stable CI gate.
func TestSearchTrick1StaysUnderTwiceTheDeadline(t *testing.T) {
	if search.OpeningSamples == 0 {
		t.Skip("the opening search is off: production does not search tricks 1 and 2")
	}
	rng := rand.New(rand.NewPCG(1, 1))
	opts := search.Options{Samples: trick1Samples, EarlyStopAfter: 8, EarlyStopSE: 1.0}

	var total, worst time.Duration
	for range trick1Positions {
		p := randomTrick1Position(rng)
		start := time.Now()
		_, ok := search.Choose(p.in, p.pos, p.legal, p.settle, opts, farFuture())
		elapsed := time.Since(start)
		require.True(t, ok, "the search must reach a decision on a consistent position")
		total += elapsed
		worst = max(worst, elapsed)
	}

	mean := total / trick1Positions
	t.Logf("trick 1 search over %d positions: mean %v, max %v", trick1Positions, mean, worst)
	if os.Getenv("BELJOT_BOT_SIM") == "1" {
		assert.Less(t, mean, trick1MeanLimit, "mean search time at trick 1")
	}
}
