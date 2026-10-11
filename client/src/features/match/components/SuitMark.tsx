import type { CSSProperties } from "react";

import type { Suit } from "@/shared/types/matchTypes";

import type { CardDeck } from "../lib/cardFace";
import { suitAccent, suitGlyph, suitIconUrl } from "../lib/suitArt";

interface SuitMarkProps {
  suit: Suit;
  deck: CardDeck;
  size: number;
  className?: string;
  style?: CSSProperties;
  shadow?: string;
}

/**
 * Decorative suit mark in the player's active deck; callers name the suit on an ancestor.
 * Glyphs take their colour from {@link SUIT_ACCENT}, and a missing icon is hidden rather than shown broken.
 */
export function SuitMark({ suit, deck, size, className, style, shadow }: SuitMarkProps) {
  const iconUrl = suitIconUrl(suit, deck);

  if (iconUrl !== null) {
    return (
      <img
        key={iconUrl}
        src={iconUrl}
        alt=""
        aria-hidden="true"
        draggable={false}
        data-testid={`suit-mark-${suit}`}
        data-deck={deck}
        className={`pointer-events-none inline-block object-contain${className ? ` ${className}` : ""}`}
        style={{
          width: size,
          height: size,
          ...(shadow !== undefined ? { filter: `drop-shadow(${shadow})` } : {}),
          ...style,
        }}
        onError={(e) => {
          e.currentTarget.style.visibility = "hidden";
        }}
        onLoad={(e) => {
          e.currentTarget.style.visibility = "";
        }}
      />
    );
  }

  return (
    <span
      aria-hidden="true"
      data-testid={`suit-mark-${suit}`}
      data-deck={deck}
      className={className}
      style={{
        color: suitAccent(suit, deck),
        fontSize: size,
        lineHeight: 1,
        ...(shadow !== undefined ? { textShadow: shadow } : {}),
        ...style,
      }}
    >
      {suitGlyph(suit)}
    </span>
  );
}
