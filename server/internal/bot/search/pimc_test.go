package search_test

import (
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/bot/search"
	"github.com/emilijan/beljot/server/internal/game"
)

var hearts = search.SuitIdx(game.SuitHearts)

func farFuture() time.Time {
	return time.Now().Add(time.Hour)
}

func cardOf(suit game.Suit, rank game.Rank) int {
	return search.CardIdx(game.Card{Suit: suit, Rank: rank})
}

func maskOfIndexes(indexes ...int) uint32 {
	var m uint32
	for _, idx := range indexes {
		m |= 1 << uint(idx)
	}
	return m
}

func informationFor(seat int, hands [4]uint32) search.Information {
	in := search.Information{Seat: seat, Own: hands[seat]}
	for s, hand := range hands {
		if s == seat {
			continue
		}
		in.Unseen |= hand
		in.Need[s] = len(search.CardsOf(hand))
	}
	return in
}

func settlementOver(trump int, trick []int, hands ...uint32) search.Settlement {
	points := 0
	for _, idx := range trick {
		points += search.Points(idx, trump)
	}
	for _, hand := range hands {
		for _, card := range search.CardsOf(hand) {
			points += search.Points(search.CardIdx(card), trump)
		}
	}
	return search.Settlement{
		HandPoints:          [2]int{90, 10},
		DeclPoints:          [2]int{20, 0},
		TricksWon:           [2]int{3, 3},
		CallerTeam:          game.TeamA,
		RemainingCardPoints: points,
	}
}

func legalFor(pos search.Position, own uint32) []game.Card {
	return search.CardsOf(search.NewSolver(pos.Trump, pos.Clockwise).Legal(own, pos.Trick))
}

type keepTheMaster struct {
	in     search.Information
	pos    search.Position
	legal  []game.Card
	settle search.Settlement
	junk   game.Card
}

// newKeepTheMaster deals the leader the master trump and a junk club, and the ace of clubs to the opponent after him.
// The master wins whichever of the two tricks it is played in, and the junk club never wins either.
func newKeepTheMaster(leader int, clockwise bool, unseen uint32) keepTheMaster {
	master := game.Card{Suit: game.SuitHearts, Rank: game.RankJack}
	junk := game.Card{Suit: game.SuitClubs, Rank: game.Rank7}
	ahead := game.StepSeat(leader, 1, clockwise)
	own := search.MaskOf([]game.Card{master, junk})
	in := search.Information{Seat: leader, Own: own, Unseen: unseen}
	in.Known[ahead] = maskOfIndexes(cardOf(game.SuitClubs, game.RankAce))
	for s := range 4 {
		switch s {
		case leader:
		case ahead:
			in.Need[s] = 1
		default:
			in.Need[s] = 2
		}
	}
	pos := search.Position{
		Trump: hearts, Clockwise: clockwise, Leader: leader, Capot: search.CapotFlag([2]int{3, 3}),
	}
	return keepTheMaster{
		in:     in,
		pos:    pos,
		legal:  []game.Card{master, junk},
		settle: settlementOver(hearts, nil, own, unseen, in.Known[ahead]),
		junk:   junk,
	}
}

func drawFiveOutside(rng *rand.Rand, excluded uint32) uint32 {
	var pool []int
	for idx := range search.NCards {
		if excluded&(1<<uint(idx)) == 0 {
			pool = append(pool, idx)
		}
	}
	rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	return maskOfIndexes(pool[:5]...)
}

func unseenWithoutClubs(rng *rand.Rand) uint32 {
	return drawFiveOutside(rng, search.SuitMask(hearts)|search.SuitMask(search.SuitIdx(game.SuitClubs)))
}

func unseenWithClubs(rng *rand.Rand) uint32 {
	held := maskOfIndexes(cardOf(game.SuitClubs, game.Rank7), cardOf(game.SuitClubs, game.RankAce))
	return drawFiveOutside(rng, search.SuitMask(hearts)|held)
}

func TestChooseIsDeterministic(t *testing.T) {
	trick := []int{cardOf(game.SuitDiamonds, game.Rank7)}
	pos := search.Position{
		Trump: hearts, Leader: 0, Trick: trick, Capot: search.CapotFlag([2]int{2, 2}),
	}
	hands := [4]uint32{
		maskOfIndexes(
			cardOf(game.SuitSpades, game.RankKing), cardOf(game.SuitHearts, game.Rank9),
			cardOf(game.SuitClubs, game.RankTen),
		),
		maskOfIndexes(
			cardOf(game.SuitDiamonds, game.RankKing), cardOf(game.SuitDiamonds, game.Rank8),
			cardOf(game.SuitSpades, game.RankAce), cardOf(game.SuitHearts, game.RankQueen),
		),
		maskOfIndexes(
			cardOf(game.SuitDiamonds, game.RankAce), cardOf(game.SuitSpades, game.Rank7),
			cardOf(game.SuitHearts, game.Rank7), cardOf(game.SuitClubs, game.RankKing),
		),
		maskOfIndexes(
			cardOf(game.SuitDiamonds, game.RankQueen), cardOf(game.SuitSpades, game.Rank9),
			cardOf(game.SuitHearts, game.RankJack), cardOf(game.SuitClubs, game.Rank9),
		),
	}
	in := informationFor(1, hands)
	legal := legalFor(pos, in.Own)
	require.Greater(t, len(legal), 1)
	settle := settlementOver(hearts, trick, hands[:]...)
	opts := search.Options{Samples: 12, EarlyStopAfter: 13, EarlyStopSE: 2}

	first, ok := search.Choose(in, pos, legal, settle, opts, farFuture())
	require.True(t, ok)
	second, ok := search.Choose(in, pos, legal, settle, opts, farFuture())
	require.True(t, ok)

	assert.Equal(t, first, second)
	assert.Equal(t, opts.Samples, first.Samples)
	assert.Contains(t, legal, first.Card)
}

func TestChooseBreaksTiesTowardTheLowestCardIndex(t *testing.T) {
	seven := game.Card{Suit: game.SuitDiamonds, Rank: game.Rank7}
	eight := game.Card{Suit: game.SuitDiamonds, Rank: game.Rank8}
	hands := [4]uint32{
		search.MaskOf([]game.Card{eight, seven}),
		maskOfIndexes(cardOf(game.SuitSpades, game.Rank7), cardOf(game.SuitSpades, game.Rank8)),
		maskOfIndexes(cardOf(game.SuitClubs, game.Rank7), cardOf(game.SuitClubs, game.Rank8)),
		maskOfIndexes(cardOf(game.SuitSpades, game.Rank9), cardOf(game.SuitClubs, game.Rank9)),
	}
	pos := search.Position{Trump: hearts, Leader: 0, Capot: search.CapotFlag([2]int{3, 3})}
	settle := settlementOver(hearts, nil, hands[:]...)
	opts := search.Options{Samples: 3, EarlyStopAfter: 4, EarlyStopSE: 1}

	for _, legal := range [][]game.Card{{seven, eight}, {eight, seven}} {
		result, ok := search.Choose(informationFor(0, hands), pos, legal, settle, opts, farFuture())
		require.True(t, ok)
		assert.Equal(t, seven, result.Card)
		assert.Zero(t, result.Margin)
	}
}

func TestChooseKeepsTheMasterTrumpForTheLastTrick(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 11))
	opts := search.Options{Samples: 1, EarlyStopAfter: 2, EarlyStopSE: 1}
	for range 40 {
		c := newKeepTheMaster(0, false, unseenWithoutClubs(rng))
		result, ok := search.Choose(c.in, c.pos, c.legal, c.settle, opts, farFuture())
		require.True(t, ok)
		assert.Equal(t, c.junk, result.Card, "unseen %b", c.in.Unseen)
		assert.Equal(t, 1, result.Samples)
		assert.InDelta(t, 20.0, result.Margin, 1e-9)
	}
}

func TestChooseScoresFromTheActingSeatsTeam(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 5))
	opts := search.Options{Samples: 4, EarlyStopAfter: 5, EarlyStopSE: 1}
	for _, clockwise := range []bool{false, true} {
		for leader := range 4 {
			c := newKeepTheMaster(leader, clockwise, unseenWithoutClubs(rng))
			result, ok := search.Choose(c.in, c.pos, c.legal, c.settle, opts, farFuture())
			require.True(t, ok)
			assert.Equal(t, c.junk, result.Card, "leader %d clockwise %v", leader, clockwise)
			assert.InDelta(t, 20.0, result.Margin, 1e-9, "leader %d clockwise %v", leader, clockwise)
		}
	}
}

func TestChooseTimesOut(t *testing.T) {
	c := newKeepTheMaster(0, false, unseenWithoutClubs(rand.New(rand.NewPCG(1, 2))))
	opts := search.Options{Samples: 8, EarlyStopAfter: 9, EarlyStopSE: 1}

	_, ok := search.Choose(c.in, c.pos, c.legal, c.settle, opts, farFuture())
	require.True(t, ok)

	result, ok := search.Choose(c.in, c.pos, c.legal, c.settle, opts, time.Now().Add(-time.Second))
	assert.False(t, ok)
	assert.Equal(t, search.Result{}, result)
}

func TestChooseStopsEarlyOnAClearMargin(t *testing.T) {
	c := newKeepTheMaster(0, false, unseenWithClubs(rand.New(rand.NewPCG(2, 2))))
	full := search.Options{Samples: 40, EarlyStopAfter: 41, EarlyStopSE: 1}
	early := search.Options{Samples: 40, EarlyStopAfter: 4, EarlyStopSE: 2}

	exhaustive, ok := search.Choose(c.in, c.pos, c.legal, c.settle, full, farFuture())
	require.True(t, ok)
	stopped, ok := search.Choose(c.in, c.pos, c.legal, c.settle, early, farFuture())
	require.True(t, ok)

	assert.Equal(t, 40, exhaustive.Samples)
	assert.Less(t, stopped.Samples, 40)
	assert.GreaterOrEqual(t, stopped.Samples, early.EarlyStopAfter)
	assert.Equal(t, exhaustive.Card, stopped.Card)
	assert.Greater(t, stopped.SE, 0.0)
	assert.Greater(t, stopped.Margin, early.EarlyStopSE*stopped.SE)
}

func TestChooseStopsEarlyWhenEveryDealAgrees(t *testing.T) {
	c := newKeepTheMaster(0, false, unseenWithoutClubs(rand.New(rand.NewPCG(9, 9))))
	opts := search.Options{Samples: 40, EarlyStopAfter: 4, EarlyStopSE: 2}

	result, ok := search.Choose(c.in, c.pos, c.legal, c.settle, opts, farFuture())
	require.True(t, ok)

	assert.Equal(t, opts.EarlyStopAfter, result.Samples)
	assert.Zero(t, result.SE)
	assert.InDelta(t, 20.0, result.Margin, 1e-9)
}

func TestChooseReportsThePairedStandardError(t *testing.T) {
	c := newKeepTheMaster(0, false, unseenWithClubs(rand.New(rand.NewPCG(18, 18))))
	var results [3]search.Result
	for i := range results {
		opts := search.Options{Samples: i + 1, EarlyStopAfter: 10, EarlyStopSE: 1}
		result, ok := search.Choose(c.in, c.pos, c.legal, c.settle, opts, farFuture())
		require.True(t, ok)
		require.Equal(t, c.junk, result.Card)
		results[i] = result
	}

	var leads [3]float64
	banked := 0.0
	for i, result := range results {
		total := result.Margin * float64(i+1)
		leads[i] = total - banked
		banked = total
	}
	mean := (leads[0] + leads[1] + leads[2]) / 3
	squares := 0.0
	for _, lead := range leads {
		squares += (lead - mean) * (lead - mean)
	}
	wantSE := math.Sqrt(squares/2) / math.Sqrt(3)

	require.Greater(t, wantSE, 0.0)
	assert.Zero(t, results[0].SE)
	assert.InDelta(t, wantSE, results[2].SE, 1e-9)
	assert.InDelta(t, mean, results[2].Margin, 1e-9)
}

func TestChooseRefusesWhatItCannotAnswer(t *testing.T) {
	farDeadline := farFuture()
	tests := []struct {
		name   string
		mutate func(c *keepTheMaster, opts *search.Options)
	}{
		{"no legal card", func(c *keepTheMaster, _ *search.Options) { c.legal = nil }},
		{"no samples requested", func(_ *keepTheMaster, opts *search.Options) { opts.Samples = 0 }},
		{"another seat is on turn", func(c *keepTheMaster, _ *search.Options) {
			c.pos.Leader = 1
			c.legal = []game.Card{{Suit: game.SuitClubs, Rank: game.RankAce}}
		}},
		{"a legal card the seat does not hold", func(c *keepTheMaster, _ *search.Options) {
			c.legal = append(c.legal, game.Card{Suit: game.SuitSpades, Rank: game.RankAce})
		}},
		{"no deal fits the information", func(c *keepTheMaster, _ *search.Options) {
			c.in.CannotHold[2] = ^uint32(0)
			c.in.CannotHold[3] = ^uint32(0)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newKeepTheMaster(0, false, unseenWithoutClubs(rand.New(rand.NewPCG(4, 4))))
			opts := search.Options{Samples: 8, EarlyStopAfter: 9, EarlyStopSE: 1}
			tt.mutate(&c, &opts)

			result, ok := search.Choose(c.in, c.pos, c.legal, c.settle, opts, farDeadline)

			assert.False(t, ok)
			assert.Equal(t, search.Result{}, result)
		})
	}
}

func TestSamplesForTrickSchedule(t *testing.T) {
	want := map[int]int{0: 0, 1: 0, 2: 0, 3: 16, 4: 30, 5: 40, 6: 60, 7: 60, 8: 60}
	for trick, samples := range want {
		assert.Equal(t, samples, search.SamplesForTrick(trick), "trick %d", trick)
	}
}
