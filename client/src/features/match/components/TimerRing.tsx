import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { MOTION } from "@/shared/lib/motion";

import { seatInnerPx } from "../lib/seatGeometry";
import { URGENT_FRACTION, useRingDrain, useTurnCountdown } from "../lib/turnCountdown";

interface TimerRingProps {
  turnExpiresAt: string | null;
  totalDuration: number;
  size?: "seat" | "seatCompact" | "button";
  hideLabel?: boolean;
  secondsLeft?: number;
}

const SIZE_CONFIG = {
  seat: { px: seatInnerPx(false), strokeWidth: 2.5, labelClass: "text-xs" },
  seatCompact: { px: seatInnerPx(true), strokeWidth: 2, labelClass: "text-2xs" },
  button: { px: 36, strokeWidth: 2, labelClass: "text-2xs" },
} as const;

const COLOR_LIME = "var(--accent-light)";
const COLOR_URGENT = "var(--danger)";

/**
 * SVG countdown ring for a seat avatar, drawn flush inside the seat's team-colour border, or around a button.
 * Turns red once at most 1/8 of `totalDuration` remains.
 */
export function TimerRing({
  turnExpiresAt,
  totalDuration,
  size = "seat",
  hideLabel = false,
  secondsLeft: secondsLeftProp,
}: TimerRingProps) {
  const internalSecondsLeft = useTurnCountdown(
    secondsLeftProp === undefined ? turnExpiresAt : null,
  );
  const secondsLeft = secondsLeftProp ?? internalSecondsLeft;
  const prefersReducedMotion = useReducedMotion();

  const { px, strokeWidth, labelClass } = SIZE_CONFIG[size];
  const radius = (px - strokeWidth) / 2;
  const circumference = 2 * Math.PI * radius;

  const drainStyle = useRingDrain(turnExpiresAt, totalDuration, circumference);

  if (!turnExpiresAt) {
    return null;
  }

  const progress = totalDuration > 0 ? Math.min(1, Math.max(0, secondsLeft / totalDuration)) : 0;
  const dashOffset = circumference * (1 - progress);

  const isUrgent = totalDuration > 0 && progress <= URGENT_FRACTION;
  const strokeColor = isUrgent ? COLOR_URGENT : COLOR_LIME;
  const pulseClass = isUrgent && secondsLeft > 0 ? "motion-safe:animate-pulse" : "";

  return (
    <div
      className={`absolute inset-0 flex items-center justify-center pointer-events-none ${pulseClass}`}
      data-testid="timer-ring"
      data-size={size}
      data-urgent={isUrgent ? "true" : "false"}
      aria-label={`${secondsLeft} seconds remaining`}
    >
      <svg
        width={px}
        height={px}
        className="motion-safe:transition-all motion-safe:duration-1000 motion-reduce:transition-none"
      >
        <circle
          cx={px / 2}
          cy={px / 2}
          r={radius}
          fill="none"
          stroke="var(--sheen-15)"
          strokeWidth={strokeWidth}
        />
        <circle
          key={turnExpiresAt}
          data-testid="timer-ring-arc"
          cx={px / 2}
          cy={px / 2}
          r={radius}
          fill="none"
          stroke={strokeColor}
          strokeWidth={strokeWidth}
          strokeDasharray={circumference}
          strokeLinecap="round"
          transform={`rotate(-90 ${px / 2} ${px / 2})`}
          style={{
            filter: `drop-shadow(0 0 6px ${strokeColor})`,
            transition: `stroke ${MOTION.RING_COLOR_FLIP}ms ease`,
            strokeDashoffset: dashOffset,
            ...(prefersReducedMotion ? undefined : drainStyle),
          }}
          className="motion-reduce:transition-none"
        />
      </svg>
      {!hideLabel && (
        <span
          className={`absolute font-semibold tabular-nums ${labelClass}`}
          style={{ color: strokeColor, transition: `color ${MOTION.RING_COLOR_FLIP}ms ease` }}
          data-testid="timer-seconds"
        >
          {secondsLeft}
        </span>
      )}
    </div>
  );
}
