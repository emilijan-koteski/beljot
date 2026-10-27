import type { ReactNode } from "react";

import { cn } from "@/shared/lib/utils";

interface SummaryStripProps {
  label: ReactNode;
  caption?: ReactNode;
  value: ReactNode;
  valueTestId?: string;
  testId?: string;
  className?: string;
}

/** Brass-tinted bottom line of a dialog: a small-caps label on the left, the figure it sums up on the right. */
export function SummaryStrip({
  label,
  caption,
  value,
  valueTestId,
  testId,
  className,
}: SummaryStripProps) {
  return (
    <div
      className={cn(
        "flex flex-wrap items-center justify-between gap-x-4 gap-y-1 rounded-lg px-4 py-3",
        className,
      )}
      style={{ background: "var(--brass-10)", border: "1px solid var(--brass-25)" }}
      data-testid={testId}
    >
      <div className="flex min-w-0 flex-col gap-0.5">
        <span className="text-2xs tracking-caps uppercase" style={{ color: "var(--brass)" }}>
          {label}
        </span>
        {caption !== undefined && (
          <span className="text-xs" style={{ color: "var(--ink-light)", opacity: 0.65 }}>
            {caption}
          </span>
        )}
      </div>
      <div
        className="flex items-baseline gap-1.5 text-xl font-bold whitespace-nowrap tabular-nums sm:text-2xl"
        style={{ color: "var(--ink-light)" }}
        data-testid={valueTestId}
      >
        {value}
      </div>
    </div>
  );
}
