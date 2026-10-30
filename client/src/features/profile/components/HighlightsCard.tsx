import { Clock, Medal, Sparkles, Trophy } from "lucide-react";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import type { BestHand } from "@/shared/api/career";
import { Panel } from "@/shared/components/ui/panel";
import { formatLocalizedDate } from "@/shared/lib/formatDate";

type HighlightsCardProps = {
  capots: number;
  careerPoints: number;
  bestHand?: BestHand;
  avgMatchSeconds: number;
};

function Tile({
  icon,
  label,
  value,
  hint,
}: {
  icon: ReactNode;
  label: string;
  value: string;
  hint: string;
}) {
  return (
    <li className="bg-surface-sunken/60 flex min-w-0 flex-col gap-0.5 rounded-lg px-3 py-2.5">
      <span className="text-ink-mute flex items-start gap-1.5 text-2xs leading-tight font-semibold tracking-wider uppercase [&>svg]:mt-px [&>svg]:size-3 [&>svg]:shrink-0">
        {icon}
        <span>{label}</span>
      </span>
      <span className="text-ink text-lg font-bold tabular-nums">{value}</span>
      <span className="text-ink-mute truncate text-2xs">{hint}</span>
    </li>
  );
}

/** Career highlights in four tiles: points scored, capots, the best single hand and the average match length. */
export function HighlightsCard({
  capots,
  careerPoints,
  bestHand,
  avgMatchSeconds,
}: HighlightsCardProps) {
  const { t } = useTranslation();
  const avgMinutes = Math.round(avgMatchSeconds / 60);

  return (
    <Panel icon={<Sparkles />} title={t("profile.milestones.title")} testId="profile-milestones">
      <ul className="m-0 grid list-none grid-cols-2 gap-2 p-0">
        <Tile
          icon={<Trophy />}
          label={t("profile.milestones.careerPointsLabel")}
          value={careerPoints.toLocaleString()}
          hint={t("profile.milestones.careerPointsHint")}
        />
        <Tile
          icon={<Medal />}
          label={t("profile.milestones.capotsLabel")}
          value={String(capots)}
          hint={t("profile.milestones.capotsHint")}
        />
        <Tile
          icon={<Sparkles />}
          label={t("profile.milestones.bestHandLabel")}
          value={bestHand ? String(bestHand.points) : t("profile.milestones.bestHandEmpty")}
          hint={
            bestHand
              ? t("profile.milestones.bestHandHint", {
                  hand: bestHand.handNumber,
                  date: formatLocalizedDate(bestHand.completedAt, t, "short"),
                })
              : t("profile.milestones.bestHandHintEmpty")
          }
        />
        <Tile
          icon={<Clock />}
          label={t("profile.milestones.avgLengthLabel")}
          value={t("profile.milestones.avgLengthValue", { minutes: avgMinutes })}
          hint={t("profile.milestones.avgLengthHint")}
        />
      </ul>
    </Panel>
  );
}
