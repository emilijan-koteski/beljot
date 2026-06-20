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
	voids      [4][4]bool     // [seat][SuitIndex]
	declared   [4][]game.Card // [seat] -> cards revealed publicly via declarations
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

// ObserveDeclarations records the publicly revealed declaration cards per seat.
// The match layer calls it only AFTER the contest resolves
// (GameState.DeclarationsResolved): by then the engine has already cleared the
// losing team's Declarations (declarations.go), so this snapshots exactly the
// public reveal — only the winning team's cards are ever stored, which keeps
// the Bitola no-peeking rule intact. Idempotent within a hand; reset by
// SyncHand on a hand advance.
func (m *Memory) ObserveDeclarations(players [4]game.PlayerState) {
	for seat := range players {
		var cs []game.Card
		for _, d := range players[seat].Declarations {
			cs = append(cs, d.Cards...) // append copies the cards into a fresh slice
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
