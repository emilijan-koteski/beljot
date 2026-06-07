package game

import "github.com/emilijan/beljot/server/internal/apperr"

// ApplyAction is the pure function entry point for the rules engine.
// It takes the current game state and a player action, and returns
// a new game state (or an error if the action is invalid).
// No side effects — session manager handles broadcasting, persistence, timers.
func ApplyAction(state *GameState, action Action) (*GameState, error) {
	// Disconnected phase blocks all actions — game is waiting for reconnection
	if state.Phase == PhaseDisconnected {
		return nil, apperr.ErrPlayerDisconnected
	}

	// Pause action is valid from playing, bidding, or already-paused (stacking)
	if action.Type == ActionPause {
		return handlePause(state, action)
	}

	// Unpause actions are only valid when paused — return ErrNotPaused otherwise
	if action.Type == ActionUnpause {
		return handleUnpause(state, action)
	}
	if action.Type == ActionOwnerUnpause {
		return handleOwnerUnpause(state, action)
	}

	// Surrender actions (Story 8.2) are matched at the same dispatch level as
	// pause/unpause so that accept/decline can resolve a pending proposal even
	// if the rules engine would otherwise reject the current phase. Each
	// handler enforces its own phase rule (request requires PhasePlaying or
	// PhaseBidding; accept/decline require a pending proposal).
	if action.Type == ActionSurrenderRequest {
		return handleSurrenderRequest(state, action)
	}
	if action.Type == ActionSurrenderAccept {
		return handleSurrenderAccept(state, action)
	}
	if action.Type == ActionSurrenderDecline {
		return handleSurrenderDecline(state, action)
	}

	// Continue acknowledges the hand-complete pause; handled at the dispatch
	// level because PhaseHandComplete is not one of the play/bid phases below.
	if action.Type == ActionContinue {
		return handleContinue(state, action)
	}

	switch state.Phase {
	case PhaseBidding:
		return handleBidding(state, action)
	case PhasePlaying:
		return handlePlaying(state, action)
	case PhaseMatchEnd:
		return nil, apperr.ErrWrongPhase
	case PhasePaused:
		return nil, apperr.ErrGamePaused
	default:
		return nil, apperr.ErrWrongPhase
	}
}
