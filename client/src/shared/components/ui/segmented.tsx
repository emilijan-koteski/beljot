import * as React from "react";

import { cn } from "@/shared/lib/utils";

type SegmentedOption<T extends string> = {
  value: T;
  label: React.ReactNode;
  icon?: React.ReactNode;
  disabled?: boolean;
};

type SegmentedProps<T extends string> = {
  value: T;
  onValueChange: (next: T) => void;
  options: SegmentedOption<T>[];
  dense?: boolean;
  className?: string;
  ariaLabel?: string;
  testId?: string;
  trailing?: React.ReactNode;
};

/**
 * Segmented radio-group toggle for the parchment option controls: equal-width options in a sunken well, the active one
 * raised, long labels wrapped by word. `trailing` adds a last cell, such as a field for a value the options lack.
 */
export function Segmented<T extends string>({
  value,
  onValueChange,
  options,
  dense,
  className,
  ariaLabel,
  testId,
  trailing,
}: SegmentedProps<T>) {
  return (
    <div
      role="radiogroup"
      aria-label={ariaLabel}
      data-testid={testId}
      className={cn("bg-surface-sunken border-border flex rounded-md border p-0.75", className)}
    >
      {options.map((o) => {
        const active = o.value === value;
        return (
          <button
            key={o.value}
            type="button"
            role="radio"
            aria-checked={active}
            aria-disabled={o.disabled || undefined}
            disabled={o.disabled}
            onClick={() => !o.disabled && onValueChange(o.value)}
            data-state={active ? "on" : "off"}
            data-testid={`${testId ?? "segmented"}-${o.value}`}
            className={cn(
              "inline-flex min-w-0 flex-1 items-center justify-center rounded-sm font-medium transition-[background-color,box-shadow,color] outline-none [&>svg]:shrink-0",
              dense
                ? "gap-1.5 px-2.5 py-1.5 text-xs"
                : "gap-1 px-2 py-2 text-sm sm:gap-1.5 sm:px-3.5",
              active
                ? "bg-surface-elevated text-ink shadow-xs ring-1 ring-border"
                : o.disabled
                  ? "text-ink-dim cursor-not-allowed opacity-50"
                  : "text-ink-dim hover:text-ink cursor-pointer",
              "focus-visible:ring-3 focus-visible:ring-ring/70",
            )}
          >
            {o.icon}
            <span className="min-w-0 text-center text-balance [overflow-wrap:anywhere]">
              {o.label}
            </span>
          </button>
        );
      })}
      {trailing}
    </div>
  );
}
