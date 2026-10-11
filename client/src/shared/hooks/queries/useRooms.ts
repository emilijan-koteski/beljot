import { useQuery } from "@tanstack/react-query";

import { queryKeys } from "@/shared/api/queryKeys";
import { getRoom, getRooms } from "@/shared/api/rooms";

export function useRoomsQuery(status: string = "waiting", enabled: boolean = true) {
  return useQuery({
    queryKey: queryKeys.rooms.list(status),
    queryFn: () => getRooms(status),
    enabled,
  });
}

/**
 * One room's details, always refetched on mount so a cached snapshot never
 * omits the just-joined viewer.
 */
export function useRoomDetailQuery(id: number | undefined) {
  return useQuery({
    queryKey: queryKeys.rooms.detail(id!),
    queryFn: () => getRoom(id!),
    enabled: id !== undefined,
    refetchOnMount: "always",
  });
}
