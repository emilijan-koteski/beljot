package user

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestLevelForXP checks the level curve (level N needs 50·N² XP) at its boundaries.
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
		{"negative clamps to level 0", -5, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantLevel, LevelForXP(tt.totalXP))
		})
	}
}

// TestLevelProgress checks the XP gained into the current level and the size of its band.
func TestLevelProgress(t *testing.T) {
	tests := []struct {
		name               string
		totalXP            int
		wantLevel          int
		wantXPIntoLevel    int
		wantXPForNextLevel int
	}{
		{"zero xp", 0, 0, 0, 50},
		{"mid level 0", 30, 0, 30, 50},
		{"exactly at level 3 threshold", 450, 3, 0, 350},
		{"mid level 3", 600, 3, 150, 350},
		{"just below level 4", 799, 3, 349, 350},
		{"exactly at level 2 threshold", 200, 2, 0, 250},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, into, forNext := LevelProgress(tt.totalXP)
			assert.Equal(t, tt.wantLevel, level, "level")
			assert.Equal(t, tt.wantXPIntoLevel, into, "xpIntoLevel")
			assert.Equal(t, tt.wantXPForNextLevel, forNext, "xpForNextLevel")
			assert.GreaterOrEqual(t, into, 0)
			assert.Less(t, into, forNext)
			assert.Positive(t, forNext)
		})
	}
}
