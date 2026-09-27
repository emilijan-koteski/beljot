import type { TFunction } from "i18next";

// Season rank tiers — display math only.
//
// MUST stay in sync with the server: server/internal/season/tier.go (the tier
// tokens, the SP floors, and which tiers have divisions 1–3). SP rises and falls on the
// win/loss ladder (Story 13.4), so a player moves down this table as well as up
// it. This is the same manual-sync convention as xpLevel.ts <-> level.go,
// honor.ts <-> honor.go and wsEvents.ts <-> events.go — there is no generated
// shared type.
//
// The SERVER IS AUTHORITATIVE for the SP total, the tier AND the division: all
// three arrive on event:season_points_awarded and on every season read. Nothing
// here ever makes a decision — no rank or SP total gates anything in this
// product, and the progress decomposition the profile's RankBanner renders comes
// from the server's own spIntoDivision / spForNextDivision. An ENDED season's
// rank is the server's stored snapshot, which this table must never re-derive
// (Story 13.5): a climb-only total in the thousands, bucketed on today's floors,
// would read as Grandmaster. So there is deliberately NO client copy of the
// division or rank-step arithmetic. These helpers exist for: the colour maps;
// the rank label ("Gold 2"); the signed-change label ("+24", "−13"); the bar
// fill from the server's own step pair; the season countdown; and the
// version-skew guards (an unrecognised tier token falls back to its SP bucket,
// an out-of-range division to none).
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
 * Inclusive SP floor of each tier band, ASCENDING. Mirrors the `ladder` table in
 * tier.go (the floor constants there). One ordered table rather than scattered
 * literals, so a server-side retune is a one-place change here too.
 */
export const SEASON_TIER_FLOORS: ReadonlyArray<readonly [SeasonTier, number]> = [
  ["iron", 0],
  ["bronze", 150],
  ["silver", 300],
  ["gold", 600],
  ["platinum", 800],
  ["diamond", 1000],
  ["master", 1200],
  ["grandmaster", 1400],
];

/** Divisions a divided tier splits into (1 lowest, 3 highest). */
const DIVISIONS_PER_TIER = 3;

/**
 * Whether a tier splits into divisions 1–3. Iron through Diamond do; Master and
 * Grandmaster are single (tier.go `HasDivisions`).
 */
export function seasonTierHasDivisions(tier: SeasonTier): boolean {
  return tier !== "master" && tier !== "grandmaster";
}

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
 * Narrow a server division to a renderable one: an integer 1–3 on a tier that
 * HAS divisions, otherwise `null` (render the bare tier name).
 *
 * NO SP FALLBACK, on purpose. `null` is a real answer — Master and Grandmaster
 * are single, and an ENDED season's row scored before divisions existed has no
 * division at all — so a missing or malformed value degrades to the bare tier
 * rather than to a division bucketed from SP, which would invent a rank the
 * player never held.
 */
export function normalizeSeasonDivision(
  tier: SeasonTier,
  division: number | null | undefined,
): number | null {
  if (!seasonTierHasDivisions(tier)) return null;
  if (!Number.isInteger(division)) return null;
  const d = division as number;
  return d >= 1 && d <= DIVISIONS_PER_TIER ? d : null;
}

/**
 * The rank as words: "Gold 2" via the `season.rank` key when there is a
 * division, the bare tier name ("Master", or an ended pre-division "Gold")
 * when there is not. The ONE rank label every surface renders — header chip,
 * banner, season section, leaderboard, archive and the rank-change dialog — so the format
 * cannot drift between them. Pass an already-normalized tier and division.
 */
export function seasonRankLabel(t: TFunction, tier: SeasonTier, division: number | null): string {
  const tierName = t(`season.tier.${tier}`);
  return division === null ? tierName : t("season.rank", { tier: tierName, division });
}

/**
 * A SIGNED SP change for display: "+24", "−13" (U+2212 MINUS SIGN, not a hyphen)
 * or "0".
 *
 * NEVER route a change through `seasonSpOrZero`, `seasonBarFill` or a
 * `finiteOrZero` clamp: those floor at 0 because a TOTAL cannot be negative, and
 * a change can — a loss clamped to "0 SP" would tell the player they lost
 * nothing. A non-finite value (a malformed frame the dispatcher should already
 * have dropped) renders as "0" rather than "NaN".
 */
export function formatSpChange(change: number): string {
  if (!Number.isFinite(change)) return "0";
  const whole = Math.round(change);
  if (whole > 0) return `+${whole.toLocaleString()}`;
  if (whole < 0) return `\u2212${Math.abs(whole).toLocaleString()}`;
  return "0";
}

/**
 * Progress-bar fill in [0, 1] from the server's own rank-step decomposition
 * (`spIntoDivision` / `spForNextDivision`).
 *
 * AT GRANDMASTER THE STEP SIZE IS 0 — there is no next rank — and the bar
 * renders FULL rather than empty. That is the whole reason this is a function
 * and not an inline division: `into / 0` is Infinity, and a naive guard that
 * returned 0 would show the top of the ladder as an empty bar.
 */
export function seasonBarFill(spIntoStep: number, spForNextStep: number): number {
  const into = seasonSpOrZero(spIntoStep);
  const span = seasonSpOrZero(spForNextStep);
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
