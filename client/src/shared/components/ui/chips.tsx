import * as React from "react";

import { cn } from "@/shared/lib/utils";

type ChipSize = "sm" | "md";

type ChipProps = Omit<React.ComponentProps<"button">, "type"> & {
  active: boolean;
  tone?: string;
  size?: ChipSize;
  icon?: React.ReactNode;
  count?: React.ReactNode;
};

const SIZE_CLASS: Record<ChipSize, string> = {
  sm: "h-7 gap-1 px-3 text-xs",
  md: "h-8 gap-1.5 px-3.5 text-sm",
};

/**
 * Interactive chip: a bordered pill that toggles, filters or selects. Active, it takes its tone as
 * text, border and a 12% fill; an optional count trails the label as a small tag.
 */
export function Chip({
  active,
  tone = "var(--accent)",
  size = "md",
  icon,
  count,
  className,
  style,
  children,
  ...rest
}: ChipProps) {
  return (
    <button
      type="button"
      className={cn(
        "inline-flex shrink-0 cursor-pointer items-center rounded-full border whitespace-nowrap tabular-nums transition-[background-color,border-color,color,box-shadow] outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/70 disabled:cursor-not-allowed disabled:opacity-50 [&_svg]:size-3.5 [&_svg]:shrink-0",
        SIZE_CLASS[size],
        active
          ? "font-semibold"
          : "border-border bg-surface-elevated text-ink-dim hover:border-ink-off hover:text-ink font-medium",
        className,
      )}
      style={
        active
          ? {
              color: tone,
              borderColor: tone,
              backgroundColor: `color-mix(in srgb, ${tone} 12%, transparent)`,
              ...style,
            }
          : style
      }
      {...rest}
    >
      {icon}
      {children}
      {count !== undefined && (
        <span
          className={cn(
            "inline-flex h-4 min-w-4 items-center justify-center rounded px-1 text-2xs leading-none font-semibold",
            active ? "bg-current/15" : "bg-surface-sunken text-ink-mute",
          )}
        >
          {count}
        </span>
      )}
    </button>
  );
}

type ChipOption<T extends string | number> = {
  value: T;
  label: React.ReactNode;
  icon?: React.ReactNode;
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
 * Single-choice chip group for a short, named set of values (buy-in, honour gate) where a raw
 * input or a continuous slider would hide the meaningful steps.
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
        return (
          <Chip
            key={String(o.value)}
            role="radio"
            aria-checked={active}
            active={active}
            tone={o.tone}
            icon={o.icon}
            onClick={() => onValueChange(o.value)}
            data-state={active ? "on" : "off"}
            data-testid={`${testId ?? "chips"}-${o.value}`}
          >
            {o.label}
          </Chip>
        );
      })}
      {children}
    </div>
  );
}
