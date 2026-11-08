package botlab

import (
	"math/rand/v2"

	"github.com/emilijan/beljot/server/internal/game"
)

const (
	humanCallProbability  = 0.8
	humanMinTrumpsAny     = 4
	humanMinTrumpsWithJ9  = 3
	humanLengthBonusScore = 10
)

// HumanBidder plays, declares and surrenders like Inner and bids by a simple human rule: it calls with probability 0.8 when the trump suit holds J or 9 with three trumps or any four trumps, the candidate counted.
type HumanBidder struct {
	Inner Policy
	Rng   *rand.Rand
}

func (h *HumanBidder) Name() string { return "human[" + h.Inner.Name() + "]" }

// Counts forwards the inner policy's counters when it keeps any.
func (h *HumanBidder) Counts() Counts {
	if c, ok := h.Inner.(Counting); ok {
		return c.Counts()
	}
	return Counts{}
}

func (h *HumanBidder) Decide(v View) game.Action {
	if v.Phase != game.PhaseBidding {
		return h.Inner.Decide(v)
	}
	hand := v.Hand
	if v.TrumpCandidate != nil {
		hand = append(append([]game.Card{}, v.Hand...), *v.TrumpCandidate)
	}
	pass := game.Action{Type: game.ActionPassTrump, PlayerSeat: v.Seat}
	if v.TrumpCandidate != nil && v.BiddingRound == 1 {
		if humanQualifies(hand, v.TrumpCandidate.Suit) && h.Rng.Float64() < humanCallProbability {
			return game.Action{Type: game.ActionPickTrump, PlayerSeat: v.Seat}
		}
		return pass
	}
	var qualifying, best *game.Suit
	qualifyingScore, bestScore := -1, -1
	for _, su := range game.AllSuits {
		if v.TrumpCandidate != nil && su == v.TrumpCandidate.Suit {
			continue
		}
		s := su
		score := humanSuitScore(hand, s)
		if score > bestScore {
			best, bestScore = &s, score
		}
		if humanQualifies(hand, s) && score > qualifyingScore {
			qualifying, qualifyingScore = &s, score
		}
	}
	if qualifying != nil && h.Rng.Float64() < humanCallProbability {
		return game.Action{Type: game.ActionPickTrump, PlayerSeat: v.Seat, Suit: qualifying}
	}
	if v.MustPickTrump {
		return game.Action{Type: game.ActionPickTrump, PlayerSeat: v.Seat, Suit: best}
	}
	return pass
}

func humanSuitScore(hand []game.Card, suit game.Suit) int {
	score := 0
	for _, c := range hand {
		if c.Suit == suit {
			score += game.TrumpCardPoints[c.Rank] + humanLengthBonusScore
		}
	}
	return score
}

func humanQualifies(hand []game.Card, suit game.Suit) bool {
	count, topTrump := 0, false
	for _, c := range hand {
		if c.Suit != suit {
			continue
		}
		count++
		if c.Rank == game.RankJack || c.Rank == game.Rank9 {
			topTrump = true
		}
	}
	return count >= humanMinTrumpsAny || (topTrump && count >= humanMinTrumpsWithJ9)
}
