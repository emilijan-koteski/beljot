package user

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestLevelForXP pins the placeholder quadratic curve (Level N requires
// 50·N² total XP). Boundaries are the load-bearing cases: a player sits at
// level N from threshold(N) up to threshold(N+1)-1 inclusive. 0 XP is Level 0.
func TestLevelForXP(t *testing.T) {
	tests := []struct {
		name      string
		totalXP   int
		wantLevel int
	}{
		{"zero xp is level 0", 0, 0},
		{"just below level 1 threshold", 49, 0},
		{"exactly level 1 threshold (50)", 50, 1},
		{"just below level 2 threshold", 199, 1},
		{"exactly level 2 threshold (200)", 200, 2},
		{"just below level 3 threshold", 449, 2},
		{"exactly level 3 threshold (450)", 450, 3},
		{"mid level 3", 600, 3},
		{"exactly level 4 threshold (800)", 800, 4},
		{"exactly level 5 threshold (1250)", 1250, 5},
		{"just below level 5 threshold", 1249, 4},
		{"large value (1,000,000 -> 141)", 1_000_000, 141},
		// Defensive: negative never happens (DB CHECK >= 0) but must not panic
		// or yield a negative level.
		{"negative clamps to level 0", -5, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantLevel, LevelForXP(tt.totalXP))
		})
	}
}

// TestLevelProgress checks the per-level progress decomposition used to drive
// the XP bar: xpIntoLevel is the XP earned past the current level's threshold,
// xpForNextLevel is the size of the current level's band. The bar fill is
// xpIntoLevel / xpForNextLevel, always in [0, span).
func TestLevelProgress(t *testing.T) {
	tests := []struct {
		name               string
		totalXP            int
		wantLevel          int
		wantXPIntoLevel    int
		wantXPForNextLevel int
	}{
		// threshold(0)=0, threshold(1)=50 -> band size 50.
		{"zero xp", 0, 0, 0, 50},
		{"mid level 0", 30, 0, 30, 50},
		// threshold(3)=450, threshold(4)=800 -> band size 350.
		{"exactly at level 3 threshold", 450, 3, 0, 350},
		{"mid level 3", 600, 3, 150, 350},
		{"just below level 4", 799, 3, 349, 350},
		// threshold(2)=200, threshold(3)=450 -> band size 250.
		{"exactly at level 2 threshold", 200, 2, 0, 250},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, into, forNext := LevelProgress(tt.totalXP)
			assert.Equal(t, tt.wantLevel, level, "level")
			assert.Equal(t, tt.wantXPIntoLevel, into, "xpIntoLevel")
			assert.Equal(t, tt.wantXPForNextLevel, forNext, "xpForNextLevel")
			// Invariants: fill numerator never exceeds the band, band is positive.
			assert.GreaterOrEqual(t, into, 0)
			assert.Less(t, into, forNext)
			assert.Positive(t, forNext)
		})
	}
}
