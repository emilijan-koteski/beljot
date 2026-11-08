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
		return len(logs.matched(searchRoomID, "bot: search reached no decision; rules played")) > 0
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
	assert.Equal(t, 1, s.FromTrick)
}

func TestBuildBotViewCarriesDeclarationsEnabled(t *testing.T) {
	for _, tt := range []struct {
		name string
		gs   *game.GameState
		want bool
	}{
		{"default rules", testfixtures.NewGameMidPlay(5), true},
		{"declarations off", testfixtures.WithoutDeclarations(testfixtures.NewGameMidPlay(5)), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mgr := match.NewManager(&hubSpy{}, newMockMatchRepo())
			require.NoError(t, mgr.StartMatch(searchRoomID, "bitola", "1001", mixedPlayers(searchBotSeat), "relaxed", 0, 10, 120, 0, true, false, true, false))
			t.Cleanup(func() { mgr.RemoveSession(searchRoomID) })
			mgr.SetGameStateForTest(searchRoomID, markBots(tt.gs, searchBotSeat))
			mgr.CancelBotTimersForTest(searchRoomID)

			assert.Equal(t, tt.want, mgr.BuildBotViewForTest(searchRoomID, searchBotSeat).DeclarationsEnabled)
		})
	}
}

// newBeloteMatch starts a Bitola match in which the bot of the calling team has just led the Queen of trump at trick 5,
// holding the King, with the opponents far ahead; no think delay is armed yet.
func newBeloteMatch(t *testing.T, tuning bot.SearchTuning, settings match.BotSearchSettings, thinkDelay time.Duration) (*match.Manager, bot.View) {
	t.Helper()
	mgr := match.NewManager(&hubSpy{}, newMockMatchRepo())
	mgr.SetDealGraceForTest(false)
	mgr.SetBotSearchForTest(settings)
	mgr.SetBotTuningForTest(tuning)
	mgr.SetBotDelayForTest(thinkDelay, thinkDelay)
	require.NoError(t, mgr.StartMatch(searchRoomID, "bitola", "1001", mixedPlayers(searchBotSeat), "relaxed", 0, 10, 120, 0, true, false, true, false))
	t.Cleanup(func() { mgr.RemoveSession(searchRoomID) })

	gs := markBots(testfixtures.NewGameMidPlay(5), searchBotSeat)
	queen := game.Card{Rank: game.RankQueen, Suit: game.SuitHearts}
	require.True(t, slices.Contains(gs.Players[searchBotSeat].Hand, queen))
	gs.Players[searchBotSeat].Hand = slices.DeleteFunc(slices.Clone(gs.Players[searchBotSeat].Hand), func(c game.Card) bool { return c == queen })
	caller, pending, lead := searchBotSeat, searchBotSeat, game.SuitHearts
	gs.TrumpCallerSeat = &caller
	gs.PendingBelotSeat = &pending
	gs.BelotAnnounced = false
	gs.CurrentTrick = []game.TrickCard{{Card: queen, PlayerSeat: searchBotSeat}}
	gs.LeadSuit = &lead
	gs.ActivePlayerSeat = searchBotSeat
	gs.HandPoints, gs.TricksWon = [2]int{0, 120}, [2]int{0, 4}
	require.True(t, gs.Rules.DeclarationsTransfer)
	mgr.SetGameStateForTest(searchRoomID, gs)
	mgr.CancelBotTimersForTest(searchRoomID)
	mgr.RecordBotPlaysForTest(searchRoomID, cardsOutOfPlay(gs))

	view := mgr.BuildBotViewForTest(searchRoomID, searchBotSeat)
	require.True(t, view.PendingBelot)
	return mgr, view
}

func beloteSkipTuning() bot.SearchTuning {
	t := bot.DefaultSearchTuning()
	t.BeloteSkipMargin = 5
	return t
}

func awaitBeloteAnswer(t *testing.T, mgr *match.Manager) *game.GameState {
	t.Helper()
	var st *game.GameState
	require.True(t, waitFor(10*time.Second, func() bool {
		st = mgr.GetStateSnapshot(searchRoomID)
		return st != nil && st.PendingBelotSeat == nil
	}), "the bot must answer the Belote")
	return st
}

func TestBotSearchAnswersTheCallersBelote(t *testing.T) {
	tuning := beloteSkipTuning()
	mgr, view := newBeloteMatch(t, tuning, match.BotSearchSettings{Workers: 1, Deadline: 10 * time.Second, FromTrick: 1}, time.Second)
	announce, ok := bot.BeloteAnswerWith(view, time.Now().Add(10*time.Second), tuning)
	require.True(t, ok)
	require.False(t, announce, "the fixture must skip")

	mgr.BotSchedule(searchRoomID)

	require.True(t, waitFor(5*time.Second, func() bool { return mgr.BotSearchResultForTest(searchRoomID, searchBotSeat) != nil }),
		"the Belote search must store its answer before the think delay ends")
	assert.Equal(t, bot.SearchResult{BeloteSkip: true}, *mgr.BotSearchResultForTest(searchRoomID, searchBotSeat))

	st := awaitBeloteAnswer(t, mgr)
	assert.False(t, st.BelotAnnounced)
	assert.Zero(t, st.BelotPoints[game.TeamA])
	assert.Nil(t, mgr.BotSearchResultForTest(searchRoomID, searchBotSeat), "acting consumes the answer")
}

func TestBotSearchDropsAStaleBeloteAnswer(t *testing.T) {
	mgr, _ := newBeloteMatch(t, beloteSkipTuning(), match.BotSearchSettings{FromTrick: 0}, 300*time.Millisecond)

	mgr.BotSchedule(searchRoomID)
	mgr.StoreBotSearchResultForTest(searchRoomID, searchBotSeat, 1<<40, bot.SearchResult{BeloteSkip: true})

	st := awaitBeloteAnswer(t, mgr)
	assert.True(t, st.BelotAnnounced, "a stale skip is ignored and the bot announces")
	assert.Equal(t, 20, st.BelotPoints[game.TeamA])
	assert.Nil(t, mgr.BotSearchResultForTest(searchRoomID, searchBotSeat), "a stale answer is cleared when the timer fires")
}

func TestBotBeloteWithoutASkipMarginQueuesNoSearch(t *testing.T) {
	mgr, _ := newBeloteMatch(t, bot.DefaultSearchTuning(), match.BotSearchSettings{Workers: 1, Deadline: 10 * time.Second, FromTrick: 1}, time.Second)
	mgr.SetBotDelayForTest(time.Second, 2*time.Second)
	started := time.Now()

	mgr.BotSchedule(searchRoomID)

	st := awaitBeloteAnswer(t, mgr)
	assert.Less(t, time.Since(started), time.Second, "the Belote beat stays")
	assert.True(t, st.BelotAnnounced)
	assert.Nil(t, mgr.BotSearchResultForTest(searchRoomID, searchBotSeat))
}
