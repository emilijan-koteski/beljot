package match

import (
	"time"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/ws"
)

const defaultPauseWindowSec = 120

// pauseWindowSec is how long each player's pause lasts: the match's reconnect window.
func pauseWindowSec(session *LiveMatch) int {
	if session.reconnectWindowSec > 0 {
		return session.reconnectWindowSec
	}
	return defaultPauseWindowSec
}

func pauseWindow(session *LiveMatch) time.Duration {
	return time.Duration(pauseWindowSec(session)) * time.Second
}

// earliestPauseExpiry returns the end of the running pause that runs out first, or nil when none is running.
func earliestPauseExpiry(gs *game.GameState) *time.Time {
	var earliest *time.Time
	for seat, held := range gs.PausedPlayers {
		expiry := gs.PlayerPauseExpiresAt[seat]
		if held && expiry != nil && (earliest == nil || expiry.Before(*earliest)) {
			earliest = expiry
		}
	}
	return earliest
}

// armPauseTimerLocked schedules the end of whichever running pause runs out first.
// Must be called under session.mu.Lock().
func (m *Manager) armPauseTimerLocked(session *LiveMatch, gs *game.GameState) {
	expiry := earliestPauseExpiry(gs)
	if expiry == nil {
		return
	}
	if session.turnTimer != nil {
		session.turnTimer.Stop()
	}
	gen := session.timerGeneration
	session.turnTimer = time.AfterFunc(max(time.Until(*expiry), 0)+expiryGrace, func() {
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

// expireRunOutPauses ends every running pause whose time has passed by now.
func expireRunOutPauses(gs *game.GameState, now time.Time) *game.GameState {
	for seat, held := range gs.PausedPlayers {
		expiry := gs.PlayerPauseExpiresAt[seat]
		if !held || expiry == nil || now.Before(*expiry) {
			continue
		}
		next, err := game.ExpireSeatPause(gs, seat)
		if err != nil {
			continue
		}
		gs = next
		if gs.Phase != game.PhasePaused {
			break
		}
	}
	return gs
}

// handlePauseTimeout ends the pauses that ran out, ignoring callbacks from stale timer generations.
// Play resumes only when no player's pause is left running.
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
	newState := expireRunOutPauses(oldState, time.Now())
	if newState == oldState {
		m.armPauseTimerLocked(session, oldState)
		session.mu.Unlock()
		return
	}
	resumed := newState.Phase != game.PhasePaused
	if resumed {
		m.resumeTurnTimerLocked(session, newState)
	} else {
		m.armPauseTimerLocked(session, newState)
	}
	session.gameState = newState
	playerIDs := session.playerIDs
	session.mu.Unlock()

	if resumed {
		payload := ws.MatchResumedPayload{ResumedBy: -1, TimedOut: true}
		m.hub.BroadcastToUsers(humanUserIDs(playerIDs), buildMessage(ws.EventMatchResumed, payload))
	}
	m.broadcastState(playerIDs, newState)
	if resumed {
		m.maybeScheduleBotAction(session)
	}
}
