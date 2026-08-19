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

// faceDownSends extracts every event:face_down_revealed message the spy saw,
// paired with the single recipient it went to.
type faceDownSend struct {
	recipients []uint
	payload    ws.FaceDownRevealedPayload
}

func faceDownSends(t *testing.T, calls []hubCall) []faceDownSend {
	t.Helper()
	var out []faceDownSend
	for _, c := range calls {
		var msg struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}
		require.NoError(t, json.Unmarshal(c.msg, &msg))
		if msg.Type != ws.EventFaceDownRevealed {
			continue
		}
		var payload ws.FaceDownRevealedPayload
		require.NoError(t, json.Unmarshal(msg.Payload, &payload))
		out = append(out, faceDownSend{recipients: c.userIDs, payload: payload})
	}
	return out
}

// TestFaceDownReveal_EachSeatGetsOnlyItsOwnCards is the per-seat delivery
// assertion: when Croatian bidding enters round 2, every seat receives exactly
// one reveal, addressed to that seat's user alone, carrying only that seat's two
// cards — and never any other seat's.
func TestFaceDownReveal_EachSeatGetsOnlyItsOwnCards(t *testing.T) {
	hub := &hubSpy{}
	mgr := match.NewManager(hub, newMockMatchRepo())

	const roomID = uint(100)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	// Inject a Croatian state one pass short of round 2. Seats map to the
	// defaultPlayers user IDs (seat i -> (i+1)*10).
	gs := testfixtures.NewGameCroatianMidBidding(3)
	gs.RoomID = roomID
	expected := map[int][]string{}
	for seat := range gs.Players {
		ids := make([]string, 0, 2)
		for _, c := range gs.Players[seat].FaceDownCards {
			ids = append(ids, c.String())
		}
		require.Len(t, ids, 2, "seat %d must start with two hidden cards", seat)
		expected[seat] = ids
	}
	mgr.SetGameStateForTest(roomID, gs)

	// The fourth round-1 pass opens round 2 and fires the reveal.
	mgr.TriggerTimerExpiryForTest(roomID, gs.ActivePlayerSeat, 10*time.Millisecond)

	// Poll rather than sleep a fixed span: the timer is asynchronous, so a fixed
	// wait is either a flake on a loaded box or wasted wall clock on an idle one.
	require.Eventually(t, func() bool {
		return len(faceDownSends(t, hub.snapshot())) == 4
	}, 2*time.Second, 5*time.Millisecond, "expected one reveal per seat")

	sends := faceDownSends(t, hub.snapshot())
	require.Len(t, sends, 4, "one reveal per seat, no more")

	seen := map[int]bool{}
	for _, s := range sends {
		require.Len(t, s.recipients, 1, "a reveal must be a single-recipient send, never a broadcast")
		seat := s.payload.PlayerSeat
		require.Contains(t, expected, seat)
		assert.False(t, seen[seat], "seat %d received two reveals", seat)
		seen[seat] = true

		// Addressed to that seat's own user.
		assert.Equal(t, uint((seat+1)*10), s.recipients[0],
			"seat %d's reveal went to the wrong user", seat)

		// Carries that seat's two cards and nothing else.
		assert.ElementsMatch(t, expected[seat], s.payload.CardIDs)
		for otherSeat, otherIDs := range expected {
			if otherSeat == seat {
				continue
			}
			for _, id := range otherIDs {
				assert.NotContains(t, s.payload.CardIDs, id,
					"seat %d's reveal leaked seat %d's card %s", seat, otherSeat, id)
			}
		}
	}
	assert.Len(t, seen, 4)

	// The authoritative state still keeps the cards out of every hand.
	after := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, after)
	assert.Equal(t, 2, after.BiddingRound)
	assert.True(t, after.FaceDownRevealed)
	for i, p := range after.Players {
		assert.Len(t, p.Hand, 6, "seat %d's open hand must not grow on the reveal", i)
		assert.Len(t, p.FaceDownCards, 2)
	}
}

// TestFaceDownReveal_SkipsBotSeats exercises the bot-seat branch of
// sendFaceDownReveals: a bot carries UserID 0 and has no socket, and it reads
// the game state directly through buildBotView, so it must be skipped rather
// than sent to. Three humans means exactly three reveals.
func TestFaceDownReveal_SkipsBotSeats(t *testing.T) {
	hub := &hubSpy{}
	mgr := match.NewManager(hub, newMockMatchRepo())

	const roomID = uint(100)
	// Seat 1 is a bot: UserID 0, no username — exactly what the room layer sends
	// for a bot seat.
	players := defaultPlayers()
	players[1] = match.PlayerSeatInfo{UserID: 0, Username: "", Seat: 1, IsBot: true}
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", players, "relaxed", 0, 10, 120, 0))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	gs := testfixtures.NewGameCroatianMidBidding(3)
	gs.RoomID = roomID
	gs.Players[1].IsBot = true
	gs.Players[1].UserID = 0
	gs.Players[1].Username = ""
	mgr.SetGameStateForTest(roomID, gs)

	mgr.TriggerTimerExpiryForTest(roomID, gs.ActivePlayerSeat, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		snap := mgr.GetStateSnapshot(roomID)
		return snap != nil && snap.FaceDownRevealed
	}, 2*time.Second, 5*time.Millisecond, "expected the auto-pass to open round 2")

	// Give any (incorrect) fourth send a chance to land before asserting the
	// count, so this cannot pass merely by observing the state early.
	require.Eventually(t, func() bool {
		return len(faceDownSends(t, hub.snapshot())) == 3
	}, 2*time.Second, 5*time.Millisecond, "expected exactly three reveals (one per human seat)")

	sends := faceDownSends(t, hub.snapshot())
	require.Len(t, sends, 3, "the bot seat must receive no reveal")
	for _, s := range sends {
		require.Len(t, s.recipients, 1)
		assert.NotEqual(t, uint(0), s.recipients[0], "a reveal must never be addressed to UserID 0")
		assert.NotEqual(t, 1, s.payload.PlayerSeat, "the bot seat must receive no reveal")
	}

	// The bot's own hidden cards still exist server-side — they are simply not
	// pushed anywhere, because buildBotView reads them from the state.
	after := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, after)
	assert.Len(t, after.Players[1].FaceDownCards, 2)
}

// TestFaceDownReveal_NotEmittedForBitola guards the Bitola wire: the round-1 to
// round-2 transition must produce no reveal at all.
func TestFaceDownReveal_NotEmittedForBitola(t *testing.T) {
	hub := &hubSpy{}
	mgr := match.NewManager(hub, newMockMatchRepo())

	const roomID = uint(100)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	gs := testfixtures.NewGameMidBidding(3)
	gs.RoomID = roomID
	mgr.SetGameStateForTest(roomID, gs)

	mgr.TriggerTimerExpiryForTest(roomID, gs.ActivePlayerSeat, 10*time.Millisecond)

	// Wait on the observable effect of the auto-pass — the round advancing —
	// then assert that no reveal rode along with it.
	require.Eventually(t, func() bool {
		snap := mgr.GetStateSnapshot(roomID)
		return snap != nil && snap.BiddingRound == 2
	}, 2*time.Second, 5*time.Millisecond, "expected the auto-pass to open round 2")

	assert.Empty(t, faceDownSends(t, hub.snapshot()),
		"a variant with no face-down cards must emit no reveal")

	after := mgr.GetStateSnapshot(roomID)
	require.NotNil(t, after)
	assert.Equal(t, 2, after.BiddingRound)
	assert.False(t, after.FaceDownRevealed)
}

// TestSyncStateOnConnect_ReplaysOwnFaceDownCards covers the reconnect row of the
// matrix: a player who reconnects after the reveal gets their own two cards
// re-sent, and nobody else's.
func TestSyncStateOnConnect_ReplaysOwnFaceDownCards(t *testing.T) {
	hub := &hubSpy{}
	mgr := match.NewManager(hub, newMockMatchRepo())

	const roomID = uint(100)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	// Round 2 already open, cards already revealed.
	gs := testfixtures.NewGameCroatianMidBidding(4)
	gs.RoomID = roomID
	require.True(t, gs.FaceDownRevealed)
	seat2Cards := []string{}
	for _, c := range gs.Players[2].FaceDownCards {
		seat2Cards = append(seat2Cards, c.String())
	}
	mgr.SetGameStateForTest(roomID, gs)

	// Seat 2's user (30) reconnects.
	mgr.SyncStateOnConnect(30)

	sends := faceDownSends(t, hub.snapshot())
	require.Len(t, sends, 1, "exactly one replay, for the reconnecting seat only")
	assert.Equal(t, []uint{30}, sends[0].recipients)
	assert.Equal(t, 2, sends[0].payload.PlayerSeat)
	assert.ElementsMatch(t, seat2Cards, sends[0].payload.CardIDs)
}

// TestSyncStateOnConnect_NoReplayBeforeReveal keeps the reconnect replay honest:
// during round 1 the cards are secret even from their owner, so nothing is sent.
func TestSyncStateOnConnect_NoReplayBeforeReveal(t *testing.T) {
	hub := &hubSpy{}
	mgr := match.NewManager(hub, newMockMatchRepo())

	const roomID = uint(100)
	require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	gs := testfixtures.NewGameCroatianJustDealt()
	gs.RoomID = roomID
	require.False(t, gs.FaceDownRevealed)
	mgr.SetGameStateForTest(roomID, gs)

	mgr.SyncStateOnConnect(30)

	assert.Empty(t, faceDownSends(t, hub.snapshot()),
		"round-1 face-down cards are secret even from their owner")
}

// TestTrumpSelected_EmittedWithEmptyCardIDWhenNoCandidate locks the emit change:
// a take under a candidate-less variant must still fire event:trump_selected,
// with an empty cardId rather than being suppressed.
func TestTrumpSelected_EmittedWithEmptyCardIDWhenNoCandidate(t *testing.T) {
	tests := []struct {
		name       string
		state      func() *game.GameState
		suit       *game.Suit
		wantCardID string
	}{
		{
			name:       "no candidate — empty cardId",
			state:      func() *game.GameState { return testfixtures.NewGameCroatianJustDealt() },
			suit:       func() *game.Suit { s := game.SuitSpades; return &s }(),
			wantCardID: "",
		},
		{
			name:       "candidate present — the absorbed card rides along",
			state:      func() *game.GameState { return testfixtures.NewGameJustDealt() },
			suit:       nil,
			wantCardID: "AH", // the fixture's face-up candidate
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hub := &hubSpy{}
			mgr := match.NewManager(hub, newMockMatchRepo())

			const roomID = uint(100)
			require.NoError(t, mgr.StartMatch(roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0))
			t.Cleanup(func() { mgr.RemoveSession(roomID) })

			gs := tc.state()
			gs.RoomID = roomID
			mgr.SetGameStateForTest(roomID, gs)

			require.NoError(t, mgr.ApplyActionForTest(roomID, game.Action{
				Type:       game.ActionPickTrump,
				PlayerSeat: gs.ActivePlayerSeat,
				Suit:       tc.suit,
			}))

			var found bool
			for _, c := range hub.snapshot() {
				var msg struct {
					Type    string          `json:"type"`
					Payload json.RawMessage `json:"payload"`
				}
				require.NoError(t, json.Unmarshal(c.msg, &msg))
				if msg.Type != ws.EventTrumpSelected {
					continue
				}
				var payload ws.TrumpSelectedPayload
				require.NoError(t, json.Unmarshal(msg.Payload, &payload))
				found = true
				assert.Equal(t, tc.wantCardID, payload.CardID)
				assert.Equal(t, gs.ActivePlayerSeat, payload.PlayerSeat)
				assert.NotEmpty(t, payload.TrumpSuit)
			}
			assert.True(t, found, "event:trump_selected must be emitted for every successful take")
		})
	}
}
