package match_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/ws"
)

// croatianDeclaringSession parks a per-move Croatian session in the declaration phase
// with all four seats unanswered and its deadline at now+windowIn (negative = already elapsed).
func croatianDeclaringSession(
	t *testing.T,
	hub match.Broadcaster,
	roomID uint,
	timerSec int,
	windowIn time.Duration,
) (*match.Manager, *game.GameState) {
	t.Helper()

	mgr := match.NewManager(hub, newMockMatchRepo())
	require.NoError(t, mgr.StartMatch(roomID, "croatia", "1001", defaultPlayers(), "per-move", timerSec, 10, 120, 0, true, false, true))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	gs := testfixtures.NewGameCroatianDeclaring(game.SuitHearts)
	gs.RoomID = roomID
	require.Equal(t, game.PhaseDeclaring, gs.Phase)
	require.False(t, gs.AwaitingDeclaration, "the phase never puts a seat on the clock")
	require.Equal(t, 1, gs.ActivePlayerSeat, "pinned to the trick-1 leader")
	for seat, p := range gs.Players {
		require.False(t, p.DeclarationAnswered, "seat %d starts unanswered", seat)
	}

	gs.TurnExpiresAt = nil
	gs.TimerDurationSec = timerSec
	mgr.SetGameStateForTest(roomID, gs)
	mgr.SetDeclarationExpiresAtForTest(roomID, time.Now().Add(windowIn))

	return mgr, gs
}

// answerAll drives every seat through the phase with the given action, applying
// each through ApplyAction so the walk exercises the real engine.
func answerAll(t *testing.T, state *game.GameState, seats []int, action string) *game.GameState {
	t.Helper()
	for _, seat := range seats {
		next, err := game.ApplyAction(state, game.Action{Type: action, PlayerSeat: seat})
		require.NoError(t, err, "seat %d", seat)
		state = next
	}
	return state
}

// wireEvent is one hub message decoded far enough to assert type and ordering.
type wireEvent struct {
	kind    string
	payload json.RawMessage
}

func wireEvents(t *testing.T, calls []hubCall) []wireEvent {
	t.Helper()
	out := make([]wireEvent, 0, len(calls))
	for _, c := range calls {
		var msg struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}
		require.NoError(t, json.Unmarshal(c.msg, &msg))
		out = append(out, wireEvent{kind: msg.Type, payload: msg.Payload})
	}
	return out
}

// indexOfKind is the position of the first message of the given type, or -1.
func indexOfKind(events []wireEvent, kind string) int {
	for i, e := range events {
		if e.kind == kind {
			return i
		}
	}
	return -1
}

// indexOfMatchStateWithPhase is the position of the first event:match_state carrying
// the given phase, or -1. It asserts nothing so it is safe inside require.Eventually.
func indexOfMatchStateWithPhase(events []wireEvent, phase string) int {
	for i, e := range events {
		if e.kind != ws.EventMatchState {
			continue
		}
		var st struct {
			Phase string `json:"phase"`
		}
		if err := json.Unmarshal(e.payload, &st); err != nil {
			continue
		}
		if st.Phase == phase {
			return i
		}
	}
	return -1
}

// declarationsResolvedWinner reads the winnerTeam off an
// event:declarations_resolved payload. nil means "no team scored".
func declarationsResolvedWinner(t *testing.T, payload json.RawMessage) *int {
	t.Helper()
	var body struct {
		WinnerTeam   *int              `json:"winnerTeam"`
		Declarations []json.RawMessage `json:"declarations"`
	}
	require.NoError(t, json.Unmarshal(payload, &body))
	return body.WinnerTeam
}

// assertRevealPrecedesPlayingState waits for the declarations reveal and the trick-1
// match_state after the watermark and asserts the reveal reached clients first.
func assertRevealPrecedesPlayingState(t *testing.T, hub *hubSpy, before int, wantWinner *int) {
	t.Helper()

	var events []wireEvent
	require.Eventually(t, func() bool {
		events = wireEvents(t, hub.snapshot()[before:])
		return indexOfKind(events, ws.EventDeclarationsResolved) >= 0 &&
			indexOfMatchStateWithPhase(events, string(game.PhasePlaying)) >= 0
	}, 2*time.Second, 5*time.Millisecond,
		"the resolving answer must emit both event:declarations_resolved and a playing-phase match_state")

	revealIdx := indexOfKind(events, ws.EventDeclarationsResolved)
	playingIdx := indexOfMatchStateWithPhase(events, string(game.PhasePlaying))

	assert.Less(t, revealIdx, playingIdx,
		"the reveal must ride ahead of the match_state that opens trick 1")

	got := declarationsResolvedWinner(t, events[revealIdx].payload)
	if wantWinner == nil {
		assert.Nil(t, got, "no team declared, so the reveal names no winner")
		return
	}
	require.NotNil(t, got, "a team scored, so the reveal must name it")
	assert.Equal(t, *wantWinner, *got)
}

// winningTeamOf derives the declaration winner from a resolved state, so the
// tests assert against what the engine actually decided rather than a
// hand-copied constant that would drift with any meld-value change.
func winningTeamOf(t *testing.T, st *game.GameState) *int {
	t.Helper()
	require.True(t, st.DeclarationsResolved)
	switch {
	case st.DeclarationPoints[game.TeamA] > 0:
		team := game.TeamA
		return &team
	case st.DeclarationPoints[game.TeamB] > 0:
		team := game.TeamB
		return &team
	}
	return nil
}

// TestDeclarationPhase_WindowForceClosesTheContest checks an elapsed window treats every
// unanswered seat as skipped, resolves the contest, and opens trick 1 with a live timer.
func TestDeclarationPhase_WindowForceClosesTheContest(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(100)
	mgr, _ := croatianDeclaringSession(t, hub, roomID, 60, -time.Second)

	before := len(hub.snapshot())
	mgr.TriggerDeclarationTimeoutForTest(roomID, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.Phase == game.PhasePlaying
	}, 2*time.Second, 5*time.Millisecond, "the elapsed window must force-close the phase")

	after := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, after)
	assert.Equal(t, 1, after.TrickNumber)
	assert.True(t, after.DeclarationsResolved)
	assert.False(t, after.AwaitingDeclaration)
	assert.Equal(t, (after.DealerSeat+1)%4, after.ActivePlayerSeat, "the leader is on the clock")
	require.NotNil(t, after.TurnExpiresAt, "trick 1 opens with a live timer")
	assert.True(t, after.TurnExpiresAt.After(time.Now()))
	for seat, p := range after.Players {
		assert.False(t, p.DeclarationAnswered, "seat %d's flag is cleared on the way out", seat)
		assert.Empty(t, p.Declarations, "seat %d never answered, so it forfeits its melds", seat)
	}

	assertRevealPrecedesPlayingState(t, hub, before, nil)
}

// TestDeclarationPhase_WindowKeepsAnswersAlreadyGiven is the other half of the
// force-close: seats that DID answer keep what they said.
func TestDeclarationPhase_WindowKeepsAnswersAlreadyGiven(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(101)
	mgr, gs := croatianDeclaringSession(t, hub, roomID, 60, -time.Second)

	state := answerAll(t, gs, []int{1, 2, 3}, game.ActionDeclare)
	require.Equal(t, game.PhaseDeclaring, state.Phase, "seat 0 still owes an answer")
	mgr.SetGameStateForTest(roomID, state)

	before := len(hub.snapshot())
	mgr.TriggerDeclarationTimeoutForTest(roomID, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.Phase == game.PhasePlaying
	}, 2*time.Second, 5*time.Millisecond, "the window must close the phase")

	after := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, after)
	assert.True(t, after.DeclarationsResolved)
	assert.NotEqual(t, [2]int{0, 0}, after.DeclarationPoints,
		"the three seats that did answer still contest the melds")

	assertRevealPrecedesPlayingState(t, hub, before, winningTeamOf(t, after))
}

// TestDeclarationPhase_FinalAnswerRevealsBeforePlayingState is the wire contract
// on the player-driven path: the fourth seat's own declare closes the contest,
// so the reveal and the trick-1 state ride out of a single action.
func TestDeclarationPhase_FinalAnswerRevealsBeforePlayingState(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(106)
	mgr, gs := croatianDeclaringSession(t, hub, roomID, 60, 30*time.Second)

	state := answerAll(t, gs, []int{1, 2, 3}, game.ActionDeclare)
	require.Equal(t, game.PhaseDeclaring, state.Phase)
	mgr.SetGameStateForTest(roomID, state)

	before := len(hub.snapshot())
	client := &ws.Client{UserID: state.Players[0].UserID}
	mgr.HandleAction(client, ws.WSMessage{Type: "action:declare", Payload: []byte(`{}`)})

	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.Phase == game.PhasePlaying
	}, 2*time.Second, 5*time.Millisecond, "the fourth answer must open trick 1")

	after := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, after)
	assert.Equal(t, 1, after.TrickNumber)
	assert.True(t, after.DeclarationsResolved)

	assertRevealPrecedesPlayingState(t, hub, before, winningTeamOf(t, after))
}

// TestDeclarationPhase_DeclareIsNotAnnouncedDuringThePhase checks no event:player_declared
// is broadcast mid-phase, so the reveal is the only announcement.
func TestDeclarationPhase_DeclareIsNotAnnouncedDuringThePhase(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(121)
	mgr, gs := croatianDeclaringSession(t, hub, roomID, 60, 30*time.Second)

	before := len(hub.snapshot())
	client := &ws.Client{UserID: gs.Players[1].UserID}
	mgr.HandleAction(client, ws.WSMessage{Type: "action:declare", Payload: []byte(`{}`)})

	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.Players[1].DeclarationAnswered
	}, 2*time.Second, 5*time.Millisecond, "the declare must commit")

	mid := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, mid)
	require.Equal(t, game.PhaseDeclaring, mid.Phase, "three seats still owe an answer")
	require.NotEmpty(t, mid.Players[1].Declarations, "the melds are banked server-side")

	events := wireEvents(t, hub.snapshot()[before:])
	assert.Equal(t, -1, indexOfKind(events, ws.EventPlayerDeclared),
		"no seat may be named to the table while the contest is still open")
	assert.Equal(t, -1, indexOfKind(events, ws.EventDeclarationsResolved),
		"and nothing is revealed until every seat has answered")
}

// TestDeclarationPhase_OtherSeatsMeldsStayMaskedMidPhase checks a seat's melds stay off
// every other seat's snapshot until the contest resolves.
func TestDeclarationPhase_OtherSeatsMeldsStayMaskedMidPhase(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(122)
	_, gs := croatianDeclaringSession(t, hub, roomID, 60, 30*time.Second)

	state := answerAll(t, gs, []int{1}, game.ActionDeclare)
	require.NotEmpty(t, state.Players[1].Declarations)
	require.False(t, state.DeclarationsResolved)

	seen := game.ProjectForSeat(state, 2)
	assert.Empty(t, seen.Players[1].Declarations, "seat 1's melds must not reach seat 2")
	assert.True(t, seen.Players[1].DeclarationAnswered,
		"that seat 1 has ANSWERED is public — it says nothing about what they hold")
	assert.False(t, seen.Players[3].DeclarationAnswered)
}

// TestDeclarationPhase_NoMeldHandStillOpensThePhase checks the phase still opens and asks
// all four seats when nobody holds a meld, so its absence leaks nothing.
func TestDeclarationPhase_NoMeldHandStillOpensThePhase(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(107)

	mgr := match.NewManager(hub, newMockMatchRepo())
	require.NoError(t, mgr.StartMatch(roomID, "croatia", "1001", defaultPlayers(), "per-move", 60, 10, 120, 0, true, false, true))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	gs := testfixtures.NewGameCroatianJustDealt()
	gs.RoomID = roomID
	layout := [4][]string{
		{"7S", "9S", "8H", "TH", "JD", "KD", "QC", "AC"},
		{"8S", "TS", "JH", "KH", "QD", "AD", "7C", "9C"},
		{"JS", "KS", "QH", "AH", "7D", "9D", "8C", "TC"},
		{"QS", "AS", "7H", "9H", "8D", "TD", "JC", "KC"},
	}
	for seat, ids := range layout {
		cards := make([]game.Card, 0, 8)
		for _, id := range ids {
			c, err := game.ParseCard(id)
			require.NoError(t, err)
			cards = append(cards, c)
		}
		gs.Players[seat].Hand = append([]game.Card(nil), cards[:6]...)
		gs.Players[seat].FaceDownCards = append([]game.Card(nil), cards[6:]...)
	}
	mgr.SetGameStateForTest(roomID, gs)

	before := len(hub.snapshot())
	client := &ws.Client{UserID: gs.Players[gs.ActivePlayerSeat].UserID}
	mgr.HandleAction(client, ws.WSMessage{Type: "action:pick_trump", Payload: []byte(`{"suit":"S"}`)})

	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.Phase == game.PhaseDeclaring
	}, 2*time.Second, 5*time.Millisecond, "a meld-less hand still opens the phase")

	opened := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, opened)
	assert.Equal(t, 0, opened.TrickNumber)
	assert.False(t, opened.DeclarationsResolved, "nothing is revealed before the seats answer")
	assert.Nil(t, opened.TurnExpiresAt, "no per-move clock in this phase")
	assert.False(t, mgr.DeclarationExpiresAtForTest(roomID).IsZero(), "the fixed window is armed")

	events := wireEvents(t, hub.snapshot()[before:])
	assert.Equal(t, -1, indexOfKind(events, ws.EventDeclarationsResolved),
		"a meld-less hand must be indistinguishable from any other until the seats answer")

	state := answerAll(t, opened, []int{1, 2, 3, 0}, game.ActionSkipDeclare)
	assert.Equal(t, game.PhasePlaying, state.Phase)
	assert.Equal(t, [2]int{0, 0}, state.DeclarationPoints)
}

// TestDeclarationPhase_ActionDoesNotExtendTheWindow checks an answer never pushes back
// the declaration deadline, which is set on entry only.
func TestDeclarationPhase_ActionDoesNotExtendTheWindow(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(102)
	mgr, gs := croatianDeclaringSession(t, hub, roomID, 60, 30*time.Second)
	original := mgr.DeclarationExpiresAtForTest(roomID)
	require.False(t, original.IsZero())

	client := &ws.Client{UserID: gs.Players[1].UserID}
	mgr.HandleAction(client, ws.WSMessage{Type: "action:skip_declare", Payload: []byte(`{}`)})

	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.Players[1].DeclarationAnswered
	}, 2*time.Second, 5*time.Millisecond, "the answer must commit")

	after := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, after)
	assert.Equal(t, game.PhaseDeclaring, after.Phase, "three seats still owe an answer")
	assert.Nil(t, after.TurnExpiresAt, "no seat is ever put on the clock")
	assert.True(t, mgr.DeclarationExpiresAtForTest(roomID).Equal(original),
		"an answer must not push the shared window back")
}

// TestDeclarationPhase_RejectedActionKeepsTheWindowArmed checks a rejected answer
// re-arms the phase timer against the same deadline.
func TestDeclarationPhase_RejectedActionKeepsTheWindowArmed(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(123)
	mgr, gs := croatianDeclaringSession(t, hub, roomID, 60, 40*time.Millisecond)

	client := &ws.Client{UserID: gs.Players[1].UserID}
	mgr.HandleAction(client, ws.WSMessage{Type: "action:skip_declare", Payload: []byte(`{}`)})
	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.Players[1].DeclarationAnswered
	}, 2*time.Second, 5*time.Millisecond)

	mgr.HandleAction(client, ws.WSMessage{Type: "action:declare", Payload: []byte(`{}`)})

	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.Phase == game.PhasePlaying
	}, 2*time.Second, 5*time.Millisecond,
		"the window must still fire after a rejected action")

	after := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, after)
	assert.True(t, after.DeclarationsResolved)
	assert.Empty(t, after.Players[1].Declarations, "the rejected declare did not overwrite the skip")
}

// TestDeclarationPhase_SurrenderLeavesTheWindowAlone replaces the old
// turn-budget test: there is no per-seat budget in this phase to preserve, but a
// surrender detour must still not disturb the shared window or the answers.
func TestDeclarationPhase_SurrenderLeavesTheWindowAlone(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(108)
	mgr, gs := croatianDeclaringSession(t, hub, roomID, 60, 30*time.Second)
	original := mgr.DeclarationExpiresAtForTest(roomID)

	proposer := &ws.Client{UserID: gs.Players[3].UserID}
	mgr.HandleAction(proposer, ws.WSMessage{Type: "action:surrender_request", Payload: []byte(`{}`)})

	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.SurrenderProposerSeat != nil
	}, 2*time.Second, 5*time.Millisecond, "surrender must be proposable inside the phase")

	proposed := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, proposed)
	assert.Equal(t, game.PhaseDeclaring, proposed.Phase, "proposing does not leave the phase")
	assert.Nil(t, proposed.TurnExpiresAt)
	assert.True(t, mgr.DeclarationExpiresAtForTest(roomID).Equal(original),
		"a proposal must not mint a fresh window")

	partner := &ws.Client{UserID: gs.Players[1].UserID}
	mgr.HandleAction(partner, ws.WSMessage{Type: "action:surrender_decline", Payload: []byte(`{}`)})

	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.SurrenderProposerSeat == nil
	}, 2*time.Second, 5*time.Millisecond, "the decline must clear the proposal")

	declined := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, declined)
	assert.Equal(t, game.PhaseDeclaring, declined.Phase)
	assert.True(t, mgr.DeclarationExpiresAtForTest(roomID).Equal(original),
		"declining must preserve the window, not restart it")
	for seat, p := range declined.Players {
		assert.False(t, p.DeclarationAnswered, "seat %d's answer is unaffected by the detour", seat)
	}

	mgr.HandleAction(partner, ws.WSMessage{Type: "action:skip_declare", Payload: []byte(`{}`)})
	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.Players[1].DeclarationAnswered
	}, 2*time.Second, 5*time.Millisecond, "the phase must still accept answers after the surrender detour")
}

// TestDeclarationPhase_IsNotPausable checks the declaration phase cannot be paused,
// since it has no active turn to preserve.
func TestDeclarationPhase_IsNotPausable(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(103)
	mgr, gs := croatianDeclaringSession(t, hub, roomID, 60, 30*time.Second)

	pauser := &ws.Client{UserID: gs.Players[3].UserID}
	mgr.HandleAction(pauser, ws.WSMessage{Type: "action:pause", Payload: []byte(`{}`)})

	assert.Never(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.Phase == game.PhasePaused
	}, 200*time.Millisecond, 10*time.Millisecond, "the phase must not be pausable")

	after := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, after)
	assert.Equal(t, game.PhaseDeclaring, after.Phase)
	assert.False(t, after.PauseUsed[3], "a refused pause must not spend the player's one pause")
}

// TestDeclarationPhase_DisconnectAndReconnectRestoreThePhase checks a drop mid-phase
// opens a reconnect window and marks the seat disconnected, and a reconnect restores it.
func TestDeclarationPhase_DisconnectAndReconnectRestoreThePhase(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(104)
	mgr, gs := croatianDeclaringSession(t, hub, roomID, 60, 30*time.Second)
	droppedSeat := 1
	droppedUserID := gs.Players[droppedSeat].UserID

	mgr.HandleDisconnect(droppedUserID)

	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.Phase == game.PhaseDisconnected
	}, 2*time.Second, 5*time.Millisecond, "a drop in the phase must open a reconnect window")

	dropped := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, dropped)
	assert.Equal(t, game.PhaseDeclaring, dropped.PreviousPhase, "the phase is the restore target")
	assert.False(t, dropped.Players[droppedSeat].Connected)
	assert.Equal(t, droppedSeat, dropped.DisconnectedSeat)
	assert.NotNil(t, dropped.ReconnectExpiresAt)

	mgr.HandleReconnect(droppedUserID)

	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.Phase == game.PhaseDeclaring
	}, 2*time.Second, 5*time.Millisecond, "reconnect must restore the phase")

	back := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, back)
	assert.True(t, back.Players[droppedSeat].Connected)
	assert.Equal(t, -1, back.DisconnectedSeat)
	assert.Nil(t, back.TurnExpiresAt, "the phase still has no per-move clock")
	assert.False(t, back.Players[droppedSeat].DeclarationAnswered,
		"the returning player still owes an answer")
	assert.True(t, mgr.DeclarationExpiresAtForTest(roomID).After(time.Now()),
		"the outage bought them a fresh window rather than an elapsed one")

	client := &ws.Client{UserID: droppedUserID}
	mgr.HandleAction(client, ws.WSMessage{Type: "action:declare", Payload: []byte(`{}`)})
	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.Players[droppedSeat].DeclarationAnswered
	}, 2*time.Second, 5*time.Millisecond, "the returning player's answer must be accepted")
}

// TestDeclarationPhase_AnsweredSeatCannotAnswerAgain checks a seat that already answered
// cannot answer again, so a skip cannot become a declare after a rejoin.
func TestDeclarationPhase_AnsweredSeatCannotAnswerAgain(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(124)
	mgr, gs := croatianDeclaringSession(t, hub, roomID, 60, 30*time.Second)

	client := &ws.Client{UserID: gs.Players[1].UserID}
	mgr.HandleAction(client, ws.WSMessage{Type: "action:skip_declare", Payload: []byte(`{}`)})
	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.Players[1].DeclarationAnswered
	}, 2*time.Second, 5*time.Millisecond)

	mgr.HandleAction(client, ws.WSMessage{Type: "action:declare", Payload: []byte(`{}`)})

	assert.Never(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && len(st.Players[1].Declarations) > 0
	}, 200*time.Millisecond, 10*time.Millisecond,
		"the first answer stands — a skip may not become a declare")
}

// TestDeclarationPhase_AllBotSeatsAnswer checks every unanswered bot seat is scheduled
// during the phase, not just one.
func TestDeclarationPhase_AllBotSeatsAnswer(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(105)

	mgr := match.NewManager(hub, newMockMatchRepo())
	mgr.SetDealGraceForTest(false)
	mgr.SetBotDelayForTest(5*time.Millisecond, 10*time.Millisecond)
	require.NoError(t, mgr.StartMatch(roomID, "croatia", "1001", mixedPlayers(1, 2, 3), "relaxed", 0, 10, 120, 0, true, false, true))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	gs := markBots(testfixtures.NewGameCroatianDeclaring(game.SuitHearts), 1, 2, 3)
	gs.RoomID = roomID
	gs.TurnExpiresAt = nil
	mgr.SetGameStateForTest(roomID, gs)
	mgr.SetDeclarationExpiresAtForTest(roomID, time.Now().Add(30*time.Second))
	mgr.BotSchedule(roomID)

	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		if st == nil {
			return false
		}
		for _, seat := range []int{1, 2, 3} {
			if !st.Players[seat].DeclarationAnswered {
				return false
			}
		}
		return true
	}, 2*time.Second, 5*time.Millisecond, "every bot seat must answer, not just one")

	after := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, after)
	assert.Equal(t, game.PhaseDeclaring, after.Phase, "the human seat 0 has not answered yet")
	for _, seat := range []int{1, 2, 3} {
		assert.NotEmpty(t, after.Players[seat].Declarations, "bot seat %d declares whenever it can", seat)
	}
	assert.Empty(t, after.CurrentTrick, "no card may be played in this phase")
}

// declarationsResolvedMelds reads the full `declarations` array off an
// event:declarations_resolved payload, flattened exactly as it travels the
// wire: one entry per meld, each carrying its own seat.
type wireMeld struct {
	PlayerSeat int      `json:"playerSeat"`
	Type       string   `json:"type"`
	Value      int      `json:"value"`
	Cards      []string `json:"cards"`
}

func declarationsResolvedBody(t *testing.T, payload json.RawMessage) (melds []wireMeld, contested bool) {
	t.Helper()
	var body struct {
		Declarations []wireMeld `json:"declarations"`
		Contested    bool       `json:"contested"`
	}
	require.NoError(t, json.Unmarshal(payload, &body))
	return body.Declarations, body.Contested
}

// TestDeclarationPhase_RevealCarriesEveryMeldPerSeat checks the reveal payload carries
// every meld of every winning seat, with expectations derived from the resolved engine state.
func TestDeclarationPhase_RevealCarriesEveryMeldPerSeat(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(118)
	mgr, gs := croatianDeclaringSession(t, hub, roomID, 60, 30*time.Second)

	state := answerAll(t, gs, []int{1, 2, 3}, game.ActionDeclare)
	require.Equal(t, game.PhaseDeclaring, state.Phase, "seat 0 still owes an answer")
	mgr.SetGameStateForTest(roomID, state)

	before := len(hub.snapshot())
	client := &ws.Client{UserID: state.Players[0].UserID}
	mgr.HandleAction(client, ws.WSMessage{Type: "action:declare", Payload: []byte(`{}`)})

	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.Phase == game.PhasePlaying
	}, 2*time.Second, 5*time.Millisecond, "the fourth answer must open trick 1")

	after := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, after)

	events := wireEvents(t, hub.snapshot()[before:])
	revealIdx := indexOfKind(events, ws.EventDeclarationsResolved)
	require.GreaterOrEqual(t, revealIdx, 0)
	melds, contested := declarationsResolvedBody(t, events[revealIdx].payload)

	want := make([]wireMeld, 0)
	perSeat := map[int]int{}
	for seat := 0; seat < 4; seat++ {
		for _, d := range after.Players[seat].Declarations {
			ids := make([]string, 0, len(d.Cards))
			for _, c := range d.Cards {
				ids = append(ids, string(c.Rank)+string(c.Suit))
			}
			want = append(want, wireMeld{PlayerSeat: d.PlayerSeat, Type: string(d.Type), Value: d.Value, Cards: ids})
			perSeat[seat]++
		}
	}

	maxPerSeat := 0
	for _, n := range perSeat {
		if n > maxPerSeat {
			maxPerSeat = n
		}
	}
	require.Greater(t, maxPerSeat, 1,
		"fixture must give at least one seat several melds, or this test cannot detect a collapse")

	assert.Equal(t, want, melds, "every meld must reach the wire, each carrying its own seat")
	assert.Len(t, melds, len(want))

	sum := 0
	for _, m := range melds {
		sum += m.Value
	}
	assert.Equal(t, after.DeclarationPoints[game.TeamA]+after.DeclarationPoints[game.TeamB], sum,
		"the rows must add up to what the engine awarded")

	assert.True(t, contested, "both teams declared, so a comparison decided the winner")
	losers := 0
	for seat := 0; seat < 4; seat++ {
		if game.TeamForSeat(seat) != *winningTeamOf(t, after) {
			losers += len(after.Players[seat].Declarations)
		}
	}
	require.Zero(t, losers,
		"resolution must have cleared the losing team's melds — otherwise this test does not "+
			"prove `contested` is derived from the pre-resolution state")
}

// TestDeclarationPhase_RevealReportsAnUncontestedWin checks the reveal reports an
// uncontested win when only one team declares.
func TestDeclarationPhase_RevealReportsAnUncontestedWin(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(119)
	mgr, gs := croatianDeclaringSession(t, hub, roomID, 60, 30*time.Second)

	state := gs
	for _, seat := range []int{1, 2, 3} {
		action := game.ActionDeclare
		if game.TeamForSeat(seat) == game.TeamA {
			action = game.ActionSkipDeclare
		}
		next, err := game.ApplyAction(state, game.Action{Type: action, PlayerSeat: seat})
		require.NoError(t, err)
		state = next
	}
	require.Equal(t, game.PhaseDeclaring, state.Phase, "seat 0 still owes an answer")
	require.Equal(t, game.TeamA, game.TeamForSeat(0), "seat 0 must be the second skipping seat")
	mgr.SetGameStateForTest(roomID, state)

	before := len(hub.snapshot())
	client := &ws.Client{UserID: state.Players[0].UserID}
	mgr.HandleAction(client, ws.WSMessage{Type: "action:skip_declare", Payload: []byte(`{}`)})

	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.Phase == game.PhasePlaying
	}, 2*time.Second, 5*time.Millisecond, "the fourth answer must open trick 1")

	events := wireEvents(t, hub.snapshot()[before:])
	revealIdx := indexOfKind(events, ws.EventDeclarationsResolved)
	require.GreaterOrEqual(t, revealIdx, 0)
	melds, contested := declarationsResolvedBody(t, events[revealIdx].payload)

	assert.False(t, contested, "only one team declared, so nothing was compared")
	require.NotEmpty(t, melds, "the declaring team's melds must still be revealed")
	assert.Greater(t, len(melds), 1)
}

// TestDeclarationPhase_RevealReportsAContestWonByAnEarlierSeat checks the reveal reports
// a contest whose losing team's only declarer answered last, which pre/post-action state cannot see.
func TestDeclarationPhase_RevealReportsAContestWonByAnEarlierSeat(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(120)
	mgr, gs := croatianDeclaringSession(t, hub, roomID, 60, 30*time.Second)

	require.Equal(t, game.TeamA, game.TeamForSeat(0))
	require.Equal(t, game.TeamA, game.TeamForSeat(2))
	state := gs
	for _, seat := range []int{1, 2, 3} {
		action := game.ActionDeclare
		if seat == 2 {
			action = game.ActionSkipDeclare
		}
		next, err := game.ApplyAction(state, game.Action{Type: action, PlayerSeat: seat})
		require.NoError(t, err)
		state = next
	}
	require.Equal(t, game.PhaseDeclaring, state.Phase, "seat 0 must be the last to answer")
	require.Empty(t, state.Players[0].Declarations,
		"seat 0 has not answered yet — its melds are absent from the pre-action state")
	mgr.SetGameStateForTest(roomID, state)

	before := len(hub.snapshot())
	client := &ws.Client{UserID: state.Players[0].UserID}
	mgr.HandleAction(client, ws.WSMessage{Type: "action:declare", Payload: []byte(`{}`)})

	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.Phase == game.PhasePlaying
	}, 2*time.Second, 5*time.Millisecond, "the fourth answer must open trick 1")

	after := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, after)
	require.Equal(t, game.TeamB, *winningTeamOf(t, after), "team B holds the stronger meld")
	require.Empty(t, after.Players[0].Declarations,
		"resolution cleared the loser's melds — so seat 0 is absent from the post-action state too, "+
			"which is what makes this row unsatisfiable from the before/after pair alone")

	events := wireEvents(t, hub.snapshot()[before:])
	revealIdx := indexOfKind(events, ws.EventDeclarationsResolved)
	require.GreaterOrEqual(t, revealIdx, 0)
	_, contested := declarationsResolvedBody(t, events[revealIdx].payload)

	assert.True(t, contested,
		"both teams put melds on the table, so a comparison decided the winner and the "+
			"reveal may name the deciding meld")
}
