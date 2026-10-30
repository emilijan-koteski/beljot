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

// LoadWindow reads every completed or abandoned match with completed_at in [from, to), in completion order, with its hands.
// Boot-reconcile placeholders are kept here and skipped later by OutcomeFor.
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

var (
	ErrReconcilePlaceholder = errors.New("abandoned with no abandoner (boot-reconcile placeholder)")
	ErrUnknownMode          = errors.New("unknown match mode")
	ErrAbandonerNotSeated   = errors.New("abandoner is not seated in the match")
)

// OutcomeFor rebuilds the outcome the live finalizers would have produced from what the match row stores.
// Surrender and instant win are inferred from the scores, and only the abandoner is marked absent.
func OutcomeFor(m match.Match) (match.MatchOutcome, error) {
	o := match.MatchOutcome{AbandonedSeat: -1}
	if m.Status == "abandoned" && m.AbandonedBy == nil {
		return o, ErrReconcilePlaceholder
	}
	target, err := strconv.Atoi(m.MatchMode)
	if err != nil || target != game.MatchTarget(m.MatchMode) {
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
	UserID         uint
	SP             int
	Tier           string
	Division       int
	GamesPlayed    int
	GamesCompleted int
	GamesWon       int
}

// ReplaySummary counts what a replay did with the matches it was given.
type ReplaySummary struct {
	Loaded  int
	Scored  int
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

// ReplayMatches runs matches through the formula in order in memory, flooring totals at 0 like the award path.
// Rejected matches are skipped and counted, and observe, when set, is called after each scored match.
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
			if s.Team == o.WinnerTeam {
				p.GamesWon++
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
