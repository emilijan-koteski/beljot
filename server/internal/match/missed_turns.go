package match

import (
	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/ws"
)

const missedTurnsForfeit = 3

var turnMoves = map[string]bool{
	game.ActionPlayCard:      true,
	game.ActionPickTrump:     true,
	game.ActionPassTrump:     true,
	game.ActionDeclare:       true,
	game.ActionSkipDeclare:   true,
	game.ActionAnnounceBelot: true,
	game.ActionSkipBelot:     true,
}

// missTurn counts a per-move timer expiry against a human seat and reports whether it is
// the one that forfeits the match. Call under session.mu.
func (s *LiveMatch) missTurn(seat int) bool {
	if seat < 0 || seat >= 4 || s.gameState.Players[seat].IsBot {
		return false
	}
	s.missedTurns[seat]++
	return s.missedTurns[seat] >= missedTurnsForfeit
}

// onLastWarning reports whether the seat's next missed turn forfeits the match. Call under session.mu.
func (s *LiveMatch) onLastWarning(seat int) bool {
	return seat >= 0 && seat < 4 && s.missedTurns[seat] == missedTurnsForfeit-1
}

// sendMissedTurnWarning tells only that player their next missed turn forfeits the match.
func (m *Manager) sendMissedTurnWarning(userID uint, missedTurns int) {
	m.hub.SendToUser(userID, buildMessage(ws.EventMissedTurnWarning, ws.MissedTurnWarningPayload{MissedTurns: missedTurns}))
}

// resetMissedTurns clears the acting seat's run of missed turns when it makes a move itself.
// Call under session.mu.
func (s *LiveMatch) resetMissedTurns(action game.Action) {
	if action.PlayerSeat < 0 || action.PlayerSeat >= 4 || !turnMoves[action.Type] {
		return
	}
	s.missedTurns[action.PlayerSeat] = 0
}
