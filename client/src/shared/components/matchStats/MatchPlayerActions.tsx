import { Check, Send } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { FetchError } from "@/shared/api/axiosClient";
import type { MatchListItem, MatchPlayer } from "@/shared/api/matches";
import { FriendButton } from "@/shared/components/friends/FriendButton";
import { Avatar } from "@/shared/components/ui/avatar";
import { Button } from "@/shared/components/ui/button";
import { useInviteToRoomMutation } from "@/shared/hooks/mutations/useRooms";
import { inviteFailure } from "@/shared/lib/inviteFailure";

interface MatchPlayerActionsProps {
  match: MatchListItem;
  roomId?: number;
  showReinvite?: boolean;
  playersInRoom?: number[];
  allowRemoveFriend?: boolean;
}

/**
 * Per-player Add-friend and (room lobby only) Reinvite actions for a finished match.
 * Skips bot seats, the viewer's own seat, and soft-deleted participants.
 */
export function MatchPlayerActions({
  match,
  roomId,
  showReinvite = false,
  playersInRoom = [],
  allowRemoveFriend = true,
}: MatchPlayerActionsProps) {
  const { t } = useTranslation();

  const others = match.players.filter(
    (p) => p.seat !== match.viewerSeat && p.isBot !== true && p.userId > 0 && p.username !== "",
  );
  if (others.length === 0) return null;

  return (
    <div className="border-border mt-3.5 border-t pt-3" data-testid="match-player-actions">
      <p className="text-brass-deep mb-2 font-mono text-2xs font-semibold tracking-caps uppercase">
        {t("matchStats.actionsTitle")}
      </p>
      <ul className="m-0 flex list-none flex-col gap-1 p-0">
        {others.map((player) => (
          <PlayerActionRow
            key={player.seat}
            player={player}
            roomId={roomId}
            showReinvite={showReinvite}
            alreadyInRoom={playersInRoom.includes(player.userId)}
            allowRemoveFriend={allowRemoveFriend}
          />
        ))}
      </ul>
    </div>
  );
}

interface PlayerActionRowProps {
  player: MatchPlayer;
  roomId?: number;
  showReinvite: boolean;
  alreadyInRoom: boolean;
  allowRemoveFriend: boolean;
}

/**
 * One player's row. "Invite back" is offered to friends and non-friends alike,
 * because the server enforces the invitee's opt-out and availability.
 */
function PlayerActionRow({
  player,
  roomId,
  showReinvite,
  alreadyInRoom,
  allowRemoveFriend,
}: PlayerActionRowProps) {
  const { t } = useTranslation();
  const inviteMutation = useInviteToRoomMutation();
  const [invited, setInvited] = useState(false);

  const canReinvite = showReinvite && roomId !== undefined && alreadyInRoom !== true;

  async function reinvite() {
    if (roomId === undefined) return;
    try {
      await inviteMutation.mutateAsync({ roomId, friendUserId: player.userId });
      setInvited(true);
      toast.success(t("matchStats.reinviteSent", { username: player.username }));
    } catch (err) {
      toast.error(inviteFailure(err instanceof FetchError ? err.code : null).message);
    }
  }

  return (
    <li
      className="flex flex-wrap items-center gap-2 rounded-lg px-1.5 py-1.5"
      data-testid="match-player-action-row"
      data-user-id={player.userId}
      data-seat={player.seat}
    >
      <Avatar name={player.username} avatarUrl={player.avatarUrl} size={24} className="shrink-0" />
      <span className="text-ink min-w-0 flex-1 truncate text-sm font-medium">
        {player.username}
      </span>
      <FriendButton
        userId={player.userId}
        username={player.username}
        compact
        allowRemove={allowRemoveFriend}
      />
      {canReinvite && (
        <Button
          type="button"
          size="sm"
          variant={invited ? "ghost" : "outline"}
          disabled={inviteMutation.isPending}
          onClick={() => void reinvite()}
          data-testid={`match-player-reinvite-${player.userId}`}
        >
          {invited ? (
            <>
              <Check className="size-3" />
              {t("matchStats.reinvited")}
            </>
          ) : (
            <>
              <Send className="size-3" />
              {t("matchStats.reinvite")}
            </>
          )}
        </Button>
      )}
    </li>
  );
}
