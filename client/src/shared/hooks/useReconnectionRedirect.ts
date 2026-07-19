import { useEffect } from "react";
import { useLocation, useNavigate } from "react-router";

import { useMatchStore } from "@/shared/stores/matchStore";

/**
 * Redirects to the game page when game state is received while not on the game page.
 * Handles the case where a disconnected player refreshes their browser and lands on
 * the lobby — the server sends event:match_state on WS reconnect, and this hook
 * navigates them back to the active game.
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
      // Push only when leaving from the lobby (stack becomes [lobby, match] so
      // a later return can pop); from anywhere else replace as before.
      navigate(`/match/${matchState.roomId}`, { replace: location.pathname !== "/lobby" });
    }
  }, [matchState, location.pathname, navigate]);
}
