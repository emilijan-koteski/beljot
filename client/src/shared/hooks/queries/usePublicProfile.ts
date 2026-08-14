import { useQuery } from "@tanstack/react-query";

import { getPublicProfile } from "@/shared/api/profile";
import { queryKeys } from "@/shared/api/queryKeys";

/**
 * Fetch another player's public profile (Story 11.3). Mirrors useProfileQuery
 * but under the `publicProfile` cache namespace and typed as the narrower
 * PublicProfileResponse — the public page never fetches the viewer's own id here.
 */
export function usePublicProfileQuery(userId: number | undefined) {
  return useQuery({
    queryKey: queryKeys.publicProfile.detail(userId!),
    queryFn: () => getPublicProfile(userId!),
    enabled: userId !== undefined,
  });
}
