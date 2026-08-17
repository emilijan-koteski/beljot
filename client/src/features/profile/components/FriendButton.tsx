import { Check, Clock, UserPlus } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/shared/components/ui/button";
import {
  useAcceptFriendRequestMutation,
  useSendFriendRequestMutation,
} from "@/shared/hooks/mutations/useFriendMutations";
import { useFriendshipStatus } from "@/shared/hooks/queries/useFriendshipStatus";
import { useAuthStore } from "@/shared/stores/authStore";
import type { FriendshipStatus } from "@/shared/types/apiTypes";

interface FriendButtonProps {
  /** The subject whose profile is being viewed (the validated /players/:id id). */
  userId: number;
}

/**
 * Friendship action on a public profile (Story 11.2, AC7). Drives its label and
 * action entirely from GET /friends/status/:id, so EVERY state maps to a real
 * affordance — it is never a dead/placeholder button:
 *
 *   none             -> "Add Friend"     (sends a request)
 *   pending_outgoing -> "Request sent"   (disabled)
 *   pending_incoming -> "Accept request" (accepts)
 *   friends          -> "Friends"        (disabled, with a check)
 *
 * Never rendered for the viewer's own profile.
 */
export function FriendButton({ userId }: FriendButtonProps) {
  const { t } = useTranslation();
  const viewer = useAuthStore((s) => s.user);

  const status = useFriendshipStatus(userId);
  const sendMutation = useSendFriendRequestMutation();
  const acceptMutation = useAcceptFriendRequestMutation();

  // Defensive self-guard: the public page is not reached for the viewer's own id
  // in practice, but never show the button there.
  if (viewer && viewer.id === userId) return null;

  // While the status loads, hold the space with a neutral skeleton — never a
  // labelled button, which would assert a false relationship ("Add friend" on an
  // existing friend) during the load.
  if (status.isPending) {
    return (
      <div
        className="bg-surface-sunken my-5 h-8 w-28 animate-pulse rounded-md"
        data-testid="friend-button-loading"
        aria-hidden="true"
      />
    );
  }

  // On a status-query error we don't know the relationship; default to the
  // actionable "none" affordance rather than a permanently dead disabled button —
  // the server is authoritative and rejects a duplicate with a 409 (surfaced by
  // the mutation's onError toast).
  const { status: state, requestId }: FriendshipStatus = status.data ?? {
    status: "none",
    requestId: null,
  };

  let content;
  switch (state) {
    case "friends":
      content = (
        <Button variant="outline" size="sm" disabled data-testid="friend-button-friends">
          <Check /> {t("friends.friends")}
        </Button>
      );
      break;
    case "pending_outgoing":
      content = (
        <Button variant="outline" size="sm" disabled data-testid="friend-button-pending">
          <Clock /> {t("friends.requestSent")}
        </Button>
      );
      break;
    case "pending_incoming":
      content = (
        <Button
          size="sm"
          data-testid="friend-button-accept"
          disabled={acceptMutation.isPending || requestId === null}
          onClick={() => {
            if (requestId !== null) acceptMutation.mutate({ requestId, userId });
          }}
        >
          <Check /> {t("friends.acceptRequest")}
        </Button>
      );
      break;
    default:
      // "none"
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
    // my-5 (not mt-only): the row needs the same 20px gap below it as above,
    // otherwise it sits flush against whatever follows on the public profile —
    // the streak callout has no top margin of its own.
    <div className="my-5" data-testid="friend-button">
      {content}
    </div>
  );
}
