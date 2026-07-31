import { ArrowRight, HelpCircle, Minus, TrendingDown, TrendingUp } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { HonorBandMeter } from "@/shared/components/HonorBandMeter";
import { HonorExplainerDialog } from "@/shared/components/HonorExplainerDialog";
import { HonorShield } from "@/shared/components/HonorShield";
import { Badge } from "@/shared/components/ui/badge";
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
};

const TREND_ICON: Record<HonorTrendDirection, typeof TrendingUp> = {
  up: TrendingUp,
  flat: Minus,
  down: TrendingDown,
};

/**
 * Trend direction tones. Deliberately NOT the tier ramp: direction is a
 * different axis from standing (a Problematic player who is climbing should read
 * as climbing), so these stay the theme's own semantic colours.
 */
const TREND_COLOR: Record<HonorTrendDirection, string> = {
  up: "var(--accent)",
  flat: "var(--ink-dim)",
  down: "var(--danger)",
};

function Stat({ testId, label, value }: { testId: string; label: string; value: string }) {
  return (
    <div className="flex flex-col gap-0.5" data-testid={testId} data-value={value}>
      <span className="text-brass-deep font-mono text-[9.5px] font-semibold tracking-[1.6px] uppercase">
        {label}
      </span>
      <span className="font-display text-ink text-[17px] leading-none font-bold tabular-nums">
        {value}
      </span>
    </div>
  );
}

/**
 * Honour as the identity hero's bottom band, replacing the standalone
 * HonorPanel section that Story 9.7 shipped.
 *
 * Why it moved: as its own section it read as a separate concern, when honour
 * belongs with the other things that describe WHO this player is — level, win
 * rate, coins. IdentityHero's own comment recorded that honour "needs more width
 * than this narrow column beside the XP bar", which was true of that column; the
 * answer is to span the full card under a hairline rule rather than to live
 * outside the card entirely.
 *
 * Layout is a WRAPPING FLEX ROW, not a fixed grid: the meter is the only elastic
 * cell, so the counts wrap as a block on narrow screens instead of the meter
 * being crushed. The explainer is an icon button on desktop — a text label there
 * would have to survive "Како функционира честа →" in mk — and becomes a labelled
 * link below sm, where there is a full row for it.
 */
export function HonorHeroBand({
  score: rawScore,
  tier,
  completedTotal: rawCompletedTotal,
  abandonedTotal: rawAbandonedTotal,
  isNewPlayer: rawIsNewPlayer,
  trendDelta: rawTrendDelta,
  trendDirection,
}: HonorHeroBandProps) {
  const { t } = useTranslation();
  const [explainerOpen, setExplainerOpen] = useState(false);

  // The HTTP profile response is not type-guarded the way the WS payload is, so a
  // bundle newer than the server can hand us `undefined` for any of these. Every
  // coercion is Number.isFinite / typeof-based, never truthiness, so a real 0 or
  // false survives. (Carried over from HonorPanel — code review 2026-07-29 and
  // its second pass both landed fixes here.)
  const score = honorScoreOrPrior(rawScore);
  const completedTotal = honorCountOrZero(rawCompletedTotal);
  const abandonedTotal = honorCountOrZero(rawAbandonedTotal);
  const trendDelta = honorCountOrZero(rawTrendDelta);
  const isNewPlayer = honorIsNewPlayer(rawIsNewPlayer);

  const resolvedTier = normalizeHonorTier(tier, score);
  const direction = normalizeHonorTrendDirection(trendDirection);
  const TrendIcon = TREND_ICON[direction];

  // Newcomer progress toward earning a score. Branches on the SERVER's flag; the
  // floor is only ever the denominator here.
  const played = completedTotal + abandonedTotal;

  return (
    <div
      className="border-border col-span-1 border-t pt-4 sm:col-span-2"
      data-testid="profile-honor"
      // The real score and tier stay on the data attributes even while
      // suppressed, deliberately: suppression is about not BRANDING a newcomer
      // with a number drawn from almost no evidence, not about hiding it from
      // downstream consumers. Carried over from HonorPanel, where a test pinned
      // this explicitly.
      data-honor={score}
      data-tier={resolvedTier}
      data-new-player={String(isNewPlayer)}
      data-trend-direction={direction}
    >
      <div className="flex flex-wrap items-center gap-x-5 gap-y-4">
        {/* Identity: label, then either the score + tier or the newcomer counter. */}
        <div className="flex min-w-0 items-center gap-2.5">
          <HonorShield tier={isNewPlayer ? "fair" : resolvedTier} size={20} className="shrink-0" />
          <div className="flex min-w-0 flex-col">
            <span className="text-brass-deep font-mono text-[9.5px] font-semibold tracking-[1.6px] uppercase">
              {t("profile.honor.topBarLabel")}
            </span>
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
                {/* Badge takes no arbitrary props, so the testid rides a wrapper
                    — same shape HonorPanel used. Tone stays NEUTRAL: across this
                    redesign only the shield is ever tinted, so the tier word does
                    not compete with it for the same signal. */}
                <span data-testid="profile-honor-tier">
                  <Badge tone="neutral">{t(`profile.honor.tier.${resolvedTier}`)}</Badge>
                </span>
              </div>
            )}
          </div>
        </div>

        {/* The one elastic cell. Suppressed for a newcomer: a marker on the ramp
            would be a claim about a score they have not earned. */}
        <div className="min-w-45 flex-1">
          {isNewPlayer ? (
            // Static copy on purpose: the "2 / 5" counter beside it already carries
            // the progress, and a count-dependent sentence would need plural forms
            // this project does not use anywhere (see profile.lastPlayed.daysAgo).
            <p className="text-ink-dim m-0 text-[13px]" data-testid="profile-honor-new-hint">
              {t("profile.honor.newPlayerHint")}
            </p>
          ) : (
            <HonorBandMeter score={score} testId="profile-honor-meter" />
          )}
        </div>

        {/* Counts + trend wrap together as one block rather than individually, so
            they never split across lines mid-group. */}
        <div className="flex items-center gap-5">
          <Stat
            testId="profile-honor-completed"
            label={t("profile.honor.completedLabel")}
            value={String(completedTotal)}
          />
          <Stat
            testId="profile-honor-abandoned"
            label={t("profile.honor.abandonedLabel")}
            value={String(abandonedTotal)}
          />
          {!isNewPlayer && (
            <div
              className="flex flex-col gap-0.5"
              data-testid="profile-honor-trend"
              data-trend-direction={direction}
              data-trend-delta={String(trendDelta)}
              title={t("profile.honor.trendTooltip", { window: HONOR_TREND_WINDOW })}
            >
              <span className="text-brass-deep font-mono text-[9.5px] font-semibold tracking-[1.6px] uppercase">
                {t("profile.honor.lastWindow", { window: HONOR_TREND_WINDOW })}
              </span>
              <span
                className="inline-flex items-center gap-1 text-[15px] leading-none font-bold tabular-nums"
                style={{ color: TREND_COLOR[direction] }}
              >
                <TrendIcon className="size-3.5 shrink-0" aria-hidden="true" />
                {/* A signed number is meaningless for "flat" — the icon says it. */}
                {direction === "flat" ? null : `${trendDelta > 0 ? "+" : ""}${trendDelta}`}
                <span className="sr-only">{t(`profile.honor.trend.${direction}`)}</span>
              </span>
            </div>
          )}
        </div>

        {/* Icon button from sm up; a labelled link below it. */}
        <button
          type="button"
          onClick={() => setExplainerOpen(true)}
          className="text-ink-dim hover:text-ink hover:border-border-2 border-border hidden size-8 shrink-0 cursor-pointer items-center justify-center rounded-full border transition-colors focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none sm:flex"
          aria-label={t("profile.honor.explainer.open")}
          title={t("profile.honor.explainer.open")}
          data-testid="profile-honor-explainer-button"
        >
          <HelpCircle className="size-4" aria-hidden="true" />
        </button>
        <button
          type="button"
          onClick={() => setExplainerOpen(true)}
          className="text-accent hover:text-accent-deep inline-flex w-full cursor-pointer items-center gap-1.5 text-[13px] font-semibold sm:hidden"
          data-testid="profile-honor-explainer-link"
        >
          {t("profile.honor.explainer.open")}
          <ArrowRight className="size-3.5" aria-hidden="true" />
        </button>
      </div>

      <HonorExplainerDialog open={explainerOpen} onOpenChange={setExplainerOpen} />
    </div>
  );
}
