import { useMutation, useQueryClient } from "@tanstack/react-query";

import { linkIdentity, unlinkIdentity } from "@/shared/api/identities";
import { queryKeys } from "@/shared/api/queryKeys";

export function useLinkIdentityMutation(userId: number) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ provider, credential }: { provider: string; credential: string }) =>
      linkIdentity(userId, provider, { credential }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.identities.detail(userId) });
    },
  });
}

export function useUnlinkIdentityMutation(userId: number) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (provider: string) => unlinkIdentity(userId, provider),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.identities.detail(userId) });
    },
  });
}
