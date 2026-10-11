import { useMutation, useQueryClient } from "@tanstack/react-query";
import i18n from "i18next";
import { toast } from "sonner";

import {
  acceptFriendRequest,
  declineFriendRequest,
  removeFriend,
  sendFriendRequest,
} from "@/shared/api/friends";
import { queryKeys } from "@/shared/api/queryKeys";

/** Send a friend request; flips the subject's status to pending_outgoing. */
export function useSendFriendRequestMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (userId: number) => sendFriendRequest(userId),
    onSuccess: (_data, userId) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.friends.status(userId) });
    },
    onError: () => {
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
      toast.error(i18n.t("friends.errors.acceptFailed"));
    },
  });
}

/** Decline an incoming request and refresh requests and the subject's status; on error resync both, since a 404 means the row is already gone. */
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
    onError: (_error, vars) => {
      toast.error(i18n.t("friends.errors.declineFailed"));
      queryClient.invalidateQueries({ queryKey: queryKeys.friends.requests() });
      if (vars.userId !== undefined) {
        queryClient.invalidateQueries({ queryKey: queryKeys.friends.status(vars.userId) });
      }
    },
  });
}

/** Remove an accepted friend and refresh the friend list and the subject's status, also on error, since a 404 means the friendship is already gone. */
export function useRemoveFriendMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (vars: { requestId: number; userId: number }) => removeFriend(vars.requestId),
    onSuccess: (_data, vars) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.friends.list() });
      queryClient.invalidateQueries({ queryKey: queryKeys.friends.status(vars.userId) });
    },
    onError: (_error, vars) => {
      toast.error(i18n.t("friends.errors.removeFailed"));
      queryClient.invalidateQueries({ queryKey: queryKeys.friends.list() });
      queryClient.invalidateQueries({ queryKey: queryKeys.friends.status(vars.userId) });
    },
  });
}
