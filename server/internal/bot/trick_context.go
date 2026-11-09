package bot

import "github.com/emilijan/beljot/server/internal/game"

// TrickContext describes the trick in progress from the deciding seat's side. An empty trick is the zero value.
type TrickContext struct {
	Position    int
	Led         *game.Suit
	TrumpCut    bool
	WinnerSeat  int
	SideWinning bool
}

func trickContextOf(v View) TrickContext {
	ctx := TrickContext{Position: len(v.CurrentTrick)}
	if ctx.Position == 0 || v.TrumpSuit == nil {
		return ctx
	}
	led := v.CurrentTrick[0].Card.Suit
	seat, winning := trickWinner(v.CurrentTrick, *v.TrumpSuit)
	ctx.Led = &led
	ctx.WinnerSeat = seat
	ctx.TrumpCut = winning.Suit == *v.TrumpSuit && led != *v.TrumpSuit
	ctx.SideWinning = game.TeamForSeat(seat) == game.TeamForSeat(v.Seat)
	return ctx
}
