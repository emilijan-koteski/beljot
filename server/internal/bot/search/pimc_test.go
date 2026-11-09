package search_test

import (
	"math"
	"math/rand/v2"
	"slices"
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
	want := map[int]int{0: 0, 1: 12, 2: 12, 3: 32, 4: 60, 5: 80, 6: 60, 7: 60, 8: 60}
	for trick, samples := range want {
		assert.Equal(t, samples, search.SamplesForTrick(trick), "trick %d", trick)
	}
}

func TestChooseReportsPerCardStatistics(t *testing.T) {
	c := newKeepTheMaster(0, false, unseenWithClubs(rand.New(rand.NewPCG(5, 5))))
	res, ok := search.Choose(c.in, c.pos, c.legal, c.settle, search.Options{Samples: 20, EarlyStopAfter: 20, EarlyStopSE: 1}, farFuture())
	require.True(t, ok)
	require.Len(t, res.Cards, len(c.legal))
	best := res.Cards[search.CardIdx(res.Card)]
	assert.InDelta(t, res.Mean, best.Mean, 1e-9)
	for _, stats := range res.Cards {
		assert.LessOrEqual(t, stats.Mean, best.Mean+1e-9)
		assert.GreaterOrEqual(t, stats.SE, 0.0)
	}
}

func TestChooseSplitsSamplesByClass(t *testing.T) {
	c := newKeepTheMaster(0, false, unseenWithClubs(rand.New(rand.NewPCG(5, 5))))
	opts := search.Options{Samples: 20, EarlyStopAfter: 20, EarlyStopSE: 1,
		SampleClass: func(hands [4]uint32) int { return int(hands[1] & 1) }}
	res, ok := search.Choose(c.in, c.pos, c.legal, c.settle, opts, farFuture())
	require.True(t, ok)
	assert.Equal(t, res.Samples, res.Classes[0].Samples+res.Classes[1].Samples)
	for class := range res.Classes {
		if res.Classes[class].Samples > 0 {
			assert.Len(t, res.Classes[class].Means, len(c.legal))
		}
	}
}

func TestChooseLeavesAnEmptyClassWithoutMeans(t *testing.T) {
	c := newKeepTheMaster(0, false, unseenWithClubs(rand.New(rand.NewPCG(5, 5))))
	tests := []struct {
		name  string
		class func(hands [4]uint32) int
		want  [2]bool
	}{
		{"every deal in class 0", func([4]uint32) int { return 0 }, [2]bool{true, false}},
		{"every deal in class 1", func([4]uint32) int { return 1 }, [2]bool{false, true}},
		{"no deal in either class", func([4]uint32) int { return -1 }, [2]bool{false, false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := search.Options{Samples: 6, EarlyStopAfter: 7, EarlyStopSE: 1, SampleClass: tt.class}
			res, ok := search.Choose(c.in, c.pos, c.legal, c.settle, opts, farFuture())
			require.True(t, ok)
			for class, filled := range tt.want {
				if filled {
					assert.Equal(t, res.Samples, res.Classes[class].Samples)
					assert.Len(t, res.Classes[class].Means, len(c.legal))
					continue
				}
				assert.Zero(t, res.Classes[class].Samples)
				assert.Nil(t, res.Classes[class].Means)
			}
		})
	}
}

func TestOneSampleHasNoStandardError(t *testing.T) {
	c := newKeepTheMaster(0, false, unseenWithClubs(rand.New(rand.NewPCG(5, 5))))
	opts := search.Options{Samples: 1, EarlyStopAfter: 2, EarlyStopSE: 1}

	res, ok := search.Choose(c.in, c.pos, c.legal, c.settle, opts, farFuture())
	require.True(t, ok)
	require.Equal(t, 1, res.Samples)
	require.Len(t, res.Cards, len(c.legal))
	assert.Zero(t, res.SE)
	for _, stats := range res.Cards {
		assert.Zero(t, stats.SE)
		assert.False(t, math.IsNaN(stats.Mean))
	}

	_, se, _, ok := search.Evaluate(c.in, c.pos, c.settle, opts, farFuture())
	require.True(t, ok)
	assert.Zero(t, se)
}

func TestChooseSeedSaltChangesTheSamplesNotTheInformation(t *testing.T) {
	c := newKeepTheMaster(0, false, unseenWithClubs(rand.New(rand.NewPCG(5, 5))))
	base := search.Options{Samples: 6, EarlyStopAfter: 6, EarlyStopSE: 1}
	salted := base
	salted.SeedSalt = 0xABCD
	a, _ := search.Choose(c.in, c.pos, c.legal, c.settle, base, farFuture())
	b, _ := search.Choose(c.in, c.pos, c.legal, c.settle, salted, farFuture())
	again, _ := search.Choose(c.in, c.pos, c.legal, c.settle, salted, farFuture())
	assert.Equal(t, b, again)
	assert.NotEqual(t, a.Cards, b.Cards)
}

func TestEvaluateMatchesChooseOnAForcedCard(t *testing.T) {
	junk := game.Card{Suit: game.SuitClubs, Rank: game.Rank7}
	master := game.Card{Suit: game.SuitHearts, Rank: game.RankJack}
	unseen := unseenWithClubs(rand.New(rand.NewPCG(9, 4)))
	own := search.MaskOf([]game.Card{junk, master})
	trick := []int{cardOf(game.SuitClubs, game.RankAce)}
	in := search.Information{Seat: 2, Own: own, Unseen: unseen, Need: [4]int{2, 1, 0, 2}}
	pos := search.Position{Trump: hearts, Leader: 1, Trick: trick, Capot: search.CapotFlag([2]int{3, 2})}
	require.Equal(t, []game.Card{junk}, legalFor(pos, own))
	settle := settlementOver(hearts, trick, own, unseen)
	opts := search.Options{Samples: 24, EarlyStopAfter: 25, EarlyStopSE: 1}

	chosen, ok := search.Choose(in, pos, []game.Card{junk}, settle, opts, farFuture())
	require.True(t, ok)
	mean, se, made, ok := search.Evaluate(in, pos, settle, opts, farFuture())
	require.True(t, ok)

	assert.Equal(t, 24, chosen.Samples)
	assert.InDelta(t, chosen.Mean, mean, 1e-9)
	assert.InDelta(t, chosen.Cards[search.CardIdx(junk)].SE, se, 1e-9)
	assert.InDelta(t, chosen.CallerMade, made, 1e-9)
}

func TestEvaluateValuesForTeamAWhenTeamBIsForced(t *testing.T) {
	junk := game.Card{Suit: game.SuitClubs, Rank: game.Rank7}
	master := game.Card{Suit: game.SuitHearts, Rank: game.RankJack}
	unseen := unseenWithClubs(rand.New(rand.NewPCG(5, 8)))
	own := search.MaskOf([]game.Card{junk, master})
	trick := []int{cardOf(game.SuitClubs, game.RankAce)}
	in := search.Information{Seat: 1, Own: own, Unseen: unseen, Need: [4]int{1, 0, 2, 2}}
	pos := search.Position{Trump: hearts, Leader: 0, Trick: trick, Capot: search.CapotFlag([2]int{3, 2})}
	require.Equal(t, []game.Card{junk}, legalFor(pos, own))
	settle := settlementOver(hearts, trick, own, unseen)
	settle.CallerTeam = game.TeamB
	opts := search.Options{Samples: 24, EarlyStopAfter: 25, EarlyStopSE: 1}

	chosen, ok := search.Choose(in, pos, []game.Card{junk}, settle, opts, farFuture())
	require.True(t, ok)
	mean, se, made, ok := search.Evaluate(in, pos, settle, opts, farFuture())
	require.True(t, ok)

	require.NotZero(t, chosen.Mean)
	assert.InDelta(t, -chosen.Mean, mean, 1e-9, "Choose scores for the seat's team, Evaluate for team A")
	assert.InDelta(t, chosen.Cards[search.CardIdx(junk)].SE, se, 1e-9)
	assert.InDelta(t, chosen.CallerMade, made, 1e-9)
}

func TestEvaluateValuesAClosedTrick(t *testing.T) {
	last := game.Card{Suit: game.SuitClubs, Rank: game.Rank8}
	open := []int{cardOf(game.SuitClubs, game.RankAce), cardOf(game.SuitClubs, game.Rank7), cardOf(game.SuitClubs, game.RankKing)}
	closed := append(slices.Clone(open), search.CardIdx(last))
	settle := settlementOver(hearts, closed)
	opts := search.Options{Samples: 4, EarlyStopAfter: 5, EarlyStopSE: 1}

	before := search.Position{Trump: hearts, Leader: 1, Trick: open, Capot: search.CapotFlag([2]int{3, 4})}
	chosen, ok := search.Choose(search.Information{Seat: 0, Own: search.MaskOf([]game.Card{last})}, before, []game.Card{last}, settle, opts, farFuture())
	require.True(t, ok)
	after := before
	after.Trick = closed
	mean, se, made, ok := search.Evaluate(search.Information{Seat: 0}, after, settle, opts, farFuture())
	require.True(t, ok)

	const takenByTeamB = -(11 + 0 + 4 + 0 + 10)
	assert.InDelta(t, float64(search.Settle(settle, takenByTeamB)), mean, 1e-9)
	assert.InDelta(t, chosen.Mean, mean, 1e-9)
	assert.Zero(t, se)
	assert.InDelta(t, 1.0, made, 1e-9)
}

func TestEvaluateTimesOut(t *testing.T) {
	c := newKeepTheMaster(0, false, unseenWithoutClubs(rand.New(rand.NewPCG(1, 2))))
	opts := search.Options{Samples: 8, EarlyStopAfter: 9, EarlyStopSE: 1}

	_, _, _, ok := search.Evaluate(c.in, c.pos, c.settle, opts, farFuture())
	require.True(t, ok)
	_, _, _, ok = search.Evaluate(c.in, c.pos, c.settle, opts, time.Now().Add(-time.Second))
	assert.False(t, ok)
}

func everyOther() func(hands [4]uint32) bool {
	draws := 0
	return func(hands [4]uint32) bool {
		draws++
		return draws%2 == 0
	}
}

func TestChooseAppliesTheSampleFilter(t *testing.T) {
	c := newKeepTheMaster(0, false, unseenWithClubs(rand.New(rand.NewPCG(5, 5))))
	base := search.Options{Samples: 12, EarlyStopAfter: 13, EarlyStopSE: 1}

	t.Run("a filter rejecting every deal leaves no answer", func(t *testing.T) {
		opts := base
		opts.SampleFilter = func(hands [4]uint32) bool { return false }
		_, ok := search.Choose(c.in, c.pos, c.legal, c.settle, opts, farFuture())
		assert.False(t, ok)
		_, _, _, ok = search.Evaluate(c.in, c.pos, c.settle, opts, farFuture())
		assert.False(t, ok)
	})

	t.Run("rejected deals are replaced up to the requested count", func(t *testing.T) {
		opts := base
		opts.SampleFilter = everyOther()
		res, ok := search.Choose(c.in, c.pos, c.legal, c.settle, opts, farFuture())
		require.True(t, ok)
		assert.Equal(t, base.Samples, res.Samples)
	})

	t.Run("a filter accepting every deal changes nothing", func(t *testing.T) {
		opts := base
		opts.SampleFilter = func(hands [4]uint32) bool { return true }
		want, ok := search.Choose(c.in, c.pos, c.legal, c.settle, base, farFuture())
		require.True(t, ok)
		got, ok := search.Choose(c.in, c.pos, c.legal, c.settle, opts, farFuture())
		require.True(t, ok)
		assert.Equal(t, want, got)
		wantMean, _, _, _ := search.Evaluate(c.in, c.pos, c.settle, base, farFuture())
		gotMean, _, _, _ := search.Evaluate(c.in, c.pos, c.settle, opts, farFuture())
		assert.InDelta(t, wantMean, gotMean, 1e-9)
	})

	t.Run("the filter sees at most four draws per requested deal", func(t *testing.T) {
		opts := base
		draws := 0
		opts.SampleFilter = func(hands [4]uint32) bool {
			draws++
			return draws > 4*base.Samples-3
		}
		res, ok := search.Choose(c.in, c.pos, c.legal, c.settle, opts, farFuture())
		require.True(t, ok)
		assert.Equal(t, 4*base.Samples, draws)
		assert.Equal(t, 3, res.Samples)
	})
}
