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
  playersInRoom?: number[];
}

/**
 * Room lobby panel showing the room's most recent match with its per-hand breakdown and per-player actions.
 * RoomPage keeps it mounted, so the card re-expands on each open rather than on mount.
 */
export function LastMatchDialog({
  open,
  roomId,
  onOpenChange,
  playersInRoom = [],
}: LastMatchDialogProps) {
  const { t } = useTranslation();
  const query = useRoomLastMatchQuery(roomId, open);
  const [expanded, setExpanded] = useState(true);

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
