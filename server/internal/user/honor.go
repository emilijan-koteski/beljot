package user

import (
	"math"
	"time"
)

const (
	honorHalfLifeDays        = 90.0
	honorAbandonPenalty      = 4.0
	honorPriorCompleted      = 4.0
	honorPriorAbandoned      = 1.0
	honorNewPlayerMinMatches = 5
	honorTrendWindow         = 20
	honorTrendThreshold      = 2
	honorEventWeight         = 1.0
	honorMaxClockSkew        = 24 * time.Hour
)

const (
	HonorTierExemplary   = "exemplary"
	HonorTierTrusted     = "trusted"
	HonorTierFair        = "fair"
	HonorTierUnreliable  = "unreliable"
	HonorTierProblematic = "problematic"
)

const (
	HonorTrendUp   = "up"
	HonorTrendFlat = "flat"
	HonorTrendDown = "down"
)

// HonorEvent is one finished match's honor contribution for one player.
type HonorEvent struct {
	Abandoned bool
}

// HonorSnapshot is one player's honor state immediately after a write. It is
// what event:honor_updated and the profile DTO render, so it carries the
// AUTHORITATIVE recomputed score rather than the lagging honor_score column.
type HonorSnapshot struct {
	Score          int
	Tier           string
	CompletedTotal int64
	AbandonedTotal int64
	IsNewPlayer    bool
	Level          int
}

// NewHonorSnapshot builds the renderable honor state from a user's stored columns,
// recomputing the score at now.
func NewHonorSnapshot(completedWeight, abandonedWeight float64, decayedAt *time.Time, completedTotal, abandonedTotal int64, now time.Time) HonorSnapshot {
	score := HonorScore(completedWeight, abandonedWeight, decayedAt, now)
	return HonorSnapshot{
		Score:          score,
		Tier:           HonorTier(score),
		CompletedTotal: completedTotal,
		AbandonedTotal: abandonedTotal,
		IsNewPlayer:    IsNewPlayer(completedTotal, abandonedTotal),
	}
}

// DecayFactor returns 0.5^(elapsedDays/honorHalfLifeDays), the multiplier that ages stored
// weights from decayedAt to now. It returns 1.0 when decayedAt is nil or after now.
func DecayFactor(decayedAt *time.Time, now time.Time) float64 {
	if decayedAt == nil {
		return 1.0
	}
	elapsed := now.Sub(*decayedAt)
	if elapsed <= 0 {
		return 1.0
	}
	days := elapsed.Hours() / 24.0
	return math.Pow(0.5, days/honorHalfLifeDays)
}

// HonorScore recomputes the authoritative honor score from the decayed stored weights,
// round(100·(C+4)/(C+4+4·A+1)) clamped to [0,100].
func HonorScore(completedWeight, abandonedWeight float64, decayedAt *time.Time, now time.Time) int {
	f := DecayFactor(decayedAt, now)

	c := completedWeight * f
	a := abandonedWeight * f
	if c < 0 || math.IsNaN(c) || math.IsInf(c, 0) {
		c = 0
	}
	if a < 0 || math.IsNaN(a) || math.IsInf(a, 0) {
		a = 0
	}

	numerator := 100 * (c + honorPriorCompleted)
	denominator := c + honorPriorCompleted + honorAbandonPenalty*a + honorPriorAbandoned
	score := int(math.Round(numerator / denominator))

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score
}

// HonorScoreForCounts computes honor from raw undecayed match counts. Used for
// the recent-window trend, where the 20-match window IS the recency mechanism so
// no additional decay applies (Story 9.7 "Trend").
func HonorScoreForCounts(completed, abandoned int, now time.Time) int {
	return HonorScore(float64(completed), float64(abandoned), nil, now)
}

// HonorTier buckets a score into one of the five tier tokens.
func HonorTier(score int) string {
	switch {
	case score >= 95:
		return HonorTierExemplary
	case score >= 85:
		return HonorTierTrusted
	case score >= 70:
		return HonorTierFair
	case score >= 50:
		return HonorTierUnreliable
	default:
		return HonorTierProblematic
	}
}

// IsNewPlayer reports whether fewer than honorNewPlayerMinMatches raw matches were completed
// or abandoned, so the client shows a New Player chip.
func IsNewPlayer(completedTotal, abandonedTotal int64) bool {
	return completedTotal+abandonedTotal < honorNewPlayerMinMatches
}

// HonorTrendWindow exposes how many recent matches the trend query should read,
// so the caller and the math cannot disagree about the window size.
func HonorTrendWindow() int {
	return honorTrendWindow
}

// HonorTrend returns the signed delta between two scores from equal-size samples and its
// direction, treating small deltas as flat.
func HonorTrend(recentScore, baselineScore int) (delta int, direction string) {
	delta = recentScore - baselineScore
	switch {
	case delta >= honorTrendThreshold:
		return delta, HonorTrendUp
	case delta <= -honorTrendThreshold:
		return delta, HonorTrendDown
	default:
		return delta, HonorTrendFlat
	}
}

// HonorTrendWindowed compares the newest window of matches against the window before it.
// It returns flat unless both windows hold the same number of matches.
func HonorTrendWindowed(recentCompleted, recentAbandoned, priorCompleted, priorAbandoned int, now time.Time) (delta int, direction string) {
	recentTotal := recentCompleted + recentAbandoned
	priorTotal := priorCompleted + priorAbandoned
	if recentTotal == 0 || recentTotal != priorTotal {
		return 0, HonorTrendFlat
	}
	return HonorTrend(
		HonorScoreForCounts(recentCompleted, recentAbandoned, now),
		HonorScoreForCounts(priorCompleted, priorAbandoned, now),
	)
}
