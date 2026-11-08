package botlab

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/emilijan/beljot/server/internal/bot"
	"github.com/emilijan/beljot/server/internal/game"
)

const noDeadline = 24 * time.Hour

// Counts tallies what a search policy did over a run.
type Counts struct{ Applied, Decided, Deferred, DefenderTrumpLeads int64 }

// Counting is implemented by policies that report Counts.
type Counting interface{ Counts() Counts }

// ShippedBot plays the production search path (bot.SearchCardWith then bot.DecideWith) under a tuning.
type ShippedBot struct {
	FromTrick int
	Tuning    bot.SearchTuning
	Deadline  time.Duration
	Flags     Flags

	applied, decided, deferred, defenderTrumpLeads atomic.Int64
}

// NewShippedBot builds a ShippedBot whose tuning comes from the flags and whose deadline is unlimited.
func NewShippedBot(fromTrick int, flags Flags) *ShippedBot {
	return &ShippedBot{FromTrick: fromTrick, Tuning: TuningFromFlags(flags), Flags: flags}
}

func (s *ShippedBot) Name() string {
	return fmt.Sprintf("shipped:%d[%s]", s.FromTrick, s.Flags.Text)
}

func (s *ShippedBot) Counts() Counts {
	return Counts{
		Applied:            s.applied.Load(),
		Decided:            s.decided.Load(),
		Deferred:           s.deferred.Load(),
		DefenderTrumpLeads: s.defenderTrumpLeads.Load(),
	}
}

func (s *ShippedBot) Decide(v View) game.Action {
	bv := toBotView(v)
	if s.Flags.ClearsBounds() {
		bv.RankBounds = bot.NoRankBounds()
	}
	if !bot.SearchAppliesWith(bv, s.FromTrick, s.Tuning) {
		return bot.Decide(bv)
	}
	s.applied.Add(1)
	wait := s.Deadline
	if wait <= 0 {
		wait = noDeadline
	}
	res, ok := bot.SearchCardWith(bv, time.Now().Add(wait), s.Tuning)
	if !ok {
		return bot.Decide(bv)
	}
	if res.Deferred {
		s.deferred.Add(1)
		return bot.Decide(bv)
	}
	s.decided.Add(1)
	if isDefenderTrumpLead(bv, res.Card) {
		s.defenderTrumpLeads.Add(1)
	}
	return bot.DecideWith(bv, &res)
}

func isDefenderTrumpLead(v bot.View, card game.Card) bool {
	if len(v.CurrentTrick) != 0 || v.TrumpSuit == nil || v.TrumpCallerSeat == nil || card.Suit != *v.TrumpSuit {
		return false
	}
	if v.Seat%2 == *v.TrumpCallerSeat%2 || card.Rank == game.RankJack || card.Rank == game.Rank9 {
		return false
	}
	for _, c := range v.Hand {
		if c.Suit != *v.TrumpSuit {
			return true
		}
	}
	return false
}

// ToBotView converts the lab view into the production bot's view.
func ToBotView(v View) bot.View { return toBotView(v) }
