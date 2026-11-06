package game

import (
	"time"

	"github.com/emilijan/beljot/server/internal/apperr"
)

// handlePause starts the player's own pause, one per match. A pause sent while already paused runs
// alongside the others on its own clock; the match stays paused while any player's pause is running.
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

// handleUnpause ends the acting player's own pause; nobody can end another player's.
// The game resumes to PreviousPhase only when no player's pause is still running.
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

	return endSeatPause(state, seat), nil
}

// ExpireSeatPause ends one player's pause whose time ran out, resuming play if it was the last one running.
// The session layer calls it from the pause timer; it is not an action a player can send.
func ExpireSeatPause(state *GameState, seat int) (*GameState, error) {
	if state.Phase != PhasePaused {
		return nil, apperr.ErrNotPaused
	}
	if seat < 0 || seat > 3 {
		return nil, apperr.ErrBadRequest
	}
	if !state.PausedPlayers[seat] {
		return nil, apperr.ErrNoActivePause
	}
	return endSeatPause(state, seat), nil
}

// ClearPause drops every running pause without touching the phase, for pauses that ended while the match
// was waiting on a disconnected player.
func ClearPause(state *GameState) {
	state.PausedPlayers = [4]bool{}
	state.PlayerPauseExpiresAt = [4]*time.Time{}
}

func endSeatPause(state *GameState, seat int) *GameState {
	newState := cloneGameState(state)
	newState.PausedPlayers[seat] = false
	newState.PlayerPauseExpiresAt[seat] = nil
	if !anyPauseHolder(newState.PausedPlayers) {
		ClearPause(newState)
		newState.Phase = newState.PreviousPhase
		newState.PreviousPhase = ""
	}
	return newState
}

func anyPauseHolder(holders [4]bool) bool {
	for _, held := range holders {
		if held {
			return true
		}
	}
	return false
}
