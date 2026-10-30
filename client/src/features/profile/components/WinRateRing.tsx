import { useTranslation } from "react-i18next";

import { useReducedMotion } from "@/shared/hooks/useReducedMotion";

type WinRateRingProps = {
  rate: number | null;
};

const SIZE = 96;
const STROKE = 9;

/**
 * Win-rate ring for the Career card: a felt-green arc over a sunken
 * track, with the percentage + "Win rate" label centered. The arc animates on
 * change unless the user prefers reduced motion.
 */
export function WinRateRing({ rate }: WinRateRingProps) {
  const { t } = useTranslation();
  const reducedMotion = useReducedMotion();

  const r = (SIZE - STROKE) / 2;
  const circumference = 2 * Math.PI * r;
  const pct = rate ?? 0;
  const filled = circumference * (pct / 100);

  return (
    <div
      className="relative flex shrink-0 items-center justify-center"
      style={{ width: SIZE, height: SIZE }}
      data-testid="profile-win-rate-ring"
      data-rate={rate === null ? "" : String(rate)}
    >
      <svg
        width={SIZE}
        height={SIZE}
        className="absolute inset-0"
        style={{ transform: "rotate(-90deg)" }}
        aria-hidden="true"
      >
        <circle
          cx={SIZE / 2}
          cy={SIZE / 2}
          r={r}
          fill="none"
          stroke="var(--surface-3)"
          strokeWidth={STROKE}
        />
        <circle
          cx={SIZE / 2}
          cy={SIZE / 2}
          r={r}
          fill="none"
          stroke="var(--accent)"
          strokeWidth={STROKE}
          strokeLinecap="round"
          strokeDasharray={`${filled} ${circumference}`}
          style={{ transition: reducedMotion ? undefined : "stroke-dasharray .8s ease" }}
        />
      </svg>
      <div className="flex flex-col items-center">
        <span className="text-ink text-xl leading-none font-bold tracking-tight tabular-nums">
          {rate === null ? t("profile.stats.winRateEmpty") : `${rate}%`}
        </span>
        <span className="text-ink-mute mt-1 max-w-18 text-center text-2xs leading-tight font-semibold">
          {t("profile.winRate")}
        </span>
      </div>
    </div>
  );
}
