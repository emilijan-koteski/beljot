import { useState } from "react";
import { useTranslation } from "react-i18next";

import type { MatchListItem } from "@/shared/api/matches";
import { MatchPlayerActions } from "@/shared/components/matchStats/MatchPlayerActions";
import { MatchStatsCard } from "@/shared/components/matchStats/MatchStatsCard";
import { Badge } from "@/shared/components/ui/badge";

interface RoomLastMatchProps {
  match: MatchListItem;
  roomId: number;
  playersInRoom: number[];
  compact: boolean;
}

/** The Last match tab: the room's latest match with every hand, and Invite back on each player who left. */
export function RoomLastMatch({ match, roomId, playersInRoom, compact }: RoomLastMatchProps) {
  const [open, setOpen] = useState(true);
  return (
    <ul
      className="bg-surface border-border divide-border m-0 flex list-none flex-col divide-y overflow-hidden rounded-xl border p-0 shadow-xs"
      data-testid="last-match-panel"
    >
      <MatchStatsCard
        match={match}
        isOpen={open}
        onToggle={() => setOpen((v) => !v)}
        handsLayout={compact ? "stacked" : "auto"}
        footer={
          <MatchPlayerActions
            match={match}
            roomId={roomId}
            showReinvite
            playersInRoom={playersInRoom}
          />
        }
      />
    </ul>
  );
}

/** The Win or Loss tag on the Last match tab. */
export function LastMatchOutcome({ outcome }: { outcome: MatchListItem["outcome"] }) {
  const { t } = useTranslation();
  if (outcome === "abandoned") return null;
  return (
    <Badge size="sm" tone={outcome === "win" ? "accent" : "danger"}>
      {outcome === "win"
        ? t("profile.matchHistory.outcomeWin")
        : t("profile.matchHistory.outcomeLoss")}
    </Badge>
  );
}
