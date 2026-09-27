package season

import (
	"fmt"
	"slices"

	"github.com/emilijan/beljot/server/internal/ws"
)

// Season Points (SP) rank ladder. THIS FILE IS ITS SINGLE SOURCE OF TRUTH.
//
// The client carries one documented mirror at
// client/src/shared/lib/seasonTier.ts, under the same manual-sync convention as
// user/level.go <-> xpLevel.ts, user/honor.go <-> honor.ts and
// ws/events.go <-> wsEvents.ts. That mirror is DISPLAY ONLY: it carries the
// tokens, floors and division rule so it can label a rank and bucket an SP
// total when a token is unrecognised, and it never makes a decision. If a floor
// or a token changes here, change it there in the same commit.
//
// Every function here is PURE: no DB, no clock reads (time is always a
// parameter), no side effects. That is what lets the same arithmetic run in the
// match-end write path (RankForSP for the stored snapshot, RankChange for the
// event), in every running-season read (RankForSP, RankProgress for
// GET /api/v1/seasons/current), and inside event:season_points_awarded without
// any of them disagreeing.
//
// The rank an award lands on is also STORED with the row (player_seasons
// rank_tier / rank_division), and that snapshot, not this table, is what an
// ENDED season shows: an old season's SP was scored under the floors that were
// live then (migration 000028, Story 13.5). A running season always derives.
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
// display string must never be substituted for one of these. Since ended
// seasons show the stored rank_tier, renaming a token now needs a data
// migration of player_seasons.rank_tier.
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

// RankChange compares the RANK before and after a match: ws.RankChangePromoted
// when (tier, division) went up, ws.RankChangeDemoted when it went down, and
// ws.RankChangeNone otherwise. It compares ranks, never SP alone, so a loss that
// stays inside one division is "none" and a win across a division line inside
// one tier (Gold 1 -> Gold 2) is a promotion. See Ladder.RankChange.
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

// Progress decomposes an SP total into the current tier plus the position within
// that tier's band, the base RankProgress splits into steps. Mirrors
// LevelProgress (user/level.go):
//
//	tier           - the highest tier whose floor is <= sp
//	spIntoTier     - SP past the current tier's floor, in [0, band)
//	spForNextTier  - size of the current tier's band, nextFloor - thisFloor
//
// AT GRANDMASTER THERE IS NO NEXT TIER: spForNextTier is 0 and spIntoTier is
// everything above the Grandmaster floor, which is also what RankProgress
// reports there, and the client renders a full/terminal bar. LevelProgress
// can lean on a strictly-increasing quadratic and never hit this case; a
// FINITE table has a top, so this branch is real. Divide only after checking
// spForNextTier > 0.
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
	tier, division, _, _ = l.RankProgress(sp)
	return tier, division
}

// RankProgress decomposes an SP total into its rank plus the position within
// the current RANK STEP, the unit the progress bar fills (Story 13.5):
//
//	tier, division  - as Rank
//	spIntoStep      - SP past the start of the current step
//	spForNextStep   - size of the current step, 0 at Grandmaster
//
// The next step of a divided tier is the next division, or the next tier from
// division 3; Master steps to the Grandmaster floor; Grandmaster is terminal.
// So Gold 2 (667-733 on a 600-800 band) fills 0 -> 67 toward Gold 3, Diamond 3
// fills toward the Master floor, and Master fills toward Grandmaster.
//
// Division d of a band starts at the smallest offset x with 1 + x·3/band >= d,
// which is ceil((d-1)·band / 3): the exact inverse of Rank's split, so the bar
// resets to empty at precisely the SP where the division number changes.
//
// AT GRANDMASTER spForNextStep IS 0 and spIntoStep is everything above the
// floor; divide only after checking spForNextStep > 0.
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
