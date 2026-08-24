package game

import "github.com/emilijan/beljot/server/internal/apperr"

// ApplyAction is the pure function entry point for the rules engine.
// It takes the current game state and a player action, and returns
// a new game state (or an error if the action is invalid).
// No side effects — session manager handles broadcasting, persistence, timers.
//
// It is also the single exit every successful action passes through, which is
// where the state's DERIVED wire flags are refreshed. Doing it here rather than
// in each handler means a new handler cannot forget one.
func ApplyAction(state *GameState, action Action) (*GameState, error) {
	newState, err := applyAction(state, action)
	if err != nil {
		return nil, err
	}
	RefreshDerivedFlags(newState)
	return newState, nil
}

// RefreshDerivedFlags recomputes every DERIVED wire flag from the rest of the
// state. It is idempotent and pure.
//
// ApplyAction calls it at its single exit, so no rules-engine handler can forget
// one. It is EXPORTED because the session manager writes state.Phase directly in
// several places that bypass the engine entirely — the dealing→bidding
// auto-transition, the disconnect pause auto-clear, and the reconnect phase
// restore — and a derived flag computed under the previous phase would then ride
// match_state to the client. Every one of those writes must be followed by a
// call here.
//
// Concretely, that is not hypothetical: pause during a forced-pick bidding turn
// recomputes MustPickTrump to false under PhasePaused, and the reconnect path
// restores PhaseBidding without going near ApplyAction — so the client would be
// offered a Pass control the engine refuses.
func RefreshDerivedFlags(state *GameState) {
	state.MustPickTrump = MustPickTrump(state, state.ActivePlayerSeat)
	// Constant for a match's whole life, unlike MustPickTrump — mirrored here
	// anyway so the wire field has exactly one writer and cannot drift from the
	// config the engine reads.
	state.DeclarationsEnabled = state.Rules.DeclarationsEnabled
	// Same deal: constant per match, mirrored here so Rules.StopAtTarget is the
	// single source and the wire field has exactly one writer.
	state.StopAtTarget = state.Rules.StopAtTarget
}

func applyAction(state *GameState, action Action) (*GameState, error) {
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
