import { useQuery } from "@tanstack/react-query";

import { queryKeys } from "@/shared/api/queryKeys";
import { listInvitableFriends } from "@/shared/api/rooms";

/**
 * The caller's friends annotated with invite availability for one room
 * (Story 11.5, AC1). Availability is live presence, so it goes stale quickly —
 * the query is only enabled while the invite panel is open and refetches on
 * mount, and the server re-checks availability at invite time regardless. This
 * list is rendering data, never an authorization.
 */
export function useInvitableFriends(roomId: number | undefined, enabled: boolean) {
  return useQuery({
    queryKey: queryKeys.rooms.invitableFriends(roomId!),
    queryFn: () => listInvitableFriends(roomId!),
    enabled: enabled && roomId !== undefined,
    staleTime: 0,
    refetchOnMount: "always",
  });
}
