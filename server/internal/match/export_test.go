package match

import (
	"errors"
	"time"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/ws"
)

// SetBotDelayForTest overrides the bot think-delay bounds so manager tests
// with bots don't sleep for real (Story 10.3).
func (m *Manager) SetBotDelayForTest(minDelay, maxDelay time.Duration) {
	m.botDelayMin = minDelay
	m.botDelayMax = maxDelay
}

// BotSchedule exposes maybeScheduleBotAction for tests that inject a game
// state via SetGameStateForTest and need the driver to re-evaluate it.
func (m *Manager) BotSchedule(roomID uint) {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return
	}
	m.maybeScheduleBotAction(lm)
}

// HandleMatchEndForTest exposes handleMatchEnd for tests in the external
// session_test package. Used by Story 8.5-1 AC4 tests to assert the
// persist-before-broadcast invariant directly without driving a full game to
// match completion.
func (m *Manager) HandleMatchEndForTest(roomID uint, finalState *game.GameState, surrenderedBy *uint, payload ws.MatchEndPayload) {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return
	}
	m.handleMatchEnd(lm, finalState, surrenderedBy, payload)
}

// AutoActionTypeFor exposes autoActionTypeFor for tests in the external
// session_test package. The wire-format mapping is the contract surface;
// keep it tested independently so future refactors don't silently drift.
func AutoActionTypeFor(actionType string) (ws.AutoActionType, bool) {
	return autoActionTypeFor(actionType)
}

// AbandonSeatForTest drives the abandonment finalize path
// (handleSeatReconnectTimeout) deterministically for Story 9.5 tests: it marks
// the seat disconnected and reads the seat's current reconnect generation so the
// staleness guard passes, then invokes the timeout handler directly — letting a
// test assert the abandonment XP award + event ordering without waiting on a
// real reconnect-window timer.
func (m *Manager) AbandonSeatForTest(roomID uint, seat int) {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return
	}
	lm.mu.Lock()
	lm.gameState.Players[seat].Connected = false
	gen := lm.seatReconnectGenerations[seat]
	lm.mu.Unlock()
	m.handleSeatReconnectTimeout(lm, seat, gen)
}

// BufferHandResultIfScored exposes bufferHandResultIfScored for tests in the
// external session_test package.
func (m *Manager) BufferHandResultIfScored(roomID uint, oldState, newState *game.GameState) {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return
	}
	m.bufferHandResultIfScored(lm, oldState, newState)
}

// HandResults returns a copy of the lm's buffered hand results for tests.
func (m *Manager) HandResults(roomID uint) []HandResult {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return nil
	}
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	out := make([]HandResult, len(lm.handResults))
	copy(out, lm.handResults)
	return out
}

// ApplyActionForTest drives applyAndBroadcastAction for one action, so tests can
// assert what a single successful action puts on the wire without going through
// a ws.Client. Tests using this helper must call StartMatch first.
func (m *Manager) ApplyActionForTest(roomID uint, action game.Action) error {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return errNoSession
	}
	return m.applyAndBroadcastAction(lm, action)
}

// SetGameStateForTest replaces the lm's game state. Used to drive
// HandleAction through specific mid-game states (declaration prompt, belot
// prompt) without scripting an entire match. Tests using this helper must
// call StartMatch first to register the lm and set timer config.
func (m *Manager) SetGameStateForTest(roomID uint, gs *game.GameState) {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return
	}
	lm.mu.Lock()
	lm.gameState = gs
	lm.mu.Unlock()
}

// HandCompleteExpiresAtForTest returns the session's fixed score-reveal
// auto-continue deadline (zero if unset). Exposed so tests can assert that a
// player's continue acknowledgement never pushes the deadline back.
func (m *Manager) HandCompleteExpiresAtForTest(roomID uint) time.Time {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return time.Time{}
	}
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.handCompleteExpiresAt
}

// SetHandCompleteExpiresAtForTest seeds the score-reveal auto-continue deadline,
// simulating a pause that has already started (in real flow the deadline is set
// when the hand-complete transition first occurs).
func (m *Manager) SetHandCompleteExpiresAtForTest(roomID uint, t time.Time) {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return
	}
	lm.mu.Lock()
	lm.handCompleteExpiresAt = t
	lm.mu.Unlock()
}

// DeclarationExpiresAtForTest returns the session's fixed declaration-phase
// deadline (zero if unset). Exposed so tests can assert that one seat's answer
// never pushes the window back for the others.
func (m *Manager) DeclarationExpiresAtForTest(roomID uint) time.Time {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return time.Time{}
	}
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.declarationExpiresAt
}

// SetDeclarationExpiresAtForTest seeds the declaration-phase deadline,
// simulating a phase that has already been entered (in real flow the deadline is
// set when the transition into PhaseDeclaring first occurs).
func (m *Manager) SetDeclarationExpiresAtForTest(roomID uint, t time.Time) {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return
	}
	lm.mu.Lock()
	lm.declarationExpiresAt = t
	lm.mu.Unlock()
}

// TriggerDeclarationTimeoutForTest cancels any pending timer and re-arms a
// short-duration one against handleDeclarationTimeout, then returns. Drives the
// window-elapsed path without waiting out declarationAutoClose. Mirrors
// TriggerTimerExpiryForTest for the phase that has no per-move timer.
func (m *Manager) TriggerDeclarationTimeoutForTest(roomID uint, fireAfter time.Duration) {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return
	}
	lm.mu.Lock()
	lm.cancelTurnTimer()
	gen := lm.timerGeneration
	lm.turnTimer = time.AfterFunc(fireAfter, func() {
		m.handleDeclarationTimeout(lm, gen)
	})
	lm.mu.Unlock()
}

// TriggerTimerExpiryForTest cancels any pending turn timer and re-arms a
// short-duration timer for the given expectedSeat, then waits for it to fire.
// Used by tests that drive the auto-action code path on an injected game state
// where the StartMatch-captured expectedSeat would not match the injected
// ActivePlayerSeat. The caller should sleep until the auto-action settles
// before snapshotting state.
func (m *Manager) TriggerTimerExpiryForTest(roomID uint, expectedSeat int, fireAfter time.Duration) {
	m.mu.RLock()
	lm, ok := m.sessions[roomID]
	m.mu.RUnlock()
	if !ok {
		return
	}
	lm.mu.Lock()
	lm.cancelTurnTimer()
	gen := lm.timerGeneration
	lm.turnTimer = time.AfterFunc(fireAfter, func() {
		m.handleTimerExpiry(lm, gen, expectedSeat)
	})
	lm.mu.Unlock()
}

// errNoSession is returned by the *ForTest helpers when the room has no live
// session — a test-setup mistake, surfaced rather than silently no-oping.
var errNoSession = errors.New("match: no live session for room")
