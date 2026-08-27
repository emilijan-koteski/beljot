import { useQuery } from "@tanstack/react-query";

import { queryKeys } from "@/shared/api/queryKeys";
import { getCurrentSeason } from "@/shared/api/season";

/**
 * The viewer's standing in the active season (Story 13.1), behind the header's
 * rank chip and the profile's RankBanner — one cache entry, read twice.
 *
 * PUSHED, NOT POLLED. There is deliberately no `refetchInterval`: SP only ever
 * changes at match end, and the `event:season_points_awarded` handler in
 * useWsDispatch invalidates `queryKeys.season.current()` the moment it does. That
 * is the established WS-to-query bridge (the same one the friend-request push
 * uses). useLobbyStats polls because its counts have no push path at all — that
 * reason does not apply here, and polling this would be pure waste.
 *
 * `refetchOnWindowFocus` is left at its default rather than forced on: a tab
 * restored after a long sleep also reconnects the socket, and the season window's
 * own countdown re-renders off the shared time tick, not off a refetch.
 */
export function useCurrentSeasonQuery(enabled: boolean = true) {
  return useQuery({
    queryKey: queryKeys.season.current(),
    queryFn: getCurrentSeason,
    enabled,
  });
}
