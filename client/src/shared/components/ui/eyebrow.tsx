import * as React from "react";

import { cn } from "@/shared/lib/utils";

type EyebrowProps = {
  tone?: "brass" | "accent" | "mute";
  size?: "sm" | "xl";
  className?: string;
  children: React.ReactNode;
};

/**
 * Mono uppercase eyebrow used at the top of parchment cards, modal headers,
 * and the room-info card. Defaults match the in-card density (1.8px tracking);
 * size="xl" bumps to 2.4px for the top-of-card AuthCard-style headline rail.
 */
export function Eyebrow({ tone = "brass", size = "sm", className, children }: EyebrowProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 font-mono font-semibold uppercase",
        size === "xl" ? "text-xs tracking-caps" : "text-2xs tracking-caps",
        tone === "accent" ? "text-accent" : tone === "mute" ? "text-ink-mute" : "text-brass-deep",
        className,
      )}
    >
      {children}
    </span>
  );
}
