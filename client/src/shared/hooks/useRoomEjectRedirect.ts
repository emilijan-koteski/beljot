import { useEffect } from "react";
import { useLocation } from "react-router";

import { useLobbyReturn } from "@/shared/hooks/useLobbyReturn";
import { useRoomStore } from "@/shared/stores/roomStore";

/**
 * Story 9.3: always-mounted navigator that routes an ejected player to the lobby
 * from wherever they are (the room page, a match result overlay, or elsewhere)
 * the instant the ejection signal is set — by the return-time 409 (MatchPage, which
 * seeds a fallback notice for both insolvency and honor), the per-user
 * `system:insolvent_ejected` or `system:honor_ejected` push (Story 9.8), or
 * `system:room_closed_insolvent` (reused for honor closes).
 *
 * It fires on ANY non-null notice, so a new ejection reason needs no change here.
 *
 * Unlike useMatchStartRedirect it does NOT clear the signal: the lobby arrival
 * modal is the sole consumer and clears it on close. Navigation fires only when
 * we are not already on the lobby, so a player ejected while browsing the lobby
 * just sees the modal with no redundant navigation.
 */
export function useRoomEjectRedirect(): void {
  const roomEjection = useRoomStore((s) => s.roomEjection);
  const returnToLobby = useLobbyReturn();
  const location = useLocation();

  useEffect(() => {
    if (roomEjection === null) return;
    if (location.pathname !== "/lobby") {
      returnToLobby();
    }
  }, [roomEjection, location.pathname, returnToLobby]);
}
