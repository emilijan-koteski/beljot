import { useMutation, useQueryClient } from "@tanstack/react-query";

import type {
  ProfileResponse,
  UpdatePreferencesRequest,
  UpdateUsernameRequest,
} from "@/shared/api/profile";
import { updatePreferences, updateUsername } from "@/shared/api/profile";
import { queryKeys } from "@/shared/api/queryKeys";
import { useAuthStore } from "@/shared/stores/authStore";

export function useUpdatePreferencesMutation(userId: number) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (prefs: UpdatePreferencesRequest) => updatePreferences(userId, prefs),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.profile.detail(userId) });
    },
  });
}

export function useUpdateUsernameMutation(userId: number) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (req: UpdateUsernameRequest) => updateUsername(userId, req),
    onSuccess: (data) => {
      // Patch the profile cache so the page reflects the new name + cooldown
      // stamp without a refetch.
      queryClient.setQueryData<ProfileResponse>(queryKeys.profile.detail(userId), (old) =>
        old ? { ...old, username: data.username, usernameChangedAt: data.usernameChangedAt } : old,
      );
      // The username also lives on the auth store, which drives the TopBar
      // (avatar initial, nav pill, "signed in as"). Update it there too or the
      // header goes stale until the next refresh-token cycle.
      const user = useAuthStore.getState().user;
      if (user && user.id === userId) {
        useAuthStore.getState().setUser({ ...user, username: data.username });
      }
    },
  });
}
