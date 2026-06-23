// XP level curve — display math only.
//
// MUST stay in sync with the server: server/internal/user/level.go
// (const levelCurveCoefficient = 50; threshold(N) = 50 * N²). This is the same
// manual-sync convention as wsEvents.ts ↔ events.go — there is no generated
// shared type. The SERVER is authoritative for the level itself (it is sent on
// the user object and the profile response); these helpers only compute the
// cosmetic XP-bar fill, never a gating decision. Keep this the ONLY client copy
// of the curve (Story 9.5 Design Decision D6).
const LEVEL_CURVE_COEFFICIENT = 50;

/** XP required to reach `level` (threshold(N) = 50·N²). */
export function xpThreshold(level: number): number {
  return LEVEL_CURVE_COEFFICIENT * level * level;
}

export interface XpBarFill {
  /** XP earned past the current level's threshold (>= 0). */
  xpIntoLevel: number;
  /** Size of the current level's band, threshold(level+1) - threshold(level). */
  xpForNextLevel: number;
  /** Bar fill in [0, 1]. */
  fraction: number;
}

/**
 * Decompose a lifetime total into the within-level progress for the XP bar,
 * using the SERVER-provided `level` as the band anchor (so the client never
 * recomputes the level for a decision). Used by the top-nav, which only has
 * `level` + `totalXp` on the auth store; the profile uses the server-provided
 * xpIntoLevel / xpForNextLevel directly instead.
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
