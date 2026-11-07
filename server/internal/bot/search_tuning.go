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

// chooseAdjusted picks the legal card with the highest adjusted mean, the lowest card index winning ties.
func chooseAdjusted(v View, res search.Result, t SearchTuning) (game.Card, bool) {
	var ten *game.Card
	if v.TrumpSuit != nil && t.TenUnderAceBonus != 0 {
		ten = tenUnderAceCandidate(v, v.LegalCards, *v.TrumpSuit)
	}
	var best game.Card
	bestIdx, bestScore, found := 0, 0.0, false
	for _, c := range v.LegalCards {
		idx := search.CardIdx(c)
		stats, ok := res.Cards[idx]
		if !ok {
			continue
		}
		score := stats.Mean
		if ten != nil && c == *ten {
			score += float64(t.TenUnderAceBonus)
		}
		if !found || score > bestScore || (score == bestScore && idx < bestIdx) {
			best, bestIdx, bestScore, found = c, idx, score, true
		}
	}
	return best, found
}
