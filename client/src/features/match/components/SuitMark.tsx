import type { CSSProperties } from "react";

import type { Suit } from "@/shared/types/matchTypes";

import type { CardDeck } from "../lib/cardFace";
import { suitAccent, suitGlyph, suitIconUrl } from "../lib/suitArt";

interface SuitMarkProps {
  suit: Suit;
  deck: CardDeck;
  /**
   * Rendered size in px. For a glyph this is `fontSize`; for an icon it is both
   * width and height, so the two decks occupy the same box at the same number.
   */
  size: number;
  /**
   * Extra classes, applied on BOTH branches. `TrumpIndicator`'s are a legacy
   * assertion hook that has never carried colour (its inline `color` always
   * won), but silently dropping a caller's classes on one branch is how a
   * surface ends up styled differently per deck.
   */
  className?: string;
  /** Merged over the mark's own styles, for callers that need to nudge layout. */
  style?: CSSProperties;
  /**
   * A CSS shadow offset/blur/colour triple, e.g. `"0 1px 1px rgba(0,0,0,0.25)"`.
   *
   * Declared as its own prop rather than left to `style` because the two
   * branches need DIFFERENT properties for the same visual effect: a glyph takes
   * `text-shadow`, an image takes `filter: drop-shadow(...)`. Passing
   * `style={{ textShadow }}` worked on French and silently did nothing on
   * Croatian, so the wax seal lost the drop shadow the French glyph had.
   */
  shadow?: string;
}

/**
 * One suit mark, in the player's active deck.
 *
 * **Always decorative.** Every one of the four call sites already names the suit
 * on an ancestor — the indicator chip's `aria-label`, the prompt button's
 * `aria-label`, the reveal seal's `aria-label`, the seat chip's `title` — so the
 * mark itself is `aria-hidden` and the icon carries `alt=""`. This mirrors
 * `PlayingCard`, where the face image is decorative and the wrapper's label is
 * the one announcement.
 *
 * The colour comes from {@link SUIT_ACCENT} for glyphs. Icons carry their own
 * ink, so no colour is applied to them — tinting owner-authored art would be
 * the same mistake as painting a red halo round a gold bell.
 *
 * A missing icon is hidden rather than left broken, exactly as `PlayingCard`
 * handles a missing face: production's SPA fallback answers a bad asset path
 * with `index.html` at 200, which browsers paint as a broken-image glyph. Hiding
 * it leaves the surrounding chrome — the orb, the tile, the seal — intact.
 */
export function SuitMark({ suit, deck, size, className, style, shadow }: SuitMarkProps) {
  const iconUrl = suitIconUrl(suit, deck);

  if (iconUrl !== null) {
    return (
      <img
        // Keyed by src for the same reason as PlayingCard's face: React reuses
        // this node across deck changes, and the imperative hide in `onError`
        // would otherwise outlive the failure that set it.
        key={iconUrl}
        src={iconUrl}
        alt=""
        aria-hidden="true"
        draggable={false}
        data-testid={`suit-mark-${suit}`}
        data-deck={deck}
        // `object-contain` so a non-square icon is letterboxed inside the box
        // rather than distorted. The shipped set is square, so this is inert
        // today and load-bearing only if a future set is not.
        className={`pointer-events-none inline-block object-contain${className ? ` ${className}` : ""}`}
        style={{
          width: size,
          height: size,
          // The image twin of the glyph branch's text-shadow.
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
