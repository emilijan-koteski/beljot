package bot

import "github.com/emilijan/beljot/server/internal/game"

// View is the redacted, seat-local projection of the game state handed to Decide.
// It carries only what a human player in that seat could know.
type View struct {
	Seat       int
	Hand       []game.Card
	LegalCards []game.Card

	Phase             game.Phase
	BiddingRound      int
	TrumpCandidate    *game.Card
	TrumpSuit         *game.Suit
	TrumpCallerSeat   *int
	DealerSeat        int
	MustPickTrump     bool
	HasTrumpCandidate bool

	CurrentTrick     []game.TrickCard
	LeadSuit         *game.Suit
	ActivePlayerSeat int
	Clockwise        bool

	AwaitingDeclaration      bool
	HasDeclarations          bool
	PendingBelot             bool
	PartnerProposedSurrender bool

	TeamScores [2]int
	HandPoints [2]int
	DeclPoints [2]int
	TricksWon  [2]int

	TrickNumber          int
	MatchTarget          int
	StopAtTarget         bool
	DeclarationsTransfer bool

	PlayedCards   []game.Card
	KnownVoids    [4][4]bool
	KnownCards    [4][]game.Card
	DeclaredCards [4][]game.Card
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
