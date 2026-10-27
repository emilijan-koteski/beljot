import * as React from "react";

import { cn } from "@/shared/lib/utils";

type BadgeTone = "neutral" | "accent" | "brass" | "bot" | "teamA" | "teamB" | "danger";

type BadgeSize = "sm" | "md";

type BadgeProps = Omit<React.ComponentProps<"span">, "children"> & {
  tone?: BadgeTone;
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
};

/**
 * Informative tag: a flat tinted label that is never clickable. Its square corners and missing
 * border set it apart from the bordered pill of an interactive {@link Chip}.
 */
export function Badge({
  tone = "neutral",
  size = "md",
  icon,
  className,
  children,
  ...rest
}: BadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center rounded leading-none whitespace-nowrap [&_svg]:size-3 [&_svg]:shrink-0",
        TONE_CLASS[tone],
        SIZE_CLASS[size],
        className,
      )}
      {...rest}
    >
      {icon}
      {children}
    </span>
  );
}
