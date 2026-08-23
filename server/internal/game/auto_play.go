package game

import (
	"fmt"
	"sort"
)

// suitOrder maps suits to their sort priority for auto-play card selection.
// Order: Spades(0) > Hearts(1) > Diamonds(2) > Clubs(3)
var suitOrder = map[Suit]int{
	SuitSpades:   0,
	SuitHearts:   1,
	SuitDiamonds: 2,
	SuitClubs:    3,
}

// rankOrder maps ranks to their sort priority for auto-play card selection.
// Order: 7(0) > 8(1) > 9(2) > T(3) > J(4) > Q(5) > K(6) > A(7)
var rankOrder = map[Rank]int{
	Rank7:     0,
	Rank8:     1,
	Rank9:     2,
	RankTen:   3,
	RankJack:  4,
	RankQueen: 5,
	RankKing:  6,
	RankAce:   7,
}

// AutoPlay selects the first legal card for the active player, sorted by
// suit (S, H, D, C) then rank (7, 8, 9, T, J, Q, K, A).
// This is a pure function — no side effects.
func AutoPlay(state *GameState) (string, error) {
	seat := state.ActivePlayerSeat
	legal := legalCards(state, seat)
	if len(legal) == 0 {
		return "", fmt.Errorf("auto-play: no legal cards for seat %d", seat)
	}

	sort.Slice(legal, func(i, j int) bool {
		si, sj := suitOrder[legal[i].Suit], suitOrder[legal[j].Suit]
		if si != sj {
			return si < sj
		}
		return rankOrder[legal[i].Rank] < rankOrder[legal[j].Rank]
	})

	return legal[0].String(), nil
}

// AutoPickTrumpSuit names a trump suit for `seat` when the engine will not
// accept a pass — the dealer bidding last (fourth) under AllPassOutcome ==
// AllPassDealerMustPick (see MustPickTrump). Pure and deterministic, exactly
// like AutoPlay: the session manager calls it on an absent player's behalf.
//
// The seat is an argument, not read from state.ActivePlayerSeat, so it cannot
// diverge from the seat the caller stamps on the resulting action — every
// sibling auto-action in handleTimerExpiry is built from expectedSeat.
//
// Policy: the seat's longest suit, ties broken by the package-level suitOrder
// (S, H, D, C) so the answer never depends on hand order or map iteration.
//
// This is deliberately NOT the bot's policy. bot.decideBid's forced branch takes
// the highest trumpSuitScore — card points plus a length bonus — so a bot and an
// absent human in the identical state can name DIFFERENT suits. That is
// accepted: this helper answers "what does a player with no stated preference
// call", which wants the simplest defensible rule, while the bot is playing to
// win. Unifying them changes bot strength, which is a balance decision and not
// this function's to make.
//
// Two details are load-bearing:
//
//   - The scan reads Hand ONLY, never FaceDownCards. The forced dealer is
//     picking mid-bidding, when the face-down pair is still hidden from
//     everyone — its owner included — so the choice must come from the six
//     visible cards, exactly as it would for a present player.
//   - A face-up candidate's own suit is excluded. handlePickTrump rejects it as
//     already spent in round 1, so naming it would trade one rejected
//     auto-action for another. Unreachable under the only config that forces a
//     pick — that variant has no candidate — but the exclusion keeps the
//     function safe for any caller.
func AutoPickTrumpSuit(state *GameState, seat int) (Suit, error) {
	if seat < 0 || seat > 3 {
		return "", fmt.Errorf("auto-pick trump: seat %d out of range", seat)
	}
	player := state.Players[seat]

	counts := make(map[Suit]int, 4)
	for _, c := range player.Hand {
		counts[c.Suit]++
	}

	var best Suit
	bestCount := 0
	for _, suit := range AllSuits {
		if state.TrumpCandidate != nil && suit == state.TrumpCandidate.Suit {
			continue
		}
		// Strictly greater, walking AllSuits in suitOrder sequence, so a tie
		// resolves to the earliest suit in that order.
		if counts[suit] > bestCount {
			best, bestCount = suit, counts[suit]
		}
	}
	if bestCount == 0 {
		return "", fmt.Errorf("auto-pick trump: seat %d holds no card in a pickable suit", seat)
	}
	return best, nil
}
