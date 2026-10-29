import type { ReactNode } from "react";

import { cn } from "@/shared/lib/utils";

/** A two-column list of label and value rows: muted labels on the left, bold values on the right, 12px type. */
export function DetailRows({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <dl
      className={cn("m-0 grid grid-cols-[minmax(0,1fr)_auto] gap-x-2 gap-y-1 text-xs", className)}
    >
      {children}
    </dl>
  );
}

interface DetailRowProps {
  label: ReactNode;
  children: ReactNode;
  tone?: "danger";
  testId?: string;
}

/** One row of DetailRows; `danger` marks a value the viewer falls short of. */
export function DetailRow({ label, children, tone, testId }: DetailRowProps) {
  return (
    <>
      <dt className="text-ink-mute truncate">{label}</dt>
      <dd
        data-testid={testId}
        className={cn(
          "m-0 flex items-center justify-end gap-1 font-semibold tabular-nums",
          tone === "danger" ? "text-danger" : "text-ink",
        )}
      >
        {children}
      </dd>
    </>
  );
}
