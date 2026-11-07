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

func TestChooseAdjustedPrefersTheSteadierLead(t *testing.T) {
	hearts := game.SuitHearts
	res := search.Result{Cards: map[int]search.CardStats{
		search.CardIdx(card("7S")): {Mean: 10, SE: 1},
		search.CardIdx(card("AC")): {Mean: 11, SE: 4},
	}}
	oneCard := []game.TrickCard{{Card: card("7D"), PlayerSeat: 3}}
	tests := []struct {
		name   string
		lambda float64
		trick  []game.TrickCard
		want   string
	}{
		{"no risk weight", 0, nil, "AC"},
		{"a mild risk weight", 0.25, nil, "AC"},
		{"a risk weight past the crossover", 0.5, nil, "7S"},
		{"a full risk weight", 1, nil, "7S"},
		{"a non-empty trick ignores the weight", 1, oneCard, "AC"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := defenderLeadView([]string{"7S", "AC"}, &hearts, 0, tt.trick)
			got, ok := bot.ChooseAdjustedForTest(v, res, bot.SearchTuning{LeadRiskLambda: tt.lambda})
			require.True(t, ok)
			assert.Equal(t, tt.want, got.String())
		})
	}
}

func TestTopOutstandingTrump(t *testing.T) {
	hearts := game.SuitHearts
	tests := []struct {
		name   string
		hand   []string
		played []string
		trick  []game.TrickCard
		want   int
	}{
		{"the jack is out there", []string{"7S", "AC"}, nil, nil, search.CardIdx(card("JH"))},
		{"the jack is in hand", []string{"JH", "AC"}, nil, nil, search.CardIdx(card("9H"))},
		{"the jack and nine are played", []string{"7S", "AC"}, []string{"JH", "9H"}, nil, search.CardIdx(card("AH"))},
		{"the jack is in the trick", []string{"7S", "AC"}, nil, []game.TrickCard{{Card: card("JH"), PlayerSeat: 3}}, search.CardIdx(card("9H"))},
		{"every trump is accounted for", []string{"JH", "AC"}, []string{"9H", "AH", "TH", "KH", "QH", "8H", "7H"}, nil, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := defenderLeadView(tt.hand, &hearts, 0, tt.trick)
			v.PlayedCards = cards(tt.played...)
			assert.Equal(t, tt.want, bot.TopOutstandingTrumpForTest(v))
		})
	}
}

func TestTopTrumpWithOpponentsClassifiesTheSample(t *testing.T) {
	top := search.CardIdx(card("JH"))
	class := bot.TopTrumpWithOpponentsForTest(0, top)
	var withPartner, withOpponent [4]uint32
	withPartner[2] = 1 << uint(top)
	withOpponent[1] = 1 << uint(top)
	assert.Equal(t, 0, class(withPartner))
	assert.Equal(t, 1, class(withOpponent))
	withOpponent[1] = 0
	withOpponent[3] = 1 << uint(top)
	assert.Equal(t, 1, class(withOpponent))
}

func guessResult(chosen string, partner, opponents map[int]float64, opponentSamples int) search.Result {
	return search.Result{Card: card(chosen), Classes: [2]search.ClassStats{
		{Samples: 5, Means: partner},
		{Samples: opponentSamples, Means: opponents},
	}}
}

func TestTrumpDrawIsAGuess(t *testing.T) {
	hearts := game.SuitHearts
	jh, s7 := search.CardIdx(card("JH")), search.CardIdx(card("7S"))
	partnerWins := map[int]float64{jh: 20, s7: 10}
	opponentsWin := map[int]float64{jh: 5, s7: 12}
	oneCard := []game.TrickCard{{Card: card("7D"), PlayerSeat: 3}}
	tests := []struct {
		name  string
		res   search.Result
		hand  []string
		trick []game.TrickCard
		want  bool
	}{
		{"the classes disagree", guessResult("JH", partnerWins, opponentsWin, 5), []string{"JH", "7S"}, nil, true},
		{"both classes prefer the trump", guessResult("JH", partnerWins, map[int]float64{jh: 15, s7: 12}, 5), []string{"JH", "7S"}, nil, false},
		{"one class has no samples", guessResult("JH", partnerWins, nil, 0), []string{"JH", "7S"}, nil, false},
		{"neither class prefers the trump", guessResult("JH", map[int]float64{jh: 5, s7: 10}, opponentsWin, 5), []string{"JH", "7S"}, nil, false},
		{"no non-trump card to compare", guessResult("JH", partnerWins, opponentsWin, 5), []string{"JH", "9H"}, nil, false},
		{"the chosen card is not a trump", guessResult("7S", partnerWins, opponentsWin, 5), []string{"JH", "7S"}, nil, false},
		{"the trick is not empty", guessResult("JH", partnerWins, opponentsWin, 5), []string{"JH", "7S"}, oneCard, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := defenderLeadView(tt.hand, &hearts, 0, tt.trick)
			assert.Equal(t, tt.want, bot.TrumpDrawIsAGuessForTest(v, tt.res))
		})
	}
}

func TestDeferTrumpDrawHandsAGuessToTheRules(t *testing.T) {
	hearts := game.SuitHearts
	jh, s7 := search.CardIdx(card("JH")), search.CardIdx(card("7S"))
	res := guessResult("JH", map[int]float64{jh: 20, s7: 10}, map[int]float64{jh: 5, s7: 12}, 5)
	res.Samples = 40
	v := defenderLeadView([]string{"JH", "7S"}, &hearts, 0, nil)

	got, deferred := bot.DeferTrumpDrawForTest(v, res, card("JH"), bot.SearchTuning{TrumpDrawConditional: true})
	require.True(t, deferred)
	assert.True(t, got.Deferred)
	assert.Equal(t, 40, got.Samples)
	assert.Contains(t, v.LegalCards, got.Card)

	_, deferred = bot.DeferTrumpDrawForTest(v, res, card("JH"), bot.SearchTuning{})
	assert.False(t, deferred, "the conditional is off")
	_, deferred = bot.DeferTrumpDrawForTest(v, res, card("7S"), bot.SearchTuning{TrumpDrawConditional: true})
	assert.False(t, deferred, "the adjusted choice is not a trump")
}
