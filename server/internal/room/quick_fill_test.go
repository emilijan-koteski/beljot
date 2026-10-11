package room_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/room"
)

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

// TestQuickFill_FastFill_ReachesFourAndStarts covers the fast cadence: each tick seats
// a bot in the lowest empty seat and the fourth seat auto-starts with only the human charged.
func TestQuickFill_FastFill_ReachesFourAndStarts(t *testing.T) {
	repo := newMockRoomRepo()
	starter := &capabilityStarter{fakeMatchStarter: &fakeMatchStarter{}}
	hub := &capabilityHub{mockBroadcaster: &mockBroadcaster{}, connected: []uint{10}}
	wallet := &stubWallet{balance: 1000}
	h := room.NewRoomHandler(repo, starter, hub, nil, wallet, nil, nil)
	h.SetQuickFillIntervals(hugeInterval, 2*hugeInterval)

	r := seedCroatiaQuickPlayRoom(repo, "FASTQP", 500, 10)

	h.StartQuickFill(r.ID, 10)
	defer h.CancelQuickFill(r.ID)

	_, interval, armed := h.QuickFillState(r.ID)
	require.True(t, armed, "scheduler must be armed after StartQuickFill")
	assert.Equal(t, hugeInterval, interval, "0 idle players selects the fast cadence")

	h.TriggerQuickFillTick(r.ID)
	assert.Equal(t, 1, countBots(repo, r.ID))
	h.TriggerQuickFillTick(r.ID)
	assert.Equal(t, 2, countBots(repo, r.ID))

	updated, _ := repo.FindByID(r.ID)
	assert.Equal(t, "waiting", updated.Status, "not yet full — still waiting")
	assert.Equal(t, 0, starter.called)

	h.TriggerQuickFillTick(r.ID)

	assert.Equal(t, 3, countBots(repo, r.ID))
	updated, _ = repo.FindByID(r.ID)
	assert.Equal(t, "playing", updated.Status, "the fourth occupant auto-starts the match")
	require.Equal(t, 1, starter.called, "StartMatch runs exactly once")

	assert.Equal(t, uint(10), starter.lastPlayers[0].UserID)
	assert.False(t, starter.lastPlayers[0].IsBot, "seat 0 is the human")
	for seat := 1; seat <= 3; seat++ {
		assert.True(t, starter.lastPlayers[seat].IsBot, "seat %d must be a bot", seat)
		assert.Equal(t, uint(0), starter.lastPlayers[seat].UserID, "bot seat %d has no user id", seat)
	}

	assert.Equal(t, 1, wallet.chargeCalls)
	assert.Equal(t, []uint{10}, wallet.chargedIDs, "only the human seat pays the stake")
	assert.Equal(t, 500, wallet.chargedAmount)

	_, _, armed = h.QuickFillState(r.ID)
	assert.False(t, armed, "the scheduler is cancelled once the match starts")
}

// TestQuickFill_MixedHumansAndBotsAutoStarts covers a room with two humans seated:
// bots fill the rest, only the humans are charged, and the scheduler cancels on start.
func TestQuickFill_MixedHumansAndBotsAutoStarts(t *testing.T) {
	repo := newMockRoomRepo()
	starter := &capabilityStarter{fakeMatchStarter: &fakeMatchStarter{}}
	hub := &capabilityHub{mockBroadcaster: &mockBroadcaster{}, connected: []uint{10, 11}}
	wallet := &stubWallet{balance: 1000}
	h := room.NewRoomHandler(repo, starter, hub, nil, wallet, nil, nil)
	h.SetQuickFillIntervals(hugeInterval, 2*hugeInterval)

	r := seedCroatiaQuickPlayRoom(repo, "MIXEDQ", 500, 10, 11)

	h.StartQuickFill(r.ID, 10)
	defer h.CancelQuickFill(r.ID)

	h.TriggerQuickFillTick(r.ID)
	assert.Equal(t, 1, countBots(repo, r.ID))
	updated, _ := repo.FindByID(r.ID)
	assert.Equal(t, "waiting", updated.Status, "3 occupants — still waiting")
	assert.Equal(t, 0, starter.called)

	h.TriggerQuickFillTick(r.ID)

	assert.Equal(t, 2, countBots(repo, r.ID))
	updated, _ = repo.FindByID(r.ID)
	assert.Equal(t, "playing", updated.Status, "the fourth occupant auto-starts the match")
	require.Equal(t, 1, starter.called, "StartMatch runs exactly once")

	assert.False(t, starter.lastPlayers[0].IsBot)
	assert.Equal(t, uint(10), starter.lastPlayers[0].UserID)
	assert.False(t, starter.lastPlayers[1].IsBot)
	assert.Equal(t, uint(11), starter.lastPlayers[1].UserID)
	for seat := 2; seat <= 3; seat++ {
		assert.True(t, starter.lastPlayers[seat].IsBot, "seat %d must be a bot", seat)
		assert.Equal(t, uint(0), starter.lastPlayers[seat].UserID, "bot seat %d has no user id", seat)
	}

	assert.Equal(t, 1, wallet.chargeCalls)
	assert.ElementsMatch(t, []uint{10, 11}, wallet.chargedIDs, "both human seats pay, and only them")
	assert.Equal(t, 500, wallet.chargedAmount)

	_, _, armed := h.QuickFillState(r.ID)
	assert.False(t, armed, "the scheduler is cancelled once the match starts")
}

// A Quick Play match auto-started by the fill carries each seated human's
// avatar into the session, exactly like a manual start; bot seats carry none.
func TestQuickFill_AutoStartCarriesSeatAvatars(t *testing.T) {
	repo := newMockRoomRepo()
	starter := &capabilityStarter{fakeMatchStarter: &fakeMatchStarter{}}
	hub := &capabilityHub{mockBroadcaster: &mockBroadcaster{}, connected: []uint{10, 11}}
	h := room.NewRoomHandler(repo, starter, hub, nil, &stubWallet{balance: 1000}, nil, nil)
	h.SetQuickFillIntervals(hugeInterval, 2*hugeInterval)

	r := seedCroatiaQuickPlayRoom(repo, "AVATQP", 500, 10, 11)
	url := "https://assets.test/avatars/88888888-8888-4888-8888-888888888888/128.webp"
	for _, p := range repo.players {
		if p.UserID == 10 {
			p.AvatarURL = &url
		}
	}

	h.StartQuickFill(r.ID, 10)
	defer h.CancelQuickFill(r.ID)
	h.TriggerQuickFillTick(r.ID)
	h.TriggerQuickFillTick(r.ID)
	require.Equal(t, 1, starter.called)

	require.NotNil(t, starter.lastPlayers[0].AvatarURL)
	assert.Equal(t, url, *starter.lastPlayers[0].AvatarURL)
	assert.Nil(t, starter.lastPlayers[1].AvatarURL, "a human without an avatar")
	for seat := 2; seat <= 3; seat++ {
		assert.Nil(t, starter.lastPlayers[seat].AvatarURL, "bot seat %d", seat)
	}
}

// TestQuickFill_PatientCadenceSelected verifies that at least one idle lobby
// player selects the patient cadence, and each quiet interval adds exactly one
// bot.
func TestQuickFill_PatientCadenceSelected(t *testing.T) {
	repo := newMockRoomRepo()
	starter := &capabilityStarter{fakeMatchStarter: &fakeMatchStarter{}}
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

	h.ResetQuickFill(r.ID)
	genAfter, intervalAfter, armed := h.QuickFillState(r.ID)
	require.True(t, armed, "still armed after a reset")
	assert.Greater(t, genAfter, genBefore, "reset restarts the countdown (bumps the generation)")
	assert.Equal(t, patient, intervalAfter, "reset keeps the cadence chosen at arm time")

	h.InvokeQuickFillTick(r.ID, genBefore)
	assert.Equal(t, 0, countBots(repo, r.ID), "a stale (pre-reset) tick is a no-op")

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

	r := seedCroatiaQuickPlayRoom(repo, "FULLQP", 0, 10, 11, 12)
	require.NoError(t, repo.AddBot(r.ID, 3))

	h.StartQuickFill(r.ID, 10)
	h.TriggerQuickFillTick(r.ID)

	assert.Equal(t, 1, countBots(repo, r.ID), "no extra bot seated into a full room")
	_, _, armed := h.QuickFillState(r.ID)
	assert.False(t, armed, "a full room self-terminates the scheduler")
}

// TestQuickFill_RealTimerFires checks the AfterFunc plumbing end-to-end on a short interval.
// It polls only QuickFillState until the scheduler self-cancels, so no goroutine races the repo reads.
func TestQuickFill_RealTimerFires(t *testing.T) {
	repo := newMockRoomRepo()
	starter := &capabilityStarter{fakeMatchStarter: &fakeMatchStarter{}}
	hub := &capabilityHub{mockBroadcaster: &mockBroadcaster{}, connected: []uint{10}}
	wallet := &stubWallet{balance: 1000}
	h := room.NewRoomHandler(repo, starter, hub, nil, wallet, nil, nil)
	h.SetQuickFillIntervals(5*time.Millisecond, time.Hour)

	r := seedCroatiaQuickPlayRoom(repo, "REALTQ", 500, 10)
	h.StartQuickFill(r.ID, 10)
	defer h.CancelQuickFill(r.ID)

	require.Eventually(t, func() bool {
		_, _, armed := h.QuickFillState(r.ID)
		return !armed
	}, 2*time.Second, 2*time.Millisecond,
		"the real timer must fill the room, auto-start the match, and self-cancel")

	updated, _ := repo.FindByID(r.ID)
	require.NotNil(t, updated)
	assert.Equal(t, "playing", updated.Status, "the match must have auto-started")
	assert.Equal(t, 3, countBots(repo, r.ID), "the lone human is joined by three bots")
	assert.Equal(t, 1, starter.called, "StartMatch runs exactly once")
}
