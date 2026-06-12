package match

// cancelTurnTimer stops the current turn timer if active. Safe to call when nil.
// Must be called under session.mu.Lock().
//
// Bumps timerGeneration unconditionally so any goroutine already past
// time.AfterFunc but blocked on session.mu fails the staleness check in
// handleTimerExpiry. This makes "cancel == invalidate stale callbacks" a
// property of this function rather than a discipline imposed on every caller.
func (s *LiveMatch) cancelTurnTimer() {
	s.timerGeneration++
	if s.turnTimer != nil {
		s.turnTimer.Stop()
		s.turnTimer = nil
	}
}

// cancelSeatReconnectTimer stops the reconnect countdown timer for a single
// seat if active and bumps that seat's generation counter so any in-flight
// callback that already passed `time.AfterFunc` but is blocked on session.mu
// fails its staleness check. Safe to call when nil. Must be called under
// session.mu.Lock().
func (s *LiveMatch) cancelSeatReconnectTimer(seat int) {
	if seat < 0 || seat >= 4 {
		return
	}
	s.seatReconnectGenerations[seat]++
	if s.seatReconnectTimers[seat] != nil {
		s.seatReconnectTimers[seat].Stop()
		s.seatReconnectTimers[seat] = nil
	}
}

// cancelAllReconnectTimers stops every per-seat reconnect timer. Used by
// session teardown + match-end paths so no stale callback can fire after the
// session is unreachable. Must be called under session.mu.Lock().
func (s *LiveMatch) cancelAllReconnectTimers() {
	for seat := 0; seat < 4; seat++ {
		s.cancelSeatReconnectTimer(seat)
	}
}

// cancelAllBotActionTimers stops every pending bot think-delay timer and
// bumps each seat's generation so in-flight callbacks blocked on session.mu
// fail their staleness check. Used by session teardown. Must be called under
// session.mu.Lock().
func (s *LiveMatch) cancelAllBotActionTimers() {
	for seat := 0; seat < 4; seat++ {
		s.botActionGenerations[seat]++
		if s.botActionTimers[seat] != nil {
			s.botActionTimers[seat].Stop()
			s.botActionTimers[seat] = nil
		}
	}
}
