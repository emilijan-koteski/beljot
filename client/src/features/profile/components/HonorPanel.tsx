import { Minus, ShieldCheck, TrendingDown, TrendingUp } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/shared/components/ui/badge";
import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import {
  HONOR_TREND_WINDOW,
  honorBarFill,
  honorCountOrZero,
  honorIsNewPlayer,
  honorScoreOrPrior,
  type HonorTier,
  type HonorTrendDirection,
  normalizeHonorTier,
  normalizeHonorTrendDirection,
} from "@/shared/lib/honor";

import { SectionHeader } from "./SectionHeader";

type HonorPanelProps = {
  score: number;
  tier: string;
  completedTotal: number;
  abandonedTotal: number;
  isNewPlayer: boolean;
  trendDelta: number;
  trendDirection: string;
};

type BadgeTone = "brass" | "accent" | "neutral" | "danger";

/**
 * Tier presentation. Colour is NEVER the only signal — the tier word and the
 * numeric score always render alongside the tone (UX spec: "No information
 * conveyed exclusively through colour"). `meter` is the bar's fill colour.
 *
 * All values come from existing design tokens: no new gold token is introduced
 * (the coinGold.ts single-const precedent covers the one place true gold was
 * genuinely required).
 */
const TIER_PRESENTATION: Record<HonorTier, { badge: BadgeTone; meter: string; value: string }> = {
  exemplary: { badge: "brass", meter: "var(--brass-deep)", value: "var(--brass-deep)" },
  trusted: { badge: "accent", meter: "var(--accent)", value: "var(--accent)" },
  fair: { badge: "neutral", meter: "var(--ink-off)", value: "var(--ink)" },
  unreliable: { badge: "neutral", meter: "var(--ink-dim)", value: "var(--ink-dim)" },
  problematic: { badge: "danger", meter: "var(--danger)", value: "var(--danger)" },
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

function CountTile({ testId, label, value }: { testId: string; label: string; value: number }) {
  return (
    <div
      className="bg-surface-2 border-border flex flex-col gap-1 rounded-lg border px-4 py-3"
      data-testid={testId}
      data-value={String(value)}
    >
      <span className="text-brass-deep font-mono text-[10.5px] font-semibold tracking-[2px] uppercase">
        {label}
      </span>
      <span className="font-display text-ink text-[26px] leading-none font-bold tabular-nums">
        {value}
      </span>
    </div>
  );
}

/**
 * Honor surface on the profile (Story 9.7). Shows the numeric score, the tier
 * label, the raw completed / abandoned counts, and the recent-form trend.
 *
 * Below the matches-played floor (completed + abandoned < 5 — it counts
 * experience, not successes) the server flags `isNewPlayer`: the score and
 * tier are then replaced by a "New Player" chip while the raw counts KEEP
 * rendering (PO decision) — a newcomer sees their progress toward earning a
 * score rather than a number computed from almost no evidence.
 *
 * Every value here is server-authoritative. The client mirror in
 * shared/lib/honor.ts only buckets a score into a colour, and never decides
 * access — Story 9.8's room gate is enforced server-side.
 */
export function HonorPanel({
  score: rawScore,
  tier,
  completedTotal: rawCompletedTotal,
  abandonedTotal: rawAbandonedTotal,
  isNewPlayer: rawIsNewPlayer,
  trendDelta: rawTrendDelta,
  trendDirection,
}: HonorPanelProps) {
  const { t } = useTranslation();
  const prefersReducedMotion = useReducedMotion();

  // The HTTP profile response is not type-guarded the way the WS payload is, so a
  // bundle newer than the server (or a server rolled back mid-deploy) can hand us
  // `undefined` for any of these. Unguarded, the score rendered blank and the tier
  // fell through to "problematic" — the worst band — instead of the 80 prior, the
  // counts rendered blank with data-value="undefined", and `undefined` isNewPlayer
  // was falsy so the numeric branch showed a confident score for accounts that
  // should have been suppressed. Every coercion below is Number.isFinite /
  // typeof-based, never truthiness, so a real 0 or false survives untouched.
  // (Code review 2026-07-29, extended in pass 2.)
  const score = honorScoreOrPrior(rawScore);
  const completedTotal = honorCountOrZero(rawCompletedTotal);
  const abandonedTotal = honorCountOrZero(rawAbandonedTotal);
  const trendDelta = honorCountOrZero(rawTrendDelta);
  const isNewPlayer = honorIsNewPlayer(rawIsNewPlayer);

  const resolvedTier = normalizeHonorTier(tier, score);
  const presentation = TIER_PRESENTATION[resolvedTier];
  const direction = normalizeHonorTrendDirection(trendDirection);
  const TrendIcon = TREND_ICON[direction];

  return (
    <section className="mb-5" data-testid="profile-honor-section">
      <SectionHeader
        eyebrow={t("profile.honor.eyebrow")}
        title={t("profile.honor.heading")}
        sub={t("profile.honor.sub")}
      />
      <div
        className="bg-surface border-border rounded-lg border p-5"
        data-testid="profile-honor"
        data-honor={String(score)}
        data-tier={resolvedTier}
        data-new-player={String(isNewPlayer)}
        data-trend-direction={direction}
      >
        {isNewPlayer ? (
          <div className="flex flex-wrap items-center gap-3" data-testid="profile-honor-new">
            <Badge tone="brass" icon={<ShieldCheck className="size-3.5" />}>
              {t("profile.honor.newPlayerChip")}
            </Badge>
            <p className="text-ink-dim m-0 text-[13.5px]">{t("profile.honor.newPlayerHint")}</p>
          </div>
        ) : (
          <>
            <div className="flex flex-wrap items-end gap-x-4 gap-y-2">
              <span
                className="font-display text-[44px] leading-none font-bold tracking-[-1.4px] tabular-nums"
                style={{ color: presentation.value }}
                data-testid="profile-honor-score"
              >
                {score}
              </span>
              <span className="text-ink-dim pb-1 text-[13px]">
                {t("profile.honor.outOf", { max: 100 })}
              </span>
              <span className="ml-auto flex items-center gap-2.5">
                <span data-testid="profile-honor-tier">
                  <Badge tone={presentation.badge}>{t(`profile.honor.tier.${resolvedTier}`)}</Badge>
                </span>
                {/* The trend renders an icon AND a signed number AND a word, so
                    it never depends on colour alone. */}
                <span
                  className="inline-flex items-center gap-1 text-xs font-semibold"
                  style={{ color: TREND_COLOR[direction] }}
                  data-testid="profile-honor-trend"
                  data-trend-delta={String(trendDelta)}
                  title={t("profile.honor.trendTooltip", { window: HONOR_TREND_WINDOW })}
                >
                  <TrendIcon className="size-3.5" aria-hidden="true" />
                  {t(`profile.honor.trend.${direction}`)}
                  {direction !== "flat" && (
                    <span className="tabular-nums">
                      {trendDelta > 0 ? `+${trendDelta}` : trendDelta}
                    </span>
                  )}
                </span>
              </span>
            </div>

            <div
              className="mt-3.5 h-2.5 overflow-hidden rounded-[5px]"
              style={{ background: "var(--surface-3)" }}
              role="img"
              aria-label={t("profile.honor.meterLabel", {
                score,
                tier: t(`profile.honor.tier.${resolvedTier}`),
              })}
              data-testid="profile-honor-meter"
            >
              <span
                className="block h-full"
                style={{
                  width: `${honorBarFill(score) * 100}%`,
                  background: presentation.meter,
                  transition: prefersReducedMotion ? "none" : "width 420ms ease-out",
                }}
              />
            </div>
          </>
        )}

        <div className="mt-4 grid grid-cols-2 gap-3">
          <CountTile
            testId="profile-honor-completed"
            label={t("profile.honor.completedLabel")}
            value={completedTotal}
          />
          <CountTile
            testId="profile-honor-abandoned"
            label={t("profile.honor.abandonedLabel")}
            value={abandonedTotal}
          />
        </div>
      </div>
    </section>
  );
}
