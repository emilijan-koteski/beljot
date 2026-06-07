import { type CSSProperties } from "react";
import { useTranslation } from "react-i18next";

import { Z } from "@/shared/lib/zLayers";

interface SurrenderOpponentBannerProps {
  proposerUsername: string;
  /**
   * Compass position of the proposer relative to the local viewer (0=south,
   * 1=east, 2=north, 3=west). Drives the banner's anchor — same per-seat
   * pattern as {@link EmoteBubble} so opponents see WHO is proposing. In
   * practice opponents only ever see compass 1 or 3 (the proposer is on the
   * opposing team, never the viewer or their partner), but all four slots
   * are typed for completeness.
   */
  compassPosition: 0 | 1 | 2 | 3;
  /**
   * Phone layout: anchor the banner to the proposer's avatar (table-facing
   * side, one seat-gap away) — mirrors {@link EmoteBubble} compact mode — with
   * a capped, wrapping width so the wide text doesn't run off the edge. The
   * parent must render this inside the positioned seat wrapper.
   */
  compact?: boolean;
}

// Desktop: viewport-anchored seat-relative positions so the banner reads as
// originating from the proposer's seat, not as a generic top-of-screen toast.
const SEAT_POSITIONS: Record<0 | 1 | 2 | 3, string> = {
  0: "bottom-[22rem] left-1/2 -translate-x-1/2",
  1: "right-[22rem] top-1/2 -translate-y-1/2",
  2: "top-[16rem] left-1/2 -translate-x-1/2",
  3: "left-[22rem] top-1/2 -translate-y-1/2",
};

// Phone: anchored to the proposer's avatar (the first/top, horizontally
// centered child of the compact vertical seat) on the table-facing side, one
// seat-gap away — same geometry as EmoteBubble compact mode. The banner
// extends inward from the edge seats (compass 1/3, the only cases that occur).
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

// Slim non-modal banner shown to opposing-team players while a surrender
// proposal is pending. Intentionally NOT a dialog: opponents must keep
// playing while the proposer's partner accepts/declines. Styling and
// anchoring mirror EmoteBubble — same dark-felt panel + brass border, parked
// next to the proposer's seat — so the in-game chrome stays consistent.
export function SurrenderOpponentBanner({
  proposerUsername,
  compassPosition,
  compact = false,
}: SurrenderOpponentBannerProps) {
  const { t } = useTranslation();

  return (
    <div
      className={`absolute motion-safe:animate-in motion-safe:zoom-in-95 motion-safe:duration-150 ${
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
      data-testid="surrender-opponent-banner"
      role="status"
      aria-live="polite"
    >
      <span
        className={`font-body ${compact ? "block text-center text-xs leading-snug" : "text-sm whitespace-nowrap"}`}
        style={{ color: "var(--ink-light, #f5f2e8)" }}
      >
        {t("match.surrender.opponentBanner", { username: proposerUsername })}
      </span>
    </div>
  );
}
