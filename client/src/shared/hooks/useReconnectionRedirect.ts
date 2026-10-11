import { useEffect } from "react";
import { useLocation, useNavigate } from "react-router";

import { useMatchStore } from "@/shared/stores/matchStore";

/**
 * Redirects to the match page when match state arrives while not on it, e.g. after a
 * disconnected player refreshes and the server resends event:match_state on reconnect.
 */
export function useReconnectionRedirect(): void {
  const navigate = useNavigate();
  const location = useLocation();
  const matchState = useMatchStore((s) => s.matchState);

  useEffect(() => {
    if (
      matchState &&
      matchState.roomId &&
      matchState.phase !== "match_end" &&
      !location.pathname.startsWith("/match/")
    ) {
      const fromPreMatch = /^\/(rooms|matchmaking)[/]/.test(location.pathname);
      navigate(`/match/${matchState.roomId}`, {
        replace: location.pathname !== "/lobby",
        ...(fromPreMatch ? { state: { fromRoom: true } } : {}),
      });
    }
  }, [matchState, location.pathname, navigate]);
}
