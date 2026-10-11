import { useQuery } from "@tanstack/react-query";

import { queryKeys } from "@/shared/api/queryKeys";
import { getSeasonArchive } from "@/shared/api/season";

/**
 * A player's prior-season archive, keyed by the profile subject's id. Waits
 * until `userId` resolves.
 */
export function useSeasonArchiveQuery(userId: number | undefined) {
  return useQuery({
    queryKey: queryKeys.season.archive(userId ?? 0),
    queryFn: () => getSeasonArchive(userId as number),
    enabled: userId !== undefined,
  });
}
