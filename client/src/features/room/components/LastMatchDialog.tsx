import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { MatchPlayerActions } from "@/shared/components/matchStats/MatchPlayerActions";
import { MatchStatsCard } from "@/shared/components/matchStats/MatchStatsCard";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/shared/components/ui/dialog";
import { useRoomLastMatchQuery } from "@/shared/hooks/queries/useMatches";

interface LastMatchDialogProps {
  open: boolean;
  roomId: number;
  onOpenChange: (open: boolean) => void;
  /** User ids currently in the room — a co-player already back at the table
   *  gets no Reinvite control. */
  playersInRoom?: number[];
}

/**
 * The room lobby's "how did that go?" panel: the room's most recent match with
 * the same per-hand breakdown the profile shows, plus per-player Add-friend and
 * Reinvite.
 *
 * Deliberately ONE match — there is no room match history, no pagination, no
 * list. Mirrors InviteFriendsDialog: mounted unconditionally by RoomPage, with
 * the transient state reset on each open rather than on unmount.
 */
export function LastMatchDialog({
  open,
  roomId,
  onOpenChange,
  playersInRoom = [],
}: LastMatchDialogProps) {
  const { t } = useTranslation();
  const query = useRoomLastMatchQuery(roomId, open);
  // Expanded by default: the breakdown IS the reason this dialog exists, so
  // opening it collapsed would cost every viewer one extra click.
  const [expanded, setExpanded] = useState(true);

  // Same reason as InviteFriendsDialog's reset: RoomPage mounts this
  // unconditionally, so closing the dialog does not unmount it. Re-expand on
  // each open so a viewer who collapsed it last time is not greeted by a
  // header-only card.
  useEffect(() => {
    if (open) setExpanded(true);
  }, [open]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-3xl" data-testid="last-match-dialog">
        <DialogHeader>
          <DialogTitle>{t("room.lastMatch.title")}</DialogTitle>
          <DialogDescription>{t("room.lastMatch.description")}</DialogDescription>
        </DialogHeader>

        {query.isPending ? (
          <p className="text-ink-mute px-1 py-2 text-xs" data-testid="last-match-loading">
            {t("room.lastMatch.loading")}
          </p>
        ) : query.data === undefined ? (
          // Covers the settled 404 and the exhausted retries alike — from the
          // viewer's side both mean "there is nothing to show here", and a
          // crash or a technical error code would be worse than a plain line.
          <p className="text-ink-dim px-1 py-2 text-xs" data-testid="last-match-unavailable">
            {t("room.lastMatch.unavailable")}
          </p>
        ) : (
          <ul className="m-0 max-h-[70vh] list-none overflow-y-auto p-0">
            <MatchStatsCard
              match={query.data}
              isOpen={expanded}
              onToggle={() => setExpanded((v) => !v)}
              footer={
                <MatchPlayerActions
                  match={query.data}
                  roomId={roomId}
                  showReinvite
                  playersInRoom={playersInRoom}
                />
              }
            />
          </ul>
        )}
      </DialogContent>
    </Dialog>
  );
}
