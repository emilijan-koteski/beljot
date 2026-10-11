import { useEffect } from "react";
import { useLocation } from "react-router";

import { useLobbyReturn } from "@/shared/hooks/useLobbyReturn";
import { useRoomStore } from "@/shared/stores/roomStore";

/**
 * Routes an ejected player to the lobby from wherever they are as soon as a room ejection is set.
 * It does not clear the signal; the lobby arrival modal consumes it on close.
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
