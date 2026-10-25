package botlab

import "github.com/emilijan/beljot/server/internal/game"

const NCards = 32

var suitList = [4]game.Suit{game.SuitSpades, game.SuitHearts, game.SuitDiamonds, game.SuitClubs}
var rankList = [8]game.Rank{game.Rank7, game.Rank8, game.Rank9, game.RankTen, game.RankJack, game.RankQueen, game.RankKing, game.RankAce}

var trumpStr = [8]int{0, 1, 6, 4, 7, 2, 3, 5}
var plainStr = [8]int{0, 1, 2, 6, 3, 4, 5, 7}
var trumpPts = [8]int{0, 0, 14, 10, 20, 3, 4, 11}
var plainPts = [8]int{0, 0, 0, 10, 2, 3, 4, 11}

func SuitIdx(s game.Suit) int {
	switch s {
	case game.SuitSpades:
		return 0
	case game.SuitHearts:
		return 1
	case game.SuitDiamonds:
		return 2
	case game.SuitClubs:
		return 3
	}
	return -1
}

func CardIdx(c game.Card) int { return SuitIdx(c.Suit)*8 + game.NaturalRankOrder[c.Rank] }
func IdxCard(i int) game.Card  { return game.Card{Suit: suitList[i/8], Rank: rankList[i%8]} }
func SuitMask(s int) uint32    { return uint32(0xFF) << (8 * uint(s)) }

func MaskOf(cards []game.Card) uint32 {
	var m uint32
	for _, c := range cards {
		m |= 1 << uint(CardIdx(c))
	}
	return m
}

func CardsOf(m uint32) []game.Card {
	var out []game.Card
	for i := 0; i < NCards; i++ {
		if m&(1<<uint(i)) != 0 {
			out = append(out, IdxCard(i))
		}
	}
	return out
}

func popcount(m uint32) int {
	n := 0
	for m != 0 {
		m &= m - 1
		n++
	}
	return n
}

type ddKey struct {
	h     [4]uint32
	lead  int8
	capot int8
}

type ddEntry struct {
	lo, hi int32
}

// DD is a double-dummy solver for one hand with a fixed trump; values are team A minus team B
// points for the remainder including the last-trick or capot bonus.
type DD struct {
	Trump  int
	memo   map[ddKey]ddEntry
	Nodes  int64
	NoMemo bool
}

func NewDD(trump int) *DD { return &DD{Trump: trump, memo: make(map[ddKey]ddEntry, 1<<14)} }

func (d *DD) Reset() { d.memo = make(map[ddKey]ddEntry, 1<<14); d.Nodes = 0 }

func (d *DD) strength(c int) int {
	if c/8 == d.Trump {
		return trumpStr[c%8]
	}
	return plainStr[c%8]
}

func (d *DD) Pts(c int) int {
	if c/8 == d.Trump {
		return trumpPts[c%8]
	}
	return plainPts[c%8]
}

func (d *DD) beats(cand, win, led int) bool {
	ct, wt := cand/8 == d.Trump, win/8 == d.Trump
	switch {
	case ct && !wt:
		return true
	case ct && wt:
		return trumpStr[cand%8] > trumpStr[win%8]
	case wt:
		return false
	default:
		return cand/8 == led && plainStr[cand%8] > plainStr[win%8]
	}
}

// Winner returns the index into trick of the winning card.
func (d *DD) Winner(trick []int) int {
	led := trick[0] / 8
	best := 0
	for i := 1; i < len(trick); i++ {
		if d.beats(trick[i], trick[best], led) {
			best = i
		}
	}
	return best
}

// Legal mirrors game.legalCards for a hand mask and the trick so far.
func (d *DD) Legal(hand uint32, trick []int) uint32 {
	if len(trick) == 0 {
		return hand
	}
	led := trick[0] / 8
	trumpIn, bestTrump, bestLed := false, -1, -1
	for _, c := range trick {
		if c/8 == d.Trump {
			trumpIn = true
			if s := trumpStr[c%8]; s > bestTrump {
				bestTrump = s
			}
		}
		if c/8 == led {
			if s := d.strength(c); s > bestLed {
				bestLed = s
			}
		}
	}
	suitCards := hand & SuitMask(led)
	if suitCards != 0 {
		if led == d.Trump || !trumpIn {
			var higher uint32
			for m := suitCards; m != 0; m &= m - 1 {
				c := trailing(m)
				if d.strength(c) > bestLed {
					higher |= 1 << uint(c)
				}
			}
			if higher != 0 {
				return higher
			}
		}
		return suitCards
	}
	trumps := hand & SuitMask(d.Trump)
	if trumps != 0 {
		if trumpIn {
			var over uint32
			for m := trumps; m != 0; m &= m - 1 {
				c := trailing(m)
				if trumpStr[c%8] > bestTrump {
					over |= 1 << uint(c)
				}
			}
			if over != 0 {
				return over
			}
		}
		return trumps
	}
	return hand
}

func trailing(m uint32) int {
	n := 0
	for m&1 == 0 {
		m >>= 1
		n++
	}
	return n
}

const inf = 1 << 20

// Value solves the position: hands are the remaining cards, leader led the current
// trick (trick holds the cards played so far), capot is the capot flag.
func (d *DD) Value(hands [4]uint32, leader int, trick []int, capot int8) int {
	return d.search(hands, leader, trick, capot, -inf, inf)
}

func (d *DD) search(hands [4]uint32, leader int, trick []int, capot int8, alpha, beta int) int {
	d.Nodes++
	var key ddKey
	boundary := len(trick) == 0
	if boundary {
		if hands[0]|hands[1]|hands[2]|hands[3] == 0 {
			return 0
		}
		key = ddKey{hands, int8(leader), capot}
		if e, ok := d.memo[key]; ok && !d.NoMemo {
			if int(e.lo) >= beta {
				return int(e.lo)
			}
			if int(e.hi) <= alpha {
				return int(e.hi)
			}
			if e.lo == e.hi {
				return int(e.lo)
			}
			if int(e.lo) > alpha {
				alpha = int(e.lo)
			}
			if int(e.hi) < beta {
				beta = int(e.hi)
			}
		}
	}
	a0, b0 := alpha, beta
	seat := (leader + len(trick)) % 4
	maximize := seat%2 == 0
	legal := d.Legal(hands[seat], trick)
	best := -inf
	if !maximize {
		best = inf
	}
	order := d.order(legal, maximize)
	var next [5]int
	copy(next[:], trick)
	for _, c := range order {
		next[len(trick)] = c
		nh := hands
		nh[seat] &^= 1 << uint(c)
		var val int
		if len(trick) == 3 {
			t := next[:4]
			w := d.Winner(t)
			wseat := (leader + w) % 4
			pts := d.Pts(t[0]) + d.Pts(t[1]) + d.Pts(t[2]) + d.Pts(t[3])
			sign := 1
			if wseat%2 == 1 {
				sign = -1
			}
			nc := capot
			switch {
			case nc == 0:
				nc = int8(1 + wseat%2)
			case nc == 1 && wseat%2 == 1, nc == 2 && wseat%2 == 0:
				nc = 3
			}
			gain := sign * pts
			if nh[0]|nh[1]|nh[2]|nh[3] == 0 {
				switch nc {
				case 1:
					gain += 100
				case 2:
					gain -= 100
				default:
					gain += sign * 10
				}
				val = gain
			} else {
				val = gain + d.search(nh, wseat, nil, nc, alpha-gain, beta-gain)
			}
		} else {
			val = d.search(nh, leader, next[:len(trick)+1], capot, alpha, beta)
		}
		if maximize {
			if val > best {
				best = val
			}
			if best > alpha {
				alpha = best
			}
		} else {
			if val < best {
				best = val
			}
			if best < beta {
				beta = best
			}
		}
		if alpha >= beta {
			break
		}
	}
	if boundary && !d.NoMemo {
		e := ddEntry{lo: -inf, hi: inf}
		if old, ok := d.memo[key]; ok {
			e = old
		}
		switch {
		case best <= a0:
			if int32(best) < e.hi {
				e.hi = int32(best)
			}
		case best >= b0:
			if int32(best) > e.lo {
				e.lo = int32(best)
			}
		default:
			e.lo, e.hi = int32(best), int32(best)
		}
		d.memo[key] = e
	}
	return best
}

func (d *DD) order(legal uint32, maximize bool) []int {
	out := make([]int, 0, 8)
	for m := legal; m != 0; m &= m - 1 {
		out = append(out, trailing(m))
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && d.strength(out[j]) > d.strength(out[j-1]); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// RootValues returns the exact value of every legal card for the seat to act.
func (d *DD) RootValues(hands [4]uint32, leader int, trick []int, capot int8) map[int]int {
	seat := (leader + len(trick)) % 4
	legal := d.Legal(hands[seat], trick)
	out := make(map[int]int)
	var next [5]int
	copy(next[:], trick)
	for m := legal; m != 0; m &= m - 1 {
		c := trailing(m)
		next[len(trick)] = c
		nh := hands
		nh[seat] &^= 1 << uint(c)
		if len(trick) == 3 {
			t := next[:4]
			w := d.Winner(t)
			wseat := (leader + w) % 4
			pts := d.Pts(t[0]) + d.Pts(t[1]) + d.Pts(t[2]) + d.Pts(t[3])
			sign := 1
			if wseat%2 == 1 {
				sign = -1
			}
			nc := capot
			switch {
			case nc == 0:
				nc = int8(1 + wseat%2)
			case nc == 1 && wseat%2 == 1, nc == 2 && wseat%2 == 0:
				nc = 3
			}
			gain := sign * pts
			if nh[0]|nh[1]|nh[2]|nh[3] == 0 {
				switch nc {
				case 1:
					gain += 100
				case 2:
					gain -= 100
				default:
					gain += sign * 10
				}
				out[c] = gain
			} else {
				out[c] = gain + d.Value(nh, wseat, nil, nc)
			}
		} else {
			out[c] = d.Value(nh, leader, next[:len(trick)+1], capot)
		}
	}
	return out
}

// CapotFlag derives the capot flag from tricks won so far.
func CapotFlag(tricksWon [2]int) int8 {
	switch {
	case tricksWon[0] == 0 && tricksWon[1] == 0:
		return 0
	case tricksWon[1] == 0:
		return 1
	case tricksWon[0] == 0:
		return 2
	}
	return 3
}

// StateDD builds solver inputs from a full engine state at a play decision.
func StateDD(gs *game.GameState) (*DD, [4]uint32, int, []int, int8) {
	d := NewDD(SuitIdx(*gs.TrumpSuit))
	var hands [4]uint32
	for i := range gs.Players {
		hands[i] = MaskOf(gs.Players[i].Hand)
	}
	trick := make([]int, 0, 4)
	leader := gs.ActivePlayerSeat
	if len(gs.CurrentTrick) > 0 {
		leader = gs.CurrentTrick[0].PlayerSeat
		for _, tc := range gs.CurrentTrick {
			trick = append(trick, CardIdx(tc.Card))
		}
	}
	return d, hands, leader, trick, CapotFlag(gs.TricksWon)
}
