import { useState } from "react";

import { serverNowMs } from "@/shared/lib/clockSync";

import { useUrgentTick } from "../lib/urgentTick";

interface UrgentTickProps {
  /** The viewer's own server deadline (their turn, bid, or a prompt on it),
   *  or null when nothing is theirs to decide. */
  turnDeadline: string | null;
  turnTotalSec: number;
  /** The dedicated declaration window is open on the viewer's screen and they
   *  have not answered it. It has no server deadline: its countdown starts
   *  when the prompt appears, so this starts one at the same moment. */
  windowOpen: boolean;
  windowTotalSec: number;
}

/**
 * The single urgent-tick driver for the match page. Renders nothing. Other
 * seats' timers, auto-close rings, the score reveal and the reconnect ring
 * never reach it, so they never tick.
 */
export function UrgentTick({
  turnDeadline,
  turnTotalSec,
  windowOpen,
  windowTotalSec,
}: UrgentTickProps) {
  const [windowDeadline, setWindowDeadline] = useState<string | null>(null);
  // Captured during render (the "adjust state on prop change" pattern) so it
  // is anchored to the same commit that mounts the declaration prompt.
  if (windowOpen && windowDeadline === null) {
    setWindowDeadline(new Date(serverNowMs() + windowTotalSec * 1000).toISOString());
  } else if (!windowOpen && windowDeadline !== null) {
    setWindowDeadline(null);
  }
  const useWindow = turnDeadline === null && windowOpen;
  useUrgentTick(
    useWindow ? windowDeadline : turnDeadline,
    useWindow ? windowTotalSec : turnTotalSec,
  );
  return null;
}
