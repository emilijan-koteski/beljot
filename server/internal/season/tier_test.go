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
	{"iron", 0, 150},
	{"bronze", 150, 150},
	{"silver", 300, 300},
	{"gold", 600, 200},
	{"platinum", 800, 200},
	{"diamond", 1000, 200},
	{"master", 1200, 200},
	{"grandmaster", 1400, 0},
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
		{"mid Iron", 75, "iron"},
		{"one below Bronze", 149, "iron"},
		{"exactly Bronze", 150, "bronze"},
		{"one below Silver", 299, "bronze"},
		{"exactly Silver", 300, "silver"},
		{"one below Gold", 599, "silver"},
		{"exactly Gold", 600, "gold"},
		{"one below Platinum", 799, "gold"},
		{"exactly Platinum", 800, "platinum"},
		{"one below Diamond", 999, "platinum"},
		{"exactly Diamond", 1000, "diamond"},
		{"one below Master", 1199, "diamond"},
		{"exactly Master", 1200, "master"},
		{"one below Grandmaster", 1399, "master"},
		{"exactly Grandmaster", 1400, "grandmaster"},
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
		{"fresh player", 0, "iron", 0, 150},
		{"negative clamps to the Iron floor", -50, "iron", 0, 150},
		{"mid Iron", 100, "iron", 100, 150},
		{"one below Bronze", 149, "iron", 149, 150},
		{"exactly Bronze resets the band", 150, "bronze", 0, 150},
		{"mid Silver", 450, "silver", 150, 300},
		{"mid Gold", 700, "gold", 100, 200},
		{"exactly Platinum", 800, "platinum", 0, 200},
		{"exactly Diamond", 1000, "diamond", 0, 200},
		{"one below Grandmaster", 1399, "master", 199, 200},
		// The terminal case. A finite table HAS a top, so unlike LevelProgress's
		// strictly-increasing quadratic this branch is real and reachable.
		{"exactly Grandmaster has no next tier", 1400, "grandmaster", 0, 0},
		{"far above Grandmaster still has no next tier", 9999, "grandmaster", 8599, 0},
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
	_, _, forNext := season.TierProgress(1400)
	require.Zero(t, forNext, "Grandmaster is terminal — the client renders a full bar")
}

func TestRankForSP(t *testing.T) {
	cases := []struct {
		name     string
		sp       int
		wantTier string
		wantDiv  int
	}{
		{"zero SP is Iron 1", 0, "iron", 1},
		{"negative clamps to Iron 1", -40, "iron", 1},
		// Iron's 150-SP band: 1 + into·3/150 steps at 50 and 100.
		{"last SP of Iron 1", 49, "iron", 1},
		{"first SP of Iron 2", 50, "iron", 2},
		{"last SP of Iron 2", 99, "iron", 2},
		{"first SP of Iron 3", 100, "iron", 3},
		{"top of Iron is still Iron 3", 149, "iron", 3},
		{"Bronze floor resets to division 1", 150, "bronze", 1},
		// Silver's 300-SP band steps at 100 and 200 in.
		{"Silver 1", 300, "silver", 1},
		{"Silver 2", 400, "silver", 2},
		{"Silver 3", 500, "silver", 3},
		// A 200-SP band steps at 67 and 134 in.
		{"Gold 1 floor", 600, "gold", 1},
		{"last SP of Gold 1", 666, "gold", 1},
		{"first SP of Gold 2", 667, "gold", 2},
		{"last SP of Gold 2", 733, "gold", 2},
		{"first SP of Gold 3", 734, "gold", 3},
		{"top of Diamond is Diamond 3", 1199, "diamond", 3},
		{"Master has no division", 1200, "master", 0},
		{"top of Master has no division", 1399, "master", 0},
		{"Grandmaster has no division", 1400, "grandmaster", 0},
		{"far above Grandmaster", 250000, "grandmaster", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tier, div := season.RankForSP(tc.sp)
			assert.Equal(t, tc.wantTier, tier, "tier")
			assert.Equal(t, tc.wantDiv, div, "division")
		})
	}
}

// Every SP value maps to a division inside 1-3 on a divided tier, divisions never
// go down as SP rises inside a tier, and each division gets a third of the band
// (to within the one SP integer division leaves over).
func TestRankForSP_DivisionsSplitEachBandInThirds(t *testing.T) {
	for _, l := range ladder {
		if !season.HasDivisions(l.tier) {
			continue
		}
		counts := map[int]int{}
		prev := 1
		for sp := l.floor; sp < l.floor+l.band; sp++ {
			tier, div := season.RankForSP(sp)
			require.Equal(t, l.tier, tier, "sp=%d", sp)
			require.GreaterOrEqual(t, div, prev, "sp=%d: division went down inside %s", sp, l.tier)
			require.LessOrEqual(t, div, 3, "sp=%d", sp)
			counts[div]++
			prev = div
		}
		for div := 1; div <= 3; div++ {
			assert.InDelta(t, float64(l.band)/3, float64(counts[div]), 1, "%s %d width", l.tier, div)
		}
	}
}

func TestHasDivisions(t *testing.T) {
	for _, tier := range []string{"iron", "bronze", "silver", "gold", "platinum", "diamond"} {
		assert.True(t, season.HasDivisions(tier), tier)
	}
	assert.False(t, season.HasDivisions("master"))
	assert.False(t, season.HasDivisions("grandmaster"))
}

func TestNewLadder(t *testing.T) {
	l, err := season.NewLadder([]int{0, 100, 250, 450, 700, 1000, 1350, 1750})
	require.NoError(t, err)

	tier, div := l.Rank(460)
	assert.Equal(t, "gold", tier, "a candidate ladder ranks by its own floors")
	assert.Equal(t, 1, div)
	floor, ok := l.Floor("grandmaster")
	require.True(t, ok)
	assert.Equal(t, 1750, floor)

	// The live ladder is untouched by building a candidate.
	assert.Equal(t, "silver", season.TierForSP(460))

	bad := []struct {
		name   string
		floors []int
	}{
		{"too few floors", []int{0, 100, 200}},
		{"first floor not zero", []int{10, 100, 250, 450, 700, 1000, 1350, 1750}},
		{"equal floors leave an empty band", []int{0, 100, 100, 450, 700, 1000, 1350, 1750}},
		{"descending floors", []int{0, 100, 250, 200, 700, 1000, 1350, 1750}},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			_, err := season.NewLadder(tc.floors)
			assert.Error(t, err)
		})
	}
}

func TestDefaultLadder_IsACopy(t *testing.T) {
	l := season.DefaultLadder()
	l[3].Floor = 1
	assert.Equal(t, "silver", season.TierForSP(599), "mutating DefaultLadder's result must not move the live floors")
}
