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
	return chooseSearch(v, search.SamplesForTrick(v.TrickNumber), deadline, 0)
}

func TenUnderAceCandidateForTest(v View) *game.Card {
	return tenUnderAceCandidate(v, v.LegalCards, *v.TrumpSuit)
}

func ChooseAdjustedForTest(v View, res search.Result, t SearchTuning) (game.Card, bool) {
	return chooseAdjusted(v, res, t)
}
