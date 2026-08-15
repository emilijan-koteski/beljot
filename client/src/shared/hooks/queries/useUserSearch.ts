import { keepPreviousData, useQuery } from "@tanstack/react-query";

import { queryKeys } from "@/shared/api/queryKeys";
import { searchUsers } from "@/shared/api/users";

/**
 * Live player-search query (Story 11.1). Pass the ALREADY-debounced term (see
 * useDebounce) — this hook keys, gates and fetches on exactly that value.
 *
 * No request fires until the trimmed query is at least 2 characters (`enabled`
 * gate), matching AC3. `placeholderData: keepPreviousData` (the v5 idiom) keeps
 * the previous results visible while the next term is in flight, so the list
 * does not flash empty between keystrokes.
 */
export function useUserSearch(debouncedQuery: string) {
  const trimmed = debouncedQuery.trim();
  return useQuery({
    queryKey: queryKeys.users.search(trimmed),
    queryFn: () => searchUsers(trimmed),
    enabled: trimmed.length >= 2,
    placeholderData: keepPreviousData,
  });
}
