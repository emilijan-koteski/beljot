import { Check, Clock, UserMinus, UserPlus, X } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "@/shared/components/ui/button";
import { Skeleton } from "@/shared/components/ui/skeleton";
import {
  useAcceptFriendRequestMutation,
  useDeclineFriendRequestMutation,
  useRemoveFriendMutation,
  useSendFriendRequestMutation,
} from "@/shared/hooks/mutations/useFriendMutations";
import { useFriendshipStatus } from "@/shared/hooks/queries/useFriendshipStatus";
import { useAuthStore } from "@/shared/stores/authStore";
import type { FriendshipStatus } from "@/shared/types/apiTypes";

import { RemoveFriendDialog } from "./RemoveFriendDialog";

interface FriendButtonProps {
  userId: number;
  username: string;
  compact?: boolean;
  allowRemove?: boolean;
}

/**
 * Friendship action for another player, driven by their friendship status so every state maps to a real action.
 * Pass allowRemove={false} on overlays above z-50, where the remove confirm dialog would render hidden behind them.
 */
export function FriendButton({
  userId,
  username,
  compact = false,
  allowRemove = true,
}: FriendButtonProps) {
  const { t } = useTranslation();
  const viewer = useAuthStore((s) => s.user);

  const status = useFriendshipStatus(userId);
  const sendMutation = useSendFriendRequestMutation();
  const acceptMutation = useAcceptFriendRequestMutation();
  const declineMutation = useDeclineFriendRequestMutation();
  const removeMutation = useRemoveFriendMutation();
  const [removeOpen, setRemoveOpen] = useState(false);

  if (viewer && viewer.id === userId) return null;

  if (status.isPending) {
    return (
      <Skeleton
        className={compact ? "h-8 w-28 rounded-md" : "my-5 h-8 w-28 rounded-md"}
        testId="friend-button-loading"
      />
    );
  }

  const { status: state, requestId }: FriendshipStatus = status.data ?? {
    status: "none",
    requestId: null,
  };

  let content;
  switch (state) {
    case "friends":
      content = (
        <>
          <Button variant="outline" size="sm" disabled data-testid="friend-button-friends">
            <Check /> {t("friends.friends")}
          </Button>
          {allowRemove && (
            <>
              <Button
                variant="outline"
                size="sm"
                data-testid="friend-button-remove"
                disabled={removeMutation.isPending || requestId === null}
                onClick={() => setRemoveOpen(true)}
              >
                <UserMinus /> {t("friends.removeFriend")}
              </Button>
              <RemoveFriendDialog
                open={removeOpen}
                username={username}
                pending={removeMutation.isPending}
                onConfirm={() => {
                  if (removeMutation.isPending || requestId === null) return;
                  removeMutation.mutate(
                    { requestId, userId },
                    { onSettled: () => setRemoveOpen(false) },
                  );
                }}
                onClose={() => {
                  if (!removeMutation.isPending) setRemoveOpen(false);
                }}
              />
            </>
          )}
        </>
      );
      break;
    case "pending_outgoing":
      content = (
        <Button variant="outline" size="sm" disabled data-testid="friend-button-pending">
          <Clock /> {t("friends.requestSent")}
        </Button>
      );
      break;
    case "pending_incoming": {
      const actionPending = acceptMutation.isPending || declineMutation.isPending;
      content = (
        <>
          <Button
            size="sm"
            data-testid="friend-button-accept"
            disabled={actionPending || requestId === null}
            onClick={() => {
              if (requestId !== null) acceptMutation.mutate({ requestId, userId });
            }}
          >
            <Check /> {t("friends.acceptRequest")}
          </Button>
          <Button
            variant="outline"
            size="sm"
            data-testid="friend-button-decline"
            disabled={actionPending || requestId === null}
            onClick={() => {
              if (requestId !== null) declineMutation.mutate({ requestId, userId });
            }}
          >
            <X /> {t("friends.decline")}
          </Button>
        </>
      );
      break;
    }
    default:
      content = (
        <Button
          size="sm"
          data-testid="friend-button-add"
          disabled={sendMutation.isPending}
          onClick={() => sendMutation.mutate(userId)}
        >
          <UserPlus /> {t("friends.addFriend")}
        </Button>
      );
  }

  return (
    <div
      className={`flex flex-wrap items-center gap-2 ${compact ? "" : "my-5"}`}
      data-testid="friend-button"
    >
      {content}
    </div>
  );
}
