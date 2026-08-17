import { Check, Send } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { FetchError } from "@/shared/api/axiosClient";
import { Avatar } from "@/shared/components/ui/avatar";
import { Button } from "@/shared/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/shared/components/ui/dialog";
import { useInviteToRoomMutation } from "@/shared/hooks/mutations/useRooms";
import { useInvitableFriends } from "@/shared/hooks/queries/useInvitableFriends";
import type { InvitableFriend } from "@/shared/types/apiTypes";

interface InviteFriendsDialogProps {
  open: boolean;
  roomId: number;
  onOpenChange: (open: boolean) => void;
}

/**
 * Story 11.5 AC1: the inviter-side panel, opened from the waiting room. Lists
 * the viewer's friends with server-computed availability and an Invite button
 * per available friend.
 *
 * Unavailable friends are shown DISABLED WITH A REASON rather than filtered out.
 * A friend silently missing from the list reads as "we lost your friendship",
 * which is a worse failure than "Ana is in a game right now".
 *
 * The list is advisory: availability is re-checked server-side at invite time,
 * so a row that has gone stale simply yields FRIEND_NOT_AVAILABLE inline.
 */
export function InviteFriendsDialog({ open, roomId, onOpenChange }: InviteFriendsDialogProps) {
  const { t } = useTranslation();
  const { data: friends, isPending, isError } = useInvitableFriends(roomId, open);
  const inviteMutation = useInviteToRoomMutation();

  // Per-friend UI state, keyed by userId: which row is in flight, which have
  // been invited this session, and any inline per-row error.
  const [invited, setInvited] = useState<Record<number, boolean>>({});
  const [pendingId, setPendingId] = useState<number | null>(null);
  const [rowErrors, setRowErrors] = useState<Record<number, string>>({});

  // Per-row state is keyed by userId and lives as long as this component, which
  // RoomPage mounts unconditionally — so closing the dialog does NOT reset it.
  // Clear on each open instead: an invite that was declined, expired unanswered,
  // or died with a disconnect leaves the friend invitable again, and without this
  // their button stays "Invited" and disabled until a full page reload.
  useEffect(() => {
    if (open) {
      setInvited({});
      setRowErrors({});
    }
  }, [open]);

  function reasonLabel(friend: InvitableFriend): string {
    switch (friend.reason) {
      case "offline":
        return t("roomInvite.reasons.offline");
      case "in_match":
        return t("roomInvite.reasons.inMatch");
      case "in_room":
        return t("roomInvite.reasons.inRoom");
      case "in_this_room":
        // Seated at THIS table. Saying "in another room" about someone the host
        // can see in the seats is the one reason that reads as a bug.
        return t("roomInvite.reasons.alreadyHere");
      case "room_full":
        return t("roomInvite.reasons.roomFull");
      default:
        // A slug this client does not know still has to say something — an empty
        // string renders a disabled button above a blank line with no stated
        // cause, which reads as a broken row rather than an unavailable friend.
        return t("roomInvite.reasons.unavailable");
    }
  }

  async function invite(friend: InvitableFriend) {
    setPendingId(friend.userId);
    setRowErrors((prev) => {
      const next = { ...prev };
      delete next[friend.userId];
      return next;
    });
    try {
      await inviteMutation.mutateAsync({ roomId, friendUserId: friend.userId });
      setInvited((prev) => ({ ...prev, [friend.userId]: true }));
    } catch (err) {
      const code = err instanceof FetchError ? err.code : null;
      // Availability is recomputed server-side, so a row can legitimately go
      // stale between render and click — report it on the row, not as a toast
      // that loses track of WHICH friend failed.
      if (code === "FRIEND_NOT_AVAILABLE") {
        setRowErrors((prev) => ({ ...prev, [friend.userId]: t("roomInvite.errors.notAvailable") }));
      } else if (code === "ROOM_FULL") {
        setRowErrors((prev) => ({ ...prev, [friend.userId]: t("roomInvite.errors.roomFull") }));
      } else if (code === "NOT_FRIENDS") {
        setRowErrors((prev) => ({ ...prev, [friend.userId]: t("roomInvite.errors.notFriends") }));
      } else if (code === "INVITE_ALREADY_PENDING") {
        // They already have a live popup for this room — show it as sent rather
        // than as a failure, which is what it means from the host's side.
        setInvited((prev) => ({ ...prev, [friend.userId]: true }));
      } else if (code === "ROOM_NOT_FOUND" || code === "NOT_IN_ROOM") {
        // The room closed, the match started, or the host was kicked in another
        // tab. "Please try again" is a lie here — nothing on this panel can ever
        // succeed again, so close it instead of inviting a pointless retry.
        toast.error(t("roomInvite.errors.roomGone"));
        onOpenChange(false);
      } else {
        toast.error(t("roomInvite.errors.sendFailed"));
      }
    } finally {
      setPendingId(null);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent data-testid="invite-friends-dialog">
        <DialogHeader>
          <DialogTitle>{t("roomInvite.panel.title")}</DialogTitle>
          <DialogDescription>{t("roomInvite.panel.description")}</DialogDescription>
        </DialogHeader>

        {isPending ? (
          <p className="text-ink-mute px-1 py-2 text-xs">{t("roomInvite.panel.loading")}</p>
        ) : isError ? (
          <p className="text-destructive px-1 py-2 text-xs" data-testid="invite-friends-error">
            {t("roomInvite.panel.loadFailed")}
          </p>
        ) : !friends || friends.length === 0 ? (
          <p className="text-ink-dim px-1 py-2 text-xs" data-testid="invite-friends-empty">
            {t("roomInvite.panel.empty")}
          </p>
        ) : (
          <ul className="flex max-h-80 flex-col gap-1 overflow-y-auto">
            {friends.map((friend) => {
              const isInvited = invited[friend.userId] === true;
              const rowError = rowErrors[friend.userId];
              const disabled = friend.available !== true || isInvited || pendingId !== null;
              return (
                <li
                  key={friend.userId}
                  className="flex items-center gap-2 rounded-lg px-1.5 py-2"
                  data-testid="invite-friend-row"
                  data-user-id={friend.userId}
                  data-available={friend.available === true ? "true" : "false"}
                >
                  <Avatar name={friend.username} size={28} className="shrink-0" />
                  <span className="flex min-w-0 flex-1 flex-col">
                    <span className="text-ink truncate font-medium">{friend.username}</span>
                    {(rowError !== undefined || friend.available !== true) && (
                      <span
                        className={
                          rowError !== undefined
                            ? "text-destructive text-[11px]"
                            : "text-ink-mute text-[11px]"
                        }
                        data-testid={`invite-friend-reason-${friend.userId}`}
                      >
                        {rowError ?? reasonLabel(friend)}
                      </span>
                    )}
                  </span>

                  <Button
                    type="button"
                    size="sm"
                    variant={isInvited ? "ghost" : "outline"}
                    disabled={disabled}
                    onClick={() => void invite(friend)}
                    data-testid={`invite-friend-${friend.userId}`}
                  >
                    {isInvited ? (
                      <>
                        <Check className="size-3" />
                        {t("roomInvite.panel.invited")}
                      </>
                    ) : (
                      <>
                        <Send className="size-3" />
                        {t("roomInvite.panel.invite")}
                      </>
                    )}
                  </Button>
                </li>
              );
            })}
          </ul>
        )}
      </DialogContent>
    </Dialog>
  );
}
