import type { CardDeck, CardId } from "@/shared/types/matchTypes";

/**
 * Re-exported so the render code that lives beside this module can keep
 * importing its deck type from one place. The type itself is declared in
 * `shared/types/matchTypes` — it is a wire value on the auth envelope, and
 * `shared/` must not import from `features/` to describe its own fields.
 */
export type { CardDeck };

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
 * One file extension per deck. The French set is vendored builder SVG; the
 * Croatian set is owner-authored raster imported to WebP by
 * `scripts/import-croatian-deck.py`. Keyed by deck rather than baked into the
 * template so a future deck brings its own format without touching callers.
 */
const DECK_EXT: Record<CardDeck, string> = { french: "svg", croatian: "webp" };

/** The deck every account gets unless it chose otherwise (and the DB default). */
export const DEFAULT_CARD_DECK: CardDeck = "french";

/**
 * Narrow an unvalidated deck value to a `CardDeck`, falling back to the French
 * default.
 *
 * Needed because both inputs are genuinely uncertain: the auth store's `user` is
 * null before hydration and on the unauthenticated paths, and HTTP responses are
 * *cast* rather than parsed (see `apiTypes.ts`), so a server that ships a third
 * deck before the client knows it would otherwise resolve to a missing asset —
 * which degrades to a blank parchment card, not an error anyone would notice.
 */
export function resolveCardDeck(value: string | null | undefined): CardDeck {
  // Object.hasOwn, NOT `in`: `in` walks the prototype chain, so `"toString" in
  // DECK_EXT` is true and a hostile or garbled value would be accepted as a deck
  // — producing `cards/toString/KS.function toString() { [native code] }` as an
  // asset URL. Own-property only.
  return value !== null && value !== undefined && Object.hasOwn(DECK_EXT, value)
    ? (value as CardDeck)
    : DEFAULT_CARD_DECK;
}

/**
 * Face artwork lives in `public/cards/{deck}/`, one file per card, named by the
 * canonical two-char card ID — so the URL is derived from deck + ID, never
 * looked up per card. See `docs/card-deck.md` for both decks' provenance and how
 * to regenerate them.
 *
 * Takes the deck as an argument rather than reading the auth store so it stays
 * pure and unit-testable; `PlayingCard` is the single component that resolves
 * the active deck, and it is already the only caller.
 *
 * Built off `BASE_URL` rather than a bare `/` so the deck still resolves if the
 * app is ever served from a sub-path.
 */
export function cardFaceUrl(cardId: CardId, deck: CardDeck): string {
  return `${import.meta.env.BASE_URL}cards/${deck}/${cardId}.${DECK_EXT[deck]}`;
}
