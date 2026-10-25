package botlab

import "github.com/emilijan/beljot/server/internal/game"

type bidCoef struct {
	count, j, nine, ace, ten, king, queen, eight, seven float64
	j9, jA, nineA                                      float64
	sideA, sideT, sideK                                float64
	round2, pos1, pos2, pos3, konst                    float64
}

var bidCoefBitola = bidCoef{35.4, 54.1, 11.9, -5.5, -0.5, 0.5, 1.1, -12.1, -13.9, 29.6, 12.2, 9.7, 18.9, 6.8, 0.4, -3.7, 1.3, 1.1, -1.5, -136.6}
var bidCoefCroatia = bidCoef{31.5, 43.5, 9.5, -17.1, 0.2, 3.5, 2.6, -5.0, -5.6, 38.6, 24.5, 13.4, 6.7, 4.8, 1.2, 0, 31.1, -8.0, 8.6, -135.2}

// bidValue is the simulation-calibrated expected gain of calling suit with this hand.
func bidValue(c bidCoef, hand []game.Card, suit game.Suit, round, pos int) float64 {
	has := map[game.Rank]float64{}
	count := 0.0
	var sideA, sideT, sideK float64
	for _, card := range hand {
		if card.Suit == suit {
			count++
			has[card.Rank] = 1
			continue
		}
		switch card.Rank {
		case game.RankAce:
			sideA++
		case game.RankTen:
			sideT++
		case game.RankKing:
			sideK++
		}
	}
	j, n, a := has[game.RankJack], has[game.Rank9], has[game.RankAce]
	v := c.konst + c.count*count + c.j*j + c.nine*n + c.ace*a + c.ten*has[game.RankTen] + c.king*has[game.RankKing] +
		c.queen*has[game.RankQueen] + c.eight*has[game.Rank8] + c.seven*has[game.Rank7] +
		c.j9*j*n + c.jA*j*a + c.nineA*n*a + c.sideA*sideA + c.sideT*sideT + c.sideK*sideK
	if round == 2 {
		v += c.round2
	}
	switch pos {
	case 1:
		v += c.pos1
	case 2:
		v += c.pos2
	case 3:
		v += c.pos3
	}
	return v
}

const bidThreshold = 0.0

// decideBidEV calls when the calibrated expected gain of the best allowed suit is positive.
func decideBidEV(v View) game.Action {
	coef := bidCoefBitola
	if v.TrumpCandidate == nil {
		coef = bidCoefCroatia
	}
	hand := bidHand(v)
	pos := (v.Seat - v.DealerSeat - 1 + 4) % 4
	if v.BiddingRound == 1 && v.TrumpCandidate != nil {
		if bidValue(coef, hand, v.TrumpCandidate.Suit, 1, pos) > bidThreshold {
			return game.Action{Type: game.ActionPickTrump, PlayerSeat: v.Seat}
		}
		return game.Action{Type: game.ActionPassTrump, PlayerSeat: v.Seat}
	}
	var best *game.Suit
	bestVal := -1e9
	for _, suit := range game.AllSuits {
		if v.TrumpCandidate != nil && suit == v.TrumpCandidate.Suit {
			continue
		}
		val := bidValue(coef, hand, suit, v.BiddingRound, pos)
		if val > bestVal {
			s := suit
			best = &s
			bestVal = val
		}
	}
	if best != nil && (bestVal > bidThreshold || v.MustPickTrump) {
		return game.Action{Type: game.ActionPickTrump, PlayerSeat: v.Seat, Suit: best}
	}
	return game.Action{Type: game.ActionPassTrump, PlayerSeat: v.Seat}
}

// matchesBidTable reports whether the hand matches a row of the proposal's hand-shape table for
// the suit; croatian selects the per-variant rows and pos is the number of passes before the seat.
func matchesBidTable(hand []game.Card, suit game.Suit, croatian bool, pos int) bool {
	count, sideA := 0, 0
	var hasJ, has9, hasA bool
	for _, c := range hand {
		if c.Suit == suit {
			count++
			switch c.Rank {
			case game.RankJack:
				hasJ = true
			case game.Rank9:
				has9 = true
			case game.RankAce:
				hasA = true
			}
			continue
		}
		if c.Rank == game.RankAce {
			sideA++
		}
	}
	switch {
	case hasJ && has9 && count >= 2:
		return true
	case hasJ && count >= 3:
		return true
	case count >= 5:
		return true
	case count >= 4 && (hasJ || has9):
		return true
	case count >= 4 && hasA:
		return true
	case count >= 4 && sideA >= 1:
		return true
	}
	if croatian {
		return pos == 1 && hasJ && count == 2 && sideA >= 1
	}
	switch {
	case has9 && hasA && count == 3 && sideA >= 1:
		return true
	case hasJ && count == 2 && sideA >= 2:
		return true
	case hasJ && hasA && count == 2 && sideA >= 1:
		return true
	case has9 && !hasJ && !hasA && count == 3 && sideA >= 1:
		return true
	case hasJ && count == 2 && sideA >= 1 && pos == 2:
		return true
	}
	return false
}

// decideBidTable calls when the best allowed suit matches a table row, ranking suits by
// trumpSuitScore; a forced pick takes the best-scoring suit.
func decideBidTable(v View) game.Action {
	croatian := v.TrumpCandidate == nil
	hand := bidHand(v)
	pos := (v.Seat - v.DealerSeat - 1 + 4) % 4
	if v.BiddingRound == 1 && v.TrumpCandidate != nil {
		if matchesBidTable(hand, v.TrumpCandidate.Suit, croatian, pos) {
			return game.Action{Type: game.ActionPickTrump, PlayerSeat: v.Seat}
		}
		return game.Action{Type: game.ActionPassTrump, PlayerSeat: v.Seat}
	}
	var best, forced *game.Suit
	bestScore, forcedScore := -1, -1
	for _, suit := range game.AllSuits {
		if v.TrumpCandidate != nil && suit == v.TrumpCandidate.Suit {
			continue
		}
		score := trumpSuitScore(hand, suit)
		if score > forcedScore {
			s := suit
			forced = &s
			forcedScore = score
		}
		if matchesBidTable(hand, suit, croatian, pos) && score > bestScore {
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
