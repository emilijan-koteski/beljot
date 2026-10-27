import { type ReactNode, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";

import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { remainingMsUntil } from "@/shared/lib/clockSync";
import { MOTION } from "@/shared/lib/motion";

import { BUTTON_RADIUS_PX } from "../../lib/buttonShape";
import {
  ringDrainStyle,
  TIMER_RING_INK,
  TIMER_RING_STROKE,
  TIMER_RING_TRACK,
  URGENT_FRACTION,
  useRingDrain,
  useTurnCountdown,
} from "../../lib/turnCountdown";

interface ButtonTimerRingProps {
  turnExpiresAt?: string | null;
  totalDuration: number;
  clientCountdown?: boolean;
  onExpire?: () => void;
  radius?: number;
  hideRing?: boolean;
  children: ReactNode;
}

/**
 * Wraps a button with a rounded-rect countdown outline that drains to a server deadline or a client-side timer.
 * Fires `onExpire` once when the countdown reaches 0.
 */
export function ButtonTimerRing({
  turnExpiresAt,
  totalDuration,
  clientCountdown = false,
  onExpire,
  radius = BUTTON_RADIUS_PX,
  hideRing = false,
  children,
}: ButtonTimerRingProps) {
  const wrapRef = useRef<HTMLDivElement | null>(null);
  const [size, setSize] = useState({ w: 0, h: 0 });

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

  const expireFiredRef = useRef(false);
  useEffect(() => {
    if (secondsLeft <= 0 && !expireFiredRef.current && (turnExpiresAt || clientCountdown)) {
      const staleZeroForLiveDeadline = turnExpiresAt && remainingMsUntil(turnExpiresAt) > 0;
      if (staleZeroForLiveDeadline) return;
      expireFiredRef.current = true;
      onExpire?.();
    }
    if (secondsLeft > 0) expireFiredRef.current = false;
  }, [secondsLeft, onExpire, turnExpiresAt, clientCountdown]);

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
  const stroke = isUrgent ? "var(--danger)" : TIMER_RING_INK;
  const pad = 4;
  const strokeW = TIMER_RING_STROKE;
  const w = size.w + pad * 2;
  const h = size.h + pad * 2;
  const perim = 2 * (w + h) - 8 * radius + 2 * Math.PI * radius;

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
  const sweepRestartKey = turnExpiresAt ?? (clientCountdown ? "client" : "static");

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
            stroke={TIMER_RING_TRACK}
            strokeWidth={strokeW}
          />
          <rect
            key={sweepRestartKey}
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
