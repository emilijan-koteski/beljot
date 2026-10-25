package game

import (
	"slices"

	"github.com/emilijan/beljot/server/internal/apperr"
)

// handleBidding processes pick_trump and pass_trump during the bidding phase.
// It is a pure function; round and all-pass behaviour come from state.Rules.
func handleBidding(state *GameState, action Action) (*GameState, error) {
	if state.Phase != PhaseBidding {
		return nil, apperr.ErrWrongPhase
	}

	if action.PlayerSeat != state.ActivePlayerSeat {
		return nil, apperr.ErrNotYourTurn
	}

	switch action.Type {
	case ActionPassTrump:
		return handlePassTrump(state)
	case ActionPickTrump:
		return handlePickTrump(state, action)
	default:
		return nil, apperr.ErrWrongPhase
	}
}

// MustPickTrump reports whether the engine will refuse seat's pass, leaving pick_trump as its only legal bid.
// Under AllPassDealerMustPick the dealer must name a suit as the fourth bidder of the free-suit stage.
func MustPickTrump(state *GameState, seat int) bool {
	return state.Phase == PhaseBidding &&
		state.Rules.AllPassOutcome == AllPassDealerMustPick &&
		(!state.Rules.HasTrumpCandidate || state.BiddingRound == 2) &&
		state.ActivePlayerSeat == seat &&
		state.BiddingPassCount >= 3
}

// handlePassTrump processes a pass action during bidding.
func handlePassTrump(state *GameState) (*GameState, error) {
	if MustPickTrump(state, state.ActivePlayerSeat) {
		return nil, apperr.ErrMustPickTrump
	}

	newState := cloneGameState(state)

	newState.BiddingPassCount++
	newState.ActivePlayerSeat = newState.Rules.NextSeat(newState.ActivePlayerSeat)

	if newState.BiddingPassCount == 4 {
		if newState.BiddingRound == 1 {
			newState.BiddingRound = 2
			newState.BiddingPassCount = 0
			newState.ActivePlayerSeat = newState.Rules.NextSeat(newState.DealerSeat)
		} else {
			newState = reshuffleAndRedeal(newState)
		}
	}

	return newState, nil
}

// handlePickTrump sets trump, completes the deal, and opens declarations or trick 1.
// With a candidate it deals the 11-card reserve in table rotation; without one it folds in the face-down cards.
func handlePickTrump(state *GameState, action Action) (*GameState, error) {
	if state.Rules.HasTrumpCandidate {
		if state.TrumpCandidate == nil || len(state.Deck) != 11 {
			return nil, apperr.ErrWrongPhase
		}
	} else if state.TrumpCandidate != nil || len(state.Deck) != 0 {
		return nil, apperr.ErrWrongPhase
	}

	newState := cloneGameState(state)

	if newState.BiddingRound == 1 && newState.Rules.HasTrumpCandidate {
		suit := newState.TrumpCandidate.Suit
		newState.TrumpSuit = &suit
	} else {
		if action.Suit == nil {
			return nil, apperr.ErrInvalidBid
		}
		if !validSuits[*action.Suit] {
			return nil, apperr.ErrInvalidBid
		}
		if newState.TrumpCandidate != nil && *action.Suit == newState.TrumpCandidate.Suit {
			return nil, apperr.ErrInvalidBid
		}
		suit := *action.Suit
		newState.TrumpSuit = &suit
	}

	seat := action.PlayerSeat
	newState.TrumpCallerSeat = &seat

	if newState.TrumpCandidate != nil {
		deck := newState.Deck
		idx := 0
		for i := 0; i < 4; i++ {
			s := newState.Rules.SeatAfter(newState.DealerSeat, 1+i)
			n := 3
			if s == seat {
				n = 2
			}
			newState.Players[s].Hand = append(newState.Players[s].Hand, deck[idx:idx+n]...)
			idx += n
		}
		newState.Players[seat].Hand = append(newState.Players[seat].Hand, *newState.TrumpCandidate)
		newState.Deck = nil
		newState.TrumpCandidate = nil
	}

	mergeFaceDownCards(newState)

	if winnerTeam := checkInstantWin(newState); winnerTeam != nil {
		newState.WinnerTeam = winnerTeam
		newState.Phase = PhaseMatchEnd
		newState.WonByInstantWin = true
		return newState, nil
	}

	if newState.Rules.DeclarationsEnabled && newState.Rules.DeclarationTiming == DeclarationTimingDedicatedPhase {
		openDeclarationPhase(newState)
		return newState, nil
	}

	newState.Phase = PhasePlaying
	newState.ActivePlayerSeat = newState.Rules.NextSeat(newState.DealerSeat)
	newState.TrickNumber = 1
	newState.CurrentTrick = []TrickCard{}

	checkDeclarationPrompt(newState)

	return newState, nil
}

// mergeFaceDownCards folds each seat's face-down cards into its hand and clears the hidden slot.
func mergeFaceDownCards(state *GameState) {
	for i := range state.Players {
		if len(state.Players[i].FaceDownCards) == 0 {
			continue
		}
		state.Players[i].Hand = append(state.Players[i].Hand, state.Players[i].FaceDownCards...)
		state.Players[i].FaceDownCards = nil
	}
	syncFaceDownCounts(state)
}

// reshuffleAndRedeal pools all 32 cards, shuffles, rotates the dealer, and re-runs stage-1.
// A short pool is replaced with a fresh deck.
func reshuffleAndRedeal(state *GameState) *GameState {
	deck := make([]Card, 0, 32)
	for i := range state.Players {
		deck = append(deck, state.Players[i].Hand...)
		state.Players[i].Hand = []Card{}
		deck = append(deck, state.Players[i].FaceDownCards...)
		state.Players[i].FaceDownCards = nil
	}
	deck = append(deck, state.Deck...)
	if state.TrumpCandidate != nil {
		deck = append(deck, *state.TrumpCandidate)
	}
	if len(deck) != 32 {
		deck = NewDeck()
	}

	state.Deck = nil
	state.TrumpCandidate = nil
	state.TrumpSuit = nil
	state.TrumpCallerSeat = nil

	ShuffleDeck(deck)
	state.DealerSeat = state.Rules.NextSeat(state.DealerSeat)

	dealCards(state, deck)

	state.Phase = PhaseDealing
	state.BiddingRound = 1
	state.BiddingPassCount = 0
	state.ActivePlayerSeat = state.Rules.NextSeat(state.DealerSeat)

	return state
}

// cloneGameState creates a deep copy of the GameState to preserve immutability
// of the original state passed to ApplyAction.
func cloneGameState(state *GameState) *GameState {
	newState := *state

	if state.TrumpSuit != nil {
		v := *state.TrumpSuit
		newState.TrumpSuit = &v
	}
	if state.TrumpCallerSeat != nil {
		v := *state.TrumpCallerSeat
		newState.TrumpCallerSeat = &v
	}
	if state.TrumpCandidate != nil {
		v := *state.TrumpCandidate
		newState.TrumpCandidate = &v
	}
	if state.LeadSuit != nil {
		v := *state.LeadSuit
		newState.LeadSuit = &v
	}
	if state.TrickWinnerSeat != nil {
		v := *state.TrickWinnerSeat
		newState.TrickWinnerSeat = &v
	}
	if state.TurnExpiresAt != nil {
		v := *state.TurnExpiresAt
		newState.TurnExpiresAt = &v
	}
	if state.PendingBelotSeat != nil {
		v := *state.PendingBelotSeat
		newState.PendingBelotSeat = &v
	}
	if state.WinnerTeam != nil {
		v := *state.WinnerTeam
		newState.WinnerTeam = &v
	}
	if state.ReconnectExpiresAt != nil {
		v := *state.ReconnectExpiresAt
		newState.ReconnectExpiresAt = &v
	}
	if state.LastHandResult != nil {
		v := *state.LastHandResult
		newState.LastHandResult = &v
	}
	if state.SurrenderProposerSeat != nil {
		v := *state.SurrenderProposerSeat
		newState.SurrenderProposerSeat = &v
	}

	newState.CurrentTrick = slices.Clone(state.CurrentTrick)
	newState.Deck = slices.Clone(state.Deck)

	for i := range newState.Players {
		newState.Players[i].Hand = slices.Clone(state.Players[i].Hand)
		newState.Players[i].FaceDownCards = slices.Clone(state.Players[i].FaceDownCards)
		newDecls := slices.Clone(state.Players[i].Declarations)
		for j := range newDecls {
			newDecls[j].Cards = slices.Clone(newDecls[j].Cards)
		}
		newState.Players[i].Declarations = newDecls
	}

	return &newState
}
