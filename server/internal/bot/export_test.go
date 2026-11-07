package bot

import (
	"time"

	"github.com/emilijan/beljot/server/internal/bot/search"
)

var (
	CardsProvablyAbsentForTest = cardsProvablyAbsent
	InformationFromForTest     = informationFrom
	PositionFromForTest        = positionFrom
	SettlementFromForTest      = settlementFrom
	RemainingCardPointsForTest = remainingCardPoints
)

func ChooserResultForTest(v View, deadline time.Time) (search.Result, bool) {
	return chooseSearch(v, search.SamplesForTrick(v.TrickNumber), deadline)
}
