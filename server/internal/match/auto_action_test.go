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
		{"pick_trump emits auto-action (forced dealer pick)", game.ActionPickTrump, ws.AutoActionPickTrump, true},
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

// TestPerMoveTimer_BiddingPhaseAutoPasses checks a bidding-phase timer expiry
// advances the active seat without panicking or leaving the session stuck.
func TestPerMoveTimer_BiddingPhaseAutoPasses(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	mgr.SetDealGraceForTest(false)

	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 1, 10, 120, 0, true, false, true))

	state := mgr.GetStateSnapshot(100)
	require.NotNil(t, state)
	require.Equal(t, game.PhaseBidding, state.Phase)
	firstBidder := state.ActivePlayerSeat

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

// TestAutoAction_SkipDeclare_ChainsToAutoPlayWithoutExtension checks a timer expiry
// during a declaration prompt chains into auto-play, so the next seat gets the fresh timer.
func TestAutoAction_SkipDeclare_ChainsToAutoPlayWithoutExtension(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 60, 10, 120, 0, true, false, true))

	gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
	gs.RoomID = 100
	gs.ActivePlayerSeat = 1
	gs.AwaitingDeclaration = true
	originalExpiry := time.Now().Add(-1 * time.Second)
	gs.TurnExpiresAt = &originalExpiry
	gs.TimerDurationSec = 60
	mgr.SetGameStateForTest(100, gs)

	mgr.TriggerTimerExpiryForTest(100, 1, 50*time.Millisecond)
	time.Sleep(400 * time.Millisecond)

	after := mgr.GetStateSnapshot(100)
	require.NotNil(t, after)
	assert.NotEqual(t, 1, after.ActivePlayerSeat,
		"chained AutoPlay must advance the seat past the doomed player")
	assert.Equal(t, 1, len(after.CurrentTrick),
		"chained auto-play should land exactly one card in the trick")
	require.NotNil(t, after.TurnExpiresAt)
	assert.True(t, after.TurnExpiresAt.After(time.Now()),
		"fresh timer for next seat must be in the future")
}

// TestAutoAction_ChainedBelotPrompt_AdvancesPastDoomedSeat checks the auto-action chain
// keeps resolving a belot prompt opened by its own K/Q play until the seat advances.
func TestAutoAction_ChainedBelotPrompt_AdvancesPastDoomedSeat(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 60, 10, 120, 0, true, false, true))

	gs := testfixtures.NewGameFirstTrick(game.SuitHearts)
	gs.RoomID = 100
	gs.ActivePlayerSeat = 0
	gs.AwaitingDeclaration = true
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
	assert.NotEqual(t, 0, after.ActivePlayerSeat,
		"chain must walk skip_declare → auto_play → skip_belot until seat advances")
	assert.Nil(t, after.PendingBelotSeat, "belot prompt for seat 0 must be resolved by the chain")
	require.NotNil(t, after.TurnExpiresAt)
	assert.True(t, after.TurnExpiresAt.After(time.Now()),
		"fresh timer must arm for the next seat once the chain settles")
	assert.Equal(t, 1, len(after.CurrentTrick), "exactly one card landed in the trick")
	played := after.CurrentTrick[0].Card.String()
	assert.Contains(t, []string{"KH", "QH"}, played,
		"the chained auto-play must commit a hearts trump K or Q (both held → belot trigger)")
}

// TestAutoAction_SkipBelot_AdvancesSeatAndArmsFreshTimer checks a timer-driven
// skip_belot advances the seat and arms a fresh timer for the next one.
func TestAutoAction_SkipBelot_AdvancesSeatAndArmsFreshTimer(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	require.NoError(t, mgr.StartMatch(100, "bitola", "1001", defaultPlayers(), "per-move", 60, 10, 120, 0, true, false, true))

	gs := testfixtures.NewGameFirstTrick(game.SuitDiamonds)
	gs.RoomID = 100
	gs.ActivePlayerSeat = 1
	pendingSeat := 1
	gs.PendingBelotSeat = &pendingSeat
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
	originalExpiry := time.Now().Add(-1 * time.Second)
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

// TestPerMoveTimer_BitolaRoundOnePassoutOpensRoundTwo checks timer-driven auto-passes
// walk Bitola round 1 into round 2, with no reshuffle until round 2 passes out.
func TestPerMoveTimer_BitolaRoundOnePassoutOpensRoundTwo(t *testing.T) {
	hub := &hubSpy{}
	mgr := match.NewManager(hub, newMockMatchRepo())

	const roomID = uint(100)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	gs := testfixtures.NewGameMidBidding(0)
	gs.RoomID = roomID
	mgr.SetGameStateForTest(roomID, gs)

	for pass := 1; pass <= 4; pass++ {
		before := mgr.GetStateSnapshot(roomID)
		require.NotNil(t, before)
		mgr.TriggerTimerExpiryForTest(roomID, before.ActivePlayerSeat, 10*time.Millisecond)
		require.Eventually(t, func() bool {
			snap := mgr.GetStateSnapshot(roomID)
			if snap == nil {
				return false
			}
			if pass < 4 {
				return snap.BiddingPassCount == pass
			}
			return snap.BiddingRound == 2
		}, 2*time.Second, 5*time.Millisecond, "auto-pass %d must land", pass)
	}

	after := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, after)
	assert.Equal(t, 2, after.BiddingRound, "a passed-out Bitola round 1 opens round 2")
	assert.Equal(t, 0, after.BiddingPassCount, "round 2 starts with a fresh pass count")
	assert.Equal(t, game.PhaseBidding, after.Phase, "no reshuffle — round 2 is live bidding")
	assert.Equal(t, gs.DealerSeat, after.DealerSeat, "the dealer does not rotate on a round-1 passout")
	assert.Equal(t, 1, after.HandNumber)
	require.NotNil(t, after.TrumpCandidate, "the candidate stays on the table into round 2")
}
