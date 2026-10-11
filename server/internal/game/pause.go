package game

import "github.com/emilijan/beljot/server/internal/apperr"

// handlePause processes a pause action from PhasePlaying or PhaseBidding, one per player per game.
// Pauses stack, and the game stays paused until all are cleared.
func handlePause(state *GameState, action Action) (*GameState, error) {
	if state.Phase != PhasePlaying && state.Phase != PhaseBidding &&
		state.Phase != PhasePaused {
		return nil, apperr.ErrWrongPhase
	}

	seat := action.PlayerSeat
	if seat < 0 || seat > 3 {
		return nil, apperr.ErrBadRequest
	}

	if state.PauseUsed[seat] {
		return nil, apperr.ErrPauseExhausted
	}

	newState := cloneGameState(state)

	if state.Phase != PhasePaused {
		newState.PreviousPhase = state.Phase
	}

	newState.PausedPlayers[seat] = true
	newState.PauseUsed[seat] = true
	newState.Phase = PhasePaused

	return newState, nil
}

// handleOwnerUnpause lets the room owner clear all active pauses and resume the game.
func handleOwnerUnpause(state *GameState, action Action) (*GameState, error) {
	if state.Phase != PhasePaused {
		return nil, apperr.ErrNotPaused
	}

	if action.PlayerSeat != state.OwnerSeat {
		return nil, apperr.ErrNotRoomOwner
	}

	newState := cloneGameState(state)
	newState.PausedPlayers = [4]bool{}
	newState.Phase = newState.PreviousPhase
	newState.PreviousPhase = ""

	return newState, nil
}

// handleUnpause clears the acting player's own pause.
// The game resumes to PreviousPhase only when all pauses are cleared.
func handleUnpause(state *GameState, action Action) (*GameState, error) {
	if state.Phase != PhasePaused {
		return nil, apperr.ErrNotPaused
	}

	seat := action.PlayerSeat
	if seat < 0 || seat > 3 {
		return nil, apperr.ErrBadRequest
	}

	if !state.PausedPlayers[seat] {
		return nil, apperr.ErrNoActivePause
	}

	newState := cloneGameState(state)
	newState.PausedPlayers[seat] = false

	allClear := true
	for _, paused := range newState.PausedPlayers {
		if paused {
			allClear = false
			break
		}
	}

	if allClear {
		newState.Phase = newState.PreviousPhase
		newState.PreviousPhase = ""
	}

	return newState, nil
}
