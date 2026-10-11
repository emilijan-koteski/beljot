import type { Rank, Suit } from "@/shared/types/matchTypes";

import type { CardDeck } from "./cardFace";

export const SUIT_GLYPH: Record<Suit, string> = {
  S: "♠",
  H: "♥",
  D: "♦",
  C: "♣",
};

const ICON_DECKS: ReadonlySet<CardDeck> = new Set<CardDeck>(["croatian"]);

export const SUIT_ACCENT: Record<CardDeck, Record<Suit, string>> = {
  french: { S: "#1a1a1a", H: "#c62828", D: "#c62828", C: "#1a1a1a" },
  croatian: { S: "#4a7a3a", H: "#c62828", D: "#c9a23c", C: "#96501e" },
};

export const SUIT_GLOW_ALPHA: Record<CardDeck, Record<Suit, string>> = {
  french: { S: "55", H: "77", D: "77", C: "55" },
  croatian: { S: "77", H: "77", D: "77", C: "73" },
};

export const SUIT_INK_ON_FELT: Record<CardDeck, Record<Suit, string>> = {
  french: { S: "#f5f2e8", H: "#ff8585", D: "#ff8585", C: "#f5f2e8" },
  croatian: { S: "#8fd07a", H: "#ff8585", D: "#e8c766", C: "#e19967" },
};

/** URL of a deck's icon for one suit under `BASE_URL`, or `null` for a deck that draws Unicode glyphs. */
export function suitIconUrl(suit: Suit, deck: CardDeck): string | null {
  if (!ICON_DECKS.has(deck)) return null;
  return `${import.meta.env.BASE_URL}suits/${deck}/${suit}.webp`;
}

const FALLBACK_ACCENT = "#1a1a1a";
const FALLBACK_GLOW_ALPHA = "55";
const FALLBACK_INK_ON_FELT = "#f5f2e8";

/**
 * Read one suit's entry from a per-deck palette row, or the fallback for an unrecognised suit.
 * Uses an own-property check so prototype keys like `toString` never leak a function into a style.
 */
function paletteEntry(row: Record<Suit, string>, suit: Suit, fallback: string): string {
  return Object.hasOwn(row, suit) ? row[suit] : fallback;
}

/** Accent for one suit on one deck, degrading to a neutral colour for an unrecognised suit. */
export function suitAccent(suit: Suit, deck: CardDeck): string {
  return paletteEntry(SUIT_ACCENT[deck], suit, FALLBACK_ACCENT);
}

/** Halo strength for one suit on one deck. See {@link suitAccent}. */
export function suitGlowAlpha(suit: Suit, deck: CardDeck): string {
  return paletteEntry(SUIT_GLOW_ALPHA[deck], suit, FALLBACK_GLOW_ALPHA);
}

/** On-felt text ink for one suit on one deck. See {@link suitAccent}. */
export function suitInkOnFelt(suit: Suit, deck: CardDeck): string {
  return paletteEntry(SUIT_INK_ON_FELT[deck], suit, FALLBACK_INK_ON_FELT);
}

/**
 * The glyph for one suit, or an empty string for anything unrecognised.
 * Uses an own-property check so a prototype key like `toString` never reaches React as a child.
 */
export function suitGlyph(suit: Suit): string {
  return Object.hasOwn(SUIT_GLYPH, suit) ? SUIT_GLYPH[suit] : "";
}

/** i18n key for a suit's name in the given deck's vocabulary, used for both visible text and aria labels. */
export function suitNameKey(suit: Suit, deck: CardDeck): string {
  return `match.card.suit.${deck}.${suit}`;
}

/** i18n key for a rank's name, which is the same in every deck. */
export function rankNameKey(rank: Rank): string {
  return `match.card.rank.${rank}`;
}
