package main

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"gorm.io/gorm"

	"github.com/emilijan/beljot/server/internal/game"
	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/season"
)

// loadWindow reads every finished match whose completed_at falls in [from, to),
// in completion order, with its hand rows, plus a display label for every user
// who sat in one. It only reads; main runs it inside a READ ONLY transaction.
//
// The status filter mirrors the one match history uses (match/gorm_repo.go):
// completed rows plus abandoned rows. Abandoned rows with no abandoned_by are
// boot-reconcile placeholders, which award nothing live, so the replay skips
// them too (see outcomeFor) rather than filtering them here, to count them.
func loadWindow(db *gorm.DB, from, to time.Time) ([]match.Match, map[uint]string, error) {
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
		return nil, nil, fmt.Errorf("loading matches: %w", err)
	}

	ids := map[uint]bool{}
	for _, m := range matches {
		for _, p := range seatIDs(m) {
			if p != nil {
				ids[*p] = true
			}
		}
	}
	names := make(map[uint]string, len(ids))
	if len(ids) == 0 {
		return matches, names, nil
	}
	list := make([]uint, 0, len(ids))
	for id := range ids {
		list = append(list, id)
	}

	// Soft-deleted accounts are deliberately INCLUDED: their matches are in the
	// replay (their SP moved their opponents' expected results), so they need a
	// label; they are marked rather than hidden.
	var rows []struct {
		ID       uint
		Username string
		Deleted  bool
	}
	err = db.Raw(`SELECT id, username, deleted_at IS NOT NULL AS deleted FROM users WHERE id IN ?`, list).
		Scan(&rows).Error
	if err != nil {
		return nil, nil, fmt.Errorf("loading usernames: %w", err)
	}
	for _, r := range rows {
		name := r.Username
		if r.Deleted {
			name += " (deleted)"
		}
		names[r.ID] = name
	}
	return matches, names, nil
}

func seatIDs(m match.Match) [4]*uint {
	return [4]*uint{m.Player1ID, m.Player2ID, m.Player3ID, m.Player4ID}
}

func seatBots(m match.Match) [4]bool {
	return [4]bool{m.Player1IsBot, m.Player2IsBot, m.Player3IsBot, m.Player4IsBot}
}

// Reasons a stored match is left out of the replay.
var (
	errReconcilePlaceholder = errors.New("abandoned with no abandoner (boot-reconcile placeholder)")
	errUnknownMode          = errors.New("unknown match mode")
	errAbandonerNotSeated   = errors.New("abandoner is not seated in the match")
)

// outcomeFor rebuilds the outcome the live finalizers would have handed the
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
func outcomeFor(m match.Match) (match.MatchOutcome, error) {
	o := match.MatchOutcome{AbandonedSeat: -1}
	if m.Status == "abandoned" && m.AbandonedBy == nil {
		return o, errReconcilePlaceholder
	}
	// The two stored modes ARE their targets. Anything else is refused rather
	// than defaulted: a replay that silently scored an unknown mode against 1001
	// would tune the constants on a wrong margin.
	target, err := strconv.Atoi(m.MatchMode)
	if err != nil || (target != 501 && target != 1001) {
		return o, fmt.Errorf("%w %q", errUnknownMode, m.MatchMode)
	}
	o.Target = target
	o.TeamScores = [2]int{m.TeamAScore, m.TeamBScore}
	o.WinnerTeam = m.WinnerTeam

	ids, bots := seatIDs(m), seatBots(m)
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
			return o, errAbandonerNotSeated
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

// playerResult is one player's line in the replay report.
type playerResult struct {
	UserID     uint
	Name       string
	Matches    int
	Wins       int
	BotMatches int
	BotWins    int
	FinalSP    int
	Tier       string
	Division   int
	// SilverAt is the player's own match count at which they first reached the
	// Silver floor, or 0 if they never did.
	SilverAt int
}

// replaySummary counts what the replay did with the window.
type replaySummary struct {
	Loaded       int
	Scored       int
	Skipped      map[string]int
	Surrenders   int
	InstantWins  int
	Abandonments int
	CapotMatches int
	BotOnly      int
	// Trace is the traced player's match-by-match record, empty unless a
	// player was traced.
	Trace []traceRow
}

// traceRow is one match of the traced player's replay.
type traceRow struct {
	N         int
	MatchID   uint
	At        time.Time
	BotOnly   bool
	Won       bool
	Surrender bool
	Abandoned bool
	Before    int
	// Change is the APPLIED change, After - Before; at the 0 floor it is smaller
	// than the formula's own number.
	Change int
	After  int
}

// replay runs the window through the formula in order, entirely in memory:
// every player starts the window on 0 SP, each match reads the running totals,
// and each total is floored at 0 exactly as the award path floors it.
func replay(matches []match.Match, names map[uint]string, f season.SPFormula, ladder season.Ladder, traceUser uint) ([]playerResult, replaySummary, error) {
	silver, ok := ladder.Floor(season.TierSilver)
	if !ok {
		return nil, replaySummary{}, errors.New("ladder has no silver tier")
	}

	sum := replaySummary{Loaded: len(matches), Skipped: map[string]int{}}
	sp := map[uint]int{}
	stats := map[uint]*playerResult{}

	for _, m := range matches {
		o, err := outcomeFor(m)
		if err != nil {
			sum.Skipped[skipReason(err)]++
			continue
		}
		changes, err := f.Changes(o, sp)
		if err != nil {
			sum.Skipped[fmt.Sprintf("formula rejected the outcome: %v", err)]++
			continue
		}

		sum.Scored++
		if o.Surrender {
			sum.Surrenders++
		}
		if o.InstantWin {
			sum.InstantWins++
		}
		if o.AbandonedSeat >= 0 {
			sum.Abandonments++
		}
		if o.CapotTeams[0] || o.CapotTeams[1] {
			sum.CapotMatches++
		}
		botOnly := humanCount(o) == 1
		if botOnly {
			sum.BotOnly++
		}

		for _, s := range o.Seats {
			if s.IsBot || s.UserID == 0 {
				continue
			}
			r := stats[s.UserID]
			if r == nil {
				r = &playerResult{UserID: s.UserID, Name: names[s.UserID]}
				stats[s.UserID] = r
			}
			before := sp[s.UserID]
			sp[s.UserID] = season.ApplySPChange(before, changes[s.UserID])

			won := s.Team == o.WinnerTeam
			r.Matches++
			if won {
				r.Wins++
			}
			if botOnly {
				r.BotMatches++
				if won {
					r.BotWins++
				}
			}
			if r.SilverAt == 0 && sp[s.UserID] >= silver {
				r.SilverAt = r.Matches
			}
			if traceUser != 0 && s.UserID == traceUser {
				sum.Trace = append(sum.Trace, traceRow{
					N: r.Matches, MatchID: m.ID, At: m.CompletedAt, BotOnly: botOnly, Won: won,
					Surrender: o.Surrender, Abandoned: o.AbandonedSeat >= 0 && o.Seats[o.AbandonedSeat].UserID == traceUser,
					Before: before, Change: sp[s.UserID] - before, After: sp[s.UserID],
				})
			}
		}
	}

	out := make([]playerResult, 0, len(stats))
	for id, r := range stats {
		r.FinalSP = sp[id]
		r.Tier, r.Division = ladder.Rank(r.FinalSP)
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].FinalSP != out[j].FinalSP {
			return out[i].FinalSP > out[j].FinalSP
		}
		if out[i].Matches != out[j].Matches {
			return out[i].Matches > out[j].Matches
		}
		return out[i].UserID < out[j].UserID
	})
	return out, sum, nil
}

func humanCount(o match.MatchOutcome) int {
	n := 0
	for _, s := range o.Seats {
		if !s.IsBot && s.UserID != 0 {
			n++
		}
	}
	return n
}

func skipReason(err error) string {
	switch {
	case errors.Is(err, errReconcilePlaceholder):
		return errReconcilePlaceholder.Error()
	case errors.Is(err, errUnknownMode):
		return errUnknownMode.Error()
	case errors.Is(err, errAbandonerNotSeated):
		return errAbandonerNotSeated.Error()
	default:
		return err.Error()
	}
}
