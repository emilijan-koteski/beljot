package season

import (
	"fmt"
	"math"

	"github.com/emilijan/beljot/server/internal/match"
)

// The competitive Season Points formula (Story 13.4, canonical statement in
// sprint-change-proposal-2026-09-26 §4). Per match, for every human seat, with
// both teammates getting the same change:
//
//	winners  + W · m · 2·(1 − E_winner)
//	losers   − L · m · 2·E_loser
//	         rounded half away from zero, then + capot bonus once per match to
//	         each human of a team that made at least one Capot
//
//	m  = 0.5 + (winner points − loser points) / target, clamped to [0.5, 1.5];
//	     on a surrender the winners' points count as the target, and an instant
//	     win is 1.5
//	E  = 1 / (1 + 10^((opponent avg − own avg) / S)), the expected result; a
//	     team's average is the mean of its two seats' current SP, and a bot seat
//	     counts as the Gold floor
//
// Abandonment (a seat's reconnect window expired) is scored as a surrender by
// the abandoning team at that moment, except that the abandoner takes a fixed
// 2 × the worst possible loss (L × 1.5 × 2) with no Capot bonus, and their
// teammate takes half of their surrender loss, plus their team's Capot bonus.
//
// W > L so a player who wins half their matches against equal opponents creeps
// up over a season instead of sitting in Iron. The multiplier 2·(1 − E) is what
// stops a strong player from farming weak tables: at the 85 % settle point it
// is about 0.2×, well below the 0.5× floor a linear gap could not get under.
//
// Every term is a named const so a retune is a one-place change; cmd/sptune
// replays stored matches against candidate values before any are written here.
const (
	// spBaseWin is W, the win scale.
	spBaseWin = 30
	// spBaseLoss is L, the loss scale. Kept below spBaseWin (W/L = 1.5).
	spBaseLoss = 20
	// spEloScale is S, the SP gap at which the stronger team is expected to
	// win 10 matches in 11.
	spEloScale = 385
	// spCapotBonus is the flat bonus, once per match, for a team that made at
	// least one Capot. Added after scaling, so it is never shrunk by E.
	spCapotBonus = 5
	// spBotSeatSP is what a bot seat counts as in a team's average: the Gold
	// floor, so the bot value moves with the ladder.
	spBotSeatSP = floorGold

	// The margin clamp. An instant win is scored at spMarginMax.
	spMarginMin = 0.5
	spMarginMax = 1.5

	// spAbandonMultiple is how many worst possible losses an abandoner pays.
	spAbandonMultiple = 2
)

// SPFormula is the formula's tunable constants as one value. The live formula is
// DefaultSPFormula(); the type exists so the offline tuning command can replay
// candidates through the same arithmetic the award path runs.
type SPFormula struct {
	BaseWin    float64
	BaseLoss   float64
	Scale      float64
	BotSeatSP  int
	CapotBonus int
}

// DefaultSPFormula returns the live constants.
func DefaultSPFormula() SPFormula {
	return SPFormula{
		BaseWin:    spBaseWin,
		BaseLoss:   spBaseLoss,
		Scale:      spEloScale,
		BotSeatSP:  spBotSeatSP,
		CapotBonus: spCapotBonus,
	}
}

// ComputeSPChanges scores one finished match with the live constants. See
// SPFormula.Changes.
func ComputeSPChanges(outcome match.MatchOutcome, currentSP map[uint]int) (map[uint]int, error) {
	return DefaultSPFormula().Changes(outcome, currentSP)
}

// ApplySPChange returns the new season total after a formula change. Season SP
// never drops below 0, so the APPLIED change can be smaller than the formula's:
// a player on 10 SP whose loss computes to -18 ends on 0, an applied -10.
func ApplySPChange(previous, change int) int {
	if total := previous + change; total > 0 {
		return total
	}
	return 0
}

// AbandonPenalty is the fixed change for the seat whose reconnect window
// expired: minus spAbandonMultiple worst possible losses, where the worst loss
// is L at the maximum margin against a certain win (E = 1).
func (f SPFormula) AbandonPenalty() int {
	return -spAbandonMultiple * int(math.Round(f.BaseLoss*spMarginMax*2))
}

// Changes returns the formula's SP change for every human seat of the match,
// keyed by user ID, BEFORE the 0 floor (see ApplySPChange). Pure: it reads only
// its arguments.
//
// currentSP holds each seated human's current-season total; a missing entry is
// 0 SP, the same as a player with no row yet. Bot seats (and any seat with no
// user) count as BotSeatSP in their team's average and get no entry.
//
// The presence flag (OutcomeSeat.Completed) is deliberately not read: every
// human seat is scored by its team's result whatever its presence, and only the
// abandoned seat takes the abandonment penalty.
//
// A malformed outcome is an error rather than a guess, because the caller is a
// match finalizer that must degrade (skip SP, log) instead of writing a wrong
// change or panicking.
func (f SPFormula) Changes(outcome match.MatchOutcome, currentSP map[uint]int) (map[uint]int, error) {
	if err := validateOutcome(outcome); err != nil {
		return nil, err
	}
	if f.Scale <= 0 {
		return nil, fmt.Errorf("sp formula: scale must be positive, got %v", f.Scale)
	}

	var teamAvg [2]float64
	for _, s := range outcome.Seats {
		sp := f.BotSeatSP
		if isHumanSeat(s) {
			sp = currentSP[s.UserID]
		}
		teamAvg[s.Team] += float64(sp) / 2
	}

	winner := outcome.WinnerTeam
	loser := 1 - winner
	// E_winner and E_loser sum to 1, so one exponent serves both.
	eWinner := 1 / (1 + math.Pow(10, (teamAvg[loser]-teamAvg[winner])/f.Scale))
	eLoser := 1 - eWinner

	m := margin(outcome)
	win := math.Round(f.BaseWin * m * 2 * (1 - eWinner))
	lossRaw := -f.BaseLoss * m * 2 * eLoser
	loss := math.Round(lossRaw)
	// Half of the surrender loss, rounded once: halving an already-rounded loss
	// would round twice.
	partnerLoss := math.Round(lossRaw / 2)

	out := make(map[uint]int, len(outcome.Seats))
	for seat, s := range outcome.Seats {
		if !isHumanSeat(s) {
			continue
		}
		capot := 0
		if outcome.CapotTeams[s.Team] {
			capot = f.CapotBonus
		}
		switch {
		case seat == outcome.AbandonedSeat:
			out[s.UserID] = f.AbandonPenalty()
		case s.Team == winner:
			out[s.UserID] = int(win) + capot
		case outcome.AbandonedSeat >= 0:
			out[s.UserID] = int(partnerLoss) + capot
		default:
			out[s.UserID] = int(loss) + capot
		}
	}
	return out, nil
}

// margin is m: how decisively the match was won, relative to its target.
//
// A surrender and an abandonment are both scored as if the winners had reached
// the target, so the losers' current points are what shrink the margin.
func margin(o match.MatchOutcome) float64 {
	if o.InstantWin {
		return spMarginMax
	}
	winner := o.WinnerTeam
	winPts := float64(o.TeamScores[winner])
	if o.Surrender || o.AbandonedSeat >= 0 {
		winPts = float64(o.Target)
	}
	m := spMarginMin + (winPts-float64(o.TeamScores[1-winner]))/float64(o.Target)
	return math.Min(spMarginMax, math.Max(spMarginMin, m))
}

func isHumanSeat(s match.OutcomeSeat) bool {
	return !s.IsBot && s.UserID != 0
}

func validateOutcome(o match.MatchOutcome) error {
	if o.WinnerTeam != 0 && o.WinnerTeam != 1 {
		return fmt.Errorf("sp formula: winner team must be 0 or 1, got %d", o.WinnerTeam)
	}
	if o.Target <= 0 {
		return fmt.Errorf("sp formula: match target must be positive, got %d", o.Target)
	}
	if o.AbandonedSeat < -1 || o.AbandonedSeat >= len(o.Seats) {
		return fmt.Errorf("sp formula: abandoned seat out of range: %d", o.AbandonedSeat)
	}
	var perTeam [2]int
	for seat, s := range o.Seats {
		if s.Team != 0 && s.Team != 1 {
			return fmt.Errorf("sp formula: seat %d has team %d", seat, s.Team)
		}
		perTeam[s.Team]++
	}
	if perTeam[0] != 2 || perTeam[1] != 2 {
		return fmt.Errorf("sp formula: teams must have two seats each, got %d and %d", perTeam[0], perTeam[1])
	}
	if o.AbandonedSeat >= 0 && o.Seats[o.AbandonedSeat].Team == o.WinnerTeam {
		return fmt.Errorf("sp formula: abandoned seat %d is on the winning team", o.AbandonedSeat)
	}
	return nil
}
