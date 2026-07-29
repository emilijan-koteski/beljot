// Honor tiers — display math only.
//
// MUST stay in sync with the server: server/internal/user/honor.go (the tier
// bands and the token strings). This is the same manual-sync convention as
// xpLevel.ts ↔ level.go and wsEvents.ts ↔ events.go — there is no generated
// shared type.
//
// The SERVER is authoritative for the score AND the tier: both arrive on the
// user object, the profile response, and event:honor_updated. Nothing here ever
// makes a gating decision (Story 9.8's room gate is enforced server-side). Keep
// this the ONLY client copy of the honor bands (Story 9.7 Design Decision D6).

/** The five stable tier tokens the server emits, in descending order. */
export const HONOR_TIERS = ["exemplary", "trusted", "fair", "unreliable", "problematic"] as const;

export type HonorTier = (typeof HONOR_TIERS)[number];

/** Inclusive lower bound of each tier band. Mirrors HonorTier in honor.go. */
const HONOR_TIER_FLOORS: ReadonlyArray<readonly [HonorTier, number]> = [
  ["exemplary", 95],
  ["trusted", 85],
  ["fair", 70],
  ["unreliable", 50],
  ["problematic", 0],
];

/**
 * The score a player with no history scores — the Beta(4,1) prior,
 * 100 * 4 / 5. Used as the render fallback when a score is somehow absent, so
 * the UI never shows a misleading 0 ("Problematic") for a blank slate.
 *
 * Wire it through `honorScoreOrPrior` — do NOT re-implement the check with `||`,
 * which would swallow a real score of 0.
 */
export const HONOR_PRIOR_SCORE = 80;

/**
 * How many matches each side of the trend comparison spans. Mirrors
 * `honorTrendWindow` in server/internal/user/honor.go — same manual-sync rule as
 * the tier bands above.
 *
 * Display only: it exists so the `profile.honor.trendTooltip` copy can interpolate
 * the real number instead of hardcoding "20" in four locale files, which is how
 * the tooltip would otherwise drift the moment the server constant is retuned
 * (review pass 2). It never feeds a calculation — the server computes the trend.
 */
export const HONOR_TREND_WINDOW = 20;

/**
 * Coerce a possibly-absent honor score into a renderable number.
 *
 * The WS handler type-guards every field before it reaches the store, but the
 * HTTP paths do not: a client bundle newer than the server (or a server rollback
 * mid-deploy) yields `undefined` here. Left unguarded that became `NaN`, every
 * tier-floor comparison went false, and the UI fell through to the WORST tier —
 * a blank number in danger red with `data-tier="problematic"`, which is the exact
 * misread HONOR_PRIOR_SCORE was documented to prevent (code review 2026-07-29).
 *
 * Uses Number.isFinite, never truthiness: a score of 0 is a REAL value from Go
 * and must survive untouched.
 */
export function honorScoreOrPrior(score: number | null | undefined): number {
  return Number.isFinite(score) ? (score as number) : HONOR_PRIOR_SCORE;
}

/**
 * Bucket a server-supplied score into its tier. Prefer the server's own
 * `honorTier` when you have it; use this only where a score arrives without one
 * (e.g. rendering a hypothetical or a fallback).
 *
 * A non-finite score falls back to the prior's band rather than "problematic".
 */
export function honorTierForScore(score: number): HonorTier {
  const clamped = Math.min(100, Math.max(0, honorScoreOrPrior(score)));
  for (const [tier, floor] of HONOR_TIER_FLOORS) {
    if (clamped >= floor) return tier;
  }
  return "problematic";
}

/**
 * Narrow an arbitrary server string to a known tier, falling back to the
 * score's own bucket for an unrecognised token.
 *
 * This is the version-skew guard: a future server tier retune must degrade to a
 * sensible colour on a stale bundle rather than rendering a missing i18n key.
 * It is why the Zod payload schema types honorTier as a plain string.
 */
export function normalizeHonorTier(tier: string, score: number): HonorTier {
  return (HONOR_TIERS as readonly string[]).includes(tier)
    ? (tier as HonorTier)
    : honorTierForScore(score);
}

/** Bar fill in [0, 1] for an honor meter. The scale is a flat 0-100. */
export function honorBarFill(score: number): number {
  return Math.min(1, Math.max(0, honorScoreOrPrior(score) / 100));
}

/**
 * Coerce a possibly-absent honor count or trend delta into a renderable integer.
 *
 * Same threat model as `honorScoreOrPrior` — the HTTP profile and auth-refresh
 * responses are not type-guarded the way the WS payload is, so a bundle newer
 * than the server yields `undefined`. Left raw those rendered a blank number and
 * `data-value="undefined"` (review pass 2). Uses Number.isFinite, so a real 0
 * survives.
 */
export function honorCountOrZero(value: number | null | undefined): number {
  return Number.isFinite(value) ? (value as number) : 0;
}

/**
 * Coerce a possibly-absent `isNewPlayer` flag, defaulting to SUPPRESSED.
 *
 * The safe default is `true`: if we cannot tell whether this account has enough
 * history to have earned a score, hiding the number is the conservative choice.
 * Left raw, `undefined` was falsy and therefore took the NUMERIC branch, so a
 * server that had not yet shipped the honor fields made every account — brand-new
 * ones included — render a confident "80 / Fair" (review pass 2).
 */
export function honorIsNewPlayer(value: boolean | null | undefined): boolean {
  return typeof value === "boolean" ? value : true;
}

/** The three stable trend-direction tokens the server emits. */
export const HONOR_TREND_DIRECTIONS = ["up", "flat", "down"] as const;

export type HonorTrendDirection = (typeof HONOR_TREND_DIRECTIONS)[number];

/**
 * Narrow an arbitrary server string to a known trend direction, defaulting to
 * "flat" — the neutral reading, which is the safe thing to show when the token
 * is unrecognised.
 */
export function normalizeHonorTrendDirection(direction: string): HonorTrendDirection {
  return (HONOR_TREND_DIRECTIONS as readonly string[]).includes(direction)
    ? (direction as HonorTrendDirection)
    : "flat";
}
