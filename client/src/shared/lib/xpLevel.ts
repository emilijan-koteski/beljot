const LEVEL_CURVE_COEFFICIENT = 50;

/** XP required to reach `level` (50·N²), mirroring server/internal/user/level.go. */
export function xpThreshold(level: number): number {
  return LEVEL_CURVE_COEFFICIENT * level * level;
}

interface XpBarFill {
  xpIntoLevel: number;
  xpForNextLevel: number;
  fraction: number;
}

/**
 * Decompose a lifetime total into within-level XP-bar progress, anchored on the
 * server-provided `level`.
 */
export function xpBarFill(totalXp: number, level: number): XpBarFill {
  const current = xpThreshold(level);
  const next = xpThreshold(level + 1);
  const span = next - current;
  const into = Math.max(0, totalXp - current);
  const fraction = span > 0 ? Math.min(1, into / span) : 0;
  return { xpIntoLevel: into, xpForNextLevel: span, fraction };
}

/** Clamp an arbitrary numerator/denominator pair to a [0,1] bar fraction. */
export function xpFraction(xpIntoLevel: number, xpForNextLevel: number): number {
  if (xpForNextLevel <= 0) return 0;
  return Math.min(1, Math.max(0, xpIntoLevel / xpForNextLevel));
}
