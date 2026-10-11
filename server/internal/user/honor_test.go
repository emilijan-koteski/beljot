package user

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var fixedNow = time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)

// daysBefore returns a timestamp `days` before fixedNow, as a pointer suitable
// for the nullable honor_decayed_at column.
func daysBefore(days float64) *time.Time {
	t := fixedNow.Add(-time.Duration(days * 24 * float64(time.Hour)))
	return &t
}

// TestHonorScore_WorkedExamples checks the honor formula against worked examples with
// no decay applied.
func TestHonorScore_WorkedExamples(t *testing.T) {
	tests := []struct {
		name      string
		completed float64
		abandoned float64
		wantScore int
		wantTier  string
	}{
		{"no history sits at the 80 prior", 0, 0, 80, HonorTierFair},
		{"five clean matches", 5, 0, 90, HonorTierTrusted},
		{"twenty clean matches", 20, 0, 96, HonorTierExemplary},
		{"fifty clean matches", 50, 0, 98, HonorTierExemplary},
		{"one abandon in twenty drops Trusted to Fair", 20, 1, 83, HonorTierFair},
		{"two abandons in twenty", 20, 2, 73, HonorTierFair},
		{"four abandons in twenty", 20, 4, 59, HonorTierUnreliable},
		{"half the matches abandoned", 10, 10, 25, HonorTierProblematic},
		{"a year-old abandon has decayed away", 20, 0.06, 95, HonorTierExemplary},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HonorScore(tt.completed, tt.abandoned, nil, fixedNow)
			assert.Equal(t, tt.wantScore, got)
			assert.Equal(t, tt.wantTier, HonorTier(got))
		})
	}
}

// TestHonorScore_Defensive covers inputs the DB CHECK constraints forbid but
// that must still degrade gracefully rather than panic or emit a nonsense score.
func TestHonorScore_Defensive(t *testing.T) {
	tests := []struct {
		name      string
		completed float64
		abandoned float64
		want      int
	}{
		{"negative completed weight clamps to the prior", -50, 0, 80},
		{"negative abandoned weight clamps to the prior", 0, -50, 80},
		{"both negative clamps to the prior", -10, -10, 80},
		{"negative completed with real abandons still penalises", -1, 2, 31},
		{"NaN completed clamps to the prior", math.NaN(), 0, 80},
		{"NaN abandoned clamps to the prior", 0, math.NaN(), 80},
		{"+Inf completed clamps to the prior", math.Inf(1), 0, 80},
		{"+Inf abandoned clamps to the prior", 0, math.Inf(1), 80},
		{"both +Inf clamp to the prior", math.Inf(1), math.Inf(1), 80},
		{"-Inf completed clamps to the prior", math.Inf(-1), 0, 80},
		{"enormous completed weight saturates below 100", 1e9, 0, 100},
		{"enormous abandoned weight floors at 0", 0, 1e9, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HonorScore(tt.completed, tt.abandoned, nil, fixedNow)
			assert.Equal(t, tt.want, got)
			assert.GreaterOrEqual(t, got, 0)
			assert.LessOrEqual(t, got, 100)
		})
	}
}

// TestDecayFactor pins the half-life curve and both no-op cases.
func TestDecayFactor(t *testing.T) {
	tests := []struct {
		name      string
		decayedAt *time.Time
		want      float64
	}{
		{"nil decayedAt (never decayed) is exactly 1.0", nil, 1.0},
		{"zero elapsed is 1.0", daysBefore(0), 1.0},
		{"one half-life halves the weight", daysBefore(90), 0.5},
		{"two half-lives quarter the weight", daysBefore(180), 0.25},
		{"one year is ~6% of the original weight", daysBefore(365), 0.0601},
		{"45 days is 1/sqrt(2)", daysBefore(45), 0.7071},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.InDelta(t, tt.want, DecayFactor(tt.decayedAt, fixedNow), 0.0001)
		})
	}

	t.Run("a future decayedAt (clock skew) never inflates the weight", func(t *testing.T) {
		future := fixedNow.Add(30 * 24 * time.Hour)
		assert.Equal(t, 1.0, DecayFactor(&future, fixedNow))
	})
}

// TestHonorScore_Decay checks decay pulls every inactive player toward the 80 prior.
func TestHonorScore_Decay(t *testing.T) {
	badFresh := HonorScore(10, 10, daysBefore(0), fixedNow)
	require.Equal(t, 25, badFresh)
	require.Equal(t, HonorTierProblematic, HonorTier(badFresh))

	badAYearOn := HonorScore(10, 10, daysBefore(365), fixedNow)
	assert.Equal(t, 57, badAYearOn)
	assert.Equal(t, HonorTierUnreliable, HonorTier(badAYearOn))
	assert.Greater(t, badAYearOn, badFresh,
		"a year of no activity must forgive old abandonments")

	goodFresh := HonorScore(20, 1, daysBefore(0), fixedNow)
	goodAYearOn := HonorScore(20, 1, daysBefore(365), fixedNow)
	assert.Equal(t, 83, goodFresh)
	assert.Equal(t, 81, goodAYearOn)
	assert.Less(t, goodAYearOn, goodFresh,
		"inactivity must drift an above-prior player back toward 80")

	recovered := HonorScore(20, 0.06, nil, fixedNow)
	assert.Equal(t, 95, recovered)
	assert.Equal(t, HonorTierExemplary, HonorTier(recovered),
		"a 365-day-old abandonment must not keep a clean player out of Exemplary")
}

// TestHonorScore_DecayIsExact checks decay-forward-then-add equals summing per-match
// weights from scratch.
func TestHonorScore_DecayIsExact(t *testing.T) {
	t180 := fixedNow.Add(-180 * 24 * time.Hour)
	t90 := fixedNow.Add(-90 * 24 * time.Hour)

	weight := 0.0
	weight = weight*DecayFactor(nil, t180) + 1.0
	weight = weight*DecayFactor(&t180, t90) + 1.0
	weight = weight*DecayFactor(&t90, fixedNow) + 1.0

	fromScratch := math.Pow(0.5, 2) + math.Pow(0.5, 1) + 1.0

	assert.InDelta(t, fromScratch, weight, 1e-12)
	assert.InDelta(t, 1.75, weight, 1e-12)
}

// TestHonorTier_Boundaries pins every band edge. These are the values a retune
// is most likely to break silently.
func TestHonorTier_Boundaries(t *testing.T) {
	tests := []struct {
		score int
		want  string
	}{
		{100, HonorTierExemplary},
		{95, HonorTierExemplary},
		{94, HonorTierTrusted},
		{85, HonorTierTrusted},
		{84, HonorTierFair},
		{80, HonorTierFair},
		{70, HonorTierFair},
		{69, HonorTierUnreliable},
		{50, HonorTierUnreliable},
		{49, HonorTierProblematic},
		{0, HonorTierProblematic},
		{101, HonorTierExemplary},
		{-1, HonorTierProblematic},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, HonorTier(tt.score))
		})
	}
}

// TestIsNewPlayer checks the New Player floor of 5 raw completed plus abandoned matches.
func TestIsNewPlayer(t *testing.T) {
	tests := []struct {
		name           string
		completedTotal int64
		abandonedTotal int64
		want           bool
	}{
		{"no history at all", 0, 0, true},
		{"one completed", 1, 0, true},
		{"four completed", 4, 0, true},
		{"five completed clears the floor", 5, 0, false},
		{"six completed", 6, 0, false},
		{"a veteran", 500, 0, false},
		{"four completed plus one abandoned clears the floor", 4, 1, false},
		{"zero completed but twenty abandoned is NOT a new player", 0, 20, false},
		{"zero completed and five abandoned clears the floor exactly", 0, 5, false},
		{"zero completed and four abandoned is still suppressed", 0, 4, true},
		{"two and two is still under the floor", 2, 2, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsNewPlayer(tt.completedTotal, tt.abandonedTotal))
		})
	}

	t.Run("a returning veteran is never relabelled New Player", func(t *testing.T) {
		assert.False(t, IsNewPlayer(500, 0))
		assert.Less(t, HonorScore(500, 0, daysBefore(730), fixedNow), 100)
	})

	t.Run("a pure abandoner's real score is visible, not suppressed", func(t *testing.T) {
		assert.False(t, IsNewPlayer(0, 20))
		assert.Equal(t, 5, HonorScore(0, 20, nil, fixedNow))
		assert.Equal(t, HonorTierProblematic, HonorTier(HonorScore(0, 20, nil, fixedNow)))
	})
}

// TestHonorTrendWindowed pins the two-window comparison introduced by the
// 2026-07-29 code review, including the equal-sample-size precondition.
func TestHonorTrendWindowed(t *testing.T) {
	tests := []struct {
		name                             string
		recentCompleted, recentAbandoned int
		priorCompleted, priorAbandoned   int
		wantDelta                        int
		wantDirection                    string
	}{
		{
			name: "improving", recentCompleted: 20, priorCompleted: 18, priorAbandoned: 2,
			wantDelta: 25, wantDirection: HonorTrendUp,
		},
		{
			name: "slipping", recentCompleted: 18, recentAbandoned: 2, priorCompleted: 20,
			wantDelta: -25, wantDirection: HonorTrendDown,
		},
		{
			name: "a spotless record in both windows is flat", recentCompleted: 20, priorCompleted: 20,
			wantDelta: 0, wantDirection: HonorTrendFlat,
		},
		{
			name: "equally bad in both windows is flat", recentCompleted: 10, recentAbandoned: 10,
			priorCompleted: 10, priorAbandoned: 10,
			wantDelta: 0, wantDirection: HonorTrendFlat,
		},
		{
			name: "a one-point move stays inside the dead band", recentCompleted: 400,
			priorCompleted: 399, priorAbandoned: 1,
			wantDelta: 1, wantDirection: HonorTrendFlat,
		},
		{
			name: "no history at all is flat", wantDelta: 0, wantDirection: HonorTrendFlat,
		},
		{
			name: "a partial prior window is flat", recentCompleted: 20, priorCompleted: 5,
			wantDelta: 0, wantDirection: HonorTrendFlat,
		},
		{
			name: "an empty prior window is flat", recentCompleted: 20,
			wantDelta: 0, wantDirection: HonorTrendFlat,
		},
		{
			name: "an empty recent window is flat", priorCompleted: 20,
			wantDelta: 0, wantDirection: HonorTrendFlat,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delta, direction := HonorTrendWindowed(
				tt.recentCompleted, tt.recentAbandoned,
				tt.priorCompleted, tt.priorAbandoned,
				fixedNow,
			)
			assert.Equal(t, tt.wantDelta, delta)
			assert.Equal(t, tt.wantDirection, direction)
		})
	}
}

// TestHonorScoreForCounts covers the trend-window entry point: raw counts, no
// decay (the 20-match window IS the recency mechanism).
func TestHonorScoreForCounts(t *testing.T) {
	assert.Equal(t, 80, HonorScoreForCounts(0, 0, fixedNow))
	assert.Equal(t, 96, HonorScoreForCounts(20, 0, fixedNow))
	assert.Equal(t, 83, HonorScoreForCounts(20, 1, fixedNow))
	assert.Equal(t, 59, HonorScoreForCounts(20, 4, fixedNow))
}

// TestHonorTrend pins the +/-2 dead band around flat.
func TestHonorTrend(t *testing.T) {
	tests := []struct {
		name          string
		window        int
		lifetime      int
		wantDelta     int
		wantDirection string
	}{
		{"identical scores are flat", 90, 90, 0, HonorTrendFlat},
		{"+1 is inside the dead band", 91, 90, 1, HonorTrendFlat},
		{"-1 is inside the dead band", 89, 90, -1, HonorTrendFlat},
		{"+2 is the up threshold", 92, 90, 2, HonorTrendUp},
		{"-2 is the down threshold", 88, 90, -2, HonorTrendDown},
		{"a big improvement trends up", 96, 60, 36, HonorTrendUp},
		{"a big regression trends down", 40, 90, -50, HonorTrendDown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delta, direction := HonorTrend(tt.window, tt.lifetime)
			assert.Equal(t, tt.wantDelta, delta)
			assert.Equal(t, tt.wantDirection, direction)
		})
	}
}

// TestHonorTrendWindow pins the exported window size so the repository query and
// the math cannot drift apart.
func TestHonorTrendWindow(t *testing.T) {
	assert.Equal(t, 20, HonorTrendWindow())
}
