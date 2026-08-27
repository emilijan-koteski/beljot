import { useInfiniteQuery, useQuery } from "@tanstack/react-query";

import { queryKeys } from "@/shared/api/queryKeys";
import { getSeasonLeaderboard } from "@/shared/api/season";

/**
 * Widget poll interval.
 *
 * POLLED, unlike `useCurrentSeason` — and the difference is the point. The
 * viewer's OWN standing changes only when the viewer finishes a match, and
 * `event:season_points_awarded` invalidates it the moment it does. Other
 * players' standings change when THEY finish matches, and this product has no
 * push channel for that: the leaderboard is explicitly pull-only (epic decision,
 * restated as a Story 13.2 boundary — do not add a WS event for standings).
 *
 * 60s rather than useLobbyStats' 10s: a ladder built from whole matches moves on
 * the order of minutes, and this is ambient motivation in the corner of the
 * lobby, not a live scoreboard. Ten times the interval for a tenth of the
 * urgency.
 */
const REFETCH_INTERVAL_MS = 60_000;

/**
 * The lobby widget's top-N slice (Story 13.2 AC1).
 *
 * `refetchOnWindowFocus` is forced on for the same reason useLobbyStats forces
 * it: a player who tabs back after a match elsewhere should see the current
 * order immediately rather than waiting out the rest of the interval.
 */
export function useSeasonLeaderboardQuery(limit: number = 10, enabled: boolean = true) {
  return useQuery({
    queryKey: queryKeys.season.leaderboard(limit),
    queryFn: () => getSeasonLeaderboard(limit, 0),
    enabled,
    refetchInterval: REFETCH_INTERVAL_MS,
    refetchOnWindowFocus: true,
  });
}

/**
 * The full page's load-more paging (Story 13.2 AC2), modelled on
 * `useUserMatchesInfiniteQuery`.
 *
 * `pageParam` is the OFFSET, advanced by the number of rows already loaded
 * rather than by `pageSize * pages`: those agree only while every page comes
 * back full, and a short page (the last one) would otherwise re-request rows the
 * client already holds.
 *
 * NOT polled. A `refetchInterval` here would re-fetch every loaded page on every
 * tick, and a re-ordered ladder underneath a reader who has paged three deep is
 * worse than slightly stale numbers. The page is fresh on mount, which is when
 * it is read.
 */
export function useSeasonLeaderboardInfiniteQuery(pageSize: number = 25) {
  return useInfiniteQuery({
    // The "infinite" suffix keeps this entry distinct from the widget's plain
    // query at the same page size: same key, different cached SHAPE (pages[]
    // versus one response), which React Query would happily mix up.
    queryKey: [...queryKeys.season.leaderboard(pageSize), "infinite"] as const,
    queryFn: ({ pageParam }) => getSeasonLeaderboard(pageSize, pageParam as number),
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) => {
      if (lastPage.items.length === 0) return undefined;
      const loaded = allPages.reduce((n, p) => n + p.items.length, 0);
      return loaded < lastPage.total ? loaded : undefined;
    },
  });
}
