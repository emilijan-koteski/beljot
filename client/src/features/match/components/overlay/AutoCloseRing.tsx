import { useEffect, useMemo, useRef, useState } from "react";

import { useReducedMotion } from "@/shared/hooks/useReducedMotion";

import {
  ringDrainStyle,
  TIMER_RING_INK,
  TIMER_RING_STROKE,
  TIMER_RING_TRACK,
} from "../../lib/turnCountdown";
import { IconX } from "./icons";

interface AutoCloseRingProps {
  duration?: number;
  onClose: () => void;
  paused?: boolean;
  testId?: string;
  ariaLabel?: string;
}

const BUTTON_PX = 32;
const BUTTON_BORDER_PX = 1;
const RING_RADIUS = BUTTON_PX / 2 - BUTTON_BORDER_PX - TIMER_RING_STROKE / 2;
const RING_CIRC = 2 * Math.PI * RING_RADIUS;
const CENTER = BUTTON_PX / 2;

/**
 * Circular X button whose progress ring, flush inside its border, drains over `duration` seconds.
 * `onClose` fires exactly once, on timeout or click, and never while `paused`.
 */
export function AutoCloseRing({
  duration = 8,
  onClose,
  paused = false,
  testId = "auto-close-ring",
  ariaLabel = "Dismiss",
}: AutoCloseRingProps) {
  const firedRef = useRef(false);
  const prefersReducedMotion = useReducedMotion();
  const onCloseRef = useRef(onClose);
  useEffect(() => {
    onCloseRef.current = onClose;
  }, [onClose]);

  useEffect(() => {
    if (paused) return;
    const fireId = setTimeout(() => {
      if (firedRef.current) return;
      firedRef.current = true;
      onCloseRef.current();
    }, duration * 1000);
    return () => clearTimeout(fireId);
  }, [duration, paused]);

  const drainStyle = useMemo(
    () => ringDrainStyle(duration * 1000, duration * 1000, RING_CIRC),
    [duration],
  );

  const [reducedPct, setReducedPct] = useState(1);
  useEffect(() => {
    if (!prefersReducedMotion || paused) return;
    const totalMs = duration * 1000;
    const deadline = Date.now() + totalMs;
    const update = () => setReducedPct(Math.max(0, (deadline - Date.now()) / totalMs));
    update();
    const id = setInterval(update, 1000);
    return () => clearInterval(id);
  }, [prefersReducedMotion, paused, duration]);

  const handleClick = () => {
    if (firedRef.current) return;
    firedRef.current = true;
    onCloseRef.current();
  };

  return (
    <button
      type="button"
      title={ariaLabel}
      aria-label={ariaLabel}
      onClick={handleClick}
      data-testid={testId}
      style={{
        position: "relative",
        width: BUTTON_PX,
        height: BUTTON_PX,
        borderRadius: "50%",
        background: "var(--shade-25)",
        border: `${BUTTON_BORDER_PX}px solid var(--brass-40)`,
        color: "var(--ink-light)",
        cursor: "pointer",
        display: "inline-flex",
        alignItems: "center",
        justifyContent: "center",
        padding: 0,
        touchAction: "manipulation",
      }}
    >
      <TouchTargetPad />
      <IconX size={14} />
      <svg
        width={BUTTON_PX}
        height={BUTTON_PX}
        viewBox={`0 0 ${BUTTON_PX} ${BUTTON_PX}`}
        aria-hidden
        style={{
          position: "absolute",
          top: -BUTTON_BORDER_PX,
          left: -BUTTON_BORDER_PX,
          transform: "rotate(-90deg)",
          pointerEvents: "none",
        }}
      >
        <circle
          cx={CENTER}
          cy={CENTER}
          r={RING_RADIUS}
          fill="none"
          stroke={TIMER_RING_TRACK}
          strokeWidth={TIMER_RING_STROKE}
        />
        <circle
          data-testid={`${testId}-arc`}
          cx={CENTER}
          cy={CENTER}
          r={RING_RADIUS}
          fill="none"
          stroke={TIMER_RING_INK}
          strokeWidth={TIMER_RING_STROKE}
          strokeLinecap="round"
          strokeDasharray={RING_CIRC}
          style={{
            strokeDashoffset: prefersReducedMotion ? RING_CIRC * (1 - reducedPct) : 0,
            ...(paused || prefersReducedMotion ? undefined : drainStyle),
          }}
        />
      </svg>
    </button>
  );
}

/** Invisible pad that widens the 32px button's tap target to 48x48 without changing its look. */
function TouchTargetPad() {
  return <span aria-hidden style={{ position: "absolute", inset: -8 }} />;
}
