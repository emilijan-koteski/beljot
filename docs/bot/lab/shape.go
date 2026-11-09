package main

import (
	"fmt"
	"math/rand/v2"
	"os"
	"strings"

	"github.com/emilijan/beljot/server/internal/botlab"
	"github.com/emilijan/beljot/server/internal/game"
)

const shapeAny = -1

// Shape is a hand-shape predicate in the vocabulary of class_table.py. Count, SideAces and Pos are shapeAny when unconstrained.
type Shape struct {
	Count, CountMin           int
	J, No9, Has9, NoJ, A, NoA bool
	SideAces, SideAcesMin     int
	Pos                       int
}

// ParseShape builds a Shape from a comma list such as "A,c4,noJ,no9,sA0,pos1". It rejects an unknown token, a
// second token for the same field and a contradictory pair.
func ParseShape(spec string) (Shape, error) {
	s := Shape{Count: shapeAny, SideAces: shapeAny, Pos: shapeAny}
	seen := map[string]bool{}
	for _, tok := range strings.Split(spec, ",") {
		tok = strings.TrimSpace(tok)
		field := shapeField(tok)
		if field != "" && seen[field] {
			return Shape{}, fmt.Errorf("repeated shape field in %q", spec)
		}
		seen[field] = true
		switch {
		case tok == "":
		case tok == "J":
			s.J = true
		case tok == "noJ":
			s.NoJ = true
		case tok == "9":
			s.Has9 = true
		case tok == "no9":
			s.No9 = true
		case tok == "A":
			s.A = true
		case tok == "noA":
			s.NoA = true
		case tok == "sA1+" || tok == "sA2+":
			s.SideAcesMin = int(tok[2] - '0')
		case len(tok) == 3 && tok[:2] == "sA" && tok[2] >= '0' && tok[2] <= '4':
			s.SideAces = int(tok[2] - '0')
		case len(tok) == 3 && tok[0] == 'c' && tok[2] == '+' && tok[1] >= '0' && tok[1] <= '8':
			s.CountMin = int(tok[1] - '0')
		case len(tok) == 2 && tok[0] == 'c' && tok[1] >= '0' && tok[1] <= '8':
			s.Count = int(tok[1] - '0')
		case len(tok) == 4 && tok[:3] == "pos" && tok[3] >= '0' && tok[3] <= '3':
			s.Pos = int(tok[3] - '0')
		default:
			return Shape{}, fmt.Errorf("unknown shape token %q", tok)
		}
	}
	if (s.J && s.NoJ) || (s.Has9 && s.No9) || (s.A && s.NoA) {
		return Shape{}, fmt.Errorf("contradictory shape %q", spec)
	}
	if (s.Count != shapeAny && s.Count < s.CountMin) || (s.SideAces != shapeAny && s.SideAces < s.SideAcesMin) {
		return Shape{}, fmt.Errorf("contradictory shape %q: an exact count below its at-least bound", spec)
	}
	return s, nil
}

// Matches reports whether the bidding position (hand plus the candidate joining it, the trump suit, passes before) fits the shape.
func (s Shape) Matches(hand []game.Card, candidate *game.Card, trump game.Suit, passesBefore int) bool {
	if s.Pos != shapeAny && passesBefore != s.Pos {
		return false
	}
	cards := hand
	if candidate != nil {
		cards = append(append([]game.Card{}, hand...), *candidate)
	}
	h := holdingOf(cards, trump)
	switch {
	case s.Count != shapeAny && h.count != s.Count,
		h.count < s.CountMin,
		s.SideAces != shapeAny && h.sideAces != s.SideAces,
		h.sideAces < s.SideAcesMin,
		s.J && !h.j, s.NoJ && h.j,
		s.Has9 && !h.nine, s.No9 && h.nine,
		s.A && !h.ace, s.NoA && h.ace:
		return false
	}
	return true
}

func biddingSuits(gs *game.GameState, step int) []game.Suit {
	switch {
	case gs.TrumpCandidate != nil && step < 4:
		return []game.Suit{gs.TrumpCandidate.Suit}
	case gs.TrumpCandidate != nil:
		var suits []game.Suit
		for _, su := range game.AllSuits {
			if su != gs.TrumpCandidate.Suit {
				suits = append(suits, su)
			}
		}
		return suits
	}
	return game.AllSuits[:]
}

// matchingPositions lists the all-pass bidding steps of the dealt state that fit the shape, as "step:suit". Step s is
// seat dealer+1+s with s%4 passes before it, which is BiddingPassCount there since the count restarts each round.
func matchingPositions(gs *game.GameState, s Shape) []string {
	var out []string
	last := 4
	if gs.TrumpCandidate != nil {
		last = 8
	}
	for step := 0; step < last; step++ {
		hand := gs.Players[(gs.DealerSeat+1+step)%4].Hand
		for _, su := range biddingSuits(gs, step) {
			if s.Matches(hand, gs.TrumpCandidate, su, step%4) {
				out = append(out, fmt.Sprintf("%d:%s", step, su))
			}
		}
	}
	return out
}

// reachesMatch walks the natural auction of pol and reports whether a position it reaches fits the shape. A forced
// dealer pick counts although class_table.py drops its row, which keeps the deal sets of §3.6 reproducible.
func reachesMatch(gs *game.GameState, s Shape, pol botlab.Policy) bool {
	gs.Phase = game.PhaseBidding
	game.RefreshDerivedFlags(gs)
	for step := 0; step < 8; step++ {
		seat := gs.ActivePlayerSeat
		hand := gs.Players[seat].Hand
		for _, su := range biddingSuits(gs, step) {
			if s.Matches(hand, gs.TrumpCandidate, su, gs.BiddingPassCount) {
				return true
			}
		}
		natural := pol.Decide(botlab.BuildView(gs, seat, botlab.NewMemory()))
		if natural.Type == game.ActionPickTrump || game.MustPickTrump(gs, seat) {
			return false
		}
		if gs.Rules.AllPassOutcome == game.AllPassReshuffleAndRotate && gs.BiddingRound == 2 && gs.BiddingPassCount == 3 {
			return false
		}
		next, err := game.ApplyAction(gs, natural)
		if err != nil {
			panic(err)
		}
		gs = next
	}
	return false
}

const targetedDealTries = 1_000_000

// TargetedDeal draws deals until one has a bidding position fitting the shape; a nil shape returns RandomDeal at once.
func TargetedDeal(rng *rand.Rand, v game.Variant, s *Shape) (botlab.Deal, bool) {
	if s == nil {
		return botlab.RandomDeal(rng), true
	}
	return drawDealUntil(rng, func(d botlab.Deal) bool {
		return len(matchingPositions(botlab.NewDealState(d, v), *s)) > 0
	})
}

// TargetedDealReached is TargetedDeal that also requires the natural auction of pol to reach the matching position;
// begin, when set, runs before each walk.
func TargetedDealReached(rng *rand.Rand, v game.Variant, s *Shape, pol botlab.Policy, begin func()) (botlab.Deal, bool) {
	if s == nil {
		return botlab.RandomDeal(rng), true
	}
	return drawDealUntil(rng, func(d botlab.Deal) bool {
		gs := botlab.NewDealState(d, v)
		if len(matchingPositions(gs, *s)) == 0 {
			return false
		}
		if begin != nil {
			begin()
		}
		return reachesMatch(gs, *s, pol)
	})
}

func drawDealUntil(rng *rand.Rand, accept func(botlab.Deal) bool) (botlab.Deal, bool) {
	for range targetedDealTries {
		if d := botlab.RandomDeal(rng); accept(d) {
			return d, true
		}
	}
	return botlab.Deal{}, false
}

// shapeField names the Shape field a token sets, so ParseShape can reject a second token for it.
func shapeField(tok string) string {
	switch {
	case tok == "":
		return ""
	case tok == "J" || tok == "noJ" || tok == "9" || tok == "no9" || tok == "A" || tok == "noA":
		return tok
	case strings.HasPrefix(tok, "sA") && strings.HasSuffix(tok, "+"):
		return "sideAcesMin"
	case strings.HasPrefix(tok, "sA"):
		return "sideAces"
	case strings.HasPrefix(tok, "c") && strings.HasSuffix(tok, "+"):
		return "countMin"
	case strings.HasPrefix(tok, "c"):
		return "count"
	case strings.HasPrefix(tok, "pos"):
		return "pos"
	}
	return ""
}

func shapeFilter() *Shape {
	if *shapeSpec == "" {
		return nil
	}
	s, err := ParseShape(*shapeSpec)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return &s
}

func mustTargetedDeal(rng *rand.Rand, v game.Variant, s *Shape, pol botlab.Policy, begin func()) botlab.Deal {
	var d botlab.Deal
	var ok bool
	if pol == nil {
		d, ok = TargetedDeal(rng, v, s)
	} else {
		d, ok = TargetedDealReached(rng, v, s, pol, begin)
	}
	if !ok {
		fmt.Fprintln(os.Stderr, "no deal fits the shape")
		os.Exit(1)
	}
	return d
}
