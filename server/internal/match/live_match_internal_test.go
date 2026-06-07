package match

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/ws"
)

// recordingBroadcaster captures every broadcast so a white-box test can assert
// which events broadcastActionResult emits, without a real WS hub/clients.
type recordingBroadcaster struct{ msgs [][]byte }

func (r *recordingBroadcaster) BroadcastToUsers(_ []uint, msg []byte) {
	cp := make([]byte, len(msg))
	copy(cp, msg)
	r.msgs = append(r.msgs, cp)
}
func (r *recordingBroadcaster) SendToUser(_ uint, msg []byte) {
	cp := make([]byte, len(msg))
	copy(cp, msg)
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

// TestTrickResolvedWinnerSeat covers the three states the winner can live in by
// the time event:trick_resolved is broadcast. The middle case is the
// regression: on a non-final hand's last trick, startNewHand has cleared
// TrickWinnerSeat and advanced ActivePlayerSeat to the next bidder, so the
// winner must be read from LastHandResult.LastTrickSeat — not ActivePlayerSeat.
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
				HandNumber:       2,   // startNewHand incremented it
				ActivePlayerSeat: 0,   // next hand's first bidder (the buggy fallback)
				TrickWinnerSeat:  nil, // cleared by startNewHand
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

// TestBroadcastActionResult_BelotCompletingTrickEmitsTrickResolved is the
// regression for the missing-collect bug: when a trump K/Q is the 4th card of a
// trick it triggers a Belot prompt and handlePlayCard defers resolution, so the
// play_card broadcast carries no trick_resolved. The resolution happens under
// the Belot action, which previously emitted only belot_announced + match_state
// — leaving the client without pendingResolvedTrick, so the trick never swept to
// the winner. The Belot branch must now emit trick_resolved (before match_state).
func TestBroadcastActionResult_BelotCompletingTrickEmitsTrickResolved(t *testing.T) {
	rec := &recordingBroadcaster{}
	m := &Manager{hub: rec}

	trump := game.SuitHearts
	leadSpades := game.SuitSpades
	pending := 2

	// oldState: trump KH was just played as the 4th card of trick 2; the Belot
	// prompt deferred resolution (PendingBelotSeat set, trick still has 4 cards).
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
	// newState: announce_belot ran finishCardPlay -> resolveTrick. Seat 2's trump
	// KH beats the three spades, so seat 2 wins and leads next (tricks 1-7 path:
	// ActivePlayerSeat = winner, TrickWinnerSeat cleared).
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

	// Ordering: trick_resolved must precede the authoritative match_state so the
	// client arms pendingResolvedTrick before the trick is cleared from state.
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

	// Two cards played; seat 1 announced Belot on its trump K. Turn advances, no
	// resolution (only 2 cards in the trick).
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
		ActivePlayerSeat:     2, // advanced to the next player; trick continues
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

// TestBroadcastActionResult_HandCompleteEmitsHandScored guards the hand-scored
// detection: scoreHand now holds in PhaseHandComplete (no HandNumber increment),
// so the broadcast must fire event:hand_scored on the PhasePlaying ->
// PhaseHandComplete transition and follow with the PhaseHandComplete state — NOT
// a next-hand state, and NOT match_end.
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
		Phase:           game.PhaseHandComplete, // scored, holding for continue
		HandNumber:      1,                      // NOT incremented (next hand not dealt)
		TrumpSuit:       &trump,
		TrickNumber:     8,
		TrickWinnerSeat: &winner,
		CurrentTrick:    []game.TrickCard{},
		TeamScores:      [2]int{70, 92},
		LastHandResult:  &game.HandScore{LastTrickTeam: 1, LastTrickSeat: 3, LastTrickBonus: 10},
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
