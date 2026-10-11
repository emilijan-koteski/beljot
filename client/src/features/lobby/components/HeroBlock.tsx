import { Activity, Coffee, DoorOpen, Gamepad2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import { CreateRoomTile } from "@/features/lobby/components/CreateRoomTile";
import { JoinByCodeTile } from "@/features/lobby/components/JoinByCodeTile";
import { QuickPlayTile } from "@/features/lobby/components/QuickPlayTile";
import { StatPill } from "@/features/lobby/components/StatPill";
import type { LobbyStats } from "@/shared/api/lobby";

type Props = {
  openCount: number;
  stats: LobbyStats | undefined;
  onQuickPlay: () => void;
  onCreateRoom: () => void;
  quickPlayDisabled?: boolean;
};

const ICON_CLS = "size-3.5";

export function HeroBlock({
  openCount,
  stats,
  onQuickPlay,
  onCreateRoom,
  quickPlayDisabled,
}: Props) {
  const { t } = useTranslation();

  return (
    <header className="mb-5 flex flex-col gap-4">
      <div className="flex flex-wrap items-end gap-6">
        <h1 className="font-display text-ink m-0 text-[38px] font-bold tracking-[-0.6px]">
          {t("lobby.title")}
          <span className="text-ink-mute ml-3.5 font-medium tabular-nums">
            {String(openCount).padStart(2, "0")}
          </span>
        </h1>

        <div
          className="ml-auto flex flex-wrap items-center gap-2.5"
          data-testid="lobby-stats-panel"
        >
          <StatPill
            icon={<Coffee className={ICON_CLS} />}
            label={t("lobby.stats.inLobby")}
            value={stats?.inLobby ?? "—"}
            testId="stats-in-lobby"
          />
          <StatPill
            icon={<DoorOpen className={ICON_CLS} />}
            label={t("lobby.stats.inRoom")}
            value={stats?.inRoom ?? "—"}
            testId="stats-in-room"
          />
          <StatPill
            icon={<Gamepad2 className={ICON_CLS} />}
            label={t("lobby.stats.inMatch")}
            value={stats?.inMatch ?? "—"}
            tone="accent"
            testId="stats-in-game"
          />
          <StatPill
            icon={<Activity className={ICON_CLS} />}
            label={t("lobby.stats.online")}
            value={stats ? `${stats.online} / ${stats.registered.toLocaleString()}` : "—"}
            testId="stats-active-ratio"
          />
        </div>
      </div>

      <div className="grid grid-cols-1 gap-3 md:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)_minmax(0,1.1fr)]">
        <QuickPlayTile onClick={onQuickPlay} disabled={quickPlayDisabled} />
        <CreateRoomTile onClick={onCreateRoom} />
        <JoinByCodeTile />
      </div>
    </header>
  );
}
