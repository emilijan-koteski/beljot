import { type CSSProperties, useEffect, useRef } from "react";

import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { MOTION, motionDuration } from "@/shared/lib/motion";
import type { EmoteID } from "@/shared/types/wsEvents";

const EMOTE_GLYPHS: Record<EmoteID, string> = {
  thumbs_up: "👍",
  clap: "👏",
  laugh: "😂",
  thinking: "🤔",
  facepalm: "🤦",
  heart: "❤️",
};

// Desktop: bubble sits on the table-facing side of each seat (between the seat
// and the table center) so the emote reads as coming from that player without
// overlapping their name pill / card stack. Viewport-anchored offsets tuned to
// the desktop seat positions (see MatchPage SEAT_POSITIONS).
const SEAT_POSITIONS: Record<0 | 1 | 2 | 3, string> = {
  0: "bottom-[22rem] left-1/2 -translate-x-1/2", // South — above the self avatar/name pill
  1: "right-[22rem] top-1/2 -translate-y-1/2", // East — left of the seat (toward table)
  2: "top-[16rem] left-1/2 -translate-x-1/2", // North — below the partner's seat + card stack
  3: "left-[22rem] top-1/2 -translate-y-1/2", // West — right of the seat (toward table)
};

// Phone: the bubble is rendered INSIDE the seat wrapper (see MatchPage) and
// anchored to that seat's avatar — the avatar is the first/top child of the
// compact vertical seat, horizontally centered — so the emote tracks the
// avatar wherever the responsive layout parks it. It sits one seat-gap (the
// avatar↔name-pill gap) away on the table-facing side.
const AVATAR_FRAME_COMPACT = 60; // compact avatar: 44px disc + 16px frame padding
const EMOTE_GAP = 4; // matches the seat's `gap-1` between avatar and name pill
const COMPACT_POSITIONS: Record<0 | 1 | 2 | 3, CSSProperties> = {
  // South / self — just above the avatar.
  0: { left: "50%", bottom: "100%", transform: "translateX(-50%)", marginBottom: EMOTE_GAP },
  // East / right opponent — just left of the avatar.
  1: {
    left: "50%",
    top: AVATAR_FRAME_COMPACT / 2,
    transform: `translate(calc(-100% - ${AVATAR_FRAME_COMPACT / 2 + EMOTE_GAP}px), -50%)`,
  },
  // North / partner — below the whole seat cluster (name pill + card stack).
  2: { left: "50%", top: "100%", transform: "translateX(-50%)", marginTop: EMOTE_GAP },
  // West / left opponent — just right of the avatar.
  3: {
    left: "50%",
    top: AVATAR_FRAME_COMPACT / 2,
    transform: `translate(calc(${AVATAR_FRAME_COMPACT / 2 + EMOTE_GAP}px), -50%)`,
  },
};

interface EmoteBubbleProps {
  emote: EmoteID;
  compassPosition: 0 | 1 | 2 | 3;
  onDismiss: () => void;
  /**
   * Phone layout: anchor the bubble to the seat's avatar (table-facing side,
   * one seat-gap away) instead of the desktop viewport-anchored offsets. The
   * parent must render this inside the positioned seat wrapper.
   */
  compact?: boolean;
}

export function EmoteBubble({
  emote,
  compassPosition,
  onDismiss,
  compact = false,
}: EmoteBubbleProps) {
  // Capture latest onDismiss in a ref so the dismiss-timer effect runs once
  // on mount instead of restarting every parent re-render. Parents commonly
  // pass an inline arrow (e.g. `() => setActiveEmote(seat, null)`) whose
  // identity changes on every render — depending on it would clear and
  // reschedule the timer continuously, so the bubble never auto-dismisses
  // during active gameplay.
  const onDismissRef = useRef(onDismiss);
  useEffect(() => {
    onDismissRef.current = onDismiss;
  }, [onDismiss]);

  const reducedMotion = useReducedMotion();

  useEffect(() => {
    const duration = motionDuration(
      reducedMotion,
      MOTION.EMOTE_BUBBLE,
      MOTION.EMOTE_BUBBLE_REDUCED,
    );
    const handle = window.setTimeout(() => onDismissRef.current(), duration);
    return () => window.clearTimeout(handle);
  }, [reducedMotion]);

  return (
    <div
      role="status"
      aria-live="polite"
      data-testid={`emote-bubble-${compassPosition}`}
      className={`absolute pointer-events-none z-20 motion-safe:animate-in motion-safe:zoom-in-95 motion-safe:duration-150 ${
        compact ? "" : SEAT_POSITIONS[compassPosition]
      }`}
      style={compact ? COMPACT_POSITIONS[compassPosition] : undefined}
    >
      <span
        className={`inline-flex items-center justify-center rounded-full leading-none ${
          compact ? "h-10 w-10 text-2xl" : "h-12 w-12 text-3xl"
        }`}
        style={{
          background: "var(--panel-dark, rgba(20,45,30,0.85))",
          border: "1px solid var(--brass, #c9a876)",
          boxShadow: "0 4px 14px rgba(0,0,0,0.55), inset 0 1px 0 rgba(255,255,255,0.06)",
        }}
        aria-hidden="true"
      >
        {EMOTE_GLYPHS[emote]}
      </span>
    </div>
  );
}
