import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { MOTION } from "@/shared/lib/motion";

import { URGENT_FRACTION, useRingDrain, useTurnCountdown } from "../lib/turnCountdown";

interface TimerRingProps {
  turnExpiresAt: string | null;
  totalDuration: number; // total timer duration in seconds (for ring progress)
  size?: "seat" | "seatCompact" | "button";
  /**
   * Suppress the inner seconds label. The ring still renders. Used by
   * [PlayerSeat] which displays the seconds outside the avatar (next to the
   * team label in the name pill) so they don't overlap the avatar initial.
   */
  hideLabel?: boolean;
  /**
   * Pre-computed seconds remaining. When provided, TimerRing skips its own
   * useTurnCountdown subscription and renders against the parent's value.
   * [PlayerSeat] uses this to share a single ticker between the seat-pill
   * label and the ring instead of running two parallel intervals.
   */
  secondsLeft?: number;
}

const SIZE_CONFIG = {
  // 70px sits inside the avatar's 80px outer frame (avatar 64 + 16 padding)
  // so the countdown traces *just inside* the gold/silver team ring rather
  // than around it. Combined with the active-state outer lime ring on the
  // frame's box-shadow, this gives a 3-band stack: outer lime · gold ·
  // inner lime timer.
  seat: { px: 70, strokeWidth: 2.5, labelClass: "text-xs" },
  // Phone seat: the avatar disc shrinks to 44px (frame 60px), so the ring
  // shrinks to 50px to keep tracing *just inside* the team frame — the same
  // proportion the 70px ring has inside the desktop 80px frame.
  seatCompact: { px: 50, strokeWidth: 2, labelClass: "text-[10px]" },
  button: { px: 36, strokeWidth: 2, labelClass: "text-[10px]" },
} as const;

const COLOR_LIME = "var(--turn-lime, #00e5a0)";
const COLOR_URGENT = "var(--turn-urgent, #ef4444)";

/**
 * SVG countdown ring around the active player's avatar (or wrapped around an
 * action button via `size="button"`).
 *
 * Color channel:
 *  • lime  → plenty of time
 *  • red   → ≤1/8 of `totalDuration` remains, including 0s (expired)
 *
 * The ring transitions stroke + dashoffset on a 1s linear sweep so the
 * countdown reads continuously rather than ticking. A drop-shadow glow on the
 * progress arc lifts it off the felt without needing a second stroke.
 */
export function TimerRing({
  turnExpiresAt,
  totalDuration,
  size = "seat",
  hideLabel = false,
  secondsLeft: secondsLeftProp,
}: TimerRingProps) {
  // When the parent already runs useTurnCountdown (PlayerSeat does, for the
  // seat-pill label), accept its value to avoid a second interval. Hooks
  // can't be conditional, so we always call useTurnCountdown — but ignore its
  // value when secondsLeftProp is provided. The hook short-circuits cheaply
  // when turnExpiresAt is null (no interval is started).
  const internalSecondsLeft = useTurnCountdown(
    secondsLeftProp === undefined ? turnExpiresAt : null,
  );
  const secondsLeft = secondsLeftProp ?? internalSecondsLeft;
  const prefersReducedMotion = useReducedMotion();

  const { px, strokeWidth, labelClass } = SIZE_CONFIG[size];
  const radius = (px - strokeWidth) / 2;
  const circumference = 2 * Math.PI * radius;

  // Deadline-anchored sweep: one animation drains the arc to empty exactly at
  // `turnExpiresAt`. The quantized dashOffset below stays as the
  // reduced-motion fallback (stepped once per second, no animation).
  const drainStyle = useRingDrain(turnExpiresAt, totalDuration, circumference);

  if (!turnExpiresAt) {
    return null;
  }

  const progress = totalDuration > 0 ? Math.min(1, Math.max(0, secondsLeft / totalDuration)) : 0;
  const dashOffset = circumference * (1 - progress);

  // Single threshold: under URGENT_FRACTION (1/8) of the original duration the
  // ring flips to red. 0s remaining still reads as red because progress = 0.
  const isUrgent = totalDuration > 0 && progress <= URGENT_FRACTION;
  const strokeColor = isUrgent ? COLOR_URGENT : COLOR_LIME;
  // Pulse the ring while urgent so the urgency reads even at a glance. Only
  // applied while the timer hasn't fully expired so a parked 0 doesn't
  // pulsate forever in edge cases.
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
        {/* Background ring */}
        <circle
          cx={px / 2}
          cy={px / 2}
          r={radius}
          fill="none"
          stroke="rgba(255,255,255,0.15)"
          strokeWidth={strokeWidth}
        />
        {/* Progress ring — drop-shadow glow lifts the arc off the felt.
            The drain animation owns stroke-dashoffset while motion is
            allowed; the inline dashOffset is the reduced-motion fallback.
            Keyed by the deadline: a running CSS animation keeps its original
            start time even when animation-delay changes, so an in-place
            deadline change (same seat active twice in a row) must recreate
            the element to re-anchor the sweep. */}
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
            filter: `drop-shadow(0 0 8px ${strokeColor})`,
            transition: `stroke ${MOTION.RING_COLOR_FLIP}ms ease`,
            strokeDashoffset: dashOffset,
            ...(prefersReducedMotion ? undefined : drainStyle),
          }}
          className="motion-reduce:transition-none"
        />
      </svg>
      {!hideLabel && (
        <span
          className={`absolute font-body font-semibold tabular-nums ${labelClass}`}
          style={{ color: strokeColor, transition: `color ${MOTION.RING_COLOR_FLIP}ms ease` }}
          data-testid="timer-seconds"
        >
          {secondsLeft}
        </span>
      )}
    </div>
  );
}
