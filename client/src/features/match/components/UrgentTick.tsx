import { useState } from "react";

import { serverNowMs } from "@/shared/lib/clockSync";

import { useUrgentTick } from "../lib/urgentTick";

interface UrgentTickProps {
  turnDeadline: string | null;
  turnTotalSec: number;
  windowOpen: boolean;
  windowTotalSec: number;
}

/**
 * Renders nothing and drives the urgent tick for the viewer's own deadlines only.
 */
export function UrgentTick({
  turnDeadline,
  turnTotalSec,
  windowOpen,
  windowTotalSec,
}: UrgentTickProps) {
  const [windowDeadline, setWindowDeadline] = useState<string | null>(null);
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
