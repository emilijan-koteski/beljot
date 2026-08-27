package season_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/season"
)

// The ladder, restated as LITERALS. Deliberately not derived from
// season.TierFloor: re-deriving the expectation from the same table the
// implementation reads would make these tests pass for any table, including an
// inverted one (the trap the Story 9.7 review pass 2 caught in the honor tests).
var ladder = []struct {
	tier  string
	floor int
	band  int // size of this tier's band; 0 at the top
}{
	{"iron", 0, 500},
	{"bronze", 500, 1000},
	{"silver", 1500, 1500},
	{"gold", 3000, 2500},
	{"platinum", 5500, 3000},
	{"diamond", 8500, 4000},
	{"master", 12500, 5500},
	{"grandmaster", 18000, 0},
}

func TestSeasonTiers_OrderAndIsolation(t *testing.T) {
	assert.Equal(t,
		[]string{"iron", "bronze", "silver", "gold", "platinum", "diamond", "master", "grandmaster"},
		season.SeasonTiers(),
		"eight tiers, lowest first")

	// A caller that mutates the returned slice must not corrupt the ladder.
	got := season.SeasonTiers()
	got[0] = "tampered"
	assert.Equal(t, "iron", season.SeasonTiers()[0], "SeasonTiers must hand out a fresh slice")
}

func TestTierFloor(t *testing.T) {
	for _, l := range ladder {
		floor, ok := season.TierFloor(l.tier)
		require.True(t, ok, "%s must be a known tier", l.tier)
		assert.Equal(t, l.floor, floor, "%s floor", l.tier)
	}

	_, ok := season.TierFloor("mythic")
	assert.False(t, ok, "an unknown token reports not-found rather than floor 0")
}

func TestTierForSP(t *testing.T) {
	cases := []struct {
		name string
		sp   int
		want string
	}{
		{"zero SP is Iron, not unranked", 0, "iron"},
		{"negative clamps to Iron", -1, "iron"},
		{"deeply negative clamps to Iron", -100000, "iron"},
		{"mid Iron", 250, "iron"},
		{"one below Bronze", 499, "iron"},
		{"exactly Bronze", 500, "bronze"},
		{"one below Silver", 1499, "bronze"},
		{"exactly Silver", 1500, "silver"},
		{"one below Gold", 2999, "silver"},
		{"exactly Gold", 3000, "gold"},
		{"one below Platinum", 5499, "gold"},
		{"exactly Platinum", 5500, "platinum"},
		{"one below Diamond", 8499, "platinum"},
		{"exactly Diamond", 8500, "diamond"},
		{"one below Master", 12499, "diamond"},
		{"exactly Master", 12500, "master"},
		{"one below Grandmaster", 17999, "master"},
		{"exactly Grandmaster", 18000, "grandmaster"},
		{"above Grandmaster stays Grandmaster", 250000, "grandmaster"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, season.TierForSP(tc.sp))
		})
	}
}

func TestTierProgress(t *testing.T) {
	cases := []struct {
		name        string
		sp          int
		wantTier    string
		wantInto    int
		wantForNext int
	}{
		{"fresh player", 0, "iron", 0, 500},
		{"negative clamps to the Iron floor", -50, "iron", 0, 500},
		{"mid Iron", 200, "iron", 200, 500},
		{"one below Bronze", 499, "iron", 499, 500},
		{"exactly Bronze resets the band", 500, "bronze", 0, 1000},
		{"mid Bronze", 900, "bronze", 400, 1000},
		{"exactly Silver", 1500, "silver", 0, 1500},
		{"mid Gold", 4000, "gold", 1000, 2500},
		{"exactly Platinum", 5500, "platinum", 0, 3000},
		{"exactly Diamond", 8500, "diamond", 0, 4000},
		{"one below Grandmaster", 17999, "master", 5499, 5500},
		// The terminal case. A finite table HAS a top, so unlike LevelProgress's
		// strictly-increasing quadratic this branch is real and reachable.
		{"exactly Grandmaster has no next tier", 18000, "grandmaster", 0, 0},
		{"far above Grandmaster still has no next tier", 99999, "grandmaster", 81999, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tier, into, forNext := season.TierProgress(tc.sp)
			assert.Equal(t, tc.wantTier, tier, "tier")
			assert.Equal(t, tc.wantInto, into, "spIntoTier")
			assert.Equal(t, tc.wantForNext, forNext, "spForNextTier")
		})
	}
}

// TierProgress and TierForSP must never disagree, and spIntoTier must stay inside
// the band everywhere below Grandmaster — the two properties the progress bar relies
// on. Swept across every boundary and every band interior.
func TestTierProgress_AgreesWithTierForSP(t *testing.T) {
	for _, l := range ladder {
		for _, sp := range []int{l.floor, l.floor + 1, l.floor + l.band/2} {
			tier, into, forNext := season.TierProgress(sp)
			assert.Equal(t, season.TierForSP(sp), tier, "sp=%d", sp)
			assert.Equal(t, l.floor, sp-into, "sp=%d: into must be measured from the tier floor", sp)
			if forNext > 0 {
				assert.Less(t, into, forNext, "sp=%d: spIntoTier must stay inside the band", sp)
			}
		}
	}
}

func TestTierProgress_GrandmasterSpForNextTierIsZero(t *testing.T) {
	// Called out on its own because it is the one case a caller must branch on:
	// dividing by spForNextTier without checking it is a division by zero.
	_, _, forNext := season.TierProgress(18000)
	require.Zero(t, forNext, "Grandmaster is terminal — the client renders a full bar")
}
