import { useReducedMotion } from "@/shared/hooks/useReducedMotion";

interface LevelRingProps {
  /** Lifetime level shown in the center (server-authoritative). */
  level: number;
  /** XP progress within the current level, 0..1 (cosmetic display math). */
  fraction: number;
  /** Accessible label, e.g. "Level 2, 8 / 250 XP". */
  label: string;
  /** Outer diameter in px. */
  size?: number;
  testId?: string;
}

/**
 * Compact lifetime-level ring for the top nav on small screens — the profile
 * WinRateRing motif shrunk down: a felt-green XP-progress arc over a sunken
 * track with the level number centered. Shown below the `sm` breakpoint, where
 * the wider "Lvl N + bar" indicator is hidden so it doesn't crowd the coin pill.
 * The arc fill is cosmetic; the level is server-authoritative.
 */
export function LevelRing({ level, fraction, label, size = 30, testId }: LevelRingProps) {
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
      <span className="text-ink font-display text-[11px] leading-none font-bold tabular-nums">
        {level}
      </span>
    </div>
  );
}
