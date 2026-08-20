import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { MOTION } from "@/shared/lib/motion";
import { Z } from "@/shared/lib/zLayers";
import type { Card } from "@/shared/types/matchTypes";

import { cardBox } from "../lib/cardFace";
import { PlayingCard } from "./PlayingCard";

/** Small card-shaped proxy flown to each seat during the deal. */
const DEAL_PROXY = cardBox(32);

interface DealAnimationProps {
  trumpCandidate: Card | null;
}

const COMPASS_LABELS = ["south", "east", "north", "west"] as const;

// Positions for card deal targets (relative to center)
const DEAL_TARGETS: Record<string, string> = {
  south: "translate-y-[120px]",
  east: "translate-x-[120px]",
  north: "-translate-y-[120px]",
  west: "-translate-x-[120px]",
};

export function DealAnimation({ trumpCandidate }: DealAnimationProps) {
  const { t } = useTranslation();
  const [dealPhase, setDealPhase] = useState<"dealing" | "revealing" | "done">("dealing");

  const prefersReducedMotion = useReducedMotion();

  // The trump-flip beat only exists when there is a candidate to flip. Without
  // one (a variant that deals every card and names trump as a bare suit) the
  // reveal phase renders nothing, so spending DEAL_PHASE_TRUMP there is a dead
  // pause on an empty table centre before bidding opens. Ending at
  // DEAL_PHASE_DEAL removes it. Deliberately NOT new choreography for the
  // face-down pair — that is a separate design question.
  const hasCandidate = trumpCandidate !== null;

  useEffect(() => {
    if (prefersReducedMotion) {
      setDealPhase("done");
      return;
    }

    // Phase 1: dealing cards (3+2 sequence)
    const dealTimer = setTimeout(() => {
      setDealPhase(hasCandidate ? "revealing" : "done");
    }, MOTION.DEAL_PHASE_DEAL);

    if (!hasCandidate) {
      return () => clearTimeout(dealTimer);
    }

    // Phase 2: reveal trump candidate — at total = phase1 + phase2-extension
    const revealTimer = setTimeout(() => {
      setDealPhase("done");
    }, MOTION.DEAL_PHASE_TRUMP);

    return () => {
      clearTimeout(dealTimer);
      clearTimeout(revealTimer);
    };
  }, [prefersReducedMotion, hasCandidate]);

  // Skip rendering once animation is complete
  if (dealPhase === "done" && !trumpCandidate) return null;

  return (
    <div
      className="absolute inset-0 flex items-center justify-center pointer-events-none"
      style={{ zIndex: Z.TABLE_ANIM }}
      data-testid="deal-animation"
      aria-label={t("match.deal.dealing")}
    >
      {/* Deal card indicators flying to each seat */}
      {dealPhase === "dealing" && (
        <>
          {COMPASS_LABELS.map((dir, i) => (
            <div
              key={dir}
              className={`absolute motion-safe:transition-transform motion-safe:duration-300 motion-safe:ease-in ${DEAL_TARGETS[dir]}`}
              style={{ transitionDelay: `${i * 80}ms` }}
            >
              {/* Card-shaped proxy. Takes its geometry from the shared box so it
                  keeps the deck's proportions and corner — a proxy at a different
                  ratio visibly changes shape when the real card takes over. */}
              <div
                className="bg-surface-elevated border border-border"
                style={{
                  width: DEAL_PROXY.width,
                  height: DEAL_PROXY.height,
                  borderRadius: DEAL_PROXY.radius,
                }}
              />
            </div>
          ))}
        </>
      )}

      {/* Trump candidate reveal in center */}
      {(dealPhase === "revealing" || dealPhase === "done") && trumpCandidate && (
        <div className="motion-safe:animate-in motion-safe:zoom-in-95 motion-safe:duration-150">
          <PlayingCard card={trumpCandidate} state="default" size="lg" withTransition={false} />
        </div>
      )}
    </div>
  );
}
