package search

import (
	"math/bits"

	"github.com/emilijan/beljot/server/internal/game"
)

func allCards(hands [4]uint32) uint32 {
	return hands[0] | hands[1] | hands[2] | hands[3]
}

// remainderBounds brackets the value of a trick boundary: no team can take more than every card point left plus
// the largest bonus the capot flag still allows it.
func (s *Solver) remainderBounds(hands [4]uint32, capot int8) (int, int) {
	remaining := s.pointsIn(allCards(hands))
	return -remaining - largestBonus(capot, capotTeamB), remaining + largestBonus(capot, capotTeamA)
}

func largestBonus(flag, team int8) int {
	if flag == capotNone || flag == team {
		return capotBonus
	}
	return lastTrickBonus
}

// claim settles a trick boundary without search when play is forced to the end or the leader takes every trick.
func (s *Solver) claim(hands [4]uint32, leader int, capot int8) (int, bool) {
	if bits.OnesCount32(hands[leader]) == 1 {
		return s.lastTrick(hands, leader, capot), true
	}
	if !s.leaderTakesEveryTrick(hands, leader) {
		return 0, false
	}
	team := game.TeamForSeat(leader)
	sign := 1 - 2*team
	return sign*s.pointsIn(allCards(hands)) + finalBonus(capotAfterTrick(capot, team), sign), true
}

func (s *Solver) lastTrick(hands [4]uint32, leader int, capot int8) int {
	var played [4]int
	for k := range played {
		played[k] = bits.TrailingZeros32(hands[s.seatAt[leader][k]])
	}
	winnerSeat, gain, capot := s.closeTrick(played, leader, capot)
	return gain + finalBonus(capot, 1-2*game.TeamForSeat(winnerSeat))
}

// leaderTakesEveryTrick reports whether the leader, drawing trumps first, wins every remaining trick: each of its
// cards outranks every other card of its suit still held, and it holds at least as many trumps as anyone else.
func (s *Solver) leaderTakesEveryTrick(hands [4]uint32, leader int) bool {
	var others uint32
	mostTrumps := 0
	for seat, hand := range hands {
		if seat != leader {
			others |= hand
			mostTrumps = max(mostTrumps, bits.OnesCount32(hand&SuitMask(s.trump)))
		}
	}
	if bits.OnesCount32(hands[leader]&SuitMask(s.trump)) < mostTrumps {
		return false
	}
	for suit := range len(suitList) {
		mine, theirs := s.strengthBits(hands[leader], suit), s.strengthBits(others, suit)
		if mine != 0 && theirs != 0 && bits.TrailingZeros8(mine) < bits.Len8(theirs)-1 {
			return false
		}
	}
	return true
}

// moves lists the cards to try at a node: every legal card for the plain solver, otherwise one card per run of
// equivalent cards, likeliest best first.
func (s *Solver) moves(hands [4]uint32, leader int, trick []int, hint int) ([ranksPerSuit]int, int) {
	legal := s.Legal(hands[s.seatAt[leader][len(trick)]], trick)
	if s.plain || legal&(legal-1) == 0 {
		return s.order(legal)
	}
	live := liveCards(hands, trick)
	return s.orderByPromise(s.withoutEquivalents(legal, live), live, trick, hint)
}

func liveCards(hands [4]uint32, trick []int) uint32 {
	live := allCards(hands)
	for _, c := range trick {
		live |= 1 << uint(c)
	}
	return live
}

// equivalentAbove returns the next stronger live card of card's suit when it is also legal and worth the same
// points: no card still in play separates the two, so either one plays exactly like the other.
func (s *Solver) equivalentAbove(card int, legal, live uint32) (int, bool) {
	suit := card / ranksPerSuit
	above := s.strengthBits(live, suit) >> uint(s.strength(card)+1)
	if above == 0 {
		return 0, false
	}
	next := s.cardAtStrength(suit, s.strength(card)+1+bits.TrailingZeros8(above))
	return next, legal&(1<<uint(next)) != 0 && s.cardPoints[next] == s.cardPoints[card]
}

// withoutEquivalents keeps one card, the strongest, of every run of equivalent legal cards.
func (s *Solver) withoutEquivalents(legal, live uint32) uint32 {
	kept := legal
	for m := legal; m != 0; m &= m - 1 {
		if _, ok := s.equivalentAbove(bits.TrailingZeros32(m), legal, live); ok {
			kept &^= m & -m
		}
	}
	return kept
}

const (
	hintPromise       = 1000
	smearPromise      = 500
	overtakePromise   = 300
	masterLeadPromise = 100
	trumpCutCost      = ranksPerSuit
)

type trickState struct {
	live           uint32
	open           bool
	winning, led   int
	partnerWinning bool
}

func (s *Solver) stateOf(trick []int, live uint32) trickState {
	if len(trick) == 0 {
		return trickState{live: live}
	}
	winner := s.Winner(trick)
	return trickState{
		live:           live,
		open:           true,
		winning:        trick[winner],
		led:            trick[0] / ranksPerSuit,
		partnerWinning: winner == len(trick)-2,
	}
}

func (s *Solver) orderByPromise(cards, live uint32, trick []int, hint int) ([ranksPerSuit]int, int) {
	var order, promise [ranksPerSuit]int
	state := s.stateOf(trick, live)
	n := 0
	for m := cards; m != 0; m &= m - 1 {
		card := bits.TrailingZeros32(m)
		order[n], promise[n] = card, s.promise(card, state, hint)
		for j := n; j > 0 && promise[j] > promise[j-1]; j-- {
			order[j], order[j-1] = order[j-1], order[j]
			promise[j], promise[j-1] = promise[j-1], promise[j]
		}
		n++
	}
	return order, n
}

// promise scores a card for move ordering only: the memo's best first, then on lead see leadPromise, and when
// following points onto a partner's winner, the cheapest card that overtakes an opponent, or the cheapest discard.
func (s *Solver) promise(card int, state trickState, hint int) int {
	if card == hint {
		return hintPromise
	}
	if !state.open {
		return s.leadPromise(card, state.live)
	}
	partnerWinning, led := state.partnerWinning, state.led
	beats := s.beats(card, state.winning, led)
	switch {
	case partnerWinning && !beats:
		return smearPromise + s.cardPoints[card]
	case beats && !partnerWinning:
		cost := s.strength(card)
		if card/ranksPerSuit == s.trump && led != s.trump {
			cost += trumpCutCost
		}
		return overtakePromise - cost
	}
	return -s.cardPoints[card]
}

// leadPromise prefers leading a card no live card of its suit outranks, richest first, and otherwise the cheapest.
func (s *Solver) leadPromise(card int, live uint32) int {
	if s.strength(card) == bits.Len8(s.strengthBits(live, card/ranksPerSuit))-1 {
		return masterLeadPromise + s.cardPoints[card]
	}
	return -s.cardPoints[card]
}
