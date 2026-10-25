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
		910, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true, false,
	))
	t.Cleanup(func() { mgr.RemoveSession(910) })

	mgr.BufferHandResultIfScored(910,
		&game.GameState{HandNumber: 2},
		&game.GameState{HandNumber: 3, LastHandResult: staleResult(2)})
	require.Len(t, mgr.HandResults(910), 1)

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
		911, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, false, true, false,
	))
	t.Cleanup(func() { mgr.RemoveSession(911) })

	mgr.BufferHandResultIfScored(911,
		&game.GameState{HandNumber: 4},
		&game.GameState{HandNumber: 4, Phase: game.PhaseMatchEnd, LastHandResult: staleResult(1)})

	hands := mgr.HandResults(911)
	require.Len(t, hands, 1)
	assert.Equal(t, 1, hands[0].HandNumber)
}

// A zero match-start timestamp would report about 6.4e10 seconds as the match
// duration, so the built payload must carry a sane value.
func TestMatchEndPayload_ReportsASaneDuration(t *testing.T) {
	hub := &hubSpy{}
	repo := newMockMatchRepo()
	mgr := match.NewManager(hub, repo)
	const roomID = uint(912)
	require.NoError(t, mgr.StartMatch(
		roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0, true, true, true, false,
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
		assert.GreaterOrEqual(t, p.MatchDurationSec, 0)
		assert.Less(t, p.MatchDurationSec, 3600,
			"a duration this large means startedAt was never set")
	}
	require.True(t, found, "no event:match_end was broadcast")
}

// The persisted match records the rules it was played under, because a room is
// reused and its settings can change between matches.
func TestMatchRecord_CarriesTheRulesItWasPlayedUnder(t *testing.T) {
	tests := []struct {
		name                 string
		declarationsEnabled  bool
		stopAtTarget         bool
		declarationsTransfer bool
		clockwise            bool
	}{
		{"all at their defaults", true, false, true, false},
		{"declarations off, dosta on, clockwise", false, true, true, true},
		{"declarations on, dosta on, no transfer", true, true, false, false},
		{"declarations off, dosta off, no transfer, clockwise", false, false, false, true},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roomID := uint(920 + i)
			hub := &hubSpy{}
			repo := newMockMatchRepo()
			mgr := match.NewManager(hub, repo)
			require.NoError(t, mgr.StartMatch(
				roomID, "bitola", "1001", defaultPlayers(), "relaxed", 0, 10, 120, 0,
				tt.declarationsEnabled, tt.stopAtTarget, tt.declarationsTransfer, tt.clockwise,
			))
			t.Cleanup(func() { mgr.RemoveSession(roomID) })

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
			assert.Equal(t, tt.declarationsTransfer, matches[0].DeclarationsTransfer)
			assert.Equal(t, tt.clockwise, matches[0].Clockwise)
		})
	}
}
