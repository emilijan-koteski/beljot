import { useEffect } from "react";
import { toast } from "sonner";

import { MOTION } from "@/shared/lib/motion";
import {
  ACTION_ANNOUNCE_BELOT,
  ACTION_DECLARE,
  ACTION_DECLINE_BELOT,
  ACTION_PASS_TRUMP,
  ACTION_PICK_TRUMP,
  ACTION_PLAY_CARD,
  ACTION_SKIP_DECLARE,
} from "@/shared/types/wsEvents";

export const MATCH_ERROR_TOAST_ID = "match-error";
export const MISSED_TURN_WARNING_TOAST_ID = "missed-turn-warning";
export const FELT_TOASTS_CLASS = "felt-toasts";

const TURN_MOVES: ReadonlySet<string> = new Set([
  ACTION_PLAY_CARD,
  ACTION_PICK_TRUMP,
  ACTION_PASS_TRUMP,
  ACTION_DECLARE,
  ACTION_SKIP_DECLARE,
  ACTION_ANNOUNCE_BELOT,
  ACTION_DECLINE_BELOT,
]);

/** Raises the in-match error as a toast; a newer error replaces the one still showing. */
export function showMatchError(message: string): void {
  toast.error(message, {
    id: MATCH_ERROR_TOAST_ID,
    duration: MOTION.TOAST_ERROR,
    closeButton: true,
  });
}

/** Warns the player that their next missed turn abandons the match; it stays until they move. */
export function showMissedTurnWarning(message: string): void {
  toast.warning(message, {
    id: MISSED_TURN_WARNING_TOAST_ID,
    duration: Infinity,
    closeButton: true,
  });
}

/** Clears the missed-turn warning. */
export function clearMissedTurnWarning(): void {
  toast.dismiss(MISSED_TURN_WARNING_TOAST_ID);
}

/** Clears the missed-turn warning when the player sends a move of their own. */
export function clearMissedTurnWarningOnMove(actionType: string): void {
  if (TURN_MOVES.has(actionType)) clearMissedTurnWarning();
}

/** Gives every toast the felt skin while the table is mounted, and clears the match toasts when it leaves. */
export function useFeltToasts(): void {
  useEffect(() => {
    document.body.classList.add(FELT_TOASTS_CLASS);
    return () => {
      document.body.classList.remove(FELT_TOASTS_CLASS);
      toast.dismiss(MATCH_ERROR_TOAST_ID);
      clearMissedTurnWarning();
    };
  }, []);
}
