import { useEffect } from "react";

import { playSfx } from "@/shared/audio/audioEngine";
import { remainingMsUntil } from "@/shared/lib/clockSync";

import { isCountdownUrgent, useTurnCountdown } from "./turnCountdown";

/**
 * Clock tick for the viewer's own decision: once the deadline is in the
 * countdown ring's red zone (≤ URGENT_FRACTION of the window, the same test
 * TimerRing uses), one tick per whole second, down to 1. Silent at 0 — the
 * server acts then — and silent with no deadline.
 *
 * Each tick is keyed by deadline and second, so a remount into an already-red
 * countdown only ticks the seconds that are still to come. The second is
 * re-read from the clock before ticking: when one deadline replaces another,
 * the countdown hook reports the old deadline's second for one render, and
 * that must not tick (or use up) the new deadline's second.
 */
export function useUrgentTick(deadline: string | null, totalSec: number): void {
  const secondsLeft = useTurnCountdown(deadline);
  const ticking = deadline !== null && secondsLeft > 0 && isCountdownUrgent(secondsLeft, totalSec);
  useEffect(() => {
    if (!ticking || deadline === null) return;
    if (Math.ceil(remainingMsUntil(deadline) / 1000) !== secondsLeft) return;
    playSfx("clockTick", { dedupeKey: `${deadline}:${secondsLeft}` });
  }, [ticking, deadline, secondsLeft]);
}
