import { keepPreviousData, useQuery } from "@tanstack/react-query";

import { queryKeys } from "@/shared/api/queryKeys";
import { listInvitablePlayers } from "@/shared/api/rooms";

/**
 * Idle lobby players invitable into one room, keyed on an already-debounced search term.
 * Rendering data only; the server re-checks eligibility at invite time.
 */
export function useInvitablePlayers(roomId: number | undefined, search: string, enabled: boolean) {
  const trimmed = search.trim();
  return useQuery({
    queryKey: queryKeys.rooms.invitablePlayers(roomId!, trimmed),
    queryFn: () => listInvitablePlayers(roomId!, trimmed),
    enabled: enabled && roomId !== undefined,
    staleTime: 0,
    refetchOnMount: "always",
    placeholderData: keepPreviousData,
  });
}
