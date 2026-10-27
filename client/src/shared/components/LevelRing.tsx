import { useReducedMotion } from "@/shared/hooks/useReducedMotion";

interface LevelRingProps {
  level: number;
  fraction: number;
  label: string;
  size?: number;
  testId?: string;
}

/** Compact lifetime-level ring for the top nav on small screens: an XP-progress arc with the level centered. */
export function LevelRing({ level, fraction, label, size = 32, testId }: LevelRingProps) {
  const reducedMotion = useReducedMotion();

  const stroke = 3;
  const r = (size - stroke) / 2;
  const circumference = 2 * Math.PI * r;
  const clamped = Math.min(1, Math.max(0, fraction));
  const filled = circumference * clamped;

  return (
    <div
      className="relative flex shrink-0 items-center justify-center"
      style={{ width: size, height: size }}
      role="progressbar"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={Math.round(clamped * 100)}
      aria-label={label}
      data-testid={testId}
    >
      <svg
        width={size}
        height={size}
        className="absolute inset-0"
        style={{ transform: "rotate(-90deg)" }}
        aria-hidden="true"
      >
        <circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          fill="none"
          stroke="var(--surface-3)"
          strokeWidth={stroke}
        />
        <circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          fill="none"
          stroke="var(--accent)"
          strokeWidth={stroke}
          strokeLinecap="round"
          strokeDasharray={`${filled} ${circumference}`}
          style={{ transition: reducedMotion ? undefined : "stroke-dasharray .8s ease" }}
        />
      </svg>
      <span className="text-ink text-xs leading-none font-bold tabular-nums">{level}</span>
    </div>
  );
}
