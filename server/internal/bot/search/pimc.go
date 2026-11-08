package search

import (
	"math"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/emilijan/beljot/server/internal/game"
)

const samplerStream = 0x5EED

// Position is the trick in progress as the solver sees it: trump suit index, direction of play, the trick's leader,
// its card indexes and the capot flag, plus Race when the solver should race for a stop-at-target match.
type Position struct {
	Trump     int
	Clockwise bool
	Leader    int
	Trick     []int
	Capot     int8
	Race      *Race
}

// Options bound the search: at most Samples deals, and a stop once at least EarlyStopAfter are in and the best card
// leads the runner-up by more than EarlyStopSE standard errors. SampleClass optionally sorts each deal into class 0
// or 1, and SeedSalt is mixed into the sampler seed.
type Options struct {
	Samples        int
	EarlyStopAfter int
	EarlyStopSE    float64
	SampleClass    func(hands [4]uint32) int
	SeedSalt       uint64
}

// CardStats is a card's mean settled value and the standard error of that mean, 0 below two samples.
type CardStats struct {
	Mean, SE float64
}

// ClassStats is how many samples fell in one class and each legal card's mean settled value over them.
type ClassStats struct {
	Samples int
	Means   map[int]float64
}

// Result is the chosen card, how many deals backed it, its mean settled value for the seat's team, its lead over
// the runner-up and the standard error of that lead, per-card and per-class statistics, and the chosen card's share
// of deals in which the caller made the contract.
type Result struct {
	Card       game.Card
	Samples    int
	Mean       float64
	Margin     float64
	SE         float64
	Cards      map[int]CardStats
	Classes    [2]ClassStats
	CallerMade float64
}

const OpeningSamples = 12

// SamplesForTrick is the number of deals to search at the given 1-based trick, or 0 where the bot should not search.
func SamplesForTrick(trick int) int {
	switch {
	case trick < 1:
		return 0
	case trick < 3:
		return OpeningSamples
	case trick == 3:
		return 32
	case trick == 4:
		return 60
	case trick == 5:
		return 80
	}
	return 60
}

// Choose plays the legal card whose settled value, averaged over deals sampled from the seat's information, is best
// for its team. It reports false when the deadline passes, no deal can be drawn or the request is inconsistent.
func Choose(
	in Information, pos Position, legal []game.Card, s Settlement, opts Options, deadline time.Time,
) (Result, bool) {
	cards := legalIndexes(legal)
	if len(cards) == 0 || game.StepSeat(pos.Leader, len(pos.Trick), pos.Clockwise) != in.Seat {
		return Result{}, false
	}
	votes := newTally(cards, pos.Race)
	teamSign := 1 - 2*game.TeamForSeat(in.Seat)
	rng := rand.New(rand.NewPCG(in.Seed()^opts.SeedSalt, samplerStream))
	solver := NewSolver(pos.Trump, pos.Clockwise)
	solver.SetRace(pos.Race)
	for votes.n < opts.Samples {
		if time.Now().After(deadline) {
			return Result{}, false
		}
		hands, ok := Sample(rng, in)
		if !ok {
			break
		}
		solver.Reset()
		remainders := solver.RootValues(hands, pos.Leader, pos.Trick, pos.Capot)
		class := -1
		if opts.SampleClass != nil {
			class = opts.SampleClass(hands)
		}
		if !votes.add(remainders, s, teamSign, class) {
			return Result{}, false
		}
		if votes.n >= opts.EarlyStopAfter && votes.decided(opts.EarlyStopSE) {
			break
		}
	}
	if votes.n == 0 {
		return Result{}, false
	}
	return votes.result(), true
}

// Evaluate values the position with no card to choose: the settled value for team A averaged over deals sampled from
// the seat's information, its standard error and the share of deals the caller made, false as Choose reports false.
func Evaluate(in Information, pos Position, s Settlement, opts Options, deadline time.Time) (mean, se, made float64, ok bool) {
	if len(pos.Trick) > seats {
		return 0, 0, 0, false
	}
	rng := rand.New(rand.NewPCG(in.Seed()^opts.SeedSalt, samplerStream))
	solver := NewSolver(pos.Trump, pos.Clockwise)
	solver.SetRace(pos.Race)
	values := make([]int, 0, opts.Samples)
	madeCount := 0
	for len(values) < opts.Samples {
		if time.Now().After(deadline) {
			return 0, 0, 0, false
		}
		hands, sampled := Sample(rng, in)
		if !sampled {
			break
		}
		solver.Reset()
		remainder := rootValue(solver, hands, pos)
		values = append(values, settledValue(pos.Race, s, remainder))
		if callerMadeIn(pos.Race, s, remainder) {
			madeCount++
		}
	}
	if len(values) == 0 {
		return 0, 0, 0, false
	}
	stats := statsOf(values)
	return stats.Mean, stats.SE, float64(madeCount) / float64(len(values)), true
}

// rootValue solves the position as it stands; a full trick is valued by replaying its last card, which closes it.
func rootValue(solver *Solver, hands [4]uint32, pos Position) int {
	n := len(pos.Trick)
	if n < seats {
		return solver.Value(hands, pos.Leader, pos.Trick, pos.Capot)
	}
	last := pos.Trick[n-1]
	hands[solver.seatAt[pos.Leader][n-1]] |= 1 << uint(last)
	return solver.play(hands, pos.Leader, pos.Trick[:n-1], last, pos.Capot, -inf, inf)
}

func legalIndexes(legal []game.Card) []int {
	cards := make([]int, len(legal))
	for i, card := range legal {
		cards[i] = CardIdx(card)
	}
	slices.Sort(cards)
	return slices.Compact(cards)
}

// tally keeps, per candidate card, the settled value of every sample so far and their sum, valuing the hand's end as
// the match when it races.
type tally struct {
	race   *Race
	cards  []int
	values [NCards][]int
	sums   [NCards]int
	made   [NCards]int
	n      int

	classSums    [2][NCards]int
	classSamples [2]int
}

func newTally(cards []int, race *Race) *tally {
	return &tally{cards: cards, race: race}
}

func (t *tally) add(remainders map[int]int, s Settlement, teamSign, class int) bool {
	settled := make([]int, len(t.cards))
	for i, card := range t.cards {
		remainder, ok := remainders[card]
		if !ok {
			return false
		}
		settled[i] = teamSign * settledValue(t.race, s, remainder)
	}
	for i, card := range t.cards {
		t.values[card] = append(t.values[card], settled[i])
		t.sums[card] += settled[i]
		if callerMadeIn(t.race, s, remainders[card]) {
			t.made[card]++
		}
	}
	if class == 0 || class == 1 {
		for i, card := range t.cards {
			t.classSums[class][card] += settled[i]
		}
		t.classSamples[class]++
	}
	t.n++
	return true
}

// settledValue is a root value as the hand's award, or ±RaceWin for a match decided mid-hand or by the settled hand.
func settledValue(race *Race, s Settlement, value int) int {
	if value == RaceWin || value == -RaceWin {
		return value
	}
	if race != nil {
		if winner, decided := HandEndDecided(*race, SettleTotals(s, value)); decided {
			return raceValue(winner)
		}
	}
	return Settle(s, value)
}

// callerMadeIn reports whether the caller made the hand behind a root value; a match decided mid-hand counts as made
// when the caller's team won it.
func callerMadeIn(race *Race, s Settlement, value int) bool {
	if race != nil && (value == RaceWin || value == -RaceWin) {
		return raceValue(s.CallerTeam) == value
	}
	return CallerMade(s, value)
}

// ranking returns the best card and the runner-up, preferring the lowest card index on equal sums.
func (t *tally) ranking() (best, runnerUp int) {
	best, runnerUp = -1, -1
	for _, card := range t.cards {
		switch {
		case best < 0 || t.sums[card] > t.sums[best]:
			best, runnerUp = card, best
		case runnerUp < 0 || t.sums[card] > t.sums[runnerUp]:
			runnerUp = card
		}
	}
	return best, runnerUp
}

// lead returns the mean of best minus runner-up over the samples and its standard error, both 0 without a rival.
func (t *tally) lead() (margin, se float64) {
	best, runnerUp := t.ranking()
	if runnerUp < 0 || t.n == 0 {
		return 0, 0
	}
	n := float64(t.n)
	margin = float64(t.sums[best]-t.sums[runnerUp]) / n
	if t.n < 2 {
		return margin, 0
	}
	squares := 0.0
	for k := range t.n {
		deviation := float64(t.values[best][k]-t.values[runnerUp][k]) - margin
		squares += deviation * deviation
	}
	return margin, math.Sqrt(squares/(n-1)) / math.Sqrt(n)
}

func (t *tally) decided(stopSE float64) bool {
	margin, se := t.lead()
	return margin > stopSE*se
}

func (t *tally) cardStats(card int) CardStats {
	return statsOf(t.values[card])
}

// statsOf is the mean of the values and its standard error, 0 below two values.
func statsOf(values []int) CardStats {
	n := float64(len(values))
	sum := 0
	for _, v := range values {
		sum += v
	}
	mean := float64(sum) / n
	if len(values) < 2 {
		return CardStats{Mean: mean}
	}
	squares := 0.0
	for _, v := range values {
		deviation := float64(v) - mean
		squares += deviation * deviation
	}
	return CardStats{Mean: mean, SE: math.Sqrt(squares/(n-1)) / math.Sqrt(n)}
}

func (t *tally) cardsAndClasses() (map[int]CardStats, [2]ClassStats) {
	cards := make(map[int]CardStats, len(t.cards))
	for _, card := range t.cards {
		cards[card] = t.cardStats(card)
	}
	var classes [2]ClassStats
	for class := range classes {
		classes[class].Samples = t.classSamples[class]
		if t.classSamples[class] == 0 {
			continue
		}
		classes[class].Means = make(map[int]float64, len(t.cards))
		for _, card := range t.cards {
			classes[class].Means[card] = float64(t.classSums[class][card]) / float64(t.classSamples[class])
		}
	}
	return cards, classes
}

func (t *tally) result() Result {
	best, _ := t.ranking()
	margin, se := t.lead()
	cards, classes := t.cardsAndClasses()
	return Result{
		Card:       IdxCard(best),
		Samples:    t.n,
		Mean:       float64(t.sums[best]) / float64(t.n),
		Margin:     margin,
		SE:         se,
		Cards:      cards,
		Classes:    classes,
		CallerMade: float64(t.made[best]) / float64(t.n),
	}
}
