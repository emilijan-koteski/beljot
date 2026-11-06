package match

import "github.com/emilijan/beljot/server/internal/game"

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

// resetMissedTurns clears the acting seat's run of missed turns when it makes a move itself.
// Call under session.mu.
func (s *LiveMatch) resetMissedTurns(action game.Action) {
	if action.PlayerSeat < 0 || action.PlayerSeat >= 4 || !turnMoves[action.Type] {
		return
	}
	s.missedTurns[action.PlayerSeat] = 0
}
