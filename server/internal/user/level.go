package user

// Lifetime level curve (Story 9.5). The level is a DERIVED, server-authoritative
// signal computed from total_xp — there is no `level` column. The curve is a
// placeholder quadratic, tuned per the 2026-06-18 economy reorg; keeping the
// coefficient as a named const makes a future tuning pass a one-line change.
//
//	threshold(N) = levelCurveCoefficient * N²   (XP required to reach level N)
//	LevelForXP(xp) = largest N with threshold(N) <= xp   (0 XP -> Level 0)
//
// e.g. 0..49 -> L0 | 50..199 -> L1 | 200..449 -> L2 | 450..799 -> L3 | 1250 -> L5.
//
// IMPORTANT: this curve is mirrored on the client (TopBar XP bar). The const
// here is the single source of truth; the client carries a documented copy
// under the manual-sync convention (see client wsEvents.ts/events.go precedent).
const levelCurveCoefficient = 50

// LevelForXP returns the largest level N such that levelCurveCoefficient*N² <=
// totalXP. A fresh player (0 XP) is Level 0; negative input (which the DB CHECK
// forbids) clamps to 0. Implemented with integer arithmetic — incrementing N
// while the next threshold still fits — to avoid float rounding errors at the
// exact thresholds that math.Sqrt would introduce.
func LevelForXP(totalXP int) int {
	if totalXP <= 0 {
		return 0
	}
	n := 0
	for levelCurveCoefficient*(n+1)*(n+1) <= totalXP {
		n++
	}
	return n
}

// LevelProgress decomposes totalXP into the current level plus the position
// within that level's band, for driving the XP progress bar:
//
//	level          — LevelForXP(totalXP)
//	xpIntoLevel    — XP earned past the current level's threshold, in [0, band)
//	xpForNextLevel — size of the current level's band, threshold(N+1)-threshold(N)
//
// The bar fill is xpIntoLevel / xpForNextLevel. xpForNextLevel is always
// positive (the quadratic is strictly increasing), so the ratio is well-defined.
func LevelProgress(totalXP int) (level, xpIntoLevel, xpForNextLevel int) {
	level = LevelForXP(totalXP)
	currentThreshold := levelCurveCoefficient * level * level
	nextThreshold := levelCurveCoefficient * (level + 1) * (level + 1)

	xpIntoLevel = totalXP - currentThreshold
	if xpIntoLevel < 0 {
		// Only reachable for negative totalXP, which LevelForXP already clamps to
		// level 0; guard anyway so the bar numerator never goes negative.
		xpIntoLevel = 0
	}
	xpForNextLevel = nextThreshold - currentThreshold
	return level, xpIntoLevel, xpForNextLevel
}
