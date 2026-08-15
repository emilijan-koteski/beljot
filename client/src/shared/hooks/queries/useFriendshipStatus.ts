import { useQuery } from "@tanstack/react-query";

import { getFriendshipStatus } from "@/shared/api/friends";
import { queryKeys } from "@/shared/api/queryKeys";

/**
 * Friendship state between the viewer and subject `userId` (Story 11.2) — the
 * source of truth for the public-profile Add-Friend button. Gated to a positive
 * integer id so it never fires for an unresolved / self / invalid subject.
 */
export function useFriendshipStatus(userId: number) {
  return useQuery({
    queryKey: queryKeys.friends.status(userId),
    queryFn: () => getFriendshipStatus(userId),
    enabled: Number.isInteger(userId) && userId > 0,
  });
}
