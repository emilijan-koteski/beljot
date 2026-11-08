package bot_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/bot"
	"github.com/emilijan/beljot/server/internal/game"
)

func blindTenView(hand []string, trick []game.TrickCard) bot.View {
	hearts := game.SuitHearts
	return bot.View{Seat: 0, Hand: cards(hand...), LegalCards: cards(hand...), TrumpSuit: &hearts, CurrentTrick: trick}
}

func TestTrickContextOf(t *testing.T) {
	t.Run("an empty trick is the zero context", func(t *testing.T) {
		assert.Equal(t, bot.TrickContext{}, bot.TrickContextOfForTest(blindTenView([]string{"TS"}, nil)))
	})
	t.Run("the partner winning is the side winning", func(t *testing.T) {
		ctx := bot.TrickContextOfForTest(blindTenView([]string{"TS"}, []game.TrickCard{{Card: card("8D"), PlayerSeat: 2}, {Card: card("7D"), PlayerSeat: 3}}))
		assert.Equal(t, 2, ctx.Position)
		assert.Equal(t, 2, ctx.WinnerSeat)
		assert.True(t, ctx.SideWinning)
		assert.False(t, ctx.TrumpCut)
		require.NotNil(t, ctx.Led)
		assert.Equal(t, game.SuitDiamonds, *ctx.Led)
	})
	t.Run("a trump cut by an opponent", func(t *testing.T) {
		ctx := bot.TrickContextOfForTest(blindTenView([]string{"TS"}, []game.TrickCard{{Card: card("8D"), PlayerSeat: 2}, {Card: card("7H"), PlayerSeat: 3}}))
		assert.Equal(t, 3, ctx.WinnerSeat)
		assert.True(t, ctx.TrumpCut)
		assert.False(t, ctx.SideWinning)
	})
}

func TestBlindTen(t *testing.T) {
	opponentLed := []game.TrickCard{{Card: card("8D"), PlayerSeat: 1}}
	tests := []struct {
		name  string
		hand  []string
		trick []game.TrickCard
		setup func(v *bot.View)
		ten   string
		want  bool
	}{
		{"ace played", []string{"TS", "7D"}, nil, func(v *bot.View) { v.PlayedCards = cards("AS") }, "TS", false},
		{"ace in hand", []string{"TS", "AS"}, nil, nil, "TS", false},
		{"ace known with the partner", []string{"TS", "7D"}, nil, func(v *bot.View) { v.KnownCards[2] = cards("AS") }, "TS", false},
		{"ace known with an opponent", []string{"TS", "7D"}, nil, func(v *bot.View) { v.KnownCards[1] = cards("AS") }, "TS", true},
		{"ace unknown, leading", []string{"TS", "7D"}, nil, nil, "TS", true},
		{"ace unknown, opponent winning", []string{"TS", "7C"}, opponentLed, nil, "TS", true},
		{"partner winning", []string{"TS", "7C"}, []game.TrickCard{{Card: card("8D"), PlayerSeat: 2}, {Card: card("7D"), PlayerSeat: 3}}, nil, "TS", false},
		{"ace in the current trick", []string{"TS", "7C"}, []game.TrickCard{{Card: card("AS"), PlayerSeat: 1}}, nil, "TS", false},
		{"trump ten", []string{"TH", "7D"}, nil, nil, "TH", false},
		{"not a ten", []string{"KS", "7D"}, nil, nil, "KS", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := blindTenView(tt.hand, tt.trick)
			if tt.setup != nil {
				tt.setup(&v)
			}
			assert.Equal(t, tt.want, bot.BlindTenForTest(v, card(tt.ten)))
		})
	}
}

func TestAvoidBlindTenKeepsTheOnlyCard(t *testing.T) {
	v := blindTenView([]string{"TS"}, nil)
	got := bot.AvoidBlindTenForTest(v, v.LegalCards, game.SuitHearts, card("TS"))
	assert.Equal(t, "TS", got.String())
}

func TestAvoidBlindTenSwapsInTheLowestOtherCard(t *testing.T) {
	v := blindTenView([]string{"TS", "AD", "KC"}, nil)
	got := bot.AvoidBlindTenForTest(v, v.LegalCards, game.SuitHearts, card("TS"))
	assert.Equal(t, "KC", got.String())
	assert.Equal(t, "AD", bot.AvoidBlindTenForTest(v, v.LegalCards, game.SuitHearts, card("AD")).String())
}
