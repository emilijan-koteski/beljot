package game

import "github.com/emilijan/beljot/server/internal/apperr"

// handlePause processes a pause action, one per player per match. A pause from play or bidding starts a
// fresh pause with no extensions; a pause sent while already paused only adds the seat as a holder.
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
		newState.PauseExtended = [4]bool{}
	}

	newState.PausedPlayers[seat] = true
	newState.PauseUsed[seat] = true
	newState.Phase = PhasePaused

	return newState, nil
}

// handleExtendPause lets each player add time to the current pause once, which also makes them a holder:
// the pause then lasts until every holder resumes or the time runs out.
func handleExtendPause(state *GameState, action Action) (*GameState, error) {
	if state.Phase != PhasePaused {
		return nil, apperr.ErrNotPaused
	}

	seat := action.PlayerSeat
	if seat < 0 || seat > 3 {
		return nil, apperr.ErrBadRequest
	}

	if state.PauseExtended[seat] {
		return nil, apperr.ErrPauseExtendUsed
	}

	newState := cloneGameState(state)
	newState.PauseExtended[seat] = true
	newState.PausedPlayers[seat] = true

	return newState, nil
}

// handleUnpause clears the acting player's own hold on the pause.
// The game resumes to PreviousPhase only when no holder is left.
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

	if !anyPauseHolder(newState.PausedPlayers) {
		resumeFromPause(newState)
	}

	return newState, nil
}

// ExpirePause ends a pause whose time has run out, whoever still holds it.
// The session layer calls it from the pause timer; it is not an action a player can send.
func ExpirePause(state *GameState) (*GameState, error) {
	if state.Phase != PhasePaused {
		return nil, apperr.ErrNotPaused
	}
	newState := cloneGameState(state)
	resumeFromPause(newState)
	return newState, nil
}

// ClearPause drops every hold and the pause clock without touching the phase, for a pause that ran out
// while the match was waiting on a disconnected player.
func ClearPause(state *GameState) {
	state.PausedPlayers = [4]bool{}
	state.PauseExtended = [4]bool{}
	state.PauseExpiresAt = nil
	state.PauseTotalSec = 0
}

func resumeFromPause(state *GameState) {
	ClearPause(state)
	state.Phase = state.PreviousPhase
	state.PreviousPhase = ""
}

func anyPauseHolder(holders [4]bool) bool {
	for _, held := range holders {
		if held {
			return true
		}
	}
	return false
}
