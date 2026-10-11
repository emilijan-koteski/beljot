import { useInfiniteQuery, useQuery } from "@tanstack/react-query";

import { queryKeys } from "@/shared/api/queryKeys";
import type { SeasonSelector } from "@/shared/api/season";
import { getSeasonLeaderboard, getSeasons } from "@/shared/api/season";

const REFETCH_INTERVAL_MS = 60_000;

/**
 * The lobby widget's top-N slice of the current season, polled because other
 * players' standings have no push channel.
 */
export function useSeasonLeaderboardQuery(limit: number = 10, enabled: boolean = true) {
  return useQuery({
    queryKey: queryKeys.season.leaderboard(limit, "current"),
    queryFn: () => getSeasonLeaderboard(limit, 0, "current"),
    enabled,
    refetchInterval: REFETCH_INTERVAL_MS,
    refetchOnWindowFocus: true,
  });
}

/**
 * The full leaderboard's load-more paging for a season, offset by rows already loaded.
 * Not polled; the "infinite" key suffix keeps its cache apart from the widget's query.
 */
export function useSeasonLeaderboardInfiniteQuery(
  pageSize: number = 25,
  season: SeasonSelector = "current",
) {
  return useInfiniteQuery({
    queryKey: [...queryKeys.season.leaderboard(pageSize, season), "infinite"] as const,
    queryFn: ({ pageParam }) => getSeasonLeaderboard(pageSize, pageParam as number, season),
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) => {
      if (lastPage.items.length === 0) return undefined;
      const loaded = allPages.reduce((n, p) => n + p.items.length, 0);
      return loaded < lastPage.total ? loaded : undefined;
    },
  });
}

/**
 * The seasons list behind the leaderboard's season picker, newest first. Not
 * polled; season rollover invalidates it explicitly.
 */
export function useSeasonsQuery() {
  return useQuery({
    queryKey: queryKeys.season.list(),
    queryFn: getSeasons,
    refetchOnWindowFocus: false,
  });
}
