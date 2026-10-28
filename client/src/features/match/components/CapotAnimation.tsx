import { type CSSProperties, useCallback, useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";

import { useSfxOnce } from "@/shared/audio/useSfxOnce";
import { Badge } from "@/shared/components/ui/badge";
import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { MOTION, motionDuration } from "@/shared/lib/motion";
import { Z } from "@/shared/lib/zLayers";

import { overlayDimBackground } from "../lib/overlayDim";
import { seatTeam, teamColors } from "../lib/tableTheme";

interface CapotAnimationProps {
  capotTeam: number;
  viewerSeat: number;
  capotBonus: number;
  onComplete: () => void;
  soundKey?: string | null;
}

/** Full-screen capot celebration from the viewer's perspective; tapping skips early. */
export function CapotAnimation({
  capotTeam,
  viewerSeat,
  capotBonus,
  onComplete,
  soundKey,
}: CapotAnimationProps) {
  const { t } = useTranslation();
  useSfxOnce("capot", soundKey);
  const prefersReducedMotion = useReducedMotion();

  const done = useRef(false);
  const finish = useCallback(() => {
    if (done.current) return;
    done.current = true;
    onComplete();
  }, [onComplete]);

  useEffect(() => {
    const duration = motionDuration(
      prefersReducedMotion,
      MOTION.CAPOT_BANNER,
      MOTION.CAPOT_BANNER_REDUCED,
    );
    const timer = setTimeout(finish, duration);
    return () => clearTimeout(timer);
  }, [finish, prefersReducedMotion]);

  const team = seatTeam(capotTeam, viewerSeat);
  const [bright] = teamColors(team);
  const isUs = team === "gold";

  return (
    <div
      className="fixed inset-0 flex cursor-pointer items-center justify-center"
      style={{
        zIndex: Z.CAPOT,
        background: overlayDimBackground("spotlight"),
        backdropFilter: "blur(3px)",
      }}
      data-testid="capot-animation"
      onClick={finish}
    >
      <div
        className={
          prefersReducedMotion ? "text-center" : "text-center motion-safe:animate-capot-scale"
        }
      >
        <div
          data-testid="capot-eyebrow"
          className=""
          style={{
            fontSize: 12,
            letterSpacing: "0.18em",
            textTransform: "uppercase",
            color: bright,
            opacity: 0.85,
            marginBottom: 12,
          }}
        >
          {isUs ? t("match.capot.eyebrowUs") : t("match.capot.eyebrowThem")}
        </div>

        <h1
          data-testid="capot-text"
          data-team={team}
          className={`text-8xl font-bold tracking-tighter sm:text-9xl ${
            prefersReducedMotion ? "" : "motion-safe:animate-capot-shimmer"
          }`}
          style={
            {
              color: bright,
              margin: 0,
              lineHeight: 1,
              textShadow: `0 0 30px ${bright}80, 0 4px 18px var(--shade-60)`,
              "--cap-glow": `${bright}80`,
            } as CSSProperties
          }
        >
          {t("match.capot.title")}
        </h1>

        <Badge
          size="lg"
          tint={bright}
          className="mt-5.5"
          icon={
            <span
              aria-hidden
              className="size-1.75 rounded-full bg-current"
              style={{ boxShadow: "0 0 6px currentColor" }}
            />
          }
          data-testid="capot-bonus"
        >
          {isUs
            ? t("match.capot.bonusUs", { points: capotBonus })
            : t("match.capot.bonusThem", { points: capotBonus })}
        </Badge>
      </div>
    </div>
  );
}
