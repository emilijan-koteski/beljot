package main

import (
	"fmt"
	"math/rand/v2"

	"github.com/emilijan/beljot/server/internal/botlab"
	"github.com/emilijan/beljot/server/internal/game"
)

// shapecheck draws random deals until 20 contain a position fitting A,c4,noJ,no9,sA0 and prints each one's matches and the hit rate.
func shapecheck(v game.Variant) {
	s, err := ParseShape("A,c4,noJ,no9,sA0")
	if err != nil {
		panic(err)
	}
	rng := rand.New(rand.NewPCG(*seed, 77))
	drawn, shown := 0, 0
	for shown < 20 {
		drawn++
		gs := botlab.NewDealState(botlab.RandomDeal(rng), v)
		matches := matchingPositions(gs, s)
		if len(matches) == 0 {
			continue
		}
		shown++
		fmt.Printf("deal %2d dealer %d cand %v matches %v\n", shown, gs.DealerSeat, gs.TrumpCandidate, matches)
		for step := 0; step < 4; step++ {
			seat := (gs.DealerSeat + 1 + step) % 4
			fmt.Printf("   step %d seat %d hand %v\n", step, seat, gs.Players[seat].Hand)
		}
	}
	fmt.Printf("%d of %d random deals have a matching position\n", shown, drawn)
}
