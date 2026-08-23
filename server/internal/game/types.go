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

// AllSuits contains all four suits in standard order.
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

// AllRanks contains all eight ranks in ascending order (7 through Ace).
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

// validSuits is a lookup set for validation.
var validSuits = map[Suit]bool{
	SuitSpades: true, SuitHearts: true, SuitDiamonds: true, SuitClubs: true,
}

// validRanks is a lookup set for validation.
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
	// DealShapeCandidate is the two-stage deal: 3 then 2 cards to each seat, one
	// card lifted face-up as the public trump candidate, and the remaining 11
	// held back for stage-2 distribution once a seat takes trump.
	DealShapeCandidate DealShape = "candidate"
	// DealShapeAllBeforeBidding deals every card before bidding opens: 3 then 3
	// to each seat, then 2 more per seat placed face-down. Nothing is held back
	// and there is no candidate, so bidding is a bare named suit.
	DealShapeAllBeforeBidding DealShape = "all_before_bidding"
)

// AllPassOutcome names what happens when bidding threatens to find no taker.
type AllPassOutcome string

const (
	// AllPassReshuffleAndRotate pools all 32 cards once the second bidding
	// round is passed out, rotates the dealer counter-clockwise, and deals the
	// same hand number again.
	AllPassReshuffleAndRotate AllPassOutcome = "reshuffle_and_rotate"
	// AllPassDealerMustPick makes the dealer — always the LAST bidder of the
	// free-suit stage — pick with no right to pass once the other three seats
	// have passed, so the hand always finds a taker and is never passed out.
	// With no candidate the free-suit stage IS round 1, so a second round
	// never opens (the shipped Croatian preset). With a candidate the force
	// applies at the end of round 2, after a passed-out round 1 opens it.
	AllPassDealerMustPick AllPassOutcome = "dealer_must_pick"
)

// DeclarationTiming names when declarations are collected and revealed.
type DeclarationTiming string

const (
	// DeclarationTimingDuringFirstTrick collects each player's declaration as
	// their turn comes round in trick 1 and reveals the winner at trick 2.
	DeclarationTimingDuringFirstTrick DeclarationTiming = "during_first_trick"
	// DeclarationTimingDedicatedPhase runs a phase between bidding and trick 1
	// in which all four seats declare or skip, then the result is revealed.
	DeclarationTimingDedicatedPhase DeclarationTiming = "dedicated_phase"
)

// TieRule names how a hand whose two sides tie on points is settled.
type TieRule string

const (
	// TieRuleAllToOpponents awards the whole tied pool to the taker's opponents.
	TieRuleAllToOpponents TieRule = "all_to_opponents"
	// TieRuleHangingPoints scores the tied hand for nobody and carries the pool
	// over to whichever side wins the next decisive hand.
	TieRuleHangingPoints TieRule = "hanging_points"
)

// VariantRules is the per-rule configuration a variant resolves to. It is the
// engine's ONLY variant-aware surface: RulesFor is resolved once at game
// initialization, carried on GameState.Rules, and every divergent branch reads
// a field here — no engine code compares a variant name (D-VAR-1).
//
// Every field is a value type (no pointer, slice, or map) on purpose, so
// cloneGameState's shallow struct copy carries the whole config correctly with
// no clone line of its own.
//
// All six fields are populated by both presets from day one. DealShape,
// HasTrumpCandidate, AllPassOutcome, DeclarationOverlap and DeclarationTiming
// are READ today; TieRule describes each variant's authentic rule and is read
// once the story that implements it lands.
type VariantRules struct {
	// DealShape selects the dealing sequence — see DealShape.
	DealShape DealShape
	// HasTrumpCandidate is true when one card is lifted face-up and round-1
	// bidding is a take-it-or-pass on that card's suit. False means trump is a
	// freely named suit and the taker draws no card.
	HasTrumpCandidate bool
	// AllPassOutcome selects what happens when bidding threatens to find no
	// taker — see AllPassOutcome.
	AllPassOutcome AllPassOutcome
	// DeclarationOverlap allows one card to count toward more than one
	// declaration. False keeps one-card-one-group dedup by higher value.
	DeclarationOverlap bool
	// DeclarationTiming selects when declarations are collected — see
	// DeclarationTiming. Read by handlePickTrump: a dedicated-phase config
	// opens PhaseDeclaring instead of going straight to trick 1.
	DeclarationTiming DeclarationTiming
	// TieRule selects how a tied hand is settled — see TieRule. This field
	// states each variant's AUTHENTIC rule; the engine currently awards every
	// tied hand to the taker's opponents for both variants as a deliberate
	// interim stand-in, so Bitola's hanging-points value here is not yet
	// reflected in behaviour.
	//
	// Not read yet — behaviour lands with the hanging-points story.
	TieRule TieRule
}

// RulesFor resolves a variant string to its fully-populated rule preset. It is
// the single variant-aware construct in this package.
//
// An unrecognised variant string resolves to the Bitola preset — explicit and
// tested, never a zero-value config (which would deal no candidate and reject
// every round-1 take).
func RulesFor(v Variant) VariantRules {
	if v == VariantCroatia {
		return VariantRules{
			DealShape:          DealShapeAllBeforeBidding,
			HasTrumpCandidate:  false,
			AllPassOutcome:     AllPassDealerMustPick,
			DeclarationOverlap: true,
			DeclarationTiming:  DeclarationTimingDedicatedPhase,
			TieRule:            TieRuleAllToOpponents,
		}
	}
	return VariantRules{
		DealShape:          DealShapeCandidate,
		HasTrumpCandidate:  true,
		AllPassOutcome:     AllPassReshuffleAndRotate,
		DeclarationOverlap: false,
		DeclarationTiming:  DeclarationTimingDuringFirstTrick,
		TieRule:            TieRuleHangingPoints,
	}
}

// Phase represents the current phase of the game state machine.
type Phase string

const (
	PhaseDealing Phase = "dealing"
	PhaseBidding Phase = "bidding"
	// PhaseDeclaring is the dedicated declaration phase between bidding and
	// trick 1, entered only under VariantRules.DeclarationTiming ==
	// DeclarationTimingDedicatedPhase. Seats are prompted one at a time
	// counter-clockwise from the trick-1 leader; once all four have answered the
	// contest resolves and the phase moves straight to PhasePlaying at trick 1.
	// Under DeclarationTimingDuringFirstTrick this phase never occurs.
	PhaseDeclaring      Phase = "declaring"
	PhasePlaying        Phase = "playing"
	PhaseTrickResolving Phase = "trick_resolving"
	PhaseHandScoring    Phase = "hand_scoring"
	// PhaseHandComplete holds after a hand is scored and before the next hand is
	// dealt: the server waits for players to acknowledge the score (action:continue)
	// or for an auto-continue timeout, so the score breakdown and trick-collect
	// animation are seen before the next hand begins.
	PhaseHandComplete Phase = "hand_complete"
	PhaseMatchEnd     Phase = "match_end"
	PhasePaused       Phase = "paused"
	PhaseDisconnected Phase = "disconnected"
)

// AllPhases is every Phase the server can put on the wire, in state-machine
// order. It exists so the phase strings can be PINNED across the Go/TypeScript
// boundary: the ws contract test writes this list to a golden, and the client
// builds its `Phase` union from that same golden, so adding a phase on one side
// without the other fails a test instead of drifting silently.
//
// The client union additionally carries "" for "no game loaded", which is a
// client-local state the server never sends and so is deliberately absent here.
//
// Keep in sync with the constants above — TestAllPhasesCoversEveryConstant
// scans this file and fails if a constant is declared but not listed.
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

// Action type constants for player actions.
const (
	ActionPlayCard      = "play_card"
	ActionPickTrump     = "pick_trump"
	ActionPassTrump     = "pass_trump"
	ActionDeclare       = "declare"
	ActionSkipDeclare   = "skip_declare"
	ActionAnnounceBelot = "announce_belot"
	ActionSkipBelot     = "skip_belot"
	// ActionContinue acknowledges the hand-complete pause; the next hand deals
	// once every connected player has continued (or the timeout fires).
	ActionContinue     = "continue"
	ActionPause        = "pause"
	ActionUnpause      = "unpause"
	ActionOwnerUnpause = "owner_unpause"

	// Surrender actions (Story 8.2). Each player may initiate at most one
	// surrender request per match; the proposer's partner accepts (ends match
	// as opponent win) or declines (consumes the proposer's attempt, play
	// resumes).
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

// TrumpCardPoints maps ranks to their point values when the suit is trump.
// J=20, 9=14, A=11, T=10, K=4, Q=3, 8=0, 7=0
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

// NonTrumpCardPoints maps ranks to their point values when the suit is not trump.
// A=11, T=10, K=4, Q=3, J=2, 9=0, 8=0, 7=0
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

// TrumpRankOrder maps ranks to their strength ordering when the suit is trump.
// Higher value wins. J is strongest (7), 7 is weakest (0).
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

// NonTrumpRankOrder maps ranks to their strength ordering when the suit is not trump.
// Higher value wins. A is strongest (7), 7 is weakest (0).
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

// NaturalRankSequence lists ranks low-to-high in declaration (sequence) order:
// 7 < 8 < 9 < T < J < Q < K < A. This is the meld ordering (Jack outranks Ten),
// distinct from the trick-taking orders above. Source of truth for both the
// declaration engine and the bot's sequence-adjacency reasoning.
var NaturalRankSequence = []Rank{Rank7, Rank8, Rank9, RankTen, RankJack, RankQueen, RankKing, RankAce}

// NaturalRankOrder maps a rank to its index in NaturalRankSequence.
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
