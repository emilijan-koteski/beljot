import { Clock, Medal } from "lucide-react";
import { useTranslation } from "react-i18next";

import { TierBadge } from "@/shared/components/season/TierBadge";
import { Badge } from "@/shared/components/ui/badge";
import { Panel } from "@/shared/components/ui/panel";
import { SEASON_TIER_COLOR, SEASON_TIER_FLOORS, type SeasonTier } from "@/shared/lib/seasonTier";
import { cn } from "@/shared/lib/utils";

/** The season card of the desktop sidebar: its name and days left, and that ranks reset each season. */
export function SeasonPanel({ line }: { line: string }) {
  const { t } = useTranslation();
  return (
    <Panel icon={<Clock />} title={t("season.picker.label")} testId="leaderboard-season-panel">
      <div className="flex flex-col gap-1 text-sm">
        <span className="text-ink font-semibold">{line}</span>
        <span className="text-ink-mute text-xs">{t("season.leaderboard.resetNote")}</span>
      </div>
    </Panel>
  );
}

/** Every rank with its SP floor, highest first, the viewer's own tier marked. */
export function RankLadderPanel({ yours }: { yours: SeasonTier | null }) {
  const { t } = useTranslation();
  return (
    <Panel icon={<Medal />} title={t("season.leaderboard.ranks")} testId="leaderboard-ranks">
      <ul className="m-0 flex list-none flex-col gap-1 p-0">
        {[...SEASON_TIER_FLOORS].reverse().map(([tier, floor]) => (
          <li
            key={tier}
            className={cn(
              "flex items-center gap-2.5 rounded-md px-2 py-1",
              tier === yours && "bg-accent-soft/70",
            )}
            data-tier={tier}
          >
            <TierBadge tier={tier} size="sm" glow={false} />
            <span className="text-sm font-semibold" style={{ color: SEASON_TIER_COLOR[tier] }}>
              {t(`season.tier.${tier}`)}
            </span>
            {tier === yours && (
              <Badge size="sm" tone="accent">
                {t("season.leaderboard.you")}
              </Badge>
            )}
            <span className="text-ink-mute ml-auto text-xs tabular-nums">
              {t("season.leaderboard.floor", { sp: floor.toLocaleString() })}
            </span>
          </li>
        ))}
      </ul>
    </Panel>
  );
}
