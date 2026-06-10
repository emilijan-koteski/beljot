import { type CSSProperties, useEffect, useMemo, useState } from "react";

import { remainingMsUntil } from "@/shared/lib/clockSync";

/**
 * Fire each label update just past the whole-second boundary it announces, so
 * a timer callback scheduled "in 1000ms" can't run a hair early and re-read
 * the old second. Small enough to be invisible.
 */
const TICK_SLACK_MS = 20;

/**
 * Subscribes to the per-second countdown derived from `turnExpiresAt`. Returns
 * `0` when `turnExpiresAt` is null or already past. Pulled out so [PlayerSeat]
 * can render the seconds outside the ring without forcing a second source of
 * truth.
 *
 * Truthfulness contract: remaining time is measured against the server's
 * clock (clockSync offset correction), and updates are scheduled on the
 * deadline's own whole-second boundaries — not a free-running interval — so
 * the label flips exactly when a true second elapses and reads 0 at the
 * deadline itself. The server's auto-action fires a grace period AFTER the
 * advertised deadline (`expiryGrace`, server side), so players always see 0
 * before the server acts in their name.
 */
export function useTurnCountdown(turnExpiresAt: string | null): number {
  // Lazy init computes the true value for the very first render — consumers
  // gate auto-actions on `secondsLeft <= 0` (ButtonTimerRing's onExpire), so
  // a transient 0 before the first effect would fire them spuriously.
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

// Urgency threshold expressed as a fraction of `totalDuration`. When ≤1/8 of
// the turn timer remains, the ring + label flip from lime to red. The flip is
// independent of team identity (Gold/Silver carry that channel separately).
// Shared so ButtonTimerRing (and any future countdown widget) can stay in
// lockstep instead of redeclaring 0.125 as a magic literal.
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
 * Inline style driving the shared `ring-drain` keyframe (index.css): a single
 * wall-clock-anchored animation sweeps the arc from its current fill to
 * empty, reaching empty exactly at the deadline. The negative delay anchors
 * the sweep — a ring mounted mid-window resumes at the right point instead
 * of restarting.
 *
 * This replaces the old per-tick `stroke-dashoffset` transitions, which drew
 * the arc one full tick behind the truth (the visible reason every loader
 * appeared to fire ~1s early).
 *
 * CSS caveat: changing animation-delay on an element whose animation is
 * already RUNNING does not move its start time — the browser re-evaluates
 * progress against the original start. Renderers must therefore `key` the
 * animated SVG element by the deadline so an in-place deadline change
 * recreates the element and the animation re-anchors.
 *
 * `ringEmpty` is the fully-drained dashoffset for this ring's geometry: the
 * circumference of a circle, a rect's perimeter, or 1 when normalized via
 * `pathLength`. Callers keep their static `strokeDashoffset` as the
 * reduced-motion / pre-animation fallback; while the animation runs it wins
 * over the inline value.
 */
export function ringDrainStyle(
  totalMs: number,
  remainingMs: number,
  ringEmpty: number | string = 1,
): CSSProperties {
  // End-anchored under skew: if corrected remaining exceeds the nominal
  // window, stretch the sweep over the longer remaining rather than clamping
  // the start — the arc must reach empty AT the deadline, never before it.
  const sweepMs = Math.max(totalMs, Math.round(remainingMs));
  return {
    "--ring-empty": `${ringEmpty}`,
    animationName: "ring-drain",
    animationDuration: `${sweepMs}ms`,
    // delay ≤ 0 by construction: an already-past deadline parks the ring
    // empty (fill-mode forwards).
    animationDelay: `${Math.round(remainingMs) - sweepMs}ms`,
    animationTimingFunction: "linear",
    animationFillMode: "forwards",
  } as CSSProperties;
}

/**
 * Deadline-anchored drain style for server-driven countdowns. Memoized per
 * (deadline, duration) so re-renders reuse the identical style object and the
 * browser never restarts the animation mid-sweep; a remount recomputes the
 * delay and resumes at the correct point. Returns undefined when there is no
 * deadline to render (caller falls back to its static arc).
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
