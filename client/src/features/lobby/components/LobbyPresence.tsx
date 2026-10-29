import { ChevronRight, Users } from "lucide-react";
import { useTranslation } from "react-i18next";

import type { LobbyStats } from "@/shared/api/lobby";
import { useFriends } from "@/shared/hooks/queries/useFriends";

/** The live head-counts in one mono line: players online and players in a match. */
export function LobbyCounts({ stats }: { stats: LobbyStats | undefined }) {
  const { t } = useTranslation();
  return (
    <span
      className="text-ink-mute font-mono text-xs whitespace-nowrap tabular-nums"
      data-testid="lobby-counts"
    >
      <span
        className="bg-accent mr-1.5 inline-block size-1.5 rounded-full align-middle"
        aria-hidden="true"
      />
      {t("lobby.stats.line", { online: stats?.online ?? "—", playing: stats?.inMatch ?? "—" })}
    </span>
  );
}

/** Friends online, opening the Friends sheet, beside the live head-counts; for screens without the Friends sidebar. */
export function PresenceLine({
  stats,
  onOpenFriends,
}: {
  stats: LobbyStats | undefined;
  onOpenFriends: () => void;
}) {
  const { t } = useTranslation();
  const { data: friends } = useFriends();
  const online = friends?.filter((f) => f.online === true).length ?? 0;
  return (
    <div className="flex items-center justify-between gap-2 text-xs">
      <button
        type="button"
        onClick={onOpenFriends}
        data-testid="open-friends"
        className="text-ink focus-visible:ring-ring/70 inline-flex cursor-pointer items-center gap-1.5 rounded-md font-semibold outline-none focus-visible:ring-3"
      >
        <Users className="text-ink-dim size-3.5" aria-hidden="true" />
        {t("friends.list.heading")}
        <span className="text-accent">{t("friends.onlineCount", { count: online })}</span>
        <ChevronRight className="text-ink-mute size-3.5" aria-hidden="true" />
      </button>
      <LobbyCounts stats={stats} />
    </div>
  );
}
