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
    // Filter + sort are part of the key so changing either refetches from
    // page 0 rather than mixing differently-ordered/filtered pages.
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

/**
 * The room's most recent match (room last-match stats). Both surfaces that show
 * it — the room lobby's dialog and the end-of-match overlay — read it here.
 *
 * `staleTime: 0` overrides the client-wide 30s default, and it is load-bearing
 * rather than tidiness. The key is per-ROOM: the room lobby populates it before
 * the match starts, so a short match (a surrender) ends well inside that window
 * and the overlay would otherwise be served match N-1 straight from cache.
 * Zero staleness makes every mount refetch; `useWsDispatch` additionally
 * REMOVES the entry on `event:match_end`, so there is no stale row left to
 * serve at all, and `MatchResult` refuses to paint a row that is not this match.
 *
 * EVERY 4xx is terminal, never retried: the server will keep repeating the same
 * settled answer. 404 (no match in this room yet, or the caller never played
 * it) is the common case for a fresh room; 400 is a malformed room id; 401/403
 * are auth. Retrying any of them just delays the empty state by three
 * round-trips. There is no persist-vs-broadcast race to absorb either — the
 * only surface live at match end is the MatchResult overlay, which mounts on
 * `match_end`, and that path persists the row BEFORE broadcasting
 * (match/live_match.go). The abandonment path, which does broadcast first,
 * renders ReconnectOverlay instead and never shows these stats.
 *
 * `retry: 2` therefore covers only genuinely transient failures (network, 5xx).
 */
export function useRoomLastMatchQuery(roomId: number | undefined, enabled: boolean) {
  return useQuery({
    queryKey: queryKeys.matches.lastByRoom(roomId ?? 0),
    queryFn: () => getRoomLastMatch(roomId!),
    enabled: enabled && roomId !== undefined,
    staleTime: 0,
    retry: (failureCount, error) => {
      // FetchError.status is 0 for a network/timeout failure, which IS worth
      // retrying — only the 4xx band is a settled answer.
      if (error instanceof FetchError && error.status >= 400 && error.status < 500) {
        return false;
      }
      return failureCount < 2;
    },
  });
}
