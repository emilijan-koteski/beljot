package game

import "github.com/emilijan/beljot/server/internal/apperr"

// ApplyAction applies a player action and returns the new state, with no side effects.
// Every successful action refreshes the derived wire flags on the way out.
func ApplyAction(state *GameState, action Action) (*GameState, error) {
	newState, err := applyAction(state, action)
	if err != nil {
		return nil, err
	}
	RefreshDerivedFlags(newState)
	return newState, nil
}

// RefreshDerivedFlags recomputes every derived wire flag from the rest of the state.
// Call it after any direct write to state.Phase that bypasses ApplyAction.
func RefreshDerivedFlags(state *GameState) {
	state.MustPickTrump = MustPickTrump(state, state.ActivePlayerSeat)
	state.DeclarationsEnabled = state.Rules.DeclarationsEnabled
	state.StopAtTarget = state.Rules.StopAtTarget
}

func applyAction(state *GameState, action Action) (*GameState, error) {
	if state.Phase == PhaseDisconnected {
		return nil, apperr.ErrPlayerDisconnected
	}

	if action.Type == ActionPause {
		return handlePause(state, action)
	}

	if action.Type == ActionUnpause {
		return handleUnpause(state, action)
	}
	if action.Type == ActionOwnerUnpause {
		return handleOwnerUnpause(state, action)
	}

	if action.Type == ActionSurrenderRequest {
		return handleSurrenderRequest(state, action)
	}
	if action.Type == ActionSurrenderAccept {
		return handleSurrenderAccept(state, action)
	}
	if action.Type == ActionSurrenderDecline {
		return handleSurrenderDecline(state, action)
	}

	if action.Type == ActionContinue {
		return handleContinue(state, action)
	}

	switch state.Phase {
	case PhaseBidding:
		return handleBidding(state, action)
	case PhaseDeclaring:
		return handleDeclaring(state, action)
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
