package game

import "github.com/emilijan/beljot/server/internal/apperr"

// handleContinue records a player's continue during PhaseHandComplete.
// The next hand is dealt once every connected player has continued.
func handleContinue(state *GameState, action Action) (*GameState, error) {
	if state.Phase != PhaseHandComplete {
		return nil, apperr.ErrWrongPhase
	}
	if action.PlayerSeat < 0 || action.PlayerSeat > 3 {
		return nil, apperr.ErrNotYourTurn
	}

	newState := cloneGameState(state)
	newState.HandCompleteReady[action.PlayerSeat] = true
	if allConnectedReady(newState) {
		startNewHand(newState)
	}
	return newState, nil
}

// allConnectedReady reports whether every connected player has acknowledged the
// hand-complete pause.
func allConnectedReady(state *GameState) bool {
	for i := range state.Players {
		if state.Players[i].Connected && !state.HandCompleteReady[i] {
			return false
		}
	}
	return true
}

// ForceAdvanceHandComplete deals the next hand regardless of ready state. The
// session manager calls it when the hand-complete auto-continue timeout fires,
// so a connected-but-idle player cannot stall the table indefinitely.
func ForceAdvanceHandComplete(state *GameState) (*GameState, error) {
	if state.Phase != PhaseHandComplete {
		return nil, apperr.ErrWrongPhase
	}
	newState := cloneGameState(state)
	startNewHand(newState)
	return newState, nil
}
