package botlab

import (
	"math"
	"math/rand/v2"

	"github.com/emilijan/beljot/server/internal/game"
)

// Constraints describe what a seat knows about the other hands.
type Constraints struct {
	Seat   int
	Own    uint32
	Unseen uint32
	Known  [4]uint32
	Need   [4]int
	Voids  [4]uint32
}

// ConstraintsFrom derives the hard constraints a bot is entitled to from its view.
func ConstraintsFrom(v View) Constraints {
	c := Constraints{Seat: v.Seat, Own: MaskOf(v.Hand)}
	gone := MaskOf(v.PlayedCards)
	for _, tc := range v.CurrentTrick {
		gone |= 1 << uint(CardIdx(tc.Card))
	}
	all := uint32(0xFFFFFFFF)
	unseen := all &^ c.Own &^ gone
	played := [4]bool{}
	for _, tc := range v.CurrentTrick {
		played[tc.PlayerSeat] = true
	}
	for s := 0; s < 4; s++ {
		if s == v.Seat {
			continue
		}
		var k uint32
		for _, kc := range v.KnownCards[s] {
			b := uint32(1) << uint(CardIdx(kc))
			if gone&b == 0 {
				k |= b
			}
		}
		c.Known[s] = k
		unseen &^= k
		n := len(v.Hand)
		if played[s] {
			n--
		}
		c.Need[s] = n - popcount(k)
		for suit := 0; suit < 4; suit++ {
			if v.KnownVoids[s][suit] {
				c.Voids[s] |= SuitMask(suit)
			}
		}
		if v.F.On("declInfer") {
			c.Voids[s] |= runNeighbours(v.KnownCards[s])
		}
	}
	c.Unseen = unseen
	return c
}

// runNeighbours returns the cards provably absent from a hand that revealed these
// declaration cards: a declared run is maximal, so the ranks just outside it are missing.
func runNeighbours(revealed []game.Card) uint32 {
	var out uint32
	for suit := 0; suit < 4; suit++ {
		var present [8]bool
		any := false
		for _, c := range revealed {
			if SuitIdx(c.Suit) == suit {
				present[game.NaturalRankOrder[c.Rank]] = true
				any = true
			}
		}
		if !any {
			continue
		}
		for i := 0; i < 8; {
			if !present[i] {
				i++
				continue
			}
			j := i
			for j+1 < 8 && present[j+1] {
				j++
			}
			if j-i+1 >= 3 {
				if i > 0 {
					out |= 1 << uint(suit*8+i-1)
				}
				if j < 7 {
					out |= 1 << uint(suit*8+j+1)
				}
			}
			i = j + 1
		}
	}
	return out
}

// bidWeight scores how consistent a sampled deal is with the observed bidding, assuming
// the other players bid like a reasonable player. Inconsistent seats are down-weighted.
func bidWeight(v View, hands [4]uint32, trump int) float64 {
	w := 1.0
	for s := 0; s < 4; s++ {
		if s == v.Seat {
			continue
		}
		h := hands[s]
		if v.TrumpCallerSeat != nil && *v.TrumpCallerSeat == s && !v.ForcedCall {
			tc := popcount(h & SuitMask(trump))
			hasJ := h&(1<<uint(trump*8+4)) != 0
			has9 := h&(1<<uint(trump*8+2)) != 0
			if tc <= 2 && !(hasJ && has9) {
				w *= 0.2
			}
			continue
		}
		var refused []int
		if v.Passed[s].Candidate {
			refused = append(refused, trump)
		}
		if v.Passed[s].FreeSuits {
			for x := 0; x < 4; x++ {
				refused = append(refused, x)
			}
		}
		for _, x := range refused {
			cnt := popcount(h & SuitMask(x))
			hasJ := h&(1<<uint(x*8+4)) != 0
			has9 := h&(1<<uint(x*8+2)) != 0
			if (hasJ && has9) || cnt >= 4 || (hasJ && cnt >= 3) {
				w *= 0.2
				break
			}
		}
	}
	return w
}

// Sample draws one deal of the unseen cards consistent with the constraints.
func Sample(rng *rand.Rand, c Constraints) ([4]uint32, bool) {
	cards := make([]int, 0, 24)
	for m := c.Unseen; m != 0; m &= m - 1 {
		cards = append(cards, trailing(m))
	}
	var hands [4]uint32
	for attempt := 0; attempt < 200; attempt++ {
		rng.Shuffle(len(cards), func(i, j int) { cards[i], cards[j] = cards[j], cards[i] })
		hands = [4]uint32{}
		hands[c.Seat] = c.Own
		ok := true
		idx := 0
		for s := 0; s < 4 && ok; s++ {
			if s == c.Seat {
				continue
			}
			hands[s] = c.Known[s]
			for k := 0; k < c.Need[s]; k++ {
				if idx >= len(cards) {
					ok = false
					break
				}
				card := cards[idx]
				idx++
				if c.Voids[s]&(1<<uint(card)) != 0 {
					ok = false
					break
				}
				hands[s] |= 1 << uint(card)
			}
		}
		if ok && idx == len(cards) {
			return hands, true
		}
	}
	for attempt := 0; attempt < 200; attempt++ {
		rng.Shuffle(len(cards), func(i, j int) { cards[i], cards[j] = cards[j], cards[i] })
		hands = [4]uint32{}
		hands[c.Seat] = c.Own
		need := c.Need
		for s := 0; s < 4; s++ {
			if s != c.Seat {
				hands[s] = c.Known[s]
			}
		}
		ok := true
		for _, card := range cards {
			opts := make([]int, 0, 3)
			for s := 0; s < 4; s++ {
				if s == c.Seat || need[s] <= 0 || c.Voids[s]&(1<<uint(card)) != 0 {
					continue
				}
				opts = append(opts, s)
			}
			if len(opts) == 0 {
				ok = false
				break
			}
			s := opts[rng.IntN(len(opts))]
			hands[s] |= 1 << uint(card)
			need[s]--
		}
		if ok {
			return hands, true
		}
	}
	return hands, false
}

// PIMC plays cards by perfect-information Monte Carlo over sampled consistent deals.
type PIMC struct {
	Samples   int
	FromTrick int
	Schedule  []int
	Rng      *rand.Rand
	Fallback Policy
	Label    string
}

func (p *PIMC) Name() string { return p.Label }

func (p *PIMC) Decide(v View) game.Action {
	if v.Phase != game.PhasePlaying || v.PendingBelot || v.AwaitingDeclaration || len(v.LegalCards) <= 1 || v.TrumpSuit == nil {
		return p.Fallback.Decide(v)
	}
	if trickNo := 9 - len(v.Hand); trickNo < p.FromTrick {
		return p.Fallback.Decide(v)
	}
	if lb, ok := p.Fallback.(LabBot); ok {
		v.F = lb.Flags
	}
	c := p.Choose(v)
	return game.Action{Type: game.ActionPlayCard, PlayerSeat: v.Seat, Card: &c}
}

// Choose returns the legal card with the best average double-dummy value.
func (p *PIMC) Choose(v View) game.Card {
	cons := ConstraintsFrom(v)
	trump := SuitIdx(*v.TrumpSuit)
	leader := v.Seat
	trick := make([]int, 0, 4)
	if len(v.CurrentTrick) > 0 {
		leader = v.CurrentTrick[0].PlayerSeat
		for _, tc := range v.CurrentTrick {
			trick = append(trick, CardIdx(tc.Card))
		}
	}
	sign := 1
	if v.Seat%2 == 1 {
		sign = -1
	}
	capot := CapotFlag(v.TricksWon)
	totals := map[int]float64{}
	squares := map[int]float64{}
	n := 0.0
	remPts := 0
	for m := cons.Unseen | cons.Own; m != 0; m &= m - 1 {
		c := trailing(m)
		if c/8 == trump {
			remPts += trumpPts[c%8]
		} else {
			remPts += plainPts[c%8]
		}
	}
	for s := 0; s < 4; s++ {
		for m := cons.Known[s]; m != 0; m &= m - 1 {
			c := trailing(m)
			if c/8 == trump {
				remPts += trumpPts[c%8]
			} else {
				remPts += plainPts[c%8]
			}
		}
	}
	for _, tc := range v.CurrentTrick {
		c := CardIdx(tc.Card)
		if c/8 == trump {
			remPts += trumpPts[c%8]
		} else {
			remPts += plainPts[c%8]
		}
	}
	samples := p.Samples
	if len(p.Schedule) > 0 {
		idx := 9 - len(v.Hand) - 1
		if idx >= len(p.Schedule) {
			idx = len(p.Schedule) - 1
		}
		if idx < 0 {
			idx = 0
		}
		samples = p.Schedule[idx]
	}
	for i := 0; i < samples; i++ {
		hands, ok := Sample(p.Rng, cons)
		if !ok {
			break
		}
		w := 1.0
		if v.F.On("bidInfer") {
			w = bidWeight(v, hands, trump)
		}
		d := NewDD(trump)
		vals := d.RootValues(hands, leader, trick, capot)
		for card, val := range vals {
			x := float64(sign * val)
			if v.F.On("contract") {
				x = float64(sign) * contractAward(v, remPts, val)
			}
			totals[card] += w * x
			squares[card] += w * x * x
		}
		n += w
	}
	if n == 0 {
		return *p.Fallback.Decide(v).Card
	}
	lambda := v.F.riskLambda()
	best := -1
	bestVal := -1e18
	for _, lc := range v.LegalCards {
		idx := CardIdx(lc)
		val, ok := totals[idx]
		if !ok {
			continue
		}
		score := val / n
		if lambda > 0 && n > 1 {
			mean := val / n
			variance := squares[idx]/n - mean*mean
			if variance < 0 {
				variance = 0
			}
			score = mean - lambda*math.Sqrt(variance/n)
		}
		if score > bestVal {
			bestVal = score
			best = idx
		}
	}
	if best < 0 {
		return v.LegalCards[0]
	}
	return IdxCard(best)
}

// OpponentRuffProbability estimates, per suit, the chance that at least one opponent is
// void in that suit while still holding a trump, by sampling consistent deals.
func OpponentRuffProbability(v View, trump game.Suit, samples int) [4]float64 {
	cons := ConstraintsFrom(v)
	rng := rand.New(rand.NewPCG(uint64(cons.Own), uint64(cons.Unseen)))
	var hits [4]int
	n := 0
	tm := SuitMask(SuitIdx(trump))
	for i := 0; i < samples; i++ {
		hands, ok := Sample(rng, cons)
		if !ok {
			break
		}
		n++
		for s := 0; s < 4; s++ {
			for _, opp := range []int{(v.Seat + 1) % 4, (v.Seat + 3) % 4} {
				if hands[opp]&SuitMask(s) == 0 && hands[opp]&tm != 0 {
					hits[s]++
					break
				}
			}
		}
	}
	var out [4]float64
	if n == 0 {
		return out
	}
	for s := 0; s < 4; s++ {
		out[s] = float64(hits[s]) / float64(n)
	}
	return out
}

// contractAward converts a remainder differential (team A minus B) into the banked award
// differential under the "taker must finish strictly ahead" rule. Capot is approximated.
func contractAward(v View, remPts int, remDiff int) float64 {
	bonus := 10
	if remDiff >= remPts+100 || -remDiff >= remPts+100 {
		bonus = 100
	}
	remA := float64(remPts+bonus+remDiff) / 2
	remB := float64(remPts+bonus-remDiff) / 2
	totA := float64(v.HandPoints[0]+v.DeclPoints[0]) + remA
	totB := float64(v.HandPoints[1]+v.DeclPoints[1]) + remB
	if bonus == 100 && v.TricksWon[1] == 0 && remDiff > 0 {
		return totA + totB
	}
	if bonus == 100 && v.TricksWon[0] == 0 && remDiff < 0 {
		return -(totA + totB)
	}
	if v.TrumpCallerSeat == nil {
		return totA - totB
	}
	caller := *v.TrumpCallerSeat % 2
	if caller == 0 && totA <= totB {
		return -(totA + totB)
	}
	if caller == 1 && totB <= totA {
		return totA + totB
	}
	return totA - totB
}
