export const HONOR_TIERS = ["exemplary", "trusted", "fair", "unreliable", "problematic"] as const;

export type HonorTier = (typeof HONOR_TIERS)[number];

const HONOR_TIER_FLOORS: ReadonlyArray<readonly [HonorTier, number]> = [
  ["exemplary", 95],
  ["trusted", 85],
  ["fair", 70],
  ["unreliable", 50],
  ["problematic", 0],
];

export const HONOR_PRIOR_SCORE = 80;

export const HONOR_TREND_WINDOW = 20;

export const HONOR_NEW_PLAYER_MIN_MATCHES = 5;

/**
 * Coerces a possibly-absent score to a renderable number, substituting the prior.
 * Uses Number.isFinite, never truthiness, so a real score of 0 survives.
 */
export function honorScoreOrPrior(score: number | null | undefined): number {
  return Number.isFinite(score) ? (score as number) : HONOR_PRIOR_SCORE;
}

/**
 * Buckets a score into its tier using the bands mirrored from server/internal/user/honor.go.
 * Prefer the server's own `honorTier`; a non-finite score falls back to the prior's band.
 */
export function honorTierForScore(score: number): HonorTier {
  const clamped = Math.min(100, Math.max(0, honorScoreOrPrior(score)));
  for (const [tier, floor] of HONOR_TIER_FLOORS) {
    if (clamped >= floor) return tier;
  }
  return "problematic";
}

/**
 * Narrows a server tier string to a known tier, falling back to the score's own
 * band so an unknown token from a newer server still renders sensibly.
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

/** Coerces a possibly-absent honor count or signed trend delta to a number, defaulting to 0. */
export function honorCountOrZero(value: number | null | undefined): number {
  return Number.isFinite(value) ? (value as number) : 0;
}

/** Coerces a possibly-absent `isNewPlayer` flag, defaulting to true so an unknown score stays hidden. */
export function honorIsNewPlayer(value: boolean | null | undefined): boolean {
  return typeof value === "boolean" ? value : true;
}

const HONOR_FLOORS_ASCENDING = [...HONOR_TIER_FLOORS].reverse();

export const HONOR_TIER_BANDS: ReadonlyArray<{ tier: HonorTier; from: number; to: number }> =
  HONOR_FLOORS_ASCENDING.map(([tier, from], i) => {
    const next = HONOR_FLOORS_ASCENDING[i + 1];
    return { tier, from, to: next ? next[1] : 100 };
  });

export const HONOR_TIER_COLOR: Record<HonorTier, string> = {
  exemplary: "var(--h5)",
  trusted: "var(--h4)",
  fair: "var(--h3)",
  unreliable: "var(--h2)",
  problematic: "var(--h1)",
};

/** Label for a room's honour floor: "60+" below the top of the scale, a bare "100" at it. */
export function honorFloorLabel(minHonor: number | null | undefined): string {
  const floor = honorCountOrZero(minHonor);
  return floor >= 100 ? String(floor) : `${floor}+`;
}

/** The subset of a room the honour gate reads, satisfied by both `Room` and preview objects. */
type HonorGateRoom = {
  minHonor?: number | null;
  allowNewPlayers?: boolean | null;
};

/** The subset of the viewer the honour gate reads. */
export type HonorGateViewer = {
  honorScore?: number | null;
  isNewPlayer?: boolean | null;
};

/**
 * Whether a room enforces an honour requirement, mirroring `(*Room).honorGated()`
 * on the server. Absent fields read as ungated.
 */
export function honorRoomIsGated(room: HonorGateRoom): boolean {
  return honorCountOrZero(room.minHonor) > 0 || room.allowNewPlayers === false;
}

/**
 * Cosmetic preview of the server's `honorGateError`: a New Player is never
 * score-checked. The server stays authoritative, so never use it to skip a request.
 */
export function honorQualifies(room: HonorGateRoom, viewer: HonorGateViewer): boolean {
  if (!honorRoomIsGated(room)) return true;
  if (honorIsNewPlayer(viewer.isNewPlayer)) return room.allowNewPlayers !== false;
  return honorScoreOrPrior(viewer.honorScore) >= honorCountOrZero(room.minHonor);
}

export const HONOR_TREND_DIRECTIONS = ["up", "flat", "down"] as const;

export type HonorTrendDirection = (typeof HONOR_TREND_DIRECTIONS)[number];

/** Narrows a server trend string to a known direction, defaulting to "flat". */
export function normalizeHonorTrendDirection(direction: string): HonorTrendDirection {
  return (HONOR_TREND_DIRECTIONS as readonly string[]).includes(direction)
    ? (direction as HonorTrendDirection)
    : "flat";
}
