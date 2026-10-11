import type { ReactNode } from "react";

import { cn } from "@/shared/lib/utils";

type Props = {
  icon: ReactNode;
  label: string;
  value: string | number;
  tone?: "neutral" | "accent";
  testId?: string;
};

export function StatPill({ icon, label, value, tone = "neutral", testId }: Props) {
  const isAccent = tone === "accent";
  return (
    <span
      data-testid={testId}
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full px-2.5 py-1.5 text-xs",
        isAccent
          ? "bg-accent-soft text-accent border border-accent/30"
          : "bg-surface-elevated text-ink-dim border border-border",
      )}
    >
      <span className={isAccent ? "text-accent" : "text-ink-mute"}>{icon}</span>
      <span>{label}</span>
      <span className={cn("tabular-nums font-semibold", isAccent ? "text-accent" : "text-ink")}>
        {value}
      </span>
    </span>
  );
}
