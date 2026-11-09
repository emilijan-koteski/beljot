package match

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/bot"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
)

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) linesWith(sub string) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []string
	for _, line := range strings.Split(b.buf.String(), "\n") {
		if strings.Contains(line, sub) {
			out = append(out, line)
		}
	}
	return out
}

func captureSlog(t *testing.T) *syncBuffer {
	t.Helper()
	b := &syncBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(b, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return b
}

func TestBotSearchStats_CountSubmittedSearchesAndMisses(t *testing.T) {
	logs := captureSlog(t)
	gs := testfixtures.NewGameMidPlay(5)
	seat := gs.ActivePlayerSeat
	session := &LiveMatch{roomID: 41, gameState: gs}
	m := NewManager(&recordingBroadcaster{}, nil)
	m.botSearch = BotSearchSettings{Workers: 1, Deadline: -time.Hour, FromTrick: 1}
	m.searchPool = newSearchPool(1)

	require.True(t, bot.SearchAppliesWith(buildBotView(gs, seat, nil), 1, m.botTuning), "the fixture must be searchable")
	session.mu.Lock()
	m.submitCardSearch(session, seat, 0, botDecisionContextFor(gs, seat))
	session.mu.Unlock()

	require.Eventually(t, func() bool { return session.botSearchStats.missed.Load() == 1 }, 5*time.Second, time.Millisecond,
		"a search past its deadline must count as a miss")
	assert.Equal(t, int64(1), session.botSearchStats.submitted.Load())
	assert.Equal(t, int64(0), session.botSearchStats.missedOpening.Load(), "trick 5 is not an opening trick")

	runBotSearch(session, bot.View{Seat: 2, TrickNumber: 1}, 0, botDecisionContext{}, time.Now(), m.botTuning)
	runBeloteSearch(session, bot.View{Seat: 3, TrickNumber: 2}, 0, botDecisionContext{}, time.Now(), m.botTuning)
	runBotSearch(session, bot.View{Seat: 1, TrickNumber: 3}, 0, botDecisionContext{}, time.Now(), m.botTuning)

	assert.Equal(t, int64(4), session.botSearchStats.missed.Load())
	assert.Equal(t, int64(2), session.botSearchStats.missedOpening.Load())

	misses := logs.linesWith("reached no decision")
	require.Len(t, misses, 4)
	for _, line := range misses {
		assert.Contains(t, line, "level=DEBUG")
		assert.Contains(t, line, "roomID=41")
		assert.Contains(t, line, "seat=")
		assert.Contains(t, line, "trick=")
	}
}

func TestBotSearchSummary_LogsOnceWithTheCounts(t *testing.T) {
	logs := captureSlog(t)
	session := &LiveMatch{roomID: 42}
	session.botSearchStats.noteSubmitted()
	session.botSearchStats.noteSubmitted()
	session.botSearchStats.noteSubmitted()
	session.botSearchStats.noteMissed(1)
	session.botSearchStats.noteMissed(6)

	session.logBotSearchSummary(9)
	session.logBotSearchSummary(9)

	lines := logs.linesWith("bot: search summary")
	require.Len(t, lines, 1, "the summary is emitted once per match")
	assert.Contains(t, lines[0], "level=INFO")
	assert.Contains(t, lines[0], `msg="bot: search summary" roomID=42 matchID=9 searches=3 missed=2 missedOpening=1`)
}

func TestBotSearchSummary_SilentWithoutSearches(t *testing.T) {
	logs := captureSlog(t)
	session := &LiveMatch{roomID: 43}

	session.logBotSearchSummary(10)

	assert.Empty(t, logs.linesWith("bot: search summary"))
}
