import type { ReactNode } from "react";

/** The brass coin the celebrations open with, drawn from tokens; it carries the "B" or a level number. */
export function CoinMedallion({ children, testId }: { children: ReactNode; testId?: string }) {
  return (
    <span
      aria-hidden="true"
      data-testid={testId}
      className="from-brass to-brass-deep text-brass-ink ring-brass-soft flex size-18 items-center justify-center rounded-full bg-linear-to-b text-3xl font-bold tabular-nums shadow-md ring-4"
    >
      {children}
    </span>
  );
}
