package bot

import (
	"slices"

	"github.com/emilijan/beljot/server/internal/game"
)

// Memory tracks the cards played this hand, the suit voids those plays revealed, and the cards
// publicly declared or located. The match layer owns one per bot match because GameState keeps no
// trick history.
type Memory struct {
	handNumber int
	played     []game.Card
	voids      [4][4]bool
	declared   [4][]game.Card
	located    [4][]game.Card
	bounds     [4][4]int8
	bids       []BidRecord
}

// BidRecord is one auction action: who acted, in which round, after how many passes, and the called suit unless it passed.
type BidRecord struct {
	Seat         int
	Round        int
	Passed       bool
	Suit         *game.Suit
	PassesBefore int
}

// NewMemory returns a Memory primed for the first hand.
func NewMemory() *Memory {
	return &Memory{handNumber: 1, bounds: noRankBounds()}
}

func noRankBounds() [4][4]int8 {
	var b [4][4]int8
	for seat := range b {
		for suit := range b[seat] {
			b[seat][suit] = -1
		}
	}
	return b
}

// SyncHand resets the per-hand sets when the hand number advances (or, on a
// reshuffle, stays within the same hand number — nothing was played, so the
// sets are already empty). Idempotent for the current hand.
func (m *Memory) SyncHand(handNumber int) {
	if handNumber != m.handNumber {
		m.handNumber = handNumber
		m.played = nil
		m.voids = [4][4]bool{}
		m.declared = [4][]game.Card{}
		m.located = [4][]game.Card{}
		m.bounds = noRankBounds()
		m.bids = nil
	}
}

// ObservePlay records a resolved play, marks the seat void in the led suit when it did not follow, and records the rank
// bounds the follow duties prove. A non-trump discard also proves a trump void; leadSuit is nil when this card led,
// trump nil only when unknown, and trick is the trick as it stood before the card.
func (m *Memory) ObservePlay(seat int, card game.Card, leadSuit, trump *game.Suit, trick []game.TrickCard) {
	m.played = append(m.played, card)
	if leadSuit == nil || trump == nil {
		if leadSuit != nil && card.Suit != *leadSuit {
			m.markVoid(seat, *leadSuit)
		}
		return
	}
	m.observeBounds(seat, card, *leadSuit, *trump, trick)
	if card.Suit == *leadSuit {
		return
	}
	m.markVoid(seat, *leadSuit)
	if card.Suit != *trump {
		m.markVoid(seat, *trump)
	}
}

func (m *Memory) observeBounds(seat int, card game.Card, lead, trump game.Suit, trick []game.TrickCard) {
	if card.Suit == lead {
		if best, ok := highestLedStrength(trick, lead, trump); ok && ledSuitCanStillWin(trick, lead, trump) &&
			strengthOf(card, trump) < best {
			m.lowerBound(seat, lead, best)
		}
		return
	}
	if card.Suit == trump {
		if best, ok := highestTrumpStrength(trick, trump); ok && strengthOf(card, trump) < best {
			m.lowerBound(seat, trump, best)
		}
	}
}

func strengthOf(c game.Card, trump game.Suit) int {
	if c.Suit == trump {
		return game.TrumpRankOrder[c.Rank]
	}
	return game.NonTrumpRankOrder[c.Rank]
}

func highestLedStrength(trick []game.TrickCard, lead, trump game.Suit) (int, bool) {
	best, found := -1, false
	for _, tc := range trick {
		if tc.Card.Suit == lead {
			best, found = max(best, strengthOf(tc.Card, trump)), true
		}
	}
	return best, found
}

func highestTrumpStrength(trick []game.TrickCard, trump game.Suit) (int, bool) {
	return highestLedStrength(trick, trump, trump)
}

func ledSuitCanStillWin(trick []game.TrickCard, lead, trump game.Suit) bool {
	if lead == trump {
		return true
	}
	_, trumped := highestTrumpStrength(trick, trump)
	return !trumped
}

func (m *Memory) lowerBound(seat int, suit game.Suit, bound int) {
	idx := SuitIndex(suit)
	if seat < 0 || seat >= 4 || idx < 0 {
		return
	}
	if cur := int(m.bounds[seat][idx]); cur < 0 || bound < cur {
		m.bounds[seat][idx] = int8(bound)
	}
}

// RankBounds returns, per seat and suit, the strength index of the highest card the seat can still hold, -1 for no bound.
func (m *Memory) RankBounds() [4][4]int {
	out := NoRankBounds()
	for seat := range out {
		for suit := range out[seat] {
			out[seat][suit] = int(m.bounds[seat][suit])
		}
	}
	return out
}

// NoRankBounds is the View.RankBounds value that bounds nothing; the zero value would forbid every card above a 7.
func NoRankBounds() [4][4]int {
	var out [4][4]int
	for seat := range out {
		for suit := range out[seat] {
			out[seat][suit] = -1
		}
	}
	return out
}

func (m *Memory) markVoid(seat int, suit game.Suit) {
	idx := SuitIndex(suit)
	if seat < 0 || seat >= 4 || idx < 0 {
		return
	}
	m.voids[seat][idx] = true
}

// ObserveLocatedCard records a card the table has publicly seen go to a seat's hand, such as the
// trump candidate or the Belote pair. Out-of-range seats and repeats are ignored.
func (m *Memory) ObserveLocatedCard(seat int, c game.Card) {
	if seat < 0 || seat >= 4 || slices.Contains(m.located[seat], c) {
		return
	}
	m.located[seat] = append(m.located[seat], c)
}

// ObserveDeclarations records each seat's revealed declaration cards once, deduplicating overlapping melds.
// Call it only after declarations resolve, so only the winning team's public reveal is stored.
func (m *Memory) ObserveDeclarations(players [4]game.PlayerState) {
	for seat := range players {
		var cs []game.Card
		for _, d := range players[seat].Declarations {
			for _, c := range d.Cards {
				if slices.Contains(cs, c) {
					continue
				}
				cs = append(cs, c)
			}
		}
		m.declared[seat] = cs
	}
}

// PlayedCards returns a copy of the cards seen this hand.
func (m *Memory) PlayedCards() []game.Card {
	return slices.Clone(m.played)
}

// KnownVoids returns the inferred void matrix ([seat][SuitIndex]).
func (m *Memory) KnownVoids() [4][4]bool {
	return m.voids
}

// DeclaredCards returns a clone of the per-seat revealed declaration cards.
func (m *Memory) DeclaredCards() [4][]game.Card {
	var out [4][]game.Card
	for seat := range m.declared {
		out[seat] = slices.Clone(m.declared[seat])
	}
	return out
}

// KnownCards returns, per seat, every card publicly proven to be in that hand: the revealed
// declaration cards followed by the located cards, without duplicates.
func (m *Memory) KnownCards() [4][]game.Card {
	var out [4][]game.Card
	for seat := range out {
		out[seat] = slices.Clone(m.declared[seat])
		for _, c := range m.located[seat] {
			if !slices.Contains(out[seat], c) {
				out[seat] = append(out[seat], c)
			}
		}
	}
	return out
}

// ObserveBid records a trump auction action; round and passesBefore are the engine's bidding round and pass count before it.
// The first action of an auction (round 1, no passes) starts a fresh record, which covers a Bitola reshuffle within one hand.
func (m *Memory) ObserveBid(seat int, action game.Action, round int, passesBefore int) {
	if round == 1 && passesBefore == 0 {
		m.bids = nil
	}
	rec := BidRecord{Seat: seat, Round: round, PassesBefore: passesBefore, Passed: action.Type == game.ActionPassTrump}
	if !rec.Passed && action.Suit != nil {
		suit := *action.Suit
		rec.Suit = &suit
	}
	m.bids = append(m.bids, rec)
}

// BidActionWithCandidate returns the action with the trump candidate's suit filled in when a round-1 pick carries none.
func BidActionWithCandidate(action game.Action, candidate *game.Card) game.Action {
	if action.Type == game.ActionPickTrump && action.Suit == nil && candidate != nil {
		suit := candidate.Suit
		action.Suit = &suit
	}
	return action
}

// Bids returns a copy of this hand's auction records in order.
func (m *Memory) Bids() []BidRecord {
	return slices.Clone(m.bids)
}
