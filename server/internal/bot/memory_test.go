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
