import { useEffect } from "react";
import { useLocation, useNavigate } from "react-router";

import { useRoomStore } from "@/shared/stores/roomStore";

/**
 * Routes a seated player into a freshly started match even when they are not on RoomPage.
 * Consumes the match-started signal after one navigation so it cannot bounce them back later.
 */
export function useMatchStartRedirect(): void {
  const matchStartedRoomId = useRoomStore((s) => s.matchStartedRoomId);
  const setMatchStartedRoomId = useRoomStore((s) => s.setMatchStartedRoomId);
  const setMatchStarted = useRoomStore((s) => s.setMatchStarted);
  const navigate = useNavigate();
  const location = useLocation();

  useEffect(() => {
    if (matchStartedRoomId === null) return;
    const target = `/match/${matchStartedRoomId}`;
    if (location.pathname !== target) {
      navigate(target, {
        replace: location.pathname !== "/lobby",
        state: { fromRoom: true },
      });
    }
    setMatchStartedRoomId(null);
    setMatchStarted(false);
  }, [matchStartedRoomId, location.pathname, navigate, setMatchStartedRoomId, setMatchStarted]);
}
