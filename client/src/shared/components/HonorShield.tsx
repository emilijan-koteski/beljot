import { Shield, ShieldAlert, ShieldCheck, ShieldX } from "lucide-react";

import { HONOR_TIER_COLOR, type HonorTier } from "@/shared/lib/honor";

/**
 * Tier → glyph. The shape carries the same information as the colour, so the
 * scale survives colour-blindness and greyscale printing (UX spec: "no
 * information conveyed exclusively through colour").
 *
 * Exemplary and Trusted deliberately SHARE ShieldCheck: the distinction between
 * "practically never quits" and "reliable partner" is a matter of degree, and
 * inventing a fifth glyph to separate them would imply a difference in kind. The
 * three that matter — fine / warning / bad — are visibly distinct shapes.
 */
const HONOR_TIER_ICON: Record<HonorTier, typeof Shield> = {
  exemplary: ShieldCheck,
  trusted: ShieldCheck,
  fair: Shield,
  unreliable: ShieldAlert,
  problematic: ShieldX,
};

type HonorShieldProps = {
  tier: HonorTier;
  /** Pixel size passed to the lucide icon. Defaults to 16. */
  size?: number;
  className?: string;
  /**
   * Accessible name. Omit for a decorative shield that sits beside a visible
   * tier word or number — the default is aria-hidden, because the overwhelmingly
   * common case is "glyph + number", where an unlabelled icon would make a screen
   * reader announce the tier twice.
   */
  label?: string;
};

/**
 * The one honour shield. Every honour surface renders through this — the top-bar
 * chip, the profile band, the lobby card chip, the room badge, each seat tile,
 * the ejection modal and the in-match overlays.
 *
 * It exists so tier colour and tier glyph can never drift apart. The colour map
 * used to be duplicated across two components which had already disagreed on
 * `fair`; a component (rather than two exported maps) makes that class of bug
 * structurally impossible instead of merely discouraged.
 *
 * Colour comes from `HONOR_TIER_COLOR`, whose values are `var(--h*)` references,
 * so this themes itself on dark felt with no prop and no branch.
 */
export function HonorShield({ tier, size = 16, className, label }: HonorShieldProps) {
  const Icon = HONOR_TIER_ICON[tier];
  return (
    <Icon
      size={size}
      className={className}
      style={{ color: HONOR_TIER_COLOR[tier] }}
      strokeWidth={2.25}
      aria-hidden={label ? undefined : true}
      aria-label={label}
      role={label ? "img" : undefined}
      data-testid="honor-shield"
      data-tier={tier}
    />
  );
}
