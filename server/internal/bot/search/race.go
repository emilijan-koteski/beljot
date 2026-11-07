package search

import "github.com/emilijan/beljot/server/internal/game"

// RaceWin is the value of a match decided mid-hand, signed for team A. It exceeds any hand remainder (162 + 100).
const RaceWin = 1000

// Race is a stop-at-target match at the root: the target, each team's running total (match score, hand points,
// declarations and Belote) and the trump caller's team, which wins a tie.
type Race struct {
	Target int
	Banked [2]int
	Caller int
}

// RaceDecided applies the engine's stop-at-target rule to the running totals after gained more card points: a total
// at the target decides the match, both there go to the higher total and a tie to the caller.
func RaceDecided(r Race, gained [2]int) (winner int, decided bool) {
	a, b := r.Banked[game.TeamA]+gained[game.TeamA], r.Banked[game.TeamB]+gained[game.TeamB]
	aOver, bOver := a >= r.Target, b >= r.Target
	switch {
	case aOver && bOver && a == b:
		return r.Caller, true
	case aOver && (!bOver || a > b):
		return game.TeamA, true
	case bOver:
		return game.TeamB, true
	}
	return 0, false
}

func raceValue(winner int) int {
	return (1 - 2*winner) * RaceWin
}
