import { useTranslation } from "react-i18next";

import { MOTION } from "@/shared/lib/motion";
import { Z } from "@/shared/lib/zLayers";
import type { Card } from "@/shared/types/matchTypes";

import { floatingPanelStyle } from "../lib/floatingPanel";
import { PlayingCard } from "./PlayingCard";

interface DealAnimationProps {
  flippedCandidate: Card | null;
  isReshuffle: boolean;
  prefersReducedMotion: boolean;
}

/** Table-centre layer of a running deal: the flipped trump candidate and the reshuffle caption. */
export function DealAnimation({
  flippedCandidate,
  isReshuffle,
  prefersReducedMotion,
}: DealAnimationProps) {
  const { t } = useTranslation();
  return (
    <div
      className="absolute inset-0 flex flex-col items-center justify-center gap-3 pointer-events-none"
      style={{ zIndex: Z.TABLE_ANIM }}
      data-testid="deal-animation"
    >
      <span className="sr-only" role="status">
        {isReshuffle ? t("match.reshuffle.message") : t("match.deal.dealing")}
      </span>
      {flippedCandidate && (
        <div
          data-testid="deal-candidate"
          style={
            prefersReducedMotion
              ? undefined
              : {
                  animation: `deal-candidate-flip ${MOTION.DEAL_CANDIDATE_FLIP}ms ease-out both`,
                }
          }
        >
          <PlayingCard card={flippedCandidate} state="default" size="lg" withTransition={false} />
        </div>
      )}
      {isReshuffle && (
        <p
          aria-hidden="true"
          className="text-sm rounded-lg px-3 py-1.5"
          style={{
            color: "var(--ink-light)",
            ...floatingPanelStyle(),
          }}
          data-testid="deal-reshuffle-caption"
        >
          {t("match.reshuffle.message")}
        </p>
      )}
    </div>
  );
}
