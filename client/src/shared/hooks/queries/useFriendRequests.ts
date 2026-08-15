import { useQuery } from "@tanstack/react-query";

import { listFriendRequests } from "@/shared/api/friends";
import { queryKeys } from "@/shared/api/queryKeys";

/**
 * The viewer's incoming pending friend requests (Story 11.2, FR6). This is the
 * DURABLE delivery path for a request — the WS system:friend_request push is
 * best-effort/online-only, so this list is what a returning recipient sees.
 */
export function useFriendRequests() {
  return useQuery({
    queryKey: queryKeys.friends.requests(),
    queryFn: listFriendRequests,
  });
}
