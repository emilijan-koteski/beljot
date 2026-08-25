import * as React from "react";

import { cn } from "@/shared/lib/utils";

export type ChipOption<T extends string | number> = {
  value: T;
  label: React.ReactNode;
  icon?: React.ReactNode;
  /** Per-chip accent colour (CSS colour value) — e.g. an honour tier's own
   *  colour, so picking that chip also shows its band. Defaults to --accent. */
  tone?: string;
};

type ChipsProps<T extends string | number> = {
  value: T;
  onValueChange: (next: T) => void;
  options: ChipOption<T>[];
  ariaLabel?: string;
  testId?: string;
  className?: string;
  children?: React.ReactNode;
};

/**
 * Preset pill picker — the Create Room redesign's replacement for a raw
 * number input (buy-in) and a continuous slider (honour gate) wherever the
 * meaningful values are a short, named set rather than a free range.
 */
export function Chips<T extends string | number>({
  value,
  onValueChange,
  options,
  ariaLabel,
  testId,
  className,
  children,
}: ChipsProps<T>) {
  return (
    <div
      role="radiogroup"
      aria-label={ariaLabel}
      data-testid={testId}
      className={cn("flex flex-wrap gap-1.5", className)}
    >
      {options.map((o) => {
        const active = o.value === value;
        const tone = o.tone ?? "var(--accent)";
        return (
          <button
            key={String(o.value)}
            type="button"
            role="radio"
            aria-checked={active}
            onClick={() => onValueChange(o.value)}
            data-state={active ? "on" : "off"}
            data-testid={`${testId ?? "chips"}-${o.value}`}
            className={cn(
              "inline-flex h-8.5 items-center gap-1.5 rounded-full border px-3.5 text-[13px] tabular-nums transition-[background-color,border-color,color]",
              active
                ? "font-bold"
                : "border-border bg-surface-elevated text-ink-dim hover:text-ink font-medium",
            )}
            style={
              active
                ? {
                    color: tone,
                    borderColor: tone,
                    backgroundColor: `color-mix(in srgb, ${tone} 12%, transparent)`,
                  }
                : undefined
            }
          >
            {o.icon}
            {o.label}
          </button>
        );
      })}
      {children}
    </div>
  );
}
