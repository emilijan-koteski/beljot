package game_test

import (
	"errors"
	"testing"
	"time"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPause(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() *game.GameState
		action      game.Action
		wantErr     *apperr.AppError
		assertState func(t *testing.T, gs *game.GameState)
	}{
		{
			name: "pause from playing phase succeeds",
			setup: func() *game.GameState {
				return testfixtures.NewGameMidPlay(1)
			},
			action: game.Action{Type: game.ActionPause, PlayerSeat: 0},
			assertState: func(t *testing.T, gs *game.GameState) {
				assert.Equal(t, game.PhasePaused, gs.Phase)
				assert.Equal(t, game.PhasePlaying, gs.PreviousPhase)
				assert.True(t, gs.PausedPlayers[0])
				assert.True(t, gs.PauseUsed[0])
			},
		},
		{
			name: "pause from bidding phase succeeds",
			setup: func() *game.GameState {
				return testfixtures.NewGameJustDealt()
			},
			action: game.Action{Type: game.ActionPause, PlayerSeat: 1},
			assertState: func(t *testing.T, gs *game.GameState) {
				assert.Equal(t, game.PhasePaused, gs.Phase)
				assert.Equal(t, game.PhaseBidding, gs.PreviousPhase)
				assert.True(t, gs.PausedPlayers[1])
				assert.True(t, gs.PauseUsed[1])
			},
		},
		{
			name: "pause from match_end returns ErrWrongPhase",
			setup: func() *game.GameState {
				gs := testfixtures.NewGameMidPlay(1)
				gs.Phase = game.PhaseMatchEnd
				return gs
			},
			action:  game.Action{Type: game.ActionPause, PlayerSeat: 0},
			wantErr: apperr.ErrWrongPhase,
		},
		{
			name: "pause from dealing returns ErrWrongPhase",
			setup: func() *game.GameState {
				gs := testfixtures.NewGameMidPlay(1)
				gs.Phase = game.PhaseDealing
				return gs
			},
			action:  game.Action{Type: game.ActionPause, PlayerSeat: 0},
			wantErr: apperr.ErrWrongPhase,
		},
		{
			name: "double pause by same player returns ErrPauseExhausted",
			setup: func() *game.GameState {
				gs := testfixtures.NewGameMidPlay(1)
				gs.PauseUsed[2] = true
				return gs
			},
			action:  game.Action{Type: game.ActionPause, PlayerSeat: 2},
			wantErr: apperr.ErrPauseExhausted,
		},
		{
			name: "pause stacking - second player pauses when first already paused",
			setup: func() *game.GameState {
				return testfixtures.NewGamePaused(0)
			},
			action: game.Action{Type: game.ActionPause, PlayerSeat: 1},
			assertState: func(t *testing.T, gs *game.GameState) {
				assert.Equal(t, game.PhasePaused, gs.Phase)
				assert.Equal(t, game.PhasePlaying, gs.PreviousPhase, "PreviousPhase should not change on stacked pause")
				assert.True(t, gs.PausedPlayers[0])
				assert.True(t, gs.PausedPlayers[1])
				assert.True(t, gs.PauseUsed[0])
				assert.True(t, gs.PauseUsed[1])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := tt.setup()
			newState, err := game.ApplyAction(state, tt.action)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr), "expected %v, got %v", tt.wantErr, err)
				assert.Nil(t, newState)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, newState)
			if tt.assertState != nil {
				tt.assertState(t, newState)
			}
		})
	}
}

func TestUnpause(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() *game.GameState
		action      game.Action
		wantErr     *apperr.AppError
		assertState func(t *testing.T, gs *game.GameState)
	}{
		{
			name: "unpause single pause resumes to previous phase",
			setup: func() *game.GameState {
				return testfixtures.NewGamePaused(0)
			},
			action: game.Action{Type: game.ActionUnpause, PlayerSeat: 0},
			assertState: func(t *testing.T, gs *game.GameState) {
				assert.Equal(t, game.PhasePlaying, gs.Phase, "should resume to playing")
				assert.Equal(t, game.Phase(""), gs.PreviousPhase, "PreviousPhase should be cleared")
				assert.False(t, gs.PausedPlayers[0], "pause should be cleared")
			},
		},
		{
			name: "unpause stacked - first unpause keeps game paused",
			setup: func() *game.GameState {
				gs := testfixtures.NewGamePaused(0)
				gs.PausedPlayers[1] = true
				gs.PauseUsed[1] = true
				return gs
			},
			action: game.Action{Type: game.ActionUnpause, PlayerSeat: 0},
			assertState: func(t *testing.T, gs *game.GameState) {
				assert.Equal(t, game.PhasePaused, gs.Phase, "should remain paused")
				assert.False(t, gs.PausedPlayers[0], "seat 0 pause cleared")
				assert.True(t, gs.PausedPlayers[1], "seat 1 pause still active")
			},
		},
		{
			name: "unpause stacked - second unpause resumes game",
			setup: func() *game.GameState {
				gs := testfixtures.NewGamePaused(0)
				gs.PausedPlayers[1] = true
				gs.PauseUsed[1] = true
				gs.PausedPlayers[0] = false
				return gs
			},
			action: game.Action{Type: game.ActionUnpause, PlayerSeat: 1},
			assertState: func(t *testing.T, gs *game.GameState) {
				assert.Equal(t, game.PhasePlaying, gs.Phase, "should resume to playing")
				assert.False(t, gs.PausedPlayers[0])
				assert.False(t, gs.PausedPlayers[1])
			},
		},
		{
			name: "unpause wrong player returns ErrNoActivePause",
			setup: func() *game.GameState {
				return testfixtures.NewGamePaused(0)
			},
			action:  game.Action{Type: game.ActionUnpause, PlayerSeat: 2},
			wantErr: apperr.ErrNoActivePause,
		},
		{
			name: "unpause when not paused returns ErrNotPaused",
			setup: func() *game.GameState {
				return testfixtures.NewGameMidPlay(1)
			},
			action:  game.Action{Type: game.ActionUnpause, PlayerSeat: 0},
			wantErr: apperr.ErrNotPaused,
		},
		{
			name: "the host cannot resume a pause they do not hold",
			setup: func() *game.GameState {
				return testfixtures.NewGamePausedWithOwner(1, 0)
			},
			action:  game.Action{Type: game.ActionUnpause, PlayerSeat: 0},
			wantErr: apperr.ErrNoActivePause,
		},
		{
			name: "the retired owner_unpause action is refused",
			setup: func() *game.GameState {
				return testfixtures.NewGamePausedWithOwner(1, 0)
			},
			action:  game.Action{Type: "owner_unpause", PlayerSeat: 0},
			wantErr: apperr.ErrGamePaused,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := tt.setup()
			newState, err := game.ApplyAction(state, tt.action)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr), "expected %v, got %v", tt.wantErr, err)
				assert.Nil(t, newState)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, newState)
			if tt.assertState != nil {
				tt.assertState(t, newState)
			}
		})
	}
}

func TestExtendPause(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() *game.GameState
		action      game.Action
		wantErr     *apperr.AppError
		assertState func(t *testing.T, gs *game.GameState)
	}{
		{
			name: "adding time spends the player's one pause and makes them a holder",
			setup: func() *game.GameState {
				return testfixtures.NewGamePaused(0)
			},
			action: game.Action{Type: game.ActionExtendPause, PlayerSeat: 2},
			assertState: func(t *testing.T, gs *game.GameState) {
				assert.Equal(t, game.PhasePaused, gs.Phase)
				assert.True(t, gs.PauseExtended[2])
				assert.True(t, gs.PauseUsed[2], "adding time burns the player's pause")
				assert.True(t, gs.PausedPlayers[2], "the extender holds the pause too")
				assert.True(t, gs.PausedPlayers[0])
			},
		},
		{
			name: "the player who paused cannot add time, having spent their pause",
			setup: func() *game.GameState {
				return testfixtures.NewGamePaused(0)
			},
			action:  game.Action{Type: game.ActionExtendPause, PlayerSeat: 0},
			wantErr: apperr.ErrPauseExhausted,
		},
		{
			name: "a second extension by the same player is refused",
			setup: func() *game.GameState {
				gs := testfixtures.NewGamePaused(0)
				gs.PauseExtended[1] = true
				gs.PauseUsed[1] = true
				gs.PausedPlayers[1] = true
				return gs
			},
			action:  game.Action{Type: game.ActionExtendPause, PlayerSeat: 1},
			wantErr: apperr.ErrPauseExhausted,
		},
		{
			name: "a player who paused earlier in the match cannot add time later",
			setup: func() *game.GameState {
				gs := testfixtures.NewGamePaused(0)
				gs.PauseUsed[3] = true
				return gs
			},
			action:  game.Action{Type: game.ActionExtendPause, PlayerSeat: 3},
			wantErr: apperr.ErrPauseExhausted,
		},
		{
			name: "a pause sent while already paused adds time like an extension",
			setup: func() *game.GameState {
				return testfixtures.NewGamePaused(0)
			},
			action: game.Action{Type: game.ActionPause, PlayerSeat: 1},
			assertState: func(t *testing.T, gs *game.GameState) {
				assert.True(t, gs.PauseExtended[1])
				assert.True(t, gs.PauseUsed[1])
				assert.True(t, gs.PausedPlayers[1])
			},
		},
		{
			name: "extending outside a pause is refused",
			setup: func() *game.GameState {
				return testfixtures.NewGameMidPlay(1)
			},
			action:  game.Action{Type: game.ActionExtendPause, PlayerSeat: 0},
			wantErr: apperr.ErrNotPaused,
		},
		{
			name: "extending does not resume when the pauser releases first",
			setup: func() *game.GameState {
				gs := testfixtures.NewGamePaused(0)
				gs.PauseExtended[3] = true
				gs.PauseUsed[3] = true
				gs.PausedPlayers[3] = true
				return gs
			},
			action: game.Action{Type: game.ActionUnpause, PlayerSeat: 0},
			assertState: func(t *testing.T, gs *game.GameState) {
				assert.Equal(t, game.PhasePaused, gs.Phase, "seat 3 still holds the pause it extended")
				assert.False(t, gs.PausedPlayers[0])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := tt.setup()
			newState, err := game.ApplyAction(state, tt.action)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr), "expected %v, got %v", tt.wantErr, err)
				assert.Nil(t, newState)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, newState)
			if tt.assertState != nil {
				tt.assertState(t, newState)
			}
		})
	}
}

func TestNewPauseStartsWithNoExtensions(t *testing.T) {
	state := testfixtures.NewGameMidPlay(1)
	state.PauseExtended = [4]bool{true, false, true, false}

	newState, err := game.ApplyAction(state, game.Action{Type: game.ActionPause, PlayerSeat: 3})

	require.NoError(t, err)
	assert.Equal(t, [4]bool{}, newState.PauseExtended)
}

func TestExpirePause(t *testing.T) {
	state := testfixtures.NewGamePaused(0)
	state.PausedPlayers[2] = true
	state.PauseExtended[2] = true
	expiry := time.Now()
	state.PauseExpiresAt = &expiry
	state.PauseTotalSec = 240

	newState, err := game.ExpirePause(state)

	require.NoError(t, err)
	assert.Equal(t, game.PhasePlaying, newState.Phase)
	assert.Equal(t, game.Phase(""), newState.PreviousPhase)
	assert.Equal(t, [4]bool{}, newState.PausedPlayers, "every hold ends with the time")
	assert.Equal(t, [4]bool{}, newState.PauseExtended)
	assert.Nil(t, newState.PauseExpiresAt)
	assert.Zero(t, newState.PauseTotalSec)
	assert.True(t, newState.PauseUsed[0], "the pause stays used for the rest of the match")
	assert.Equal(t, game.PhasePaused, state.Phase, "the original state is not mutated")
}

func TestExpirePauseOutsideAPauseIsRefused(t *testing.T) {
	_, err := game.ExpirePause(testfixtures.NewGameMidPlay(1))
	assert.ErrorIs(t, err, apperr.ErrNotPaused)
}

func TestLastUnpauseClearsThePauseClock(t *testing.T) {
	state := testfixtures.NewGamePaused(0)
	expiry := time.Now().Add(time.Minute)
	state.PauseExpiresAt = &expiry
	state.PauseTotalSec = 120

	newState, err := game.ApplyAction(state, game.Action{Type: game.ActionUnpause, PlayerSeat: 0})

	require.NoError(t, err)
	assert.Equal(t, game.PhasePlaying, newState.Phase)
	assert.Nil(t, newState.PauseExpiresAt)
	assert.Zero(t, newState.PauseTotalSec)
}

func TestActionsBlockedWhilePaused(t *testing.T) {
	tests := []struct {
		name   string
		action game.Action
	}{
		{name: "play_card blocked", action: game.Action{Type: game.ActionPlayCard, PlayerSeat: 0}},
		{name: "pick_trump blocked", action: game.Action{Type: game.ActionPickTrump, PlayerSeat: 0}},
		{name: "pass_trump blocked", action: game.Action{Type: game.ActionPassTrump, PlayerSeat: 0}},
		{name: "declare blocked", action: game.Action{Type: game.ActionDeclare, PlayerSeat: 0}},
		{name: "skip_declare blocked", action: game.Action{Type: game.ActionSkipDeclare, PlayerSeat: 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := testfixtures.NewGamePaused(0)
			newState, err := game.ApplyAction(state, tt.action)
			require.Error(t, err)
			assert.True(t, errors.Is(err, apperr.ErrGamePaused))
			assert.Nil(t, newState)
		})
	}
}

func TestUnpauseAllowedWhilePaused(t *testing.T) {
	state := testfixtures.NewGamePaused(0)
	newState, err := game.ApplyAction(state, game.Action{Type: game.ActionUnpause, PlayerSeat: 0})
	require.NoError(t, err)
	assert.Equal(t, game.PhasePlaying, newState.Phase)
}

func TestPauseDoesNotMutateOriginalState(t *testing.T) {
	state := testfixtures.NewGameMidPlay(1)
	originalPhase := state.Phase

	_, err := game.ApplyAction(state, game.Action{Type: game.ActionPause, PlayerSeat: 0})
	require.NoError(t, err)

	assert.Equal(t, originalPhase, state.Phase, "original state should not be mutated")
	assert.False(t, state.PausedPlayers[0], "original PausedPlayers should not be mutated")
	assert.False(t, state.PauseUsed[0], "original PauseUsed should not be mutated")
}
