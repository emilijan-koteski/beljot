import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { MOTION } from "@/shared/lib/motion";

import {
  formatCountdown,
  TIMER_RING_INK,
  URGENT_FRACTION,
  useRingDrain,
  useTurnCountdown,
} from "../../lib/turnCountdown";

const RING_DANGER = "var(--danger-light)";
const RING_SIZE = 132;
const RING_STROKE_INSET = 4;
const RING_RADIUS = (RING_SIZE - RING_STROKE_INSET) / 2;
const RING_CIRC = 2 * Math.PI * RING_RADIUS;
const FELT_DARK = "var(--felt-shadow)";

type Props = {
  expiresAt: string;
  totalSeconds: number;
  testId: string;
};

/**
 * The large draining ring of the reconnect and pause overlays, counting down to an absolute server deadline.
 * It turns danger red in its last eighth.
 */
export function CountdownRing({ expiresAt, totalSeconds, testId }: Props) {
  const remainingSeconds = useTurnCountdown(expiresAt);
  const prefersReducedMotion = useReducedMotion();
  const ringDrain = useRingDrain(expiresAt, totalSeconds, RING_CIRC);
  const pct = Math.max(0, Math.min(1, remainingSeconds / totalSeconds));
  const isUrgent = pct <= URGENT_FRACTION;
  const ringColor = isUrgent ? RING_DANGER : TIMER_RING_INK;

  return (
    <div className="relative" style={{ width: RING_SIZE, height: RING_SIZE }}>
      <svg
        width={RING_SIZE}
        height={RING_SIZE}
        viewBox={`0 0 ${RING_SIZE} ${RING_SIZE}`}
        aria-hidden
        style={{ transform: "rotate(-90deg)" }}
      >
        <circle
          cx={RING_SIZE / 2}
          cy={RING_SIZE / 2}
          r={RING_RADIUS}
          fill={FELT_DARK}
          stroke="var(--sheen-10)"
          strokeWidth={2}
        />
        <circle
          key={expiresAt}
          cx={RING_SIZE / 2}
          cy={RING_SIZE / 2}
          r={RING_RADIUS}
          fill="none"
          stroke={ringColor}
          strokeWidth={2}
          strokeLinecap="round"
          strokeDasharray={RING_CIRC}
          style={{
            transition: `stroke ${MOTION.RING_COLOR_FLIP}ms ease-out`,
            strokeDashoffset: RING_CIRC * (1 - pct),
            ...(prefersReducedMotion ? undefined : ringDrain),
          }}
        />
      </svg>
      <div
        className="absolute inset-0 flex items-center justify-center"
        style={{ pointerEvents: "none" }}
      >
        <span
          className="font-semibold tabular-nums"
          style={{
            fontSize: 36,
            color: isUrgent ? RING_DANGER : "var(--ink-light)",
            transition: `color ${MOTION.RING_COLOR_FLIP}ms ease-out`,
          }}
          data-testid={testId}
        >
          {formatCountdown(remainingSeconds)}
        </span>
      </div>
    </div>
  );
}
