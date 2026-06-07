import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { MOTION } from "@/shared/lib/motion";
import { Z } from "@/shared/lib/zLayers";

interface ReshuffleAnimationProps {
  onComplete: () => void;
}

export function ReshuffleAnimation({ onComplete }: ReshuffleAnimationProps) {
  const { t } = useTranslation();
  const [phase, setPhase] = useState<"shuffle" | "done">("shuffle");

  const prefersReducedMotion = useReducedMotion();

  useEffect(() => {
    if (prefersReducedMotion) {
      setPhase("done");
      onComplete();
      return;
    }

    const timer = setTimeout(() => {
      setPhase("done");
      onComplete();
    }, MOTION.RESHUFFLE_PULSE);

    return () => clearTimeout(timer);
  }, [prefersReducedMotion, onComplete]);

  if (phase === "done") return null;

  return (
    <div
      className="absolute inset-0 flex items-center justify-center pointer-events-none"
      style={{ zIndex: Z.TABLE_ANIM }}
      data-testid="reshuffle-animation"
    >
      <div className="text-center">
        <div className="flex gap-1 justify-center mb-3">
          {[0, 1, 2, 3].map((i) => (
            <div
              key={i}
              className="w-8 h-12 rounded bg-surface-elevated border border-border motion-safe:animate-pulse"
              style={{ animationDelay: `${i * MOTION.RESHUFFLE_STAGGER}ms` }}
            />
          ))}
        </div>
        <p className="text-text-secondary font-body text-sm">{t("match.reshuffle.message")}</p>
      </div>
    </div>
  );
}
