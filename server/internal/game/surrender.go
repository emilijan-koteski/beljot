package game

import "github.com/emilijan/beljot/server/internal/apperr"

// handleSurrenderRequest opens a surrender proposal during bidding, declaring or play.
// Each seat gets one request per match, spent on request whether or not the partner accepts.
func handleSurrenderRequest(state *GameState, action Action) (*GameState, error) {
	if state.Phase == PhasePaused {
		return nil, apperr.ErrGamePaused
	}
	if state.Phase != PhasePlaying && state.Phase != PhaseBidding && state.Phase != PhaseDeclaring {
		return nil, apperr.ErrWrongPhase
	}

	seat := action.PlayerSeat
	if seat < 0 || seat > 3 {
		return nil, apperr.ErrBadRequest
	}

	if state.SurrenderProposerSeat != nil {
		return nil, apperr.ErrActionRequired
	}

	if state.SurrenderUsed[seat] {
		return nil, apperr.ErrSurrenderExhausted
	}

	newState := cloneGameState(state)
	seatCopy := seat
	newState.SurrenderProposerSeat = &seatCopy
	newState.SurrenderUsed[seat] = true

	return newState, nil
}

// handleSurrenderAccept lets the proposer's partner accept, ending the match with the opposing team as winner.
// It clears the turn deadline fields; the session manager stops the actual timer.
func handleSurrenderAccept(state *GameState, action Action) (*GameState, error) {
	if state.Phase == PhasePaused {
		return nil, apperr.ErrGamePaused
	}
	if state.SurrenderProposerSeat == nil {
		return nil, apperr.ErrWrongPhase
	}

	seat := action.PlayerSeat
	if seat < 0 || seat > 3 {
		return nil, apperr.ErrBadRequest
	}

	proposer := *state.SurrenderProposerSeat
	partner := (proposer + 2) % 4
	if seat != partner {
		return nil, apperr.ErrInvalidAction
	}

	newState := cloneGameState(state)

	if stopAtTargetIfReached(newState) {
		return newState, nil
	}

	opponentTeam := 1 - TeamForSeat(proposer)
	newState.WinnerTeam = &opponentTeam
	newState.Phase = PhaseMatchEnd
	newState.SurrenderProposerSeat = nil
	newState.TurnExpiresAt = nil
	newState.TurnTimeRemaining = 0

	return newState, nil
}

// handleSurrenderDecline lets the proposer's partner decline, closing the proposal and leaving all other state untouched.
// The proposer's attempt stays spent, so they cannot retry.
func handleSurrenderDecline(state *GameState, action Action) (*GameState, error) {
	if state.Phase == PhasePaused {
		return nil, apperr.ErrGamePaused
	}
	if state.SurrenderProposerSeat == nil {
		return nil, apperr.ErrWrongPhase
	}

	seat := action.PlayerSeat
	if seat < 0 || seat > 3 {
		return nil, apperr.ErrBadRequest
	}

	proposer := *state.SurrenderProposerSeat
	partner := (proposer + 2) % 4
	if seat != partner {
		return nil, apperr.ErrInvalidAction
	}

	newState := cloneGameState(state)
	newState.SurrenderProposerSeat = nil
	return newState, nil
}
