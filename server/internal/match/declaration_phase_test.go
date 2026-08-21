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
// the declaration phase with the cursor on its opening seat.
//
// deadlineIn is added to now to produce the injected TurnExpiresAt: pass a
// negative span for the timer-expiry tests (an already-elapsed deadline is what
// makes "the next seat got a FRESH window" a real assertion) and a positive one
// for tests that must observe an existing deadline being preserved.
func croatianDeclaringSession(
	t *testing.T,
	hub match.Broadcaster,
	roomID uint,
	timerSec int,
	deadlineIn time.Duration,
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
	require.True(t, gs.AwaitingDeclaration)
	require.Equal(t, 1, gs.ActivePlayerSeat, "the fixture opens the cursor on seat 1")

	deadline := time.Now().Add(deadlineIn)
	gs.TurnExpiresAt = &deadline
	gs.TimerDurationSec = timerSec
	mgr.SetGameStateForTest(roomID, gs)

	return mgr, gs
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

// TestDeclarationPhase_TimerExpiryAutoSkipsAndContinues pins the matrix row for
// an expired prompt: the seat is auto-skipped, the phase moves on to the next
// seat, and a FRESH timer is armed for it. Without the phase in
// handleTimerExpiry's chain the default arm returns with no timer at all and the
// table freezes forever.
func TestDeclarationPhase_TimerExpiryAutoSkipsAndContinues(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(100)
	mgr, gs := croatianDeclaringSession(t, hub, roomID, 60, -time.Second)
	promptedSeat := gs.ActivePlayerSeat

	mgr.TriggerTimerExpiryForTest(roomID, promptedSeat, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.ActivePlayerSeat != promptedSeat
	}, 2*time.Second, 5*time.Millisecond, "the auto-skip must advance the cursor")

	after := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, after)
	assert.Equal(t, game.PhaseDeclaring, after.Phase, "the phase continues to the next seat")
	assert.True(t, after.AwaitingDeclaration, "the next meld-holder is prompted")
	assert.Equal(t, 2, after.ActivePlayerSeat, "the cursor walks counter-clockwise")
	assert.Empty(t, after.Players[promptedSeat].Declarations, "an auto-skip stores nothing")

	require.NotNil(t, after.TurnExpiresAt, "the next seat must have a clock")
	assert.True(t, after.TurnExpiresAt.After(time.Now()),
		"a fresh window, never the elapsed deadline the expiry fired on")

	events := wireEvents(t, hub.snapshot())
	autoIdx := indexOfKind(events, ws.EventAutoAction)
	require.GreaterOrEqual(t, autoIdx, 0, "the timed-out player must be named to the table")
	var auto ws.AutoActionPayload
	require.NoError(t, json.Unmarshal(events[autoIdx].payload, &auto))
	assert.Equal(t, ws.AutoActionSkipDeclare, auto.Type)
	assert.Equal(t, promptedSeat, auto.PlayerSeat)
	assert.Equal(t, -1, indexOfKind(events, ws.EventDeclarationsResolved),
		"the contest is still open — nothing may be revealed yet")
}

// TestDeclarationPhase_TimerExpiryOnLastSeatOpensTrick1 covers the handoff under
// timer expiry rather than a player action: the fourth auto-skip must resolve
// the contest, emit the reveal ahead of the playing-phase state, start trick 1,
// and arm the leader's timer.
func TestDeclarationPhase_TimerExpiryOnLastSeatOpensTrick1(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(101)
	mgr, gs := croatianDeclaringSession(t, hub, roomID, 60, -time.Second)

	// Walk to the last seat of the cursor's rotation by hand, leaving one answer
	// outstanding for the timer to supply. Seats 1-3 DECLARE so the contest has a
	// real winner and the reveal payload is non-trivial.
	state := gs
	for _, seat := range []int{1, 2, 3} {
		require.Equal(t, seat, state.ActivePlayerSeat)
		next, err := game.ApplyAction(state, game.Action{Type: game.ActionDeclare, PlayerSeat: seat})
		require.NoError(t, err)
		state = next
	}
	require.Equal(t, game.PhaseDeclaring, state.Phase)
	require.Equal(t, 0, state.ActivePlayerSeat, "the dealer answers last")

	elapsed := time.Now().Add(-time.Second)
	state.TurnExpiresAt = &elapsed
	state.TimerDurationSec = 60
	mgr.SetGameStateForTest(roomID, state)

	before := len(hub.snapshot())
	mgr.TriggerTimerExpiryForTest(roomID, 0, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.Phase == game.PhasePlaying
	}, 2*time.Second, 5*time.Millisecond, "the last auto-skip must open trick 1")

	after := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, after)
	assert.Equal(t, 1, after.TrickNumber)
	assert.True(t, after.DeclarationsResolved)
	assert.False(t, after.AwaitingDeclaration)
	assert.Equal(t, (after.DealerSeat+1)%4, after.ActivePlayerSeat, "the leader is on the clock")
	require.NotNil(t, after.TurnExpiresAt)
	assert.True(t, after.TurnExpiresAt.After(time.Now()), "trick 1 opens with a live timer")

	assertRevealPrecedesPlayingState(t, hub, before, winningTeamOf(t, after))
}

// TestDeclarationPhase_FinalAnswerRevealsBeforePlayingState is the same wire
// contract on the player-driven path: the fourth seat's own declare closes the
// contest, so the reveal and the trick-1 state ride out of a single action.
func TestDeclarationPhase_FinalAnswerRevealsBeforePlayingState(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(106)
	mgr, gs := croatianDeclaringSession(t, hub, roomID, 60, 30*time.Second)

	state := gs
	for _, seat := range []int{1, 2, 3} {
		require.Equal(t, seat, state.ActivePlayerSeat)
		next, err := game.ApplyAction(state, game.Action{Type: game.ActionDeclare, PlayerSeat: seat})
		require.NoError(t, err)
		state = next
	}
	require.Equal(t, 0, state.ActivePlayerSeat)
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
	events := wireEvents(t, hub.snapshot()[before:])
	assert.GreaterOrEqual(t, indexOfKind(events, ws.EventPlayerDeclared), 0,
		"a manual declare still announces who declared")
}

// TestDeclarationPhase_NoMeldHandStillRevealsOnPick is the edge the pick_trump
// broadcast arm owns: when NO seat holds a meld the phase opens and closes
// inside the pick_trump transition, so DeclarationsResolved flips there and the
// fire-once latch must be spent by an arm that actually emits. Otherwise the
// hand silently loses its reveal while the equivalent Bitola hand fires one.
func TestDeclarationPhase_NoMeldHandStillRevealsOnPick(t *testing.T) {
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
		return st != nil && st.Phase == game.PhasePlaying
	}, 2*time.Second, 5*time.Millisecond, "a meld-less hand goes straight to trick 1")

	after := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, after)
	assert.Equal(t, 1, after.TrickNumber)
	assert.True(t, after.DeclarationsResolved, "the contest resolved with nothing declared")
	assert.Equal(t, [2]int{0, 0}, after.DeclarationPoints)

	// No team scored, so the reveal names no winner — but it must still fire.
	assertRevealPrecedesPlayingState(t, hub, before, nil)
}

// TestDeclarationPhase_ActionArmsTimerForNextSeat guards the timer-arm branch in
// HandleAction: a declare/skip that advances the cursor must issue a fresh
// window for the next seat, not silently leave the phase unclocked.
func TestDeclarationPhase_ActionArmsTimerForNextSeat(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(102)
	mgr, gs := croatianDeclaringSession(t, hub, roomID, 60, -time.Second)

	client := &ws.Client{UserID: gs.Players[gs.ActivePlayerSeat].UserID}
	mgr.HandleAction(client, ws.WSMessage{Type: "action:skip_declare", Payload: []byte(`{}`)})

	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.ActivePlayerSeat == 2
	}, 2*time.Second, 5*time.Millisecond, "the answer must advance the cursor")

	after := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, after)
	assert.Equal(t, game.PhaseDeclaring, after.Phase)
	require.NotNil(t, after.TurnExpiresAt)
	assert.True(t, after.TurnExpiresAt.After(time.Now()),
		"the next seat gets a fresh window, not the previous seat's elapsed one")
}

// TestDeclarationPhase_SurrenderDeclinePreservesTheTurnBudget is the other half
// of the same timer branch — the WITHIN-turn one. A surrender request and its
// decline leave both seat and phase unchanged, so preserveTimer must keep the
// ORIGINAL deadline: minting a fresh window would let any client extend the
// prompted player's turn indefinitely by proposing and declining.
func TestDeclarationPhase_SurrenderDeclinePreservesTheTurnBudget(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(108)
	mgr, gs := croatianDeclaringSession(t, hub, roomID, 60, 30*time.Second)
	cursorSeat := gs.ActivePlayerSeat
	originalDeadline := *gs.TurnExpiresAt

	// Seat 3 proposes; its partner is seat 1 — the seat currently on the clock,
	// which is exactly the budget that must not be refreshed.
	proposer := &ws.Client{UserID: gs.Players[3].UserID}
	mgr.HandleAction(proposer, ws.WSMessage{Type: "action:surrender_request", Payload: []byte(`{}`)})

	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.SurrenderProposerSeat != nil
	}, 2*time.Second, 5*time.Millisecond, "surrender must be proposable inside the phase")

	proposed := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, proposed)
	assert.Equal(t, game.PhaseDeclaring, proposed.Phase, "proposing does not leave the phase")
	assert.True(t, proposed.AwaitingDeclaration, "the prompt survives the proposal")
	require.NotNil(t, proposed.TurnExpiresAt)
	assert.True(t, proposed.TurnExpiresAt.Equal(originalDeadline),
		"a proposal must not mint a fresh window for the prompted seat")

	partner := &ws.Client{UserID: gs.Players[cursorSeat].UserID}
	mgr.HandleAction(partner, ws.WSMessage{Type: "action:surrender_decline", Payload: []byte(`{}`)})

	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.SurrenderProposerSeat == nil
	}, 2*time.Second, 5*time.Millisecond, "the decline must clear the proposal")

	declined := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, declined)
	assert.Equal(t, game.PhaseDeclaring, declined.Phase)
	assert.Equal(t, cursorSeat, declined.ActivePlayerSeat, "the cursor never moved")
	assert.True(t, declined.AwaitingDeclaration, "the declaration is still owed")
	require.NotNil(t, declined.TurnExpiresAt)
	assert.True(t, declined.TurnExpiresAt.Equal(originalDeadline),
		"declining must preserve the original deadline, not restart the turn")

	// And the phase still resolves normally afterwards.
	mgr.HandleAction(partner, ws.WSMessage{Type: "action:skip_declare", Payload: []byte(`{}`)})
	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.ActivePlayerSeat != cursorSeat
	}, 2*time.Second, 5*time.Millisecond, "the phase must still advance after the surrender detour")
}

// TestDeclarationPhase_IsPausable pins the phase into the pause allowlist and
// checks the session manager restores it — a pause that silently failed here
// would also break disconnect handling, which rides on the same path.
func TestDeclarationPhase_IsPausable(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(103)
	mgr, gs := croatianDeclaringSession(t, hub, roomID, 60, -time.Second)
	cursorSeat := gs.ActivePlayerSeat

	// Pause from a seat that is NOT on the clock — pausing must not depend on
	// owning the turn.
	pauser := &ws.Client{UserID: gs.Players[3].UserID}
	mgr.HandleAction(pauser, ws.WSMessage{Type: "action:pause", Payload: []byte(`{}`)})

	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.Phase == game.PhasePaused
	}, 2*time.Second, 5*time.Millisecond, "the phase must be pausable")

	paused := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, paused)
	assert.Equal(t, game.PhaseDeclaring, paused.PreviousPhase)
	assert.Nil(t, paused.TurnExpiresAt, "the turn clock is held during the pause")

	mgr.HandleAction(pauser, ws.WSMessage{Type: "action:unpause", Payload: []byte(`{}`)})

	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.Phase == game.PhaseDeclaring
	}, 2*time.Second, 5*time.Millisecond, "unpause must restore the declaration phase")

	resumed := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, resumed)
	assert.Equal(t, cursorSeat, resumed.ActivePlayerSeat, "the cursor survives the pause")
	assert.True(t, resumed.AwaitingDeclaration)
	require.NotNil(t, resumed.TurnExpiresAt, "the clock restarts on resume")
	assert.True(t, resumed.TurnExpiresAt.After(time.Now()))
}

// TestDeclarationPhase_DisconnectAndReconnectRestoreCursor is the matrix's
// disconnect/reconnect pair. HandleDisconnect's phase switch has a silent
// `default` arm: without the phase listed, a drop opens no reconnect window at
// all, leaves the seat marked Connected, and the table waits on a player who is
// gone.
func TestDeclarationPhase_DisconnectAndReconnectRestoreCursor(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(104)
	mgr, gs := croatianDeclaringSession(t, hub, roomID, 60, -time.Second)
	cursorSeat := gs.ActivePlayerSeat
	droppedUserID := gs.Players[cursorSeat].UserID

	mgr.HandleDisconnect(droppedUserID)

	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.Phase == game.PhaseDisconnected
	}, 2*time.Second, 5*time.Millisecond, "a drop in the phase must open a reconnect window")

	dropped := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, dropped)
	assert.Equal(t, game.PhaseDeclaring, dropped.PreviousPhase, "the phase is the restore target")
	assert.False(t, dropped.Players[cursorSeat].Connected)
	assert.Equal(t, cursorSeat, dropped.DisconnectedSeat)
	assert.NotNil(t, dropped.ReconnectExpiresAt)
	assert.Nil(t, dropped.TurnExpiresAt, "the turn clock is frozen for the outage")

	mgr.HandleReconnect(droppedUserID)

	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.Phase == game.PhaseDeclaring
	}, 2*time.Second, 5*time.Millisecond, "reconnect must restore the phase")

	back := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, back)
	assert.Equal(t, cursorSeat, back.ActivePlayerSeat, "the cursor is exactly where it was")
	assert.True(t, back.AwaitingDeclaration, "the returning player still owes an answer")
	assert.True(t, back.Players[cursorSeat].Connected)
	assert.Equal(t, -1, back.DisconnectedSeat)
	require.NotNil(t, back.TurnExpiresAt, "their turn timer is re-armed")
	assert.True(t, back.TurnExpiresAt.After(time.Now()))
}

// TestDeclarationPhase_BotAnswersPrompt pins the bot scheduler: botDecisionSeats
// returns an empty list for any phase it does not name, and an empty list stalls
// the table with no error and no log line.
func TestDeclarationPhase_BotAnswersPrompt(t *testing.T) {
	hub := &hubSpy{}
	const roomID = uint(105)

	mgr := match.NewManager(hub, newMockMatchRepo())
	mgr.SetBotDelayForTest(5*time.Millisecond, 10*time.Millisecond)
	require.NoError(t, mgr.StartMatch(roomID, "croatia", "1001", mixedPlayers(1), "relaxed", 0, 10, 120, 0))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	gs := markBots(testfixtures.NewGameCroatianDeclaring(game.SuitHearts), 1)
	gs.RoomID = roomID
	require.Equal(t, 1, gs.ActivePlayerSeat, "the bot seat is the one on the clock")
	mgr.SetGameStateForTest(roomID, gs)
	mgr.BotSchedule(roomID)

	require.Eventually(t, func() bool {
		st := mgr.GetStateSnapshot(roomID)
		return st != nil && st.ActivePlayerSeat != 1
	}, 2*time.Second, 5*time.Millisecond, "the bot must answer its declaration prompt")

	after := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, after)
	assert.NotEmpty(t, after.Players[1].Declarations, "the bot declares whenever it can")
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

	state := gs
	for _, seat := range []int{1, 2, 3} {
		next, err := game.ApplyAction(state, game.Action{Type: game.ActionDeclare, PlayerSeat: seat})
		require.NoError(t, err)
		state = next
	}
	require.Equal(t, 0, state.ActivePlayerSeat)
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
	require.Equal(t, 0, state.ActivePlayerSeat)
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

	// Cursor order is 1 → 2 → 3 → 0. Seats 1 and 3 are team B and declare;
	// seat 2 (team A) skips, so seat 0 — answering last — is team A's only
	// declarer.
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
	require.Equal(t, 0, state.ActivePlayerSeat, "seat 0 must be the last to answer")
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
