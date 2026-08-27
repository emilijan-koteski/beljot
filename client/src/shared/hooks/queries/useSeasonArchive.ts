import { useQuery } from "@tanstack/react-query";

import { queryKeys } from "@/shared/api/queryKeys";
import { getSeasonArchive } from "@/shared/api/season";

/**
 * A player's prior-season archive (Story 13.3), behind the profile pages'
 * SeasonSection. Keyed per SUBJECT id — both the self profile and the public
 * one read the same endpoint for whoever the page is about.
 *
 * `userId` is undefined while the route param / auth store has not resolved,
 * and the query simply waits — the archive section is absent from the DOM
 * until there is something to show, so there is no loading state to feed.
 *
 * Not polled: the archive only ever changes at a season boundary (a whole
 * quarter), and a fresh read on mount is already newer than that.
 */
export function useSeasonArchiveQuery(userId: number | undefined) {
  return useQuery({
    queryKey: queryKeys.season.archive(userId ?? 0),
    queryFn: () => getSeasonArchive(userId as number),
    enabled: userId !== undefined,
  });
}
