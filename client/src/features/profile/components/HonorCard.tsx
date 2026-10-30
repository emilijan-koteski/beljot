import { Minus, TrendingDown, TrendingUp } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { HonorBandMeter } from "@/shared/components/HonorBandMeter";
import { HonorExplainerDialog } from "@/shared/components/HonorExplainerDialog";
import { HonorShield } from "@/shared/components/HonorShield";
import { Button } from "@/shared/components/ui/button";
import { Panel } from "@/shared/components/ui/panel";
import {
  HONOR_NEW_PLAYER_MIN_MATCHES,
  HONOR_TIER_COLOR,
  HONOR_TREND_WINDOW,
  honorCountOrZero,
  honorIsNewPlayer,
  honorScoreOrPrior,
  type HonorTrendDirection,
  normalizeHonorTier,
  normalizeHonorTrendDirection,
} from "@/shared/lib/honor";

export type HonorCardProps = {
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
 * The Honor card: score and tier with the recent trend, the band meter, and finished versus left-early counts.
 * Newcomers see match progress instead of a score; the real score stays on the data attributes.
 */
export function HonorCard({
  score: rawScore,
  tier,
  completedTotal: rawCompletedTotal,
  abandonedTotal: rawAbandonedTotal,
  isNewPlayer: rawIsNewPlayer,
  trendDelta: rawTrendDelta,
  trendDirection,
  subjectIsSelf = true,
}: HonorCardProps) {
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
    <Panel
      icon={<HonorShield tier={isNewPlayer ? "fair" : resolvedTier} size={16} />}
      title={t("profile.honor.eyebrow")}
      aside={
        <Button
          variant="link"
          size="inline"
          className="text-xs"
          onClick={() => setExplainerOpen(true)}
          data-testid="profile-honor-explainer-button"
        >
          {t("profile.honor.explainer.open")}
        </Button>
      }
    >
      <div
        className="flex flex-col gap-3"
        data-testid="profile-honor"
        data-honor={score}
        data-tier={resolvedTier}
        data-new-player={String(isNewPlayer)}
        data-trend-direction={direction}
      >
        <div className="flex items-center gap-3">
          {isNewPlayer ? (
            <span
              className="text-ink text-2xl leading-none font-bold tabular-nums tracking-tight"
              data-testid="profile-honor-new"
              data-value={played}
            >
              {t("profile.honor.newPlayerProgress", {
                played,
                total: HONOR_NEW_PLAYER_MIN_MATCHES,
              })}
            </span>
          ) : (
            <span className="flex items-baseline gap-2">
              <span
                className="text-ink text-3xl leading-none font-bold tracking-tight tabular-nums"
                data-testid="profile-honor-score"
              >
                {score}
              </span>
              <span
                className="text-sm font-semibold"
                style={{ color: HONOR_TIER_COLOR[resolvedTier] }}
                data-testid="profile-honor-tier"
              >
                {t(`profile.honor.tier.${resolvedTier}`)}
              </span>
            </span>
          )}
          {!isNewPlayer && (
            <span
              className="bg-surface-sunken ml-auto inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-bold tabular-nums"
              style={{ color: TREND_COLOR[direction] }}
              data-testid="profile-honor-trend"
              data-trend-direction={direction}
              data-trend-delta={String(trendDelta)}
              title={t(
                subjectIsSelf ? "profile.honor.trendTooltip" : "profile.honor.trendTooltipPublic",
                { window: HONOR_TREND_WINDOW },
              )}
            >
              <TrendIcon className="size-3.5 shrink-0" aria-hidden="true" />
              {direction === "flat" ? null : `${trendDelta > 0 ? "+" : ""}${trendDelta}`}
              <span className="sr-only">{t(`profile.honor.trend.${direction}`)}</span>
            </span>
          )}
        </div>

        {isNewPlayer ? (
          <p className="text-ink-dim m-0 text-sm" data-testid="profile-honor-new-hint">
            {t(subjectIsSelf ? "profile.honor.newPlayerHint" : "profile.honor.newPlayerHintPublic")}
          </p>
        ) : (
          <HonorBandMeter score={score} testId="profile-honor-meter" />
        )}

        <span className="text-ink-mute text-xs tabular-nums" data-testid="profile-honor-record">
          {t("profile.honor.record", { finished: completedTotal, left: abandonedTotal })}
        </span>
      </div>

      <HonorExplainerDialog open={explainerOpen} onOpenChange={setExplainerOpen} />
    </Panel>
  );
}
