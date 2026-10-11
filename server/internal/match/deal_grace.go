package match

import (
	"time"

	"github.com/emilijan/beljot/server/internal/game"
)

const (
	dealGraceCandidateFirst   = 2100 * time.Millisecond
	dealGraceAllBeforeBidding = 2260 * time.Millisecond
	dealGraceCandidateSecond  = 740 * time.Millisecond
	matchStartSplash          = 1500 * time.Millisecond
)

// firstDealGrace is the animation length of a hand's opening deal under the
// given rules. The deal shape decides it, never the variant name.
func firstDealGrace(rules game.VariantRules) time.Duration {
	if rules.DealShape == game.DealShapeAllBeforeBidding {
		return dealGraceAllBeforeBidding
	}
	return dealGraceCandidateFirst
}

// dealGraceFor reports how long the client animates the deal that the
// oldState → newState transition performed (a new hand, a reshuffle, or the
// candidate variant's second deal), or 0 when it dealt nothing.
func dealGraceFor(oldState, newState *game.GameState) time.Duration {
	switch {
	case newState.Phase == game.PhaseBidding &&
		(oldState.HandNumber != newState.HandNumber || oldState.DealerSeat != newState.DealerSeat):
		return firstDealGrace(newState.Rules)
	case oldState.Phase == game.PhaseBidding && newState.Phase == game.PhasePlaying &&
		oldState.TrumpCandidate != nil && newState.TrumpCandidate == nil:
		return dealGraceCandidateSecond
	default:
		return 0
	}
}

// setDealGraceLocked records the grace the transition just earned, for the next
// actor only. Call under session.mu.Lock(), before setTurnExpiry/startTimerLocked.
func (m *Manager) setDealGraceLocked(session *LiveMatch, grace time.Duration) {
	if !m.dealGraceEnabled || grace <= 0 {
		session.dealGraceUntil = time.Time{}
		return
	}
	session.dealGraceUntil = time.Now().Add(grace)
}

// dealGraceRemaining is how much of the current deal animation is still
// running. Call under session.mu.
func (s *LiveMatch) dealGraceRemaining() time.Duration {
	if s.dealGraceUntil.IsZero() {
		return 0
	}
	return max(time.Until(s.dealGraceUntil), 0)
}
