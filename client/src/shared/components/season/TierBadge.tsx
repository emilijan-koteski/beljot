import { Shield } from "lucide-react";

import type { SeasonTier } from "@/shared/lib/seasonTier";
import { SEASON_TIER_COLOR, SEASON_TIER_SOFT } from "@/shared/lib/seasonTier";
import { cn } from "@/shared/lib/utils";

/** Badge scale. `md` is the RankBanner's original size; `sm` fits a list row. */
export type TierBadgeSize = "sm" | "md";

/**
 * Outer ring / inner glyph sizes per scale. A LOOKUP, NOT INTERPOLATION: Tailwind
 * v4 scans source text for complete class names, so `size-${n}` would compile to
 * nothing at all. Every class here is written out.
 */
const SIZE_CLASSES: Record<TierBadgeSize, { ring: string; glyph: string }> = {
  sm: { ring: "size-7", glyph: "size-3.5" },
  md: { ring: "size-11", glyph: "size-5" },
};

type Props = {
  /**
   * An ALREADY-NORMALIZED tier token. Callers run the server's string through
   * `normalizeSeasonTier` first, so the colour lookups below are total and a
   * version-skew token can never reach a missing map entry.
   */
  tier: SeasonTier;
  size?: TierBadgeSize;
  className?: string;
  /** Test hook. Kept a prop so each surface can name its own badge. */
  "data-testid"?: string;
};

/**
 * The seasonal tier badge — colour ring, tinted ground and a coloured glow.
 *
 * EXTRACTED FROM RankBanner (Story 13.2), verbatim, because the same treatment
 * is now rendered on the leaderboard's rows, the profile's season archive and
 * the header's rank chip. seasonTier.ts's own header names this story as the duplication the
 * colour maps exist to prevent; this is the JSX half of the same argument.
 *
 * The colours come from SEASON_TIER_COLOR / _SOFT, which are `var()` references
 * into the rank ramp in index.css. They are applied through inline `style` and
 * NOT as Tailwind classes: the ramp is a runtime value, so a class name cannot
 * carry it — and going through the variables is what lets the badge re-root on
 * the `.game-table` felt scope with no fork.
 *
 * `aria-hidden` because the badge is decoration: every surface that renders it
 * also renders the tier NAME as text beside it, so announcing it twice would
 * only add noise.
 */
export function TierBadge({
  tier,
  size = "md",
  className,
  "data-testid": testId = "tier-badge",
}: Props) {
  const color = SEASON_TIER_COLOR[tier];
  const { ring, glyph } = SIZE_CLASSES[size];

  return (
    <span
      data-testid={testId}
      data-tier={tier}
      aria-hidden="true"
      className={cn("grid shrink-0 place-items-center rounded-full", ring, className)}
      style={{
        background: SEASON_TIER_SOFT[tier],
        border: `1px solid ${color}`,
        boxShadow: `0 0 14px -2px ${color}`,
        color,
      }}
    >
      <Shield className={glyph} />
    </span>
  );
}
