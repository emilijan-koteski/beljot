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
	t.Run("bitola", func(t *testing.T) { boundsMatchTheTrueHands(t, game.VariantBitola, randomDriver) })
	t.Run("croatia", func(t *testing.T) { boundsMatchTheTrueHands(t, game.VariantCroatia, randomCroatianPlayDriver) })
}

func boundsMatchTheTrueHands(t *testing.T, variant game.Variant, drive simDriver) {
	rng := rand.New(rand.NewPCG(8, 41))
	boundsSeen := 0
	for range 300 {
		gs := game.NewGame([4]uint{}, [4]string{}, [4]bool{true, true, true, true}, variant, "1001", 1, true, false, true, false)
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
		_, _ = playOneHandWith(t, gs, mem, rng, drive, check)
	}
	assert.Positive(t, boundsSeen, "no bound was ever checked against a held card")
}

func randomDriver(gs *game.GameState, _ *bot.Memory, rng *rand.Rand) (int, game.Action) {
	seat := actingSeat(gs)
	return seat, randomLegalAction(gs, seat, rng)
}

func randomCroatianPlayDriver(gs *game.GameState, mem *bot.Memory, rng *rand.Rand) (int, game.Action) {
	if gs.Phase != game.PhasePlaying {
		return croatianBotDriver(gs, mem, rng)
	}
	return randomDriver(gs, mem, rng)
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
		cand := card("QS")
		for i := 0; i < 4; i++ {
			mem.ObserveBid(i, bidPass(i), &cand, 1, i)
		}
		mem.ObserveBid(0, bidPick(0, &hearts), &cand, 2, 0)
		bids := mem.Bids()
		require.Len(t, bids, 5)
		for i := 0; i < 4; i++ {
			assert.Equal(t, bot.BidRecord{Seat: i, Round: 1, Passed: true, PassesBefore: i, Candidate: &cand}, bids[i])
		}
		require.NotNil(t, bids[4].Candidate)
		assert.Equal(t, cand, *bids[4].Candidate)
		assert.False(t, bids[4].Passed)
		assert.Equal(t, 2, bids[4].Round)
		assert.Equal(t, 0, bids[4].PassesBefore)
		require.NotNil(t, bids[4].Suit)
		assert.Equal(t, hearts, *bids[4].Suit)
	})

	t.Run("croatian forced dealer pick", func(t *testing.T) {
		mem := bot.NewMemory()
		for i := 0; i < 3; i++ {
			mem.ObserveBid(i+1, bidPass(i+1), nil, 1, i)
		}
		mem.ObserveBid(0, bidPick(0, &spades), nil, 1, 3)
		bids := mem.Bids()
		require.Len(t, bids, 4)
		for i, b := range bids {
			assert.Equal(t, 1, b.Round)
			assert.Equal(t, i, b.PassesBefore)
		}
		for _, b := range bids[:3] {
			assert.True(t, b.Passed)
			assert.Nil(t, b.Suit, "a pass names no suit")
		}
		assert.False(t, bids[3].Passed)
		assert.Equal(t, spades, *bids[3].Suit)
		for _, b := range bids {
			assert.Nil(t, b.Candidate)
		}
	})

	t.Run("round one pick takes the candidate suit", func(t *testing.T) {
		mem := bot.NewMemory()
		cand := card("QD")
		mem.ObserveBid(1, bidPick(1, nil), &cand, 1, 0)
		cand = card("7C")
		bids := mem.Bids()
		require.Len(t, bids, 1)
		require.NotNil(t, bids[0].Candidate)
		assert.Equal(t, card("QD"), *bids[0].Candidate, "the record keeps its own copy of the candidate")
		require.NotNil(t, bids[0].Suit)
		assert.Equal(t, game.SuitDiamonds, *bids[0].Suit)
		assert.False(t, bids[0].Passed)
	})

	t.Run("a new auction replaces a passed-out one", func(t *testing.T) {
		mem := bot.NewMemory()
		for i := 0; i < 4; i++ {
			mem.ObserveBid(i, bidPass(i), nil, 1, i)
		}
		for i := 0; i < 4; i++ {
			mem.ObserveBid(i, bidPass(i), nil, 2, i)
		}
		mem.ObserveBid(1, bidPass(1), nil, 1, 0)
		bids := mem.Bids()
		require.Len(t, bids, 1)
		assert.Equal(t, 1, bids[0].Seat)
	})

	t.Run("sync hand clears the record", func(t *testing.T) {
		mem := bot.NewMemory()
		mem.ObserveBid(0, bidPass(0), nil, 1, 0)
		mem.SyncHand(1)
		assert.Len(t, mem.Bids(), 1)
		mem.SyncHand(2)
		assert.Empty(t, mem.Bids())
	})

	t.Run("bids returns a copy", func(t *testing.T) {
		mem := bot.NewMemory()
		assert.Empty(t, mem.Bids())
		mem.ObserveBid(0, bidPass(0), nil, 1, 0)
		got := mem.Bids()
		got[0].Seat = 3
		assert.Equal(t, 0, mem.Bids()[0].Seat)

		cand := card("QD")
		mem.ObserveBid(1, bidPick(1, nil), &cand, 1, 1)
		got = mem.Bids()
		*got[1].Suit = game.SuitClubs
		*got[1].Candidate = card("7C")
		assert.Equal(t, game.SuitDiamonds, *mem.Bids()[1].Suit, "the suit is copied too")
		assert.Equal(t, card("QD"), *mem.Bids()[1].Candidate, "the candidate is copied too")
	})
}

func TestMemoryRecordsThePlaysPerSeat(t *testing.T) {
	spades, hearts := game.SuitSpades, game.SuitHearts
	mem := bot.NewMemory()
	mem.ObservePlay(0, card("KS"), nil, &hearts, nil)
	mem.ObservePlay(1, card("8S"), &spades, &hearts, trickOf(0, "KS"))
	mem.ObservePlay(0, card("7D"), nil, &hearts, nil)

	played := mem.PlayedBy()
	assert.Equal(t, cards("KS", "7D"), played[0])
	assert.Equal(t, cards("8S"), played[1])
	assert.Empty(t, played[2])

	played[0][0] = card("AH")
	assert.Equal(t, card("KS"), mem.PlayedBy()[0][0], "PlayedBy returns copies")

	mem.SyncHand(1)
	assert.Len(t, mem.PlayedBy()[0], 2)
	mem.SyncHand(2)
	assert.Equal(t, [4][]game.Card{}, mem.PlayedBy())
}
