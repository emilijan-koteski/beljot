import { History } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { EmptyState } from "@/shared/components/EmptyState";
import { MatchPlayerActions } from "@/shared/components/matchStats/MatchPlayerActions";
import { MatchStatsCard } from "@/shared/components/matchStats/MatchStatsCard";
import { AppDialog } from "@/shared/components/ui/app-dialog";
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
    <AppDialog
      open={open}
      onClose={() => onOpenChange(false)}
      size="lg"
      icon={<History />}
      title={t("room.lastMatch.title")}
      primary={{
        label: t("common.close"),
        onClick: () => onOpenChange(false),
        testId: "last-match-close",
      }}
      testId="last-match-dialog"
    >
      {query.isPending ? (
        <p className="text-ink-mute px-1 py-2 text-xs" data-testid="last-match-loading">
          {t("room.lastMatch.loading")}
        </p>
      ) : query.data === undefined ? (
        <EmptyState compact testId="last-match-unavailable">
          {t("room.lastMatch.unavailable")}
        </EmptyState>
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
    </AppDialog>
  );
}
