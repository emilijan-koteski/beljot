package bot

import (
	"slices"

	"github.com/emilijan/beljot/server/internal/game"
)

// Memory tracks the publicly observable per-hand information bots use to
// reason about unseen cards: every card resolved via play_card this hand and
// the suit voids those plays revealed. The session manager owns one Memory
// per bot-inclusive match (GameState keeps no trick history — CurrentTrick
// clears on resolution), updates it on every successful play_card, and syncs
// it across hand boundaries.
type Memory struct {
	handNumber int
	played     []game.Card
	voids      [4][4]bool // [seat][SuitIndex]
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
	}
}

// ObservePlay records a resolved play_card. leadSuit is the OLD state's lead
// suit — nil when this card led the trick. A player who does not follow the
// led suit is void in it (following with trump also reveals the led-suit
// void; Bitola's must-cut rule makes no difference to the inference).
func (m *Memory) ObservePlay(seat int, card game.Card, leadSuit *game.Suit) {
	m.played = append(m.played, card)
	if leadSuit != nil && card.Suit != *leadSuit && seat >= 0 && seat < 4 {
		if idx := SuitIndex(*leadSuit); idx >= 0 {
			m.voids[seat][idx] = true
		}
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
