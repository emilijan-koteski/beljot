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
};

/**
 * Segmented radio-group toggle used for the parchment option controls.
 * The active option is raised out of a sunken well; disabled options are dimmed and inert.
 */
export function Segmented<T extends string>({
  value,
  onValueChange,
  options,
  dense,
  className,
  ariaLabel,
  testId,
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
              "inline-flex flex-1 items-center justify-center gap-1.5 rounded-sm font-medium transition-[background-color,box-shadow,color] outline-none",
              dense ? "px-2.5 py-1.5 text-[12.5px]" : "px-3.5 py-2 text-[13px]",
              active
                ? "bg-surface-elevated text-ink shadow-[0_1px_2px_rgba(14,58,36,0.06),0_0_0_1px_var(--border)]"
                : o.disabled
                  ? "text-ink-off cursor-not-allowed opacity-55"
                  : "text-ink-dim hover:text-ink cursor-pointer",
              "focus-visible:ring-3 focus-visible:ring-ring/70",
            )}
          >
            {o.icon}
            {o.label}
          </button>
        );
      })}
    </div>
  );
}
