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

// Deal grace: each animated deal adds its client animation length to the next
// actor's deadline and to a bot actor's think delay (deal_grace.go). These
// values mirror client/src/shared/lib/motion.ts; the client pins its side in
// dealSchedule.test.ts.
const (
	graceCandidateFirst  = 2100 * time.Millisecond
	graceAllBeforeBid    = 2260 * time.Millisecond
	graceCandidateSecond = 740 * time.Millisecond
	graceSplash          = 1500 * time.Millisecond
	perMoveWindow        = 10 * time.Second
)

// assertWindow checks that the turn deadline sits `want` after `from`,
// allowing for the few milliseconds the action itself took.
func assertWindow(t *testing.T, st *game.GameState, from time.Time, want time.Duration) {
	t.Helper()
	require.NotNil(t, st)
	require.NotNil(t, st.TurnExpiresAt, "a per-move turn must carry a deadline")
	got := st.TurnExpiresAt.Sub(from)
	assert.InDelta(t, want.Milliseconds(), got.Milliseconds(), 150,
		"deadline should be %v after the transition, was %v", want, got)
}

func sendAs(mgr *match.Manager, userID uint, msgType string, payload string) {
	mgr.HandleAction(&ws.Client{UserID: userID}, ws.WSMessage{Type: msgType, Payload: json.RawMessage(payload)})
}

func TestDealGrace_MatchStartCoversSplashAndFirstDeal(t *testing.T) {
	for _, tc := range []struct {
		variant string
		deal    time.Duration
	}{
		{"bitola", graceCandidateFirst},
		{"croatia", graceAllBeforeBid},
	} {
		t.Run(tc.variant, func(t *testing.T) {
			mgr := match.NewManager(&hubSpy{}, newMockMatchRepo())
			from := time.Now()
			require.NoError(t, mgr.StartMatch(100, tc.variant, "1001", defaultPlayers(), "per-move", 10, 10, 120, 0, true, false))
			t.Cleanup(func() { mgr.RemoveSession(100) })

			st := mgr.GetStateSnapshot(100)
			require.Equal(t, game.PhaseBidding, st.Phase)
			assertWindow(t, st, from, perMoveWindow+graceSplash+tc.deal)
		})
	}
}

func TestDealGrace_NextHandPushesOnlyTheFirstBidder(t *testing.T) {
	mgr := match.NewManager(&hubSpy{}, newMockMatchRepo())
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 10, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(100) })

	gs := mgr.GetStateSnapshot(100)
	gs.Phase = game.PhaseHandComplete
	gs.TurnExpiresAt = nil
	gs.HandCompleteReady = [4]bool{true, true, true, false}
	mgr.SetGameStateForTest(100, gs)

	// The last acknowledgement deals the next hand.
	from := time.Now()
	sendAs(mgr, 40, "action:continue", `{}`)
	st := mgr.GetStateSnapshot(100)
	require.Equal(t, game.PhaseBidding, st.Phase)
	require.Equal(t, 2, st.HandNumber)
	assertWindow(t, st, from, perMoveWindow+graceCandidateFirst)

	// The first bidder's pass opens an ordinary turn: no deal, no grace.
	first := st.ActivePlayerSeat
	from = time.Now()
	sendAs(mgr, st.Players[first].UserID, "action:pass_trump", `{}`)
	st = mgr.GetStateSnapshot(100)
	require.NotEqual(t, first, st.ActivePlayerSeat)
	assertWindow(t, st, from, perMoveWindow)
	assert.Zero(t, mgr.DealGraceRemainingForTest(100))
}

func TestDealGrace_ReshuffleIsADeal(t *testing.T) {
	mgr := match.NewManager(&hubSpy{}, newMockMatchRepo())
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 10, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(100) })

	// Round 2, three passes: the dealer (seat 0) passing passes the hand out.
	mgr.SetGameStateForTest(100, testfixtures.NewGameMidBidding(7))
	from := time.Now()
	sendAs(mgr, 10, "action:pass_trump", `{}`)

	st := mgr.GetStateSnapshot(100)
	require.Equal(t, game.PhaseBidding, st.Phase)
	require.Equal(t, 1, st.DealerSeat, "the reshuffle rotates the dealer")
	require.Equal(t, 1, st.HandNumber, "a reshuffle stays on the same hand")
	assertWindow(t, st, from, perMoveWindow+graceCandidateFirst)
}

// The timer-driven paths deal too: a round-2 dealer who times out auto-passes
// the hand out, and the reshuffle's first bidder gets the deal's grace.
func TestDealGrace_TimeoutReshuffleIsADeal(t *testing.T) {
	mgr := match.NewManager(&hubSpy{}, newMockMatchRepo())
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 10, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(100) })

	mgr.SetGameStateForTest(100, testfixtures.NewGameMidBidding(7))
	const fireAfter = 5 * time.Millisecond
	from := time.Now().Add(fireAfter)
	mgr.TriggerTimerExpiryForTest(100, 0, fireAfter)
	require.True(t, waitFor(time.Second, func() bool {
		st := mgr.GetStateSnapshot(100)
		return st != nil && st.DealerSeat == 1
	}), "the timed-out dealer's auto-pass must reshuffle")

	st := mgr.GetStateSnapshot(100)
	require.Equal(t, game.PhaseBidding, st.Phase)
	assertWindow(t, st, from, perMoveWindow+graceCandidateFirst)
}

// The score-reveal auto-continue deals the next hand: its first bidder gets the
// deal's grace too.
func TestDealGrace_HandCompleteTimeoutIsADeal(t *testing.T) {
	mgr := match.NewManager(&hubSpy{}, newMockMatchRepo())
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 10, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(100) })

	deadline := time.Now().Add(30 * time.Millisecond)
	injectHandCompletePause(t, mgr, deadline)
	// One acknowledgement arms the fallback against the seeded deadline; the
	// other three never come, so the timeout deals.
	sendAs(mgr, 10, "action:continue", `{}`)
	require.True(t, waitFor(time.Second, func() bool {
		st := mgr.GetStateSnapshot(100)
		return st != nil && st.HandNumber == 2
	}), "the auto-continue must deal the next hand")

	st := mgr.GetStateSnapshot(100)
	require.Equal(t, game.PhaseBidding, st.Phase)
	assertWindow(t, st, deadline, perMoveWindow+graceCandidateFirst)
}

func TestDealGrace_SecondDealPushesTheLeader(t *testing.T) {
	mgr := match.NewManager(&hubSpy{}, newMockMatchRepo())
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 10, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(100) })

	// Seat 1 takes the face-up candidate: the reserve goes out 3/3/3/(2+candidate).
	mgr.SetGameStateForTest(100, testfixtures.NewGameJustDealt())
	from := time.Now()
	sendAs(mgr, 20, "action:pick_trump", `{}`)

	st := mgr.GetStateSnapshot(100)
	require.Equal(t, game.PhasePlaying, st.Phase)
	require.Nil(t, st.TrumpCandidate)
	assertWindow(t, st, from, perMoveWindow+graceCandidateSecond)
}

func TestDealGrace_DisabledLeavesTheBareWindow(t *testing.T) {
	mgr := match.NewManager(&hubSpy{}, newMockMatchRepo())
	mgr.SetDealGraceForTest(false)
	from := time.Now()
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 10, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(100) })

	assertWindow(t, mgr.GetStateSnapshot(100), from, perMoveWindow)
}

// A bot opening the bidding right after a deal waits the deal out on top of
// its think delay, so it never bids under the dealer's hands.
func TestDealGrace_DelaysABotFirstBidder(t *testing.T) {
	hub := &hubSpy{}
	mgr := match.NewManager(hub, newMockMatchRepo())
	mgr.SetBotDelayForTest(time.Millisecond, 2*time.Millisecond)
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", mixedPlayers(1, 2, 3), "relaxed", 0, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(100) })

	// StartMatch may already have armed the opening bidder. Deal the next hand
	// so that a DIFFERENT bot opens it, or that armed delay would stand in for
	// the one under test.
	gs := mgr.GetStateSnapshot(100)
	armed := gs.ActivePlayerSeat
	bidder := 1
	for bidder == armed {
		bidder++
	}
	gs = markBots(gs, 1, 2, 3)
	gs.Phase = game.PhaseHandComplete
	gs.DealerSeat = (bidder + 2) % 4 // next dealer = +1, first bidder = +2
	gs.HandCompleteReady = [4]bool{false, true, true, true}
	mgr.SetGameStateForTest(100, gs)

	dealtAt := time.Now()
	sendAs(mgr, 10, "action:continue", `{}`)
	st := mgr.GetStateSnapshot(100)
	require.Equal(t, game.PhaseBidding, st.Phase)
	require.Equal(t, bidder, st.ActivePlayerSeat)

	acted := func() bool {
		s := mgr.GetStateSnapshot(100)
		return s == nil || s.Phase != game.PhaseBidding || s.BiddingPassCount > 0 || s.ActivePlayerSeat != bidder
	}
	time.Sleep(graceCandidateFirst - 400*time.Millisecond - time.Since(dealtAt))
	assert.False(t, acted(), "the bot must not bid while the deal is still animating")
	assert.True(t, waitFor(graceCandidateFirst+time.Second, acted), "the bot must bid once the deal has played out")
}

// The Belote beat itself: at most 200 ms under the production 1–2.5 s bounds,
// for the announcer only — every other seat keeps its full think delay.
func TestBot_BelotBeatIsAtMost200ms(t *testing.T) {
	mgr := match.NewManager(&hubSpy{}, newMockMatchRepo()) // production delays
	announcer := 1
	gs := testfixtures.NewGameMidPlay(1)
	gs.PendingBelotSeat = &announcer

	for range 50 {
		assert.LessOrEqual(t, mgr.BotThinkDelayForTest(gs, announcer), 200*time.Millisecond)
		assert.GreaterOrEqual(t, mgr.BotThinkDelayForTest(gs, 2), time.Second)
	}
}

// A bot answering its own Belote prompt takes a short beat, not a second full
// think delay: with the production 1–2.5 s delays it still answers well under
// a second after the K/Q lands.
func TestBot_BelotAnswerTakesAShortBeat(t *testing.T) {
	mgr := match.NewManager(&hubSpy{}, newMockMatchRepo()) // production delays
	mgr.SetDealGraceForTest(false)
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", mixedPlayers(1, 2, 3), "relaxed", 0, 10, 120, 0, true, false))
	t.Cleanup(func() { mgr.RemoveSession(100) })

	// StartMatch armed the opening bidder's delay; hold the prompt on another
	// bot so that pending delay cannot stand in for the one under test.
	armed := mgr.GetStateSnapshot(100).ActivePlayerSeat
	announcer := 1
	for announcer == armed {
		announcer++
	}
	gs := markBots(testfixtures.NewGameMidPlay(1), 1, 2, 3)
	gs.BelotAnnounced = false
	gs.PendingBelotSeat = &announcer
	gs.CurrentTrick = []game.TrickCard{{Card: game.Card{Rank: game.RankKing, Suit: game.SuitHearts}, PlayerSeat: announcer}}
	lead := game.SuitHearts
	gs.LeadSuit = &lead
	gs.ActivePlayerSeat = announcer
	mgr.SetGameStateForTest(100, gs)

	armedAt := time.Now()
	mgr.BotSchedule(100)
	require.True(t, waitFor(2*time.Second, func() bool {
		st := mgr.GetStateSnapshot(100)
		return st != nil && st.BelotAnnounced
	}), "the bot must announce")
	elapsed := time.Since(armedAt)

	assert.Less(t, elapsed, 500*time.Millisecond, "the Belote answer must not wait a full think delay")
	st := mgr.GetStateSnapshot(100)
	assert.Nil(t, st.PendingBelotSeat)
	assert.NotEqual(t, announcer, st.ActivePlayerSeat, "the turn moves on as soon as the bot answers")
}
