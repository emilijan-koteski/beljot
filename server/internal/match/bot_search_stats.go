package match

import (
	"log/slog"
	"sync/atomic"
)

const lastOpeningTrick = 2

// botSearchStats counts one match's bot searches and those that reached no decision by their deadline, atomically
// because a miss is counted on a pool worker. A miss landing after the summary has fired, as when the match is
// abandoned mid-search, is not counted.
type botSearchStats struct {
	submitted     atomic.Int64
	missed        atomic.Int64
	missedOpening atomic.Int64
	reported      atomic.Bool
}

// noteSubmitted counts a search the pool accepted; a job the full queue dropped is not counted.
func (s *botSearchStats) noteSubmitted() {
	s.submitted.Add(1)
}

// noteMissed counts a search that reached no decision, and an opening miss when it was for trick 1 or 2.
func (s *botSearchStats) noteMissed(trick int) {
	s.missed.Add(1)
	if trick <= lastOpeningTrick {
		s.missedOpening.Add(1)
	}
}

// logBotSearchSummary logs the match's search counts once; a match that never submitted a search logs nothing.
func (s *LiveMatch) logBotSearchSummary(matchID uint) {
	stats := &s.botSearchStats
	searches := stats.submitted.Load()
	if searches == 0 || !stats.reported.CompareAndSwap(false, true) {
		return
	}
	slog.Info("bot: search summary", "roomID", s.roomID, "matchID", matchID, "searches", searches,
		"missed", stats.missed.Load(), "missedOpening", stats.missedOpening.Load())
}
