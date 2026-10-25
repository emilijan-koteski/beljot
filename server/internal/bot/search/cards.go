package search

import (
	"math/bits"

	"github.com/emilijan/beljot/server/internal/game"
)

const NCards = 32

const ranksPerSuit = 8

var suitList = [4]game.Suit{game.SuitSpades, game.SuitHearts, game.SuitDiamonds, game.SuitClubs}

var (
	trumpStrength = rankTable(game.TrumpRankOrder)
	plainStrength = rankTable(game.NonTrumpRankOrder)
	trumpPoints   = rankTable(game.TrumpCardPoints)
	plainPoints   = rankTable(game.NonTrumpCardPoints)
)

// rankTable lays a per-rank engine table out by natural rank order, so a card index reads it directly.
func rankTable(byRank map[game.Rank]int) [ranksPerSuit]int {
	var table [ranksPerSuit]int
	for rank, natural := range game.NaturalRankOrder {
		table[natural] = byRank[rank]
	}
	return table
}

func SuitIdx(s game.Suit) int {
	switch s {
	case game.SuitSpades:
		return 0
	case game.SuitHearts:
		return 1
	case game.SuitDiamonds:
		return 2
	case game.SuitClubs:
		return 3
	}
	return -1
}

func CardIdx(c game.Card) int {
	return SuitIdx(c.Suit)*ranksPerSuit + game.NaturalRankOrder[c.Rank]
}

func IdxCard(i int) game.Card {
	return game.Card{Suit: suitList[i/ranksPerSuit], Rank: game.NaturalRankSequence[i%ranksPerSuit]}
}

func SuitMask(suit int) uint32 {
	return uint32(0xFF) << (ranksPerSuit * uint(suit))
}

func MaskOf(cards []game.Card) uint32 {
	var m uint32
	for _, c := range cards {
		m |= 1 << uint(CardIdx(c))
	}
	return m
}

func CardsOf(m uint32) []game.Card {
	var out []game.Card
	for ; m != 0; m &= m - 1 {
		out = append(out, IdxCard(bits.TrailingZeros32(m)))
	}
	return out
}

// Points is the card-point value of the card at idx when trump is the trump suit index.
func Points(idx, trump int) int {
	if idx/ranksPerSuit == trump {
		return trumpPoints[idx%ranksPerSuit]
	}
	return plainPoints[idx%ranksPerSuit]
}
