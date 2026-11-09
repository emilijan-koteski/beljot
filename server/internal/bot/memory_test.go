package bot_test

import (
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/bot"
	"github.com/emilijan/beljot/server/internal/game"
)

func trickOf(seatCards ...any) []game.TrickCard {
	var trick []game.TrickCard
	for i := 0; i < len(seatCards); i += 2 {
		trick = append(trick, game.TrickCard{PlayerSeat: seatCards[i].(int), Card: card(seatCards[i+1].(string))})
	}
	return trick
}

func TestMemoryRecordsOverplayBound(t *testing.T) {
	spades, hearts := game.SuitSpades, game.SuitHearts
	mem := bot.NewMemory()

	mem.ObservePlay(1, card("8S"), &spades, &hearts, trickOf(0, "KS"))
	assert.Equal(t, game.NonTrumpRankOrder[game.RankKing], mem.RankBounds()[1][bot.SuitIndex(spades)])
	assert.Equal(t, -1, mem.RankBounds()[1][bot.SuitIndex(hearts)])
	assert.Equal(t, -1, mem.RankBounds()[2][bot.SuitIndex(spades)])

	mem.ObservePlay(1, card("7S"), &spades, &hearts, trickOf(0, "QS"))
	assert.Equal(t, game.NonTrumpRankOrder[game.RankQueen], mem.RankBounds()[1][bot.SuitIndex(spades)])

	mem.ObservePlay(1, card("8S"), &spades, &hearts, trickOf(0, "KS"))
	assert.Equal(t, game.NonTrumpRankOrder[game.RankQueen], mem.RankBounds()[1][bot.SuitIndex(spades)], "a bound only lowers")
}

func TestMemoryRecordsOverTrumpBound(t *testing.T) {
	clubs, hearts := game.SuitClubs, game.SuitHearts
	mem := bot.NewMemory()

	mem.ObservePlay(2, card("8H"), &clubs, &hearts, trickOf(0, "7C", 1, "JH"))

	assert.Equal(t, game.TrumpRankOrder[game.RankJack], mem.RankBounds()[2][bot.SuitIndex(hearts)])
	assert.Equal(t, -1, mem.RankBounds()[2][bot.SuitIndex(clubs)])
}

func TestMemoryIgnoresLowFollowAfterATrumpCut(t *testing.T) {
	spades, hearts := game.SuitSpades, game.SuitHearts
	mem := bot.NewMemory()

	mem.ObservePlay(2, card("8S"), &spades, &hearts, trickOf(0, "KS", 1, "7H"))

	assert.Equal(t, [4][4]int{{-1, -1, -1, -1}, {-1, -1, -1, -1}, {-1, -1, -1, -1}, {-1, -1, -1, -1}}, mem.RankBounds())
}

func TestMemoryResetsBoundsWithTheHand(t *testing.T) {
	spades, hearts := game.SuitSpades, game.SuitHearts
	mem := bot.NewMemory()
	mem.ObservePlay(1, card("8S"), &spades, &hearts, trickOf(0, "KS"))

	mem.SyncHand(2)

	assert.Equal(t, -1, mem.RankBounds()[1][bot.SuitIndex(spades)])
}

func TestMemoryBoundsMatchTheTrueHands(t *testing.T) {
	rng := rand.New(rand.NewPCG(8, 41))
	boundsSeen := 0
	for range 300 {
		gs := newBitolaGame()
		mem := bot.NewMemory()
		check := func(t *testing.T, cur *game.GameState, _ int, _ game.Action) {
			if cur.TrumpSuit == nil {
				return
			}
			bounds := mem.RankBounds()
			for seat := range 4 {
				for _, c := range cur.Players[seat].Hand {
					bound := bounds[seat][bot.SuitIndex(c.Suit)]
					if bound < 0 {
						continue
					}
					boundsSeen++
					strength := game.NonTrumpRankOrder[c.Rank]
					if c.Suit == *cur.TrumpSuit {
						strength = game.TrumpRankOrder[c.Rank]
					}
					require.LessOrEqual(t, strength, bound, "seat %d holds %v above its bound", seat, c)
				}
			}
		}
		_, _ = playOneHandWith(t, gs, mem, rng, randomDriver, check)
	}
	assert.Positive(t, boundsSeen, "no bound was ever checked against a held card")
}

func randomDriver(gs *game.GameState, _ *bot.Memory, rng *rand.Rand) (int, game.Action) {
	seat := actingSeat(gs)
	return seat, randomLegalAction(gs, seat, rng)
}

func bidPass(seat int) game.Action {
	return game.Action{Type: game.ActionPassTrump, PlayerSeat: seat}
}

func bidPick(seat int, suit *game.Suit) game.Action {
	return game.Action{Type: game.ActionPickTrump, PlayerSeat: seat, Suit: suit}
}

func TestMemoryRecordsTheAuction(t *testing.T) {
	hearts, spades := game.SuitHearts, game.SuitSpades

	t.Run("bitola two rounds", func(t *testing.T) {
		mem := bot.NewMemory()
		for i := 0; i < 4; i++ {
			mem.ObserveBid(i, bidPass(i), 1, i)
		}
		mem.ObserveBid(0, bidPick(0, &hearts), 2, 0)
		bids := mem.Bids()
		require.Len(t, bids, 5)
		for i := 0; i < 4; i++ {
			assert.Equal(t, bot.BidRecord{Seat: i, Round: 1, Passed: true, PassesBefore: i}, bids[i])
		}
		assert.False(t, bids[4].Passed)
		assert.Equal(t, 2, bids[4].Round)
		assert.Equal(t, 0, bids[4].PassesBefore)
		require.NotNil(t, bids[4].Suit)
		assert.Equal(t, hearts, *bids[4].Suit)
	})

	t.Run("croatian forced dealer pick", func(t *testing.T) {
		mem := bot.NewMemory()
		for i := 0; i < 3; i++ {
			mem.ObserveBid(i+1, bidPass(i+1), 1, i)
		}
		mem.ObserveBid(0, bidPick(0, &spades), 1, 3)
		bids := mem.Bids()
		require.Len(t, bids, 4)
		for i, b := range bids {
			assert.Equal(t, 1, b.Round)
			assert.Equal(t, i, b.PassesBefore)
		}
		assert.True(t, bids[2].Passed)
		assert.False(t, bids[3].Passed)
		assert.Equal(t, spades, *bids[3].Suit)
	})

	t.Run("round one pick takes the candidate suit", func(t *testing.T) {
		mem := bot.NewMemory()
		cand := card("QD")
		mem.ObserveBid(1, bot.BidActionWithCandidate(bidPick(1, nil), &cand), 1, 0)
		bids := mem.Bids()
		require.Len(t, bids, 1)
		require.NotNil(t, bids[0].Suit)
		assert.Equal(t, game.SuitDiamonds, *bids[0].Suit)
		assert.False(t, bids[0].Passed)
	})

	t.Run("a new auction replaces a passed-out one", func(t *testing.T) {
		mem := bot.NewMemory()
		for i := 0; i < 4; i++ {
			mem.ObserveBid(i, bidPass(i), 1, i)
		}
		for i := 0; i < 4; i++ {
			mem.ObserveBid(i, bidPass(i), 2, i)
		}
		mem.ObserveBid(1, bidPass(1), 1, 0)
		bids := mem.Bids()
		require.Len(t, bids, 1)
		assert.Equal(t, 1, bids[0].Seat)
	})

	t.Run("sync hand clears the record", func(t *testing.T) {
		mem := bot.NewMemory()
		mem.ObserveBid(0, bidPass(0), 1, 0)
		mem.SyncHand(1)
		assert.Len(t, mem.Bids(), 1)
		mem.SyncHand(2)
		assert.Empty(t, mem.Bids())
	})

	t.Run("bids returns a copy", func(t *testing.T) {
		mem := bot.NewMemory()
		assert.Empty(t, mem.Bids())
		mem.ObserveBid(0, bidPass(0), 1, 0)
		got := mem.Bids()
		got[0].Seat = 3
		assert.Equal(t, 0, mem.Bids()[0].Seat)
	})
}
