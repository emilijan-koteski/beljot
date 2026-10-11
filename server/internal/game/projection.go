package game

// ProjectForSeat returns a deep-cloned copy of gs masked to what seat may see: other hands, unresolved
// declarations and others' PendingBelotSeat are hidden. An out-of-range seat matches nobody and sees everything masked.
func ProjectForSeat(gs *GameState, seat int) *GameState {
	p := cloneGameState(gs)

	for i := range p.Players {
		p.Players[i].HandCount = len(p.Players[i].Hand)
		if i == seat {
			continue
		}
		p.Players[i].Hand = []Card{}
		if !p.DeclarationsResolved {
			p.Players[i].Declarations = []Declaration{}
		}
	}

	if p.PendingBelotSeat != nil && *p.PendingBelotSeat != seat {
		p.PendingBelotSeat = nil
	}

	return p
}
