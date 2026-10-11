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
