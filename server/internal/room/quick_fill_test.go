package room_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/room"
)

// --- Capability-providing stubs -------------------------------------------------
//
// The idle-lobby cadence read type-asserts the hub for ConnectedUserIDs() and
// the match starter for InMatchUserIDs(). These embed the existing fakes and add
// exactly those methods so a scheduler test can force the fast/patient decision.

type capabilityStarter struct {
	*fakeMatchStarter
	inMatch []uint
}

func (c *capabilityStarter) InMatchUserIDs() []uint { return c.inMatch }

type capabilityHub struct {
	*mockBroadcaster
	connected []uint
}

func (c *capabilityHub) ConnectedUserIDs() []uint { return c.connected }

// hugeInterval keeps auto-re-armed real timers from ever firing mid-test; ticks
// are driven manually via TriggerQuickFillTick / InvokeQuickFillTick.
const hugeInterval = time.Hour

// seedCroatiaQuickPlayRoom creates a waiting croatia/501 quick-play room with the
// given humans seated at seats 0..n-1 and returns it.
func seedCroatiaQuickPlayRoom(repo *mockRoomRepo, code string, buyIn int, seatedUserIDs ...uint) *room.Room {
	r := &room.Room{
		Name:        "Quick Play " + code,
		Code:        code,
		OwnerID:     seatedUserIDs[0],
		Variant:     "croatia",
		MatchMode:   "501",
		TimerStyle:  "per-move",
		IsQuickPlay: true,
		Status:      "waiting",
		PlayerCount: len(seatedUserIDs),
		CoinBuyIn:   buyIn,
	}
	_ = repo.Create(r)
	for i, uid := range seatedUserIDs {
		seat := i
		team := "teamA"
		if seat%2 == 1 {
			team = "teamB"
		}
		_ = repo.AddPlayer(&room.RoomPlayer{RoomID: r.ID, UserID: uid, Seat: &seat, Team: &team, Username: "H"})
	}
	return r
}

func countBots(repo *mockRoomRepo, roomID uint) int {
	bots, _ := repo.FindBotsByRoomID(roomID)
	return len(bots)
}

// TestQuickFill_FastFill_ReachesFourAndStarts covers the fast (nobody idle)
// cadence: each tick seats a bot in the lowest empty seat, and the tick that
// fills the fourth seat auto-starts the match with 1 human + 3 bots — the human
// seat carries the real identity, the three bot seats are flagged IsBot, and
// only the human is charged.
func TestQuickFill_FastFill_ReachesFourAndStarts(t *testing.T) {
	repo := newMockRoomRepo()
	starter := &capabilityStarter{fakeMatchStarter: &fakeMatchStarter{}}
	hub := &capabilityHub{mockBroadcaster: &mockBroadcaster{}, connected: []uint{10}} // only the initiator online
	wallet := &stubWallet{balance: 1000}
	h := room.NewRoomHandler(repo, starter, hub, nil, wallet, nil, nil)
	h.SetQuickFillIntervals(hugeInterval, 2*hugeInterval)

	r := seedCroatiaQuickPlayRoom(repo, "FASTQP", 500, 10)

	h.StartQuickFill(r.ID, 10)
	defer h.CancelQuickFill(r.ID)

	_, interval, armed := h.QuickFillState(r.ID)
	require.True(t, armed, "scheduler must be armed after StartQuickFill")
	assert.Equal(t, hugeInterval, interval, "0 idle players selects the fast cadence")

	h.TriggerQuickFillTick(r.ID) // seat 1
	assert.Equal(t, 1, countBots(repo, r.ID))
	h.TriggerQuickFillTick(r.ID) // seat 2
	assert.Equal(t, 2, countBots(repo, r.ID))

	updated, _ := repo.FindByID(r.ID)
	assert.Equal(t, "waiting", updated.Status, "not yet full — still waiting")
	assert.Equal(t, 0, starter.called)

	h.TriggerQuickFillTick(r.ID) // seat 3 → full → auto-start

	assert.Equal(t, 3, countBots(repo, r.ID))
	updated, _ = repo.FindByID(r.ID)
	assert.Equal(t, "playing", updated.Status, "the fourth occupant auto-starts the match")
	require.Equal(t, 1, starter.called, "StartMatch runs exactly once")

	// Seat info: seat 0 human, seats 1..3 bots.
	assert.Equal(t, uint(10), starter.lastPlayers[0].UserID)
	assert.False(t, starter.lastPlayers[0].IsBot, "seat 0 is the human")
	for seat := 1; seat <= 3; seat++ {
		assert.True(t, starter.lastPlayers[seat].IsBot, "seat %d must be a bot", seat)
		assert.Equal(t, uint(0), starter.lastPlayers[seat].UserID, "bot seat %d has no user id", seat)
	}

	// Only the human is charged.
	assert.Equal(t, 1, wallet.chargeCalls)
	assert.Equal(t, []uint{10}, wallet.chargedIDs, "only the human seat pays the stake")
	assert.Equal(t, 500, wallet.chargedAmount)

	// Scheduler cancelled on start.
	_, _, armed = h.QuickFillState(r.ID)
	assert.False(t, armed, "the scheduler is cancelled once the match starts")
}

// TestQuickFill_MixedHumansAndBotsAutoStarts covers a partially-human room: two
// humans already seated, the scheduler fills the remaining two seats with bots
// and auto-starts. Both humans are charged (only the humans), the two bot seats
// are flagged, and the scheduler cancels on start.
func TestQuickFill_MixedHumansAndBotsAutoStarts(t *testing.T) {
	repo := newMockRoomRepo()
	starter := &capabilityStarter{fakeMatchStarter: &fakeMatchStarter{}}
	hub := &capabilityHub{mockBroadcaster: &mockBroadcaster{}, connected: []uint{10, 11}}
	wallet := &stubWallet{balance: 1000}
	h := room.NewRoomHandler(repo, starter, hub, nil, wallet, nil, nil)
	h.SetQuickFillIntervals(hugeInterval, 2*hugeInterval)

	// Two humans seated at seats 0 and 1.
	r := seedCroatiaQuickPlayRoom(repo, "MIXEDQ", 500, 10, 11)

	h.StartQuickFill(r.ID, 10)
	defer h.CancelQuickFill(r.ID)

	h.TriggerQuickFillTick(r.ID) // seat 2 (bot)
	assert.Equal(t, 1, countBots(repo, r.ID))
	updated, _ := repo.FindByID(r.ID)
	assert.Equal(t, "waiting", updated.Status, "3 occupants — still waiting")
	assert.Equal(t, 0, starter.called)

	h.TriggerQuickFillTick(r.ID) // seat 3 (bot) → full → auto-start

	assert.Equal(t, 2, countBots(repo, r.ID))
	updated, _ = repo.FindByID(r.ID)
	assert.Equal(t, "playing", updated.Status, "the fourth occupant auto-starts the match")
	require.Equal(t, 1, starter.called, "StartMatch runs exactly once")

	// Seats 0 and 1 are the two humans; seats 2 and 3 are bots.
	assert.False(t, starter.lastPlayers[0].IsBot)
	assert.Equal(t, uint(10), starter.lastPlayers[0].UserID)
	assert.False(t, starter.lastPlayers[1].IsBot)
	assert.Equal(t, uint(11), starter.lastPlayers[1].UserID)
	for seat := 2; seat <= 3; seat++ {
		assert.True(t, starter.lastPlayers[seat].IsBot, "seat %d must be a bot", seat)
		assert.Equal(t, uint(0), starter.lastPlayers[seat].UserID, "bot seat %d has no user id", seat)
	}

	// Both humans are charged exactly once, at the room stake; bots never pay.
	assert.Equal(t, 1, wallet.chargeCalls)
	assert.ElementsMatch(t, []uint{10, 11}, wallet.chargedIDs, "both human seats pay, and only them")
	assert.Equal(t, 500, wallet.chargedAmount)

	_, _, armed := h.QuickFillState(r.ID)
	assert.False(t, armed, "the scheduler is cancelled once the match starts")
}

// TestQuickFill_PatientCadenceSelected verifies that at least one idle lobby
// player selects the patient cadence, and each quiet interval adds exactly one
// bot.
func TestQuickFill_PatientCadenceSelected(t *testing.T) {
	repo := newMockRoomRepo()
	starter := &capabilityStarter{fakeMatchStarter: &fakeMatchStarter{}}
	// User 10 is the initiator (seated); user 99 is connected and idle (in no
	// room, no match) → one idle player online.
	hub := &capabilityHub{mockBroadcaster: &mockBroadcaster{}, connected: []uint{10, 99}}
	h := room.NewRoomHandler(repo, starter, hub, nil, nil, nil, nil)
	fast := hugeInterval
	patient := 2 * hugeInterval
	h.SetQuickFillIntervals(fast, patient)

	r := seedCroatiaQuickPlayRoom(repo, "PATQP1", 0, 10)

	require.Equal(t, 1, h.IdleLobbyCount(10), "user 99 is idle; the initiator is excluded")

	h.StartQuickFill(r.ID, 10)
	defer h.CancelQuickFill(r.ID)

	_, interval, armed := h.QuickFillState(r.ID)
	require.True(t, armed)
	assert.Equal(t, patient, interval, "≥1 idle player selects the patient cadence")

	h.TriggerQuickFillTick(r.ID)
	assert.Equal(t, 1, countBots(repo, r.ID), "one quiet interval adds exactly one bot")

	h.TriggerQuickFillTick(r.ID)
	assert.Equal(t, 2, countBots(repo, r.ID), "a second quiet interval adds a second bot")

	updated, _ := repo.FindByID(r.ID)
	assert.Equal(t, "waiting", updated.Status)
}

// TestQuickFill_HumanJoinResetsTimer proves resetQuickFill restarts the
// inactivity countdown (a fresh generation) while keeping the chosen cadence,
// and that the superseded (stale-generation) callback is inert.
func TestQuickFill_HumanJoinResetsTimer(t *testing.T) {
	repo := newMockRoomRepo()
	starter := &capabilityStarter{fakeMatchStarter: &fakeMatchStarter{}}
	hub := &capabilityHub{mockBroadcaster: &mockBroadcaster{}, connected: []uint{10, 99}}
	h := room.NewRoomHandler(repo, starter, hub, nil, nil, nil, nil)
	patient := 2 * hugeInterval
	h.SetQuickFillIntervals(hugeInterval, patient)

	r := seedCroatiaQuickPlayRoom(repo, "RESETQ", 0, 10)

	h.StartQuickFill(r.ID, 10)
	defer h.CancelQuickFill(r.ID)
	genBefore, intervalBefore, armed := h.QuickFillState(r.ID)
	require.True(t, armed)
	assert.Equal(t, patient, intervalBefore)

	// A human joins → reset.
	h.ResetQuickFill(r.ID)
	genAfter, intervalAfter, armed := h.QuickFillState(r.ID)
	require.True(t, armed, "still armed after a reset")
	assert.Greater(t, genAfter, genBefore, "reset restarts the countdown (bumps the generation)")
	assert.Equal(t, patient, intervalAfter, "reset keeps the cadence chosen at arm time")

	// The OLD timer (pre-reset generation) firing now must seat NOTHING.
	h.InvokeQuickFillTick(r.ID, genBefore)
	assert.Equal(t, 0, countBots(repo, r.ID), "a stale (pre-reset) tick is a no-op")

	// The fresh timer still works.
	h.TriggerQuickFillTick(r.ID)
	assert.Equal(t, 1, countBots(repo, r.ID), "the reset timer still seats a bot when it fires")
}

// TestQuickFill_CancelledTickIsNoOp verifies a cancelled scheduler seats no bot.
func TestQuickFill_CancelledTickIsNoOp(t *testing.T) {
	repo := newMockRoomRepo()
	starter := &capabilityStarter{fakeMatchStarter: &fakeMatchStarter{}}
	hub := &capabilityHub{mockBroadcaster: &mockBroadcaster{}, connected: []uint{10}}
	h := room.NewRoomHandler(repo, starter, hub, nil, nil, nil, nil)
	h.SetQuickFillIntervals(hugeInterval, 2*hugeInterval)

	r := seedCroatiaQuickPlayRoom(repo, "CANCLQ", 0, 10)
	h.StartQuickFill(r.ID, 10)
	genBefore, _, _ := h.QuickFillState(r.ID)

	h.CancelQuickFill(r.ID)
	_, _, armed := h.QuickFillState(r.ID)
	assert.False(t, armed, "cancel forgets the timer")

	// Both a current-generation trigger (no entry) and the old-generation
	// callback are no-ops.
	h.TriggerQuickFillTick(r.ID)
	h.InvokeQuickFillTick(r.ID, genBefore)
	assert.Equal(t, 0, countBots(repo, r.ID))
}

// TestQuickFill_NonWaitingRoomSelfTerminates covers the self-terminate guard: a
// tick on a room that is no longer waiting seats nothing and cancels itself.
func TestQuickFill_NonWaitingRoomSelfTerminates(t *testing.T) {
	repo := newMockRoomRepo()
	starter := &capabilityStarter{fakeMatchStarter: &fakeMatchStarter{}}
	hub := &capabilityHub{mockBroadcaster: &mockBroadcaster{}, connected: []uint{10}}
	h := room.NewRoomHandler(repo, starter, hub, nil, nil, nil, nil)
	h.SetQuickFillIntervals(hugeInterval, 2*hugeInterval)

	r := seedCroatiaQuickPlayRoom(repo, "PLAYQP", 0, 10)
	h.StartQuickFill(r.ID, 10)

	// Simulate the room leaving "waiting" (started elsewhere, or closed after
	// the sole human left).
	r.Status = "playing"

	h.TriggerQuickFillTick(r.ID)
	assert.Equal(t, 0, countBots(repo, r.ID), "no bot seated into a non-waiting room")
	_, _, armed := h.QuickFillState(r.ID)
	assert.False(t, armed, "the tick self-terminated the scheduler")
}

// TestQuickFill_ClosedRoomSelfTerminates covers the "sole human leaves → room
// closes → no bot-only room lingers" edge: a completed room self-terminates.
func TestQuickFill_ClosedRoomSelfTerminates(t *testing.T) {
	repo := newMockRoomRepo()
	starter := &capabilityStarter{fakeMatchStarter: &fakeMatchStarter{}}
	hub := &capabilityHub{mockBroadcaster: &mockBroadcaster{}, connected: []uint{10}}
	h := room.NewRoomHandler(repo, starter, hub, nil, nil, nil, nil)
	h.SetQuickFillIntervals(hugeInterval, 2*hugeInterval)

	r := seedCroatiaQuickPlayRoom(repo, "CLOSEQ", 0, 10)
	h.StartQuickFill(r.ID, 10)

	// The sole human left; LeaveRoom closed the room.
	r.Status = "completed"

	h.TriggerQuickFillTick(r.ID)
	assert.Equal(t, 0, countBots(repo, r.ID), "a closed room never gets a bot")
	_, _, armed := h.QuickFillState(r.ID)
	assert.False(t, armed)
}

// TestQuickFill_AlreadyFullSelfTerminates verifies a bot is never dropped when
// humans + bots already cover all four seats (a bot keeps its seat; no fifth
// occupant).
func TestQuickFill_AlreadyFullSelfTerminates(t *testing.T) {
	repo := newMockRoomRepo()
	starter := &capabilityStarter{fakeMatchStarter: &fakeMatchStarter{}}
	hub := &capabilityHub{mockBroadcaster: &mockBroadcaster{}, connected: []uint{10}}
	h := room.NewRoomHandler(repo, starter, hub, nil, nil, nil, nil)
	h.SetQuickFillIntervals(hugeInterval, 2*hugeInterval)

	// Three humans seated (0,1,2) + one bot at seat 3 = full.
	r := seedCroatiaQuickPlayRoom(repo, "FULLQP", 0, 10, 11, 12)
	require.NoError(t, repo.AddBot(r.ID, 3))

	h.StartQuickFill(r.ID, 10)
	h.TriggerQuickFillTick(r.ID)

	assert.Equal(t, 1, countBots(repo, r.ID), "no extra bot seated into a full room")
	_, _, armed := h.QuickFillState(r.ID)
	assert.False(t, armed, "a full room self-terminates the scheduler")
}

// TestQuickFill_RealTimerFires is the smoke test that the AfterFunc plumbing
// actually fires end-to-end on a real (short) interval: it fills the room, the
// match auto-starts, and the scheduler self-cancels. It waits for that TERMINAL
// state so no scheduler goroutine outlives the test.
//
// Race safety: the poll condition touches ONLY QuickFillState (mutex-guarded,
// no repo access), so it never races the tick goroutine's repo writes. Once the
// scheduler reports not-armed it has been cancelled on the auto-start path — no
// further repo writes happen — so the post-loop repo reads are race-clean.
func TestQuickFill_RealTimerFires(t *testing.T) {
	repo := newMockRoomRepo()
	starter := &capabilityStarter{fakeMatchStarter: &fakeMatchStarter{}}
	hub := &capabilityHub{mockBroadcaster: &mockBroadcaster{}, connected: []uint{10}}
	wallet := &stubWallet{balance: 1000}
	h := room.NewRoomHandler(repo, starter, hub, nil, wallet, nil, nil)
	h.SetQuickFillIntervals(5*time.Millisecond, time.Hour)

	r := seedCroatiaQuickPlayRoom(repo, "REALTQ", 500, 10)
	h.StartQuickFill(r.ID, 10)
	defer h.CancelQuickFill(r.ID) // safety net if the terminal state is never reached

	require.Eventually(t, func() bool {
		_, _, armed := h.QuickFillState(r.ID)
		return !armed
	}, 2*time.Second, 2*time.Millisecond,
		"the real timer must fill the room, auto-start the match, and self-cancel")

	// The scheduler has self-cancelled after auto-start; the tick goroutine has
	// finished its writes, so these reads are safe.
	updated, _ := repo.FindByID(r.ID)
	require.NotNil(t, updated)
	assert.Equal(t, "playing", updated.Status, "the match must have auto-started")
	assert.Equal(t, 3, countBots(repo, r.ID), "the lone human is joined by three bots")
	assert.Equal(t, 1, starter.called, "StartMatch runs exactly once")
}
