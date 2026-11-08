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
	gained [2]uint8
}

type memoEntry struct {
	lo, hi int32
	best   int8
}

const noCard = -1

// Solver is a double-dummy solver for one trump suit and one direction of play. Its values are team A (seats 0
// and 2) minus team B points for the rest of the hand, last-trick or capot bonus included.
type Solver struct {
	Nodes        int64
	trump        int
	seatAt       [4][4]int
	cardStrength [NCards]int
	cardPoints   [NCards]int
	memo         map[memoKey]memoEntry
	plain        bool
	race         *Race
	gained       [2]int
}

// NewSolver returns a solver for the trump suit index and the room's direction of play.
func NewSolver(trump int, clockwise bool) *Solver {
	s := &Solver{trump: trump, memo: make(map[memoKey]memoEntry)}
	for leader := range s.seatAt {
		for k := range s.seatAt[leader] {
			s.seatAt[leader][k] = game.StepSeat(leader, k, clockwise)
		}
	}
	for card := range NCards {
		s.cardStrength[card] = rankStrength(card, trump)
		s.cardPoints[card] = Points(card, trump)
	}
	return s
}

// Reset forgets every memoised position so the solver can take the next deal as if it were new.
func (s *Solver) Reset() {
	clear(s.memo)
	s.Nodes = 0
}

// SetRace makes the solver race for the match: a trick that decides it is worth ±RaceWin, and nil turns that off.
// A deciding-hand race has no mid-hand stops, so the solver plays it for points and leaves the hand's end to Choose.
func (s *Solver) SetRace(r *Race) {
	if r != nil && r.DecidingHand {
		r = nil
	}
	s.race = r
	clear(s.memo)
}

// pointBounded reports whether the claims and the point bounds apply: they assume every value is a point count.
func (s *Solver) pointBounded() bool {
	return !s.plain && s.race == nil
}

func (s *Solver) gainedKey() [2]uint8 {
	return [2]uint8{uint8(s.gained[0]), uint8(s.gained[1])}
}

func rankStrength(card, trump int) int {
	if card/ranksPerSuit == trump {
		return trumpStrength[card%ranksPerSuit]
	}
	return plainStrength[card%ranksPerSuit]
}

func (s *Solver) strength(card int) int {
	return s.cardStrength[card]
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
	legal := s.Legal(hands[seat], trick)
	live := liveCards(hands, trick)
	out := make(map[int]int)
	for m := legal; m != 0; m &= m - 1 {
		card := bits.TrailingZeros32(m)
		rep := s.representative(card, legal, live)
		if _, ok := out[rep]; !ok {
			out[rep] = s.play(hands, leader, trick, rep, capot, -inf, inf)
		}
		out[card] = out[rep]
	}
	return out
}

// representative is the strongest card the optimised solver treats as equivalent to card, card itself when none is.
func (s *Solver) representative(card int, legal, live uint32) int {
	if s.plain {
		return card
	}
	for {
		next, ok := s.equivalentAbove(card, legal, live)
		if !ok {
			return card
		}
		card = next
	}
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
	hint := noCard
	if boundary {
		if hands == [4]uint32{} {
			return 0
		}
		if s.pointBounded() {
			if value, ok := s.claim(hands, leader, capot); ok {
				return value
			}
			lo, hi := s.remainderBounds(hands, capot)
			if lo >= beta {
				return lo
			}
			if hi <= alpha {
				return hi
			}
			alpha, beta = max(alpha, lo), min(beta, hi)
		}
		key = memoKey{hands: hands, leader: int8(leader), capot: capot, gained: s.gainedKey()}
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
			hint = int(e.best)
		}
	}
	windowLow, windowHigh := alpha, beta

	seat := s.seatAt[leader][len(trick)]
	maximize := game.TeamForSeat(seat) == game.TeamA
	best, bestCard := inf, noCard
	if maximize {
		best = -inf
	}
	order, n := s.moves(hands, leader, trick, hint)
	for i, card := range order[:n] {
		val := s.playWithin(hands, leader, trick, card, capot, alpha, beta, maximize, i == 0)
		if maximize && val > best || !maximize && val < best {
			best, bestCard = val, card
		}
		if maximize {
			alpha = max(alpha, best)
		} else {
			beta = min(beta, best)
		}
		if alpha >= beta {
			break
		}
	}
	if boundary {
		s.remember(key, best, bestCard, windowLow, windowHigh)
	}
	return best
}

// playWithin values a card inside the window. The optimised solver first probes a later card with a null window
// at the bound its side must beat, and only re-searches the remaining window when the probe beats it.
func (s *Solver) playWithin(
	hands [4]uint32, leader int, trick []int, card int, capot int8, alpha, beta int, maximize, first bool,
) int {
	if s.plain || first || beta-alpha <= 1 {
		return s.play(hands, leader, trick, card, capot, alpha, beta)
	}
	if maximize {
		val := s.play(hands, leader, trick, card, capot, alpha, alpha+1)
		if val <= alpha || val >= beta {
			return val
		}
		return s.play(hands, leader, trick, card, capot, val, beta)
	}
	val := s.play(hands, leader, trick, card, capot, beta-1, beta)
	if val >= beta || val <= alpha {
		return val
	}
	return s.play(hands, leader, trick, card, capot, alpha, val)
}

func (s *Solver) remember(key memoKey, best, bestCard, windowLow, windowHigh int) {
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
	e.best = int8(bestCard)
	s.memo[key] = e
}

func (s *Solver) play(hands [4]uint32, leader int, trick []int, card int, capot int8, alpha, beta int) int {
	seat := s.seatAt[leader][len(trick)]
	hands[seat] &^= 1 << uint(card)
	if s.race != nil && s.race.announcesBelote(seat, card, hands[seat]) {
		if winner, decided := RaceDecided(*s.race, s.race.totals(s.gained, hands)); decided {
			return raceValue(winner)
		}
	}
	var played [4]int
	n := copy(played[:], trick)
	played[n] = card
	if n < 3 {
		return s.search(hands, leader, played[:n+1], capot, alpha, beta)
	}

	winnerSeat, gain, capot := s.closeTrick(played, leader, capot)
	if s.race != nil {
		return s.raceAfterTrick(hands, winnerSeat, gain, capot, alpha, beta)
	}
	if hands == [4]uint32{} {
		return gain + finalBonus(capot, 1-2*game.TeamForSeat(winnerSeat))
	}
	return gain + s.search(hands, winnerSeat, nil, capot, alpha-gain, beta-gain)
}

// raceAfterTrick values a closed trick in race mode, where values count from the root: ±RaceWin once a trick
// before the last decides the match, the points gained so far plus the hand's end when nothing does.
func (s *Solver) raceAfterTrick(hands [4]uint32, winnerSeat, gain int, capot int8, alpha, beta int) int {
	soFar := s.gained[game.TeamA] - s.gained[game.TeamB] + gain
	if hands == [4]uint32{} {
		return soFar + finalBonus(capot, 1-2*game.TeamForSeat(winnerSeat))
	}
	saved := s.gained
	team := game.TeamForSeat(winnerSeat)
	s.gained[team] += (1 - 2*team) * gain
	winner, decided := RaceDecided(*s.race, s.race.totals(s.gained, hands))
	value := raceValue(winner)
	if !decided {
		value = s.search(hands, winnerSeat, nil, capot, alpha, beta)
	}
	s.gained = saved
	return value
}

// closeTrick scores a full trick: the seat that won it, its points signed for team A and the capot flag after it.
func (s *Solver) closeTrick(played [4]int, leader int, capot int8) (int, int, int8) {
	winnerSeat := s.seatAt[leader][s.Winner(played[:])]
	points := 0
	for _, c := range played {
		points += s.cardPoints[c]
	}
	team := game.TeamForSeat(winnerSeat)
	return winnerSeat, (1 - 2*team) * points, capotAfterTrick(capot, team)
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

var (
	trumpStrengthBits = strengthBitsTable(trumpStrength)
	plainStrengthBits = strengthBitsTable(plainStrength)
	trumpRankAt       = rankAtStrength(trumpStrength)
	plainRankAt       = rankAtStrength(plainStrength)
	trumpSuitPoints   = suitPointsTable(trumpPoints)
	plainSuitPoints   = suitPointsTable(plainPoints)
)

// strengthBitsTable maps a suit's natural-rank byte to the same cards as a byte with one bit per strength.
func strengthBitsTable(strength [ranksPerSuit]int) [256]uint8 {
	var table [256]uint8
	for m := range table {
		for rank := range ranksPerSuit {
			if m&(1<<rank) != 0 {
				table[m] |= 1 << uint(strength[rank])
			}
		}
	}
	return table
}

func rankAtStrength(strength [ranksPerSuit]int) [ranksPerSuit]int {
	var ranks [ranksPerSuit]int
	for rank, k := range strength {
		ranks[k] = rank
	}
	return ranks
}

func suitPointsTable(points [ranksPerSuit]int) [256]int {
	var table [256]int
	for m := range table {
		for rank := range ranksPerSuit {
			if m&(1<<rank) != 0 {
				table[m] += points[rank]
			}
		}
	}
	return table
}

func suitByte(cards uint32, suit int) uint8 {
	return uint8(cards >> (ranksPerSuit * uint(suit)))
}

func (s *Solver) strengthBits(cards uint32, suit int) uint8 {
	if suit == s.trump {
		return trumpStrengthBits[suitByte(cards, suit)]
	}
	return plainStrengthBits[suitByte(cards, suit)]
}

func (s *Solver) cardAtStrength(suit, strength int) int {
	if suit == s.trump {
		return suit*ranksPerSuit + trumpRankAt[strength]
	}
	return suit*ranksPerSuit + plainRankAt[strength]
}

func (s *Solver) pointsIn(cards uint32) int {
	total := 0
	for suit := range len(suitList) {
		if suit == s.trump {
			total += trumpSuitPoints[suitByte(cards, suit)]
		} else {
			total += plainSuitPoints[suitByte(cards, suit)]
		}
	}
	return total
}

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
