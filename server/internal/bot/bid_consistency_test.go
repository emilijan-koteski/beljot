package bot_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/bot"
	"github.com/emilijan/beljot/server/internal/bot/search"
	"github.com/emilijan/beljot/server/internal/game"
)

func passRec(seat, round, passesBefore int, candidate *game.Card) bot.BidRecord {
	return bot.BidRecord{Seat: seat, Round: round, Passed: true, PassesBefore: passesBefore, Candidate: candidate}
}

func callRec(seat, round, passesBefore int, suit game.Suit, candidate *game.Card) bot.BidRecord {
	return bot.BidRecord{Seat: seat, Round: round, Suit: &suit, PassesBefore: passesBefore, Candidate: candidate}
}

func croatianAuctionView(bids ...bot.BidRecord) bot.View {
	return bot.View{Seat: 0, Bids: bids}
}

func bitolaAuctionView(bids ...bot.BidRecord) bot.View {
	return bot.View{Seat: 0, HasTrumpCandidate: true, Bids: bids}
}

func handsWith(seat int, ids ...string) [4]uint32 {
	var hands [4]uint32
	hands[seat] = search.MaskOf(cards(ids...))
	return hands
}

func TestBidConsistentRejectsAPasserWithATableHand(t *testing.T) {
	filter := bot.BidConsistentForTest(croatianAuctionView(passRec(1, 1, 0, nil)))
	require.NotNil(t, filter)

	assert.False(t, filter(handsWith(1, "JH", "9H", "JS", "9S", "JD", "9D", "JC", "9C")),
		"every six of these cards keep a Jack and Nine together")
	assert.True(t, filter(handsWith(1, "7H", "8H", "7S", "8S", "7D", "8D", "7C", "8C")))
}

func TestBidConsistentAcceptsTheForcedPick(t *testing.T) {
	weakCaller := handsWith(2, "7S", "8H", "7H", "8D", "7D", "8C", "7C", "QD")

	forced := bot.BidConsistentForTest(croatianAuctionView(callRec(2, 1, 3, game.SuitSpades, nil)))
	if forced != nil {
		assert.True(t, forced(weakCaller), "the dealer had to call, so the hand proves nothing")
	}

	free := bot.BidConsistentForTest(croatianAuctionView(callRec(2, 1, 1, game.SuitSpades, nil)))
	require.NotNil(t, free)
	assert.False(t, free(weakCaller), "no six of these cards fit the table for spades")
	assert.True(t, free(handsWith(2, "JS", "9S", "7H", "8D", "7D", "8C", "7C", "QD")))
}

func TestBidConsistentCountsThePassersPlayedCards(t *testing.T) {
	v := croatianAuctionView(passRec(1, 1, 0, nil))
	v.CurrentTrick = []game.TrickCard{{Card: card("7C"), PlayerSeat: 1}}
	remaining := handsWith(1, "9D", "8C")

	filter := bot.BidConsistentForTest(v)
	require.NotNil(t, filter)
	require.True(t, filter(remaining), "the cards in sight explain the pass")

	v.PlayedBy[1] = cards("JH", "9H", "JS", "9S", "JD")
	filter = bot.BidConsistentForTest(v)
	assert.False(t, filter(remaining), "with its played cards every six-card hand fits the table")
}

func TestBidConsistentTriesEveryBidTimeSubset(t *testing.T) {
	cand := card("QH")

	t.Run("a bitola passer is explained by any five cards that miss the table", func(t *testing.T) {
		filter := bot.BidConsistentForTest(bitolaAuctionView(passRec(1, 1, 0, &cand)))
		require.NotNil(t, filter)
		assert.True(t, filter(handsWith(1, "JH", "9H", "7S", "8S", "7D", "8D", "7C", "8C")))
	})

	t.Run("a bitola passer whose every five cards fit is rejected", func(t *testing.T) {
		filter := bot.BidConsistentForTest(bitolaAuctionView(passRec(1, 1, 0, &cand)))
		assert.False(t, filter(handsWith(1, "7H", "8H", "9H", "TH", "JH", "KH", "AH", "AS")))
	})

	t.Run("a croatian caller without a fitting six is rejected", func(t *testing.T) {
		filter := bot.BidConsistentForTest(croatianAuctionView(callRec(2, 1, 0, game.SuitSpades, nil)))
		assert.False(t, filter(handsWith(2, "7S", "8H", "7H", "8D", "7D", "8C", "7C", "QD")))
	})

	t.Run("a bitola caller holds the candidate", func(t *testing.T) {
		filter := bot.BidConsistentForTest(bitolaAuctionView(passRec(1, 1, 0, &cand), callRec(2, 1, 1, game.SuitHearts, &cand)))
		assert.True(t, filter(handsWith(2, "QH", "JH", "9H", "7S", "8S", "7D", "8D", "7C")))
		assert.False(t, filter(handsWith(2, "QH", "7S", "8S", "9S", "7D", "8D", "7C", "8C")))
	})

	t.Run("a bitola auction without a recorded candidate is not filtered", func(t *testing.T) {
		assert.Nil(t, bot.BidConsistentForTest(bitolaAuctionView(passRec(1, 1, 0, nil))))
	})

	t.Run("the bot's own bids and an empty auction are not filtered", func(t *testing.T) {
		assert.Nil(t, bot.BidConsistentForTest(croatianAuctionView()))
		own := bot.BidConsistentForTest(croatianAuctionView(passRec(0, 1, 0, nil)))
		if own != nil {
			assert.True(t, own(handsWith(0, "JH", "9H", "JS", "9S", "JD", "9D", "JC", "9C")))
		}
	})
}
