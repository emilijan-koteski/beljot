import { useEffect } from "react";
import { toast } from "sonner";

import { MOTION } from "@/shared/lib/motion";

export const MATCH_ERROR_TOAST_ID = "match-error";
export const FELT_TOASTS_CLASS = "felt-toasts";

/** Raises the in-match error as a toast; a newer error replaces the one still showing. */
export function showMatchError(message: string): void {
  toast.error(message, {
    id: MATCH_ERROR_TOAST_ID,
    duration: MOTION.TOAST_ERROR,
    closeButton: true,
  });
}

/** Gives every toast the felt skin while the table is mounted, and clears the match error when it leaves. */
export function useFeltToasts(): void {
  useEffect(() => {
    document.body.classList.add(FELT_TOASTS_CLASS);
    return () => {
      document.body.classList.remove(FELT_TOASTS_CLASS);
      toast.dismiss(MATCH_ERROR_TOAST_ID);
    };
  }, []);
}
