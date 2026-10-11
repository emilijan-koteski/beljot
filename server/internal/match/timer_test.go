package match

import (
	"testing"
	"time"
)

// cancelTurnTimer must bump timerGeneration on every call so blocked AfterFunc
// callbacks fail the staleness check in handleTimerExpiry.
func TestCancelTurnTimer_BumpsGeneration(t *testing.T) {
	s := &LiveMatch{timerGeneration: 7}

	s.mu.Lock()
	before := s.timerGeneration
	s.cancelTurnTimer()
	after := s.timerGeneration
	s.mu.Unlock()

	if after != before+1 {
		t.Fatalf("cancelTurnTimer must bump timerGeneration by 1; before=%d after=%d", before, after)
	}
}

// The bump also happens when an active timer is being stopped.
func TestCancelTurnTimer_BumpsGeneration_WithActiveTimer(t *testing.T) {
	s := &LiveMatch{timerGeneration: 0}
	s.turnTimer = time.AfterFunc(time.Hour, func() {})

	s.mu.Lock()
	before := s.timerGeneration
	s.cancelTurnTimer()
	after := s.timerGeneration
	tt := s.turnTimer
	s.mu.Unlock()

	if after != before+1 {
		t.Fatalf("cancelTurnTimer must bump generation when stopping an active timer; before=%d after=%d", before, after)
	}
	if tt != nil {
		t.Fatalf("cancelTurnTimer must clear the turnTimer pointer; got %v", tt)
	}
}
