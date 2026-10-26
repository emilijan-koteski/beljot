import { useTranslation } from "react-i18next";

import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import {
  HONOR_TIER_BANDS,
  HONOR_TIER_COLOR,
  honorScoreOrPrior,
  type HonorTier,
  honorTierForScore,
} from "@/shared/lib/honor";
import { cn } from "@/shared/lib/utils";

const BAND_BOUNDARY_TICKS = [
  0,
  ...HONOR_TIER_BANDS.filter((b) => b.from > 0).map((b) => b.from),
  100,
];

type HonorBandMeterProps = {
  score?: number;
  showTierLabels?: boolean;
  showTicks?: boolean;
  className?: string;
  testId?: string;
};

/** Draws the honour scale as five tier bands, with an optional score marker and true-position ticks. */
export function HonorBandMeter({
  score,
  showTierLabels = false,
  showTicks = true,
  className,
  testId,
}: HonorBandMeterProps) {
  const { t } = useTranslation();
  const reducedMotion = useReducedMotion();
  const hasScore = typeof score === "number" && Number.isFinite(score);
  const value = hasScore ? Math.min(100, Math.max(0, honorScoreOrPrior(score))) : 0;
  const tier: HonorTier = honorTierForScore(value);

  const gradient = `linear-gradient(90deg, ${HONOR_TIER_BANDS.map(
    (b) => `${HONOR_TIER_COLOR[b.tier]} ${b.from}% ${b.to}%`,
  ).join(", ")})`;

  return (
    <div className={cn("@container flex flex-col", className)} data-testid={testId}>
      <div className="relative h-2 w-full">
        <div
          className="absolute inset-0 rounded"
          style={{ background: gradient, opacity: 0.24 }}
          aria-hidden="true"
        />
        {hasScore && (
          <>
            <div
              className="absolute inset-y-0 left-0 rounded"
              style={{
                width: `${value}%`,
                background: HONOR_TIER_COLOR[tier],
                opacity: 0.85,
                transition: reducedMotion ? undefined : "width 420ms ease-out",
              }}
              aria-hidden="true"
            />
            <div
              className="absolute top-1/2 h-4 w-[3px] -translate-x-1/2 -translate-y-1/2 rounded-xs"
              style={{
                left: `${value}%`,
                background: "var(--ink)",
                transition: reducedMotion ? undefined : "left 420ms ease-out",
              }}
              role="img"
              aria-label={t("profile.honor.meterLabel", {
                score: value,
                tier: t(`profile.honor.tier.${tier}`),
              })}
              data-testid={testId ? `${testId}-marker` : undefined}
              data-value={value}
            />
          </>
        )}
      </div>

      {showTicks && (
        <div className="text-ink-mute relative mt-1.5 h-3 font-mono text-2xs leading-none tracking-wider">
          {BAND_BOUNDARY_TICKS.map((v, i) => {
            const prev = BAND_BOUNDARY_TICKS[i - 1];
            const endTickCrowdsTopFloor = v === 100 && prev !== undefined && v - prev < 12;
            return (
              <span
                key={v}
                className={cn(
                  "absolute top-0 tabular-nums",
                  endTickCrowdsTopFloor && "hidden @min-[30rem]:block",
                )}
                style={{
                  left: `${v}%`,
                  transform:
                    i === 0 ? undefined : v === 100 ? "translateX(-100%)" : "translateX(-50%)",
                }}
              >
                {v}
              </span>
            );
          })}
        </div>
      )}

      {showTierLabels && (
        <div className="mt-1 flex text-2xs font-semibold" aria-hidden="true">
          {HONOR_TIER_BANDS.map((b) => (
            <span
              key={b.tier}
              className="truncate text-center"
              style={{ width: `${b.to - b.from}%`, color: HONOR_TIER_COLOR[b.tier] }}
            >
              {t(`profile.honor.tier.${b.tier}`)}
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
