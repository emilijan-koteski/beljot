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

// TestAutoActionTypeFor locks the wire-format mapping that drives
// EventAutoAction. ActionPlayCard intentionally returns ok=false because
// card auto-play is signalled via the AutoPlayed flag on EventCardPlayed.
func TestAutoActionTypeFor(t *testing.T) {
	cases := []struct {
		name       string
		actionType string
		wantType   ws.AutoActionType
		wantOK     bool
	}{
		{"pass_trump emits auto-action", game.ActionPassTrump, ws.AutoActionPassTrump, true},
		{"skip_declare emits auto-action", game.ActionSkipDeclare, ws.AutoActionSkipDeclare, true},
		{"skip_belot emits auto-action", game.ActionSkipBelot, ws.AutoActionSkipBelot, true},
		{"play_card uses autoPlayed flag, not auto-action", game.ActionPlayCard, "", false},
		{"announce_belot is not a timeout action", game.ActionAnnounceBelot, "", false},
		{"declare is not a timeout action", game.ActionDeclare, "", false},
		{"pick_trump is not a timeout action", game.ActionPickTrump, "", false},
		{"empty action", "", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := match.AutoActionTypeFor(tc.actionType)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.wantType, got)
		})
	}
}

// TestPerMoveTimer_BiddingPhaseAutoPasses confirms the auto-pass path runs
// end-to-end: a bidding-phase timer expiry advances the active seat without
// panicking and without leaving the session stuck. The wire-format payload
// itself is locked by TestAutoActionTypeFor; this test guards against silent
// regression where the new EventAutoAction broadcast path corrupts state.
func TestPerMoveTimer_BiddingPhaseAutoPasses(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)

	// 1-second timer so it expires quickly
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 1, 10, 120, 0))

	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)
	require.Equal(t, game.PhaseBidding, state.Phase)
	firstBidder := state.ActivePlayerSeat

	// Wait through one bidding timeout — server auto-passes for the active bidder.
	// The timer fires expiryGrace (400ms) past the 1s advertised deadline, so
	// sleep comfortably past deadline+grace for the broadcast to settle.
	time.Sleep(2200 * time.Millisecond)

	after := mgr.GetStateSnapshot(100)
	require.NotNil(t, after, "session must survive the bidding timeout broadcast path")
	assert.NotEqual(t, firstBidder, after.ActivePlayerSeat, "auto-pass should advance the active seat")
}

// TestPerMoveTimer_PickTrumpJSONShapeUnchanged is a regression guard: the new
// EventAutoAction wire-format must be parseable as JSON. We construct a
// payload directly and round-trip it to catch any future struct-tag drift.
func TestPerMoveTimer_PickTrumpJSONShapeUnchanged(t *testing.T) {
	payload := ws.AutoActionPayload{PlayerSeat: 2, Type: ws.AutoActionPassTrump}
	b, err := json.Marshal(payload)
	require.NoError(t, err)
	assert.JSONEq(t, `{"playerSeat":2,"type":"pass_trump"}`, string(b))
}

// TestAutoAction_SkipDeclare_ChainsToAutoPlayWithoutExtension is the regression
// for the bug: when the per-move timer fires while a declaration prompt is
// open, the auto-action used to be skip_declare followed by setTurnExpiry
// granting the same player a fresh full-duration window. Fix: chain AutoPlay
// inside handleTimerExpiry so the seat advances and the next seat (not the
// timed-out player) gets the fresh timer.
func TestAutoAction_SkipDeclare_ChainsToAutoPlayWithoutExtension(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	// 60-second timerDurationSec — if the bug regressed (fresh expiry on
	// timed-out seat), that fresh expiry would land ~60s in the future, far
	// past the chained next-seat expiry (which sits ~60s after the chain
	// runs but starts on the seat that just received the play, not seat 1).
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 60, 10, 120, 0))

	// Inject a declaration-prompt state on seat 1 with an already-elapsed
	// expiry. The next TriggerTimerExpiryForTest fires the auto-action.
	gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
	gs.RoomID = 100
	gs.ActivePlayerSeat = 1
	gs.AwaitingDeclaration = true
	originalExpiry := time.Now().Add(-1 * time.Second) // already past
	gs.TurnExpiresAt = &originalExpiry
	gs.TimerDurationSec = 60
	mgr.SetGameStateForTest(100, gs)

	mgr.TriggerTimerExpiryForTest(100, 1, 50*time.Millisecond)
	time.Sleep(400 * time.Millisecond)

	after := mgr.GetStateSnapshot(100)
	require.NotNil(t, after)
	// Chain proof: seat advanced past 1 (skip_declare alone leaves seat=1).
	assert.NotEqual(t, 1, after.ActivePlayerSeat,
		"chained AutoPlay must advance the seat past the doomed player")
	// Trick has exactly the chained card (no other plays could happen).
	assert.Equal(t, 1, len(after.CurrentTrick),
		"chained auto-play should land exactly one card in the trick")
	// Fresh timer belongs to the new seat. Crucially, oldState (seat 1) had
	// expiry in the past; a regression of the bug would arm a fresh timer for
	// seat 1 (the doomed player). Assert the seat changed AND the expiry is
	// in the future.
	require.NotNil(t, after.TurnExpiresAt)
	assert.True(t, after.TurnExpiresAt.After(time.Now()),
		"fresh timer for next seat must be in the future")
}

// TestAutoAction_ChainedBelotPrompt_AdvancesPastDoomedSeat is a regression for
// a stall edge case: when the chain auto-plays K of trump while the player
// also holds Q, handlePlayCard opens a belot prompt and does NOT advance the
// seat. Without a multi-step chain, no timer would arm and the game would
// stall. The structural loop must keep auto-resolving (here: skip_belot)
// until the seat actually advances.
func TestAutoAction_ChainedBelotPrompt_AdvancesPastDoomedSeat(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 60, 10, 120, 0))

	// Seat 0 with hearts trump holds KH+QH. Force AutoPlay to land on KH by
	// stripping all of seat 0's other cards down to just K and Q. Set
	// AwaitingDeclaration=true on seat 0 so the chain runs:
	//   skip_declare → auto_play(KH) → belot prompt → skip_belot.
	gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
	gs.RoomID = 100
	gs.ActivePlayerSeat = 0
	gs.AwaitingDeclaration = true
	// Seat 0 fixture hand starts with KH QH 7S 8S 9S 7C 8C 9C — keep all,
	// AutoPlay sorts S→H→D→C then 7→8→9→T→J→Q→K→A and lead-suit-first; with
	// no LeadSuit (seat 0 leads), the smallest of the smallest suit wins, so
	// 7S leads. That doesn't trigger belot — hearts trump K isn't played.
	// Restrict seat 0's hand to hearts only so AutoPlay must pick QH or KH.
	gs.Players[0].Hand = []game.Card{
		{Rank: game.RankKing, Suit: game.SuitHearts},
		{Rank: game.RankQueen, Suit: game.SuitHearts},
	}
	originalExpiry := time.Now().Add(-1 * time.Second)
	gs.TurnExpiresAt = &originalExpiry
	gs.TimerDurationSec = 60
	mgr.SetGameStateForTest(100, gs)

	mgr.TriggerTimerExpiryForTest(100, 0, 50*time.Millisecond)
	time.Sleep(500 * time.Millisecond)

	after := mgr.GetStateSnapshot(100)
	require.NotNil(t, after)
	// Seat must advance past 0 — the chain must not stall on the belot prompt.
	// (The new active seat may have its own AwaitingDeclaration if their hand
	// contains declarations — that's a fresh prompt for the new seat, not the
	// stale one for seat 0.)
	assert.NotEqual(t, 0, after.ActivePlayerSeat,
		"chain must walk skip_declare → auto_play → skip_belot until seat advances")
	assert.Nil(t, after.PendingBelotSeat, "belot prompt for seat 0 must be resolved by the chain")
	require.NotNil(t, after.TurnExpiresAt)
	assert.True(t, after.TurnExpiresAt.After(time.Now()),
		"fresh timer must arm for the next seat once the chain settles")
	// One trump card was auto-played (which one depends on AutoPlay's sort —
	// either K or Q triggers the belot prompt since seat 0 holds both).
	assert.Equal(t, 1, len(after.CurrentTrick), "exactly one card landed in the trick")
	played := after.CurrentTrick[0].Card.String()
	assert.Contains(t, []string{"KH", "QH"}, played,
		"the chained auto-play must commit a hearts trump K or Q (both held → belot trigger)")
}

// TestAutoAction_SkipBelot_AdvancesSeatAndArmsFreshTimer covers the simpler
// case: skip_belot flows through finishCardPlay which advances the seat, so a
// fresh timer for the next seat is correct. This locks the contract that the
// auto-action path in handleTimerExpiry still arms a timer when the seat
// genuinely transitions.
func TestAutoAction_SkipBelot_AdvancesSeatAndArmsFreshTimer(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 60, 10, 120, 0))

	// Set up a belot-prompt state on seat 1, using diamonds trump so seat 1's
	// fixture hand (KD, QD) qualifies. Seat 1 just played KD; PendingBelotSeat
	// == 1; KD already moved from hand into the trick.
	gs := testfixtures.NewGameFirstTrick(game.SuitDiamonds)
	gs.RoomID = 100
	gs.ActivePlayerSeat = 1
	pendingSeat := 1
	gs.PendingBelotSeat = &pendingSeat
	// Move KD from seat 1's hand into the trick to mirror the post-play state.
	kdIdx := -1
	for i, c := range gs.Players[1].Hand {
		if c.Rank == game.RankKing && c.Suit == game.SuitDiamonds {
			kdIdx = i
			break
		}
	}
	require.GreaterOrEqual(t, kdIdx, 0, "fixture must contain KD in seat 1's hand")
	playedCard := gs.Players[1].Hand[kdIdx]
	gs.Players[1].Hand = append(gs.Players[1].Hand[:kdIdx], gs.Players[1].Hand[kdIdx+1:]...)
	gs.CurrentTrick = []game.TrickCard{{PlayerSeat: 1, Card: playedCard}}
	leadSuit := game.SuitDiamonds
	gs.LeadSuit = &leadSuit
	originalExpiry := time.Now().Add(-1 * time.Second) // already past
	gs.TurnExpiresAt = &originalExpiry
	gs.TimerDurationSec = 60
	mgr.SetGameStateForTest(100, gs)

	mgr.TriggerTimerExpiryForTest(100, 1, 50*time.Millisecond)
	time.Sleep(400 * time.Millisecond)

	after := mgr.GetStateSnapshot(100)
	require.NotNil(t, after)
	assert.NotEqual(t, 1, after.ActivePlayerSeat,
		"skip_belot advances seat via finishCardPlay")
	assert.Nil(t, after.PendingBelotSeat, "skip_belot clears the pending prompt")
	require.NotNil(t, after.TurnExpiresAt)
	assert.True(t, after.TurnExpiresAt.After(time.Now()),
		"fresh timer for next seat must be in the future")
}
