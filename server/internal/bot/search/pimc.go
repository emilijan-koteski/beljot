package search

import (
	"math"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/emilijan/beljot/server/internal/game"
)

const samplerStream = 0x5EED

// Position is the trick in progress as the solver sees it: the trump suit index, the direction of play, who led the
// trick, the card indexes already played to it and the capot flag.
type Position struct {
	Trump     int
	Clockwise bool
	Leader    int
	Trick     []int
	Capot     int8
}

// Options bound the search: at most Samples deals, and a stop once at least EarlyStopAfter are in and the best card
// leads the runner-up by more than EarlyStopSE standard errors.
type Options struct {
	Samples        int
	EarlyStopAfter int
	EarlyStopSE    float64
}

// Result is the chosen card, how many deals backed it, its mean settled value for the seat's team, its lead over
// the runner-up and the standard error of that lead.
type Result struct {
	Card    game.Card
	Samples int
	Mean    float64
	Margin  float64
	SE      float64
}

// SamplesForTrick is the number of deals to search at the given 1-based trick, or 0 where the bot should not search.
func SamplesForTrick(trick int) int {
	switch {
	case trick < 3:
		return 0
	case trick == 3:
		return 16
	case trick == 4:
		return 30
	case trick == 5:
		return 40
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
	votes := newTally(cards)
	teamSign := 1 - 2*game.TeamForSeat(in.Seat)
	rng := rand.New(rand.NewPCG(in.Seed(), samplerStream))
	solver := NewSolver(pos.Trump, pos.Clockwise)
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
		if !votes.add(remainders, s, teamSign) {
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

func legalIndexes(legal []game.Card) []int {
	cards := make([]int, len(legal))
	for i, card := range legal {
		cards[i] = CardIdx(card)
	}
	slices.Sort(cards)
	return slices.Compact(cards)
}

// tally keeps, per candidate card, the settled value of every sample so far and their sum.
type tally struct {
	cards  []int
	values [NCards][]int
	sums   [NCards]int
	n      int
}

func newTally(cards []int) *tally {
	return &tally{cards: cards}
}

func (t *tally) add(remainders map[int]int, s Settlement, teamSign int) bool {
	settled := make([]int, len(t.cards))
	for i, card := range t.cards {
		remainder, ok := remainders[card]
		if !ok {
			return false
		}
		settled[i] = teamSign * Settle(s, remainder)
	}
	for i, card := range t.cards {
		t.values[card] = append(t.values[card], settled[i])
		t.sums[card] += settled[i]
	}
	t.n++
	return true
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
	return se > 0 && margin > stopSE*se
}

func (t *tally) result() Result {
	best, _ := t.ranking()
	margin, se := t.lead()
	return Result{
		Card:    IdxCard(best),
		Samples: t.n,
		Mean:    float64(t.sums[best]) / float64(t.n),
		Margin:  margin,
		SE:      se,
	}
}
