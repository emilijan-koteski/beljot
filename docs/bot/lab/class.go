package main

import (
	"fmt"
	"math/rand/v2"
	"os"

	"github.com/emilijan/beljot/server/internal/botlab"
	"github.com/emilijan/beljot/server/internal/game"
)

const (
	classBelow = "below"
	classTable = "table"
	classCore  = "core"
)

type trumpHolding struct {
	count, sideAces int
	j, nine, ace    bool
}

func holdingOf(cards []game.Card, trump game.Suit) trumpHolding {
	var h trumpHolding
	for _, c := range cards {
		if c.Suit != trump {
			if c.Rank == game.RankAce {
				h.sideAces++
			}
			continue
		}
		h.count++
		switch c.Rank {
		case game.RankJack:
			h.j = true
		case game.Rank9:
			h.nine = true
		case game.RankAce:
			h.ace = true
		}
	}
	return h
}

// ruleA is the Go port of class_table.py's ruleA: the hand shapes whose call gains points over the pass.
func ruleA(h trumpHolding) bool {
	switch {
	case h.j && h.nine && h.count >= 2,
		h.j && h.count >= 3,
		h.count >= 4 && (h.j || h.nine || h.ace),
		h.count >= 5,
		h.nine && h.ace && h.count >= 3 && h.sideAces >= 1,
		h.j && h.count == 2 && h.sideAces >= 2:
		return true
	}
	return false
}

// coreHolding is the tighten class: J and 9 with two trumps or more, J with four or more, or five trumps or more.
func coreHolding(h trumpHolding) bool {
	return (h.j && h.nine && h.count >= 2) || (h.j && h.count >= 4) || h.count >= 5
}

func classCards(gs *game.GameState, seat int) []game.Card {
	cards := gs.Players[seat].Hand
	if gs.TrumpCandidate != nil {
		cards = append(append([]game.Card{}, cards...), *gs.TrumpCandidate)
	}
	return cards
}

func suitsWhere(gs *game.GameState, step, seat int, pred func(trumpHolding) bool) []game.Suit {
	cards := classCards(gs, seat)
	var out []game.Suit
	for _, su := range biddingSuits(gs, step) {
		if pred(holdingOf(cards, su)) {
			out = append(out, su)
		}
	}
	return out
}

func validClass(class string) bool {
	switch class {
	case "", classBelow, classTable, classCore:
		return true
	}
	return false
}

// classCall returns the call action of the tested seat when its position is in the class: the natural call for table
// and core, and for below the ruleA suit with the best forcedSuitScore (the candidate itself in Bitola round 1).
func classCall(class string, gs *game.GameState, step, seat int, natural game.Action) (game.Action, bool) {
	naturalCall := natural.Type == game.ActionPickTrump
	switch class {
	case classTable:
		return natural, naturalCall
	case classCore:
		return natural, naturalCall && len(suitsWhere(gs, step, seat, coreHolding)) > 0
	case classBelow:
		suits := suitsWhere(gs, step, seat, ruleA)
		if naturalCall || len(suits) == 0 {
			return game.Action{}, false
		}
		return belowCall(gs, seat, suits), true
	}
	return game.Action{}, false
}

func belowCall(gs *game.GameState, seat int, suits []game.Suit) game.Action {
	act := game.Action{Type: game.ActionPickTrump, PlayerSeat: seat}
	if gs.TrumpCandidate != nil && gs.BiddingRound == 1 {
		return act
	}
	cards := classCards(gs, seat)
	best, bestScore := suits[0], -1
	for _, su := range suits {
		if sc := forcedSuitScore(cards, su); sc > bestScore {
			best, bestScore = su, sc
		}
	}
	act.Suit = &best
	return act
}

type ruleACase struct {
	h    trumpHolding
	want bool
}

var ruleACases = []ruleACase{
	{trumpHolding{count: 2, j: true, nine: true}, true},
	{trumpHolding{count: 1, j: true, nine: true}, false},
	{trumpHolding{count: 3, j: true}, true},
	{trumpHolding{count: 2, j: true}, false},
	{trumpHolding{count: 2, j: true, sideAces: 1}, false},
	{trumpHolding{count: 2, j: true, sideAces: 2}, true},
	{trumpHolding{count: 4, nine: true}, true},
	{trumpHolding{count: 4, ace: true}, true},
	{trumpHolding{count: 4}, false},
	{trumpHolding{count: 4, sideAces: 3}, false},
	{trumpHolding{count: 5}, true},
	{trumpHolding{count: 3, nine: true, ace: true, sideAces: 1}, true},
	{trumpHolding{count: 3, nine: true, ace: true}, false},
	{trumpHolding{count: 2, nine: true, ace: true, sideAces: 2}, false},
	{trumpHolding{count: 3, nine: true, sideAces: 2}, false},
}

// classcheck checks ruleA against a hand-written truth table, then walks the natural auction of -a over random deals
// and counts the free positions per class, including core positions the policy passes (core should be inside table).
func classcheck(v game.Variant) {
	bad := 0
	for _, c := range ruleACases {
		if ruleA(c.h) != c.want {
			bad++
			fmt.Printf("ruleA mismatch: %+v want %t\n", c.h, c.want)
		}
	}
	fmt.Printf("classcheck ruleA: %d/%d truth-table cases agree\n", len(ruleACases)-bad, len(ruleACases))
	rng := rand.New(rand.NewPCG(*seed, 78))
	pol := makePolicy(*polA, rand.New(rand.NewPCG(*seed, 499)), *flagsA)
	var positions, table, below, core, corePassed int
	for i := 0; i < *n; i++ {
		gs := botlab.NewDealState(botlab.RandomDeal(rng), v)
		gs.Phase = game.PhaseBidding
		game.RefreshDerivedFlags(gs)
		for step := 0; step < 8; step++ {
			seat := gs.ActivePlayerSeat
			if game.MustPickTrump(gs, seat) {
				break
			}
			positions++
			natural := pol.Decide(botlab.BuildView(gs, seat, botlab.NewMemory()))
			isCore := len(suitsWhere(gs, step, seat, coreHolding)) > 0
			if _, ok := classCall(classTable, gs, step, seat, natural); ok {
				table++
			}
			if _, ok := classCall(classBelow, gs, step, seat, natural); ok {
				below++
			}
			if _, ok := classCall(classCore, gs, step, seat, natural); ok {
				core++
			}
			if isCore && natural.Type != game.ActionPickTrump {
				corePassed++
			}
			if natural.Type == game.ActionPickTrump || isLastReshuffleSeat(gs) {
				break
			}
			next, err := game.ApplyAction(gs, natural)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			gs = next
		}
	}
	fmt.Printf("classcheck %s -a=%s: %d deals, %d free positions: table %d, below %d, core %d, core positions the policy passes %d\n",
		v, *polA, *n, positions, table, below, core, corePassed)
}
