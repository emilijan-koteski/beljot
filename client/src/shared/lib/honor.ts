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
 * How many FINISHED-OR-ABANDONED matches an account needs before it earns a
 * score. Mirrors `honorNewPlayerMinMatches` in server/internal/user/honor.go —
 * same manual-sync rule as the tier bands and the trend window above.
 *
 * Display only, and only ever as a denominator: it renders the newcomer's
 * "2 / 5" progress so the New Player state says how to LEAVE it, instead of
 * being a label that explains nothing. The server owns the decision — always
 * branch on its `isNewPlayer` flag, never on `completed + abandoned < 5`
 * computed here. That floor counts experience, not successes, and re-deriving it
 * client-side is exactly the bypass two review passes closed on the server.
 */
export const HONOR_NEW_PLAYER_MIN_MATCHES = 5;

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

/**
 * The five bands as half-open [from, to) ranges over 0-100, ascending.
 *
 * DERIVED from HONOR_TIER_FLOORS rather than restated, so the banded meter's
 * gradient stops, its tick labels and the tier a score buckets into can never
 * disagree. Retuning a floor moves all three at once.
 */
const HONOR_FLOORS_ASCENDING = [...HONOR_TIER_FLOORS].reverse();

export const HONOR_TIER_BANDS: ReadonlyArray<{ tier: HonorTier; from: number; to: number }> =
  HONOR_FLOORS_ASCENDING.map(([tier, from], i) => {
    const next = HONOR_FLOORS_ASCENDING[i + 1];
    return { tier, from, to: next ? next[1] : 100 };
  });

/**
 * Tier → colour, the ONLY copy. Values are `var()` references into the honour
 * ramp declared in index.css (`--h1`…`--h5`, worst→best), so they re-root
 * automatically inside `.game-table` and every honour surface themes itself on
 * felt with no fork.
 *
 * Centralised here for the reason coinGold.ts states for COIN_GOLD: before this,
 * the map was duplicated in HonorPanel.tsx and TopBar.tsx and the two had
 * ALREADY drifted — they disagreed on `fair` — with nothing in TypeScript able
 * to notice. The redesign adds five more consumers (the room badge, the seat
 * shield, the ejection meter, the slider fill, the reconnect line), so the
 * duplication had to stop before it multiplied.
 *
 * Prefer `<HonorShield>` (shared/components/HonorShield.tsx) over reading these
 * directly: it pairs the colour with the tier's GLYPH, which is what keeps the
 * scale legible without relying on colour alone.
 */
export const HONOR_TIER_COLOR: Record<HonorTier, string> = {
  exemplary: "var(--h5)",
  trusted: "var(--h4)",
  fair: "var(--h3)",
  unreliable: "var(--h2)",
  problematic: "var(--h1)",
};

/** Tier → low-alpha fill, for badge/chip grounds and meter bands. */
export const HONOR_TIER_SOFT: Record<HonorTier, string> = {
  exemplary: "var(--h5-soft)",
  trusted: "var(--h4-soft)",
  fair: "var(--h3-soft)",
  unreliable: "var(--h2-soft)",
  problematic: "var(--h1-soft)",
};

/** Tier → hairline/border tone, for outlined variants. */
export const HONOR_TIER_LINE: Record<HonorTier, string> = {
  exemplary: "var(--h5-line)",
  trusted: "var(--h4-line)",
  fair: "var(--h3-line)",
  unreliable: "var(--h2-line)",
  problematic: "var(--h1-line)",
};

/** The subset of a room the honour gate reads. Structural, so both the API
 *  `Room` type and a hand-built preview object satisfy it. */
export type HonorGateRoom = {
  minHonor?: number | null;
  allowNewPlayers?: boolean | null;
};

/** The subset of the viewer the honour gate reads. */
export type HonorGateViewer = {
  honorScore?: number | null;
  isNewPlayer?: boolean | null;
};

/**
 * Does this room enforce an honour requirement at all? Mirrors
 * `(*Room).honorGated()` in server/internal/room/handler.go.
 *
 * Absent fields read as UNGATED, which matters in practice: the QuickPlay
 * `system:room_created` payload is hand-built and omits both keys, and a
 * synthesized quick-play room genuinely is ungated.
 */
export function honorRoomIsGated(room: HonorGateRoom): boolean {
  return honorCountOrZero(room.minHonor) > 0 || room.allowNewPlayers === false;
}

/**
 * Would this viewer pass this room's honour gate? Mirrors `honorGateError` in
 * server/internal/room/handler.go — isNewPlayer is evaluated FIRST and a New
 * Player is never score-checked, which is the counter-intuitive half of Story
 * 9.8 D1 (the toggle is the owner's explicit "I'll take an unknown" switch).
 *
 * COSMETIC ONLY. The server re-validates on every join, return and start; this
 * exists so the lobby can render a Locked button and an "I qualify" filter
 * instead of making the player discover the gate by clicking. Never use it to
 * suppress a request — send it and let the 409 be authoritative.
 *
 * Note the deliberate asymmetry with CreateRoomModal's `honorKnown` check: there
 * an unknown envelope must NOT deny (a capability gate turning "unknown" into
 * "denied" would lock a veteran out of their own room), whereas here an unknown
 * viewer resolves through `honorIsNewPlayer`'s suppressed default and so is
 * treated leniently. Both err toward letting the player try.
 */
export function honorQualifies(room: HonorGateRoom, viewer: HonorGateViewer): boolean {
  if (!honorRoomIsGated(room)) return true;
  if (honorIsNewPlayer(viewer.isNewPlayer)) return room.allowNewPlayers !== false;
  return honorScoreOrPrior(viewer.honorScore) >= honorCountOrZero(room.minHonor);
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
