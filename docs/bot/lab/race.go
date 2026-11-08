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
	raceTarget     = 1001
	raceCloseSpan  = 151
	raceFarSpan    = 391
	raceMinimumGap = 10
)

type raceDeal struct {
	deal   botlab.Deal
	scores [2]int
}

type raceResult struct {
	pair                   stat
	games, aWins, bWins    int
	passed, instant, ended int
}

func (r *raceResult) merge(o *raceResult) {
	r.pair.merge(&o.pair)
	r.games += o.games
	r.aWins += o.aWins
	r.bWins += o.bWins
	r.passed += o.passed
	r.instant += o.instant
	r.ended += o.ended
}

// newRaceDeal draws a deal and match scores where one random team is 10 to 160 short of the target and the other
// 10 to 400 short.
func newRaceDeal(rng *rand.Rand) raceDeal {
	d := raceDeal{deal: botlab.RandomDeal(rng)}
	closeTeam := rng.IntN(2)
	d.scores[closeTeam] = raceTarget - (raceMinimumGap + rng.IntN(raceCloseSpan))
	d.scores[1-closeTeam] = raceTarget - (raceMinimumGap + rng.IntN(raceFarSpan))
	return d
}

// raceGame plays game g of a duplicate pair, A on the seats with (s+g)%2 == 0, and scores it for A's team: +1 for
// winning the match, -1 for losing it, 0 when the hand leaves it open.
func raceGame(d raceDeal, v game.Variant, A, B botlab.Policy, g int, res *raceResult) float64 {
	var pol [4]botlab.Policy
	for s := 0; s < 4; s++ {
		if (s+g)%2 == 0 {
			pol[s] = A
		} else {
			pol[s] = B
		}
	}
	out, err := botlab.PlayHand(botlab.NewDealStateWith(d.deal, v, d.scores, *stopAtTarget), pol, nil)
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
	res.games++
	switch out.Status {
	case botlab.HandPassedOut:
		res.passed++
	case botlab.HandInstantWin:
		res.instant++
	}
	if out.Winner == nil {
		return 0
	}
	res.ended++
	if *out.Winner == g%2 {
		res.aWins++
		return 1
	}
	res.bWins++
	return -1
}

func race(v game.Variant) {
	t0 := time.Now()
	deals := make([]raceDeal, *n)
	rng := rand.New(rand.NewPCG(*seed, 4))
	for i := range deals {
		deals[i] = newRaceDeal(rng)
	}
	results := make([]raceResult, *workers)
	var wg sync.WaitGroup
	next := make(chan int)
	go func() {
		for i := 0; i < *n; i++ {
			next <- i
		}
		close(next)
	}()
	var nameA, nameB string
	var countsA, countsB botlab.Counts
	var countingA, countingB bool
	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			A := makePolicy(*polA, rand.New(rand.NewPCG(*seed, uint64(100+w))), *flagsA)
			B := makePolicy(*polB, rand.New(rand.NewPCG(*seed, uint64(200+w))), *flagsB)
			if w == 0 {
				nameA, nameB = A.Name(), B.Name()
			}
			defer func() {
				countsMu.Lock()
				defer countsMu.Unlock()
				countingA = addCounts(&countsA, A) || countingA
				countingB = addCounts(&countsB, B) || countingB
			}()
			res := &results[w]
			for i := range next {
				total := raceGame(deals[i], v, A, B, 0, res) + raceGame(deals[i], v, A, B, 1, res)
				res.pair.add(total / 2)
			}
		}(w)
	}
	wg.Wait()
	var tot raceResult
	for w := range results {
		tot.merge(&results[w])
	}
	fmt.Printf("%s vs %s (%s race to %d, %d deals x2, %.0fs)\n", nameA, nameB, v, raceTarget, *n, time.Since(t0).Seconds())
	fmt.Printf("  A match edge per game: %+.4f ± %.4f (95%% CI ±%.4f)\n", tot.pair.mean(), tot.pair.se(), 1.96*tot.pair.se())
	fmt.Printf("  decided %d / %d games (%.1f%%): A won %d, B won %d; mid-hand stops %d, passed out %d\n",
		tot.ended, tot.games, pct(tot.ended, tot.games), tot.aWins, tot.bWins, tot.instant, tot.passed)
	if countingA {
		printCounts("A", countsA)
	}
	if countingB {
		printCounts("B", countsB)
	}
}
