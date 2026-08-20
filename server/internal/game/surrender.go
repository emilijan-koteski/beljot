package game

import "github.com/emilijan/beljot/server/internal/apperr"

// handleSurrenderRequest processes a player's request to surrender the match.
// Valid only from PhasePlaying, PhaseBidding or PhaseDeclaring. Each player may
// initiate at most one surrender request per match — the SurrenderUsed flag flips on
// request, regardless of whether the partner later accepts or declines.
// A proposal already pending is rejected with ErrActionRequired (must be
// resolved first). PhasePaused is rejected with ErrGamePaused (AC#1: pause
// rejection mirrors the existing rule — pause must be cleared first).
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

// handleSurrenderAccept processes the partner accepting a pending surrender.
// Only the proposer's partner (seat = (proposer + 2) % 4) may accept.
// Transitions the match into PhaseMatchEnd with the opposing team as winner
// and clears the per-move TurnExpiresAt / TurnTimeRemaining fields on the
// returned state (rules engine is pure — the actual *time.Timer goroutine is
// stopped by the session manager's preemptive cancelTurnTimer in HandleAction).
// SurrenderUsed[proposer] stays true — the attempt has been spent.
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
	opponentTeam := 1 - TeamForSeat(proposer)
	newState.WinnerTeam = &opponentTeam
	newState.Phase = PhaseMatchEnd
	newState.SurrenderProposerSeat = nil
	newState.TurnExpiresAt = nil
	newState.TurnTimeRemaining = 0

	return newState, nil
}

// handleSurrenderDecline processes the partner declining a pending surrender.
// Only the proposer's partner may decline. Clears SurrenderProposerSeat;
// SurrenderUsed[proposer] is preserved (declined attempts still count) so the
// proposer cannot retry. All other game state is untouched — phase, scores,
// hands, trick state continue exactly as before the proposal opened.
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
	// SurrenderUsed[proposer] stays true — attempt is consumed even on decline.
	return newState, nil
}
