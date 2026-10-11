import { useInfiniteQuery, useQuery } from "@tanstack/react-query";

import { FetchError } from "@/shared/api/axiosClient";
import type { MatchFilter, MatchSort } from "@/shared/api/matches";
import { getRoomLastMatch, getUserMatches } from "@/shared/api/matches";
import { queryKeys } from "@/shared/api/queryKeys";

interface UseUserMatchesOptions {
  outcome?: MatchFilter;
  sort?: MatchSort;
  pageSize?: number;
}

export function useUserMatchesInfiniteQuery(
  userId: number | undefined,
  { outcome = "all", sort = "new", pageSize = 20 }: UseUserMatchesOptions = {},
) {
  return useInfiniteQuery({
    queryKey: [...queryKeys.matches.byUser(userId ?? 0, outcome, sort), pageSize] as const,
    queryFn: ({ pageParam }) =>
      getUserMatches(userId!, pageSize, pageParam as number, { outcome, sort }),
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) => {
      if (lastPage.items.length === 0) return undefined;
      const loaded = allPages.reduce((n, p) => n + p.items.length, 0);
      return loaded < lastPage.total ? loaded : undefined;
    },
    enabled: userId !== undefined,
  });
}

function isClientError(error: Error) {
  return error instanceof FetchError && error.status >= 400 && error.status < 500;
}

/**
 * The room's most recent match, refetched on every mount because the per-room
 * cache can still hold the previous match. A 4xx is a settled answer and never retried.
 */
export function useRoomLastMatchQuery(roomId: number | undefined, enabled: boolean) {
  return useQuery({
    queryKey: queryKeys.matches.lastByRoom(roomId ?? 0),
    queryFn: () => getRoomLastMatch(roomId!),
    enabled: enabled && roomId !== undefined,
    staleTime: 0,
    retry: (failureCount, error) => {
      if (isClientError(error)) return false;
      return failureCount < 2;
    },
  });
}
