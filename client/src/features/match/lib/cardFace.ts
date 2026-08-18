import type { CardId } from "@/shared/types/matchTypes";

export type CardSize = "sm" | "md" | "lg";

export interface CardDimensions {
  width: number;
  height: number;
  /**
   * Corner radius in px. The deck draws its face as `rx="12"` inside a 240-unit
   * viewBox, i.e. 5% of the card's width, so this is `width * 0.05`. Matching it
   * exactly is what keeps the card's rounded corner on top of the artwork's own
   * corner instead of cutting outside it and exposing a sliver of backing.
   */
  radius: number;
}

/** The artwork's viewBox — 240x336 with `rx="12"`. Both ratios derive from it. */
const ART_ASPECT = 336 / 240; // 1.4
const ART_RADIUS_RATIO = 12 / 240; // 0.05

/**
 * A card box is fully determined by its width: the faces are authored with
 * `preserveAspectRatio="none"`, so they stretch to whatever box they are given
 * and any ratio other than the artwork's own silently distorts them. Computed
 * rather than typed out so the two invariants cannot be broken by editing a
 * single number.
 */
export function cardBox(width: number): CardDimensions {
  return { width, height: width * ART_ASPECT, radius: width * ART_RADIUS_RATIO };
}

/**
 * The card box at each size. Widths are the design's; everything else follows.
 *
 * Single source of truth: `HandCards`, `TrickArea`, and `CardFlight` all position
 * cards with their own arithmetic and must read these rather than restate them.
 */
export const CARD_SIZES: Record<CardSize, CardDimensions> = {
  // `sm` is wider than a straight scale-down would suggest. The artwork's corner
  // rank index is 32 of 336 viewBox units, so it shrinks with the card: at 45px
  // wide it renders ~5.5px tall, and in an overlapped meld row the index is the
  // only visible cue to which cards scored. 56 keeps it readable.
  sm: cardBox(56),
  md: cardBox(75),
  lg: cardBox(90),
};

/**
 * The card face itself. The vendored artwork's own face rect is transparent (see
 * `scripts/recolor-cards.mjs`), so this gradient is what a card face actually
 * is — the ink is drawn straight onto it, at its exact token colour, with no
 * blend layer in between. Shared so every card-face surface is the same material.
 */
export const CARD_FACE_BACKGROUND = "linear-gradient(180deg, #fdfaf0 0%, #f4ecd8 100%)";

/**
 * Hairline edge for surfaces that *imitate* a card face but have no artwork of
 * their own — the trump suit tiles. Real cards deliberately do NOT use this: the
 * deck SVG strokes its own outline, so adding a CSS border on top draws the edge
 * twice and leaves a visible seam at the corners.
 */
export const CARD_FACE_BORDER = "1px solid rgba(0,0,0,0.15)";

/**
 * Vendored face artwork lives in `public/cards/`, one SVG per card, named by the
 * canonical two-char card ID — so the URL is derived, never looked up. See
 * `docs/card-deck.md` for provenance and how to regenerate the deck.
 *
 * Built off `BASE_URL` rather than a bare `/` so the deck still resolves if the
 * app is ever served from a sub-path.
 */
export function cardFaceUrl(cardId: CardId): string {
  return `${import.meta.env.BASE_URL}cards/${cardId}.svg`;
}
