package match

import (
	"time"

	"github.com/emilijan/beljot/server/internal/game"
)

// Deal grace. Every animated deal (a new hand, an all-pass reshuffle, match
// start, the candidate variant's second deal once trump is taken) keeps the
// table busy on the client for the length of its animation: the first bidder's
// prompt, or the leader's playable hand, only appears once the last packet has
// landed. The server never pauses its broadcasts for that. Instead the NEXT
// actor's deadline, and a bot actor's think delay, are pushed back by exactly
// the animation's length, so the first ring starts full and a bot does not act
// under the dealer's hands.
//
// Keep each value equal to its client counterpart in
// client/src/shared/lib/motion.ts (the DEAL_DURATION_* constants and
// GAME_STARTING_SPLASH); the client's deal schedule test pins its side.
const (
	// dealGraceCandidateFirst: 3 then 2 cards to each seat plus the face-up
	// candidate flip (MOTION.DEAL_DURATION_CANDIDATE_FIRST).
	dealGraceCandidateFirst = 2100 * time.Millisecond
	// dealGraceAllBeforeBidding: 3, 3, then 2 face-down cards to each seat
	// (MOTION.DEAL_DURATION_ALL_BEFORE_BIDDING).
	dealGraceAllBeforeBidding = 2260 * time.Millisecond
	// dealGraceCandidateSecond: the post-pick 3/3/3/(2 + candidate) round
	// (MOTION.DEAL_DURATION_CANDIDATE_SECOND).
	dealGraceCandidateSecond = 740 * time.Millisecond
	// matchStartSplash: the "match is starting" hold the client shows before
	// the table (and so the first deal) is on screen at all
	// (MOTION.GAME_STARTING_SPLASH).
	matchStartSplash = 1500 * time.Millisecond
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
// oldState → newState transition performed, or 0 when it dealt nothing.
//
//   - A new hand or a reshuffle: bidding opens under a different deal
//     (hand number or dealer changed).
//   - The candidate variant's second deal: bidding resolved into play and the
//     face-up candidate (with the 11-card reserve) went out to the seats.
//
// A transition that ends the match (an instant win on the fresh hands) has no
// next actor and gets nothing.
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

// setDealGraceLocked records the grace the transition just earned. It applies
// to the next actor only: every later transition overwrites it (with 0 unless
// it dealt again), so the second bidder never inherits the first one's deal.
// Must be called under session.mu.Lock(), before setTurnExpiry/startTimerLocked
// for the new state.
func (m *Manager) setDealGraceLocked(session *LiveMatch, grace time.Duration) {
	if !m.dealGraceEnabled || grace <= 0 {
		session.dealGraceUntil = time.Time{}
		return
	}
	session.dealGraceUntil = time.Now().Add(grace)
}

// dealGraceRemaining is how much of the current deal animation is still
// running. Zero once it has played out or when no deal is in progress. Must be
// called under session.mu.
func (s *LiveMatch) dealGraceRemaining() time.Duration {
	if s.dealGraceUntil.IsZero() {
		return 0
	}
	return max(time.Until(s.dealGraceUntil), 0)
}
