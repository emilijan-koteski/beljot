package bot

import (
	"math"

	"github.com/emilijan/beljot/server/internal/bot/search"
	"github.com/emilijan/beljot/server/internal/game"
)

const DefenderTrumpLeadVeto = 10_000

// SearchTuning holds the phase-3 behaviours layered on the search. The zero value is the phase-2 behaviour, and SeedSalt
// is a lab-only sampler seed offset that production leaves at 0.
type SearchTuning struct {
	DefenderTrumpLeadBonus int
	TenUnderAceBonus       int
	LeadRiskLambda         float64
	TrumpDrawConditional   bool
	RaceObjective          bool
	OpeningSamples         int
	SampleCaps             [9]int
	SeedSalt               uint64
}

// DefaultSearchTuning is the tuning the bot ships with.
func DefaultSearchTuning() SearchTuning {
	return SearchTuning{DefenderTrumpLeadBonus: 10, TenUnderAceBonus: 3, RaceObjective: true}
}

// samplesFor is the deal count at a trick: a non-zero cap in SampleCaps wins over the opening samples and the schedule.
func samplesFor(trick int, t SearchTuning) int {
	if trick >= 1 && trick < len(t.SampleCaps) && t.SampleCaps[trick] != 0 {
		return t.SampleCaps[trick]
	}
	if trick < 3 && t.OpeningSamples > 0 {
		return t.OpeningSamples
	}
	return search.SamplesForTrick(trick)
}

// defenderTrumpLeadConvention reports whether the seat is on lead for the defending team while holding a legal non-trump card.
func defenderTrumpLeadConvention(v View) bool {
	if len(v.CurrentTrick) != 0 || v.TrumpSuit == nil || myTeamCalledTrump(v) {
		return false
	}
	for _, c := range v.LegalCards {
		if c.Suit != *v.TrumpSuit {
			return true
		}
	}
	return false
}

func isPenalisedDefenderTrump(c game.Card, trump game.Suit) bool {
	return c.Suit == trump && c.Rank != game.RankJack && c.Rank != game.Rank9
}

// chooseAdjusted picks the legal card with the highest adjusted mean, the lowest card index winning ties.
func chooseAdjusted(v View, res search.Result, t SearchTuning) (game.Card, bool) {
	var ten *game.Card
	if v.TrumpSuit != nil && t.TenUnderAceBonus != 0 {
		ten = tenUnderAceCandidate(v, v.LegalCards, *v.TrumpSuit)
	}
	penaliseTrump := t.DefenderTrumpLeadBonus > 0 && defenderTrumpLeadConvention(v)
	riskAdjustLead := t.LeadRiskLambda > 0 && len(v.CurrentTrick) == 0
	var best game.Card
	bestIdx, bestScore, found := 0, 0.0, false
	for _, c := range v.LegalCards {
		idx := search.CardIdx(c)
		stats, ok := res.Cards[idx]
		if !ok {
			continue
		}
		score := stats.Mean
		if riskAdjustLead {
			score -= t.LeadRiskLambda * stats.SE
		}
		if ten != nil && c == *ten {
			score += float64(t.TenUnderAceBonus)
		}
		if penaliseTrump && isPenalisedDefenderTrump(c, *v.TrumpSuit) {
			score -= float64(t.DefenderTrumpLeadBonus)
		}
		if !found || score > bestScore || (score == bestScore && idx < bestIdx) {
			best, bestIdx, bestScore, found = c, idx, score, true
		}
	}
	return best, found
}

// topOutstandingTrump is the index of the strongest trump the seat has not placed, or -1 when every trump is in the
// seat's hand, played or in the trick.
func topOutstandingTrump(v View) int {
	in := informationFrom(v)
	outstanding := in.Unseen
	for seat := range 4 {
		if seat != v.Seat {
			outstanding |= in.Known[seat]
		}
	}
	best, bestOrder := -1, -1
	for rank, order := range game.TrumpRankOrder {
		idx := search.CardIdx(game.Card{Suit: *v.TrumpSuit, Rank: rank})
		if outstanding&(1<<uint(idx)) != 0 && order > bestOrder {
			best, bestOrder = idx, order
		}
	}
	return best
}

// topTrumpWithOpponents classes a sampled deal 1 when either opponent of the seat holds the card top, else 0.
func topTrumpWithOpponents(v View, top int) func(hands [4]uint32) int {
	left, right := (v.Seat+1)%4, (v.Seat+3)%4
	return func(hands [4]uint32) int {
		if (hands[left]|hands[right])&(1<<uint(top)) != 0 {
			return 1
		}
		return 0
	}
}

func trumpDrawClass(v View, t SearchTuning) func(hands [4]uint32) int {
	if !t.TrumpDrawConditional || len(v.CurrentTrick) != 0 || v.TrumpSuit == nil {
		return nil
	}
	top := topOutstandingTrump(v)
	if top < 0 {
		return nil
	}
	return topTrumpWithOpponents(v, top)
}

// trumpDrawIsAGuess reports whether the adjusted trump lead is right in only one of the two top-trump classes.
func trumpDrawIsAGuess(v View, adjusted game.Card, res search.Result) bool {
	if len(v.CurrentTrick) != 0 || v.TrumpSuit == nil || adjusted.Suit != *v.TrumpSuit {
		return false
	}
	preferring := 0
	for _, class := range res.Classes {
		if class.Samples == 0 {
			return false
		}
		if classPrefersTrump(v, class) {
			preferring++
		}
	}
	return preferring == 1
}

func classPrefersTrump(v View, class search.ClassStats) bool {
	bestTrump, bestOther := math.Inf(-1), math.Inf(-1)
	for _, c := range v.LegalCards {
		mean, ok := class.Means[search.CardIdx(c)]
		if !ok {
			continue
		}
		if c.Suit == *v.TrumpSuit {
			bestTrump = max(bestTrump, mean)
		} else {
			bestOther = max(bestOther, mean)
		}
	}
	return !math.IsInf(bestTrump, -1) && !math.IsInf(bestOther, -1) && bestTrump > bestOther
}

// deferTrumpDraw hands the decision to the rule layer when the adjusted choice is a trump lead that the top trump's
// whereabouts decide.
func deferTrumpDraw(v View, res search.Result, adjusted game.Card, t SearchTuning) (SearchResult, bool) {
	if !t.TrumpDrawConditional || !trumpDrawIsAGuess(v, adjusted, res) {
		return SearchResult{}, false
	}
	return SearchResult{Card: chooseCard(v), Samples: res.Samples, Margin: res.Margin, SE: res.SE, Deferred: true}, true
}
