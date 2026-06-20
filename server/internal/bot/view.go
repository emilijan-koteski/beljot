// Package bot implements the server-side bot decision layer (Story 10.3).
// Decide is a pure, deterministic function over a redacted View — humanized
// think delays, scheduling, and memory upkeep live in the match layer, never
// here. The View makes no-peeking structural: it carries only what a human
// player in that seat could know, so Decide cannot read other hands even
// though the server state has them.
package bot

import "github.com/emilijan/beljot/server/internal/game"

// View is the redacted, seat-local projection of the game state handed to
// Decide. Built by the match layer (buildBotView in bot_driver.go).
type View struct {
	Seat       int
	Hand       []game.Card
	LegalCards []game.Card

	Phase           game.Phase
	BiddingRound    int
	TrumpCandidate  *game.Card
	TrumpSuit       *game.Suit
	TrumpCallerSeat *int
	DealerSeat      int

	CurrentTrick     []game.TrickCard
	LeadSuit         *game.Suit
	ActivePlayerSeat int

	AwaitingDeclaration bool
	// PendingBelot is true when the engine is waiting on THIS seat's
	// Belote/Rebelote announcement decision.
	PendingBelot bool
	// PartnerProposedSurrender is true when this seat's partner has a
	// surrender proposal pending (the bot always accepts; it never initiates
	// and never responds to opponents' proposals).
	PartnerProposedSurrender bool

	TeamScores [2]int
	HandPoints [2]int
	TricksWon  [2]int

	// PlayedCards is every card seen resolved this hand (from match-layer
	// Memory); KnownVoids[seat][SuitIndex(suit)] marks inferred voids.
	PlayedCards []game.Card
	KnownVoids  [4][4]bool
	// KnownCards[seat] holds the exact cards we KNOW that seat is holding,
	// learned from the public declaration reveal. Only the winning declaration
	// team's cards are ever populated (the engine clears the losing team's on
	// resolution), so this never leaks the Bitola no-peeking rule. Empty before
	// the reveal; cards already played stay listed here, so consumers must drop
	// played/in-trick cards when reasoning about current holdings.
	KnownCards [4][]game.Card
}

// SuitIndex maps a suit to its index in KnownVoids' second dimension.
// Returns -1 for an unknown suit.
func SuitIndex(s game.Suit) int {
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
