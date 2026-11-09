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
		{"the Nine of trump is exempt", []string{"9H", "7S"}, means(map[string]float64{"9H": 13, "7S": 12}), 10, "9H"},
		{"a side Jack or Nine is not penalised", []string{"8H", "JS", "9C"}, means(map[string]float64{"8H": 5, "JS": 13, "9C": 12}), 10, "JS"},
		{"the veto loses to any side lead", []string{"8H", "7S"}, means(map[string]float64{"8H": 100, "7S": 1}), bot.ConventionVeto, "7S"},
		{"only trumps legal still chooses a card", []string{"8H", "7H"}, means(map[string]float64{"8H": 5, "7H": 3}), bot.ConventionVeto, "8H"},
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

func TestChooseAdjustedEdgeCases(t *testing.T) {
	hearts := game.SuitHearts
	v := defenderLeadView([]string{"7S", "AC"}, &hearts, 0, nil)
	stats := func(m map[string]search.CardStats) search.Result {
		res := search.Result{Cards: map[int]search.CardStats{}}
		for id, s := range m {
			res.Cards[search.CardIdx(card(id))] = s
		}
		return res
	}

	t.Run("a legal card the search did not value is skipped", func(t *testing.T) {
		got, ok := bot.ChooseAdjustedForTest(v, stats(map[string]search.CardStats{"AC": {Mean: -50}}), bot.SearchTuning{})
		require.True(t, ok)
		assert.Equal(t, "AC", got.String())
	})

	t.Run("no valued legal card chooses nothing", func(t *testing.T) {
		_, ok := bot.ChooseAdjustedForTest(v, stats(map[string]search.CardStats{"KD": {Mean: 5}}), bot.SearchTuning{})
		assert.False(t, ok)
	})

	t.Run("an empty legal set chooses nothing", func(t *testing.T) {
		empty := v
		empty.LegalCards = nil
		_, ok := bot.ChooseAdjustedForTest(empty, stats(map[string]search.CardStats{"7S": {Mean: 5}}), bot.SearchTuning{})
		assert.False(t, ok)
	})

	t.Run("a tie goes to the lowest card index", func(t *testing.T) {
		want := "7S"
		if search.CardIdx(card("AC")) < search.CardIdx(card("7S")) {
			want = "AC"
		}
		res := stats(map[string]search.CardStats{"7S": {Mean: 4, SE: 1}, "AC": {Mean: 4, SE: 1}})
		for _, order := range [][]string{{"7S", "AC"}, {"AC", "7S"}} {
			tied := v
			tied.LegalCards = cards(order...)
			got, ok := bot.ChooseAdjustedForTest(tied, res, bot.SearchTuning{LeadRiskLambda: 1})
			require.True(t, ok)
			assert.Equal(t, want, got.String(), "legal order %v", order)
		}
	})
}

func TestChooseAdjustedCombinesTheRiskWeightWithABonus(t *testing.T) {
	hearts := game.SuitHearts
	v := defenderLeadView([]string{"8H", "7S"}, &hearts, 1, nil)
	res := search.Result{Cards: map[int]search.CardStats{
		search.CardIdx(card("8H")): {Mean: 15, SE: 1},
		search.CardIdx(card("7S")): {Mean: 12, SE: 4},
	}}
	tests := []struct {
		name   string
		lambda float64
		bonus  int
		want   string
	}{
		{"the bonus alone moves the lead to the side card", 0, 5, "7S"},
		{"the risk weight alone keeps the steadier trump", 1, 0, "8H"},
		{"both: 15-1-5 beats 12-4", 1, 5, "8H"},
		{"both: 15-1-7 loses to 12-4", 1, 7, "7S"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tuning := bot.SearchTuning{LeadRiskLambda: tt.lambda, DefenderTrumpLeadBonus: tt.bonus}
			got, ok := bot.ChooseAdjustedForTest(v, res, tuning)
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
		{"the trick is not empty", guessResult("JH", partnerWins, opponentsWin, 5), []string{"JH", "7S"}, oneCard, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := defenderLeadView(tt.hand, &hearts, 0, tt.trick)
			assert.Equal(t, tt.want, bot.TrumpDrawIsAGuessForTest(v, tt.res.Card, tt.res))
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

func TestTrumpDrawIsAGuessJudgesTheAdjustedCard(t *testing.T) {
	hearts := game.SuitHearts
	jh, s7 := search.CardIdx(card("JH")), search.CardIdx(card("7S"))
	res := guessResult("7S", map[int]float64{jh: 20, s7: 10}, map[int]float64{jh: 5, s7: 12}, 5)
	v := defenderLeadView([]string{"JH", "7S"}, &hearts, 0, nil)

	assert.True(t, bot.TrumpDrawIsAGuessForTest(v, card("JH"), res), "a promoted trump lead is judged")
	assert.False(t, bot.TrumpDrawIsAGuessForTest(v, card("7S"), res), "a non-trump adjusted card is not a trump draw")
}

func TestSamplesForHonoursACap(t *testing.T) {
	caps := bot.SearchTuning{SampleCaps: [9]int{3: 32, 4: 60, 5: 80}}
	assert.Equal(t, 32, bot.SamplesForForTest(3, caps))
	assert.Equal(t, 60, bot.SamplesForForTest(4, caps))
	assert.Equal(t, 80, bot.SamplesForForTest(5, caps))
	assert.Equal(t, search.SamplesForTrick(6), bot.SamplesForForTest(6, caps))
	assert.Equal(t, 12, bot.SamplesForForTest(1, bot.SearchTuning{OpeningSamples: 12, SampleCaps: [9]int{3: 32}}))
	assert.Equal(t, 7, bot.SamplesForForTest(2, bot.SearchTuning{OpeningSamples: 12, SampleCaps: [9]int{2: 7}}))
	assert.Equal(t, search.SamplesForTrick(3), bot.SamplesForForTest(3, bot.SearchTuning{}))
}

func TestChooseAdjustedPenalisesABlindTen(t *testing.T) {
	hearts := game.SuitHearts
	res := search.Result{Cards: map[int]search.CardStats{
		search.CardIdx(card("TS")): {Mean: 4},
		search.CardIdx(card("7D")): {Mean: 0},
	}}
	v := defenderLeadView([]string{"TS", "7D"}, &hearts, 1, nil)
	tests := []struct {
		name    string
		penalty int
		want    string
	}{
		{"the penalty moves the lead off the blind ten", 10, "7D"},
		{"without the penalty the ten wins", 0, "TS"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := bot.ChooseAdjustedForTest(v, res, bot.SearchTuning{BlindTenPenalty: tt.penalty})
			require.True(t, ok)
			assert.Equal(t, tt.want, got.String())
		})
	}
}

func TestChooseAdjustedKeepsTheOnlyLegalTen(t *testing.T) {
	hearts := game.SuitHearts
	res := search.Result{Cards: map[int]search.CardStats{search.CardIdx(card("TS")): {Mean: -3}}}
	v := defenderLeadView([]string{"TS"}, &hearts, 1, nil)
	got, ok := bot.ChooseAdjustedForTest(v, res, bot.SearchTuning{BlindTenPenalty: bot.ConventionVeto})
	require.True(t, ok)
	assert.Equal(t, "TS", got.String())
}

func TestDefaultSearchTuningBlindTenPenalty(t *testing.T) {
	assert.Equal(t, bot.ConventionVeto, bot.DefaultSearchTuning().BlindTenPenalty)
}

func cashHighView(hand []string, trick []game.TrickCard) bot.View {
	hearts := game.SuitHearts
	v := bot.View{Seat: 0, Hand: cards(hand...), TrumpSuit: &hearts, CurrentTrick: trick}
	for _, c := range v.Hand {
		if len(trick) == 0 || c.Suit == trick[0].Card.Suit {
			v.LegalCards = append(v.LegalCards, c)
		}
	}
	if len(v.LegalCards) == 0 {
		v.LegalCards = v.Hand
	}
	return v
}

func lastToPlayOnDiamonds(third string) []game.TrickCard {
	return []game.TrickCard{{Card: card("8D"), PlayerSeat: 1}, {Card: card("7D"), PlayerSeat: 2}, {Card: card(third), PlayerSeat: 3}}
}

func TestSuitAtRisk(t *testing.T) {
	hearts := game.SuitHearts
	tests := []struct {
		name  string
		hand  []string
		setup func(v *bot.View)
		want  bool
	}{
		{"three held", []string{"AD", "QD", "9D", "7S"}, func(v *bot.View) {}, true},
		{"two held and nothing declared", []string{"AD", "QD", "7S", "8S"}, func(v *bot.View) {}, false},
		{"an opponent declared a run in the suit", []string{"AD", "QD", "7S", "8S"}, func(v *bot.View) {
			v.DeclaredCards[1] = cards("7D", "8D", "9D")
		}, true},
		{"an opponent void in the suit with trumps possible", []string{"AD", "QD", "7S", "8S"}, func(v *bot.View) {
			v.KnownVoids[1][bot.SuitIndex(game.SuitDiamonds)] = true
		}, true},
		{"an opponent void in the suit and in trumps", []string{"AD", "QD", "7S", "8S"}, func(v *bot.View) {
			v.KnownVoids[1][bot.SuitIndex(game.SuitDiamonds)] = true
			v.KnownVoids[1][bot.SuitIndex(hearts)] = true
		}, false},
		{"the partner void in the suit is no risk", []string{"AD", "QD", "7S", "8S"}, func(v *bot.View) {
			v.KnownVoids[2][bot.SuitIndex(game.SuitDiamonds)] = true
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := cashHighView(tt.hand, nil)
			tt.setup(&v)
			assert.Equal(t, tt.want, bot.SuitAtRiskForTest(v, game.SuitDiamonds, hearts))
		})
	}
}

func TestCashHighWinner(t *testing.T) {
	tests := []struct {
		name  string
		hand  []string
		trick []game.TrickCard
		card  string
		want  bool
	}{
		{"the Ace with three held", []string{"AD", "QD", "9D", "7S"}, lastToPlayOnDiamonds("JD"), "AD", true},
		{"the Queen is never cashed", []string{"AD", "QD", "9D", "7S"}, lastToPlayOnDiamonds("JD"), "QD", false},
		{"the Ten with three held", []string{"TD", "KD", "QD", "7S"}, lastToPlayOnDiamonds("JD"), "TD", true},
		{"the King is never cashed", []string{"TD", "KD", "QD", "7S"}, lastToPlayOnDiamonds("JD"), "KD", false},
		{"a trump cut", []string{"AD", "QD", "9D", "7S"}, []game.TrickCard{{Card: card("8D"), PlayerSeat: 1}, {Card: card("7D"), PlayerSeat: 2}, {Card: card("7H"), PlayerSeat: 3}}, "AD", false},
		{"the led suit is trump", []string{"AH", "QH", "7H", "7S"}, []game.TrickCard{{Card: card("8H"), PlayerSeat: 1}, {Card: card("9S"), PlayerSeat: 2}, {Card: card("7H"), PlayerSeat: 3}}, "AH", false},
		{"only the Ace overtakes", []string{"AD", "7D", "8D", "7S"}, lastToPlayOnDiamonds("JD"), "AD", false},
		{"two held and nothing declared", []string{"AD", "QD", "7S", "8S"}, lastToPlayOnDiamonds("JD"), "AD", false},
		{"not last to play", []string{"AD", "QD", "9D", "7S"}, lastToPlayOnDiamonds("JD")[:2], "AD", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, bot.CashHighWinnerForTest(cashHighView(tt.hand, tt.trick), card(tt.card)))
		})
	}
}

func TestChooseAdjustedCashesTheAceAtRisk(t *testing.T) {
	res := search.Result{Cards: map[int]search.CardStats{
		search.CardIdx(card("AD")): {Mean: 1},
		search.CardIdx(card("QD")): {Mean: 3},
		search.CardIdx(card("9D")): {Mean: -5},
	}}
	v := cashHighView([]string{"AD", "QD", "9D", "7S"}, lastToPlayOnDiamonds("JD"))
	tests := []struct {
		name  string
		bonus int
		want  string
	}{
		{"the bonus moves the take to the Ace", 10, "AD"},
		{"without the bonus the Queen wins", 0, "QD"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := bot.ChooseAdjustedForTest(v, res, bot.SearchTuning{CashHighWinnerBonus: tt.bonus})
			require.True(t, ok)
			assert.Equal(t, tt.want, got.String())
		})
	}
}

func TestDefaultSearchTuningCashHighWinnerBonus(t *testing.T) {
	assert.Equal(t, bot.ConventionVeto, bot.DefaultSearchTuning().CashHighWinnerBonus)
}

func TestDefaultSearchTuningDecidingHand(t *testing.T) {
	assert.False(t, bot.DefaultSearchTuning().DecidingHand, "D6b was not better at 2 SE")
}

func fourthHandOnSpades() []game.TrickCard {
	return []game.TrickCard{{Card: card("7S"), PlayerSeat: 1}, {Card: card("8S"), PlayerSeat: 2}, {Card: card("QS"), PlayerSeat: 3}}
}

func TestBlindTenIsALeadOrADiscard(t *testing.T) {
	tests := []struct {
		name  string
		hand  []string
		trick []game.TrickCard
		want  bool
	}{
		{"fourth hand following takes the trick", []string{"TS", "KS"}, fourthHandOnSpades(), false},
		{"second hand following an opponent's lead", []string{"TS", "KS"}, []game.TrickCard{{Card: card("7S"), PlayerSeat: 3}}, false},
		{"void in the led suit with an opponent winning", []string{"TS", "7C"}, []game.TrickCard{{Card: card("8D"), PlayerSeat: 3}}, true},
		{"following after an opponent's trump cut", []string{"TS", "7C"}, []game.TrickCard{{Card: card("8S"), PlayerSeat: 2}, {Card: card("7H"), PlayerSeat: 3}}, true},
		{"leading", []string{"TS", "7C"}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, bot.BlindTenForTest(blindTenView(tt.hand, tt.trick), card("TS")))
		})
	}
}

func TestChooseAdjustedTakesTheTrickWithTheTenLast(t *testing.T) {
	v := cashHighView([]string{"TS", "KS"}, fourthHandOnSpades())
	res := search.Result{Cards: map[int]search.CardStats{
		search.CardIdx(card("TS")): {Mean: 8},
		search.CardIdx(card("KS")): {Mean: 0},
	}}
	assert.False(t, bot.BlindTenForTest(v, card("TS")))
	got, ok := bot.ChooseAdjustedForTest(v, res, bot.DefaultSearchTuning())
	require.True(t, ok)
	assert.Equal(t, "TS", got.String())
}

func TestChooseAdjustedCashesTheTenAtRiskWithItsAceOut(t *testing.T) {
	v := cashHighView([]string{"TS", "KS", "9S"}, fourthHandOnSpades())
	res := search.Result{Cards: map[int]search.CardStats{
		search.CardIdx(card("TS")): {Mean: 0},
		search.CardIdx(card("KS")): {Mean: 4},
		search.CardIdx(card("9S")): {Mean: -6},
	}}
	assert.True(t, bot.CashHighWinnerForTest(v, card("TS")))
	assert.False(t, bot.BlindTenForTest(v, card("TS")))
	got, ok := bot.ChooseAdjustedForTest(v, res, bot.DefaultSearchTuning())
	require.True(t, ok)
	assert.Equal(t, "TS", got.String())
}

func blindTenResult(means map[string]float64) search.Result {
	res := search.Result{Cards: map[int]search.CardStats{}}
	for id, mean := range means {
		res.Cards[search.CardIdx(card(id))] = search.CardStats{Mean: mean}
	}
	return res
}

func TestConventionVetoYieldsOnlyToADecidedRace(t *testing.T) {
	hearts := game.SuitHearts
	v := defenderLeadView([]string{"TS", "7D"}, &hearts, 1, nil)
	require.True(t, bot.BlindTenForTest(v, card("TS")))
	tests := []struct {
		name     string
		ten, low float64
		want     string
	}{
		{"the ten wins the match in every sample and the alternative loses it", search.RaceWin, -search.RaceWin, "TS"},
		{"a race gap short of decided keeps the veto", 600, -600, "7D"},
		{"a points margin of 600 keeps the veto", 600, 0, "7D"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := blindTenResult(map[string]float64{"TS": tt.ten, "7D": tt.low})
			got, ok := bot.ChooseAdjustedForTest(v, res, bot.DefaultSearchTuning())
			require.True(t, ok)
			assert.Equal(t, tt.want, got.String())
		})
	}
}

func TestBlindTenIsTheBetterDiscardThanAnAce(t *testing.T) {
	onClubs := []game.TrickCard{{Card: card("8C"), PlayerSeat: 3}}
	tests := []struct {
		name  string
		hand  []string
		trick []game.TrickCard
		means map[string]float64
		want  string
	}{
		{"a discard with only an Ace beside the ten", []string{"AD", "TS"}, onClubs, map[string]float64{"AD": 0, "TS": 4}, "TS"},
		{"a discard with a low card available", []string{"AD", "TS", "7C"}, []game.TrickCard{{Card: card("8H"), PlayerSeat: 3}},
			map[string]float64{"AD": 0, "TS": 4, "7C": 1}, "7C"},
		{"a lead keeps the veto", []string{"TS", "AD"}, nil, map[string]float64{"AD": 0, "TS": 4}, "AD"},
		{"a ten following after a trump cut keeps the veto", []string{"TS", "7S"},
			[]game.TrickCard{{Card: card("8S"), PlayerSeat: 2}, {Card: card("7H"), PlayerSeat: 3}}, map[string]float64{"7S": 0, "TS": 4}, "7S"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := blindTenView(tt.hand, tt.trick)
			if len(tt.trick) > 0 {
				v.LegalCards = nil
				for _, c := range v.Hand {
					if c.Suit == tt.trick[0].Card.Suit {
						v.LegalCards = append(v.LegalCards, c)
					}
				}
				if len(v.LegalCards) == 0 {
					v.LegalCards = v.Hand
				}
			}
			got, ok := bot.ChooseAdjustedForTest(v, blindTenResult(tt.means), bot.DefaultSearchTuning())
			require.True(t, ok)
			assert.Equal(t, tt.want, got.String())
		})
	}
}
