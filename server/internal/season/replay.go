package season

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"gorm.io/gorm"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/match"
)

// The REPLAY: stored matches run back through the SP formula in completion
// order, every player starting on 0 SP. Two callers share this one core so they
// cannot disagree about what a stored match was worth:
//
//   - cmd/sptune, the offline tuning command, which replays a window in memory
//     against candidate constants and prints a report (Story 13.4);
//   - RunPendingRecalculations (recalc.go), which replays a season with the
//     live constants and writes the result into its player_seasons rows (the
//     2026 Q3 recalculation, migration 000029).
//
// It reads `matches` and `hand_results` only, never player_seasons: a replayed
// total is a function of the stored matches alone, so replaying the same window
// twice gives the same totals.

// LoadWindow reads every finished match whose completed_at falls in [from, to),
// in completion order (completed_at, then id), with its hand rows in hand order.
//
// The status filter mirrors the one match history uses (match/gorm_repo.go):
// completed rows plus abandoned rows. Abandoned rows with no abandoned_by are
// boot-reconcile placeholders, which award nothing live, so the replay skips
// them too (see OutcomeFor) rather than filtering them here, to count them.
func LoadWindow(db *gorm.DB, from, to time.Time) ([]match.Match, error) {
	var matches []match.Match
	err := db.Model(&match.Match{}).
		Where("completed_at >= ? AND completed_at < ?", from, to).
		Where("status IN ?", []string{"completed", "abandoned"}).
		Preload("Hands", func(db *gorm.DB) *gorm.DB {
			return db.Order("hand_number ASC")
		}).
		Order("completed_at ASC").
		Order("id ASC").
		Find(&matches).Error
	if err != nil {
		return nil, fmt.Errorf("loading matches: %w", err)
	}
	return matches, nil
}

// seatUserIDs returns the four stored seat user columns, seat order. A bot or
// empty seat is nil.
func seatUserIDs(m match.Match) [4]*uint {
	return [4]*uint{m.Player1ID, m.Player2ID, m.Player3ID, m.Player4ID}
}

func seatBots(m match.Match) [4]bool {
	return [4]bool{m.Player1IsBot, m.Player2IsBot, m.Player3IsBot, m.Player4IsBot}
}

// Reasons a stored match is left out of the replay.
var (
	ErrReconcilePlaceholder = errors.New("abandoned with no abandoner (boot-reconcile placeholder)")
	ErrUnknownMode          = errors.New("unknown match mode")
	ErrAbandonerNotSeated   = errors.New("abandoner is not seated in the match")
)

// OutcomeFor rebuilds the outcome the live finalizers would have handed the
// season service, from what the match row stores.
//
// Two facts are not stored and are inferred:
//
//   - SURRENDER: surrendered_by is set AND the winners finished below the
//     target. A surrender accepted on a hand that took the winners over the
//     target finalizes as a target-reached finish, and live scores it as one.
//   - INSTANT WIN: a completed match with no surrender whose winners finished
//     below the target. Nothing else ends a completed match short of it.
//
// An abandonment is scored as live scores it: the seat of abandoned_by is the
// abandoned seat, and the winner is the other team, whatever winner_team says.
// Presence is not stored either, so only the abandoner is marked absent
// (Completed false); every other human seat counts as present.
func OutcomeFor(m match.Match) (match.MatchOutcome, error) {
	o := match.MatchOutcome{AbandonedSeat: -1}
	if m.Status == "abandoned" && m.AbandonedBy == nil {
		return o, ErrReconcilePlaceholder
	}
	// The two stored modes ARE their targets. Anything else is refused rather
	// than defaulted: a replay that silently scored an unknown mode against 1001
	// would score it on a wrong margin.
	target, err := strconv.Atoi(m.MatchMode)
	if err != nil || (target != 501 && target != 1001) {
		return o, fmt.Errorf("%w %q", ErrUnknownMode, m.MatchMode)
	}
	o.Target = target
	o.TeamScores = [2]int{m.TeamAScore, m.TeamBScore}
	o.WinnerTeam = m.WinnerTeam

	ids, bots := seatUserIDs(m), seatBots(m)
	for seat := range o.Seats {
		s := match.OutcomeSeat{Team: game.TeamForSeat(seat), IsBot: bots[seat], Completed: true}
		if ids[seat] != nil && !bots[seat] {
			s.UserID = *ids[seat]
		}
		o.Seats[seat] = s
	}

	if m.Status == "abandoned" {
		for seat, s := range o.Seats {
			if s.UserID != 0 && s.UserID == *m.AbandonedBy {
				o.AbandonedSeat = seat
			}
		}
		if o.AbandonedSeat < 0 {
			return o, ErrAbandonerNotSeated
		}
		o.Seats[o.AbandonedSeat].Completed = false
		o.WinnerTeam = 1 - o.Seats[o.AbandonedSeat].Team
	} else if o.WinnerTeam == 0 || o.WinnerTeam == 1 {
		belowTarget := o.TeamScores[o.WinnerTeam] < target
		o.Surrender = m.SurrenderedBy != nil && belowTarget
		o.InstantWin = m.SurrenderedBy == nil && belowTarget
	}

	for _, h := range m.Hands {
		if h.CapotTeam != nil && (*h.CapotTeam == 0 || *h.CapotTeam == 1) {
			o.CapotTeams[*h.CapotTeam] = true
		}
	}
	return o, nil
}

// SkipReason is the summary key for a match the replay left out: the sentinel's
// message for the known reasons (so every unknown mode counts under one key),
// the error itself otherwise.
func SkipReason(err error) string {
	for _, known := range []error{ErrReconcilePlaceholder, ErrUnknownMode, ErrAbandonerNotSeated} {
		if errors.Is(err, known) {
			return known.Error()
		}
	}
	return err.Error()
}

// ReplayedPlayer is one player's result at the end of a replay.
type ReplayedPlayer struct {
	UserID uint
	// SP is the final total, floored at 0 after every match like the award path.
	SP int
	// Tier and Division are the rank of SP on the replay's ladder; Division is 0
	// for Master and Grandmaster.
	Tier     string
	Division int
	// GamesPlayed counts every scored match the player sat in. GamesCompleted
	// leaves out the matches they abandoned: presence at the terminal end is not
	// stored, so the abandoner is the only seat a stored match can mark absent.
	GamesPlayed    int
	GamesCompleted int
}

// ReplaySummary counts what a replay did with the matches it was given.
type ReplaySummary struct {
	Loaded int
	Scored int
	// Skipped counts the matches left out, by SkipReason.
	Skipped map[string]int
}

// ReplayedMatch is one scored match of a replay, as handed to an observer.
// Before and After are each human seat's total before and after the match
// (After floored at 0), indexed by seat; bot and empty seats hold 0.
type ReplayedMatch struct {
	Match   match.Match
	Outcome match.MatchOutcome
	Before  [4]int
	After   [4]int
}

// Replay is a replay's result: every player who sat in a scored match, in
// ascending user-ID order, plus the summary.
type Replay struct {
	Players []ReplayedPlayer
	Summary ReplaySummary
}

// ReplayMatches runs matches through the formula in the order given, entirely
// in memory: every player starts on 0 SP, each match reads the running totals,
// and each new total is floored at 0 exactly as the award path floors it
// (ApplySPChange). A match OutcomeFor or the formula rejects is skipped and
// counted, and changes nobody, as live it would have awarded nothing.
//
// observe, when not nil, is called once per scored match after its totals are
// applied; it is how cmd/sptune collects its report-only statistics.
func ReplayMatches(matches []match.Match, f SPFormula, l Ladder, observe func(ReplayedMatch)) Replay {
	sum := ReplaySummary{Loaded: len(matches), Skipped: map[string]int{}}
	sp := map[uint]int{}
	players := map[uint]*ReplayedPlayer{}

	for _, m := range matches {
		o, err := OutcomeFor(m)
		if err != nil {
			sum.Skipped[SkipReason(err)]++
			continue
		}
		changes, err := f.Changes(o, sp)
		if err != nil {
			sum.Skipped[fmt.Sprintf("formula rejected the outcome: %v", err)]++
			continue
		}
		sum.Scored++

		step := ReplayedMatch{Match: m, Outcome: o}
		for seat, s := range o.Seats {
			if !isHumanSeat(s) {
				continue
			}
			p := players[s.UserID]
			if p == nil {
				p = &ReplayedPlayer{UserID: s.UserID}
				players[s.UserID] = p
			}
			step.Before[seat] = sp[s.UserID]
			sp[s.UserID] = ApplySPChange(step.Before[seat], changes[s.UserID])
			step.After[seat] = sp[s.UserID]

			p.GamesPlayed++
			if s.Completed {
				p.GamesCompleted++
			}
		}
		if observe != nil {
			observe(step)
		}
	}

	out := make([]ReplayedPlayer, 0, len(players))
	for id, p := range players {
		p.SP = sp[id]
		p.Tier, p.Division = l.Rank(p.SP)
		out = append(out, *p)
	}
	slices.SortFunc(out, func(a, b ReplayedPlayer) int { return cmp.Compare(a.UserID, b.UserID) })
	return Replay{Players: out, Summary: sum}
}

// ReplayWindow loads the matches completed in [from, to) (LoadWindow) and
// replays them (ReplayMatches).
func ReplayWindow(db *gorm.DB, from, to time.Time, f SPFormula, l Ladder) (Replay, error) {
	matches, err := LoadWindow(db, from, to)
	if err != nil {
		return Replay{}, err
	}
	return ReplayMatches(matches, f, l, nil), nil
}
