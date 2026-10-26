import { Shield } from "lucide-react";

import type { SeasonTier } from "@/shared/lib/seasonTier";
import { SEASON_TIER_COLOR, SEASON_TIER_SOFT } from "@/shared/lib/seasonTier";
import { cn } from "@/shared/lib/utils";

/**
 * Badge scale. `md` is the RankBanner's original size; `sm` fits a list row;
 * `lg` is the medallion of the post-match rank-change dialog.
 */
type TierBadgeSize = "sm" | "md" | "lg";

const SIZE_CLASSES: Record<TierBadgeSize, { ring: string; glyph: string }> = {
  sm: { ring: "size-7", glyph: "size-3.5" },
  md: { ring: "size-11", glyph: "size-5" },
  lg: { ring: "size-20", glyph: "size-9" },
};

type Props = {
  tier: SeasonTier;
  size?: TierBadgeSize;
  glow?: boolean;
  className?: string;
  "data-testid"?: string;
};

/**
 * The seasonal tier badge: colour ring, tinted ground and an optional glow.
 * Decorative and aria-hidden, since every surface also renders the tier name as text.
 */
export function TierBadge({
  tier,
  size = "md",
  glow = true,
  className,
  "data-testid": testId = "tier-badge",
}: Props) {
  const color = SEASON_TIER_COLOR[tier];
  const { ring, glyph } = SIZE_CLASSES[size];

  return (
    <span
      data-testid={testId}
      data-tier={tier}
      data-glow={glow}
      aria-hidden="true"
      className={cn("grid shrink-0 place-items-center rounded-full", ring, className)}
      style={{
        background: SEASON_TIER_SOFT[tier],
        border: `1px solid ${color}`,
        boxShadow: glow ? `0 0 12px -2px ${color}` : "none",
        color,
      }}
    >
      <Shield className={glyph} />
    </span>
  );
}
