import type { QueryClient } from "@tanstack/react-query";
import { useMutation, useQueryClient } from "@tanstack/react-query";

import type {
  ProfileResponse,
  UpdatePreferencesRequest,
  UpdateUsernameRequest,
} from "@/shared/api/profile";
import {
  removeAvatar,
  updatePreferences,
  updateUsername,
  uploadAvatar,
} from "@/shared/api/profile";
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

/**
 * Adopt a new avatar (or its removal) everywhere the viewer's own picture is
 * already on screen, without a refetch: the profile cache (both sizes, so the
 * hero swaps at once) and the auth store (the 128 image, which drives the nav
 * pill). The viewer's public-profile entry, if one was ever cached, is simply
 * invalidated.
 */
function adoptAvatar(
  queryClient: QueryClient,
  userId: number,
  avatarUrl: string | null,
  avatarLargeUrl: string | null,
) {
  queryClient.setQueryData<ProfileResponse>(queryKeys.profile.detail(userId), (old) =>
    old ? { ...old, avatarUrl, avatarLargeUrl } : old,
  );
  const user = useAuthStore.getState().user;
  if (user && user.id === userId) {
    useAuthStore.getState().setUser({ ...user, avatarUrl });
  }
  queryClient.invalidateQueries({ queryKey: queryKeys.publicProfile.detail(userId) });
}

export interface UploadAvatarVariables {
  file: File;
  /** Sent fraction of the upload, 0 to 1, for the progress bar. */
  onProgress?: (fraction: number) => void;
}

export function useUploadAvatarMutation(userId: number) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ file, onProgress }: UploadAvatarVariables) =>
      uploadAvatar(userId, file, onProgress),
    onSuccess: (data) => {
      adoptAvatar(queryClient, userId, data.avatarUrl, data.avatarLargeUrl);
    },
  });
}

export function useRemoveAvatarMutation(userId: number) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => removeAvatar(userId),
    onSuccess: () => {
      adoptAvatar(queryClient, userId, null, null);
    },
  });
}
