import { useSyncExternalStore } from "react";
import { useTranslation } from "react-i18next";

import { TierBadge } from "@/shared/components/season/TierBadge";
import {
  normalizeSeasonDivision,
  normalizeSeasonTier,
  SEASON_TIER_COLOR,
  SEASON_TIER_LINE,
  seasonBarFill,
  seasonDaysRemaining,
  seasonRankLabel,
  seasonSpOrZero,
} from "@/shared/lib/seasonTier";
import { getTimeTick, subscribeTimeTick } from "@/shared/lib/timeTick";
import type { CurrentSeasonResponse } from "@/shared/types/apiTypes";

type Props = {
  season: CurrentSeasonResponse | undefined;
};

/**
 * Presentational season rank banner: tier badge, rank, SP, next-rank-step progress, days left.
 * The season-rollover watch lives on the header rank chip; adding it here would double its toast.
 */
export function RankBanner({ season }: Props) {
  const { t } = useTranslation();
  useSyncExternalStore(subscribeTimeTick, getTimeTick, getTimeTick);

  if (!season) return null;

  const sp = seasonSpOrZero(season.sp);
  const tier = normalizeSeasonTier(season.rankTier, sp);
  const rankName = seasonRankLabel(t, tier, normalizeSeasonDivision(tier, season.rankDivision));
  const intoStep = seasonSpOrZero(season.spIntoDivision);
  const forNextStep = seasonSpOrZero(season.spForNextDivision);
  const fill = seasonBarFill(intoStep, forNextStep);
  const pct = Math.round(fill * 100);
  const atTop = forNextStep <= 0;
  const daysLeft = seasonDaysRemaining(season.endsAt, Date.now());

  const color = SEASON_TIER_COLOR[tier];

  return (
    <section
      data-testid="rank-banner"
      data-tier={tier}
      className="border-border bg-surface mb-5 flex flex-wrap items-center gap-4 rounded-xl border px-4 py-3.5"
      style={{ borderColor: SEASON_TIER_LINE[tier] }}
    >
      <TierBadge tier={tier} size="md" data-testid="rank-badge" />

      <div className="flex min-w-0 flex-1 flex-col gap-1.5">
        <div className="flex flex-wrap items-baseline gap-x-2.5 gap-y-1">
          <span
            data-testid="rank-tier-name"
            className="font-display text-base font-semibold"
            style={{ color }}
          >
            {rankName}
          </span>
          <span data-testid="rank-sp" className="text-ink-dim text-xs tabular-nums">
            {t("season.banner.sp", { sp: sp.toLocaleString() })}
          </span>
          <span
            data-testid="rank-season-days"
            className="text-ink-mute ml-auto text-xs tabular-nums"
            title={season.seasonName}
          >
            {t("season.banner.daysLeft", { days: daysLeft })}
          </span>
        </div>

        <div
          data-testid="rank-progress"
          role="progressbar"
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={pct}
          aria-label={
            atTop
              ? t("season.banner.progressLabelTop", { tier: rankName, sp: sp.toLocaleString() })
              : t("season.banner.progressLabel", {
                  tier: rankName,
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

        <span
          data-testid="rank-progress-caption"
          className="text-ink-mute text-[11px] tabular-nums"
        >
          {atTop
            ? t("season.banner.atTop")
            : t("season.banner.progress", {
                current: intoStep.toLocaleString(),
                next: forNextStep.toLocaleString(),
              })}
        </span>
      </div>
    </section>
  );
}
