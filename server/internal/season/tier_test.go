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

// The tier half of RankForSP at every floor boundary.
func TestRankForSP_TierAtEveryBoundary(t *testing.T) {
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
			tier, _ := season.RankForSP(tc.sp)
			assert.Equal(t, tc.want, tier)
		})
	}
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
	liveTier, _ := season.RankForSP(460)
	assert.Equal(t, "silver", liveTier)

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
	tier, _ := season.RankForSP(599)
	assert.Equal(t, "silver", tier, "mutating DefaultLadder's result must not move the live floors")
}

// The rank STEP the progress bar fills (Story 13.5): the next division, the next
// tier from Diamond 3 and Master, nothing at Grandmaster.
func TestRankProgress(t *testing.T) {
	cases := []struct {
		name        string
		sp          int
		wantTier    string
		wantDiv     int
		wantInto    int
		wantForNext int
	}{
		{"fresh player is the start of Iron 1", 0, "iron", 1, 0, 50},
		{"negative clamps to the Iron 1 floor", -30, "iron", 1, 0, 50},
		{"last SP of Iron 1", 49, "iron", 1, 49, 50},
		{"Iron 2 resets the bar", 50, "iron", 2, 0, 50},
		{"Iron 3 steps to the Bronze floor", 120, "iron", 3, 20, 50},
		{"Silver 2 is a 100-SP step", 450, "silver", 2, 50, 100},
		{"Gold 1 floor", 600, "gold", 1, 0, 67},
		{"Gold 1, 50 in", 650, "gold", 1, 50, 67},
		// The spec's worked example: Gold 2 (667-733) fills 0 -> 67 toward Gold 3.
		{"Gold 2 floor", 667, "gold", 2, 0, 67},
		{"mid Gold 2", 700, "gold", 2, 33, 67},
		{"last SP of Gold 2", 733, "gold", 2, 66, 67},
		{"Gold 3 is the 66-SP remainder", 734, "gold", 3, 0, 66},
		{"Diamond 3 fills toward the Master floor", 1150, "diamond", 3, 16, 66},
		{"Master fills toward Grandmaster", 1250, "master", 0, 50, 200},
		{"top of Master", 1399, "master", 0, 199, 200},
		{"Grandmaster is terminal", 1400, "grandmaster", 0, 0, 0},
		{"far above Grandmaster is still terminal", 1500, "grandmaster", 0, 100, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tier, div, into, forNext := season.RankProgress(tc.sp)
			assert.Equal(t, tc.wantTier, tier, "tier")
			assert.Equal(t, tc.wantDiv, div, "division")
			assert.Equal(t, tc.wantInto, into, "spIntoStep")
			assert.Equal(t, tc.wantForNext, forNext, "spForNextStep")
		})
	}
}

// Swept over every SP below Grandmaster: RankProgress agrees with RankForSP, the
// bar stays inside its step, and it resets to empty exactly where the rank
// changes, never one SP early or late.
func TestRankProgress_StepsResetExactlyWhereTheRankChanges(t *testing.T) {
	prevTier, prevDiv := season.RankForSP(0)
	for sp := 0; sp < 1400; sp++ {
		tier, div, into, forNext := season.RankProgress(sp)
		wantTier, wantDiv := season.RankForSP(sp)
		require.Equal(t, wantTier, tier, "sp=%d", sp)
		require.Equal(t, wantDiv, div, "sp=%d", sp)
		require.Positive(t, forNext, "sp=%d: only Grandmaster is terminal", sp)
		require.GreaterOrEqual(t, into, 0, "sp=%d", sp)
		require.Less(t, into, forNext, "sp=%d: the bar must stay inside its step", sp)

		rankChanged := tier != prevTier || div != prevDiv
		if sp > 0 {
			assert.Equal(t, rankChanged, into == 0, "sp=%d: the bar is empty exactly on a new rank", sp)
		}
		// The step ends where the next rank begins.
		if into == forNext-1 {
			nextTier, nextDiv := season.RankForSP(sp + 1)
			assert.True(t, nextTier != tier || nextDiv != div, "sp=%d: the last SP of a step", sp)
		}
		prevTier, prevDiv = tier, div
	}
}

// RankChange compares RANKS, never SP alone (Story 13.5).
func TestRankChange(t *testing.T) {
	cases := []struct {
		name     string
		prev, sp int
		want     string
	}{
		{"division up inside a tier", 650, 690, "promoted"},
		{"tier up", 580, 610, "promoted"},
		{"Diamond 3 to Master", 1190, 1210, "promoted"},
		{"Master to Grandmaster", 1390, 1405, "promoted"},
		{"tier down", 610, 590, "demoted"},
		{"division down inside a tier", 670, 660, "demoted"},
		{"Master down to Diamond 3", 1205, 1195, "demoted"},
		{"a loss inside one division", 700, 690, "none"},
		{"a win inside one division", 690, 700, "none"},
		{"a move inside Master", 1250, 1300, "none"},
		{"a move inside Grandmaster", 1500, 1450, "none"},
		{"no move at all", 700, 700, "none"},
		{"a loss at 0 SP", 0, 0, "none"},
		{"a big climb across tiers", 100, 900, "promoted"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, season.RankChange(tc.prev, tc.sp))
		})
	}
}
