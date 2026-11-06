package match

import (
	"time"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/ws"
)

const defaultPauseWindowSec = 120

// pauseWindowSec is how long a pause, and each extension of it, lasts: the match's reconnect window.
func pauseWindowSec(session *LiveMatch) int {
	if session.reconnectWindowSec > 0 {
		return session.reconnectWindowSec
	}
	return defaultPauseWindowSec
}

func pauseWindow(session *LiveMatch) time.Duration {
	return time.Duration(pauseWindowSec(session)) * time.Second
}

// armPauseTimerLocked schedules the end of the current pause at its absolute expiry.
// Must be called under session.mu.Lock().
func (m *Manager) armPauseTimerLocked(session *LiveMatch, gs *game.GameState) {
	if gs.PauseExpiresAt == nil {
		return
	}
	if session.turnTimer != nil {
		session.turnTimer.Stop()
	}
	gen := session.timerGeneration
	session.turnTimer = time.AfterFunc(max(time.Until(*gs.PauseExpiresAt), 0)+expiryGrace, func() {
		m.handlePauseTimeout(session, gen)
	})
}

// resumeTurnTimerLocked restarts the per-move clock after a pause, with at least three seconds left.
// Must be called under session.mu.Lock().
func (m *Manager) resumeTurnTimerLocked(session *LiveMatch, gs *game.GameState) {
	if session.timerStyle != "per-move" {
		gs.TurnTimeRemaining = 0
		return
	}
	const minResumeMs int64 = 3000
	remaining := time.Duration(max(gs.TurnTimeRemaining, minResumeMs)) * time.Millisecond
	expiry := time.Now().Add(remaining)
	gs.TurnExpiresAt = &expiry
	gs.TurnTimeRemaining = 0
	session.cancelTurnTimer()
	m.armTurnTimerLocked(session, remaining, gs.ActivePlayerSeat)
}

// handlePauseTimeout ends a pause whose time ran out, ignoring callbacks from stale timer generations.
func (m *Manager) handlePauseTimeout(session *LiveMatch, generation uint64) {
	session.mu.Lock()
	if session.closed || session.timerGeneration != generation {
		session.mu.Unlock()
		return
	}
	oldState := session.gameState
	if oldState.Phase != game.PhasePaused {
		session.mu.Unlock()
		return
	}
	newState, err := game.ExpirePause(oldState)
	if err != nil {
		session.mu.Unlock()
		return
	}
	m.resumeTurnTimerLocked(session, newState)
	session.gameState = newState
	playerIDs := session.playerIDs
	session.mu.Unlock()

	resumed := ws.MatchResumedPayload{ResumedBy: -1, TimedOut: true}
	m.hub.BroadcastToUsers(humanUserIDs(playerIDs), buildMessage(ws.EventMatchResumed, resumed))
	m.broadcastState(playerIDs, newState)
	m.maybeScheduleBotAction(session)
}

// broadcastPauseExtended tells the table who added time to the pause and when it now ends.
func (m *Manager) broadcastPauseExtended(userIDs []uint, gs *game.GameState, seat int) {
	if gs.PauseExpiresAt == nil {
		return
	}
	extended := ws.PauseExtendedPayload{
		ExtendedBy:     seat,
		PauseExpiresAt: gs.PauseExpiresAt.UTC().Format(time.RFC3339Nano),
	}
	m.hub.BroadcastToUsers(userIDs, buildMessage(ws.EventPauseExtended, extended))
}
