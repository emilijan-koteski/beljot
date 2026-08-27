package season

// Season Points (SP) rank ladder (Story 13.1). THIS FILE IS ITS SINGLE SOURCE
// OF TRUTH.
//
// The client carries one documented mirror at
// client/src/shared/lib/seasonTier.ts, under the same manual-sync convention as
// user/level.go <-> xpLevel.ts, user/honor.go <-> honor.ts and
// ws/events.go <-> wsEvents.ts. That mirror is DISPLAY ONLY: it buckets a
// server-supplied SP total for colouring and bar fill and never makes a
// decision. If a threshold or a token changes here, change it there in the same
// commit.
//
// Every function here is PURE: no DB, no clock reads (time is always a
// parameter), no side effects. That is what lets the same arithmetic run in the
// match-end write path, in GET /api/v1/seasons/current, and inside
// event:season_points_awarded without any of them disagreeing.
//
// FLAT LADDER, NO SUB-RANKS. Eight tiers, no I/II/III divisions, no placement
// matches, no LP and no ELO. The older PRD journey prose and the product brief
// describe that model; it was retired by sprint-change-proposal-2026-04-18 and
// the epic AC (SP formula + flat 8 tiers) is canonical. There is likewise NO
// "unranked" state: a player at 0 SP is Iron, which is a real tier that renders
// normally.

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

// tierFloor pairs a tier token with the INCLUSIVE lower bound of its band.
type tierFloor struct {
	Tier  string
	Floor int
}

// tierFloors is the ladder, ascending. ONE ordered table rather than thresholds
// scattered through switch arms, so a retune is a one-place change (the same
// convention as levelCurveCoefficient and honorHalfLifeDays). Every other
// function in this file derives from it, including SeasonTiers, so the token
// order and the thresholds can never disagree.
//
// The values are placeholders per sprint-change-proposal-2026-04-18: tuned
// during planning, expected to be retuned once real SP-per-match data exists.
var tierFloors = [...]tierFloor{
	{TierIron, 0},
	{TierBronze, 500},
	{TierSilver, 1500},
	{TierGold, 3000},
	{TierPlatinum, 5500},
	{TierDiamond, 8500},
	{TierMaster, 12500},
	{TierGrandmaster, 18000},
}

// SeasonTiers returns the ordered tier tokens, lowest first. A fresh slice per
// call so no caller can mutate the ladder out from under another.
func SeasonTiers() []string {
	out := make([]string, 0, len(tierFloors))
	for _, tf := range tierFloors {
		out = append(out, tf.Tier)
	}
	return out
}

// TierFloor returns the inclusive SP floor of a tier token, and whether the
// token is known. Exposed for tests and for any future operator tooling that
// needs the band without re-stating the numbers.
func TierFloor(tier string) (int, bool) {
	for _, tf := range tierFloors {
		if tf.Tier == tier {
			return tf.Floor, true
		}
	}
	return 0, false
}

// TierForSP returns the highest tier whose floor is <= sp. Integer arithmetic
// only. sp <= 0 is Iron (the DB CHECK forbids a negative, but a negative input
// clamps rather than falling off the bottom of the table).
func TierForSP(sp int) string {
	tier := tierFloors[0].Tier
	for _, tf := range tierFloors {
		if sp < tf.Floor {
			break
		}
		tier = tf.Tier
	}
	return tier
}

// TierProgress decomposes an SP total into the current tier plus the position
// within that tier's band, for driving the rank progress bar. Mirrors
// LevelProgress (user/level.go):
//
//	tier           - TierForSP(sp)
//	spIntoTier     - SP earned past the current tier's floor, in [0, band)
//	spForNextTier  - size of the current tier's band, nextFloor - thisFloor
//
// The bar fill is spIntoTier / spForNextTier.
//
// AT GRANDMASTER THERE IS NO NEXT TIER: spForNextTier is 0 and spIntoTier is
// everything above the Grandmaster floor, and the client renders a full/terminal
// bar. LevelProgress can lean on a strictly-increasing quadratic and never hit
// this case; a FINITE table has a top, so this branch is real. Divide only
// after checking spForNextTier > 0.
func TierProgress(sp int) (tier string, spIntoTier, spForNextTier int) {
	if sp < 0 {
		sp = 0
	}
	idx := 0
	for i, tf := range tierFloors {
		if sp < tf.Floor {
			break
		}
		idx = i
	}
	tier = tierFloors[idx].Tier
	spIntoTier = sp - tierFloors[idx].Floor
	if idx+1 < len(tierFloors) {
		spForNextTier = tierFloors[idx+1].Floor - tierFloors[idx].Floor
	}
	return tier, spIntoTier, spForNextTier
}
