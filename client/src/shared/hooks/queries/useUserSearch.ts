import { keepPreviousData, useQuery } from "@tanstack/react-query";

import { queryKeys } from "@/shared/api/queryKeys";
import { searchUsers } from "@/shared/api/users";

/**
 * Live player search keyed on an already-debounced term. Fires only for 2+
 * trimmed characters and keeps previous results visible while loading.
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
