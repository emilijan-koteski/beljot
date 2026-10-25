package match

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/bot"
	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
	"github.com/emilijan/beljot/server/internal/ws"
)

// recordingBroadcaster captures every broadcast so a white-box test can assert
// which events broadcastActionResult emits, without a real WS hub/clients.
type recordingBroadcaster struct{ msgs [][]byte }

// rejectUnprojectedState panics when an event:match_state payload is sent
// through an identical-bytes primitive instead of per-seat projected SendFrames.
func rejectUnprojectedState(primitive string, msg []byte) {
	if json.Valid(msg) {
		var env struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(msg, &env); err == nil && env.Type == ws.EventMatchState {
			panic("recordingBroadcaster: " + ws.EventMatchState + " must ride SendFrames with a per-seat projection (Story 12.10); got it via " + primitive)
		}
	}
}

func (r *recordingBroadcaster) BroadcastToUsers(_ []uint, msg []byte) {
	rejectUnprojectedState("BroadcastToUsers", msg)
	cp := make([]byte, len(msg))
	copy(cp, msg)
	r.msgs = append(r.msgs, cp)
}
func (r *recordingBroadcaster) SendToUser(_ uint, msg []byte) {
	rejectUnprojectedState("SendToUser", msg)
	cp := make([]byte, len(msg))
	copy(cp, msg)
	r.msgs = append(r.msgs, cp)
}

// SendFrames records one entry per frames batch (the first frame stands in for
// the whole call — all frames in a batch carry the same event type), so the
// event-type/order assertions keep seeing one entry per logical state event.
func (r *recordingBroadcaster) SendFrames(frames []ws.UserFrame) {
	if len(frames) == 0 {
		return
	}
	cp := make([]byte, len(frames[0].Msg))
	copy(cp, frames[0].Msg)
	r.msgs = append(r.msgs, cp)
}

// eventTypes returns the "type" field of every recorded message, in order.
func (r *recordingBroadcaster) eventTypes() []string {
	out := make([]string, 0, len(r.msgs))
	for _, raw := range r.msgs {
		var env struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &env); err == nil {
			out = append(out, env.Type)
		}
	}
	return out
}

// payloadOf returns the decoded payload of the first message of the given type.
func (r *recordingBroadcaster) payloadOf(t *testing.T, eventType string) map[string]any {
	t.Helper()
	for _, raw := range r.msgs {
		var env struct {
			Type    string         `json:"type"`
			Payload map[string]any `json:"payload"`
		}
		if err := json.Unmarshal(raw, &env); err == nil && env.Type == eventType {
			return env.Payload
		}
	}
	return nil
}

func intPtr(n int) *int { return &n }

// TestTrickResolvedWinnerSeat checks the trick_resolved winner seat, which on a
// non-final hand's last trick must come from LastHandResult.LastTrickSeat.
func TestTrickResolvedWinnerSeat(t *testing.T) {
	tests := []struct {
		name     string
		oldState *game.GameState
		newState *game.GameState
		want     int
	}{
		{
			name:     "tricks 1-7: winner leads next via ActivePlayerSeat",
			oldState: &game.GameState{HandNumber: 1},
			newState: &game.GameState{HandNumber: 1, ActivePlayerSeat: 2, TrickWinnerSeat: nil},
			want:     2,
		},
		{
			name:     "last trick of a continuing hand: read preserved seat, not the next bidder",
			oldState: &game.GameState{HandNumber: 1},
			newState: &game.GameState{
				HandNumber:       2,
				ActivePlayerSeat: 0,
				TrickWinnerSeat:  nil,
				LastHandResult:   &game.HandScore{LastTrickSeat: 3},
			},
			want: 3,
		},
		{
			name:     "last trick at match end: TrickWinnerSeat still set",
			oldState: &game.GameState{HandNumber: 1},
			newState: &game.GameState{HandNumber: 1, ActivePlayerSeat: 0, TrickWinnerSeat: intPtr(3)},
			want:     3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, trickResolvedWinnerSeat(tt.oldState, tt.newState))
		})
	}
}

func indexOf(xs []string, s string) int {
	for i, x := range xs {
		if x == s {
			return i
		}
	}
	return len(xs)
}

// TestBroadcastActionResult_BelotCompletingTrickEmitsTrickResolved checks that a
// Belot action resolving a deferred trick emits trick_resolved before match_state.
func TestBroadcastActionResult_BelotCompletingTrickEmitsTrickResolved(t *testing.T) {
	rec := &recordingBroadcaster{}
	m := &Manager{hub: rec}

	trump := game.SuitHearts
	leadSpades := game.SuitSpades
	pending := 2

	oldState := &game.GameState{
		Phase:                game.PhasePlaying,
		HandNumber:           1,
		TrumpSuit:            &trump,
		TrickNumber:          2,
		LeadSuit:             &leadSpades,
		DeclarationsResolved: true,
		PendingBelotSeat:     &pending,
		ActivePlayerSeat:     2,
		CurrentTrick: []game.TrickCard{
			{Card: game.Card{Rank: game.RankAce, Suit: game.SuitSpades}, PlayerSeat: 3},
			{Card: game.Card{Rank: game.RankKing, Suit: game.SuitSpades}, PlayerSeat: 0},
			{Card: game.Card{Rank: game.RankQueen, Suit: game.SuitSpades}, PlayerSeat: 1},
			{Card: game.Card{Rank: game.RankKing, Suit: game.SuitHearts}, PlayerSeat: 2},
		},
	}
	newState := &game.GameState{
		Phase:                game.PhasePlaying,
		HandNumber:           1,
		TrumpSuit:            &trump,
		TrickNumber:          3,
		DeclarationsResolved: true,
		BelotAnnounced:       true,
		CurrentTrick:         []game.TrickCard{},
		ActivePlayerSeat:     2,
	}

	m.broadcastActionResult([4]uint{10, 20, 30, 40}, oldState, newState,
		game.Action{Type: game.ActionAnnounceBelot, PlayerSeat: 2}, false)

	types := rec.eventTypes()
	assert.Contains(t, types, ws.EventTrickResolved,
		"a Belot-completed trick must broadcast event:trick_resolved (got %v)", types)

	payload := rec.payloadOf(t, ws.EventTrickResolved)
	require.NotNil(t, payload)
	assert.Equal(t, float64(2), payload["winnerSeat"], "seat 2 wins the trick with trump KH")

	assert.Less(t, indexOf(types, ws.EventTrickResolved), indexOf(types, ws.EventMatchState),
		"trick_resolved must come before match_state (got %v)", types)
}

// TestBroadcastActionResult_BelotMidTrickEmitsNoTrickResolved guards the other
// branch: a Belot on a non-final card of the trick (here the 2nd) must NOT emit
// trick_resolved — the trick hasn't resolved yet.
func TestBroadcastActionResult_BelotMidTrickEmitsNoTrickResolved(t *testing.T) {
	rec := &recordingBroadcaster{}
	m := &Manager{hub: rec}

	trump := game.SuitHearts
	leadHearts := game.SuitHearts
	pending := 1

	oldState := &game.GameState{
		Phase:                game.PhasePlaying,
		HandNumber:           1,
		TrumpSuit:            &trump,
		TrickNumber:          2,
		LeadSuit:             &leadHearts,
		DeclarationsResolved: true,
		PendingBelotSeat:     &pending,
		ActivePlayerSeat:     1,
		CurrentTrick: []game.TrickCard{
			{Card: game.Card{Rank: game.Rank7, Suit: game.SuitHearts}, PlayerSeat: 0},
			{Card: game.Card{Rank: game.RankKing, Suit: game.SuitHearts}, PlayerSeat: 1},
		},
	}
	newState := &game.GameState{
		Phase:                game.PhasePlaying,
		HandNumber:           1,
		TrumpSuit:            &trump,
		TrickNumber:          2,
		LeadSuit:             &leadHearts,
		DeclarationsResolved: true,
		BelotAnnounced:       true,
		ActivePlayerSeat:     2,
		CurrentTrick: []game.TrickCard{
			{Card: game.Card{Rank: game.Rank7, Suit: game.SuitHearts}, PlayerSeat: 0},
			{Card: game.Card{Rank: game.RankKing, Suit: game.SuitHearts}, PlayerSeat: 1},
		},
	}

	m.broadcastActionResult([4]uint{10, 20, 30, 40}, oldState, newState,
		game.Action{Type: game.ActionAnnounceBelot, PlayerSeat: 1}, false)

	assert.NotContains(t, rec.eventTypes(), ws.EventTrickResolved,
		"a mid-trick Belot must not emit trick_resolved")
}

// TestBroadcastActionResult_HandCompleteEmitsHandScored checks that the last trick
// emits hand_scored followed by the PhaseHandComplete state, not match_end.
func TestBroadcastActionResult_HandCompleteEmitsHandScored(t *testing.T) {
	rec := &recordingBroadcaster{}
	m := &Manager{hub: rec}

	trump := game.SuitHearts
	winner := 3
	oldState := &game.GameState{
		Phase:       game.PhasePlaying,
		HandNumber:  1,
		TrumpSuit:   &trump,
		TrickNumber: 8,
		CurrentTrick: []game.TrickCard{
			{Card: game.Card{Rank: game.RankAce, Suit: game.SuitSpades}, PlayerSeat: 0},
			{Card: game.Card{Rank: game.RankKing, Suit: game.SuitSpades}, PlayerSeat: 1},
			{Card: game.Card{Rank: game.RankQueen, Suit: game.SuitSpades}, PlayerSeat: 2},
		},
	}
	newState := &game.GameState{
		Phase:           game.PhaseHandComplete,
		HandNumber:      1,
		TrumpSuit:       &trump,
		TrickNumber:     8,
		TrickWinnerSeat: &winner,
		CurrentTrick:    []game.TrickCard{},
		TeamScores:      [2]int{70, 92},
		LastHandResult: &game.HandScore{
			HandNumber:    1,
			LastTrickTeam: 1, LastTrickSeat: 3, LastTrickBonus: 10,
		},
	}
	card := game.Card{Rank: game.Rank7, Suit: game.SuitHearts}

	m.broadcastActionResult([4]uint{10, 20, 30, 40}, oldState, newState,
		game.Action{Type: game.ActionPlayCard, PlayerSeat: 3, Card: &card}, false)

	types := rec.eventTypes()
	assert.Contains(t, types, ws.EventHandScored,
		"hand_scored must fire on the PhaseHandComplete transition (got %v)", types)
	assert.Contains(t, types, ws.EventTrickResolved)
	assert.Contains(t, types, ws.EventMatchState)
	assert.NotContains(t, types, ws.EventMatchEnd)
	assert.Equal(t, float64(3), rec.payloadOf(t, ws.EventTrickResolved)["winnerSeat"],
		"final trick winner comes from the still-set TrickWinnerSeat")
}

// TestBroadcastActionResult_ContinueAdvanceBroadcastsState verifies that a
// continue which dealt the next hand simply syncs the authoritative state.
func TestBroadcastActionResult_ContinueAdvanceBroadcastsState(t *testing.T) {
	rec := &recordingBroadcaster{}
	m := &Manager{hub: rec}

	oldState := &game.GameState{Phase: game.PhaseHandComplete, HandNumber: 1}
	newState := &game.GameState{Phase: game.PhaseBidding, HandNumber: 2}

	m.broadcastActionResult([4]uint{10, 20, 30, 40}, oldState, newState,
		game.Action{Type: game.ActionContinue, PlayerSeat: 0}, false)

	assert.Equal(t, []string{ws.EventMatchState}, rec.eventTypes(),
		"a continue advance just syncs authoritative state")
}

// TestBroadcastActionResult_DeclareEmitsPlayerDeclared checks that a declare
// broadcasts player_declared before match_state and a skip stays silent.
func TestBroadcastActionResult_DeclareEmitsPlayerDeclared(t *testing.T) {
	trump := game.SuitHearts

	declareStates := func() (*game.GameState, *game.GameState) {
		oldState := &game.GameState{
			Phase:               game.PhasePlaying,
			HandNumber:          1,
			TrumpSuit:           &trump,
			TrickNumber:         1,
			AwaitingDeclaration: true,
			ActivePlayerSeat:    2,
		}
		newState := &game.GameState{
			Phase:            game.PhasePlaying,
			HandNumber:       1,
			TrumpSuit:        &trump,
			TrickNumber:      1,
			ActivePlayerSeat: 2,
		}
		return oldState, newState
	}

	t.Run("declare broadcasts player_declared before match_state", func(t *testing.T) {
		rec := &recordingBroadcaster{}
		m := &Manager{hub: rec}
		oldState, newState := declareStates()

		m.broadcastActionResult([4]uint{10, 20, 30, 40}, oldState, newState,
			game.Action{Type: game.ActionDeclare, PlayerSeat: 2}, false)

		types := rec.eventTypes()
		assert.Contains(t, types, ws.EventPlayerDeclared,
			"a declare must announce the declarer to the table (got %v)", types)

		payload := rec.payloadOf(t, ws.EventPlayerDeclared)
		require.NotNil(t, payload)
		assert.Equal(t, float64(2), payload["playerSeat"])
		assert.Len(t, payload, 1,
			"payload must carry the seat ONLY — meld details stay secret until declarations_resolved")

		assert.Less(t, indexOf(types, ws.EventPlayerDeclared), indexOf(types, ws.EventMatchState),
			"player_declared must come before match_state (got %v)", types)
	})

	t.Run("skip_declare stays silent", func(t *testing.T) {
		rec := &recordingBroadcaster{}
		m := &Manager{hub: rec}
		oldState, newState := declareStates()

		m.broadcastActionResult([4]uint{10, 20, 30, 40}, oldState, newState,
			game.Action{Type: game.ActionSkipDeclare, PlayerSeat: 2}, false)

		assert.NotContains(t, rec.eventTypes(), ws.EventPlayerDeclared,
			"skipping declarations must not announce anything")
	})
}

// TestHandleTimerExpiry_ForcedDealerPickResolvesBidding checks that a timeout on
// a dealer who cannot pass picks trump instead of re-arming the same seat forever.
func TestHandleTimerExpiry_ForcedDealerPickResolvesBidding(t *testing.T) {
	rec := &recordingBroadcaster{}
	m := NewManager(rec, nil)

	const roomID = uint(100)
	players := [4]PlayerSeatInfo{
		{UserID: 10, Username: "a", Seat: 0},
		{UserID: 20, Username: "b", Seat: 1},
		{UserID: 30, Username: "c", Seat: 2},
		{UserID: 40, Username: "d", Seat: 3},
	}
	require.NoError(t, m.StartMatch(roomID, string(game.VariantCroatia), "1001", players, "per-move", 10, 10, 120, 0, true, false, true, false))
	t.Cleanup(func() { m.RemoveSession(roomID) })

	gs := testfixtures.NewGameCroatianMidBidding(3)
	gs.RoomID = roomID
	require.True(t, game.MustPickTrump(gs, gs.ActivePlayerSeat), "the fixture must be the forced-pick state")
	require.Equal(t, gs.DealerSeat, gs.ActivePlayerSeat, "the dealer must be the seat on the clock")
	m.SetGameStateForTest(roomID, gs)

	m.TriggerTimerExpiryForTest(roomID, gs.ActivePlayerSeat, 10*time.Millisecond)

	var resolvedSnap *game.GameState
	var declExpiry time.Time
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		if snap := m.GetStateSnapshot(roomID); snap != nil && snap.TrumpSuit != nil {
			resolvedSnap = snap
			declExpiry = m.DeclarationExpiresAtForTest(roomID)
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	require.NotNil(t, resolvedSnap, "the expired forced-dealer turn must resolve bidding, not loop on a rejected pass")

	assert.Contains(t, rec.eventTypes(), ws.EventAutoAction,
		"the forced auto-pick must announce itself (got %v)", rec.eventTypes())
	autoPayload := rec.payloadOf(t, ws.EventAutoAction)
	require.NotNil(t, autoPayload, "event:auto_action carried no payload")
	assert.Equal(t, string(ws.AutoActionPickTrump), autoPayload["type"])
	assert.Equal(t, float64(gs.DealerSeat), autoPayload["playerSeat"])

	after := m.GetStateSnapshot(roomID)
	require.NotNil(t, after)
	require.NotNil(t, after.TrumpCallerSeat)
	assert.Equal(t, gs.DealerSeat, *after.TrumpCallerSeat, "the dealer is the taker — nobody else can be")
	assert.NotEqual(t, game.PhaseBidding, after.Phase, "bidding must be over")
	for seat, p := range after.Players {
		assert.Len(t, p.Hand, 8, "seat %d must hold all eight cards once bidding resolves", seat)
		assert.Zero(t, p.FaceDownCount, "seat %d", seat)
	}

	require.Equal(t, game.PhaseDeclaring, resolvedSnap.Phase, "the pick opens the declaration phase")
	assert.Nil(t, resolvedSnap.TurnExpiresAt, "the phase has no active turn and no per-move clock")
	assert.False(t, declExpiry.IsZero(),
		"the phase's fixed window must be armed by the timer-expiry door too")
}

// TestBuildBotView_FaceDownNeverVisible checks that no seat's face-down cards
// ever reach a bot's View during bidding, its own included.
func TestBuildBotView_FaceDownNeverVisible(t *testing.T) {
	for passCount := 0; passCount <= 3; passCount++ {
		gs := testfixtures.NewGameCroatianMidBidding(passCount)
		for seat := 0; seat < 4; seat++ {
			v := buildBotView(gs, seat, nil)
			assert.Len(t, v.Hand, 6, "passCount %d seat %d bids on six visible cards", passCount, seat)
			for other := 0; other < 4; other++ {
				for _, c := range gs.Players[other].FaceDownCards {
					assert.NotContains(t, v.Hand, c,
						"passCount %d: seat %d's view leaked seat %d's hidden card %s",
						passCount, seat, other, c)
				}
			}
		}
	}
}

// TestBuildBotView_MustPickTrumpIsSeatScoped checks that the must-pick-trump flag
// is set only for the seat on the clock and derives from the rule config.
func TestBuildBotView_MustPickTrumpIsSeatScoped(t *testing.T) {
	forced := testfixtures.NewGameCroatianMidBidding(3)
	require.True(t, game.MustPickTrump(forced, forced.ActivePlayerSeat))
	for seat := 0; seat < 4; seat++ {
		v := buildBotView(forced, seat, nil)
		assert.Equal(t, seat == forced.ActivePlayerSeat, v.MustPickTrump, "seat %d", seat)
	}

	open := testfixtures.NewGameCroatianMidBidding(2)
	require.False(t, game.MustPickTrump(open, open.ActivePlayerSeat))
	for seat := 0; seat < 4; seat++ {
		assert.False(t, buildBotView(open, seat, nil).MustPickTrump, "seat %d", seat)
	}

	bitola := testfixtures.NewGameMidBidding(7)
	require.False(t, game.MustPickTrump(bitola, bitola.ActivePlayerSeat))
	for seat := 0; seat < 4; seat++ {
		assert.False(t, buildBotView(bitola, seat, nil).MustPickTrump, "seat %d", seat)
	}
}

// TestBuildBotView_HasTrumpCandidateFollowsTheVariant checks that the candidate flag mirrors the
// rule config: set for Bitola and unset for Croatian, for every seat.
func TestBuildBotView_HasTrumpCandidateFollowsTheVariant(t *testing.T) {
	bitola := testfixtures.NewGameMidBidding(0)
	require.True(t, bitola.Rules.HasTrumpCandidate)
	croatian := testfixtures.NewGameCroatianMidBidding(0)
	require.False(t, croatian.Rules.HasTrumpCandidate)
	for seat := 0; seat < 4; seat++ {
		assert.True(t, buildBotView(bitola, seat, nil).HasTrumpCandidate, "bitola seat %d", seat)
		assert.False(t, buildBotView(croatian, seat, nil).HasTrumpCandidate, "croatian seat %d", seat)
	}
}

// TestBuildBotView_CarriesTheRevealedDeclarationCards checks that the cards the bot memory holds
// as publicly declared reach the view, seat by seat.
func TestBuildBotView_CarriesTheRevealedDeclarationCards(t *testing.T) {
	gs := testfixtures.NewGameMidPlay(1)
	declared := []game.Card{
		{Rank: game.RankQueen, Suit: game.SuitSpades},
		{Rank: game.RankKing, Suit: game.SuitSpades},
		{Rank: game.RankAce, Suit: game.SuitSpades},
	}
	gs.Players[1].Declarations = []game.Declaration{{
		Type:       game.DeclarationSequence,
		Cards:      declared,
		PlayerSeat: 1,
	}}
	mem := bot.NewMemory()
	mem.ObserveDeclarations(gs.Players)

	v := buildBotView(gs, 0, mem)

	assert.Equal(t, declared, v.DeclaredCards[1])
	assert.Empty(t, v.DeclaredCards[0])
	assert.Empty(t, v.DeclaredCards[2])
	assert.Empty(t, v.DeclaredCards[3])
}
