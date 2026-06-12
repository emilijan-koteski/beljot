package match

import (
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
