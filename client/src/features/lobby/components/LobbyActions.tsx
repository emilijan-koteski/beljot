import { Hash, Plus, Zap } from "lucide-react";
import { useTranslation } from "react-i18next";

import { JoinByCodeTile } from "@/features/lobby/components/JoinByCodeTile";
import { ActionTile } from "@/shared/components/ui/action-tile";
import type { LayoutBand } from "@/shared/hooks/useLayoutBand";

interface LobbyActionsProps {
  band: LayoutBand;
  onQuickPlay: () => void;
  quickPlayDisabled?: boolean;
  onOpenCode: () => void;
  onNewRoom: () => void;
}

/**
 * The lobby's action tiles: Quick Play and the room-code tile, plus New room from a tablet up.
 * A desktop types the code into its tile, while smaller screens open the code sheet.
 */
export function LobbyActions({
  band,
  onQuickPlay,
  quickPlayDisabled,
  onOpenCode,
  onNewRoom,
}: LobbyActionsProps) {
  const { t } = useTranslation();
  const wide = band !== "phone";
  return (
    <div
      className={wide ? "grid grid-cols-3 gap-3" : "grid grid-cols-2 gap-2.5"}
      data-testid="lobby-actions"
    >
      <ActionTile
        tone="primary"
        icon={<Zap aria-hidden="true" />}
        title={t("lobby.actions.quickPlay.title")}
        subtitle={t("lobby.actions.quickPlay.subtitle")}
        onClick={onQuickPlay}
        disabled={quickPlayDisabled}
        data-testid="quick-play-card"
      />
      {band === "desktop" ? (
        <JoinByCodeTile />
      ) : (
        <ActionTile
          icon={<Hash className="text-brass-deep" aria-hidden="true" />}
          title={t("lobby.actions.joinByCode.title")}
          subtitle={t("lobby.actions.joinByCode.subtitle")}
          onClick={onOpenCode}
          data-testid="join-by-code-card"
        />
      )}
      {wide && (
        <ActionTile
          icon={<Plus className="text-accent" aria-hidden="true" />}
          title={t("lobby.actions.createRoom.title")}
          subtitle={t("lobby.actions.createRoom.subtitle")}
          onClick={onNewRoom}
          data-testid="create-room-card"
        />
      )}
    </div>
  );
}
