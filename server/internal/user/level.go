package user

const levelCurveCoefficient = 50

// LevelForXP returns the largest level N with levelCurveCoefficient*N² <= totalXP, using
// integer arithmetic to stay exact at thresholds.
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

// LevelProgress returns the level, the XP earned past its threshold, and the size of its band
// for the XP progress bar.
func LevelProgress(totalXP int) (level, xpIntoLevel, xpForNextLevel int) {
	level = LevelForXP(totalXP)
	currentThreshold := levelCurveCoefficient * level * level
	nextThreshold := levelCurveCoefficient * (level + 1) * (level + 1)

	xpIntoLevel = totalXP - currentThreshold
	if xpIntoLevel < 0 {
		xpIntoLevel = 0
	}
	xpForNextLevel = nextThreshold - currentThreshold
	return level, xpIntoLevel, xpForNextLevel
}
