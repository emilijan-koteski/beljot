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
      //
      // Leaving a room or the quick-play queue is the match starting, not a
      // reconnect: the first state can beat that page's own redirect here, so
      // carry the same arrival flag it would (starting splash, opening deal).
      const fromPreMatch = /^\/(rooms|matchmaking)\//.test(location.pathname);
      navigate(`/match/${matchState.roomId}`, {
        replace: location.pathname !== "/lobby",
        ...(fromPreMatch ? { state: { fromRoom: true } } : {}),
      });
    }
  }, [matchState, location.pathname, navigate]);
}
