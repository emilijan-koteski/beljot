package game

import "fmt"

// Suit represents a card suit using single-character encoding.
type Suit string

const (
	SuitSpades   Suit = "S"
	SuitHearts   Suit = "H"
	SuitDiamonds Suit = "D"
	SuitClubs    Suit = "C"
)

var AllSuits = [4]Suit{SuitSpades, SuitHearts, SuitDiamonds, SuitClubs}

// Rank represents a card rank using single-character encoding.
type Rank string

const (
	Rank7     Rank = "7"
	Rank8     Rank = "8"
	Rank9     Rank = "9"
	RankTen   Rank = "T"
	RankJack  Rank = "J"
	RankQueen Rank = "Q"
	RankKing  Rank = "K"
	RankAce   Rank = "A"
)

var AllRanks = [8]Rank{Rank7, Rank8, Rank9, RankTen, RankJack, RankQueen, RankKing, RankAce}

// Card represents a playing card with a rank and suit.
type Card struct {
	Rank Rank `json:"rank"`
	Suit Suit `json:"suit"`
}

// String returns the 2-character card ID (e.g., "KS" for King of Spades).
func (c Card) String() string {
	return string(c.Rank) + string(c.Suit)
}

var validSuits = map[Suit]bool{
	SuitSpades: true, SuitHearts: true, SuitDiamonds: true, SuitClubs: true,
}

var validRanks = map[Rank]bool{
	Rank7: true, Rank8: true, Rank9: true, RankTen: true,
	RankJack: true, RankQueen: true, RankKing: true, RankAce: true,
}

// ParseCard parses a 2-character card ID string (e.g., "KS") into a Card.
func ParseCard(id string) (Card, error) {
	if len(id) != 2 {
		return Card{}, fmt.Errorf("invalid card ID %q: must be exactly 2 characters", id)
	}
	rank := Rank(id[0:1])
	suit := Suit(id[1:2])
	if !validRanks[rank] {
		return Card{}, fmt.Errorf("invalid card ID %q: unknown rank %q", id, rank)
	}
	if !validSuits[suit] {
		return Card{}, fmt.Errorf("invalid card ID %q: unknown suit %q", id, suit)
	}
	return Card{Rank: rank, Suit: suit}, nil
}

// Variant represents a game variant.
type Variant string

const (
	VariantBitola  Variant = "bitola"
	VariantCroatia Variant = "croatia"
)

// DealShape names how a hand's 32 cards reach the table.
type DealShape string

const (
	DealShapeCandidate        DealShape = "candidate"
	DealShapeAllBeforeBidding DealShape = "all_before_bidding"
)

// AllPassOutcome names what happens when bidding threatens to find no taker.
type AllPassOutcome string

const (
	AllPassReshuffleAndRotate AllPassOutcome = "reshuffle_and_rotate"
	AllPassDealerMustPick     AllPassOutcome = "dealer_must_pick"
)

// DeclarationTiming names when declarations are collected and revealed.
type DeclarationTiming string

const (
	DeclarationTimingDuringFirstTrick DeclarationTiming = "during_first_trick"
	DeclarationTimingDedicatedPhase   DeclarationTiming = "dedicated_phase"
)

// TieRule names how a hand whose two sides tie on points is settled.
type TieRule string

const (
	TieRuleAllToOpponents TieRule = "all_to_opponents"
	TieRuleHangingPoints  TieRule = "hanging_points"
)

type VariantRules struct {
	DealShape           DealShape
	HasTrumpCandidate   bool
	AllPassOutcome      AllPassOutcome
	DeclarationOverlap  bool
	DeclarationTiming   DeclarationTiming
	DeclarationsEnabled bool
	StopAtTarget        bool
	TieRule             TieRule
}

func RulesFor(v Variant) VariantRules {
	if v == VariantCroatia {
		return VariantRules{
			DealShape:           DealShapeAllBeforeBidding,
			HasTrumpCandidate:   false,
			AllPassOutcome:      AllPassDealerMustPick,
			DeclarationOverlap:  true,
			DeclarationTiming:   DeclarationTimingDedicatedPhase,
			DeclarationsEnabled: true,
			StopAtTarget:        false,
			TieRule:             TieRuleAllToOpponents,
		}
	}
	return VariantRules{
		DealShape:           DealShapeCandidate,
		HasTrumpCandidate:   true,
		AllPassOutcome:      AllPassReshuffleAndRotate,
		DeclarationOverlap:  false,
		DeclarationTiming:   DeclarationTimingDuringFirstTrick,
		DeclarationsEnabled: true,
		StopAtTarget:        false,
		TieRule:             TieRuleHangingPoints,
	}
}

// Phase represents the current phase of the game state machine.
type Phase string

const (
	PhaseDealing        Phase = "dealing"
	PhaseBidding        Phase = "bidding"
	PhaseDeclaring      Phase = "declaring"
	PhasePlaying        Phase = "playing"
	PhaseTrickResolving Phase = "trick_resolving"
	PhaseHandScoring    Phase = "hand_scoring"
	PhaseHandComplete   Phase = "hand_complete"
	PhaseMatchEnd       Phase = "match_end"
	PhasePaused         Phase = "paused"
	PhaseDisconnected   Phase = "disconnected"
)

func AllPhases() []Phase {
	return []Phase{
		PhaseDealing,
		PhaseBidding,
		PhaseDeclaring,
		PhasePlaying,
		PhaseTrickResolving,
		PhaseHandScoring,
		PhaseHandComplete,
		PhaseMatchEnd,
		PhasePaused,
		PhaseDisconnected,
	}
}

const (
	ActionPlayCard      = "play_card"
	ActionPickTrump     = "pick_trump"
	ActionPassTrump     = "pass_trump"
	ActionDeclare       = "declare"
	ActionSkipDeclare   = "skip_declare"
	ActionAnnounceBelot = "announce_belot"
	ActionSkipBelot     = "skip_belot"
	ActionContinue      = "continue"
	ActionPause         = "pause"
	ActionUnpause       = "unpause"
	ActionOwnerUnpause  = "owner_unpause"

	ActionSurrenderRequest = "surrender_request"
	ActionSurrenderAccept  = "surrender_accept"
	ActionSurrenderDecline = "surrender_decline"
)

// Action represents a player action submitted to the rules engine.
type Action struct {
	Type       string `json:"type"`
	PlayerSeat int    `json:"playerSeat"`
	Card       *Card  `json:"card,omitempty"`
	Suit       *Suit  `json:"suit,omitempty"`
}

// DeclarationType represents the kind of declaration.
type DeclarationType string

const (
	DeclarationSequence    DeclarationType = "sequence"
	DeclarationFourOfAKind DeclarationType = "four_of_a_kind"
)

// Declaration represents a declarable combination of cards.
type Declaration struct {
	Type       DeclarationType `json:"type"`
	Cards      []Card          `json:"cards"`
	PlayerSeat int             `json:"playerSeat"`
	Value      int             `json:"value"`
}

var TrumpCardPoints = map[Rank]int{
	RankJack:  20,
	Rank9:     14,
	RankAce:   11,
	RankTen:   10,
	RankKing:  4,
	RankQueen: 3,
	Rank8:     0,
	Rank7:     0,
}

var NonTrumpCardPoints = map[Rank]int{
	RankAce:   11,
	RankTen:   10,
	RankKing:  4,
	RankQueen: 3,
	RankJack:  2,
	Rank9:     0,
	Rank8:     0,
	Rank7:     0,
}

var TrumpRankOrder = map[Rank]int{
	RankJack:  7,
	Rank9:     6,
	RankAce:   5,
	RankTen:   4,
	RankKing:  3,
	RankQueen: 2,
	Rank8:     1,
	Rank7:     0,
}

var NonTrumpRankOrder = map[Rank]int{
	RankAce:   7,
	RankTen:   6,
	RankKing:  5,
	RankQueen: 4,
	RankJack:  3,
	Rank9:     2,
	Rank8:     1,
	Rank7:     0,
}

var NaturalRankSequence = []Rank{Rank7, Rank8, Rank9, RankTen, RankJack, RankQueen, RankKing, RankAce}

var NaturalRankOrder = func() map[Rank]int {
	m := make(map[Rank]int, len(NaturalRankSequence))
	for i, r := range NaturalRankSequence {
		m[r] = i
	}
	return m
}()

// NewDeck returns a full 32-card deck (7 through Ace in all 4 suits).
func NewDeck() []Card {
	deck := make([]Card, 0, 32)
	for _, suit := range AllSuits {
		for _, rank := range AllRanks {
			deck = append(deck, Card{Rank: rank, Suit: suit})
		}
	}
	return deck
}
