import { useQuery } from "@tanstack/react-query";

import { listFriends } from "@/shared/api/friends";
import { queryKeys } from "@/shared/api/queryKeys";

/** The viewer's accepted friends with a live online flag. */
export function useFriends() {
  return useQuery({
    queryKey: queryKeys.friends.list(),
    queryFn: listFriends,
  });
}
