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
import { inviteFailure } from "@/shared/lib/inviteFailure";
import type { InvitableFriend } from "@/shared/types/apiTypes";

interface InviteFriendsDialogProps {
  open: boolean;
  roomId: number;
  onOpenChange: (open: boolean) => void;
}

/**
 * Inviter-side panel listing the viewer's friends with server-computed availability, unavailable ones disabled with a reason.
 * Per-row state resets on each open because RoomPage keeps this dialog mounted and earlier invites may have lapsed.
 */
export function InviteFriendsDialog({ open, roomId, onOpenChange }: InviteFriendsDialogProps) {
  const { t } = useTranslation();
  const { data: friends, isPending, isError } = useInvitableFriends(roomId, open);
  const inviteMutation = useInviteToRoomMutation();

  const [invited, setInvited] = useState<Record<number, boolean>>({});
  const [pendingId, setPendingId] = useState<number | null>(null);
  const [rowErrors, setRowErrors] = useState<Record<number, string>>({});

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
        return t("roomInvite.reasons.alreadyHere");
      case "room_full":
        return t("roomInvite.reasons.roomFull");
      default:
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
      const { kind, message } = inviteFailure(err instanceof FetchError ? err.code : null);
      if (kind === "alreadyPending") {
        setInvited((prev) => ({ ...prev, [friend.userId]: true }));
      } else if (kind === "roomGone") {
        toast.error(message);
        onOpenChange(false);
      } else {
        setRowErrors((prev) => ({ ...prev, [friend.userId]: message }));
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
                  <Avatar
                    name={friend.username}
                    avatarUrl={friend.avatarUrl}
                    size={28}
                    className="shrink-0"
                  />
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
