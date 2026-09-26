import { useTranslation } from "react-i18next";

import { MOTION } from "@/shared/lib/motion";
import { Z } from "@/shared/lib/zLayers";
import type { Card } from "@/shared/types/matchTypes";

import { PlayingCard } from "./PlayingCard";

interface DealAnimationProps {
  /** The face-up candidate, once the deal has turned it over; null before. */
  flippedCandidate: Card | null;
  /** The deal follows an all-pass: say so, or it reads as a random re-deal. */
  isReshuffle: boolean;
  prefersReducedMotion: boolean;
}

/**
 * Table-centre layer of a deal in progress. The packets themselves fly in the
 * page's CardFlight overlay and the seats fill as they land (see
 * `useDealController`); this only carries what belongs to the middle of the
 * table: the trump candidate turning face-up at the end of a candidate deal,
 * and the "reshuffling" caption after everyone passed. It is mounted only
 * while a deal runs, and announces it to assistive tech.
 */
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
          className="font-body text-sm rounded-md px-3 py-1.5"
          style={{
            color: "var(--ink-light, #f5f2e8)",
            background: "var(--panel-dark, rgba(20,45,30,0.85))",
            border: "1px solid rgba(201,168,118,0.4)",
          }}
          data-testid="deal-reshuffle-caption"
        >
          {t("match.reshuffle.message")}
        </p>
      )}
    </div>
  );
}
