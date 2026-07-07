import { useQuery } from "@tanstack/react-query";

import { getIdentities } from "@/shared/api/identities";
import { queryKeys } from "@/shared/api/queryKeys";

export function useIdentitiesQuery(userId: number | undefined) {
  return useQuery({
    queryKey: queryKeys.identities.detail(userId!),
    queryFn: () => getIdentities(userId!),
    enabled: userId !== undefined,
  });
}
