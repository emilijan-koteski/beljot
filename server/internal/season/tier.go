package season

import (
	"fmt"
	"slices"
)

// Season Points (SP) rank ladder. THIS FILE IS ITS SINGLE SOURCE OF TRUTH.
//
// The client carries one documented mirror at
// client/src/shared/lib/seasonTier.ts, under the same manual-sync convention as
// user/level.go <-> xpLevel.ts, user/honor.go <-> honor.ts and
// ws/events.go <-> wsEvents.ts. That mirror is DISPLAY ONLY: it buckets a
// server-supplied SP total for colouring and bar fill and never makes a
// decision. If a floor or a token changes here, change it there in the same
// commit.
//
// Every function here is PURE: no DB, no clock reads (time is always a
// parameter), no side effects. That is what lets the same arithmetic run in the
// match-end write path, in GET /api/v1/seasons/current, and inside
// event:season_points_awarded without any of them disagreeing.
//
// A WIN/LOSS LADDER WITH DIVISIONS (sprint-change-proposal-2026-09-26, which
// reverses the flat, climb-only ladder of 2026-04-18; that proposal's rejection
// of matchmaking by rank still stands). SP rises with wins and falls with
// losses (sp_formula.go), so players climb and drop through eight tiers. Iron
// through Diamond split their band into three equal divisions, 1 lowest and 3
// highest; Master and Grandmaster are single. There is NO "unranked" state: a
// player at 0 SP is Iron 1, a real rank that renders normally.
//
// THE FLOORS ARE TUNED TOGETHER WITH THE FORMULA (owner-approved 2026-09-26 from
// a replay of 2026 Q3). A bot seat counts as the Gold floor (sp_formula.go), and
// on a table of one human plus bots a player who wins a share p of their matches
// settles where the expected change is zero. With equal win and loss margins
// that is
//
//	gold floor + 2·S·log10(p·W / ((1−p)·L))
//
// Real wins are by wider margins than real losses, which lifts every settle
// point, so S was set on the replay's real margins rather than on this line:
// 50/60/70/80 % settle in Gold 3 / Platinum 2 / Diamond 1 / Master and 85 % on
// the Grandmaster floor. Iron and Bronze are narrower than the bands above Gold
// so a 50 % player reaches Silver in about 17 matches. A change to a floor here,
// or to S, W or L there, moves every settle point, so replay candidates with
// cmd/sptune before writing them in.

// Season tier tokens. STABLE MACHINE TOKENS: they cross the wire, land in
// player_seasons.rank_tier, and key the client's i18n labels and colour map. A
// display string must never be substituted for one of these.
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

// The INCLUSIVE SP floor of each tier. Named so the formula can say "a bot seat
// is the Gold floor" (spBotSeatSP) without restating a number.
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

// divisionsPerTier is how many equal divisions a divided tier's band splits
// into. Tiers without divisions report division 0.
const divisionsPerTier = 3

// Rung pairs a tier token with the INCLUSIVE lower bound of its band.
type Rung struct {
	Tier  string
	Floor int
}

// Ladder is a tier table, ascending: eight tiers in token order, the first
// floor 0, every floor strictly above the one before it.
//
// The live ladder is the package's own (the functions below); the type exists
// so the offline tuning command can replay a CANDIDATE table through the same
// arithmetic before it is written in here. NewLadder is the only way to build
// one that is not the live table, and it enforces the shape.
type Ladder []Rung

// ladder is the live table. ONE ordered table rather than thresholds scattered
// through switch arms, so a retune is a one-place change (the same convention
// as levelCurveCoefficient and honorHalfLifeDays). Every function in this file
// derives from it, including SeasonTiers, so the token order and the floors can
// never disagree.
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

// NewLadder builds a candidate ladder over the live tier tokens from eight
// floors, lowest tier first. The first floor must be 0 and every floor must be
// strictly above the previous one: an empty band would make a tier unreachable
// and break the division arithmetic.
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

// TierForSP returns the highest tier whose floor is <= sp. Integer arithmetic
// only. sp <= 0 is Iron (the DB CHECK forbids a negative, but a negative input
// clamps rather than falling off the bottom of the table).
func TierForSP(sp int) string {
	tier, _, _ := ladder.Progress(sp)
	return tier
}

// TierProgress decomposes an SP total into the current tier plus the position
// within that tier's band, for driving the rank progress bar. See
// Ladder.Progress.
func TierProgress(sp int) (tier string, spIntoTier, spForNextTier int) {
	return ladder.Progress(sp)
}

// RankForSP returns the tier and division an SP total sits at on the live
// ladder. See Ladder.Rank.
func RankForSP(sp int) (tier string, division int) {
	return ladder.Rank(sp)
}

// TierClimbed reports whether moving from previousSP to sp raised the TIER. A
// drop, a move inside one tier and no move at all are all false, so a demotion
// can never read as a promotion.
func TierClimbed(previousSP, sp int) bool {
	return ladder.index(sp) > ladder.index(previousSP)
}

// HasDivisions reports whether a tier splits into divisions. Master and
// Grandmaster are single; every lower tier has divisions 1-3.
func HasDivisions(tier string) bool {
	return tier != TierMaster && tier != TierGrandmaster
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

// Progress decomposes an SP total into the current tier plus the position within
// that tier's band, for driving the rank progress bar. Mirrors LevelProgress
// (user/level.go):
//
//	tier           - the highest tier whose floor is <= sp
//	spIntoTier     - SP past the current tier's floor, in [0, band)
//	spForNextTier  - size of the current tier's band, nextFloor - thisFloor
//
// The bar fill is spIntoTier / spForNextTier.
//
// AT GRANDMASTER THERE IS NO NEXT TIER: spForNextTier is 0 and spIntoTier is
// everything above the Grandmaster floor, and the client renders a full/terminal
// bar. LevelProgress can lean on a strictly-increasing quadratic and never hit
// this case; a FINITE table has a top, so this branch is real. Divide only
// after checking spForNextTier > 0.
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

// Rank returns the tier an SP total sits in and its division within that tier.
//
// A divided tier's band splits into three equal parts by integer arithmetic,
// 1 + spIntoTier·3 / band, so on a 200-SP band division 2 starts 67 SP in and
// division 3 at 134. Master and Grandmaster, which have no divisions, report 0.
// A negative total clamps to Iron 1, like Progress.
func (l Ladder) Rank(sp int) (tier string, division int) {
	tier, into, band := l.Progress(sp)
	if !HasDivisions(tier) || band <= 0 {
		return tier, 0
	}
	return tier, 1 + into*divisionsPerTier/band
}
