package match_test

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/ws"
)

const windowMoveSec = 30

// bitolaWindowSession parks a per-move Bitola session at the start of NewGameBitolaWindow's trick 1, with the leader,
// seat 1, prompted and its move clock running.
func bitolaWindowSession(t *testing.T, hub match.Broadcaster, roomID uint) (*match.Manager, *game.GameState) {
	t.Helper()

	mgr := match.NewManager(hub, newMockMatchRepo())
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "per-move", windowMoveSec, 10, 120, 0,
		true, false, true, false, 3))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	gs := testfixtures.NewGameBitolaWindow()
	gs.RoomID = roomID
	gs.AwaitingDeclaration = true
	expiry := time.Now().Add(windowMoveSec * time.Second)
	gs.TurnExpiresAt = &expiry
	gs.TimerDurationSec = windowMoveSec
	game.RefreshDerivedFlags(gs)
	mgr.SetGameStateForTest(roomID, gs)
	return mgr, gs
}

// framesSince returns, for every match_state batch after before, the user IDs it reached.
func framesSince(hub *hubSpy, before int) [][]uint {
	var out [][]uint
	for _, c := range hub.snapshot()[before:] {
		if c.frames != nil && containsType(c.msg, ws.EventMatchState) {
			out = append(out, slices.Clone(c.userIDs))
		}
	}
	return out
}

// projectedFor decodes the match_state frame one user received in a batch.
func projectedFor(t *testing.T, call hubCall, userID uint) game.GameState {
	t.Helper()
	raw, ok := call.frames[userID]
	require.True(t, ok, "user %d got no frame in this batch", userID)
	var msg struct {
		Payload game.GameState `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(raw, &msg))
	return msg.Payload
}

// TestDeclarationWindow_StartMatchCarriesTheWindow checks the room's window reaches a Bitola engine and is
// recorded on its match row, and that a Croatian match records the default.
func TestDeclarationWindow_StartMatchCarriesTheWindow(t *testing.T) {
	tests := []struct {
		variant    string
		requested  int
		wantEngine int
		wantRow    int
	}{
		{"bitola", 1, 1, 1},
		{"bitola", 2, 2, 2},
		{"bitola", 3, 3, 3},
		{"croatia", 3, 0, 3},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s window %d", tc.variant, tc.requested), func(t *testing.T) {
			const roomID = uint(880)
			repo := newMockMatchRepo()
			mgr := match.NewManager(&hubSpy{}, repo)
			require.NoError(t, mgr.StartMatch(roomID, tc.variant, "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0,
				true, false, true, false, tc.requested))

			st := mgr.GetStateSnapshot(roomID)
			require.NotNil(t, st)
			assert.Equal(t, tc.wantEngine, st.Rules.DeclarationLastTrick)
			assert.Equal(t, tc.wantEngine, st.DeclarationLastTrick, "the wire mirror carries it")

			final := *st
			winner := game.TeamA
			final.WinnerTeam = &winner
			final.Phase = game.PhaseMatchEnd
			mgr.HandleMatchEndForTest(roomID, &final, nil, ws.MatchEndPayload{OutcomeReason: ws.OutcomeReasonNatural})

			matches := repo.getMatches()
			require.Len(t, matches, 1)
			assert.Equal(t, tc.wantRow, matches[0].DeclarationLastTrick)
		})
	}

	t.Run("an abandoned match records the window too", func(t *testing.T) {
		const roomID = uint(881)
		repo := newMockMatchRepo()
		mgr := match.NewManager(&hubSpy{}, repo)
		require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0,
			true, false, true, false, 2))

		mgr.AbandonSeatForTest(roomID, 1)

		matches := repo.getMatches()
		require.Len(t, matches, 1)
		assert.Equal(t, "abandoned", matches[0].Status)
		assert.Equal(t, 2, matches[0].DeclarationLastTrick)
	})
}

// TestDeclarationWindow_PassReachesOnlyThePassingSeat checks a pass at a turn prompt keeps the move clock and sends
// its state frame to the actor alone, while every other seat never saw the prompt flag.
func TestDeclarationWindow_PassReachesOnlyThePassingSeat(t *testing.T) {
	const roomID = uint(882)
	hub := &hubSpy{}
	mgr, gs := bitolaWindowSession(t, hub, roomID)

	before := len(hub.snapshot())
	require.NoError(t, mgr.ApplyActionForTest(roomID, game.Action{Type: game.ActionSkipDeclare, PlayerSeat: 1}))

	calls := hub.snapshot()[before:]
	require.Len(t, calls, 1, "a pass sends one frame and no event: %v", eventKindsAfter(t, hub, before))
	assert.Equal(t, []uint{20}, calls[0].userIDs)
	assert.False(t, projectedFor(t, calls[0], 20).AwaitingDeclaration)

	after := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, after.TurnExpiresAt)
	assert.True(t, after.TurnExpiresAt.Equal(*gs.TurnExpiresAt), "the prompt shares the move clock")
	assert.Equal(t, 1, after.ActivePlayerSeat)
}

// TestDeclarationWindow_DeclareOpensOneCycleForEveryone checks a declaration at a turn prompt announces only the
// declarer, opens the cycle for every seat on one window that answers never restart, and resumes at the declarer.
func TestDeclarationWindow_DeclareOpensOneCycleForEveryone(t *testing.T) {
	const roomID = uint(883)
	hub := &hubSpy{}
	mgr, _ := bitolaWindowSession(t, hub, roomID)

	before := len(hub.snapshot())
	require.NoError(t, mgr.ApplyActionForTest(roomID, game.Action{Type: game.ActionDeclare, PlayerSeat: 1}))

	kinds := eventKindsAfter(t, hub, before)
	declaredIdx := indexOf(kinds, ws.EventPlayerDeclared)
	stateIdx := indexOf(kinds, ws.EventMatchState)
	require.GreaterOrEqual(t, declaredIdx, 0, "%v", kinds)
	assert.Less(t, declaredIdx, stateIdx, "player_declared rides ahead of the cycle's state: %v", kinds)
	assert.Equal(t, [][]uint{{10, 20, 30, 40}}, framesSince(hub, before), "every seat sees the cycle")

	cycle := mgr.GetStateSnapshot(roomID)
	require.Equal(t, game.PhaseDeclaring, cycle.Phase)
	assert.Nil(t, cycle.TurnExpiresAt)
	window := mgr.DeclarationExpiresAtForTest(roomID)
	assert.WithinDuration(t, time.Now().Add(20*time.Second), window, 2*time.Second)

	before = len(hub.snapshot())
	require.NoError(t, mgr.ApplyActionForTest(roomID, game.Action{Type: game.ActionDeclare, PlayerSeat: 3}))
	assert.Equal(t, -1, indexOf(eventKindsAfter(t, hub, before), ws.EventPlayerDeclared),
		"a declaration inside the cycle is not announced")
	assert.Equal(t, window, mgr.DeclarationExpiresAtForTest(roomID), "an answer never restarts the window")

	require.NoError(t, mgr.ApplyActionForTest(roomID, game.Action{Type: game.ActionSkipDeclare, PlayerSeat: 0}))
	before = len(hub.snapshot())
	require.NoError(t, mgr.ApplyActionForTest(roomID, game.Action{Type: game.ActionSkipDeclare, PlayerSeat: 2}))

	assertRevealPrecedesPlayingState(t, hub, before, ptrTeam(game.TeamB))
	resumed := mgr.GetStateSnapshot(roomID)
	assert.Equal(t, game.PhasePlaying, resumed.Phase)
	assert.Equal(t, 1, resumed.ActivePlayerSeat, "play resumes at the declarer")
	assert.False(t, resumed.AwaitingDeclaration, "with no new prompt")
	require.NotNil(t, resumed.TurnExpiresAt, "and a fresh move clock")
	assert.WithinDuration(t, time.Now().Add(windowMoveSec*time.Second), *resumed.TurnExpiresAt, 2*time.Second)
}

func ptrTeam(team int) *int {
	return &team
}

// TestDeclarationWindow_CycleTimeoutResumesAtTheDeclarer checks an elapsed cycle window resolves the contest and
// re-arms the declarer's move clock, with no timeout auto-action counted against anyone.
func TestDeclarationWindow_CycleTimeoutResumesAtTheDeclarer(t *testing.T) {
	const roomID = uint(884)
	hub := &hubSpy{}
	mgr, _ := bitolaWindowSession(t, hub, roomID)
	require.NoError(t, mgr.ApplyActionForTest(roomID, game.Action{Type: game.ActionDeclare, PlayerSeat: 1}))

	before := len(hub.snapshot())
	mgr.SetDeclarationExpiresAtForTest(roomID, time.Now().Add(-time.Second))
	mgr.TriggerDeclarationTimeoutForTest(roomID, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.Phase == game.PhasePlaying
	}, 2*time.Second, 5*time.Millisecond, "the elapsed window must close the cycle")

	resumed := mgr.GetStateSnapshot(roomID)
	assert.True(t, resumed.DeclarationsResolved)
	assert.Equal(t, 50, resumed.DeclarationPoints[game.TeamB])
	assert.Equal(t, 1, resumed.ActivePlayerSeat)
	require.NotNil(t, resumed.TurnExpiresAt)
	assert.True(t, resumed.TurnExpiresAt.After(time.Now()), "the declarer gets a fresh move clock")

	assertRevealPrecedesPlayingState(t, hub, before, ptrTeam(game.TeamB))
	kinds := eventKindsAfter(t, hub, before)
	assert.Equal(t, -1, indexOf(kinds, ws.EventAutoAction), "the cycle window is no missed turn: %v", kinds)
}

// TestDeclarationWindow_PromptTimeoutPassesAndPlays checks an expired move clock at a turn prompt passes, telling only
// that seat, and auto-plays the card, after which the next meld holder is prompted.
func TestDeclarationWindow_PromptTimeoutPassesAndPlays(t *testing.T) {
	const roomID = uint(885)
	hub := &hubSpy{}
	mgr, _ := bitolaWindowSession(t, hub, roomID)

	before := len(hub.snapshot())
	mgr.TriggerTimerExpiryForTest(roomID, 1, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && len(st.CurrentTrick) == 1
	}, 2*time.Second, 5*time.Millisecond, "the timed-out seat's card must be auto-played")

	after := mgr.GetStateSnapshot(roomID)
	assert.Equal(t, 2, after.ActivePlayerSeat)
	assert.True(t, after.AwaitingDeclaration, "seat 2 holds a quarte and is asked next")
	assert.False(t, after.DeclarationsResolved, "a timed-out prompt declares nothing")

	var autoAction hubCall
	require.Eventually(t, func() bool {
		for _, c := range hub.snapshot()[before:] {
			if containsType(c.msg, ws.EventAutoAction) {
				autoAction = c
				return true
			}
		}
		return false
	}, 2*time.Second, 5*time.Millisecond, "the timed-out seat must hear about its pass")
	require.NotNil(t, autoAction.frames, "as a frame to that seat alone")
	assert.Equal(t, []uint{20}, autoAction.userIDs, "the others hear nothing, as for a timed-out card play")
	assert.Contains(t, string(autoAction.frames[20]), string(ws.AutoActionSkipDeclare), "the seat learns it passed")
}

// TestDeclarationWindow_DisconnectKeepsThePrompt checks a seat dropping at its own turn prompt finds the prompt
// waiting on reconnect with the rest of its move clock, and that nobody else ever saw it.
func TestDeclarationWindow_DisconnectKeepsThePrompt(t *testing.T) {
	const roomID = uint(886)
	hub := &hubSpy{}
	mgr, _ := bitolaWindowSession(t, hub, roomID)

	before := len(hub.snapshot())
	mgr.HandleDisconnect(20)

	paused := mgr.GetStateSnapshot(roomID)
	require.Equal(t, game.PhaseDisconnected, paused.Phase)
	assert.Equal(t, game.PhasePlaying, paused.PreviousPhase)
	assert.True(t, paused.AwaitingDeclaration)
	assert.Positive(t, paused.TurnTimeRemaining)
	for _, c := range hub.snapshot()[before:] {
		for uid := range c.frames {
			assert.False(t, projectedFor(t, c, uid).AwaitingDeclaration, "user %d must not see seat 1's prompt", uid)
		}
	}

	mgr.HandleReconnect(20)

	resumed := mgr.GetStateSnapshot(roomID)
	assert.Equal(t, game.PhasePlaying, resumed.Phase)
	assert.True(t, resumed.AwaitingDeclaration, "the prompt is still owed")
	assert.Equal(t, 1, resumed.ActivePlayerSeat)
	require.NotNil(t, resumed.TurnExpiresAt)
	assert.True(t, resumed.TurnExpiresAt.After(time.Now()))
}

// TestDeclarationWindow_DisconnectMidCycleReopensAFreshWindow checks a seat dropping inside the cycle pauses it, and
// reconnecting reopens it on a fresh window with the trick and the declarer's answer intact.
func TestDeclarationWindow_DisconnectMidCycleReopensAFreshWindow(t *testing.T) {
	const roomID = uint(887)
	hub := &hubSpy{}
	mgr, _ := bitolaWindowSession(t, hub, roomID)
	require.NoError(t, mgr.ApplyActionForTest(roomID, game.Action{Type: game.ActionSkipDeclare, PlayerSeat: 1}))
	require.NoError(t, mgr.ApplyActionForTest(roomID, playCardAction(t, 1, "9D")))
	require.NoError(t, mgr.ApplyActionForTest(roomID, game.Action{Type: game.ActionDeclare, PlayerSeat: 2}))
	mgr.SetDeclarationExpiresAtForTest(roomID, time.Now().Add(time.Second))

	mgr.HandleDisconnect(40)
	paused := mgr.GetStateSnapshot(roomID)
	require.Equal(t, game.PhaseDisconnected, paused.Phase)
	require.Equal(t, game.PhaseDeclaring, paused.PreviousPhase)

	mgr.HandleReconnect(40)

	reopened := mgr.GetStateSnapshot(roomID)
	require.Equal(t, game.PhaseDeclaring, reopened.Phase)
	assert.WithinDuration(t, time.Now().Add(20*time.Second), mgr.DeclarationExpiresAtForTest(roomID), 2*time.Second,
		"reconnecting opens a fresh window")
	assert.Len(t, reopened.CurrentTrick, 1, "the trick waits on the table")
	assert.Equal(t, 2, reopened.ActivePlayerSeat)
	assert.True(t, reopened.Players[2].DeclarationAnswered, "the declarer stays answered")

	for _, seat := range []int{0, 1, 3} {
		require.NoError(t, mgr.ApplyActionForTest(roomID, game.Action{Type: game.ActionSkipDeclare, PlayerSeat: seat}))
	}
	resumed := mgr.GetStateSnapshot(roomID)
	assert.Equal(t, game.PhasePlaying, resumed.Phase)
	assert.Equal(t, 2, resumed.ActivePlayerSeat)
	assert.Equal(t, 50, resumed.DeclarationPoints[game.TeamA])
}

func playCardAction(t *testing.T, seat int, id string) game.Action {
	t.Helper()
	card, err := game.ParseCard(id)
	require.NoError(t, err)
	return game.Action{Type: game.ActionPlayCard, PlayerSeat: seat, Card: &card}
}

// TestDeclarationWindow_BotDeclaresAtOnce checks a bot answers its own turn prompt after the shortest think delay.
func TestDeclarationWindow_BotDeclaresAtOnce(t *testing.T) {
	mgr := match.NewManager(&hubSpy{}, newMockMatchRepo())
	mgr.SetBotDelayForTest(40*time.Millisecond, 2*time.Second)

	gs := testfixtures.NewGameBitolaWindow()
	gs.Players[1].IsBot = true
	gs.AwaitingDeclaration = true
	assert.Equal(t, 40*time.Millisecond, mgr.BotThinkDelayForTest(gs, 1), "its own prompt")

	gs.AwaitingDeclaration = false
	for range 20 {
		if mgr.BotThinkDelayForTest(gs, 1) > 40*time.Millisecond {
			return
		}
	}
	t.Fatal("a card play must keep the humanized spread")
}
