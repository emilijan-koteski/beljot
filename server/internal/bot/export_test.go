package bot

import (
	"time"

	"github.com/emilijan/beljot/server/internal/bot/search"
	"github.com/emilijan/beljot/server/internal/game"
)

var (
	CardsProvablyAbsentForTest = cardsProvablyAbsent
	InformationFromForTest     = informationFrom
	PositionFromForTest        = positionFrom
	SettlementFromForTest      = settlementFrom
	RemainingCardPointsForTest = remainingCardPoints
)

func ChooserResultForTest(v View, deadline time.Time) (search.Result, bool) {
	return chooseSearch(v, search.SamplesForTrick(v.TrickNumber), deadline, SearchTuning{}, nil)
}

func TenUnderAceCandidateForTest(v View) *game.Card {
	return tenUnderAceCandidate(v, v.LegalCards, *v.TrumpSuit)
}

func ChooseAdjustedForTest(v View, res search.Result, t SearchTuning) (game.Card, bool) {
	return chooseAdjusted(v, res, t)
}

func DefenderTrumpLeadConventionForTest(v View) bool {
	return defenderTrumpLeadConvention(v)
}

func TopOutstandingTrumpForTest(v View) int {
	return topOutstandingTrump(v)
}

func TopTrumpWithOpponentsForTest(seat, top int) func(hands [4]uint32) int {
	return topTrumpWithOpponents(View{Seat: seat}, top)
}

func TrumpDrawIsAGuessForTest(v View, adjusted game.Card, res search.Result) bool {
	return trumpDrawIsAGuess(v, adjusted, res)
}

func SamplesForForTest(trick int, t SearchTuning) int {
	return samplesFor(trick, t)
}

func DeferTrumpDrawForTest(v View, res search.Result, adjusted game.Card, t SearchTuning) (SearchResult, bool) {
	return deferTrumpDraw(v, res, adjusted, t)
}

func ChooseCardForTest(v View) game.Card {
	return chooseCard(v)
}

func TrickContextOfForTest(v View) TrickContext {
	return trickContextOf(v)
}

func BlindTenForTest(v View, c game.Card) bool {
	return blindTen(v, c, trickContextOf(v))
}

func AvoidBlindTenForTest(v View, legal []game.Card, trump game.Suit, pick game.Card) game.Card {
	return avoidBlindTen(v, legal, trump, pick)
}
