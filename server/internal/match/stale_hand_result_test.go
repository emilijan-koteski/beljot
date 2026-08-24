package match_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/ws"
)

// A HandScore OUTLIVES its hand: startNewHand deliberately never clears
// LastHandResult, because it has to survive for the broadcast that follows
// scoring. So from hand 2 onward every state carries the PREVIOUS hand's result,
// and anything reacting to "LastHandResult is non-nil" must ask which hand it
// describes.
//
// Two match ends used to get that wrong, both re-announcing and re-persisting a
// hand that had already finished:
//
//   - an accepted surrender in hand 2 or later;
//   - an instant win on a freshly dealt hand (continue -> deal -> all 8 trumps).
//
// The duplicate row is the worse half: hand_results has a UNIQUE
// (match_id, hand_number) constraint, so it fails the whole CreateWithHands
// transaction and the match record is lost entirely rather than one row.
//
// Found while establishing why the "dosta" stop must nil LastHandResult — the
// stop trips the same two gates.

func staleResult(handNumber int) *game.HandScore {
	return &game.HandScore{
		HandNumber:      handNumber,
		TeamACardPoints: 81,
		TeamBCardPoints: 81,
		LastTrickTeam:   game.TeamA,
		LastTrickBonus:  10,
		ContractingTeam: game.TeamA,
		TeamAHandTotal:  111,
		TeamBHandTotal:  81,
	}
}

func TestBufferHandResult_StaleResultIsNotRePersisted(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	mgr := match.NewManager(hub, newMockMatchRepo())
	require.NoError(t, mgr.StartMatch(
		910, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false,
	))
	t.Cleanup(func() { mgr.RemoveSession(910) })

	// Hand 2 finishes and is buffered by the continue that deals hand 3.
	mgr.BufferHandResultIfScored(910,
		&game.GameState{HandNumber: 2},
		&game.GameState{HandNumber: 3, LastHandResult: staleResult(2)})
	require.Len(t, mgr.HandResults(910), 1)

	// Now a surrender is accepted during hand 3. The state still carries hand 2's
	// result, and a surrender transitions into match_end, so the old gate fired.
	mgr.BufferHandResultIfScored(910,
		&game.GameState{HandNumber: 3},
		&game.GameState{HandNumber: 3, Phase: game.PhaseMatchEnd, LastHandResult: staleResult(2)})

	hands := mgr.HandResults(910)
	require.Len(t, hands, 1,
		"hand 2 was already recorded; a surrender in hand 3 must not record it again")
	assert.Equal(t, 2, hands[0].HandNumber,
		"and the one row must carry the hand it actually describes, not hand 3")
}

func TestBufferHandResult_StampsTheResultsOwnHand(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()
	defer hub.Shutdown()

	mgr := match.NewManager(hub, newMockMatchRepo())
	require.NoError(t, mgr.StartMatch(
		911, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false,
	))
	t.Cleanup(func() { mgr.RemoveSession(911) })

	// A stale hand-1 result reaching a match end in hand 4 must be stamped 1, not
	// 4. Stamping it 4 is what produced a row for a hand nobody played.
	mgr.BufferHandResultIfScored(911,
		&game.GameState{HandNumber: 4},
		&game.GameState{HandNumber: 4, Phase: game.PhaseMatchEnd, LastHandResult: staleResult(1)})

	hands := mgr.HandResults(911)
	require.Len(t, hands, 1)
	assert.Equal(t, 1, hands[0].HandNumber)
}

// MatchDurationSec is computed from a session timestamp captured separately at
// each of four match-end call sites, and a zero time.Time yields the seconds
// since year 1 — about 6.4e10, which the client renders as a match duration.
// Nothing downstream sanity-checks it, and no test ever asserted it on a payload
// the manager actually built, so the whole class was invisible.
func TestMatchEndPayload_ReportsASaneDuration(t *testing.T) {
	hub := &hubSpy{}
	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	const roomID = uint(912)
	require.NoError(t, mgr.StartMatch(
		roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, true,
	))
	t.Cleanup(func() { mgr.RemoveSession(roomID) })

	mgr.SetGameStateForTest(roomID, beloteStopPrompt(t, roomID, true))
	before := len(hub.snapshot())
	require.NoError(t, mgr.ApplyActionForTest(roomID, game.Action{
		Type:       game.ActionAnnounceBelot,
		PlayerSeat: 2,
	}))

	var found bool
	for _, e := range wireEvents(t, hub.snapshot()[before:]) {
		if e.kind != ws.EventMatchEnd {
			continue
		}
		var p struct {
			MatchDurationSec int `json:"matchDurationSec"`
		}
		require.NoError(t, json.Unmarshal(e.payload, &p))
		found = true
		// The session started microseconds ago. An hour is a generous ceiling that
		// still catches the year-1 arithmetic by nine orders of magnitude.
		assert.GreaterOrEqual(t, p.MatchDurationSec, 0)
		assert.Less(t, p.MatchDurationSec, 3600,
			"a duration this large means startedAt was never set")
	}
	require.True(t, found, "no event:match_end was broadcast")
}

// The persisted match must record the RULES it was played under. `rooms` already
// stores them, but a room is reusable and mutable: it hosts match after match and
// its settings can change between them, so the room cannot answer what a finished
// match was played under. Without this, a dosta match in history was
// indistinguishable from an ordinary one.
func TestMatchRecord_CarriesTheRulesItWasPlayedUnder(t *testing.T) {
	tests := []struct {
		name                string
		declarationsEnabled bool
		stopAtTarget        bool
	}{
		{"both at their defaults", true, false},
		{"declarations off, dosta on", false, true},
		{"declarations on, dosta on", true, true},
		{"declarations off, dosta off", false, false},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roomID := uint(920 + i)
			hub := &hubSpy{}
			repo := newMockMatchRepo()
			mgr := match.NewManager(hub, repo)
			require.NoError(t, mgr.StartMatch(
				roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0,
				tt.declarationsEnabled, tt.stopAtTarget,
			))
			t.Cleanup(func() { mgr.RemoveSession(roomID) })

			// Any match end will do; a surrender is the cheapest to drive.
			gs := mgr.GetStateSnapshot(roomID)
			gs.Phase = game.PhasePlaying
			seat := 0
			gs.SurrenderProposerSeat = &seat
			mgr.SetGameStateForTest(roomID, gs)
			require.NoError(t, mgr.ApplyActionForTest(roomID, game.Action{
				Type:       game.ActionSurrenderAccept,
				PlayerSeat: 2,
			}))

			matches := repo.getMatches()
			require.Len(t, matches, 1, "the match must be persisted")
			assert.Equal(t, tt.declarationsEnabled, matches[0].DeclarationsEnabled)
			assert.Equal(t, tt.stopAtTarget, matches[0].StopAtTarget)
		})
	}
}
