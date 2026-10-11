package game_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
)

func TestDisconnectedPhaseRejectsAllActions(t *testing.T) {
	tests := []struct {
		name   string
		action game.Action
	}{
		{
			name:   "play card rejected during disconnect",
			action: game.Action{Type: game.ActionPlayCard, PlayerSeat: 0, Card: &game.Card{Rank: game.RankAce, Suit: game.SuitSpades}},
		},
		{
			name:   "pick trump rejected during disconnect",
			action: game.Action{Type: game.ActionPickTrump, PlayerSeat: 1},
		},
		{
			name:   "pass trump rejected during disconnect",
			action: game.Action{Type: game.ActionPassTrump, PlayerSeat: 1},
		},
		{
			name:   "pause rejected during disconnect",
			action: game.Action{Type: game.ActionPause, PlayerSeat: 0},
		},
		{
			name:   "unpause rejected during disconnect",
			action: game.Action{Type: game.ActionUnpause, PlayerSeat: 0},
		},
		{
			name:   "owner unpause rejected during disconnect",
			action: game.Action{Type: game.ActionOwnerUnpause, PlayerSeat: 0},
		},
		{
			name:   "declare rejected during disconnect",
			action: game.Action{Type: game.ActionDeclare, PlayerSeat: 1},
		},
		{
			name:   "skip declare rejected during disconnect",
			action: game.Action{Type: game.ActionSkipDeclare, PlayerSeat: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gs := testfixtures.NewGameDisconnected(2)
			result, err := game.ApplyAction(gs, tt.action)
			assert.Nil(t, result)
			assert.ErrorIs(t, err, apperr.ErrPlayerDisconnected)
		})
	}
}

func TestNewGameDisconnectedFixture(t *testing.T) {
	gs := testfixtures.NewGameDisconnected(1)

	assert.Equal(t, game.PhaseDisconnected, gs.Phase)
	assert.Equal(t, game.PhasePlaying, gs.PreviousPhase)
	assert.Equal(t, 1, gs.DisconnectedSeat)
	assert.False(t, gs.Players[1].Connected)

	assert.True(t, gs.Players[0].Connected)
	assert.True(t, gs.Players[2].Connected)
	assert.True(t, gs.Players[3].Connected)
}
