package main

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"

	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/season"
)

// The replay core (loading the window, rebuilding each outcome, the scoring
// loop) is season's, shared with the server's season recalculation
// (season/replay.go), so the tuning report and the recalculated rows can never
// disagree. What lives here is the report: usernames, win rates, bot-table
// statistics, the Silver match and the trace.

// loadWindow reads the window's matches (season.LoadWindow) plus a display label
// for every user who sat in one. It only reads; main runs it inside a READ ONLY
// transaction.
func loadWindow(db *gorm.DB, from, to time.Time) ([]match.Match, map[uint]string, error) {
	matches, err := season.LoadWindow(db, from, to)
	if err != nil {
		return nil, nil, err
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

// replay runs the window through season.ReplayMatches and builds the report
// from what it scored: the final totals and ranks are the core's, the per-player
// statistics and the trace are collected match by match by its observer.
func replay(matches []match.Match, names map[uint]string, f season.SPFormula, ladder season.Ladder, traceUser uint) ([]playerResult, replaySummary, error) {
	silver, ok := ladder.Floor(season.TierSilver)
	if !ok {
		return nil, replaySummary{}, errors.New("ladder has no silver tier")
	}

	var sum replaySummary
	stats := map[uint]*playerResult{}

	result := season.ReplayMatches(matches, f, ladder, func(rm season.ReplayedMatch) {
		o := rm.Outcome
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

		for seat, s := range o.Seats {
			if s.IsBot || s.UserID == 0 {
				continue
			}
			r := stats[s.UserID]
			if r == nil {
				r = &playerResult{UserID: s.UserID, Name: names[s.UserID]}
				stats[s.UserID] = r
			}

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
			if r.SilverAt == 0 && rm.After[seat] >= silver {
				r.SilverAt = r.Matches
			}
			if traceUser != 0 && s.UserID == traceUser {
				sum.Trace = append(sum.Trace, traceRow{
					N: r.Matches, MatchID: rm.Match.ID, At: rm.Match.CompletedAt, BotOnly: botOnly, Won: won,
					Surrender: o.Surrender, Abandoned: seat == o.AbandonedSeat,
					Before: rm.Before[seat], Change: rm.After[seat] - rm.Before[seat], After: rm.After[seat],
				})
			}
		}
	})
	sum.Loaded, sum.Scored, sum.Skipped = result.Summary.Loaded, result.Summary.Scored, result.Summary.Skipped

	out := make([]playerResult, 0, len(result.Players))
	for _, p := range result.Players {
		r := stats[p.UserID]
		if r == nil {
			// The core scored a player the observer's seat check did not count:
			// report them with the core's own game count rather than panic.
			r = &playerResult{UserID: p.UserID, Name: names[p.UserID], Matches: p.GamesPlayed}
		}
		r.FinalSP, r.Tier, r.Division = p.SP, p.Tier, p.Division
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
