import type { Rank, Suit } from "./types";

export type CardBase = { rank: Rank; pts: number; strength: number };

export const TRUMP_ROWS: CardBase[] = [
  { rank: "J", pts: 20, strength: 1 },
  { rank: "9", pts: 14, strength: 2 },
  { rank: "A", pts: 11, strength: 3 },
  { rank: "10", pts: 10, strength: 4 },
  { rank: "K", pts: 4, strength: 5 },
  { rank: "Q", pts: 3, strength: 6 },
  { rank: "8", pts: 0, strength: 7 },
  { rank: "7", pts: 0, strength: 8 },
];

export const PLAIN_ROWS: CardBase[] = [
  { rank: "A", pts: 11, strength: 1 },
  { rank: "10", pts: 10, strength: 2 },
  { rank: "K", pts: 4, strength: 3 },
  { rank: "Q", pts: 3, strength: 4 },
  { rank: "J", pts: 2, strength: 5 },
  { rank: "9", pts: 0, strength: 6 },
  { rank: "8", pts: 0, strength: 7 },
  { rank: "7", pts: 0, strength: 8 },
];

export const TRUMP_SUIT: Suit = "C";
export const PLAIN_SUIT: Suit = "H";

type DeclarationBase = {
  id: string;
  pts: number;
  cards: Rank[];
  suit: Suit;
  set: boolean;
  wins: boolean;
};

export const DECLARATIONS_BASE: DeclarationBase[] = [
  { id: "belot", pts: 20, cards: ["K", "Q"], suit: TRUMP_SUIT, set: false, wins: false },
  { id: "terca", pts: 20, cards: ["7", "8", "9"], suit: "H", set: false, wins: false },
  { id: "kvarta", pts: 50, cards: ["9", "10", "J", "Q"], suit: "S", set: false, wins: false },
  { id: "kvinta", pts: 100, cards: ["10", "J", "Q", "K", "A"], suit: "D", set: false, wins: false },
  { id: "carre", pts: 100, cards: ["A", "A", "A", "A"], suit: "S", set: true, wins: false },
  { id: "carre9", pts: 150, cards: ["9", "9", "9", "9"], suit: "S", set: true, wins: false },
  { id: "carreJ", pts: 200, cards: ["J", "J", "J", "J"], suit: "S", set: true, wins: false },
  { id: "bela", pts: 1001, cards: [], suit: TRUMP_SUIT, set: false, wins: true },
];
