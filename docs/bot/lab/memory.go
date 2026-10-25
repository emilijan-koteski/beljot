package botlab

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
	trumpVoids [4][4]bool
	declared   [4][]game.Card
	located    [4][]game.Card
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
		m.trumpVoids = [4][4]bool{}
		m.declared = [4][]game.Card{}
		m.located = [4][]game.Card{}
	}
}

// ObservePlay records a resolved play and marks the seat void when it did not follow leadSuit.
// leadSuit is the pre-play lead suit, nil when this card led the trick.
func (m *Memory) ObservePlay(seat int, card game.Card, leadSuit *game.Suit) {
	m.played = append(m.played, card)
	if leadSuit != nil && card.Suit != *leadSuit && seat >= 0 && seat < 4 {
		if idx := SuitIndex(*leadSuit); idx >= 0 {
			m.voids[seat][idx] = true
		}
	}
}

// ObserveDiscard marks the seat void in trump as well when it played neither the led suit nor a trump.
func (m *Memory) ObserveDiscard(seat int, card game.Card, leadSuit *game.Suit, trump *game.Suit) {
	if leadSuit == nil || trump == nil || seat < 0 || seat > 3 {
		return
	}
	if card.Suit != *leadSuit && card.Suit != *trump {
		if idx := SuitIndex(*trump); idx >= 0 {
			m.trumpVoids[seat][idx] = true
		}
	}
}

// KnownVoidsWithTrump returns the void matrix including trump voids proven by discards.
func (m *Memory) KnownVoidsWithTrump() [4][4]bool {
	out := m.voids
	for s := range out {
		for i := range out[s] {
			out[s][i] = out[s][i] || m.trumpVoids[s][i]
		}
	}
	return out
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

// ObserveCandidate records that the caller received the face-up candidate card.
func (m *Memory) ObserveCandidate(seat int, c game.Card) {
	if seat < 0 || seat > 3 || slices.Contains(m.located[seat], c) {
		return
	}
	m.located[seat] = append(m.located[seat], c)
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

// KnownCards returns, per seat, the revealed declaration cards followed by the located cards, without duplicates.
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
