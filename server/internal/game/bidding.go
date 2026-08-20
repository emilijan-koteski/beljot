package game

import (
	"slices"

	"github.com/emilijan/beljot/server/internal/apperr"
)

// handleBidding processes trump bidding actions (pick_trump, pass_trump) during
// the bidding phase. Pure function — no side effects.
//
// Both variants bid over two rounds; the divergences all read state.Rules:
//
// Round 1: with VariantRules.HasTrumpCandidate, a pick adopts the face-up
// candidate's suit and action.Suit is ignored; without it, the picker names any
// suit freely and action.Suit is required.
// Round 2: the picker names a suit. A candidate's own suit is locked out
// (already spent in round 1); with no candidate all four suits stay open.
// Passed out: VariantRules.AllPassOutcome decides — either reshuffle, rotate
// the dealer, and re-deal, or refuse the dealer's fourth round-2 pass so the
// hand always finds a taker.
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

// MustPickTrump reports whether the engine will REFUSE `seat`'s pass, leaving
// pick_trump as its only legal bid. Under AllPassDealerMustPick the round-2
// bidding cannot be passed out: the dealer bids last and must name a suit,
// which is also what makes reshuffleAndRedeal unreachable under that config.
//
// This is the single definition of that condition, exported because three
// callers outside the engine need to agree with it exactly: the session
// manager's timer-expiry auto-action, the bot view handed to bot.Decide, and
// the per-seat flag the client reads to hide the Pass control. Each reads the
// rule config through this function — never the variant name (D-VAR-1).
//
// The condition describes exactly ONE seat — whichever is on the clock — so
// `seat` is a required argument rather than something callers are trusted to
// check themselves. A seat-less version returned true for all four seats and
// every caller had to remember to scope it; one that forgot would have three
// seats picking out of turn.
//
// `>= 3` rather than `== 3`: a state that somehow arrived with a higher count
// must not fall through and keep accepting passes forever.
func MustPickTrump(state *GameState, seat int) bool {
	return state.Phase == PhaseBidding &&
		state.ActivePlayerSeat == seat &&
		state.Rules.AllPassOutcome == AllPassDealerMustPick &&
		state.BiddingRound == 2 &&
		state.BiddingPassCount >= 3
}

// handlePassTrump processes a pass action during bidding.
func handlePassTrump(state *GameState) (*GameState, error) {
	// This pass would be round 2's fourth, i.e. the dealer's, and their config
	// gives them no right to pass — pick_trump is their only legal action.
	if MustPickTrump(state, state.ActivePlayerSeat) {
		return nil, apperr.ErrMustPickTrump
	}

	newState := cloneGameState(state)

	newState.BiddingPassCount++
	newState.ActivePlayerSeat = (newState.ActivePlayerSeat + 1) % 4

	// Check if all 4 players have passed in the current round
	if newState.BiddingPassCount == 4 {
		if newState.BiddingRound == 1 {
			// Transition to round 2
			newState.BiddingRound = 2
			newState.BiddingPassCount = 0
			newState.ActivePlayerSeat = (newState.DealerSeat + 1) % 4
			// Mark the round-2 reveal: every seat's two face-down cards are now
			// known to their owner, so round 2 is bid on a full eight-card hand.
			// The cards stay out of Hand (and therefore out of every snapshot) —
			// the match layer delivers each seat its own two through a per-seat
			// event.
			if newState.Rules.RevealFaceDownOnRound2 {
				newState.FaceDownRevealed = true
			}
		} else {
			// Round 2 complete — reshuffle and re-deal
			newState = reshuffleAndRedeal(newState)
		}
	}

	return newState, nil
}

// handlePickTrump processes a pick action during bidding.
//
// With a trump candidate, the pick also completes the deal — stage-2
// distribution (real-table rotation): walk seats from (Dealer+1)%4, taking
// cards off the front of newState.Deck — 3 per non-picker seat, 2 in the
// picker's slot. After the rotation, append the public TrumpCandidate to the
// picker's hand.
//
// Without a candidate every card was already dealt, so there is no rotation and
// the picker draws nothing; instead each seat's two face-down cards fold into
// its hand. Either way, instant-win detection then runs against the final
// 8-card hands and the phase moves to PhasePlaying (or PhaseMatchEnd on
// instant-win).
func handlePickTrump(state *GameState, action Action) (*GameState, error) {
	// Defensive: reject a state that does not match its own deal shape rather
	// than panicking on a slice index in the rotation below. Config-gated, not
	// removed.
	//
	// With a candidate, stage-2 distribution requires both the public candidate
	// and the full 11-card reserve; either being missing means the state has
	// skipped or already completed stage-1. Without one, the deal held nothing
	// back, so a candidate or a non-empty reserve means the state is malformed —
	// proving the rotation cannot run on an unvalidated deck under either
	// config.
	if state.Rules.HasTrumpCandidate {
		if state.TrumpCandidate == nil || len(state.Deck) != 11 {
			return nil, apperr.ErrWrongPhase
		}
	} else if state.TrumpCandidate != nil || len(state.Deck) != 0 {
		return nil, apperr.ErrWrongPhase
	}

	newState := cloneGameState(state)

	if newState.BiddingRound == 1 && newState.Rules.HasTrumpCandidate {
		// Round 1 with a candidate: trump is the candidate card's suit
		// (action.Suit ignored).
		suit := newState.TrumpCandidate.Suit
		newState.TrumpSuit = &suit
	} else {
		// Free-suit pick — round 2 in either variant, and round 1 too when
		// there is no candidate. action.Suit is required and must be a real
		// suit. A candidate's own suit is locked out (already "spent" in round
		// 1); with no candidate all four suits stay open.
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

	// Stage-2 distribution — only when a candidate was on the table. The guard
	// above already proved Deck holds exactly 11 in that case.
	if newState.TrumpCandidate != nil {
		deck := newState.Deck
		idx := 0
		for i := 0; i < 4; i++ {
			s := (newState.DealerSeat + 1 + i) % 4
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

	// Bidding has resolved, so the face-down cards are no longer secret from
	// anyone — fold them into their owners' hands and every seat holds eight,
	// exactly as it does after a stage-2 deal.
	mergeFaceDownCards(newState)

	// Instant-win check against final 8-card hands.
	if winnerTeam := checkInstantWin(newState); winnerTeam != nil {
		newState.WinnerTeam = winnerTeam
		newState.Phase = PhaseMatchEnd
		return newState, nil
	}

	// Declaration timing decides what a resolved bid opens into. Under a
	// dedicated-phase config every seat answers before a card is played, so the
	// hand enters PhaseDeclaring; otherwise trick 1 starts immediately and each
	// seat is prompted as its turn comes round.
	if newState.Rules.DeclarationTiming == DeclarationTimingDedicatedPhase {
		openDeclarationPhase(newState)
		return newState, nil
	}

	newState.Phase = PhasePlaying
	newState.ActivePlayerSeat = (newState.DealerSeat + 1) % 4
	newState.TrickNumber = 1
	newState.CurrentTrick = []TrickCard{}

	// Check if first player has declarable combinations
	checkDeclarationPrompt(newState)

	return newState, nil
}

// mergeFaceDownCards folds each seat's face-down cards into its hand and clears
// the hidden slot, so from this point on the state is indistinguishable from a
// variant that never had face-down cards. A no-op for deal shapes that produce
// none.
func mergeFaceDownCards(state *GameState) {
	for i := range state.Players {
		if len(state.Players[i].FaceDownCards) == 0 {
			continue
		}
		state.Players[i].Hand = append(state.Players[i].Hand, state.Players[i].FaceDownCards...)
		state.Players[i].FaceDownCards = nil
	}
	state.FaceDownRevealed = false
	syncFaceDownCounts(state)
}

// reshuffleAndRedeal pools all 32 cards (hands + Deck + TrumpCandidate),
// shuffles, rotates the dealer counter-clockwise, and re-runs stage-1.
// Instant-win cannot be detected here — only stage-2 (post-pick) produces
// the final 8-card hands needed for that check.
func reshuffleAndRedeal(state *GameState) *GameState {
	// Pool 32 cards: hands + remaining deck + visible candidate. If the pool
	// is malformed (an upstream code path mishandled state and dropped cards),
	// rebuild from a fresh deck instead of silently re-dealing a short pool —
	// dealCards's stage-1 indexing assumes exactly 32 cards.
	deck := make([]Card, 0, 32)
	for i := range state.Players {
		deck = append(deck, state.Players[i].Hand...)
		state.Players[i].Hand = []Card{}
		// Face-down cards are part of the pool too. Unreachable under
		// AllPassDealerMustPick (the only config that deals them refuses the
		// fourth round-2 pass), but a short pool would silently re-deal 30
		// cards, so recover them rather than trust the reachability argument.
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

	// Reset bidding/trump artifacts before re-dealing.
	state.Deck = nil
	state.TrumpCandidate = nil
	state.TrumpSuit = nil
	state.TrumpCallerSeat = nil
	state.FaceDownRevealed = false

	// Shuffle and rotate dealer
	ShuffleDeck(deck)
	state.DealerSeat = (state.DealerSeat + 1) % 4

	// Re-deal stage-1 (5 cards per seat + new candidate + 11-card Deck).
	dealCards(state, deck)

	// Reset bidding state
	state.Phase = PhaseDealing
	state.BiddingRound = 1
	state.BiddingPassCount = 0
	state.ActivePlayerSeat = (state.DealerSeat + 1) % 4

	return state
}

// cloneGameState creates a deep copy of the GameState to preserve immutability
// of the original state passed to ApplyAction.
func cloneGameState(state *GameState) *GameState {
	newState := *state // shallow copy of struct

	// Deep-copy pointer fields to break aliasing (D34 fix)
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

	// Deep clone slice fields
	newState.CurrentTrick = slices.Clone(state.CurrentTrick)
	newState.Deck = slices.Clone(state.Deck)

	// Deep clone player hands, face-down cards, and declarations
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
