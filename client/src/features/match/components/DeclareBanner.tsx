import { type CSSProperties, useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";

import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { MOTION, motionDuration } from "@/shared/lib/motion";
import { Z } from "@/shared/lib/zLayers";

interface DeclareBannerProps {
  declarerUsername: string;
  compassPosition: 0 | 1 | 2 | 3;
  onDismiss: () => void;
  compact?: boolean;
}

const SEAT_POSITIONS: Record<0 | 1 | 2 | 3, string> = {
  0: "bottom-[22rem] left-1/2 -translate-x-1/2",
  1: "right-[22rem] top-1/2 -translate-y-1/2",
  2: "top-[16rem] left-1/2 -translate-x-1/2",
  3: "left-[22rem] top-1/2 -translate-y-1/2",
};

const AVATAR_FRAME_COMPACT = 60;
const GAP = 4;
const COMPACT_POSITIONS: Record<0 | 1 | 2 | 3, CSSProperties> = {
  0: { left: "50%", bottom: "100%", transform: "translateX(-50%)", marginBottom: GAP },
  1: {
    left: "50%",
    top: AVATAR_FRAME_COMPACT / 2,
    transform: `translate(calc(-100% - ${AVATAR_FRAME_COMPACT / 2 + GAP}px), -50%)`,
  },
  2: { left: "50%", top: "100%", transform: "translateX(-50%)", marginTop: GAP },
  3: {
    left: "50%",
    top: AVATAR_FRAME_COMPACT / 2,
    transform: `translate(calc(${AVATAR_FRAME_COMPACT / 2 + GAP}px), -50%)`,
  },
};

/**
 * Seat banner announcing that a player has a declaration, without revealing which melds or how much.
 * It auto-dismisses like the emote bubble.
 */
export function DeclareBanner({
  declarerUsername,
  compassPosition,
  onDismiss,
  compact = false,
}: DeclareBannerProps) {
  const { t } = useTranslation();

  const onDismissRef = useRef(onDismiss);
  useEffect(() => {
    onDismissRef.current = onDismiss;
  }, [onDismiss]);

  const reducedMotion = useReducedMotion();

  useEffect(() => {
    const duration = motionDuration(
      reducedMotion,
      MOTION.DECLARE_BANNER,
      MOTION.DECLARE_BANNER_REDUCED,
    );
    const handle = window.setTimeout(() => onDismissRef.current(), duration);
    return () => window.clearTimeout(handle);
  }, [reducedMotion]);

  return (
    <div
      className={`absolute pointer-events-none motion-safe:animate-in motion-safe:zoom-in-95 motion-safe:duration-150 ${
        compact
          ? "rounded-2xl px-3 py-1"
          : `${SEAT_POSITIONS[compassPosition]} rounded-full px-4 py-1.5`
      }`}
      style={{
        zIndex: Z.SEAT_BANNER,
        background: "var(--panel-dark, rgba(20,45,30,0.85))",
        border: "1px solid var(--brass)",
        boxShadow: "var(--shadow-felt-md), inset 0 1px 0 rgba(255,255,255,0.06)",
        ...(compact ? { ...COMPACT_POSITIONS[compassPosition], maxWidth: 150 } : null),
      }}
      data-testid="declare-banner"
      role="status"
      aria-live="polite"
    >
      <span
        className={`${compact ? "block text-center text-xs leading-snug" : "text-sm whitespace-nowrap"}`}
        style={{ color: "var(--ink-light)" }}
      >
        {t("match.declaration.declaredBanner", { username: declarerUsername })}
      </span>
    </div>
  );
}
