import { useMutation, useQueryClient } from "@tanstack/react-query";
import i18n from "i18next";
import { toast } from "sonner";

import { acceptFriendRequest, declineFriendRequest, sendFriendRequest } from "@/shared/api/friends";
import { queryKeys } from "@/shared/api/queryKeys";

/**
 * Friend request mutations (Story 11.2, FR6). Each invalidates exactly the
 * caches its action changes so the profile button and the friends/requests
 * surfaces stay live without a manual refetch.
 *
 * Accept/decline take the row id AND the counterpart `userId` (optional) so the
 * public-profile button — which lives on a subject's page — can also refresh
 * that subject's friends.status() entry. The lobby requests list passes the
 * sender's fromUserId for the same reason.
 */

/** Send a friend request; flips the subject's status to pending_outgoing. */
export function useSendFriendRequestMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (userId: number) => sendFriendRequest(userId),
    onSuccess: (_data, userId) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.friends.status(userId) });
    },
    onError: () => {
      // A stale button (e.g. a racing/mutual request) yields a 409, or the target
      // vanished — surface it instead of silently re-enabling the button.
      toast.error(i18n.t("friends.errors.sendFailed"));
    },
  });
}

/** Accept an incoming request; refreshes requests, the friend list, and (if known) the subject's status. */
export function useAcceptFriendRequestMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (vars: { requestId: number; userId?: number }) =>
      acceptFriendRequest(vars.requestId),
    onSuccess: (_data, vars) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.friends.requests() });
      queryClient.invalidateQueries({ queryKey: queryKeys.friends.list() });
      if (vars.userId !== undefined) {
        queryClient.invalidateQueries({ queryKey: queryKeys.friends.status(vars.userId) });
      }
    },
    onError: () => {
      // The request may have been accepted/declined elsewhere first → 404.
      toast.error(i18n.t("friends.errors.acceptFailed"));
    },
  });
}

/** Decline an incoming request; refreshes requests and (if known) the subject's status. */
export function useDeclineFriendRequestMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (vars: { requestId: number; userId?: number }) =>
      declineFriendRequest(vars.requestId),
    onSuccess: (_data, vars) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.friends.requests() });
      if (vars.userId !== undefined) {
        queryClient.invalidateQueries({ queryKey: queryKeys.friends.status(vars.userId) });
      }
    },
    onError: () => {
      // The request may have been accepted/declined elsewhere first → 404.
      toast.error(i18n.t("friends.errors.declineFailed"));
    },
  });
}
