import { useEffect, useMemo, useRef, useState } from "react";

import { useReducedMotion } from "@/shared/hooks/useReducedMotion";

import { ringDrainStyle } from "../../lib/turnCountdown";
import { IconX } from "./icons";

interface AutoCloseRingProps {
  duration?: number;
  onClose: () => void;
  paused?: boolean;
  testId?: string;
  ariaLabel?: string;
}

const RING_RADIUS = 9;
const RING_CIRC = 2 * Math.PI * RING_RADIUS;

/**
 * Circular X button whose progress ring drains over `duration` seconds, in step with the close timer.
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
        width: 32,
        height: 32,
        borderRadius: "50%",
        background: "rgba(0,0,0,0.3)",
        border: "1px solid rgba(201,168,118,0.35)",
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
        viewBox="0 0 24 24"
        aria-hidden
        style={{
          position: "absolute",
          inset: -2,
          transform: "rotate(-90deg)",
          pointerEvents: "none",
        }}
      >
        <circle
          cx="12"
          cy="12"
          r={RING_RADIUS}
          fill="none"
          stroke="rgba(255,255,255,0.1)"
          strokeWidth="1.5"
        />
        <circle
          data-testid={`${testId}-arc`}
          cx="12"
          cy="12"
          r={RING_RADIUS}
          fill="none"
          stroke="#d4d0c4"
          strokeWidth="1.5"
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
