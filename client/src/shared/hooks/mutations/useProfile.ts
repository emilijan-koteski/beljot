import type { QueryClient } from "@tanstack/react-query";
import { useMutation, useQueryClient } from "@tanstack/react-query";

import type { ProfileResponse, UpdateUsernameRequest } from "@/shared/api/profile";
import { removeAvatar, updateUsername, uploadAvatar } from "@/shared/api/profile";
import { queryKeys } from "@/shared/api/queryKeys";
import { useAuthStore } from "@/shared/stores/authStore";

/** Rename the user, patching both the profile cache and the auth store so the page and the TopBar update without a refetch. */
export function useUpdateUsernameMutation(userId: number) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (req: UpdateUsernameRequest) => updateUsername(userId, req),
    onSuccess: (data) => {
      queryClient.setQueryData<ProfileResponse>(queryKeys.profile.detail(userId), (old) =>
        old ? { ...old, username: data.username, usernameChangedAt: data.usernameChangedAt } : old,
      );
      const user = useAuthStore.getState().user;
      if (user && user.id === userId) {
        useAuthStore.getState().setUser({ ...user, username: data.username });
      }
    },
  });
}

/**
 * Apply a new or removed avatar to the profile cache and the auth store without a refetch.
 * The viewer's cached public-profile entry is invalidated instead.
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

/** Avatar upload input; onProgress receives the sent fraction from 0 to 1. */
interface UploadAvatarVariables {
  file: File;
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
