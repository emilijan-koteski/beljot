import { type CSSProperties } from "react";
import { useTranslation } from "react-i18next";

import { Z } from "@/shared/lib/zLayers";

interface SurrenderOpponentBannerProps {
  proposerUsername: string;
  compassPosition: 0 | 1 | 2 | 3;
  compact?: boolean;
}

const SEAT_POSITIONS: Record<0 | 1 | 2 | 3, string> = {
  0: "bottom-[22rem] left-1/2 -translate-x-1/2",
  1: "right-[22rem] top-1/2 -translate-y-1/2",
  2: "top-[16rem] left-1/2 -translate-x-1/2",
  3: "left-[22rem] top-1/2 -translate-y-1/2",
};

const AVATAR_FRAME_COMPACT = 60;
const SEAT_AVATAR_NAME_GAP = 4;
const COMPACT_POSITIONS: Record<0 | 1 | 2 | 3, CSSProperties> = {
  0: {
    left: "50%",
    bottom: "100%",
    transform: "translateX(-50%)",
    marginBottom: SEAT_AVATAR_NAME_GAP,
  },
  1: {
    left: "50%",
    top: AVATAR_FRAME_COMPACT / 2,
    transform: `translate(calc(-100% - ${AVATAR_FRAME_COMPACT / 2 + SEAT_AVATAR_NAME_GAP}px), -50%)`,
  },
  2: { left: "50%", top: "100%", transform: "translateX(-50%)", marginTop: SEAT_AVATAR_NAME_GAP },
  3: {
    left: "50%",
    top: AVATAR_FRAME_COMPACT / 2,
    transform: `translate(calc(${AVATAR_FRAME_COMPACT / 2 + SEAT_AVATAR_NAME_GAP}px), -50%)`,
  },
};

/**
 * Non-modal banner telling opponents a surrender proposal is pending, parked next to the proposer's seat.
 * It is deliberately not a dialog so opponents can keep playing.
 */
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
        border: "1px solid var(--brass)",
        boxShadow: "var(--shadow-felt-md), inset 0 1px 0 rgba(255,255,255,0.06)",
        ...(compact ? { ...COMPACT_POSITIONS[compassPosition], maxWidth: 150 } : null),
      }}
      data-testid="surrender-opponent-banner"
      role="status"
      aria-live="polite"
    >
      <span
        className={`${compact ? "block text-center text-xs leading-snug" : "text-sm whitespace-nowrap"}`}
        style={{ color: "var(--ink-light)" }}
      >
        {t("match.surrender.opponentBanner", { username: proposerUsername })}
      </span>
    </div>
  );
}
