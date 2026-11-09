package search

import "github.com/emilijan/beljot/server/internal/game"

// Settlement is what Settle needs to know about a hand: the points and tricks banked before the remainder, both
// teams' declarations and Belote, who called trump, the room's declaration and tie rules, any points hanging from an
// earlier tie and the card points still to play.
type Settlement struct {
	HandPoints           [2]int
	DeclPoints           [2]int
	TricksWon            [2]int
	CallerTeam           int
	DeclarationsTransfer bool
	TieRule              game.TieRule
	Hanging              *game.HangingPoints
	RemainingCardPoints  int
}

// Settle returns the team A minus team B award the engine banks for a hand whose remainder is worth remDiff.
func Settle(s Settlement, remDiff int) int {
	totals := SettleTotals(s, remDiff)
	return totals[game.TeamA] - totals[game.TeamB]
}

// SettleTotals returns what each team banks for a hand whose remainder is worth remDiff. It splits the remainder and
// detects a Capot, then settles through game.SettleHand, so hanging points paid count and a new entry counts for nobody.
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
	tally := game.HandTally{
		CardPoints:           [2]int{s.HandPoints[game.TeamA] + remA, s.HandPoints[game.TeamB] + remB},
		DeclPoints:           s.DeclPoints,
		CallerTeam:           s.CallerTeam,
		Capot:                capotA || capotB,
		DeclarationsTransfer: s.DeclarationsTransfer,
		TieRule:              s.TieRule,
	}
	if capotB {
		tally.CapotTeam = game.TeamB
	}
	return game.SettleHand(tally, s.Hanging).Awarded
}

// CallerMade reports whether the trump caller's team banks more than the opponents for a hand whose remainder is worth
// remDiff, so a fall, an opposing Capot and a Bitola tie all report false.
func CallerMade(s Settlement, remDiff int) bool {
	totals := SettleTotals(s, remDiff)
	return totals[s.CallerTeam] > totals[1-s.CallerTeam]
}
