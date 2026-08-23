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

// Session-manager coverage for the Croatian dedicated declaration phase
// (Story 12.6). The engine can be perfectly correct and the table still freeze:
// six switches and if-chains here treat an unrecognised phase as "do nothing"
// — timer arming, timer expiry, the auto-action chain, bot scheduling,
// disconnect handling, and the reconnect re-arm. None of them fails loudly, so
// these tests carry more weight than the detection logic.

// croatianDeclaringSession starts a per-move Croatian session and parks it in
// the declaration phase with all four seats unanswered.
//
// windowIn is added to now to produce the session's fixed declaration deadline:
// pass a negative span to model a window that has already elapsed, a positive
// one for tests that must observe an existing deadline being preserved. The
// phase carries NO TurnExpiresAt — nobody is on the clock — so that is asserted
// here rather than injected.
func croatianDeclaringSession(
	t *testing.T,
	hub match.Broadcaster,
	roomID uint,
	timerSec int,
	windowIn time.Duration,
) (*match.Manager, *game.GameState) {
	t.Helper()

	mgr := match.NewManager(hub, newMockMatchRepo())
	// The session's own variant must match the state it runs, or session-level
	// code that reads it is exercised in the wrong configuration.
	require.NoError(t, mgr.StartMatch(roomID, "croatia", "1001", defaultPlayers(), "per-move", timerSec, 10, 120, 0))
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

// --- Wire helpers ---

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

// indexOfMatchStateWithPhase is the position of the first event:match_state
// carrying the given phase, or -1.
//
// Deliberately takes no *testing.T and asserts nothing: it is called from inside
// a require.Eventually condition, and testify runs that condition on its own
// goroutine. A require in there would call t.FailNow off the test goroutine,
// which Go's testing package does not support. A payload that will not decode
// simply is not a match.
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

// assertRevealPrecedesPlayingState is the wire contract the whole handoff rests
// on: the reveal must reach clients BEFORE the match_state that starts trick 1,
// because the client's declRevealReady latch expects them in that order.
//
// It also proves the fire-once latch was actually consumed by a broadcast arm
// that emits — deleting the broadcastDeclarationsResolvedIfTransition call from
// the declare/skip arm leaves every other assertion in this package green.
// It takes the spy and a watermark rather than a pre-taken slice because it has
// to WAIT for those two events, not merely look for them. handleTimerExpiry
// assigns session.gameState and unlocks BEFORE it broadcasts anything
// (live_match.go), so a test that polls GetStateSnapshot for PhasePlaying can win
// the race against the broadcasts by a few milliseconds and read a hub that is
// still empty. Synchronising on the state while asserting on the wire is what
// made TestDeclarationPhase_TimerExpiryOnLastSeatOpensTrick1 fail 19 runs out of
// 30 in isolation, with a bare "-1 is not greater than or equal to 0". Wait for
// the thing you assert.
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

// --- Tests ---

// TestDeclarationPhase_WindowForceClosesTheContest pins the matrix row for an
// elapsed window: every unanswered seat is treated as having skipped, the
// contest resolves, and trick 1 opens with a live per-move timer. The phase runs
// no per-move timer of its own, so this fallback is its ONLY clock — without it
// an idle table sits in the phase forever.
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

	// Nobody declared, so the reveal names no winner — but it must still fire,
	// and still ride ahead of the trick-1 state.
	assertRevealPrecedesPlayingState(t, hub, before, nil)
}

// TestDeclarationPhase_WindowKeepsAnswersAlreadyGiven is the other half of the
// force-close: seats that DID answer keep what they said.
func TestDeclarationPhase_WindowKeepsAnswersAlreadyGiven(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(101)
	mgr, gs := croatianDeclaringSession(t, hub, roomID, 60, -time.Second)

	// Seats 1-3 declare; seat 0 never answers and is force-closed.
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

// TestDeclarationPhase_DeclareIsNotAnnouncedDuringThePhase is the information-
// hiding rule at the wire. Bitola broadcasts event:player_declared the moment a
// declare commits, so the table learns a meld exists; here that would out
// whoever clicked first, before the contest has even decided whose melds become
// public — and the losing team's never do. The reveal that follows is the whole
// announcement.
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

// TestDeclarationPhase_OtherSeatsMeldsStayMaskedMidPhase is the same rule one
// layer down: even though the melds are on the server the instant a seat
// declares, the per-recipient projection must keep them off every other seat's
// snapshot until the contest resolves.
func TestDeclarationPhase_OtherSeatsMeldsStayMaskedMidPhase(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(122)
	_, gs := croatianDeclaringSession(t, hub, roomID, 60, 30*time.Second)

	state := answerAll(t, gs, []int{1}, game.ActionDeclare)
	require.NotEmpty(t, state.Players[1].Declarations)
	require.False(t, state.DeclarationsResolved)

	// Seat 2 is an opponent of seat 1 and has not answered.
	seen := game.ProjectForSeat(state, 2)
	assert.Empty(t, seen.Players[1].Declarations, "seat 1's melds must not reach seat 2")
	assert.True(t, seen.Players[1].DeclarationAnswered,
		"that seat 1 has ANSWERED is public — it says nothing about what they hold")
	assert.False(t, seen.Players[3].DeclarationAnswered)
}

// TestDeclarationPhase_NoMeldHandStillOpensThePhase is the reversal that closes
// the last leak: when NOBODY holds a meld the phase still opens and still asks
// all four seats. A phase that resolved instantly in that case would announce,
// by its very absence, that somebody holds one.
func TestDeclarationPhase_NoMeldHandStillOpensThePhase(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(107)

	mgr := match.NewManager(hub, newMockMatchRepo())
	require.NoError(t, mgr.StartMatch(roomID, "croatia", "1001", defaultPlayers(), "per-move", 60, 10, 120, 0))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	// A Croatian hand one action short of a resolved bid, with a layout in which
	// no seat holds a meld: every suit contributes two non-adjacent ranks per
	// seat, and no rank lands four times in one hand.
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

	// And it closes normally once they do.
	state := answerAll(t, opened, []int{1, 2, 3, 0}, game.ActionSkipDeclare)
	assert.Equal(t, game.PhasePlaying, state.Phase)
	assert.Equal(t, [2]int{0, 0}, state.DeclarationPoints)
}

// TestDeclarationPhase_ActionDoesNotExtendTheWindow guards the fixed-deadline
// rule in HandleAction's PhaseDeclaring arm. The deadline is set on ENTRY only;
// if each answer refreshed it, three prompt players could keep extending the
// wait on a fourth who never answers.
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

// TestDeclarationPhase_RejectedActionKeepsTheWindowArmed is the error path of
// the same arm. HandleAction cancels the turn timer BEFORE applying, so a
// rejected action — a second answer from a seat that already spoke is the common
// one — would permanently disarm the phase's only clock unless the error branch
// re-creates it against the same deadline.
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

	// Same seat answers again — rejected, and the rejection must not disarm the
	// window that is about to fire.
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

	// Seat 3 proposes; its partner is seat 1.
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

	// And the phase still resolves normally afterwards.
	mgr.HandleAction(partner, ws.WSMessage{Type: "action:skip_declare", Payload: []byte(`{}`)})
	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.Players[1].DeclarationAnswered
	}, 2*time.Second, 5*time.Millisecond, "the phase must still accept answers after the surrender detour")
}

// TestDeclarationPhase_IsNotPausable pins the phase OUT of the pause allowlist.
// It is a fixed-length window with no active turn and no TurnExpiresAt, so
// pause/unpause — whose whole job is preserving and restoring TurnTimeRemaining
// — has nothing to carry.
func TestDeclarationPhase_IsNotPausable(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(103)
	mgr, gs := croatianDeclaringSession(t, hub, roomID, 60, 30*time.Second)

	pauser := &ws.Client{UserID: gs.Players[3].UserID}
	mgr.HandleAction(pauser, ws.WSMessage{Type: "action:pause", Payload: []byte(`{}`)})

	// Nothing to wait for — assert the phase never changes.
	assert.Never(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.Phase == game.PhasePaused
	}, 200*time.Millisecond, 10*time.Millisecond, "the phase must not be pausable")

	after := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, after)
	assert.Equal(t, game.PhaseDeclaring, after.Phase)
	assert.False(t, after.PauseUsed[3], "a refused pause must not spend the player's one pause")
}

// TestDeclarationPhase_DisconnectAndReconnectRestoreThePhase is the matrix's
// disconnect/reconnect pair. HandleDisconnect's phase switch has a silent
// `default` arm: without the phase listed, a drop opens no reconnect window at
// all and leaves the seat marked Connected — so the close gate, which counts
// connected seats, waits out the whole window for an answer that can never come.
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

	// They can still answer, and the phase still closes.
	client := &ws.Client{UserID: droppedUserID}
	mgr.HandleAction(client, ws.WSMessage{Type: "action:declare", Payload: []byte(`{}`)})
	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.Players[droppedSeat].DeclarationAnswered
	}, 2*time.Second, 5*time.Millisecond, "the returning player's answer must be accepted")
}

// TestDeclarationPhase_AnsweredSeatCannotAnswerAgain is the reconnect row's
// fairness half: a client that rejoins mid-phase re-renders its dialog from the
// snapshot, and the answered flag is what stops a skip being upgraded to a
// declare after the fact.
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

// TestDeclarationPhase_AllBotSeatsAnswer pins the bot scheduler. Under the
// simultaneous phase botDecisionSeats must return EVERY unanswered bot seat, not
// one: scheduling a single seat would leave the rest silent until the window
// elapsed, turning every hand with bots into a full-ceiling stall.
func TestDeclarationPhase_AllBotSeatsAnswer(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(105)

	mgr := match.NewManager(hub, newMockMatchRepo())
	mgr.SetBotDelayForTest(5*time.Millisecond, 10*time.Millisecond)
	require.NoError(t, mgr.StartMatch(roomID, "croatia", "1001", mixedPlayers(1, 2, 3), "relaxed", 0, 10, 120, 0))
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

// TestDeclarationPhase_RevealCarriesEveryMeldPerSeat closes the gap the
// broadcast-loop deferral named: the producing loop flattens several melds per
// seat into the payload, and nothing asserted the resulting array. Collapsing
// that loop to the first meld per seat kept every other test in this package
// green — while a Croatian hand scored 300 and the reveal rendered one 100 row.
//
// The expectations are derived from the resolved engine state rather than
// hand-copied, so a meld-value change moves both sides together instead of
// turning this into a stale constant.
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

	// What the engine actually kept, flattened the same way.
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

	// Guard the guard: if the fixture ever stops producing a multi-meld seat,
	// this test would still pass while proving nothing about flattening.
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

	// The awarded total the reveal renders is the sum of the rows it carries.
	sum := 0
	for _, m := range melds {
		sum += m.Value
	}
	assert.Equal(t, after.DeclarationPoints[game.TeamA]+after.DeclarationPoints[game.TeamB], sum,
		"the rows must add up to what the engine awarded")

	// Every seat declared, so both teams put melds on the table and the reveal
	// may name the deciding one.
	//
	// This also pins WHERE the flag is computed. Resolution has already deleted
	// the losing team's melds — assert that, so the point is not theoretical —
	// which means an implementation reading the post-resolution state would see
	// only the winner and report `false` here. It has to read the pre-resolution
	// clone.
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

// TestDeclarationPhase_RevealReportsAnUncontestedWin is the other half: when
// only ONE team declares, nothing was compared, and the reveal must say so.
// Without the flag the client cannot tell the two apart — it only ever receives
// the winner's melds — and under the Croatian overlap rule a single seat
// holding several melds is the ordinary shape rather than evidence of a clash.
func TestDeclarationPhase_RevealReportsAnUncontestedWin(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(119)
	mgr, gs := croatianDeclaringSession(t, hub, roomID, 60, 30*time.Second)

	// Seats 1 and 3 are team B and hold the melds; seats 2 and 0 are team A and
	// SKIP, so exactly one team ever declares.
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
	// The uncontested win is still a multi-meld payload — which is exactly why
	// meld count could never have stood in for "was this contested".
	assert.Greater(t, len(melds), 1)
}

// TestDeclarationPhase_RevealReportsAContestWonByAnEarlierSeat is the row the
// other two `contested` tests structurally could not fail on.
//
// Both of them let the LAST seat to answer be redundant: one has every earlier
// seat declare (so both teams are on the table before the final answer), the
// other has only one team declare at all. A `contested` derived from the
// pre-action state passes both.
//
// Here the last answerer is the ONLY declarer on its team, which is the shape
// that breaks that derivation: at broadcast time seat 0's melds exist in
// neither the pre-action state (it had not answered yet) nor the post-action
// state (its team lost, so resolution cleared them). Only the engine, inside
// resolveDeclarationsForHand, ever sees both teams at once — which is why
// GameState.DeclarationsContested is computed there and merely read here.
func TestDeclarationPhase_RevealReportsAContestWonByAnEarlierSeat(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(120)
	mgr, gs := croatianDeclaringSession(t, hub, roomID, 60, 30*time.Second)

	// Seats 1 and 3 are team B and declare; seat 2 (team A) skips, and seat 0 is
	// held back so that team A's ONLY declarer is the seat whose answer closes
	// the phase.
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
