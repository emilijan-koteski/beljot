import { useQuery } from "@tanstack/react-query";

import { getPublicStats } from "@/shared/api/lobby";
import { queryKeys } from "@/shared/api/queryKeys";

const REFETCH_INTERVAL_MS = 30_000;

/** Public landing-page counts, polled and available to guests without auth. */
export function usePublicStatsQuery() {
  return useQuery({
    queryKey: queryKeys.stats.public,
    queryFn: getPublicStats,
    refetchInterval: REFETCH_INTERVAL_MS,
    refetchOnWindowFocus: true,
  });
}
