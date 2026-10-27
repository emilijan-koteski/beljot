import * as React from "react";

import { cn } from "@/shared/lib/utils";

type BadgeTone = "neutral" | "accent" | "brass" | "bot" | "teamA" | "teamB" | "danger";

type BadgeSize = "sm" | "md" | "lg";

type BadgeProps = Omit<React.ComponentProps<"span">, "children"> & {
  tone?: BadgeTone;
  tint?: string;
  size?: BadgeSize;
  icon?: React.ReactNode;
  children: React.ReactNode;
};

const TONE_CLASS: Record<BadgeTone, string> = {
  neutral: "bg-surface-sunken text-ink-dim",
  accent: "bg-accent-soft text-accent-deep",
  brass: "bg-brass-soft text-brass-deep",
  bot: "bg-bot-soft text-bot-deep",
  teamA: "bg-(--team-a-tint) text-team-a",
  teamB: "bg-(--team-b-tint) text-team-b",
  danger: "bg-danger/10 text-danger",
};

const SIZE_CLASS: Record<BadgeSize, string> = {
  sm: "h-5 gap-1 px-1.5 text-2xs font-bold tracking-wider uppercase",
  md: "h-6 gap-1.5 px-2 text-xs font-semibold",
  lg: "h-8 gap-2 px-3 text-sm font-semibold [&_svg]:size-4",
};

const TINTED_CLASS =
  "bg-[color-mix(in_srgb,var(--badge-tint)_13%,transparent)] text-(--badge-tint)";

/**
 * Informative tag: a flat tinted label, never clickable, whose square corners set it apart from a {@link Chip}.
 * `tint` swaps the tone for any colour, such as a viewer-relative team or an honour tier.
 */
export function Badge({
  tone = "neutral",
  tint,
  size = "md",
  icon,
  className,
  style,
  children,
  ...rest
}: BadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center rounded leading-none whitespace-nowrap [&_svg]:size-3 [&_svg]:shrink-0",
        tint === undefined ? TONE_CLASS[tone] : TINTED_CLASS,
        SIZE_CLASS[size],
        className,
      )}
      style={
        tint === undefined ? style : ({ "--badge-tint": tint, ...style } as React.CSSProperties)
      }
      {...rest}
    >
      {icon}
      {children}
    </span>
  );
}
