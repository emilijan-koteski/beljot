package game

import (
	"fmt"
	"sort"
)

var suitOrder = map[Suit]int{
	SuitSpades:   0,
	SuitHearts:   1,
	SuitDiamonds: 2,
	SuitClubs:    3,
}

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

// AutoPickTrumpSuit names the longest pickable suit in seat's visible hand,
// ties broken by suitOrder, for a dealer forced to pick on an absent player's behalf.
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
		if counts[suit] > bestCount {
			best, bestCount = suit, counts[suit]
		}
	}
	if bestCount == 0 {
		return "", fmt.Errorf("auto-pick trump: seat %d holds no card in a pickable suit", seat)
	}
	return best, nil
}
