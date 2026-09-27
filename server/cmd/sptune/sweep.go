package main

import (
	"errors"
	"math"
	"math/rand/v2"
	"sort"

	"github.com/emilijan/beljot/server/internal/match"
	"github.com/emilijan/beljot/server/internal/season"
)

// The synthetic sweep: one human plus a bot partner against two bots, winning
// each match with probability p, from 0 SP. It answers the two tuning targets
// the replay cannot (few real players sustain one win rate for long): where a
// p-player SETTLES, and where they are after the first N matches.
//
// Each match is scored by the real formula. Its margin comes from a template
// drawn from the replay window's own bot-table matches, split by whether the
// human won, so the sweep inherits real scorelines, surrenders and Capots. With
// no window, or with a fixed margin requested, it uses a natural 1001 finish at
// that margin instead.

// sweepSeatUser is the synthetic human's user ID.
const sweepSeatUser uint = 1

// defaultSweepMargin is the fixed margin used when there is no window to sample.
// It is Q3 2026's mean margin over natural 1001 finishes.
const defaultSweepMargin = 0.84

// outcomeTemplate is a bot-table outcome relabelled so the human's team is team
// 0: it keeps the scoreline, target, surrender, instant-win and Capot facts.
type outcomeTemplate struct {
	scores     [2]int
	target     int
	surrender  bool
	instantWin bool
	capot      [2]bool
}

// templatePools splits the window's single-human, non-abandoned outcomes by
// whether the human's team won. Abandonments are left out: the sweep measures a
// win rate, not a quit rate.
func templatePools(matches []match.Match) (won, lost []outcomeTemplate) {
	for _, m := range matches {
		o, err := outcomeFor(m)
		if err != nil || o.AbandonedSeat >= 0 || humanCount(o) != 1 {
			continue
		}
		team := 0
		for _, s := range o.Seats {
			if !s.IsBot && s.UserID != 0 {
				team = s.Team
			}
		}
		t := outcomeTemplate{
			scores:     [2]int{o.TeamScores[team], o.TeamScores[1-team]},
			target:     o.Target,
			surrender:  o.Surrender,
			instantWin: o.InstantWin,
			capot:      [2]bool{o.CapotTeams[team], o.CapotTeams[1-team]},
		}
		if o.WinnerTeam == team {
			won = append(won, t)
		} else {
			lost = append(lost, t)
		}
	}
	return won, lost
}

// fixedTemplate is a natural 1001 finish at margin m, from the human's side.
func fixedTemplate(m float64, humanWon bool) outcomeTemplate {
	const target = 1001
	losePts := int(math.Round(target * (1.5 - m)))
	t := outcomeTemplate{target: target}
	if humanWon {
		t.scores = [2]int{target, losePts}
	} else {
		t.scores = [2]int{losePts, target}
	}
	return t
}

func botTableOutcome(t outcomeTemplate, humanWon bool) match.MatchOutcome {
	winner := 1
	if humanWon {
		winner = 0
	}
	return match.MatchOutcome{
		Seats: [4]match.OutcomeSeat{
			{UserID: sweepSeatUser, Team: 0, Completed: true},
			{IsBot: true, Team: 1},
			{IsBot: true, Team: 0},
			{IsBot: true, Team: 1},
		},
		WinnerTeam:    winner,
		TeamScores:    t.scores,
		Target:        t.target,
		Surrender:     t.surrender,
		InstantWin:    t.instantWin,
		CapotTeams:    t.capot,
		AbandonedSeat: -1,
	}
}

type sweepConfig struct {
	WinRates []float64
	Runs     int
	Horizon  int
	AfterN   int
	Seed     uint64
	// FixedMargin > 0 forces a natural 1001 finish at that margin; otherwise the
	// pools are sampled, falling back to defaultSweepMargin when a pool is empty.
	FixedMargin float64
	Won, Lost   []outcomeTemplate
}

type sweepRow struct {
	P float64
	// AnalyticSettle is gold + 2·S·log10(p·W/((1−p)·L)): where the expected
	// change is zero if wins and losses had the same margin.
	AnalyticSettle float64
	// SimSettle is the mean SP over the second half of the horizon, across runs.
	SimSettle float64
	// AfterN* are the 10th, 50th and 90th percentile SP after AfterN matches.
	AfterNP10, AfterNMedian, AfterNP90 int
	// SilverMedian is the median match count at which a run first reached the
	// Silver floor, over runs that did; SilverWithinN is the share of runs that
	// got there within AfterN matches.
	SilverMedian  int
	SilverReached float64
	SilverWithinN float64
}

func sweep(cfg sweepConfig, f season.SPFormula, ladder season.Ladder) ([]sweepRow, error) {
	silver, ok := ladder.Floor(season.TierSilver)
	if !ok {
		return nil, errors.New("ladder has no silver tier")
	}
	if cfg.Runs <= 0 || cfg.Horizon <= 0 || cfg.AfterN <= 0 || cfg.AfterN > cfg.Horizon {
		return nil, errors.New("sweep needs runs > 0 and 0 < after-n <= horizon")
	}

	pick := func(rng *rand.Rand, humanWon bool) outcomeTemplate {
		pool := cfg.Lost
		if humanWon {
			pool = cfg.Won
		}
		if cfg.FixedMargin > 0 {
			return fixedTemplate(cfg.FixedMargin, humanWon)
		}
		if len(pool) == 0 {
			return fixedTemplate(defaultSweepMargin, humanWon)
		}
		return pool[rng.IntN(len(pool))]
	}

	rows := make([]sweepRow, 0, len(cfg.WinRates))
	for pi, p := range cfg.WinRates {
		rng := rand.New(rand.NewPCG(cfg.Seed, uint64(pi)))
		afterN := make([]int, 0, cfg.Runs)
		var silverAt []int
		withinN := 0
		settleSum, settleCount := 0.0, 0

		for run := 0; run < cfg.Runs; run++ {
			sp := 0
			reached := 0
			for n := 1; n <= cfg.Horizon; n++ {
				won := rng.Float64() < p
				changes, err := f.Changes(botTableOutcome(pick(rng, won), won), map[uint]int{sweepSeatUser: sp})
				if err != nil {
					return nil, err
				}
				sp = season.ApplySPChange(sp, changes[sweepSeatUser])
				if reached == 0 && sp >= silver {
					reached = n
				}
				if n == cfg.AfterN {
					afterN = append(afterN, sp)
				}
				if n > cfg.Horizon/2 {
					settleSum += float64(sp)
					settleCount++
				}
			}
			if reached > 0 {
				silverAt = append(silverAt, reached)
				if reached <= cfg.AfterN {
					withinN++
				}
			}
		}

		sort.Ints(afterN)
		sort.Ints(silverAt)
		row := sweepRow{
			P:              p,
			AnalyticSettle: float64(f.BotSeatSP) + 2*f.Scale*math.Log10(p*f.BaseWin/((1-p)*f.BaseLoss)),
			SimSettle:      settleSum / float64(settleCount),
			AfterNP10:      percentile(afterN, 0.1),
			AfterNMedian:   percentile(afterN, 0.5),
			AfterNP90:      percentile(afterN, 0.9),
			SilverReached:  float64(len(silverAt)) / float64(cfg.Runs),
			SilverWithinN:  float64(withinN) / float64(cfg.Runs),
		}
		if len(silverAt) > 0 {
			row.SilverMedian = percentile(silverAt, 0.5)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// percentile reads the q-quantile of an ascending slice (nearest rank).
func percentile(sorted []int, q float64) int {
	if len(sorted) == 0 {
		return 0
	}
	i := int(math.Ceil(q*float64(len(sorted)))) - 1
	if i < 0 {
		i = 0
	}
	return sorted[i]
}
