import { type CSSProperties, useCallback, useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";

import { useSfxOnce } from "@/shared/audio/useSfxOnce";
import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { MOTION, motionDuration } from "@/shared/lib/motion";
import { Z } from "@/shared/lib/zLayers";

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
        background:
          "radial-gradient(ellipse at center, rgba(0,0,0,0.66) 0%, rgba(0,0,0,0.82) 100%)",
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
              textShadow: `0 0 30px ${bright}80, 0 4px 18px rgba(0,0,0,0.6)`,
              "--cap-glow": `${bright}80`,
            } as CSSProperties
          }
        >
          {t("match.capot.title")}
        </h1>

        <div
          data-testid="capot-bonus"
          className=""
          style={{
            marginTop: 22,
            display: "inline-flex",
            alignItems: "center",
            gap: 10,
            padding: "8px 16px",
            borderRadius: 999,
            background: `${bright}1f`,
            border: `1px solid ${bright}88`,
            fontSize: 14,
            letterSpacing: "0.18em",
            textTransform: "uppercase",
            color: bright,
            fontWeight: 700,
          }}
        >
          <span
            style={{
              width: 7,
              height: 7,
              borderRadius: "50%",
              background: bright,
              boxShadow: `0 0 8px ${bright}`,
            }}
          />
          {isUs
            ? t("match.capot.bonusUs", { points: capotBonus })
            : t("match.capot.bonusThem", { points: capotBonus })}
        </div>
      </div>
    </div>
  );
}
