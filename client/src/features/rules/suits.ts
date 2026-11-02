import type { Suit } from "./content/types";

export const SUIT_GLYPH: Record<Suit, string> = { S: "♠︎", H: "♥︎", D: "♦︎", C: "♣︎" };

export const SUIT_IS_RED: Record<Suit, boolean> = { S: false, C: false, H: true, D: true };
