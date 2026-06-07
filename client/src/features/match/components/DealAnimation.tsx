import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { MOTION } from "@/shared/lib/motion";
import { Z } from "@/shared/lib/zLayers";
import type { Card } from "@/shared/types/matchTypes";

import { PlayingCard } from "./PlayingCard";

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

  useEffect(() => {
    if (prefersReducedMotion) {
      setDealPhase("done");
      return;
    }

    // Phase 1: dealing cards (3+2 sequence)
    const dealTimer = setTimeout(() => {
      setDealPhase("revealing");
    }, MOTION.DEAL_PHASE_DEAL);

    // Phase 2: reveal trump candidate — at total = phase1 + phase2-extension
    const revealTimer = setTimeout(() => {
      setDealPhase("done");
    }, MOTION.DEAL_PHASE_TRUMP);

    return () => {
      clearTimeout(dealTimer);
      clearTimeout(revealTimer);
    };
  }, [prefersReducedMotion]);

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
              <div className="w-8 h-12 rounded bg-surface-elevated border border-border" />
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
