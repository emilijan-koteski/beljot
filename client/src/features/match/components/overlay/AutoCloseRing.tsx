import { useEffect, useMemo, useRef, useState } from "react";

import { useReducedMotion } from "@/shared/hooks/useReducedMotion";

import { ringDrainStyle } from "../../lib/turnCountdown";
import { IconX } from "./icons";

interface AutoCloseRingProps {
  /** Total countdown duration in seconds. Default 8 (informational reveals). */
  duration?: number;
  /** Callback fired when the countdown reaches 0 OR the user clicks the X.
   *  Guaranteed to fire at most once per mount. */
  onClose: () => void;
  /** When true, no countdown runs — useful if the parent wants to display the
   *  ring without auto-firing (e.g. a static preview). */
  paused?: boolean;
  /** Test id override; defaults to `auto-close-ring`. */
  testId?: string;
  /** ARIA label for the dismiss button. */
  ariaLabel?: string;
}

const RING_RADIUS = 9;
const RING_CIRC = 2 * Math.PI * RING_RADIUS;

/**
 * Small circular X button with an SVG progress ring around it. Used by every
 * informational overlay (trump-taken, belot/declarations reveals, hand-end,
 * match-end) for the unified "auto-closes in 8 s, but you can close it
 * earlier" pattern.
 *
 * The countdown sweeps full → empty over `duration`. When the duration
 * elapses OR the user clicks the X, `onClose()` fires exactly once.
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
  // Wrap `onClose` in a ref so the firing-timer effect doesn't re-run when the
  // parent re-creates the callback identity mid-reveal.
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

  // Mount-anchored drain: the sweep and the fire timer above start on the
  // same commit and share the same duration, so the ring reads empty at the
  // exact moment the close fires — never one tick behind it. Stable values ⇒
  // re-renders don't restart the animation.
  const drainStyle = useMemo(
    () => ringDrainStyle(duration * 1000, duration * 1000, RING_CIRC),
    [duration],
  );

  // Reduced-motion fallback: no drain animation, but the ring must still
  // reflect the countdown — step the static dashoffset once per second,
  // anchored to the same mount instant as the fire timer.
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
        color: "var(--ink-light, #f5f2e8)",
        cursor: "pointer",
        display: "inline-flex",
        alignItems: "center",
        justifyContent: "center",
        padding: 0,
        touchAction: "manipulation",
      }}
    >
      {/* Invisible hit-area extender: the visible circle is 32px, below the
          ~44px touch-target minimum, which made the X frustratingly precise to
          tap on phones. Clicks on any child of a <button> fire its onClick, so
          this pad widens the effective target to 48×48 without moving a pixel
          of the visuals. */}
      <span aria-hidden style={{ position: "absolute", inset: -8 }} />
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
