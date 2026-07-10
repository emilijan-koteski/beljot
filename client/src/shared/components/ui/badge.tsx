import * as React from "react";

import { cn } from "@/shared/lib/utils";

type BadgeTone = "neutral" | "accent" | "brass" | "bot" | "teamA" | "teamB" | "danger";

type BadgeProps = {
  tone?: BadgeTone;
  icon?: React.ReactNode;
  className?: string;
  children: React.ReactNode;
};

const toneStyles: Record<BadgeTone, { bg: string; color: string; border: string }> = {
  neutral: {
    bg: "var(--surface-3)",
    color: "var(--ink-dim)",
    border: "var(--border)",
  },
  accent: {
    bg: "var(--accent-soft)",
    color: "var(--accent-deep)",
    border: "transparent",
  },
  brass: {
    bg: "var(--brass-soft)",
    color: "var(--brass-deep)",
    border: "color-mix(in srgb, var(--brass) 40%, transparent)",
  },
  // Frost-blue bot marker — see the --bot token block in index.css.
  bot: {
    bg: "var(--bot-soft)",
    color: "var(--bot-deep)",
    border: "var(--bot-edge)",
  },
  teamA: {
    bg: "var(--team-a-tint)",
    color: "var(--team-a)",
    border: "var(--team-a-edge-soft)",
  },
  teamB: {
    bg: "var(--team-b-tint)",
    color: "var(--team-b)",
    border: "var(--team-b-edge-soft)",
  },
  danger: {
    bg: "rgba(139,42,31,0.10)",
    color: "var(--danger)",
    border: "rgba(139,42,31,0.30)",
  },
};

/**
 * Pill badge used for room-info chips, team labels, and matchmaking meta
 * rows. Tones map 1:1 to the design's badge styles in room-flow-parts.jsx.
 */
export function Badge({ tone = "neutral", icon, className, children }: BadgeProps) {
  const s = toneStyles[tone];
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-semibold leading-none",
        className,
      )}
      style={{
        background: s.bg,
        color: s.color,
        border: `1px solid ${s.border}`,
        letterSpacing: 0.2,
      }}
    >
      {icon}
      {children}
    </span>
  );
}
