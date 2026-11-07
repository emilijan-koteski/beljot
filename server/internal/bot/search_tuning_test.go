package bot_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/bot"
	"github.com/emilijan/beljot/server/internal/bot/search"
	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/game/testfixtures"
)

func TestChooseAdjustedAppliesTheTenUnderAceBonus(t *testing.T) {
	v := viewForTweakCase(tenUnderAceCases()[0])
	require.NotNil(t, bot.TenUnderAceCandidateForTest(v))
	resultWith := func(tenMean, otherMean float64) search.Result {
		return search.Result{Cards: map[int]search.CardStats{
			search.CardIdx(card("TS")): {Mean: tenMean},
			search.CardIdx(card("7D")): {Mean: otherMean},
		}}
	}
	tests := []struct {
		name  string
		res   search.Result
		bonus int
		want  string
	}{
		{"the bonus lifts the ten past a close alternative", resultWith(10, 12), 3, "TS"},
		{"without the bonus the alternative wins", resultWith(10, 12), 0, "7D"},
		{"the bonus does not rescue a clearly worse ten", resultWith(10, 14), 3, "7D"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := bot.ChooseAdjustedForTest(v, tt.res, bot.SearchTuning{TenUnderAceBonus: tt.bonus})
			require.True(t, ok)
			assert.Equal(t, tt.want, got.String())
		})
	}
}

func defenderLeadView(hand []string, trump *game.Suit, callerSeat int, trick []game.TrickCard) bot.View {
	gs := testfixtures.NewGameMidPlay(1)
	gs.Players[0].Hand = cards(hand...)
	gs.CurrentTrick = trick
	gs.ActivePlayerSeat = 0
	gs.TrumpSuit = trump
	gs.TrumpCallerSeat = &callerSeat
	v := viewFromState(gs, 0, nil)
	v.LegalCards = v.Hand
	return v
}

func TestDefenderTrumpLeadConvention(t *testing.T) {
	hearts := game.SuitHearts
	oneCard := []game.TrickCard{{Card: card("7S"), PlayerSeat: 3}}
	tests := []struct {
		name   string
		hand   []string
		trump  *game.Suit
		caller int
		trick  []game.TrickCard
		want   bool
	}{
		{"the caller's partner", []string{"8H", "7S"}, &hearts, 2, nil, false},
		{"a defender with side cards on an empty trick", []string{"8H", "7S"}, &hearts, 1, nil, true},
		{"a defender on a non-empty trick", []string{"8H", "7S"}, &hearts, 1, oneCard, false},
		{"a defender holding only trumps", []string{"8H", "7H"}, &hearts, 1, nil, false},
		{"no trump suit", []string{"8H", "7S"}, nil, 1, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := defenderLeadView(tt.hand, tt.trump, tt.caller, tt.trick)
			assert.Equal(t, tt.want, bot.DefenderTrumpLeadConventionForTest(v))
		})
	}
}

func TestChooseAdjustedPenalisesTheDefendersTrumpLead(t *testing.T) {
	hearts := game.SuitHearts
	means := func(m map[string]float64) search.Result {
		res := search.Result{Cards: map[int]search.CardStats{}}
		for id, mean := range m {
			res.Cards[search.CardIdx(card(id))] = search.CardStats{Mean: mean}
		}
		return res
	}
	tests := []struct {
		name  string
		hand  []string
		res   search.Result
		bonus int
		want  string
	}{
		{"the penalty moves the lead to the side card", []string{"8H", "7S"}, means(map[string]float64{"8H": 15, "7S": 12, "JH": 14}), 10, "7S"},
		{"without the bonus the trump wins", []string{"8H", "7S"}, means(map[string]float64{"8H": 15, "7S": 12, "JH": 14}), 0, "8H"},
		{"the Jack of trump is exempt", []string{"JH", "7S"}, means(map[string]float64{"JH": 13, "7S": 12}), 10, "JH"},
		{"the veto loses to any side lead", []string{"8H", "7S"}, means(map[string]float64{"8H": 100, "7S": 1}), bot.DefenderTrumpLeadVeto, "7S"},
		{"only trumps legal still chooses a card", []string{"8H", "7H"}, means(map[string]float64{"8H": 5, "7H": 3}), bot.DefenderTrumpLeadVeto, "8H"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := defenderLeadView(tt.hand, &hearts, 1, nil)
			got, ok := bot.ChooseAdjustedForTest(v, tt.res, bot.SearchTuning{DefenderTrumpLeadBonus: tt.bonus})
			require.True(t, ok)
			assert.Equal(t, tt.want, got.String())
		})
	}
}
