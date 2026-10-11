import { type CSSProperties, useEffect, useMemo, useState } from "react";

import { remainingMsUntil } from "@/shared/lib/clockSync";

const TICK_SLACK_MS = 20;

/**
 * Server-clock-corrected seconds left until `turnExpiresAt`, updated on the deadline's own
 * whole-second boundaries. Returns 0 when `turnExpiresAt` is null or already past.
 */
export function useTurnCountdown(turnExpiresAt: string | null): number {
  const [secondsLeft, setSecondsLeft] = useState(() =>
    turnExpiresAt ? Math.ceil(remainingMsUntil(turnExpiresAt) / 1000) : 0,
  );

  useEffect(() => {
    if (!turnExpiresAt) {
      setSecondsLeft(0);
      return;
    }

    let timeoutId: ReturnType<typeof setTimeout> | undefined;

    const tick = () => {
      const remainingMs = remainingMsUntil(turnExpiresAt);
      setSecondsLeft(Math.ceil(remainingMs / 1000));
      if (remainingMs <= 0) return;
      timeoutId = setTimeout(tick, (remainingMs % 1000 || 1000) + TICK_SLACK_MS);
    };
    tick();

    return () => clearTimeout(timeoutId);
  }, [turnExpiresAt]);

  return secondsLeft;
}

export const URGENT_FRACTION = 0.125;

/**
 * Whether the countdown should read as urgent (≤1/8 of the total duration
 * remains, including 0). Shared so [PlayerSeat] colors the external label
 * the same way the ring does.
 */
export function isCountdownUrgent(secondsLeft: number, totalDuration: number): boolean {
  if (totalDuration <= 0) return false;
  const progress = Math.min(1, Math.max(0, secondsLeft / totalDuration));
  return progress <= URGENT_FRACTION;
}

/**
 * Inline `ring-drain` animation style that sweeps the arc to `ringEmpty` exactly at the
 * deadline. Renderers must `key` the animated element by deadline, since a running animation
 * ignores delay changes.
 */
export function ringDrainStyle(
  totalMs: number,
  remainingMs: number,
  ringEmpty: number | string = 1,
): CSSProperties {
  const sweepMs = Math.max(totalMs, Math.round(remainingMs));
  return {
    "--ring-empty": `${ringEmpty}`,
    animationName: "ring-drain",
    animationDuration: `${sweepMs}ms`,
    animationDelay: `${Math.round(remainingMs) - sweepMs}ms`,
    animationTimingFunction: "linear",
    animationFillMode: "forwards",
  } as CSSProperties;
}

/**
 * Memoized deadline-anchored drain style, so re-renders never restart the animation mid-sweep.
 * Returns undefined when there is no deadline to render.
 */
export function useRingDrain(
  expiresAt: string | null | undefined,
  totalDurationSec: number,
  ringEmpty: number | string = 1,
): CSSProperties | undefined {
  return useMemo(() => {
    if (!expiresAt || totalDurationSec <= 0) return undefined;
    const totalMs = totalDurationSec * 1000;
    return ringDrainStyle(totalMs, remainingMsUntil(expiresAt), ringEmpty);
  }, [expiresAt, totalDurationSec, ringEmpty]);
}
