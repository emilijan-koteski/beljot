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

/** Band boundaries worth labelling under the track — every tier floor above 0,
 *  plus the 0 and 100 ends. Derived, so a retuned floor relabels itself. */
const TICKS = [0, ...HONOR_TIER_BANDS.filter((b) => b.from > 0).map((b) => b.from), 100];

type HonorBandMeterProps = {
  /** The score to mark on the track. Omit for an unmarked reference scale. */
  score?: number;
  /** Show the tier name under each band. Used by the explainer, not the profile. */
  showTierLabels?: boolean;
  /** Show the numeric boundary ticks under the track. */
  showTicks?: boolean;
  className?: string;
  testId?: string;
};

/**
 * The honour scale drawn as five BANDS rather than a single fill.
 *
 * A flat fill answers "how much of 100 do I have", which is not the question
 * players actually have — they want to know which tier they are in and how far
 * the next one is. Banding the track and putting a marker on it answers both at
 * a glance, and it is why the shipped flat meter is replaced rather than
 * recoloured.
 *
 * Bands, their widths and the tick labels all come from HONOR_TIER_BANDS, which
 * is itself derived from the tier floors — so this cannot drift from the tiers
 * it draws.
 */
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

  // Bands are painted at their true positions and held well back in opacity: the
  // marker has to be the thing the eye lands on, not the ramp behind it.
  const gradient = `linear-gradient(90deg, ${HONOR_TIER_BANDS.map(
    (b) => `${HONOR_TIER_COLOR[b.tier]} ${b.from}% ${b.to}%`,
  ).join(", ")})`;

  return (
    // @container: the tick row thins itself against the METER's width, not the
    // viewport — the same meter is a wide hero band on the profile and a ~420px
    // row inside the explainer dialog.
    <div className={cn("@container flex flex-col", className)} data-testid={testId}>
      <div className="relative h-2 w-full">
        <div
          className="absolute inset-0 rounded-[4px]"
          style={{ background: gradient, opacity: 0.24 }}
          aria-hidden="true"
        />
        {hasScore && (
          <>
            {/* Filled portion up to the score, in that score's own tier colour —
                so the bar's weight says "this is where you are", while the faint
                bands behind say "and this is the map". */}
            <div
              className="absolute inset-y-0 left-0 rounded-[4px]"
              style={{
                width: `${value}%`,
                background: HONOR_TIER_COLOR[tier],
                opacity: 0.85,
                transition: reducedMotion ? undefined : "width 420ms ease-out",
              }}
              aria-hidden="true"
            />
            <div
              className="absolute top-1/2 h-4 w-[3px] -translate-x-1/2 -translate-y-1/2 rounded-[2px]"
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

      {/* Ticks are placed at their TRUE position on the track (left: v%), not
          spread evenly. Evenly spread is what shipped, and it made the axis lie:
          six labels at 0/20/40/60/80/100% of the width put "95" at 80%, so a
          marker at its honest 87% landed to the RIGHT of the 95 label and read
          as near-perfect. The track, the fill and the marker are all linear in
          the score, so the labels have to be too. */}
      {showTicks && (
        <div className="text-ink-mute relative mt-1.5 h-3 font-mono text-[9.5px] leading-none tracking-[0.6px]">
          {TICKS.map((v, i) => {
            const prev = TICKS[i - 1];
            // The 100 end tick is the only droppable label: it marks no tier
            // floor, and it is the one that collides with the top tier's floor
            // (5 points away) on a narrow track. Below ~30rem of meter width it
            // gives way to that floor, which is the tick carrying information.
            const droppable = v === 100 && prev !== undefined && v - prev < 12;
            return (
              <span
                key={v}
                className={cn(
                  "absolute top-0 tabular-nums",
                  droppable && "hidden @min-[30rem]:block",
                )}
                style={{
                  left: `${v}%`,
                  // The end labels hang inside the track instead of straddling
                  // its ends, so neither is clipped by the meter's own box.
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
        <div className="mt-1 flex text-[10px] font-semibold" aria-hidden="true">
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
