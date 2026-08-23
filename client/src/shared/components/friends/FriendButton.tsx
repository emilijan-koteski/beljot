import { Check, Clock, UserMinus, UserPlus, X } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "@/shared/components/ui/button";
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
  /** The subject whose profile is being viewed (the validated /players/:id id). */
  userId: number;
  /** The subject's username, interpolated into the remove-friend confirm dialog. */
  username: string;
  /**
   * Drops the profile-page `my-5` rhythm (see the wrapper below) for surfaces
   * that own their own spacing — the per-player action rows in the match-stats
   * card, where 20px of vertical margin per row would shred the list. Layout
   * only: every state, label and handler is unchanged.
   */
  compact?: boolean;
  /**
   * Offer "Remove friend" in the `friends` state. FALSE inside the in-match
   * overlay: the confirm it opens is a shadcn Dialog at the primitive's `z-50`,
   * while the match-result panel sits at `Z.PROMPT` (74) — the confirm would
   * paint BEHIND the overlay, invisible and unclickable, with the pending-guard
   * locking every dismissal path. Suppressing the affordance is the honest fix;
   * unfriending belongs on the profile, not mid-celebration.
   */
  allowRemove?: boolean;
}

/**
 * Friendship action on a public profile (Story 11.2, AC7). Drives its label and
 * action entirely from GET /friends/status/:id, so EVERY state maps to a real
 * affordance — it is never a dead/placeholder button:
 *
 *   none             -> "Add Friend"        (sends a request)
 *   pending_outgoing -> "Request sent"      (disabled)
 *   pending_incoming -> "Accept request"    (accepts) + "Decline" (declines)
 *   friends          -> "Friends" (disabled, with a check) + "Remove friend"
 *                       (opens a confirm dialog, then unfriends)
 *
 * Never rendered for the viewer's own profile.
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

  // Defensive self-guard: the public page is not reached for the viewer's own id
  // in practice, but never show the button there.
  if (viewer && viewer.id === userId) return null;

  // While the status loads, hold the space with a neutral skeleton — never a
  // labelled button, which would assert a false relationship ("Add friend" on an
  // existing friend) during the load.
  if (status.isPending) {
    return (
      <div
        className={`bg-surface-sunken h-8 w-28 animate-pulse rounded-md ${compact ? "" : "my-5"}`}
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
                // requestId === null should be impossible for "friends", but never
                // offer a confirm that could fire without a row id.
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
                    // Close on settled: on success the invalidated status query
                    // flips the button to "Add friend"; on error (e.g. the other
                    // party won the both-unfriend race → 404) the hook's toast
                    // reports it and a lingering dialog would just restate a
                    // stale question.
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
      // Accept and Decline act on the same request row — never let them race:
      // either one in flight disables both.
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
          {/* Deliberate asymmetry: decline needs no confirm dialog (mirrors the
              lobby requests list — the sender can simply re-send), while
              remove-friend destroys an established relationship and gets one. */}
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
    // the streak callout has no top margin of its own. `compact` drops it for
    // hosts that space their own rows.
    <div
      className={`flex flex-wrap items-center gap-2 ${compact ? "" : "my-5"}`}
      data-testid="friend-button"
    >
      {content}
    </div>
  );
}
