import { useTranslation } from "react-i18next";

import { TierBadge } from "@/shared/components/season/TierBadge";
import { AppDialog } from "@/shared/components/ui/app-dialog";
import {
  formatSpChange,
  SEASON_TIER_COLOR,
  seasonBarFill,
  seasonRankLabel,
  seasonSpOrZero,
  type SeasonTier,
} from "@/shared/lib/seasonTier";
import { cn } from "@/shared/lib/utils";

/** The rank step the bar fills, from the server's current-season read. */
export interface RankStepProgress {
  spIntoStep: number;
  spForNextStep: number;
}

interface RankChangeDialogProps {
  open: boolean;
  direction: "promoted" | "demoted";
  tier: SeasonTier;
  division: number | null;
  spChange: number;
  sp: number;
  progress: RankStepProgress | null;
  onClose: () => void;
}

/**
 * Post-match season rank-change announcement on the celebration shell.
 * Promotions colour the rank and light the badge; demotions keep the same layout neutral.
 */
export function RankChangeDialog({
  open,
  direction,
  tier,
  division,
  spChange,
  sp,
  progress,
  onClose,
}: RankChangeDialogProps) {
  const { t } = useTranslation();
  const promoted = direction === "promoted";
  const rank = seasonRankLabel(t, tier, division);
  const title = promoted ? t("season.rankUpDialog.title") : t("season.rankDownDialog.title");
  const color = SEASON_TIER_COLOR[tier];

  const intoStep = progress ? seasonSpOrZero(progress.spIntoStep) : 0;
  const forNextStep = progress ? seasonSpOrZero(progress.spForNextStep) : 0;
  const atTop = progress !== null && forNextStep <= 0;
  const pct = progress ? Math.round(seasonBarFill(intoStep, forNextStep) * 100) : 0;

  return (
    <AppDialog
      open={open}
      onClose={onClose}
      tone="celebration"
      art={<TierBadge tier={tier} size="lg" glow={promoted} data-testid="rank-change-badge" />}
      title={
        <>
          {title} <span className="sr-only">{rank}</span>
        </>
      }
      primary={{
        label: t("season.rankDialog.continue"),
        onClick: onClose,
        testId: "rank-change-continue",
      }}
      testId="rank-change-dialog"
    >
      <p
        data-testid="rank-change-rank"
        data-direction={direction}
        className={cn(
          "m-0 text-5xl leading-none font-bold tracking-tight",
          !promoted && "text-ink",
        )}
        style={promoted ? { color } : undefined}
      >
        {rank}
      </p>
      <p data-testid="rank-change-sp" className="text-ink-dim mt-2 mb-0 text-sm tabular-nums">
        {t("season.rankDialog.sp", { change: formatSpChange(spChange) })}
      </p>
      {progress ? (
        <div className="mt-5 w-full">
          <div className="text-ink-dim mb-1 flex justify-end text-xs tabular-nums">
            {atTop
              ? t("season.banner.atTop")
              : t("season.banner.progress", {
                  current: intoStep.toLocaleString(),
                  next: forNextStep.toLocaleString(),
                })}
          </div>
          <div
            data-testid="rank-change-progress"
            role="progressbar"
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={pct}
            aria-label={
              atTop
                ? t("season.banner.progressLabelTop", { tier: rank, sp: sp.toLocaleString() })
                : t("season.banner.progressLabel", {
                    tier: rank,
                    current: intoStep.toLocaleString(),
                    next: forNextStep.toLocaleString(),
                  })
            }
            className="bg-surface-sunken h-1.5 overflow-hidden rounded-full"
          >
            <div
              className="h-full rounded-full transition-[width] duration-500 ease-out"
              style={{ width: `${pct}%`, background: color }}
            />
          </div>
        </div>
      ) : (
        <div
          aria-hidden="true"
          data-testid="rank-change-progress-placeholder"
          className="mt-5 w-full"
        >
          <div className="mb-1 text-xs">{" "}</div>
          <div className="bg-surface-sunken h-1.5 rounded-full" />
        </div>
      )}
    </AppDialog>
  );
}
