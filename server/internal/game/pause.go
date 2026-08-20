package game

import "github.com/emilijan/beljot/server/internal/apperr"

// handlePause processes a pause action from a player.
// Valid from PhasePlaying, PhaseBidding or PhaseDeclaring. Each player gets 1
// pause per game.
// Multiple players can pause (stacking); game stays paused until all clear.
func handlePause(state *GameState, action Action) (*GameState, error) {
	// Allow pause from playing, bidding, declaring, or already-paused (stacking).
	// PhaseDeclaring is a turn-taking phase with a live per-move timer just like
	// the other two, so a phase where pause silently failed would be a hole.
	if state.Phase != PhasePlaying && state.Phase != PhaseBidding &&
		state.Phase != PhaseDeclaring && state.Phase != PhasePaused {
		return nil, apperr.ErrWrongPhase
	}

	seat := action.PlayerSeat
	if seat < 0 || seat > 3 {
		return nil, apperr.ErrBadRequest
	}

	// Each player gets exactly 1 pause per game
	if state.PauseUsed[seat] {
		return nil, apperr.ErrPauseExhausted
	}

	newState := cloneGameState(state)

	// Only save PreviousPhase on the first pause (when not already paused via stacking)
	if state.Phase != PhasePaused {
		newState.PreviousPhase = state.Phase
	}

	newState.PausedPlayers[seat] = true
	newState.PauseUsed[seat] = true
	newState.Phase = PhasePaused

	return newState, nil
}

// handleOwnerUnpause processes an owner_unpause action from the room owner.
// Clears ALL active pauses regardless of who initiated them and resumes the game.
// Only the room owner (identified by OwnerSeat) may use this action.
func handleOwnerUnpause(state *GameState, action Action) (*GameState, error) {
	if state.Phase != PhasePaused {
		return nil, apperr.ErrNotPaused
	}

	if action.PlayerSeat != state.OwnerSeat {
		return nil, apperr.ErrNotRoomOwner
	}

	newState := cloneGameState(state)
	// Clear ALL active pauses
	newState.PausedPlayers = [4]bool{}
	newState.Phase = newState.PreviousPhase
	newState.PreviousPhase = ""

	return newState, nil
}

// handleUnpause processes an unpause action from a player.
// Only the player who has an active pause can unpause (clears their own pause).
// Game resumes to PreviousPhase only when ALL active pauses are cleared.
func handleUnpause(state *GameState, action Action) (*GameState, error) {
	if state.Phase != PhasePaused {
		return nil, apperr.ErrNotPaused
	}

	seat := action.PlayerSeat
	if seat < 0 || seat > 3 {
		return nil, apperr.ErrBadRequest
	}

	// Player must have an active pause to clear
	if !state.PausedPlayers[seat] {
		return nil, apperr.ErrNoActivePause
	}

	newState := cloneGameState(state)
	newState.PausedPlayers[seat] = false

	// Check if all pauses are cleared
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
