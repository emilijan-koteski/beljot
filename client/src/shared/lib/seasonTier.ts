import type { TFunction } from "i18next";

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

const DIVISIONS_PER_TIER = 3;

/**
 * Whether a tier splits into divisions 1–3; mirrors `HasDivisions` in
 * server/internal/season/tier.go.
 */
export function seasonTierHasDivisions(tier: SeasonTier): boolean {
  return tier !== "master" && tier !== "grandmaster";
}

/** Coerce a possibly-absent SP total into a non-negative integer, keeping a real 0. */
export function seasonSpOrZero(sp: number | null | undefined): number {
  return Number.isFinite(sp) ? Math.max(0, sp as number) : 0;
}

/** Bucket an SP total into its tier using the floors mirrored from server tier.go. */
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
 * own bucket so an unrecognised token from a newer server still renders.
 */
export function normalizeSeasonTier(tier: string, sp: number): SeasonTier {
  return (SEASON_TIERS as readonly string[]).includes(tier)
    ? (tier as SeasonTier)
    : seasonTierForSp(sp);
}

/** Narrow a server division to 1–3 on a divided tier, otherwise null; never derived from SP. */
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
 * The rank label: "Gold 2", or the bare tier name when there is no division.
 * Pass an already-normalized tier and division.
 */
export function seasonRankLabel(t: TFunction, tier: SeasonTier, division: number | null): string {
  const tierName = t(`season.tier.${tier}`);
  return division === null ? tierName : t("season.rank", { tier: tierName, division });
}

/** A signed SP change for display ("+24", "−13" with U+2212, or "0"); never clamped at 0. */
export function formatSpChange(change: number): string {
  if (!Number.isFinite(change)) return "0";
  const whole = Math.round(change);
  if (whole > 0) return `+${whole.toLocaleString()}`;
  if (whole < 0) return `\u2212${Math.abs(whole).toLocaleString()}`;
  return "0";
}

/**
 * Progress-bar fill in [0, 1] from the server's rank-step pair. Full when the
 * step size is 0, as at Grandmaster.
 */
export function seasonBarFill(spIntoStep: number, spForNextStep: number): number {
  const into = seasonSpOrZero(spIntoStep);
  const span = seasonSpOrZero(spForNextStep);
  if (span <= 0) return 1;
  return Math.min(1, Math.max(0, into / span));
}

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

/** Whole days left until `endsAt`, rounded up; 0 when past, missing, or unparseable. */
export function seasonDaysRemaining(endsAt: string | null | undefined, now: number): number {
  if (!endsAt) return 0;
  const end = new Date(endsAt).getTime();
  if (!Number.isFinite(end)) return 0;
  const ms = end - now;
  if (ms <= 0) return 0;
  return Math.ceil(ms / 86_400_000);
}
