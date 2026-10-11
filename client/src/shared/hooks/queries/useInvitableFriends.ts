import { useQuery } from "@tanstack/react-query";

import { queryKeys } from "@/shared/api/queryKeys";
import { listInvitableFriends } from "@/shared/api/rooms";

/**
 * The caller's friends annotated with live invite availability for one room.
 * Rendering data only; the server re-checks availability at invite time.
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
