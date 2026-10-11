import { Shield, ShieldAlert, ShieldCheck, ShieldX } from "lucide-react";

import { HONOR_TIER_COLOR, type HonorTier } from "@/shared/lib/honor";

const HONOR_TIER_ICON: Record<HonorTier, typeof Shield> = {
  exemplary: ShieldCheck,
  trusted: ShieldCheck,
  fair: Shield,
  unreliable: ShieldAlert,
  problematic: ShieldX,
};

type HonorShieldProps = {
  tier: HonorTier;
  size?: number;
  className?: string;
  label?: string;
};

/**
 * Renders the honour shield for a tier, pairing tier colour and glyph so they never drift apart.
 * Decorative (aria-hidden) unless `label` is given.
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
