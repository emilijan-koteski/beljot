package main

import (
	"fmt"
	"math/rand/v2"
	"os"
	"sync"
	"time"

	"github.com/emilijan/beljot/server/internal/botlab"
	"github.com/emilijan/beljot/server/internal/game"
)

const (
	bidScoreHandCap  = 12
	bidScoreNearMin  = 10
	bidScoreNearSpan = 91
	bidScoreFarMin   = 200
	bidScoreFarSpan  = 401
	bidScoreMidMin   = 300
	bidScoreMidSpan  = 301
)

type bidCase struct {
	deal    botlab.Deal
	seat    int
	prefix  []game.Action
	scores  [2]int
	pairIdx int
}

type bidScoreResult struct {
	diff                  stat
	callWins, passWins    int
	callOpen, passOpen    int
	callHands, passHands  int
	passedOutForcedBranch int
}

func (r *bidScoreResult) merge(o *bidScoreResult) {
	r.diff.merge(&o.diff)
	r.callWins += o.callWins
	r.passWins += o.passWins
	r.callOpen += o.callOpen
	r.passOpen += o.passOpen
	r.callHands += o.callHands
	r.passHands += o.passHands
	r.passedOutForcedBranch += o.passedOutForcedBranch
}

func validScoreState(state string) bool {
	switch state {
	case "oppClose", "ownClose", "bothClose", "neither":
		return true
	}
	return false
}

// bidScoreStart draws the match scores of a score state for the tested seat's team: the close side 10 to 100 short
// of the target, the far side 200 to 600, and both 300 to 600 for neither.
func bidScoreStart(state string, team int, rng *rand.Rand) [2]int {
	near := func() int { return raceTarget - (bidScoreNearMin + rng.IntN(bidScoreNearSpan)) }
	far := func() int { return raceTarget - (bidScoreFarMin + rng.IntN(bidScoreFarSpan)) }
	mid := func() int { return raceTarget - (bidScoreMidMin + rng.IntN(bidScoreMidSpan)) }
	var s [2]int
	switch state {
	case "oppClose":
		s[1-team], s[team] = near(), far()
	case "ownClose":
		s[team], s[1-team] = near(), far()
	case "bothClose":
		s[team], s[1-team] = near(), near()
	default:
		s[team], s[1-team] = mid(), mid()
	}
	return s
}

func shapeFitsSeat(gs *game.GameState, step int, s *Shape, seat int) bool {
	if s == nil {
		return seat == 0
	}
	for _, su := range biddingSuits(gs, step) {
		if s.Matches(gs.Players[seat].Hand, gs.TrumpCandidate, su, gs.BiddingPassCount) {
			return true
		}
	}
	return false
}

// bidScoreWalk plays the natural auction of pol until the first seat whose position fits the shape (seat 0 without a
// shape) and returns the actions before it; ok is false when the auction ends first or the seat has no free choice.
func bidScoreWalk(gs *game.GameState, s *Shape, pol botlab.Policy) (prefix []game.Action, seat int, ok bool) {
	gs.Phase = game.PhaseBidding
	game.RefreshDerivedFlags(gs)
	for step := 0; step < 8; step++ {
		seat = gs.ActivePlayerSeat
		forced := game.MustPickTrump(gs, seat)
		if shapeFitsSeat(gs, step, s, seat) {
			return prefix, seat, !forced
		}
		natural := pol.Decide(botlab.BuildView(gs, seat, botlab.NewMemory()))
		if natural.Type == game.ActionPickTrump || forced || isLastReshuffleSeat(gs) {
			return nil, 0, false
		}
		next, err := game.ApplyAction(gs, natural)
		if err != nil {
			panic(err)
		}
		prefix = append(prefix, natural)
		gs = next
	}
	return nil, 0, false
}

func isLastReshuffleSeat(gs *game.GameState) bool {
	return gs.Rules.AllPassOutcome == game.AllPassReshuffleAndRotate && gs.BiddingRound == 2 && gs.BiddingPassCount == 3
}

// forcedSuitScore ranks a suit for a forced call by its length, then the trump-order card points of the hand.
func forcedSuitScore(cards []game.Card, suit game.Suit) int {
	points := map[game.Rank]int{game.RankJack: 20, game.Rank9: 14, game.RankAce: 11, game.RankTen: 10, game.RankKing: 4, game.RankQueen: 3}
	score := 0
	for _, c := range cards {
		if c.Suit == suit {
			score += 100 + points[c.Rank]
		}
	}
	return score
}

// forcedCall is the pick action of the tested seat: the candidate's suit in round 1 of a candidate variant, else the
// allowed suit with the highest forcedSuitScore over the hand and the candidate.
func forcedCall(gs *game.GameState, seat int) game.Action {
	act := game.Action{Type: game.ActionPickTrump, PlayerSeat: seat}
	if gs.TrumpCandidate != nil && gs.BiddingRound == 1 {
		return act
	}
	cards := gs.Players[seat].Hand
	if gs.TrumpCandidate != nil {
		cards = append(append([]game.Card{}, cards...), *gs.TrumpCandidate)
	}
	best, bestScore := game.Suit(""), -1
	for _, su := range game.AllSuits {
		if gs.TrumpCandidate != nil && su == gs.TrumpCandidate.Suit {
			continue
		}
		if sc := forcedSuitScore(cards, su); sc > bestScore {
			best, bestScore = su, sc
		}
	}
	act.Suit = &best
	return act
}

// bidScoreMatch plays the case with the tested seat's forced action and continues with fresh deals of the pair's own
// stream until the match is decided; it returns +1, -1 or 0 for the tested team, the hands played and whether the
// forced pass was a passed-out hand.
func bidScoreMatch(c bidCase, v game.Variant, call bool, pol [4]botlab.Policy) (result float64, hands int, passedOut bool) {
	gs := botlab.NewDealStateWith(c.deal, v, c.scores, *stopAtTarget)
	gs.Phase = game.PhaseBidding
	game.RefreshDerivedFlags(gs)
	for _, a := range c.prefix {
		next, err := game.ApplyAction(gs, a)
		if err != nil {
			panic(err)
		}
		gs = next
	}
	team := c.seat % 2
	stream := rand.New(rand.NewPCG(*seed, uint64(7000+c.pairIdx)))
	var out botlab.HandOutcome
	var err error
	switch {
	case call:
		candidate := gs.TrumpCandidate
		forced, e := game.ApplyAction(gs, forcedCall(gs, c.seat))
		if e != nil {
			panic(e)
		}
		out, err = botlab.PlayHandFrom(forced, pol, nil, botlab.MemoryAfterForcedPick(candidate, c.seat))
	case isLastReshuffleSeat(gs):
		out = botlab.HandOutcome{Status: botlab.HandPassedOut}
		passedOut = true
	default:
		passed, e := game.ApplyAction(gs, game.Action{Type: game.ActionPassTrump, PlayerSeat: c.seat})
		if e != nil {
			panic(e)
		}
		out, err = botlab.PlayHand(passed, pol, nil)
	}
	scores := c.scores
	for hands = 1; ; hands++ {
		if err != nil {
			fmt.Println("error:", err)
			os.Exit(1)
		}
		a, b := botlab.Award(out)
		scores[0] += a
		scores[1] += b
		if out.Winner != nil {
			if *out.Winner == team {
				return 1, hands, passedOut
			}
			return -1, hands, passedOut
		}
		if hands >= bidScoreHandCap {
			return 0, hands, passedOut
		}
		next := botlab.NewDealStateWith(botlab.RandomDeal(stream), v, scores, *stopAtTarget)
		out, err = botlab.PlayHand(next, pol, nil)
	}
}

func bidScoreCases(v game.Variant, shape *Shape) []bidCase {
	rng := rand.New(rand.NewPCG(*seed, 12))
	reach := makePolicy(*polA, rand.New(rand.NewPCG(*seed, 499)), *flagsA)
	cases := make([]bidCase, 0, *n)
	for len(cases) < *n {
		var d botlab.Deal
		if shape == nil {
			d = botlab.RandomDeal(rng)
		} else {
			d = mustTargetedDeal(rng, v, shape, reach)
		}
		prefix, seat, ok := bidScoreWalk(botlab.NewDealState(d, v), shape, reach)
		if !ok {
			continue
		}
		cases = append(cases, bidCase{deal: d, seat: seat, prefix: prefix, scores: bidScoreStart(*scoreState, seat%2, rng), pairIdx: len(cases)})
	}
	return cases
}

func bidscore(v game.Variant) {
	if !validScoreState(*scoreState) {
		fmt.Fprintf(os.Stderr, "-scoreState must be oppClose, ownClose, bothClose or neither, got %q\n", *scoreState)
		os.Exit(1)
	}
	t0 := time.Now()
	shape := shapeFilter()
	cases := bidScoreCases(v, shape)
	results := make([]bidScoreResult, *workers)
	var wg sync.WaitGroup
	next := make(chan int)
	go func() {
		for i := range cases {
			next <- i
		}
		close(next)
	}()
	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			A := makePolicy(*polA, rand.New(rand.NewPCG(*seed, uint64(600+w))), *flagsA)
			pol := [4]botlab.Policy{A, A, A, A}
			res := &results[w]
			for i := range next {
				call, callHands, _ := bidScoreMatch(cases[i], v, true, pol)
				pass, passHands, passedOut := bidScoreMatch(cases[i], v, false, pol)
				if passedOut {
					res.passedOutForcedBranch++
				}
				res.diff.add(call - pass)
				res.callHands += callHands
				res.passHands += passHands
				tally(call, &res.callWins, &res.callOpen)
				tally(pass, &res.passWins, &res.passOpen)
			}
		}(w)
	}
	wg.Wait()
	var tot bidScoreResult
	for w := range results {
		tot.merge(&results[w])
	}
	if tot.diff.n == 0 {
		fmt.Println("bidscore: no pairs")
		return
	}
	fmt.Printf("bidscore %s: %d pairs, call-minus-pass match wins %+.3f ± %.3f, call wins %d/%d, pass wins %d/%d\n",
		*scoreState, tot.diff.n, tot.diff.mean(), tot.diff.se(), tot.callWins, tot.diff.n, tot.passWins, tot.diff.n)
	fmt.Printf("  %s, stopAtTarget=%t, open after %d hands: call %d, pass %d; mean hands per match: call %.2f, pass %.2f; forced pass at the reshuffle seat %d; %.1fs (%.2fs per pair)\n",
		v, *stopAtTarget, bidScoreHandCap, tot.callOpen, tot.passOpen, float64(tot.callHands)/float64(tot.diff.n),
		float64(tot.passHands)/float64(tot.diff.n), tot.passedOutForcedBranch, time.Since(t0).Seconds(), time.Since(t0).Seconds()/float64(tot.diff.n))
	if shape != nil {
		fmt.Printf("WARNING: targeted deals (-shape=%s): the result is valid only for the targeted cell\n", *shapeSpec)
	}
}

func tally(result float64, wins, open *int) {
	switch {
	case result > 0:
		*wins++
	case result == 0:
		*open++
	}
}
