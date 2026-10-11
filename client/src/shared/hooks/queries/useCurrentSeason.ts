import { useQuery } from "@tanstack/react-query";

import { queryKeys } from "@/shared/api/queryKeys";
import { getCurrentSeason } from "@/shared/api/season";

/**
 * The viewer's standing in the active season. Not polled: the season_points_awarded
 * WS event invalidates this query when SP changes at match end.
 */
export function useCurrentSeasonQuery(enabled: boolean = true) {
  return useQuery({
    queryKey: queryKeys.season.current(),
    queryFn: getCurrentSeason,
    enabled,
  });
}
