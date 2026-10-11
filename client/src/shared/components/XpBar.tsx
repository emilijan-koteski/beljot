import { cn } from "@/shared/lib/utils";

interface XpBarProps {
  fraction: number;
  label: string;
  className?: string;
  testId?: string;
}

/** Slim lifetime-XP progress bar; clamps the fraction to [0, 1] and exposes the progressbar role. */
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
