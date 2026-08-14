import { useTranslation } from "react-i18next";

import type { BestHand } from "@/shared/api/career";
import { formatLocalizedDate } from "@/shared/lib/formatDate";

import { SidePanel } from "./SidePanel";

type MilestonesProps = {
  capots: number;
  /** Lifetime total game points scored across completed matches (Story 11.3). */
  careerPoints: number;
  bestHand?: BestHand;
  avgMatchSeconds: number;
};

function MilestoneRow({ label, hint, value }: { label: string; hint: string; value: string }) {
  return (
    <li className="bg-surface-elevated border-border grid grid-cols-[1fr_auto] items-center gap-2.5 rounded-[10px] border px-3 py-2.5">
      <div className="min-w-0">
        <div className="text-ink text-[13px] font-medium">{label}</div>
        <div className="text-ink-mute mt-0.5 truncate text-[11.5px]">{hint}</div>
      </div>
      <div className="text-ink font-display self-center text-[22px] font-bold tracking-[-0.5px] tabular-nums">
        {value}
      </div>
    </li>
  );
}

/**
 * Sidebar panel of three career highlights: capots called, best single hand,
 * and average match length. Driven by the career aggregates endpoint.
 */
export function Milestones({ capots, careerPoints, bestHand, avgMatchSeconds }: MilestonesProps) {
  const { t } = useTranslation();
  const avgMinutes = Math.round(avgMatchSeconds / 60);

  return (
    <SidePanel
      eyebrow={t("profile.milestones.eyebrow")}
      title={t("profile.milestones.title")}
      testId="profile-milestones"
    >
      <ul className="m-0 flex list-none flex-col gap-2 p-0">
        <MilestoneRow
          label={t("profile.milestones.careerPointsLabel")}
          hint={t("profile.milestones.careerPointsHint")}
          value={careerPoints.toLocaleString()}
        />
        <MilestoneRow
          label={t("profile.milestones.capotsLabel")}
          hint={t("profile.milestones.capotsHint")}
          value={String(capots)}
        />
        <MilestoneRow
          label={t("profile.milestones.bestHandLabel")}
          hint={
            bestHand
              ? t("profile.milestones.bestHandHint", {
                  hand: bestHand.handNumber,
                  date: formatLocalizedDate(bestHand.completedAt, t, "short"),
                })
              : t("profile.milestones.bestHandHintEmpty")
          }
          value={bestHand ? String(bestHand.points) : t("profile.milestones.bestHandEmpty")}
        />
        <MilestoneRow
          label={t("profile.milestones.avgLengthLabel")}
          hint={t("profile.milestones.avgLengthHint")}
          value={t("profile.milestones.avgLengthValue", { minutes: avgMinutes })}
        />
      </ul>
    </SidePanel>
  );
}
