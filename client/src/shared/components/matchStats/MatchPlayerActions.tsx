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
import { useFriendshipStatus } from "@/shared/hooks/queries/useFriendshipStatus";
import { inviteFailure } from "@/shared/lib/inviteFailure";

interface MatchPlayerActionsProps {
  match: MatchListItem;
  /**
   * Room to invite back into. Required for the Reinvite control to render at
   * all — without it there is nothing to invite anyone to.
   */
  roomId?: number;
  /**
   * Offer the Reinvite control. False in the end-of-match dialog: everyone is
   * still present and "Return to room" already covers regrouping.
   */
  showReinvite?: boolean;
  /** User ids currently in the room — someone already at the table needs no invite. */
  playersInRoom?: number[];
  /**
   * Offer "Remove friend" alongside the friendship state. FALSE in the
   * end-of-match overlay, whose z-order the confirm dialog cannot escape — see
   * FriendButton's `allowRemove`.
   */
  allowRemoveFriend?: boolean;
}

/**
 * Per-player follow-up actions for a finished match: Add-friend for everyone
 * you played with, plus (in the room lobby only) an invite back into the room.
 *
 * Rendered as the `footer` of `MatchStatsCard`, which is why it takes the match
 * rather than a player list — the seats, the bot flags and the viewer's own
 * seat all come from the same viewer-relative DTO the card draws.
 *
 * Bot seats and the viewer's own seat are skipped, as is a soft-deleted
 * participant (userId > 0 with an empty username): there is no account left to
 * befriend or invite.
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
      <p className="text-brass-deep mb-2 font-mono text-[10px] font-semibold tracking-[1.5px] uppercase">
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
 * One player's row. The friendship read lives here rather than in the parent
 * because it is a hook per subject; TanStack dedupes it against the identical
 * read inside `FriendButton`, so the row still costs one request.
 */
function PlayerActionRow({
  player,
  roomId,
  showReinvite,
  alreadyInRoom,
  allowRemoveFriend,
}: PlayerActionRowProps) {
  const { t } = useTranslation();
  const status = useFriendshipStatus(player.userId);
  const inviteMutation = useInviteToRoomMutation();
  // Confirmation only — NOT a latch. The invite's fate is server-side (declined,
  // lapsed on its TTL, or died with a disconnect), and this row cannot observe
  // any of them; disabling on success would strand a friend who is invitable
  // again behind a page reload. The button keeps its "Invited" read-back but
  // stays pressable, and a redundant press comes back as
  // INVITE_ALREADY_PENDING — which says exactly the right thing.
  const [invited, setInvited] = useState(false);

  // Reinvite is friends-only because POST /rooms/:id/invite is friends-only —
  // offering it to a stranger would render a button whose only outcome is
  // NOT_FRIENDS. It also hides for someone already at the table.
  const canReinvite =
    showReinvite &&
    roomId !== undefined &&
    status.data?.status === "friends" &&
    alreadyInRoom !== true;

  async function reinvite() {
    if (roomId === undefined) return;
    try {
      await inviteMutation.mutateAsync({ roomId, friendUserId: player.userId });
      setInvited(true);
      toast.success(t("matchStats.reinviteSent", { username: player.username }));
    } catch (err) {
      // Every kind is a toast here: unlike the invite panel's list rows, this
      // control has no inline slot of its own. The row stays interactive after
      // a rejection — room-full and not-available are both transient, and a
      // retry a moment later is the natural response.
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
      <Avatar name={player.username} size={24} className="shrink-0" />
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
