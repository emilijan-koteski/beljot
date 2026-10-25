package bot

import (
	"slices"

	"github.com/emilijan/beljot/server/internal/game"
)

// Memory tracks the cards played this hand and the suit voids those plays revealed.
// The match layer owns one per bot match because GameState keeps no trick history.
type Memory struct {
	handNumber int
	played     []game.Card
	voids      [4][4]bool
	declared   [4][]game.Card
}

// NewMemory returns a Memory primed for the first hand.
func NewMemory() *Memory {
	return &Memory{handNumber: 1}
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
	}
}

// ObservePlay records a resolved play and marks the seat void in the led suit when it did not follow.
// A non-trump discard also proves a trump void; leadSuit is nil when this card led, trump nil only when unknown.
func (m *Memory) ObservePlay(seat int, card game.Card, leadSuit, trump *game.Suit) {
	m.played = append(m.played, card)
	if leadSuit == nil || card.Suit == *leadSuit {
		return
	}
	m.markVoid(seat, *leadSuit)
	if trump != nil && card.Suit != *trump {
		m.markVoid(seat, *trump)
	}
}

func (m *Memory) markVoid(seat int, suit game.Suit) {
	idx := SuitIndex(suit)
	if seat < 0 || seat >= 4 || idx < 0 {
		return
	}
	m.voids[seat][idx] = true
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

// KnownCards returns a clone of the per-seat revealed declaration cards.
func (m *Memory) KnownCards() [4][]game.Card {
	var out [4][]game.Card
	for seat := range m.declared {
		out[seat] = slices.Clone(m.declared[seat])
	}
	return out
}
