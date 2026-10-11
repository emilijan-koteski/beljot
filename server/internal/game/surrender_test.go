package game_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSurrenderRequest(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() *game.GameState
		action      game.Action
		wantErr     *apperr.AppError
		assertState func(t *testing.T, gs *game.GameState)
	}{
		{
			name: "request from PhasePlaying succeeds",
			setup: func() *game.GameState {
				return testfixtures.NewGameMidPlay(1)
			},
			action: game.Action{Type: game.ActionSurrenderRequest, PlayerSeat: 0},
			assertState: func(t *testing.T, gs *game.GameState) {
				require.NotNil(t, gs.SurrenderProposerSeat)
				assert.Equal(t, 0, *gs.SurrenderProposerSeat)
				assert.True(t, gs.SurrenderUsed[0])
				assert.Equal(t, game.PhasePlaying, gs.Phase, "phase unchanged on request")
			},
		},
		{
			name: "request from PhaseBidding succeeds",
			setup: func() *game.GameState {
				return testfixtures.NewGameJustDealt()
			},
			action: game.Action{Type: game.ActionSurrenderRequest, PlayerSeat: 1},
			assertState: func(t *testing.T, gs *game.GameState) {
				require.NotNil(t, gs.SurrenderProposerSeat)
				assert.Equal(t, 1, *gs.SurrenderProposerSeat)
				assert.True(t, gs.SurrenderUsed[1])
				assert.Equal(t, game.PhaseBidding, gs.Phase)
			},
		},
		{
			name: "request from PhaseDealing returns ErrWrongPhase",
			setup: func() *game.GameState {
				gs := testfixtures.NewGameMidPlay(1)
				gs.Phase = game.PhaseDealing
				return gs
			},
			action:  game.Action{Type: game.ActionSurrenderRequest, PlayerSeat: 0},
			wantErr: apperr.ErrWrongPhase,
		},
		{
			name: "request from PhaseHandScoring returns ErrWrongPhase",
			setup: func() *game.GameState {
				gs := testfixtures.NewGameMidPlay(1)
				gs.Phase = game.PhaseHandScoring
				return gs
			},
			action:  game.Action{Type: game.ActionSurrenderRequest, PlayerSeat: 0},
			wantErr: apperr.ErrWrongPhase,
		},
		{
			name: "request from PhaseTrickResolving returns ErrWrongPhase",
			setup: func() *game.GameState {
				gs := testfixtures.NewGameMidPlay(1)
				gs.Phase = game.PhaseTrickResolving
				return gs
			},
			action:  game.Action{Type: game.ActionSurrenderRequest, PlayerSeat: 0},
			wantErr: apperr.ErrWrongPhase,
		},
		{
			name: "request from PhaseMatchEnd returns ErrWrongPhase",
			setup: func() *game.GameState {
				gs := testfixtures.NewGameMidPlay(1)
				gs.Phase = game.PhaseMatchEnd
				return gs
			},
			action:  game.Action{Type: game.ActionSurrenderRequest, PlayerSeat: 0},
			wantErr: apperr.ErrWrongPhase,
		},
		{
			name: "request when SurrenderUsed[seat] is true returns ErrSurrenderExhausted",
			setup: func() *game.GameState {
				gs := testfixtures.NewGameMidPlay(1)
				gs.SurrenderUsed[0] = true
				return gs
			},
			action:  game.Action{Type: game.ActionSurrenderRequest, PlayerSeat: 0},
			wantErr: apperr.ErrSurrenderExhausted,
		},
		{
			name: "request when proposal already pending returns ErrActionRequired",
			setup: func() *game.GameState {
				gs := testfixtures.NewGameMidPlay(1)
				existing := 0
				gs.SurrenderProposerSeat = &existing
				gs.SurrenderUsed[0] = true
				return gs
			},
			action:  game.Action{Type: game.ActionSurrenderRequest, PlayerSeat: 1},
			wantErr: apperr.ErrActionRequired,
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

func TestSurrenderAccept(t *testing.T) {
	cases := []struct {
		proposer   int
		wantWinner int
	}{
		{proposer: 0, wantWinner: 1},
		{proposer: 1, wantWinner: 0},
		{proposer: 2, wantWinner: 1},
		{proposer: 3, wantWinner: 0},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("proposer=%d", c.proposer), func(t *testing.T) {
			gs := testfixtures.NewGameMidPlay(1)
			seat := c.proposer
			gs.SurrenderProposerSeat = &seat
			gs.SurrenderUsed[seat] = true
			expiry := gs.TurnExpiresAt
			_ = expiry

			partner := (c.proposer + 2) % 4
			newState, err := game.ApplyAction(gs, game.Action{
				Type:       game.ActionSurrenderAccept,
				PlayerSeat: partner,
			})
			require.NoError(t, err)
			require.NotNil(t, newState)

			assert.Equal(t, game.PhaseMatchEnd, newState.Phase)
			require.NotNil(t, newState.WinnerTeam)
			assert.Equal(t, c.wantWinner, *newState.WinnerTeam)
			assert.Nil(t, newState.SurrenderProposerSeat, "proposer pointer cleared")
			assert.True(t, newState.SurrenderUsed[c.proposer], "proposer's used flag preserved")
			assert.Nil(t, newState.TurnExpiresAt, "turn timer cancelled at match end")
			assert.Equal(t, int64(0), newState.TurnTimeRemaining)
		})
	}

	t.Run("reject: accept by proposer themselves returns ErrInvalidAction", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		seat := 0
		gs.SurrenderProposerSeat = &seat
		gs.SurrenderUsed[0] = true

		_, err := game.ApplyAction(gs, game.Action{
			Type:       game.ActionSurrenderAccept,
			PlayerSeat: 0,
		})
		require.Error(t, err)
		assert.True(t, errors.Is(err, apperr.ErrInvalidAction))
	})

	t.Run("reject: accept by opponent returns ErrInvalidAction", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		seat := 0
		gs.SurrenderProposerSeat = &seat
		gs.SurrenderUsed[0] = true

		_, err := game.ApplyAction(gs, game.Action{
			Type:       game.ActionSurrenderAccept,
			PlayerSeat: 1,
		})
		require.Error(t, err)
		assert.True(t, errors.Is(err, apperr.ErrInvalidAction))

		_, err = game.ApplyAction(gs, game.Action{
			Type:       game.ActionSurrenderAccept,
			PlayerSeat: 3,
		})
		require.Error(t, err)
		assert.True(t, errors.Is(err, apperr.ErrInvalidAction))
	})

	t.Run("reject: accept when no proposal returns ErrWrongPhase", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		_, err := game.ApplyAction(gs, game.Action{
			Type:       game.ActionSurrenderAccept,
			PlayerSeat: 2,
		})
		require.Error(t, err)
		assert.True(t, errors.Is(err, apperr.ErrWrongPhase))
	})
}

func TestSurrenderDecline(t *testing.T) {
	t.Run("decline by partner clears proposer; consumes attempt", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		seat := 1
		gs.SurrenderProposerSeat = &seat
		gs.SurrenderUsed[1] = true
		origPhase := gs.Phase
		origActive := gs.ActivePlayerSeat
		origScores := gs.TeamScores

		newState, err := game.ApplyAction(gs, game.Action{
			Type:       game.ActionSurrenderDecline,
			PlayerSeat: 3,
		})
		require.NoError(t, err)
		require.NotNil(t, newState)

		assert.Nil(t, newState.SurrenderProposerSeat)
		assert.True(t, newState.SurrenderUsed[1], "proposer's attempt remains consumed")
		assert.False(t, newState.SurrenderUsed[3], "decliner is unaffected")
		assert.Equal(t, origPhase, newState.Phase)
		assert.Equal(t, origActive, newState.ActivePlayerSeat)
		assert.Equal(t, origScores, newState.TeamScores)
	})

	t.Run("partner can decline even with their own SurrenderUsed=true", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		seat := 0
		gs.SurrenderProposerSeat = &seat
		gs.SurrenderUsed[0] = true
		gs.SurrenderUsed[2] = true

		newState, err := game.ApplyAction(gs, game.Action{
			Type:       game.ActionSurrenderDecline,
			PlayerSeat: 2,
		})
		require.NoError(t, err)
		require.NotNil(t, newState)
		assert.Nil(t, newState.SurrenderProposerSeat)
		assert.True(t, newState.SurrenderUsed[0])
		assert.True(t, newState.SurrenderUsed[2])
	})

	t.Run("reject: decline by non-partner returns ErrInvalidAction", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		seat := 0
		gs.SurrenderProposerSeat = &seat
		gs.SurrenderUsed[0] = true

		_, err := game.ApplyAction(gs, game.Action{
			Type:       game.ActionSurrenderDecline,
			PlayerSeat: 1,
		})
		require.Error(t, err)
		assert.True(t, errors.Is(err, apperr.ErrInvalidAction))
	})

	t.Run("reject: decline when no proposal returns ErrWrongPhase", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		_, err := game.ApplyAction(gs, game.Action{
			Type:       game.ActionSurrenderDecline,
			PlayerSeat: 0,
		})
		require.Error(t, err)
		assert.True(t, errors.Is(err, apperr.ErrWrongPhase))
	})
}

// TestSurrenderPureFunctionDiscipline verifies the rules engine's pure-function
// contract — applying a surrender action must not mutate the input state.
func TestSurrenderPureFunctionDiscipline(t *testing.T) {
	t.Run("request does not mutate input", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		origUsed := gs.SurrenderUsed
		origProposer := gs.SurrenderProposerSeat

		_, err := game.ApplyAction(gs, game.Action{
			Type:       game.ActionSurrenderRequest,
			PlayerSeat: 0,
		})
		require.NoError(t, err)

		assert.Equal(t, origUsed, gs.SurrenderUsed, "input SurrenderUsed must not change")
		assert.Equal(t, origProposer, gs.SurrenderProposerSeat, "input proposer pointer must not change")
	})

	t.Run("accept does not mutate input", func(t *testing.T) {
		gs := testfixtures.NewGameMidPlay(1)
		seat := 0
		gs.SurrenderProposerSeat = &seat
		gs.SurrenderUsed[0] = true
		origPhase := gs.Phase

		_, err := game.ApplyAction(gs, game.Action{
			Type:       game.ActionSurrenderAccept,
			PlayerSeat: 2,
		})
		require.NoError(t, err)

		assert.Equal(t, origPhase, gs.Phase, "input phase unchanged")
		assert.Nil(t, gs.WinnerTeam, "input WinnerTeam unchanged")
	})
}

// TestSurrenderCrossStateIsolation locks in that surrender does not touch
// pause flags, scores, hands, or trick state.
func TestSurrenderCrossStateIsolation(t *testing.T) {
	gs := testfixtures.NewGameMidPlay(1)
	gs.PauseUsed[0] = true
	gs.HandPoints = [2]int{40, 30}
	gs.TricksWon = [2]int{2, 1}
	gs.TeamScores = [2]int{600, 400}

	newState, err := game.ApplyAction(gs, game.Action{
		Type:       game.ActionSurrenderRequest,
		PlayerSeat: 1,
	})
	require.NoError(t, err)
	require.NotNil(t, newState)

	assert.Equal(t, gs.PauseUsed, newState.PauseUsed, "PauseUsed unchanged")
	assert.Equal(t, gs.PausedPlayers, newState.PausedPlayers, "PausedPlayers unchanged")
	assert.Equal(t, gs.HandPoints, newState.HandPoints, "HandPoints unchanged")
	assert.Equal(t, gs.TricksWon, newState.TricksWon, "TricksWon unchanged")
	assert.Equal(t, gs.TeamScores, newState.TeamScores, "TeamScores unchanged")
	for i := range gs.Players {
		assert.Equal(t, len(gs.Players[i].Hand), len(newState.Players[i].Hand), "hand size unchanged")
	}
}
