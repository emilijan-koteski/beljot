import type { CardDeck, CardId } from "@/shared/types/matchTypes";

export type { CardDeck };

export type CardSize = "sm" | "md" | "lg";

interface CardDimensions {
  width: number;
  height: number;
  radius: number;
}

const ART_ASPECT = 336 / 240;
const ART_RADIUS_RATIO = 12 / 240;

/**
 * Build a card box from its width at the artwork's exact aspect and corner ratio.
 * The faces stretch to fill any box, so any other ratio would distort them.
 */
export function cardBox(width: number): CardDimensions {
  return { width, height: width * ART_ASPECT, radius: width * ART_RADIUS_RATIO };
}

const MIN_RANK_INDEX_READABLE_WIDTH = 60;

export const CARD_SIZES: Record<CardSize, CardDimensions> = {
  sm: cardBox(MIN_RANK_INDEX_READABLE_WIDTH),
  md: cardBox(80),
  lg: cardBox(90),
};

export const SEAT_STACK_CARD_WIDTH = { desktop: 30, compact: 20 } as const;

export const CARD_FACE_BACKGROUND =
  "linear-gradient(180deg, var(--card-face) 0%, var(--card-face-bottom) 100%)";

export const CARD_FACE_BORDER = "1px solid var(--shade-15)";

export const CARD_BACK_BACKGROUND =
  "linear-gradient(135deg, #2a1a10 0%, #4a2818 50%, #2a1a10 100%)";

const DECK_EXT: Record<CardDeck, string> = { french: "svg", croatian: "webp" };

const DEFAULT_CARD_DECK: CardDeck = "french";

/**
 * Narrow an unvalidated deck value to a `CardDeck`, falling back to the French default.
 * Uses an own-property check so prototype keys like `toString` are never accepted as decks.
 */
export function resolveCardDeck(value: string | null | undefined): CardDeck {
  return value !== null && value !== undefined && Object.hasOwn(DECK_EXT, value)
    ? (value as CardDeck)
    : DEFAULT_CARD_DECK;
}

/**
 * Build the face artwork URL for a card in a deck, `public/cards/{deck}/{cardId}.{ext}` under `BASE_URL`.
 */
export function cardFaceUrl(cardId: CardId, deck: CardDeck): string {
  return `${import.meta.env.BASE_URL}cards/${deck}/${cardId}.${DECK_EXT[deck]}`;
}
