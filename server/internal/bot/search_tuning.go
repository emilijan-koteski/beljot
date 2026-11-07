package bot

import (
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
	SeedSalt               uint64
}

// DefaultSearchTuning is the tuning the bot ships with.
func DefaultSearchTuning() SearchTuning {
	return SearchTuning{}
}

func samplesFor(trick int, t SearchTuning) int {
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
