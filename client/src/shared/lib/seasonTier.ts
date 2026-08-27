// Season rank tiers — display math only.
//
// MUST stay in sync with the server: server/internal/season/tier.go (the tier
// tokens and the SP thresholds). This is the same manual-sync convention as
// xpLevel.ts <-> level.go, honor.ts <-> honor.go and wsEvents.ts <-> events.go —
// there is no generated shared type.
//
// The SERVER IS AUTHORITATIVE for both the SP total and the tier: both arrive on
// event:season_points_awarded and on GET /api/v1/seasons/current. Nothing here
// ever makes a decision — no tier or SP total gates anything in this product, and
// the progress decomposition the RankBanner renders comes from the server's own
// spIntoTier / spForNextTier. These helpers exist for the cases where only an SP
// number is on hand (a toast fired from a WS payload) and for the colour map.
// Keep this the ONLY client copy of the ladder.

/** The eight stable tier tokens the server emits, in ASCENDING order. */
export const SEASON_TIERS = [
  "iron",
  "bronze",
  "silver",
  "gold",
  "platinum",
  "diamond",
  "master",
  "grandmaster",
] as const;

export type SeasonTier = (typeof SEASON_TIERS)[number];

/**
 * Inclusive SP floor of each tier band, ASCENDING. Mirrors `tierFloors` in
 * tier.go. One ordered table rather than scattered literals, so a server-side
 * retune is a one-place change here too.
 */
export const SEASON_TIER_FLOORS: ReadonlyArray<readonly [SeasonTier, number]> = [
  ["iron", 0],
  ["bronze", 500],
  ["silver", 1500],
  ["gold", 3000],
  ["platinum", 5500],
  ["diamond", 8500],
  ["master", 12500],
  ["grandmaster", 18000],
];

/**
 * Coerce a possibly-absent SP total into a renderable integer.
 *
 * Uses Number.isFinite, never truthiness: 0 SP is a REAL value from Go — it is
 * every new player's total — and a `||` fallback would be indistinguishable from
 * a missing one. `undefined` reaches here when a client bundle is newer than the
 * server (or a server rolls back mid-deploy), which left raw produced NaN and
 * sent every threshold comparison false.
 */
export function seasonSpOrZero(sp: number | null | undefined): number {
  return Number.isFinite(sp) ? Math.max(0, sp as number) : 0;
}

/**
 * Bucket an SP total into its tier. Prefer the server's own `rankTier` when you
 * have it; use this only where SP arrives without one, or as the fallback for an
 * unrecognised token (see normalizeSeasonTier).
 */
export function seasonTierForSp(sp: number): SeasonTier {
  const total = seasonSpOrZero(sp);
  let tier: SeasonTier = "iron";
  for (const [candidate, floor] of SEASON_TIER_FLOORS) {
    if (total < floor) break;
    tier = candidate;
  }
  return tier;
}

/**
 * Narrow an arbitrary server string to a known tier, falling back to the SP's
 * own bucket for an unrecognised token.
 *
 * This is the VERSION-SKEW GUARD: a future server-side tier retune (a ninth tier,
 * a rename) must degrade to a sensible colour and a real i18n key on a stale
 * bundle rather than rendering `season.tier.mythic` verbatim. It is exactly why
 * the Zod payload schema types `rankTier` as a plain string rather than a union.
 */
export function normalizeSeasonTier(tier: string, sp: number): SeasonTier {
  return (SEASON_TIERS as readonly string[]).includes(tier)
    ? (tier as SeasonTier)
    : seasonTierForSp(sp);
}

/**
 * Progress-bar fill in [0, 1] from the server's own decomposition.
 *
 * AT GRANDMASTER `spForNextTier` IS 0 — there is no next tier — and the bar renders
 * FULL rather than empty. That is the whole reason this is a function and not an
 * inline division: `spIntoTier / 0` is Infinity, and a naive guard that returned
 * 0 would show the top of the ladder as an empty bar.
 */
export function seasonBarFill(spIntoTier: number, spForNextTier: number): number {
  const into = seasonSpOrZero(spIntoTier);
  const span = seasonSpOrZero(spForNextTier);
  if (span <= 0) return 1;
  return Math.min(1, Math.max(0, into / span));
}

/**
 * Tier -> colour, the ONLY copy. Values are `var()` references into the rank ramp
 * declared in index.css (`--rt1`…`--rt8`, lowest -> highest), so they re-root
 * automatically inside `.game-table` and any rank surface themes itself on felt
 * with no fork.
 *
 * Centralised here for the reason honor.ts records for HONOR_TIER_COLOR: that map
 * was duplicated across HonorPanel and TopBar and the two had ALREADY drifted on
 * `fair`, with nothing in TypeScript able to notice. Story 13.2 adds a
 * leaderboard row and 13.3 a season-archive list, so the duplication is
 * prevented before it starts.
 */
export const SEASON_TIER_COLOR: Record<SeasonTier, string> = {
  iron: "var(--rt1)",
  bronze: "var(--rt2)",
  silver: "var(--rt3)",
  gold: "var(--rt4)",
  platinum: "var(--rt5)",
  diamond: "var(--rt6)",
  master: "var(--rt7)",
  grandmaster: "var(--rt8)",
};

/** Tier -> low-alpha fill, for badge/chip grounds and bar tracks. */
export const SEASON_TIER_SOFT: Record<SeasonTier, string> = {
  iron: "var(--rt1-soft)",
  bronze: "var(--rt2-soft)",
  silver: "var(--rt3-soft)",
  gold: "var(--rt4-soft)",
  platinum: "var(--rt5-soft)",
  diamond: "var(--rt6-soft)",
  master: "var(--rt7-soft)",
  grandmaster: "var(--rt8-soft)",
};

/** Tier -> hairline/border tone, for outlined variants. */
export const SEASON_TIER_LINE: Record<SeasonTier, string> = {
  iron: "var(--rt1-line)",
  bronze: "var(--rt2-line)",
  silver: "var(--rt3-line)",
  gold: "var(--rt4-line)",
  platinum: "var(--rt5-line)",
  diamond: "var(--rt6-line)",
  master: "var(--rt7-line)",
  grandmaster: "var(--rt8-line)",
};

/**
 * Whole days left until `endsAt`, floored at 0.
 *
 * The server sends an ABSOLUTE timestamp and the countdown is computed here (the
 * wire rule is absolute timestamps, never relative durations — a "daysRemaining"
 * integer is stale the moment it is serialised). Rounded UP, so the last partial
 * day reads "1 day left" rather than "0": a season that still accepts matches
 * must never render as already over.
 *
 * An unparseable or absent timestamp yields 0 rather than NaN.
 */
export function seasonDaysRemaining(endsAt: string | null | undefined, now: number): number {
  if (!endsAt) return 0;
  const end = new Date(endsAt).getTime();
  if (!Number.isFinite(end)) return 0;
  const ms = end - now;
  if (ms <= 0) return 0;
  return Math.ceil(ms / 86_400_000);
}
