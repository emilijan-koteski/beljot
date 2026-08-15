import { useQuery } from "@tanstack/react-query";

import { listFriends } from "@/shared/api/friends";
import { queryKeys } from "@/shared/api/queryKeys";

/**
 * The viewer's accepted friends with a live online flag (Story 11.2, FR6).
 * Server-collection data, so it lives in the Query cache (not Zustand); the WS
 * system:friend_request push invalidates the requests list, and accept/decline
 * mutations invalidate this one.
 */
export function useFriends() {
  return useQuery({
    queryKey: queryKeys.friends.list(),
    queryFn: listFriends,
  });
}
