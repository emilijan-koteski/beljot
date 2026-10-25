package match_test

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/bot"
	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
	"github.com/emilijan/beljot/server/internal/match"
)

const (
	searchRoomID  = uint(100)
	searchBotSeat = 2
)

func cardsOutOfPlay(gs *game.GameState) []game.Card {
	held := map[game.Card]bool{}
	for _, p := range gs.Players {
		for _, c := range p.Hand {
			held[c] = true
		}
	}
	for _, tc := range gs.CurrentTrick {
		held[tc.Card] = true
	}
	var out []game.Card
	for _, c := range game.NewDeck() {
		if !held[c] {
			out = append(out, c)
		}
	}
	return out
}

// newSearchMatch starts a Bitola match whose bot leads trick 5 and returns the bot's view of it, with no think delay
// armed yet so the test decides when scheduling happens.
func newSearchMatch(t *testing.T, settings match.BotSearchSettings, thinkDelay time.Duration) (*match.Manager, bot.View) {
	t.Helper()
	mgr := match.NewManager(&hubSpy{}, newMockMatchRepo())
	mgr.SetDealGraceForTest(false)
	mgr.SetBotSearchForTest(settings)
	mgr.SetBotDelayForTest(thinkDelay, thinkDelay)
	require.NoError(t, mgr.StartMatch(searchRoomID, "bitola", "1001", mixedPlayers(searchBotSeat), "relaxed", 0, 10, 120, 0, true, false, true, false))
	t.Cleanup(func() { mgr.RemoveSession(searchRoomID) })

	gs := markBots(testfixtures.NewGameMidPlay(5), searchBotSeat)
	gs.ActivePlayerSeat = searchBotSeat
	mgr.SetGameStateForTest(searchRoomID, gs)
	mgr.CancelBotTimersForTest(searchRoomID)
	mgr.RecordBotPlaysForTest(searchRoomID, cardsOutOfPlay(gs))

	view := mgr.BuildBotViewForTest(searchRoomID, searchBotSeat)
	require.Equal(t, game.PhasePlaying, view.Phase)
	require.Equal(t, searchBotSeat, view.ActivePlayerSeat)
	require.Greater(t, len(view.LegalCards), 1)
	return mgr, view
}

func rulesCard(view bot.View) game.Card {
	return *bot.Decide(view).Card
}

func searchedCard(t *testing.T, view bot.View) bot.SearchResult {
	t.Helper()
	res, ok := bot.SearchCard(view, time.Now().Add(10*time.Second))
	require.True(t, ok, "the fixture must be searchable")
	require.NotEqual(t, rulesCard(view), res.Card, "the fixture must tell the search from the rules")
	return res
}

func awaitBotPlay(t *testing.T, mgr *match.Manager) game.Card {
	t.Helper()
	var played game.Card
	require.True(t, waitFor(10*time.Second, func() bool {
		st := mgr.GetStateSnapshot(searchRoomID)
		if st == nil || len(st.CurrentTrick) == 0 {
			return false
		}
		played = st.CurrentTrick[0].Card
		return true
	}), "the bot must play")
	return played
}

func TestBotSearchResultIsUsedWhenReady(t *testing.T) {
	mgr, view := newSearchMatch(t, match.BotSearchSettings{Workers: 1, Deadline: 10 * time.Second, FromTrick: 1}, time.Second)
	want := searchedCard(t, view)

	mgr.BotSchedule(searchRoomID)

	require.True(t, waitFor(5*time.Second, func() bool { return mgr.BotSearchResultForTest(searchRoomID, searchBotSeat) != nil }),
		"the search must store its result before the think delay ends")
	assert.Equal(t, want, *mgr.BotSearchResultForTest(searchRoomID, searchBotSeat))

	assert.Equal(t, want.Card, awaitBotPlay(t, mgr))
	assert.Nil(t, mgr.BotSearchResultForTest(searchRoomID, searchBotSeat), "acting consumes the result")
}

func TestBotSearchIgnoresAStaleResult(t *testing.T) {
	mgr, view := newSearchMatch(t, match.BotSearchSettings{FromTrick: 0}, 300*time.Millisecond)
	want := rulesCard(view)
	other := view.LegalCards[0]
	if other == want {
		other = view.LegalCards[1]
	}

	mgr.BotSchedule(searchRoomID)
	mgr.StoreBotSearchResultForTest(searchRoomID, searchBotSeat, 1<<40, bot.SearchResult{Card: other})

	assert.Equal(t, want, awaitBotPlay(t, mgr))
	assert.Nil(t, mgr.BotSearchResultForTest(searchRoomID, searchBotSeat), "a stale result is cleared when the timer fires")
}

func TestBotSearchDisabledPlaysTheRules(t *testing.T) {
	mgr, view := newSearchMatch(t, match.BotSearchSettings{FromTrick: 0}, 300*time.Millisecond)
	searchedCard(t, view)

	mgr.BotSchedule(searchRoomID)
	time.Sleep(100 * time.Millisecond)

	assert.Nil(t, mgr.BotSearchResultForTest(searchRoomID, searchBotSeat), "no search runs when it is disabled")
	assert.Equal(t, rulesCard(view), awaitBotPlay(t, mgr))
}

func TestBotSearchTimeoutFallsBackToRules(t *testing.T) {
	logs := captureLogs(t)
	mgr, view := newSearchMatch(t, match.BotSearchSettings{Workers: 1, Deadline: time.Nanosecond, FromTrick: 1}, 300*time.Millisecond)
	searchedCard(t, view)

	mgr.BotSchedule(searchRoomID)

	require.True(t, waitFor(5*time.Second, func() bool {
		return len(logs.matched(searchRoomID, "bot: search timed out; rules played")) > 0
	}), "the timed-out search must be logged")
	assert.Nil(t, mgr.BotSearchResultForTest(searchRoomID, searchBotSeat), "a timed-out search stores nothing")

	assert.Equal(t, rulesCard(view), awaitBotPlay(t, mgr))
	st := mgr.GetStateSnapshot(searchRoomID)
	require.NotNil(t, st)
	assert.Equal(t, game.PhasePlaying, st.Phase, "the match continues")
	assert.False(t, slices.Contains(st.Players[searchBotSeat].Hand, rulesCard(view)), "the card left the bot's hand")
}

func TestDefaultBotSearchSettings(t *testing.T) {
	s := match.DefaultBotSearchSettings()
	assert.GreaterOrEqual(t, s.Workers, 1)
	assert.LessOrEqual(t, s.Workers, 4)
	assert.Equal(t, 500*time.Millisecond, s.Deadline)
	assert.Equal(t, 3, s.FromTrick)
}
