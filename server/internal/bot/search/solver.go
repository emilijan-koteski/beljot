package search

import (
	"math/bits"

	"github.com/emilijan/beljot/server/internal/game"
)

const inf = 1 << 20

const (
	lastTrickBonus = 10
	capotBonus     = 100
)

const (
	capotNone  int8 = 0
	capotTeamA int8 = 1
	capotTeamB int8 = 2
	capotMixed int8 = 3
)

type memoKey struct {
	hands  [4]uint32
	leader int8
	capot  int8
}

type memoEntry struct {
	lo, hi int32
}

// Solver is a double-dummy solver for one trump suit and one direction of play. Its values are team A (seats 0
// and 2) minus team B points for the rest of the hand, last-trick or capot bonus included.
type Solver struct {
	Nodes  int64
	trump  int
	seatAt [4][4]int
	memo   map[memoKey]memoEntry
}

// NewSolver returns a solver for the trump suit index and the room's direction of play.
func NewSolver(trump int, clockwise bool) *Solver {
	s := &Solver{trump: trump, memo: make(map[memoKey]memoEntry, 1<<14)}
	for leader := range s.seatAt {
		for k := range s.seatAt[leader] {
			s.seatAt[leader][k] = game.StepSeat(leader, k, clockwise)
		}
	}
	return s
}

func (s *Solver) strength(card int) int {
	if card/ranksPerSuit == s.trump {
		return trumpStrength[card%ranksPerSuit]
	}
	return plainStrength[card%ranksPerSuit]
}

func (s *Solver) beats(candidate, winning, led int) bool {
	candidateTrump := candidate/ranksPerSuit == s.trump
	winningTrump := winning/ranksPerSuit == s.trump
	switch {
	case candidateTrump && !winningTrump:
		return true
	case winningTrump && !candidateTrump:
		return false
	case candidateTrump:
		return s.strength(candidate) > s.strength(winning)
	}
	return candidate/ranksPerSuit == led && s.strength(candidate) > s.strength(winning)
}

// Winner returns the index into trick of the winning card.
func (s *Solver) Winner(trick []int) int {
	led := trick[0] / ranksPerSuit
	best := 0
	for i := 1; i < len(trick); i++ {
		if s.beats(trick[i], trick[best], led) {
			best = i
		}
	}
	return best
}

func (s *Solver) strongerThan(cards uint32, threshold int) uint32 {
	var out uint32
	for m := cards; m != 0; m &= m - 1 {
		card := bits.TrailingZeros32(m)
		if s.strength(card) > threshold {
			out |= 1 << uint(card)
		}
	}
	return out
}

// Legal mirrors game.LegalCards for a hand mask and the cards already in the trick.
func (s *Solver) Legal(hand uint32, trick []int) uint32 {
	if len(trick) == 0 {
		return hand
	}
	led := trick[0] / ranksPerSuit
	trumpCut, bestTrump, bestLed := false, -1, -1
	for _, c := range trick {
		if c/ranksPerSuit == s.trump {
			trumpCut = true
			bestTrump = max(bestTrump, s.strength(c))
		}
		if c/ranksPerSuit == led {
			bestLed = max(bestLed, s.strength(c))
		}
	}

	if suitCards := hand & SuitMask(led); suitCards != 0 {
		if led == s.trump || !trumpCut {
			if higher := s.strongerThan(suitCards, bestLed); higher != 0 {
				return higher
			}
		}
		return suitCards
	}
	if trumps := hand & SuitMask(s.trump); trumps != 0 {
		if trumpCut {
			if over := s.strongerThan(trumps, bestTrump); over != 0 {
				return over
			}
		}
		return trumps
	}
	return hand
}

// Value solves the position: hands are the cards still held, leader led the open trick, trick holds the cards
// already played to it and capot is the capot flag.
func (s *Solver) Value(hands [4]uint32, leader int, trick []int, capot int8) int {
	return s.search(hands, leader, trick, capot, -inf, inf)
}

// RootValues returns the exact value of every legal card of the seat to act.
func (s *Solver) RootValues(hands [4]uint32, leader int, trick []int, capot int8) map[int]int {
	seat := s.seatAt[leader][len(trick)]
	out := make(map[int]int)
	for m := s.Legal(hands[seat], trick); m != 0; m &= m - 1 {
		card := bits.TrailingZeros32(m)
		out[card] = s.play(hands, leader, trick, card, capot, -inf, inf)
	}
	return out
}

// CapotFlag derives the capot flag from the tricks each team has won so far.
func CapotFlag(tricksWon [2]int) int8 {
	switch {
	case tricksWon[0] == 0 && tricksWon[1] == 0:
		return capotNone
	case tricksWon[1] == 0:
		return capotTeamA
	case tricksWon[0] == 0:
		return capotTeamB
	}
	return capotMixed
}

func (s *Solver) search(hands [4]uint32, leader int, trick []int, capot int8, alpha, beta int) int {
	s.Nodes++
	boundary := len(trick) == 0
	var key memoKey
	if boundary {
		if hands == [4]uint32{} {
			return 0
		}
		key = memoKey{hands: hands, leader: int8(leader), capot: capot}
		if e, ok := s.memo[key]; ok {
			if int(e.lo) >= beta {
				return int(e.lo)
			}
			if int(e.hi) <= alpha {
				return int(e.hi)
			}
			if e.lo == e.hi {
				return int(e.lo)
			}
			alpha = max(alpha, int(e.lo))
			beta = min(beta, int(e.hi))
		}
	}
	windowLow, windowHigh := alpha, beta

	seat := s.seatAt[leader][len(trick)]
	maximize := game.TeamForSeat(seat) == game.TeamA
	best := inf
	if maximize {
		best = -inf
	}
	order, n := s.order(s.Legal(hands[seat], trick))
	for _, card := range order[:n] {
		val := s.play(hands, leader, trick, card, capot, alpha, beta)
		if maximize {
			best = max(best, val)
			alpha = max(alpha, best)
		} else {
			best = min(best, val)
			beta = min(beta, best)
		}
		if alpha >= beta {
			break
		}
	}
	if boundary {
		s.remember(key, best, windowLow, windowHigh)
	}
	return best
}

func (s *Solver) remember(key memoKey, best, windowLow, windowHigh int) {
	e, ok := s.memo[key]
	if !ok {
		e = memoEntry{lo: -inf, hi: inf}
	}
	switch {
	case best <= windowLow:
		e.hi = min(e.hi, int32(best))
	case best >= windowHigh:
		e.lo = max(e.lo, int32(best))
	default:
		e = memoEntry{lo: int32(best), hi: int32(best)}
	}
	s.memo[key] = e
}

func (s *Solver) play(hands [4]uint32, leader int, trick []int, card int, capot int8, alpha, beta int) int {
	seat := s.seatAt[leader][len(trick)]
	hands[seat] &^= 1 << uint(card)
	var played [4]int
	n := copy(played[:], trick)
	played[n] = card
	if n < 3 {
		return s.search(hands, leader, played[:n+1], capot, alpha, beta)
	}

	winnerSeat := s.seatAt[leader][s.Winner(played[:])]
	points := 0
	for _, c := range played {
		points += Points(c, s.trump)
	}
	team := game.TeamForSeat(winnerSeat)
	sign := 1 - 2*team
	gain := sign * points
	capot = capotAfterTrick(capot, team)
	if hands == [4]uint32{} {
		return gain + finalBonus(capot, sign)
	}
	return gain + s.search(hands, winnerSeat, nil, capot, alpha-gain, beta-gain)
}

func capotAfterTrick(flag int8, winnerTeam int) int8 {
	switch {
	case flag == capotNone:
		return capotTeamA + int8(winnerTeam)
	case flag == capotTeamA && winnerTeam == game.TeamB, flag == capotTeamB && winnerTeam == game.TeamA:
		return capotMixed
	}
	return flag
}

func finalBonus(flag int8, lastTrickSign int) int {
	switch flag {
	case capotTeamA:
		return capotBonus
	case capotTeamB:
		return -capotBonus
	}
	return lastTrickSign * lastTrickBonus
}

func (s *Solver) order(legal uint32) ([ranksPerSuit]int, int) {
	var cards [ranksPerSuit]int
	n := 0
	for m := legal; m != 0; m &= m - 1 {
		cards[n] = bits.TrailingZeros32(m)
		n++
	}
	for i := 1; i < n; i++ {
		for j := i; j > 0 && s.strength(cards[j]) > s.strength(cards[j-1]); j-- {
			cards[j], cards[j-1] = cards[j-1], cards[j]
		}
	}
	return cards, n
}
