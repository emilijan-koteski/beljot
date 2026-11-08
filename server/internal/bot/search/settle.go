package search

import "github.com/emilijan/beljot/server/internal/game"

// Settlement is what Settle needs to know about a hand: the points and tricks banked before the remainder, both
// teams' declarations and Belote, who called trump, the room's declaration rule and the card points still to play.
type Settlement struct {
	HandPoints           [2]int
	DeclPoints           [2]int
	TricksWon            [2]int
	CallerTeam           int
	DeclarationsTransfer bool
	RemainingCardPoints  int
}

// Settle returns the team A minus team B award the engine banks for a hand whose remainder is worth remDiff.
func Settle(s Settlement, remDiff int) int {
	totals := SettleTotals(s, remDiff)
	return totals[game.TeamA] - totals[game.TeamB]
}

// SettleTotals returns what each team banks for a hand whose remainder is worth remDiff. It mirrors scoreHand: a
// Capot or a failed contract hands one side the whole hand and the other nothing, otherwise both sides keep their own.
func SettleTotals(s Settlement, remDiff int) [2]int {
	capotA := s.TricksWon[game.TeamB] == 0 && remDiff == s.RemainingCardPoints+capotBonus
	capotB := s.TricksWon[game.TeamA] == 0 && remDiff == -(s.RemainingCardPoints+capotBonus)
	bonus := lastTrickBonus
	if capotA || capotB {
		bonus = capotBonus
	}

	remaining := s.RemainingCardPoints + bonus
	remA := (remaining + remDiff) / 2
	remB := remaining - remA
	cardsA := s.HandPoints[game.TeamA] + remA
	cardsB := s.HandPoints[game.TeamB] + remB
	fullA := cardsA + s.DeclPoints[game.TeamA]
	fullB := cardsB + s.DeclPoints[game.TeamB]

	switch {
	case capotA:
		return [2]int{s.wholeHand(cardsA+cardsB, game.TeamA), 0}
	case capotB:
		return [2]int{0, s.wholeHand(cardsA+cardsB, game.TeamB)}
	case s.CallerTeam == game.TeamA && fullA <= fullB:
		return [2]int{0, s.wholeHand(cardsA+cardsB, game.TeamB)}
	case s.CallerTeam == game.TeamB && fullB <= fullA:
		return [2]int{s.wholeHand(cardsA+cardsB, game.TeamA), 0}
	}
	return [2]int{fullA, fullB}
}

func (s Settlement) wholeHand(cardPoints, winner int) int {
	if s.DeclarationsTransfer {
		return cardPoints + s.DeclPoints[game.TeamA] + s.DeclPoints[game.TeamB]
	}
	return cardPoints + s.DeclPoints[winner]
}
