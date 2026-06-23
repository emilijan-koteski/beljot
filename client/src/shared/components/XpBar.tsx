import { cn } from "@/shared/lib/utils";

interface XpBarProps {
  /** Fill fraction; clamped to [0, 1] here so callers can pass raw ratios. */
  fraction: number;
  /** Accessible label (e.g. "Level 3, 150 / 350 XP"). */
  label: string;
  className?: string;
  testId?: string;
}

/**
 * Slim lifetime-XP progress bar (Story 9.5). Presentational only — the fill is
 * cosmetic; level + totals are server-authoritative. The accent (green) fill
 * keeps it visually distinct from the gold coin pill. Exposes the standard
 * progressbar a11y role so screen readers announce the percentage.
 */
export function XpBar({ fraction, label, className, testId }: XpBarProps) {
  const pct = Math.round(Math.min(1, Math.max(0, fraction)) * 100);
  return (
    <div
      className={cn("bg-surface-sunken h-1.5 overflow-hidden rounded-full", className)}
      role="progressbar"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={pct}
      aria-label={label}
      data-testid={testId}
    >
      <div
        className="bg-accent h-full rounded-full transition-[width] duration-500 ease-out"
        style={{ width: `${pct}%` }}
      />
    </div>
  );
}
