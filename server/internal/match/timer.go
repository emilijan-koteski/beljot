package match

// cancelTurnTimer stops the turn timer and bumps timerGeneration so blocked callbacks
// fail their staleness check. Must be called under session.mu.Lock().
func (s *LiveMatch) cancelTurnTimer() {
	s.timerGeneration++
	if s.turnTimer != nil {
		s.turnTimer.Stop()
		s.turnTimer = nil
	}
}

// cancelSeatReconnectTimer stops one seat's reconnect timer and bumps its generation
// so blocked callbacks fail their staleness check. Must be called under session.mu.Lock().
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

// cancelAllReconnectTimers stops every per-seat reconnect timer.
// Must be called under session.mu.Lock().
func (s *LiveMatch) cancelAllReconnectTimers() {
	for seat := 0; seat < 4; seat++ {
		s.cancelSeatReconnectTimer(seat)
	}
}

// cancelAllBotActionTimers stops every bot think-delay timer and bumps each seat's
// generation. Must be called under session.mu.Lock().
func (s *LiveMatch) cancelAllBotActionTimers() {
	for seat := 0; seat < 4; seat++ {
		s.botActionGenerations[seat]++
		if s.botActionTimers[seat] != nil {
			s.botActionTimers[seat].Stop()
			s.botActionTimers[seat] = nil
		}
	}
}
