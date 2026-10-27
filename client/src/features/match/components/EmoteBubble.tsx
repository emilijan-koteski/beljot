import { type CSSProperties, useEffect, useRef } from "react";

import { useReducedMotion } from "@/shared/hooks/useReducedMotion";
import { MOTION, motionDuration } from "@/shared/lib/motion";
import { Z } from "@/shared/lib/zLayers";
import type { EmoteID } from "@/shared/types/wsEvents";

import { floatingPanelStyle } from "../lib/floatingPanel";

const EMOTE_GLYPHS: Record<EmoteID, string> = {
  thumbs_up: "👍",
  clap: "👏",
  laugh: "😂",
  thinking: "🤔",
  facepalm: "🤦",
  heart: "❤️",
};

const SEAT_POSITIONS: Record<0 | 1 | 2 | 3, string> = {
  0: "bottom-[22rem] left-1/2 -translate-x-1/2",
  1: "right-[22rem] top-1/2 -translate-y-1/2",
  2: "top-[16rem] left-1/2 -translate-x-1/2",
  3: "left-[22rem] top-1/2 -translate-y-1/2",
};

const AVATAR_FRAME_COMPACT = 60;
const EMOTE_GAP = 4;
const COMPACT_POSITIONS: Record<0 | 1 | 2 | 3, CSSProperties> = {
  0: { left: "50%", bottom: "100%", transform: "translateX(-50%)", marginBottom: EMOTE_GAP },
  1: {
    left: "50%",
    top: AVATAR_FRAME_COMPACT / 2,
    transform: `translate(calc(-100% - ${AVATAR_FRAME_COMPACT / 2 + EMOTE_GAP}px), -50%)`,
  },
  2: { left: "50%", top: "100%", transform: "translateX(-50%)", marginTop: EMOTE_GAP },
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
  compact?: boolean;
}

export function EmoteBubble({
  emote,
  compassPosition,
  onDismiss,
  compact = false,
}: EmoteBubbleProps) {
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
      className={`absolute pointer-events-none motion-safe:animate-in motion-safe:zoom-in-95 motion-safe:duration-150 ${
        compact ? "" : SEAT_POSITIONS[compassPosition]
      }`}
      style={{ zIndex: Z.SEAT_BANNER, ...(compact ? COMPACT_POSITIONS[compassPosition] : {}) }}
    >
      <span
        className={`inline-flex items-center justify-center rounded-full leading-none ${
          compact ? "h-10 w-10 text-2xl" : "h-12 w-12 text-3xl"
        }`}
        style={{
          ...floatingPanelStyle({ edge: "announce" }),
        }}
        aria-hidden="true"
      >
        {EMOTE_GLYPHS[emote]}
      </span>
    </div>
  );
}
