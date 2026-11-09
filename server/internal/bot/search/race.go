package search

import "github.com/emilijan/beljot/server/internal/game"

const (
	RaceWin     = 1000
	beloteBonus = 20
)

// Race is a match the root hand may decide: target, running totals, match scores, the caller's team (it wins a tie) and
// the Belote BeloteSeat announces on the first of BeloteCards (0 for none); DecidingHand checks only the hand's end.
// A decided match is worth ±RaceWin for team A, above any remainder (152 card points and a 100 capot).
type Race struct {
	Target       int
	Banked       [2]int
	Scores       [2]int
	Caller       int
	BeloteSeat   int
	BeloteCards  uint32
	DecidingHand bool
}

// RaceDecided applies the engine's stop-at-target rule to the running totals after gained more points: a total at
// the target decides the match, both there go to the higher total and a tie to the caller.
func RaceDecided(r Race, gained [2]int) (winner int, decided bool) {
	return r.matchWinner(r.Banked, gained)
}

// HandEndDecided applies the engine's rule at the hand's end to the match scores plus each team's settled hand total:
// a total at the target decides the match, both there go to the higher total and a tie to the caller.
func HandEndDecided(r Race, settled [2]int) (winner int, decided bool) {
	return r.matchWinner(r.Scores, settled)
}

func (r *Race) matchWinner(base, added [2]int) (winner int, decided bool) {
	a, b := base[game.TeamA]+added[game.TeamA], base[game.TeamB]+added[game.TeamB]
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

// announcesBelote reports that seat, left holding rest, just played the first card of the Belote pair, which is when
// the engine credits the Belote and checks the target.
func (r *Race) announcesBelote(seat, card int, rest uint32) bool {
	return seat == r.BeloteSeat && r.BeloteCards&(1<<uint(card)) != 0 && rest&r.BeloteCards != 0
}

// totals is the points gained since the root plus the Belote once its holder no longer holds the whole pair.
func (r *Race) totals(gained [2]int, hands [4]uint32) [2]int {
	if r.BeloteCards != 0 && hands[r.BeloteSeat]&r.BeloteCards != r.BeloteCards {
		gained[game.TeamForSeat(r.BeloteSeat)] += beloteBonus
	}
	return gained
}

func raceValue(winner int) int {
	return (1 - 2*winner) * RaceWin
}
