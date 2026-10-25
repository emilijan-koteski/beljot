package bot

import (
	"slices"

	"github.com/emilijan/beljot/server/internal/game"
)

const trumpLengthBonus = 10

type suitProfile struct {
	count    int
	jack     bool
	nine     bool
	ace      bool
	sideAces int
}

func profileSuit(hand []game.Card, suit game.Suit) suitProfile {
	var p suitProfile
	for _, c := range hand {
		if c.Suit != suit {
			if c.Rank == game.RankAce {
				p.sideAces++
			}
			continue
		}
		p.count++
		switch c.Rank {
		case game.RankJack:
			p.jack = true
		case game.Rank9:
			p.nine = true
		case game.RankAce:
			p.ace = true
		}
	}
	return p
}

// matchesBidTable reports whether the bid hand fits one of the twelve simulation-calibrated shapes
// for calling suit. Rows 7 to 11 need a face-up candidate, row 12 needs a candidate-less variant.
func matchesBidTable(hand []game.Card, suit game.Suit, hasCandidate bool, pos int) bool {
	p := profileSuit(hand, suit)
	return p.jack && p.nine ||
		p.jack && p.count >= 3 ||
		p.count >= 4 && p.nine ||
		p.count >= 4 && p.ace ||
		p.count >= 4 && p.sideAces >= 1 ||
		p.count >= 5 ||
		hasCandidate && p.nine && p.ace && p.count == 3 && p.sideAces >= 1 ||
		hasCandidate && p.jack && p.ace && p.count == 2 && p.sideAces >= 1 ||
		hasCandidate && p.jack && p.count == 2 && p.sideAces >= 2 ||
		hasCandidate && p.nine && !p.jack && !p.ace && p.count == 3 && p.sideAces >= 1 ||
		hasCandidate && pos == 2 && p.jack && p.count == 2 && p.sideAces >= 1 ||
		!hasCandidate && pos == 1 && p.jack && p.count == 2 && p.sideAces >= 1
}

// trumpSuitScore ranks a suit among those that match the table: trump-order card points plus a
// per-card length bonus.
func trumpSuitScore(hand []game.Card, suit game.Suit) int {
	score := 0
	for _, c := range hand {
		if c.Suit == suit {
			score += game.TrumpCardPoints[c.Rank] + trumpLengthBonus
		}
	}
	return score
}

// bidHand returns the cards the bot would hold after picking: the visible hand plus any candidate.
// It clones before appending so v.Hand's backing array is never mutated.
func bidHand(v View) []game.Card {
	if v.TrumpCandidate == nil {
		return v.Hand
	}
	return append(slices.Clone(v.Hand), *v.TrumpCandidate)
}

func seatsPassedBefore(v View) int {
	return (v.Seat - v.DealerSeat - 1 + 4) % 4
}

// decideBid picks or passes trump from the hand-shape table, taking the best-scoring suit when
// MustPickTrump forbids a pass.
func decideBid(v View) game.Action {
	hand := bidHand(v)
	pos := seatsPassedBefore(v)

	if v.HasTrumpCandidate && v.BiddingRound == 1 {
		if matchesBidTable(hand, v.TrumpCandidate.Suit, true, pos) {
			return game.Action{Type: game.ActionPickTrump, PlayerSeat: v.Seat}
		}
		return game.Action{Type: game.ActionPassTrump, PlayerSeat: v.Seat}
	}

	var best, forced *game.Suit
	bestScore, forcedScore := -1, -1
	for _, suit := range game.AllSuits {
		if v.HasTrumpCandidate && suit == v.TrumpCandidate.Suit {
			continue
		}
		score := trumpSuitScore(hand, suit)
		if score > forcedScore {
			s := suit
			forced = &s
			forcedScore = score
		}
		if !matchesBidTable(hand, suit, v.HasTrumpCandidate, pos) {
			continue
		}
		if score > bestScore {
			s := suit
			best = &s
			bestScore = score
		}
	}
	if best != nil {
		return game.Action{Type: game.ActionPickTrump, PlayerSeat: v.Seat, Suit: best}
	}
	if v.MustPickTrump && forced != nil {
		return game.Action{Type: game.ActionPickTrump, PlayerSeat: v.Seat, Suit: forced}
	}
	return game.Action{Type: game.ActionPassTrump, PlayerSeat: v.Seat}
}
