import { type CSSProperties, useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";

import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { MOTION, motionDuration } from "@/shared/lib/motion";
import { Z } from "@/shared/lib/zLayers";

interface DeclareBannerProps {
  /** Player who committed the declare action. */
  declarerUsername: string;
  /**
   * Compass position of the declarer relative to the local viewer (0=south,
   * 1=east, 2=north, 3=west). Same per-seat anchoring as {@link EmoteBubble} /
   * {@link SurrenderOpponentBanner}.
   */
  compassPosition: 0 | 1 | 2 | 3;
  /** Fired by the auto-dismiss timer so the parent clears the store slot. */
  onDismiss: () => void;
  /**
   * Phone layout: anchor the banner to the declarer's avatar (table-facing
   * side, one seat-gap away) — the parent must render this inside the
   * positioned seat wrapper. Desktop uses the viewport-anchored offsets.
   */
  compact?: boolean;
}

// Desktop: viewport-anchored seat-relative positions — identical to
// EmoteBubble / SurrenderOpponentBanner so all seat banners share one anchor.
const SEAT_POSITIONS: Record<0 | 1 | 2 | 3, string> = {
  0: "bottom-[22rem] left-1/2 -translate-x-1/2",
  1: "right-[22rem] top-1/2 -translate-y-1/2",
  2: "top-[16rem] left-1/2 -translate-x-1/2",
  3: "left-[22rem] top-1/2 -translate-y-1/2",
};

// Phone: anchored to the declarer's avatar on the table-facing side, one
// seat-gap away — same geometry as the other compact seat banners.
const AVATAR_FRAME_COMPACT = 60; // compact avatar: 44px disc + 16px frame padding
const GAP = 4; // matches the seat's `gap-1` between avatar and name pill
const COMPACT_POSITIONS: Record<0 | 1 | 2 | 3, CSSProperties> = {
  // South / self — above the avatar.
  0: { left: "50%", bottom: "100%", transform: "translateX(-50%)", marginBottom: GAP },
  // East / right opponent — left of the avatar, extending inward.
  1: {
    left: "50%",
    top: AVATAR_FRAME_COMPACT / 2,
    transform: `translate(calc(-100% - ${AVATAR_FRAME_COMPACT / 2 + GAP}px), -50%)`,
  },
  // North / partner — below the avatar.
  2: { left: "50%", top: "100%", transform: "translateX(-50%)", marginTop: GAP },
  // West / left opponent — right of the avatar, extending inward.
  3: {
    left: "50%",
    top: AVATAR_FRAME_COMPACT / 2,
    transform: `translate(calc(${AVATAR_FRAME_COMPACT / 2 + GAP}px), -50%)`,
  },
};

/**
 * Trick-1 "has a declaration" seat banner. Shown the moment a player commits
 * a declare action so the whole table knows a declaration exists — WITHOUT
 * revealing which melds or how much; that stays secret until the
 * DeclarationReveal at the end of trick 1. Styling and anchoring mirror
 * EmoteBubble / SurrenderOpponentBanner; auto-dismisses like the emote bubble.
 */
export function DeclareBanner({
  declarerUsername,
  compassPosition,
  onDismiss,
  compact = false,
}: DeclareBannerProps) {
  const { t } = useTranslation();

  // Capture latest onDismiss in a ref so the dismiss-timer effect runs once
  // on mount instead of restarting whenever the parent re-creates the inline
  // callback (same pattern as EmoteBubble).
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
        border: "1px solid var(--brass, #c9a876)",
        boxShadow: "0 4px 14px rgba(0,0,0,0.55), inset 0 1px 0 rgba(255,255,255,0.06)",
        ...(compact ? { ...COMPACT_POSITIONS[compassPosition], maxWidth: 150 } : null),
      }}
      data-testid="declare-banner"
      role="status"
      aria-live="polite"
    >
      <span
        className={`font-body ${compact ? "block text-center text-xs leading-snug" : "text-sm whitespace-nowrap"}`}
        style={{ color: "var(--ink-light, #f5f2e8)" }}
      >
        {t("match.declaration.declaredBanner", { username: declarerUsername })}
      </span>
    </div>
  );
}
