import { useEffect } from "react";

import { playSfx } from "@/shared/audio/audioEngine";
import { remainingMsUntil } from "@/shared/lib/clockSync";

import { isCountdownUrgent, useTurnCountdown } from "./turnCountdown";

/**
 * Plays one clock tick per whole second while the viewer's own deadline is urgent, down to 1.
 * Each tick is keyed by deadline and second, so remounts and deadline swaps never replay a tick.
 */
export function useUrgentTick(deadline: string | null, totalSec: number): void {
  const secondsLeft = useTurnCountdown(deadline);
  const ticking = deadline !== null && secondsLeft > 0 && isCountdownUrgent(secondsLeft, totalSec);
  useEffect(() => {
    if (!ticking || deadline === null) return;
    const secondIsStaleFromPreviousDeadline =
      Math.ceil(remainingMsUntil(deadline) / 1000) !== secondsLeft;
    if (secondIsStaleFromPreviousDeadline) return;
    playSfx("clockTick", { dedupeKey: `${deadline}:${secondsLeft}` });
  }, [ticking, deadline, secondsLeft]);
}
