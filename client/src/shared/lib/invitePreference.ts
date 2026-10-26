import { updatePreferences } from "@/shared/api/profile";
import { useAuthStore } from "@/shared/stores/authStore";

let latestRequest = 0;

/** The effective non-friend invite opt-in. Missing means ON, matching the server default. */
export function resolveAllowNonFriendInvites(value: boolean | null | undefined): boolean {
  return value ?? true;
}

/**
 * Optimistically writes the non-friend invite opt-in to the auth store, then persists it.
 * A failure silently restores the previous value unless a later toggle superseded it.
 */
export async function persistAllowNonFriendInvites(next: boolean): Promise<void> {
  const user = useAuthStore.getState().user;
  if (!user) return;
  const previous = resolveAllowNonFriendInvites(user.allowNonFriendInvites);
  if (next === previous) return;

  const request = ++latestRequest;
  useAuthStore.getState().setUser({ ...user, allowNonFriendInvites: next });

  try {
    await updatePreferences(user.id, { allowNonFriendInvites: next });
  } catch {
    if (request !== latestRequest) return;
    const current = useAuthStore.getState().user;
    if (current?.id === user.id) {
      useAuthStore.getState().setUser({ ...current, allowNonFriendInvites: previous });
    }
  }
}

/** Test seam: reset the in-flight sequence between cases. */
export function resetInvitePreferenceRequestSequence(): void {
  latestRequest = 0;
}
