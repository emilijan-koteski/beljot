import { useEffect, useRef, useState } from "react";

import { MOTION } from "@/shared/lib/motion";

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
  const [pct, setPct] = useState(1);
  const firedRef = useRef(false);
  // Wrap `onClose` in a ref so the firing-timer effect doesn't re-run when the
  // parent re-creates the callback identity mid-reveal.
  const onCloseRef = useRef(onClose);
  useEffect(() => {
    onCloseRef.current = onClose;
  }, [onClose]);

  useEffect(() => {
    if (paused) return;
    // One fire timer for the close action, plus a per-second tick for the
    // visual sweep. Splitting them means the close trigger isn't dependent
    // on the per-second state-update chain — this matters under fake timers.
    const deadline = Date.now() + duration * 1000;
    const updatePct = () => {
      const remainingMs = Math.max(0, deadline - Date.now());
      setPct(remainingMs / (duration * 1000));
    };
    updatePct();

    const tickId = setInterval(updatePct, MOTION.COUNTDOWN_TICK);
    const fireId = setTimeout(() => {
      if (firedRef.current) return;
      firedRef.current = true;
      setPct(0);
      onCloseRef.current();
    }, duration * 1000);

    return () => {
      clearInterval(tickId);
      clearTimeout(fireId);
    };
  }, [duration, paused]);

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
          cx="12"
          cy="12"
          r={RING_RADIUS}
          fill="none"
          stroke="#d4d0c4"
          strokeWidth="1.5"
          strokeLinecap="round"
          strokeDasharray={RING_CIRC}
          strokeDashoffset={RING_CIRC * (1 - pct)}
          style={{ transition: "stroke-dashoffset 1s linear" }}
        />
      </svg>
    </button>
  );
}
