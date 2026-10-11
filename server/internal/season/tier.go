package season

import (
	"fmt"
	"slices"

	"github.com/emilijan/beljot/server/internal/ws"
)

const (
	TierIron        = "iron"
	TierBronze      = "bronze"
	TierSilver      = "silver"
	TierGold        = "gold"
	TierPlatinum    = "platinum"
	TierDiamond     = "diamond"
	TierMaster      = "master"
	TierGrandmaster = "grandmaster"
)

const (
	floorIron        = 0
	floorBronze      = 150
	floorSilver      = 300
	floorGold        = 600
	floorPlatinum    = 800
	floorDiamond     = 1000
	floorMaster      = 1200
	floorGrandmaster = 1400
)

const divisionsPerTier = 3

// Rung pairs a tier token with the INCLUSIVE lower bound of its band.
type Rung struct {
	Tier  string
	Floor int
}

// Ladder is an ascending tier table of eight rungs starting at floor 0.
// The live table is mirrored for display in client/src/shared/lib/seasonTier.ts; change both together.
type Ladder []Rung

var ladder = Ladder{
	{TierIron, floorIron},
	{TierBronze, floorBronze},
	{TierSilver, floorSilver},
	{TierGold, floorGold},
	{TierPlatinum, floorPlatinum},
	{TierDiamond, floorDiamond},
	{TierMaster, floorMaster},
	{TierGrandmaster, floorGrandmaster},
}

// DefaultLadder returns a copy of the live table, so no caller can mutate it.
func DefaultLadder() Ladder {
	return slices.Clone(ladder)
}

// NewLadder builds a candidate ladder from eight floors, lowest tier first.
// The first floor must be 0 and each floor must be strictly above the previous one.
func NewLadder(floors []int) (Ladder, error) {
	if len(floors) != len(ladder) {
		return nil, fmt.Errorf("ladder needs %d floors, got %d", len(ladder), len(floors))
	}
	if floors[0] != 0 {
		return nil, fmt.Errorf("the %s floor must be 0, got %d", ladder[0].Tier, floors[0])
	}
	out := make(Ladder, len(ladder))
	for i, f := range floors {
		if i > 0 && f <= floors[i-1] {
			return nil, fmt.Errorf("the %s floor (%d) must be above the %s floor (%d)",
				ladder[i].Tier, f, ladder[i-1].Tier, floors[i-1])
		}
		out[i] = Rung{Tier: ladder[i].Tier, Floor: f}
	}
	return out, nil
}

// SeasonTiers returns the ordered tier tokens, lowest first. A fresh slice per
// call so no caller can mutate the ladder out from under another.
func SeasonTiers() []string {
	out := make([]string, 0, len(ladder))
	for _, tf := range ladder {
		out = append(out, tf.Tier)
	}
	return out
}

// TierFloor returns the inclusive SP floor of a tier token, and whether the
// token is known. Exposed for tests and for any future operator tooling that
// needs the band without re-stating the numbers.
func TierFloor(tier string) (int, bool) {
	return ladder.Floor(tier)
}

// RankForSP returns the tier and division an SP total sits at on the live
// ladder. See Ladder.Rank.
func RankForSP(sp int) (tier string, division int) {
	return ladder.Rank(sp)
}

// RankProgress returns the rank an SP total sits at on the live ladder and the
// position within its rank step, for driving the rank progress bar. See
// Ladder.RankProgress.
func RankProgress(sp int) (tier string, division, spIntoStep, spForNextStep int) {
	return ladder.RankProgress(sp)
}

// RankChange compares the (tier, division) rank before and after a match and returns the ws.RankChange token.
func RankChange(previousSP, sp int) string {
	return ladder.RankChange(previousSP, sp)
}

// HasDivisions reports whether a tier splits into divisions. Master and
// Grandmaster are single; every lower tier has divisions 1-3.
func HasDivisions(tier string) bool {
	return tier != TierMaster && tier != TierGrandmaster
}

// divisionOrNil maps the ladder's "no division" 0 to nil, the NULL / null that
// player_seasons.rank_division and every wire `division` carry for Master and
// Grandmaster.
func divisionOrNil(division int) *int {
	if division == 0 {
		return nil
	}
	return &division
}

// Floor returns the inclusive SP floor of a tier token, and whether the token
// is on this ladder.
func (l Ladder) Floor(tier string) (int, bool) {
	for _, tf := range l {
		if tf.Tier == tier {
			return tf.Floor, true
		}
	}
	return 0, false
}

// index returns the position of the tier an SP total sits in, 0 for Iron.
func (l Ladder) index(sp int) int {
	idx := 0
	for i, tf := range l {
		if sp < tf.Floor {
			break
		}
		idx = i
	}
	return idx
}

// Progress returns the tier for an SP total, the SP past its floor, and the size of its band.
// At Grandmaster spForNextTier is 0, so check it before dividing.
func (l Ladder) Progress(sp int) (tier string, spIntoTier, spForNextTier int) {
	if sp < 0 {
		sp = 0
	}
	idx := l.index(sp)
	tier = l[idx].Tier
	spIntoTier = sp - l[idx].Floor
	if idx+1 < len(l) {
		spForNextTier = l[idx+1].Floor - l[idx].Floor
	}
	return tier, spIntoTier, spForNextTier
}

// Rank returns the tier an SP total sits in and its division 1-3, or 0 for Master and Grandmaster.
func (l Ladder) Rank(sp int) (tier string, division int) {
	tier, division, _, _ = l.RankProgress(sp)
	return tier, division
}

// RankProgress returns the rank plus the SP into and size of the current rank step, which the progress bar fills.
// At Grandmaster spForNextStep is 0, so check it before dividing.
func (l Ladder) RankProgress(sp int) (tier string, division, spIntoStep, spForNextStep int) {
	tier, into, band := l.Progress(sp)
	if !HasDivisions(tier) || band <= 0 {
		return tier, 0, into, band
	}
	division = 1 + into*divisionsPerTier/band
	start := divisionStart(division, band)
	next := band
	if division < divisionsPerTier {
		next = divisionStart(division+1, band)
	}
	return tier, division, into - start, next - start
}

// divisionStart is the offset into a band at which division d begins,
// ceil((d-1)·band / divisionsPerTier) in integer arithmetic.
func divisionStart(division, band int) int {
	return ((division-1)*band + divisionsPerTier - 1) / divisionsPerTier
}

// RankChange reports how the rank moved between two SP totals on this ladder,
// as a wire token (ws.RankChangePromoted / Demoted / None).
func (l Ladder) RankChange(previousSP, sp int) string {
	prev, next := l.rankOrdinal(previousSP), l.rankOrdinal(sp)
	switch {
	case next > prev:
		return ws.RankChangePromoted
	case next < prev:
		return ws.RankChangeDemoted
	default:
		return ws.RankChangeNone
	}
}

// rankOrdinal orders ranks by tier first and division second. Master and
// Grandmaster carry division 0, which never matters: no other rank shares
// their tier.
func (l Ladder) rankOrdinal(sp int) int {
	_, division := l.Rank(sp)
	return l.index(sp)*(divisionsPerTier+1) + division
}
