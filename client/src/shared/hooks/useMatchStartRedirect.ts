import { useEffect } from "react";
import { useLocation, useNavigate } from "react-router";

import { useRoomStore } from "@/shared/stores/roomStore";

/**
 * D145b: routes a seated player into a freshly started match even when they are
 * not on RoomPage. RoomPage's own effect handles the common case (the player is
 * sitting in the room when it starts); this always-mounted navigator covers the
 * gap where the player is elsewhere (e.g. still on the prior match's result
 * overlay) when `system:match_started` arrives.
 *
 * `system:match_started` is broadcast only to room members, so a recorded
 * `matchStartedRoomId` always means "navigate me in". The flag is cleared after
 * one navigation to avoid loops; navigating to the page we are already on is a
 * no-op.
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
      // Push only when leaving from the lobby (stack becomes [lobby, match]);
      // from anywhere else replace, so no dead entry lingers beneath the match.
      navigate(target, {
        replace: location.pathname !== "/lobby",
        state: { fromRoom: true },
      });
    }
    // Consume the signal: clear matchStartedRoomId AND the sticky matchStarted
    // flag. Without clearing matchStarted, a player who received match_started
    // while already on /match (e.g. still on a prior result dialog) keeps a
    // stale true flag with no RoomPage mounted to reset it — which would bounce
    // them straight back to /match the next time RoomPage mounts. RoomPage's own
    // navigation effect (a descendant, so its effect runs first) has already
    // fired for the on-RoomPage case, so clearing here is safe.
    setMatchStartedRoomId(null);
    setMatchStarted(false);
  }, [matchStartedRoomId, location.pathname, navigate, setMatchStartedRoomId, setMatchStarted]);
}
