import { type CSSProperties, useCallback, useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";

import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { MOTION, motionDuration } from "@/shared/lib/motion";
import { Z } from "@/shared/lib/zLayers";

import { seatTeam, teamColors } from "../lib/tableTheme";

interface CapotAnimationProps {
  /** Absolute capot team (0 = team A, 1 = team B). */
  capotTeam: number;
  /** Viewer's seat — used to render the moment from their perspective. */
  viewerSeat: number;
  /** Capot bonus actually awarded this hand (from the `hand_scored` payload). */
  capotBonus: number;
  onComplete: () => void;
}

/**
 * Full-screen capot celebration. Reads viewer-relative: the viewer's own team
 * always shows as Gold ("Us"), the opponents as Silver ("Them") — matching the
 * rest of the in-game table. Auto-advances to the score reveal after a short
 * dwell; tapping anywhere skips early.
 */
export function CapotAnimation({
  capotTeam,
  viewerSeat,
  capotBonus,
  onComplete,
}: CapotAnimationProps) {
  const { t } = useTranslation();
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
  // Grammar is viewer-relative: when the viewer's own team swept ("gold") the
  // copy reads first-person ("we swept" / "to us"); the opponents read
  // third-person. A single {{team}} label can't carry verb agreement or noun
  // case across languages, so each perspective gets its own string.
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
          className="font-display"
          style={{
            fontSize: 12,
            letterSpacing: 4,
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
          className={`font-display text-8xl font-bold tracking-tighter sm:text-9xl ${
            prefersReducedMotion ? "" : "motion-safe:animate-capot-shimmer"
          }`}
          style={
            {
              color: bright,
              margin: 0,
              lineHeight: 0.95,
              textShadow: `0 0 30px ${bright}80, 0 4px 18px rgba(0,0,0,0.6)`,
              "--cap-glow": `${bright}80`,
            } as CSSProperties
          }
        >
          {t("match.capot.title")}
        </h1>

        <div
          data-testid="capot-bonus"
          className="font-display"
          style={{
            marginTop: 22,
            display: "inline-flex",
            alignItems: "center",
            gap: 10,
            padding: "8px 16px",
            borderRadius: 999,
            background: `${bright}1f`,
            border: `1px solid ${bright}88`,
            fontSize: 13,
            letterSpacing: 2.4,
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
