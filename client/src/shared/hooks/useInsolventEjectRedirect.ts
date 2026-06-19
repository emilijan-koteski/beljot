import { useEffect } from "react";
import { useLocation, useNavigate } from "react-router";

import { useRoomStore } from "@/shared/stores/roomStore";

/**
 * Story 9.3: always-mounted navigator that routes an insolvency-ejected player
 * to the lobby from wherever they are (the room page, a match result overlay, or
 * elsewhere) the instant the ejection signal is set — by the return-time 409
 * (MatchPage), the per-user `system:insolvent_ejected` push, or
 * `system:room_closed_insolvent`.
 *
 * Unlike useMatchStartRedirect it does NOT clear the signal: the lobby arrival
 * modal is the sole consumer and clears it on close. Navigation fires only when
 * we are not already on the lobby, so a player ejected while browsing the lobby
 * just sees the modal with no redundant navigation.
 */
export function useInsolventEjectRedirect(): void {
  const insolventEjection = useRoomStore((s) => s.insolventEjection);
  const navigate = useNavigate();
  const location = useLocation();

  useEffect(() => {
    if (insolventEjection === null) return;
    if (location.pathname !== "/lobby") {
      navigate("/lobby");
    }
  }, [insolventEjection, location.pathname, navigate]);
}
