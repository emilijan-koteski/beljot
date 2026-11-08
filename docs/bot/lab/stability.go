package main

import (
	"fmt"
	"math/rand/v2"
	"os"
	"sync"
	"time"

	"github.com/emilijan/beljot/server/internal/bot"
	"github.com/emilijan/beljot/server/internal/botlab"
	"github.com/emilijan/beljot/server/internal/game"
)

type stabilityTally struct{ positions, changed [9]int }

func (s *stabilityTally) merge(o *stabilityTally) {
	for t := range s.positions {
		s.positions[t] += o.positions[t]
		s.changed[t] += o.changed[t]
	}
}

func (s *stabilityTally) pooled(from, to int) (int, int) {
	positions, changed := 0, 0
	for t := from; t <= to; t++ {
		positions += s.positions[t]
		changed += s.changed[t]
	}
	return positions, changed
}

func share(changed, positions int) float64 {
	if positions == 0 {
		return 0
	}
	return float64(changed) / float64(positions)
}

// saltedCardsDiffer reports whether any seed salt below seeds picks another card than salt 0 does.
func saltedCardsDiffer(bv bot.View, tuning bot.SearchTuning, seeds int) bool {
	tuning.SeedSalt = 0
	base, ok := bot.SearchCardWith(bv, time.Now().Add(24*time.Hour), tuning)
	if !ok {
		return false
	}
	for salt := 1; salt < seeds; salt++ {
		tuning.SeedSalt = uint64(salt)
		got, ok := bot.SearchCardWith(bv, time.Now().Add(24*time.Hour), tuning)
		if ok && got.Card != base.Card {
			return true
		}
	}
	return false
}

// stability plays self-play hands and, at every searched position, re-runs the search under other sampler seeds to
// count how often the chosen card changes.
func stability(v game.Variant) {
	t0 := time.Now()
	deals := make([]botlab.Deal, *n)
	rng := rand.New(rand.NewPCG(*seed, 5))
	for i := range deals {
		deals[i] = botlab.RandomDeal(rng)
	}
	tallies := make([]stabilityTally, *workers)
	var wg sync.WaitGroup
	next := make(chan int)
	go func() {
		for i := range deals {
			next <- i
		}
		close(next)
	}()
	var name string
	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			A := makePolicy(*polA, rand.New(rand.NewPCG(*seed, uint64(300+w))), *flagsA)
			shipped, ok := A.(*botlab.ShippedBot)
			if !ok {
				fmt.Println("stability needs a shipped policy")
				os.Exit(1)
			}
			if w == 0 {
				name = A.Name()
			}
			hook := func(gs *game.GameState, seat int, action game.Action, view botlab.View) {
				bv := botlab.ToBotView(view)
				if !bot.SearchAppliesWith(bv, shipped.FromTrick, shipped.Tuning) {
					return
				}
				trick := view.TrickNumber
				tallies[w].positions[trick]++
				if saltedCardsDiffer(bv, shipped.Tuning, *seeds) {
					tallies[w].changed[trick]++
				}
			}
			for i := range next {
				gs := botlab.NewDealState(deals[i], v)
				if _, err := botlab.PlayHand(gs, [4]botlab.Policy{A, A, A, A}, hook); err != nil {
					fmt.Println("error:", err)
					os.Exit(1)
				}
			}
		}(w)
	}
	wg.Wait()
	var tot stabilityTally
	for w := range tallies {
		tot.merge(&tallies[w])
	}
	fmt.Printf("stability of %s (%s, %d hands, %d seeds, %.0fs)\n", name, v, *n, *seeds, time.Since(t0).Seconds())
	for t := 1; t <= 8; t++ {
		fmt.Printf("  trick %d: %5d positions, %5.1f%% changed\n", t, tot.positions[t], 100*share(tot.changed[t], tot.positions[t]))
	}
	for _, r := range [][2]int{{3, 4}, {5, 8}} {
		positions, changed := tot.pooled(r[0], r[1])
		fmt.Printf("  tricks %d-%d: %5d positions, %5.1f%% changed\n", r[0], r[1], positions, 100*share(changed, positions))
	}
}
