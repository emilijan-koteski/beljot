package game

import (
	"slices"

	"github.com/emilijan/beljot/server/internal/apperr"
)

// handlePlaying processes card play actions during the playing phase.
// When the 4th card is played, the trick is resolved within the same call.
func handlePlaying(state *GameState, action Action) (*GameState, error) {
	if state.Phase != PhasePlaying {
		return nil, apperr.ErrWrongPhase
	}

	switch action.Type {
	case ActionPlayCard:
		return handlePlayCard(state, action)
	case ActionDeclare:
		return handleDeclare(state, action)
	case ActionSkipDeclare:
		return handleSkipDeclare(state, action)
	case ActionAnnounceBelot:
		return handleAnnounceBelot(state, action)
	case ActionSkipBelot:
		return handleSkipBelot(state, action)
	default:
		return nil, apperr.ErrWrongPhase
	}
}

// handlePlayCard validates and processes a play_card action.
func handlePlayCard(state *GameState, action Action) (*GameState, error) {
	if action.PlayerSeat < 0 || action.PlayerSeat > 3 {
		return nil, apperr.ErrNotYourTurn
	}

	if state.AwaitingDeclaration {
		return nil, apperr.ErrActionRequired
	}
	if state.PendingBelotSeat != nil {
		return nil, apperr.ErrActionRequired
	}

	if action.PlayerSeat != state.ActivePlayerSeat {
		return nil, apperr.ErrNotYourTurn
	}

	if action.Card == nil {
		return nil, apperr.ErrInvalidCard
	}
	if !playerHasCard(state, action.PlayerSeat, *action.Card) {
		return nil, apperr.ErrInvalidCard
	}

	if !isCardLegal(state, action.PlayerSeat, *action.Card) {
		return nil, apperr.ErrIllegalPlay
	}

	newState := cloneGameState(state)
	card := *action.Card

	handBeforePlay := slices.Clone(newState.Players[action.PlayerSeat].Hand)

	newState.Players[action.PlayerSeat].Hand = removeCard(
		newState.Players[action.PlayerSeat].Hand, card,
	)

	newState.CurrentTrick = append(newState.CurrentTrick, TrickCard{
		Card:       card,
		PlayerSeat: action.PlayerSeat,
	})

	if len(newState.CurrentTrick) == 1 {
		suit := card.Suit
		newState.LeadSuit = &suit
	}

	if shouldPromptBelot(newState, card, handBeforePlay) {
		seat := action.PlayerSeat
		newState.PendingBelotSeat = &seat
		return newState, nil
	}

	newState.ActivePlayerSeat = (newState.ActivePlayerSeat + 1) % 4

	if newState.TrickNumber == 1 && len(newState.CurrentTrick) < 4 {
		checkDeclarationPrompt(newState)
	}

	if len(newState.CurrentTrick) == 4 {
		resolveTrickWithDeclarations(newState)
	}

	return newState, nil
}

// resolveTrick determines the trick winner, calculates points, and sets up
// the next trick (or transitions to hand scoring after trick 8).
// Mutates the already-cloned state in place.
func resolveTrick(state *GameState) {
	trumpSuit := *state.TrumpSuit

	winnerSeat := determineTrickWinner(state.CurrentTrick, trumpSuit)
	points := calculateTrickPoints(state.CurrentTrick, trumpSuit)

	winnerTeam := TeamForSeat(winnerSeat)
	state.HandPoints[winnerTeam] += points
	state.TricksWon[winnerTeam]++
	state.TrickWinnerSeat = &winnerSeat

	if state.TrickNumber == 8 {
		state.Phase = PhaseHandScoring
		return
	}

	state.TrickNumber++
	state.CurrentTrick = nil
	state.LeadSuit = nil
	state.TrickWinnerSeat = nil
	state.ActivePlayerSeat = winnerSeat
}

// determineTrickWinner returns the seat of the trick winner: the highest trump if any was played,
// otherwise the highest card of the led suit.
func determineTrickWinner(trick []TrickCard, trumpSuit Suit) int {
	return currentTrickWinnerSeat(trick, trumpSuit)
}

// calculateTrickPoints returns the total card point value of all cards in the trick.
func calculateTrickPoints(trick []TrickCard, trumpSuit Suit) int {
	total := 0
	for _, tc := range trick {
		if tc.Card.Suit == trumpSuit {
			total += TrumpCardPoints[tc.Card.Rank]
		} else {
			total += NonTrumpCardPoints[tc.Card.Rank]
		}
	}
	return total
}

// playerHasCard checks whether the player at the given seat holds the specified card.
func playerHasCard(state *GameState, seat int, card Card) bool {
	for _, c := range state.Players[seat].Hand {
		if c.Rank == card.Rank && c.Suit == card.Suit {
			return true
		}
	}
	return false
}

// removeCard returns a new slice with the first matching card removed.
func removeCard(hand []Card, card Card) []Card {
	c := len(hand)
	if c > 0 {
		c--
	}
	result := make([]Card, 0, c)
	removed := false
	for _, c := range hand {
		if !removed && c.Rank == card.Rank && c.Suit == card.Suit {
			removed = true
			continue
		}
		result = append(result, c)
	}
	return result
}
