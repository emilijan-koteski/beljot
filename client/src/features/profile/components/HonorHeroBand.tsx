import { HelpCircle, Minus, TrendingDown, TrendingUp } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { HonorBandMeter } from "@/shared/components/HonorBandMeter";
import { HonorExplainerDialog } from "@/shared/components/HonorExplainerDialog";
import { HonorShield } from "@/shared/components/HonorShield";
import { Eyebrow } from "@/shared/components/ui/eyebrow";
import {
  HONOR_NEW_PLAYER_MIN_MATCHES,
  HONOR_TREND_WINDOW,
  honorCountOrZero,
  honorIsNewPlayer,
  honorScoreOrPrior,
  type HonorTrendDirection,
  normalizeHonorTier,
  normalizeHonorTrendDirection,
} from "@/shared/lib/honor";

export type HonorHeroBandProps = {
  score: number;
  tier: string;
  completedTotal: number;
  abandonedTotal: number;
  isNewPlayer: boolean;
  trendDelta: number;
  trendDirection: string;
  subjectIsSelf?: boolean;
};

const TREND_ICON: Record<HonorTrendDirection, typeof TrendingUp> = {
  up: TrendingUp,
  flat: Minus,
  down: TrendingDown,
};

const TREND_COLOR: Record<HonorTrendDirection, string> = {
  up: "var(--accent)",
  flat: "var(--ink-dim)",
  down: "var(--danger)",
};

/**
 * Full-width honour band at the foot of the identity hero: score and tier, band meter, and trend.
 * Newcomers see match progress instead of a score; the real score stays on the data attributes.
 */
export function HonorHeroBand({
  score: rawScore,
  tier,
  completedTotal: rawCompletedTotal,
  abandonedTotal: rawAbandonedTotal,
  isNewPlayer: rawIsNewPlayer,
  trendDelta: rawTrendDelta,
  trendDirection,
  subjectIsSelf = true,
}: HonorHeroBandProps) {
  const { t } = useTranslation();
  const [explainerOpen, setExplainerOpen] = useState(false);

  const score = honorScoreOrPrior(rawScore);
  const completedTotal = honorCountOrZero(rawCompletedTotal);
  const abandonedTotal = honorCountOrZero(rawAbandonedTotal);
  const trendDelta = honorCountOrZero(rawTrendDelta);
  const isNewPlayer = honorIsNewPlayer(rawIsNewPlayer);

  const resolvedTier = normalizeHonorTier(tier, score);
  const direction = normalizeHonorTrendDirection(trendDirection);
  const TrendIcon = TREND_ICON[direction];

  const played = completedTotal + abandonedTotal;

  return (
    <div
      className="border-border col-span-1 border-t pt-4 sm:col-span-2"
      data-testid="profile-honor"
      data-honor={score}
      data-tier={resolvedTier}
      data-new-player={String(isNewPlayer)}
      data-trend-direction={direction}
    >
      <div className="mb-1 flex items-center gap-1.5">
        <Eyebrow>{t("profile.honor.eyebrow")}</Eyebrow>
        <button
          type="button"
          onClick={() => setExplainerOpen(true)}
          className="text-brass-deep hover:text-ink flex shrink-0 cursor-pointer items-center justify-center rounded-full transition-colors focus-visible:ring-3 focus-visible:ring-ring/70 focus-visible:outline-none"
          aria-label={t("profile.honor.explainer.open")}
          title={t("profile.honor.explainer.open")}
          data-testid="profile-honor-explainer-button"
        >
          <HelpCircle className="size-3.5" aria-hidden="true" />
        </button>
      </div>

      <div className="flex flex-wrap items-center gap-x-5 gap-y-4">
        <div className="flex min-w-0 items-center gap-2.5">
          <HonorShield tier={isNewPlayer ? "fair" : resolvedTier} size={20} className="shrink-0" />
          {isNewPlayer ? (
            <span
              className="font-display text-ink text-[22px] leading-none font-bold tabular-nums"
              data-testid="profile-honor-new"
              data-value={played}
            >
              {t("profile.honor.newPlayerProgress", {
                played,
                total: HONOR_NEW_PLAYER_MIN_MATCHES,
              })}
            </span>
          ) : (
            <div className="flex items-baseline gap-2">
              <span
                className="font-display text-[26px] leading-none font-bold tracking-[-0.8px] tabular-nums"
                style={{ color: "var(--ink)" }}
                data-testid="profile-honor-score"
              >
                {score}
              </span>
              <span
                className="text-brass-deep font-mono text-[11px] font-semibold tracking-[1px] uppercase"
                data-testid="profile-honor-tier"
              >
                {t(`profile.honor.tier.${resolvedTier}`)}
              </span>
            </div>
          )}
        </div>

        <div className="min-w-45 flex-1">
          {isNewPlayer ? (
            <p className="text-ink-dim m-0 text-[13px]" data-testid="profile-honor-new-hint">
              {t(
                subjectIsSelf ? "profile.honor.newPlayerHint" : "profile.honor.newPlayerHintPublic",
              )}
            </p>
          ) : (
            <HonorBandMeter score={score} testId="profile-honor-meter" />
          )}
        </div>

        {!isNewPlayer && (
          <div
            className="flex flex-col gap-0.5"
            data-testid="profile-honor-trend"
            data-trend-direction={direction}
            data-trend-delta={String(trendDelta)}
            title={t(
              subjectIsSelf ? "profile.honor.trendTooltip" : "profile.honor.trendTooltipPublic",
              { window: HONOR_TREND_WINDOW },
            )}
          >
            <span className="text-brass-deep font-mono text-[9.5px] font-semibold tracking-[1.6px] uppercase">
              {t("profile.honor.lastWindow", { window: HONOR_TREND_WINDOW })}
            </span>
            <span
              className="inline-flex items-center gap-1 text-[15px] leading-none font-bold tabular-nums"
              style={{ color: TREND_COLOR[direction] }}
            >
              <TrendIcon className="size-3.5 shrink-0" aria-hidden="true" />
              {direction === "flat" ? null : `${trendDelta > 0 ? "+" : ""}${trendDelta}`}
              <span className="sr-only">{t(`profile.honor.trend.${direction}`)}</span>
            </span>
          </div>
        )}
      </div>

      <HonorExplainerDialog open={explainerOpen} onOpenChange={setExplainerOpen} />
    </div>
  );
}
