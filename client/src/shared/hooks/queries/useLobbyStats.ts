import { useQuery } from "@tanstack/react-query";

import { getLobbyStats } from "@/shared/api/lobby";
import { queryKeys } from "@/shared/api/queryKeys";

const REFETCH_INTERVAL_MS = 10_000;

/** Lobby activity counts, polled because they have no WS push path. */
export function useLobbyStatsQuery(enabled: boolean = true) {
  return useQuery({
    queryKey: queryKeys.lobby.stats,
    queryFn: getLobbyStats,
    enabled,
    refetchInterval: REFETCH_INTERVAL_MS,
    refetchOnWindowFocus: true,
  });
}
