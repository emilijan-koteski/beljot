import { type ReactNode, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";

import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { remainingMsUntil } from "@/shared/lib/clockSync";
import { MOTION } from "@/shared/lib/motion";

import {
  ringDrainStyle,
  URGENT_FRACTION,
  useRingDrain,
  useTurnCountdown,
} from "../../lib/turnCountdown";

interface ButtonTimerRingProps {
  /**
   * Either a wall-clock expiry (`turnExpiresAt` ISO string) **or** a
   * client-side countdown duration in seconds. Server-driven prompts use
   * `turnExpiresAt + totalDuration` so the ring stays in sync across clients;
   * client-side reveals (e.g. score continue) just pass `duration` instead.
   */
  turnExpiresAt?: string | null;
  /** Total ring duration in seconds — drives the dasharray sweep. */
  totalDuration: number;
  /**
   * When set without `turnExpiresAt`, the ring counts down purely client-side
   * from `totalDuration` to 0. Useful for non-server-bound timers.
   */
  clientCountdown?: boolean;
  /**
   * Callback fired the moment the ring reaches 0. Used by reveals that wrap
   * a Continue button — when the timer expires we auto-fire the action so an
   * AFK player doesn't get stuck on the reveal screen.
   */
  onExpire?: () => void;
  /** Corner radius of the wrapped button (in px). Default 8 (matches
   *  ClassicButton). */
  radius?: number;
  /** Reduced-motion hint — when true, skip the SVG rendering entirely. */
  hideRing?: boolean;
  children: ReactNode;
}

/**
 * Wraps a button with an SVG rounded-rect outline countdown that traces the
 * button's border, sweeping clockwise from the top.
 *
 * Two channels:
 *   • lime  → plenty of time
 *   • red   → ≤1/8 remaining (urgent — same threshold as PlayerSeat ring)
 *
 * The wrapper measures the wrapped button via ResizeObserver so the sweep
 * traces the actual button shape regardless of label width / padding.
 */
export function ButtonTimerRing({
  turnExpiresAt,
  totalDuration,
  clientCountdown = false,
  onExpire,
  radius = 8,
  hideRing = false,
  children,
}: ButtonTimerRingProps) {
  const wrapRef = useRef<HTMLDivElement | null>(null);
  const [size, setSize] = useState({ w: 0, h: 0 });

  // Two countdown sources — server expiry (clock-offset corrected,
  // deadline-aligned ticks via the shared hook) or a client-only seconds
  // counter anchored at mount.
  const serverSecondsLeft = useTurnCountdown(turnExpiresAt ?? null);
  const [clientSecondsLeft, setClientSecondsLeft] = useState(totalDuration);

  useEffect(() => {
    if (turnExpiresAt || !clientCountdown) return;
    setClientSecondsLeft(totalDuration);
    const id = setInterval(() => {
      setClientSecondsLeft((s) => {
        if (s <= 0) {
          clearInterval(id);
          return 0;
        }
        return s - 1;
      });
    }, MOTION.COUNTDOWN_TICK);
    return () => clearInterval(id);
  }, [turnExpiresAt, totalDuration, clientCountdown]);

  const secondsLeft = turnExpiresAt
    ? serverSecondsLeft
    : clientCountdown
      ? clientSecondsLeft
      : totalDuration;

  // Fire onExpire exactly once when secondsLeft hits 0.
  const expireFiredRef = useRef(false);
  useEffect(() => {
    if (secondsLeft <= 0 && !expireFiredRef.current && (turnExpiresAt || clientCountdown)) {
      // A deadline that just arrived (null → set, or value swap) renders one
      // commit with the previous countdown state — re-verify against the
      // wall clock so a stale 0 can't fire the expiry for a live deadline.
      if (turnExpiresAt && remainingMsUntil(turnExpiresAt) > 0) return;
      expireFiredRef.current = true;
      onExpire?.();
    }
    if (secondsLeft > 0) expireFiredRef.current = false;
  }, [secondsLeft, onExpire, turnExpiresAt, clientCountdown]);

  // Re-measure the wrapped button so the SVG rect always matches the button's
  // actual painted size (the scene is also CSS-scaled, but offsetWidth gives
  // un-scaled px which is what stroke-dasharray needs).
  useLayoutEffect(() => {
    const measure = () => {
      const el = wrapRef.current;
      if (!el) return;
      setSize({ w: el.offsetWidth, h: el.offsetHeight });
    };
    measure();
    const ro = new ResizeObserver(measure);
    if (wrapRef.current) ro.observe(wrapRef.current);
    return () => ro.disconnect();
  }, []);

  const pct = totalDuration > 0 ? Math.min(1, Math.max(0, secondsLeft) / totalDuration) : 0;
  const isUrgent = totalDuration > 0 && pct <= URGENT_FRACTION;
  // Cream / red palette to match the AutoCloseRing on info-toast X buttons
  // (the avatar's lime/red ring stays the primary turn signal — dialogs use
  // this softer pair so two countdown channels don't compete for attention).
  const stroke = isUrgent ? "var(--turn-urgent, #ef4444)" : "#d4d0c4";
  const pad = 4;
  const strokeW = 1.75;
  const w = size.w + pad * 2;
  const h = size.h + pad * 2;
  // Approximate rounded-rect perimeter — good enough for a visually consistent
  // dasharray, no need to be analytically perfect.
  const perim = 2 * (w + h) - 8 * radius + 2 * Math.PI * radius;

  // Deadline-anchored sweep (server mode) or mount-anchored sweep (client
  // mode): the arc reaches empty exactly when the countdown truly hits 0.
  // The quantized dashoffset below remains the reduced-motion fallback.
  const prefersReducedMotion = useReducedMotion();
  const serverDrain = useRingDrain(turnExpiresAt ?? null, totalDuration, perim);
  const clientDrain = useMemo(
    () =>
      !turnExpiresAt && clientCountdown && totalDuration > 0
        ? ringDrainStyle(totalDuration * 1000, totalDuration * 1000, perim)
        : undefined,
    [turnExpiresAt, clientCountdown, totalDuration, perim],
  );
  const drainStyle = prefersReducedMotion ? undefined : (serverDrain ?? clientDrain);

  return (
    <div
      ref={wrapRef}
      className="relative inline-block"
      data-testid="button-timer-ring"
      data-urgent={isUrgent ? "true" : "false"}
    >
      {children}
      {!hideRing && size.w > 0 && (
        <svg
          width={w}
          height={h}
          aria-hidden
          style={{
            position: "absolute",
            top: -pad,
            left: -pad,
            pointerEvents: "none",
            overflow: "visible",
          }}
        >
          <rect
            x={strokeW / 2}
            y={strokeW / 2}
            width={w - strokeW}
            height={h - strokeW}
            rx={radius + pad - strokeW / 2}
            ry={radius + pad - strokeW / 2}
            fill="none"
            stroke="rgba(255,255,255,0.12)"
            strokeWidth={strokeW}
          />
          {/* Keyed by the countdown source: a running CSS animation keeps
              its original start time even when animation-delay changes, so a
              deadline swap must recreate the element to re-anchor the sweep. */}
          <rect
            key={turnExpiresAt ?? (clientCountdown ? "client" : "static")}
            x={strokeW / 2}
            y={strokeW / 2}
            width={w - strokeW}
            height={h - strokeW}
            rx={radius + pad - strokeW / 2}
            ry={radius + pad - strokeW / 2}
            fill="none"
            stroke={stroke}
            strokeWidth={strokeW}
            strokeLinecap="round"
            strokeDasharray={perim}
            style={{
              transition: `stroke ${MOTION.RING_COLOR_FLIP_FAST}ms ease-out`,
              strokeDashoffset: perim * (1 - pct),
              ...drainStyle,
            }}
          />
        </svg>
      )}
    </div>
  );
}
